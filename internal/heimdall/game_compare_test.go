package heimdall

import (
	"reflect"
	"testing"
)

func mkSummary(winner int, name string, turns int, end string, mvps []string, decks []string, tps []TurningPoint) GameSummary {
	mvpCards := make([]MVPCard, 0, len(mvps))
	for _, n := range mvps {
		mvpCards = append(mvpCards, MVPCard{CardName: n, Score: 10})
	}
	return GameSummary{
		Winner:        winner,
		WinnerName:    name,
		Turns:         turns,
		EndReason:     end,
		DeckKeys:      decks,
		MVPCards:      mvpCards,
		TurningPoints: tps,
		DataSource:    "rich",
	}
}

// TestCompareGames_Outcome pins the OutcomeDiff: per-field same/diff
// booleans, independent of the other fields.
func TestCompareGames_Outcome(t *testing.T) {
	a := mkSummary(0, "Krenko", 12, "combat", nil, nil, nil)
	b := mkSummary(0, "Krenko", 18, "combat", nil, nil, nil)
	c := mkSummary(1, "Atraxa", 12, "decking", nil, nil, nil)

	got := CompareGames(a, b).Outcome
	if !got.SameWinnerSeat || !got.SameWinnerName || !got.SameEndReason {
		t.Errorf("a-vs-b same-everything: got %+v", got)
	}

	got = CompareGames(a, c).Outcome
	if got.SameWinnerSeat || got.SameWinnerName || got.SameEndReason {
		t.Errorf("a-vs-c all-different: got %+v", got)
	}
}

// TestCompareGames_TurnsDelta pins the signed delta (A - B).
func TestCompareGames_TurnsDelta(t *testing.T) {
	a := mkSummary(0, "X", 20, "combat", nil, nil, nil)
	b := mkSummary(0, "X", 12, "combat", nil, nil, nil)
	cmp := CompareGames(a, b)
	if cmp.Turns.Delta != 8 || cmp.Turns.GameATurns != 20 || cmp.Turns.GameBTurns != 12 {
		t.Errorf("turns diff: got %+v, want delta=8 a=20 b=12", cmp.Turns)
	}
	cmp = CompareGames(b, a)
	if cmp.Turns.Delta != -8 {
		t.Errorf("reverse delta: got %d, want -8", cmp.Turns.Delta)
	}
}

// TestCompareGames_MVPDiff pins the three-way partition (only-A,
// only-B, shared) with alpha-sorted output.
func TestCompareGames_MVPDiff(t *testing.T) {
	a := mkSummary(0, "X", 10, "", []string{"Sol Ring", "Craterhoof Behemoth", "Avenger of Zendikar"}, nil, nil)
	b := mkSummary(0, "X", 10, "", []string{"Sol Ring", "Cyclonic Rift", "Avenger of Zendikar"}, nil, nil)
	d := CompareGames(a, b).MVPDiff

	wantOnlyA := []string{"Craterhoof Behemoth"}
	wantOnlyB := []string{"Cyclonic Rift"}
	wantShared := []string{"Avenger of Zendikar", "Sol Ring"}
	if !reflect.DeepEqual(d.OnlyInA, wantOnlyA) {
		t.Errorf("OnlyInA = %v, want %v", d.OnlyInA, wantOnlyA)
	}
	if !reflect.DeepEqual(d.OnlyInB, wantOnlyB) {
		t.Errorf("OnlyInB = %v, want %v", d.OnlyInB, wantOnlyB)
	}
	if !reflect.DeepEqual(d.Shared, wantShared) {
		t.Errorf("Shared = %v, want %v", d.Shared, wantShared)
	}
}

// TestCompareGames_MVPDiff_EmptyOnBothSides pins that empty MVP lists
// produce empty (non-nil) slices — JSON-stable for frontends that
// iterate without nil-checks.
func TestCompareGames_MVPDiff_EmptyOnBothSides(t *testing.T) {
	d := CompareGames(GameSummary{}, GameSummary{}).MVPDiff
	if d.OnlyInA == nil || d.OnlyInB == nil || d.Shared == nil {
		t.Errorf("empty diff slices should be non-nil: %+v", d)
	}
	if len(d.OnlyInA) != 0 || len(d.OnlyInB) != 0 || len(d.Shared) != 0 {
		t.Errorf("empty diff lengths should be 0: %+v", d)
	}
}

// TestCompareGames_TurningPointDivergence pins per-turn grouping and
// the (Kind, Seat) intersection: same kind on the same seat at the
// same turn → BothFired, otherwise routed to OnlyInA / OnlyInB.
func TestCompareGames_TurningPointDivergence(t *testing.T) {
	tpA := []TurningPoint{
		{Turn: 3, Kind: "commander_first_cast", Seat: 0, Description: "A: seat 0 cast cmdr"},
		{Turn: 5, Kind: "commander_first_cast", Seat: 1, Description: "A: seat 1 cast cmdr"},
		{Turn: 12, Kind: "game_end", Seat: 0, Description: "A: seat 0 won"},
	}
	tpB := []TurningPoint{
		{Turn: 3, Kind: "commander_first_cast", Seat: 0, Description: "B: seat 0 cast cmdr"},
		{Turn: 4, Kind: "commander_first_cast", Seat: 2, Description: "B: seat 2 cast cmdr"},
		{Turn: 18, Kind: "game_end", Seat: 1, Description: "B: seat 1 won"},
	}
	a := mkSummary(0, "X", 12, "", nil, nil, tpA)
	b := mkSummary(1, "Y", 18, "", nil, nil, tpB)
	div := CompareGames(a, b).TurningPointDivergence

	byTurn := make(map[int]TurningPointDivergence)
	for _, d := range div {
		byTurn[d.Turn] = d
	}
	// Turn 3: same kind+seat in both → BothFired.
	t3, ok := byTurn[3]
	if !ok {
		t.Fatal("missing turn 3 divergence row")
	}
	if len(t3.BothFired) != 1 || t3.BothFired[0].Kind != "commander_first_cast" || t3.BothFired[0].Seat != 0 {
		t.Errorf("turn 3 BothFired = %+v, want one commander_first_cast for seat 0", t3.BothFired)
	}
	if len(t3.OnlyInA) != 0 || len(t3.OnlyInB) != 0 {
		t.Errorf("turn 3 should have no diverging points: %+v", t3)
	}
	// Turn 4: only in B.
	t4, ok := byTurn[4]
	if !ok || len(t4.OnlyInB) != 1 || len(t4.OnlyInA) != 0 || t4.OnlyInB[0].Seat != 2 {
		t.Errorf("turn 4 should be only-B (seat 2), got %+v", t4)
	}
	// Turn 5: only in A.
	t5, ok := byTurn[5]
	if !ok || len(t5.OnlyInA) != 1 || len(t5.OnlyInB) != 0 || t5.OnlyInA[0].Seat != 1 {
		t.Errorf("turn 5 should be only-A (seat 1), got %+v", t5)
	}
	// Turn 12: game_end only in A.
	t12, ok := byTurn[12]
	if !ok || len(t12.OnlyInA) != 1 || t12.OnlyInA[0].Kind != "game_end" {
		t.Errorf("turn 12 should be only-A game_end, got %+v", t12)
	}
	// Turn 18: game_end only in B.
	t18, ok := byTurn[18]
	if !ok || len(t18.OnlyInB) != 1 || t18.OnlyInB[0].Kind != "game_end" {
		t.Errorf("turn 18 should be only-B game_end, got %+v", t18)
	}
}

// TestCompareGames_TurningPointDivergence_OrderingByTurn pins the
// ascending-turn output ordering. Frontend renders the timeline
// chronologically and relies on this.
func TestCompareGames_TurningPointDivergence_OrderingByTurn(t *testing.T) {
	tpA := []TurningPoint{
		{Turn: 12, Kind: "game_end", Seat: 0},
		{Turn: 3, Kind: "commander_first_cast", Seat: 0},
	}
	tpB := []TurningPoint{
		{Turn: 7, Kind: "commander_first_cast", Seat: 1},
	}
	a := mkSummary(0, "X", 12, "", nil, nil, tpA)
	b := mkSummary(1, "Y", 7, "", nil, nil, tpB)
	div := CompareGames(a, b).TurningPointDivergence

	if len(div) != 3 {
		t.Fatalf("expected 3 rows (turns 3, 7, 12), got %d", len(div))
	}
	if div[0].Turn != 3 || div[1].Turn != 7 || div[2].Turn != 12 {
		t.Errorf("turn order: got %d, %d, %d; want 3, 7, 12", div[0].Turn, div[1].Turn, div[2].Turn)
	}
}

// TestCompareGames_SharedDecks pins the intersection of DeckKeys
// (alpha-sorted, duplicates removed).
func TestCompareGames_SharedDecks(t *testing.T) {
	a := mkSummary(0, "X", 10, "", nil, []string{"alice/krenko", "bob/atraxa", "carol/edgar"}, nil)
	b := mkSummary(1, "Y", 10, "", nil, []string{"carol/edgar", "dave/marrow", "alice/krenko"}, nil)
	shared := CompareGames(a, b).SharedDecks
	want := []string{"alice/krenko", "carol/edgar"}
	if !reflect.DeepEqual(shared, want) {
		t.Errorf("SharedDecks = %v, want %v", shared, want)
	}
}

// TestCompareGames_SharedDecks_EmptyWhenDisjoint pins the disjoint
// case — empty (non-nil) slice.
func TestCompareGames_SharedDecks_EmptyWhenDisjoint(t *testing.T) {
	a := mkSummary(0, "X", 10, "", nil, []string{"alice/a"}, nil)
	b := mkSummary(1, "Y", 10, "", nil, []string{"bob/b"}, nil)
	shared := CompareGames(a, b).SharedDecks
	if shared == nil {
		t.Errorf("SharedDecks should be empty slice (not nil) when disjoint")
	}
	if len(shared) != 0 {
		t.Errorf("SharedDecks = %v, want empty", shared)
	}
}

// TestCompareGames_EmbedsBothSummaries pins that GameA and GameB are
// embedded verbatim — frontends rely on this to render the two-pane
// view without a follow-up summary fetch.
func TestCompareGames_EmbedsBothSummaries(t *testing.T) {
	a := mkSummary(0, "Krenko", 12, "combat", []string{"Sol Ring"}, []string{"alice/krenko"}, nil)
	a.GameID = "100"
	b := mkSummary(1, "Atraxa", 18, "decking", []string{"Cyclonic Rift"}, []string{"bob/atraxa"}, nil)
	b.GameID = "200"

	cmp := CompareGames(a, b)
	if cmp.GameA.GameID != "100" || cmp.GameB.GameID != "200" {
		t.Errorf("GameIDs: a=%s b=%s, want 100/200", cmp.GameA.GameID, cmp.GameB.GameID)
	}
	if cmp.GameA.WinnerName != "Krenko" || cmp.GameB.WinnerName != "Atraxa" {
		t.Errorf("winner names not preserved: %s / %s", cmp.GameA.WinnerName, cmp.GameB.WinnerName)
	}
}
