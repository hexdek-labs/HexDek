package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// rakdos_charm.go — per_card handler for Rakdos Charm.
//
// Oracle text (Instant, {B}{R}):
//
//	Choose one —
//	• Exile all cards from target player's graveyard.
//	• Destroy target artifact.
//	• Each creature deals 1 damage to its controller.
//
// Why a per_card handler: the modal body parsed to an inert `parsed_tail`
// scaffold, so this flexible interaction spell (57 decks) did NOTHING.
// Verified inert: no registered handler.
//
// Mode policy (AI): destroying an artifact is the most reliably useful line →
// prefer the best opponent artifact; else exile the fullest opponent graveyard
// (graveyard hate). The symmetric "each creature pings its controller" mode is
// self-harming and intentionally not auto-chosen (logged not-modeled) — the
// handler fizzles rather than damage its own board.
func init() {
	registerRakdosCharm(Global())
	AddResetHook(registerRakdosCharm)
}

func registerRakdosCharm(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Rakdos Charm", rakdosCharmResolve)
}

func rakdosCharmResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "rakdos_charm"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller

	// Mode 2: destroy best opponent artifact (reuses Abrade's picker).
	if art := bestOpponentArtifact(gs, seat); art != nil {
		name := art.Card.DisplayName()
		gameengine.DestroyPermanent(gs, art, nil)
		emit(gs, slug, "Rakdos Charm", map[string]interface{}{
			"seat": seat, "mode": "destroy_artifact", "target": name,
		})
		_ = gs.CheckEnd()
		return
	}

	// Mode 1: exile the fullest opponent graveyard.
	target := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost || len(s.Graveyard) == 0 {
			continue
		}
		if target < 0 || len(s.Graveyard) > len(gs.Seats[target].Graveyard) {
			target = opp
		}
	}
	if target >= 0 {
		snapshot := append([]*gameengine.Card(nil), gs.Seats[target].Graveyard...)
		exiled := 0
		for _, c := range snapshot {
			if c == nil {
				continue
			}
			gameengine.MoveCard(gs, c, target, "graveyard", "exile", "Rakdos Charm")
			exiled++
		}
		emit(gs, slug, "Rakdos Charm", map[string]interface{}{
			"seat": seat, "mode": "exile_graveyard", "target": target, "exiled": exiled,
		})
		return
	}

	emitPartial(gs, slug, "Rakdos Charm", "each_creature_pings_controller_mode_not_modeled_symmetric")
	emitFail(gs, slug, "Rakdos Charm", "no_artifact_or_graveyard_target", nil)
}
