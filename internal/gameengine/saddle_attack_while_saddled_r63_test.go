package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// saddle_attack_while_saddled_r63_test.go — r63 mechanic-probe (CR §702.171c).
// A Mount's "Whenever this creature attacks while saddled, …" trigger parses
// with event "attack_while_saddled" (aliased to "attack"), and the "while
// saddled" condition is encoded in that event NAME — there is no separate
// Condition/intervening-if. Generic-AST Mounts (no per_card handler:
// Gloryheath Lynx, Drover Grizzly, Dracosaur Auxiliary, …) fired the saddled
// bonus on EVERY attack because the self-attack dispatch never checked
// PermIsSaddled. Fix: gate event-name-encoded "saddled" attack triggers on
// the live saddled state in fireAttackTriggers. (BYPASS class.)

func countAttackTriggerFires(gs *GameState, source string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "trigger_fires" && ev.Source == source {
			if e, _ := ev.Details["event"].(string); e == "attack" {
				n++
			}
		}
	}
	return n
}

func addAttackTriggerCreature(gs *GameState, seat int, name, event string, types ...string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     2,
		BaseToughness: 2,
		Types:         append([]string{"creature"}, types...),
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Triggered{Trigger: gameast.Trigger{Event: event}},
			},
		},
	}
	p := &Permanent{
		Card: card, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// (BYPASS fix) attack_while_saddled fires only while the Mount is saddled.
func TestSaddle_AttackWhileSaddledFiresOnlyWhenSaddled(t *testing.T) {
	gs := newKWCombatGame(t)
	mount := addAttackTriggerCreature(gs, 0, "Probe Mount", "attack_while_saddled", "mount")

	// Unsaddled attack → the saddled-gated trigger must NOT fire.
	fireAttackTriggers(gs, 0, []*Permanent{mount})
	if got := countAttackTriggerFires(gs, "Probe Mount"); got != 0 {
		t.Errorf("attack_while_saddled must NOT fire while unsaddled, fired %d", got)
	}

	// Saddle it, then attack → the trigger fires.
	mount.Flags["saddled"] = 1
	fireAttackTriggers(gs, 0, []*Permanent{mount})
	if got := countAttackTriggerFires(gs, "Probe Mount"); got != 1 {
		t.Errorf("attack_while_saddled must fire exactly once while saddled, fired %d", got)
	}
}

// Control: a plain "whenever this attacks" trigger is unaffected by the gate —
// it fires regardless of saddled state (the fix only gates saddled-encoded
// events, not all attack triggers).
func TestSaddle_PlainAttackTriggerUnaffectedByGate(t *testing.T) {
	gs := newKWCombatGame(t)
	plain := addAttackTriggerCreature(gs, 0, "Plain Attacker", "attack")

	fireAttackTriggers(gs, 0, []*Permanent{plain})
	if got := countAttackTriggerFires(gs, "Plain Attacker"); got != 1 {
		t.Errorf("a plain attack trigger must fire on attack regardless of saddled state, fired %d", got)
	}
}
