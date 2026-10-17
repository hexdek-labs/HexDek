package hexapi

import (
	"strings"
	"testing"
)

// The two real firings this lane has had, with the numbers straight out of
// .206's server.log. They are the whole point of r154: the same detector
// fired on both, and they mean opposite things.
//
//	r152  21:23:17  alloc=3417MB  140 games
//	      21:23:47  alloc=3237MB  106 games   <- fired 1.81x
//	      a 506s game stalled one of two workers; allocation FELL
//
//	r153  22:28:22  alloc=3580MB  222 games
//	      22:28:52  alloc=4868MB  165 games   <- fired 1.85x
//	      22:29:22  alloc=7239MB  108 games   <- fired 4.20x
//	      a genuine burst; allocation nearly doubled, gc/game 0.05->0.22

func TestSpikeCauseR154_ReplaysTheR152StalledWorker(t *testing.T) {
	got := spikeCause(3237, 3417, 106, 140, true)
	if !strings.Contains(got, "DENOMINATOR FELL") {
		t.Fatalf("r152's stalled worker should be labelled DENOMINATOR FELL, got: %s", got)
	}
	if strings.Contains(got, "NUMERATOR ROSE") {
		t.Fatalf("allocation FELL in this window; must not claim the numerator rose: %s", got)
	}
	if !strings.Contains(got, "SLOW GAME") {
		t.Fatalf("a denominator drop should point the reader at the SLOW GAME line: %s", got)
	}
}

func TestSpikeCauseR154_ReplaysTheR153RealBurst(t *testing.T) {
	// Both firings of the real burst moved BOTH sides — the burst slowed
	// the games down as well as allocating more. The label must say so
	// rather than picking one.
	for _, c := range []struct {
		alloc, prevAlloc float64
		g, prevG         int
	}{
		{4868, 3580, 165, 222},
		{7239, 4868, 108, 165},
	} {
		got := spikeCause(c.alloc, c.prevAlloc, c.g, c.prevG, true)
		if !strings.Contains(got, "BURST DURING A STALL") {
			t.Fatalf("real burst %v should be labelled a burst, got: %s", c, got)
		}
		if strings.Contains(got, "DENOMINATOR FELL — fewer") {
			t.Fatalf("must not report this as a mere throughput dip: %s", got)
		}
	}
}

// The raw figures must appear in full, so a reader who disagrees with the
// label can still do the arithmetic.
func TestSpikeCauseR154_PrintsTheRawFiguresNotJustTheLabel(t *testing.T) {
	got := spikeCause(7239, 4868, 108, 165, true)
	for _, want := range []string{"7239MB", "4868MB", "108", "165", "%"} {
		if !strings.Contains(got, want) {
			t.Fatalf("raw figure %q missing from %q", want, got)
		}
	}
}

func TestSpikeCauseR154_NoPriorWindowSaysSo(t *testing.T) {
	if got := spikeCause(5000, 0, 100, 0, false); !strings.Contains(got, "no prior window") {
		t.Fatalf("want a no-prior-window notice, got: %s", got)
	}
	// A prior window that completed zero games cannot be a denominator.
	if got := spikeCause(5000, 3000, 100, 0, true); !strings.Contains(got, "no prior window") {
		t.Fatalf("zero-game prior must not be divided by, got: %s", got)
	}
}

func TestSpikeCauseR154_SteadyWindowIsNotLabelledEitherWay(t *testing.T) {
	got := spikeCause(3400, 3350, 205, 210, true)
	if !strings.Contains(got, "neither side moved much") {
		t.Fatalf("a steady pair should not be labelled a burst or a stall, got: %s", got)
	}
}

// CONTROL — a cause function that only looks at allocation, which is the
// obvious simpler design. It must MISLABEL r152's stalled worker, because
// that is exactly the case the games term exists to catch.
func r154AllocOnlyCause(allocMB, prevAllocMB float64) string {
	if !(prevAllocMB > 0) {
		return "no prior window to compare"
	}
	if (allocMB-prevAllocMB)/prevAllocMB*100 >= 20 {
		return "NUMERATOR ROSE — real allocation burst"
	}
	return "neither side moved much"
}

func TestSpikeCauseR154_ControlAllocOnlyMisreadsTheStall(t *testing.T) {
	ctl := r154AllocOnlyCause(3237, 3417)
	if strings.Contains(ctl, "DENOMINATOR") {
		t.Fatalf("CONTROL DID NOT GO RED: an alloc-only cause somehow identified the stall: %s", ctl)
	}
	real := spikeCause(3237, 3417, 106, 140, true)
	if strings.Contains(real, "neither side moved much") {
		t.Fatalf("the real cause function fell back to the alloc-only answer: %s", real)
	}
	t.Logf("alloc-only says %q; r154 says %q", ctl, real)
}
