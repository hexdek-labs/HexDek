package auth

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

// apikey_lifecycle_test.go — SQLite-backed lifecycle. Reuses
// openTestDB + seedDevice from session_lifecycle_r60_test.go.

// --- IssueAPIKey ----------------------------------------------------------

func TestIssueAPIKey_ReturnsPlaintextAndRow(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, key, err := IssueAPIKey(context.Background(), d, alice, "ci-runner", 0)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !IsAPIKeyShape(pt) {
		t.Errorf("returned plaintext %q failed shape check", pt)
	}
	if key.DeviceID != alice {
		t.Errorf("key.DeviceID = %q, want %q", key.DeviceID, alice)
	}
	if key.Name != "ci-runner" {
		t.Errorf("key.Name = %q, want ci-runner", key.Name)
	}
	if key.KeyHash != HashAPIKey(pt) {
		t.Errorf("key.KeyHash doesn't match HashAPIKey(plaintext)")
	}
	if key.RevokedAt != 0 || key.ExpiresAt != 0 {
		t.Errorf("fresh key should be active: revoked=%d expires=%d",
			key.RevokedAt, key.ExpiresAt)
	}
}

func TestIssueAPIKey_PlaintextNeverPersisted(t *testing.T) {
	// Critical security pin: the plaintext must NOT appear in any
	// column of the api_key table. Search every text column for the
	// plaintext string; finding it would mean we accidentally stored
	// it (e.g. someone wired Name = plaintext as a "convenience").
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, _, err := IssueAPIKey(context.Background(), d, alice, "no-leak-here", 0)
	if err != nil {
		t.Fatal(err)
	}
	var id, devID, name, hash string
	if err := d.QueryRowContext(context.Background(),
		`SELECT id, device_id, name, key_hash FROM api_key WHERE device_id = ?`, alice).
		Scan(&id, &devID, &name, &hash); err != nil {
		t.Fatalf("read back: %v", err)
	}
	for label, val := range map[string]string{"id": id, "device_id": devID, "name": name, "key_hash": hash} {
		if strings.Contains(val, pt) {
			t.Errorf("plaintext leaked into %s column: %q contains %q", label, val, pt)
		}
	}
}

func TestIssueAPIKey_WithExpiry(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	before := time.Now().Unix()
	_, key, err := IssueAPIKey(context.Background(), d, alice, "30s-key", 30)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().Unix()
	if key.ExpiresAt < before+30 || key.ExpiresAt > after+30 {
		t.Errorf("expires_at = %d, want ~%d (before=%d after=%d)",
			key.ExpiresAt, before+30, before, after)
	}
}

func TestIssueAPIKey_EmptyDeviceRejected(t *testing.T) {
	d := openTestDB(t)
	if _, _, err := IssueAPIKey(context.Background(), d, "", "x", 0); err == nil {
		t.Error("empty deviceID should error")
	}
}

// --- ValidateAPIKey -------------------------------------------------------

func TestValidateAPIKey_AcceptsFreshKey(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, issued, _ := IssueAPIKey(context.Background(), d, alice, "k", 0)
	got, err := ValidateAPIKey(context.Background(), d, pt)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.ID != issued.ID {
		t.Errorf("got.ID = %q, want %q", got.ID, issued.ID)
	}
	if got.DeviceID != alice {
		t.Errorf("got.DeviceID = %q, want %q", got.DeviceID, alice)
	}
}

func TestValidateAPIKey_TouchesLastUsed(t *testing.T) {
	// Validation should update last_used_at as a side effect — that's
	// the audit-trail value of having the column.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, issued, _ := IssueAPIKey(context.Background(), d, alice, "k", 0)
	if issued.LastUsedAt != 0 {
		t.Fatalf("baseline last_used_at = %d, want 0", issued.LastUsedAt)
	}
	got, err := ValidateAPIKey(context.Background(), d, pt)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastUsedAt == 0 {
		t.Errorf("last_used_at should be set after validate")
	}
	// Re-read the row to confirm the side-effect actually persisted.
	var stored int64
	_ = d.QueryRowContext(context.Background(),
		`SELECT last_used_at FROM api_key WHERE id = ?`, issued.ID).Scan(&stored)
	if stored == 0 {
		t.Errorf("last_used_at not persisted to DB")
	}
}

func TestValidateAPIKey_RejectsWrongPlaintext(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, _, _ = IssueAPIKey(context.Background(), d, alice, "k", 0)
	// Try with a key-shaped but unrelated plaintext.
	bogus, _, _, _ := GenerateAPIKey()
	if _, err := ValidateAPIKey(context.Background(), d, bogus); err != ErrInvalidAPIKey {
		t.Errorf("validate unknown key: want ErrInvalidAPIKey, got %v", err)
	}
}

func TestValidateAPIKey_RejectsBadShape(t *testing.T) {
	d := openTestDB(t)
	for _, bad := range []string{
		"", "not-a-key", "hxk_short",
		strings.Repeat("a", 96), // session-shape — should reject
	} {
		if _, err := ValidateAPIKey(context.Background(), d, bad); err != ErrInvalidAPIKey {
			t.Errorf("validate %q: want ErrInvalidAPIKey, got %v", bad, err)
		}
	}
}

func TestValidateAPIKey_RejectsExpired(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, key, _ := IssueAPIKey(context.Background(), d, alice, "k", 0)
	// Hand-stamp expires_at to the past.
	_, err := d.ExecContext(context.Background(),
		`UPDATE api_key SET expires_at = 1 WHERE id = ?`, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAPIKey(context.Background(), d, pt); err != ErrAPIKeyExpired {
		t.Errorf("validate expired: want ErrAPIKeyExpired, got %v", err)
	}
}

func TestValidateAPIKey_RejectsRevoked(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, key, _ := IssueAPIKey(context.Background(), d, alice, "k", 0)
	if err := RevokeAPIKey(context.Background(), d, key.ID, alice); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAPIKey(context.Background(), d, pt); err != ErrAPIKeyRevoked {
		t.Errorf("validate revoked: want ErrAPIKeyRevoked, got %v", err)
	}
}

// --- ListAPIKeys ----------------------------------------------------------

func TestListAPIKeys_ReturnsNewestFirst(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, k1, _ := IssueAPIKey(context.Background(), d, alice, "first", 0)
	// Stamp the second key's created_at one second later so the
	// newest-first sort is deterministic regardless of test speed.
	_, k2, _ := IssueAPIKey(context.Background(), d, alice, "second", 0)
	if _, err := d.ExecContext(context.Background(),
		`UPDATE api_key SET created_at = ? WHERE id = ?`, k1.CreatedAt+10, k2.ID); err != nil {
		t.Fatal(err)
	}

	list, err := ListAPIKeys(context.Background(), d, alice)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 keys, got %d", len(list))
	}
	if list[0].ID != k2.ID || list[1].ID != k1.ID {
		t.Errorf("order wrong: got %q,%q want %q,%q",
			list[0].ID, list[1].ID, k2.ID, k1.ID)
	}
}

func TestListAPIKeys_DevicesIsolated(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	bob := seedDevice(t, d, "device_bob", "Bob")
	_, _, _ = IssueAPIKey(context.Background(), d, alice, "ak", 0)
	_, _, _ = IssueAPIKey(context.Background(), d, bob, "bk1", 0)
	_, _, _ = IssueAPIKey(context.Background(), d, bob, "bk2", 0)

	aList, _ := ListAPIKeys(context.Background(), d, alice)
	bList, _ := ListAPIKeys(context.Background(), d, bob)
	if len(aList) != 1 {
		t.Errorf("alice list = %d, want 1", len(aList))
	}
	if len(bList) != 2 {
		t.Errorf("bob list = %d, want 2", len(bList))
	}
}

func TestListAPIKeys_EmptyDeviceRejected(t *testing.T) {
	d := openTestDB(t)
	if _, err := ListAPIKeys(context.Background(), d, ""); err == nil {
		t.Error("empty deviceID should error so a typo can't enumerate the whole table")
	}
}

func TestListAPIKeys_IncludesRevokedAndExpired(t *testing.T) {
	// List is for management UI — must show inactive keys so the user
	// can see "your CI key was revoked on Jan 12".
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, k1, _ := IssueAPIKey(context.Background(), d, alice, "active", 0)
	_, k2, _ := IssueAPIKey(context.Background(), d, alice, "revoked", 0)
	_, k3, _ := IssueAPIKey(context.Background(), d, alice, "expired", 0)
	_ = RevokeAPIKey(context.Background(), d, k2.ID, alice)
	_, _ = d.ExecContext(context.Background(),
		`UPDATE api_key SET expires_at = 1 WHERE id = ?`, k3.ID)

	list, _ := ListAPIKeys(context.Background(), d, alice)
	if len(list) != 3 {
		t.Errorf("list = %d keys, want 3 (active + revoked + expired)", len(list))
	}
	seen := map[string]bool{}
	for _, k := range list {
		seen[k.ID] = true
	}
	for _, id := range []string{k1.ID, k2.ID, k3.ID} {
		if !seen[id] {
			t.Errorf("missing key %q from list", id)
		}
	}
}

// --- RevokeAPIKey ---------------------------------------------------------

func TestRevokeAPIKey_StampsRevokedAt(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, key, _ := IssueAPIKey(context.Background(), d, alice, "k", 0)

	before := time.Now().Unix()
	if err := RevokeAPIKey(context.Background(), d, key.ID, alice); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Unix()

	var revokedAt int64
	_ = d.QueryRowContext(context.Background(),
		`SELECT revoked_at FROM api_key WHERE id = ?`, key.ID).Scan(&revokedAt)
	if revokedAt < before || revokedAt > after {
		t.Errorf("revoked_at = %d, want %d..%d", revokedAt, before, after)
	}
}

func TestRevokeAPIKey_RequiresDeviceMatch(t *testing.T) {
	// Pin the cross-device guard: alice MUST NOT be able to revoke
	// bob's key just by knowing its id.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	bob := seedDevice(t, d, "device_bob", "Bob")
	_, bobKey, _ := IssueAPIKey(context.Background(), d, bob, "bobs-key", 0)

	if err := RevokeAPIKey(context.Background(), d, bobKey.ID, alice); err != ErrInvalidAPIKey {
		t.Errorf("cross-device revoke: want ErrInvalidAPIKey, got %v", err)
	}
	// Bob's key should still validate.
	var revoked int64
	_ = d.QueryRowContext(context.Background(),
		`SELECT revoked_at FROM api_key WHERE id = ?`, bobKey.ID).Scan(&revoked)
	if revoked != 0 {
		t.Errorf("bob's key revoked despite cross-device attempt: revoked_at=%d", revoked)
	}
}

func TestRevokeAPIKey_IdempotentOnAlreadyRevoked(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, key, _ := IssueAPIKey(context.Background(), d, alice, "k", 0)
	if err := RevokeAPIKey(context.Background(), d, key.ID, alice); err != nil {
		t.Fatal(err)
	}
	// Second revoke must succeed (idempotent), not error.
	if err := RevokeAPIKey(context.Background(), d, key.ID, alice); err != nil {
		t.Errorf("double revoke: want nil, got %v", err)
	}
}

func TestRevokeAPIKey_UnknownIDRejected(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	if err := RevokeAPIKey(context.Background(), d, "deadbeef0000", alice); err != ErrInvalidAPIKey {
		t.Errorf("unknown id: want ErrInvalidAPIKey, got %v", err)
	}
}

func TestRevokeAPIKey_EmptyArgsRejected(t *testing.T) {
	d := openTestDB(t)
	if err := RevokeAPIKey(context.Background(), d, "", "alice"); err != ErrInvalidAPIKey {
		t.Errorf("empty id: want ErrInvalidAPIKey, got %v", err)
	}
	if err := RevokeAPIKey(context.Background(), d, "abc", ""); err != ErrInvalidAPIKey {
		t.Errorf("empty deviceID: want ErrInvalidAPIKey, got %v", err)
	}
}

// --- APIKey.Active --------------------------------------------------------

func TestAPIKey_Active(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		k    APIKey
		want bool
	}{
		{"fresh", APIKey{}, true},
		{"revoked", APIKey{RevokedAt: now.Unix()}, false},
		{"past expiry", APIKey{ExpiresAt: now.Unix() - 1}, false},
		{"at expiry boundary", APIKey{ExpiresAt: now.Unix()}, false}, // >= now → inactive
		{"future expiry", APIKey{ExpiresAt: now.Unix() + 3600}, true},
		{"no expiry", APIKey{ExpiresAt: 0}, true},
		{"revoked + future expiry → still inactive", APIKey{ExpiresAt: now.Unix() + 3600, RevokedAt: now.Unix()}, false},
	}
	for _, tc := range cases {
		if got := tc.k.Active(now); got != tc.want {
			t.Errorf("%s: Active() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// --- End-to-end lifecycle round-trip --------------------------------------

func TestAPIKey_LifecycleEndToEnd(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")

	// 1. Issue.
	pt, issued, err := IssueAPIKey(context.Background(), d, alice, "round-trip", 0)
	if err != nil {
		t.Fatal(err)
	}
	if pt == "" {
		t.Fatal("no plaintext returned")
	}

	// 2. Validate the freshly-issued key.
	got, err := ValidateAPIKey(context.Background(), d, pt)
	if err != nil {
		t.Fatalf("validate fresh: %v", err)
	}
	if got.ID != issued.ID {
		t.Errorf("validated id = %q, want %q", got.ID, issued.ID)
	}

	// 3. List shows the active key.
	list, _ := ListAPIKeys(context.Background(), d, alice)
	if len(list) != 1 || !list[0].Active(time.Now()) {
		t.Errorf("list: want 1 active key, got %+v", list)
	}

	// 4. Revoke.
	if err := RevokeAPIKey(context.Background(), d, issued.ID, alice); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// 5. Validation now rejects with ErrAPIKeyRevoked.
	if _, err := ValidateAPIKey(context.Background(), d, pt); err != ErrAPIKeyRevoked {
		t.Errorf("validate post-revoke: want ErrAPIKeyRevoked, got %v", err)
	}

	// 6. List still shows the (now-revoked) key — audit trail intact.
	list, _ = ListAPIKeys(context.Background(), d, alice)
	if len(list) != 1 {
		t.Errorf("list post-revoke: want 1 row (audit), got %d", len(list))
	}
	if list[0].Active(time.Now()) {
		t.Errorf("revoked key should report Active=false")
	}
}

// --- Boundary: validation after IssueAPIKey but before any other access --

func TestValidateAPIKey_ImmediatelyAfterIssue(t *testing.T) {
	// Defends against a race where validation runs before INSERT
	// commits — would manifest as "I just issued a key and it
	// doesn't work." Since SQLite is single-writer this shouldn't
	// happen, but the test is cheap and pins the contract.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	for i := 0; i < 10; i++ {
		pt, _, err := IssueAPIKey(context.Background(), d, alice, "x", 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateAPIKey(context.Background(), d, pt); err != nil {
			t.Fatalf("iteration %d: validate immediately failed: %v", i, err)
		}
	}
}

// _ = db.Now used as a marker that we depend on the canonical time
// source the rest of the package uses; lifts the unused-import guard
// if a future refactor drops the only direct call site.
var _ = db.Now

// _ = sql.ErrNoRows used to mark intentional dependency on the
// stdlib sentinel that ValidateAPIKey uses to distinguish "no row"
// from real DB errors.
var _ = sql.ErrNoRows
