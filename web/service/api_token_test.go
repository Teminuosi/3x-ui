package service

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// Upstream 3x-ui (MHSanaei >= v3.11) stores hex(sha256(token)) plus scope /
// expires_at columns. A panel switched to this fork with update.sh keeps those
// rows; the tokens the user already holds must keep working, the digest itself
// must not, and upstream's scope / expiry limits must still apply.

func setupApiTokenTestDB(t *testing.T, upstreamColumns bool) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Fatal(err)
		}
	})
	if upstreamColumns {
		db := database.GetDB()
		for _, sql := range []string{
			"ALTER TABLE api_tokens ADD COLUMN scope text NOT NULL DEFAULT 'admin'",
			"ALTER TABLE api_tokens ADD COLUMN expires_at integer NOT NULL DEFAULT 0",
		} {
			if err := db.Exec(sql).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
}

// insertUpstreamToken writes a row the way upstream v3.11 does: only the digest.
func insertUpstreamToken(t *testing.T, name, plaintext, scope string, expiresAt int64, enabled bool) int {
	t.Helper()
	db := database.GetDB()
	if err := db.Exec("INSERT INTO api_tokens (name, token, enabled, created_at, scope, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		name, hashTokenSHA256(plaintext), enabled, time.Now().Unix(), scope, expiresAt).Error; err != nil {
		t.Fatal(err)
	}
	var id int
	if err := db.Raw("SELECT id FROM api_tokens WHERE name = ?", name).Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func TestMatchAcceptsUpstreamHashedToken(t *testing.T) {
	setupApiTokenTestDB(t, true)
	const old = "cg6l3p6DupstreamPlaintextToken0123456789abcdefXY"
	insertUpstreamToken(t, "install", old, "admin", 0, true)
	s := &ApiTokenService{}

	if !s.Match(old) {
		t.Fatal("token issued by upstream must keep working after the switch")
	}
	if s.Match(hashTokenSHA256(old)) {
		t.Fatal("the stored digest must not authenticate by itself")
	}
	if s.Match(old + "x") || s.Match(strings.ToUpper(old)) || s.Match("wrong-token") {
		t.Fatal("wrong tokens must be rejected")
	}
	if s.Match("") || s.Match(" ") {
		t.Fatal("empty / blank tokens must be rejected")
	}
}

func TestMatchKeepsForkPlaintextTokens(t *testing.T) {
	setupApiTokenTestDB(t, false) // a fork-native database: no upstream columns
	s := &ApiTokenService{}
	tok, err := s.Create("install")
	if err != nil {
		t.Fatal(err)
	}
	if len(tok.Token) != apiTokenLength || tok.Hashed {
		t.Fatalf("fork token format changed: len=%d hashed=%v", len(tok.Token), tok.Hashed)
	}
	if !s.Match(tok.Token) {
		t.Fatal("fork plaintext token must keep working")
	}
	if s.Match(hashTokenSHA256(tok.Token)) {
		t.Fatal("digest of a fork token must not authenticate")
	}
	if s.Match(tok.Token[:len(tok.Token)-1]) || s.Match("") {
		t.Fatal("prefix / empty must be rejected")
	}
}

func TestMatchMixedRowsAfterSwitch(t *testing.T) {
	setupApiTokenTestDB(t, true)
	const old = "upstreamAdminToken-ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	insertUpstreamToken(t, "install", old, "admin", 0, true)
	s := &ApiTokenService{}
	forkTok, err := s.Create("tms")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Match(old) || !s.Match(forkTok.Token) {
		t.Fatal("both the upstream token and a token created after the switch must work")
	}
}

func TestMatchAppliesUpstreamScopeAndExpiry(t *testing.T) {
	setupApiTokenTestDB(t, true)
	s := &ApiTokenService{}
	now := time.Now().UnixMilli()
	cases := []struct {
		name, plain, scope string
		expires            int64
		enabled            bool
		want               bool
	}{
		{"admin-noexp", "tok-admin-noexp-0123456789", "admin", 0, true, true},
		{"admin-future", "tok-admin-future-0123456789", "admin", now + 3600_000, true, true},
		{"admin-expired", "tok-admin-expired-0123456789", "admin", now - 1000, true, false},
		{"monitor", "tok-monitor-0123456789", "monitor", 0, true, false},
		{"node-sync", "tok-nodesync-0123456789", "node-sync", 0, true, false},
		{"disabled", "tok-disabled-0123456789", "admin", 0, false, false},
	}
	for _, c := range cases {
		insertUpstreamToken(t, c.name, c.plain, c.scope, c.expires, c.enabled)
	}
	for _, c := range cases {
		if got := s.Match(c.plain); got != c.want {
			t.Errorf("%s: Match=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestViewAndFirstUsableNeverExposeDigest(t *testing.T) {
	setupApiTokenTestDB(t, true)
	const old = "upstreamOnlyToken-0123456789abcdefghijklmnopqrstu"
	insertUpstreamToken(t, "install", old, "admin", 0, true)
	s := &ApiTokenService{}

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Token != "" || !list[0].Hashed {
		t.Fatalf("upstream row must be listed without its digest: %#v", list)
	}

	// Only digests on the panel: -getApiToken must mint a usable fork token, not print the digest.
	tok, err := s.FirstUsable()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token == "" || tok.Hashed || tok.Name == "install" || !s.Match(tok.Token) {
		t.Fatalf("FirstUsable must create a working fork token under a free name: %#v", tok)
	}
	// Second call returns the same token, doesn't keep minting.
	again, err := s.FirstUsable()
	if err != nil {
		t.Fatal(err)
	}
	if again.Token != tok.Token {
		t.Fatal("FirstUsable must be stable once a usable token exists")
	}
	if !s.Match(old) {
		t.Fatal("the upstream token must still work after FirstUsable")
	}
}

func TestFirstUsableOnForkPanelUnchanged(t *testing.T) {
	setupApiTokenTestDB(t, false)
	s := &ApiTokenService{}
	// No tokens: same as before, mints "install".
	tok, err := s.FirstUsable()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Name != "install" || !s.Match(tok.Token) {
		t.Fatalf("empty fork panel must get an install token: %#v", tok)
	}
	again, _ := s.FirstUsable()
	if again.Token != tok.Token {
		t.Fatal("existing fork token must be returned as-is")
	}
	var n int64
	database.GetDB().Model(model.ApiToken{}).Count(&n)
	if n != 1 {
		t.Fatalf("no extra tokens should be created, have %d", n)
	}

	// Every token disabled on purpose: keep the old behavior (print the first row),
	// never mint a fresh admin token behind the user's back.
	if err := s.SetEnabled(1, false); err != nil {
		t.Fatal(err)
	}
	dis, err := s.FirstUsable()
	if err != nil {
		t.Fatal(err)
	}
	database.GetDB().Model(model.ApiToken{}).Count(&n)
	if dis.Token != tok.Token || n != 1 {
		t.Fatalf("disabled-only fork panel must not get a new token (have %d rows)", n)
	}
	if s.Match(tok.Token) {
		t.Fatal("disabled token must not authenticate")
	}
}

func TestFirstUsablePrefersEnabledPlaintextAfterSwitch(t *testing.T) {
	setupApiTokenTestDB(t, true)
	insertUpstreamToken(t, "install", "upstream-token-0123456789abcdefghij", "admin", 0, true)
	s := &ApiTokenService{}
	a, _ := s.Create("a")
	b, _ := s.Create("b")
	if err := s.SetEnabled(a.Id, false); err != nil {
		t.Fatal(err)
	}
	got, err := s.FirstUsable()
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != b.Token {
		t.Fatal("switched panel: the first enabled plaintext token should be printed")
	}
}

func TestMatchRejectsEmptyUpstreamScope(t *testing.T) {
	setupApiTokenTestDB(t, true)
	insertUpstreamToken(t, "weird", "tok-empty-scope-0123456789", "", 0, true)
	if (&ApiTokenService{}).Match("tok-empty-scope-0123456789") {
		t.Fatal("an upstream row with an empty scope must not authenticate (upstream rejects it too)")
	}
}

func TestIsUpstreamHashedToken(t *testing.T) {
	if !isUpstreamHashedToken(hashTokenSHA256("x")) {
		t.Fatal("sha256 hex must be recognised")
	}
	for _, s := range []string{"", "abc", strings.Repeat("A", 64), strings.Repeat("g", 64), strings.Repeat("a", 63), strings.Repeat("a", 65)} {
		if isUpstreamHashedToken(s) {
			t.Fatalf("%q must not be treated as a digest", s)
		}
	}
}
