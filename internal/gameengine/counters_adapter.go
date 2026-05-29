package gameengine

// Counter DB Phase 2 — *Permanent adapter for the counters.Target
// interface, plus the HasKeywordCounter convenience method that wraps
// counters.HasKeywordCounter for callers already holding a *Permanent.
//
// The adapter lives in a wrapper struct because the Permanent.CounterStacks
// FIELD collides with the CounterStacks() METHOD required by
// counters.Target. counterTarget bridges the two without renaming a field
// that Phase 1 callers and tests already depend on.

import "github.com/hexdek/hexdek/internal/gameengine/counters"

// counterTarget wraps a *Permanent so it satisfies counters.Target. Callers
// should use Permanent.AsCounterTarget() rather than constructing this
// directly; the indirection lets future Phase 6+ work add validation
// or replacement-effect plumbing without changing every call site.
type counterTarget struct{ p *Permanent }

// HasCardType implements counters.Target. Defers to the existing Permanent
// type-line predicate so layer-modified types are respected when present.
func (ct counterTarget) HasCardType(t string) bool {
	if ct.p == nil {
		return false
	}
	return ct.p.hasType(t)
}

// HasSubtype implements counters.Target. Sagas are identified by a
// "saga" entry in Card.Types per the resolver's ETB convention; future
// Phase 8 layer work may swap this to a GameState.HasSubtypeOf-style
// call once non-saga subtype counters arrive.
func (ct counterTarget) HasSubtype(t string) bool {
	if ct.p == nil || ct.p.Card == nil {
		return false
	}
	for _, st := range ct.p.Card.Types {
		if st == t {
			return true
		}
	}
	return false
}

// CounterStacks implements counters.Target.
func (ct counterTarget) CounterStacks() []counters.CounterStack {
	if ct.p == nil {
		return nil
	}
	return ct.p.CounterStacks
}

// SetCounterStacks implements counters.Target.
func (ct counterTarget) SetCounterStacks(s []counters.CounterStack) {
	if ct.p == nil {
		return
	}
	ct.p.CounterStacks = s
}

// AsCounterTarget returns a counters.Target view of this permanent. The
// returned value is safe to pass to any counters package API; mutations
// flow back to the underlying Permanent.
func (p *Permanent) AsCounterTarget() counters.Target {
	return counterTarget{p: p}
}

// HasKeywordCounter reports whether this permanent carries at least one
// counter that, per CR §122.1c, grants the named keyword. Convenience
// wrapper over counters.HasKeywordCounter.
//
// §122.6: the result is independent of card type. A creature stripped of
// its types by Humility still reports HasKeywordCounter("flying")=true if
// a flying counter remains.
func (p *Permanent) HasKeywordCounter(keyword string) bool {
	if p == nil {
		return false
	}
	return counters.HasKeywordCounter(p.AsCounterTarget(), keyword)
}
