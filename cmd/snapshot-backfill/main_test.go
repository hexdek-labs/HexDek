package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// Test_parseDeckKey covers the canonical shapes the import path
// produces, plus the malformed inputs the runner has to gracefully
// skip rather than panic on.
func Test_parseDeckKey(t *testing.T) {
	for _, tc := range []struct {
		in        string
		wantOwner string
		wantID    string
	}{
		{"moxfield/ajani_b3_alice_aBcD", "moxfield", "ajani_b3_alice_aBcD"},
		{"alice/my_deck_v3", "alice", "my_deck_v3"},
		// ID itself can't contain a slash per sanitizeFilename, but
		// the parser shouldn't choke if a future schema relaxes
		// that — first slash wins.
		{"o/path/with/extra", "o", "path/with/extra"},
		{"", "", ""},
		{"no-slash", "", ""},
		{"/leading", "", "leading"},
		{"trailing/", "trailing", ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			o, i := parseDeckKey(tc.in)
			if o != tc.wantOwner || i != tc.wantID {
				t.Errorf("parseDeckKey(%q) = (%q, %q), want (%q, %q)",
					tc.in, o, i, tc.wantOwner, tc.wantID)
			}
		})
	}
}

// Test_resolveDeckPath confirms .txt is preferred over .json when
// both exist (mirrors findDeckFile order), and that absent files
// return "" rather than a stale path.
func Test_resolveDeckPath(t *testing.T) {
	dir := t.TempDir()
	ownerDir := filepath.Join(dir, "alice")
	if err := os.MkdirAll(ownerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ownerDir, "txt_only.txt"), "x")
	writeFile(t, filepath.Join(ownerDir, "json_only.json"), "{}")
	writeFile(t, filepath.Join(ownerDir, "both.txt"), "x")
	writeFile(t, filepath.Join(ownerDir, "both.json"), "{}")

	for _, tc := range []struct {
		id   string
		want string
	}{
		{"txt_only", filepath.Join(ownerDir, "txt_only.txt")},
		{"json_only", filepath.Join(ownerDir, "json_only.json")},
		{"both", filepath.Join(ownerDir, "both.txt")},
		{"missing", ""},
	} {
		got := resolveDeckPath(dir, "alice", tc.id)
		if got != tc.want {
			t.Errorf("resolveDeckPath(%q): got %q want %q", tc.id, got, tc.want)
		}
	}
}

// stubInvoker stands in for the real hexdek-freya CLI in tests. It
// writes a canned .profile.json sidecar at the expected location so
// the runner's post-invoke read path executes exactly as it does in
// production.
type stubInvoker struct {
	sidecarContent string
	calls          []string
	shouldFail     bool
	skipSidecar    bool // if true, "succeed" without writing the sidecar (covers freyaFailed branch)
}

func (s *stubInvoker) Invoke(_ context.Context, deckPath string) error {
	s.calls = append(s.calls, deckPath)
	if s.shouldFail {
		return fmt.Errorf("stub freya failure")
	}
	if s.skipSidecar {
		return nil
	}
	freyaDir := filepath.Join(filepath.Dir(deckPath), "freya")
	if err := os.MkdirAll(freyaDir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(deckPath)
	for _, ext := range []string{".txt", ".json"} {
		base = stripSuffix(base, ext)
	}
	sidecar := filepath.Join(freyaDir, base+".profile.json")
	return os.WriteFile(sidecar, []byte(s.sidecarContent), 0o644)
}

func stripSuffix(s, suf string) string {
	if len(s) >= len(suf) && s[len(s)-len(suf):] == suf {
		return s[:len(s)-len(suf)]
	}
	return s
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// canonicalSidecar is a fixture matching the contract pinned by
// Test_SaveProfileJSON_EmitsExpectedScalars in cmd/hexdek-freya.
// Tests parse this through the real db.FreyaProfileFromJSON, so any
// future renaming on the freya side breaks both tests in tandem.
const canonicalSidecar = `{
  "deck_name": "edgar_markov_b2_alice",
  "commander": "Edgar Markov",
  "primary_archetype": "Aggro / Tribal",
  "secondary_archetype": "Tokens",
  "bracket": 2,
  "commander_synergy": 0.625,
  "power_percentile": 47,
  "power_tier_counts": {"S": 2, "A": 8, "B": 35, "C": 24, "D": 6},
  "top_roles": [
    {"role": "threat", "count": 22},
    {"role": "ramp", "count": 11}
  ]
}`

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// SQLite :memory: databases are per-connection — schema only
	// lives on the connection that ran the initial CREATE TABLE
	// pass. The concurrency test exercises a worker pool larger
	// than 1; without pinning to a single connection the second
	// worker gets a fresh empty in-memory DB and "no such table"
	// surfaces. Production code talks to a file-backed DB where
	// every connection sees the schema, so the pin is test-only.
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func seedShowmatchRow(t *testing.T, sqlDB *sql.DB, deckKey, commander, owner string) {
	t.Helper()
	_, err := sqlDB.Exec(
		`INSERT INTO showmatch_elo (deck_key, commander, owner, updated_at)
		 VALUES (?, ?, ?, unixepoch())`,
		deckKey, commander, owner)
	if err != nil {
		t.Fatalf("seed showmatch_elo: %v", err)
	}
}

// Test_runBackfill_SidecarOnlyPath confirms a deck whose sidecar
// already exists on disk gets upserted without ever calling the
// freya invoker. This is the cheap path the production backfill
// will hit for the bulk of the 1,319 r60 decks (they all have
// sidecars after PR #517 lands).
func Test_runBackfill_SidecarOnlyPath(t *testing.T) {
	decksDir := t.TempDir()
	// Pre-write the sidecar; intentionally NO deck .txt — confirms
	// the sidecar path doesn't depend on the deck file being present
	// (the canonical flow is "sidecar fresh from a prior Freya run").
	writeFile(t, filepath.Join(decksDir, "alice", "freya", "my_deck.profile.json"), canonicalSidecar)

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/my_deck", "Edgar Markov", "alice")

	stub := &stubInvoker{}
	cfg := backfillConfig{DecksDir: decksDir}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, stub)
	if err != nil {
		t.Fatalf("runBackfill: %v", err)
	}

	if len(stub.calls) != 0 {
		t.Errorf("freya invoked %d times for sidecar-only path; want 0 (%v)", len(stub.calls), stub.calls)
	}
	if stats.upsertedSidecar.Load() != 1 {
		t.Errorf("upsertedSidecar=%d want 1; stats=%s", stats.upsertedSidecar.Load(), stats)
	}

	// Confirm the row landed with the right scalars (not just a
	// blank row with the right deck_key).
	got, err := db.LoadFreyaProfile(context.Background(), sqlDB, "alice/my_deck")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.PrimaryArchetype != "Aggro / Tribal" || got.Bracket != 2 || got.SynergyPct != 62.5 {
		t.Errorf("loaded profile mismatch: %+v", got)
	}
}

// Test_runBackfill_InvokesFreyaWhenSidecarMissing covers the
// expensive branch: no sidecar, but a deck file exists → invoke
// freya, expect a sidecar to materialize, then upsert.
func Test_runBackfill_InvokesFreyaWhenSidecarMissing(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "my_deck.txt"), "1 Sol Ring\n")

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/my_deck", "Edgar Markov", "alice")

	stub := &stubInvoker{sidecarContent: canonicalSidecar}
	cfg := backfillConfig{DecksDir: decksDir}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, stub)
	if err != nil {
		t.Fatalf("runBackfill: %v", err)
	}

	if len(stub.calls) != 1 {
		t.Errorf("freya invoked %d times; want 1 (%v)", len(stub.calls), stub.calls)
	}
	if stats.upsertedFreya.Load() != 1 {
		t.Errorf("upsertedFreya=%d want 1; stats=%s", stats.upsertedFreya.Load(), stats)
	}
}

// Test_runBackfill_IsIdempotent confirms the second invocation skips
// every deck whose profile is already populated (no Freya re-runs,
// no DB write).
func Test_runBackfill_IsIdempotent(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "freya", "my_deck.profile.json"), canonicalSidecar)

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/my_deck", "Edgar Markov", "alice")

	stub := &stubInvoker{}
	cfg := backfillConfig{DecksDir: decksDir}

	// First pass populates the row.
	if _, err := runBackfill(context.Background(), sqlDB, cfg, stub); err != nil {
		t.Fatal(err)
	}
	// Re-write the sidecar with DIFFERENT content so a (wrong)
	// second-pass upsert would change the persisted scalars.
	writeFile(t, filepath.Join(decksDir, "alice", "freya", "my_deck.profile.json"),
		`{"primary_archetype": "Different", "bracket": 5, "commander_synergy": 0.99}`)

	stats, err := runBackfill(context.Background(), sqlDB, cfg, stub)
	if err != nil {
		t.Fatal(err)
	}
	if stats.skippedExists.Load() != 1 {
		t.Errorf("second pass skipped=%d want 1; stats=%s", stats.skippedExists.Load(), stats)
	}
	got, err := db.LoadFreyaProfile(context.Background(), sqlDB, "alice/my_deck")
	if err != nil {
		t.Fatal(err)
	}
	if got.PrimaryArchetype == "Different" {
		t.Error("second pass overwrote the row without --force")
	}
	if got.PrimaryArchetype != "Aggro / Tribal" {
		t.Errorf("first-pass row was lost: %+v", got)
	}
}

// Test_runBackfill_ForceFlagReprocesses confirms --force bypasses
// the "already populated" skip and re-reads the sidecar (so a
// changed deck → re-run → updated row workflow works).
func Test_runBackfill_ForceFlagReprocesses(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "freya", "my_deck.profile.json"), canonicalSidecar)

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/my_deck", "Edgar Markov", "alice")

	cfg := backfillConfig{DecksDir: decksDir}
	if _, err := runBackfill(context.Background(), sqlDB, cfg, nil); err != nil {
		t.Fatal(err)
	}

	// Mutate the sidecar to reflect a "fresh Freya re-analysis" with
	// a new archetype call.
	writeFile(t, filepath.Join(decksDir, "alice", "freya", "my_deck.profile.json"),
		`{"primary_archetype": "Updated", "bracket": 4, "commander_synergy": 0.5}`)

	cfg.Force = true
	stats, err := runBackfill(context.Background(), sqlDB, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.skippedExists.Load() != 0 {
		t.Errorf("--force still skipped: stats=%s", stats)
	}
	got, _ := db.LoadFreyaProfile(context.Background(), sqlDB, "alice/my_deck")
	if got.PrimaryArchetype != "Updated" || got.Bracket != 4 {
		t.Errorf("--force did not refresh row: %+v", got)
	}
}

// Test_runBackfill_DryRunWritesNothing confirms --dry-run reports
// what would happen but neither calls freya nor writes to the DB.
func Test_runBackfill_DryRunWritesNothing(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "my_deck.txt"), "x")

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/my_deck", "Edgar Markov", "alice")

	stub := &stubInvoker{sidecarContent: canonicalSidecar}
	cfg := backfillConfig{DecksDir: decksDir, DryRun: true}
	if _, err := runBackfill(context.Background(), sqlDB, cfg, stub); err != nil {
		t.Fatal(err)
	}
	if len(stub.calls) != 0 {
		t.Errorf("dry-run invoked freya %d times; want 0", len(stub.calls))
	}
	_, err := db.LoadFreyaProfile(context.Background(), sqlDB, "alice/my_deck")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("dry-run wrote a row (load err=%v want ErrNoRows)", err)
	}
}

// Test_runBackfill_OwnerFilter confirms the --owner flag restricts
// processing to a single owner partition.
func Test_runBackfill_OwnerFilter(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "freya", "deck_a.profile.json"), canonicalSidecar)
	writeFile(t, filepath.Join(decksDir, "bob", "freya", "deck_b.profile.json"), canonicalSidecar)

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/deck_a", "X", "alice")
	seedShowmatchRow(t, sqlDB, "bob/deck_b", "Y", "bob")

	cfg := backfillConfig{DecksDir: decksDir, OwnerFilter: "alice"}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.considered.Load() != 1 {
		t.Errorf("considered=%d want 1 (filter should drop bob); stats=%s",
			stats.considered.Load(), stats)
	}
	// Alice's row landed.
	if _, err := db.LoadFreyaProfile(context.Background(), sqlDB, "alice/deck_a"); err != nil {
		t.Errorf("alice row missing: %v", err)
	}
	// Bob's didn't.
	if _, err := db.LoadFreyaProfile(context.Background(), sqlDB, "bob/deck_b"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("bob row should not exist; err=%v", err)
	}
}

// Test_runBackfill_LimitCaps confirms --limit truncates the input
// set. With a 5-row seed and limit=2 the runner should process
// exactly 2 rows.
func Test_runBackfill_LimitCaps(t *testing.T) {
	decksDir := t.TempDir()
	sqlDB := openTestDB(t)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("alice/deck_%d", i)
		writeFile(t, filepath.Join(decksDir, "alice", "freya",
			fmt.Sprintf("deck_%d.profile.json", i)), canonicalSidecar)
		seedShowmatchRow(t, sqlDB, key, "X", "alice")
	}
	cfg := backfillConfig{DecksDir: decksDir, Limit: 2}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.considered.Load() != 2 {
		t.Errorf("considered=%d want 2; stats=%s", stats.considered.Load(), stats)
	}
}

// Test_runBackfill_MissingDeckCounted confirms a deck whose .txt /
// .json AND sidecar are both absent counts toward missing_deck_file
// and doesn't fail the whole run.
func Test_runBackfill_MissingDeckCounted(t *testing.T) {
	decksDir := t.TempDir()
	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/never_imported", "X", "alice")

	stub := &stubInvoker{sidecarContent: canonicalSidecar}
	cfg := backfillConfig{DecksDir: decksDir}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, stub)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.missingDeck.Load() != 1 {
		t.Errorf("missingDeck=%d want 1; stats=%s", stats.missingDeck.Load(), stats)
	}
	if len(stub.calls) != 0 {
		t.Errorf("freya invoked %d times; want 0 when no deck file", len(stub.calls))
	}
}

// Test_runBackfill_FreyaFailureCounted confirms the runner survives
// an invoker error: counted toward freyaFailed, run continues to
// subsequent rows.
func Test_runBackfill_FreyaFailureCounted(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "bad_deck.txt"), "x")
	writeFile(t, filepath.Join(decksDir, "alice", "good_deck.txt"), "x")

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/bad_deck", "X", "alice")
	seedShowmatchRow(t, sqlDB, "alice/good_deck", "Y", "alice")

	stub := &stubInvoker{sidecarContent: canonicalSidecar, shouldFail: true}
	cfg := backfillConfig{DecksDir: decksDir}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, stub)
	if err != nil {
		t.Fatal(err)
	}
	// Both decks fail (stub fails every call).
	if stats.freyaFailed.Load() != 2 {
		t.Errorf("freyaFailed=%d want 2; stats=%s", stats.freyaFailed.Load(), stats)
	}
	// No row written for either.
	for _, k := range []string{"alice/bad_deck", "alice/good_deck"} {
		if _, err := db.LoadFreyaProfile(context.Background(), sqlDB, k); !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("%s: should not exist; err=%v", k, err)
		}
	}
}

// Test_runBackfill_FreyaSucceedsButNoSidecar covers the defensive
// branch where the invoker returns nil but the sidecar still isn't
// on disk afterward (theoretical Freya bug or interrupted write).
// Counts as freya_failed without crashing.
func Test_runBackfill_FreyaSucceedsButNoSidecar(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "deck.txt"), "x")

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/deck", "X", "alice")

	stub := &stubInvoker{skipSidecar: true}
	cfg := backfillConfig{DecksDir: decksDir}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, stub)
	if err != nil {
		t.Fatal(err)
	}
	if stats.freyaFailed.Load() != 1 {
		t.Errorf("freyaFailed=%d want 1; stats=%s", stats.freyaFailed.Load(), stats)
	}
}

// Test_runBackfill_SidecarOnlyMode (invoker == nil) confirms the
// runner reports missing decks rather than panicking when --sidecar-
// only is in effect and a deck has no profile sidecar.
func Test_runBackfill_SidecarOnlyMode(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "with_sidecar.txt"), "x")
	writeFile(t, filepath.Join(decksDir, "alice", "freya", "with_sidecar.profile.json"), canonicalSidecar)
	writeFile(t, filepath.Join(decksDir, "alice", "no_sidecar.txt"), "x")

	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/with_sidecar", "X", "alice")
	seedShowmatchRow(t, sqlDB, "alice/no_sidecar", "Y", "alice")

	cfg := backfillConfig{DecksDir: decksDir}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.upsertedSidecar.Load() != 1 {
		t.Errorf("upsertedSidecar=%d want 1; stats=%s", stats.upsertedSidecar.Load(), stats)
	}
	if stats.missingDeck.Load() != 1 {
		t.Errorf("missingDeck=%d want 1 (no_sidecar should count missing without an invoker); stats=%s",
			stats.missingDeck.Load(), stats)
	}
}

// Test_runBackfill_MalformedDeckKeyCounted defends against a stray
// row in showmatch_elo whose deck_key violates the "owner/id" shape
// (corruption, manual edit, etc.). Should be counted as filter-
// skipped without panic.
func Test_runBackfill_MalformedDeckKeyCounted(t *testing.T) {
	decksDir := t.TempDir()
	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "no-slash-key", "X", "alice")
	cfg := backfillConfig{DecksDir: decksDir}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.skippedFilter.Load() != 1 {
		t.Errorf("skippedFilter=%d want 1; stats=%s", stats.skippedFilter.Load(), stats)
	}
}

// Test_runBackfill_ParseFailureCounted confirms a sidecar that
// exists but contains junk JSON counts toward parse_failed without
// crashing.
func Test_runBackfill_ParseFailureCounted(t *testing.T) {
	decksDir := t.TempDir()
	writeFile(t, filepath.Join(decksDir, "alice", "freya", "deck.profile.json"), `not json {`)
	sqlDB := openTestDB(t)
	seedShowmatchRow(t, sqlDB, "alice/deck", "X", "alice")
	cfg := backfillConfig{DecksDir: decksDir}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.parseFailed.Load() != 1 {
		t.Errorf("parseFailed=%d want 1; stats=%s", stats.parseFailed.Load(), stats)
	}
}

// Test_runBackfill_ConcurrencyDoesntCorrupt is a smoke test for the
// worker pool: with concurrency=4 and 20 rows all having sidecars,
// every row should land exactly once in deck_freya_profile.
func Test_runBackfill_ConcurrencyDoesntCorrupt(t *testing.T) {
	decksDir := t.TempDir()
	sqlDB := openTestDB(t)
	const n = 20
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("alice/deck_%02d", i)
		writeFile(t, filepath.Join(decksDir, "alice", "freya",
			fmt.Sprintf("deck_%02d.profile.json", i)), canonicalSidecar)
		seedShowmatchRow(t, sqlDB, key, "X", "alice")
	}
	cfg := backfillConfig{DecksDir: decksDir, Concurrency: 4}
	stats, err := runBackfill(context.Background(), sqlDB, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.upsertedSidecar.Load() != n {
		t.Errorf("upsertedSidecar=%d want %d; stats=%s", stats.upsertedSidecar.Load(), n, stats)
	}
	// Confirm via DB count.
	var got int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM deck_freya_profile`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != n {
		t.Errorf("deck_freya_profile rowcount=%d want %d", got, n)
	}
}

// Test_resolveFreyaBin_ExplicitMissing confirms an explicit path
// that doesn't exist resolves to "" (so main() degrades to sidecar-
// only rather than handing exec.Command a bogus path that fails
// per-deck and bloats the freya_failed counter).
func Test_resolveFreyaBin_ExplicitMissing(t *testing.T) {
	got := resolveFreyaBin("/nonexistent/freya")
	if got != "" {
		t.Errorf("resolveFreyaBin('/nonexistent/...') = %q; want \"\"", got)
	}
}
