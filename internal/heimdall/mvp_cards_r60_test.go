package heimdall

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60: ExtractMVPCards picks the top mvpTopN permanents per seat at
// game end by score = CMC + positive_counters + commander_bonus.
// Companion to RegretCards (the "drew it, never cast it" signal).

func newMVPGS(t *testing.T, seats int) *gameengine.GameState {
	t.Helper()
	gs := gameengine.NewGameState(seats, rand.New(rand.NewSource(1)), nil)
	gs.Turn = 10
	gs.CardFirstPlayed = map[string]int{}
	return gs
}

func bfCard(name string, cmc int, types ...string) *gameengine.Card {
	return &gameengine.Card{Name: name, CMC: cmc, Types: types}
}

func bfPerm(c *gameengine.Card) *gameengine.Permanent {
	return &gameengine.Permanent{Card: c, Counters: map[string]int{}}
}

// Basic case: one big permanent on the battlefield surfaces as MVP
// with score = CMC.
func TestExtractMVPCards_SingleHighCMCPermanent(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Blightsteel Colossus", 12, "artifact", "creature")),
	}
	gs.CardFirstPlayed["Blightsteel Colossus"] = 7

	mvps := ExtractMVPCards(gs)
	if len(mvps) != 1 {
		t.Fatalf("want 1 MVP, got %d", len(mvps))
	}
	m := mvps[0]
	if m.Seat != 0 || m.CardName != "Blightsteel Colossus" || m.CMC != 12 || m.Score != 12 || m.TurnPlayed != 7 {
		t.Errorf("unexpected MVP: %+v", m)
	}
	if m.IsCommander {
		t.Errorf("Blightsteel is not a commander here")
	}
}

// Basic lands are filtered — they're not MVPs even when present.
func TestExtractMVPCards_BasicLandsExcluded(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Forest", 0, "land", "basic")),
		bfPerm(bfCard("Mountain", 0, "land", "basic")),
		bfPerm(bfCard("Sol Ring", 1, "artifact")),
	}

	mvps := ExtractMVPCards(gs)
	if len(mvps) != 1 || mvps[0].CardName != "Sol Ring" {
		t.Errorf("want only Sol Ring; got %+v", mvps)
	}
}

// Nonbasic lands are eligible — Gaea's Cradle, Field of the Dead,
// etc. can legitimately be a deck's MVP. The filter is on "basic"
// type, not on "land".
func TestExtractMVPCards_NonbasicLandsEligible(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Gaea's Cradle", 0, "land")),
		bfPerm(bfCard("Plains", 0, "land", "basic")),
	}

	mvps := ExtractMVPCards(gs)
	if len(mvps) != 1 || mvps[0].CardName != "Gaea's Cradle" {
		t.Errorf("want Gaea's Cradle (nonbasic land MVP); got %+v", mvps)
	}
}

// Commander on the battlefield earns the +4 score bonus.
func TestExtractMVPCards_CommanderBonusApplied(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.Seats[0].CommanderNames = []string{"Atraxa, Praetors' Voice"}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Atraxa, Praetors' Voice", 4, "creature", "legendary")),
		bfPerm(bfCard("Blightsteel Colossus", 12, "artifact", "creature")),
	}

	mvps := ExtractMVPCards(gs)
	if len(mvps) != 2 {
		t.Fatalf("want 2 MVPs, got %d", len(mvps))
	}
	byName := map[string]MVPCard{}
	for _, m := range mvps {
		byName[m.CardName] = m
	}
	atraxa := byName["Atraxa, Praetors' Voice"]
	if !atraxa.IsCommander || atraxa.Score != 8 {
		t.Errorf("Atraxa: want IsCommander=true Score=8 (4 CMC + 4 bonus); got %+v", atraxa)
	}
	blightsteel := byName["Blightsteel Colossus"]
	if blightsteel.IsCommander || blightsteel.Score != 12 {
		t.Errorf("Blightsteel: want IsCommander=false Score=12; got %+v", blightsteel)
	}
	// Blightsteel still ranks above commander Atraxa (12 > 8).
	if mvps[0].CardName != "Blightsteel Colossus" {
		t.Errorf("position 0 should be highest-score (Blightsteel); got %s", mvps[0].CardName)
	}
}

// Positive counters add to score: +1/+1, loyalty, charge, level.
// Negative/other counters do not.
func TestExtractMVPCards_PositiveCountersAdded(t *testing.T) {
	gs := newMVPGS(t, 2)
	walker := bfPerm(bfCard("Teferi, Hero of Dominaria", 5, "planeswalker", "legendary"))
	walker.Counters["loyalty"] = 7
	creature := bfPerm(bfCard("Champion of Lambholt", 3, "creature"))
	creature.Counters["+1/+1"] = 9
	creature.Counters["-1/-1"] = 2 // should not subtract
	creature.Counters["stun"] = 1  // should not add
	gs.Seats[0].Battlefield = []*gameengine.Permanent{walker, creature}

	mvps := ExtractMVPCards(gs)
	byName := map[string]MVPCard{}
	for _, m := range mvps {
		byName[m.CardName] = m
	}
	if m := byName["Teferi, Hero of Dominaria"]; m.Counters != 7 || m.Score != 12 {
		t.Errorf("Teferi: want counters=7 score=12 (5+7); got %+v", m)
	}
	if m := byName["Champion of Lambholt"]; m.Counters != 9 || m.Score != 12 {
		t.Errorf("Champion: want counters=9 score=12 (3+9); got %+v", m)
	}
}

// Top-N cap: only the top mvpTopN per seat survive. Lowest-score
// permanent on a 4-permanent battlefield is dropped.
func TestExtractMVPCards_TopNCapAppliedPerSeat(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Lightning Bolt", 1, "creature")), // CMC 1
		bfPerm(bfCard("Sol Ring", 2, "artifact")),       // CMC 2
		bfPerm(bfCard("Counterspell", 3, "creature")),   // CMC 3
		bfPerm(bfCard("Cyclonic Rift", 7, "creature")),  // CMC 7
	}

	mvps := ExtractMVPCards(gs)
	if len(mvps) != mvpTopN {
		t.Fatalf("want %d MVPs (top-N cap), got %d", mvpTopN, len(mvps))
	}
	// Sorted by score desc: Cyclonic Rift, Counterspell, Sol Ring.
	want := []string{"Cyclonic Rift", "Counterspell", "Sol Ring"}
	for i, n := range want {
		if mvps[i].CardName != n {
			t.Errorf("position %d: want %s, got %s", i, n, mvps[i].CardName)
		}
	}
}

// Tie-breaker: equal-score permanents sort by card name ascending.
func TestExtractMVPCards_EqualScoreTieBreakByName(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Zur the Enchanter", 4, "creature", "legendary")),
		bfPerm(bfCard("Atraxa, Praetors' Voice", 4, "creature", "legendary")),
		bfPerm(bfCard("Muldrotha, the Gravetide", 4, "creature", "legendary")),
	}

	mvps := ExtractMVPCards(gs)
	if len(mvps) != mvpTopN {
		t.Fatalf("want %d, got %d", mvpTopN, len(mvps))
	}
	want := []string{"Atraxa, Praetors' Voice", "Muldrotha, the Gravetide", "Zur the Enchanter"}
	for i, n := range want {
		if mvps[i].CardName != n {
			t.Errorf("position %d: want %s, got %s", i, n, mvps[i].CardName)
		}
	}
}

// Multi-seat: each seat contributes up to mvpTopN, results appended
// seat-ascending so consumers can group by seat without re-sorting.
func TestExtractMVPCards_MultiSeatGrouping(t *testing.T) {
	gs := newMVPGS(t, 4)
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Sol Ring", 1, "artifact")),
	}
	gs.Seats[2].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Craterhoof Behemoth", 8, "creature")),
		bfPerm(bfCard("Llanowar Elves", 1, "creature")),
	}

	mvps := ExtractMVPCards(gs)
	if len(mvps) != 3 {
		t.Fatalf("want 3 MVPs (1+2 across seats), got %d", len(mvps))
	}
	if mvps[0].Seat != 0 || mvps[1].Seat != 2 || mvps[2].Seat != 2 {
		t.Errorf("want seat ordering [0,2,2]; got [%d,%d,%d]",
			mvps[0].Seat, mvps[1].Seat, mvps[2].Seat)
	}
}

// TurnPlayed comes from gs.CardFirstPlayed when present, 0 otherwise
// (tokens, copies, never-recorded permanents).
func TestExtractMVPCards_TurnPlayedFromFirstPlayedMap(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.CardFirstPlayed["Atraxa, Praetors' Voice"] = 4
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Atraxa, Praetors' Voice", 4, "creature", "legendary")),
		bfPerm(bfCard("Soldier Token", 0, "creature")),
	}

	mvps := ExtractMVPCards(gs)
	byName := map[string]MVPCard{}
	for _, m := range mvps {
		byName[m.CardName] = m
	}
	if m := byName["Atraxa, Praetors' Voice"]; m.TurnPlayed != 4 {
		t.Errorf("Atraxa: want TurnPlayed=4 (from map); got %d", m.TurnPlayed)
	}
	if m := byName["Soldier Token"]; m.TurnPlayed != 0 {
		t.Errorf("Soldier Token: want TurnPlayed=0 (unrecorded); got %d", m.TurnPlayed)
	}
}

// Defensive: nil game state and nil battlefield entries.
func TestExtractMVPCards_NilGameStateReturnsNil(t *testing.T) {
	if got := ExtractMVPCards(nil); got != nil {
		t.Errorf("nil gs should return nil, got %v", got)
	}
}

func TestExtractMVPCards_NilPermanentIgnored(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		nil,
		bfPerm(bfCard("Sol Ring", 1, "artifact")),
		nil,
		{Card: nil}, // permanent with nil Card
	}

	mvps := ExtractMVPCards(gs)
	if len(mvps) != 1 || mvps[0].CardName != "Sol Ring" {
		t.Errorf("nil entries should be skipped; got %+v", mvps)
	}
}

// Partner pair: both partners on the battlefield each get the
// commander bonus.
func TestExtractMVPCards_PartnerPairBothGetCommanderBonus(t *testing.T) {
	gs := newMVPGS(t, 2)
	gs.Seats[0].CommanderNames = []string{"Tymna the Weaver", "Thrasios, Triton Hero"}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		bfPerm(bfCard("Tymna the Weaver", 3, "creature", "legendary")),
		bfPerm(bfCard("Thrasios, Triton Hero", 4, "creature", "legendary")),
	}

	mvps := ExtractMVPCards(gs)
	byName := map[string]MVPCard{}
	for _, m := range mvps {
		byName[m.CardName] = m
	}
	if m := byName["Tymna the Weaver"]; !m.IsCommander || m.Score != 7 {
		t.Errorf("Tymna: want commander=true score=7 (3+4); got %+v", m)
	}
	if m := byName["Thrasios, Triton Hero"]; !m.IsCommander || m.Score != 8 {
		t.Errorf("Thrasios: want commander=true score=8 (4+4); got %+v", m)
	}
}
