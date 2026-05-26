package main

// tracer.go — structured per-card trace diagnostics for Thor.
//
// Builds on top of trace.go's low-level Tracer with higher-level types:
//
//   - TraceEntry: a single step with structured Phase/Detail/StateHash
//   - CardTrace:  all entries for one card × interaction, plus outcome
//   - TraceCollector: thread-safe accumulator that writes .trace files
//
// The collector writes human-readable indented text (NOT JSON) to
// data/traces/ (or --trace-dir). When --trace-failures-only is set,
// only failing tests produce trace files.
//
// Integration point: testCard() in main.go can wrap its execution with
// a CardTraceRecorder to capture structured steps. The recorder's API
// is designed to be called at each stage of the test pipeline:
//
//     rec := collector.Begin("Sol Ring", "destroy")
//     rec.Setup("2-seat game, Sol Ring on seat-0 battlefield")
//     rec.Snapshot("life=40/40, bf=1/0")
//     rec.ConditionCheck("indestructible → false")
//     rec.HandlerEntry("DestroyPermanent")
//     rec.EffectDispatch("move battlefield→graveyard")
//     rec.Resolution("Sol Ring moved to graveyard")
//     rec.SnapshotDiff("bf=0/0, gy=1/0")
//     rec.Complete(OutcomePass, "")

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// Phase enum — the stage of test execution a trace entry belongs to.
// ---------------------------------------------------------------------------

// TracePhase categorises where in the test pipeline an entry was recorded.
type TracePhase string

const (
	PhaseSetup          TracePhase = "setup"
	PhaseConditionCheck TracePhase = "condition_check"
	PhaseHandlerEntry   TracePhase = "handler_entry"
	PhaseEffectDispatch TracePhase = "effect_dispatch"
	PhaseResolution     TracePhase = "resolution"
	PhaseSnapshotDiff   TracePhase = "snapshot_diff"
	PhaseAssert         TracePhase = "assert"
	PhasePanic          TracePhase = "panic"
)

// ---------------------------------------------------------------------------
// Outcome — the final result of a card × interaction test.
// ---------------------------------------------------------------------------

type TraceOutcome string

const (
	OutcomePass          TraceOutcome = "pass"
	OutcomeFailNoChange  TraceOutcome = "fail_no_change"
	OutcomeFailPanic     TraceOutcome = "fail_panic"
	OutcomeFailInvariant TraceOutcome = "fail_invariant"
)

// ---------------------------------------------------------------------------
// TraceEntry — one step in the execution trace.
// ---------------------------------------------------------------------------

// TraceEntry records a single diagnostic step during a card test.
type TraceEntry struct {
	Step      int        // 1-based step number within the trace
	Phase     TracePhase // which pipeline stage
	Card      string     // card name (usually the card under test)
	Detail    string     // human-readable description of what happened
	StateHash string     // optional: hex digest or summary of game state
	Timestamp time.Time  // when the entry was recorded
}

// String renders a trace entry as a single indented line.
func (e TraceEntry) String() string {
	ts := ""
	if !e.Timestamp.IsZero() {
		ts = fmt.Sprintf(" @%s", e.Timestamp.Format("15:04:05.000"))
	}
	hash := ""
	if e.StateHash != "" {
		hash = fmt.Sprintf(" [%s]", e.StateHash)
	}
	return fmt.Sprintf("[%d] %s: %s%s%s", e.Step, strings.ToUpper(string(e.Phase)), e.Detail, hash, ts)
}

// ---------------------------------------------------------------------------
// CardTrace — the full execution trace for one card × interaction.
// ---------------------------------------------------------------------------

// CardTrace captures the complete diagnostic record for a single test:
// which card, which interaction, every step, and the final outcome.
type CardTrace struct {
	CardName     string       // e.g. "Sol Ring"
	Interaction  string       // e.g. "destroy", "phase_untap", "goldilocks"
	Entries      []TraceEntry // ordered steps
	Outcome      TraceOutcome // final result
	FailurePoint string       // human-readable description of where it broke (empty on pass)
	Duration     time.Duration
}

// AddEntry appends a new trace entry with auto-incrementing step number.
func (ct *CardTrace) AddEntry(phase TracePhase, detail string) {
	ct.Entries = append(ct.Entries, TraceEntry{
		Step:      len(ct.Entries) + 1,
		Phase:     phase,
		Card:      ct.CardName,
		Detail:    detail,
		Timestamp: time.Now(),
	})
}

// AddEntryWithHash appends a trace entry that includes a state hash/summary.
func (ct *CardTrace) AddEntryWithHash(phase TracePhase, detail, stateHash string) {
	ct.Entries = append(ct.Entries, TraceEntry{
		Step:      len(ct.Entries) + 1,
		Phase:     phase,
		Card:      ct.CardName,
		Detail:    detail,
		StateHash: stateHash,
		Timestamp: time.Now(),
	})
}

// Failed returns true if the outcome is any failure type.
func (ct *CardTrace) Failed() bool {
	return ct.Outcome != OutcomePass
}

// Render produces the human-readable trace text for writing to a .trace file.
func (ct *CardTrace) Render() string {
	var b strings.Builder

	// Header line.
	b.WriteString(fmt.Sprintf("=== %s × %s ===\n", ct.CardName, ct.Interaction))

	// All entries.
	for _, e := range ct.Entries {
		b.WriteString(e.String())
		b.WriteByte('\n')
	}

	// Outcome footer.
	b.WriteString(fmt.Sprintf("[*] OUTCOME: %s", strings.ToUpper(string(ct.Outcome))))
	if ct.FailurePoint != "" {
		b.WriteString(fmt.Sprintf(" — %s", ct.FailurePoint))
	}
	if ct.Duration > 0 {
		b.WriteString(fmt.Sprintf(" (%s)", ct.Duration.Round(time.Microsecond)))
	}
	b.WriteByte('\n')

	return b.String()
}

// ---------------------------------------------------------------------------
// CardTraceRecorder — convenience wrapper for building a CardTrace step-by-step.
// ---------------------------------------------------------------------------

// CardTraceRecorder provides typed methods for each pipeline phase, making
// call sites in testCard() clean and self-documenting.
type CardTraceRecorder struct {
	trace *CardTrace
	start time.Time
}

// newCardTraceRecorder creates a recorder for the given card and interaction.
// The caller (TraceCollector.Begin) is responsible for the nil-collector
// guard; this constructor always returns a non-nil *CardTraceRecorder.
func newCardTraceRecorder(cardName, interaction string) *CardTraceRecorder {
	return &CardTraceRecorder{
		trace: &CardTrace{
			CardName:    cardName,
			Interaction: interaction,
			Entries:     make([]TraceEntry, 0, 16),
		},
		start: time.Now(),
	}
}

// Setup records a setup-phase entry.
func (r *CardTraceRecorder) Setup(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhaseSetup, detail)
}

// Snapshot records the pre-execution snapshot state.
func (r *CardTraceRecorder) Snapshot(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhaseSetup, fmt.Sprintf("snapshot taken (%s)", detail))
}

// SnapshotWithHash records a snapshot with a state hash.
func (r *CardTraceRecorder) SnapshotWithHash(detail, hash string) {
	if r == nil {
		return
	}
	r.trace.AddEntryWithHash(PhaseSetup, fmt.Sprintf("snapshot taken (%s)", detail), hash)
}

// ConditionCheck records a condition evaluation step.
func (r *CardTraceRecorder) ConditionCheck(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhaseConditionCheck, detail)
}

// HandlerEntry records entry into a specific handler function.
func (r *CardTraceRecorder) HandlerEntry(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhaseHandlerEntry, detail)
}

// EffectDispatch records an effect being dispatched to the resolver.
func (r *CardTraceRecorder) EffectDispatch(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhaseEffectDispatch, detail)
}

// Resolution records the result of effect resolution.
func (r *CardTraceRecorder) Resolution(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhaseResolution, detail)
}

// SnapshotDiff records the post-execution state comparison.
func (r *CardTraceRecorder) SnapshotDiff(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhaseSnapshotDiff, detail)
}

// Assert records an assertion check (pass or fail).
func (r *CardTraceRecorder) Assert(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhaseAssert, detail)
}

// Panic records a panic recovery.
func (r *CardTraceRecorder) Panic(detail string) {
	if r == nil {
		return
	}
	r.trace.AddEntry(PhasePanic, detail)
}

// Complete finalises the trace with an outcome and optional failure description.
func (r *CardTraceRecorder) Complete(outcome TraceOutcome, failurePoint string) {
	if r == nil {
		return
	}
	r.trace.Outcome = outcome
	r.trace.FailurePoint = failurePoint
	r.trace.Duration = time.Since(r.start)
}

// Trace returns the completed CardTrace. Returns nil if the recorder is nil.
func (r *CardTraceRecorder) Trace() *CardTrace {
	if r == nil {
		return nil
	}
	return r.trace
}

// ---------------------------------------------------------------------------
// TraceCollector — thread-safe accumulator of CardTrace records.
// ---------------------------------------------------------------------------

// TraceCollector accumulates per-card traces and writes them to disk.
// It is safe for concurrent use by multiple Thor worker goroutines.
type TraceCollector struct {
	mu           sync.Mutex
	traces       []*CardTrace
	dir          string // output directory for .trace files
	failuresOnly bool   // only write traces for failing tests
	written      int64  // count of trace files written (atomic)
	skipped      int64  // count of traces skipped due to failuresOnly (atomic)
}

// NewTraceCollector creates a collector that writes to the given directory.
// If failuresOnly is true, passing tests are silently discarded.
// Returns nil if dir is empty (tracing disabled).
func NewTraceCollector(dir string, failuresOnly bool) *TraceCollector {
	if dir == "" {
		return nil
	}
	return &TraceCollector{
		dir:          dir,
		failuresOnly: failuresOnly,
		traces:       make([]*CardTrace, 0, 256),
	}
}

// Begin creates a new CardTraceRecorder for a test about to run.
// Returns nil if the collector is nil, so callers can safely chain.
func (tc *TraceCollector) Begin(cardName, interaction string) *CardTraceRecorder {
	if tc == nil {
		return nil
	}
	return newCardTraceRecorder(cardName, interaction)
}

// Collect accepts a completed trace. If failuresOnly is set and the trace
// passed, it is counted but not stored or written. Thread-safe.
func (tc *TraceCollector) Collect(ct *CardTrace) {
	if tc == nil || ct == nil {
		return
	}

	if tc.failuresOnly && !ct.Failed() {
		atomic.AddInt64(&tc.skipped, 1)
		return
	}

	tc.mu.Lock()
	tc.traces = append(tc.traces, ct)
	tc.mu.Unlock()

	// Write immediately so we get partial output even if Thor crashes.
	tc.writeTrace(ct)
}

// writeTrace writes a single CardTrace to a .trace file.
func (tc *TraceCollector) writeTrace(ct *CardTrace) {
	seq := atomic.AddInt64(&tc.written, 1)
	name := fmt.Sprintf("%s.%s.%06d.trace",
		slugify(ct.CardName),
		slugify(ct.Interaction),
		seq)
	path := filepath.Join(tc.dir, name)

	body := ct.Render()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "thor: trace write failed for %s: %v\n", path, err)
	}
}

// Stats returns (total collected, total written, total skipped).
func (tc *TraceCollector) Stats() (collected, written, skipped int64) {
	if tc == nil {
		return 0, 0, 0
	}
	tc.mu.Lock()
	collected = int64(len(tc.traces))
	tc.mu.Unlock()
	written = atomic.LoadInt64(&tc.written)
	skipped = atomic.LoadInt64(&tc.skipped)
	return
}

// WriteSummary writes a summary file listing all collected traces with outcomes.
func (tc *TraceCollector) WriteSummary() error {
	if tc == nil {
		return nil
	}

	tc.mu.Lock()
	traces := make([]*CardTrace, len(tc.traces))
	copy(traces, tc.traces)
	tc.mu.Unlock()

	if len(traces) == 0 {
		return nil
	}

	path := filepath.Join(tc.dir, "SUMMARY.trace")
	var b strings.Builder

	b.WriteString("=== THOR TRACE SUMMARY ===\n\n")

	// Count by outcome.
	counts := map[TraceOutcome]int{}
	for _, ct := range traces {
		counts[ct.Outcome]++
	}
	b.WriteString(fmt.Sprintf("Total traces: %d\n", len(traces)))
	for _, outcome := range []TraceOutcome{OutcomePass, OutcomeFailNoChange, OutcomeFailPanic, OutcomeFailInvariant} {
		if c := counts[outcome]; c > 0 {
			b.WriteString(fmt.Sprintf("  %-20s %d\n", string(outcome)+":", c))
		}
	}
	skipped := atomic.LoadInt64(&tc.skipped)
	if skipped > 0 {
		b.WriteString(fmt.Sprintf("  %-20s %d\n", "skipped (pass):", skipped))
	}
	b.WriteByte('\n')

	// List each trace.
	for i, ct := range traces {
		status := string(ct.Outcome)
		if ct.FailurePoint != "" {
			status += " — " + ct.FailurePoint
		}
		b.WriteString(fmt.Sprintf("%4d. %-40s × %-20s → %s\n", i+1, ct.CardName, ct.Interaction, status))
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ---------------------------------------------------------------------------
// Helpers for building snapshot summaries (used in trace detail strings).
// ---------------------------------------------------------------------------

// FormatSnapshotSummary creates a compact summary string from a Goldilocks
// snapshot, suitable for trace entry detail fields.
func FormatSnapshotSummary(snap goldilocksSnapshot, seats int) string {
	if seats <= 0 || seats > 4 {
		seats = 2
	}
	parts := make([]string, 0, 4)
	// Life totals.
	lifeParts := make([]string, seats)
	for i := 0; i < seats; i++ {
		lifeParts[i] = fmt.Sprintf("%d", snap.life[i])
	}
	parts = append(parts, "life="+strings.Join(lifeParts, "/"))

	// Battlefield counts.
	bfParts := make([]string, seats)
	for i := 0; i < seats; i++ {
		bfParts[i] = fmt.Sprintf("%d", snap.battlefieldCnt[i])
	}
	parts = append(parts, "bf="+strings.Join(bfParts, "/"))

	// Hand sizes.
	handParts := make([]string, seats)
	for i := 0; i < seats; i++ {
		handParts[i] = fmt.Sprintf("%d", snap.handSize[i])
	}
	parts = append(parts, "hand="+strings.Join(handParts, "/"))

	// Graveyard sizes.
	gyParts := make([]string, seats)
	for i := 0; i < seats; i++ {
		gyParts[i] = fmt.Sprintf("%d", snap.graveyardSize[i])
	}
	parts = append(parts, "gy="+strings.Join(gyParts, "/"))

	return strings.Join(parts, ", ")
}

// FormatSnapshotDiffSummary returns a string describing what changed between
// two snapshots. Only includes fields that actually differ.
func FormatSnapshotDiffSummary(before, after goldilocksSnapshot, seats int) string {
	if seats <= 0 || seats > 4 {
		seats = 2
	}
	var diffs []string

	for i := 0; i < seats; i++ {
		if before.life[i] != after.life[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.life %d→%d", i, before.life[i], after.life[i]))
		}
		if before.battlefieldCnt[i] != after.battlefieldCnt[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.bf %d→%d", i, before.battlefieldCnt[i], after.battlefieldCnt[i]))
		}
		if before.handSize[i] != after.handSize[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.hand %d→%d", i, before.handSize[i], after.handSize[i]))
		}
		if before.graveyardSize[i] != after.graveyardSize[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.gy %d→%d", i, before.graveyardSize[i], after.graveyardSize[i]))
		}
		if before.exileSize[i] != after.exileSize[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.exile %d→%d", i, before.exileSize[i], after.exileSize[i]))
		}
		if before.libSize[i] != after.libSize[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.lib %d→%d", i, before.libSize[i], after.libSize[i]))
		}
		if before.manaPool[i] != after.manaPool[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.mana %d→%d", i, before.manaPool[i], after.manaPool[i]))
		}
		if before.poisonCounters[i] != after.poisonCounters[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.poison %d→%d", i, before.poisonCounters[i], after.poisonCounters[i]))
		}
		if before.energy[i] != after.energy[i] {
			diffs = append(diffs, fmt.Sprintf("seat%d.energy %d→%d", i, before.energy[i], after.energy[i]))
		}
	}

	if before.stackSize != after.stackSize {
		diffs = append(diffs, fmt.Sprintf("stack %d→%d", before.stackSize, after.stackSize))
	}

	if len(diffs) == 0 {
		return "no change"
	}
	return strings.Join(diffs, ", ")
}
