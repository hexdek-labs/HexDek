package gameengine

import "testing"

// Regressions for CR §510.1c combat-damage assignment. The pre-r63 loop capped
// each blocker at its lethal amount and DROPPED any non-trample excess —
// under-counting lifelink / "damage dealt" / wither. A non-trample attacker
// must assign ALL its combat damage to the blockers (overkill); lethal is the
// per-blocker MINIMUM before moving to the next, not a cap.

func dmgOrderAtk(gs *GameState, seat int, name string, power, tough int, kws ...string) *Permanent {
	p := addP1P2Battlefield(gs, seat, name, power, tough, "creature")
	for _, kw := range kws {
		p.Flags["kw:"+kw] = 1
	}
	return p
}

// (b)/(d) No trample, multiple blockers: all damage assigned (lifelink sees the
// full 10, not just the 4 lethal across the two blockers).
func TestDamageOrder_NoTrample_MultiBlocker_AssignsAllDamage(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	gs.Seats[0].Life = 20
	atk := dmgOrderAtk(gs, 0, "Big Lifelinker", 10, 10, "lifelink")
	b1 := addP1P2Battlefield(gs, 1, "Bear A", 2, 2, "creature")
	b2 := addP1P2Battlefield(gs, 1, "Bear B", 2, 2, "creature")

	DealCombatDamageStep(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {b1, b2}}, false)

	if gain := gs.Seats[0].Life - 20; gain != 10 {
		t.Fatalf("lifelink must see all 10 damage assigned to blockers, gained %d", gain)
	}
}

// (b) No trample, single blocker: overkill — all power assigned to the one blocker.
func TestDamageOrder_NoTrample_SingleBlocker_Overkill(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	gs.Seats[0].Life = 20
	atk := dmgOrderAtk(gs, 0, "Lifelinker", 5, 5, "lifelink")
	blk := addP1P2Battlefield(gs, 1, "Bear", 2, 2, "creature")

	DealCombatDamageStep(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {blk}}, false)

	if gain := gs.Seats[0].Life - 20; gain != 5 {
		t.Fatalf("lifelink on a 5-power no-trample attacker vs one blocker must see all 5, gained %d", gain)
	}
	if blk.MarkedDamage != 5 {
		t.Fatalf("blocker should be assigned all 5 (overkill), marked %d", blk.MarkedDamage)
	}
}

// (c) Trample: lethal to blocker, remainder to the defending player.
func TestDamageOrder_Trample_SpillsRemainderToPlayer(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	gs.Seats[1].Life = 20
	atk := dmgOrderAtk(gs, 0, "Trampler", 5, 5, "trample")
	SetAttackerDefender(atk, 1)
	blk := addP1P2Battlefield(gs, 1, "Bear", 2, 2, "creature")

	DealCombatDamageStep(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {blk}}, false)

	if blk.MarkedDamage != 2 {
		t.Fatalf("trample should assign exactly lethal (2) to the blocker, marked %d", blk.MarkedDamage)
	}
	if loss := 20 - gs.Seats[1].Life; loss != 3 {
		t.Fatalf("trample remainder to player should be 3 (5-2), player lost %d", loss)
	}
}

// (e) Deathtouch frees trample: lethal-for-assignment is 1, so more spills over.
func TestDamageOrder_Deathtouch_FreesTrample(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	gs.Seats[1].Life = 20
	atk := dmgOrderAtk(gs, 0, "Deathtrampler", 5, 5, "trample", "deathtouch")
	SetAttackerDefender(atk, 1)
	blk := addP1P2Battlefield(gs, 1, "Wall", 4, 4, "creature")

	DealCombatDamageStep(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {blk}}, false)

	if blk.MarkedDamage != 1 {
		t.Fatalf("deathtouch makes 1 lethal-for-assignment, blocker should take 1, marked %d", blk.MarkedDamage)
	}
	if loss := 20 - gs.Seats[1].Life; loss != 4 {
		t.Fatalf("deathtouch trample should spill 4 to player (5-1), player lost %d", loss)
	}
}

// (b)/(c) Trample, multiple blockers: lethal to each in order, then remainder to player.
func TestDamageOrder_Trample_MultiBlocker_LethalEachThenPlayer(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	gs.Seats[1].Life = 20
	atk := dmgOrderAtk(gs, 0, "Big Trampler", 7, 7, "trample")
	SetAttackerDefender(atk, 1)
	b1 := addP1P2Battlefield(gs, 1, "Bear", 2, 2, "creature")
	b2 := addP1P2Battlefield(gs, 1, "Ogre", 3, 3, "creature")

	DealCombatDamageStep(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {b1, b2}}, false)

	// Lethal 2 + 3 = 5 to blockers, remainder 2 to player.
	if loss := 20 - gs.Seats[1].Life; loss != 2 {
		t.Fatalf("trample over two blockers (lethal 2+3) should spill 2 to player, player lost %d", loss)
	}
}

// (d) Blocked, no trample, all blockers gone: no damage assigned anywhere.
func TestDamageOrder_NoTrample_AllBlockersGone_NoDamage(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	atk := dmgOrderAtk(gs, 0, "Lifelinker", 5, 5, "lifelink")
	SetAttackerDefender(atk, 1)
	blk := addP1P2Battlefield(gs, 1, "Bear", 2, 2, "creature")
	// Blocker declared then removed before damage.
	blk.Flags["dead"] = 0
	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[:0] // remove the blocker

	DealCombatDamageStep(gs, []*Permanent{atk}, map[*Permanent][]*Permanent{atk: {blk}}, false)

	if gs.Seats[0].Life != 20 {
		t.Fatalf("non-trample attacker whose blocker is gone deals no damage (no lifelink), life %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 20 {
		t.Fatalf("non-trample attacker whose blocker is gone deals no damage to player, life %d", gs.Seats[1].Life)
	}
}
