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

// UnpairOnLeave breaks a soulbond pairing when `perm` leaves the battlefield
// or changes controller (CR §702.97e — "this effect lasts as long as both
// creatures remain on the battlefield under their controller's control").
//
// The pairing is mutual: perm.Flags["paired_timestamp"] holds the partner's
// (unique, monotonic) Timestamp and vice-versa. This finds that partner on any
// battlefield and clears its flag, then clears perm's own — so the surviving
// partner becomes unpaired and eligible for a new pairing, and any
// bonus/eligibility gated on IsPaired stops. Idempotent: a no-op when `perm`
// isn't paired. Must be called while `perm` still carries its flags (i.e.
// before/at the LTB / control-change moment).
func UnpairOnLeave(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Flags == nil {
		return
	}
	partnerStamp, ok := perm.Flags["paired_timestamp"]
	if !ok || partnerStamp <= 0 {
		return
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p == perm || p.Flags == nil {
				continue
			}
			// Unique timestamp identifies the partner; the mutual-link check is
			// belt-and-suspenders against a coincidental stamp collision.
			if p.Timestamp == partnerStamp && p.Flags["paired_timestamp"] == perm.Timestamp {
				delete(p.Flags, "paired_timestamp")
				name := ""
				if p.Card != nil {
					name = p.Card.DisplayName()
				}
				gs.LogEvent(Event{
					Kind:   "soulbond_break",
					Seat:   p.Controller,
					Source: name,
					Details: map[string]interface{}{
						"rule": "702.97e",
					},
				})
			}
		}
	}
	delete(perm.Flags, "paired_timestamp")
}

// HasSoulbond reports whether a permanent has the Soulbond keyword. Mirrors
// the evolve detection (`HasKeyword("evolve") || permHasTriggerEvent(...)`):
// soulbond is a parsed keyword on a creature, so the canonical
// `HasKeyword("soulbond")` is the source of truth (covers AST keywords, granted
// abilities, and the `kw:soulbond` runtime flag tests/tokens use). Face-down
// permanents have no abilities (CR §708.2) and never have soulbond.
func HasSoulbond(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	if p.Flags != nil && p.Flags["face_down"] != 0 {
		return false
	}
	return p.HasKeyword("soulbond")
}

// FireSoulbondTriggers is the generic soulbond ETB-observer hook (CR §702.97e/f),
// mirroring FireEvolveTriggers. Called once per creature entering the
// battlefield, at the single ETB observer chokepoint
// (fireObserverETBTriggers → here). It implements BOTH halves of the soulbond
// pairing rule for `newCreature` and the creatures its controller already
// controls:
//
//	(e) If newCreature itself HAS soulbond and is unpaired, try to pair it
//	    with the first eligible unpaired creature its controller controls.
//	(f) For each unpaired soulbond creature the same controller already
//	    controls, try to pair newCreature with it.
//
// All pairing routes through the canonical PairSoulbond, which enforces the
// same-controller / both-creature / both-unpaired gates and is idempotent
// (a second attempt on an already-paired creature no-ops). This is what makes
// the hook safe to run AFTER Deadeye Navigator's per-card OnETB self-pair
// (which already calls PairSoulbond): if Deadeye paired itself on the way in,
// it now reports IsPaired, so (e) no-ops here.
//
// "You may" is modeled as auto-pair-the-first-eligible (AI-policy
// simplification, consistent with Deadeye's per-card handler; no Hat choice).
// Battlefield order is the stable iteration order — no Go map iteration — so
// the choice is deterministic.
func FireSoulbondTriggers(gs *GameState, seatIdx int, newCreature *Permanent) {
	if gs == nil || newCreature == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if !newCreature.IsCreature() {
		return
	}
	bf := gs.Seats[seatIdx].Battlefield

	// (e) The entering creature itself has soulbond: pair it with the first
	// eligible unpaired creature its controller controls. Skipped cleanly if
	// it's already paired (e.g. Deadeye's per-card OnETB ran first).
	if HasSoulbond(newCreature) && !IsPaired(newCreature) {
		for _, p := range bf {
			if p == nil || p == newCreature || !p.IsCreature() || IsPaired(p) {
				continue
			}
			// PairSoulbond re-validates every gate; let it decide.
			if PairSoulbond(gs, newCreature, p) {
				break
			}
		}
	}

	// (f) For each unpaired soulbond creature the controller already controls,
	// pair the newly-entered creature with it. (The entering creature is the
	// "another creature" that an already-out soulbond creature pairs with.)
	// Stops once newCreature is paired — a creature pairs with only one other.
	for _, p := range bf {
		if p == nil || p == newCreature || !p.IsCreature() {
			continue
		}
		if IsPaired(newCreature) {
			break
		}
		if !HasSoulbond(p) || IsPaired(p) {
			continue
		}
		if PairSoulbond(gs, p, newCreature) {
			break
		}
	}
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
