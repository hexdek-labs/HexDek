package game

import (
	"context"
	"testing"
)

// R60 — pins the two-level loss/win architecture introduced under
// dev/seat-level-loss-conditions-r60.
//
// Level 1 (per-seat): Player.CheckLossConditions() — pure method, no DB
// access, returns (LossCause, bool). Tested below as the "per-clause
// seat tests."
//
// Level 2 (engine-level): CheckGameEnd in combat.go — collects every
// seat's loss result, then applies CR §104.2c win-effects, §104.2a
// last-seat-wins, §104.2b simultaneous-elim-draw. Tested below as
// the "aggregation game tests."
//
// The empty-library #678 work now flows through the new method
// instead of inline checks — TestCheckLossConditions_EmptyLibrary
// pins that integration.

// -----------------------------------------------------------------------
// Per-seat tests — Player.CheckLossConditions() in isolation.
// -----------------------------------------------------------------------

func TestCheckLossConditions_HealthySeatNotLost(t *testing.T) {
	p := &Player{Life: 40, PoisonCounters: 0, AttemptedEmptyDraw: false}
	cause, lost := p.CheckLossConditions()
	if lost {
		t.Fatalf("healthy seat should not be lost; got cause=%q", cause)
	}
	if cause != LossNone {
		t.Errorf("expected LossNone; got %q", cause)
	}
}

func TestCheckLossConditions_LifeZeroLost_704_5a(t *testing.T) {
	p := &Player{Life: 0, PoisonCounters: 0}
	cause, lost := p.CheckLossConditions()
	if !lost {
		t.Fatal("life=0 seat should be lost per CR §704.5a")
	}
	if cause != LossLife {
		t.Errorf("expected LossLife; got %q", cause)
	}
}

func TestCheckLossConditions_LifeNegativeLost_704_5a(t *testing.T) {
	p := &Player{Life: -3}
	cause, lost := p.CheckLossConditions()
	if !lost || cause != LossLife {
		t.Errorf("life=-3 should be LossLife; got cause=%q lost=%v", cause, lost)
	}
}

func TestCheckLossConditions_EmptyLibraryLost_704_5b(t *testing.T) {
	p := &Player{Life: 40, AttemptedEmptyDraw: true}
	cause, lost := p.CheckLossConditions()
	if !lost {
		t.Fatal("AttemptedEmptyDraw seat should be lost per CR §704.5b")
	}
	if cause != LossEmptyLibrary {
		t.Errorf("expected LossEmptyLibrary; got %q", cause)
	}
}

func TestCheckLossConditions_PoisonLost_704_5c(t *testing.T) {
	p := &Player{Life: 40, PoisonCounters: 10}
	cause, lost := p.CheckLossConditions()
	if !lost {
		t.Fatal("poison=10 seat should be lost per CR §704.5c")
	}
	if cause != LossPoison {
		t.Errorf("expected LossPoison; got %q", cause)
	}
}

func TestCheckLossConditions_PoisonAboveThreshold(t *testing.T) {
	p := &Player{Life: 40, PoisonCounters: 25}
	if _, lost := p.CheckLossConditions(); !lost {
		t.Error("poison=25 should still be lost")
	}
}

func TestCheckLossConditions_LifeFirstWhenMultipleClauses(t *testing.T) {
	// CR §704.5 evaluates clauses simultaneously, but our method
	// returns the FIRST matched cause. Document and pin that ordering
	// — Life is checked before EmptyLibrary, EmptyLibrary before
	// Poison. Stable order keeps action-log payloads deterministic.
	p := &Player{Life: -5, AttemptedEmptyDraw: true, PoisonCounters: 12}
	cause, lost := p.CheckLossConditions()
	if !lost {
		t.Fatal("multi-cause seat must be lost")
	}
	if cause != LossLife {
		t.Errorf("ordering: Life first; got %q", cause)
	}
}

func TestCheckLossConditions_NilPlayerSafe(t *testing.T) {
	var p *Player
	cause, lost := p.CheckLossConditions()
	if lost || cause != LossNone {
		t.Errorf("nil player should report (LossNone, false); got (%q, %v)", cause, lost)
	}
}

// -----------------------------------------------------------------------
// Aggregation tests — CheckGameEnd master logic across §104.2 clauses.
// -----------------------------------------------------------------------

// (a) One seat left → that seat wins (§104.2a). Already covered by
// TestCheckGameEnd_AttemptedEmptyDrawEliminatesSeat in
// draw_empty_library_r60_test.go; add a life-loss variant here to
// confirm the refactored aggregator routes life-loss through the new
// method.

func TestCheckGameEnd_LifeLossEliminatesSeatLeavesOneAlive(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	p1, err := GetGamePlayer(ctx, d, gameID, 1)
	if err != nil {
		t.Fatalf("get seat 1: %v", err)
	}
	p1.Life = 0
	if err := UpdateGamePlayer(ctx, d, p1); err != nil {
		t.Fatalf("kill seat 1: %v", err)
	}

	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}
	g, err := GetGame(ctx, d, gameID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if g.FinishedAt == 0 {
		t.Error("game should be finished")
	}
	if g.Winner != "dev-r60-empty-lib" {
		t.Errorf("seat 0 should win after seat 1 dies to life=0; got winner=%q", g.Winner)
	}
}

// (b) Simultaneous loss → draw (§104.2b). Pin the multi-clause case:
// one seat hits life=0, the other hits poison=10 in the same window.

func TestCheckGameEnd_SimultaneousLifeAndPoisonDraws(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	p0, _ := GetGamePlayer(ctx, d, gameID, 0)
	p1, _ := GetGamePlayer(ctx, d, gameID, 1)
	p0.Life = 0
	p1.PoisonCounters = 10
	if err := UpdateGamePlayer(ctx, d, p0); err != nil {
		t.Fatalf("kill p0: %v", err)
	}
	if err := UpdateGamePlayer(ctx, d, p1); err != nil {
		t.Fatalf("infect p1: %v", err)
	}

	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}
	g, _ := GetGame(ctx, d, gameID)
	if g.FinishedAt == 0 {
		t.Error("game should be finished after both seats hit §704.5")
	}
	if g.Winner != "" {
		t.Errorf("simultaneous life+poison elim should draw (Winner=\"\"); got %q", g.Winner)
	}
}

// (c) Approach-second-cast: §104.2c win effect overrides simultaneous
// loss. The seat is at life=0 AND has WinEffectTriggered=true — they
// win, not lose. CR §104.3a is explicit: win effects beat loss effects
// on the same player.

func TestCheckGameEnd_WinEffectOverridesOwnLossCondition_104_2c(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	// Seat 0: Approach of the Second Sun resolved with second cast
	// confirmed (WinEffectTriggered=true), but the same turn took
	// lethal damage in response → life=0.
	p0, _ := GetGamePlayer(ctx, d, gameID, 0)
	p0.Life = 0
	p0.WinEffectTriggered = true
	if err := UpdateGamePlayer(ctx, d, p0); err != nil {
		t.Fatalf("update p0: %v", err)
	}

	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}
	g, _ := GetGame(ctx, d, gameID)
	if g.FinishedAt == 0 {
		t.Error("game should be finished")
	}
	if g.Winner != "dev-r60-empty-lib" {
		t.Errorf("Approach winner with life=0 should still win per §104.3a; got winner=%q", g.Winner)
	}
}

// (d) Felidar Sovereign upkeep win — healthy seat triggers a win effect
// while opponents are still alive. The win is immediate; opponents
// don't get to respond by CheckGameEnd time (the response window is
// upstream, on the stack).

func TestCheckGameEnd_WinEffectGrantsImmediateVictory(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	// Seat 0 has Felidar Sovereign and 50+ life at upkeep — the
	// engine flips WinEffectTriggered when the upkeep trigger
	// resolves. Seat 1 is healthy.
	p0, _ := GetGamePlayer(ctx, d, gameID, 0)
	p0.Life = 55
	p0.WinEffectTriggered = true
	if err := UpdateGamePlayer(ctx, d, p0); err != nil {
		t.Fatalf("flip Felidar trigger: %v", err)
	}

	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}
	g, _ := GetGame(ctx, d, gameID)
	if g.FinishedAt == 0 {
		t.Error("game should end on Felidar trigger")
	}
	if g.Winner != "dev-r60-empty-lib" {
		t.Errorf("Felidar controller should win; got winner=%q", g.Winner)
	}
}

// (c)/(d) reinforcement: when MULTIPLE seats trigger a win effect in
// the same window (Approach + Felidar resolving on the same upkeep,
// e.g.), the lowest seat index wins (deterministic tie-break). Pins
// the documented behavior in CheckGameEnd so a future ordering change
// doesn't silently flip results.

func TestCheckGameEnd_MultipleWinEffectsLowestSeatWins(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	p0, _ := GetGamePlayer(ctx, d, gameID, 0)
	p1, _ := GetGamePlayer(ctx, d, gameID, 1)
	p0.WinEffectTriggered = true
	p1.WinEffectTriggered = true
	if err := UpdateGamePlayer(ctx, d, p0); err != nil {
		t.Fatalf("update p0: %v", err)
	}
	if err := UpdateGamePlayer(ctx, d, p1); err != nil {
		t.Fatalf("update p1: %v", err)
	}

	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}
	g, _ := GetGame(ctx, d, gameID)
	if g.Winner != "dev-r60-empty-lib" {
		t.Errorf("seat 0 (lower seat) should win the tie-break; got winner=%q", g.Winner)
	}
}

// Storage round-trip for the new WinEffectTriggered column. Mirrors
// the AttemptedEmptyDraw round-trip in draw_empty_library_r60_test.go.

func TestWinEffectTriggered_PersistsThroughStorage(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	p, _ := GetGamePlayer(ctx, d, gameID, 0)
	if p.WinEffectTriggered {
		t.Fatal("fresh player should have WinEffectTriggered=false")
	}
	p.WinEffectTriggered = true
	if err := UpdateGamePlayer(ctx, d, p); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := GetGamePlayer(ctx, d, gameID, 0)
	if !got.WinEffectTriggered {
		t.Error("GetGamePlayer should read WinEffectTriggered=true")
	}
	listed, _ := ListGamePlayers(ctx, d, gameID)
	for _, lp := range listed {
		if lp.SeatPosition == 0 && !lp.WinEffectTriggered {
			t.Error("ListGamePlayers should read WinEffectTriggered=true for seat 0")
		}
	}
}
