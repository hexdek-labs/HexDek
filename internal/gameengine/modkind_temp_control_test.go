package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// modkind_temp_control_test.go — pins the generic temp_control handler
// (Threaten / Act of Treason): steal an opponent creature until EOT,
// untap it, give it haste, and revert at the cleanup step.

func tcPerm(gs *GameState, seat int, name string, power int, types ...string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Owner: seat, Types: append([]string{}, types...), BasePower: power, BaseToughness: power},
		Controller: seat,
		Owner:      seat,
		Tapped:     true, // start tapped to prove the untap
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func bfHas(gs *GameState, seat int, p *Permanent) bool {
	for _, q := range gs.Seats[seat].Battlefield {
		if q == p {
			return true
		}
	}
	return false
}

func runTempControl(gs *GameState, src *Permanent) {
	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: "temp_control"})
}

func TestTempControl_StealsUntapsAndGivesHaste(t *testing.T) {
	gs := newTestGameState(2)
	src := tcPerm(gs, 0, "Act of Treason", 0, "sorcery")
	small := tcPerm(gs, 1, "Grizzly Bears", 2, "creature", "bear")
	big := tcPerm(gs, 1, "Colossal Dreadmaw", 6, "creature", "dinosaur")

	runTempControl(gs, src)

	// The highest-power opponent creature is stolen to seat 0.
	if big.Controller != 0 || !bfHas(gs, 0, big) {
		t.Fatalf("Act of Treason should steal the biggest opponent creature to seat 0")
	}
	if bfHas(gs, 1, big) {
		t.Error("stolen creature must leave the opponent's battlefield")
	}
	if big.Tapped {
		t.Error("stolen creature must be untapped")
	}
	if big.SummoningSick {
		t.Error("stolen creature must be able to attack (not summoning sick)")
	}
	if !big.HasKeyword("haste") {
		t.Error("stolen creature must gain haste")
	}
	// Owner is unchanged (CR §108.3).
	if big.Owner != 1 {
		t.Errorf("owner must remain seat 1, got %d", big.Owner)
	}
	_ = small
}

func TestTempControl_RevertsAtEndOfTurn(t *testing.T) {
	gs := newTestGameState(2)
	src := tcPerm(gs, 0, "Threaten", 0, "sorcery")
	creature := tcPerm(gs, 1, "Serra Angel", 4, "creature", "angel")

	runTempControl(gs, src)
	if creature.Controller != 0 {
		t.Fatalf("precondition: creature should be stolen to seat 0")
	}

	// Cleanup step reverts until-EOT control grants.
	ExpireTempControlGrants(gs)

	if creature.Controller != 1 || !bfHas(gs, 1, creature) {
		t.Fatalf("creature must return to its owner (seat 1) at end of turn, controller=%d", creature.Controller)
	}
	if bfHas(gs, 0, creature) {
		t.Error("creature must leave the thief's battlefield after revert")
	}
}

func TestTempControl_NoOpponentCreature_NoOp(t *testing.T) {
	gs := newTestGameState(2)
	src := tcPerm(gs, 0, "Act of Treason", 0, "sorcery")
	// Opponent controls only a noncreature.
	tcPerm(gs, 1, "Sol Ring", 0, "artifact")

	runTempControl(gs, src) // must not panic / steal nothing

	if len(gs.TempControlGrants) != 0 {
		t.Errorf("no creature to steal → no temp-control grant should be registered")
	}
}
