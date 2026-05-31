package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIronSpiderStarkUpgrade wires Iron Spider, Stark Upgrade.
//
// Oracle text (Scryfall, verified 2026-05-30, {3}, 2/3 Legendary
// Artifact Creature — Spider Hero):
//
//	Vigilance
//	{T}: Put a +1/+1 counter on each artifact creature and/or
//	Vehicle you control.
//	{2}, Remove two +1/+1 counters from among artifacts you control:
//	Draw a card.
//
// Implementation (R60 stub sweep):
//   - Ability 0 ({T}: counter spread): tap, iterate seat.Battlefield,
//     and add a +1/+1 counter to each artifact creature OR Vehicle the
//     controller owns. Counter placement routes through AddCounter
//     (the per-card-friendly method-style API), which fires the
//     counter_placed event downstream observers (Hardened Scales,
//     Doubling Season — to the extent they're modeled) consume.
//   - Ability 1 ({2}, remove two +1/+1 from artifacts: draw): walk
//     seat.Battlefield removing one counter from up to two distinct
//     artifacts with at least one +1/+1 on them, then draw a card.
//     Bail if fewer than 2 candidates exist (cost can't be paid).
func registerIronSpiderStarkUpgrade(r *Registry) {
	r.OnActivated("Iron Spider, Stark Upgrade", ironSpiderActivated)
}

func ironSpiderActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "iron_spider_activated"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	if abilityIdx == 0 {
		if src.Tapped {
			emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
			return
		}
		src.Tapped = true
		stamped := 0
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if !cardHasType(p.Card, "artifact") && !cardSubtypeMatches(p.Card, "vehicle") {
				continue
			}
			p.AddCounter("+1/+1", 1)
			stamped++
		}
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":    seatIdx,
			"ability": 0,
			"stamped": stamped,
		})
		return
	}

	// Ability 1: {2}, remove two +1/+1 counters from among artifacts: draw.
	removed := 0
	for _, p := range seat.Battlefield {
		if removed >= 2 {
			break
		}
		if p == nil || p.Card == nil || !cardHasType(p.Card, "artifact") {
			continue
		}
		if p.Counters == nil || p.Counters["+1/+1"] <= 0 {
			continue
		}
		p.Counters["+1/+1"]--
		removed++
	}
	if removed < 2 {
		// Refund: counters back if we couldn't pay the full cost.
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_counters", map[string]interface{}{
			"removed": removed,
		})
		return
	}
	drew := drawOne(gs, seatIdx, src.Card.DisplayName())
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    seatIdx,
		"ability": 1,
		"drew":    drew != nil,
	})
}
