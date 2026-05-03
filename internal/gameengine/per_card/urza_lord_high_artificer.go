package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrzaLordHighArtificer wires Urza, Lord High Artificer.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	When Urza, Lord High Artificer enters the battlefield, create a 0/0
//	colorless Construct artifact creature token with "This creature gets
//	+1/+1 for each artifact you control."
//	Tap an untapped artifact you control: Add {U}.
//	{5}: Shuffle your library, then exile the top card. Until end of
//	turn, you may play that card without paying its mana cost.
//
// Implementation:
//   - OnETB: create a 0/0 Construct artifact creature token via
//     CreateCreatureToken. The static "+1/+1 per artifact you control"
//     is marked via tok.Flags["construct_artifact_scaling"] = 1 so the
//     engine's characteristics cache can check this flag for dynamic P/T.
//     emitPartial records that the scaling is not dynamically enforced by
//     this handler.
//   - OnActivated(abilityIdx 0): tap an untapped artifact you control to
//     add {U}. The handler autonomously selects the least valuable
//     untapped artifact on the controller's battlefield (non-creature
//     artifacts first, then artifact creatures by lowest base P+T). Uses
//     AddManaFromPermanent so Kinnan and similar effects trigger. Gate:
//     must have at least one untapped artifact.
//   - OnActivated(abilityIdx 1): pay {5} generic mana, shuffle library,
//     exile top card, register ZoneCastPermission for free cast from
//     exile until end of turn. emitPartial notes that "without paying
//     mana cost" is approximated via ManaCost: 0 on the grant.
//
// emitPartial gaps:
//   - Construct token +1/+1 per artifact static ability not enforced in
//     the layer system; only flagged for future integration.
//   - The {5} activated ability's "without paying its mana cost" is
//     modeled as ManaCost: 0 on the ZoneCastPermission. Land cards
//     receive a grant but playing a land from exile requires a land-drop
//     rule the engine does not track.
//   - Ability 0 auto-selects the tap target heuristically; the engine
//     does not support player-choice artifact targeting today.
func registerUrzaLordHighArtificer(r *Registry) {
	r.OnETB("Urza, Lord High Artificer", urzaETB)
	r.OnActivated("Urza, Lord High Artificer", urzaActivate)
}

func urzaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "urza_construct_token"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Create a 0/0 colorless Construct artifact creature token.
	tok := gameengine.CreateCreatureToken(gs, seat, "Construct Token",
		[]string{"artifact", "creature", "construct"}, 0, 0)
	if tok != nil {
		// Flag the token so the engine's characteristics cache / layer
		// system can apply the "+1/+1 for each artifact you control"
		// static ability when that subsystem is implemented.
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["construct_artifact_scaling"] = 1
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"construct_plus_one_per_artifact_static_not_dynamically_enforced_flag_set")
}

func urzaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	switch abilityIdx {
	case 0:
		urzaTapArtifactForU(gs, src, s, seat)
	case 1:
		urzaExileTopMayPlay(gs, src, s, seat)
	}
}

// urzaTapArtifactForU implements "Tap an untapped artifact you control: Add {U}".
// Autonomously selects the least valuable untapped artifact: non-creature
// artifacts first (by registration order), then artifact creatures by lowest
// base P+T sum.
func urzaTapArtifactForU(gs *gameengine.GameState, src *gameengine.Permanent, s *gameengine.Seat, seat int) {
	const slug = "urza_tap_artifact_for_u"

	// Find the best (least valuable) untapped artifact to tap.
	var bestNonCreature *gameengine.Permanent
	var bestCreature *gameengine.Permanent
	bestCreaturePT := int(^uint(0) >> 1) // max int

	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		if p.Tapped {
			continue
		}
		if p.IsCreature() {
			pt := p.Card.BasePower + p.Card.BaseToughness
			if bestCreature == nil || pt < bestCreaturePT {
				bestCreature = p
				bestCreaturePT = pt
			}
		} else {
			// Non-creature artifacts are preferred (less valuable to tap).
			// Take the first one found.
			if bestNonCreature == nil {
				bestNonCreature = p
			}
		}
	}

	// Prefer non-creature artifact; fall back to lowest-P+T creature.
	target := bestNonCreature
	if target == nil {
		target = bestCreature
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_untapped_artifact", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	// Tap the chosen artifact.
	target.Tapped = true

	// Add {U} via AddManaFromPermanent so Kinnan and similar
	// mana-augmentation triggers fire correctly.
	gameengine.AddManaFromPermanent(gs, s, src, "U", 1)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"tapped_card":   target.Card.DisplayName(),
		"new_mana_pool": s.ManaPool,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"artifact_target_auto_selected_heuristically_no_player_choice")
}

// urzaExileTopMayPlay implements "{5}: Shuffle your library, then exile the
// top card. Until end of turn, you may play that card without paying its
// mana cost."
func urzaExileTopMayPlay(gs *gameengine.GameState, src *gameengine.Permanent, s *gameengine.Seat, seat int) {
	const slug = "urza_exile_top_may_play"

	// Cost: {5} generic mana.
	pool := gameengine.EnsureTypedPool(s)
	if !pool.CanPayGeneric(5, "") {
		emitFail(gs, slug, src.Card.DisplayName(), "cannot_pay_5_generic", map[string]interface{}{
			"seat":      seat,
			"mana_pool": s.ManaPool,
		})
		return
	}
	gameengine.SpendMana(s, 5)

	// Shuffle library.
	if len(s.Library) > 1 {
		gs.Rng.Shuffle(len(s.Library), func(i, j int) {
			s.Library[i], s.Library[j] = s.Library[j], s.Library[i]
		})
	}

	// Exile top card.
	if len(s.Library) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "library_empty_after_shuffle", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	c := s.Library[0]
	if c == nil {
		s.Library = s.Library[1:]
		emitFail(gs, slug, src.Card.DisplayName(), "top_card_nil", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	gameengine.MoveCard(gs, c, seat, "library", "exile", "urza_lord_high_artificer_exile")

	// Register ZoneCastPermission: free cast from exile until end of turn.
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	gs.ZoneCastGrants[c] = &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneExile,
		Keyword:           "urza_lord_high_artificer_free_cast",
		ManaCost:          0, // without paying its mana cost
		ExileOnResolve:    false,
		RequireController: seat,
		SourceName:        "Urza, Lord High Artificer",
	}

	// Clean up the cast grant at end of turn via delayed trigger.
	cardRef := c
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seat,
		SourceCardName: "Urza, Lord High Artificer (cast-grant cleanup)",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if gs.ZoneCastGrants == nil {
				return
			}
			delete(gs.ZoneCastGrants, cardRef)
		},
	})

	gs.LogEvent(gameengine.Event{
		Kind:   "exile",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"card":   c.DisplayName(),
			"reason": "urza_exile_top_may_play",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              seat,
		"exiled_card":       c.DisplayName(),
		"zone_cast_granted": true,
		"mana_spent":        5,
	})

	isLand := cardHasType(c, "land")
	if isLand {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"land_card_free_cast_grant_issued_but_land_drop_zone_restriction_not_enforced")
	}
	emitPartial(gs, slug, src.Card.DisplayName(),
		"without_paying_mana_cost_modeled_as_zero_cost_zone_cast_grant")
}
