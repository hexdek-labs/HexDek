package main

import (
	"bytes"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Combo interaction matrix tests — pin matrix shape and derived metrics
// against decks with 3+ combos in known configurations.
//
// Fixtures use minimal hand-constructed FreyaReports so the matrix
// behavior is isolated from the detection pipeline. Tests verify both
// the structural matrix (Combos / Overlap / PieceFragility) and the
// derived deck-level metrics (RedundancyOneCardRemoved, MostFragile /
// MostIndependent combo indices, IndependentComboCount).
// ---------------------------------------------------------------------------

func reportWithCombos(trueInfs, det, gy [][]string) *FreyaReport {
	r := &FreyaReport{}
	for _, cards := range trueInfs {
		r.TrueInfinites = append(r.TrueInfinites, ComboResult{
			Cards: append([]string(nil), cards...), LoopType: "true_infinite",
		})
	}
	for _, cards := range det {
		r.Determined = append(r.Determined, ComboResult{
			Cards: append([]string(nil), cards...), LoopType: "determined",
		})
	}
	for _, cards := range gy {
		r.GraveyardLoops = append(r.GraveyardLoops, ComboResult{
			Cards: append([]string(nil), cards...), LoopType: "synergy",
		})
	}
	return r
}

func indexOfCombo(t *testing.T, m *ComboInteractionMatrix, cards ...string) int {
	t.Helper()
	want := append([]string(nil), cards...)
	sort.Strings(want)
	for i, e := range m.Combos {
		if len(e.Cards) != len(want) {
			continue
		}
		match := true
		for k, c := range e.Cards {
			if c != want[k] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	t.Fatalf("no combo found for %v in matrix.Combos=%v", cards, m.Combos)
	return -1
}

// TestMatrix_ThreeIndependentCombos: three combos with no shared pieces.
// All three independent; redundancy after any 1 removed = 2 (the removed
// card belongs to one combo, leaving 2 untouched).
func TestMatrix_ThreeIndependentCombos(t *testing.T) {
	r := reportWithCombos(
		[][]string{{"A1", "A2"}},
		[][]string{{"B1", "B2"}, {"C1", "C2"}},
		nil,
	)
	m := BuildComboInteractionMatrix(r)
	if m == nil {
		t.Fatal("expected non-nil matrix")
	}
	if len(m.Combos) != 3 {
		t.Fatalf("expected 3 combos, got %d", len(m.Combos))
	}
	if m.IndependentComboCount != 3 {
		t.Errorf("IndependentComboCount: got %d, want 3", m.IndependentComboCount)
	}
	if m.RedundancyOneCardRemoved != 2 {
		t.Errorf("RedundancyOneCardRemoved: got %d, want 2", m.RedundancyOneCardRemoved)
	}
	// Off-diagonal must be 0; diagonal must be 2 (each combo has 2 cards).
	for i := 0; i < len(m.Combos); i++ {
		for j := 0; j < len(m.Combos); j++ {
			if i == j {
				if m.Overlap[i][j] != 2 {
					t.Errorf("Overlap[%d][%d] (diagonal): got %d, want 2", i, j, m.Overlap[i][j])
				}
			} else if m.Overlap[i][j] != 0 {
				t.Errorf("Overlap[%d][%d]: got %d, want 0", i, j, m.Overlap[i][j])
			}
		}
	}
}

// TestMatrix_SharedKeystonePiece: three combos all sharing one card.
// PieceFragility's top entry is that shared card with ComboCount=3.
// RedundancyOneCardRemoved = 0 (removing the keystone breaks all 3).
func TestMatrix_SharedKeystonePiece(t *testing.T) {
	r := reportWithCombos(
		[][]string{{"Keystone", "A2"}},
		[][]string{{"Keystone", "B2"}, {"Keystone", "C2"}},
		nil,
	)
	m := BuildComboInteractionMatrix(r)
	if m == nil {
		t.Fatal("expected non-nil matrix")
	}
	if len(m.Combos) != 3 {
		t.Fatalf("expected 3 combos, got %d", len(m.Combos))
	}
	if m.PieceFragility[0].Card != "Keystone" {
		t.Errorf("top fragility: got %q, want \"Keystone\"", m.PieceFragility[0].Card)
	}
	if m.PieceFragility[0].ComboCount != 3 {
		t.Errorf("Keystone ComboCount: got %d, want 3", m.PieceFragility[0].ComboCount)
	}
	if m.RedundancyOneCardRemoved != 0 {
		t.Errorf("RedundancyOneCardRemoved: got %d, want 0 (Keystone is critical)",
			m.RedundancyOneCardRemoved)
	}
	if m.IndependentComboCount != 0 {
		t.Errorf("IndependentComboCount: got %d, want 0 (every combo shares Keystone)",
			m.IndependentComboCount)
	}
	// Off-diagonal entries between any two combos = 1 (they share Keystone).
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i == j {
				continue
			}
			if m.Overlap[i][j] != 1 {
				t.Errorf("Overlap[%d][%d]: got %d, want 1", i, j, m.Overlap[i][j])
			}
		}
	}
}

// TestMatrix_MixedOverlap: four combos with mixed sharing. Pin the
// overlap matrix values exactly.
//
// Combos: AB shares "X" with BC; CD shares nothing; AB shares "A" with
// AC; AC shares "C" with CD.
//
// Names: combo1=[A,X], combo2=[X,B], combo3=[A,C], combo4=[C,D]
// Overlap:
//   1 vs 2: share X → 1
//   1 vs 3: share A → 1
//   1 vs 4: share nothing → 0
//   2 vs 3: share nothing → 0
//   2 vs 4: share nothing → 0
//   3 vs 4: share C → 1
//
// PieceFragility: A in 2 combos (1, 3); C in 2 combos (3, 4); X in 2
// combos (1, 2). Others in 1 each.
//
// RedundancyOneCardRemoved: removing A leaves 2 combos (2, 4) untouched;
// removing C leaves 2 (1, 2); removing X leaves 2 (3, 4). Removing any
// single-combo card leaves 3. min = 2.
func TestMatrix_MixedOverlap(t *testing.T) {
	r := reportWithCombos(
		[][]string{{"A", "X"}, {"X", "B"}},
		[][]string{{"A", "C"}, {"C", "D"}},
		nil,
	)
	m := BuildComboInteractionMatrix(r)
	if m == nil {
		t.Fatal("expected non-nil matrix")
	}
	if len(m.Combos) != 4 {
		t.Fatalf("expected 4 combos, got %d", len(m.Combos))
	}

	i_AX := indexOfCombo(t, m, "A", "X")
	i_BX := indexOfCombo(t, m, "B", "X")
	i_AC := indexOfCombo(t, m, "A", "C")
	i_CD := indexOfCombo(t, m, "C", "D")

	cases := []struct {
		a, b int
		want int
		desc string
	}{
		{i_AX, i_BX, 1, "AX vs BX share X"},
		{i_AX, i_AC, 1, "AX vs AC share A"},
		{i_AX, i_CD, 0, "AX vs CD share nothing"},
		{i_BX, i_AC, 0, "BX vs AC share nothing"},
		{i_BX, i_CD, 0, "BX vs CD share nothing"},
		{i_AC, i_CD, 1, "AC vs CD share C"},
	}
	for _, c := range cases {
		if m.Overlap[c.a][c.b] != c.want {
			t.Errorf("%s: got %d, want %d", c.desc, m.Overlap[c.a][c.b], c.want)
		}
		// Symmetry check.
		if m.Overlap[c.b][c.a] != c.want {
			t.Errorf("%s (sym): got %d, want %d", c.desc, m.Overlap[c.b][c.a], c.want)
		}
	}

	// RedundancyOneCardRemoved: removing any 2-combo shared card leaves
	// 4 - 2 = 2 combos untouched. min = 2.
	if m.RedundancyOneCardRemoved != 2 {
		t.Errorf("RedundancyOneCardRemoved: got %d, want 2", m.RedundancyOneCardRemoved)
	}

	// PieceFragility ordering: A, C, X (all ComboCount=2; tied — sorted
	// by name asc). Then B, D (ComboCount=1).
	topThree := m.PieceFragility[:3]
	wantTopThree := []string{"A", "C", "X"}
	for i, want := range wantTopThree {
		if topThree[i].Card != want {
			t.Errorf("PieceFragility[%d].Card: got %q, want %q", i, topThree[i].Card, want)
		}
		if topThree[i].ComboCount != 2 {
			t.Errorf("PieceFragility[%d].ComboCount: got %d, want 2", i, topThree[i].ComboCount)
		}
	}
}

// TestMatrix_NilOnSingleCombo: a 1-combo deck has nothing to interact
// with itself — return nil so the JSON / text sections are omitted.
func TestMatrix_NilOnSingleCombo(t *testing.T) {
	r := reportWithCombos([][]string{{"A", "B"}}, nil, nil)
	if m := BuildComboInteractionMatrix(r); m != nil {
		t.Errorf("expected nil for 1-combo deck, got %+v", m)
	}
}

// TestMatrix_NilOnZeroCombos: a 0-combo deck → nil.
func TestMatrix_NilOnZeroCombos(t *testing.T) {
	r := &FreyaReport{}
	if m := BuildComboInteractionMatrix(r); m != nil {
		t.Errorf("expected nil for 0-combo deck, got %+v", m)
	}
}

// TestMatrix_NilOnNilReport: defensive nil-input handling.
func TestMatrix_NilOnNilReport(t *testing.T) {
	if m := BuildComboInteractionMatrix(nil); m != nil {
		t.Errorf("expected nil for nil report, got %+v", m)
	}
}

// TestMatrix_TwoCombosOverlapping: pair sharing one card.
// Independent count = 0; RedundancyOneCardRemoved = 0 (every combo
// includes one of the two cards, and removing the shared card leaves
// 0, removing a unique card leaves 1).
func TestMatrix_TwoCombosOverlapping(t *testing.T) {
	r := reportWithCombos(
		[][]string{{"Shared", "X"}, {"Shared", "Y"}},
		nil, nil,
	)
	m := BuildComboInteractionMatrix(r)
	if m == nil {
		t.Fatal("expected non-nil matrix")
	}
	if len(m.Combos) != 2 {
		t.Fatalf("expected 2 combos, got %d", len(m.Combos))
	}
	if m.RedundancyOneCardRemoved != 0 {
		t.Errorf("RedundancyOneCardRemoved: got %d, want 0", m.RedundancyOneCardRemoved)
	}
	if m.IndependentComboCount != 0 {
		t.Errorf("IndependentComboCount: got %d, want 0", m.IndependentComboCount)
	}
	if m.PieceFragility[0].Card != "Shared" || m.PieceFragility[0].ComboCount != 2 {
		t.Errorf("top fragility: got %+v, want Shared/2", m.PieceFragility[0])
	}
}

// TestMatrix_MostFragileVsMostIndependent: with 3 combos where one is
// fully independent and two share a piece, the independent one is
// MostIndependent and one of the entangled pair is MostFragile.
func TestMatrix_MostFragileVsMostIndependent(t *testing.T) {
	r := reportWithCombos(
		[][]string{{"S", "X1"}, {"S", "X2"}},
		[][]string{{"Lone", "Wolf"}},
		nil,
	)
	m := BuildComboInteractionMatrix(r)
	if m == nil {
		t.Fatal("expected non-nil matrix")
	}
	loneIdx := indexOfCombo(t, m, "Lone", "Wolf")
	if m.MostIndependentComboIndex != loneIdx {
		t.Errorf("MostIndependentComboIndex: got %d, want %d (Lone+Wolf)",
			m.MostIndependentComboIndex, loneIdx)
	}
	if m.MostFragileComboIndex == loneIdx {
		t.Errorf("MostFragileComboIndex should NOT be Lone+Wolf (=%d), got %d", loneIdx, m.MostFragileComboIndex)
	}
	if m.IndependentComboCount != 1 {
		t.Errorf("IndependentComboCount: got %d, want 1 (only Lone+Wolf is independent)",
			m.IndependentComboCount)
	}
}

// TestMatrix_GraveyardLoopSourceTagged: source tag flows through.
func TestMatrix_GraveyardLoopSourceTagged(t *testing.T) {
	r := reportWithCombos(
		[][]string{{"A1", "A2"}},
		nil,
		[][]string{{"B1", "B2", "B3"}, {"C1", "C2", "C3"}},
	)
	m := BuildComboInteractionMatrix(r)
	if m == nil {
		t.Fatal("expected non-nil matrix")
	}
	sources := map[string]int{}
	for _, e := range m.Combos {
		sources[e.Source]++
	}
	if sources["true_infinite"] != 1 {
		t.Errorf("true_infinite source count: got %d, want 1", sources["true_infinite"])
	}
	if sources["graveyard_loop"] != 2 {
		t.Errorf("graveyard_loop source count: got %d, want 2", sources["graveyard_loop"])
	}
}

// TestPrintComboInteraction_RendersExpectedShape: text output contains
// the canonical headers + load-bearing pieces + single-point-of-failure
// warning when applicable.
func TestPrintComboInteraction_RendersExpectedShape(t *testing.T) {
	r := reportWithCombos(
		[][]string{{"Keystone", "A2"}},
		[][]string{{"Keystone", "B2"}, {"Keystone", "C2"}},
		nil,
	)
	m := BuildComboInteractionMatrix(r)
	var buf bytes.Buffer
	printComboInteraction(&buf, m)
	out := buf.String()
	musts := []string{
		"COMBO INTERACTION",
		"redundancy 0/3",
		"Load-bearing pieces",
		"Keystone (in 3 combos)",
		"single point of failure",
	}
	for _, s := range musts {
		if !strings.Contains(out, s) {
			t.Errorf("text output missing %q\nfull output:\n%s", s, out)
		}
	}
}

// TestPrintComboInteraction_NilNoOp: nil input renders nothing.
func TestPrintComboInteraction_NilNoOp(t *testing.T) {
	var buf bytes.Buffer
	printComboInteraction(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected zero output for nil matrix, got: %s", buf.String())
	}
}
