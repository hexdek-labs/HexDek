package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_spell_mastery_riders.go — round-33 rider wiring for
// §702.106 Spell Mastery (Magic Origins 2015) and §702.131
// Constellation (Journey into Nyx 2014).
//
// IMPORTANT — Constellation timing note:
//
// The round-33 task spec asked to wire Constellation into
// resolveGatedRider. That would be timing-wrong: resolveGatedRider
// fires DURING resolveSequence (effect resolution), but CR §702.131
// constellation is a triggered ability keyed on enchantment ETB —
// AFTER the spell-resolves-as-permanent step. If constellation were
// called from resolveGatedRider, FireConstellationTriggers would scan
// a battlefield that doesn't yet contain the resolving enchantment.
//
// The correct hook point — matching the existing §702.182 Eerie
// pattern (OnEnchantmentETB) — is FirePermanentETBTriggers in
// etb_dispatch.go. OnConstellationETB sits next to OnEnchantmentETB
// there and runs at the right moment in the ETB cascade.
//
// SpellMastery, by contrast, IS a resolve-time gating rider exactly
// like Threshold/Metalcraft/Hellbent — it slots into resolveGatedRider
// cleanly.

// ===========================================================================
// §702.106 — Spell Mastery
// ===========================================================================
//
// "Spell mastery — [effect] if there are two or more instant and/or
//  sorcery cards in your graveyard."
//
// Same shape as Threshold/Metalcraft/Hellbent: passive gating, no
// triggered ETB component. CheckSpellMastery in keywords_misc.go
// already provides the active-state predicate; we mirror it as
// SpellMasteryActive for naming parity with the other gated riders.

// HasSpellMastery reports whether the card carries the §702.106 rider.
// Detection mirrors HasStrive / HasHellbent:
//
//  1. cardHasKeywordByName(card, "spell mastery") — AST tagging.
//  2. Oracle text contains "spell mastery —" / "spell mastery -" — the
//     printed reminder-text introducer for the rider.
//
// Returns false for nil / AST-less cards.
func HasSpellMastery(card *Card) bool {
	if card == nil {
		return false
	}
	if cardHasKeywordByName(card, "spell mastery") {
		return true
	}
	text := OracleTextLower(card)
	if text == "" {
		return false
	}
	return strings.Contains(text, "spell mastery —") ||
		strings.Contains(text, "spell mastery -")
}

// SpellMasteryActive reports whether seatIdx currently satisfies
// §702.106 — at least two instant or sorcery cards in their graveyard.
// Thin wrapper around CheckSpellMastery for naming consistency with
// the other gated-rider predicates (ThresholdActive / MetalcraftActive
// / HellbentActive).
func SpellMasteryActive(gs *GameState, seatIdx int) bool {
	return CheckSpellMastery(gs, seatIdx)
}

// findSpellMasteryRiderEffect walks the card's AST looking for the
// tagged "spell_mastery" rider payload. Tagging convention mirrors
// the other gated riders: Cost.Extra contains "spell_mastery_rider"
// OR Activated.Raw begins with "spell mastery" (case-insensitive
// fallback for corpus dumps).
func findSpellMasteryRiderEffect(card *Card) gameast.Effect {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		a, ok := ab.(*gameast.Activated)
		if !ok || a == nil || a.Effect == nil {
			continue
		}
		if abilityIsTaggedSpellMasteryRider(a) {
			return a.Effect
		}
	}
	return nil
}

func abilityIsTaggedSpellMasteryRider(a *gameast.Activated) bool {
	if a == nil {
		return false
	}
	for _, extra := range a.Cost.Extra {
		if strings.EqualFold(extra, "spell_mastery_rider") {
			return true
		}
	}
	raw := strings.ToLower(strings.TrimSpace(a.Raw))
	return strings.HasPrefix(raw, "spell mastery")
}

// ApplySpellMasteryRider executes the spell-mastery rider for `src`,
// if any. Returns true if the rider actually fired.
//
// Conditions to fire:
//   - src is non-nil and has a Card.
//   - HasSpellMastery(src.Card).
//   - SpellMasteryActive(gs, src.Controller) — recomputed live, so
//     graveyard state that changes mid-resolution flips the rider.
//
// Logs spell_mastery_rider on fire. If the AST carries a tagged
// rider effect, ResolveEffect runs it with `src` as the source;
// otherwise logs spell_mastery_rider_pending so per_card and
// corpus-backfill jobs can find affected spells.
func ApplySpellMasteryRider(gs *GameState, src *Permanent) bool {
	if gs == nil || src == nil || src.Card == nil {
		return false
	}
	if !HasSpellMastery(src.Card) {
		return false
	}
	if !SpellMasteryActive(gs, src.Controller) {
		return false
	}

	graveCount := countInstantsAndSorceriesInGraveyard(gs, src.Controller)

	gs.LogEvent(Event{
		Kind:   "spell_mastery_rider",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":               "702.106",
			"instant_sorcery_in_grave": graveCount,
		},
	})

	if eff := findSpellMasteryRiderEffect(src.Card); eff != nil {
		ResolveEffect(gs, src, eff)
		return true
	}

	gs.LogEvent(Event{
		Kind:   "spell_mastery_rider_pending",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "702.106",
			"reason": "rider_payload_not_in_ast",
		},
	})
	return true
}

// countInstantsAndSorceriesInGraveyard returns how many cards in the
// seat's graveyard are instants or sorceries. Used as log detail for
// the spell-mastery rider event so analytics layers can reconstruct
// the gating state at fire time.
func countInstantsAndSorceriesInGraveyard(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	n := 0
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "instant") || cardHasType(c, "sorcery") {
			n++
		}
	}
	return n
}

// ===========================================================================
// §702.131 — Constellation
// ===========================================================================
//
// "Constellation — Whenever this or another enchantment enters the
//  battlefield under your control, [effect]."
//
// Triggered ability, NOT a resolve-time gating rider. Wired here for
// detector + ETB hook completeness; the actual hook into the engine
// runs from FirePermanentETBTriggers in etb_dispatch.go (next to
// OnEnchantmentETB / Eerie), which is the timing CR §702.131 demands.

// HasConstellationRider reports whether the card has the §702.131
// constellation keyword. Detection paths:
//
//  1. AST keyword named "constellation".
//  2. Oracle text contains "constellation —" / "constellation -".
//
// Returns false for nil / AST-less cards.
func HasConstellationRider(card *Card) bool {
	if card == nil {
		return false
	}
	if cardHasKeywordByName(card, "constellation") {
		return true
	}
	text := OracleTextLower(card)
	if text == "" {
		return false
	}
	return strings.Contains(text, "constellation —") ||
		strings.Contains(text, "constellation -")
}

// OnConstellationETB is the §702.131 ETB hook. Called from
// FirePermanentETBTriggers when ANY permanent enters the battlefield;
// the hook returns early when the entering perm isn't an enchantment.
//
// When it does fire, it delegates to the existing
// FireConstellationTriggers helper in keywords_misc.go (which scans
// the controller's battlefield for constellation-bearing permanents
// and emits constellation_trigger events for each). This is the
// CR-correct timing: the entering enchantment is already on the
// battlefield by the time FirePermanentETBTriggers runs, so the
// scan includes self-triggering constellation cards.
//
// Idempotency: a single ETB event corresponds to a single
// OnConstellationETB call, so each constellation source fires once
// per real ETB. No depth counter needed (unlike the resolveSequence
// gated-rider hook where nested Sequence nodes could re-enter).
func OnConstellationETB(gs *GameState, enteredPerm *Permanent) {
	if gs == nil || enteredPerm == nil || enteredPerm.Card == nil {
		return
	}
	if !enteredPerm.IsEnchantment() {
		return
	}
	// Face-down permanents have no abilities (CR §708.2) — but their
	// ETB still triggers OTHER permanents' constellation abilities, so
	// we don't filter on face_down here. The face-down guard belongs
	// inside FireConstellationTriggers if the carrier itself is face-
	// down; today carriers are checked via HasKeyword which already
	// reads the printed AST.
	FireConstellationTriggers(gs, enteredPerm.Controller, enteredPerm)
}
