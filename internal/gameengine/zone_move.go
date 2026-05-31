package gameengine

import "strconv"

// zone_move.go — universal zone-change entry point.
//
// Background: before this file existed, 200+ raw zone-move call sites
// across the engine bypassed the §614 replacement chain, §903.9b
// commander redirect, and trigger dispatch. Only battlefield exits went
// through the full machinery (via stack.go / resolve.go). Mill, discard,
// surveil, dredge, exile-from-non-battlefield, bounce-to-hand, and
// library-tuck were all silent: no replacements, no commander redirect,
// no triggers. This affected Sidisi, Gitrog Monster, Narcomoeba,
// Bone Miser, Dimir Spybug, Opposition Agent, and every descend card.
//
// MoveCard is the single entry point callers should use for any
// non-battlefield-to-battlefield zone transition. It wraps the existing
// FireZoneChange (commander.go) + FireZoneChangeTriggers (zone_change.go)
// so that replacements and triggers fire correctly from every call site.

// MoveResult holds the outcome of a MoveCard call. FinalZone is the
// destination after §614 replacements and §903.9b commander redirect
// (may differ from the requested toZone, or "" if the move was cancelled).
// Permanent is non-nil when the card landed on the battlefield, pointing
// to the newly created *Permanent wrapper.
type MoveResult struct {
	FinalZone string
	Permanent *Permanent // non-nil when FinalZone == "battlefield"
}

// MoveCard is the universal zone-change entry point. It:
//   1. Removes card from its source zone (by pointer identity) on the
//      named seat. No-op for battlefield/stack sources — see notes.
//   2. Calls FireZoneChange, which runs the §614 replacement chain and
//      the §903.9b commander redirect, and PLACES the card in the final
//      destination zone.
//   3. Calls FireZoneChangeTriggers to emit self-triggers (§603.10
//      look-back), observer triggers (APNAP-ordered), and per-card hooks
//      (creature_dies, permanent_ltb, etc.).
//   4. Sets Seat.DescendedThisTurn = true when a permanent card enters
//      that seat's graveyard. Used by "you've descended this turn"
//      threshold checks on Ixalan descend cards.
//
// Returns a MoveResult whose FinalZone is the final destination zone
// string (may differ from the caller's toZone when §614 or §903.9b
// redirected — e.g. commander on its way to the hand gets bounced to
// command_zone). FinalZone is "" when a replacement effect cancelled the
// move. When the card lands on the battlefield, MoveResult.Permanent
// points to the newly created *Permanent wrapper.
//
// Callers moving FROM the battlefield should not use MoveCard directly:
// battlefield exits have their own Permanent-lifecycle semantics (removal
// from []*Permanent, detach auras, drop counters, fire LTB, etc.) handled
// in the existing stack/combat/SBA code paths. MoveCard intentionally
// does nothing on fromZone=="battlefield" to avoid a second zombie exit.
//
// reason is a free-form log string (e.g. "mill", "discard", "surveil",
// "cascade-exile") — recorded in the fired events for debug traceability.
func MoveCard(gs *GameState, card *Card, ownerSeat int, fromZone, toZone, reason string) MoveResult {
	if gs == nil || card == nil {
		return MoveResult{}
	}
	removeCardFromZone(gs, ownerSeat, card, fromZone)
	dest := FireZoneChange(gs, nil, card, ownerSeat, fromZone, toZone)
	if dest == "" {
		// A §614 replacement cancelled the zone change entirely.
		return MoveResult{}
	}
	// Ramp / recursion / "put onto the battlefield" effects bypass
	// the cast path (tryPlayLand, resolvePermanentSpellETB), so an
	// MDFC with a land back face would otherwise enter under its
	// front-face (instant/sorcery) identity. Swap before triggers
	// fire so ETB observers see the land identity.
	if dest == "battlefield" {
		EnsureBattlefieldFrontFace(card)
		if ownerSeat >= 0 && ownerSeat < len(gs.Seats) && gs.Seats[ownerSeat] != nil {
			seat := gs.Seats[ownerSeat]
			if card != nil {
				if cardIsCreatureType(card) {
					seat.Turn.CreaturesEntered++
				}
				if cardHasType(card, "artifact") {
					seat.Turn.ArtifactsEntered++
				}
				if cardHasType(card, "enchantment") {
					seat.Turn.EnchantmentsEntered++
				}
			}
		}
	}
	if dest == "exile" && ownerSeat >= 0 && ownerSeat < len(gs.Seats) && gs.Seats[ownerSeat] != nil {
		gs.Seats[ownerSeat].Turn.ExiledCards++
	}
	FireZoneChangeTriggers(gs, nil, card, fromZone, dest)

	if fromZone == "graveyard" {
		FireCardTrigger(gs, "graveyard_leave", map[string]interface{}{
			"card":      card,
			"seat":      ownerSeat,
			"from_zone": fromZone,
			"to_zone":   dest,
			"reason":    reason,
		})
	}

	// card_exiled: fires when any card moves to exile (non-battlefield
	// sources). Battlefield→exile is handled in FireZoneChangeTriggers.
	// Used by The War Doctor, Syr Vondam Sunstar Exemplar.
	if dest == "exile" {
		FireCardTrigger(gs, "card_exiled", map[string]interface{}{
			"seat":      ownerSeat,
			"card":      card.DisplayName(),
			"from_zone": fromZone,
			"source":    reason,
		})
	}

	// zone_change: generic event for ANY zone transition (non-battlefield
	// sources). Battlefield exits are handled in FireZoneChangeTriggers.
	// Used by Ketramose, Necrotic Ooze, Underworld Breach, Sidisi Brood
	// Tyrant.
	FireCardTrigger(gs, "zone_change", map[string]interface{}{
		"seat":      ownerSeat,
		"card":      card.DisplayName(),
		"from_zone": fromZone,
		"to_zone":   dest,
		"source":    reason,
	})

	// CR Ixalan descend: a permanent card entering any graveyard — the
	// owner's or not — counts as a "descend" event for that permanent's
	// OWNER. We only track the owner seat's flag here; observer triggers
	// handle opponent-graveyard cases via the generic put_into_graveyard
	// event. The bool is sufficient for "have you descended this turn"
	// threshold checks; count-based thresholds should be computed from
	// graveyard-entered events directly.
	if dest == "graveyard" && ownerSeat >= 0 && ownerSeat < len(gs.Seats) {
		if seat := gs.Seats[ownerSeat]; seat != nil && cardIsPermanentType(card) {
			seat.DescendedThisTurn = true
			seat.Turn.Descended = true
			// Write gs.Flags["descended_N"] so resolve.go condition checks
			// can detect descend without reaching into seat fields.
			if gs.Flags == nil {
				gs.Flags = map[string]int{}
			}
			gs.Flags[descendedKey(ownerSeat)] = 1
			// Track total permanent cards entering any graveyard this turn
			// for Henzie "Toolbox" Torre and similar batch-count mechanics.
			gs.Flags["permanents_to_graveyard_this_turn"]++
		}
	}
	var perm *Permanent
	if dest == "battlefield" && ownerSeat >= 0 && ownerSeat < len(gs.Seats) && gs.Seats[ownerSeat] != nil {
		bf := gs.Seats[ownerSeat].Battlefield
		for i := len(bf) - 1; i >= 0; i-- {
			if bf[i] != nil && bf[i].Card == card {
				perm = bf[i]
				break
			}
		}
	}
	return MoveResult{FinalZone: dest, Permanent: perm}
}

// MoveCardToZone is the canonical name documented in the structural-analysis
// report (docs/structural-analysis-r60.md §2). Delegates to MoveCard verbatim;
// kept as a thin alias so the sweep call sites match the report's vocabulary
// and future readers grep-find both names.
func MoveCardToZone(gs *GameState, card *Card, ownerSeat int, fromZone, toZone, reason string) MoveResult {
	return MoveCard(gs, card, ownerSeat, fromZone, toZone, reason)
}

// RemoveCardFromAllPrivateZones sweeps a card pointer out of every
// non-battlefield, non-stack zone on the given seat (hand, library,
// graveyard, exile, command zone). Returns the count of zones from
// which the card was removed (typically 0 or 1; >1 indicates a prior
// duplication that this call has now collapsed).
//
// Use this defensively before placing a card on the battlefield via a
// non-stack path (reanimation, fetchland search, Sneak Attack, etc.)
// when the caller manages Permanent construction itself (per_card
// hooks via createPermanent / enterBattlefieldWithETB). MoveCard's
// `toZone="battlefield"` path now has a proper battlefield arm in
// moveToZone that wraps the Card in a Permanent, so callers routed
// through MoveCard no longer need a pre-sweep. Per-card hooks that
// construct their own Permanent should still call this to keep zone
// accounting honest. See docs/zone-accounting-analysis.md.
func RemoveCardFromAllPrivateZones(gs *GameState, seatIdx int, card *Card) int {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return 0
	}
	swept := 0
	sweep := func(slice []*Card) []*Card {
		out := slice[:0]
		for _, c := range slice {
			if c == card {
				swept++
				continue
			}
			out = append(out, c)
		}
		return out
	}
	s.Hand = sweep(s.Hand)
	s.Library = sweep(s.Library)
	s.Graveyard = sweep(s.Graveyard)
	s.Exile = sweep(s.Exile)
	s.CommandZone = sweep(s.CommandZone)
	return swept
}

// removeCardFromZone pulls card (by pointer identity) out of the named
// source zone on the given seat. No-op if card isn't found there, or if
// the fromZone is battlefield/stack (those zones have their own removal
// semantics and are handled by the callers).
func removeCardFromZone(gs *GameState, seatIdx int, card *Card, fromZone string) {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return
	}
	removeFromSlice := func(slice []*Card) []*Card {
		for i, c := range slice {
			if c == card {
				return append(slice[:i], slice[i+1:]...)
			}
		}
		return slice
	}
	switch fromZone {
	case "hand":
		s.Hand = removeFromSlice(s.Hand)
	case "library":
		s.Library = removeFromSlice(s.Library)
	case "graveyard":
		s.Graveyard = removeFromSlice(s.Graveyard)
	case "exile":
		s.Exile = removeFromSlice(s.Exile)
	case "command_zone":
		s.CommandZone = removeFromSlice(s.CommandZone)
	case "battlefield", "stack":
		// Battlefield is []*Permanent with its own lifecycle (auras,
		// counters, LTB); stack is gs.Stack managed by stack machinery.
		// Callers starting from these zones must do their own source
		// removal before invoking MoveCard.
	}
}

// cardIsPermanentType reports whether card has any permanent card type
// (artifact, creature, enchantment, planeswalker, land, battle). Gate
// for DescendedThisTurn — only permanent cards entering a graveyard
// count as a descend event per Ixalan block rules.
func cardIsPermanentType(card *Card) bool {
	if card == nil {
		return false
	}
	return cardHasType(card, "artifact") ||
		cardHasType(card, "creature") ||
		cardHasType(card, "enchantment") ||
		cardHasType(card, "planeswalker") ||
		cardHasType(card, "land") ||
		cardHasType(card, "battle")
}

func cardIsCreatureType(card *Card) bool {
	return card != nil && cardHasType(card, "creature")
}

// CardCanEnterBattlefield reports whether card has at least one
// permanent card type and is therefore a legal target for "put onto
// the battlefield" effects. Per CR 304.4 / 307.1, an instant or
// sorcery card cannot enter the battlefield — effects that try are
// no-ops and the card remains in its previous zone.
//
// Use as a gate at every battlefield-entry path that constructs a
// *Permanent from a real Card pointer (not a synthesized token).
// Public wrapper around cardIsPermanentType so the per_card subpackage
// can call it without re-implementing the type check.
func CardCanEnterBattlefield(card *Card) bool {
	return cardIsPermanentType(card)
}

// descendedKey returns the gs.Flags key for a seat's descended-this-turn
// tracking. Matches the "descended_N" pattern read in resolve.go.
func descendedKey(seat int) string {
	return "descended_" + strconv.Itoa(seat)
}
