package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// migrationModels is the FK-aware order in which tables are created and copied.
// Parents come before their children so foreign-key constraints stay satisfied
// even when checks are not explicitly disabled.
func migrationModels() []any {
	return []any{
		&model.User{},
		&model.Setting{},
		&model.HistoryOfSeeders{},
		&model.CustomGeoResource{},
		&model.Node{},
		&model.ApiToken{},
		&model.Inbound{},
		&xray.ClientTraffic{},
		&model.OutboundTraffics{},
		&model.InboundClientIps{},
		&model.ClientRecord{},
		&model.ClientInbound{},
		&model.InboundFallback{},
	}
}

// MigrateData copies every row from the configured SQLite file at srcPath into
// a fresh PostgreSQL database described by dstDSN. The destination tables are
// (re)created with AutoMigrate before the copy. Source data is left untouched.
func MigrateData(srcPath, dstDSN string) error {
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("source sqlite not found at %s: %w", srcPath, err)
	}
	if dstDSN == "" {
		return errors.New("destination DSN is required")
	}

	if err := os.MkdirAll(path.Dir(srcPath), 0755); err != nil {
		return err
	}

	srcDSN := srcPath + "?_journal_mode=WAL&_busy_timeout=10000"
	src, err := gorm.Open(sqlite.Open(srcDSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return fmt.Errorf("open sqlite source: %w", err)
	}
	srcSQL, err := src.DB()
	if err != nil {
		return err
	}
	defer srcSQL.Close()

	dst, err := gorm.Open(postgres.Open(dstDSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return fmt.Errorf("open postgres destination: %w", err)
	}
	dstSQL, err := dst.DB()
	if err != nil {
		return err
	}
	defer dstSQL.Close()
	dstSQL.SetConnMaxLifetime(time.Hour)

	log.Println("Creating destination schema...")
	for _, m := range migrationModels() {
		if err := dst.AutoMigrate(m); err != nil {
			return fmt.Errorf("AutoMigrate %T: %w", m, err)
		}
	}

	restricted, err := restrictedUpstreamTokenIDs(src)
	if err != nil {
		return fmt.Errorf("read api_tokens limits: %w", err)
	}

	totalRows := 0
	for _, m := range migrationModels() {
		n, err := copyTable(src, dst, m)
		if err != nil {
			return fmt.Errorf("copy %T: %w", m, err)
		}
		if _, ok := m.(*model.ApiToken); ok && len(restricted) > 0 {
			res := dst.Where("id IN ?", restricted).Delete(&model.ApiToken{})
			if res.Error != nil {
				return fmt.Errorf("drop restricted api tokens: %w", res.Error)
			}
			n -= int(res.RowsAffected)
			log.Printf("  skipped %d api token(s) from upstream with a non-admin scope or an expiry date", res.RowsAffected)
		}
		totalRows += n
		log.Printf("  %-32s %d rows", reflect.TypeOf(m).Elem().Name(), n)
	}

	if err := resetPostgresSequences(dst); err != nil {
		log.Printf("warning: failed to reset some postgres sequences: %v", err)
	}

	log.Printf("Migration complete: %d rows across %d tables.", totalRows, len(migrationModels()))
	log.Println("Set XUI_DB_TYPE=postgres and XUI_DB_DSN=... in /etc/default/x-ui, then restart x-ui.")
	return nil
}

// restrictedUpstreamTokenIDs lists api_tokens rows inherited from upstream
// (stored as hex sha256) that upstream itself would refuse as an admin token:
// scope other than "admin", or any expiry at all (expires_at unix ms, 0 =
// never; NULL is refused at runtime too). The destination table is built from
// this fork's model, which has neither column, so copying these rows would
// silently widen them to full, never-expiring access.
func restrictedUpstreamTokenIDs(src *gorm.DB) ([]int, error) {
	m := src.Migrator()
	hasScope := m.HasColumn(&model.ApiToken{}, "scope")
	hasExpiry := m.HasColumn(&model.ApiToken{}, "expires_at")
	if !hasScope && !hasExpiry {
		return nil, nil
	}
	cols := "id, token"
	if hasScope {
		cols += ", scope"
	} else {
		cols += ", 'admin' AS scope"
	}
	if hasExpiry {
		cols += ", COALESCE(expires_at, -1) AS expires_at"
	} else {
		cols += ", 0 AS expires_at"
	}
	var rows []struct {
		Id        int
		Token     string
		Scope     string
		ExpiresAt int64
	}
	if err := src.Raw("SELECT " + cols + " FROM api_tokens").Scan(&rows).Error; err != nil {
		return nil, err
	}
	var ids []int
	for _, r := range rows {
		if !isSHA256Hex(r.Token) {
			continue
		}
		if r.Scope != "admin" || r.ExpiresAt != 0 {
			ids = append(ids, r.Id)
		}
	}
	return ids, nil
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// copyTable streams every row of `mdl` from src to dst in batches.
func copyTable(src, dst *gorm.DB, mdl any) (int, error) {
	sliceType := reflect.SliceOf(reflect.PointerTo(reflect.TypeOf(mdl).Elem()))
	batchPtr := reflect.New(sliceType)
	batchPtr.Elem().Set(reflect.MakeSlice(sliceType, 0, 0))

	total := 0
	err := src.Model(mdl).FindInBatches(batchPtr.Interface(), 500, func(tx *gorm.DB, _ int) error {
		batch := batchPtr.Elem()
		if batch.Len() == 0 {
			return nil
		}
		if err := dst.CreateInBatches(batchPtr.Interface(), 200).Error; err != nil {
			return err
		}
		total += batch.Len()
		return nil
	}).Error
	return total, err
}

// resetPostgresSequences advances each table's id sequence past MAX(id),
// otherwise the next INSERT-without-id would clash with copied rows.
func resetPostgresSequences(dst *gorm.DB) error {
	tables := []string{
		"users", "inbounds", "outbound_traffics", "settings", "inbound_client_ips",
		"client_traffics", "history_of_seeders", "custom_geo_resources", "nodes",
		"api_tokens", "client_records", "client_inbounds", "inbound_fallback_children",
	}
	for _, t := range tables {
		// setval is a no-op if the table or its id sequence doesn't exist; we ignore errors per-table.
		_ = dst.Exec(fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE((SELECT MAX(id) FROM "%s"), 1), true)
			 WHERE pg_get_serial_sequence('%s','id') IS NOT NULL`,
			t, t, t,
		)).Error
	}
	return nil
}
