package moxfield

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// snapshot_r60_test.go — pins the append-only deck-snapshot history
// that protects against Moxfield-source edits/deletions after import.
// Snapshots are captured every time fetchDeckRaw returns fresh API
// JSON (not on cache hits), deduped by content hash so back-to-back
// imports of an unchanged deck only land one file.

// newSnapshotEnv isolates the snapshot dir to t.TempDir(), keeps
// snapshots enabled, and returns the resolved dir path.
func newSnapshotEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv(snapshotEnvDir, tmp)
	t.Setenv(snapshotEnvDisable, "")
	return tmp
}

// ---------------------------------------------------------------------------
// SaveSnapshot / ListSnapshots unit tests
// ---------------------------------------------------------------------------

func TestSnapshot_SaveCreatesFileWithTimestampHashName(t *testing.T) {
	dir := newSnapshotEnv(t)
	raw := []byte(`{"name":"X","format":"commander"}`)

	s, err := SaveSnapshot("abc123", raw)
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	if s.Deduped {
		t.Errorf("first save should not be deduped")
	}
	if s.DeckID != "abc123" {
		t.Errorf("DeckID = %q, want abc123", s.DeckID)
	}
	if len(s.Hash) != snapshotHashLen {
		t.Errorf("Hash length = %d, want %d", len(s.Hash), snapshotHashLen)
	}
	if s.Size != int64(len(raw)) {
		t.Errorf("Size = %d, want %d", s.Size, len(raw))
	}
	// File on disk under tmp/abc123/.
	expectedDir := filepath.Join(dir, "abc123")
	if filepath.Dir(s.Path) != expectedDir {
		t.Errorf("Path dir = %q, want %q", filepath.Dir(s.Path), expectedDir)
	}
	if _, err := os.Stat(s.Path); err != nil {
		t.Errorf("file not on disk: %v", err)
	}
	// Filename format: {timestamp}_{hash}.json
	base := filepath.Base(s.Path)
	if !strings.HasSuffix(base, ".json") {
		t.Errorf("filename should end .json: %q", base)
	}
	if !strings.Contains(base, s.Hash) {
		t.Errorf("filename should embed hash %q: %q", s.Hash, base)
	}
}

func TestSnapshot_DedupesIdenticalContent(t *testing.T) {
	newSnapshotEnv(t)
	raw := []byte(`{"name":"DedupCheck"}`)

	first, err := SaveSnapshot("dedup", raw)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	if first.Deduped {
		t.Fatal("first save should not be deduped")
	}
	second, err := SaveSnapshot("dedup", raw)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if !second.Deduped {
		t.Errorf("second save with identical content should be deduped")
	}
	if second.Path != first.Path {
		t.Errorf("dedup should reuse existing path; got %q != %q", second.Path, first.Path)
	}
	// Only one file in the deck dir.
	snaps, err := ListSnapshots("dedup")
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("expected 1 file after dedup, got %d: %+v", len(snaps), snaps)
	}
}

func TestSnapshot_DifferentContentCreatesNewFile(t *testing.T) {
	newSnapshotEnv(t)

	first, _ := SaveSnapshot("evolve", []byte(`{"v":1}`))
	// Sleep 1s to ensure the second snapshot's timestamp differs at
	// second-precision (filename layout is 20060102T150405Z — second-
	// precision). Otherwise both files could collide on filename.
	time.Sleep(1100 * time.Millisecond)
	second, _ := SaveSnapshot("evolve", []byte(`{"v":2}`))

	if second.Deduped {
		t.Errorf("different content should not dedup")
	}
	if second.Hash == first.Hash {
		t.Errorf("different content should produce different hashes; both %q", first.Hash)
	}
	if second.Path == first.Path {
		t.Errorf("different content should land in different files; both %q", first.Path)
	}
	snaps, _ := ListSnapshots("evolve")
	if len(snaps) != 2 {
		t.Errorf("expected 2 files after evolution, got %d", len(snaps))
	}
}

func TestSnapshot_ListNewestFirst(t *testing.T) {
	newSnapshotEnv(t)
	_, _ = SaveSnapshot("order", []byte(`{"v":1}`))
	time.Sleep(1100 * time.Millisecond)
	_, _ = SaveSnapshot("order", []byte(`{"v":2}`))
	time.Sleep(1100 * time.Millisecond)
	_, _ = SaveSnapshot("order", []byte(`{"v":3}`))

	snaps, err := ListSnapshots("order")
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(snaps))
	}
	for i := 1; i < len(snaps); i++ {
		if snaps[i-1].Timestamp.Before(snaps[i].Timestamp) {
			t.Errorf("snapshots not newest-first: [%d]=%v before [%d]=%v",
				i-1, snaps[i-1].Timestamp, i, snaps[i].Timestamp)
		}
	}
}

func TestSnapshot_DifferentDecksIsolated(t *testing.T) {
	newSnapshotEnv(t)
	_, _ = SaveSnapshot("deckA", []byte(`{"deck":"A"}`))
	_, _ = SaveSnapshot("deckB", []byte(`{"deck":"B"}`))
	_, _ = SaveSnapshot("deckC", []byte(`{"deck":"C"}`))

	for _, id := range []string{"deckA", "deckB", "deckC"} {
		snaps, _ := ListSnapshots(id)
		if len(snaps) != 1 {
			t.Errorf("deck %s expected 1 snapshot, got %d", id, len(snaps))
		}
		if len(snaps) > 0 && snaps[0].DeckID != id {
			t.Errorf("snapshot DeckID = %q, want %q", snaps[0].DeckID, id)
		}
	}
}

func TestSnapshot_DisableViaEnvSilences(t *testing.T) {
	dir := newSnapshotEnv(t)
	t.Setenv(snapshotEnvDisable, "0")

	s, err := SaveSnapshot("nosnap", []byte(`{"v":1}`))
	if err != nil {
		t.Errorf("disabled save should return nil error, got: %v", err)
	}
	if s.Path != "" || s.Hash != "" {
		t.Errorf("disabled save should return zero Snapshot, got %+v", s)
	}
	// Nothing on disk.
	if _, err := os.Stat(filepath.Join(dir, "nosnap")); !os.IsNotExist(err) {
		t.Errorf("disabled snapshots must not create dirs; stat=%v", err)
	}
}

func TestSnapshot_EmptyDeckIDErrors(t *testing.T) {
	newSnapshotEnv(t)
	if _, err := SaveSnapshot("", []byte(`{}`)); err == nil {
		t.Error("expected error for empty deck ID")
	}
}

func TestSnapshot_EmptyRawJSONErrors(t *testing.T) {
	newSnapshotEnv(t)
	if _, err := SaveSnapshot("x", nil); err == nil {
		t.Error("expected error for nil rawJSON")
	}
	if _, err := SaveSnapshot("x", []byte{}); err == nil {
		t.Error("expected error for empty rawJSON")
	}
}

func TestSnapshot_LoadRoundTrip(t *testing.T) {
	newSnapshotEnv(t)
	original := []byte(`{"name":"Yuriko","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Yuriko, the Tiger's Shadow"}}}}}}`)
	s, err := SaveSnapshot("rt01", original)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadSnapshot(s.Path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if string(loaded) != string(original) {
		t.Errorf("round-trip mismatch: got %q want %q", loaded, original)
	}
}

func TestSnapshot_ListEmptyDeckReturnsNil(t *testing.T) {
	newSnapshotEnv(t)
	snaps, err := ListSnapshots("never-imported")
	if err != nil {
		t.Errorf("ListSnapshots on nonexistent deck should be nil err, got %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}
}

func TestSnapshot_PathSanitizationContained(t *testing.T) {
	dir := newSnapshotEnv(t)
	// Adversarial deck IDs (path traversal, dir separators) get
	// flattened to safe chars before joining.
	for _, evil := range []string{"../etc/passwd", "../../escape", "abc/def", "abc\\..\\def"} {
		got := snapshotDeckDir(evil)
		if got == "" {
			t.Errorf("snapshotDeckDir(%q) returned empty", evil)
			continue
		}
		rel, err := filepath.Rel(dir, got)
		if err != nil {
			t.Errorf("Rel: %v", err)
			continue
		}
		if rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
			t.Errorf("snapshotDeckDir(%q) escapes base dir: %s", evil, got)
		}
	}
}

// ---------------------------------------------------------------------------
// fetchDeckRaw integration: API hit → snapshot; cache hit → no new snapshot
// ---------------------------------------------------------------------------

// stubFetchDeckRaw spins an httptest server returning `body` for every
// /v3/decks/all/* request, isolates the moxfield cache + snapshot dir,
// and returns a hit counter + cleanup.
func stubFetchDeckRaw(t *testing.T, body string) (counter *int32, snapDir string, cleanup func()) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	origBase := apiBaseVar
	apiBaseVar = srv.URL

	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(cacheEnvTTL, "")
	snap := filepath.Join(tmp, "snapshots")
	t.Setenv(snapshotEnvDir, snap)
	t.Setenv(snapshotEnvDisable, "")

	resetCachesForTest()
	return &hits, snap, func() {
		srv.Close()
		apiBaseVar = origBase
	}
}

func TestSnapshot_FetchDeckRaw_APIHitCreatesSnapshot(t *testing.T) {
	body := `{"name":"Yuriko","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Yuriko, the Tiger's Shadow"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Sol Ring"}}}}}}`
	_, _, cleanup := stubFetchDeckRaw(t, body)
	defer cleanup()

	if _, err := fetchDeckRaw("apihit01"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	snaps, err := ListSnapshots("apihit01")
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("API hit should create 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Size != int64(len(body)) {
		t.Errorf("snapshot size = %d, want %d", snaps[0].Size, len(body))
	}
}

func TestSnapshot_FetchDeckRaw_CacheHitDoesNotSnapshot(t *testing.T) {
	body := `{"name":"X","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Atraxa, Praetors' Voice"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Sol Ring"}}}}}}`
	hits, _, cleanup := stubFetchDeckRaw(t, body)
	defer cleanup()

	// First fetch: API hit + 1 snapshot.
	if _, err := fetchDeckRaw("cachehit01"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("first fetch expected 1 HTTP hit, got %d", got)
	}
	snapsAfterFirst, _ := ListSnapshots("cachehit01")

	// Second fetch: in-proc cache hit → no new HTTP, no new snapshot.
	if _, err := fetchDeckRaw("cachehit01"); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("second fetch should be cache hit (0 new HTTP); got %d total", got)
	}
	snapsAfterSecond, _ := ListSnapshots("cachehit01")
	if len(snapsAfterSecond) != len(snapsAfterFirst) {
		t.Errorf("cache hit must not add a snapshot; before=%d after=%d",
			len(snapsAfterFirst), len(snapsAfterSecond))
	}
}

func TestSnapshot_FetchDeckRaw_DedupesAcrossDifferentImports(t *testing.T) {
	// Two separate processes (simulated by dropping the in-proc cache
	// between calls) fetching the SAME deck whose Moxfield content
	// hasn't changed should produce 1 snapshot total — the second
	// fetch's hash matches the first.
	body := `{"name":"Stable","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Krenko, Mob Boss"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Goblin Bombardment"}}}}}}`
	_, _, cleanup := stubFetchDeckRaw(t, body)
	defer cleanup()

	if _, err := fetchDeckRaw("stable01"); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Drop in-proc cache (simulate fresh process) but leave disk cache
	// in place so we don't accidentally re-trigger an API hit.
	inProcCache.Range(func(k, _ any) bool { inProcCache.Delete(k); return true })
	if _, err := fetchDeckRaw("stable01"); err != nil {
		t.Fatalf("second: %v", err)
	}

	snaps, _ := ListSnapshots("stable01")
	if len(snaps) != 1 {
		t.Errorf("unchanged deck across two fetches: expected 1 snapshot, got %d", len(snaps))
	}
}

func TestSnapshot_FetchDeckRaw_HTTPErrorDoesNotSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprintln(w, "boom")
	}))
	defer srv.Close()

	origBase := apiBaseVar
	apiBaseVar = srv.URL
	defer func() { apiBaseVar = origBase }()

	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	t.Setenv(snapshotEnvDisable, "")
	resetCachesForTest()

	if _, err := fetchDeckRaw("err01"); err == nil {
		t.Fatal("expected error on 500")
	}
	snaps, _ := ListSnapshots("err01")
	if len(snaps) != 0 {
		t.Errorf("failed fetch must not snapshot; got %d", len(snaps))
	}
}

// Refresh() invalidates cache layers. After Refresh + re-fetch with
// CHANGED content, a new snapshot lands (mirroring real "user edited
// the Moxfield deck, re-imported" flow).
func TestSnapshot_FetchDeckRaw_RefreshThenChangedContentNewSnapshot(t *testing.T) {
	v1 := `{"name":"Vers1","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Atraxa, Praetors' Voice"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Sol Ring"}}}}}}`
	v2 := `{"name":"Vers2","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Atraxa, Praetors' Voice"}}}},"mainboard":{"count":2,"cards":{"m":{"quantity":1,"card":{"name":"Sol Ring"}},"n":{"quantity":1,"card":{"name":"Mana Crypt"}}}}}}`

	var responseBody atomic.Value
	responseBody.Store(v1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody.Load().(string)))
	}))
	defer srv.Close()

	origBase := apiBaseVar
	apiBaseVar = srv.URL
	defer func() { apiBaseVar = origBase }()

	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	t.Setenv(snapshotEnvDisable, "")
	resetCachesForTest()

	if _, err := fetchDeckRaw("evolve01"); err != nil {
		t.Fatalf("v1 fetch: %v", err)
	}
	// Simulate the user editing the deck on Moxfield, then re-importing
	// with --refresh.
	responseBody.Store(v2)
	Refresh("evolve01")
	// Wait 1.1s so the v2 snapshot's filename timestamp differs.
	time.Sleep(1100 * time.Millisecond)
	if _, err := fetchDeckRaw("evolve01"); err != nil {
		t.Fatalf("v2 fetch: %v", err)
	}

	snaps, _ := ListSnapshots("evolve01")
	if len(snaps) != 2 {
		t.Errorf("expected 2 snapshots after edit+refresh+re-import, got %d", len(snaps))
	}
	if len(snaps) >= 2 && snaps[0].Hash == snaps[1].Hash {
		t.Errorf("two versions should have different hashes; both %q", snaps[0].Hash)
	}
	// Newest snapshot's content matches v2.
	if len(snaps) >= 1 {
		loaded, err := LoadSnapshot(snaps[0].Path)
		if err != nil {
			t.Fatalf("LoadSnapshot newest: %v", err)
		}
		if string(loaded) != v2 {
			t.Errorf("newest snapshot content mismatch:\ngot  %s\nwant %s", loaded, v2)
		}
	}
}

// History survives Moxfield-side deletion: even if a subsequent fetch
// would 404, we still hold the original snapshot.
func TestSnapshot_HistoryPreservedAfterUpstreamDeletion(t *testing.T) {
	body := `{"name":"Doomed","format":"commander","boards":{"commanders":{"count":1,"cards":{"c":{"quantity":1,"card":{"name":"Krenko, Mob Boss"}}}},"mainboard":{"count":1,"cards":{"m":{"quantity":1,"card":{"name":"Goblin Bombardment"}}}}}}`

	// Toggle to 404 mid-test.
	var serve404 atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serve404.Load() {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	origBase := apiBaseVar
	apiBaseVar = srv.URL
	defer func() { apiBaseVar = origBase }()

	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	t.Setenv(snapshotEnvDisable, "")
	resetCachesForTest()

	// Successful first import → snapshot captured.
	if _, err := fetchDeckRaw("doomed01"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	snapsBefore, _ := ListSnapshots("doomed01")
	if len(snapsBefore) != 1 {
		t.Fatalf("expected 1 snapshot after first fetch, got %d", len(snapsBefore))
	}

	// Owner deletes the deck on Moxfield. New fetch attempts fail.
	Refresh("doomed01")
	serve404.Store(true)
	if _, err := fetchDeckRaw("doomed01"); err == nil {
		t.Fatal("expected error after upstream 404")
	}

	// Snapshot from before the deletion must still be loadable.
	snapsAfter, _ := ListSnapshots("doomed01")
	if len(snapsAfter) != 1 {
		t.Errorf("history must survive upstream deletion; got %d snapshots", len(snapsAfter))
	}
	if len(snapsAfter) > 0 {
		loaded, err := LoadSnapshot(snapsAfter[0].Path)
		if err != nil {
			t.Errorf("could not load pre-deletion snapshot: %v", err)
		}
		if string(loaded) != body {
			t.Errorf("pre-deletion snapshot content corrupted")
		}
	}
}
