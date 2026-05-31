package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Synthetic snapshot helpers — local to this test file.
// ---------------------------------------------------------------------------

func snapWith(cards map[string]float64) Snapshot {
	out := Snapshot{Version: 1, Cards: map[string]SnapshotCard{}}
	for name, score := range cards {
		out.Cards[name] = SnapshotCard{Score: score, MinScore: score, NumAbilities: 1}
		out.TotalCards++
	}
	return out
}

// ---------------------------------------------------------------------------
// BuildSnapshotFromCards
// ---------------------------------------------------------------------------

func TestBuildSnapshotFromCards_RecordsPerCardConfidence(t *testing.T) {
	cards := []*gameast.CardAST{
		cardClean("Clean"),
		cardBoundary("Boundary"),
		cardStacked("Stacked"),
	}
	s := BuildSnapshotFromCards(cards)
	if s.TotalCards != 3 {
		t.Fatalf("TotalCards: want 3, got %d", s.TotalCards)
	}
	if got := s.Cards["Clean"].Score; got != 1.0 {
		t.Errorf("Clean.Score: want 1.0, got %v", got)
	}
	if got := s.Cards["Boundary"].Score; got != 0.5 {
		t.Errorf("Boundary.Score: want 0.5, got %v", got)
	}
	if got := s.Cards["Stacked"].Score; got > 0.21 || got < 0.19 {
		t.Errorf("Stacked.Score: want ~0.2, got %v", got)
	}
	if s.Cards["Stacked"].NumFallback != 1 {
		t.Errorf("Stacked.NumFallback: want 1, got %d", s.Cards["Stacked"].NumFallback)
	}
}

func TestBuildSnapshotFromCards_NilCardSafe(t *testing.T) {
	s := BuildSnapshotFromCards([]*gameast.CardAST{nil, cardClean("ok")})
	if s.TotalCards != 1 {
		t.Errorf("want 1 (nil skipped), got %d", s.TotalCards)
	}
	if _, ok := s.Cards["ok"]; !ok {
		t.Errorf("want 'ok' present in snapshot.Cards")
	}
}

// ---------------------------------------------------------------------------
// JSON round-trip via Write/Read
// ---------------------------------------------------------------------------

func TestSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	original := snapWith(map[string]float64{"A": 1.0, "B": 0.5, "C": 0.2})
	if err := WriteSnapshot(path, original); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	loaded, err := ReadSnapshot(path)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	if loaded.TotalCards != original.TotalCards {
		t.Errorf("TotalCards drift: want %d, got %d", original.TotalCards, loaded.TotalCards)
	}
	if loaded.Cards["B"].Score != 0.5 {
		t.Errorf("B.Score: want 0.5, got %v", loaded.Cards["B"].Score)
	}
}

func TestReadSnapshot_MissingFileReportsError(t *testing.T) {
	_, err := ReadSnapshot("/tmp/definitely-nonexistent-snapshot-path-1234567.json")
	if err == nil {
		t.Errorf("missing file: want error, got nil")
	}
}

func TestReadSnapshot_NilCardsMapInitialised(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap-empty.json")
	// Write a snapshot whose serialised form omits the Cards key (older
	// version, hypothetically).
	if err := WriteSnapshot(path, Snapshot{Version: 1, TotalCards: 0}); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	loaded, err := ReadSnapshot(path)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	if loaded.Cards == nil {
		t.Errorf("Cards map: want initialised (non-nil), got nil")
	}
}

// ---------------------------------------------------------------------------
// ComputeTrend — categorisation
// ---------------------------------------------------------------------------

func TestComputeTrend_AllUnchanged(t *testing.T) {
	a := snapWith(map[string]float64{"A": 1.0, "B": 0.5})
	tr := ComputeTrend(a, a)
	if len(tr.Regressed) != 0 || len(tr.Improved) != 0 {
		t.Errorf("identical snapshots: want no deltas")
	}
	if tr.Unchanged != 2 {
		t.Errorf("Unchanged: want 2, got %d", tr.Unchanged)
	}
}

func TestComputeTrend_DetectsRegression(t *testing.T) {
	from := snapWith(map[string]float64{"X": 1.0, "Y": 0.8})
	to := snapWith(map[string]float64{"X": 0.5, "Y": 0.8})
	tr := ComputeTrend(from, to)
	if len(tr.Regressed) != 1 {
		t.Fatalf("Regressed: want 1, got %d", len(tr.Regressed))
	}
	r := tr.Regressed[0]
	if r.Name != "X" {
		t.Errorf("name: want X, got %s", r.Name)
	}
	if r.DeltaScore > -0.49 || r.DeltaScore < -0.51 {
		t.Errorf("DeltaScore: want ~-0.5, got %v", r.DeltaScore)
	}
	if r.Kind != "regressed" {
		t.Errorf("Kind: want regressed, got %s", r.Kind)
	}
	if tr.Unchanged != 1 {
		t.Errorf("Unchanged: want 1 (Y), got %d", tr.Unchanged)
	}
}

func TestComputeTrend_DetectsImprovement(t *testing.T) {
	from := snapWith(map[string]float64{"X": 0.3})
	to := snapWith(map[string]float64{"X": 0.9})
	tr := ComputeTrend(from, to)
	if len(tr.Improved) != 1 {
		t.Fatalf("Improved: want 1, got %d", len(tr.Improved))
	}
	if tr.Improved[0].DeltaScore < 0.59 {
		t.Errorf("DeltaScore: want ~+0.6, got %v", tr.Improved[0].DeltaScore)
	}
}

func TestComputeTrend_NewAndDropped(t *testing.T) {
	from := snapWith(map[string]float64{"OnlyFrom": 0.8})
	to := snapWith(map[string]float64{"OnlyTo": 0.4})
	tr := ComputeTrend(from, to)
	if len(tr.Dropped) != 1 || tr.Dropped[0].Name != "OnlyFrom" {
		t.Errorf("Dropped: want [OnlyFrom], got %v", tr.Dropped)
	}
	if len(tr.New) != 1 || tr.New[0].Name != "OnlyTo" {
		t.Errorf("New: want [OnlyTo], got %v", tr.New)
	}
}

func TestComputeTrend_RegressionsSortedBiggestDropFirst(t *testing.T) {
	from := snapWith(map[string]float64{"big": 1.0, "small": 1.0, "mid": 1.0})
	to := snapWith(map[string]float64{"big": 0.1, "small": 0.9, "mid": 0.5})
	tr := ComputeTrend(from, to)
	if len(tr.Regressed) != 3 {
		t.Fatalf("want 3 regressed, got %d", len(tr.Regressed))
	}
	if tr.Regressed[0].Name != "big" {
		t.Errorf("biggest drop first: want big, got %s", tr.Regressed[0].Name)
	}
	if tr.Regressed[1].Name != "mid" {
		t.Errorf("second: want mid, got %s", tr.Regressed[1].Name)
	}
	if tr.Regressed[2].Name != "small" {
		t.Errorf("third: want small, got %s", tr.Regressed[2].Name)
	}
}

func TestComputeTrend_ImprovedSortedBiggestGainFirst(t *testing.T) {
	from := snapWith(map[string]float64{"big": 0.1, "small": 0.7, "mid": 0.4})
	to := snapWith(map[string]float64{"big": 0.9, "small": 0.9, "mid": 0.9})
	tr := ComputeTrend(from, to)
	if tr.Improved[0].Name != "big" {
		t.Errorf("biggest gain first: want big, got %s", tr.Improved[0].Name)
	}
}

func TestComputeTrend_EpsilonTreatsTinyDeltasAsUnchanged(t *testing.T) {
	// A 1e-12 difference should NOT show up as a regression/improvement.
	from := snapWith(map[string]float64{"X": 0.5})
	to := snapWith(map[string]float64{"X": 0.5 + 1e-12})
	tr := ComputeTrend(from, to)
	if len(tr.Regressed) != 0 || len(tr.Improved) != 0 {
		t.Errorf("tiny float delta: want unchanged, got R=%d I=%d",
			len(tr.Regressed), len(tr.Improved))
	}
	if tr.Unchanged != 1 {
		t.Errorf("Unchanged: want 1, got %d", tr.Unchanged)
	}
}

func TestComputeTrend_DroppedRegressionDeltaNegative(t *testing.T) {
	from := snapWith(map[string]float64{"D": 0.7})
	to := snapWith(map[string]float64{})
	tr := ComputeTrend(from, to)
	if len(tr.Dropped) != 1 {
		t.Fatalf("want 1 dropped, got %d", len(tr.Dropped))
	}
	if tr.Dropped[0].DeltaScore >= 0 {
		t.Errorf("dropped DeltaScore: want negative (lost score), got %v", tr.Dropped[0].DeltaScore)
	}
}

// ---------------------------------------------------------------------------
// RenderTrendMarkdown — output shape
// ---------------------------------------------------------------------------

func TestRenderTrendMarkdown_SummaryRowsPresent(t *testing.T) {
	from := snapWith(map[string]float64{"A": 1.0, "B": 1.0, "C": 1.0})
	to := snapWith(map[string]float64{"A": 0.5, "B": 1.0, "D": 0.8})
	tr := ComputeTrend(from, to)
	var buf bytes.Buffer
	if err := RenderTrendMarkdown(&buf, tr, 0); err != nil {
		t.Fatalf("RenderTrendMarkdown: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"# AST Confidence Trend",
		"| Regressed | 1 |",
		"| Improved  | 0 |",
		"| Unchanged | 1 |",
		"| New (in `to` only) | 1 |",
		"| Dropped (in `from` only) | 1 |",
		"## Regressed (biggest drops first)",
		"## Improved (biggest gains first)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderTrendMarkdown_LimitClampsAndAnnotates(t *testing.T) {
	from := snapWith(map[string]float64{"a": 1.0, "b": 1.0, "c": 1.0, "d": 1.0, "e": 1.0})
	to := snapWith(map[string]float64{"a": 0.1, "b": 0.2, "c": 0.3, "d": 0.4, "e": 0.5})
	tr := ComputeTrend(from, to)
	var buf bytes.Buffer
	if err := RenderTrendMarkdown(&buf, tr, 2); err != nil {
		t.Fatalf("err: %v", err)
	}
	out := buf.String()
	// Top-2 most-regressed are 'a' and 'b'; 'c'/'d'/'e' should NOT appear
	// in the regressed table body.
	for _, want := range []string{"| a |", "| b |"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing top-2 entry %q", want)
		}
	}
	// "_(N more not shown_" annotation must appear.
	if !strings.Contains(out, "more not shown") {
		t.Errorf("expected 'more not shown' annotation in output:\n%s", out)
	}
}

func TestRenderTrendMarkdown_EmptyTrendStillWritesHeader(t *testing.T) {
	a := snapWith(map[string]float64{"X": 1.0})
	tr := ComputeTrend(a, a)
	var buf bytes.Buffer
	if err := RenderTrendMarkdown(&buf, tr, 0); err != nil {
		t.Fatalf("err: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "# AST Confidence Trend") {
		t.Errorf("missing title")
	}
	if !strings.Contains(out, "| Unchanged | 1 |") {
		t.Errorf("unchanged row missing")
	}
	if !strings.Contains(out, "_(none)_") {
		t.Errorf("expected '(none)' for empty regressed/improved sections")
	}
}

func TestRenderTrendMarkdown_NewAndDroppedSectionsConditional(t *testing.T) {
	// Trend with no new/dropped — sections should be OMITTED.
	a := snapWith(map[string]float64{"X": 1.0})
	tr := ComputeTrend(a, a)
	var buf bytes.Buffer
	_ = RenderTrendMarkdown(&buf, tr, 0)
	out := buf.String()
	if strings.Contains(out, "## New cards") {
		t.Errorf("New section should be omitted when empty")
	}
	if strings.Contains(out, "## Dropped cards") {
		t.Errorf("Dropped section should be omitted when empty")
	}

	// Trend with new + dropped — sections should be PRESENT.
	from := snapWith(map[string]float64{"Old": 1.0})
	to := snapWith(map[string]float64{"NewOne": 0.5})
	tr2 := ComputeTrend(from, to)
	buf.Reset()
	_ = RenderTrendMarkdown(&buf, tr2, 0)
	out = buf.String()
	if !strings.Contains(out, "## New cards") {
		t.Errorf("New section should be present when populated")
	}
	if !strings.Contains(out, "## Dropped cards") {
		t.Errorf("Dropped section should be present when populated")
	}
}
