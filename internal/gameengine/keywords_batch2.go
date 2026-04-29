package gameengine

// keywords_batch2.go — Combat & Damage modification keywords (batch 2).
//
// Only keywords that are NOT already implemented elsewhere are included here.
// Already covered in other files:
//   - Infect       (keywords_p0.go + combat.go)
//   - Wither       (keywords_p1p2.go + combat.go)
//   - Bestow       (keywords_p0.go)
//   - Reconfigure  (keywords_misc.go)
//   - Banding      (keywords_combat.go)
//   - Cipher       (keywords_combat.go)
//
// This file adds:
//   1. Soulbond  — CR §702.95
//   2. Haunt     — CR §702.55

// ===========================================================================
// Soulbond — CR §702.95
// ===========================================================================
//
// "When this creature enters the battlefield, if you control both this
//  creature and another creature and both are unpaired, you may pair this
//  creature with another unpaired creature you control for as long as both
//  remain on the battlefield under your control."
//
// Paired creatures share an ability granted by the soulbond creature.
// When either creature leaves the battlefield or changes controller, the
// pair breaks.
// ---------------------------------------------------------------------------

// HasSoulbond returns true if the permanent has the soulbond keyword.
func HasSoulbond(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("soulbond")
}

// IsPaired returns true if the permanent is currently paired with another
// creature via soulbond (tracked via the "paired_timestamp" flag).
func IsPaired(p *Permanent) bool {
	if p == nil || p.Flags == nil {
		return false
	}
	return p.Flags["paired_timestamp"] > 0
}

// GetPairedPartner returns the creature that is paired with p, or nil if
// p is not paired or the partner is no longer on the battlefield.
func GetPairedPartner(gs *GameState, p *Permanent) *Permanent {
	if gs == nil || p == nil || p.Flags == nil {
		return nil
	}
	partnerStamp := p.Flags["paired_timestamp"]
	if partnerStamp <= 0 {
		return nil
	}
	seatIdx := p.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	for _, c := range gs.Seats[seatIdx].Battlefield {
		if c != nil && c != p && c.Timestamp == partnerStamp {
			return c
		}
	}
	return nil
}

// PairSoulbond pairs two creatures under the same controller. Both must be
// unpaired and both must be creatures. This is typically called when a
// creature with soulbond enters the battlefield, or when another creature
// enters while an unpaired soulbond creature is already on the battlefield.
//
// Returns true if the pairing succeeded.
func PairSoulbond(gs *GameState, perm *Permanent, partner *Permanent) bool {
	if gs == nil || perm == nil || partner == nil {
		return false
	}
	if perm.Card == nil || partner.Card == nil {
		return false
	}
	// Both must be creatures.
	if !perm.IsCreature() || !partner.IsCreature() {
		return false
	}
	// Both must be controlled by the same player.
	if perm.Controller != partner.Controller {
		return false
	}
	// Neither can already be paired.
	if IsPaired(perm) || IsPaired(partner) {
		return false
	}

	// Establish the pair using mutual timestamp references.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if partner.Flags == nil {
		partner.Flags = map[string]int{}
	}
	perm.Flags["paired_timestamp"] = partner.Timestamp
	partner.Flags["paired_timestamp"] = perm.Timestamp

	seatIdx := perm.Controller

	gs.LogEvent(Event{
		Kind:   "soulbond_pair",
		Seat:   seatIdx,
		Target: seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"paired_with": partner.Card.DisplayName(),
			"rule":        "702.95",
		},
	})
	return true
}

// BreakSoulbondPair breaks the soulbond pairing for a permanent and its
// partner. Called when either creature leaves the battlefield, changes
// controller, or stops being a creature. (CR §702.95d)
func BreakSoulbondPair(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	partner := GetPairedPartner(gs, perm)

	// Clear the pairing on this permanent.
	if perm.Flags != nil {
		delete(perm.Flags, "paired_timestamp")
	}
	// Clear the partner's side too.
	if partner != nil && partner.Flags != nil {
		delete(partner.Flags, "paired_timestamp")
	}

	seatIdx := 0
	sourceName := "unknown"
	if perm.Card != nil {
		sourceName = perm.Card.DisplayName()
	}
	if perm.Controller >= 0 {
		seatIdx = perm.Controller
	}

	gs.LogEvent(Event{
		Kind:   "soulbond_break",
		Seat:   seatIdx,
		Source: sourceName,
		Details: map[string]interface{}{
			"rule": "702.95d",
		},
	})
}

// CheckSoulbondBreaks scans all permanents and breaks any soulbond pair
// whose partner is no longer on the battlefield or has changed controller.
// Returns true if any pair was broken.
func CheckSoulbondBreaks(gs *GameState) bool {
	if gs == nil {
		return false
	}
	broken := false
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || !IsPaired(p) {
				continue
			}
			partner := GetPairedPartner(gs, p)
			if partner == nil {
				// Partner gone — break.
				if p.Flags != nil {
					delete(p.Flags, "paired_timestamp")
				}
				gs.LogEvent(Event{
					Kind:   "soulbond_break",
					Seat:   p.Controller,
					Source: p.Card.DisplayName(),
					Details: map[string]interface{}{
						"reason": "partner_left_battlefield",
						"rule":   "702.95d",
					},
				})
				broken = true
			}
		}
	}
	return broken
}

// ===========================================================================
// Haunt — CR §702.55
// ===========================================================================
//
// When a creature with haunt dies (or a spell with haunt resolves), exile
// it haunting target creature. When the haunted creature dies, the haunt
// ability triggers again.
//
// For creatures: "When this creature dies, exile it haunting target creature."
// For instants/sorceries: "When the spell resolves, exile it haunting target
// creature."
// ---------------------------------------------------------------------------

// HasHaunt returns true if the permanent has the haunt keyword.
func HasHaunt(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("haunt")
}

// HasHauntCard returns true if the card has the haunt keyword.
// Used for instants/sorceries with haunt that are not permanents.
func HasHauntCard(card *Card) bool {
	return cardHasKeywordByName(card, "haunt")
}

// IsHaunted returns true if the permanent currently has a card exiled
// haunting it (tracked via the "haunted_by" flag).
func IsHaunted(p *Permanent) bool {
	if p == nil || p.Flags == nil {
		return false
	}
	return p.Flags["haunted_by"] > 0
}

// ApplyHaunt exiles the dying permanent (or resolved spell card) and
// attaches it as a haunt on the target creature. When the haunted creature
// dies, the haunting card's ability triggers.
//
// For creatures with haunt: called when the creature dies.
// For spells with haunt: called when the spell resolves.
//
// The dying card is moved to exile and the target is marked as haunted.
func ApplyHaunt(gs *GameState, seatIdx int, hauntCard *Card, target *Permanent) {
	if gs == nil || hauntCard == nil || target == nil {
		return
	}
	if target.Card == nil {
		return
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	// Target must be a creature.
	if !target.IsCreature() {
		return
	}

	seat := gs.Seats[seatIdx]

	// Move the haunting card from graveyard to exile. It may already be
	// in exile if this is a spell (the spell resolves and then gets exiled
	// haunting). Try graveyard first, then hand, then skip zone removal
	// if the card isn't found (it may have been moved by another effect).
	removed := removeFromZone(seat, hauntCard, ZoneGraveyard)
	fromZone := "graveyard"
	if !removed {
		if removeFromZone(seat, hauntCard, ZoneHand) {
			fromZone = "hand"
		}
	}
	// Even if not removed from a zone (edge case), still exile it.
	MoveCard(gs, hauntCard, seatIdx, fromZone, "exile", "effect")

	// Mark the target as haunted.
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["haunted_by"] = 1

	// Store the haunting card name for trigger resolution.
	if target.Counters == nil {
		target.Counters = map[string]int{}
	}
	// Use a counter-like marker to track the haunt source. The engine
	// can reference this when the haunted creature dies.
	target.Counters["haunt_source_seat"] = seatIdx

	gs.LogEvent(Event{
		Kind:   "haunt_exile",
		Seat:   seatIdx,
		Target: target.Controller,
		Source: hauntCard.DisplayName(),
		Details: map[string]interface{}{
			"haunted_creature": target.Card.DisplayName(),
			"rule":             "702.55",
		},
	})
}

// FireHauntTrigger is called when a haunted creature dies. It triggers the
// haunting card's ability (the "when the haunted creature dies" half of
// haunt). The haunted_by flag is cleared after firing.
func FireHauntTrigger(gs *GameState, hauntedPerm *Permanent) {
	if gs == nil || hauntedPerm == nil {
		return
	}
	if !IsHaunted(hauntedPerm) {
		return
	}

	sourceSeat := 0
	if hauntedPerm.Counters != nil {
		sourceSeat = hauntedPerm.Counters["haunt_source_seat"]
	}

	gs.LogEvent(Event{
		Kind:   "haunt_trigger",
		Seat:   sourceSeat,
		Target: hauntedPerm.Controller,
		Source: hauntedPerm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.55",
		},
	})

	// Clear the haunt markers.
	delete(hauntedPerm.Flags, "haunted_by")
	if hauntedPerm.Counters != nil {
		delete(hauntedPerm.Counters, "haunt_source_seat")
	}
}
