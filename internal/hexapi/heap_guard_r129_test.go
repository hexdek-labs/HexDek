package hexapi

import "testing"

// r129 — the grinder heap guard.
//
// The defect was the sampling rate, so the sampling rate is what these
// pin: a guard that only looks every tenth turn cannot bound anything
// that grows faster than ten turns, and on 2026-10-15 the process went
// from ~940MB to 15216MB inside a single 30-second window — twice.
//
// Every heap abort in the log is at a turn that is a multiple of ten,
// which is the evidence that the cadence, not the threshold, was wrong.

const r129GB = 1024 * 1024 * 1024

// CATCHES THE BUG: once a game is over half its budget the guard must
// look on EVERY turn, including the nine turns in ten it used to skip.
func TestR129_EscalatesToEveryTurnOncePushingTheBudget(t *testing.T) {
	_, watch, abort := heapGuardEvaluate(1*r129GB+500*1024*1024, 0)
	if !watch {
		t.Fatal("1.5GB of a 2GB budget must escalate sampling to every turn")
	}
	if abort != "" {
		t.Fatalf("1.5GB is under the 2GB budget and must not abort, got %q", abort)
	}
	for turn := 41; turn <= 49; turn++ {
		if !heapGuardSamples(turn, watch) {
			t.Fatalf("turn %d must be sampled while watching; these are the turns the old guard skipped", turn)
		}
	}
}

// MUST NEVER COST ANYTHING: a normal game keeps the cheap 1-in-10
// cadence. ReadMemStats stops the world, so this is the reason the
// escalation is conditional.
func TestR129_NormalGameKeepsTheCheapCadence(t *testing.T) {
	_, watch, abort := heapGuardEvaluate(300*1024*1024, 0)
	if watch {
		t.Fatal("300MB is nowhere near the budget and must not escalate sampling")
	}
	if abort != "" {
		t.Fatalf("300MB must not abort, got %q", abort)
	}
	sampled := 0
	for turn := 1; turn <= 60; turn++ {
		if heapGuardSamples(turn, watch) {
			sampled++
		}
	}
	if sampled != 6 {
		t.Fatalf("an unwatched 60-turn game should sample 6 times, got %d", sampled)
	}
}

// The per-game delta still fires where it always did.
func TestR129_DeltaAbortStillFires(t *testing.T) {
	delta, _, abort := heapGuardEvaluate(3*r129GB, 500*1024*1024)
	if abort != heapAbortDelta {
		t.Fatalf("delta of %dMB over a 2GB budget must abort, got %q", delta/(1024*1024), abort)
	}
}

// The attribution-free backstop: a process past the ceiling aborts even
// when this game's own delta is small, because the delta is measured
// against a process-wide counter shared with the other worker and cannot
// tell whose memory it is.
func TestR129_ProcessCeilingFiresOnASmallDelta(t *testing.T) {
	_, _, abort := heapGuardEvaluate(7*r129GB, 7*r129GB-100*1024*1024)
	if abort != heapAbortCeiling {
		t.Fatalf("a 7GB process heap must abort on the ceiling even with a 100MB delta, got %q", abort)
	}
}

// A baseline above the current reading must not underflow the unsigned
// subtraction into a gigantic delta and abort a healthy game.
func TestR129_GCBelowBaselineDoesNotUnderflow(t *testing.T) {
	delta, watch, abort := heapGuardEvaluate(200*1024*1024, 900*1024*1024)
	if delta != 0 {
		t.Fatalf("a reading below the baseline is a zero delta, got %d", delta)
	}
	if watch || abort != "" {
		t.Fatalf("a shrinking heap must neither escalate nor abort, got watch=%v abort=%q", watch, abort)
	}
}
