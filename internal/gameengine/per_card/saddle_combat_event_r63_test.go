package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// saddle_combat_event_r63_test.go — mechanic-probe regression for SADDLE
// property (c) "attacks while saddled" via the REAL combat event name.
//
// The existing mount_saddle_consumers tests fire the already-canonical
// "attack" event. The live combat path (combat.go fireAttackTriggers) fires
// "creature_attacks", which NormalizeEventSingle maps to "attack". This pins
// that the saddled-attack payoff actually reaches the handler through the
// alias when combat dispatches its real event name — the gap the existing
// suite bypasses by firing "attack" directly.
func TestSaddle_AttackWhileSaddled_RealCombatEventName(t *testing.T) {
	gs := newGame(t, 2)
	jaguar := addPerm(gs, 0, "Alacrian Jaguar", "creature", "mount")
	jaguar.Card.BasePower, jaguar.Card.BaseToughness = 3, 3
	saddleMount(jaguar)

	// Fire the actual event combat emits, NOT the canonical "attack".
	gameengine.FireCardTrigger(gs, "creature_attacks", attackCtx(jaguar))

	if jaguar.Power() != 5 || jaguar.Toughness() != 5 {
		t.Errorf("saddled-attack +2/+2 must fire on the real combat event "+
			"creature_attacks (alias→attack); got %d/%d", jaguar.Power(), jaguar.Toughness())
	}
}
