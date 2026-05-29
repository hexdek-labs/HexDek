package counters

import "errors"

// CounterStack is the per-Permanent record of N counters of one type with
// the InstanceID lineage of whichever ability/effect placed them. Same-type
// counters from different sources MAY be aggregated into a single stack
// (single PlacedByInstanceID retained — the most recent placement); the
// aggregation policy is set by AddCounters and is observable through
// CounterStacks ordering.
type CounterStack struct {
	// Type is the canonical counter name per the registry (e.g. "+1/+1").
	Type string
	// Count is the aggregated number of counters of this type in the stack.
	Count int
	// PlacedByInstanceID is the InstanceID of the ability/effect that
	// placed (or most recently augmented) this stack. Empty when the
	// placement source predates the InstanceID Phase 2+ migration; the
	// counters_property_test pins this is preserved across mutations that
	// don't change source.
	PlacedByInstanceID string
	// PlacedAtTick is the game-clock value at placement. Phase 1 callers
	// pass gs.EventClock (or equivalent) — Loki replay uses this for
	// forensic ordering when multiple placements land in the same priority
	// pass.
	PlacedAtTick int
}

// Target is the minimal interface a counter-bearing object must satisfy.
// *gameengine.Permanent will adapt to this in Phase 2+ wiring; Phase 1
// tests use a struct mock to pin the API contract.
type Target interface {
	// HasCardType reports whether the target's type line includes the
	// given (lowercased) card type — "creature", "planeswalker",
	// "artifact", "enchantment", "land", "battle".
	HasCardType(string) bool
	// HasSubtype reports whether the target's subtype line includes the
	// given (lowercased) subtype — used for "saga" target validation.
	HasSubtype(string) bool
	// CounterStacks returns the live counter slice. Callers MUST NOT
	// mutate the slice in place; use AddCounters / RemoveCounters
	// instead. The §122.6 persistence invariant rests on Layer-4
	// type-change code never replacing this slice.
	CounterStacks() []CounterStack
	// SetCounterStacks installs a new slice. Called by AddCounters /
	// RemoveCounters after computing the post-state. Implementations
	// MUST simply replace the field — no validation, no doubling, no
	// trigger fires here (the API runs those upstream).
	SetCounterStacks([]CounterStack)
}

// ErrUnknownCounterType is returned by AddCounters / RemoveCounters when
// the registry has no entry for the requested type. Phase 1 surfaces this
// as an error rather than a panic so the engine can degrade gracefully if
// the corpus parser emits a counter type ahead of registry coverage.
var ErrUnknownCounterType = errors.New("counters: unknown counter type")

// ErrInvalidTarget is returned when the counter type's ValidTargets does
// not include any of the target's card types.
var ErrInvalidTarget = errors.New("counters: target does not satisfy ValidTargets")

// AddCounters places count counters of the given type on the target.
// Validates placement, applies the §122.1g doubling pipeline, records the
// PlacedByInstanceID lineage, and returns the actual count placed (may
// exceed count if doubling fired) or an error if the placement is illegal.
//
// Phase 1 doubling pipeline is the placeholder ApplyDoublingPipeline
// helper; Phase 6 replaces it with the full replacement-effect walk.
//
// count <= 0 is a silent no-op returning (0, nil) — matches the engine's
// convention for "place X counters where X=0".
func AddCounters(target Target, counterType string, count int, sourceInstanceID string, tick int) (int, error) {
	if count <= 0 {
		return 0, nil
	}
	def := Lookup(counterType)
	if def == nil {
		return 0, ErrUnknownCounterType
	}
	if !targetMatches(def, target) {
		return 0, ErrInvalidTarget
	}
	canonical := def.Name
	placed := ApplyDoublingPipeline(target, canonical, count)

	stacks := cloneStacks(target.CounterStacks())
	stacks = appendOrMerge(stacks, CounterStack{
		Type:               canonical,
		Count:              placed,
		PlacedByInstanceID: sourceInstanceID,
		PlacedAtTick:       tick,
	})
	target.SetCounterStacks(stacks)
	return placed, nil
}

// RemoveCounters removes up to count counters of the given type from the
// target. Returns the number actually removed (may be less than count if
// the target had fewer). Drains stacks oldest-first (insertion order) per
// CR §122.3 — a counter is just a counter; we don't track removal lineage
// beyond emptying stacks as they exhaust.
//
// count <= 0 is a silent no-op returning 0.
func RemoveCounters(target Target, counterType string, count int) int {
	if count <= 0 {
		return 0
	}
	canonical := CanonicalName(counterType)
	stacks := cloneStacks(target.CounterStacks())
	removed := 0
	out := stacks[:0]
	for _, s := range stacks {
		if s.Type != canonical || removed >= count {
			out = append(out, s)
			continue
		}
		take := count - removed
		if take >= s.Count {
			removed += s.Count
			continue
		}
		s.Count -= take
		removed += take
		out = append(out, s)
	}
	target.SetCounterStacks(out)
	return removed
}

// CounterCount returns the aggregated count of a given counter type on
// the target. Sums across stacks (a counter type may live in multiple
// stacks if AddCounters chose not to merge — Phase 1 merges by default).
func CounterCount(target Target, counterType string) int {
	if target == nil {
		return 0
	}
	canonical := CanonicalName(counterType)
	total := 0
	for _, s := range target.CounterStacks() {
		if s.Type == canonical {
			total += s.Count
		}
	}
	return total
}

// ApplyDoublingPipeline is the §122.1g placeholder. Phase 1 returns the
// baseCount unchanged for every type — the full replacement-effect walk
// (Doubling Season, Hardened Scales, Primal Vigor, Branching Evolution,
// Vorinclex Voice of Hunger) lands in Phase 6 alongside the rest of the
// replacement-effect plumbing.
//
// The signature is fixed now so AddCounters call sites are stable across
// the Phase 1 → Phase 6 migration: only the implementation changes.
//
// DoublingApplies=false types (lore, time) bypass even the future
// pipeline. Returning baseCount here is consistent with that — Phase 6
// will short-circuit on the registry flag before walking the effects.
func ApplyDoublingPipeline(target Target, counterType string, baseCount int) int {
	def := Lookup(counterType)
	if def == nil || !def.DoublingApplies {
		return baseCount
	}
	// Phase 6: walk gs.Replacements for "if one or more counters would be
	// placed" effects in §616 timestamp order, controller-prompted on
	// multiple-applicable. Phase 1: identity.
	return baseCount
}

// targetMatches checks whether ANY of the def's ValidTargets matches the
// target's card type / subtype profile. Player-target counters (poison,
// experience) are out of Phase 1 scope; they get their own non-Target
// API in Phase 8.
func targetMatches(def *CounterTypeDef, target Target) bool {
	if target == nil {
		return false
	}
	for _, vt := range def.ValidTargets {
		switch vt {
		case TargetCreature:
			if target.HasCardType("creature") {
				return true
			}
		case TargetPlaneswalker:
			if target.HasCardType("planeswalker") {
				return true
			}
		case TargetArtifact:
			if target.HasCardType("artifact") {
				return true
			}
		case TargetEnchantment:
			if target.HasCardType("enchantment") {
				return true
			}
		case TargetLand:
			if target.HasCardType("land") {
				return true
			}
		case TargetBattle:
			if target.HasCardType("battle") {
				return true
			}
		case TargetSaga:
			if target.HasSubtype("saga") {
				return true
			}
		case TargetPlayer:
			// Phase 8 carves these out to a Seat-level API.
		}
	}
	return false
}

// cloneStacks returns a defensive copy so mutations don't alias the
// caller's slice. Critical for the §122.6 persistence invariant — the
// engine's Layer-4 type-strip code reads the slice; it MUST observe the
// pre-mutation snapshot, not a half-applied update.
func cloneStacks(in []CounterStack) []CounterStack {
	if len(in) == 0 {
		return nil
	}
	out := make([]CounterStack, len(in))
	copy(out, in)
	return out
}

// appendOrMerge folds a new placement into the most recent same-type
// stack if present; otherwise appends a fresh stack. This is the Phase 1
// merge policy — Phase 2+ may switch to per-source stacks if invariant
// work demands tighter lineage. PlacedByInstanceID and PlacedAtTick of
// the most recent placement are retained on the merged stack.
func appendOrMerge(stacks []CounterStack, fresh CounterStack) []CounterStack {
	for i := len(stacks) - 1; i >= 0; i-- {
		if stacks[i].Type == fresh.Type && stacks[i].PlacedByInstanceID == fresh.PlacedByInstanceID {
			stacks[i].Count += fresh.Count
			stacks[i].PlacedAtTick = fresh.PlacedAtTick
			return stacks
		}
	}
	return append(stacks, fresh)
}
