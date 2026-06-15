package gameengine

// keywords_disturb_cast.go — Disturb cast helper (CR §702.146,
// Innistrad: Midnight Hunt 2021).
//
// CR §702.146a: Disturb is a keyword that represents two abilities.
//               It appears on the front face of certain transforming
//               double-faced cards. The static and triggered
//               abilities together let the card be cast from
//               graveyard as its back face for the disturb cost and
//               replace its "would die" zone change with exile.
// CR §702.146b: "Disturb [cost]" means "You may cast this card
//               transformed from your graveyard for its disturb
//               cost." Casting a spell using its disturb ability
//               follows the rules for paying alternative costs in
//               §601.2b and §601.2f-h.
// CR §702.146c: If a permanent that resolved as a disturb-cast
//               spell would be put into a graveyard from the
//               battlefield, exile it instead. The
//               RegisterDisturbExileReplacement helper in
//               keywords_p1p2.go already does this; CastWithDisturb
//               routes through ApplyDisturbETB on resolve so the
//               replacement is wired automatically.
//
// Engine model
// ------------
// The existing zone-cast infrastructure already shipped
// NewDisturbPermission (Zone=graveyard, ExileOnResolve=false) and
// ApplyDisturbETB (transforms the perm + registers the dies→exile
// replacement) but had no cast entry point — calls to those
// helpers were orphaned. This file adds:
//
//   - HasDisturb(card) / DisturbCost(card) keyword readers.
//   - CastWithDisturb — cast-from-graveyard alt-cost helper that
//     pays the disturb cost, removes the card from the caller's
//     graveyard, and pushes a StackItem stamped CostMeta
//     {"disturb_cast": true, "disturb_cost": N,
//     "zone_cast_keyword": "disturb"} with CastZone=ZoneGraveyard.
//   - A small stack.go hook (in this PR, see resolvePermanentSpellETB)
//     that checks CostMeta["disturb_cast"] at resolve time and
//     invokes ApplyDisturbETB so the back-face transform and the
//     §702.146c dies→exile replacement land deterministically.
//
// Note on resolution destination: disturb does NOT use the
// exile_on_resolve hook (that's flashback / mayhem / omen / escape).
// A disturb-cast spell's back face is a permanent (typically a
// Spirit enchantment-creature), so it resolves to the BATTLEFIELD
// transformed. The §702.146c "exile instead of graveyard" clause
// is a separate replacement that fires later when the disturbed
// permanent would die — wired by ApplyDisturbETB via
// RegisterDisturbExileReplacement.

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasDisturb / DisturbCost
// ---------------------------------------------------------------------------

// HasDisturb reports whether the card has the disturb keyword in its
// AST.
func HasDisturb(card *Card) bool {
	return cardHasKeywordByName(card, "disturb")
}

// DisturbCost returns the converted mana cost of the disturb
// keyword's alternative cost. Accepts the keyword arg as either a
// mana string ("{1}{W}") or a plain numeric value. Returns 0 if the
// keyword is absent or args are malformed; callers should treat 0
// as "free" only when they have positively confirmed HasDisturb.
// Mirrors the FlashbackCost / MayhemCost / OmenCost / EscapeCost /
// PlotCost / ProwlCost / MadnessCostParsed reader pattern.
func DisturbCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "disturb") {
			continue
		}
		if len(kw.Args) == 0 {
			return card.CMC
		}
		switch v := kw.Args[0].(type) {
		case string:
			if cost, err := mana.Parse(v); err == nil {
				return cost.CMC()
			}
			return 0
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// CastWithDisturb
// ---------------------------------------------------------------------------

// CastWithDisturb casts a card from `seatIdx`'s graveyard for its
// disturb cost. CR §702.146.
//
// Preconditions enforced here:
//   - card has the disturb keyword (HasDisturb)
//   - card is in seat's graveyard
//   - card has a back face configured (BackFaceAST != nil) — the
//     "transformed" semantic requires a back face to flip to
//   - seat can afford `disturbCost`. Pass -1 to use the printed
//     DisturbCost.
//
// On success: spell removed from graveyard, mana paid, StackItem
// pushed with CostMeta{"disturb_cast": true, "disturb_cost": N,
// "zone_cast_keyword": "disturb"} and CastZone=ZoneGraveyard.
// Per-turn flag spell_disturbed_this_turn:<seat> set. A
// ZoneCastPermission is also registered on the card pointer so
// observers / replay see the grant lifecycle.
//
// Timing: disturb itself doesn't impose sorcery-speed. The back
// face's card type determines the timing — instant back-faces can
// be cast at instant speed; sorcery back-faces are restricted to
// sorcery timing. The generic cast-pipeline timing check in
// stack.go applies to the front-face type; for back-face timing
// the upstream caller is responsible for honoring the spec'd
// "instant speed for instant back-half, sorcery for sorcery."
// CastWithDisturb does not block by phase.
func CastWithDisturb(gs *GameState, seatIdx int, card *Card, disturbCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasDisturb(card) {
		return nil, &CastError{Reason: "no_disturb_keyword"}
	}
	// A disturb cast IS a transformed cast — without a configured
	// back face there's nothing to flip to. Reject defensively so a
	// corpus card with stray "disturb" wording but no back face
	// can't sneak through.
	// A disturb cast IS a transformed cast — without a back face
	// configured (parser populates BackFaceName for any DFC/MDFC)
	// there's nothing to flip to. Reject defensively so a corpus
	// card with stray "disturb" wording but no back face can't
	// sneak through.
	if !card.IsMDFC() {
		return nil, &CastError{Reason: "no_back_face"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// Drannith Magistrate: opponents can't cast from non-hand zones.
	if drannithRestrictsZoneCast(gs, seatIdx) {
		gs.LogEvent(Event{
			Kind:   "cast_suppressed",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "drannith_magistrate",
				"zone":   ZoneGraveyard,
				"rule":   "601.2a",
			},
		})
		return nil, &CastError{Reason: "drannith_magistrate"}
	}
	if disturbCost < 0 {
		disturbCost = DisturbCost(card)
	}
	if disturbCost < 0 {
		return nil, &CastError{Reason: "invalid_disturb_cost"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the disturb alternative
	// cost too (disturb casts the back face from the graveyard).
	disturbCost = EffectiveAlternativeCost(gs, card, seatIdx, disturbCost, CastContext{FromGraveyard: true})
	if seat.ManaPool < disturbCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return nil, &CastError{Reason: "not_in_graveyard"}
	}
	seat.ManaPool -= disturbCost
	SyncManaAfterSpend(seat)
	if disturbCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: disturbCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "disturb_cast",
				"keyword": "disturb",
				"rule":    "601.2f",
			},
		})
	}
	costMeta := map[string]interface{}{
		"disturb_cast":      true,
		"disturb_cost":      disturbCost,
		"zone_cast_keyword": "disturb",
	}
	// Forward the back-face AST hint (if present on Card.Meta) so
	// the stack.go disturb-resolve hook can populate
	// perm.FrontFaceAST / perm.BackFaceAST before ApplyDisturbETB
	// runs. Tests that build cards directly stash the back-face AST
	// on Card.Meta["disturb_back_face_ast"]; corpus-loaded cards
	// can either populate the same key or arrange for
	// InitDFCFaces to fire through their own ETB-pre hook.
	if card.Meta != nil {
		if v, ok := card.Meta["disturb_back_face_ast"]; ok && v != nil {
			costMeta["disturb_back_face_ast"] = v
		}
	}
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneGraveyard,
		Effect:     collectSpellEffect(card),
		CostMeta:   costMeta,
	}
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_disturbed_this_turn:"+itoa(seatIdx)] = 1

	// Register a ZoneCastPermission keyed on the card pointer so
	// AI/Hat policy code and replay observers see the grant.
	// Single-use: removed by RemoveZoneCastGrant when consumed (or
	// when the card moves out of graveyard, which already happened
	// above).
	RegisterZoneCastGrant(gs, card, NewDisturbPermission(disturbCost))
	RemoveZoneCastGrant(gs, card) // card has left graveyard — consume grant

	gs.LogEvent(Event{
		Kind:   "disturb_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: disturbCost,
		Details: map[string]interface{}{
			"rule": "702.146a",
		},
	})
	return &CostPaymentResult{}, nil
}

// ---------------------------------------------------------------------------
// Stack predicates
// ---------------------------------------------------------------------------

// IsDisturbCast reports whether a StackItem was cast via disturb.
func IsDisturbCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["disturb_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// SpellDisturbedThisTurn returns true if any spell was cast via
// disturb by `seatIdx` during the current turn.
func SpellDisturbedThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_disturbed_this_turn:"+itoa(seatIdx)] > 0
}
