package gameengine

// Phase 7 — §614 Replacement Effects framework.
//
// This file implements the CR §614 "if [event] would happen, [modified
// event] happens instead" machinery on top of the Phase 6 state-based
// action engine. It mirrors the Python reference framework built in
// scripts/playloop.py — specifically fire_event / ReplacementEffect /
// register_replacements_for_permanent.
//
// Scope (Phase 7):
//
//   - ReplEvent                        — mutable event wrapper (CR §614.5/6)
//   - ReplacementEffect                — registry entry with §616.1 category
//   - GameState.Replacements           — registry slice on GameState
//   - FireEvent(gs, ev)                — dispatcher (§616.1 ordering, §614.5
//                                         applied-once, §616.1f iterate-until-
//                                         no-applicable, APNAP tiebreak)
//   - RegisterReplacement / Unregister — add/remove helpers
//   - 12 canonical card handlers       — Laboratory Maniac, Jace Wielder,
//                                         Alhammarret's Archive, Boon
//                                         Reflection, Rhox Faithmender,
//                                         Rest in Peace, Leyline of the Void,
//                                         Anafenza, Doubling Season, Hardened
//                                         Scales, Panharmonicon, Platinum
//                                         Angel.
//
// Wire-up points (minor additive edits in other files):
//
//   - resolve.go Damage        → FireEvent("would_be_dealt_damage")
//   - resolve.go Draw          → FireEvent("would_draw") per-card
//   - resolve.go GainLife      → FireEvent("would_gain_life")
//   - resolve.go LoseLife      → FireEvent("would_lose_life")
//   - resolve.go CounterMod    → FireEvent("would_put_counter")
//   - resolve.go CreateToken   → FireEvent("would_create_token")
//   - sba.go destroyPermSBA    → FireEvent("would_die") + "would_be_put_into_graveyard"
//   - sba.go sba704_5a         → FireEvent("would_lose_game")
//   - combat.go trigger fires  → FireEvent("would_fire_etb_trigger")
//
// Rule citations:
//   §614.5  — applied-once       → event.AppliedIDs
//   §614.6  — modified event     → ApplyFn mutates event in place
//   §614.7  — replaced events that never happen → ReplCategory no-op
//   §616.1  — category ordering  → CategorySelfReplacement < Control < Copy < Back < Other
//   §616.1f — iterate-until-done → FireEvent inner loop (safety cap 64)
//   §101.4  — APNAP tiebreak     → (MVP: deterministic by timestamp within category)
//
// Thread-safety: single-goroutine per GameState (same as rest of engine).

import (
	"sort"
)

// -----------------------------------------------------------------------------
// ReplEvent — the mutable event object (CR §614.5/6)
// -----------------------------------------------------------------------------

// ReplEvent is the replacement-framework event wrapper. Mirrors Python's
// Event dataclass. Used as a *pointer* throughout so handlers mutate it
// in place (CR §614.6 modified event).
//
// Wire-in helpers (DrawOneReplaced, etc.) allocate a ReplEvent per would-X,
// call FireEvent, and then act on the post-mutation state (skip if
// Cancelled, honor modified payload counts).
type ReplEvent struct {
	// Type is the event discriminator. Stable set:
	//   would_draw / would_gain_life / would_lose_life /
	//   would_be_dealt_damage / would_put_counter /
	//   would_create_token / would_fire_etb_trigger /
	//   would_die / would_be_put_into_graveyard /
	//   would_lose_game / would_win_game
	Type string

	// Source is the permanent causing this event (e.g. Lightning Bolt's
	// controller's creature). May be nil for player-initiated actions
	// like "draw a card at upkeep".
	Source *Permanent

	// TargetSeat is the seat index that the event affects. -1 if N/A.
	TargetSeat int

	// TargetPerm is the permanent affected. nil if N/A. For "would_die"
	// this is the dying creature; for "would_be_put_into_graveyard" the
	// card's controller's permanent; for counter events the target.
	TargetPerm *Permanent

	// Payload is the arbitrary kwargs bucket. Canonical keys:
	//   "count"        — int. card-draw count, gain-life amount, counters, etc.
	//   "counter_type" — string. "+1/+1", "-1/-1", "loyalty", "charge".
	//   "source_kind"  — string. "damage" / "destroy" / "sac" / "discard" when relevant.
	//   "to_zone"      — string. "graveyard" / "exile" — mutable target zone.
	Payload map[string]any

	// Cancelled signals §614.5 cancellation — the underlying action does
	// not happen. Handlers set this via ApplyFn.
	Cancelled bool

	// AppliedIDs is the §614.5 applied-once set. Keys are
	// ReplacementEffect.HandlerID values; a handler in this set is
	// skipped on subsequent iterations of the same event chain.
	AppliedIDs map[string]bool
}

// NewReplEvent constructs a fresh event with initialized maps.
func NewReplEvent(typ string) *ReplEvent {
	return &ReplEvent{
		Type:       typ,
		TargetSeat: -1,
		Payload:    map[string]any{},
		AppliedIDs: map[string]bool{},
	}
}

// Count returns event.Payload["count"] as int, defaulting to 0.
func (e *ReplEvent) Count() int {
	if e == nil || e.Payload == nil {
		return 0
	}
	if v, ok := e.Payload["count"].(int); ok {
		return v
	}
	return 0
}

// SetCount writes event.Payload["count"].
func (e *ReplEvent) SetCount(n int) {
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	e.Payload["count"] = n
}

// String returns a string-typed payload key (empty on miss).
func (e *ReplEvent) String(key string) string {
	if e == nil || e.Payload == nil {
		return ""
	}
	if v, ok := e.Payload[key].(string); ok {
		return v
	}
	return ""
}

// -----------------------------------------------------------------------------
// ReplacementEffect — registry entry
// -----------------------------------------------------------------------------

// Replacement effect categories per CR §616.1a–e.
const (
	CategorySelfReplacement = "self_replacement" // §616.1a
	CategoryControlETB      = "control_etb"      // §616.1b
	CategoryCopyETB         = "copy_etb"         // §616.1c
	CategoryBackFaceUp      = "back_face_up"     // §616.1d
	CategoryOther           = "other"            // §616.1e
)

// replCategoryOrder returns the §616.1 ordering rank for a category.
// Lower rank = applied earlier.
func replCategoryOrder(cat string) int {
	switch cat {
	case CategorySelfReplacement:
		return 0
	case CategoryControlETB:
		return 1
	case CategoryCopyETB:
		return 2
	case CategoryBackFaceUp:
		return 3
	default: // CategoryOther and any unknown
		return 4
	}
}

// ReplacementEffect is a registry entry representing one card's
// replacement ability, scoped to a single permanent while it's on the
// battlefield. Mirrors Python ReplacementEffect dataclass.
type ReplacementEffect struct {
	// EventType is the ReplEvent.Type this effect listens on. Must match
	// exactly (no substring match).
	EventType string

	// HandlerID is unique per effect instance. Used as the §614.5
	// applied-once key. Convention: "<CardName>:<AbilityDiscriminator>:<permaddr>".
	HandlerID string

	// SourcePerm points at the permanent whose ability this is. Used by
	// UnregisterReplacementsForPermanent on LTB.
	SourcePerm *Permanent

	// ControllerSeat is the seat that controls this replacement. Drives
	// APNAP tiebreaks (§101.4).
	ControllerSeat int

	// Timestamp is the §613 layer timestamp of the source. Tiebreaker
	// within a §616.1 category.
	Timestamp int

	// Category is one of the Category* constants; drives §616.1 ordering.
	Category string

	// Applies is the predicate — return true if this effect wants to
	// replace the given event. Called fresh on every iteration so
	// post-mutation applicability works correctly.
	Applies func(*GameState, *ReplEvent) bool

	// ApplyFn mutates the event in place per CR §614.6. May set
	// event.Cancelled = true to suppress the action entirely.
	ApplyFn func(*GameState, *ReplEvent)
}

// -----------------------------------------------------------------------------
// Registry on GameState — add fields via methods to avoid editing state.go
// too heavily (state.go stays the type definition file; we add the slice
// there minimally below and use helpers here).
// -----------------------------------------------------------------------------

// RegisterReplacement appends a replacement effect to gs.Replacements.
// No sort here — FireEvent sorts the filtered subset on demand.
func (gs *GameState) RegisterReplacement(re *ReplacementEffect) {
	if gs == nil || re == nil {
		return
	}
	gs.Replacements = append(gs.Replacements, re)
}

// UnregisterReplacementsForPermanent removes every replacement whose
// SourcePerm == p. Called on LTB (§603.10). Also called from
// destroyPermSBA before the permanent leaves the battlefield so
// subsequent FireEvent calls within the same SBA pass don't see a
// dangling registry entry.
func (gs *GameState) UnregisterReplacementsForPermanent(p *Permanent) {
	if gs == nil || p == nil || len(gs.Replacements) == 0 {
		return
	}
	kept := gs.Replacements[:0]
	for _, re := range gs.Replacements {
		if re == nil {
			continue
		}
		if re.SourcePerm == p {
			continue
		}
		kept = append(kept, re)
	}
	gs.Replacements = kept
}

// -----------------------------------------------------------------------------
// FireEvent — the dispatcher
// -----------------------------------------------------------------------------

// maxReplacementIterations caps §616.1f iteration depth. A contrived
// A-replaces-B-into-B'-replaces-B-into-B pattern would loop forever
// without a cap; 64 is generous relative to any real card interaction.
const maxReplacementIterations = 64

// FireEvent runs the CR §614/§616 replacement chain for `ev`. Callers
// allocate a ReplEvent, call FireEvent, then read back Cancelled +
// Payload to act on the (possibly modified) event.
//
// Algorithm (CR §616.1):
//
//  1. Scan registry for applicable effects: EventType matches, handler
//     not in AppliedIDs, Applies(gs, ev) returns true.
//  2. Partition by §616.1 category. Lowest rank first.
//  3. Within the lowest-rank non-empty category, pick ONE effect by
//     APNAP-ish priority: active player's effects first, then by
//     timestamp. (§101.4 says players choose simultaneously; MVP
//     deterministic by timestamp is safe for the 12 canonical cards.)
//  4. Apply chosen effect, add HandlerID to AppliedIDs (§614.5).
//  5. If event.Cancelled, return.
//  6. GOTO 1, up to maxReplacementIterations (§616.1f).
//
// Returns the (same) event pointer so callers can chain `.Cancelled`
// checks inline.
func FireEvent(gs *GameState, ev *ReplEvent) *ReplEvent {
	if gs == nil || ev == nil {
		return ev
	}
	if ev.AppliedIDs == nil {
		ev.AppliedIDs = map[string]bool{}
	}

	for iter := 0; iter < maxReplacementIterations; iter++ {
		if ev.Cancelled {
			return ev
		}
		candidate := pickReplacement(gs, ev)
		if candidate == nil {
			return ev
		}
		// §614.5: record applied-once BEFORE calling ApplyFn so a handler
		// that re-fires the same event (nested) doesn't re-hit itself.
		ev.AppliedIDs[candidate.HandlerID] = true
		candidate.ApplyFn(gs, ev)
	}
	// Hit safety cap — log and return so the caller can continue.
	gs.LogEvent(Event{
		Kind:   "replacement_iteration_cap",
		Source: ev.Type,
		Details: map[string]interface{}{
			"rule": "616.1f",
			"cap":  maxReplacementIterations,
		},
	})
	return ev
}

// pickReplacement returns the next applicable replacement per §616.1,
// or nil if none apply.
func pickReplacement(gs *GameState, ev *ReplEvent) *ReplacementEffect {
	if len(gs.Replacements) == 0 {
		return nil
	}
	// Gather applicable effects.
	applicable := make([]*ReplacementEffect, 0, 4)
	for _, re := range gs.Replacements {
		if re == nil {
			continue
		}
		if re.EventType != ev.Type {
			continue
		}
		if ev.AppliedIDs[re.HandlerID] {
			continue
		}
		if re.Applies != nil && !re.Applies(gs, ev) {
			continue
		}
		applicable = append(applicable, re)
	}
	if len(applicable) == 0 {
		return nil
	}
	// Sort by (category rank asc, APNAP order, timestamp asc).
	active := gs.Active
	sort.SliceStable(applicable, func(i, j int) bool {
		a, b := applicable[i], applicable[j]
		ca, cb := replCategoryOrder(a.Category), replCategoryOrder(b.Category)
		if ca != cb {
			return ca < cb
		}
		aActive := a.ControllerSeat == active
		bActive := b.ControllerSeat == active
		if aActive != bActive {
			return aActive
		}
		return a.Timestamp < b.Timestamp
	})

	// §616.1: affected player chooses order among same-category effects.
	// Delegate to Hat if available; otherwise use the deterministic sort.
	affectedSeat := ev.TargetSeat
	if affectedSeat >= 0 && affectedSeat < len(gs.Seats) && gs.Seats[affectedSeat] != nil && gs.Seats[affectedSeat].Hat != nil {
		reordered := gs.Seats[affectedSeat].Hat.OrderReplacements(gs, affectedSeat, applicable)
		if len(reordered) > 0 {
			return reordered[0]
		}
	}

	return applicable[0]
}

// -----------------------------------------------------------------------------
// Wire-in helpers — invoked by the rest of the engine
// -----------------------------------------------------------------------------

// FireDrawEvent builds and dispatches a would_draw event. Returns the
// modified count (may be 0 if cancelled, may be >1 if a doubler fired).
// Used by resolveDraw's per-card loop.
func FireDrawEvent(gs *GameState, seat int, src *Permanent) (count int, cancelled bool) {
	ev := NewReplEvent("would_draw")
	ev.TargetSeat = seat
	ev.Source = src
	ev.SetCount(1)
	FireEvent(gs, ev)
	return ev.Count(), ev.Cancelled
}

// FireGainLifeEvent builds and dispatches a would_gain_life event. Returns
// the post-doubler amount (0 if cancelled).
func FireGainLifeEvent(gs *GameState, seat, amount int, src *Permanent) (int, bool) {
	ev := NewReplEvent("would_gain_life")
	ev.TargetSeat = seat
	ev.Source = src
	ev.SetCount(amount)
	FireEvent(gs, ev)
	return ev.Count(), ev.Cancelled
}

// FireLoseLifeEvent builds and dispatches a would_lose_life event.
func FireLoseLifeEvent(gs *GameState, seat, amount int, src *Permanent) (int, bool) {
	ev := NewReplEvent("would_lose_life")
	ev.TargetSeat = seat
	ev.Source = src
	ev.SetCount(amount)
	FireEvent(gs, ev)
	return ev.Count(), ev.Cancelled
}

// FireDamageEvent builds and dispatches a would_be_dealt_damage event.
// Returns the modified damage amount (0 if cancelled / prevented).
func FireDamageEvent(gs *GameState, src *Permanent, targetSeat int, targetPerm *Permanent, amount int) (int, bool) {
	ev := NewReplEvent("would_be_dealt_damage")
	ev.Source = src
	ev.TargetSeat = targetSeat
	ev.TargetPerm = targetPerm
	ev.SetCount(amount)
	FireEvent(gs, ev)
	return ev.Count(), ev.Cancelled
}

// FirePutCounterEvent builds and dispatches a would_put_counter event.
// Payload["counter_type"] is set from counterType.
func FirePutCounterEvent(gs *GameState, target *Permanent, counterType string, count int, src *Permanent) (int, bool) {
	ev := NewReplEvent("would_put_counter")
	ev.TargetPerm = target
	ev.Source = src
	ev.Payload["counter_type"] = counterType
	ev.SetCount(count)
	FireEvent(gs, ev)
	return ev.Count(), ev.Cancelled
}

// FireCreateTokenEvent builds and dispatches a would_create_token event.
// Returns the modified count (Doubling Season doubles to 2x).
func FireCreateTokenEvent(gs *GameState, seat, count int, src *Permanent) (int, bool) {
	ev := NewReplEvent("would_create_token")
	ev.TargetSeat = seat
	ev.Source = src
	ev.SetCount(count)
	FireEvent(gs, ev)
	return ev.Count(), ev.Cancelled
}

// FireETBTriggerEvent builds and dispatches a would_fire_etb_trigger event.
// Returns the modified count (Panharmonicon doubles triggers).
func FireETBTriggerEvent(gs *GameState, src *Permanent) (int, bool) {
	ev := NewReplEvent("would_fire_etb_trigger")
	ev.Source = src
	ev.TargetSeat = src.Controller
	ev.SetCount(1)
	FireEvent(gs, ev)
	return ev.Count(), ev.Cancelled
}

// FireDieEvent builds and dispatches a would_die event. If cancelled,
// the creature survives (Anafenza exile, Rest in Peace redirect).
// payload["to_zone"] default is "graveyard"; handlers may change it.
func FireDieEvent(gs *GameState, perm *Permanent) *ReplEvent {
	ev := NewReplEvent("would_die")
	ev.TargetPerm = perm
	if perm != nil {
		ev.TargetSeat = perm.Controller
	}
	ev.Payload["to_zone"] = "graveyard"
	FireEvent(gs, ev)
	return ev
}

// FireGraveyardEvent builds and dispatches a would_be_put_into_graveyard
// event. Used for non-creature cards / non-SBA routes (Rest in Peace
// redirecting Destroy → exile).
func FireGraveyardEvent(gs *GameState, perm *Permanent, src *Permanent) *ReplEvent {
	ev := NewReplEvent("would_be_put_into_graveyard")
	ev.TargetPerm = perm
	ev.Source = src
	if perm != nil {
		ev.TargetSeat = perm.Controller
	}
	ev.Payload["to_zone"] = "graveyard"
	FireEvent(gs, ev)
	return ev
}

// FireLoseGameEvent builds and dispatches a would_lose_game event.
// Platinum Angel cancels this.
func FireLoseGameEvent(gs *GameState, seat int) bool {
	ev := NewReplEvent("would_lose_game")
	ev.TargetSeat = seat
	FireEvent(gs, ev)
	return ev.Cancelled
}

// FireWinGameEvent builds and dispatches a would_win_game event. Opponent's
// Platinum Angel cancels.
func FireWinGameEvent(gs *GameState, seat int) bool {
	ev := NewReplEvent("would_win_game")
	ev.TargetSeat = seat
	FireEvent(gs, ev)
	return ev.Cancelled
}

// -----------------------------------------------------------------------------
// Canonical card handlers — 12 MVP cards
// -----------------------------------------------------------------------------
//
// Each card exports a Register<Card> function that callers invoke on ETB
// (either via the resolver's token / ETB path or by tests directly). The
// handlers use deterministic timestamps from gs.NextTimestamp so APNAP
// tiebreaks are reproducible.
//
// Naming convention for HandlerID: "<CardName>:<key>:<permaddr>".

// RegisterLaboratoryManiac wires the "if you would draw a card and your
// library is empty, you win the game instead" replacement (CR §614 alt-win).
func RegisterLaboratoryManiac(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	re := &ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      handlerKey("Laboratory Maniac", "altwin", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			// Applies when the draw targets this card's controller AND
			// their library is empty.
			if ev.TargetSeat != p.Controller {
				return false
			}
			return len(gs.Seats[p.Controller].Library) == 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Cancelled = true
			gs.Seats[p.Controller].Won = true
			gs.LogEvent(Event{
				Kind:   "replacement_applied",
				Seat:   p.Controller,
				Source: "Laboratory Maniac",
				Details: map[string]interface{}{
					"rule":   "614",
					"effect": "alt_win_empty_library_draw",
				},
			})
		},
	}
	gs.RegisterReplacement(re)
}

// RegisterJaceWielderOfMysteries — same alt-win as Lab Maniac (Jace is
// effectively the same effect on a planeswalker body).
func RegisterJaceWielderOfMysteries(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	re := &ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      handlerKey("Jace, Wielder of Mysteries", "altwin", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetSeat != p.Controller {
				return false
			}
			return len(gs.Seats[p.Controller].Library) == 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Cancelled = true
			gs.Seats[p.Controller].Won = true
			gs.LogEvent(Event{
				Kind:   "replacement_applied",
				Seat:   p.Controller,
				Source: "Jace, Wielder of Mysteries",
				Details: map[string]interface{}{
					"rule":   "614",
					"effect": "alt_win_empty_library_draw",
				},
			})
		},
	}
	gs.RegisterReplacement(re)
}

// RegisterAlhammarretsArchive — "If you would draw a card, draw two
// cards instead. If you would gain life, you gain twice that much life
// instead." Two distinct effects sharing a source permanent.
func RegisterAlhammarretsArchive(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	// Draw doubler.
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      handlerKey("Alhammarret's Archive", "draw_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Alhammarret's Archive",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "draw_doubler"},
			})
		},
	})
	// Life doubler.
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      handlerKey("Alhammarret's Archive", "life_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Alhammarret's Archive",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "life_doubler"},
			})
		},
	})
}

// RegisterBoonReflection — "If you would gain life, you gain twice
// that much life instead."
func RegisterBoonReflection(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      handlerKey("Boon Reflection", "life_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Boon Reflection",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "life_doubler"},
			})
		},
	})
}

// RegisterRhoxFaithmender — "If you would gain life, you gain twice
// that much life instead." Stacks with Boon Reflection (chained
// doublers → quadruple, then octuple, etc.).
func RegisterRhoxFaithmender(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      handlerKey("Rhox Faithmender", "life_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Rhox Faithmender",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "life_doubler"},
			})
		},
	})
}

// RegisterRestInPeace — "If a card or token would be put into a
// graveyard from anywhere, exile it instead." Affects ALL seats
// universally (redirect to exile).
func RegisterRestInPeace(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	apply := func(gs *GameState, ev *ReplEvent) {
		ev.Payload["to_zone"] = "exile"
		gs.LogEvent(Event{
			Kind: "replacement_applied", Seat: p.Controller,
			Source: "Rest in Peace",
			Details: map[string]interface{}{
				"rule":   "614",
				"effect": "redirect_graveyard_to_exile",
			},
		})
	}
	applies := func(gs *GameState, ev *ReplEvent) bool {
		return ev.String("to_zone") == "graveyard"
	}
	// Fires on both "would_die" and "would_be_put_into_graveyard".
	gs.RegisterReplacement(&ReplacementEffect{
		EventType: "would_die", HandlerID: handlerKey("Rest in Peace", "die", p),
		SourcePerm: p, ControllerSeat: p.Controller, Timestamp: p.Timestamp,
		Category: CategoryOther, Applies: applies, ApplyFn: apply,
	})
	gs.RegisterReplacement(&ReplacementEffect{
		EventType: "would_be_put_into_graveyard",
		HandlerID:  handlerKey("Rest in Peace", "gy", p),
		SourcePerm: p, ControllerSeat: p.Controller, Timestamp: p.Timestamp,
		Category: CategoryOther, Applies: applies, ApplyFn: apply,
	})
}

// RegisterLeylineOfTheVoid — "If a card an opponent would put into a
// graveyard from anywhere is put into exile instead." Unlike Rest in
// Peace, this only affects opponents' cards (not controller's). Also
// doesn't affect tokens (tokens cease to exist per §704.5d anyway).
func RegisterLeylineOfTheVoid(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	applies := func(gs *GameState, ev *ReplEvent) bool {
		toZone := ev.String("to_zone")
		if toZone != "graveyard" {
			return false
		}
		if ev.TargetPerm != nil {
			if ev.TargetPerm.Controller == p.Controller {
				return false
			}
			if ev.TargetPerm.IsToken() {
				return false
			}
		} else if ev.TargetSeat == p.Controller {
			return false
		}
		return true
	}
	apply := func(gs *GameState, ev *ReplEvent) {
		ev.Payload["to_zone"] = "exile"
		gs.LogEvent(Event{
			Kind: "replacement_applied", Seat: p.Controller,
			Source: "Leyline of the Void",
			Details: map[string]interface{}{"rule": "614", "effect": "opp_gy_to_exile"},
		})
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType: "would_die", HandlerID: handlerKey("Leyline of the Void", "die", p),
		SourcePerm: p, ControllerSeat: p.Controller, Timestamp: p.Timestamp,
		Category: CategoryOther, Applies: applies, ApplyFn: apply,
	})
	gs.RegisterReplacement(&ReplacementEffect{
		EventType: "would_be_put_into_graveyard",
		HandlerID:  handlerKey("Leyline of the Void", "gy", p),
		SourcePerm: p, ControllerSeat: p.Controller, Timestamp: p.Timestamp,
		Category: CategoryOther, Applies: applies, ApplyFn: apply,
	})
}

// RegisterAnafenzaTheForemost — "If a nontoken creature an opponent
// controls would die, exile it instead."
func RegisterAnafenzaTheForemost(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      handlerKey("Anafenza, the Foremost", "exile", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil {
				return false
			}
			// Only opponents' nontoken creatures.
			if ev.TargetPerm.Controller == p.Controller {
				return false
			}
			if ev.TargetPerm.IsToken() {
				return false
			}
			if !ev.TargetPerm.IsCreature() {
				return false
			}
			return true
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Payload["to_zone"] = "exile"
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Anafenza, the Foremost",
				Details: map[string]interface{}{"rule": "614", "effect": "opp_creature_to_exile"},
			})
		},
	})
}

// RegisterDoublingSeason — "If an effect would create one or more
// tokens under your control, it creates twice that many of those tokens
// instead." + "If an effect would put one or more counters on a
// permanent you control, it puts twice that many of those counters on
// that permanent instead."
func RegisterDoublingSeason(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	// Token doubler.
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_create_token",
		HandlerID:      handlerKey("Doubling Season", "token_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Doubling Season",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "token_doubler"},
			})
		},
	})
	// Counter doubler (only +1/+1 per MVP; real card covers all kinds but
	// the 12-card suite only tests +1/+1).
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      handlerKey("Doubling Season", "counter_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil || ev.TargetPerm.Controller != p.Controller {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Doubling Season",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "counter_doubler"},
			})
		},
	})
}

// RegisterHardenedScales — "If one or more +1/+1 counters would be put
// on a creature you control, that many plus one +1/+1 counters are put
// on it instead."
func RegisterHardenedScales(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      handlerKey("Hardened Scales", "plus1", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil || ev.TargetPerm.Controller != p.Controller {
				return false
			}
			if ev.String("counter_type") != "+1/+1" {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Hardened Scales",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "plus_one_counter"},
			})
		},
	})
}

// RegisterPanharmonicon — "If an artifact or creature entering the
// battlefield causes a triggered ability of a permanent you control to
// trigger, that ability triggers an additional time."
func RegisterPanharmonicon(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_fire_etb_trigger",
		HandlerID:      handlerKey("Panharmonicon", "etb_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			// Applies to triggers on this card's controller's permanents.
			if ev.Source == nil {
				return false
			}
			return ev.Source.Controller == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Panharmonicon",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "etb_trigger_extra"},
			})
		},
	})
}

// RegisterYarok — "If a permanent entering the battlefield causes a
// triggered ability of a permanent you control to trigger, that ability
// triggers an additional time." This is Panharmonicon for ALL permanents.
func RegisterYarok(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_fire_etb_trigger",
		HandlerID:      handlerKey("Yarok, the Desecrated", "etb_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.Source == nil {
				return false
			}
			return ev.Source.Controller == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Yarok, the Desecrated",
				Amount: ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "etb_trigger_extra"},
			})
		},
	})
}

// RegisterPlatinumAngel — "You can't lose the game and your opponents
// can't win the game." Cancels would_lose_game for controller and
// would_win_game for opponents.
func RegisterPlatinumAngel(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_lose_game",
		HandlerID:      handlerKey("Platinum Angel", "cant_lose", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == p.Controller
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Cancelled = true
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Platinum Angel",
				Details: map[string]interface{}{"rule": "614", "effect": "cant_lose"},
			})
		},
	})
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_win_game",
		HandlerID:      handlerKey("Platinum Angel", "cant_win", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			// Opponents of Platinum Angel's controller can't win.
			return ev.TargetSeat != p.Controller
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Cancelled = true
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source: "Platinum Angel",
				Details: map[string]interface{}{"rule": "614", "effect": "opps_cant_win"},
			})
		},
	})
}

// -----------------------------------------------------------------------------
// Dispatcher — RegisterReplacementsForPermanent keys off card name.
// -----------------------------------------------------------------------------

// RegisterReplacementsForPermanent inspects p.Card.Name and invokes the
// matching Register<Card> helper if known. Called by ETB hooks (future
// resolver integration) and by tests directly.
//
// Unknown cards are silent no-ops — future cards register via new cases
// here or by calling the specific Register* function.
func RegisterReplacementsForPermanent(gs *GameState, p *Permanent) {
	if p == nil || p.Card == nil {
		return
	}
	switch p.Card.DisplayName() {
	case "Laboratory Maniac":
		RegisterLaboratoryManiac(gs, p)
	case "Jace, Wielder of Mysteries":
		RegisterJaceWielderOfMysteries(gs, p)
	case "Alhammarret's Archive":
		RegisterAlhammarretsArchive(gs, p)
	case "Boon Reflection":
		RegisterBoonReflection(gs, p)
	case "Rhox Faithmender":
		RegisterRhoxFaithmender(gs, p)
	case "Rest in Peace":
		RegisterRestInPeace(gs, p)
	case "Leyline of the Void":
		RegisterLeylineOfTheVoid(gs, p)
	case "Anafenza, the Foremost":
		RegisterAnafenzaTheForemost(gs, p)
	case "Doubling Season":
		RegisterDoublingSeason(gs, p)
	case "Hardened Scales":
		RegisterHardenedScales(gs, p)
	case "Panharmonicon":
		RegisterPanharmonicon(gs, p)
	case "Yarok, the Desecrated":
		RegisterYarok(gs, p)
	case "Platinum Angel":
		RegisterPlatinumAngel(gs, p)
	case "Notion Thief":
		RegisterNotionThiefReplacement(gs, p)
	case "Dauthi Voidwalker":
		RegisterDauthiVoidwalker(gs, p)
	}
}

// RegisterDauthiVoidwalker — "If a card would be put into an opponent's
// graveyard from anywhere, instead exile it with a void counter on it."
// Like Leyline of the Void, only affects opponents' cards. Additionally
// marks exiled cards with a void counter so the activated ability can
// identify them.
func RegisterDauthiVoidwalker(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	applies := func(gs *GameState, ev *ReplEvent) bool {
		toZone := ev.String("to_zone")
		if toZone != "graveyard" {
			return false
		}
		// Only opponents' cards — not the controller's.
		if ev.TargetPerm != nil {
			if ev.TargetPerm.Controller == p.Controller {
				return false
			}
			if ev.TargetPerm.IsToken() {
				return false
			}
		} else if ev.TargetSeat == p.Controller {
			return false
		}
		return true
	}
	apply := func(gs *GameState, ev *ReplEvent) {
		ev.Payload["to_zone"] = "exile"
		ev.Payload["void_counter"] = true
		gs.LogEvent(Event{
			Kind: "replacement_applied", Seat: p.Controller,
			Source: "Dauthi Voidwalker",
			Details: map[string]interface{}{
				"rule":   "614",
				"effect": "opp_gy_to_exile_with_void_counter",
			},
		})
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType: "would_die", HandlerID: handlerKey("Dauthi Voidwalker", "die", p),
		SourcePerm: p, ControllerSeat: p.Controller, Timestamp: p.Timestamp,
		Category: CategoryOther, Applies: applies, ApplyFn: apply,
	})
	gs.RegisterReplacement(&ReplacementEffect{
		EventType: "would_be_put_into_graveyard",
		HandlerID:  handlerKey("Dauthi Voidwalker", "gy", p),
		SourcePerm: p, ControllerSeat: p.Controller, Timestamp: p.Timestamp,
		Category: CategoryOther, Applies: applies, ApplyFn: apply,
	})
}

// -----------------------------------------------------------------------------
// Small helpers
// -----------------------------------------------------------------------------

// handlerKey builds a unique ID from card name + discriminator + perm pointer.
// The pointer address gives us per-instance uniqueness (two Boon Reflections
// yield distinct IDs even on the same turn).
func handlerKey(cardName, disc string, p *Permanent) string {
	// Use Timestamp as a stable distinguisher without importing fmt/unsafe.
	// Collisions only happen if two perms share a timestamp AND card name,
	// which the ETB counter prevents.
	return cardName + ":" + disc + ":" + itoaRepl(p.Timestamp)
}

// RegisterNotionThiefReplacement registers Notion Thief's draw-redirect
// replacement effect. "If an opponent would draw a card except the first
// one they draw in each of their draw steps, instead that player skips
// that draw and you draw a card instead."
//
// Single replacement: Applies to all opponent draws. ApplyFn checks
// whether this is the active player's first draw this turn — if so,
// marks the flag and lets the draw proceed normally. Otherwise, cancels
// the opponent's draw and makes the controller draw instead.
func RegisterNotionThiefReplacement(gs *GameState, p *Permanent) {
	if gs == nil || p == nil {
		return
	}
	controller := p.Controller

	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      handlerKey("Notion Thief", "draw_redirect", p),
		SourcePerm:     p,
		ControllerSeat: controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetSeat == controller {
				return false
			}
			if ev.TargetSeat < 0 || ev.TargetSeat >= len(gs.Seats) {
				return false
			}
			if gs.Seats[ev.TargetSeat] == nil || gs.Seats[ev.TargetSeat].Lost {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			// Active player's first draw each turn is the draw-step draw —
			// let it through per the "except the first one" clause.
			if ev.TargetSeat == gs.Active {
				flagKey := "notion_thief_normal_draw_" + itoaRepl(ev.TargetSeat)
				if p.Flags == nil {
					p.Flags = map[string]int{}
				}
				if p.Flags[flagKey] != gs.Turn {
					p.Flags[flagKey] = gs.Turn
					return // don't modify the event — first draw goes through
				}
			}

			victim := ev.TargetSeat
			count := ev.Count()
			ev.Cancelled = true
			if controller >= 0 && controller < len(gs.Seats) {
				seat := gs.Seats[controller]
				if seat != nil && !seat.Lost {
					for i := 0; i < count; i++ {
						if _, ok := gs.drawOne(controller); !ok {
							break
						}
					}
				}
			}
			gs.LogEvent(Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Notion Thief",
				Amount: count,
				Details: map[string]interface{}{
					"rule":   "614",
					"effect": "draw_redirect",
					"victim": victim,
				},
			})
		},
	})
}

// RegisterDredge registers a dredge replacement effect on draw events.
// Per CR §702.52, dredge is a replacement effect: "If you would draw a
// card, instead you may mill N cards, then return this card from your
// graveyard to your hand." The card and ownerSeat identify the dredge
// card in the graveyard; dredgeN is the number of cards to mill.
func RegisterDredge(gs *GameState, card *Card, ownerSeat int, dredgeN int) {
	if gs == nil || card == nil {
		return
	}
	apply := func(gs *GameState, ev *ReplEvent) {
		if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
			return
		}
		seat := gs.Seats[ownerSeat]
		if seat == nil {
			return
		}
		// Mill N cards
		milled := 0
		for i := 0; i < dredgeN && len(seat.Library) > 0; i++ {
			top := seat.Library[0]
			MoveCard(gs, top, ownerSeat, "library", "graveyard", "dredge")
			milled++
		}
		// Return dredge card from graveyard to hand
		MoveCard(gs, card, ownerSeat, "graveyard", "hand", "return-from-graveyard")
		ev.Cancelled = true // replace the draw
		gs.LogEvent(Event{
			Kind:   "dredge",
			Seat:   ownerSeat,
			Source: card.DisplayName(),
			Amount: milled,
			Details: map[string]interface{}{
				"rule":   "702.52",
				"milled": milled,
			},
		})
	}
	applies := func(gs *GameState, ev *ReplEvent) bool {
		if ev.TargetSeat != ownerSeat {
			return false
		}
		// Check card is still in graveyard and library has enough cards
		if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
			return false
		}
		seat := gs.Seats[ownerSeat]
		if seat == nil {
			return false
		}
		if len(seat.Library) < dredgeN {
			return false
		}
		for _, c := range seat.Graveyard {
			if c == card {
				return true
			}
		}
		return false
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      "dredge_" + card.DisplayName(),
		ControllerSeat: ownerSeat,
		Category:       CategoryOther,
		Applies:        applies,
		ApplyFn:        apply,
	})
}

// itoaRepl is a tiny base-10 int→string without pulling in strconv.
func itoaRepl(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
