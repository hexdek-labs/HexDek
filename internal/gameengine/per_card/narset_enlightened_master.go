package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNarsetEnlightenedMaster wires Narset, Enlightened Master.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	First strike, hexproof
//	Whenever Narset attacks, exile the top four cards of your library.
//	Until end of turn, you may cast noncreature spells from among them
//	without paying their mana costs.
//
// Implementation:
//   - First strike and hexproof are handled by the AST keyword pipeline;
//     this handler does not re-implement them.
//   - OnTrigger("creature_attacks"): gate on atk == perm (Narset herself is
//     attacking). Exile the top four cards of the controller's library via
//     MoveCard (library → exile). For each exiled card that is not a
//     creature, register a ZoneCastPermission (ManaCost: 0, zone: "exile")
//     so the AI/Hat treats it as free-castable from exile this turn.
//     Creature cards are also exiled but receive no cast grant (Narset's
//     ability only covers noncreature spells). A single DelayedTrigger at
//     "next_end_step" removes the cast grants for all exiled cards,
//     bounding the "until end of turn" window.
//   - emitPartial: the creature-card window is not granted per oracle text,
//     which is correctly enforced by skipping creature cards during the
//     grant loop. Land cards that are noncreature receive a grant but
//     cannot be "cast" as spells; this is noted as a partial because
//     playing a land from exile via this ability requires a special
//     land-drop rule that the current engine does not track.
//
// emitPartial gaps:
//   - Land cards among the four receive a cast grant but playing a land
//     from exile this way is not enforced as a land-drop restriction.
//   - "until end of turn" is modeled via a next_end_step delayed trigger
//     that removes grants; the window is bounded correctly for typical
//     turns but may drift on extra turns (same caveat as other impulse
//     handlers: ob_nixilis_captive, ashling_limitless).
func registerNarsetEnlightenedMaster(r *Registry) {
	r.OnTrigger("Narset, Enlightened Master", "creature_attacks", narsetEnlightenedMasterAttack)
}

func narsetEnlightenedMasterAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "narset_enlightened_master_attack_exile"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: only fires when Narset herself is the attacking creature.
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}

	// Exile up to 4 cards from the top of the library.
	const exileCount = 4
	exiledCards := make([]*gameengine.Card, 0, exileCount)
	exiledNames := make([]string, 0, exileCount)
	grantedNames := make([]string, 0, exileCount)
	hasLand := false

	for i := 0; i < exileCount && len(seat.Library) > 0; i++ {
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "exile", "narset_enlightened_master_exile")
		exiledCards = append(exiledCards, top)
		exiledNames = append(exiledNames, top.DisplayName())

		// Grant free cast permission for noncreature cards.
		if !cardHasType(top, "creature") {
			gs.ZoneCastGrants[top] = &gameengine.ZoneCastPermission{
				Zone:              gameengine.ZoneExile,
				Keyword:           "narset_enlightened_master_free_cast",
				ManaCost:          0, // without paying mana cost
				ExileOnResolve:    false,
				RequireController: perm.Controller,
				SourceName:        perm.Card.DisplayName(),
			}
			grantedNames = append(grantedNames, top.DisplayName())
			if cardHasType(top, "land") {
				hasLand = true
			}
		}
	}

	// Register a next_end_step delayed trigger to revoke all cast grants,
	// bounding Narset's ability to "until end of turn".
	if len(grantedNames) > 0 {
		cardsToRevoke := exiledCards // capture slice
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName() + " (cast-grant cleanup)",
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				if gs.ZoneCastGrants == nil {
					return
				}
				for _, c := range cardsToRevoke {
					if c == nil {
						continue
					}
					delete(gs.ZoneCastGrants, c)
				}
			},
		})
	}

	emitPartial(gs, slug, perm.Card.DisplayName(),
		"until_end_of_turn_bounded_by_next_end_step_delayed_trigger_may_drift_on_extra_turns")
	if hasLand {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"land_card_free_cast_grant_issued_but_land_drop_zone_restriction_not_enforced")
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"exiled_count":   len(exiledNames),
		"exiled":         exiledNames,
		"granted_count":  len(grantedNames),
		"granted":        grantedNames,
	})
}
