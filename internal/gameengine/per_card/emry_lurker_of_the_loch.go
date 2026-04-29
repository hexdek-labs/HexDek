package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEmryLurkerOfTheLoch wires up Emry, Lurker of the Loch.
//
// Oracle text:
//
//	This spell costs {1} less to cast for each artifact you control.
//	When Emry, Lurker of the Loch enters the battlefield, mill four
//	cards.
//	{T}: You may cast target artifact card from your graveyard. If
//	that spell would be put into a graveyard this turn, exile it
//	instead. Activate only once each turn.
//
// Urza / Sai / artifact-combo staple. The mill-4 ETB seeds the grave,
// and the tap ability recurs artifacts — Mox Opal, Mox Amber, Chromatic
// Sphere, etc., often combo'd with Isochron Scepter or Aetherflux
// Reservoir.
//
// Batch #2 scope:
//   - OnETB: mill 4 (top 4 of controller's library → graveyard).
//   - OnActivated(0, ctx["target_card"]): move a target artifact card
//     from graveyard to exile and log a "may_cast" event. We don't
//     execute the cast-from-graveyard (zone-cast plumbing pending).
//     Set perm.Flags["emry_activated_this_turn"] = 1 to enforce the
//     "once each turn" clause at the caller level.
//
// Cost reduction clause ({1} less per artifact you control) is handled
// by the cast pipeline — not per_card business.
func registerEmryLurkerOfTheLoch(r *Registry) {
	r.OnETB("Emry, Lurker of the Loch", emryETB)
	r.OnActivated("Emry, Lurker of the Loch", emryActivate)
}

func emryETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "emry_mill_four"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	milled := 0
	for i := 0; i < 4 && len(s.Library) > 0; i++ {
		c := s.Library[0]
		gameengine.MoveCard(gs, c, seat, "library", "graveyard", "mill")
		milled++
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "mill",
		Seat:   seat,
		Target: seat,
		Source: perm.Card.DisplayName(),
		Amount: milled,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"milled": milled,
	})
}

func emryActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "emry_cast_artifact_from_grave"
	if gs == nil || src == nil {
		return
	}
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	if src.Flags["emry_activated_this_turn"] > 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "already_activated_this_turn", nil)
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	var target *gameengine.Card
	if v, ok := ctx["target_card"].(*gameengine.Card); ok {
		target = v
	}
	if target == nil {
		// Fallback: pick the most recently milled artifact in graveyard.
		for i := len(s.Graveyard) - 1; i >= 0; i-- {
			c := s.Graveyard[i]
			if cardHasType(c, "artifact") {
				target = c
				break
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_artifact_in_graveyard", nil)
		return
	}
	// Per oracle: "If that spell would be put into a graveyard this
	// turn, exile it instead." For MVP we go straight to exile to
	// approximate the end-state.
	gameengine.MoveCard(gs, target, seat, "graveyard", "exile", "exile-from-graveyard")
	src.Flags["emry_activated_this_turn"] = 1
	gs.LogEvent(gameengine.Event{
		Kind:   "per_card_cast_from_grave",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"card":   target.DisplayName(),
			"to":     "exile",
			"reason": "emry_activated",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"cast_card":     target.DisplayName(),
		"final_zone":    "exile",
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"cast_from_grave_not_truly_cast_zone_cast_plumbing_not_implemented")
}
