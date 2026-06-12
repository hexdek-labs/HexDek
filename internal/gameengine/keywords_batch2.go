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

// IsPaired returns true if the permanent is currently paired with another
// creature via soulbond (tracked via the "paired_timestamp" flag).
func IsPaired(p *Permanent) bool {
	if p == nil || p.Flags == nil {
		return false
	}
	return p.Flags["paired_timestamp"] > 0
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
