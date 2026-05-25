package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// seedCorrection inserts a row directly into archetype_feedback_log
// — avoids going through the POST handler so tests can craft
// specific (owner, deck, archetype) combinations without setting up
// deck files, strategy.json fixtures, or CSRF tokens.
func seedCorrection(t *testing.T, d *sql.DB, owner, deckID, freya, userArch string, when int64) {
	t.Helper()
	if _, err := d.ExecContext(context.Background(),
		`INSERT INTO archetype_feedback_log
		 (owner, deck_id, freya_archetype, user_action, user_archetype, recorded_at)
		 VALUES (?, ?, ?, 'correct', ?, ?)`,
		owner, deckID, freya, userArch, when); err != nil {
		t.Fatalf("seed log row: %v", err)
	}
}

func setupTrainingTest(t *testing.T) (*Handler, http.Handler, *sql.DB) {
	t.Helper()
	tmp := t.TempDir()
	d, err := sql.Open("sqlite", filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := EnsureDeckMetaSchema(context.Background(), d); err != nil {
		t.Fatalf("schema: %v", err)
	}
	h := &Handler{DecksDir: tmp}
	h.SetDB(d)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux, d
}

// ───── loadArchetypeCorrectionClusters ─────

func TestLoadClusters_EmptyLogReturnsEmpty(t *testing.T) {
	_, _, d := setupTrainingTest(t)

	clusters, err := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if clusters == nil || len(clusters) != 0 {
		t.Errorf("empty log should return non-nil empty slice; got %v", clusters)
	}
}

func TestLoadClusters_BelowThresholdReturnsNothing(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	// 4 distinct owners — under the default 5 threshold.
	for i, owner := range []string{"alice", "bob", "carol", "dave"} {
		seedCorrection(t, d, owner, "krenko", "Combo / Infinite", "stax", int64(1000+i))
	}

	clusters, _ := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if len(clusters) != 0 {
		t.Errorf("4-owner cluster under threshold 5 should not surface; got %d", len(clusters))
	}
}

func TestLoadClusters_AtThresholdSurfaces(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	// 5 distinct owners, all correcting Combo → Stax.
	for i, owner := range []string{"alice", "bob", "carol", "dave", "eve"} {
		seedCorrection(t, d, owner, "krenko", "Combo / Infinite", "stax", int64(1000+i))
	}

	clusters, err := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("at-threshold cluster should surface; got %d", len(clusters))
	}
	c := clusters[0]
	if c.FreyaArchetype != "Combo / Infinite" || c.UserArchetype != "stax" {
		t.Errorf("cluster mismatch: got freya=%q user=%q", c.FreyaArchetype, c.UserArchetype)
	}
	if c.OwnerCount != 5 || c.TotalCorrections != 5 {
		t.Errorf("counts: want owner=5 total=5, got owner=%d total=%d",
			c.OwnerCount, c.TotalCorrections)
	}
	if c.FirstSeenRFC3339 == "" || c.LastSeenRFC3339 == "" {
		t.Errorf("RFC3339 timestamps should populate")
	}
}

// One loud owner clicking the same correction 50 times does NOT
// inflate the OwnerCount — only distinct owners count toward the
// signal. TotalCorrections still records the raw click count
// for operator context.
func TestLoadClusters_DistinctOwnersNotTotalClicks(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	// alice clicks 10 times, no one else.
	for i := 0; i < 10; i++ {
		seedCorrection(t, d, "alice", "krenko", "Combo / Infinite", "stax", int64(1000+i))
	}

	clusters, _ := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if len(clusters) != 0 {
		t.Errorf("10 clicks from one owner should not trigger 5-distinct threshold; got %d clusters", len(clusters))
	}

	// Lower the threshold to 1 — now the single-owner cluster
	// surfaces, with TotalCorrections=10 showing the raw click count.
	clusters2, _ := loadArchetypeCorrectionClusters(context.Background(), d, 1)
	if len(clusters2) != 1 {
		t.Fatalf("threshold=1 should surface even single-owner cluster; got %d", len(clusters2))
	}
	if clusters2[0].OwnerCount != 1 || clusters2[0].TotalCorrections != 10 {
		t.Errorf("counts: want owner=1 total=10, got owner=%d total=%d",
			clusters2[0].OwnerCount, clusters2[0].TotalCorrections)
	}
}

func TestLoadClusters_MultiPatternSortByOwnerCountDesc(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	// Pattern A: Combo→Stax, 7 distinct owners (will rank first)
	for i := 0; i < 7; i++ {
		seedCorrection(t, d, fmt.Sprintf("user%d", i), "deck-a",
			"Combo / Infinite", "stax", int64(1000+i))
	}
	// Pattern B: Voltron→Aggro, 5 distinct owners
	for i := 100; i < 105; i++ {
		seedCorrection(t, d, fmt.Sprintf("user%d", i), "deck-b",
			"Voltron", "aggro", int64(2000+i))
	}

	clusters, _ := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if len(clusters) != 2 {
		t.Fatalf("want 2 clusters; got %d", len(clusters))
	}
	if clusters[0].OwnerCount != 7 {
		t.Errorf("position 0 should be the 7-owner cluster; got owner_count=%d", clusters[0].OwnerCount)
	}
	if clusters[1].OwnerCount != 5 {
		t.Errorf("position 1 should be the 5-owner cluster; got owner_count=%d", clusters[1].OwnerCount)
	}
}

// "confirm" actions are NOT counted toward correction clusters —
// confirming Freya is the OPPOSITE signal.
func TestLoadClusters_ConfirmActionsExcluded(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	// 5 owners CONFIRM the same Freya call.
	for i, owner := range []string{"alice", "bob", "carol", "dave", "eve"} {
		if _, err := d.Exec(`INSERT INTO archetype_feedback_log
			(owner, deck_id, freya_archetype, user_action, user_archetype, recorded_at)
			VALUES (?, ?, 'Combo / Infinite', 'confirm', '', ?)`,
			owner, "krenko", int64(1000+i)); err != nil {
			t.Fatal(err)
		}
	}

	clusters, _ := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if len(clusters) != 0 {
		t.Errorf("confirm actions should NOT surface as correction clusters; got %d", len(clusters))
	}
}

// Clear actions (retractions) also don't contribute.
func TestLoadClusters_ClearActionsExcluded(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	for i, owner := range []string{"alice", "bob", "carol", "dave", "eve"} {
		if _, err := d.Exec(`INSERT INTO archetype_feedback_log
			(owner, deck_id, freya_archetype, user_action, user_archetype, recorded_at)
			VALUES (?, ?, 'Combo / Infinite', 'clear', '', ?)`,
			owner, "krenko", int64(1000+i)); err != nil {
			t.Fatal(err)
		}
	}

	clusters, _ := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if len(clusters) != 0 {
		t.Errorf("clear actions should NOT surface; got %d", len(clusters))
	}
}

// ExampleDecks surfaces up to 3 distinct deck_keys per cluster,
// newest-activity first.
func TestLoadClusters_ExampleDecksLimitedAndOrdered(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	// 5 distinct owners, 5 distinct decks, monotonically-later
	// timestamps so the newest 3 are deterministic.
	for i, owner := range []string{"alice", "bob", "carol", "dave", "eve"} {
		seedCorrection(t, d, owner, fmt.Sprintf("deck-%d", i),
			"Combo / Infinite", "stax", int64(1000+i*100))
	}

	clusters, _ := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if len(clusters) != 1 {
		t.Fatalf("expected one cluster; got %d", len(clusters))
	}
	ex := clusters[0].ExampleDecks
	if len(ex) != 3 {
		t.Errorf("ExampleDecks should cap at 3; got %d (%v)", len(ex), ex)
	}
	// Newest 3 by recorded_at are the i=2,3,4 decks (eve/deck-4 is latest).
	wantFirst := "eve/deck-4"
	if len(ex) > 0 && ex[0] != wantFirst {
		t.Errorf("newest example should be %q; got %q (full: %v)", wantFirst, ex[0], ex)
	}
}

// Empty freya_archetype rows are excluded (those are "freya hasn't
// classified yet" cases — no fingerprint to retune).
func TestLoadClusters_SkipsEmptyFreyaArchetype(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	for i, owner := range []string{"alice", "bob", "carol", "dave", "eve"} {
		seedCorrection(t, d, owner, "krenko", "", "stax", int64(1000+i))
	}

	clusters, _ := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if len(clusters) != 0 {
		t.Errorf("empty freya_archetype rows should be skipped; got %d clusters", len(clusters))
	}
}

// ───── handler contract ─────

func TestHandleTrainingSignal_PublicReadHappyPath(t *testing.T) {
	_, mux, d := setupTrainingTest(t)
	for i, owner := range []string{"alice", "bob", "carol", "dave", "eve"} {
		seedCorrection(t, d, owner, "krenko", "Combo / Infinite", "stax", int64(1000+i))
	}

	req := httptest.NewRequest("GET", "/api/freya/training-signal", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Clusters    []ArchetypeCorrectionCluster `json:"clusters"`
		Threshold   int                          `json:"threshold"`
		GeneratedAt string                       `json:"generated_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Threshold != 5 {
		t.Errorf("default threshold: want 5, got %d", resp.Threshold)
	}
	if len(resp.Clusters) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(resp.Clusters))
	}
	if resp.GeneratedAt == "" {
		t.Errorf("generated_at should populate")
	}
}

func TestHandleTrainingSignal_MinQueryOverridesDefault(t *testing.T) {
	_, mux, d := setupTrainingTest(t)
	// 3 distinct owners — under default 5 but above explicit min=2.
	for i, owner := range []string{"alice", "bob", "carol"} {
		seedCorrection(t, d, owner, "krenko", "Combo / Infinite", "stax", int64(1000+i))
	}

	req := httptest.NewRequest("GET", "/api/freya/training-signal?min=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp struct {
		Clusters  []ArchetypeCorrectionCluster `json:"clusters"`
		Threshold int                          `json:"threshold"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Threshold != 2 {
		t.Errorf("threshold echo: want 2, got %d", resp.Threshold)
	}
	if len(resp.Clusters) != 1 {
		t.Errorf("min=2 should surface 3-owner cluster; got %d", len(resp.Clusters))
	}
}

func TestHandleTrainingSignal_InvalidMinFallsBackToDefault(t *testing.T) {
	_, mux, _ := setupTrainingTest(t)

	req := httptest.NewRequest("GET", "/api/freya/training-signal?min=abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("invalid min should not 4xx; got %d", rec.Code)
	}
	var resp struct {
		Threshold int `json:"threshold"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Threshold != 5 {
		t.Errorf("invalid min should fall back to default 5; got %d", resp.Threshold)
	}
}

func TestHandleTrainingSignal_MinCappedAtMax(t *testing.T) {
	_, mux, _ := setupTrainingTest(t)

	req := httptest.NewRequest("GET", "/api/freya/training-signal?min=999999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var resp struct {
		Threshold int `json:"threshold"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Threshold > MaxTrainingSignalThreshold {
		t.Errorf("abusive min should cap at %d; got %d", MaxTrainingSignalThreshold, resp.Threshold)
	}
}

func TestHandleTrainingSignal_EmptyLogReturnsNonNullArray(t *testing.T) {
	_, mux, _ := setupTrainingTest(t)

	req := httptest.NewRequest("GET", "/api/freya/training-signal", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	// Must contain `"clusters": []` (with whitespace from indented
	// encoder) not `"clusters": null`.
	if !contains(body, `"clusters": []`) && !contains(body, `"clusters":[]`) {
		t.Errorf("empty result should serialize as [] not null; got: %s", body)
	}
}

func TestHandleTrainingSignal_503WhenDBMissing(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/freya/training-signal", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 503 {
		t.Errorf("no DB: want 503, got %d", rec.Code)
	}
}

// Same-owner-multiple-decks: alice corrects the same pattern on 5
// different decks. Counts as 1 distinct owner (not 5), so the
// default threshold doesn't surface it.
func TestLoadClusters_SameOwnerAcrossDecksCountsOnce(t *testing.T) {
	_, _, d := setupTrainingTest(t)
	for i := 0; i < 5; i++ {
		seedCorrection(t, d, "alice", fmt.Sprintf("deck-%d", i),
			"Combo / Infinite", "stax", int64(1000+i))
	}

	clusters, _ := loadArchetypeCorrectionClusters(context.Background(), d, 5)
	if len(clusters) != 0 {
		t.Errorf("one owner across 5 decks shouldn't trigger 5-distinct-owner threshold; got %d", len(clusters))
	}
	// At threshold=1, surfaces with OwnerCount=1, TotalCorrections=5.
	clusters2, _ := loadArchetypeCorrectionClusters(context.Background(), d, 1)
	if len(clusters2) != 1 || clusters2[0].OwnerCount != 1 || clusters2[0].TotalCorrections != 5 {
		t.Errorf("threshold=1 should surface with owner=1 total=5; got %+v", clusters2)
	}
}
