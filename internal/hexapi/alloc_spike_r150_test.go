package hexapi

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// r150 — regressions for the allocation-spike trap.
//
// NOTE ON WHAT THESE CAN AND CANNOT PROVE: the trap is pure
// instrumentation. It changes no engine behaviour, so no test here can
// go red for a rules reason — reverting the trap does not break a game,
// it only blinds the operator. What IS testable, and what these pin, is
// the detector's judgement: that on the real run it fires inside the
// excursions and nowhere else.

// r150Series is the measured allocation of every window of the
// 2026-10-16 run on .206, in megabytes per completed game, derived from
// server.log by dividing each window's alloc= delta by its games delta.
// Windows 9-10 are the 17:35 excursion; 101, 103 and 104 are the
// 18:21-18:22 one, which peaked at 172.67 MB/game with the heap at
// 17,445MB. Everything else is a healthy window.
var r150Series = []float64{
	17.26, 15.99, 16.10, 17.96, 16.44, 16.92, 16.11, 16.07, 17.19, 24.62, 32.90, 17.89,
	16.03, 16.09, 16.23, 16.59, 16.41, 17.16, 16.49, 16.28, 17.15, 17.28, 16.54, 16.38,
	17.69, 16.48, 16.34, 15.93, 16.05, 16.98, 16.21, 16.71, 15.37, 16.20, 16.65, 16.26,
	16.34, 17.14, 19.60, 15.60, 16.34, 16.66, 16.55, 16.44, 16.60, 16.08, 16.59, 16.45,
	17.25, 16.74, 15.40, 17.11, 16.74, 19.23, 19.61, 17.96, 18.94, 18.24, 18.63, 17.99,
	19.07, 18.49, 18.27, 17.78, 16.89, 15.28, 18.99, 17.14, 18.63, 19.74, 19.44, 18.41,
	19.48, 18.89, 18.71, 20.65, 23.74, 21.25, 17.34, 17.93, 18.79, 16.80, 20.35, 17.29,
	17.83, 16.46, 18.24, 18.52, 17.15, 17.55, 18.59, 17.51, 19.16, 17.82, 18.65, 18.84,
	17.30, 20.02, 18.36, 17.98, 18.16, 27.27, 17.04, 172.67, 25.10, 13.04, 15.81, 16.83,
	15.73, 17.24, 16.55, 16.40, 16.64, 16.58, 18.97, 16.63, 17.06, 17.51, 17.15, 17.62,
	16.37, 18.61, 16.22, 16.73, 16.62, 17.32, 16.45, 16.16, 16.91, 20.60, 16.73, 16.69,
	16.11, 16.00, 15.94, 16.08, 15.65, 16.42, 17.05, 17.79, 16.75, 17.37,
}

// r150Excursion is the set of indices in r150Series that fall inside a
// known excursion. A firing anywhere else is a false positive.
var r150Excursion = map[int]bool{9: true, 10: true, 101: true, 103: true, 104: true}

// r150Replay runs the detector over the real series and returns the
// indices at which it fired.
func r150Replay(t *testing.T, series []float64) []int {
	t.Helper()
	var d allocSpikeDetector
	var fired []int
	for i, v := range series {
		if spike, _, _ := d.observe(v); spike {
			fired = append(fired, i)
		}
	}
	return fired
}

func TestAllocSpikeR150_ReplaysTheRealRun_FiresOnlyInsideExcursions(t *testing.T) {
	fired := r150Replay(t, r150Series)
	if len(fired) == 0 {
		t.Fatalf("detector never fired on a run containing two excursions")
	}
	for _, i := range fired {
		if !r150Excursion[i] {
			t.Errorf("FALSE POSITIVE at window %d (%.2f MB/game) — healthy window", i, r150Series[i])
		}
	}
	// Both excursions must be caught, not just the big one.
	first, second := false, false
	for _, i := range fired {
		if i <= 10 {
			first = true
		}
		if i >= 101 {
			second = true
		}
	}
	if !first {
		t.Errorf("missed the 17:35 excursion entirely; fired at %v", fired)
	}
	if !second {
		t.Errorf("missed the 18:21 excursion entirely; fired at %v", fired)
	}
	t.Logf("fired at %v of %d windows, 0 false positives", fired, len(r150Series))
}

// The peak window is the one whose profile is worth having. A design
// that spends its one profile on the precursor and sleeps through the
// peak is useless, so pin that the 172.67 window is reported.
func TestAllocSpikeR150_ReportsThePeakNotJustThePrecursor(t *testing.T) {
	fired := r150Replay(t, r150Series)
	for _, i := range fired {
		if i == 103 {
			return
		}
	}
	t.Fatalf("did not report the peak window 103 (172.67 MB/game); fired at %v", fired)
}

func TestAllocSpikeR150_CooldownSuppressesAStorm(t *testing.T) {
	var d allocSpikeDetector
	for i := 0; i < allocSpikeMinWindows; i++ {
		d.observe(17)
	}
	if spike, _, _ := d.observe(40); !spike {
		t.Fatalf("precondition: 40 against a median of 17 should fire")
	}
	// A storm of further elevated windows, none of them dwarfing the
	// first, must stay quiet.
	for i := 0; i < allocSpikeCooldown-1; i++ {
		if spike, _, _ := d.observe(45); spike {
			t.Fatalf("fired again %d windows into the cooldown", i+1)
		}
	}
}

func TestAllocSpikeR150_EscalationBeatsTheCooldown(t *testing.T) {
	var d allocSpikeDetector
	for i := 0; i < allocSpikeMinWindows; i++ {
		d.observe(17)
	}
	if spike, _, _ := d.observe(27); !spike {
		t.Fatalf("precondition: the precursor should fire")
	}
	if spike, _, _ := d.observe(20); spike {
		t.Fatalf("a quiet window inside the cooldown must not fire")
	}
	if spike, _, _ := d.observe(173); !spike {
		t.Fatalf("a window dwarfing the last report must fire despite the cooldown")
	}
}

func TestAllocSpikeR150_NotArmedBeforeMinWindows(t *testing.T) {
	var d allocSpikeDetector
	for i := 0; i < allocSpikeMinWindows-1; i++ {
		if spike, _, _ := d.observe(10000); spike {
			t.Fatalf("fired at window %d, before %d windows of history exist", i, allocSpikeMinWindows)
		}
	}
}

// A window that completed no games has no per-game rate, and recording a
// zero for it would drag the median down and manufacture a spike on the
// next ordinary window.
func TestAllocSpikeR150_EmptyWindowsDoNotPoisonTheMedian(t *testing.T) {
	var d allocSpikeDetector
	for i := 0; i < allocSpikeMinWindows; i++ {
		d.observe(17)
	}
	// Interleave enough empty windows to fill half the ring. Flooding it
	// entirely would be the easy case — the median would reach zero and
	// be rejected outright. Half is the case that bites: recorded as
	// zeros they would halve the median, and the next perfectly ordinary
	// window would read as a spike against it.
	// Assert on EVERY window in the interleave, not just the one after
	// it. If the zeros are recorded, the median halves and the spike is
	// manufactured on one of the perfectly ordinary 17s partway through
	// — which then consumes the cooldown and leaves a check made only at
	// the end reading clean. That is the shape this test originally
	// missed.
	for i := 0; i < allocSpikeHistory/2; i++ {
		if spike, median, ratio := d.observe(0); spike {
			t.Fatalf("an empty window fired at i=%d (median %.2f, ratio %.2f)", i, median, ratio)
		}
		if spike, median, ratio := d.observe(17); spike {
			t.Fatalf("an ordinary 17 MB/game window fired at i=%d — the median was dragged to %.2f by empty windows (ratio %.2f)", i, median, ratio)
		}
	}
	if spike, median, ratio := d.observe(18); spike {
		t.Fatalf("an ordinary 18 MB/game window fired after empty ones; median %.2f, ratio %.2f", median, ratio)
	}
}

// The thing watching allocation must not allocate.
func TestAllocSpikeR150_ObserveDoesNotAllocate(t *testing.T) {
	var d allocSpikeDetector
	for i := 0; i < allocSpikeHistory; i++ {
		d.observe(17)
	}
	n := testing.AllocsPerRun(200, func() { d.observe(17.5) })
	if n != 0 {
		t.Fatalf("observe allocates %.0f times per call, want 0", n)
	}
}

func TestAllocSpikeR150_WritesAProfilePprofCanRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pb.gz")
	if err := writeAllocProfile(path); err != nil {
		t.Fatalf("writeAllocProfile: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(b) < 64 {
		t.Fatalf("profile is %d bytes, implausibly small", len(b))
	}
	if !bytes.HasPrefix(b, []byte{0x1f, 0x8b}) {
		t.Fatalf("profile is not gzipped; first bytes %x", b[:4])
	}
}
