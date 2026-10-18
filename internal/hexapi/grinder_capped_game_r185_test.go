package hexapi

import (
	"bytes"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// r185 — attribute capped games INDEPENDENTLY OF DURATION.
//
// r184 hung the per-game swallow count on the SLOW GAME line. One
// production tick later that instrument measured itself and failed:
//
//	13,509 games, 280 hit the depth cap, 8 crossed the 10s slow
//	threshold, and 2 of those 8 had swallowed > 0.
//
// 278 of 280 capped games are invisible to the join. Capped games are
// not slow games — swallowing a dispatch is the engine declining to
// do work — so conditioning the attribution on duration sampled
// almost exactly the wrong population. The CAPPED GAME line fires on
// the swallow count alone.
//
// WHAT THESE TESTS CAN AND CANNOT DO, STATED UP FRONT. The hexapi
// fixture decks are synthetic 36-Forest piles with no triggered
// abilities whatsoever, so trigger nesting never reaches 2, let
// alone the cap at 8. THIS PACKAGE CANNOT MANUFACTURE A SWALLOW
// without reaching into the engine's hot path, which is out of
// bounds for an instrumentation-only change. That is the same limit
// r184 documented, and it is why these tests drive the floor to 0
// (every game is "capped") to pin the TRANSPORT and the LINE SHAPE,
// while the VALUE semantics are pinned at their source in
// gameengine/depth_cap_cascade_game_max_r185_test.go. Both halves
// are covered; neither half covers the other.
//
// MUTATION CONTROLS (counts are tests that go RED in this package):
//
//	M5  gate the CAPPED line on duration too (&& d >= slowGameThreshold)
//	    -> predicted 4, ACTUAL 3: EmitsRegardlessOfDuration,
//	       LineShapeAndFieldOrder, CarriesTheCascadePeak.
//	       BothLinesJoinOnTheSameRun stays GREEN, correctly — it
//	       drives the slow floor to 0 so that both lines appear, which
//	       also satisfies the mutant's duration gate. It is a join
//	       test, not a sampling test, and the sampling property is the
//	       one the other three hold.
//	M6  emit the line for a nil GameState (drop the gs != nil guard,
//	    rendering swallowed=n/a)
//	    -> predicted 1, actual 1: CappedLineIsNotEmittedForANonGame.
//	M7  reorder to "swallowed=N cascademax=N ... seed=N decks=[...]"
//	    (the shape the brief suggested)
//	    -> predicted 2, ACTUAL 4: LineShapeAndFieldOrder and
//	       BothLinesJoinOnTheSameRun as predicted, plus
//	       EmitsRegardlessOfDuration and CarriesTheCascadePeak, which
//	       both match through r185CappedGame and so pin the full field
//	       order rather than just the line's presence. I under-counted
//	       by forgetting the shared regex.
//	M8  print gs.DepthCapSwallowedCount() for cascademax as well
//	    -> predicted 0, actual 0. CHASED, and it is the same
//	       fixture-coverage limit as r184's M1, not a missing
//	       assertion: every driveable game here swallows zero, so the
//	       game total and the cascade peak are both 0 and no
//	       assertion in this package can separate them. What separates
//	       them is TestR185_PeaksBelowTheGameTotal in gameengine,
//	       which is RED under the equivalent mutation. Recorded as a
//	       known limit of the fixtures rather than papered over with a
//	       hand-fed value at the destination, which is exactly the
//	       mistake r167 shipped.

var r185CappedGame = regexp.MustCompile(
	`CAPPED GAME [0-9.]+s turns=\d+ seed=(-?\d+) decks=\[[^\]]*\] swallowed=(\d+) cascademax=(\d+)`)

// r185Grind runs one real grinder game with both thresholds driven,
// and returns the log output. slowThresh is set absurdly high by
// default so the SLOW GAME line stays out of the way.
func r185Grind(t *testing.T, sm *Showmatch, slowThresh time.Duration, capThresh int) string {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	prevSlow, prevCap := slowGameThreshold, cappedGameThreshold
	slowGameThreshold, cappedGameThreshold = slowThresh, capThresh
	t.Cleanup(func() {
		slowGameThreshold, cappedGameThreshold = prevSlow, prevCap
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	sm.runOneGameFast(rand.New(rand.NewSource(1)))
	return buf.String()
}

// THE POINT OF THE ROUND. A game far under the slow threshold must
// still be attributed. The slow threshold is set to an hour here, so
// no SLOW GAME line can possibly appear, and the CAPPED line must.
func TestR185_EmitsRegardlessOfDuration(t *testing.T) {
	out := r185Grind(t, maintenanceFixture(), time.Hour, 0)
	if strings.Contains(out, "SLOW GAME") {
		t.Fatalf("premise stale: a fixture game crossed a 1h slow threshold:\n%s", out)
	}
	if r185CappedGame.FindStringSubmatch(out) == nil {
		t.Fatalf("a fast game that met the swallow floor emitted no CAPPED "+
			"GAME line — the attribution is still gated on duration, which "+
			"is the 99.3%% sampling hole this round exists to close. Got:\n%s", out)
	}
}

// Below the floor, silence. At the production default of 1 a fixture
// game (which swallows nothing) must not be logged, or the line
// degenerates into one record per game.
func TestR185_SilentBelowTheFloor(t *testing.T) {
	out := r185Grind(t, maintenanceFixture(), time.Hour, 1)
	if strings.Contains(out, "CAPPED GAME") {
		t.Fatalf("a game that swallowed nothing was logged as capped at the "+
			"production floor of 1:\n%s", out)
	}
}

// The floor is a measured value, not a taste. If it moves, the
// distribution justifying it (see cappedGameThreshold's comment) has
// to move with it.
func TestR185_DefaultThresholdIsOne(t *testing.T) {
	if cappedGameThreshold != 1 {
		t.Fatalf("cappedGameThreshold = %d, want 1. The floor is derived "+
			"from the production survival curve hist[1:280,2:280,4:222,"+
			"8:172,...] over 13,509 games: no game swallows exactly one, so "+
			"1 and 2 log the same 280 games, and 4 silently drops 21%% of "+
			"them. Changing it means re-deriving it, not retuning it.", cappedGameThreshold)
	}
}

// Field order and shape. seed= must be immediately followed by
// decks=[...] so the line joins abortContext and SLOW GAME on the
// same literal run, with the new fields strictly after the bracket.
func TestR185_LineShapeAndFieldOrder(t *testing.T) {
	out := r185Grind(t, maintenanceFixture(), time.Hour, 0)
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "CAPPED GAME") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("no CAPPED GAME line:\n%s", out)
	}
	iSeed := strings.Index(line, "seed=")
	iDecks := strings.Index(line, "decks=")
	iSw := strings.Index(line, "swallowed=")
	iCM := strings.Index(line, "cascademax=")
	if iSeed < 0 || iDecks < 0 || iSw < 0 || iCM < 0 {
		t.Fatalf("missing one of seed=/decks=/swallowed=/cascademax=: %s", line)
	}
	if !(iSeed < iDecks && iDecks < iSw && iSw < iCM) {
		t.Fatalf("key order must stay seed= decks= swallowed= cascademax=, "+
			"matching SLOW GAME so a bracket-terminated deck parse is "+
			"unaffected; got: %s", line)
	}
	// The run abortContext renders, verbatim, inside this line.
	ref := abortContext(123, []string{"a/b", "c/d"})
	if !strings.HasPrefix(ref, "seed=123 decks=[") {
		t.Fatalf("abortContext shape changed (%q) — this test's premise is stale", ref)
	}
	if !strings.HasSuffix(strings.TrimSpace(line[iSeed:iSw]), "]") {
		t.Fatalf("the seed=/decks= run must end at the deck list's closing "+
			"bracket; got %q", line[iSeed:iSw])
	}
}

// cascademax must be present and parse. Its VALUE is pinned in
// gameengine — see the M8 note in the header for why it cannot be
// pinned here.
func TestR185_CarriesTheCascadePeak(t *testing.T) {
	out := r185Grind(t, maintenanceFixture(), time.Hour, 0)
	m := r185CappedGame.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no CAPPED GAME line:\n%s", out)
	}
	peak, err := strconv.Atoi(m[3])
	if err != nil {
		t.Fatalf("cascademax=%q does not parse: %v", m[3], err)
	}
	total, err := strconv.Atoi(m[2])
	if err != nil {
		t.Fatalf("swallowed=%q does not parse: %v", m[2], err)
	}
	if peak > total {
		t.Fatalf("cascademax=%d exceeds swallowed=%d — a single cascade "+
			"cannot swallow more than the whole game did: %s", peak, total, m[0])
	}
}

// No game, no record. A deferred log can run before the GameState
// exists (the under-sized-pool path sleeps and returns; a panic in
// deck setup unwinds through the same defer). r184 printed "n/a"
// there because its field rides a line that exists anyway; here the
// LINE ITSELF is the measurement, so emitting one would enter a
// fabricated capped game into the count this instrument produces.
func TestR185_CappedLineIsNotEmittedForANonGame(t *testing.T) {
	sm := maintenanceFixture()
	sm.deckPool = sm.deckPool[:showmatchSeats-1] // returns before gs exists
	out := r185Grind(t, sm, 0, 0)                // both floors wide open
	if strings.Contains(out, "CAPPED GAME") {
		t.Fatalf("emitted a CAPPED GAME line for a game that never "+
			"started:\n%s", out)
	}
	// Control: the SLOW GAME line DOES fire on this path (r184), so
	// the absence above is the guard working and not the defer being
	// skipped entirely.
	if !strings.Contains(out, "SLOW GAME") {
		t.Fatalf("premise stale: the under-sized-pool path no longer "+
			"reaches the deferred log at all, so this test proves "+
			"nothing:\n%s", out)
	}
}

// When a game is both slow and capped, BOTH lines are emitted and
// both carry the same seed. Collapsing them would make "capped games
// are not slow games" unmeasurable from the log, which is the finding
// that motivated this round.
func TestR185_BothLinesJoinOnTheSameRun(t *testing.T) {
	out := r185Grind(t, maintenanceFixture(), 0, 0)
	slow := regexp.MustCompile(`SLOW GAME [0-9.]+s turns=\d+ s/turn=[0-9.]+ seed=(-?\d+) decks=\[[^\]]*\]`).
		FindStringSubmatch(out)
	capped := r185CappedGame.FindStringSubmatch(out)
	if slow == nil || capped == nil {
		t.Fatalf("expected BOTH a SLOW GAME and a CAPPED GAME line with both "+
			"floors open; got:\n%s", out)
	}
	if slow[1] != capped[1] {
		t.Fatalf("the two lines report different seeds (%s vs %s) and so "+
			"cannot be joined", slow[1], capped[1])
	}
}
