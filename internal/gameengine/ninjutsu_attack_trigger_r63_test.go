package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// countTriggerFiresR63 counts "trigger_fires" events for a given source card
// and trigger event ("attack" / "deals_combat_damage").
func countTriggerFiresR63(gs *GameState, source, event string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind != "trigger_fires" || ev.Source != source {
			continue
		}
		if e, _ := ev.Details["event"].(string); e == event {
			n++
		}
	}
	return n
}

// TestNinjutsu_NoAttackTriggerButCombatDamageTriggerFires pins the CR
// §509.1 / §702.49b cardinality contract that intersects the bucket-1/2/3
// attack-trigger work: a creature put onto the battlefield by ninjutsu was
// PUT into the attacking state — it was never DECLARED as an attacker — so
// it must NOT fire "whenever this/a creature you control attacks" triggers.
// Combat-damage triggers, by contrast, fire normally when it connects.
func TestNinjutsu_NoAttackTriggerButCombatDamageTriggerFires(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	// Unblocked attacker to bounce as the ninjutsu cost.
	attacker := &Permanent{
		Card: &Card{
			Name: "Ornithopter", Owner: 0,
			Types: []string{"creature"}, BasePower: 0, BaseToughness: 2,
		},
		Controller: 0, Owner: 0, Flags: map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	// Ninja in hand carrying ninjutsu PLUS both an "attack" trigger and a
	// "deals_combat_damage" trigger. iterAttackTriggers / the combat-damage
	// AST loop pick these up, so the trigger_fires log is an honest probe:
	// if the ninjutsu path ever wrongly routed the ninja through attack
	// dispatch, the attack trigger_fires count would be non-zero.
	ninja := &Card{
		Name: "Ninja of the Deep Hours", Owner: 0,
		Types: []string{"creature", "ninja", "cost:4"}, BasePower: 2, BaseToughness: 2,
		AST: &gameast.CardAST{
			Name: "Ninja of the Deep Hours",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
				&gameast.Triggered{Trigger: gameast.Trigger{Event: "attack"}},
				&gameast.Triggered{Trigger: gameast.Trigger{Event: "deals_combat_damage"}},
			},
		},
	}
	seat.Hand = append(seat.Hand, ninja)
	seat.ManaPool = 5

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{attacker: nil}

	attackers = CheckNinjutsu(gs, 0, attackers, blockerMap)

	// Sanity: the ninja is in the attacking state but was NOT declared.
	var ninjaPerm *Permanent
	for _, p := range seat.Battlefield {
		if p.Card != nil && p.Card.Name == "Ninja of the Deep Hours" {
			ninjaPerm = p
		}
	}
	if ninjaPerm == nil {
		t.Fatal("ninja should be on the battlefield after ninjutsu")
	}
	if !permFlag(ninjaPerm, flagAttacking) {
		t.Fatal("ninja should be attacking")
	}
	if permFlag(ninjaPerm, flagDeclaredAttacker) {
		t.Fatal("ninja must NOT be a declared attacker (CR §702.49b)")
	}

	// (b) The ninja must NOT have fired any "attack" trigger.
	if n := countTriggerFiresR63(gs, "Ninja of the Deep Hours", "attack"); n != 0 {
		t.Fatalf("ninjutsu'd creature fired %d attack trigger(s); must be 0 (CR §509.1/§702.49b)", n)
	}

	// (c) When the ninja connects, its combat-damage trigger fires normally.
	defLifeBefore := gs.Seats[1].Life
	DealCombatDamageStep(gs, attackers, blockerMap, false)

	if n := countTriggerFiresR63(gs, "Ninja of the Deep Hours", "deals_combat_damage"); n < 1 {
		t.Fatalf("ninja's combat-damage trigger must fire on connect; got %d", n)
	}
	if gs.Seats[1].Life >= defLifeBefore {
		t.Fatalf("ninja should have dealt combat damage to the defender; life %d -> %d", defLifeBefore, gs.Seats[1].Life)
	}
	// Still no attack trigger after damage.
	if n := countTriggerFiresR63(gs, "Ninja of the Deep Hours", "attack"); n != 0 {
		t.Fatalf("attack trigger fired during/after combat damage; must be 0, got %d", n)
	}
}
