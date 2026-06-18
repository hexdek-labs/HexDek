package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// coven_dynamic_r63_test.go — r63 mechanic-probe (CR §702.152, ability word).
// Coven = "you control three or more creatures with different powers." The
// CovenActive evaluator was correct and live, but the generic condition
// evaluator (evalCondition) had no "coven" case, so a coven intervening-if /
// conditional / static gate fell through to the fail-closed default — the
// ability never fired even at coven. Fix wires evalCondition "coven" → the
// live CovenActive predicate (mirroring the hellbent → HellbentActive case),
// so coven re-evaluates dynamically as powers change.

// (1) distinct-power set: 3 distinct powers → coven; a shared power → not.
func TestCoven_DistinctPowerSet(t *testing.T) {
	// 1/1 + 2/2 + 3/3 → three distinct powers → coven.
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "A", 1, 1, "creature")
	addBattlefield(gs, 0, "B", 2, 2, "creature")
	addBattlefield(gs, 0, "C", 3, 3, "creature")
	if !CovenActive(gs, 0) {
		t.Error("(1) 1/1+2/2+3/3 must be coven (three distinct powers)")
	}

	// 2/2 + 2/2 + 3/3 → only two distinct powers → NOT coven.
	gs2 := newFixtureGame(t)
	addBattlefield(gs2, 0, "A", 2, 2, "creature")
	addBattlefield(gs2, 0, "B", 2, 2, "creature")
	addBattlefield(gs2, 0, "C", 3, 3, "creature")
	if CovenActive(gs2, 0) {
		t.Error("(1) 2/2+2/2+3/3 must NOT be coven (power 2 shared)")
	}

	// Only two creatures, distinct → not coven (needs 3+).
	gs3 := newFixtureGame(t)
	addBattlefield(gs3, 0, "A", 1, 1, "creature")
	addBattlefield(gs3, 0, "B", 2, 2, "creature")
	if CovenActive(gs3, 0) {
		t.Error("(1) two creatures cannot be coven (needs 3+)")
	}
}

// (2) DYNAMIC: a +1/+1 counter flips coven on in real time (no snapshot).
func TestCoven_FlipsLiveOnCounter(t *testing.T) {
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "A", 1, 1, "creature")
	low := addBattlefield(gs, 0, "B", 1, 1, "creature") // shares power 1 with A
	addBattlefield(gs, 0, "C", 3, 3, "creature")

	// 1,1,3 → only two distinct powers → not coven.
	if CovenActive(gs, 0) {
		t.Fatal("baseline 1/1+1/1+3/3 must NOT be coven")
	}

	// Put a +1/+1 counter on one of the 1/1s → powers become 1,2,3 → coven.
	if low.Counters == nil {
		low.Counters = map[string]int{}
	}
	low.Counters["+1/+1"]++
	if low.Power() != 2 {
		t.Fatalf("fixture: counter should make power 2, got %d", low.Power())
	}
	if !CovenActive(gs, 0) {
		t.Error("(2) a +1/+1 counter making powers 1,2,3 must flip coven ON live")
	}

	// Remove it again → back to 1,1,3 → coven off (re-evaluated, not sticky).
	low.Counters["+1/+1"]--
	if CovenActive(gs, 0) {
		t.Error("(2) removing the counter must flip coven OFF live (not a stale snapshot)")
	}
}

// (3) the fix: evalCondition "coven" reads the live state (previously
// fell through to the fail-closed default → coven gate never satisfied).
func TestCoven_EvalConditionWiredLive(t *testing.T) {
	gs := newFixtureGame(t)
	a := addBattlefield(gs, 0, "A", 1, 1, "creature")
	addBattlefield(gs, 0, "B", 2, 2, "creature")
	addBattlefield(gs, 0, "C", 3, 3, "creature")

	cond := &gameast.Condition{Kind: "coven"}

	// src controlled by seat 0, which has coven → condition TRUE.
	if !evalCondition(gs, a, cond) {
		t.Error("(3) evalCondition(coven) must be TRUE when the controller has coven")
	}

	// Break coven (counter on the 1/1 → 2,2,3 shares power 2) → condition FALSE.
	if a.Counters == nil {
		a.Counters = map[string]int{}
	}
	a.Counters["+1/+1"]++ // A becomes 2/2, now 2,2,3
	if evalCondition(gs, a, cond) {
		t.Error("(3) evalCondition(coven) must be FALSE once a power is shared (live re-eval)")
	}

	// nil source → false (no controller), mirroring the hellbent guard.
	if evalCondition(gs, nil, cond) {
		t.Error("(3) evalCondition(coven) with nil source must be false")
	}
}

// (4) face-down creatures count as power 2 (CR §707.2); tokens count by power.
func TestCoven_FaceDownAndTokensCountByPower(t *testing.T) {
	gs := newFixtureGame(t)
	// A face-down (2 power), a 1/1, a 3/3 → powers 2,1,3 → coven.
	fd := addBattlefield(gs, 0, "FaceDown", 5, 5, "creature")
	if fd.Flags == nil {
		fd.Flags = map[string]int{}
	}
	fd.Flags["face_down"] = 1 // power now 2 regardless of printed 5
	addBattlefield(gs, 0, "Token", 1, 1, "creature", "token")
	addBattlefield(gs, 0, "Big", 3, 3, "creature")
	if fd.Power() != 2 {
		t.Fatalf("fixture: face-down power should be 2, got %d", fd.Power())
	}
	if !CovenActive(gs, 0) {
		t.Error("(4) face-down(2) + token(1) + 3/3 = powers 2,1,3 → coven")
	}
}
