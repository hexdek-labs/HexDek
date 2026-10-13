package hexapi

import "testing"

// r76 — the grinder sized its worker pool from runtime.NumCPU(), the
// MACHINE's logical CPU count, ignoring GOMAXPROCS. Where an operator
// has throttled the process (on .206 the wrapper sets GOMAXPROCS=2 to
// keep the box usable) that oversubscribed the allowed procs 2x.
func TestGrinderWorkerCount_R76(t *testing.T) {
	cases := []struct {
		name             string
		numCPU, maxProcs int
		want             int
	}{
		// The case that was wrong in production: 8-CPU box, operator
		// throttle of 2. Was 4 workers on 2 procs.
		{"throttled prod box", 8, 2, 2},
		// Unthrottled: the original "half the machine" intent is kept.
		{"unthrottled 8-cpu", 8, 8, 4},
		{"unthrottled 32-cpu", 32, 32, 12}, // the 12 cap still applies
		{"cap applies under high procs", 64, 64, 12},
		// Floor: never fewer than 2, even on a 1-CPU box.
		{"single cpu", 1, 1, 2},
		{"two cpu", 2, 2, 2},
		// A throttle tighter than the floor still yields the floor —
		// the pool must stay functional.
		{"throttle below floor", 8, 1, 2},
		// maxProcs unknown/zero must not zero the pool.
		{"unknown maxprocs", 8, 0, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := grinderWorkerCount(c.numCPU, c.maxProcs); got != c.want {
				t.Errorf("grinderWorkerCount(numCPU=%d, maxProcs=%d) = %d, want %d",
					c.numCPU, c.maxProcs, got, c.want)
			}
		})
	}
}

// The property that actually matters: never schedule more CPU-bound
// workers than the procs we are allowed to use — except where the
// floor of 2 forces it, which is deliberate and bounded.
func TestGrinderWorkerCount_R76_NeverOversubscribesBeyondFloor(t *testing.T) {
	for numCPU := 1; numCPU <= 64; numCPU++ {
		for maxProcs := 1; maxProcs <= 64; maxProcs++ {
			got := grinderWorkerCount(numCPU, maxProcs)
			if got > maxProcs && got != 2 {
				t.Fatalf("numCPU=%d maxProcs=%d -> %d workers: oversubscribed "+
					"beyond the floor", numCPU, maxProcs, got)
			}
			if got < 2 {
				t.Fatalf("numCPU=%d maxProcs=%d -> %d workers: below floor",
					numCPU, maxProcs, got)
			}
		}
	}
}
