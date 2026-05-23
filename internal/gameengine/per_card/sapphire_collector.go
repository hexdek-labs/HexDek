package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSapphireCollector wires Sapphire Collector.
//
// Oracle text (verified via hexdek.dev oracle endpoint 2026-05-22):
//
//	{2}{R}
//	Creature — Human Mercenary
//	Prowess
//	When you cast your second noncreature spell in a turn, conjure a
//	card named Mox Sapphire into your hand. This ability triggers
//	only once.
//	{2}{U}: Target instant or sorcery card in your graveyard gains
//	flashback until end of turn. The flashback cost is equal to its
//	mana cost.
//
// Implementation:
//   - OnActivated abilityIdx 0 ({2}{U} flashback grant): pays the mana
//     cost (mana-pool-only model; symbol-typed pool work is out of
//     scope), calls gameengine.ActivatedFlashbackGrant.
//   - Prowess is keyword pipeline.
//   - "conjure a card named Mox Sapphire" trigger requires a noncreature-
//     cast counter plus the conjure primitive (creating a brand-new card
//     ex nihilo) — neither is in per_card scope; emitPartial.
func registerSapphireCollector(r *Registry) {
	r.OnActivated("Sapphire Collector", sapphireCollectorActivate)
}

func sapphireCollectorActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sapphire_collector_flashback_grant"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	const cost = 3 // {2}{U}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	if seat.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required": cost,
			"have":     seat.ManaPool,
		})
		return
	}
	seat.ManaPool -= cost
	gameengine.SyncManaAfterSpend(seat)

	var target *gameengine.Card
	if v, ok := ctx["target_card"].(*gameengine.Card); ok {
		target = v
	}
	granted := gameengine.ActivatedFlashbackGrant(gs, gameengine.ActivatedFlashbackGrantOptions{
		Source: src.Card.DisplayName(),
		Seat:   src.Controller,
		Target: target,
	})
	if len(granted) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_instant_or_sorcery_in_graveyard", nil)
		return
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": granted[0].DisplayName(),
		"cost":   cost,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"second_noncreature_cast_trigger_and_mox_sapphire_conjure_not_modeled")
}
