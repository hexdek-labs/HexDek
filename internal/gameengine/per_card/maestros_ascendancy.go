package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMaestrosAscendancy wires Maestros Ascendancy (SNC, U/B/R, 3
// mana enchantment).
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Maestros%20Ascendancy):
//
//	Once during each of your turns, you may cast an instant or sorcery
//	spell from your graveyard by sacrificing a creature in addition to
//	paying its other costs. If a spell cast this way would be put into
//	your graveyard, exile it instead.
//
// Same Kess Dissident Mage shape with a sacrifice-a-creature
// additional cost layered on. Engine handles cost payment, cast
// triggers, and the exile-on-resolve replacement via
// NewOncePerTurnGraveyardCastPermission (gameengine/zone_cast.go).
// LTB cleanup is handled by the engine's EOT cleanup
// (ExpireZoneCastGrants via the "while_source_on_bf" duration). Until
// that runs, the oncePerTurnConsumed guard in CastFromZone keeps stale
// grants inert.
func registerMaestrosAscendancy(r *Registry) {
	r.OnETB("Maestros Ascendancy", maestrosAscendancyETB)
	r.OnTrigger("Maestros Ascendancy", "zone_change", maestrosAscendancyRefresh)
	r.OnTrigger("Maestros Ascendancy", "creature_dies", maestrosAscendancyRefresh)
}

func maestrosAscendancyETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "maestros_ascendancy"
	if gs == nil || perm == nil {
		return
	}
	granted := grantMaestrosAscendancy(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"grants_added":     granted,
		"keyword":          "once_per_turn_cast_from_graveyard",
		"filter":           "instant_or_sorcery",
		"additional_cost":  "sacrifice_creature",
		"exile_on_resolve": true,
	})
}

func maestrosAscendancyRefresh(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	grantMaestrosAscendancy(gs, perm)
}

func grantMaestrosAscendancy(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	granted := 0
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			continue
		}
		if gameengine.GetZoneCastGrant(gs, c) != nil {
			continue
		}
		p := gameengine.NewOncePerTurnGraveyardCastPermission(
			seatIdx,
			perm.Card.DisplayName(),
			perm.Timestamp,
			-1,   // pay card's own mana cost
			true, // exile on resolve
			[]*gameengine.AdditionalCost{
				{
					Kind:            gameengine.AddCostKindSacrifice,
					Label:           "sacrifice a creature (Maestros Ascendancy)",
					SacrificeFilter: "creature",
				},
			},
		)
		gameengine.RegisterZoneCastGrant(gs, c, p)
		granted++
	}
	return granted
}
