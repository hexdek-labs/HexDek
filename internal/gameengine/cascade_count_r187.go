package gameengine

import "sync/atomic"

// r187 — HOW MANY cascades swallowed, per game.
//
// THE GAP r182/r185/r186 LEFT. Three fields describe the swallows of
// one game: swallowed= (the total), cascademax= (the largest run
// inside ONE top-level cascade), and r186's shape= for that one peak
// cascade. Nothing says how many top-level cascades swallowed at all,
// and that number is the one that separates the three regimes
// production actually shows:
//
//	 1 one wide cascade (cascademax == swallowed)  285 games (82%)
//	   median    11 swallows, 0.30s
//	 2 re-entry (shape loopN, n > boardMax)         25 games ( 7%)
//	   median    77 swallows, 0.20s
//	 3 FAN-OUT: many independent shallow cascades    3 games ( 1%)
//	   median 2,157 swallows, 89.7s, worst 203.3s
//
// Regime 3 holds ALL of the wall-clock cost — both of the only two
// capped games over ten seconds — and it is the regime the existing
// fields cannot name. Its worst case printed swallowed=3122
// cascademax=7, from which "at least 446 cascades" can only be
// INFERRED by dividing one field by another. Cross-field arithmetic
// over two numbers that are maxima rather than means is exactly the
// kind of inference this instrument family exists to retire, so the
// count is counted.
//
// WHY IT IS NOT DERIVABLE. swallowed/cascademax is a LOWER bound and
// a loose one: cascademax is the largest run, so dividing by it
// under-counts whenever the cascades differ in size, which in regime
// 3 they always do. There is no arrangement of the three existing
// fields that yields the count, and no process-wide aggregate can
// supply it either — cascadeShapeCascades (r186) counts classified
// cascades across the whole run, which is the sum over games and
// cannot be attributed to one.
//
// WHERE IT IS COUNTED, AND WHY NOT IN NoteCascadeRoot. The obvious
// site is r186's `depthCapCascade != 0` arm of NoteCascadeRoot, which
// already fires once per swallowing cascade. It is the WRONG site,
// for the reason r186's own header documents about finishCascade: a
// cascade is only seen there when the NEXT top-level cascade starts,
// so a game whose last swallowing cascade is also its last top-level
// cascade never counts it. The loss is one per game against a median
// of one in regime 1 — i.e. in 82% of capped games that site would
// report zero.
//
// So the count is taken at the 0 -> 1 transition of depthCapCascade
// inside NoteDepthCapSwallow instead. depthCapCascade is reset to
// zero by NoteCascadeRoot at every new top-level cascade and
// incremented only here, so `== 1` after the increment is true
// exactly once per cascade that swallows, at its FIRST swallow —
// which needs no later cascade to observe it and is therefore exact,
// including for the last cascade of a game.
//
// COST, AND WHERE EACH LINE RUNS. Every line in this file runs inside
// NoteDepthCapSwallow, on the capped branch that 2% of games ever
// reach, or in a read-only accessor called once per game from a
// deferred log. The UNCAPPED dispatch path is untouched: fireTrigger
// still calls exactly NoteCascadeRoot (one int compare at depth 1)
// and NoteTriggerDepth, and neither gained a line. Per swallow this
// adds one int compare; per swallowing CASCADE it adds one int
// increment and one bumpAtomicMax, whose common case is a single
// atomic load. No allocation, no map, no string building.
//
// Clone note: CloneForRollout builds from an explicit field list, so
// a rollout clone starts at zero and is counted as its own game.
// Same trade r163/r182/r185/r186 took; the grinder reports clones=0.

// depthCapCascadesMax is the largest per-GAME count of swallowing
// top-level cascades seen anywhere in the process. It is a per-game
// maximum and so belongs beside r182's max: and cascademax:, not
// beside r186's per-CASCADE histogram.
var depthCapCascadesMax atomic.Uint64

// noteSwallowingCascade records that the CURRENT top-level cascade
// has begun swallowing. Called ONLY from NoteDepthCapSwallow, and
// only at that cascade's first swallow.
func (gs *GameState) noteSwallowingCascade() {
	gs.depthCapCascadesSwallowed++
	bumpAtomicMax(&depthCapCascadesMax, uint64(gs.depthCapCascadesSwallowed))
}

// DepthCapSwallowingCascadeCount is this ONE game's count of
// top-level dispatch cascades that swallowed at least one dispatch.
//
// WHAT IT IS NOT. It is not the number of top-level cascades the game
// ran — fireTrigger starts one on every depth-1 dispatch and the
// overwhelming majority swallow nothing, so that number is a measure
// of how much the game played, not of what the cap cost it. Only
// cascades that actually lost a dispatch are counted.
//
// HOW TO READ IT ALONGSIDE THE OTHERS. With swallowed= and
// cascademax=, it names the regime directly rather than by division:
// cascades=1 is one wide chain; cascades small with cascademax large
// is re-entry; cascades large with cascademax small is fan-out, which
// is where the wall-clock cost lives.
//
// COST. One field load behind a nil check, read once per game from a
// deferred log. Read-only: no reset, no side effect, nothing the
// engine can branch on. Mirrors DepthCapSwallowedCount exactly,
// including returning 0 for a nil receiver — the caller decides what
// a missing GameState should render, because only the caller knows
// whether a game ran at all.
func (gs *GameState) DepthCapSwallowingCascadeCount() int {
	if gs == nil {
		return 0
	}
	return gs.depthCapCascadesSwallowed
}

// DepthCapCascadesMaxPerGame is the process-wide largest per-game
// count of swallowing cascades.
func DepthCapCascadesMaxPerGame() uint64 {
	return depthCapCascadesMax.Load()
}

// ResetDepthCapCascadeCountForTest clears the process-wide maximum.
// Called by ResetDepthCapForTest so no test has to know r187 exists.
func ResetDepthCapCascadeCountForTest() {
	depthCapCascadesMax.Store(0)
}
