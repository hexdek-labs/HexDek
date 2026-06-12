package validation

import "sync"

// router.go — the LogViolation sink (consolidation step 4).
//
// Every correctness surface (InstanceID census invariants, ride-along
// legality, Feynman post-game oracle, seat-outcome checker, Loki chaos
// runs, cross-engine parity, muninn persistence) routes its violations
// through LogViolation as canonical ValidationViolation values. Sinks
// registered here observe the unified stream — this is the seam the
// engine-embedded Hex Judge attaches to.
//
// Design constraints:
//   - Zero overhead when no sink is registered (one RLock + len check).
//   - Emitting surfaces keep returning violations to their existing
//     callers — LogViolation is an observation tap, not a control-flow
//     replacement, so registering a sink can never change game behavior.
//   - Safe under concurrent games (tournament pool workers share the
//     process): registry guarded by RWMutex; sinks must be internally
//     thread-safe.

// ViolationSink observes canonical violations. Implementations must be
// fast and non-blocking — they run inline on engine/check paths.
type ViolationSink func(ValidationViolation)

var (
	sinkMu sync.RWMutex
	sinks  []ViolationSink
)

// RegisterSink adds a sink to the router and returns its unregister
// function. Multiple sinks fan out in registration order.
func RegisterSink(s ViolationSink) (unregister func()) {
	if s == nil {
		return func() {}
	}
	sinkMu.Lock()
	defer sinkMu.Unlock()
	idx := len(sinks)
	sinks = append(sinks, s)
	done := false
	return func() {
		sinkMu.Lock()
		defer sinkMu.Unlock()
		if done || idx >= len(sinks) {
			done = true
			return
		}
		// Nil out rather than splice so other unregister closures'
		// indices stay valid; LogViolation skips nil entries.
		sinks[idx] = nil
		done = true
	}
}

// LogViolation routes one canonical violation to every registered sink.
// No-op (cheap) when no sinks are registered. Never panics: a panicking
// sink is recovered so a broken observer can't take down a game.
func LogViolation(v ValidationViolation) {
	sinkMu.RLock()
	defer sinkMu.RUnlock()
	for _, s := range sinks {
		if s == nil {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			s(v)
		}()
	}
}

// LogViolations routes a batch.
func LogViolations(vs []ValidationViolation) {
	for _, v := range vs {
		LogViolation(v)
	}
}