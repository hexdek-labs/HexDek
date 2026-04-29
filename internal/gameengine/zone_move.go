package gameengine

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
// Returns the final destination zone string (may differ from the caller's
// toZone when §614 or §903.9b redirected — e.g. commander on its way to
// the hand gets bounced to command_zone). Returns "" when a replacement
// effect cancelled the move.
//
// Callers moving FROM the battlefield should not use MoveCard directly:
// battlefield exits have their own Permanent-lifecycle semantics (removal
// from []*Permanent, detach auras, drop counters, fire LTB, etc.) handled
// in the existing stack/combat/SBA code paths. MoveCard intentionally
// does nothing on fromZone=="battlefield" to avoid a second zombie exit.
//
// reason is a free-form log string (e.g. "mill", "discard", "surveil",
// "cascade-exile") — recorded in the fired events for debug traceability.
func MoveCard(gs *GameState, card *Card, ownerSeat int, fromZone, toZone, reason string) string {
	if gs == nil || card == nil {
		return ""
	}
	removeCardFromZone(gs, ownerSeat, card, fromZone)
	dest := FireZoneChange(gs, nil, card, ownerSeat, fromZone, toZone)
	if dest == "" {
		// A §614 replacement cancelled the zone change entirely.
		return ""
	}
	FireZoneChangeTriggers(gs, nil, card, fromZone, dest)

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
		}
	}
	return dest
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
