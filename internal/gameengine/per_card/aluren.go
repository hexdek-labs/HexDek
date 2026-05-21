package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAluren wires Aluren.
//
// Oracle text (Scryfall, verified — Tempest):
//
//	Any player may cast creature spells with mana value 3 or less
//	without paying their mana cost and as though they had flash.
//
// R55 implementation — built on the ZoneCastPolicy primitive:
//   - ETB registers a ZoneCastPolicy on gs.ZoneCastPolicies with:
//       Zone        = "hand"
//       OwnerScope  = "caster"    (each player casts cards from
//                                  their OWN hand — per-caster
//                                  symmetry; the "caster" scope was
//                                  added in R55 specifically for this
//                                  shape and contrasts with "self"
//                                  which means "owned by the source's
//                                  controller")
//       CasterScope = "any"       (any player may use the grant)
//       Predicate   = creature card AND mana value ≤ 3
//       ManaCost    = 0           (without paying their mana cost)
//       Duration    = while_source_on_bf
//   - The "as though they had flash" timing rider is approximated by
//     also stamping a per-seat "aluren_flash_grant_active" flag so
//     the cast-legality check on instant-speed castability for cheap
//     creatures has a contract flag to read. The full flash-timing
//     pipeline is engine-deep; emitPartial flags the boundary.
//   - LTB unregisters the policy and clears the flash flags.
func registerAluren(r *Registry) {
	r.OnETB("Aluren", alurenETBRegisterPolicy)
	r.OnTrigger("Aluren", "permanent_ltb", alurenLTBUnregister)
}

func alurenETBRegisterPolicy(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "aluren_etb_register_policy"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:      perm,
		HandlerID:       "aluren_free_creature_mv3",
		Zone:            gameengine.ZoneHand,
		OwnerScope:      "caster", // each player casts from their OWN hand
		CasterScope:     "any",    // every player may use the grant
		ControllerSeat:  controller,
		Predicate:       alurenPredicate,
		ManaCost:        0,
		Duration:        "while_source_on_bf",
		SourceTimestamp: perm.Timestamp,
		GrantTurn:       gs.Turn,
	})
	// Per-seat flash grant flag for cast-legality scanners that gate
	// on sorcery-speed restrictions.
	for i := range gs.Seats {
		s := gs.Seats[i]
		if s == nil {
			continue
		}
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		s.Flags["aluren_flash_grant_active"]++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   controller,
		"policy": "any_player_free_creature_mv3_from_hand_with_flash",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"flash_timing_rider_engine_cast_pipeline_partial")
}

func alurenLTBUnregister(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterZoneCastPoliciesForPermanent(perm)
	for _, s := range gs.Seats {
		if s == nil || s.Flags == nil {
			continue
		}
		if s.Flags["aluren_flash_grant_active"] > 0 {
			s.Flags["aluren_flash_grant_active"]--
			if s.Flags["aluren_flash_grant_active"] == 0 {
				delete(s.Flags, "aluren_flash_grant_active")
			}
		}
	}
}

// alurenPredicate filters cards eligible for the free cast: creature
// type AND mana value <= 3. Reads mana value via per_card cardCMC
// helper (which honors "cmc:N" type tags + falls back to base P/T
// proxy for test fixtures) rather than engine manaCostOf (which keys
// off "cost:N" type tags — a different test convention).
func alurenPredicate(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if !cardHasType(c, "creature") {
		return false
	}
	return cardCMC(c) <= 3
}
