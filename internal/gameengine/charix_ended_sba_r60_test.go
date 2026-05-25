package gameengine

import (
	"math/rand"
	"strings"
	"testing"
)

// Loki r60 deep-stress (seed 2025, game 3180) surfaced a single
// SBACompleteness violation: Charix, the Raging Isle on the battlefield
// with toughness=-7 (base 0/17, three stacked +8/-8 mods from sequential
// activations). The mods are rules-correct per CR §608.2b — each
// activation creates a separate continuous effect that stacks. The
// invariant fired because the third activation resolved AFTER the game
// ended in combat (seat 0 [WON], all opponents [LOST]); StateBasedActions
// short-circuits on `gs.Flags["ended"] == 1` (sba.go:41), so the §704.5f
// pass that would have moved Charix to the graveyard never ran.
//
// Fix: mirror checkStackIntegrity's ended-skip in checkSBACompleteness.
// Once the game has ended, residual SBA state is rules-irrelevant and
// the invariant should not flag it.

// charixEndedFixture builds a 2-seat state with a Charix-shaped creature
// (base 0/17) carrying three +8/-8 mods, producing toughness=-7 on the
// battlefield. Caller chooses whether the game has ended.
func charixEndedFixture(t *testing.T, ended bool) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Flags = map[string]int{}
	if ended {
		gs.Flags["ended"] = 1
	}
	perm := &Permanent{
		Card: &Card{
			Name:          "Charix, the Raging Isle",
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 17,
		},
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Modifications: []Modification{
			{Power: 8, Toughness: -8, Duration: "until_end_of_turn", Timestamp: 1},
			{Power: 8, Toughness: -8, Duration: "until_end_of_turn", Timestamp: 2},
			{Power: 8, Toughness: -8, Duration: "until_end_of_turn", Timestamp: 3},
		},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	if ended {
		// Mirror the deep-stress snapshot: opponents already eliminated.
		gs.Seats[1].Lost = true
	}
	return gs
}

func TestSBACompleteness_SkipsToughnessCheckWhenGameEnded(t *testing.T) {
	gs := charixEndedFixture(t, true)
	if got := gs.ToughnessOf(gs.Seats[0].Battlefield[0]); got != -7 {
		t.Fatalf("fixture sanity: expected toughness -7 from 17 + 3×(-8), got %d", got)
	}
	if err := checkSBACompleteness(gs); err != nil {
		t.Fatalf("SBACompleteness toughness check should skip once gs.Flags[\"ended\"]==1, got: %v", err)
	}
}

// TestSBACompleteness_LifeCheckStillFiresWhenEnded pins the narrowness
// of the ended-skip: the life-loss arm must still flag an alive seat at
// life ≤ 0 even when ended=1. This is what catches the cap-draw r60
// zombie-game shape (ended=1 set but seats not properly marked Lost).
// Mirrors sba_cap_draw_r60_test.go's counterfactual contract.
func TestSBACompleteness_LifeCheckStillFiresWhenEnded(t *testing.T) {
	gs := charixEndedFixture(t, true)
	// Reset the Charix mods so the toughness arm wouldn't fire either
	// way — we're isolating the life-check arm.
	gs.Seats[0].Battlefield[0].Modifications = nil
	// Reanimate seat 1 at life=0 to simulate the cap-draw zombie shape.
	gs.Seats[1].Lost = false
	gs.Seats[1].Life = 0
	err := checkSBACompleteness(gs)
	if err == nil {
		t.Fatal("life-loss arm must still fire on alive seat at life ≤ 0 even when ended=1 (cap-draw regression pin)")
	}
	if !strings.Contains(err.Error(), "704.5a missed") {
		t.Fatalf("expected 704.5a missed diagnostic, got: %v", err)
	}
}

func TestSBACompleteness_FiresWhenGameLive(t *testing.T) {
	gs := charixEndedFixture(t, false)
	err := checkSBACompleteness(gs)
	if err == nil {
		t.Fatal("SBACompleteness must still fire on toughness ≤ 0 in a live game")
	}
	if !strings.Contains(err.Error(), "704.5f missed") {
		t.Fatalf("expected 704.5f missed diagnostic, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Charix") {
		t.Fatalf("expected diagnostic to name Charix, got: %v", err)
	}
}

// Defense-in-depth: confirm StateBasedActions itself bails out on the
// ended flag — this is the upstream cause of the residual state the
// invariant skip is forgiving. If a future change makes SBAs continue
// past game-end, the invariant skip becomes dead code and this test
// will flag it.
func TestStateBasedActions_ShortCircuitsWhenEnded(t *testing.T) {
	gs := charixEndedFixture(t, true)
	if StateBasedActions(gs) {
		t.Fatal("StateBasedActions must report no change once the game has ended")
	}
	// Charix still on the battlefield post-call.
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("Charix should remain on battlefield post-ended SBA call, got len=%d", len(gs.Seats[0].Battlefield))
	}
}
