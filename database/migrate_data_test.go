package database

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func hexSHA(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func openTokenDB(t *testing.T, ddl string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "src.db")), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("ddl: %v", err)
	}
	t.Cleanup(func() {
		if s, err := db.DB(); err == nil {
			_ = s.Close()
		}
	})
	return db
}

func TestRestrictedUpstreamTokenIDs_UpstreamColumns(t *testing.T) {
	db := openTokenDB(t, `CREATE TABLE api_tokens (id integer primary key, name text, token text, enabled numeric, created_at integer, scope text not null default 'admin', expires_at integer not null default 0)`)
	past := time.Now().Add(-time.Hour).UnixMilli()
	future := time.Now().Add(time.Hour).UnixMilli()
	rows := []struct {
		id      int
		token   string
		scope   string
		expires int64
	}{
		{1, hexSHA("a"), "admin", 0},                                        // kept
		{2, hexSHA("b"), "monitor", 0},                                      // dropped: scope
		{3, hexSHA("c"), "node-sync", 0},                                    // dropped: scope
		{4, hexSHA("d"), "", 0},                                             // dropped: empty scope
		{5, hexSHA("e"), "admin", past},                                     // dropped: expired
		{6, hexSHA("f"), "admin", future},                                   // dropped: target schema can't keep the expiry
		{7, "plainForkToken123456789012345678901234567890ab", "monitor", 0}, // plaintext: not upstream, kept
	}
	for _, r := range rows {
		if err := db.Exec(`INSERT INTO api_tokens (id, name, token, enabled, created_at, scope, expires_at) VALUES (?, ?, ?, 1, 0, ?, ?)`,
			r.id, "n"+string(rune('0'+r.id)), r.token, r.scope, r.expires).Error; err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	ids, err := restrictedUpstreamTokenIDs(db)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	sort.Ints(ids)
	want := []int{2, 3, 4, 5, 6}
	if len(ids) != len(want) {
		t.Fatalf("got %v want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got %v want %v", ids, want)
		}
	}
}

func TestRestrictedUpstreamTokenIDs_NullColumns(t *testing.T) {
	db := openTokenDB(t, `CREATE TABLE api_tokens (id integer primary key, name text, token text, enabled numeric, created_at integer, scope text, expires_at integer)`)
	if err := db.Exec(`INSERT INTO api_tokens (id, name, token, enabled, created_at, scope, expires_at) VALUES
		(1, 'a', ?, 1, 0, 'admin', 0), (2, 'b', ?, 1, 0, 'admin', NULL), (3, 'c', ?, 1, 0, NULL, 0)`,
		hexSHA("a"), hexSHA("b"), hexSHA("c")).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	ids, err := restrictedUpstreamTokenIDs(db)
	sort.Ints(ids)
	if err != nil || len(ids) != 2 || ids[0] != 2 || ids[1] != 3 {
		t.Fatalf("NULL scope / expires_at must be dropped, got %v %v", ids, err)
	}
}

func TestRestrictedUpstreamTokenIDs_ForkSchemaUntouched(t *testing.T) {
	db := openTokenDB(t, `CREATE TABLE api_tokens (id integer primary key, name text, token text, enabled numeric, created_at integer)`)
	if err := db.Exec(`INSERT INTO api_tokens (id, name, token, enabled, created_at) VALUES (1, 'x', ?, 1, 0)`, hexSHA("old")).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	ids, err := restrictedUpstreamTokenIDs(db)
	if err != nil || len(ids) != 0 {
		t.Fatalf("fork schema must yield no restricted ids, got %v %v", ids, err)
	}
}
