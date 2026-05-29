package counters

// Counter DB Phase 6 — Doubling pipeline + §616 replacement-effect integration
// per docs/counter-db-implementation-plan-r60.md Section 6.
//
// CR §122.1g: "If an effect would put a counter on a permanent or player,
// and another effect would put twice (or any multiple of) that many of
// those counters there instead, the resulting effect puts twice (or any
// multiple of) that many counters there." Doubling Season, Hardened Scales,
// Primal Vigor, Branching Evolution, Conclave Mentor, Ozolith / Shattered
// Spire, and Vorinclex Monstrous Raider all match this shape; some
// asymmetrically (Vorinclex halves opponents' placements, not the
// controller's).
//
// Per CR §616.1 ("Interaction of Replacement and Prevention Effects"), when
// multiple replacement effects could apply to the same event, the controller
// of the AFFECTED OBJECT (the player who would put the counter) chooses
// application order one effect at a time. This file provides the pure,
// gameengine-independent pipeline; the engine-side bridge lives in
// internal/gameengine/counter_doublers.go and registers each card's
// ReplacementEffect into gs.Replacements while populating the audit-surface
// Permanent.ProvidesReplacements field defined by Phase 5.
//
// Energy / poison / experience / rad are EXCLUDED from doubling — energy by
// CR §106.11 (resource pool, not a counter), the player-counter family by
// the registry's DoublingApplies=false flag. Lore / time are similarly
// gated by DoublingApplies=false.

// DoublerKind classifies a doubler's mutation for the audit trail. Distinct
// from gameengine.ReplacementOp so the counters package stays gameengine-
// independent; the engine-side bridge maps Kind → ReplacementOp at log time.
type DoublerKind int

const (
	// DoublerKindDouble multiplies inbound counts by 2 (Doubling Season,
	// Primal Vigor, Branching Evolution, Ozolith Shattered Spire, the
	// controller-side arm of Vorinclex Monstrous Raider).
	DoublerKindDouble DoublerKind = iota
	// DoublerKindAddOne adds 1 extra counter per placement event
	// (Hardened Scales, Conclave Mentor). Per CR §122.1g each Hardened
	// Scales instance adds +1 once independently — two Hardened Scales
	// applied in sequence produce +1, +1 (total +2), not +2 in a single
	// step. The pipeline enforces this by re-evaluating Applies + Apply
	// for each doubler in turn.
	DoublerKindAddOne
	// DoublerKindHalve halves inbound counts, rounded down (the
	// opponent-side arm of Vorinclex Monstrous Raider).
	DoublerKindHalve
)

// String renders DoublerKind for logs and invariant errors.
func (k DoublerKind) String() string {
	switch k {
	case DoublerKindAddOne:
		return "AddOne"
	case DoublerKindHalve:
		return "Halve"
	}
	return "Double"
}

// Doubler is the abstract handle for one counter-doubling replacement
// effect. Implementations live in the gameengine package; the counters
// package only needs Applies + Apply + the §616 ordering metadata.
//
// The interface is intentionally narrow — no GameState dependency —
// because the counters package is the package-cycle root for counter-aware
// code and cannot import gameengine. The engine-side bridge constructs
// concrete Doubler structs from gs.Replacements entries.
type Doubler interface {
	// Name returns a human-readable label (e.g. "Doubling Season").
	Name() string
	// SourceInstanceID returns the InstanceID of the providing Permanent.
	// Empty string is permitted for synthetic doublers used in tests; the
	// pipeline does not gate on it.
	SourceInstanceID() string
	// HandlerID is the gs.Replacements key for the underlying effect; the
	// engine-side bridge uses this to keep ReplacementRef.HandlerID and
	// DoublingApplication.HandlerID in sync.
	HandlerID() string
	// Timestamp returns the §613 timestamp used for §616 stable ordering.
	// The pipeline does NOT sort the input slice; the caller (engine-side
	// bridge or test) pre-orders per §616.1's controller-chosen ordering.
	// Timestamp is exposed here so the engine bridge can construct a
	// deterministic default ordering (timestamp ascending) when no
	// controller prompt is active.
	Timestamp() int
	// ControllerSeat returns the seat that controls the doubler. Used by
	// asymmetric handlers (Vorinclex Monstrous Raider: opponent-side arm
	// applies only when targetController != ControllerSeat) — symmetric
	// handlers ignore the gate.
	ControllerSeat() int
	// Applies reports whether this doubler applies to the (target,
	// counterType, targetController) tuple. The pipeline calls this for
	// every doubler in input order; only those returning true are applied.
	//
	// Card-specific gates live here: Hardened Scales gates on counterType
	// == "+1/+1" AND targetController == ControllerSeat AND target is a
	// creature or artifact; Branching Evolution narrows to creatures only;
	// Primal Vigor accepts ANY creature regardless of controller; etc.
	Applies(target Target, counterType string, targetController int) bool
	// Kind classifies the mutation for the audit trail.
	Kind() DoublerKind
	// Apply transforms a baseCount into the post-replacement count.
	// Doublers are pure functions of count — same input → same output, no
	// side effects. The pipeline chains the result into the next doubler's
	// baseCount.
	Apply(baseCount int) int
}

// DoublingApplication is one entry in the chain recorded by ApplyDoublers.
// Mirrors gameengine.ReplacementRef in shape (Source / Name / HandlerID /
// CountBefore / CountAfter) so the engine-side bridge can translate one
// slice to the other 1:1 when stamping the CounterStack's EffectsApplied
// audit trail.
type DoublingApplication struct {
	SourceInstanceID string
	SourceName       string
	HandlerID        string
	Kind             DoublerKind
	CountBefore      int
	CountAfter       int
}

// ApplyDoublers walks the provided doubler slice in input order (the §616
// controller-chosen application order assembled by the caller) and applies
// each Doubler whose Applies returns true to the running count. Returns the
// final post-pipeline count and the audit chain — one DoublingApplication
// entry per applied doubler.
//
// Per CR §122.1g, doubling applies ONLY to counter types with
// DoublingApplies=true. Energy / poison / experience / rad / lore / time
// short-circuit at the registry gate and return (baseCount, nil) regardless
// of which doublers are passed in.
//
// Per CR §616.1, the affected object's controller chooses application order
// when multiple replacements apply. The caller assembles the doublers slice
// in the chosen order; this function does not reorder. For symmetric
// scenarios with no controller prompt, the engine-side bridge sorts by
// (Timestamp ascending, HandlerID) for replay determinism.
//
// nil / empty doublers slice yields (baseCount, nil). baseCount <= 0 also
// short-circuits — there's no doubling of zero-count placements per the
// rules' "one or more counters" gate.
func ApplyDoublers(
	target Target,
	counterType string,
	baseCount int,
	targetController int,
	doublers []Doubler,
) (int, []DoublingApplication) {
	if baseCount <= 0 {
		return baseCount, nil
	}
	def := Lookup(counterType)
	if def == nil || !def.DoublingApplies {
		return baseCount, nil
	}
	if len(doublers) == 0 {
		return baseCount, nil
	}
	canonical := def.Name
	var chain []DoublingApplication
	current := baseCount
	for _, d := range doublers {
		if d == nil {
			continue
		}
		if !d.Applies(target, canonical, targetController) {
			continue
		}
		before := current
		after := d.Apply(before)
		if after < 0 {
			after = 0
		}
		chain = append(chain, DoublingApplication{
			SourceInstanceID: d.SourceInstanceID(),
			SourceName:       d.Name(),
			HandlerID:        d.HandlerID(),
			Kind:             d.Kind(),
			CountBefore:      before,
			CountAfter:       after,
		})
		current = after
	}
	return current, chain
}

// AddCountersWithDoublers is the Phase 6 entry point for callers with an
// active doubler slice. Returns the actual count placed plus the audit
// chain. The chain is stamped onto the resulting CounterStack via the
// engine-side bridge — the counters package itself stores the final count
// only (the audit trail lives on the engine's event log, not on the
// CounterStack struct, to keep the counters package's storage shape
// independent of replacement-effect surfaces).
//
// Unlike the simple AddCounters form (which routes through the Phase 1-5
// ApplyDoublingPipeline identity stub), this variant routes through
// ApplyDoublers above. Use this when the engine-side bridge has assembled
// the live doubler list from gs.Replacements.
//
// Existing simple AddCounters callers keep working unchanged.
//
// targetController is the seat whose counter is being placed — passed
// explicitly so asymmetric doublers (Vorinclex) can gate on opponent-ness
// without the package needing to know about Seat layout. The caller passes
// the controller of the target permanent (or the targeted player's seat
// index when the future player-counter API lands).
func AddCountersWithDoublers(
	target Target,
	counterType string,
	count int,
	sourceInstanceID string,
	tick int,
	targetController int,
	doublers []Doubler,
) (int, []DoublingApplication, error) {
	if count <= 0 {
		return 0, nil, nil
	}
	def := Lookup(counterType)
	if def == nil {
		return 0, nil, ErrUnknownCounterType
	}
	if !targetMatches(def, target) {
		return 0, nil, ErrInvalidTarget
	}
	canonical := def.Name
	placed, chain := ApplyDoublers(target, canonical, count, targetController, doublers)

	stacks := cloneStacks(target.CounterStacks())
	stacks = appendOrMerge(stacks, CounterStack{
		Type:               canonical,
		Count:              placed,
		PlacedByInstanceID: sourceInstanceID,
		PlacedAtTick:       tick,
	})
	target.SetCounterStacks(stacks)
	return placed, chain, nil
}
