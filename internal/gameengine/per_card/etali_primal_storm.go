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
// Implementation:
//
//   - OnTrigger("creature_attacks"). Filter: attacker_perm == self.
//
//   - For each LIVING seat (including Etali's controller), pop the
//     top card of their library (last index — engine's library
//     convention: top is len-1, verified against Hermit Druid in
//     per_card_test.go:587-600) and move it to Etali's controller's
//     exile zone via MoveCard with from_zone="library" → "exile".
//
//   - For each exiled card that is NOT a land, register a
//     ZoneCastGrant via NewFreeCastFromExilePermission with Duration
//     "until_end_of_turn" + GrantTurn = gs.Turn, so the grant expires
//     at end of turn per CR §608.2g and the r60 ZoneCastGrantExpiry
//     plumbing (see CLAUDE.md 2026-05-27 closure row + PR #554 /
//     commit 72b95136 ExpireSourceGrants paths). The Hat / AI may then
//     greedy-cast during the trigger's resolution window.
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
		// Move from this seat's library to Etali's controller's exile.
		moveCardBetweenZones(gs, seatIdx, top, "library", "library_remove", "etali_exile")
		// Append to Etali's controller's exile zone explicitly — we
		// can't use MoveCard's exile arm for a cross-seat move (the
		// exile zone is per-seat in this engine; Etali's clause keeps
		// the exiled cards visible to the owner Hat, but for the cast-
		// from-exile grant we need them on Etali's controller's exile
		// pile so RequireController matches).
		gs.Seats[perm.Controller].Exile = append(gs.Seats[perm.Controller].Exile, top)
		exiled = append(exiled, top.DisplayName())
		gs.LogEvent(gameengine.Event{
			Kind:   "etali_exile",
			Seat:   seatIdx,
			Target: perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"card":         top.DisplayName(),
				"from_library": seatIdx,
				"to_exile":     perm.Controller,
			},
		})
		// Register the free-cast grant for nonlands only.
		if !cardHasType(top, "land") {
			grant := gameengine.NewFreeCastFromExilePermission(perm.Controller, perm.Card.DisplayName())
			grant.Duration = "until_end_of_turn"
			grant.GrantTurn = gs.Turn
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
		"seat":            perm.Controller,
		"exiled_count":    len(exiled),
		"nonland_grants":  nonLandGrants,
		"exiled":          exiled,
	})
}
