package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKarnTheGreatCreator wires Karn, the Great Creator.
//
// Oracle text (Scryfall, verified — War of the Spark):
//
//	Activated abilities of artifacts your opponents control can't
//	be activated.
//	You may cast artifact spells from outside the game (your sideboard).
//	−2: You may reveal an artifact card you own from outside the
//	  game or in exile, then put it into your hand.
//
// R55 implementation — built on the ZoneCastPolicy primitive plus an
// existing per-seat "opp_artifact_activation_disabled" flag for the
// static activation lockout:
//   - ETB registers a ZoneCastPolicy with:
//       Zone        = "outside_the_game"
//       OwnerScope  = "self"          (the printed "an artifact card
//                                      you own"; for the cast clause
//                                      "from outside the game (your
//                                      sideboard)" the implicit owner
//                                      is the casting player)
//       CasterScope = "controller"    (only Karn's controller may cast)
//       Predicate   = artifact card
//       ManaCost    = -1              (normal cost — wishboard cast
//                                      pays the printed mana cost)
//       Duration    = while_source_on_bf
//   - ETB stamps the per-seat "opp_artifact_activation_disabled" flag
//     (refcounted across multiple Karns) for the static lockout. The
//     activated-ability legality scanner reads the flag.
//   - The -2 loyalty ability (tutor an artifact from outside the game
//     or exile to hand) is engine-deep planeswalker-loyalty path that
//     the per_card surface doesn't drive on its own; OnActivated
//     captures the call but emitPartial-flags the wishboard-to-hand
//     tutoring (the engine doesn't model an outside-the-game zone in
//     gameplay, so the tutor is best-effort scoped to exile only).
//   - LTB unregisters the policy and decrements the static lockout
//     refcount.
func registerKarnTheGreatCreator(r *Registry) {
	r.OnETB("Karn, the Great Creator", karnGreatCreatorETB)
	r.OnTrigger("Karn, the Great Creator", "permanent_ltb", karnGreatCreatorLTB)
	r.OnActivated("Karn, the Great Creator", karnGreatCreatorActivate)
}

func karnGreatCreatorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "karn_great_creator_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:      perm,
		HandlerID:       "karn_great_creator_wishboard",
		Zone:            "outside_the_game",
		OwnerScope:      "self",
		CasterScope:     "controller",
		ControllerSeat:  controller,
		Predicate:       karnGreatCreatorArtifactPredicate,
		ManaCost:        -1, // normal cost
		Duration:        "while_source_on_bf",
		SourceTimestamp: perm.Timestamp,
		GrantTurn:       gs.Turn,
	})
	// Static lockout: opponents' artifact activated abilities are
	// disabled. Refcounted (multiple Karns stack).
	for i := range gs.Seats {
		if i == controller {
			continue
		}
		s := gs.Seats[i]
		if s == nil {
			continue
		}
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		s.Flags["opp_artifact_activation_disabled"]++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": controller,
	})
}

func karnGreatCreatorLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterZoneCastPoliciesForPermanent(perm)
	for i := range gs.Seats {
		if i == perm.Controller {
			continue
		}
		s := gs.Seats[i]
		if s == nil || s.Flags == nil {
			continue
		}
		if s.Flags["opp_artifact_activation_disabled"] > 0 {
			s.Flags["opp_artifact_activation_disabled"]--
			if s.Flags["opp_artifact_activation_disabled"] == 0 {
				delete(s.Flags, "opp_artifact_activation_disabled")
			}
		}
	}
}

func karnGreatCreatorActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "karn_great_creator_minus2"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	// -2 loyalty: tutor an artifact card you own from outside the
	// game or exile to hand. The outside-the-game zone isn't modeled
	// in gameplay; tutor from exile only.
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	var bestIdx int = -1
	bestCMC := -1
	for i, c := range seat.Exile {
		if c == nil || c.Owner != seatIdx {
			continue
		}
		if !cardHasType(c, "artifact") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx >= 0 {
		pick := seat.Exile[bestIdx]
		gameengine.MoveCard(gs, pick, seatIdx, "exile", "hand", "karn_great_creator_tutor")
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      seatIdx,
			"retrieved": pick.DisplayName(),
			"from":      "exile",
		})
	}
	emitPartial(gs, slug, src.Card.DisplayName(),
		"outside_the_game_wishboard_tutor_engine_not_modeled_in_gameplay")
}

func karnGreatCreatorArtifactPredicate(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return cardHasType(c, "artifact")
}
