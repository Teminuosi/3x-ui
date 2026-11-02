package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/util/common"
	"github.com/mhsanaei/3x-ui/v3/util/random"
)

type ApiTokenService struct{}

const apiTokenLength = 48

type ApiTokenView struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt"`
	// Hashed marks a token migrated from upstream 3x-ui (MHSanaei >= v3.11), which
	// only keeps the SHA-256 of the token: the plaintext can't be shown, but the
	// token the user already holds keeps working.
	Hashed bool `json:"hashed,omitempty"`
}

func toView(t *model.ApiToken) *ApiTokenView {
	v := &ApiTokenView{
		Id:        t.Id,
		Name:      t.Name,
		Token:     t.Token,
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt,
	}
	if isUpstreamHashedToken(t.Token) {
		// The stored value is a digest, not a credential: never hand it out as one.
		v.Token = ""
		v.Hashed = true
	}
	return v
}

// isUpstreamHashedToken reports whether a stored api_tokens.token is an upstream
// SHA-256 digest (64 lowercase hex chars). This fork's own tokens are 48-char
// random.Seq strings, so the two formats never collide.
//
// Why this exists: upstream 3x-ui v3.11 stores only crypto.HashTokenSHA256(token)
// and verifies by hashing the presented token, while this fork compared the
// presented token to the stored value verbatim. After switching an upstream
// panel to this fork with update.sh every existing token (TMS client adoptions,
// scripts) got 404 — and the digest itself became usable as a bearer token.
func isUpstreamHashedToken(stored string) bool {
	if len(stored) != sha256.Size*2 {
		return false
	}
	for i := 0; i < len(stored); i++ {
		c := stored[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// hashTokenSHA256 is upstream's crypto.HashTokenSHA256: hex(sha256(token)).
func hashTokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// upstreamTokenAllowed applies upstream's own checks to a migrated row: only
// admin scope (this fork has no read-only / node-sync scopes, so anything
// narrower is refused rather than widened to full access) and not past expiry
// (upstream expires_at is unix milliseconds, 0 = never). A database without
// those columns was never upstream's, so it has no limits to apply.
func upstreamTokenAllowed(id int) bool {
	db := database.GetDB()
	m := db.Migrator()
	if m.HasColumn(&model.ApiToken{}, "scope") {
		var scope string
		if err := db.Raw("SELECT scope FROM api_tokens WHERE id = ?", id).Scan(&scope).Error; err != nil {
			return false
		}
		if scope != "admin" { // upstream's own IsKnownApiScope("") is false too
			return false
		}
	}
	if m.HasColumn(&model.ApiToken{}, "expires_at") {
		var expiresAt int64
		if err := db.Raw("SELECT expires_at FROM api_tokens WHERE id = ?", id).Scan(&expiresAt).Error; err != nil {
			return false
		}
		if expiresAt != 0 && time.Now().UnixMilli() >= expiresAt {
			return false
		}
	}
	return true
}

func (s *ApiTokenService) List() ([]*ApiTokenView, error) {
	db := database.GetDB()
	var rows []*model.ApiToken
	if err := db.Model(model.ApiToken{}).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*ApiTokenView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toView(r))
	}
	return out, nil
}

// FirstUsable picks the token `x-ui setting -getApiToken true` prints (install.sh,
// x-ui.sh and the TMS client read it):
//   - a fork-native panel (no upstream digests): exactly the old behavior — the
//     first row, even if disabled (a user who disabled every token must not get a
//     new admin token minted behind their back);
//   - a panel switched from upstream: the first enabled plaintext row, else the
//     first plaintext row; digests are never printed (they don't authenticate);
//   - no plaintext row at all (empty table, or digests only): mint one, as before
//     on an empty table.
func (s *ApiTokenService) FirstUsable() (*ApiTokenView, error) {
	db := database.GetDB()
	var rows []*model.ApiToken
	if err := db.Model(model.ApiToken{}).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	var plain []*model.ApiToken
	hasDigest := false
	for _, r := range rows {
		if isUpstreamHashedToken(r.Token) {
			hasDigest = true
		} else {
			plain = append(plain, r)
		}
	}
	if len(plain) > 0 {
		if hasDigest {
			for _, r := range plain {
				if r.Enabled {
					return toView(r), nil
				}
			}
		}
		return toView(plain[0]), nil
	}
	name := "install"
	for i := 2; ; i++ {
		var count int64
		if err := db.Model(model.ApiToken{}).Where("name = ?", name).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			break
		}
		name = "install-" + strconv.Itoa(i)
	}
	return s.Create(name)
}

func (s *ApiTokenService) Create(name string) (*ApiTokenView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, common.NewError("token name is required")
	}
	if len(name) > 64 {
		return nil, common.NewError("token name must be 64 characters or fewer")
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(model.ApiToken{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewError("a token with that name already exists")
	}
	row := &model.ApiToken{
		Name:    name,
		Token:   random.Seq(apiTokenLength),
		Enabled: true,
	}
	if err := db.Create(row).Error; err != nil {
		return nil, err
	}
	return toView(row), nil
}

func (s *ApiTokenService) Delete(id int) error {
	if id <= 0 {
		return common.NewError("invalid token id")
	}
	db := database.GetDB()
	return db.Where("id = ?", id).Delete(model.ApiToken{}).Error
}

func (s *ApiTokenService) SetEnabled(id int, enabled bool) error {
	if id <= 0 {
		return common.NewError("invalid token id")
	}
	db := database.GetDB()
	res := db.Model(model.ApiToken{}).Where("id = ?", id).Update("enabled", enabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("token not found")
	}
	return nil
}

// Match returns true when the presented bearer token matches any enabled
// row in api_tokens. Uses constant-time compare per row so a remote
// attacker can't time-attack tokens byte-by-byte.
//
// Two stored formats are accepted: this fork's plaintext (compared as-is) and
// upstream's SHA-256 digest (compared against the digest of the presented token,
// with upstream's scope / expiry rules). A digest row never matches the digest
// string itself, so the database value is not a credential.
func (s *ApiTokenService) Match(presented string) bool {
	if presented == "" {
		return false
	}
	db := database.GetDB()
	var rows []*model.ApiToken
	if err := db.Model(model.ApiToken{}).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return false
	}
	presentedBytes := []byte(presented)
	presentedDigest := []byte(hashTokenSHA256(presented))
	matched := false
	for _, r := range rows {
		if isUpstreamHashedToken(r.Token) {
			if subtle.ConstantTimeCompare([]byte(r.Token), presentedDigest) == 1 && upstreamTokenAllowed(r.Id) {
				matched = true
			}
			continue
		}
		if subtle.ConstantTimeCompare([]byte(r.Token), presentedBytes) == 1 {
			matched = true
		}
	}
	return matched
}
