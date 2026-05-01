package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNicolBolas wires Nicol Bolas, the Ravager // Nicol Bolas, the Arisen.
//
// Front face — Nicol Bolas, the Ravager (Legendary Creature — Elder Dragon, 4/4, {1}{U}{B}{R}):
//
//	Flying
//	When Nicol Bolas, the Ravager enters, each opponent discards a card.
//	{4}{U}{B}{R}: Exile Nicol Bolas, the Ravager, then return him to the
//	battlefield transformed under his owner's control. Activate only as
//	a sorcery.
//
// Back face — Nicol Bolas, the Arisen (Legendary Planeswalker — Bolas, loyalty 7):
//
//	+2: Draw two cards.
//	−3: Nicol Bolas, the Arisen deals 10 damage to target creature or
//	    planeswalker.
//	−4: Put target creature or planeswalker card from a graveyard onto
//	    the battlefield under your control.
//	−12: Exile all but the bottom card of target player's library.
//
// Implementation:
//   - Flying — AST keyword pipeline.
//   - OnETB (front face only — gate on !Transformed): each opponent
//     discards one card.
//   - OnActivated front face: pay {4}{U}{B}{R} (≈ 7 generic), then
//     TransformPermanent. The "exile and return" wording is rules-text
//     flavour — the engine treats the face flip in place as equivalent
//     for game-state purposes (no auras/counters to wash off in our
//     simplified model). On transform, initialize loyalty to 7.
//   - OnActivated back face: dispatch by abilityIdx.
//       * +2: gain 2 loyalty, draw 2.
//       * −3: spend 3 loyalty, deal 10 to highest-power opponent
//         creature (or any planeswalker if no creature exists).
//       * −4: spend 4 loyalty, reanimate the highest-power creature/PW
//         in any opponent's graveyard onto controller's battlefield.
//       * −12: spend 12 loyalty, exile all but the bottom card of the
//         most threatening opponent's library.
//
// DFC dispatch: register every face-name variant since perm.Card.Name
// swaps after TransformPermanent (mirrors esika.go / kefka.go pattern).
func registerNicolBolas(r *Registry) {
	full := "Nicol Bolas, the Ravager // Nicol Bolas, the Arisen"
	r.OnETB(full, nicolBolasETB)
	r.OnETB("Nicol Bolas, the Ravager", nicolBolasETB)
	r.OnActivated(full, nicolBolasActivated)
	r.OnActivated("Nicol Bolas, the Ravager", nicolBolasActivated)
	r.OnActivated("Nicol Bolas, the Arisen", nicolBolasActivated)
}

func nicolBolasETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "nicol_bolas_the_ravager_etb_discard"
	if gs == nil || perm == nil {
		return
	}
	if perm.Transformed {
		return
	}
	discarded := 0
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		discarded += gameengine.DiscardN(gs, oppIdx, 1, "")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"discarded": discarded,
	})
}

func nicolBolasActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	if !src.Transformed {
		nicolBolasFrontActivate(gs, src)
		return
	}
	switch abilityIdx {
	case 0:
		nicolBolasPlusTwo(gs, src)
	case 1:
		nicolBolasMinusThree(gs, src)
	case 2:
		nicolBolasMinusFour(gs, src)
	case 3:
		nicolBolasMinusTwelve(gs, src)
	default:
		nicolBolasPlusTwo(gs, src)
	}
}

func nicolBolasFrontActivate(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "nicol_bolas_the_ravager_transform"
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	const cost = 7 // {4}{U}{B}{R}
	if seat.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      seatIdx,
			"required":  cost,
			"available": seat.ManaPool,
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
			"reason": "nicol_bolas_transform",
		},
	})
	if !gameengine.TransformPermanent(gs, src, "nicol_bolas_activated_transform") {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"transform_failed_face_data_missing")
		return
	}
	if src.Counters == nil {
		src.Counters = map[string]int{}
	}
	src.Counters["loyalty"] = 7
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"to":        "Nicol Bolas, the Arisen",
		"loyalty":   7,
		"cost_paid": cost,
	})
}

func nicolBolasPlusTwo(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "nicol_bolas_the_arisen_plus_two_draw"
	src.AddCounter("loyalty", 2)
	drawn := 0
	for i := 0; i < 2; i++ {
		if c := drawOne(gs, src.Controller, src.Card.DisplayName()); c == nil {
			break
		}
		drawn++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    src.Controller,
		"loyalty": src.Counters["loyalty"],
		"drew":    drawn,
	})
}

func nicolBolasMinusThree(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "nicol_bolas_the_arisen_minus_three_burn"
	src.AddCounter("loyalty", -3)
	target := nicolBolasPickBurnTarget(gs, src.Controller)
	if target == nil {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":    src.Controller,
			"loyalty": src.Counters["loyalty"],
			"target":  "",
			"damage":  0,
		})
		return
	}
	if target.IsPlaneswalker() {
		// CR §306.7 — damage to a planeswalker removes that many loyalty counters.
		target.AddCounter("loyalty", -10)
	} else {
		target.MarkedDamage += 10
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   src.Controller,
		Target: target.Controller,
		Source: src.Card.DisplayName(),
		Amount: 10,
		Details: map[string]interface{}{
			"target_kind": "creature_or_planeswalker",
			"target_card": target.Card.DisplayName(),
			"combat":      false,
		},
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         src.Controller,
		"loyalty":      src.Counters["loyalty"],
		"target":       target.Card.DisplayName(),
		"target_seat":  target.Controller,
		"damage":       10,
	})
}

func nicolBolasPickBurnTarget(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestPower := -1
	for _, oppIdx := range gs.Opponents(seat) {
		opp := gs.Seats[oppIdx]
		if opp == nil || opp.Lost {
			continue
		}
		for _, p := range opp.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !p.IsCreature() && !p.IsPlaneswalker() {
				continue
			}
			pw := p.Power()
			if pw > bestPower {
				bestPower = pw
				best = p
			}
		}
	}
	return best
}

func nicolBolasMinusFour(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "nicol_bolas_the_arisen_minus_four_reanimate"
	src.AddCounter("loyalty", -4)
	owner := src.Controller
	var bestCard *gameengine.Card
	var bestSeat int = -1
	bestCMC := -1
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasType(c, "creature") && !cardHasType(c, "planeswalker") {
				continue
			}
			cmc := cardCMC(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
				bestSeat = i
			}
		}
	}
	if bestCard == nil {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":    owner,
			"loyalty": src.Counters["loyalty"],
			"target":  "",
		})
		return
	}
	gameengine.MoveCard(gs, bestCard, bestSeat, "graveyard", "battlefield", "nicol_bolas_minus_four_reanimate")
	// "Under your control" — find the freshly-created permanent and
	// reparent it to Nicol's controller if it landed elsewhere.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card != bestCard {
				continue
			}
			if p.Controller != owner {
				removePermanent(gs, p)
				p.Controller = owner
				gs.Seats[owner].Battlefield = append(gs.Seats[owner].Battlefield, p)
			}
			break
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         owner,
		"loyalty":      src.Counters["loyalty"],
		"target":       bestCard.DisplayName(),
		"from_seat":    bestSeat,
	})
}

func nicolBolasMinusTwelve(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "nicol_bolas_the_arisen_minus_twelve_mill"
	src.AddCounter("loyalty", -12)
	// Most threatening opponent = highest life total alive opponent.
	target := -1
	bestLife := -1
	for _, oppIdx := range gs.Opponents(src.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			target = oppIdx
		}
	}
	if target < 0 {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":    src.Controller,
			"loyalty": src.Counters["loyalty"],
			"target":  -1,
		})
		return
	}
	s := gs.Seats[target]
	exiled := 0
	for len(s.Library) > 1 {
		top := s.Library[0]
		gameengine.MoveCard(gs, top, target, "library", "exile", "nicol_bolas_minus_twelve_exile")
		exiled++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         src.Controller,
		"loyalty":      src.Counters["loyalty"],
		"target_seat":  target,
		"exiled":       exiled,
		"library_left": len(s.Library),
	})
}
