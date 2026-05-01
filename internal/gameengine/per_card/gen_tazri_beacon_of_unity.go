package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTazriBeaconOfUnity wires Tazri, Beacon of Unity.
//
// Oracle text:
//
//   This spell costs {1} less to cast for each creature in your party.
//   {2/U}{2/B}{2/R}{2/G}: Look at the top six cards of your library. You may reveal up to two Cleric, Rogue, Warrior, Wizard, and/or Ally cards from among them and put them into your hand. Put the rest on the bottom of your library in a random order.
//
// Auto-generated activated ability handler.
func registerTazriBeaconOfUnity(r *Registry) {
	r.OnActivated("Tazri, Beacon of Unity", tazriBeaconOfUnityActivate)
}

func tazriBeaconOfUnityActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "tazri_beacon_of_unity_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(), "auto-gen: activated effect not parsed from oracle text")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
