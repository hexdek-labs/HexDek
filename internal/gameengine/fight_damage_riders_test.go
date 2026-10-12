package gameengine

import "testing"

// fight_damage_riders_test.go — fight damage must key its lifelink and
// deathtouch riders off the damage ACTUALLY dealt, not the attacker's
// printed power (filed 2026-09-17, mechanic-probe (fight), rows 2 and 3).
//
// Combat already does this: it computes the post-prevention amount and
// gates deathtouch on `amount > 0`. dealFightDamage had no way to know
// what landed, so it used `pow` for both — over-gaining life when fight
// damage was prevented, under-gaining under a damage doubler, and
// marking a creature deathtouched that was never damaged at all.

// TestFightLifelink_GainsActualDamageNotPower — the baseline: with no
// prevention in play the two agree, so this pins that the refactor did
// not change ordinary behaviour.
func TestFightLifelink_GainsActualDamageNotPower(t *testing.T) {
	gs := fightGame()
	src := fightCreature(gs, 0, "Lifelinker", 3, 3, "lifelink")
	tgt := fightCreature(gs, 1, "Bear", 2, 4)

	before := gs.Seats[0].Life
	dealFightDamage(gs, src, tgt, 3)
	if got := gs.Seats[0].Life - before; got != 3 {
		t.Fatalf("life gained = %d, want 3", got)
	}
}

// TestFightLifelink_PreventedDamageGainsNothing is the bug: damage fully
// prevented must gain zero life, not the attacker's power.
func TestFightLifelink_PreventedDamageGainsNothing(t *testing.T) {
	gs := fightGame()
	src := fightCreature(gs, 0, "Lifelinker", 3, 3, "lifelink")
	tgt := fightCreature(gs, 1, "Warded Bear", 2, 4)
	AddPreventionShield(gs, PreventionShield{TargetSeat: -1, TargetPerm: tgt, Amount: -1, SourceCard: "Test Shield"})

	before := gs.Seats[0].Life
	dealFightDamage(gs, src, tgt, 3)
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("life gained = %d on fully prevented fight damage, want 0", got)
	}
}

// TestFightDeathtouch_NotMarkedWhenDamagePrevented is the other half: a
// creature that took no damage must not be flagged deathtouched. Left
// set, the §704.5h SBA can kill a creature that was never damaged once
// it has marked damage from any other source.
func TestFightDeathtouch_NotMarkedWhenDamagePrevented(t *testing.T) {
	gs := fightGame()
	src := fightCreature(gs, 0, "Deathtoucher", 1, 1, "deathtouch")
	tgt := fightCreature(gs, 1, "Warded Bear", 2, 4)
	AddPreventionShield(gs, PreventionShield{TargetSeat: -1, TargetPerm: tgt, Amount: -1, SourceCard: "Test Shield"})

	dealFightDamage(gs, src, tgt, 1)
	if tgt.Flags["deathtouch_damaged"] == 1 {
		t.Fatal("target flagged deathtouch_damaged despite taking no damage")
	}
}

// TestFightDeathtouch_MarkedWhenDamageLands — the guard must not break
// the ordinary case.
func TestFightDeathtouch_MarkedWhenDamageLands(t *testing.T) {
	gs := fightGame()
	src := fightCreature(gs, 0, "Deathtoucher", 1, 1, "deathtouch")
	tgt := fightCreature(gs, 1, "Bear", 2, 4)

	dealFightDamage(gs, src, tgt, 1)
	if tgt.Flags["deathtouch_damaged"] != 1 {
		t.Fatal("target not flagged deathtouch_damaged after real fight damage")
	}
}

// TestApplyDamage_ReturnsActualAmount pins the new contract directly:
// the dealt amount, and 0 when prevented.
func TestApplyDamage_ReturnsActualAmount(t *testing.T) {
	gs := fightGame()
	src := fightCreature(gs, 0, "Bolt Source", 1, 1)
	tgt := fightCreature(gs, 1, "Bear", 2, 4)

	if got := applyDamage(gs, src, Target{Kind: TargetKindPermanent, Permanent: tgt}, 2); got != 2 {
		t.Errorf("applyDamage returned %d, want 2", got)
	}

	warded := fightCreature(gs, 1, "Warded Bear", 2, 4)
	AddPreventionShield(gs, PreventionShield{TargetSeat: -1, TargetPerm: warded, Amount: -1, SourceCard: "Test Shield"})
	if got := applyDamage(gs, src, Target{Kind: TargetKindPermanent, Permanent: warded}, 2); got != 0 {
		t.Errorf("applyDamage returned %d on prevented damage, want 0", got)
	}
}

// TestApplyDamage_ReturnsActualAmountToPlayer — same contract on the
// player branch.
func TestApplyDamage_ReturnsActualAmountToPlayer(t *testing.T) {
	gs := fightGame()
	src := fightCreature(gs, 0, "Bolt Source", 1, 1)
	if got := applyDamage(gs, src, Target{Kind: TargetKindSeat, Seat: 1}, 3); got != 3 {
		t.Errorf("applyDamage to player returned %d, want 3", got)
	}
	if gs.Seats[1].Life != 37 {
		t.Errorf("life = %d, want 37", gs.Seats[1].Life)
	}
}
