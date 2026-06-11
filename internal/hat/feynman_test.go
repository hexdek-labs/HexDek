package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func newFeynmanGame(t *testing.T, nSeats int) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, nSeats)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
		// Give each seat a 100-card library to satisfy zone accounting.
		for j := 0; j < 100; j++ {
			gs.Seats[i].Library = append(gs.Seats[i].Library,
				&gameengine.Card{Name: "Card", Owner: i})
		}
	}
	return gs
}

func TestFeynman_CleanGame(t *testing.T) {
	gs := newFeynmanGame(t, 4)
	// Normal end: 3 seats lost.
	gs.Seats[1].Lost = true
	gs.Seats[2].Lost = true
	gs.Seats[3].Lost = true
	gs.Turn = 15

	result := CheckGame(gs)
	if !result.Clean() {
		t.Errorf("clean game should have no violations, got %d: %v",
			len(result.Violations), result.Violations)
	}
	if result.Checked < 5 {
		t.Errorf("should check at least 5 invariants, checked %d", result.Checked)
	}
}

func TestFeynman_LifeSBA(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	gs.Seats[0].Life = -5
	gs.Seats[0].Lost = false // BUG: should be lost
	gs.Seats[1].Lost = false
	gs.Turn = 10

	result := CheckGame(gs)
	found := false
	for _, v := range result.Violations {
		if v.Rule == "704.5a" {
			found = true
		}
	}
	if !found {
		t.Error("expected 704.5a violation for seat 0 with negative life")
	}
}

func TestFeynman_PoisonSBA(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	gs.Seats[0].PoisonCounters = 10
	gs.Seats[0].Lost = false // BUG
	gs.Seats[1].Lost = false
	gs.Turn = 8

	result := CheckGame(gs)
	found := false
	for _, v := range result.Violations {
		if v.Rule == "704.5c" {
			found = true
		}
	}
	if !found {
		t.Error("expected 704.5c violation for 10 poison")
	}
}

func TestFeynman_TurnBounds(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	gs.Turn = 250
	gs.Seats[1].Lost = true

	result := CheckGame(gs)
	found := false
	for _, v := range result.Violations {
		if v.Rule == "turn_bound" {
			found = true
		}
	}
	if !found {
		t.Error("expected turn_bound violation for 250-turn game")
	}
}

// hasGameEndViolation reports whether CheckGame flagged a game_end violation.
func hasGameEndViolation(gs *gameengine.GameState) bool {
	for _, v := range CheckGame(gs).Violations {
		if v.Rule == "game_end" {
			return true
		}
	}
	return false
}

// r61 correction: the "exactly N-1 lost" invariant only applies to a CR §104.2a
// last-seat-standing win (Flags ended+winner, one seat alive). The cases below
// are LEGITIMATE non-§104.2a terminal/incomplete states that the old check
// flagged as ~8,764 false positives across the fishtank — they must NOT flag.

func TestFeynman_IncompleteGame_NotFlagged(t *testing.T) {
	gs := newFeynmanGame(t, 4)
	// No ended flag, nobody lost — an in-progress / prematurely-snapshotted
	// game, not a §104.2a win. Must not flag game_end.
	gs.Turn = 10
	if hasGameEndViolation(gs) {
		t.Error("incomplete game (no ended/winner flag) should NOT flag game_end")
	}
}

func TestFeynman_TurnCapped_PartialLosers_NotFlagged(t *testing.T) {
	gs := newFeynmanGame(t, 4)
	// Turn-cap finish: the loop exits without CheckEnd setting ended; the
	// leader wins on life but the other living seats are NOT marked Lost.
	// 2 of 3 lost is legitimate here, not a bug.
	gs.Seats[1].Lost = true
	gs.Seats[2].Lost = true
	gs.Turn = 80
	if hasGameEndViolation(gs) {
		t.Error("turn-capped game with partial losers (no ended flag) should NOT flag game_end")
	}
}

func TestFeynman_TurnCapped_NobodyLost_NotFlagged(t *testing.T) {
	gs := newFeynmanGame(t, 4)
	gs.Turn = 80 // turn cap, nobody eliminated, no ended/winner flag
	if hasGameEndViolation(gs) {
		t.Error("turn-capped game with 0 losers (no ended flag) should NOT flag game_end")
	}
}

func TestFeynman_WinEffect_OpponentsAlive_NotFlagged(t *testing.T) {
	gs := newFeynmanGame(t, 4)
	// CR §104.2c "you win the game" effect (Thassa's Oracle etc.): a winner is
	// set without every opponent dying. ended + winner but >1 seat alive.
	gs.Flags = map[string]int{"ended": 1, "winner": 0}
	gs.Seats[1].Lost = true // only one opponent dead; seats 2,3 alive
	if hasGameEndViolation(gs) {
		t.Error("win-effect win with living opponents should NOT flag game_end")
	}
}

func TestFeynman_LastSeatStanding_CleanWin_NotFlagged(t *testing.T) {
	gs := newFeynmanGame(t, 4)
	// CR §104.2a clean elimination win: ended + winner + exactly one alive +
	// N-1 lost. Must NOT flag.
	gs.Flags = map[string]int{"ended": 1, "winner": 0}
	gs.Seats[1].Lost = true
	gs.Seats[2].Lost = true
	gs.Seats[3].Lost = true
	if hasGameEndViolation(gs) {
		t.Error("clean last-seat-standing win should NOT flag game_end")
	}
}

func TestFeynman_ZoneAccounting_CopyTolerance(t *testing.T) {
	gs := newFeynmanGame(t, 4)
	gs.Seats[1].Lost = true
	gs.Seats[2].Lost = true
	gs.Seats[3].Lost = true
	gs.Turn = 15

	// Add 15 extra non-token cards to seat 0 (simulating copy/clone effects).
	for i := 0; i < 15; i++ {
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
			&gameengine.Permanent{
				Card:       &gameengine.Card{Name: "Clone Copy", Owner: 0, Types: []string{"creature"}, IsCopy: true},
				Controller: 0,
				Owner:      0,
				Flags:      map[string]int{},
			})
	}

	result := CheckGame(gs)
	for _, v := range result.Violations {
		if v.Rule == "zone_accounting" && v.Seat == 0 {
			t.Errorf("15 copy-created cards should not trigger zone_accounting (positive tolerance is 20), got: %v", v)
		}
	}
}

func TestFeynman_ZoneAccounting_NegativeDiffStillFlags(t *testing.T) {
	gs := newFeynmanGame(t, 4)
	gs.Seats[1].Lost = true
	gs.Seats[2].Lost = true
	gs.Seats[3].Lost = true
	gs.Turn = 15

	// Remove 5 cards from seat 0's library (simulating missing cards bug).
	gs.Seats[0].Library = gs.Seats[0].Library[:95]

	result := CheckGame(gs)
	found := false
	for _, v := range result.Violations {
		if v.Rule == "zone_accounting" && v.Seat == 0 {
			found = true
		}
	}
	if !found {
		t.Error("negative diff of -5 should still trigger zone_accounting violation")
	}
}

func TestFeynman_PermanentTypes_Clean(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	creature := &gameengine.Card{
		Name:     "Llanowar Elves",
		Owner:    0,
		Types:    []string{"creature"},
		TypeLine: "Creature — Elf Druid",
	}
	newTestPermanent(gs.Seats[0], creature, 1, 1)
	// Blood-Moon-style: a non-basic land gains the Mountain subtype.
	// Runtime types ["land","mountain"] and a "Land" type line are both
	// valid permanent shapes.
	moonLand := &gameengine.Card{
		Name:     "Reflecting Pool",
		Owner:    0,
		Types:    []string{"land", "mountain"},
		TypeLine: "Land",
	}
	newTestPermanent(gs.Seats[0], moonLand, 0, 0)
	gs.Seats[1].Lost = true
	gs.Turn = 5

	result := CheckGame(gs)
	for _, v := range result.Violations {
		if v.Rule == "permanent_types" {
			t.Errorf("clean board should not have permanent_types violation: %v", v)
		}
	}
}

func TestFeynman_PermanentTypes_InstantOnBattlefield(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	bogus := &gameengine.Card{
		Name:     "Lightning Bolt",
		Owner:    0,
		Types:    []string{"instant"},
		TypeLine: "Instant",
	}
	newTestPermanent(gs.Seats[0], bogus, 0, 0)
	gs.Seats[1].Lost = true
	gs.Turn = 5

	result := CheckGame(gs)
	foundType, foundTypeLine := false, false
	for _, v := range result.Violations {
		if v.Rule != "permanent_types" {
			continue
		}
		if v.Severity != "critical" {
			t.Errorf("expected critical severity, got %s", v.Severity)
		}
		if got, _ := v.Details["type"].(string); got == "instant" {
			foundType = true
		}
		if got, _ := v.Details["type_line"].(string); got == "Instant" {
			foundTypeLine = true
		}
	}
	if !foundType {
		t.Error("expected permanent_types violation for runtime type=instant")
	}
	if !foundTypeLine {
		t.Error("expected permanent_types violation for printed TypeLine=Instant")
	}
}

func TestFeynman_PermanentTypes_HumilityToleratesStrippedCreatureType(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	// Type-stripping continuous effect: runtime "creature" type was
	// removed but the printed type line still says Creature. Legal
	// post-effect state — must NOT be flagged.
	stripped := &gameengine.Card{
		Name:     "Grizzly Bears",
		Owner:    0,
		Types:    []string{},
		TypeLine: "Creature — Bear",
	}
	newTestPermanent(gs.Seats[0], stripped, 2, 2)
	gs.Seats[1].Lost = true
	gs.Turn = 5

	result := CheckGame(gs)
	for _, v := range result.Violations {
		if v.Rule == "permanent_types" {
			t.Errorf("type-stripped creature should not be flagged: %v", v)
		}
	}
}

func TestFeynman_PermanentTypes_TokenSkipped(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	tok := &gameengine.Card{
		Name:     "Soldier Token",
		Owner:    0,
		Types:    []string{"creature", "token"},
		TypeLine: "",
	}
	newTestPermanent(gs.Seats[0], tok, 1, 1)
	gs.Seats[1].Lost = true
	gs.Turn = 5

	result := CheckGame(gs)
	for _, v := range result.Violations {
		if v.Rule == "permanent_types" {
			t.Errorf("token should be skipped, got: %v", v)
		}
	}
}

func TestFeynman_FormatViolations(t *testing.T) {
	violations := []OracleViolation{
		{Rule: "704.5a", Description: "test", Seat: 0, Severity: "critical"},
	}
	s := FormatViolations(violations)
	if s == "" {
		t.Error("format should produce non-empty string")
	}
	clean := FormatViolations(nil)
	if clean == "" {
		t.Error("clean format should produce non-empty string")
	}
}
