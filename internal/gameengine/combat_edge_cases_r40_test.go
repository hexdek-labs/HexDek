package gameengine

// R40 — combat damage edge-case purification suite.
//
// Documents and pins the engine's current behavior across seven combat
// interactions that the R37 audit flagged as "broadly correct but worth
// nailing down with tests." Each test exercises one well-known
// MTG interaction and asserts the canonical CR outcome.
//
//   (a) Multi-blocker damage assignment order (§510.1c)
//   (b) First-strike + deathtouch killing before regular damage step
//   (c) Trample with deathtouch — only 1 damage per blocker required,
//       rest tramples (§702.19c + §702.2c)
//   (d) Double-strike vs first-strike — DS attacker survives because
//       the FS blocker only fires once
//   (e) Lifelink with trample — lifelink gains life from every
//       instance of damage including the trample carry-over
//   (f) Menace blocker requirement — single-blocker can't satisfy menace,
//       attacker connects (§702.110b)
//   (g) Protection from color — blocking is legal but damage to the
//       protected blocker is prevented (§702.16 DEBT)

import (
	"math/rand"
	"testing"
)

// -----------------------------------------------------------------------------
// (a) Multi-blocker damage assignment order — §510.1c
// -----------------------------------------------------------------------------

func TestCombat_R40_MultiBlockerOrder_LethalFirstThenSecond(t *testing.T) {
	// 5-power vanilla attacker into two blockers (2/2 and 3/3). Per
	// §510.1c the attacker assigns at least lethal damage to the first
	// blocker in the order before any damage goes to the second. The
	// engine's current heuristic orders blockers by ascending leftover
	// toughness, so the 2/2 receives 2 and the 3/3 receives 3.
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Active = 0

	atk := addCreature(gs, 0, "Hill Giant", 5, 5)
	smallBlk := addCreature(gs, 1, "Grizzly Bears", 2, 2)
	bigBlk := addCreature(gs, 1, "Hill Giant B", 3, 3)

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{smallBlk, bigBlk}}

	CombatPhase(gs)

	if smallBlk.MarkedDamage != 2 {
		t.Errorf("first blocker (2/2): want exactly 2 marked (lethal), got %d", smallBlk.MarkedDamage)
	}
	if bigBlk.MarkedDamage != 3 {
		t.Errorf("second blocker (3/3): want exactly 3 marked (lethal after first), got %d",
			bigBlk.MarkedDamage)
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("no trample on attacker — defender life should be untouched, got %d", gs.Seats[1].Life)
	}
	// Attacker took 2+3=5 from blockers vs 5 toughness — will die at SBA.
	if atk.MarkedDamage != 5 {
		t.Errorf("attacker should take 2+3=5 marked from both blockers, got %d", atk.MarkedDamage)
	}
}

// -----------------------------------------------------------------------------
// (b) First-strike + deathtouch killing before regular damage step
// -----------------------------------------------------------------------------

func TestCombat_R40_FirstStrikeDeathtouch_KillsBeforeStrikeBack(t *testing.T) {
	// 1/1 first-strike deathtouch attacker into a 4/4 vanilla blocker.
	// During the first-strike step the attacker deals 1 damage which
	// deathtouch bumps to lethal; the SBA between damage steps removes
	// the blocker; in the regular step the blocker is gone, so no
	// strike-back damage reaches the attacker.
	gs := newCombatGame(t)

	atk := addCreature(gs, 0, "Royal Assassin", 1, 1, "first strike", "deathtouch")
	blk := addCreature(gs, 1, "Serra Angel", 4, 4)

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{blk}}

	CombatPhase(gs)
	StateBasedActions(gs)

	if alive(gs, blk) {
		t.Errorf("blocker should die in first-strike step from deathtouch + SBA between steps")
	}
	if !alive(gs, atk) {
		t.Errorf("attacker should survive — blocker died before regular damage step")
	}
	if atk.MarkedDamage != 0 {
		t.Errorf("attacker should take 0 damage (blocker died before striking back); got marked=%d",
			atk.MarkedDamage)
	}
}

// -----------------------------------------------------------------------------
// (c) Trample + deathtouch — 1 dmg per blocker is lethal, rest tramples
// -----------------------------------------------------------------------------

func TestCombat_R40_TrampleDeathtouch_OnePerBlockerThenCarry(t *testing.T) {
	// 5-power deathtouch+trample into three 3/3 blockers. Per §702.2c
	// the attacker only needs to assign 1 damage to each blocker (the
	// deathtouch lethal amount). The remaining 2 damage tramples to
	// the defending player.
	gs := newCombatGame(t)

	_ = addCreature(gs, 0, "Lotleth Troll", 5, 5, "trample", "deathtouch")
	b1 := addCreature(gs, 1, "Centaur A", 3, 3)
	b2 := addCreature(gs, 1, "Centaur B", 3, 3)
	b3 := addCreature(gs, 1, "Centaur C", 3, 3)

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{b1, b2, b3}}

	CombatPhase(gs)
	StateBasedActions(gs)

	if gs.Seats[1].Life != 18 {
		t.Errorf("trample carry-over from deathtouch = 5 - 3*1 = 2; want defender life 18, got %d",
			gs.Seats[1].Life)
	}
	for i, b := range []*Permanent{b1, b2, b3} {
		if alive(gs, b) {
			t.Errorf("blocker[%d] should die from deathtouch lethal damage", i)
		}
	}
}

// -----------------------------------------------------------------------------
// (d) Double-strike vs first-strike — DS attacker survives a one-strike FS
// -----------------------------------------------------------------------------

func TestCombat_R40_DoubleStrike_BeatsFirstStrikeBlocker(t *testing.T) {
	// 3/3 double-strike attacker into a 2/4 first-strike blocker.
	//   - First-strike step: both fire. Atk takes 2 (3 toughness, survives).
	//     Blocker takes 3 (4 toughness, survives).
	//   - SBA between steps: nobody dies.
	//   - Regular step: only DS attacker fires (plain FS doesn't fire here).
	//     Blocker takes 3 more — total 6 marked vs toughness 4 — dies on SBA.
	//   - Result: attacker alive at 2 marked, blocker in graveyard.
	gs := newCombatGame(t)

	atk := addCreature(gs, 0, "Mirran Crusader", 3, 3, "double strike")
	blk := addCreature(gs, 1, "Silvercoat Knight", 2, 4, "first strike")

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{blk}}

	CombatPhase(gs)
	StateBasedActions(gs)

	if !alive(gs, atk) {
		t.Errorf("DS attacker should survive — FS blocker only fires once for 2 dmg into 3 toughness")
	}
	if atk.MarkedDamage != 2 {
		t.Errorf("attacker should be marked 2 from blocker's one FS strike; got %d", atk.MarkedDamage)
	}
	if alive(gs, blk) {
		t.Errorf("FS blocker should die — DS attacker hit it twice (3+3=6) vs 4 toughness")
	}
}

// -----------------------------------------------------------------------------
// (e) Lifelink with trample — gain equals total damage including trample
// -----------------------------------------------------------------------------

func TestCombat_R40_LifelinkTrample_GainsAllDamageIncludingCarry(t *testing.T) {
	// 5/5 lifelink+trample attacker into a 2/2 blocker.
	//   - Phase A: 2 to blocker (kills), 3 trample to defender.
	//   - Lifelink gains 2 + 3 = 5.
	//   - Phase B: blocker (alive during this step, SBA runs after) deals
	//     2 back to attacker — marked 2 vs toughness 5, attacker survives.
	gs := newCombatGame(t)

	atk := addCreature(gs, 0, "Baneslayer Angel", 5, 5, "lifelink", "trample")
	blk := addCreature(gs, 1, "Walking Corpse", 2, 2)

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{blk}}

	startLifeAttacker := gs.Seats[0].Life
	startLifeDefender := gs.Seats[1].Life

	CombatPhase(gs)
	StateBasedActions(gs)

	if gs.Seats[0].Life != startLifeAttacker+5 {
		t.Errorf("lifelink should gain 2+3=5 (kill + carry); want %d, got %d",
			startLifeAttacker+5, gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != startLifeDefender-3 {
		t.Errorf("trample carry = 5-2 = 3; want defender life %d, got %d",
			startLifeDefender-3, gs.Seats[1].Life)
	}
	if alive(gs, blk) {
		t.Errorf("blocker should die from 2 damage = its toughness")
	}
	if !alive(gs, atk) {
		t.Errorf("attacker took 2 marked vs 5 toughness; should survive")
	}
}

// -----------------------------------------------------------------------------
// (f) Menace blocker requirement — §702.110b
// -----------------------------------------------------------------------------

func TestCombat_R40_Menace_SoloBlockerFailsRequirement(t *testing.T) {
	// 2/2 menace attacker into a defender that controls exactly one
	// untapped creature. Per §702.110b the attacker can't be blocked
	// except by two or more creatures. With the no-Hat fallback in
	// DeclareBlockers, the single candidate doesn't satisfy the menace
	// gate, the block is skipped, and the attacker connects with the
	// defending player.
	gs := newCombatGame(t)
	// Deliberately NO Hat on seat 1 — exercise the fallback path so
	// menace gating in DeclareBlockers actually fires.

	addCreature(gs, 0, "Boggart Brute", 2, 2, "menace")
	addCreature(gs, 1, "Lone Soldier", 5, 5)

	CombatPhase(gs)

	if gs.Seats[1].Life != 18 {
		t.Errorf("solo blocker can't satisfy menace — attacker hits player for 2; want life 18, got %d",
			gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// (g) Protection from color — block legal, damage prevented
// -----------------------------------------------------------------------------

func TestCombat_R40_ProtectionFromColor_BlocksLegalDamagePrevented(t *testing.T) {
	// CR §702.16 DEBT: "can't be blocked by" only applies on the
	// attacking side. A blocker with protection from red CAN legally
	// block a red attacker; the damage from the red source is just
	// prevented. The blocker then deals damage back normally.
	gs := newCombatGame(t)

	atk := addColoredCreature(gs, 0, "Shivan Dragon", 4, 4, "red")
	blk := addCreature(gs, 1, "Mother of Runes", 2, 2)
	blk.Flags["prot:R"] = 1
	// No Hat — fallback assigns the legal blocker.

	CombatPhase(gs)

	if blk.MarkedDamage != 0 {
		t.Errorf("prot-from-red blocker should take 0 damage from red attacker; got marked=%d",
			blk.MarkedDamage)
	}
	if atk.MarkedDamage != 2 {
		t.Errorf("blocker's 2 damage to attacker should go through unaffected; got marked=%d",
			atk.MarkedDamage)
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("no trample on attacker, attacker was blocked — defender life should be 20, got %d",
			gs.Seats[1].Life)
	}
}
