package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZimoneAndDina wires Zimone and Dina.
//
// Oracle text:
//
//	Whenever you draw your second card each turn, target opponent loses
//	2 life and you gain 2 life.
//	{T}, Sacrifice another creature: Draw a card. You may put a land
//	card from your hand onto the battlefield tapped. If you control
//	eight or more lands, repeat this process once.
//
// Implementation:
//   - "card_drawn" gated on drawer_seat == perm.Controller. Track
//     draws-this-turn on perm.Flags. On the second draw of the turn,
//     target lowest-life living opponent: drain 2, gain 2.
//   - Activated ability: tap + sac another creature → draw 1; optionally
//     play a land from hand tapped; if 8+ lands, repeat once.
func registerZimoneAndDina(r *Registry) {
	r.OnTrigger("Zimone and Dina", "card_drawn", zimoneDinaCardDrawn)
	r.OnActivated("Zimone and Dina", zimoneDinaActivate)
}

func zimoneDinaCardDrawn(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zimone_and_dina_second_draw_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	drawerSeat, ok := ctx["drawer_seat"].(int)
	if !ok || drawerSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["zimone_turn"] != gs.Turn {
		perm.Flags["zimone_turn"] = gs.Turn
		perm.Flags["zimone_draws"] = 0
	}
	perm.Flags["zimone_draws"]++
	if perm.Flags["zimone_draws"] != 2 {
		return
	}
	target := -1
	bestLife := 1 << 30
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		return
	}
	gameengine.LoseLife(gs, target, 2, perm.Card.DisplayName())
	_ = gs.CheckEnd()
	gameengine.GainLife(gs, perm.Controller, 2, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"opponent": target,
	})
}

func zimoneDinaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "zimone_and_dina_sac_draw"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 || src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "tapped_or_wrong_idx", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}
	// Need another creature to sacrifice.
	var sacFodder *gameengine.Permanent
	bestPow := 1 << 30
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == src || !p.IsCreature() {
			continue
		}
		if pp := p.Power(); pp < bestPow {
			bestPow = pp
			sacFodder = p
		}
	}
	if sacFodder == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_sac_target", nil)
		return
	}
	src.Tapped = true

	zimoneDinaDoOnce(gs, src, sacFodder)

	// Repeat if 8+ lands. We need fresh fodder.
	landCount := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsLand() {
			landCount++
		}
	}
	if landCount >= 8 {
		var sac2 *gameengine.Permanent
		bp := 1 << 30
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || p == src || !p.IsCreature() {
				continue
			}
			if pp := p.Power(); pp < bp {
				bp = pp
				sac2 = p
			}
		}
		if sac2 != nil {
			zimoneDinaDoOnce(gs, src, sac2)
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  src.Controller,
		"lands": landCount,
	})
}

func zimoneDinaDoOnce(gs *gameengine.GameState, src, fodder *gameengine.Permanent) {
	// SacrificePermanent (CR §701.17) handles §614 "would die"
	// replacements, §903.9b commander redirect (Zimone-and-Dina decks
	// often sacrifice legendary commanders for value — Yawgmoth /
	// Ashnod's Altar style), detachAll for auras/equipment, replacement
	// unregister, dies/LTB triggers, descend tracking, and the proper
	// sacrifice event log. The pre-fix moveCardBetweenZones direct call
	// bypassed all of these.
	gameengine.SacrificePermanent(gs, fodder, "zimone_dina_sac")
	drawOne(gs, src.Controller, src.Card.DisplayName())
	// Drop a land from hand tapped if possible.
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	for _, c := range seat.Hand {
		if c == nil || !cardHasType(c, "land") {
			continue
		}
		moveCardBetweenZones(gs, src.Controller, c, "hand", "battlefield", "zimone_dina_land_drop")
		// Find the new permanent and tap it.
		for _, p := range seat.Battlefield {
			if p != nil && p.Card == c {
				p.Tapped = true
				break
			}
		}
		break
	}
}
