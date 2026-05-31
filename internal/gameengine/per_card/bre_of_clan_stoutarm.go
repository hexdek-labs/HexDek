package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBreOfClanStoutarm wires Bre of Clan Stoutarm.
//
// Oracle text:
//
//	{1}{W}, {T}: Another target creature you control gains flying and
//	lifelink until end of turn.
//	At the beginning of your end step, if you gained life this turn,
//	exile cards from the top of your library until you exile a nonland
//	card. You may cast that card without paying its mana cost if the
//	spell's mana value is less than or equal to the amount of life you
//	gained this turn. Otherwise, put it into your hand.
//
// Implementation: track life gained per turn; at end step exile down to
// a nonland; if its CMC <= life gained, drop it onto the battlefield (if
// permanent) or send it to hand (instants/sorceries).
//
// R60 stub sweep batch 3: wired the {1}{W}, {T} activated ability —
// tap, pick the highest-power OTHER creature we control as the target,
// stamp Flags["kw:flying"] + Flags["kw:lifelink"] until end of turn
// via a next_end_step delayed-trigger cleanup. Pattern mirrors K-9
// Mark I's unblockable_until_eot grant from batch 1.
func registerBreOfClanStoutarm(r *Registry) {
	r.OnTrigger("Bre of Clan Stoutarm", "life_gained", breTrackLifeGained)
	r.OnTrigger("Bre of Clan Stoutarm", "end_step", breEndStep)
	r.OnActivated("Bre of Clan Stoutarm", breActivate)
}

func breActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "bre_clan_stoutarm_pump"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil {
		// AI fallback: best other creature we control.
		seat := gs.Seats[src.Controller]
		if seat == nil {
			emitFail(gs, slug, src.Card.DisplayName(), "no_seat", nil)
			return
		}
		bestPower := -1
		for _, p := range seat.Battlefield {
			if p == nil || p == src || p.Card == nil || !p.IsCreature() {
				continue
			}
			if p.Power() > bestPower {
				bestPower = p.Power()
				target = p
			}
		}
	}
	if target == nil || target.Card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target", nil)
		return
	}
	if target == src {
		emitFail(gs, slug, src.Card.DisplayName(), "target_is_self_must_be_another", nil)
		return
	}
	if !target.IsCreature() {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_creature", nil)
		return
	}
	src.Tapped = true
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	prevFlying := target.Flags["kw:flying"]
	prevLifelink := target.Flags["kw:lifelink"]
	target.Flags["kw:flying"] = 1
	target.Flags["kw:lifelink"] = 1
	gs.InvalidateCharacteristicsCache()
	captured := target
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: src.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			// Only strip the flags WE set — preserve native AST keywords
			// or other granted flags (same per-flag-discipline as
			// Vihaan from batch 2).
			if prevFlying == 0 {
				delete(captured.Flags, "kw:flying")
			}
			if prevLifelink == 0 {
				delete(captured.Flags, "kw:lifelink")
			}
			gs.InvalidateCharacteristicsCache()
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": target.Card.DisplayName(),
		"effect": "flying_lifelink_until_eot",
	})
}

func breGainKey(turn int) string { return "bre_gained_t" + strconv.Itoa(turn+1) }

func breTrackLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	seat, _ := ctx["seat"].(int)
	if seat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags[breGainKey(gs.Turn)] += amount
}

func breEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bre_clan_stoutarm_end_step"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	gainKey := breGainKey(gs.Turn)
	gained := perm.Flags[gainKey]
	delete(perm.Flags, gainKey)
	if gained <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	landsExiled := 0
	var nonland *gameengine.Card
	for len(seat.Library) > 0 {
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "exile", "bre_end_step")
		if cardHasType(top, "land") {
			landsExiled++
			continue
		}
		nonland = top
		break
	}
	went := "none"
	if nonland != nil {
		cmc := gameengine.ManaCostOf(nonland)
		if cmc <= gained {
			if cardHasType(nonland, "creature") || cardHasType(nonland, "artifact") ||
				cardHasType(nonland, "enchantment") || cardHasType(nonland, "planeswalker") ||
				cardHasType(nonland, "battle") || cardHasType(nonland, "land") {
				gameengine.MoveCard(gs, nonland, perm.Controller, "exile", "battlefield", "bre_free_cast")
				went = "battlefield"
			} else {
				went = "free_cast_partial"
				emitPartial(gs, slug, perm.Card.DisplayName(),
					"instant_or_sorcery_free_cast_resolution_shortcut_unimplemented")
			}
		} else {
			gameengine.MoveCard(gs, nonland, perm.Controller, "exile", "hand", "bre_to_hand")
			went = "hand"
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"life_gained":  gained,
		"lands_exiled": landsExiled,
		"went":         went,
	})
}
