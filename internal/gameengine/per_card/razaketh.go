package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRazakethTheFoulblooded wires up Razaketh, the Foulblooded.
//
// Oracle text:
//
//	Flying, trample
//	Pay 2 life, Sacrifice another creature: Search your library for a
//	card, put that card into your hand, then shuffle.
//
// A repeatable tutor stapled to an 8/8 flying/trample body — the
// dream reanimator target (pairs with Animate Dead, Necromancy), and
// a combo engine when paired with sacrifice fodder + lifegain.
// Canonical cEDH line: reanimate Razaketh + have a single creature in
// play + Bolas's Citadel OR a sac engine → tutor for combo piece →
// sac-fodder-generates another fodder → tutor again → win.
//
// Batch #3 scope:
//   - OnActivated(0, ctx["sacrifice_target"]): pay 2 life, sacrifice
//     a target creature, tutor for a named card. ctx["named_card"]
//     specifies the target; if absent, we pick the top spell in the
//     library (arbitrary) — real tournament code passes the name.
//
// We require the sacrifice_target to be a creature Permanent the
// controller owns and NOT Razaketh itself (oracle: "another
// creature").
func registerRazakethTheFoulblooded(r *Registry) {
	r.OnActivated("Razaketh, the Foulblooded", razakethActivate)
}

func razakethActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "razaketh_tutor"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	var sacTarget *gameengine.Permanent
	if v, ok := ctx["sacrifice_target"].(*gameengine.Permanent); ok {
		sacTarget = v
	}
	if sacTarget == nil {
		// Fallback: pick the lowest-value creature (by timestamp) that
		// is NOT Razaketh itself.
		for _, p := range s.Battlefield {
			if p == nil || p == src {
				continue
			}
			if !p.IsCreature() {
				continue
			}
			if sacTarget == nil || p.Timestamp < sacTarget.Timestamp {
				sacTarget = p
			}
		}
	}
	if sacTarget == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_to_sacrifice", nil)
		return
	}
	if sacTarget == src {
		emitFail(gs, slug, src.Card.DisplayName(), "cannot_sacrifice_razaketh_itself", nil)
		return
	}
	if !sacTarget.IsCreature() {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_creature", nil)
		return
	}
	if sacTarget.Controller != seat {
		emitFail(gs, slug, src.Card.DisplayName(), "must_sacrifice_own_creature", nil)
		return
	}
	// Pay 2 life.
	s.Life -= 2
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: src.Card.DisplayName(),
		Amount: 2,
		Details: map[string]interface{}{
			"reason": "razaketh_activation_cost",
		},
	})
	// Sacrifice the target.
	gameengine.SacrificePermanent(gs, sacTarget, "razaketh_activation")

	// Tutor: find the named card in library, move to hand, then
	// shuffle. If named_card is not specified, pick the first card —
	// real tournament code passes the name.
	var namedCard string
	if v, ok := ctx["named_card"].(string); ok {
		namedCard = v
	}
	var found *gameengine.Card
	for _, c := range s.Library {
		if c == nil {
			continue
		}
		if namedCard != "" && c.DisplayName() == namedCard {
			found = c
			break
		}
	}
	if found == nil && namedCard == "" && len(s.Library) > 0 {
		found = s.Library[0]
	}
	if found == nil {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":         seat,
			"sacrificed":   sacTarget.Card.DisplayName(),
			"named_card":   namedCard,
			"tutor_result": "not_found",
		})
		// Still must shuffle after the "find nothing" resolution.
		// Our deterministic shuffle is a no-op in MVP.
		return
	}
	gameengine.MoveCard(gs, found, seat, "library", "hand", "tutor-to-hand")
	// Shuffle remaining library using gs.Rng.
	if gs.Rng != nil {
		gs.Rng.Shuffle(len(s.Library), func(i, j int) {
			s.Library[i], s.Library[j] = s.Library[j], s.Library[i]
		})
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"sacrificed":   sacTarget.Card.DisplayName(),
		"tutor_result": found.DisplayName(),
		"life_after":   s.Life,
	})
	_ = gs.CheckEnd()
}
