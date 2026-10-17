package hexapi

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"
)

// r150 — the allocation-spike profile trap.
//
// The heap excursions are an allocation RATE event. r146's alloc= proved
// it: on 2026-10-16 the grinder churned 20,202MB in a single 30s window
// against a ~2,250MB baseline, nine times the rate, with the heap at
// 11,262MB. Bytes were being CREATED, not retained, which settles the
// fork docs/heap-rate-r146.md wrote down and leaves exactly one question:
// created BY WHAT.
//
// A profile taken afterwards cannot answer that. Go's allocs profile is
// CUMULATIVE over the process lifetime, so a 30GB burst sits inside more
// than a terabyte of ordinary steady-state allocation and rounds to
// nothing. The burst is only visible as the DIFFERENCE between a profile
// from before it and one from during it, which is what `pprof -base`
// computes — and nobody is at the keyboard when an excursion starts.
//
// So the process takes them itself: a rolling baseline refreshed every
// few minutes, and a spike profile written the moment the detector below
// fires. Freezing the rotation on a fire keeps the pre-spike baseline on
// disk instead of letting the next rotation overwrite the one that
// matters.
//
// This is instrumentation. It changes no engine behaviour and cannot be
// red in a regression that tests behaviour — see the note in the test.

const (
	// Trailing windows used for the median. Median, not mean, so that
	// the spike itself cannot drag the baseline it is compared against.
	allocSpikeHistory = 20

	// Windows of history required before the detector arms at all.
	// Without this a cold start compares against almost nothing.
	allocSpikeMinWindows = 10

	// Fire at this multiple of the trailing median. MEASURED, not
	// chosen: over the 142 windows of the 2026-10-16 run this is the
	// highest threshold that catches both excursions and the lowest
	// that never fires outside one. See the replay test.
	allocSpikeFactor = 1.5

	// Windows to stay quiet after firing, so that a 600-abort storm
	// does not write 600 profiles.
	allocSpikeCooldown = 20

	// ...unless the new window dwarfs the one last reported by this
	// much. The 2026-10-16 excursion opened at 27.3 MB/game and peaked
	// two windows later at 172.7; a plain cooldown would have spent its
	// one profile on the precursor and slept through the event.
	allocSpikeEscalate = 2.0

	// How often the rolling baseline is refreshed, in windows (30s
	// each), so the baseline on disk is never more than this old.
	//
	// r150.1 — this was 10 (five minutes) and that was too slack to be
	// useful. The first real capture, within ten minutes of deploying,
	// differenced a 4.5-minute interval in which only about 60 seconds
	// was actually spiking: the excess attributable to the spike was
	// roughly 2-3GB out of 31.5GB differenced, so the result was 90%
	// steady state and could not support a claim about the spike. At 2
	// the diff covers at most 90 seconds. A 300KB write per minute is
	// not worth protecting.
	allocBaselineEvery = 2
)

// allocSpikeDetector decides whether one window's allocation is a spike.
// It is deliberately separate from the file writing: this part is pure,
// allocation-free and testable against a real log series.
type allocSpikeDetector struct {
	hist      [allocSpikeHistory]float64
	n         int // number of observations recorded, capped at len(hist)
	next      int // ring cursor
	sinceFire int
	lastFired float64
	fired     bool
}

// observe records one window's allocation, expressed in megabytes per
// completed game, and reports whether it is a spike.
//
// Per GAME rather than per window because the per-window figure is a
// rate and throughput here swings between a 250 g/min and a 550 g/min
// regime. Over the 137 healthy windows of the 2026-10-16 run the
// per-window figure spans 1,925 to 4,919MB, a factor of 2.56, while the
// per-game figure spans 13.0 to 23.7MB, a factor of 1.82 — so a fixed
// multiple of the median means the same thing in both regimes only for
// the per-game quantity.
//
// Measured honestly, that is a robustness argument and not a
// false-positive one: replaying this same detector over the per-WINDOW
// series of that run also yields zero false positives. It reports two of
// the three windows rather than three, losing the precursor. The
// per-game choice buys the precursor and buys not having to re-tune the
// threshold when throughput shifts; it does not buy a cleaner run.
//
// It returns the trailing median and the ratio for the log line.
func (d *allocSpikeDetector) observe(mbPerGame float64) (spike bool, median, ratio float64) {
	// A window that completed no games has no per-game rate. Discarding
	// it rather than recording a zero keeps the median honest.
	if !(mbPerGame > 0) { // also rejects NaN
		return false, 0, 0
	}
	if d.sinceFire < allocSpikeCooldown {
		d.sinceFire++
	}
	if d.n < allocSpikeMinWindows {
		d.push(mbPerGame)
		return false, 0, 0
	}
	median = d.median()
	d.push(mbPerGame)
	if median <= 0 {
		return false, median, 0
	}
	ratio = mbPerGame / median
	if ratio < allocSpikeFactor {
		return false, median, ratio
	}
	// Above the threshold. Report it unless we are still inside the
	// cooldown from the last report AND this window does not dwarf it.
	if d.fired && d.sinceFire < allocSpikeCooldown && mbPerGame < allocSpikeEscalate*d.lastFired {
		return false, median, ratio
	}
	d.fired = true
	d.sinceFire = 0
	d.lastFired = mbPerGame
	return true, median, ratio
}

func (d *allocSpikeDetector) push(v float64) {
	d.hist[d.next] = v
	d.next = (d.next + 1) % len(d.hist)
	if d.n < len(d.hist) {
		d.n++
	}
}

// median sorts a stack copy of the ring with an insertion sort. The
// window is twenty elements and this runs once per thirty seconds, so the
// point is not speed — it is that the thing watching allocation must not
// allocate. The test pins that at zero.
func (d *allocSpikeDetector) median() float64 {
	var buf [allocSpikeHistory]float64
	n := d.n
	copy(buf[:n], d.hist[:n])
	for i := 1; i < n; i++ {
		v := buf[i]
		j := i - 1
		for j >= 0 && buf[j] > v {
			buf[j+1] = buf[j]
			j--
		}
		buf[j+1] = v
	}
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return buf[n/2]
	}
	return (buf[n/2-1] + buf[n/2]) / 2
}

// allocSpikeRecorder wires the detector to the filesystem: it keeps a
// recent baseline profile on disk and writes a spike profile when the
// detector fires.
type allocSpikeRecorder struct {
	det           allocSpikeDetector
	dir           string
	disabled      bool
	sinceBaseline int
	frozen        int // windows left with baseline rotation suspended

	// r154 — the previous window's raw numerator and denominator, so a
	// firing can state which side of MB/game actually moved.
	prevAllocMB float64
	prevGames   int
	havePrev    bool
}

func newAllocSpikeRecorder() *allocSpikeRecorder {
	dir := os.Getenv("HEXDEK_ALLOC_PROFILE_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "hexdek-allocprof")
	}
	r := &allocSpikeRecorder{dir: dir}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("grinder: alloc-spike trap DISABLED — cannot create %s: %v", dir, err)
		r.disabled = true
		return r
	}
	log.Printf("grinder: alloc-spike trap armed — %.1fx trailing median over %d windows, profiles in %s",
		allocSpikeFactor, allocSpikeHistory, dir)
	return r
}

// spikeCause renders which side of the MB/game ratio moved.
//
// It reports percentages and a short tag and deliberately stops there: it
// is a pointer at the next thing to read, not a diagnosis. The thresholds
// are descriptive labels on numbers that are printed in full beside them,
// so a reader who disagrees with the label still has the figures.
func spikeCause(allocMB, prevAllocMB float64, games, prevGames int, havePrev bool) string {
	if !havePrev || prevGames <= 0 || !(prevAllocMB > 0) {
		return "no prior window to compare"
	}
	dAlloc := (allocMB - prevAllocMB) / prevAllocMB * 100
	dGames := (float64(games) - float64(prevGames)) / float64(prevGames) * 100

	tag := "neither side moved much"
	switch {
	case dAlloc >= 20 && dGames <= -20:
		tag = "BURST DURING A STALL — both sides moved"
	case dAlloc >= 20:
		tag = "NUMERATOR ROSE — real allocation burst"
	case dGames <= -20:
		tag = "DENOMINATOR FELL — fewer games completed, check for a SLOW GAME"
	}
	return fmt.Sprintf("alloc %.0fMB vs %.0fMB prev (%+.0f%%), games %d vs %d prev (%+.0f%%): %s",
		allocMB, prevAllocMB, dAlloc, games, prevGames, dGames, tag)
}

func (r *allocSpikeRecorder) baselinePath() string {
	return filepath.Join(r.dir, "alloc-baseline.pb.gz")
}

// observe is called once per grinder window with that window's
// allocation in megabytes per completed game, plus the raw numerator and
// denominator that rate was computed from.
//
// r154 — the raw pair is carried purely so a firing can say which SIDE of
// the rate moved. MB/game rises either because the process allocated more
// or because fewer games completed, and those are different events with
// different diagnoses: r152's firing was a stalled worker (alloc flat,
// games halved) and r153's was a real burst (alloc doubled, gc/game
// 0.05->0.22). Working that out meant going back to the log and reading
// neighbouring windows by hand, three rounds running. The line can say it
// itself.
func (r *allocSpikeRecorder) observe(mbPerGame, allocMB float64, games int) {
	if r == nil || r.disabled {
		return
	}
	prevAllocMB, prevGames, havePrev := r.prevAllocMB, r.prevGames, r.havePrev
	if games > 0 {
		r.prevAllocMB, r.prevGames, r.havePrev = allocMB, games, true
	}
	spike, median, ratio := r.det.observe(mbPerGame)
	if r.frozen > 0 {
		r.frozen--
	}

	if spike {
		stamp := time.Now().UTC().Format("20060102T150405Z")
		spikePath := filepath.Join(r.dir, "alloc-spike-"+stamp+".pb.gz")
		// A log line's own text is a claim. This one previously said
		// "none yet" in both the no-baseline case and the already-frozen
		// case, which are different situations and send a reader to
		// different files.
		base := "none yet"
		if r.frozen > 0 {
			base = "already frozen by the preceding spike — use that one"
		}
		// Freeze the rotation and move the pre-spike baseline out of the
		// way of it before writing anything, so that even if the spike
		// write fails the baseline survives.
		if r.frozen == 0 {
			frozen := filepath.Join(r.dir, "alloc-baseline-frozen-"+stamp+".pb.gz")
			if err := os.Rename(r.baselinePath(), frozen); err == nil {
				base = frozen
			}
		}
		r.frozen = allocSpikeCooldown
		why := spikeCause(allocMB, prevAllocMB, games, prevGames, havePrev)
		if err := writeAllocProfile(spikePath); err != nil {
			log.Printf("grinder: ALLOC SPIKE %.1f MB/game vs median %.1f (%.2fx) [%s] — profile write FAILED: %v",
				mbPerGame, median, ratio, why, err)
			return
		}
		log.Printf("grinder: ALLOC SPIKE %.1f MB/game vs median %.1f (%.2fx) [%s] — spike=%s baseline=%s",
			mbPerGame, median, ratio, why, spikePath, base)
		return
	}

	r.sinceBaseline++
	if r.frozen == 0 && r.sinceBaseline >= allocBaselineEvery {
		r.sinceBaseline = 0
		if err := writeAllocProfile(r.baselinePath()); err != nil {
			log.Printf("grinder: alloc baseline write failed: %v", err)
		}
	}
}

// writeAllocProfile writes the cumulative allocs profile. debug=0 emits
// the gzipped protobuf `go tool pprof` reads directly. It does not force
// a GC, so the cost is proportional to the number of distinct allocation
// sites rather than to the live heap.
func writeAllocProfile(path string) error {
	p := pprof.Lookup("allocs")
	if p == nil {
		return fmt.Errorf("no allocs profile registered")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := p.WriteTo(f, 0); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
