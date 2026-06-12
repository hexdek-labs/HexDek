package per_card

// Lyzolda, the Blood Witch — goldilocks A-M round-2 regression. Covers
// both victim paths: dispatcher-paid (sacrificed_perm in ctx, the real
// activation flow post-6bc9caab) and handler-paid (per_card-only /
// goldilocks InvokeActivatedHook flow), plus the color riders.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func lyzoldaFixture(t *testing.T) (*gameengine.GameState, *gameengine.Permanent) {
	t.Helper()
	gs := newGame(t, 2)
	lyz := addPerm(gs, 0, "Lyzolda, the Blood Witch", "legendary", "creature")
	addLibrary(gs, 0, "L1", "L2")
	return gs, lyz
}

// Dispatcher path: red+black victim already sacrificed by the engine —
// the handler must NOT re-sacrifice, and both riders fire.
func TestLyzolda_DispatcherVictim_RakdosColorsBothRiders(t *testing.T) {
	gs, lyz := lyzoldaFixture(t)
	victim := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Rakdos Sac", Owner: 0,
			Types:  []string{"creature"},
			Colors: []string{"B", "R"},
		},
		Controller: 0, Owner: 0,
	}
	bystander := addPerm(gs, 0, "Bystander", "creature")
	oppLife := gs.Seats[1].Life
	hand := len(gs.Seats[0].Hand)

	lyzoldaBloodWitchActivate(gs, lyz, 0, map[string]interface{}{
		"sacrificed_perm": victim,
	})

	if gs.Seats[1].Life != oppLife-2 {
		t.Errorf("red rider: want opponent at %d, got %d", oppLife-2, gs.Seats[1].Life)
	}
	if len(gs.Seats[0].Hand) != hand+1 {
		t.Errorf("black rider: want %d cards in hand, got %d", hand+1, len(gs.Seats[0].Hand))
	}
	if !permOnBattlefield(gs, bystander) {
		t.Error("handler must not sacrifice anything when the dispatcher already paid")
	}
}

// Handler-paid path (goldilocks shape): no ctx victim — the handler
// sacrifices a creature itself, preferring red/black, never Lyzolda.
func TestLyzolda_HandlerPaidPath_PrefersRedBlackVictim(t *testing.T) {
	gs, lyz := lyzoldaFixture(t)
	colorless := addPerm(gs, 0, "DamageFodder 0", "creature")
	red := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Red Fodder", Owner: 0,
			Types:  []string{"creature"},
			Colors: []string{"R"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, red)
	oppLife := gs.Seats[1].Life

	lyzoldaBloodWitchActivate(gs, lyz, 0, map[string]interface{}{"controller": 0})

	if permOnBattlefield(gs, red) {
		t.Error("red victim should have been chosen and sacrificed")
	}
	if !permOnBattlefield(gs, colorless) {
		t.Error("colorless fodder should survive when a red victim exists")
	}
	if !permOnBattlefield(gs, lyz) {
		t.Error("Lyzolda must never sacrifice herself while other creatures exist")
	}
	if gs.Seats[1].Life != oppLife-2 {
		t.Errorf("red rider after handler-paid sac: want %d, got %d", oppLife-2, gs.Seats[1].Life)
	}
}

// Colorless victim: cost paid, no riders — still observable (the
// sacrifice) and no spurious damage/draw.
func TestLyzolda_ColorlessVictim_NoRiders(t *testing.T) {
	gs, lyz := lyzoldaFixture(t)
	fodder := addPerm(gs, 0, "DamageFodder 0", "creature")
	oppLife := gs.Seats[1].Life
	hand := len(gs.Seats[0].Hand)

	lyzoldaBloodWitchActivate(gs, lyz, 0, nil)

	if permOnBattlefield(gs, fodder) {
		t.Error("fodder should have been sacrificed")
	}
	if gs.Seats[1].Life != oppLife || len(gs.Seats[0].Hand) != hand {
		t.Error("colorless victim must trigger neither rider")
	}
}

func permOnBattlefield(gs *gameengine.GameState, target *gameengine.Permanent) bool {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == target {
				return true
			}
		}
	}
	return false
}
