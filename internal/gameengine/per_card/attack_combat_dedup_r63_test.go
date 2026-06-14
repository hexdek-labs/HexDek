package per_card

// r63 — once-per-combat dedup for broad "whenever you attack" handlers.
//
// DISTINCT class from the attack_self_gate_r63 sweep: these 5 handlers are NOT
// self-triggers (they fire on the controller declaring ANY attacker, not on
// THIS creature attacking), so a self-gate is the wrong fix. The engine fires
// the attack event ONCE PER DECLARED ATTACKER, so a "whenever you attack"
// handler with no gate resolves N times on an N-attacker combat instead of once.
//
// Each test invokes the handler TWICE (simulating two declared attackers in one
// combat) and asserts the effect resolved EXACTLY ONCE — the once-per-turn
// perm.Flags gate (Caesar/Raffine pattern) suppressing the second invocation.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func countRobotTokens(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && cardHasType(p.Card, "robot") {
			n++
		}
	}
	return n
}

// Toph: "Whenever you attack, earthbend X." Two attackers must earthbend once.
func TestAttackDedup_Toph_EarthbendOncePerCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 3
	toph := stampCreaturePT(addPerm(gs, 0, "Toph, Earthbending Master", "creature", "legendary"), 2, 2)
	gs.Seats[0].Flags = map[string]int{"experience": 2}
	land := addPerm(gs, 0, "Forest", "land")
	ctx := map[string]interface{}{"seat": 0}

	tophEarthbend(gs, toph, ctx) // attacker #1
	tophEarthbend(gs, toph, ctx) // attacker #2 — must be gated

	if got := land.Counters["+1/+1"]; got != 2 {
		t.Fatalf("Toph earthbend over-fired: land has %d +1/+1 counters, want 2 (X=experience, once per combat)", got)
	}
}

// Temmet: "Whenever you attack, draw a card, then discard a card." Once.
func TestAttackDedup_Temmet_LootOncePerCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 3
	temmet := stampCreaturePT(addPerm(gs, 0, "Temmet, Naktamun's Will", "creature", "legendary"), 2, 2)
	addLibrary(gs, 0, "Draw A", "Draw B", "Draw C")
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Hand A", Owner: 0, Types: []string{"cost:1"}},
		{Name: "Hand B", Owner: 0, Types: []string{"cost:5"}},
		{Name: "Hand C", Owner: 0, Types: []string{"cost:3"}},
	}

	temmetOnAttack(gs, temmet, nil) // attacker #1
	temmetOnAttack(gs, temmet, nil) // attacker #2 — must be gated

	if got := len(gs.Seats[0].Graveyard); got != 1 {
		t.Fatalf("Temmet loot over-fired: %d cards discarded to graveyard, want 1 (one loot per combat)", got)
	}
}

// Squall: "Whenever one or more creatures attack one of your opponents, ...
// Squall deals damage equal to its power to defending player." Once per
// (combat, defender).
func TestAttackDedup_Squall_PingOncePerCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 3
	squall := stampCreaturePT(addPerm(gs, 0, "Squall, Gunblade Duelist", "creature", "legendary"), 2, 2)
	squall.Flags["squall_chosen_number"] = 3
	// Two attacking 3/3s (match chosen number 3) declared at seat 1.
	a1 := stampCreaturePT(addPerm(gs, 0, "Attacker One", "creature"), 3, 3)
	a2 := stampCreaturePT(addPerm(gs, 0, "Attacker Two", "creature"), 3, 3)
	gameengine.MarkEnteredAttacking(a1)
	gameengine.MarkEnteredAttacking(a2)
	gs.Seats[1].Life = 40
	ctx := map[string]interface{}{"defender_seat": 1}

	squallGunbladeAttackers(gs, squall, ctx) // attacker #1
	squallGunbladeAttackers(gs, squall, ctx) // attacker #2 — must be gated

	if got := gs.Seats[1].Life; got != 38 {
		t.Fatalf("Squall ping over-fired: defender at %d life, want 38 (one ping of power 2 per combat)", got)
	}
}

// ED-E: "Whenever you attack, if attackers > quest counters, put a quest
// counter on it." At most one quest counter per combat.
func TestAttackDedup_EdE_QuestCounterOncePerCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 3
	ede := stampCreaturePT(addPerm(gs, 0, "ED-E, Lonesome Eyebot", "artifact", "creature", "legendary"), 1, 1)
	// Two attacking creatures so the count (2) exceeds the starting quest (0).
	a1 := stampCreaturePT(addPerm(gs, 0, "Attacker One", "creature"), 2, 2)
	a2 := stampCreaturePT(addPerm(gs, 0, "Attacker Two", "creature"), 2, 2)
	gameengine.MarkEnteredAttacking(a1)
	gameengine.MarkEnteredAttacking(a2)

	edEAttackTrigger(gs, ede, nil) // attacker #1
	edEAttackTrigger(gs, ede, nil) // attacker #2 — must be gated

	if got := ede.Counters["quest"]; got != 1 {
		t.Fatalf("ED-E quest over-fired: %d quest counters, want 1 (at most one per combat)", got)
	}
}

// Dyadrine: "Whenever you attack, you may remove a +1/+1 counter from each of
// two creatures you control. If you do, draw a card and create a 2/2 Robot."
// One Robot per combat.
func TestAttackDedup_Dyadrine_RobotOncePerCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 3
	dyadrine := stampCreaturePT(addPerm(gs, 0, "Dyadrine, Synthesis Amalgam", "creature", "legendary"), 2, 2)
	// Two creatures carrying +1/+1 counters to pay the removal cost twice over.
	c1 := stampCreaturePT(addPerm(gs, 0, "Counter Bearer One", "creature"), 1, 1)
	c2 := stampCreaturePT(addPerm(gs, 0, "Counter Bearer Two", "creature"), 1, 1)
	c1.AddCounter("+1/+1", 3)
	c2.AddCounter("+1/+1", 3)
	addLibrary(gs, 0, "Draw A", "Draw B", "Draw C")
	ctx := map[string]interface{}{"seat": 0}

	dyadrineAttack(gs, dyadrine, ctx) // attacker #1
	dyadrineAttack(gs, dyadrine, ctx) // attacker #2 — must be gated

	if got := countRobotTokens(gs, 0); got != 1 {
		t.Fatalf("Dyadrine over-fired: %d Robot tokens created, want 1 (one per combat)", got)
	}
	// Exactly two +1/+1 counters removed total (one from each bearer), not four.
	if removed := (3 - c1.Counters["+1/+1"]) + (3 - c2.Counters["+1/+1"]); removed != 2 {
		t.Fatalf("Dyadrine removed %d +1/+1 counters, want 2 (one per bearer, once per combat)", removed)
	}
}
