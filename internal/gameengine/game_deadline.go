package gameengine

import "time"

// Wall-clock game deadline — the backstop for runaway loops that no
// counter-based guard happens to watch.
//
// Every liveness guard we have (trigger_loop_cap, trigger_drain_cap,
// sba_max_passes, drain_iteration_cap, percard_dispatch_*_cap,
// move_no_progress) is a counter inside one specific loop. That only
// protects the loops someone thought to instrument. On 2026-10-08 the
// production grinder sat on a single game for sixteen days, holding a
// core, with the game counter frozen at 30,238 — and zero loop guards
// had ever fired on that box. So the spin was in a loop nobody had
// capped.
//
// The grinder's own protections could not help, because of where they
// sit:
//
//	for turn := 1; turn <= showmatchMaxTurn; turn++ {
//	    tournament.TakeTurn(gs)   // never returns
//	    ... heap check ...        // never reached again
//	}
//
// Both the 80-turn ceiling and the per-10-turn heap abort live *inside*
// the loop body that is stuck, so neither is ever evaluated. A turn cap
// cannot bound a single turn.
//
// A deadline is different in kind: it is not tied to any one loop, so a
// new unguarded loop is covered the day it is written rather than the
// day someone notices it. On trip the game ends as a draw, which is also
// what CR §104.4b prescribes for a mandatory loop no player can break.
//
// Storage note: the deadline lives in gs.Flags rather than as a
// GameState field on purpose. CloneForRollout builds its result from an
// explicit field list, so a new struct field would be silently dropped
// from every rollout clone; Flags is copied wholesale and therefore
// inherits correctly for free. Unix SECONDS are used so the value fits
// an int on every platform (nanoseconds would overflow a 32-bit int).
const gameDeadlineFlag = "game_deadline_unix"

// SetGameDeadline arms a wall-clock budget for this game. Callers that
// never arm one are completely unaffected: an absent or zero flag means
// "no deadline", so the default stays unlimited.
func SetGameDeadline(gs *GameState, at time.Time) {
	if gs == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags[gameDeadlineFlag] = int(at.Unix())
}

// ClearGameDeadline disarms the budget.
func ClearGameDeadline(gs *GameState) {
	if gs == nil || gs.Flags == nil {
		return
	}
	delete(gs.Flags, gameDeadlineFlag)
}

// GameDeadline returns the armed deadline and whether one is set.
func GameDeadline(gs *GameState) (time.Time, bool) {
	if gs == nil || gs.Flags == nil {
		return time.Time{}, false
	}
	secs, ok := gs.Flags[gameDeadlineFlag]
	if !ok || secs <= 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(secs), 0), true
}

// GameDeadlineExceeded reports whether an armed deadline has passed. It
// is false when no deadline is armed, so it is safe to call from any
// loop regardless of caller.
func GameDeadlineExceeded(gs *GameState) bool {
	at, ok := GameDeadline(gs)
	if !ok {
		return false
	}
	return !time.Now().Before(at)
}

// CheckGameDeadline is the call site helper: if the budget has been
// exceeded it emits the uniform loop-guard event naming the loop that
// noticed, ends the game as a draw, and returns true so the caller can
// bail out of its loop. It returns false (and does nothing) when no
// deadline is armed or time remains.
//
// `site` identifies the loop, so the log says which one was spinning —
// that is the diagnostic the 2026-10-08 stall never produced.
func CheckGameDeadline(gs *GameState, site string) bool {
	if gs == nil {
		return false
	}
	if !GameDeadlineExceeded(gs) {
		return false
	}
	// r161 — an already-ended game STILL reports the breach.
	//
	// This used to return false the moment gs.Flags["ended"] == 1, which
	// looked like simple double-end protection and was in fact the hole
	// that made the whole backstop ineffective. EndGameAsLoopDraw cannot
	// interrupt the turn that is currently executing: the grinder's
	// CheckEnd() break sits BETWEEN turns, so a spin inside a single
	// TakeTurn never reaches it. The first trip therefore ended the game
	// and returned into the spin — and from then on every call site here
	// answered "no deadline problem", so nothing ever unwound.
	//
	// Measured on production 2026-10-17 with r160's counters: two games
	// tripped the 10-minute cap (guards=game_deadline:1 then :2 — once
	// each, exactly as this early return predicts) and then ran on for
	// 77s and 1094s respectively, completing at 677.2s and 1693.9s. The
	// second spent over eighteen minutes simulating a game already
	// recorded as a draw.
	//
	// So: report the breach every time, but end the game only once.
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		return true
	}
	at, _ := GameDeadline(gs)
	LogLoopGuardFired(gs, "game_deadline", map[string]interface{}{
		"site":         site,
		"turn":         gs.Turn,
		"stack":        len(gs.Stack),
		"deadline_utc": at.UTC().Format(time.RFC3339),
		"over_by_sec":  int(time.Since(at).Seconds()),
	})
	EndGameAsLoopDraw(gs, "game_deadline")
	return true
}
