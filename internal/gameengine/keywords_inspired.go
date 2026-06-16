package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// keywords_inspired.go — Inspired (CR §702.124, Born of the Gods 2014).
//
// CR §702.124a: Inspired is a triggered ability. "Inspired — Ability"
//               means "Whenever this creature becomes untapped, Ability."
// CR §702.124b: An untap is a §701.20a state transition: a permanent
//               that was tapped is no longer tapped. The trigger fires
//               on the transition itself, not on the permanent merely
//               being untapped.
//
// Engine model
// ------------
// Inspired is fired through the per-card trigger hook ("inspired"
// event) anchored on the source permanent that just transitioned
// tapped → untapped. Per-card handlers receive ctx["source"] = *perm
// and consult the source's AST / state to do whatever the printed
// trigger asks.
//
// Two firing surfaces:
//
//   1. UntapAll (UntapStep) — the canonical untap step in phases.go
//      walks the active player's battlefield, flips Tapped on
//      eligible permanents, and (post-this-PR) invokes
//      FireInspiredTriggers on each one that actually transitioned.
//   2. UntapPermanent — the centralised mid-turn untap entry point
//      for Bear Umbra-style effects ("untap target creature",
//      "untap all creatures you control after combat damage", etc.).
//      Callers that want inspired to fire correctly for mid-turn
//      untaps should call UntapPermanent rather than poking
//      perm.Tapped = false directly. Existing callsites that still
//      do the direct assignment work fine for everything except the
//      inspired trigger; migrating them is a separate sweep.
//
// Double-firing guard: FireInspiredTriggers no-ops when the
// permanent is currently tapped (the transition hasn't happened
// yet), and UntapPermanent only fires when there was an actual
// tapped → untapped transition. The same permanent can fire inspired
// multiple times per turn via repeated transitions (tap, untap,
// retap, untap → 2 inspired fires), which matches the printed rules.

// ---------------------------------------------------------------------------
// HasInspired
// ---------------------------------------------------------------------------

// HasInspired reports whether the card has the inspired keyword in its AST —
// either a bare Keyword node or (the shape real cards parse to) a
// Static{ability_word "inspired" → Triggered} wrapper. Defense-in-depth nil
// check so the helper is safe to call on stub cards.
func HasInspired(card *Card) bool {
	return cardHasKeywordByName(card, "inspired") || inspiredTriggerFromCard(card) != nil
}

// inspiredTriggerFromCard extracts the inner *Triggered from a
// Static{ability_word "inspired" → Triggered} wrapper (the shape Thor emits
// for "Inspired — …" = "Whenever this creature becomes untapped, …"), or nil.
func inspiredTriggerFromCard(card *Card) *gameast.Triggered {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "ability_word" {
			continue
		}
		args := st.Modification.Args
		if len(args) < 3 {
			continue
		}
		if w, _ := args[0].(string); w != "inspired" {
			continue
		}
		if t, ok := args[2].(*gameast.Triggered); ok {
			return t
		}
	}
	return nil
}

// HasInspiredPerm reports whether the permanent has inspired, either
// via the printed keyword on its underlying card or via a granted
// ability (perm.HasKeyword, which consults perm.GrantedAbilities).
func HasInspiredPerm(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	if perm.HasKeyword("inspired") {
		return true
	}
	return HasInspired(perm.Card)
}

// ---------------------------------------------------------------------------
// FireInspiredTriggers
// ---------------------------------------------------------------------------

// FireInspiredTriggers fires the "inspired" per-card trigger for
// `perm` if and only if `perm` has the inspired keyword AND is
// currently untapped (i.e. the tapped→untapped transition has
// already landed). The post-condition check is the double-firing
// guard: callers that invoke this before flipping Tapped, or for a
// permanent that is still tapped because §122.4 stun-counter
// replacement intercepted the untap, will silently no-op.
//
// ctx is the standard per_card_hooks payload shape: source key
// carries the *Permanent that became untapped, controller_seat
// echoes the controller for handlers that prefer a flat int over
// chasing source.Controller.
func FireInspiredTriggers(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if !HasInspiredPerm(perm) {
		return
	}
	// Transition guard — the trigger fires on becoming untapped, not
	// on being untapped. If the permanent is still tapped (e.g. a
	// stun counter ate the untap per §122.4) no transition occurred,
	// so no trigger fires.
	if perm.Tapped {
		return
	}
	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "inspired_trigger",
		Seat:   perm.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"rule": "702.124",
		},
	})
	// Per_card handlers (Pain Seer, Oreskos Sun Guide, …) own the "inspired"
	// event and implement the printed payoff themselves.
	FireCardTrigger(gs, "inspired", map[string]interface{}{
		"source":          perm,
		"controller_seat": perm.Controller,
	})
	// r63 dead-dispatch sweep (untap class): resolve the wrapped inspired
	// effect for cards WITHOUT a per_card handler. The whole inspired family
	// was inert pre-r63 — HasInspired missed the ability_word wrapper, so this
	// hook no-op'd before even reaching the per_card dispatch. With detection
	// fixed, per_card-handled cards fire via FireCardTrigger above; the rest
	// resolve their inner Triggered here. The HasTriggerHook guard prevents
	// firing BOTH for the same card (the double-fire the sweep cautioned on).
	if HasTriggerHook == nil || !HasTriggerHook(cardName, "inspired") {
		if trig := inspiredTriggerFromCard(perm.Card); trig != nil && trig.Effect != nil {
			PushTriggeredAbilityWithIf(gs, perm, trig.Effect, trig.InterveningIf)
		}
	}
}

// ---------------------------------------------------------------------------
// UntapPermanent — the canonical mid-turn untap entry point
// ---------------------------------------------------------------------------

// UntapPermanent flips `perm` from tapped to untapped, emits an
// untap_done event, and fires the inspired trigger if the permanent
// has it. Returns true iff a transition occurred (the permanent was
// tapped and is now untapped); returns false if perm was nil, was
// already untapped, or its §122.4 stun-counter replacement vetoed
// the untap.
//
// `reason` is a short tag describing what caused the untap ("bear_umbra",
// "kiora_emergency", "untap_target", etc.). Surfaces in the
// untap_done event for replay/Heimdall observability.
//
// This is the preferred entry point for any "untap target permanent"
// / "untap all <X>" / mid-turn untap effect. The existing
// perm.Tapped = false direct-assignment callsites continue to work
// — they just won't fire inspired. Migration of those callsites is
// out of scope for this commit.
func UntapPermanent(gs *GameState, perm *Permanent, reason string) bool {
	if gs == nil || perm == nil {
		return false
	}
	if !perm.Tapped {
		return false
	}
	// §122.4 stun counters — if a permanent with a stun counter would
	// untap, remove one stun counter instead. Mirror the inline check
	// in UntapAll so mid-turn untaps respect stun.
	stunCount := 0
	if perm.Counters != nil {
		stunCount = perm.Counters["stun"]
	}
	if stunCount == 0 && perm.Flags != nil {
		stunCount = perm.Flags["stun"]
	}
	if stunCount > 0 {
		if perm.Counters != nil && perm.Counters["stun"] > 0 {
			perm.Counters["stun"]--
			if perm.Counters["stun"] <= 0 {
				delete(perm.Counters, "stun")
			}
		} else if perm.Flags != nil && perm.Flags["stun"] > 0 {
			perm.Flags["stun"]--
			if perm.Flags["stun"] <= 0 {
				delete(perm.Flags, "stun")
			}
		}
		cardName := ""
		if perm.Card != nil {
			cardName = perm.Card.DisplayName()
		}
		gs.LogEvent(Event{
			Kind:   "stun_counter_removed",
			Seat:   perm.Controller,
			Source: cardName,
			Details: map[string]interface{}{
				"reason": reason,
				"rule":   "122.4",
			},
		})
		return false
	}
	perm.Tapped = false
	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "untap_done",
		Seat:   perm.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"reason": reason,
		},
	})
	FireInspiredTriggers(gs, perm)
	return true
}
