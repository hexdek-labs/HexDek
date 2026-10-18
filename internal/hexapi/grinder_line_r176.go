package hexapi

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// grinderWindow carries the per-window numbers the grinder's periodic
// line reports. Everything in here is a delta or a sample the ticker
// loop owns; the process-global engine counters are deliberately NOT
// fields, because renderGrinderLine reads them itself (see below).
type grinderWindow struct {
	games        int64
	cumGPM       float64
	winGPM       float64
	turnsPerGame float64
	gcPerGame    float64
	eloPool      int
	heapMB       float64
	sysMB        float64
	numGC        uint32
	dLimited     int64
	allocMB      float64
	dClones      int64
	ccRate       float64
	ccHits       uint64
	ccLooks      uint64

	ccInvalPerGame float64

	// tmemo is pre-rendered by the caller because it is a WINDOW rate
	// (hits this window / looks this window), computed from the
	// loop's own previous-sample state. Every other counter column is
	// cumulative and is read here.
	tmemo string
}

// renderGrinderLine builds the grinder's periodic log line.
//
// r176 — why this is a function instead of an inline log.Printf.
//
// Three rounds in a row (r167, r169, and the counter r142 replaced)
// shipped a column that compiled, read correctly at its source, and
// printed nothing, because the only thing testable was either the
// counter (at one end) or the source text of the Printf (at the
// other). r169 says so in its own comment and then asserts on
// os.ReadFile("showmatch.go"), which cannot catch an argument passed
// in the wrong position or a field whose getter returns "" forever.
//
// Splitting the rendering out makes the TRANSPORT testable: a test
// can drive the real producing code, call this, and read the actual
// bytes that reach the log. The window numbers come in as a struct;
// the cumulative engine counters are read in here, so a test that
// hand-feeds nothing still gets a fully populated line.
func renderGrinderLine(w grinderWindow) string {
	// r160 — loop-guard firings, cumulative by guard name. Same
	// defect r142 found for the trigger limiter and fixed for that
	// ONE counter: LogLoopGuardFired writes only to the event log,
	// the grinder runs EventLogNone, so all NINE guards were
	// invisible on the only box where they matter. `grep -c
	// game_deadline server.log` returns 0 over the whole log while
	// that log holds games of 3102.6s and 2148.3s against an armed
	// 10-minute cap. Empty string when nothing has ever fired, so
	// the normal line is unchanged.
	// r162 — the busiest (guard, label) pairs. Separate field so
	// guards= stays a small stable column; empty when nothing fired.
	guardTop := gameengine.LoopGuardLabelTop(5)
	if guardTop != "" {
		guardTop = " guardtop=" + guardTop
	}
	// r164 — text-predicate memo hit rate. r155's lesson: publish it,
	// do not assume it. My charCache prediction was wrong by 4x.
	tpHits, tpMiss, tpUnc := gameengine.TextPredicateStats()
	tpField := ""
	if tpHits+tpMiss > 0 {
		tpField = fmt.Sprintf(" tp=%.1f%% (%d/%d)",
			100*float64(tpHits)/float64(tpHits+tpMiss), tpHits, tpHits+tpMiss)
		if tpUnc > 0 {
			tpField += fmt.Sprintf(" tpunc=%d", tpUnc)
		}
	}
	// r163 — trigger-nesting survival curve: games reaching >= depth d.
	tdepth := gameengine.TriggerDepthSummary()
	if tdepth != "" {
		tdepth = " tdepth=" + tdepth
	}
	guards := gameengine.LoopGuardSummary()
	if guards != "" {
		guards = " guards=" + guards
	}
	// r167 — every label for game_deadline, not just the top 5.
	// This is the field that says WHICH check site caught a capped
	// game; guardtop= cannot, because the deadline fires once per
	// game and is buried under per-card counters in the thousands.
	// Empty until a game actually hits its cap.
	deadlines := gameengine.LoopGuardLabelsFor("game_deadline")
	if deadlines != "" {
		deadlines = " deadlines=" + deadlines
	}
	// r176 — unrecognized condition kinds, cumulative. evalCondition
	// fails closed on a kind it has no case for, which silently turns
	// the gated effect into a no-op; its comment promises the kinds
	// are logged as "a measurable burn-down list", but that log is
	// gs.EventLog and the grinder discards it, so `grep -c
	// condition_unrecognized server.log` is 0 over the whole log. A
	// corpus scan says 197 cards are affected and "if" dominates at
	// 107 — but corpus presence is not play frequency, and this is
	// the field that gives the real one. Top 8 rather than 5: the
	// scan found exactly five kinds and the headroom makes a sixth
	// visible instead of hiding it. Empty until one actually fires.
	condUnk := gameengine.ConditionUnrecognizedTop(8)
	if condUnk != "" {
		condUnk = " condunk=" + condUnk
	}
	// r182 — per-GAME shape of the depth-cap swallows. guards= and
	// guardtop= are window totals, so "1,857 games losing one
	// dispatch" and "one game losing 1,857" print identically; this
	// is the field that separates them. Empty until the cap fires.
	capSwallow := gameengine.DepthCapSummary()
	if capSwallow != "" {
		capSwallow = " capswallow=" + capSwallow
	}
	// r182 — which event a game's FIRST swallow was on, counted once
	// per game. guardtop= attributes per DISPATCH, so one looping game
	// dominates it; this column says how many GAMES each event starts.
	capFirst := gameengine.LoopGuardLabelsFor(gameengine.DepthCapFirstEventGuard)
	if capFirst != "" {
		capFirst = " capfirst=" + capFirst
	}
	// r186 — the SHAPE of each capped cascade, counted per CASCADE
	// (capswallow= above counts GAMES; two denominators do not belong
	// in one bracketed list). capfirst= names the event, but a
	// re-entering chain and a wide finite wave that is merely wider
	// than the cap print the SAME name. This crosses the distinct
	// swallowed-event count with the battlefield population the
	// cascade ran against: `wave`/`widemix` mean the cascade could not
	// have swallowed more than the board supplied, so the cap is the
	// problem; `loop1`/`loopN` mean it outran the board and is worth a
	// loop hunt. loop1-vs-loopN is NOT self-versus-mutual — see the
	// header of cascade_composition_r186.go. Empty until a cascade is
	// classified.
	capShape := gameengine.DepthCapShapeSummary()
	if capShape != "" {
		capShape = " capshape=" + capShape
	}

	return fmt.Sprintf("grinder: %d games (%.1f g/min cum, %.1f g/min now), "+
		"turns/game=%.1f gc/game=%.2f, ELO pool: %d, heap=%.0fMB, sys=%.0fMB, gc=%d, limited=%d, "+
		"alloc=%.0fMB clones=%d, chit=%.1f%% (%d/%d) cinval/game=%.0f%s%s%s%s%s%s%s%s%s%s",
		w.games, w.cumGPM, w.winGPM, w.turnsPerGame, w.gcPerGame,
		w.eloPool, w.heapMB, w.sysMB, w.numGC, w.dLimited,
		w.allocMB, w.dClones, w.ccRate, w.ccHits, w.ccLooks, w.ccInvalPerGame,
		guards, deadlines, w.tmemo, guardTop, tdepth, tpField, condUnk,
		capSwallow, capFirst, capShape)
}
