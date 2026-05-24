package gameengine

// R41 — combat damage edge-case purification suite.
//
// Sibling/deeper variants to the R40 edge-case suite (combat_edge_cases_r40_test.go).
// Where R40 pins the canonical case for each interaction, R41 pins a second
// angle so a regression in either limb is caught:
//
//   (a) Multi-blocker order §510.1c — sub-lethal total: attacker's power
//       is less than total blocker toughness, so the back blocker takes
//       only a fraction after the front blocker's lethal assignment.
//   (b) First-strike + deathtouch mirror — both sides FS+DT, mutual kill
//       in the first-strike step.
//   (c) Trample with deathtouch carry-over — single big blocker: §702.19c
//       lets the attacker assign 1 (deathtouch lethal) and trample the rest.
//   (d) Double-strike with lifelink — life gained from BOTH strike steps,
//       not just the regular damage step.
//   (e) Lifelink with trample lethal — defender goes below 0; gain equals
//       damage dealt, not damage required to kill.
//   (f) Menace satisfied — exactly two blockers meet §702.110b and the
//       attacker is contained (does not connect with the player).
//   (g) Protection from color on the attacker side — a red blocker can't
//       block an attacker with prot-from-red; fallback skips, attacker
//       connects with the defending player.

import (
	"math/rand"
	"testing"
)

// -----------------------------------------------------------------------------
// (a) Multi-blocker order §510.1c — sub-lethal total carry-through
// -----------------------------------------------------------------------------

func TestCombat_R41_MultiBlockerOrder_SubLethalSpilloverToSecond(t *testing.T) {
	// 3-power vanilla attacker into a 2/2 then a 4/4. Per §510.1c the
	// attacker must assign at least lethal to the first blocker before
	// assigning any to the second. Order-by-ascending-toughness puts the
	// 2/2 first → 2 lethal to it, 1 leftover to the 4/4 (non-lethal).
	// Attacker takes 2+4 = 6 back, far more than its 3 toughness.
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Active = 0

	atk := addCreature(gs, 0, "Hill Giant 3p", 3, 3)
	smallBlk := addCreature(gs, 1, "Grizzly Bears", 2, 2)
	bigBlk := addCreature(gs, 1, "Serra Angel", 4, 4)

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{smallBlk, bigBlk}}

	CombatPhase(gs)
	StateBasedActions(gs)

	if smallBlk.MarkedDamage != 2 {
		t.Errorf("first blocker (2/2): want lethal 2, got %d", smallBlk.MarkedDamage)
	}
	if bigBlk.MarkedDamage != 1 {
		t.Errorf("second blocker (4/4): want leftover 1 (3 - lethal 2), got %d", bigBlk.MarkedDamage)
	}
	if alive(gs, atk) {
		t.Errorf("attacker (3 toughness) should die — took 2+4=6 from both blockers")
	}
	if !alive(gs, bigBlk) {
		t.Errorf("4/4 blocker took only 1 — should survive")
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("attacker has no trample and was blocked — defender life should be 20, got %d",
			gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// (b) First-strike + deathtouch mirror — mutual kill in FS step
// -----------------------------------------------------------------------------

func TestCombat_R41_FirstStrikeDeathtouchMirror_MutualKillInFSStep(t *testing.T) {
	// Both sides FS+DT, both 1/1. In the first-strike step they each
	// deal 1, which deathtouch promotes to lethal. SBA after the FS
	// step removes both — no regular damage step fires for either.
	gs := newCombatGame(t)

	atk := addCreature(gs, 0, "Tyrant's Choice", 1, 1, "first strike", "deathtouch")
	blk := addCreature(gs, 1, "Royal Assassin", 1, 1, "first strike", "deathtouch")

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{blk}}

	CombatPhase(gs)
	StateBasedActions(gs)

	if alive(gs, atk) {
		t.Errorf("FS+DT attacker should die in FS step (deathtouch lethal)")
	}
	if alive(gs, blk) {
		t.Errorf("FS+DT blocker should die in FS step (deathtouch lethal)")
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("no trample, blocked — defender life should be 20, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// (c) Trample + deathtouch — single fat blocker, 1 dmg assigned, rest trample
// -----------------------------------------------------------------------------

func TestCombat_R41_TrampleDeathtouch_SingleBigBlocker_CarryToPlayer(t *testing.T) {
	// 6/6 deathtouch+trample into a single 8/8 blocker. §702.19c +
	// §702.2c: 1 damage is "lethal" thanks to deathtouch, the other 5
	// tramples to the defending player. Blocker dies on SBA from
	// deathtouch flag even though only 1 was marked.
	gs := newCombatGame(t)

	_ = addCreature(gs, 0, "Wickerbough Elder", 6, 6, "trample", "deathtouch")
	blk := addCreature(gs, 1, "Colossal Dreadmaw", 8, 8)

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{blk}}

	CombatPhase(gs)
	StateBasedActions(gs)

	if alive(gs, blk) {
		t.Errorf("8/8 blocker took deathtouch damage; should die on SBA regardless of marked amount")
	}
	if gs.Seats[1].Life != 15 {
		t.Errorf("trample carry should be 6-1=5; want defender life 15, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// (d) Double-strike with lifelink — life gained on BOTH strikes
// -----------------------------------------------------------------------------

func TestCombat_R41_DoubleStrikeLifelink_GainsOnBothSteps(t *testing.T) {
	// 3/3 double-strike+lifelink unblocked attacker → 6 damage to
	// player across two strike steps; lifelink gains the full 6.
	gs := newCombatGame(t)

	addCreature(gs, 0, "Skullbriar-style", 3, 3, "double strike", "lifelink")

	startAttackerLife := gs.Seats[0].Life
	startDefenderLife := gs.Seats[1].Life

	CombatPhase(gs)
	StateBasedActions(gs)

	if gs.Seats[0].Life != startAttackerLife+6 {
		t.Errorf("double-strike + lifelink unblocked = +6 life; want %d, got %d",
			startAttackerLife+6, gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != startDefenderLife-6 {
		t.Errorf("double-strike unblocked = 6 dmg; want defender %d, got %d",
			startDefenderLife-6, gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// (e) Lifelink with trample — gain equals damage dealt (kill + carry)
// -----------------------------------------------------------------------------

func TestCombat_R41_LifelinkTrample_GainEqualsDamageDealt(t *testing.T) {
	// 7/7 lifelink+trample into a 2/2 blocker. 2 to blocker (kills),
	// 5 tramples to player. Lifelink gains 2+5 = 7 (the entire damage
	// dealt by the source).
	gs := newCombatGame(t)

	atk := addCreature(gs, 0, "Wurmcoil-ish", 7, 7, "lifelink", "trample")
	blk := addCreature(gs, 1, "Walking Corpse", 2, 2)

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{blk}}

	startAttackerLife := gs.Seats[0].Life
	startDefenderLife := gs.Seats[1].Life

	CombatPhase(gs)
	StateBasedActions(gs)

	if gs.Seats[0].Life != startAttackerLife+7 {
		t.Errorf("lifelink should gain 7 (full damage by source); want %d, got %d",
			startAttackerLife+7, gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != startDefenderLife-5 {
		t.Errorf("trample carry = 7-2 = 5; want defender life %d, got %d",
			startDefenderLife-5, gs.Seats[1].Life)
	}
	if alive(gs, blk) {
		t.Errorf("2/2 blocker should die from 2 marked")
	}
	if !alive(gs, atk) {
		t.Errorf("attacker (7 toughness) took 2 back — should survive")
	}
}

// -----------------------------------------------------------------------------
// (f) Menace satisfied — two blockers gate open, attacker contained
// -----------------------------------------------------------------------------

func TestCombat_R41_Menace_TwoBlockersSatisfyRequirement(t *testing.T) {
	// 4/4 menace attacker, defender has two 2/2s. §702.110b is satisfied
	// with two blockers. The attacker is blocked and does not connect.
	// The blockers absorb 2+2 = 4 damage (lethal split) and the attacker
	// takes 2+2 = 4 back, dying on SBA.
	gs := newCombatGame(t)

	atk := addCreature(gs, 0, "Boggart Brute Big", 4, 4, "menace")
	b1 := addCreature(gs, 1, "Bear A", 2, 2)
	b2 := addCreature(gs, 1, "Bear B", 2, 2)

	gs.Seats[1].Hat = &r38BlockerHat{blockers: []*Permanent{b1, b2}}

	CombatPhase(gs)
	StateBasedActions(gs)

	if gs.Seats[1].Life != 20 {
		t.Errorf("two blockers satisfy menace — attacker should NOT connect; want defender life 20, got %d",
			gs.Seats[1].Life)
	}
	if alive(gs, b1) || alive(gs, b2) {
		t.Errorf("both blockers took 2 = their toughness; both should die")
	}
	if alive(gs, atk) {
		t.Errorf("attacker (4 toughness) took 2+2=4 back — should die")
	}
}

// -----------------------------------------------------------------------------
// (g) Protection from color on the attacker — blocker can't legally block
// -----------------------------------------------------------------------------

func TestCombat_R41_ProtectionOnAttacker_BlockerSkipped_PlayerHit(t *testing.T) {
	// Per §702.16 DEBT, an attacker with protection from red CAN'T be
	// blocked by red creatures. The fallback DeclareBlockers gate skips
	// the illegal block and the attacker connects with the defending
	// player.
	gs := newCombatGame(t)

	atk := addCreature(gs, 0, "Mirran Crusader", 3, 3)
	atk.Flags["prot:R"] = 1
	blk := addColoredCreature(gs, 1, "Goblin Piker", 2, 1, "red")
	// No Hat — exercise the fallback's protection gate.

	startDefenderLife := gs.Seats[1].Life

	CombatPhase(gs)
	StateBasedActions(gs)

	if blk.MarkedDamage != 0 {
		t.Errorf("illegal block skipped — blocker should be unmarked, got %d", blk.MarkedDamage)
	}
	if atk.MarkedDamage != 0 {
		t.Errorf("blocker never blocked, never strikes — attacker unmarked, got %d", atk.MarkedDamage)
	}
	if gs.Seats[1].Life != startDefenderLife-3 {
		t.Errorf("unblockable-by-color attacker connects for 3; want defender %d, got %d",
			startDefenderLife-3, gs.Seats[1].Life)
	}
}
