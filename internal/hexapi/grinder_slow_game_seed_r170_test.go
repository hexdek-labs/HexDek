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

// r170 — the SLOW GAME line now carries the seed.
//
// Duration, turns and s/turn describe how bad a game was; none of them
// let anyone re-run it. Nine capped pods have been logged since r166
// and not one is replayable, which is what has blocked every repro
// attempt in the r166-r169 chain.
//
// The value already existed (gs.Seed, and abortContext has printed it
// for aborts since r131) — it was simply out of scope at the deferred
// log, because `gameSeed :=` was assigned eighty lines below the
// defer. So the whole round is a scope change, and the ONLY way it can
// break is by someone re-introducing that shadow: the line would still
// render, still look correct, and print 0 forever.
//
// That is why these tests drive the real runOneGameFast and bracket the
// printed seed between two timestamps taken around the call, instead of
// asserting on the format string. r167 shipped broken because four of
// its five tests hand-fed the value at the destination; a shadowed
// gameSeed prints 0, and 0 is not between two UnixNano readings.

var r170SlowGame = regexp.MustCompile(`SLOW GAME [0-9.]+s turns=\d+ s/turn=[0-9.]+ seed=(-?\d+) decks=`)

// r170Grind runs one real grinder game with the slow-game threshold set
// to thresh, and returns everything written to the log during it.
func r170Grind(t *testing.T, thresh time.Duration) string {
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

	sm := maintenanceFixture()
	sm.runOneGameFast(rand.New(rand.NewSource(1)))
	return buf.String()
}

// The transport test. A threshold of 0 makes every completed game slow,
// so the line must appear, and the seed it carries must be the one this
// call generated — bracketed by wall-clock readings either side.
func TestR170_SlowGameLineCarriesTheSeedTheGameActuallyUsed(t *testing.T) {
	before := time.Now().UnixNano()
	out := r170Grind(t, 0)
	after := time.Now().UnixNano()

	m := r170SlowGame.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no SLOW GAME line with a seed= field; got:\n%s", out)
	}
	seed, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		t.Fatalf("seed %q does not parse: %v", m[1], err)
	}
	if seed == 0 {
		t.Fatalf("seed printed as 0 — gameSeed is shadowed, so the line reports "+
			"a variable the game never used. Line:\n%s", out)
	}
	if seed < before || seed > after {
		t.Fatalf("seed %d is outside [%d, %d], so it is not the seed generated "+
			"during this call — the value is not being transported from the "+
			"assignment to the deferred log. Line:\n%s", seed, before, after, out)
	}
}

// The other direction: below the threshold, nothing is logged. Without
// this, a change that logged every game unconditionally would still
// pass the test above.
func TestR170_AFastGameLogsNoSlowGameLine(t *testing.T) {
	out := r170Grind(t, time.Hour)
	if strings.Contains(out, "SLOW GAME") {
		t.Fatalf("a game under the threshold must not be logged as slow; got:\n%s", out)
	}
}

// The seed is only useful if it lands in the same shape abortContext
// uses, because the two lines are joined by hand when tracing a pod.
// abortContext (game_census_r131.go:78) renders "seed=%d decks=[...]".
func TestR170_SeedFieldMatchesTheAbortContextShape(t *testing.T) {
	out := r170Grind(t, 0)
	if !strings.Contains(out, "seed=") {
		t.Fatalf("no seed= field at all:\n%s", out)
	}
	i, j := strings.Index(out, "seed="), strings.Index(out, "decks=")
	if i < 0 || j < 0 || i > j {
		t.Fatalf("seed= must precede decks=, as abortContext renders it; got:\n%s", out)
	}
	ref := abortContext(123, []string{"a/b", "c/d"})
	if !strings.HasPrefix(ref, "seed=123 decks=[") {
		t.Fatalf("abortContext shape changed (%q) — this test's premise is stale", ref)
	}
}
