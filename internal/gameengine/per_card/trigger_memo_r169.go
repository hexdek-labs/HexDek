package per_card

import (
	"sync/atomic"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r169 — memoize the resolved trigger-handler table per Card.
//
// fireTrigger walks every permanent on every battlefield for every
// event. For each one the collect closure takes reg.mu.RLock(), loads
// the card's lookup candidates from a sync.Map keyed on the display
// name, and probes reg.onTrigger for each candidate until one hits.
// A 30s CPU profile of a verified clean band on the r168 binary
// (474.1 -> 513.7 g/min across the sample) puts that closure at
// 4.13% of all CPU — 25.36% of fireTrigger, whose remaining 69.40% is
// PushPerCardTrigger genuinely resolving triggers and is NOT overhead.
//
// The overwhelming majority of permanents have no handler at all, so
// the overwhelming majority of that work produces the answer "no".
//
// WHAT THE MEMO IS KEYED ON, AND WHY NOT A READINESS BOOL.
//
// Two things can change the correct answer:
//
//  1. The card's NAME. Card.Name is mutable — the clone/rename paths
//     append parenthetical riders ("Crown of Gondor (Urza copy)"), and
//     lookupCandidates exists precisely because of that. A memo keyed
//     on identity alone would serve the pre-rename table forever.
//
//  2. The REGISTRY. Reset() installs a brand-new registry and replays
//     registerDefaults plus every AddResetHook callback; tests do call
//     it. Any registration at all can add a handler for a name that
//     previously had none.
//
// So the key is (DisplayName, registry generation) and never a "have we
// computed this" bool. r157 spent an entire round on exactly that
// mistake — three caches guarded by readiness flags kept serving a
// DFC's front face for the rest of the game after a transform
// reassigned the AST. A key made of the things the answer is derived
// from makes the staleness impossible to reintroduce rather than
// something a future author has to remember.
//
// WHY THE STORED TABLE IS A COPY.
//
// A memo hit takes no lock. That is only sound if what it hands back
// can never be mutated by a concurrent registration, so the table is
// copied — map and slices — at build time, under the read lock, and is
// immutable thereafter. Cards WITH handlers are the small minority, and
// the copy happens once per (card, generation), so this costs nothing
// on the path it is meant to speed up: a card with no handler memoizes
// a nil table and allocates nothing.
//
// Concurrency: Card objects are per-game (decks are cloned per game),
// the same assumption the r157, r158 and r164 Card-level caches already
// ship on, covered by the gameengine and hat race gates.

// registryGeneration increments on every mutation of the global
// registry's contents and on the construction of a new registry. It is
// a package-level counter rather than a Registry field so that a memo
// built against an OLD registry cannot be mistaken for a current one
// after Reset() swaps the pointer: generations are never reused.
var registryGeneration atomic.Uint64

// bumpRegistryGeneration invalidates every outstanding trigger memo.
// Called from newRegistry and from each registration method.
func bumpRegistryGeneration() { registryGeneration.Add(1) }

// RegistryGeneration exposes the current generation. Tests use it to
// assert that a registration really did invalidate.
func RegistryGeneration() uint64 { return registryGeneration.Load() }

// triggerMemo is the value stored in Card's opaque memo slot.
// Every field is written once, before the value is published, and is
// read-only afterwards.
type triggerMemo struct {
	gen     uint64
	name    string
	byEvent map[string][]TriggerHandler // nil means "this card has no handlers"
}

var (
	triggerMemoHits   atomic.Uint64
	triggerMemoMisses atomic.Uint64
)

// TriggerMemoStats returns (hits, misses) since process start or the
// last reset. Published rather than assumed — r155 and r164 both found
// a predicted hit rate badly wrong in opposite directions.
func TriggerMemoStats() (hits, misses uint64) {
	return triggerMemoHits.Load(), triggerMemoMisses.Load()
}

// ResetTriggerMemoStatsForTest zeroes the counters.
func ResetTriggerMemoStatsForTest() {
	triggerMemoHits.Store(0)
	triggerMemoMisses.Store(0)
}

// triggerTableFor returns the card's resolved event->handlers table, or
// nil when the card has no registered handlers.
//
// It is the ONLY reader of the memo slot and the only writer of it.
// The returned map must not be mutated by the caller.
func (r *Registry) triggerTableFor(c *gameengine.Card) map[string][]TriggerHandler {
	if c == nil {
		return nil
	}
	name := c.DisplayName()
	gen := registryGeneration.Load()
	if m, ok := c.PerCardTriggerMemo().(*triggerMemo); ok && m.gen == gen && m.name == name {
		triggerMemoHits.Add(1)
		return m.byEvent
	}
	triggerMemoMisses.Add(1)

	// Slow path: exactly what the collect closure used to do inline —
	// first candidate with a registered table wins, and the search stops
	// there rather than unioning. Preserved verbatim so the memo cannot
	// quietly change which handlers a card dispatches.
	r.mu.RLock()
	var found map[string][]TriggerHandler
	for _, k := range lookupCandidates(name) {
		if t := r.onTrigger[k]; t != nil {
			found = t
			break
		}
	}
	var owned map[string][]TriggerHandler
	if found != nil {
		owned = make(map[string][]TriggerHandler, len(found))
		for ev, hs := range found {
			owned[ev] = append([]TriggerHandler(nil), hs...)
		}
	}
	r.mu.RUnlock()

	c.SetPerCardTriggerMemo(&triggerMemo{gen: gen, name: name, byEvent: owned})
	return owned
}
