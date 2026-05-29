package counters

import "errors"

// Counter DB Phase 4 — proliferate primitive per CR §701.23:
//
//	"To proliferate means to choose any number of permanents and/or
//	players that have a counter, then give each another counter of a
//	kind already there."
//
// Per choice the controller picks ONE counter type already on the target
// and gains one more counter of that kind. Doubling pipeline still
// applies to the placement (so Hardened Scales / Doubling Season amplify
// a proliferate-driven +1/+1 once Phase 6 lands the replacement walk).
//
// The primitive lives at the counters package level (operating on
// Target) so the gameengine wrapper can adapt both *Permanent and *Seat
// uniformly. The wrapper handles event logging, trigger dispatch, and
// the §122.1g doubling pipeline through the shared AddCounters path.

// ProliferateChoice is one (target, counter-type) selection within a
// single proliferate event. A caller proliferating an Atraxa end step
// builds one ProliferateChoice per (target, kind-already-present) pair.
type ProliferateChoice struct {
	// Target is the permanent or player that already carries a counter.
	Target Target
	// CounterType is the registry name (or alias) of the counter kind the
	// controller chose to add one more of. MUST already be present on
	// Target with Count > 0 — Proliferate rejects choices that don't
	// satisfy CR §701.23's "of a kind already there" gate.
	CounterType string
}

// ErrCounterTypeNotPresent is returned by Proliferate when a choice
// names a counter type the target does not currently carry. CR §701.23
// requires the kind to be "already there" — silently no-op'ing would
// mask caller bugs (Atraxa's controller picks a kind by inspecting
// CounterStacks; a mismatch means stale state or a per_card race).
var ErrCounterTypeNotPresent = errors.New("counters: chosen counter type not present on target")

// ErrNilTarget is returned by Proliferate when a choice carries a nil
// Target. The wrapper layer prunes these before calling, but the
// primitive defends against direct callers (tests, future call sites).
var ErrNilTarget = errors.New("counters: proliferate choice has nil target")

// Proliferate applies one additional counter of the chosen kind to each
// choice's target. Returns the number of choices actually applied (may
// be less than len(choices) if some targets had the kind removed in a
// concurrent SBA pass — but never silently — the first failing choice
// short-circuits and returns the accumulated count plus the error).
//
// Doubling per §122.1g applies to each placement individually: a
// proliferate with two +1/+1 choices under Hardened Scales places three
// counters per choice, not three total.
//
// sourceInstanceID and tick are stamped onto every placed CounterStack
// so the InstanceID Phase 3 lineage continues through proliferate-driven
// counter additions. Callers MUST pass the proliferating ability's
// instance id (Atraxa's end-step trigger, Inexorable Tide's static
// trigger, etc.) not the source permanent's id.
//
// Empty choices slice is a clean (0, nil) no-op — matches the engine's
// "choose any number" wording, which permits zero.
func Proliferate(choices []ProliferateChoice, sourceInstanceID string, tick int) (int, error) {
	if len(choices) == 0 {
		return 0, nil
	}
	applied := 0
	for _, c := range choices {
		if c.Target == nil {
			return applied, ErrNilTarget
		}
		canonical := CanonicalName(c.CounterType)
		def := Lookup(canonical)
		if def == nil {
			return applied, ErrUnknownCounterType
		}
		if CounterCount(c.Target, canonical) <= 0 {
			return applied, ErrCounterTypeNotPresent
		}
		// Skip the AddCounters ValidTargets gate: the target already
		// proved it can hold this kind by carrying a counter. This is
		// the §701.23 contract — proliferate never validates legality
		// beyond presence. Still routes through the doubling pipeline.
		placed := ApplyDoublingPipeline(c.Target, canonical, 1)
		stacks := cloneStacks(c.Target.CounterStacks())
		stacks = appendOrMerge(stacks, CounterStack{
			Type:               canonical,
			Count:              placed,
			PlacedByInstanceID: sourceInstanceID,
			PlacedAtTick:       tick,
		})
		c.Target.SetCounterStacks(stacks)
		applied++
	}
	return applied, nil
}

// IsProliferateEligibleType reports whether a counter type is a §122
// counter that proliferate may target. Energy (CR §106.11) and any
// unregistered type return false. Used by gameengine wrappers and AI
// target pickers to skip ineligible candidates before assembling a
// ProliferateChoice slice.
func IsProliferateEligibleType(name string) bool {
	def := Lookup(name)
	if def == nil {
		return false
	}
	return def.Proliferate
}
