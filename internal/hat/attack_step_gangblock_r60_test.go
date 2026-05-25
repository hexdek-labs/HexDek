package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// attack_step_gangblock_r60_test.go — R60 AttackStep survey.
//
// Survey finding: canSwingProfitably only considered single-blocker
// deadly responses ("blocker survives AND kills attacker"). It missed
// the gang-block case entirely — three 3/3s collectively kill a 6/6
// despite no single 3/3 being individually deadly. The hat would
// happily send a 6/6 into a board of 3/3s and lose the threat for
// (at best) 6 mutual stats.
//
// Survey finding #2: the trample short-circuit unconditionally
// returned profitable, but trample only leaks damage when
// attacker.Power > sum(blocker.Toughness). A 3-power trampler into a
// 5-toughness wall leaks 0 — the attacker just dies for nothing. The
// short-circuit hid this.
//
// Two new signals added:
//
//   1. gangBlockKillsAttacker(gs, attacker, legal): 2+ blockers whose
//      combined power ≥ attacker toughness kill the attacker via the
//      damage-assignment chain. Gated on "no single blocker can
//      deliver lethal on its own" — defenders single-block when they
//      can, so the gang signal is only relevant when ganging is the
//      ONLY path to lethal. FS/DS attackers get one blocker's worth
//      of return damage shaved off the gang's sum-power.
//
//   2. leakIsMeaningfulAgainst(gs, attacker, legal): trample leak
//      must be ≥ 2 AND ≥ half attacker.Power. A 1-damage dribble at
//      40-life opponents isn't worth losing the attacker.

// -----------------------------------------------------------------------------
// gangBlockKillsAttacker
// -----------------------------------------------------------------------------

// TestGangBlockKillsAttacker_TwoBlockersCombineForLethal pins the
// canonical case: 6/6 attacker, 3/3 + 3/3 blockers. Neither single
// blocker has bp ≥ at (3 < 6), so single-block can't deliver lethal;
// the gang's 6 combined power matches at and kills.
func TestGangBlockKillsAttacker_TwoBlockersCombineForLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Big Beast", []string{"creature"}, 6, nil), 6, 6)
	b1 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear A", []string{"creature"}, 2, nil), 3, 3)
	b2 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear B", []string{"creature"}, 2, nil), 3, 3)

	if !gangBlockKillsAttacker(gs, atk, []*gameengine.Permanent{b1, b2}) {
		t.Errorf("two 3/3s should gang-kill a 6/6 attacker (sum power 6 ≥ toughness 6)")
	}
}

// TestGangBlockKillsAttacker_NoFireWhenSingleBlockerLethal pins the
// defender-incentive gate. Three 3/3s vs a 3/3 attacker — single
// 3/3 mutually trades (delivers lethal even while dying), so defender
// single-blocks. Gang is irrelevant and the signal must not fire.
func TestGangBlockKillsAttacker_NoFireWhenSingleBlockerLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Beast", []string{"creature"}, 4, nil), 3, 3)
	b1 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear A", []string{"creature"}, 2, nil), 3, 3)
	b2 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear B", []string{"creature"}, 2, nil), 3, 3)
	b3 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear C", []string{"creature"}, 2, nil), 3, 3)

	if gangBlockKillsAttacker(gs, atk, []*gameengine.Permanent{b1, b2, b3}) {
		t.Errorf("must not fire when a single blocker can already deliver lethal — defender won't gang")
	}
}

// TestGangBlockKillsAttacker_NoFireOnIndestructible — gang block can't
// kill what doesn't die.
func TestGangBlockKillsAttacker_NoFireOnIndestructible(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Avacyn", []string{"creature"}, 8, nil), 8, 8)
	addKeyword(atk, "indestructible")
	b1 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear A", []string{"creature"}, 2, nil), 3, 3)
	b2 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear B", []string{"creature"}, 2, nil), 3, 3)
	b3 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear C", []string{"creature"}, 2, nil), 3, 3)

	if gangBlockKillsAttacker(gs, atk, []*gameengine.Permanent{b1, b2, b3}) {
		t.Errorf("indestructible attacker cannot be killed by a gang block")
	}
}

// TestGangBlockKillsAttacker_RequiresTwoActiveBlockers — one blocker
// (even if combined power would suffice) isn't a gang; defer to the
// single-blocker logic.
func TestGangBlockKillsAttacker_RequiresTwoActiveBlockers(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Beast", []string{"creature"}, 6, nil), 6, 6)
	b1 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)

	if gangBlockKillsAttacker(gs, atk, []*gameengine.Permanent{b1}) {
		t.Errorf("single blocker should not register as a gang block (defer to single-blocker logic)")
	}
}

// TestGangBlockKillsAttacker_FirstStrikeReducesGangPower — FS attacker
// kills one blocker in §510.5 before the gang swings back. Subtract
// attacker.Power from the gang's sum-power. With a 5/5 FS attacker
// vs 3x 2/3 blockers: gang power 6, FS shaves 5 → 1, less than at=5
// → not deadly. (Single-blocker: 2 < 5 → no single kills.)
func TestGangBlockKillsAttacker_FirstStrikeReducesGangPower(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("FS Knight", []string{"creature"}, 4, nil), 5, 5)
	addKeyword(atk, "first strike")
	b1 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Soldier A", []string{"creature"}, 2, nil), 2, 3)
	b2 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Soldier B", []string{"creature"}, 2, nil), 2, 3)
	b3 := newTestPermanent(gs.Seats[1], newTestCardMinimal("Soldier C", []string{"creature"}, 2, nil), 2, 3)

	if gangBlockKillsAttacker(gs, atk, []*gameengine.Permanent{b1, b2, b3}) {
		t.Errorf("FS attacker should survive the gang — shaved gang power (6-5=1) < attacker toughness (5)")
	}
}

func TestGangBlockKillsAttacker_NilSafe(t *testing.T) {
	gs := newTestGame(t, 2)
	if gangBlockKillsAttacker(gs, nil, nil) {
		t.Errorf("nil attacker must return false")
	}
}

// -----------------------------------------------------------------------------
// canSwingProfitably integration with gang-block detection
// -----------------------------------------------------------------------------

// TestCanSwingProfitably_GangBlockPrunesBigAttacker — end-to-end. A
// 6/6 attacker into a single opponent with 3/3 + 3/3 was pre-fix
// considered profitable (no single 3/3 is "deadly" by old logic).
// Post-fix, gangBlockKillsAttacker fires and prunes.
func TestCanSwingProfitably_GangBlockPrunesBigAttacker(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Big Beast", []string{"creature"}, 6, nil), 6, 6)
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear A", []string{"creature"}, 2, nil), 3, 3)
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear B", []string{"creature"}, 2, nil), 3, 3)

	if canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("6/6 into 3/3 + 3/3 should be pruned — gang-block kills the attacker for at most mutual trade")
	}
}

// TestCanSwingProfitably_GangBlockWithTrampleLeak — trample turns a
// gang-block trade into a profitable swing if the leak is meaningful.
// 8/8 trample into 3/3 + 3/3 leaks 2 damage past (8 power − 6 total
// toughness), which clears the threshold (≥ 2 AND ≥ half attacker
// power → 8/2 = 4; 2 < 4, so actually does NOT clear). Use 10/10
// trample to ensure the leak (10-6=4 ≥ 5 fails too; need stronger).
// Use 10/2 with 2 1/1 chumps to get leak = 10-2 = 8 ≥ 5 ✓.
func TestCanSwingProfitably_GangBlockWithTrampleMeaningfulLeak(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Trampler", []string{"creature"}, 4, nil), 10, 2)
	addKeyword(atk, "trample")
	// Two 1/1 chumps — gang-block kills the attacker (sum power 2 ≥
	// toughness 2), but trample leaks 10-2 = 8 (≥ 2 AND ≥ 10/2=5).
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Chump A", []string{"creature"}, 1, nil), 1, 1)
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Chump B", []string{"creature"}, 1, nil), 1, 1)

	if !canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("10-power trample leaking 8 damage past two 1/1 chumps should be profitable")
	}
}

// TestCanSwingProfitably_GangBlockWithTrampleInsufficientLeak —
// trample doesn't save a gang-block trade when leak is negative
// (blockers absorb all attacker power). 6/6 trample into 4/4 + 4/4:
// no single blocker delivers lethal (4 < 6), gang sum-power 8 ≥
// toughness 6 → gang kills; leak = 6 - 8 = -2 → no spillover at all.
// The trade just burns the attacker.
func TestCanSwingProfitably_GangBlockWithTrampleInsufficientLeak(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Trampler", []string{"creature"}, 5, nil), 6, 6)
	addKeyword(atk, "trample")
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall A", []string{"creature"}, 4, nil), 4, 4)
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall B", []string{"creature"}, 4, nil), 4, 4)

	if canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("6-power trample into 4+4 blockers leaks no damage (sumT=8 ≥ ap=6) — should be pruned")
	}
}

// -----------------------------------------------------------------------------
// leakIsMeaningfulAgainst — unit
// -----------------------------------------------------------------------------

func TestLeakIsMeaningful_NoLeak(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Tiny", []string{"creature"}, 1, nil), 2, 2)
	wall := newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 0, 5)

	if leakIsMeaningfulAgainst(gs, atk, []*gameengine.Permanent{wall}) {
		t.Errorf("2-power attacker into 5-toughness wall has 0 leak; should not be meaningful")
	}
}

func TestLeakIsMeaningful_OneDamageLeakRejected(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Beast", []string{"creature"}, 4, nil), 6, 6)
	wall := newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 0, 5)

	// 6 - 5 = 1 leak → < 2 minimum, rejected.
	if leakIsMeaningfulAgainst(gs, atk, []*gameengine.Permanent{wall}) {
		t.Errorf("1-damage leak does not meaningfully advance the win clock; should be rejected")
	}
}

func TestLeakIsMeaningful_BigLeakAccepted(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Hydra", []string{"creature"}, 5, nil), 10, 10)
	wall := newTestPermanent(gs.Seats[1], newTestCardMinimal("Chump", []string{"creature"}, 1, nil), 1, 1)

	// 10 - 1 = 9 leak → ≥ 2 AND ≥ 10/2=5 → accepted.
	if !leakIsMeaningfulAgainst(gs, atk, []*gameengine.Permanent{wall}) {
		t.Errorf("9-damage leak past a 1/1 chump should be meaningful")
	}
}

func TestLeakIsMeaningful_NilSafe(t *testing.T) {
	gs := newTestGame(t, 2)
	if leakIsMeaningfulAgainst(gs, nil, nil) {
		t.Errorf("nil attacker must return false")
	}
}
