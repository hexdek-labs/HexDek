package per_card

// Regressions for the r63 A-M goldilocks dead-effect fixes. Each test
// mirrors the goldilocks battery's hook path (InvokeETBHook for the
// ETB untaps, InvokeActivatedHook for the random shot) so a pass here
// predicts a goldilocks pass.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func addTappedLand(gs *gameengine.GameState, seat int, name string) *gameengine.Permanent {
	p := addPerm(gs, seat, name, "land", "basic")
	p.Tapped = true
	return p
}

func countTapped(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Tapped {
			n++
		}
	}
	return n
}

// Cloud of Faeries: ETB untaps up to two lands, own lands first.
func TestCloudOfFaeries_ETBUntapsUpToTwoLands(t *testing.T) {
	gs := newGame(t, 2)
	addTappedLand(gs, 0, "Island A")
	addTappedLand(gs, 0, "Island B")
	addTappedLand(gs, 0, "Island C")
	tappedCreature := addPerm(gs, 0, "Bystander Bear", "creature")
	tappedCreature.Tapped = true

	cof := addPerm(gs, 0, "Cloud of Faeries", "creature", "faerie")
	gameengine.InvokeETBHook(gs, cof)

	if got := countTapped(gs, 0); got != 2 { // 3 lands + 1 creature tapped; 2 lands untap
		t.Errorf("want exactly 2 permanents still tapped (1 land + the creature), got %d", got)
	}
	if !tappedCreature.Tapped {
		t.Error("untap must only pick lands — the creature should stay tapped")
	}
}

// The "up to" is land-only and crosses seats only when the controller
// has no tapped land (the goldilocks scaffold shape: the only tapped
// land on the board belongs to the opponent).
func TestCloudOfFaeries_FallsThroughToAnySeatLands(t *testing.T) {
	gs := newGame(t, 2)
	oppLand := addTappedLand(gs, 1, "Opponent Forest")

	cof := addPerm(gs, 0, "Cloud of Faeries", "creature", "faerie")
	gameengine.InvokeETBHook(gs, cof)

	if oppLand.Tapped {
		t.Error("with no own tapped land, the up-to-two untap should reach the opponent's tapped land (oracle: 'up to two lands', any)")
	}
}

// Great Whale: same shape, up to seven.
func TestGreatWhale_ETBUntapsUpToSevenLands(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 9; i++ {
		addTappedLand(gs, 0, "Island")
	}
	whale := addPerm(gs, 0, "Great Whale", "creature", "whale")
	gameengine.InvokeETBHook(gs, whale)

	if got := countTapped(gs, 0); got != 2 { // 9 tapped - 7 untapped
		t.Errorf("want 2 lands still tapped after untapping 7 of 9, got %d", got)
	}
}

// Goblin Test Pilot: the random shot lands on SOMETHING — a player
// loses life or a creature gains marked damage — via the activated
// hook (the dispatcher settles the {T} cost; not under test).
func TestGoblinTestPilot_RandomShotIsObservable(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(7)), nil)
	bear := addPerm(gs, 1, "Target Bear", "creature")

	pilot := addPerm(gs, 0, "Goblin Test Pilot", "creature", "goblin", "pilot")
	life0, life1 := gs.Seats[0].Life, gs.Seats[1].Life

	gameengine.InvokeActivatedHook(gs, pilot, 0, map[string]interface{}{"controller": 0})

	changed := gs.Seats[0].Life != life0 || gs.Seats[1].Life != life1 ||
		bear.MarkedDamage > 0 || pilot.MarkedDamage > 0
	if !changed {
		t.Fatal("random 2-damage shot produced no observable change (dead effect)")
	}
}

// Determinism pin: same seed, same target.
func TestGoblinTestPilot_DeterministicUnderSeed(t *testing.T) {
	run := func() (int, int, int, int) {
		gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
		bear := addPerm(gs, 1, "Target Bear", "creature")
		pilot := addPerm(gs, 0, "Goblin Test Pilot", "creature", "goblin")
		gameengine.InvokeActivatedHook(gs, pilot, 0, map[string]interface{}{"controller": 0})
		return gs.Seats[0].Life, gs.Seats[1].Life, bear.MarkedDamage, pilot.MarkedDamage
	}
	a0, a1, a2, a3 := run()
	b0, b1, b2, b3 := run()
	if a0 != b0 || a1 != b1 || a2 != b2 || a3 != b3 {
		t.Errorf("same seed must pick the same target: run1=(%d,%d,%d,%d) run2=(%d,%d,%d,%d)",
			a0, a1, a2, a3, b0, b1, b2, b3)
	}
}
