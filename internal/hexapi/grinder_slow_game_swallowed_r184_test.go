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

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r184 — the SLOW GAME line now carries the per-game depth-cap swallow
// count, so a capped game can be named.
//
// r182 shipped per-game accounting but published only process-wide
// aggregates: 5,794 production games, 138 (2.4%) hit the cap, 2,936
// dispatches swallowed between them, worst game 334 — and no way to
// say which decks or which seed. The identity (seed=, decks=) was
// already on this line; the count was the missing half.
//
// Like r170 before it, the whole change is a SCOPE change: `gs :=` sat
// a hundred lines below the defer, so the deferred log could not see
// it. r170's lesson applies verbatim — these tests drive the real
// runOneGameFast rather than hand-feeding the value at the
// destination, because the only interesting failure mode is a
// re-shadowed variable that still renders a plausible-looking line.
//
// MUTATION CONTROLS (counts are tests that go RED in this package;
// the gameengine half is in depth_cap_swallow_accessor_r184_test.go):
//
//	M4  drop " swallowed=%s" from the format string (and its arg)
//	    -> 5 RED: every test in this file. (I predicted 3 and was
//	       wrong: NoGameRendersNotApplicable and
//	       AgreesWithTheProcessWideMaximum both match on the field too,
//	       so they catch its removal as well as its value.)
//	M5  render "0" instead of "n/a" on the gs == nil path
//	    -> 1 RED: NoGameRendersNotApplicable
//	M6  revert the hoist, i.e. `gs := gameengine.NewGameState(...)`
//	    -> DOES NOT COMPILE: showmatch.go:2063 "no new variables on
//	       left side of :=", because the hoisted var is already in
//	       scope at that point. This is
//	       a stronger control than a red test, and it is why r170's
//	       shadow trap cannot recur here in the same shape: r170's
//	       gameSeed is an int64 whose shadow still compiles and prints
//	       0, whereas a shadowed gs leaves a nil pointer that the
//	       compiler refuses. Recorded as a control, not a gap.
//
//	M1  (gameengine side) DepthCapSwallowedCount returns 0 always
//	    -> 0 RED in this package. CHASED, and it is a real finding, not
//	       a missing test. Every game these tests can drive swallows
//	       exactly zero dispatches: synthetic 36-Forest decks have no
//	       triggered abilities at all, so nesting never reaches 2, let
//	       alone the cap at 8. A hardcoded 0 is observationally
//	       identical to the truth here and no assertion in this package
//	       can tell them apart. What CAN tell them apart lives in two
//	       other places, and both are covered: the accessor-returns-
//	       the-live-field property (2 RED in gameengine under M1), and
//	       AgreesWithTheProcessWideMaximum below, which cross-checks the
//	       printed number against r182's independent atomic and fires
//	       the moment any driveable game swallows anything. The honest
//	       summary is that the transport is pinned end to end and the
//	       VALUE is pinned at its source, because this package cannot
//	       manufacture a swallow without reaching into the engine's hot
//	       path, which the brief forbids.

var r184SlowGame = regexp.MustCompile(
	`SLOW GAME [0-9.]+s turns=\d+ s/turn=[0-9.]+ seed=-?\d+ decks=\[[^\]]*\] swallowed=(\S+)`)

// r184Grind runs one real grinder game against sm with the slow-game
// threshold set to thresh, and returns the log output.
func r184Grind(t *testing.T, sm *Showmatch, thresh time.Duration) string {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	prevThresh := slowGameThreshold
	slowGameThreshold = thresh
	t.Cleanup(func() {
		slowGameThreshold = prevThresh
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	sm.runOneGameFast(rand.New(rand.NewSource(1)))
	return buf.String()
}

// Transport. A threshold of 0 makes every completed game slow, so the
// line must appear and must carry a swallowed= field holding an
// integer — "n/a" here would mean gs never reached the defer.
func TestR184_LineCarriesASwallowCount(t *testing.T) {
	out := r184Grind(t, maintenanceFixture(), 0)
	m := r184SlowGame.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no SLOW GAME line with a swallowed= field; got:\n%s", out)
	}
	if m[1] == "n/a" {
		t.Fatalf("swallowed=n/a on a game that actually ran — the GameState "+
			"is not reaching the deferred log (re-shadowed gs?). Line:\n%s", out)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("swallowed=%q does not parse as an integer: %v", m[1], err)
	}
	if n < 0 {
		t.Fatalf("swallowed=%d is negative", n)
	}
}

// Zero must be PRINTED, not omitted. A missing key is ambiguous
// between "swallowed nothing" and "build predates r184", and only the
// second reading corrupts an aggregate taken over a mixed log.
func TestR184_FieldIsPrintedEvenWhenZero(t *testing.T) {
	out := r184Grind(t, maintenanceFixture(), 0)
	if !strings.Contains(out, "swallowed=0") {
		t.Fatalf("a synthetic-deck game swallows nothing, so the line must "+
			"read swallowed=0 rather than omitting the field; got:\n%s", out)
	}
}

// The join with abortContext is the reason this field is appended
// rather than spliced in. Both lines must still share the literal
// "seed=N decks=[...]" run, with the new field strictly after it.
func TestR184_FieldComesAfterTheAbortContextJoin(t *testing.T) {
	out := r184Grind(t, maintenanceFixture(), 0)
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "SLOW GAME") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("no SLOW GAME line:\n%s", out)
	}
	iSeed := strings.Index(line, "seed=")
	iDecks := strings.Index(line, "decks=")
	iSw := strings.Index(line, "swallowed=")
	if iSeed < 0 || iDecks < 0 || iSw < 0 {
		t.Fatalf("missing one of seed=/decks=/swallowed=: %s", line)
	}
	if !(iSeed < iDecks && iDecks < iSw) {
		t.Fatalf("key order must stay seed= decks= swallowed=; got: %s", line)
	}
	// The shared substring abortContext renders, verbatim.
	ref := abortContext(123, []string{"a/b", "c/d"})
	if !strings.HasPrefix(ref, "seed=123 decks=[") {
		t.Fatalf("abortContext shape changed (%q) — this test's premise is stale", ref)
	}
	joinable := line[iSeed:iSw]
	if !strings.HasSuffix(strings.TrimSpace(joinable), "]") {
		t.Fatalf("the seed=/decks= run must still end at the deck list's "+
			"closing bracket, or a bracket-terminated parse breaks; got %q", joinable)
	}
}

// No game, no number. A slow line can be emitted before the GameState
// exists (the under-sized-pool path sleeps and returns; a panic during
// setup unwinds through the same defer), and printing 0 there would
// feed a fabricated zero into a count of capped games.
func TestR184_NoGameRendersNotApplicable(t *testing.T) {
	sm := maintenanceFixture()
	sm.deckPool = sm.deckPool[:showmatchSeats-1] // under-sized: returns before gs exists
	out := r184Grind(t, sm, 0)
	if !strings.Contains(out, "swallowed=n/a") {
		t.Fatalf("a slow line emitted before the GameState exists must read "+
			"swallowed=n/a, not a fabricated count; got:\n%s", out)
	}
	if strings.Contains(out, "swallowed=0") {
		t.Fatalf("printed swallowed=0 for a game that never started:\n%s", out)
	}
}

// Independent oracle. r182's depthCapMaxPerGame atomic is the largest
// per-game total seen; reset it, run exactly one game, and it must
// equal what the line printed. This is vacuous while the driveable
// decks swallow nothing (0 == 0) and becomes a real cross-check the
// instant any of them does — see the M1 note in the header.
func TestR184_AgreesWithTheProcessWideMaximum(t *testing.T) {
	gameengine.ResetDepthCapForTest()
	t.Cleanup(gameengine.ResetDepthCapForTest)

	out := r184Grind(t, maintenanceFixture(), 0)
	m := r184SlowGame.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no SLOW GAME line:\n%s", out)
	}
	printed, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("swallowed=%q does not parse: %v", m[1], err)
	}
	_, _, maxPerGame, _, _ := gameengine.DepthCapStats()
	if uint64(printed) != maxPerGame {
		t.Fatalf("line printed swallowed=%d but r182's per-game maximum over "+
			"this single game is %d — the two accountings disagree",
			printed, maxPerGame)
	}
}
