package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMeria wires Meria, Scholar of Antiquity.
//
// Oracle text:
//
//	Nontoken artifacts you control have "{T}: Add {G}."
//	Nontoken artifacts you control have "{T}: Exile the top card of
//	your library. You may play it this turn."
//
// The grants-static-ability-to-other-permanents shape isn't yet
// supported by the per-card layer system, so we register an
// activated-ability handler on Meria herself. The hat's activation
// heuristic surfaces these abilities through Meria as a routing point;
// the handler resolves them as if a granted artifact tap had fired.
//
// Ability 0: tap an artifact you control → add {G}.
// Ability 1: tap an artifact you control → exile top of library, may
// play this turn (Narset-style ZoneCastGrant).
//
// ctx["target_perm"] (optional) carries the artifact being tapped. If
// absent we pick the first untapped nontoken artifact we control as a
// fallback so the activation still resolves cleanly in simulation.
func registerMeria(r *Registry) {
	r.OnActivated("Meria, Scholar of Antiquity", meriaActivate)
}

func meriaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil {
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || p == src {
				continue
			}
			if !p.IsArtifact() || p.Tapped {
				continue
			}
			if cardHasType(p.Card, "token") {
				continue
			}
			target = p
			break
		}
	}

	switch abilityIdx {
	case 0:
		const slug = "meria_artifact_tap_for_g"
		if target == nil {
			emitFail(gs, slug, src.Card.DisplayName(), "no_untapped_nontoken_artifact", nil)
			return
		}
		if target.Controller != seat || !target.IsArtifact() || target.Tapped || cardHasType(target.Card, "token") {
			emitFail(gs, slug, src.Card.DisplayName(), "invalid_target_artifact", nil)
			return
		}
		target.Tapped = true
		gameengine.AddMana(gs, s, "G", 1, src.Card.DisplayName())
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"tapped_card": target.Card.DisplayName(),
			"color":       "G",
		})
	case 1:
		const slug = "meria_artifact_tap_for_impulse"
		if target == nil {
			emitFail(gs, slug, src.Card.DisplayName(), "no_untapped_nontoken_artifact", nil)
			return
		}
		if target.Controller != seat || !target.IsArtifact() || target.Tapped || cardHasType(target.Card, "token") {
			emitFail(gs, slug, src.Card.DisplayName(), "invalid_target_artifact", nil)
			return
		}
		if len(s.Library) == 0 {
			emitFail(gs, slug, src.Card.DisplayName(), "library_empty", nil)
			return
		}
		target.Tapped = true
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "exile", "meria_impulse")
		if gs.ZoneCastGrants == nil {
			gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
		}
		gs.ZoneCastGrants[card] = &gameengine.ZoneCastPermission{
			Zone:              "exile",
			Keyword:           "meria_impulse",
			ManaCost:          -1,
			RequireController: seat,
			SourceName:        src.Card.DisplayName(),
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "exile_from_library",
			Seat:   seat,
			Source: src.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"card":   card.DisplayName(),
				"reason": "meria_impulse",
			},
		})
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"tapped_card": target.Card.DisplayName(),
			"exiled_card": card.DisplayName(),
		})
		emitPartial(gs, slug, src.Card.DisplayName(),
			"granted_ability_on_artifacts_relies_on_meria_routing_until_static_grants_layer_lands")
	}
}
