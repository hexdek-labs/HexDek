// Package per_card holds the snowflake-handler registry for the Go engine,
// mirroring scripts/extensions/per_card_runtime.py. Each card whose oracle
// text cannot be expressed cleanly via the general gameast grammar gets a
// hand-rolled handler keyed on the card NAME (exact match on printed name).
//
// Wiring into the engine:
//
//   - gameengine exposes function-pointer hooks (ETBHook, CastHook,
//     ResolveHook, ActivatedHook, TriggerHook).
//   - This package's init() installs dispatch closures into those hooks.
//   - gameengine.stack.go / resolve.go call the hooks at the appropriate
//     moments; when the hook is nil (no per_card import), dispatch is a
//     no-op. This keeps gameengine dependency-free of per_card.
//
// All registrations are keyed by the card's printed name. Case-sensitive
// display, but we normalize for matching (lower-case, punctuation-free).
package per_card

import (
	"reflect"
	"strings"
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Registry is the per-card handler lookup table.
type Registry struct {
	mu        sync.RWMutex
	etb       map[string][]ETBHandler
	onCast    map[string][]CastHandler
	onResolve map[string][]ResolveHandler
	activated map[string][]ActivatedHandler
	onTrigger map[string]map[string][]TriggerHandler
	// ownsETBTrigger marks cards whose OnETB handler FULLY IMPLEMENTS
	// the card's printed self-ETB triggered ability (the engine's
	// generic AST push is suppressed — Judge r63 double-fire gate).
	// OnETB registrations WITHOUT this mark are bookkeeping
	// (replacement/static registration, e.g. the ETB-doubler family)
	// and the AST trigger keeps firing.
	ownsETBTrigger map[string]bool
}

// ETBHandler fires when a permanent with the registered name enters the
// battlefield, AFTER stock AST-driven ETB triggers have resolved.
type ETBHandler func(gs *gameengine.GameState, perm *gameengine.Permanent)

// CastHandler fires when a spell with the registered name is cast but
// before the stack item resolves.
type CastHandler func(gs *gameengine.GameState, item *gameengine.StackItem)

// ResolveHandler fires when a non-permanent spell with the registered
// name resolves. When any ResolveHandler is registered for a card, the
// stock Effect dispatch is SKIPPED — the handler is authoritative.
type ResolveHandler func(gs *gameengine.GameState, item *gameengine.StackItem)

// ActivatedHandler fires when an activated ability of the registered
// card is resolved. AbilityIdx is 0-based; ctx carries X, targets, etc.
type ActivatedHandler func(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{})

// TriggerHandler fires for a custom game-event trigger.
type TriggerHandler func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{})

var (
	globalMu sync.RWMutex
	global   = newRegistry()
)

func newRegistry() *Registry {
	return &Registry{
		etb:            map[string][]ETBHandler{},
		onCast:         map[string][]CastHandler{},
		onResolve:      map[string][]ResolveHandler{},
		activated:      map[string][]ActivatedHandler{},
		onTrigger:      map[string]map[string][]TriggerHandler{},
		ownsETBTrigger: map[string]bool{},
	}
}

// Global returns the process-wide registry.
func Global() *Registry {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// Reset wipes the global registry and re-runs the batch-#1 default
// registration. Tests call this in t.Cleanup() to isolate runs.
//
// Reset also re-invokes every callback registered via AddResetHook so
// that handlers wired in by sibling init() functions (tribal_lords.go,
// obeka_support.go, the zz_*_register.go batches, etc.) are restored.
// Go runs init() exactly once per process, so without this the post-
// Reset registry permanently loses anything registered outside
// registerDefaults().
func Reset() {
	globalMu.Lock()
	global = newRegistry()
	globalMu.Unlock()
	registerDefaults()
	r := Global()
	resetHooksMu.Lock()
	hooks := append([]func(*Registry){}, resetHooks...)
	resetHooksMu.Unlock()
	for _, fn := range hooks {
		fn(r)
	}
	installEngineHooks()
}

// resetHooks holds callbacks that restore handlers wired by sibling
// init() functions. AddResetHook lets each init() participate in
// Reset() without registry.go needing to know about every batch.
var (
	resetHooksMu sync.Mutex
	resetHooks   []func(*Registry)
)

// AddResetHook registers fn to be called on the fresh global registry
// after every Reset(). init() functions that populate the registry
// should call AddResetHook(themselves) so their handlers survive a
// test-driven Reset().
func AddResetHook(fn func(*Registry)) {
	if fn == nil {
		return
	}
	resetHooksMu.Lock()
	resetHooks = append(resetHooks, fn)
	resetHooksMu.Unlock()
}

// NormalizeName lowercases, strips punctuation, and collapses whitespace.
func NormalizeName(name string) string {
	return normalizeName(name)
}

// normalizedNameCache memoizes normalizeName by raw input. fireTrigger
// calls normalizeName(perm.Card.DisplayName()) for every battlefield
// permanent on every trigger event, and the underlying string-builder
// path showed up as the single largest allocator in the perf profile
// (~14M alloc objects / 285MB across a 500-game tournament). The card
// name space is bounded (~32K oracle entries \u00d7 ~2 face variants), so a
// process-wide sync.Map both eliminates the alloc churn and keeps
// goroutine-parallel tournament workers contention-free.
var normalizedNameCache sync.Map // map[string]string

func normalizeName(name string) string {
	if v, ok := normalizedNameCache.Load(name); ok {
		return v.(string)
	}
	out := normalizeNameSlow(name)
	normalizedNameCache.Store(name, out)
	return out
}

func normalizeNameSlow(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	b.Grow(len(name))
	prevSpace := false
	for _, r := range name {
		switch r {
		case '\'', '\u2019', ',', '.', '!', '?', ':', ';', '-', '\u2014', '\u2013':
			continue
		case ' ', '\t':
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

// lookupCandidates returns the ordered list of normalized name keys to try
// when dispatching a handler. The first hit wins. The fallbacks unify five
// runtime-rename surfaces under one rule, so existing handlers fire on
// variants without requiring per-variant aliases:
//
//   - "X (cascade)"                — cascade-cast spells (cascade.go renames
//     StackItem.Card.Name; the same name shows up on resulting permanents)
//   - "X (Urza copy)"              — Urza, Lord High Artificer copies and
//     other ETB-copy-token patterns that suffix the source name
//   - "X (Miirym Token)" / "(... Token)" — token rider tags from the
//     dragon-doubling family
//   - "X (Restore-Relic token)"    — Lorehold Archivist's spell-token
//     surrogate
//   - "Front // Back"              — DFC: also try the front face alone
//
// All variants are runtime mutations that never appear as base card names,
// so stripping a single trailing parenthetical or a " // " split is safe
// against collision with real printed names.
//
// Wave dev/muninn-handlers-201-220: previously the variant suffixes silently
// bypassed every dispatcher except fireETB/fireTrigger (which had a partial
// " // " fallback only). That left ~5k cumulative gap-log hits for cards
// that DO have handlers — Necromancy (cascade), Vibrance (cascade), Land Tax
// (cascade), Phoenix Fleet Airship (Urza copy), Tiamat (Miirym Token), etc.
// This helper centralizes the fallback chain so all five fire* dispatchers
// use the same rules.
// lookupCandidatesCache memoizes lookupCandidates by raw display name.
// fireTrigger / fireETB / fireCast / fireResolve / fireActivated each call
// lookupCandidates once per dispatched permanent and the returned slice is
// strictly read-only at every call site (`for _, k := range lookupCandidates(...)`).
// The card-name surface is bounded (~32K oracle entries × ~handful of
// runtime-rename variants) so a process-wide sync.Map eliminates the
// inner-loop slice + map churn that the dispatch_bench_r60 baseline showed
// dominating the dispatcher's allocation profile (~390 B / 3 allocs per
// FireCardTrigger drop to ~0 B / 0 allocs after caching).
//
// CONTRACT: the cached slice MUST NOT be mutated by callers. All current
// callers iterate it as a read-only range; any future caller that needs
// to mutate must copy first.
var lookupCandidatesCache sync.Map // map[string][]string

func lookupCandidates(displayName string) []string {
	if v, ok := lookupCandidatesCache.Load(displayName); ok {
		return v.([]string)
	}
	out := lookupCandidatesSlow(displayName)
	lookupCandidatesCache.Store(displayName, out)
	return out
}

func lookupCandidatesSlow(displayName string) []string {
	nk := normalizeName(displayName)
	out := []string{nk}
	seen := map[string]bool{nk: true}
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	// Iteratively strip trailing parentheticals — covers stacked renames
	// like "Crown of Gondor (Urza copy) (Urza copy)" where the same rename
	// source ran twice. Each layer is also tried as a DFC front face.
	cur := nk
	for {
		idx := strings.LastIndex(cur, " (")
		if idx <= 0 || !strings.HasSuffix(cur, ")") {
			break
		}
		stripped := strings.TrimSpace(cur[:idx])
		if stripped == "" || stripped == cur {
			break
		}
		add(stripped)
		if dfc := strings.Index(stripped, " // "); dfc > 0 {
			if front := strings.TrimSpace(stripped[:dfc]); front != "" {
				add(front)
			}
		}
		cur = stripped
	}

	// Strip trailing " token" suffix — covers paren-less token names like
	// "Myr Token", "Claim Jumper Token", "Rankle and Torbran Token" that
	// the dragon-doubling / Restore-Relic / direct-mint paths emit when
	// the token Name lacks a parenthetical rider tag. No printed card
	// ends in " Token", so this is safe against collision.
	for _, base := range append([]string{}, out...) {
		if trimmed := strings.TrimSuffix(base, " token"); trimmed != base && trimmed != "" {
			add(trimmed)
		}
	}

	if idx := strings.Index(nk, " // "); idx > 0 {
		if front := strings.TrimSpace(nk[:idx]); front != "" {
			add(front)
		}
	}
	return out
}

// -----------------------------------------------------------------------------
// Registration
// -----------------------------------------------------------------------------

// OnETB registers an ETB handler for cardName.
func (r *Registry) OnETB(cardName string, h ETBHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	// r62 #1034 generalization: idempotent on handler identity — the
	// registerDefaults-vs-batch-init cross-file duplication that made
	// OnTrigger handlers fire twice had ALSO double-registered 38 ETB
	// handlers (Anointed Procession, Wayward Servant, Propaganda,
	// Academy Manufactor, the zombie-lord family, …), firing each
	// twice per ETB.
	hp := reflect.ValueOf(h).Pointer()
	for _, existing := range r.etb[k] {
		if reflect.ValueOf(existing).Pointer() == hp {
			return
		}
	}
	r.etb[k] = append(r.etb[k], h)
}

// OnCast registers a cast-time handler for cardName.
func (r *Registry) OnCast(cardName string, h CastHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	hp := reflect.ValueOf(h).Pointer()
	for _, existing := range r.onCast[k] {
		if reflect.ValueOf(existing).Pointer() == hp {
			return
		}
	}
	r.onCast[k] = append(r.onCast[k], h)
}

// OnResolve registers a resolve-time handler for non-permanent spells.
func (r *Registry) OnResolve(cardName string, h ResolveHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	hp := reflect.ValueOf(h).Pointer()
	for _, existing := range r.onResolve[k] {
		if reflect.ValueOf(existing).Pointer() == hp {
			return
		}
	}
	r.onResolve[k] = append(r.onResolve[k], h)
}

// OnActivated registers an activated-ability handler.
func (r *Registry) OnActivated(cardName string, h ActivatedHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	hp := reflect.ValueOf(h).Pointer()
	for _, existing := range r.activated[k] {
		if reflect.ValueOf(existing).Pointer() == hp {
			return
		}
	}
	r.activated[k] = append(r.activated[k], h)
}

// EnumerateOnTriggerRegistrations returns every (normalizedCardName,
// canonicalEvent, handlerCount) tuple registered in the trigger map.
// Used exclusively by the engine-event integration test in this
// package to walk the full handler base for cross-handler regression
// detection. Returns a snapshot — callers can mutate the result
// without affecting registry state.
func (r *Registry) EnumerateOnTriggerRegistrations() []struct {
	CardName     string
	Event        string
	HandlerCount int
} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []struct {
		CardName     string
		Event        string
		HandlerCount int
	}
	for cardName, byEvent := range r.onTrigger {
		for event, handlers := range byEvent {
			out = append(out, struct {
				CardName     string
				Event        string
				HandlerCount int
			}{cardName, event, len(handlers)})
		}
	}
	return out
}

// OnTrigger registers a custom-event trigger handler.
//
// r62 — IDEMPOTENT on (card, canonical event, handler identity). The
// registry had accumulated 53 double-registrations through two
// mechanisms, each making the handler FIRE TWICE per event (the
// Wave-1b double-dispatch disease at the registry layer):
//
//   - alias collapse: OnTrigger(card, "opponent_draws", fn) +
//     OnTrigger(card, "card_drawn", fn) both canonicalize to
//     "card_drawn" (Smothering Tithe minted two Treasures per opponent
//     draw; Consecrated Sphinx drew 4 instead of 2; Orcish Bowmasters
//     pinged twice; Nekusar drained twice);
//   - cross-file duplication: registerDefaults() AND a sibling batch
//     init both call the same register fn (Howling Mine drew 2 per
//     upkeep via registry.go:~1743 + batch17_sweep.go's init).
//
// Skipping a re-registration of the SAME function for the SAME
// (card, canonical event) closes both shapes generically — a card with
// two genuinely distinct abilities registers distinct functions and is
// unaffected. See /tmp/fable-review/wave1b-doubletax-audit.md.
func (r *Registry) OnTrigger(cardName, event string, h TriggerHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	if r.onTrigger[k] == nil {
		r.onTrigger[k] = map[string][]TriggerHandler{}
	}
	canonical := gameengine.NormalizeEventSingle(event)
	hp := reflect.ValueOf(h).Pointer()
	for _, existing := range r.onTrigger[k][canonical] {
		if reflect.ValueOf(existing).Pointer() == hp {
			return // same handler already registered for this event
		}
	}
	r.onTrigger[k][canonical] = append(r.onTrigger[k][canonical], h)
}

// -----------------------------------------------------------------------------
// Dispatch helpers (called by the hook installers below)
// -----------------------------------------------------------------------------

// fireETB runs every ETB handler registered for perm's card name.
func fireETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if perm == nil || perm.Card == nil {
		return
	}
	reg := Global()
	reg.mu.RLock()
	var handlers []ETBHandler
	for _, k := range lookupCandidates(perm.Card.DisplayName()) {
		if hs := reg.etb[k]; len(hs) > 0 {
			handlers = append([]ETBHandler(nil), hs...)
			break
		}
	}
	reg.mu.RUnlock()
	for _, h := range handlers {
		if gs != nil && gs.Flags != nil && gs.Flags["ended"] == 1 {
			break
		}
		h(gs, perm)
	}
}

// fireOnCast runs every OnCast handler.
func fireOnCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	if item == nil || item.Card == nil {
		return
	}
	reg := Global()
	reg.mu.RLock()
	var handlers []CastHandler
	for _, k := range lookupCandidates(item.Card.DisplayName()) {
		if hs := reg.onCast[k]; len(hs) > 0 {
			handlers = append([]CastHandler(nil), hs...)
			break
		}
	}
	reg.mu.RUnlock()
	for _, h := range handlers {
		h(gs, item)
	}
}

// fireOnResolve runs every OnResolve handler and returns the count fired.
// The caller (stack.go) uses the count to decide whether to skip stock
// effect dispatch.
func fireOnResolve(gs *gameengine.GameState, item *gameengine.StackItem) int {
	if item == nil || item.Card == nil {
		return 0
	}
	reg := Global()
	reg.mu.RLock()
	var handlers []ResolveHandler
	for _, k := range lookupCandidates(item.Card.DisplayName()) {
		if hs := reg.onResolve[k]; len(hs) > 0 {
			handlers = append([]ResolveHandler(nil), hs...)
			break
		}
	}
	reg.mu.RUnlock()
	for _, h := range handlers {
		h(gs, item)
	}
	return len(handlers)
}

// fireActivated runs every activated-ability handler.
func fireActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if src == nil || src.Card == nil {
		return
	}
	reg := Global()
	reg.mu.RLock()
	var handlers []ActivatedHandler
	for _, k := range lookupCandidates(src.Card.DisplayName()) {
		if hs := reg.activated[k]; len(hs) > 0 {
			handlers = append([]ActivatedHandler(nil), hs...)
			break
		}
	}
	reg.mu.RUnlock()
	for _, h := range handlers {
		h(gs, src, abilityIdx, ctx)
	}
}

// isManaAbilityEvent returns true for events that correspond to mana
// abilities per CR §605.1b. Triggered abilities that trigger from
// activating a mana ability AND could produce mana are themselves mana
// abilities and do NOT use the stack (CR §605.3a). These events must
// resolve immediately, not be pushed to the stack.
func isManaAbilityEvent(event string) bool {
	switch event {
	case "mana_added_from_permanent":
		// Kinnan, Bonder Prodigy — adds one mana when a nonland permanent
		// is tapped for mana. This is a mana ability per CR §605.1b.
		return true
	}
	return false
}

// isLeavingPermEvent reports whether the event semantically carries a
// "leaving permanent" via ctx["perm"] — i.e. the bearer has already left
// the battlefield by the time the trigger fires. fireTrigger uses this to
// also dispatch against ctx["perm"], since the standard battlefield walk
// would miss the leaving perm's own self-trigger handlers (Banisher
// Priest's ltbReturnLinkedExile, Hostage Taker's LTB-return, etc.).
func isLeavingPermEvent(event string) bool {
	switch event {
	case "permanent_ltb",
		"creature_dies",
		"permanent_sacrificed",
		"creature_sacrificed",
		"artifact_sacrificed",
		"food_sacrificed",
		"card_exiled":
		return true
	}
	return false
}

// fireTrigger walks every permanent on the battlefield and fires
// matching (name, event) handlers. Used for Rhystic Study, Mystic Remora,
// Aetherflux Reservoir, Displacer Kitten, Hullbreaker Horror, Cloudstone
// Curio — all triggered-by-game-event cards.
//
// Per CR §603.3, triggered abilities are pushed to the stack via
// PushPerCardTrigger so players have priority to respond. Mana abilities
// (CR §605.1b / §605.3a) are exempt and resolve immediately.
//
// Per CR §101.4 (APNAP ordering), when multiple triggered abilities
// trigger simultaneously, the active player's triggers go on the stack
// FIRST (resolve LAST due to LIFO), then each other player in turn
// order. Within a single player's triggers, that player chooses the
// order (MVP: arbitrary/registration order).
func fireTrigger(gs *gameengine.GameState, event string, ctx map[string]interface{}) {
	if gs == nil || event == "" {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["trigger_depth"]++
	defer func() { gs.Flags["trigger_depth"]-- }()
	if gs.Flags["trigger_depth"] > 8 {
		gameengine.LogLoopGuardFired(gs, "percard_dispatch_depth_cap", map[string]interface{}{
			"depth": gs.Flags["trigger_depth"], "event": event,
		})
		// Emit a marker so TriggerCompleteness sees a follow-up event even
		// when the runaway-loop guard silently swallows the dispatch.
		gs.LogEvent(gameengine.Event{
			Kind:   "trigger_evaluated",
			Seat:   -1,
			Target: -1,
			Source: event,
			Details: map[string]interface{}{
				"event":  event,
				"capped": "trigger_depth",
				"rule":   "603.3",
			},
		})
		return
	}
	canonical := gameengine.NormalizeEventSingle(event)

	// Collect hits grouped by controller seat.
	type hit struct {
		perm *gameengine.Permanent
		hs   []TriggerHandler
	}
	hitsBySeat := map[int][]hit{}

	reg := Global()
	scanned := map[*gameengine.Permanent]bool{}
	collect := func(seatIdx int, perm *gameengine.Permanent) {
		if perm == nil || perm.Card == nil || scanned[perm] {
			return
		}
		scanned[perm] = true
		reg.mu.RLock()
		var byEvent map[string][]TriggerHandler
		for _, k := range lookupCandidates(perm.Card.DisplayName()) {
			if m := reg.onTrigger[k]; m != nil {
				byEvent = m
				break
			}
		}
		var handlers []TriggerHandler
		if byEvent != nil {
			handlers = append([]TriggerHandler(nil), byEvent[canonical]...)
		}
		reg.mu.RUnlock()
		if len(handlers) > 0 {
			hitsBySeat[seatIdx] = append(hitsBySeat[seatIdx], hit{perm: perm, hs: handlers})
		}
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, perm := range seat.Battlefield {
			collect(seat.Idx, perm)
		}
	}
	// LTB-shape events fire AFTER the leaving perm has been removed from
	// the battlefield (zone_change.go calls removePermanent before
	// FireZoneChangeTriggers → FireCardTrigger). Without this fallback,
	// self-trigger handlers on the leaving perm (Banisher Priest's
	// ltbReturnLinkedExile, Hostage Taker's LTB-return, Oblivion Ring,
	// Detention Sphere, Faceless Butcher — every LTBReturn-shape card)
	// never fire, leaving their LinkedExile slice + ExiledByMe + the
	// exiled card's ExiledByTimestamp permanently orphaned. The new
	// ExileLinkageIntegrity invariant (Loki r60 fresh main 2026-05-30)
	// surfaced this as 72 hits across Prison Barricade / Leonardo, Sewer
	// Samurai / Myr Prototype / Great Hall of the Biblioplex — all of
	// which were the EXILED TARGETS of legitimate Banisher-Priest-family
	// sources that died without their LTB cleanup ever running.
	//
	// Include the leaving perm from ctx so its own handlers dispatch.
	// Scoped to events that carry a leaving-perm semantically (LTB,
	// dies, sacrificed, exiled). Deduped via scanned[] so a perm still
	// on the battlefield (some LTB-shape callers detach AFTER firing)
	// doesn't get double-handled.
	if isLeavingPermEvent(event) {
		if cp, ok := ctx["perm"].(*gameengine.Permanent); ok && cp != nil {
			seatIdx := cp.Controller
			if seatIdx < 0 || seatIdx >= len(gs.Seats) {
				seatIdx = 0
			}
			collect(seatIdx, cp)
		}
	}

	if len(hitsBySeat) == 0 {
		// r60 TriggerCompleteness fix: emit a synthetic
		// trigger_evaluated marker before the zero-hits early
		// return. Sister to the trigger_depth / trigger_total
		// cap-path synthetic emissions above (lines 442-452,
		// 457-467) and the per-permanent emission below (lines
		// 521-525) — all OTHER return paths in fireTrigger emit
		// trigger_evaluated; this one was the silent gap.
		//
		// The invariant checkTriggerCompleteness consults
		// HasTriggerHook to identify trigger-bearer presence on
		// the battlefield, then looks for a follow-up trigger
		// event. When HasTriggerHook reports the bearer has the
		// hook but fireTrigger finds zero registered handlers
		// (handler registered against a different event alias,
		// per_card handler de-registered between Begin and
		// dispatch, name-canonicalisation mismatch between
		// HasTriggerHook's lookup keys and the registry's
		// onTrigger keys), the invariant fires the
		// "death event but no subsequent trigger event found"
		// false-positive. The synthetic emit closes that gap.
		//
		// Surfaced by the layer-stress 1000-game seed-42 sweep
		// (PR #735): game 467 turn 47, Daxos Blessed by the Sun
		// on the battlefield, sba_704_5f death event at index
		// 2537, no trigger-evaluated emission — TriggerCompleteness
		// false-positive against an SBA mass-death cascade.
		gs.LogEvent(gameengine.Event{
			Kind:   "trigger_evaluated",
			Seat:   -1,
			Target: -1,
			Source: event,
			Details: map[string]interface{}{
				"event":  event,
				"capped": "no_handlers",
				"rule":   "603.3",
			},
		})
		return
	}

	// Per-turn dispatch budget — counted ONLY for dispatches that found
	// handlers. The r63 liveness firehose caught a Rebel token storm
	// burning the whole 2000 budget with ZERO-handler token_created /
	// permanent_etb dispatches (capped:no_handlers on every line), after
	// which every REAL per_card trigger in the game (Rhystic, Blood
	// Artist, ...) was silently swallowed for the rest of the turn —
	// cross-event starvation. Zero-handler dispatches cannot loop
	// through handlers, so they cost nothing; the budget exists to stop
	// HANDLER loops and now counts exactly those.
	gs.Flags["trigger_total"]++
	if gs.Flags["trigger_total"] > 2000 {
		gameengine.LogLoopGuardFired(gs, "percard_dispatch_total_cap", map[string]interface{}{
			"total": gs.Flags["trigger_total"], "event": event,
		})
		gs.LogEvent(gameengine.Event{
			Kind:   "trigger_evaluated",
			Seat:   -1,
			Target: -1,
			Source: event,
			Details: map[string]interface{}{
				"event":  event,
				"capped": "trigger_total",
				"rule":   "603.3",
			},
		})
		return
	}

	// Push triggers in APNAP order (CR §101.4).
	// Active player's triggers go on stack FIRST (resolve LAST due to LIFO).
	apnap := gameengine.APNAPOrder(gs)
	usesStack := !isManaAbilityEvent(event)
	for _, seatIdx := range apnap {
		hits, ok := hitsBySeat[seatIdx]
		if !ok {
			continue
		}
		for _, h := range hits {
			// Log that trigger handlers were evaluated for this permanent.
			// Even if the handler's internal condition didn't fire (e.g.
			// Vindictive Vampire only fires on YOUR creatures dying),
			// the engine DID dispatch the trigger — TriggerCompleteness
			// needs to see this follow-up event to avoid false positives.
			gs.LogEvent(gameengine.Event{
				Kind:   "trigger_evaluated",
				Seat:   h.perm.Controller,
				Target: -1,
				Source: h.perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"event":     event,
					"card_name": h.perm.Card.DisplayName(),
					"rule":      "603.3",
				},
			})
			for _, handler := range h.hs {
				if gs.Flags != nil && gs.Flags["ended"] == 1 {
					return
				}
				if usesStack {
					// Push to stack instead of resolving immediately (CR §603.3).
					// PushPerCardTrigger handles the priority round and resolution
					// inline, matching the PushTriggeredAbility pattern.
					gameengine.PushPerCardTrigger(gs, h.perm, handler, ctx)
				} else {
					// Mana ability — resolve immediately without using the stack.
					handler(gs, h.perm, ctx)
				}
			}
		}
	}
}

// OwnsETBTrigger declares that cardName's OnETB handler fully
// implements its printed self-ETB triggered ability — the Judge r63
// double-fire gate suppresses the generic AST push for declared cards.
func (r *Registry) OwnsETBTrigger(cardName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ownsETBTrigger == nil {
		r.ownsETBTrigger = map[string]bool{}
	}
	r.ownsETBTrigger[normalizeName(cardName)] = true
}

// OwnsETB reports whether cardName declared self-ETB ownership. Uses
// the same lookupCandidates name resolution as dispatch so DFC full
// names ("Front // Back") match front-face registrations.
func OwnsETB(cardName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	for _, k := range lookupCandidates(cardName) {
		if reg.ownsETBTrigger[k] {
			return true
		}
	}
	return false
}

// HasETB reports whether any ETB handler is registered for cardName.
func HasETB(cardName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.etb[normalizeName(cardName)]) > 0
}

// HasResolve reports whether any Resolve handler is registered.
func HasResolve(cardName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.onResolve[normalizeName(cardName)]) > 0
}

// HasActivated reports whether any activated handler is registered.
func HasActivated(cardName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.activated[normalizeName(cardName)]) > 0
}

// RegisteredCardNames returns the deduplicated list of card names that have
// at least one handler registered (ETB, cast, resolve, activated, or trigger).
func (r *Registry) RegisteredCardNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]bool{}
	for k := range r.etb {
		seen[k] = true
	}
	for k := range r.onCast {
		seen[k] = true
	}
	for k := range r.onResolve {
		seen[k] = true
	}
	for k := range r.activated {
		seen[k] = true
	}
	for k := range r.onTrigger {
		seen[k] = true
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	return names
}

// HasCastAndTrigger reports whether cast or trigger handlers exist for a normalized name.
func (r *Registry) HasCastAndTrigger(normName string) (hasCast, hasTrigger bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hasCast = len(r.onCast[normName]) > 0
	hasTrigger = len(r.onTrigger[normName]) > 0
	return
}

// RegisteredTriggerEvents returns the set of canonical event names that have
// at least one OnTrigger handler registered (across all cards).
func (r *Registry) RegisteredTriggerEvents() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := map[string]bool{}
	for _, byEvent := range r.onTrigger {
		for ev := range byEvent {
			events[ev] = true
		}
	}
	return events
}

// HasTrigger reports whether a trigger handler is registered for cardName+event.
func HasTrigger(cardName, event string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	canonical := gameengine.NormalizeEventSingle(event)
	// Same lookupCandidates resolution as dispatch (DFC full names
	// match front-face registrations) — r63: Nicol Bolas, the Ravager
	// double-fired through the etb gate because the ownership query
	// missed the "Front // Back" corpus name.
	for _, k := range lookupCandidates(cardName) {
		if byEvent := reg.onTrigger[k]; byEvent != nil && len(byEvent[canonical]) > 0 {
			return true
		}
	}
	return false
}

// installEngineHooks wires our dispatch functions into gameengine's
// function-pointer hooks. Called from init() and Reset().
func installEngineHooks() {
	gameengine.ETBHook = fireETB
	gameengine.CastHook = fireOnCast
	gameengine.ResolveHook = fireOnResolve
	gameengine.ActivatedHook = fireActivated
	gameengine.TriggerHook = fireTrigger
	gameengine.HasTriggerHook = HasTrigger
	gameengine.HasETBHook = OwnsETB
	gameengine.HasActivatedHook = HasActivated
}

// registerDefaults is populated by handlers.go — the card-specific
// Register* functions append to Global(). Declared here so init() can
// call it before handlers.go's init runs (Go guarantees init order
// within a package by file-lexical order — safer to have one entry
// point).
func registerDefaults() {
	// Batch #1 (landed 2026-04-16 earlier in session).
	registerThassasOracle(Global())
	registerDemonicConsultation(Global())
	registerTaintedPact(Global())
	registerUnderworldBreach(Global())
	registerAetherfluxReservoir(Global())
	registerFoodChain(Global())
	registerDoomsday(Global())
	registerDisplacerKitten(Global())
	registerRhysticStudy(Global())
	registerMysticRemora(Global())
	registerCloudstoneCurio(Global())
	registerHullbreakerHorror(Global())
	registerNecroticOoze(Global())
	registerHermitDruid(Global())
	registerWalkingBallista(Global())
	// Batch #2 (this session). 15 more handlers.
	registerLaboratoryManiac(Global())
	registerJaceWielderOfMysteries(Global())
	registerAdNauseam(Global())
	registerPeregrineDrake(Global())
	registerPalinchron(Global())
	registerDeadeyeNavigator(Global())
	registerBolassCitadel(Global())
	registerAluren(Global())
	registerKarnTheGreatCreator(Global())
	registerKarganIntimidator(Global())
	registerManaCrypt(Global())
	registerDocksideExtortionist(Global())
	registerUrzaLordHighArtificer(Global())
	registerEmryLurkerOfTheLoch(Global())
	registerSenseisDiviningTop(Global())
	registerAngelsGrace(Global())
	registerSundialOfTheInfinite(Global())
	registerGrandAbolisher(Global())
	// Batch #3 (this session). 15 more handlers — mana-rock combo
	// pieces, static hate, reanimator staples.
	registerKinnanBonderProdigy(Global())
	registerBasaltMonolith(Global())
	registerGrimMonolith(Global())
	registerIsochronScepter(Global())
	registerDramaticReversal(Global())
	registerNullRod(Global())
	registerCollectorOuphe(Global())
	registerCursedTotem(Global())
	registerDrannithMagistrate(Global())
	registerOppositionAgent(Global())
	registerNecropotence(Global())
	registerGriselbrand(Global())
	registerRazakethTheFoulblooded(Global())
	// Deadeye Navigator is already registered in batch #2; we
	// upgraded its ETB handler in-place (soulbond auto-pairing).
	registerPhantasmalImage(Global())
	// Batch #4 — nightmare verification fixes.
	registerChainsOfMephistopheles(Global())
	registerIxidron(Global())
	registerPanglacialWurm(Global())
	registerMindslaver(Global())
	registerPerplexingChimera(Global())
	// Batch #5 — nightmare verification PARTIAL/FAIL promotions.
	registerSneakAttack(Global())
	registerSwordOfFeastAndFamine(Global())
	registerPactOfNegation(Global())
	registerSanguineBond(Global())
	registerExquisiteBlood(Global())
	registerTormentOfHailfire(Global())
	registerAggravatedAssault(Global())
	registerBumiUnleashed(Global())
	registerPortRazer(Global())
	registerStrionicResonator(Global())
	registerMirageMirror(Global())
	registerReleaseToTheWind(Global())
	// Batch #6 — Pact cycle: remaining 4 Pacts.
	registerPactOfTheTitan(Global())
	registerSlaughterPact(Global())
	registerInterventionPact(Global())
	registerSummonersPact(Global())
	// Batch #7 — portfolio staples: fetchlands, shocklands, tutors,
	// utility lands, removal, fast mana, card advantage.
	registerFetchlands(Global())
	registerShocklands(Global())
	registerDemonicTutor(Global())
	registerVampiricTutor(Global())
	registerMysticalTutor(Global())
	registerEnlightenedTutor(Global())
	registerWorldlyTutor(Global())
	// R60 batch 10: green ramp staples (sorcery-speed land tutors).
	registerRampantGrowth(Global())
	registerCultivate(Global())
	registerThreeVisits(Global())
	registerFarseek(Global())
	registerRagavan(Global())
	registerTheOneRing(Global())
	registerEternalWitness(Global())
	registerSylvanLibrary(Global())
	registerChromeMox(Global())
	registerMoxDiamond(Global())
	registerMoxAmber(Global())
	registerMoxOpal(Global())
	registerGemstoneCaverns(Global())
	registerBojukaBog(Global())
	registerOtawara(Global())
	registerBoseiju(Global())
	registerRoguesPassage(Global())
	registerReliquaryTower(Global())
	registerAncientTomb(Global())
	registerUrzasSaga(Global())
	registerPathToExile(Global())
	registerSwordsToPlowshares(Global())
	registerCyclonicRift(Global())
	// Batch #8 — critical gap: sac outlets, board wipes, counterspells,
	// cantrips. The cards that make Commander PLAY like Commander.
	// Sac outlets:
	registerAshnodsAltar(Global())
	registerPhyrexianAltar(Global())
	registerVisceraSeer(Global())
	registerCarrionFeeder(Global())
	registerAltarOfDementia(Global())
	registerGoblinBombardment(Global())
	registerYahenni(Global())
	registerWoeStrider(Global())
	// Board wipes:
	registerWrathOfGod(Global())
	registerDamnation(Global())
	registerToxicDeluge(Global())
	registerBlasphemousAct(Global())
	registerVanquishTheHorde(Global())
	registerFarewell(Global())
	registerAustereCommand(Global())
	registerChildOfAlara(Global())
	// Counterspells:
	registerNegate(Global())
	registerSwanSong(Global())
	registerDovinsVeto(Global())
	registerArcaneDenial(Global())
	registerDispel(Global())
	registerManaDrain(Global())
	// R60 batch 11: CMC-filtered + uncounterable counters.
	registerMentalMisstep(Global())
	registerSpellSnare(Global())
	registerMindbreakTrap(Global())
	registerCounterflux(Global())
	// Cantrips:
	registerBrainstorm(Global())
	registerPonder(Global())
	registerPreordain(Global())
	registerGitaxianProbe(Global())
	registerOpt(Global())
	registerConsider(Global())

	// Kraum — draw on opponent's 2nd spell each turn
	registerKraum(Global())
	// Yarok — ETB trigger doubler (Panharmonicon for all permanents)
	registerYarok(Global())
	// Yuriko — ninja commander trigger (reveal top, drain opponents)
	registerYuriko(Global())
	// Simic Basilisk — grant basilisk combat-damage destroy ability
	registerSimicBasilisk(Global())
	// Fynn, the Fangbearer — deathtouch creatures give 2 poison on combat damage
	registerFynn(Global())
	// Skithiryx, the Blight Dragon — infect dragon; {B} haste until EOT,
	// {B}{B} regeneration shield. Flying + infect via AST keyword pipeline.
	registerSkithiryxTheBlightDragon(Global())
	// Davros, Dalek Creator — opponents get 3 rad counters when a player loses
	registerDavros(Global())

	// Batch #10 — Treasure Nabber, chaos/cascade cards, Phylactery Lich.
	registerTreasureNabber(Global())
	registerPossibilityStorm(Global())
	registerChaosWand(Global())
	registerThousandYearStorm(Global())
	registerPhylacteryLich(Global())
	registerEyeOfVecna(Global())
	registerHandOfVecna(Global())
	registerBookOfVileDarkness(Global())

	// Batch #11 — aristocrat death-trigger cards.
	registerBloodArtist(Global())
	registerZulaportCutthroat(Global())
	registerBastionOfRemembrance(Global())
	registerCruelCelebrant(Global())
	registerVindictiveVampire(Global())
	registerSyrKonrad(Global())

	// Batch #9 — 23 commander per-card handlers for the deck collection.
	registerArdenn(Global())
	registerCoram(Global())
	registerFireLordAzula(Global())
	registerKaust(Global())
	registerLordOfExtinction(Global())
	registerLordOfTheNazgul(Global())
	registerMortivore(Global())
	registerMultaniMaroSorcerer(Global())
	registerLumra(Global())
	registerMaja(Global())
	registerMoraug(Global())
	registerMuldrotha(Global())
	registerNarsetEnlightenedExile(Global())
	registerObeka(Global())
	registerOloro(Global())
	registerRagost(Global())
	registerRal(Global())
	registerRiku(Global())
	registerSatya(Global())
	registerSoraya(Global())
	registerTergrid(Global())
	registerUlrich(Global())
	registerVarina(Global())
	registerVoja(Global())
	registerYshtola(Global())
	registerYarus(Global())
	registerAshling(Global())

	// Ragost combo pieces — infinite damage via artifact/food sacrifice loop.
	registerCrimeNovelist(Global())
	registerNukaColaVendingMachine(Global())
	registerPenregonStrongbull(Global())

	// Batch #12 — forced-discard spells and payoffs (Tergrid ecosystem).
	registerHymnToTourach(Global())
	registerMindTwist(Global())
	registerDarkDeal(Global())
	registerDeliriumSkeins(Global())
	registerSyphonMind(Global())
	registerNecrogenMists(Global())
	registerBottomlessPit(Global())
	registerRankleMasterOfPranks(Global())
	registerOppression(Global())
	registerWasteNot(Global())
	registerLilianasCaress(Global())
	registerMegrim(Global())
	registerTinybones(Global())

	// Batch #13 — sacrifice-forcers and stax pieces (Tergrid ecosystem).
	registerFleshbagMarauder(Global())
	registerMercilessExecutioner(Global())
	registerPlaguecrafter(Global())
	registerInnocentBlood(Global())
	registerSmallpox(Global())
	registerPox(Global())
	registerGravePact(Global())
	registerGraveBetrayal(Global())
	registerLivingDeath(Global())
	registerArchfiendOfDepravity(Global())
	registerDeathCloud(Global())
	registerVictimize(Global())
	registerVonasHunger(Global())

	// Batch #14 — stax lock pieces (7174n1c request).
	registerDefenseGrid(Global())
	registerNotionThief(Global())
	// Trinisphere: already wired in ScanCostModifiers by name, no per-card handler needed.

	// Batch #15 — high-impact commander staples that were skewing ELO.
	registerSolRing(Global())
	registerManaVault(Global())
	registerDarkRitual(Global())
	registerSmotheringTithe(Global())
	registerWindfall(Global())
	registerWheelOfFortune(Global())
	registerEntomb(Global())
	registerReanimate(Global())
	registerAnimateDead(Global())
	registerForceOfWill(Global())
	registerForceOfNegation(Global())
	registerFierceGuardianship(Global())
	registerLionsEyeDiamond(Global())
	registerDauthiVoidwalker(Global())
	registerYawgmothThranPhysician(Global())
	registerGildedDrake(Global())
	registerSurvivalOfTheFittest(Global())

	// Batch #16 — WotC Game Changers: the 25 highest-impact cards in
	// Commander that were missing handlers, causing cEDH decks to
	// underperform against precons.
	registerHumility(Global())
	registerTeferisProtection(Global())
	registerConsecratedSphinx(Global())
	registerGiftsUngiven(Global())
	registerIntuition(Global())
	registerNarsetParterOfVeils(Global())
	registerNarsetEnlightenedMaster(Global())
	registerBraidsCabalMinion(Global())
	registerImperialSeal(Global())
	registerOrcishBowmasters(Global())
	registerGamble(Global())
	registerJeskasWill(Global())
	registerBiorhythm(Global())
	registerCropRotation(Global())
	registerNaturalOrder(Global())
	registerSeedbornMuse(Global())
	registerAuraShards(Global())
	registerCoalitionVictory(Global())
	registerGrandArbiterAugustinIV(Global())
	registerGrunnTheLonelyKing(Global())
	registerFieldOfTheDead(Global())
	registerGaeasCradle(Global())
	registerGlacialChasm(Global())
	registerMishrasWorkshop(Global())
	registerPanopticMirror(Global())
	registerSerrasSanctum(Global())
	registerTabernacleAtPendrellVale(Global())

	// R55 — mana-pool primitive cards (see mana_pool_primitive_r55.go).
	// Three-facet triad: exempt-from-drain, per-X mana add, and spend-
	// restricted mana — all data-driven via the new ManaPoolExemption /
	// AddManaPerCount / AddRestrictedMana primitives.
	registerOmnathLocusOfMana(Global())
	registerUpwelling(Global())
	registerCabalCoffers(Global())
	registerSanctumWeaver(Global())
	registerAncientZiggurat(Global())
	// R57 — three new mana-primitive ports. (Gaea's Cradle + Serra's
	// Sanctum already registered upstream; R57 refactors their bodies
	// in game_changers.go to use AddManaPerCount.)
	registerAshlingFlameDancerManaExemption(Global())
	registerMagusOfTheCoffers(Global())
	registerCavernOfSouls(Global())
	// r63 mana-pool audit — Leyline Tyrant red-retention exemption +
	// self-dies "pay any amount of {R}, deal that much" payoff.
	registerLeylineTyrant(Global())
	// r63 mana-pool audit follow-up — "if you would lose unspent mana,
	// that mana becomes colorless instead" converter (Horizon Stone;
	// Kruphix's fourth line uses the same engine primitive).
	registerHorizonStone(Global())
	// R58 — counter-accumulator-threshold primitive + 4 ports.
	registerHelixPinnacle(Global())
	registerDarksteelReactor(Global())
	registerAzorsElocutors(Global())
	registerQuestForUlasTemple(Global())

	// r60 batch AJ — 5 high-impact win-the-game / lose-the-game cards
	// wired through the §704.5c master-check pattern (#687).
	registerApproachOfTheSecondSun(Global())
	registerFelidarSovereign(Global())
	registerTestOfEndurance(Global())
	registerMayaelsAria(Global())
	registerTriskaidekaphobia(Global())

	// Batch #17 — token-creation triggers and aristocrat payoffs.
	// Anointed Procession's token_created trigger is upgraded in
	// batch17_sweep.go (registered via its own init()).
	registerChatterfang(Global())
	registerPitilessPlunderer(Global())
	registerSlimefootAndSquee(Global())

	// Batch #18 — drain commanders (lifegain→drain + ETB→drain).
	registerDinaSoulSteeper(Global())
	registerVitoThornOfTheDuskRose(Global())
	registerVitoFanaticOfAclazotz(Global())
	registerMaraudingBlightPriest(Global())
	registerCorpseKnight(Global())

	// Batch #19 — enchantment toolbox commanders (attack-trigger tutors).
	registerZurTheEnchanter(Global())
	// Batch #19 — artifact-commander synergy.
	registerDrafna(Global())
	registerJolene(Global())

	// Batch #20 — high-drift partner/toolbox commanders.
	registerThrasios(Global())
	registerVialSmasher(Global())

	// Batch #21 — Phase 5 drift commanders.
	registerAbdelAdrian(Global())
	registerMeria(Global())
	registerRaffine(Global())

	// Batch #22 — B5 partner commanders.
	registerTymna(Global())
	registerMalcolm(Global())
	registerEtali(Global())
	registerKrark(Global())

	// Batch #23 — B5 value commanders.
	registerTivit(Global())
	registerObNixilisCaptive(Global())
	registerMeren(Global())
	registerMizzixOfTheIzmagnus(Global())
	registerSelvala(Global())

	// Batch #24 — B5 combo/value commanders.
	registerElsha(Global())
	registerEmiel(Global())
	registerEdgarMarkov(Global())
	registerAtraxaGrandUnifier(Global())
	registerTayam(Global())
	registerArcumDagsson(Global())

	// Batch #25 — B5 commanders.
	registerAtraxaPraetorsVoice(Global())
	registerQueza(Global())
	registerFrancisco(Global())
	registerSidarKondo(Global())
	registerBladewing(Global())
	registerDihada(Global())
	registerRowanScion(Global())
	registerTeveshSzat(Global())
	registerOjerAxonil(Global())
	registerZurgoOjutai(Global())

	// Batch #26 — high-impact tutor / token / ninjutsu commanders.
	registerSisayWeatherlightCaptain(Global())
	registerKrenkoMobBoss(Global())
	registerSatoruUmezawa(Global())
	// Batch #26 — Spacecraft station, DFC saga commander, Merfolk tribal.
	registerHearthhull(Global())
	registerTerra(Global())
	registerHakbal(Global())

	// Batch #26 (cont) — newer commanders: death-trigger growth, discard
	// copy, graveyard recast.
	registerGorma(Global())
	registerHashaton(Global())
	registerShiko(Global())

	// Batch #26 (cont) — full-coverage push: Elder Dragon Cycle (B/G + U/R)
	// + planeswalker commander.
	registerWitherbloom(Global())
	registerPrismari(Global())
	registerQuintorius(Global())

	// Batch #26 (cont) — Elder Dragon historian, lifegain pest anthem,
	// -1/-1 counter trigger goblin.
	registerLorehold(Global())
	registerBlech(Global())
	registerAuntieOol(Global())

	// r60 — unified WardCost test cards (ward_alt_payment.go refactor,
	// 2026-05-27). Charging War Boar covers Ward—Pay N life; Terror of
	// the Peaks covers the non-keyword damage-on-target structural shape.
	registerChargingWarBoar(Global())
	registerTerrorOfThePeaks(Global())

	// Batch #27 — staple commanders: attack-trigger cheat-in,
	// counter-scaling cost reduction, global P/T swap.
	registerKaaliaOfTheVast(Global())
	registerAnimarSoulOfElements(Global())
	registerDoranTheSiegeTower(Global())
	// Arixmethes — slumber-counter land/creature transformer.
	registerArixmethesSlumberingIsle(Global())

	// Batch #27 (cont) — Dinosaur ETB discover, attack-life-gradient draw,
	// opp-creature exile-instead-of-die / end-step Rat tokens.
	registerPantlaza(Global())
	registerBreena(Global())
	registerVren(Global())

	// Batch #27 (cont) — high-impact ETB/activated/trigger commanders:
	// Sauron (amass + Ring tempt), Kenrith (5-mode activated),
	// Niv-Mizzet Reborn (guild-pair reveal-10).
	registerSauronDarkLord(Global())
	registerKenrithReturnedKing(Global())
	registerNivMizzetReborn(Global())

	// Batch #27 (cont) — extra-draw / DFC reveal-cheat / Eldrazi copy.
	registerNekusar(Global())
	registerEsika(Global())
	registerUlalek(Global())

	// Batch #27 (cont) — cost-reducer (creature-target), top-of-library
	// MV>=4 cast + upkeep surveil, X-spell counter scaler.
	registerKillianInkDuelist(Global())
	registerGlarbCalamitysAugur(Global())
	registerZimoneInfiniteAnalyst(Global())

	// Batch #28 — beginning-of-combat creature-count gradient, ETB-draw +
	// discard drain, graveyard/exile recast treasure engine.
	registerTeval(Global())
	registerMorcant(Global())
	registerGolbez(Global())

	// Batch #28 (cont) — {0}-cost partner stub, library-exile combat
	// trigger, copy-spell-and-bounce-self activated commander.
	registerRograkh(Global())
	registerAshlingTheLimitless(Global())
	registerRootha(Global())

	// Batch #28 (cont) — prowess team-anthem + cast-trigger unblockable,
	// Dragon-tribal ETB/attack reveal-cheat, life-delta end-step double
	// payoff (counter buff + reanimate).
	registerBriaRiptideRogue(Global())
	registerUreniOfTheUnwritten(Global())
	registerBetorAncestorsVoice(Global())

	// Batch #28 (cont) — combat-payoff Tarkir dragon-clan commander,
	// FF DFC drain/transform/sac-draw, Rat-tribal token swarm.
	registerEshkiDragonclaw(Global())
	registerSephiroth(Global())
	registerMarrowGnawer(Global())

	// Batch #28 (cont) — combat-damage free-cast from graveyard,
	// lifegain end-step draw + Cleric token activation, second-draw
	// Rabbit token + token-count draw activation.
	registerViviOrnitier(Global())
	registerBertaWiseExtrapolator(Global())
	registerMsBumbleflower(Global())

	// Batch #29 — spell-copy commanders + DFC discard/transform.
	// Alania (first instant/sorcery/Otter copy w/ opponent draw),
	// Mica (sac-artifact instant/sorcery copy),
	// Kefka DFC (ETB/attack wheel + transform + opp-life-loss draw).
	registerAlania(Global())
	registerMica(Global())
	registerKefka(Global())

	// Batch #29 (cont) — graveyard-recast token-swarm + ward + surveil
	// (Saruman), multicolored-cast WURG modal payoff (Aragorn),
	// noncreature-cast Spirit token + sac-for-red ritual (Kykar).
	registerSarumanTheWhiteHand(Global())
	registerAragornTheUniter(Global())
	registerKykarWindsFury(Global())

	// Batch #29 (cont) — token-on-attack + combat-death-draw soldier
	// commander (Caesar), ETB-draw + legendary-spell scry artificer
	// (Kilo), combat-damage-toughness lifegain partner snake (Ikra).
	registerCaesarLegionsEmperor(Global())
	registerKiloApogeeMind(Global())
	registerIkraShidiqi(Global())

	// Batch #29 (cont) — Jund blitz cost-reducer, mono-B life-for-mana
	// snowballer, GW counter-spreader attack trigger.
	registerHenzieToolboxTorre(Global())
	registerKrrikSonOfYawgmoth(Global())
	registerSovereignOkinecAhau(Global())

	// Batch #29 (cont) — creature-spell-cast scaler + power-X mana
	// (Helga), owner-bounce-on-death w/ pay-3-life (Athreos), first
	// life-loss-per-opponent-turn growth + draw (Valgavoth).
	registerHelgaSkittishSeer(Global())
	registerAthreosGodOfPassage(Global())
	registerValgavothHarrowerOfSouls(Global())

	// Batch #30 — high-deck-count commanders missing handlers:
	// Brago combat-blink (any number of nonland permanents you control),
	// Korvold ETB/attack sac + sacrifice payoff (counter + draw).
	// Prismari, the Inspiration storm grant is already wired in batch #26.
	registerBragoKingEternal(Global())
	registerKorvoldFaeCursedKing(Global())
	// Batch #30 (cont) — FF Alien combat-counter / Mutant-death draw,
	// Elder Dragon DFC ETB-discard / activate-transform-to-planeswalker,
	// Zombie tribal death-token-with-decayed / end-step sac-draw.
	registerJenovaAncientCalamity(Global())
	registerNicolBolas(Global())
	registerWilheltTheRotcleaver(Global())
	// Batch #30 (cont) — high-deck-count commanders missing handlers:
	// Jodah legendary-cast cascade-into-legendary-nonland, Umbris
	// nightmare/horror ETB exile-from-opp-library-until-land, The
	// Necrobloom landfall token (Plant or Zombie at 7+ distinct lands).
	registerJodahTheUnifier(Global())
	registerUmbrisFearManifest(Global())
	registerTheNecrobloom(Global())

	// Batch #30 (cont) — Fallout dice/treasure activated commander,
	// FF7 Melee + 7+-power damage untap & extra-combat trigger,
	// Ashling Rekindled // Rimebound modal DFC alternation loot/ramp.
	registerMrHousePresidentAndCEO(Global())
	registerTifaMartialArtist(Global())
	registerAshlingRekindled(Global())

	// Batch #30 (cont) — oracle-text correctness sweep for high-deck-count
	// commanders whose original handlers in batches #9 and #26 were built
	// from inaccurate descriptions. The register* calls themselves remain
	// in their batch-of-record above; the function bodies were rewritten in
	// dedicated files (fire_lord_azula.go, yshtola_nights_blessed.go,
	// witherbloom.go) and stubbed out of commanders_batch.go. Rewrites:
	//   - Fire Lord Azula: firebending 2 + spell-copy while attacking
	//     (was: noncreature spell during combat → 1/1 token).
	//   - Y'shtola, Night's Blessed: vigilance + end-step 4-life-loss
	//     draw + noncreature MV3+ ping-each-opp/gain-2 (was: ETB draw
	//     equal to power).
	//   - Witherbloom, the Balancer: affinity-for-creatures cost
	//     reduction wired through cost_modifiers.go for both Witherbloom
	//     herself and the granted instant/sorcery clause.

	// Batch #30 (cont) — Fallout rad-counter Insect Mutant (Mothman ETB +
	// attack rad-each-player), Final Fantasy equipment-attach + power-7
	// treasure attack (Cloud), and the classic 5C Dragon eminence
	// cost-reducer + group-attack draw + permanent-from-hand cheat
	// (The Ur-Dragon).
	registerTheWiseMothman(Global())
	registerCloudExSoldier(Global())
	registerTheUrDragon(Global())

	// Batch #31 — under-supported commanders surfaced by deck-coverage
	// audit: Phyrexian sacrifice growth (Szarel), mobilize attack-token
	// commander with combat LTB drain/draw (Zurgo Stormrender), AC
	// crossover treasure-on-tapped-tribe + Vehicle exile/play
	// (Edward Kenway).
	registerSzarelGenesisShepherd(Global())
	registerZurgoStormrender(Global())
	registerEdwardKenway(Global())

	// Batch #31 (cont) — Bloomburrow draw-or-discard frog with extra
	// land drop, Strixhaven prepare DFC bird bard (creature-cast →
	// +1/+1 counter via Heroic Stanza copy), Assassin's Creed attack-
	// trigger graveyard-exile-and-copy swarm.
	registerFlubsTheFool(Global())
	registerAbigalePoetLaureate(Global())
	registerAltairIbnLaAhad(Global())

	// Batch #32 — high-deck-count commanders missing handlers:
	// Najeela (Warrior tribal token spawn + WUBRG extra-combat untap),
	// Winota (non-Human attack → top-six Human cheat),
	// Prosper (ETB/attack impulse exile + end-step drain & treasure).
	registerNajeelaBladeBlossom(Global())
	registerWinotaJoinerOfForces(Global())
	registerProsperTomeBound(Global())

	// Batch #32 (cont) — three more commanders surfaced by the deck-
	// coverage audit: Scriv, the Obligator (W/B Aetherdrift Inkling — ETB
	// + attack Contract Aura token attached to opponent creature), Gonti,
	// Canny Acquisitor (Sultai cost-reduction for unowned spells +
	// combat-damage exile/play with any-mana clause), Gishath, Sun's
	// Avatar (Naya Dinosaur tribal — combat-damage reveal-N and put
	// dinosaur creature cards onto the battlefield).
	registerScrivTheObligator(Global())
	registerGontiCannyAcquisitor(Global())
	registerGishathSunsAvatar(Global())

	// Batch #32 — high-deck-count commander handlers (oracle-text-correct):
	// Killian, Decisive Mentor (NOT Killian, Ink Duelist) — enchantment ETB
	// tap+goad + aura-attacker draw; Doran, Besieged by Time (NOT Doran,
	// the Siege Tower) — toughness>power cost reduction + +X/+X attack
	// buff where X = T-P; Leonardo, the Balance (TMNT crossover) —
	// once-per-turn +1/+1 spread on token ETB + WUBRG menace/trample/
	// lifelink team grant.
	registerKillianDecisiveMentor(Global())
	registerDoranBesiegedByTime(Global())
	registerLeonardoTheBalance(Global())

	// Batch #32 (cont) — three more commanders surfaced by deck-coverage
	// audit: Urza, Chief Artificer (NOT Urza, Lord High Artificer) —
	// affinity-for-artifact-creatures cost reducer + artifact-creature
	// menace grant + end-step Construct token; Lathril, Blade of the Elves
	// (Elf tribal Combat-damage Elf Warrior tokens + tap-ten-Elves drain
	// activated); The Gitrog, Ravenous Ride (NOT The Gitrog Monster) —
	// Mount with Saddle 1, combat-damage sacrifice-saddler / draw X /
	// fetch X lands tapped.
	registerUrzaChiefArtificer(Global())
	registerLathrilBladeOfTheElves(Global())
	registerGitrogRavenousRide(Global())

	// Batch #32 (cont) — under-supported transforming commanders:
	// MOM saga DFC praetor with combat-damage punishment trigger and
	// {2}{W} + sac-three-creatures activated transform (Elesh Norn //
	// The Argent Etchings); Cat-tribal flip planeswalker — ETB token,
	// Cat-death may-transform, +2 counter spread / 0 token+damage /
	// -4 selective opponent sweep (Ajani, Nacatl Pariah // Ajani,
	// Nacatl Avenger); 4C cascade-on-combat-damage commander — grants
	// hand-cast cascade until end of turn (Yidris, Maelstrom Wielder).
	registerEleshNornArgentEtchings(Global())
	registerAjaniNacatlPariah(Global())
	registerYidrisMaelstromWielder(Global())

	// Batch #33 — high-deck-count commanders missing handlers:
	// Syr Gwyn, Hero of Ashvale (Mardu Knight/Equipment commander —
	// equip-{0} static while you control a Knight + equipped-attacker
	// draw card / drain each opponent 1), Riku of Two Reflections
	// (Temur copy commander — pay {2}{G/U} to token-copy a creature
	// spell or copy an instant/sorcery on the stack). Brago, King
	// Eternal already wired in Batch #30.
	registerSyrGwynHeroOfAshvale(Global())
	registerRikuOfTwoReflections(Global())

	// Batch #33 (cont) — three more commanders surfaced by the deck-
	// coverage audit: Lord Windgrace (BRG land-recursion planeswalker
	// commander — discard-for-land/draw, graveyard-land reanimate, and
	// nonland-wipe + library-land-fetch ultimate), Ghave, Guru of Spores
	// (Abzan +1/+1-counter & Saproling generator — ETB with five +1/+1
	// counters, counter-for-Saproling and sac-for-counter activations),
	// Jhoira, Weatherlight Captain (Esper artifact-historic cantrip
	// commander — draw on each historic spell cast).
	registerLordWindgrace(Global())
	registerGhaveGuruOfSpores(Global())
	registerJhoiraWeatherlightCaptain(Global())
	registerTheGitrogMonster(Global())

	// Batch #33 (cont) — three more commanders surfaced by the deck-
	// coverage audit: Jin Sakai, Ghost of Tsushima (Esper Samurai —
	// combat-damage draw + lone-attacker modal Standoff/Ghost), Wan Shi
	// Tong, All-Knowing (mono-U Bird Spirit — ETB tuck nonland to bottom
	// + library-add Spirit token doubler), Ashnod, Flesh Mechanist
	// (mono-B Artificer — attack sac → tapped Powerstone + {5}+exile-
	// creature-card activated 3/3 Zombie artifact creature token).
	registerJinSakaiGhostOfTsushima(Global())
	registerWanShiTongAllKnowing(Global())
	registerAshnodFleshMechanist(Global())

	// Batch #33 (cont) — three more commanders surfaced by deck-coverage
	// audit: Zask, Skittering Swarmlord (mono-G/G Insect graveyard
	// recursion — graveyard land/Insect cast permission + another-Insect-
	// dies bottom-of-library + mill-2 trigger + {1}{B/G} +1/+0 deathtouch
	// activated), Kibo, Uktabi Prince (R/G Monkey Noble — {T}-create-Banana-
	// for-each-player + opponent-artifact-dies Ape/Monkey buff + attack-
	// force-sac-artifact), Frodo, Adventurous Hobbit (W/B Halfling Scout —
	// partner with Sam, vigilance, gained-3+-life attack-trigger Ring tempt
	// with Ring-bearer ≥2-tempt draw payoff).
	registerZaskSkitteringSwarmlord(Global())
	registerKiboUktabiPrince(Global())
	registerFrodoAdventurousHobbit(Global())

	// Batch #33 (cont) — Grist, the Hunger Tide (Golgari planeswalker/
	// insect commander — Insect token + mill-loop +1, sac-creature →
	// destroy -2, creature-graveyard drain -5), The Infamous Cruelclaw
	// (Rakdos Weasel Mercenary — combat-damage exile-until-nonland +
	// alt cost discard-instead-of-mana cast permission), Rivaz of the
	// Claw (Rakdos Lizard Warlock — typed-mana Dragon-only ramp,
	// once-per-turn Dragon graveyard cast permission with exile-on-die
	// rider).
	registerGristTheHungerTide(Global())
	registerInfamousCruelclaw(Global())
	registerRivazOfTheClaw(Global())

	// Batch #33 (cont) — Final Fantasy + March of the Machine commanders
	// surfaced by the deck-coverage audit: Ardbert, Warrior of Darkness
	// (FF14 Orzhov Spirit Warrior — color-cast +1/+1 spread on legendaries
	// + vigilance/menace UEOT), Terra, Herald of Hope (FF6 Mardu Wizard
	// Warrior — Trance combat-begin self-mill 2 + flying UEOT, combat-
	// damage may-pay {2} return ≤3-power creature card from graveyard
	// tapped), Sidar Jabari of Zhalfir (Esper Human Knight — Eminence
	// attack-with-Knight loot + combat-damage Knight reanimate).
	registerArdbertWarriorOfDarkness(Global())
	registerTerraHeraldOfHope(Global())
	registerSidarJabariOfZhalfir(Global())

	// Batch #33 (cont) — three more high-deck-count commanders surfaced
	// by deck-coverage scan: Primo, the Unbounded ({X}{G}{G}{U} Fractal
	// Wolf — ETB with twice-X +1/+1 counters + base-power-0 combat
	// damage spawns a counter-loaded Fractal token); Karlach, Fury of
	// Avernus ({4}{R} Tiefling Barbarian — first-combat untap + first-
	// strike grant + extra combat); Hinata, Dawn-Crowned ({1}{U}{R}{W}
	// Kirin Spirit — symmetric per-target cost reduction/tax wired into
	// ScanCostModifiers).
	registerPrimoTheUnbounded(Global())
	registerKarlachFuryOfAvernus(Global())
	registerHinataDawnCrowned(Global())

	// Anje Falkenrath — haste loot ({T}, discard → draw) + madness-discard
	// untap trigger enabling the Anje madness loop.
	registerAnjeFalkenrath(Global())

	// Ayara, First of Locthwain — black-creature-ETB drain (each opp -1,
	// you +1) + {T}, sac-another-creature: draw a card.
	registerAyaraFirstOfLocthwain(Global())

	// Magus Lucea Kane — upkeep scry 1 + {T}: Add {C}{C} (X-spell copy partial).
	registerMagusLuceaKane(Global())

	// Ovika, Enigma Goliath — flying + ward {3}/pay-3-life; noncreature-
	// spell-cast creates X 1/1 red Phyrexian Goblin tokens (X = spell's MV).
	registerOvikaEnigmaGoliath(Global())

	// Shalai and Hallar — flying/vigilance legend; +1/+1 counter on
	// any creature you control → deal that much damage to each opponent.
	registerShalaiAndHallar(Global())

	// Xyris, the Writhing Storm — RUG flying legend; whenever an opponent
	// draws a card except the first one they draw in each of their draw
	// steps, create a 1/1 green Snake creature token.
	registerXyrisTheWrithingStorm(Global())

	// Alela, Cunning Conqueror — first-spell-on-opponent's-turn creates 1/1
	// black Faerie Rogue with flying; Faerie combat damage triggers tap/untap
	// of target nonland permanent.
	registerAlelaCunningConqueror(Global())

	// Anowon, the Ruin Thief — Rogue lord (+1/+1 ETB) + combat-damage
	// mill-X / draw-if-creature-milled trigger.
	registerAnowonTheRuinThief(Global())

	// Braids, Arisen Nightmare — end-step sac (artifact/creature/enchantment/
	// land/planeswalker): each opponent may match-type sac or lose 2 + you draw.
	registerBraidsArisenNightmare(Global())

	// Bhaal, Lord of Murder — conditional indestructible (life <= half start)
	// + nontoken-creature-dies: +1/+1 counter + goad opponent creature.
	registerBhaalLordOfMurder(Global())

	// Sefris of the Hidden Ways — once-per-turn venture on creature card to
	// graveyard (dies or discard); Create Undead reanimation on dungeon
	// completion (highest-CMC creature from graveyard to battlefield).
	registerSefrisOfTheHiddenWays(Global())

	// Brudiclad, Telchor Engineer — UR artifact/token commander: beginning-of-
	// combat creates 1/1 blue Phyrexian Myr artifact creature token, then may
	// copy chosen token across all other tokens the controller controls.
	// Continuous haste-grant to creature tokens handled by layers pipeline
	// (emitPartial).
	registerBrudicladTelchorEngineer(Global())

	// Arahbo, Roar of the World — GW Cat eminence commander: beginning-of-
	// combat +3/+3 to another Cat (eminence; command zone not tracked) +
	// optional {1}{G}{W} on Cat attack for trample and +X/+X where X = power.
	registerArahboRoarOfTheWorld(Global())

	// Ezuri, Claw of Progress — GU Elf Warrior: small-creature-ETB grants
	// one experience counter; beginning-of-combat puts X +1/+1 counters on
	// another target creature, where X = experience counters on the seat.
	registerEzuriClawOfProgress(Global())

	// Anafenza, the Foremost — WBG Human Soldier: attack trigger puts a
	// +1/+1 counter on another tapped creature you control (best target =
	// highest-power); death-replacement exile clause is emitPartial.
	registerAnafenzaTheForemost(Global())

	// Heliod, Sun-Crowned — indestructible mono-W enchantment god; lifegain
	// trigger puts a +1/+1 counter on a target creature or enchantment you
	// control; {1}{W} lifelink-grant is emitPartial (UEOT layers gap).
	registerHeliodSunCrowned(Global())

	// Zaxara, the Exemplary — BUG Nightmare Hydra legend; deathtouch +
	// {T}: add two mana (mana system); X-cost spell cast creates a 0/0
	// green Hydra token then puts X +1/+1 counters on it.
	registerZaxaraTheExemplary(Global())

	// Niv-Mizzet, Parun — uncounterable flying Izzet dragon; draw→1 damage
	// ping + any-player instant/sorcery cast→draw (creates draw-damage chain).
	registerNivMizzetParun(Global())

	// Muldrotha, the Gravetide — full ETB + upkeep per-type tracking reset.
	registerMuldrothaTheGravetide(Global())

	// Teysa Karlov — death trigger doubler + token vigilance/lifelink.
	registerTeysaKarlov(Global())

	// The First Sliver — cascade keyword (AST) + grants cascade to Sliver
	// spells cast by its controller.
	registerTheFirstSliver(Global())

	// Myrel, Shield of Argive — opponent silence during your turn + Soldier tokens on attack.
	registerMyrelShieldOfArgive(Global())

	// Light-Paws, Emperor's Voice — ETB-triggered Aura tutor.
	registerLightPawsEmperorsVoiceETB(Global())

	// Strefan, Maurer Progenitor — Blood tokens on end step + Vampire cheat on attack.
	registerStrefanMaurerProgenitor(Global())

	// Worker-generated handlers (round 1 + round 2 + DFC cleanup).
	registerAangAndKatara(Global())
	registerAcademyManufactor(Global())
	registerAerithGainsborough(Global())
	registerAishaOfSparksAndSmoke(Global())
	registerAnointedProcession(Global())
	registerApheliaViperWhisperer(Global())
	registerAsForetold(Global())
	registerAtemsisAllSeeing(Global())
	registerAthreosShroudVeiled(Global())
	registerAvatarAang(Global())
	registerAyeshaTanakaArmorer(Global())
	registerAyulaQueenAmongBears(Global())
	registerAzizaMageTowerCaptain(Global())
	registerAzlaskSwellingScourge(Global())
	registerBalmorBattlemageCaptain(Global())
	registerBaralAndKariZev(Global())
	registerBaylenTheHaymaker(Global())
	registerBelakorDarkMaster(Global())
	registerBetorKinToAll(Global())
	registerBlackMarketConnections(Global())
	registerBladewingDeathlessTyrant(Global())
	registerBlimComedicGenius(Global())
	registerBoneMiser(Global())
	registerBraidOfFire(Global())
	registerBraidsConjurerAdept(Global())
	registerBreOfClanStoutarm(Global())
	registerBruceBanner(Global())
	registerBruseTarlBoorishHerder(Global())
	registerBugenhagenWiseElder(Global())
	registerBurakosPartyLeader(Global())
	registerCagedSun(Global())
	registerCaitCageBrawler(Global())
	registerCalixGuidedByFate(Global())
	registerCaptainHowlerSeaScourge(Global())
	registerCarmenCruelSkymarcher(Global())
	registerChainerNightmareAdept(Global())
	registerChandraFireOfKaladesh(Global())
	registerChiefOfTheFoundry(Global())
	registerChishiroTheShatteredBlade(Global())
	registerChronozoa(Global())
	registerClavilenoFirstBlessed(Global())
	registerCloudPlanetsChampion(Global())
	registerCoatOfArms(Global())
	registerColfenorTheLastYew(Global())
	registerCommodoreGuff(Global())
	registerCormelaGlamourThief(Global())
	registerCourtOfEmbereth(Global())
	registerCourtOfVantress(Global())
	registerCraigBooneNovacGuard(Global())
	registerCryptbreaker(Global())
	registerDarettiScrapSavant(Global())
	registerDarkLeoAndShredder(Global())
	registerDaxosBlessedByTheSun(Global())
	registerDeathBaron(Global())
	registerDiregrafCaptain(Global())
	registerDiregrafColossus(Global())
	registerDonAndresTheRenegade(Global())
	registerDoorOfDestinies(Global())
	registerDragonmasterOutcast(Global())
	registerDrEggman(Global())
	registerDrMadisonLi(Global())
	registerDyadrineSynthesisAmalgam(Global())
	registerEdeaPossessedSorceress(Global())
	registerEdELonesomeEyebot(Global())
	registerElasIlKor(Global())
	registerElminster(Global())
	registerElrondMasterOfHealing(Global())
	registerElugeTheShorelessSea(Global())
	registerEmetSelch(Global())
	registerEmmaraSoulOfTheAccord(Global())
	registerEndlessRanks(Global())
	registerEowynShieldmaiden(Global())
	registerErisRoarOfTheStorm(Global())
	registerEshkiTemursRoar(Global())
	registerEtrataDeadlyFugitive(Global())
	registerEtrataTheSilencer(Global())
	registerExcavaTheRisenPast(Global())
	registerExtravagantReplication(Global())
	registerFinneasAceArcher(Global())
	registerFoundryInspector(Global())
	registerGargosViciousWatcher(Global())
	registerGarthOneEye(Global())
	registerGauntletOfPower(Global())
	registerGenerousPlunderer(Global())
	registerGerrardWeatherlightHero(Global())
	registerGeth(Global())
	registerGisaGloriousResurrector(Global())
	registerGisaTheHellraiser(Global())
	registerGixYawgmothPraetor(Global())
	registerGlissaSunslayer(Global())
	registerGornogTheRedReaper(Global())
	registerGoroGoroAndSatoru(Global())
	registerGoShintaiLifesOrigin(Global())
	registerGraaz(Global())
	registerGreensleevesMaroSorcerer(Global())
	registerGrevenPredatorCaptain(Global())
	registerGrubStoriedMatriarch(Global())
	registerGwenomRemorseless(Global())
	registerGwenStacyGhostSpider(Global())
	registerHalanaAndAlenaPartners(Global())
	registerHazelOfTheRootbloom(Global())
	registerHenryWuIngenGeneticist(Global())
	registerHerigastEruptingNullkite(Global())
	registerHermesOverseerOfElpis(Global())
	registerHexingSquelcher(Global())
	registerHopeEstheim(Global())
	registerHowlingMine(Global())
	registerHyldaOfTheIcyCrown(Global())
	registerIllunaApexOfWishes(Global())
	registerImotekh(Global())
	registerIronSpiderStarkUpgrade(Global())
	registerJacobFrye(Global())
	registerJanJansenChaosCrafter(Global())
	registerJawsRelentlessPredator(Global())
	registerJelevaNephaliasScourge(Global())
	registerJensonCarthalionDruidExile(Global())
	registerJhoiraOfTheGhitu(Global())
	registerJinGitaxiasProgressTyrant(Global())
	registerJinnieFay(Global())
	registerJoshuaPhoenixsDominant(Global())
	registerK9MarkI(Global())
	registerKadenaSlinkingSorcerer(Global())
	registerKamizObscuraOculus(Global())
	registerKataraWaterbendingMaster(Global())
	registerKazuulTyrantOfTheCliffs(Global())
	registerKedissEmberclawFamiliar(Global())
	registerKellanTheKid(Global())
	registerKetramoseTheNewDawn(Global())
	registerKirolAttentiveFirstYear(Global())
	registerKirriTalentedSprout(Global())
	registerKomaCosmosSerpent(Global())
	registerKraumViolentCacophony(Global())
	registerKrosDefenseContractor(Global())
	registerKumenaTyrantOfOrazca(Global())
	registerKyloxVisionaryInventor(Global())
	registerKynaiosAndTiroOfMeletis(Global())
	registerLandTax(Global())
	registerLathielTheBounteousDawn(Global())
	registerLaughingJasperFlint(Global())
	registerLaviniaAzoriusRenegade(Global())
	registerLeoriSparktouchedHunter(Global())
	registerLichLordOfUnx(Global())
	registerLichsMastery(Global())
	registerLilahUndefeatedSlickshot(Global())
	registerLonisCryptozoologist(Global())
	registerLootThePathfinder(Global())
	registerLordOfTheAccursed(Global())
	registerMagdaBrazenOutlaw(Global())
	registerMagdaTheHoardmaster(Global())
	registerMahadiEmporiumMaster(Global())
	registerMaralen(Global())
	registerMarcusMutantMayor(Global())
	registerMarwynTheNurturer(Global())
	registerMaryReadAndAnneBonny(Global())
	registerMassOfMysteries(Global())
	registerMazirekKraulDeathPriest(Global())
	registerMechanizedProduction(Global())
	registerMegatronTyrant(Global())
	registerMiirymSentinelWyrm(Global())
	registerMikeyAndLeo(Global())
	registerMillicentRestlessRevenant(Global())
	registerMinnWilyIllusionist(Global())
	registerMishraClaimedByGix(Global())
	registerMomoPlayfulPet(Global())
	registerMoseoVeinsNewDean(Global())
	registerMrFoxglove(Global())
	registerMuddleEverChanging(Global())
	registerNamorSubMariner(Global())
	registerNashiMoonSagesScion(Global())
	registerNehebTheWorthy(Global())
	registerNellyBorcaImpulsiveAccuser(Global())
	registerNevinyrral(Global())
	registerNineFingersKeene(Global())
	registerNitaForumConciliator(Global())
	registerNivMizzetTheFiremind(Global())
	registerNivMizzetVisionary(Global())
	registerOhabiCaleria(Global())
	registerOliviaOpulentOutlaw(Global())
	registerOmoQueenOfVesuva(Global())
	registerOrmacarRelicWraith(Global())
	registerOrvarAllForm(Global())
	registerOsgirTheReconstructor(Global())
	registerOtharriSunsGlory(Global())
	registerPadeem(Global())
	registerPageLooseLeaf(Global())
	registerPakoArcaneRetriever(Global())
	registerPropaganda(Global())
	registerPurphorosGodOfTheForge(Global())
	registerRadagastWizardOfWilds(Global())
	registerRadhaHeirToKeld(Global())
	registerRaffWeatherlightStalwart(Global())
	registerRakdosPatronOfChaos(Global())
	registerRalMonsoonMage(Global())
	registerRaphaelFiendishSavior(Global())
	registerRashmiAndRagavan(Global())
	registerRasputinDreamweaver(Global())
	registerRatadrabikOfUrborg(Global())
	registerReplicatingRing(Global())
	registerRevelInRiches(Global())
	registerRexCyberHound(Global())
	registerRiaIvor(Global())
	registerRielleTheEverwise(Global())
	registerRinAndSeri(Global())
	registerRithTheAwakener(Global())
	registerRoamingThrone(Global())
	registerRoccoStreetChef(Global())
	registerRooftopStorm(Global())
	registerRosaResoluteWhiteMage(Global())
	registerRoxanneStarfallSavant(Global())
	registerRunoStromkirk(Global())
	// Sai, Master Thopterist is registered via tribal_lords.go init().
	registerSarumanOfManyColors(Global())
	registerSauronLordOfTheRings(Global())
	registerSavraQueenOfTheGolgari(Global())
	registerScionOfTheUrDragon(Global())
	registerScrawlingCrawler(Global())
	registerSelvalaHeartOfTheWilds(Global())
	registerShadowOfTheSecondSun(Global())
	registerShannaPurifyingBlade(Global())
	registerShaunFatherOfSynths(Global())
	registerSheoldredTheApocalypse(Global())
	registerSheoldredTrueScriptures(Global())
	registerShepherdOfRot(Global())
	registerShireiShizosCaretaker(Global())
	registerShroofusSproutsire(Global())
	registerSidisiBroodTyrant(Global())
	registerSilentArbiter(Global())
	registerSkylineDespot(Global())
	registerSlicerHiredMuscle(Global())
	registerSlimefootTheStowaway(Global())
	registerSmellerbeeRebelFighter(Global())
	registerSokkaAndSuki(Global())
	registerSorinOfHouseMarkov(Global())
	registerSphinxOfTheSecondSun(Global())
	registerSquallGunbladeDuelist(Global())
	registerSquallSeedMercenary(Global())
	registerStarWhale(Global())
	registerStellaLeeWildCard(Global())
	registerSunSpiderNimbleWebber(Global())
	registerSvellaIceShaper(Global())
	registerSyggRiverCutthroat(Global())
	registerSyggWanderwineWisdom(Global())
	registerSyrVondamTheLucent(Global())
	registerSythisHarvestsHand(Global())
	registerTaiiWakeenPerfectShot(Global())
	registerTalionTheKindlyLord(Global())
	registerTamiyosJournal(Global())
	registerTashaTheWitchQueen(Global())
	registerTeferiTemporalArchmage(Global())
	registerTellahGreatSage(Global())
	registerTemmetNaktamunsWill(Global())
	registerTeysaOrzhovScion(Global())
	registerThaliaAndTheGitrogMonster(Global())
	registerThantisTheWarweaver(Global())
	registerThassaDeepDwelling(Global())
	registerThassaGodOfTheSea(Global())
	registerTheAncientOne(Global())
	registerTheArchimandrite(Global())
	registerTheCouncilOfFour(Global())
	registerTheDawningArchaic(Global())
	registerTheEverChangingDane(Global())
	registerTheGhoulGunslinger(Global())
	registerTheMimeoplasm(Global())
	registerTheRani(Global())
	registerTheWarDoctor(Global())
	registerTheWatcherInTheWater(Global())
	registerThrakkusTheButcher(Global())
	registerTinybonesThePickpocket(Global())
	registerTophEarthbendingMaster(Global())
	registerTormod(Global())
	registerTovolarDireOverlord(Global())
	registerTrevaTheRenewer(Global())
	registerUndeadWarchief(Global())
	registerUrabraskTheGreatWork(Global())
	registerUrtetRemnantOfMemnarch(Global())
	registerUurgSpawnOfTurg(Global())
	registerValkiGodOfLies(Global())
	registerVannifarEvolvedEnigma(Global())
	registerVaziKeenNegotiator(Global())
	registerVenserCorpsePuppet(Global())
	registerVihaanGoldwaker(Global())
	registerVincentValentine(Global())
	registerVirtueOfPersistence(Global())
	registerVraskaTheSilencer(Global())
	registerVrondissRageOfAncients(Global())
	registerWaywardServant(Global())
	registerWinterCursedRider(Global())
	registerWinterCynicalOpportunist(Global())
	registerWinterMisanthropicGuide(Global())
	registerWortBoggartAuntie(Global())
	registerWylethSoulOfSteel(Global())
	registerXenagosGodOfRevels(Global())
	registerYgraEaterOfAll(Global())
	registerYorvoLordOfGarenbrig(Global())
	registerYoshimaruEverFaithful(Global())
	registerYunaGrandSummoner(Global())
	registerZabazTheGlimmerwasp(Global())
	registerZadaHedronGrinder(Global())
	registerZeriamGoldenWind(Global())
	registerZimoneAndDina(Global())
	registerZndrspltEyeOfWisdom(Global())
	registerZoralineCosmosCaller(Global())

	// Manual fill of unhandled-commander gaps (oracle text from Scryfall bulk).
	registerGolosTirelessPilgrim(Global())
	registerIsamaruHoundOfKonda(Global())
	registerJaradGolgariLichLord(Global())
	registerSinSpirasPunishment(Global())
	registerSyrVondamSunstarExemplar(Global())
	registerTidusYunasGuardian(Global())
	registerVarolzTheScarStriped(Global())
	registerXuIfitOsteoharmonist(Global())
	registerYusriFortunesFlame(Global())
	registerZinniaValleysVoice(Global())
	registerZoyowaLavaTongue(Global())
	registerZurgoThundersDecree(Global())

	// Additional fill: well-known commanders from broader deck pool gap.
	registerAdelineResplendentCathar(Global())
	registerAdrianaCaptainOfTheGuard(Global())
	registerAdrixAndNevTwincasters(Global())
	registerAkiriLineSlinger(Global())
	registerAkromaAngelOfWrath(Global())
	registerAkromaAngelOfFury(Global())
	registerAleshaWhoSmilesAtDeath(Global())
	registerAnafenzaKinTreeSpirit(Global())
	registerAdelizTheCinderWind(Global())
	registerAlibouAncientWitness(Global())

	// Paradigm cards — Secrets of Strixhaven instant/sorcery cycle.
	registerDecorumDissertation(Global())
	registerEchocastingSymposium(Global())
	registerGerminationPracticum(Global())
	registerImprovisationCapstone(Global())
	registerRestorationSeminar(Global())

	// Auto-generated handlers (cmd/gen-handlers).
	registerGeneratedHandlers(Global())

	// Era 3 unification — SNC / BRO / DMU / NEO / CLB commanders. Each
	// handler stacks alongside the gen_*.go stub that was emitting
	// partials only.
	registerJetmirEra3(Global())
	registerFalcoSparaEra3(Global())
	registerLordXanderEra3(Global())
	registerHidetsuguAndKairiEra3(Global())
	registerShorikaiEra3(Global())
	registerAcererakEra3(Global())
	registerTazriEra3(Global())
	registerSivrissEra3(Global())
	registerUrzaPrinceEra3(Global())
	registerPlarggNassariEra3(Global())
	registerWillScionEra3(Global())
	registerFelotharEra3(Global())

	// Era 5 unification — replace gen_*.go static stubs with real
	// implementations for IKO/ZNR/KHM/CMR/C13-C18/pre-Modern commanders.
	// (Aesi already wired by Era 1's modified gen_*.go; Kardur already
	// wired by Era 2's zz_era2_register.go init().)
	registerMarchesaTheBlackRoseCustom(Global())
	registerKaradorGhostChieftainCustom(Global())
	registerDereviEmpyrialTacticianCustom(Global())
	registerYasharnImplacableEarthCustom(Global())
	registerCharixTheRagingIsleCustom(Global())
	registerKalamaxTheStormsireCustom(Global())
	registerChainerDementiaMasterCustom(Global())
	registerRuricTharTheUnbowedCustom(Global())
	registerSeleniaDarkAngelCustom(Global())
	registerYurlokOfScorchThrashCustom(Global())
	registerSakashimaOfAThousandFacesCustom(Global())
	registerAraumiOfTheDeadTideCustom(Global())
	registerMairsilThePretenderCustom(Global())

	// Era 4 unification — STX, MH2, AFR, MID, VOW, C19-C21 commanders.
	// Tiamat, Veyran, Acererak, and Kalamax are owned by other batches
	// (handler-coverage-2 / Era 3 / Era 5) so are intentionally not
	// re-registered here.
	registerGalazethPrismariCustom(Global())
	registerLierDiscipleOfTheDrownedCustom(Global())
	registerToxrillTheCorrosiveCustom(Global())
	registerAsmoranomardicadaistinaculdacarCustom(Global())
	registerJadziOracleOfArcaviosCustom(Global())
	registerSilverquillTheDisputantCustom(Global())
	registerQuandrixTheProofCustom(Global())

	// dev/etb-stub-handlers — fill ETB stubs that gen_*.go left as
	// no-ops. Mabel and Rendmaw already have hand-edited era1 handlers
	// in their gen_*.go (preserved); only Zegana's stub and Karumonix's
	// stub were genuine no-ops.
	registerPrimeSpeakerZeganaCustom(Global())
	registerKarumonixTheRatKingCustom(Global())

	// dev/muninn-top5-handlers — top Muninn parser-gap snowflakes:
	// Necromancy (#3, 237,901 hits), Bloodchief Ascension (#4, 223,731),
	// Kodama of the East Tree (#7, 121,703). Light-Paws and Tiamat (#5/#6)
	// are already covered by light_paws_emperors_voice.go and
	// custom_tiamat.go.
	registerNecromancy(Global())
	registerBloodchiefAscension(Global())
	registerKodamaOfTheEastTree(Global())

	// dev/muninn-handlers-13-20 — next tranche of Muninn parser-gap
	// snowflakes by hit count: Claim Jumper (74K), Twilight Prophet
	// (71K), Grave Venerations (67K), Birthing Ritual (66K), Lasting
	// Tarfire (61K), Valakut Exploration (60K). Chainer Nightmare Adept
	// (#14, 73K) and Frodo Adventurous Hobbit (#18, 64K) already have
	// hand-written handlers registered earlier in this file.
	registerClaimJumper(Global())
	registerTwilightProphet(Global())
	registerGraveVenerations(Global())
	registerBirthingRitual(Global())
	registerLastingTarfire(Global())
	registerValakutExploration(Global())

	// dev/muninn-handlers-8-12 — Muninn parser-gap snowflakes #8-#12:
	// Great Hall of the Biblioplex (#8, 111K), Acererak the Archlich
	// (#9, 95K — ETB venture half; attack trigger already covered by
	// registerAcererakEra3 in era3_batch.go), Knight of the White Orchid
	// (#10, 93K), Vibrance (#11, 91K), Oversold Cemetery (#12, 76K).
	registerGreatHallOfTheBiblioplex(Global())
	registerAcererakTheArchlichETB(Global())
	registerKnightOfTheWhiteOrchid(Global())
	registerVibrance(Global())
	registerOversoldCemetery(Global())

	// dev/muninn-bulk-patterns — generic family handlers, one body per
	// shape rather than one per card. Future family members drop in by
	// adding a config row, not a new file.
	//
	// land_tax_family.go covers Loyal Warhound, Sand Scout, Aerial
	// Surveyor — the "ETB/attack/upkeep: if opponent controls more
	// lands than you, fetch a basic <type>" shape. Existing hand-rolled
	// siblings (Knight of the White Orchid, Claim Jumper, Land Tax)
	// stay because they were already registered above; the family
	// handler only owns the gap cards.
	//
	// evoke_color_gate.go covers Wistfulness and Deceit — the
	// "hybrid evoke, two color-gated ETBs" shape. Vibrance keeps its
	// own handler (registered above) so this file only owns the gap
	// cards.
	registerLandTaxFamily(Global())
	registerEvokeColorGateFamily(Global())

	// dev/muninn-handlers-21-30 — snowflakes #21-#30 + 3 stragglers.
	// Skipped (covered elsewhere): Taii Wakeen (#24), Lathiel (#27),
	// Zoyowa (#29), Wistfulness (#21, in evoke_color_gate family).
	registerWeddingRing(Global())
	registerKaitoShizuki(Global())
	registerSunderflock(Global())
	registerLuxArtillery(Global())
	registerSmirkingSpelljacker(Global())
	registerCracklingSpellslinger(Global())
	registerWitchOfTheMoors(Global())
	registerLordJyscalGuado(Global())
	registerGenesisChamber(Global())

	// dev/muninn-handlers-31-40 — snowflakes #31-#40. Sand Scout's ETB
	// Desert fetch is covered by registerLandTaxFamily above; only the
	// land-to-graveyard token half lives here.
	registerRavenloftAdventurer(Global())
	registerGiselaTheBrokenBlade(Global())
	registerEvercoatUrsine(Global())
	registerPhoenixFleetAirship(Global())
	registerArchmageAscension(Global())
	registerCycloneSummoner(Global())
	registerUnstableGlyphbridge(Global())
	registerSandScout(Global())

	// dev/muninn-handlers-41-50 — snowflakes from the 41-60 range.
	registerCrownOfGondor(Global())
	registerRoccoCabarettiCaterer(Global())
	registerSepticRats(Global())
	registerAngelOfDestiny(Global())
	registerWildPair(Global())
	registerCourierBat(Global())
	registerIngeniousProdigy(Global())
	registerStarCharter(Global())
	registerStarlitSoothsayer(Global())
	registerElanorGardner(Global())

	// dev/muninn-bulk-patterns-2 — three more bulk-pattern families.
	// lifegain_endstep_family.go covers Markov Purifier, Tivash.
	// etb_tribe_gate_family.go covers Ghitu Journeymage, Dreamcaller Siren, Acclaimed Contender.
	// lifegain_counter_family.go covers Celestial Unicorn, Exemplar of Light, Archangel of Thune.
	registerLifegainEndStepFamily(Global())
	registerEtbTribeGateFamily(Global())
	registerLifegainCounterFamily(Global())

	// dev/muninn-handlers-51-70 — 10 new snowflakes pulled from the top-80
	// parser gaps. Skipped (already covered): Wistfulness (#21) +
	// Deceit (#44) in evoke_color_gate family; Loyal Warhound (#46) in
	// land_tax_family; Ghitu Journeymage / Dreamcaller Siren / Acclaimed
	// Contender in etb_tribe_gate_family.
	registerTiamat(Global())
	registerTitaniaVoiceOfGaea(Global())
	registerSmeagolHelpfulGuide(Global())
	registerYathanRoadwatcher(Global())
	registerBreachingLeviathan(Global())
	registerWaryFarmer(Global())
	registerCompySwarm(Global())
	registerSarcomancy(Global())
	registerGoblinGoliath(Global())
	registerKamiOfTransience(Global())

	// dev/muninn-handlers-51-60-finish — closes the remaining top-50 + 51-60 holdouts.
	registerTranscendentDragon(Global())
	registerRiverSongsDiary(Global())
	registerEonFrolicker(Global())
	registerTombstoneStairwell(Global())
	registerFrodoSauronsBane(Global())

	// dev/muninn-handlers-81-100 — 10 snowflakes from the top-100 parser gaps.
	registerViconiaDrowApostate(Global())
	registerMintharaMercilessSoul(Global())
	registerRuinLurkerBat(Global())
	registerDoomsdayExcruciator(Global())
	registerRankleAndTorbran(Global())
	registerScorpionSeethingStriker(Global())
	registerTolsimirMidnightsLight(Global())
	registerGeologicalAppraiser(Global())
	registerElderscaleWurm(Global())
	registerPrestonTheVanisher(Global())

	// dev/muninn-handlers-61-80 — 10 snowflakes from the top-80 parser
	// gaps that the 51-70 and 81-100 waves skipped. Skipped here (already
	// covered): Tiamat / Titania / Sméagol / Yathan / Breaching Leviathan
	// / Wary Farmer / Compy Swarm / Sarcomancy / Goblin Goliath / Kami
	// of Transience (51-70); Ghitu Journeymage / Dreamcaller Siren /
	// Acclaimed Contender (etb_tribe_gate_family); Loyal Warhound
	// (land_tax_family).
	registerNessianWildsRavager(Global())
	registerIchorid(Global())
	registerCuriousHomunculus(Global())
	registerBringerOfTheLastGift(Global())
	registerFearlessSwashbuckler(Global())
	registerLifeOfTheParty(Global())
	registerDarigaazReincarnated(Global())
	registerFeastOfTheVictoriousDead(Global())
	registerGauFeralYouth(Global())
	registerLoreholdArchivist(Global())
	// Muninn parser-gap wave #101-120 (11 handlers). Pulls high-frequency
	// uncovered cards from a fresh parser_gaps.json snapshot. Most cards
	// in the 101-120 ordinal frequency band already had per_card files;
	// this wave picks the next-tier gaps still missing handlers across
	// the top 250. Markov Purifier, Tivash, and Celestial Unicorn extend
	// the lifegain-trigger family. Skitterbeam, Gruff Triplets,
	// Quicksilver Fountain, Grave Scrabbler, Hurkyl, Oracle of Bones,
	// Bygone Bishop (investigate-on-cheap-creature), and Trumpeting
	// Carnosaur (ETB discover 5) close the rest. Note: Wistfulness +
	// Deceit are already wired by registerEvokeColorGateFamily above.
	registerSkitterbeamBattalion(Global())
	registerMarkovPurifier(Global())
	registerTivashGloomSummoner(Global())
	registerCelestialUnicorn(Global())
	registerGruffTriplets(Global())
	registerQuicksilverFountain(Global())
	registerGraveScrabbler(Global())
	registerHurkylMasterWizard(Global())
	registerOracleOfBones(Global())
	registerBygoneBishop(Global())
	registerTrumpetingCarnosaur(Global())

	// dev/muninn-bulk-patterns-3 — two more bulk-pattern families
	// covering 3 gap cards across two ETB/end-step shapes.
	// gated_etb_effect_family.go scaffolds "When ~ enters, if <self-gate>,
	// <effect>" with pluggable gates (was_cast / not_token / sneak_entry).
	// Entries: Weftwalking (was_cast), Leonardo, Leader in Blue
	// (sneak_entry). end_step_intervening_if_family.go scaffolds
	// "At the beginning of [each|your] end step, if <condition>, <effect>"
	// with pluggable gates. Entry: Lighthouse Chronologist (not_my_turn).
	// Cards investigated for both families but already covered by
	// concurrent per_card files (Gruff Triplets, Hurkyl Master Wizard,
	// Skitterbeam Battalion) are noted in each entry list and kept on
	// their bespoke handlers.
	registerGatedEtbEffectFamily(Global())
	registerEndStepInterveningIfFamily(Global())

	// dev/muninn-handlers-121-140 — 10 handlers.
	registerEmeritusOfWoe(Global())
	registerArguelsBloodFast(Global())
	registerAradeshTheFounder(Global())
	registerEccentricPestfinder(Global())
	registerSproutbackTrudge(Global())
	registerSenuKeenEyedProtector(Global())
	registerPhyrexianDreadnought(Global())
	registerNoxiousGearhulk(Global())
	registerMasterOfDeath(Global())

	// dev/muninn-handlers-141-160 — 12 handlers for tail-of-corpus snowflakes + Rune-Scarred Demon.
	registerRuneScarredDemon(Global())
	registerRoilElemental(Global())
	registerAbzanFalconer(Global())
	registerArchangelOfTithes(Global())
	registerQuilledGreatwurm(Global())
	registerBloodthirstyConqueror(Global())
	registerAnticausalVestige(Global())
	registerItThatBetrays(Global())
	registerRootwaterMatriarch(Global())
	registerWorldspineWurm(Global())
	registerArchpriestOfShadows(Global())
	registerVolatileStormdrake(Global())

	// dev/muninn-bulk-patterns-4 — two more bulk-pattern families.
	// shuffle_self_from_grave_family.go covers Dread / Purity / Guile /
	// Vigor — the Planar Chaos eternal cycle's "When ~ is put into a
	// graveyard from anywhere, shuffle it into its owner's library"
	// line. Worldspine Wurm shares the shape but keeps its bespoke
	// handler (it bundles the die→three-Wurm-token half on the same
	// trigger). etb_library_tutor_family.go covers nine ETB
	// search-for-card-to-hand tutors (Trophy / Treasure / Trinket /
	// Stoneforge Mystic / Heliod's Pilgrim / Spellseeker / Imperial
	// Recruiter / Fierce Empath / Thalia's Lancers); Rune-Scarred
	// Demon's unfiltered "any card" case keeps its bespoke handler
	// because it wants a smarter highest-CMC chooser than first-match.
	// (bulk-patterns-4 family registrations will land via separate merge)

	// dev/muninn-handlers-161-180 — gap log saturation. After wave 141-160
	// merged, the only top-170 parser_gaps entry without a bespoke or
	// family-backed handler is Sam, Loyal Attendant (#159, single-game
	// 2026-05-14 hit). Every other tail entry is covered by an existing
	// per_card file or one of the family scaffolds (etb_tribe_gate,
	// evoke_color_gate, gated_etb_effect, end_step_intervening_if,
	// lifegain_counter, land_tax, shuffle_self_from_grave,
	// etb_library_tutor). Wave intentionally narrow.
	registerSamLoyalAttendant(Global())

	// dev/muninn-handlers-181-200 — saturation continues. Today's
	// (2026-05-17) parser_gaps.json adds two genuinely uncovered bespoke
	// cards at the tail: Burnished Hart (#171, sac-for-two-basics fetcher)
	// and Trostani, Selesnya's Voice (#174, another-creature-enters
	// toughness-lifegain + populate). All other 2026-05-17 first-seen
	// entries are token strings ("Wistfulness Token", "creature token
	// knight Token", etc.) or cascade-mode variants of already-covered
	// cards. Signal is genuinely saturated against the current corpus —
	// further tail coverage now waits on fresh tournament runs to surface
	// new card types.
	registerBurnishedHart(Global())
	registerTrostaniSelesnyasVoice(Global())

	// dev/muninn-handlers-201-220 — dispatch-layer fix instead of more
	// bespoke handlers. Re-running hexdek-muninn --all --top 250 against
	// fresh data confirms wave 181-200's saturation note: every base card
	// in the top 175 already has a per_card file or family-scaffold
	// registration. What's still polluting the gap log is name-variant
	// dispatch: cascade-cast spells get renamed to "X (cascade)" by
	// gameengine/cascade.go, Urza copies become "X (Urza copy)", dragon-
	// doubler tokens become "X (Miirym Token)", etc. None of those
	// variant names matched the registered handler key, so dozens of
	// existing handlers silently bypassed cascade/copy/token hits — about
	// 5k cumulative gap-log counts across Necromancy/Vibrance/Land Tax/
	// Phoenix Fleet Airship/Tiamat cascades, with fresh last_seen=2026-05-17
	// on most.
	//
	// Fix lives in lookupCandidates (above): single name-fallback chain
	// shared by all five fire* dispatchers. Strips one trailing
	// parenthetical, then falls back to the DFC front face. Net effect is
	// that every existing handler now fires on its cascade/copy/token
	// variants without per-card alias plumbing — equivalent to wiring 10+
	// new handlers in terms of gap-log coverage, but anchored in one
	// place that future renames don't need to re-discover.
	//
	// No new card registrations in this wave. See
	// muninn_handlers_201_220_test.go for the dispatch regression test.

	// R55 batch — damage-replacement consumers (Furnace of Rath +
	// followups) registered via the gs.DamageReplacements primitive
	// added in R54.
	registerFurnaceOfRath(Global())
	registerGiselaBladeOfGoldnight(Global())
	registerDictateOfTheTwinGods(Global())
	registerQuestForPureFlame(Global())
	registerCurseOfBloodletting(Global())

	// R60 — Yawgmoth's Will family. Play-lands-and-cast-spells-from-
	// graveyard with exile-on-resolve and graveyard→exile redirect.
	// All four share the play_from_graveyard primitive in
	// gameengine/play_from_graveyard.go.
	registerYawgmothsWill(Global())
	registerGaeasWill(Global())
	registerYawgmothsAgenda(Global())
	registerMagusOfTheWill(Global())

	// R60 — Chitinous Crawler + Illusionary Mask. Both AST'd to the
	// over-applied impulse_play modkind; per_card handlers implement the
	// correct cost-and-grant semantics (Crawler) or stub the face-down
	// hand cast (Mask). Combined with the resolve_helpers.go
	// args[0]-gated guard, this closes the last 4 ZoneCastGrantExpiry
	// hits from loki r60 round 2 and the cross-validated goldilocks
	// surface. See docs/loki-r60-round2-report.md.
	registerChitinousCrawler(Global())
	registerIllusionaryMask(Global())

	// Batch AL — copy / clone family stress-tested by the Loki 100K
	// Naru Meha + Panharmonicon residual. Shares cloneVariantETB.
	registerBatchALClonesR60(Global())
}

func init() {
	registerDefaults()
	installEngineHooks()
}
