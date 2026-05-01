package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDoranTheSiegeTower wires Doran, the Siege Tower.
//
// Oracle text:
//
//	Defender
//	Each creature assigns combat damage equal to its toughness rather
//	than its power.
//
// Implementation:
//   - The combat-damage rule change is implemented as a layer 7d P/T
//     swap on every creature, registered by gameengine.RegisterDoranSiegeTower
//     (layers.go). That helper is auto-invoked from
//     RegisterContinuousEffectsForPermanent on every ETB, so the layer
//     effect is wired without help from this handler.
//   - This per_card stub fires on ETB to emit a tracking event so audits,
//     parity checks, and registered-card sweeps see Doran as covered.
func registerDoranTheSiegeTower(r *Registry) {
	r.OnETB("Doran, the Siege Tower", doranTheSiegeTowerETB)
}

func doranTheSiegeTowerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "doran_siege_tower_pt_swap"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"layer": "7d",
		"rule":  "613.4d",
		"note":  "global P/T swap registered by RegisterDoranSiegeTower in layers.go",
	})
}
