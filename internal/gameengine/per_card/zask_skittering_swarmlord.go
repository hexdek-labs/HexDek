package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZaskSkitteringSwarmlord wires Zask, Skittering Swarmlord.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	You may play lands and cast Insect spells from your graveyard.
//	Whenever another Insect you control dies, put it on the bottom of
//	its owner's library, then mill two cards. (Put the top two cards
//	of your library into your graveyard.)
//	{1}{B/G}: Target Insect gets +1/+0 and gains deathtouch until end
//	of turn. ({B/G} can be paid with either {B} or {G}.)
//
// Implementation:
//   - "Play lands / cast Insects from graveyard": modeled as static seat
//     flags `zask_lands_from_gy` and `zask_insect_cast_from_gy` (mirrors
//     Muldrotha's `muldrotha_gy_cast` pattern). Downstream zone-cast
//     logic / hat-policy reads the flags to value graveyard cards.
//   - "creature_dies" trigger: when another Insect controlled by Zask's
//     controller dies, locate the dying card in the controller's
//     graveyard, move it to the bottom of the OWNER's library, then mill
//     the top two cards of Zask's controller's library. Per oracle, the
//     bottom-of-library destination is the OWNER (could be an opponent
//     if Zask's controller stole the Insect), so we route via Owner.
//   - OnActivated(0): {1}{B/G} = 2 mana in this engine's ManaPool model.
//     Targets ctx["target_perm"]; falls back to picking the strongest
//     Insect Zask's controller controls. Grants +1/+0 via Modification
//     and deathtouch via GrantedAbilities (both wiped at cleanup §514.2).
func registerZaskSkitteringSwarmlord(r *Registry) {
	r.OnETB("Zask, Skittering Swarmlord", zaskETB)
	r.OnTrigger("Zask, Skittering Swarmlord", "creature_dies", zaskInsectDies)
	r.OnActivated("Zask, Skittering Swarmlord", zaskActivate)
}

func zaskETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["zask_lands_from_gy"] = 1
	seat.Flags["zask_insect_cast_from_gy"] = 1
	emit(gs, "zask_etb_graveyard_cast_static", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "graveyard_land_and_insect_cast_permission_set",
	})
}

func zaskInsectDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zask_insect_dies_recycle_and_mill"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	if dyingCard == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	// "another Insect" — not Zask herself, must be an Insect.
	if dyingCard == perm.Card {
		return
	}
	if !cardHasType(dyingCard, "insect") {
		return
	}

	// Move the dying card from the controller's graveyard to the bottom of
	// its OWNER's library. CR §400.3: a card put into a public zone goes to
	// its owner's zone. The engine's MoveCard already routes via owner; we
	// just need the right (fromSeat, fromZone, toZone) args. The card was
	// just moved to controller's graveyard by the dies event, so we walk
	// every seat's graveyard to find it (defensive — controller_seat is the
	// canonical home, but a §614 replacement could have rerouted).
	gySeat := -1
	for si, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, gc := range s.Graveyard {
			if gc == dyingCard {
				gySeat = si
				break
			}
		}
		if gySeat >= 0 {
			break
		}
	}
	if gySeat < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "dying_card_not_in_graveyard", map[string]interface{}{
			"card": dyingCard.DisplayName(),
		})
		return
	}
	ownerSeat := dyingCard.Owner
	if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
		ownerSeat = gySeat
	}

	// Remove from current graveyard and append to owner's library tail.
	src := gs.Seats[gySeat]
	removed := false
	for i, c := range src.Graveyard {
		if c == dyingCard {
			src.Graveyard = append(src.Graveyard[:i], src.Graveyard[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		emitFail(gs, slug, perm.Card.DisplayName(), "graveyard_remove_failed", map[string]interface{}{
			"card": dyingCard.DisplayName(),
		})
		return
	}
	owner := gs.Seats[ownerSeat]
	owner.Library = append(owner.Library, dyingCard)
	gs.LogEvent(gameengine.Event{
		Kind:   "bottom_of_library",
		Seat:   ownerSeat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug": slug,
			"card": dyingCard.DisplayName(),
			"from": "graveyard",
		},
	})

	// Mill two cards from Zask's controller's library.
	milled := 0
	for i := 0; i < 2; i++ {
		zs := gs.Seats[perm.Controller]
		if zs == nil || len(zs.Library) == 0 {
			break
		}
		c := zs.Library[0]
		gameengine.MoveCard(gs, c, perm.Controller, "library", "graveyard", "mill")
		milled++
	}
	if milled > 0 {
		gs.LogEvent(gameengine.Event{
			Kind:   "mill",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: milled,
			Details: map[string]interface{}{
				"slug": slug,
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"recycled":        dyingCard.DisplayName(),
		"recycled_owner":  ownerSeat,
		"milled":          milled,
	})
}

func zaskActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "zask_pump_insect_deathtouch"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}
	const cost = 2 // {1}{B/G}
	if seat.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      seatIdx,
			"required":  cost,
			"available": seat.ManaPool,
		})
		return
	}

	var target *gameengine.Permanent
	if ctx != nil {
		if v, ok := ctx["target_perm"].(*gameengine.Permanent); ok && v != nil {
			target = v
		}
	}
	if target == nil {
		target = pickBestInsectToPump(gs, seatIdx)
	}
	if target == nil || !target.IsCreature() || target.Card == nil || !cardHasType(target.Card, "insect") {
		emitFail(gs, slug, src.Card.DisplayName(), "no_insect_target", map[string]interface{}{
			"seat": seatIdx,
		})
		return
	}

	seat.ManaPool -= cost
	gameengine.SyncManaAfterSpend(seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "pay_mana",
		Seat:   seatIdx,
		Amount: cost,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"reason": "zask_pump_activation",
		},
	})

	target.Modifications = append(target.Modifications, gameengine.Modification{
		Power:     1,
		Toughness: 0,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	if !target.HasKeyword("deathtouch") {
		target.GrantedAbilities = append(target.GrantedAbilities, "deathtouch")
	}
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"target":    target.Card.DisplayName(),
		"buff":      "+1/+0",
		"granted":   "deathtouch",
		"cost_paid": cost,
	})
}

// pickBestInsectToPump selects the most-impactful Insect controlled by seat
// to receive Zask's pump. Heuristic: highest power (deathtouch makes any
// power lethal in combat, so power directly translates to damage prevented
// or dealt); ties broken by toughness, then earliest timestamp.
func pickBestInsectToPump(gs *gameengine.GameState, seatIdx int) *gameengine.Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestScore := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "insect") {
			continue
		}
		score := p.Power()*4 + p.Toughness()
		if score > bestScore {
			bestScore = score
			best = p
		}
	}
	return best
}
