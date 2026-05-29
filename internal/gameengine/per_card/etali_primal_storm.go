package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEtaliPrimalStorm wires Etali, Primal Storm.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Etali%2C%20Primal%20Storm):
//
//	Whenever Etali, Primal Storm attacks, exile the top card of each
//	player's library, then you may cast any number of nonland cards
//	from among them without paying their mana costs.
//
// {4}{R}{R} 6/6 Legendary Creature — Elder Dinosaur. Big-mana attack
// trigger commander — every swing exiles ~4 random cards across the
// table and lets Etali's controller free-cast any nonlands among them.
// High-variance, archetype-defining for impulse-cascade / chaos shells.
//
// Implementation (CR §400.7c-compliant after PR #683):
//
//   - OnTrigger("creature_attacks"). Filter: attacker_perm == self.
//
//   - For each LIVING seat (including Etali's controller), pop the
//     top card of their library (last index — engine's library
//     convention: top is len-1, verified against Hermit Druid in
//     per_card_test.go:587-600) and move it to THAT SEAT'S
//     (the OWNER's) exile zone via MoveCard with from="library" →
//     to="exile". Per CR §400.7c, "If an effect causes a player to
//     put a card into a zone, that card moves to the corresponding
//     zone owned by that player" — so the exile destination is the
//     OWNER's exile, not Etali's controller's exile.
//
//   - For each exiled card that is NOT a land, register a
//     ZoneCastGrant via NewFreeCastFromExilePermission with
//     RequireController = Etali's controller seat (only Etali's
//     controller may free-cast), Duration "until_end_of_turn",
//     GrantTurn = gs.Turn so the grant expires at EOT per CR §608.2g
//     and the r60 ZoneCastGrantExpiry plumbing. The grant is keyed
//     on the *Card pointer (gs.ZoneCastGrants[*Card]), so the cast-
//     from-exile lookup doesn't care WHICH seat's exile pile the
//     card physically lives in — only the RequireController gate
//     matters for who can cast.
//
//   - Pre-PR-#683 bug: the handler wrote `gs.Seats[perm.Controller]
//     .Exile = append(..., top)` to cross-seat-route every exiled
//     card into Etali's controller's exile pile (the comment in
//     the old handler claimed "the exile zone is per-seat... we
//     need them on Etali's controller's pile so RequireController
//     matches" — but RequireController is a SEAT FIELD on the grant,
//     not a zone-location requirement, so the routing was unneeded).
//     That cross-seat routing caused:
//       (a) ZoneConservation false-positives: seat 0's per-seat card
//           census counted the foreign cards as "extra real cards
//           appeared" because the cards belonged to seats 1/2/3 but
//           were physically in seat 0's exile.
//       (b) CardIdentity false-positives: when seat 0 later refilled
//           library / drew / shuffled, the engine could surface the
//           same *Card pointer in two zones (owner's graveyard AND
//           Etali-controller's exile) because the EOT grant cleanup
//           only reclaimed the grant, not the *Card residue.
//     Surfaced as the 828-violation cluster in the Loki r60 25K
//     sweep PR #682 (docs/loki-r60-25k-report.md).
//
//   - The actual "you may cast" choice is deferred to the AI layer
//     (the grant is REGISTERED, not auto-cast). This matches how
//     impulse-play cards like Outpost Siege / Birgi already work in
//     the engine. emitPartial flags the deferred cast for audit.
func registerEtaliPrimalStorm(r *Registry) {
	r.OnTrigger("Etali, Primal Storm", "creature_attacks", etaliPrimalStormAttack)
}

func etaliPrimalStormAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "etali_primal_storm"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Phase 3 (docs/instanceid-system-v2-r60.md §4.2 + §7): mint an
	// AbilityInstance for this attack trigger and stamp its InstanceID
	// on every cast-grant registered below. Per CR §112.7a the ability
	// is independent of its source on the stack — the grant lifetime
	// binds to the AbilityInstance, not to Etali's battlefield-lifetime.
	// Etali leaving play after the trigger resolves does NOT reclaim
	// the until-EOT grant; the exiled cards stay in exile per design.
	abilityInst := gameengine.NewAbilityInstance(gs, perm, perm.Controller, "trig:creature_attacks", "", nil)
	abilityInstID := ""
	if abilityInst != nil {
		abilityInstID = abilityInst.InstanceID
	}

	exiled := []string{}
	nonLandGrants := 0

	for seatIdx, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if len(s.Library) == 0 {
			continue
		}
		// Library top = LAST index in this engine's convention.
		top := s.Library[len(s.Library)-1]
		if top == nil {
			continue
		}
		// PR #683: route to the OWNER's exile, not Etali-controller's,
		// per CR §400.7c. The grant lookup keys on the *Card pointer
		// (gs.ZoneCastGrants[*Card]), so which seat's exile pile the
		// card lives in doesn't affect the cast-from-exile permission
		// path — only the RequireController gate on the grant matters
		// for who's allowed to cast. Using MoveCard with the canonical
		// "library" → "exile" zones lets the engine fire
		// FireZoneChangeTriggers + card_exiled + zone_change cleanup
		// hooks (which the old buggy moveCardBetweenZones(..., "library
		// _remove", ...) + manual append bypassed entirely — the
		// "library_remove" string wasn't a real zone, so MoveCard's
		// destination branch returned "" and skipped every cleanup).
		moveCardBetweenZones(gs, seatIdx, top, "library", "exile", "etali_exile")
		exiled = append(exiled, top.DisplayName())
		gs.LogEvent(gameengine.Event{
			Kind:   "etali_exile",
			Seat:   seatIdx,
			Target: perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"card":         top.DisplayName(),
				"from_library": seatIdx,
				"to_exile":     seatIdx, // PR #683: owner's exile, not Etali-controller's
				"cast_grant":   perm.Controller, // RequireController for any free-cast
			},
		})
		// Register the free-cast grant for nonlands only. Phase 3
		// (design v2 §4.2 + §7): stamp AbilityInstanceID and
		// LinkageKind=CastGrant so the grant's forensic lineage points
		// back to this attack trigger's AbilityInstance, and the
		// two-pronged ExileLinkageIntegrity invariant routes through
		// the self-managed (cast-window state machine) check rather
		// than the source-held (LTBReturn) check. The until-EOT
		// Duration is retained — the AI cast window for impulse-cast
		// is deferred past the trigger's immediate resolution.
		if !cardHasType(top, "land") {
			grant := gameengine.NewFreeCastFromExilePermission(perm.Controller, perm.Card.DisplayName())
			grant.Duration = "until_end_of_turn"
			grant.GrantTurn = gs.Turn
			grant.AbilityInstanceID = abilityInstID
			grant.LinkageKind = gameengine.CastGrant
			gameengine.RegisterZoneCastGrant(gs, top, grant)
			nonLandGrants++
		}
	}

	// The "you may cast" choice is AI/Hat territory — the grants are
	// registered with end-of-turn expiry; the engine's cast-from-exile
	// pipeline will resolve any chosen casts during this trigger's
	// resolution window. Flag the deferred choice so audits notice.
	if nonLandGrants > 0 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"auto-cast of exiled nonlands deferred to AI/Hat layer")
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"exiled_count":      len(exiled),
		"nonland_grants":    nonLandGrants,
		"exiled":            exiled,
		"ability_instance":  abilityInstID,
	})
}
