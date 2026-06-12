package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_gated_state_batch.go — Round-33 batch of gated-state riders.
//
// Adds five Threshold/Metalcraft/Hellbent-shaped gating mechanics in a
// single file: Delirium (§702.151), Coven (§702.152), Ferocious
// (§702.135), Raid (§702.128), and Revolt (§702.146). Each follows the
// exact pattern established by Threshold:
//
//   - HasXxx(card) bool           detector. Matches an AST Keyword
//                                 node named "xxx" OR an oracle-text
//                                 substring "xxx" (case-insensitive,
//                                 via OracleTextLower's cached form).
//   - XxxActive(gs, seat) bool    predicate. Recomputed live every
//                                 call (no caching) so mid-resolution
//                                 state changes flip the gate
//                                 correctly.
//   - findXxxRiderEffect /         tagged-AST payload lookup.
//     abilityIsTaggedXxxRider     Two-shape tolerance:
//                                   1. Cost.Extra contains
//                                      "xxx_rider"
//                                   2. Activated.Raw begins with
//                                      "xxx" (case-insensitive).
//   - ApplyXxxRider(gs, src)      resolver-side executor. Logs
//                                 xxx_rider on fire; runs the
//                                 tagged payload via ResolveEffect
//                                 or logs xxx_rider_pending when
//                                 no payload tagged.
//
// All five are wired into resolveGatedRider (keywords_gated_riders.go)
// alongside Threshold/Metalcraft/Hellbent, in the order
// Delirium → Coven → Ferocious → Raid → Revolt (lowest §-section
// number first within this batch; the round-31 trio fires first by
// historical order).
//
// Per §-rule cites:
//
//   Delirium (§702.151) — 4+ distinct card types in YOUR graveyard.
//     Card types: creature, instant, sorcery, artifact, enchantment,
//     land, planeswalker, tribal, battle, kindred. Wraps the existing
//     CheckDelirium (keywords_misc.go) so we don't drift from the
//     canonical type set.
//
//   Coven (§702.152) — YOU control three or more creatures with
//     different powers. Same-power duplicates do NOT count as
//     additional distinct powers (a board of three 2/2s is NOT
//     coven-active). Uses the live characteristics layer
//     (Permanent.Power) so anthem buffs / counters that change power
//     are respected.
//
//   Ferocious (§702.135) — YOU control a creature with power 4 or
//     greater.
//
//   Raid (§702.128) — YOU attacked with a creature this turn. Reads
//     Seat.Turn.Attacked (set by combat.go's DeclareAttackers).
//
//   Revolt (§702.146) — A permanent YOU controlled left the
//     battlefield this turn. Reads Seat.Turn.PermanentsLeft, which
//     ticks on dies+exile+bounce+sacrifice.
//
// All five gates are per-seat: your delirium does NOT enable an
// opponent's delirium-gated cards, etc.

// ===========================================================================
// Delirium (§702.151) — 4+ distinct card types in YOUR graveyard
// ===========================================================================

// HasDelirium reports whether the card prints a delirium ability.
// AST Keyword "delirium" OR oracle-text substring "delirium".
func HasDelirium(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "delirium") {
			return true
		}
	}
	return strings.Contains(OracleTextLower(card), "delirium")
}

// DeliriumActive returns true when `seatIdx`'s graveyard contains
// cards covering 4 or more distinct card types. Thin wrapper around
// CheckDelirium (keywords_misc.go) so the per-seat predicate stays in
// sync with the canonical type set there (no drift if the type list
// expands — e.g., a future "spacecraft" card type).
//
// Returns false for nil gs or invalid seat.
func DeliriumActive(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	return CheckDelirium(gs, seatIdx)
}

func findDeliriumRiderEffect(card *Card) gameast.Effect {
	return findTaggedRiderEffect(card, "delirium")
}

// ApplyDeliriumRider executes the delirium rider for `src`, if any.
// CR §702.151a. Returns true if the rider fired.
func ApplyDeliriumRider(gs *GameState, src *Permanent) bool {
	if gs == nil || src == nil || src.Card == nil {
		return false
	}
	if !HasDelirium(src.Card) {
		return false
	}
	if !DeliriumActive(gs, src.Controller) {
		return false
	}

	gs.LogEvent(Event{
		Kind:   "delirium_rider",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.151",
		},
	})

	if eff := findDeliriumRiderEffect(src.Card); eff != nil {
		ResolveEffect(gs, src, eff)
		return true
	}
	gs.LogEvent(Event{
		Kind:   "delirium_rider_pending",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "702.151",
			"reason": "rider_payload_not_in_ast",
		},
	})
	return true
}

// ===========================================================================
// Coven (§702.152) — control 3+ creatures with different powers
// ===========================================================================

// HasCoven reports whether the card prints a coven ability.
func HasCoven(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "coven") {
			return true
		}
	}
	return strings.Contains(OracleTextLower(card), "coven")
}

// CovenActive returns true when `seatIdx` controls three or more
// creatures with three distinct power values (e.g., a 1/1, 2/2, 3/3
// trio). Two 2/2s and a 4/4 is NOT coven-active (only two distinct
// powers). Uses the live Permanent.Power() so layer-driven power
// changes (anthems, counters, abilities like Vigor, etc.) are
// respected. CR §702.152a.
//
// Phased-out creatures are excluded per §702.26 "treated as though
// they don't exist." Tokens count when they're creatures (which they
// are by the IsCreature() type check).
func CovenActive(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	distinctPowers := map[int]struct{}{}
	creatures := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.PhasedOut {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		creatures++
		distinctPowers[p.Power()] = struct{}{}
	}
	return creatures >= 3 && len(distinctPowers) >= 3
}

func findCovenRiderEffect(card *Card) gameast.Effect {
	return findTaggedRiderEffect(card, "coven")
}

// ApplyCovenRider executes the coven rider for `src`, if any.
// CR §702.152a. Returns true if the rider fired.
func ApplyCovenRider(gs *GameState, src *Permanent) bool {
	if gs == nil || src == nil || src.Card == nil {
		return false
	}
	if !HasCoven(src.Card) {
		return false
	}
	if !CovenActive(gs, src.Controller) {
		return false
	}

	gs.LogEvent(Event{
		Kind:   "coven_rider",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.152",
		},
	})

	if eff := findCovenRiderEffect(src.Card); eff != nil {
		ResolveEffect(gs, src, eff)
		return true
	}
	gs.LogEvent(Event{
		Kind:   "coven_rider_pending",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "702.152",
			"reason": "rider_payload_not_in_ast",
		},
	})
	return true
}

// ===========================================================================
// Ferocious (§702.135) — control a creature with power 4+
// ===========================================================================

// HasFerocious reports whether the card prints a ferocious ability.
func HasFerocious(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "ferocious") {
			return true
		}
	}
	return strings.Contains(OracleTextLower(card), "ferocious")
}

// FerociousActive returns true when `seatIdx` controls at least one
// creature with power 4 or greater. CR §702.135a. Live characteristics
// via Permanent.Power so anthem buffs, +1/+1 counters, modifications
// all count.
//
// Phased-out creatures are excluded.
func FerociousActive(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.PhasedOut {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if p.Power() >= 4 {
			return true
		}
	}
	return false
}

func findFerociousRiderEffect(card *Card) gameast.Effect {
	return findTaggedRiderEffect(card, "ferocious")
}

// ApplyFerociousRider executes the ferocious rider for `src`, if any.
// CR §702.135a. Returns true if the rider fired.
func ApplyFerociousRider(gs *GameState, src *Permanent) bool {
	if gs == nil || src == nil || src.Card == nil {
		return false
	}
	if !HasFerocious(src.Card) {
		return false
	}
	if !FerociousActive(gs, src.Controller) {
		return false
	}

	gs.LogEvent(Event{
		Kind:   "ferocious_rider",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.135",
		},
	})

	if eff := findFerociousRiderEffect(src.Card); eff != nil {
		ResolveEffect(gs, src, eff)
		return true
	}
	gs.LogEvent(Event{
		Kind:   "ferocious_rider_pending",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "702.135",
			"reason": "rider_payload_not_in_ast",
		},
	})
	return true
}

// ===========================================================================
// Raid (§702.128) — attacked with a creature this turn
// ===========================================================================

// HasRaid reports whether the card prints a raid ability.
func HasRaid(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "raid") {
			return true
		}
	}
	return strings.Contains(OracleTextLower(card), "raid")
}

// RaidActive returns true when `seatIdx` declared at least one
// attacker this turn. CR §702.128a. Reads Seat.Turn.Attacked, which
// combat.go's DeclareAttackers sets when any attacker is declared
// (regardless of whether the attack landed).
func RaidActive(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	return seat.Turn.Attacked
}

func findRaidRiderEffect(card *Card) gameast.Effect {
	return findTaggedRiderEffect(card, "raid")
}

// ApplyRaidRider executes the raid rider for `src`, if any.
// CR §702.128a. Returns true if the rider fired.
func ApplyRaidRider(gs *GameState, src *Permanent) bool {
	if gs == nil || src == nil || src.Card == nil {
		return false
	}
	if !HasRaid(src.Card) {
		return false
	}
	if !RaidActive(gs, src.Controller) {
		return false
	}

	gs.LogEvent(Event{
		Kind:   "raid_rider",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.128",
		},
	})

	if eff := findRaidRiderEffect(src.Card); eff != nil {
		ResolveEffect(gs, src, eff)
		return true
	}
	gs.LogEvent(Event{
		Kind:   "raid_rider_pending",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "702.128",
			"reason": "rider_payload_not_in_ast",
		},
	})
	return true
}

// ===========================================================================
// Revolt (§702.146) — a permanent you controlled left the battlefield
// this turn
// ===========================================================================

// HasRevolt reports whether the card prints a revolt ability.
func HasRevolt(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "revolt") {
			return true
		}
	}
	return strings.Contains(OracleTextLower(card), "revolt")
}

// RevoltActive returns true when at least one permanent `seatIdx`
// controlled left the battlefield this turn. CR §702.146a. Reads
// Seat.Turn.PermanentsLeft, which the existing engine bumps on dies
// + exile + bounce + sacrifice (so any LTB causes revolt to enable).
func RevoltActive(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	return seat.Turn.PermanentsLeft > 0
}

func findRevoltRiderEffect(card *Card) gameast.Effect {
	return findTaggedRiderEffect(card, "revolt")
}

// ApplyRevoltRider executes the revolt rider for `src`, if any.
// CR §702.146a. Returns true if the rider fired.
func ApplyRevoltRider(gs *GameState, src *Permanent) bool {
	if gs == nil || src == nil || src.Card == nil {
		return false
	}
	if !HasRevolt(src.Card) {
		return false
	}
	if !RevoltActive(gs, src.Controller) {
		return false
	}

	gs.LogEvent(Event{
		Kind:   "revolt_rider",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.146",
		},
	})

	if eff := findRevoltRiderEffect(src.Card); eff != nil {
		ResolveEffect(gs, src, eff)
		return true
	}
	gs.LogEvent(Event{
		Kind:   "revolt_rider_pending",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "702.146",
			"reason": "rider_payload_not_in_ast",
		},
	})
	return true
}

// ===========================================================================
// Shared rider-payload tagging helpers
// ===========================================================================

// findTaggedRiderEffect walks `card.AST.Abilities` looking for the
// first Activated ability whose tag matches `prefix`. Tag matching:
//
//   - Cost.Extra contains "<prefix>_rider"
//   - Activated.Raw begins with `prefix` (case-insensitive)
//
// Returns the matched Activated.Effect, or nil if no tagged ability is
// present. Shared by all five Round-33 riders so the tagging
// convention is defined exactly once.
func findTaggedRiderEffect(card *Card, prefix string) gameast.Effect {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		a, ok := ab.(*gameast.Activated)
		if !ok || a == nil || a.Effect == nil {
			continue
		}
		if abilityIsTaggedRider(a, prefix) {
			return a.Effect
		}
	}
	return nil
}

// abilityIsTaggedRider returns true when an Activated ability carries
// a tag matching `prefix` per the convention above.
func abilityIsTaggedRider(a *gameast.Activated, prefix string) bool {
	if a == nil {
		return false
	}
	tag := prefix + "_rider"
	for _, extra := range a.Cost.Extra {
		if strings.EqualFold(extra, tag) {
			return true
		}
	}
	raw := strings.ToLower(strings.TrimSpace(a.Raw))
	return strings.HasPrefix(raw, strings.ToLower(prefix))
}
