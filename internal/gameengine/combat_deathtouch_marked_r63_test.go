package gameengine

import "testing"

// CR §702.2b / §704.5h — deathtouch marks the ACTUAL damage dealt and the
// creature is destroyed by the §704.5h SBA via the deathtouch flag, NOT by
// marked damage reaching toughness. Regression for the r63 combat-keyword
// probe: applyCombatDamageToCreature bumped MarkedDamage up to the target's
// toughness for deathtouch damage, which (a) is wrong — the creature only
// took `amount` damage — and (b) corrupts a creature that SURVIVES the
// deathtouch (indestructible): it then carried lethal marked damage and
// would be wrongly destroyed by §704.5g the moment it lost indestructibility.

// Indestructible blocker takes deathtouch damage: survives, and carries only
// the ACTUAL damage (1), not a bumped-to-toughness value.
func TestCombat_DeathtouchIndestructible_MarksActualDamage(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Deathtouch Rat", 1, 1, "deathtouch")
	blk := addCreature(gs, 1, "Darksteel Wall", 5, 5, "indestructible")
	atk.Flags[flagAttacking] = 1
	atk.Flags[flagDeclaredAttacker] = 1
	blockerMap := map[*Permanent][]*Permanent{atk: {blk}}

	DealCombatDamageStep(gs, []*Permanent{atk}, blockerMap, false)
	StateBasedActions(gs)

	if !alive(gs, blk) {
		t.Fatal("indestructible creature must survive deathtouch (§704.5h skips indestructible)")
	}
	if blk.MarkedDamage != 1 {
		t.Errorf("indestructible survivor must carry only the actual 1 damage, not a bumped value; got %d",
			blk.MarkedDamage)
	}
}

// The decisive consequence: a creature that survived deathtouch via
// indestructibility, then LOSES indestructibility the same turn, must NOT
// die — it only took 1 actual damage (< toughness). Pre-fix the bumped
// marked damage (== toughness) destroyed it under §704.5g.
func TestCombat_DeathtouchSurvivor_LosesIndestructible_StillLives(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Deathtouch Rat", 1, 1, "deathtouch")
	blk := addCreature(gs, 1, "Darksteel Wall", 5, 5, "indestructible")
	atk.Flags[flagAttacking] = 1
	atk.Flags[flagDeclaredAttacker] = 1
	blockerMap := map[*Permanent][]*Permanent{atk: {blk}}

	DealCombatDamageStep(gs, []*Permanent{atk}, blockerMap, false)
	StateBasedActions(gs)
	if !alive(gs, blk) {
		t.Fatal("precondition: indestructible blocker should have survived")
	}

	// Indestructibility wears off later this turn.
	delete(blk.Flags, "kw:indestructible")
	gs.InvalidateCharacteristicsCache()
	StateBasedActions(gs)

	if !alive(gs, blk) {
		t.Error("a 5/5 with only 1 actual damage must survive after losing indestructibility " +
			"(deathtouch is a one-time SBA, not lethal marked damage)")
	}
}

// Sanity: a NON-indestructible creature still dies to deathtouch via the
// flag, even though only 1 damage is marked.
func TestCombat_DeathtouchNormal_DiesViaFlag(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Deathtouch Rat", 1, 1, "deathtouch")
	blk := addCreature(gs, 1, "Giant", 5, 5)
	atk.Flags[flagAttacking] = 1
	atk.Flags[flagDeclaredAttacker] = 1
	blockerMap := map[*Permanent][]*Permanent{atk: {blk}}

	DealCombatDamageStep(gs, []*Permanent{atk}, blockerMap, false)
	if blk.MarkedDamage != 1 {
		t.Errorf("only 1 damage should be marked, got %d", blk.MarkedDamage)
	}
	StateBasedActions(gs)
	if alive(gs, blk) {
		t.Error("deathtouch must still kill the 5/5 via the §704.5h flag")
	}
}
