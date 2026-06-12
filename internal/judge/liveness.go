package judge

import (
	"fmt"
	"time"
)

// liveness.go — the LIVENESS dimension (Hex Judge dimension #6).
//
// The five existing dimensions judge what a finished game DID. None of
// them can see a game that never finishes: the r63 firehose found the
// Plargg/Possibility-Storm/Chaos-Wand eliminated-seat loop (a 32-minute
// hang) only via wall-clock anomaly + process sampling — the violation
// stream stayed empty because a hung game emits nothing. LIVENESS makes
// non-termination a first-class, counted violation:
//
//	liveness/wall_clock    — the game exceeded its wall-clock budget.
//	                         Emitted by the DRIVER's watchdog (the game
//	                         itself can't report its own hang); the
//	                         snapshot-level check covers completed-but-
//	                         over-budget near-misses.
//	liveness/turn_overrun  — the turn loop ran PAST the configured
//	                         max-turns cap: the outermost progress bound
//	                         failed to stop the game.
//	liveness/event_flood   — the event log hit its cap while the game
//	                         was still undecided: something is looping
//	                         and logging without advancing the game.
//	liveness/cap_contract  — one of the engine's loop guards FIRED
//	                         (trigger_loop_cap, percard_inline_depth_cap,
//	                         sba max-passes, a no-progress break) but the
//	                         game did NOT end afterwards. The guards'
//	                         contract is "abort the game as a draw, never
//	                         hang or limp on" (the sba704 depth-guard
//	                         pattern) — a fired-but-game-continued guard
//	                         is a liveness bug waiting for a worse seed.
//
// Like stateintegrity.go, the checks assert on a neutral snapshot — no
// engine dependency; the driver resolves engine facts (event counts,
// cap-fire markers, the ended flag) before calling. There are NO engine
// hooks: outside an explicitly-invoked driver this dimension is
// trivially off, so production behavior is untouched.

// Dimension and surface tags for liveness (kept here, not in
// validation.go, so the shared constants file stays untouched while
// sibling dimensions are in flight).
const (
	DimensionLiveness = "liveness" // does the game terminate / do guards stop runaways?
	SurfaceLiveness   = "liveness" // judge/liveness snapshot checks + driver watchdog
)

// LivenessSnapshot is the neutral end-of-game (or end-of-budget) shape
// the driver resolves for CheckLiveness.
type LivenessSnapshot struct {
	// Seed and GameIdx identify the repro.
	Seed    int64
	GameIdx int

	// Turns the game actually took vs the driver's configured cap.
	Turns    int
	MaxTurns int

	// Ended reports whether the game reached a decided end state
	// (winner, draw, or every seat lost) — gs.Flags["ended"] == 1 in
	// engine terms, resolved by the driver.
	Ended bool

	// EventCount is the game's event-log length; EventBudget the cap at
	// which the engine stops retaining (LogEvent's maxEventLog). Zero
	// budget disables the flood check.
	EventCount  int
	EventBudget int

	// CapFires lists every engine loop-guard that fired during the game
	// (game_draw reasons like "trigger_loop_cap" /
	// "percard_inline_depth_cap", SBA max-passes markers, per-handler
	// no-progress breaks). The cap_contract check holds each one to the
	// "guard ends the game" contract.
	CapFires []string

	// Elapsed wall-clock for the game vs the watchdog budget. Zero
	// budget disables the wall-clock check.
	Elapsed time.Duration
	Budget  time.Duration
}

// CheckLiveness runs every liveness rule against the snapshot and
// returns the violations. Each violation is ALSO routed through
// LogViolation (Surface set, so driver normalization loops know it is
// already routed) — same convention as stateintegrity.go.
func CheckLiveness(snap LivenessSnapshot) []ValidationViolation {
	var out []ValidationViolation
	add := func(name, msg string) {
		v := ValidationViolation{
			Surface:   SurfaceLiveness,
			Dimension: DimensionLiveness,
			Name:      name,
			Severity:  SeverityCritical,
			Message:   msg,
			Context: map[string]interface{}{
				"seed":     snap.Seed,
				"game_idx": snap.GameIdx,
				"turns":    snap.Turns,
			},
		}
		LogViolation(v)
		out = append(out, v)
	}

	if snap.Budget > 0 && snap.Elapsed > snap.Budget {
		add("wall_clock", fmt.Sprintf(
			"game ran %s against a %s liveness budget (seed=%d game=%d) — non-termination or pathological slowdown",
			snap.Elapsed.Round(time.Millisecond), snap.Budget, snap.Seed, snap.GameIdx))
	}
	if snap.MaxTurns > 0 && snap.Turns > snap.MaxTurns {
		add("turn_overrun", fmt.Sprintf(
			"turn loop ran to turn %d past the max-turns cap %d (seed=%d game=%d) — outermost progress bound failed",
			snap.Turns, snap.MaxTurns, snap.Seed, snap.GameIdx))
	}
	if snap.EventBudget > 0 && snap.EventCount >= snap.EventBudget && !snap.Ended {
		add("event_flood", fmt.Sprintf(
			"event log hit its %d cap with the game still undecided (seed=%d game=%d) — looping without advancing",
			snap.EventBudget, snap.Seed, snap.GameIdx))
	}
	if len(snap.CapFires) > 0 && !snap.Ended {
		add("cap_contract", fmt.Sprintf(
			"loop guard(s) %v fired but the game did not end (seed=%d game=%d) — guards must abort the game as a draw, never limp on",
			snap.CapFires, snap.Seed, snap.GameIdx))
	}
	return out
}

// WatchdogViolation builds (and routes) the violation a DRIVER emits
// when a game exceeds its wall-clock budget while still running — the
// hung game cannot snapshot itself, so this is the preemptive form of
// liveness/wall_clock. The driver should abandon the game's goroutine
// (an accepted leak in an offline auditing run) and continue the sweep.
func WatchdogViolation(seed int64, gameIdx int, budget time.Duration) ValidationViolation {
	v := ValidationViolation{
		Surface:   SurfaceLiveness,
		Dimension: DimensionLiveness,
		Name:      "wall_clock",
		Severity:  SeverityCritical,
		Message: fmt.Sprintf(
			"game STILL RUNNING at the %s liveness budget (seed=%d game=%d) — abandoned by watchdog; repro and sample-profile this seed",
			budget, seed, gameIdx),
		Context: map[string]interface{}{
			"seed":     seed,
			"game_idx": gameIdx,
			"hung":     true,
		},
	}
	LogViolation(v)
	return v
}
