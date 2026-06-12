package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// FirePermanentETBTriggers fires the complete ETB trigger cascade for a
// permanent that has already been added to the battlefield. Handles:
//
//  1. Self-AST ETB triggers (the entering permanent's own ETB abilities)
//  2. Per-card self-ETB hook (snowflake handlers)
//  3. Ascend check (city's blessing at 10+ permanents)
//  4. Per-card hook events (permanent_etb, nonland_permanent_etb)
//  5. AST observer ETB triggers (other permanents watching for ETBs)
//
// The permanent must already be on the battlefield and replacement effects
// registered before calling this function.
//
// CR §603.3b batching — the entire cascade above represents simultaneous
// triggers from a single ETB event: the entering permanent's self-AST
// triggers fire AT THE SAME TIME as every observer's ETB trigger watching
// for "a creature enters" etc. They must be collected, ordered APNAP +
// controller-choice, then pushed together. Without the batch wrapper each
// PushTriggeredAbility inside the cascade pushed and resolved inline in
// AST-iteration order — bypassing OrderTriggersAPNAP entirely and
// silently violating the §603.3b ordering rule for every token-mint /
// mass-mint / per_card-flicker code path (resolve.go has 5 callers,
// resolve_helpers.go has 7, keywords_batch4.go has 1, plus the 4
// per_card flicker handlers: Brago / Deadeye Navigator / Displacer
// Kitten / Emiel). The stack-cast path in stack.go:resolvePermanentSpellETB
// happened to wrap its OWN cascade in a batch — but it doesn't call
// FirePermanentETBTriggers, it inlines the equivalent logic. So the
// engine's two ETB cascade paths had divergent §603.3b conformance.
//
// Opening the batch HERE means re-entrant calls from inside the cascade
// (per_card snowflake handlers firing token_created → FireCardTrigger
// → MaybePushTriggeredAbility) collapse into the same frame, and
// EndTriggerBatch on the outermost call does the single APNAP-ordered
// push + drain. BeginTriggerBatch is explicitly re-entrant per its
// docstring; the existing stack.go:resolvePermanentSpellETB wrapper
// will become a nested no-op frame for the rare paths that route
// through both.
func FirePermanentETBTriggers(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// Defensive auto-stamp: per_card token mints that haven't migrated to
	// MintTokenInstanceID yet still arrive at the ETB dispatcher without
	// an InstanceID. Catch them here so observer triggers + invariants
	// see a stamped Card regardless of which mint site brought it. The
	// helper no-ops for non-tokens and for cards already stamped.
	EnsureTokenInstanceID(gs, perm)
	// r63 §108.3 normalization: Card.Owner is the ownership authority;
	// Permanent.Owner is a denormalized mirror that ~54 raw construction
	// sites historically left at the zero default (every seat-1+ land or
	// token entering via an effect path read as owner-0). Align it once
	// at the single chokepoint every permanent passes — any LATER
	// divergence is a real mutation and trips the OwnerImmutability
	// invariant.
	if perm.Owner != perm.Card.Owner {
		perm.Owner = perm.Card.Owner
	}
	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))

	// CR §708.4: face-down permanents have no abilities — skip self-triggers.
	faceDown := perm.Flags != nil && perm.Flags["face_down"] != 0

	// CR §122.1g / §614.1d — generic Static `etb_with_counters` self-
	// replacement. Apply BEFORE the ETB trigger cascade so observers
	// (Hardened Scales, Doubling Season, Mentor of the Meek, etc.) see
	// the entering permanent in its final counter state. Covers blink /
	// reanimate / token-mint paths that enter via FirePermanentETBTriggers
	// rather than the stack-cast path in resolvePermanentSpellETB.
	if !faceDown {
		ApplyStaticETBCounters(gs, perm)
	}

	// CR §613 — register the entering permanent's continuous effects: named
	// layer handlers (Doran, Blood Moon, Humility, Opalescence, …) AND the
	// generic AST anthem / keyword-grant statics via registerASTStaticEffects.
	// The stack-cast path (resolvePermanentSpellETB) does this inline; EVERY
	// non-cast entry — reanimate, token-mint, blink, manifest/disguise flip —
	// routes through here, and pre-r61 none of them registered continuous
	// effects, so reanimated/tokened/blinked permanents silently lost all
	// static abilities (a reanimated Doran assigned damage by power, a
	// tokened anthem buffed nobody). Face-down permanents have no abilities
	// (CR §708.2), so skip until they turn face up. RegisterContinuousEffect
	// dedupes by HandlerID, so this is idempotent if a caller already wired
	// effects, and the cast path doesn't route here so it can't double-fire.
	if !faceDown {
		RegisterContinuousEffectsForPermanent(gs, perm)
	}

	if !faceDown && perm.Card.AST != nil {
		for _, ab := range perm.Card.AST.Abilities {
			trig, ok := ab.(*gameast.Triggered)
			if !ok || trig.Effect == nil {
				continue
			}
			if !EventEquals(trig.Trigger.Event, "etb") {
				continue
			}
			// Judge r63 double-fire gate: per_card owns this card's
			// self-ETB (OnETB handler, or an OnTrigger registration on
			// the etb-family events) — skip the generic AST push or the
			// printed ability resolves twice (PROGRESSION findings:
			// Gyruda, Thraben Inspector, Omnath, 19 more).
			if name := perm.Card.DisplayName(); (HasETBHook != nil && HasETBHook(name)) ||
				PerCardOwnsTrigger(name, "permanent_etb", "etb") {
				continue
			}
			// CR §614 — consult the would_fire_etb_trigger replacement
			// chain so Panharmonicon / Yarok / Cloud / Clara / Katara can
			// add additional firings. The handler returns the modified
			// count (1 by default, 2+ with doublers) and a cancel flag.
			// Pre-r60 this site pushed once unconditionally, leaving the
			// entire ETB-doubler family silently inert in real games even
			// though the replacement framework's unit tests pass.
			n, cancelled := FireETBTriggerEvent(gs, perm)
			if cancelled {
				continue
			}
			for i := 0; i < n; i++ {
				PushTriggeredAbilityWithIf(gs, perm, trig.Effect, trig.InterveningIf)
				if gs.CheckEnd() {
					return
				}
			}
		}
	}

	if !faceDown {
		InvokeETBHook(gs, perm)
	}

	if !faceDown && perm.IsSaga() {
		initSagaLoreCounters(gs, perm)
	}

	// CR §726.2: the first time a daybound/nightbound permanent enters
	// the battlefield while the game has no day/night designation, the
	// game becomes day. Cheap fast-path: the hook returns immediately
	// when state is already set or the entering permanent isn't a
	// werewolf-class card.
	OnDayboundOrNightboundETB(gs, perm)

	// CR §205.3m Outlaw type-group ETB fan-out. Fires
	// FireCardTrigger("outlaw_etb", ctx) to every battlefield permanent
	// whose oracle text watches for outlaw ETBs, when `perm` itself
	// qualifies as an Outlaw. Cheap fast-path: returns immediately if
	// the entering permanent isn't an outlaw.
	FireOutlawETBTriggers(gs, perm)

	CheckAscend(gs, perm.Controller)

	if !perm.IsLand() {
		FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
			"perm":            perm,
			"controller_seat": perm.Controller,
			"card":            perm.Card,
		})
	}
	FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            perm,
		"controller_seat": perm.Controller,
		"card":            perm.Card,
	})

	// CR §702.182 — Eerie fires on enchantment ETBs for the entering
	// permanent's controller. Cheap early return inside the hook when
	// the entering permanent isn't an enchantment.
	OnEnchantmentETB(gs, perm)

	// CR §702.131 — Constellation fires on enchantment ETBs the same
	// way Eerie does. The hook returns early for non-enchantments.
	OnConstellationETB(gs, perm)

	fireObserverETBTriggers(gs, perm)

	// Ride-along legality validator (phase 3): the ETB cascade has
	// settled — self-replacement counters are final. nil-receiver no-op.
	gs.Legality.ObserveETB(gs, perm)
}

var romanToInt = map[string]int{
	"I": 1, "II": 2, "III": 3, "IV": 4, "V": 5,
	"VI": 6, "VII": 7, "VIII": 8, "IX": 9, "X": 10,
}

// initSagaLoreCounters scans the saga's AST for chapter abilities and sets
// saga_final_chapter to the highest chapter number found. Then adds 1 lore
// counter per CR §714.3a (saga gets first lore counter on ETB).
func initSagaLoreCounters(gs *GameState, perm *Permanent) {
	if perm.Card.AST == nil {
		return
	}
	maxChapter := 0
	for _, ab := range perm.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil {
			continue
		}
		if st.Modification.ModKind != "saga_chapter" && st.Modification.ModKind != "parsed_tail" {
			continue
		}
		if st.Modification.ModKind == "saga_chapter" && len(st.Modification.Args) >= 1 {
			switch v := st.Modification.Args[0].(type) {
			case string:
				if n, ok := romanToInt[v]; ok && n > maxChapter {
					maxChapter = n
				}
			case []interface{}:
				for _, item := range v {
					if s, ok := item.(string); ok {
						if n, ok := romanToInt[s]; ok && n > maxChapter {
							maxChapter = n
						}
					}
				}
			}
		}
	}
	if maxChapter == 0 {
		maxChapter = 3
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["saga_final_chapter"] = maxChapter
	perm.AddCounter("lore", 1)
	gs.LogEvent(Event{
		Kind:   "saga_etb",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"final_chapter": maxChapter,
			"lore":          perm.Counters["lore"],
		},
	})
	// Fire lore_counter_added for Urza's Saga, Sheoldred // The True Scriptures, etc.
	FireCardTrigger(gs, "lore_counter_added", map[string]interface{}{
		"seat":    perm.Controller,
		"card":    perm.Card.DisplayName(),
		"chapter": perm.Counters["lore"],
	})
}
