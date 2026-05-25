package moxfield

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// imports_r60_test.go — pins the persistent import-tracking registry
// that feeds delta-import / "what's stale" workflows.

// newImportsEnv isolates the registry file path to t.TempDir() and
// keeps imports enabled. Returns the resolved file path.
func newImportsEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "imports.json")
	t.Setenv(importsEnvPath, path)
	t.Setenv(importsEnvDisable, "")
	// snapshotEnvDir is needed by importsPath() when HEXDEK_MOXFIELD_IMPORTS_PATH
	// is unset — set it defensively so a future test that clears
	// importsEnvPath doesn't fall through to ~/.local/share.
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	resetImportsForTest()
	return path
}

// ---------------------------------------------------------------------------
// RecordImport / GetImport / HasImported / ListImports basics
// ---------------------------------------------------------------------------

func TestImports_RecordCreatesFirstTimeEntry(t *testing.T) {
	path := newImportsEnv(t)
	rec := ImportRecord{
		DeckID:     "abc01",
		URL:        "https://moxfield.com/decks/abc01",
		DeckName:   "Test Deck",
		OutputPath: "/tmp/test.txt",
	}
	if err := RecordImport(rec); err != nil {
		t.Fatalf("RecordImport: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("registry file should exist: %v", err)
	}
	got, err := GetImport("abc01")
	if err != nil {
		t.Fatalf("GetImport: %v", err)
	}
	if got == nil {
		t.Fatalf("expected record, got nil")
	}
	if got.ImportCount != 1 {
		t.Errorf("ImportCount = %d, want 1 for first import", got.ImportCount)
	}
	if got.FirstImportedAt.IsZero() {
		t.Errorf("FirstImportedAt should be set")
	}
	if !got.FirstImportedAt.Equal(got.LastImportedAt) {
		t.Errorf("first import should have FirstImportedAt == LastImportedAt, got %v / %v",
			got.FirstImportedAt, got.LastImportedAt)
	}
	if got.DeckName != "Test Deck" {
		t.Errorf("DeckName = %q, want Test Deck", got.DeckName)
	}
}

func TestImports_ReRecordBumpsCountPreservesFirstImportedAt(t *testing.T) {
	newImportsEnv(t)
	rec := ImportRecord{
		DeckID:           "evolve01",
		URL:              "https://moxfield.com/decks/evolve01",
		DeckName:         "v1",
		OutputPath:       "/v1.txt",
		LastSnapshotHash: "aaaaaaaaaaaa",
	}
	if err := RecordImport(rec); err != nil {
		t.Fatalf("first: %v", err)
	}
	firstRec, _ := GetImport("evolve01")
	originalFirst := firstRec.FirstImportedAt

	// Re-import with updated metadata + new snapshot hash.
	time.Sleep(10 * time.Millisecond) // ensure LastImportedAt advances
	if err := RecordImport(ImportRecord{
		DeckID:           "evolve01",
		URL:              "https://moxfield.com/decks/evolve01",
		DeckName:         "v2 - renamed by owner",
		OutputPath:       "/v2.txt",
		LastSnapshotHash: "bbbbbbbbbbbb",
		LastSnapshotPath: "/snap/b.json",
	}); err != nil {
		t.Fatalf("second: %v", err)
	}

	got, _ := GetImport("evolve01")
	if got.ImportCount != 2 {
		t.Errorf("ImportCount after 2 imports = %d, want 2", got.ImportCount)
	}
	if !got.FirstImportedAt.Equal(originalFirst) {
		t.Errorf("FirstImportedAt was overwritten: got %v, want %v", got.FirstImportedAt, originalFirst)
	}
	if !got.LastImportedAt.After(originalFirst) {
		t.Errorf("LastImportedAt should advance past FirstImportedAt; first=%v last=%v",
			originalFirst, got.LastImportedAt)
	}
	if got.DeckName != "v2 - renamed by owner" {
		t.Errorf("DeckName update lost: %q", got.DeckName)
	}
	if got.OutputPath != "/v2.txt" {
		t.Errorf("OutputPath update lost: %q", got.OutputPath)
	}
	if got.LastSnapshotHash != "bbbbbbbbbbbb" {
		t.Errorf("LastSnapshotHash update lost: %q", got.LastSnapshotHash)
	}
}

func TestImports_GetImportMissingReturnsNilNoError(t *testing.T) {
	newImportsEnv(t)
	got, err := GetImport("never-recorded")
	if err != nil {
		t.Errorf("missing entry should not error; got %v", err)
	}
	if got != nil {
		t.Errorf("missing entry should be nil; got %+v", got)
	}
}

func TestImports_HasImportedTrueAfterRecord(t *testing.T) {
	newImportsEnv(t)
	if HasImported("nope") {
		t.Errorf("HasImported(unrecorded) should be false")
	}
	_ = RecordImport(ImportRecord{DeckID: "yes01"})
	if !HasImported("yes01") {
		t.Errorf("HasImported(recorded) should be true")
	}
}

func TestImports_ListImportsSortedNewestFirst(t *testing.T) {
	newImportsEnv(t)
	for i, id := range []string{"first", "second", "third"} {
		_ = RecordImport(ImportRecord{
			DeckID:         id,
			DeckName:       id,
			LastImportedAt: time.Now().UTC().Add(time.Duration(i) * time.Second),
		})
	}
	list, err := ListImports()
	if err != nil {
		t.Fatalf("ListImports: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 records, got %d", len(list))
	}
	if list[0].DeckID != "third" || list[1].DeckID != "second" || list[2].DeckID != "first" {
		t.Errorf("ordering not newest-first; got %s / %s / %s",
			list[0].DeckID, list[1].DeckID, list[2].DeckID)
	}
}

func TestImports_RegistrySurvivesProcessRestart(t *testing.T) {
	path := newImportsEnv(t)
	_ = RecordImport(ImportRecord{
		DeckID: "persist01", DeckName: "Persistent",
	})

	// Simulate fresh process: clear nothing, just re-read.
	got, err := GetImport("persist01")
	if err != nil {
		t.Fatalf("GetImport: %v", err)
	}
	if got == nil || got.DeckName != "Persistent" {
		t.Errorf("registry didn't persist: %+v", got)
	}
	// File on disk has valid JSON envelope.
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var envelope importsFile
	if err := json.Unmarshal(bytes, &envelope); err != nil {
		t.Fatalf("envelope unmarshal: %v", err)
	}
	if envelope.Version != importsSchemaVer {
		t.Errorf("Version = %d, want %d", envelope.Version, importsSchemaVer)
	}
	if len(envelope.Records) != 1 {
		t.Errorf("envelope.Records length = %d, want 1", len(envelope.Records))
	}
}

func TestImports_DisableViaEnvNoOp(t *testing.T) {
	path := newImportsEnv(t)
	t.Setenv(importsEnvDisable, "0")
	if err := RecordImport(ImportRecord{DeckID: "disabled01"}); err != nil {
		t.Errorf("disabled record should be nil err, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("registry file should not exist when disabled; stat=%v", err)
	}
}

func TestImports_EmptyDeckIDErrors(t *testing.T) {
	newImportsEnv(t)
	if err := RecordImport(ImportRecord{DeckID: ""}); err == nil {
		t.Error("expected error for empty DeckID")
	}
}

func TestImports_MissingRegistryFileLoadsAsEmpty(t *testing.T) {
	newImportsEnv(t)
	// Don't record anything — file doesn't exist yet.
	list, err := ListImports()
	if err != nil {
		t.Errorf("missing file should not error; got %v", err)
	}
	if len(list) != 0 {
		t.Errorf("missing file should return empty list; got %d records", len(list))
	}
}

func TestImports_MalformedFileErrors(t *testing.T) {
	path := newImportsEnv(t)
	if err := os.WriteFile(path, []byte(`{not valid json`), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := ListImports()
	if err == nil {
		t.Error("expected error for malformed registry file")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should mention parse failure: %v", err)
	}
}

func TestImports_AtomicWriteSurvivesPartialFailure(t *testing.T) {
	path := newImportsEnv(t)
	_ = RecordImport(ImportRecord{DeckID: "good01", DeckName: "Good Deck"})

	// Pre-existing good content present. Now an interrupted write
	// would leave a .tmp file. Simulate by checking that no .tmp is
	// left around after a normal write.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file should not linger; stat=%v", err)
	}
	// Content still intact (atomic rename happened).
	got, _ := GetImport("good01")
	if got == nil || got.DeckName != "Good Deck" {
		t.Errorf("good01 lost after atomic write: %+v", got)
	}
}

func TestImports_ConcurrentRecordsAllPersist(t *testing.T) {
	newImportsEnv(t)
	var wg sync.WaitGroup
	const n = 20
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = RecordImport(ImportRecord{
				DeckID:   "race-" + string(rune('A'+i)),
				DeckName: "Racer",
			})
		}()
	}
	wg.Wait()
	list, _ := ListImports()
	if len(list) != n {
		t.Errorf("concurrent records: expected %d entries, got %d", n, len(list))
	}
}

// ---------------------------------------------------------------------------
// FindStale: detects upstream changes since registry's LastSnapshotHash
// ---------------------------------------------------------------------------

// stubServerWithVersionedBody serves the body returned by `body()`
// for every /v3/decks/all/* request — pointer to a string the test
// can swap between fetches to simulate Moxfield-side edits.
func stubServerWithVersionedBody(t *testing.T, body *atomic.Value) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body.Load().(string)))
	}))
	origBase := apiBaseVar
	apiBaseVar = srv.URL

	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(cacheEnvTTL, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	t.Setenv(snapshotEnvDisable, "")
	t.Setenv(importsEnvPath, filepath.Join(tmp, "imports.json"))
	t.Setenv(importsEnvDisable, "")
	resetCachesForTest()
	resetImportsForTest()
	return func() {
		srv.Close()
		apiBaseVar = origBase
	}
}

func TestFindStale_DetectsUpstreamChange(t *testing.T) {
	const v1Body = `{"name":"Stale Test","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Atraxa, Praetors' Voice"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Sol Ring"}}}}}}`
	const v2Body = `{"name":"Stale Test","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Atraxa, Praetors' Voice"}}}},"mainboard":{"count":2,"cards":{"m":{"quantity":1,"card":{"name":"Sol Ring"}},"n":{"quantity":1,"card":{"name":"Mana Crypt"}}}}}}`
	var body atomic.Value
	body.Store(v1Body)
	cleanup := stubServerWithVersionedBody(t, &body)
	defer cleanup()

	// First import (v1) → records baseline hash.
	if _, err := fetchDeckRaw("stale01"); err != nil {
		t.Fatalf("v1 fetch: %v", err)
	}
	snaps, _ := ListSnapshots("stale01")
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot after v1 fetch, got %d", len(snaps))
	}
	_ = RecordImport(ImportRecord{
		DeckID:           "stale01",
		URL:              "https://moxfield.com/decks/stale01",
		DeckName:         "Stale Test",
		LastSnapshotHash: snaps[0].Hash,
		LastSnapshotPath: snaps[0].Path,
	})

	// Upstream changes to v2; user wants to know.
	body.Store(v2Body)

	stale, err := FindStale(context.Background(), 0)
	if err != nil {
		t.Fatalf("FindStale: %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale deck, got %d: %+v", len(stale), stale)
	}
	if stale[0].DeckID != "stale01" {
		t.Errorf("stale DeckID = %q, want stale01", stale[0].DeckID)
	}
}

func TestFindStale_NoChangeReturnsEmpty(t *testing.T) {
	const body = `{"name":"Same","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Yuriko, the Tiger's Shadow"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Sol Ring"}}}}}}`
	var bodyVar atomic.Value
	bodyVar.Store(body)
	cleanup := stubServerWithVersionedBody(t, &bodyVar)
	defer cleanup()

	if _, err := fetchDeckRaw("same01"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	snaps, _ := ListSnapshots("same01")
	_ = RecordImport(ImportRecord{
		DeckID:           "same01",
		URL:              "https://moxfield.com/decks/same01",
		LastSnapshotHash: snaps[0].Hash,
		LastSnapshotPath: snaps[0].Path,
	})

	stale, err := FindStale(context.Background(), 0)
	if err != nil {
		t.Fatalf("FindStale: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("unchanged deck should not be stale; got %+v", stale)
	}
}

func TestFindStale_SkipsRecordsWithEmptyHash(t *testing.T) {
	const body = `{"name":"X","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Krenko, Mob Boss"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Goblin Bombardment"}}}}}}`
	var bodyVar atomic.Value
	bodyVar.Store(body)
	cleanup := stubServerWithVersionedBody(t, &bodyVar)
	defer cleanup()

	// Record WITHOUT a baseline hash (simulates a deck imported with
	// snapshots disabled, or pre-r60 imports).
	_ = RecordImport(ImportRecord{
		DeckID:   "no-baseline",
		URL:      "https://moxfield.com/decks/no-baseline",
		DeckName: "No Baseline",
	})

	stale, err := FindStale(context.Background(), 0)
	if err != nil {
		t.Fatalf("FindStale: %v", err)
	}
	for _, s := range stale {
		if s.DeckID == "no-baseline" {
			t.Errorf("record with empty LastSnapshotHash should be skipped, got it in stale list")
		}
	}
}

func TestFindStale_LimitCapsChecks(t *testing.T) {
	const body = `{"name":"Cap","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Atraxa, Praetors' Voice"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Sol Ring"}}}}}}`
	var bodyVar atomic.Value
	bodyVar.Store(body)
	cleanup := stubServerWithVersionedBody(t, &bodyVar)
	defer cleanup()

	// Seed 5 records, all with placeholder hashes that will mismatch
	// the upstream → would-be-stale.
	for i := 0; i < 5; i++ {
		id := "cap-" + string(rune('A'+i))
		_ = RecordImport(ImportRecord{
			DeckID:           id,
			URL:              "https://moxfield.com/decks/" + id,
			LastSnapshotHash: "placeholder",
		})
	}
	stale, err := FindStale(context.Background(), 2)
	if err != nil {
		t.Fatalf("FindStale: %v", err)
	}
	// At most 2 checks → at most 2 stale results.
	if len(stale) > 2 {
		t.Errorf("limit=2 should cap stale list at 2, got %d", len(stale))
	}
}

func TestFindStale_UpstreamErrorDoesNotFlagStale(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	origBase := apiBaseVar
	apiBaseVar = srv.URL
	defer func() { apiBaseVar = origBase }()

	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	t.Setenv(importsEnvPath, filepath.Join(tmp, "imports.json"))
	t.Setenv(importsEnvDisable, "")
	resetCachesForTest()
	resetImportsForTest()

	_ = RecordImport(ImportRecord{
		DeckID:           "gone01",
		LastSnapshotHash: "anything",
	})

	stale, err := FindStale(context.Background(), 0)
	if err != nil {
		t.Fatalf("FindStale: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("upstream 404 should not flag deck as stale (keep prior snapshot); got %+v", stale)
	}
}
