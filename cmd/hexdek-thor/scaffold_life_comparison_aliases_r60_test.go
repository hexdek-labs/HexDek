package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase 1C audit (PR #476) flagged three condition kinds bucketed in
// scripts/era1_scaffold_audit.py's BUCKETED_KINDS but lacking Go-side
// dispatch: repeat_any_optional, life_delta_threshold, life_threshold_both.
// Each is wired as a 1-line alias to an existing scaffold arm. These tests
// pin the alias routing so the audit's structured-bucketed residual for
// those kinds drops to zero.

func TestDetectConditionScaffold_RepeatAnyOptional_AliasesToRepeatN(t *testing.T) {
	cond := &gameast.Condition{Kind: "repeat_any_optional"}
	got := detectConditionScaffold(cond)
	if got.kind != condScaffoldRepeatN {
		t.Fatalf("repeat_any_optional: want condScaffoldRepeatN, got %v", got.kind)
	}
	if got.count != 3 {
		t.Errorf("repeat_any_optional default count: want 3, got %d", got.count)
	}
}

func TestSetupCondition_LifeDeltaThreshold_AliasesToLifeThreshold(t *testing.T) {
	gs := newTestGameState(2)
	gs.Seats[0].Life = 40
	cond := &gameast.Condition{
		Kind: "life_delta_threshold",
		Args: []interface{}{"you", ">=", 5},
	}
	setupCondition(gs, cond)
	if gs.Seats[0].Life != 5 {
		t.Fatalf("life_delta_threshold: want seat-0 life=5 (life_threshold scaffold), got %d", gs.Seats[0].Life)
	}
}

func TestSetupCondition_LifeThresholdBoth_AliasesToLifeThreshold(t *testing.T) {
	gs := newTestGameState(2)
	gs.Seats[0].Life = 40
	cond := &gameast.Condition{
		Kind: "life_threshold_both",
		Args: []interface{}{
			[]interface{}{"you", ">=", 20},
			[]interface{}{"opponent", "<=", 10},
		},
	}
	setupCondition(gs, cond)
	if gs.Seats[0].Life != 5 {
		t.Fatalf("life_threshold_both: want seat-0 life=5 (life_threshold scaffold), got %d", gs.Seats[0].Life)
	}
}
