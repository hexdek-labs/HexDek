package gameengine

import "testing"

// bushido_behavior_r63_test.go — behavioral regressions for Bushido (CR
// §702.45). Bushido triggers "whenever this creature blocks or becomes
// blocked" — ONCE per such event, even when multiple creatures are involved.
// FireBushidoTriggers double-applied to a blocker that blocks multiple
// attackers (once per attacker). These pin the once-per-event semantics plus
// the attacker-side single fire and the EOT cleanup.

func bushidoCreature(gs *GameState, seat int, name string, n int) *Permanent {
	p := addP1P2Battlefield(gs, seat, name, 2, 2, "creature")
	p.Flags["kw:bushido"] = 1
	p.Flags["bushido_n"] = n
	return p
}

func bushidoMods(p *Permanent) int {
	c := 0
	for _, m := range p.Modifications {
		if m.Duration == "until_end_of_turn" && m.Power > 0 {
			c++
		}
	}
	return c
}

// (c) An attacker blocked by MULTIPLE creatures becomes blocked ONCE → bushido
// fires once, not per blocker.
func TestBushido_BecomesBlockedFiresOnce(t *testing.T) {
	gs := newP1P2Game(t)
	atk := bushidoCreature(gs, 0, "Bushido Attacker", 1)
	b1 := addP1P2Battlefield(gs, 1, "Blocker1", 1, 1, "creature")
	b2 := addP1P2Battlefield(gs, 1, "Blocker2", 1, 1, "creature")
	b3 := addP1P2Battlefield(gs, 1, "Blocker3", 1, 1, "creature")

	blockerMap := map[*Permanent][]*Permanent{atk: {b1, b2, b3}}
	FireBushidoTriggers(gs, []*Permanent{atk}, blockerMap)

	if got := bushidoMods(atk); got != 1 {
		t.Errorf("(c) becoming blocked by 3 creatures should fire bushido ONCE, got %d", got)
	}
	if atk.Power() != 3 { // base 2 + 1
		t.Errorf("(c) attacker should be 3/3 (one +1/+1), got %d/%d", atk.Power(), atk.Toughness())
	}
}

// (c)+(d) A blocker that blocks MULTIPLE attackers "blocks" once → bushido fires
// once, not per attacker blocked.
func TestBushido_BlockingMultipleFiresOnce(t *testing.T) {
	gs := newP1P2Game(t)
	a1 := addP1P2Battlefield(gs, 0, "Attacker1", 2, 2, "creature")
	a2 := addP1P2Battlefield(gs, 0, "Attacker2", 2, 2, "creature")
	blk := bushidoCreature(gs, 1, "Multi-Blocker", 2) // blocks both a1 and a2

	blockerMap := map[*Permanent][]*Permanent{a1: {blk}, a2: {blk}}
	FireBushidoTriggers(gs, []*Permanent{a1, a2}, blockerMap)

	if got := bushidoMods(blk); got != 1 {
		t.Errorf("(d) a blocker blocking 2 attackers must fire bushido ONCE, got %d", got)
	}
	if blk.Power() != 4 { // base 2 + 2
		t.Errorf("(d) blocker should be 4/4 (one +2/+2), got %d/%d", blk.Power(), blk.Toughness())
	}
}

// (a) Bushido fires on the BLOCKER side too (a blocker blocking one attacker).
func TestBushido_BlockerSideFires(t *testing.T) {
	gs := newP1P2Game(t)
	atk := addP1P2Battlefield(gs, 0, "Plain Attacker", 2, 2, "creature")
	blk := bushidoCreature(gs, 1, "Bushido Blocker", 1)

	FireBushidoTriggers(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {blk}})

	if blk.Power() != 3 {
		t.Errorf("(a) a bushido blocker should get +1/+1 when it blocks, got %d/%d", blk.Power(), blk.Toughness())
	}
}

// (e) The bonus stacks across separate combats (each combat fires once).
func TestBushido_StacksAcrossCombats(t *testing.T) {
	gs := newP1P2Game(t)
	atk := bushidoCreature(gs, 0, "Repeat Attacker", 1)
	blk := addP1P2Battlefield(gs, 1, "Wall", 0, 4, "creature")

	FireBushidoTriggers(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {blk}})
	FireBushidoTriggers(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {blk}})

	if got := bushidoMods(atk); got != 2 {
		t.Errorf("(e) two separate block events should stack two bushido buffs, got %d", got)
	}
}

// (b)+(f) The +N/+N is until-end-of-turn and clears at the cleanup step.
func TestBushido_ClearsAtEndOfTurn(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	atk := bushidoCreature(gs, 0, "Bushido One", 2)
	blk := addP1P2Battlefield(gs, 1, "Blocker", 1, 1, "creature")
	FireBushidoTriggers(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {blk}})
	if atk.Power() != 4 {
		t.Fatalf("setup: expected 4/4, got %d/%d", atk.Power(), atk.Toughness())
	}

	ScanExpiredDurations(gs, "ending", "cleanup")

	if atk.Power() != 2 {
		t.Errorf("(f) bushido +N/+N must clear at end of turn; got %d/%d", atk.Power(), atk.Toughness())
	}
}
