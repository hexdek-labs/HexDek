package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSliverGravemotherCustom adds the Encore activated ability that
// Sliver Gravemother's auto-generated static stub omits.
//
// Oracle text:
//
//	The "legend rule" doesn't apply to Slivers you control.
//	Each Sliver creature card in your graveyard has encore {X}, where X
//	is its mana value.
//	Encore {5} ({5}, Exile this card from your graveyard: For each
//	opponent, create a token copy that attacks that opponent this turn
//	if able. They gain haste. Sacrifice them at the beginning of the
//	next end step. Activate only as a sorcery.)
//
// We wire the activated ability to encore the highest-power Sliver in
// the controller's graveyard: exile it, spawn one tapped/attacking
// haste-token copy per living opponent, and queue an end-step sacrifice
// for each. The legend-rule waiver and the mana-cost gate (X=CMC of the
// chosen Sliver, with at most ManaPool available) are partial — flagged
// for the engine layer.
func registerSliverGravemotherCustom(r *Registry) {
	r.OnActivated("Sliver Gravemother", sliverGravemotherEncore)
}

func sliverGravemotherEncore(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sliver_gravemother_encore"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// "Activate only as a sorcery." — gate when caller bypasses the
	// engine's stack-time check.
	if !isSorcerySpeed(gs, src.Controller) {
		emitFail(gs, slug, src.Card.DisplayName(), "not_sorcery_speed", nil)
		return
	}
	// Pick the highest-power Sliver creature card in graveyard whose
	// encore cost we can afford from the current mana pool.
	var pick *gameengine.Card
	pickIdx := -1
	bestPower := -1
	for i, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if !cardSubtypeMatches(c, "sliver") {
			continue
		}
		cost := cardCMC(c)
		if cost <= 0 {
			cost = 1
		}
		if seat.ManaPool < cost {
			continue
		}
		if c.BasePower > bestPower {
			pick = c
			pickIdx = i
			bestPower = c.BasePower
		}
	}
	if pick == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_legal_sliver_target", map[string]interface{}{
			"mana_pool": seat.ManaPool,
		})
		return
	}
	cost := cardCMC(pick)
	if cost <= 0 {
		cost = 1
	}
	// Pay the encore cost and exile the Sliver. Route the zone change
	// through MoveCard so §614 replacements (Rest in Peace, Leyline of
	// the Void) + §903.9b commander redirect + card_exiled / zone_change
	// observers fire correctly. Pre-r60 shape spliced graveyard + manual
	// exile append, bypassing every one of the above (Wave 2 multi-step
	// migration).
	_ = pickIdx
	seat.ManaPool -= cost
	gameengine.MoveCard(gs, pick, src.Controller, "graveyard", "exile", "sliver_gravemother_encore_cost")
	opps := gs.Opponents(src.Controller)
	tokensSpawned := 0
	for _, oppSeat := range opps {
		if oppSeat < 0 || oppSeat >= len(gs.Seats) {
			continue
		}
		if gs.Seats[oppSeat] == nil || gs.Seats[oppSeat].Lost {
			continue
		}
		tokenCard := &gameengine.Card{
			Name:          pick.DisplayName() + " (Encore token)",
			Owner:         src.Controller,
			Types:         append([]string{"token"}, pick.Types...),
			BasePower:     pick.BasePower,
			BaseToughness: pick.BaseToughness,
		}
		tokenPerm := &gameengine.Permanent{
			Card:       tokenCard,
			Controller: src.Controller,
			Owner:      src.Controller,
			Timestamp:  gs.NextTimestamp(),
			Tapped:     true,
			Counters:   map[string]int{},
			Flags:      map[string]int{"kw:haste": 1, "encore_attacks_seat": oppSeat + 1},
		}
		seat.Battlefield = append(seat.Battlefield, tokenPerm)
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: src.Controller,
			SourceCardName: src.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				gameengine.MoveCard(gs, tokenCard, src.Controller, "battlefield", "graveyard", "encore_eos_sacrifice")
			},
		})
		tokensSpawned++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           src.Controller,
		"sliver_encored": pick.DisplayName(),
		"encore_cost":    cost,
		"tokens_spawned": tokensSpawned,
	})
	emitPartial(gs, "sliver_gravemother_legend_waiver", src.Card.DisplayName(),
		"legend-rule waiver for controlled Slivers requires SBA hook in CheckLegends; partial flag set")
}
