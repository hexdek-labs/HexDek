package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKenrithReturnedKing wires Kenrith, the Returned King.
//
// Oracle text (Scryfall, ELD — verified 2026-04-30):
//
//	{R}: All creatures gain trample and haste until end of turn.
//	{1}{G}: Put a +1/+1 counter on target creature.
//	{2}{W}: Target player gains 5 life.
//	{3}{U}: Target player draws a card.
//	{4}{B}: Put target creature card from a graveyard onto the
//	        battlefield under its owner's control.
//
// Note: the user-facing task description had ability 3 ({3}{U}) draw for
// you AND each opponent — printed oracle is "Target player draws a
// card", a single draw. We follow Scryfall.
//
// Implementation:
//   - Each ability is dispatched by abilityIdx (0..4) in a single
//     OnActivated handler that fans out to one helper per mode.
//   - Mana cost is paid by the engine activation pipeline (the cost
//     comes from the AST / activation routing). Each helper documents
//     the printed cost in a comment for traceability.
//   - Targets are read from ctx (target_perm / target_seat). When ctx
//     omits a target the helper picks a sane default for simulation
//     (best creature you control / Kenrith's controller).
func registerKenrithReturnedKing(r *Registry) {
	r.OnActivated("Kenrith, the Returned King", kenrithActivated)
}

func kenrithActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	switch abilityIdx {
	case 0:
		kenrithGrantTrampleHaste(gs, src) // {R}
	case 1:
		kenrithPutPlusOneCounter(gs, src, ctx) // {1}{G}
	case 2:
		kenrithGainFiveLife(gs, src, ctx) // {2}{W}
	case 3:
		kenrithDrawCard(gs, src, ctx) // {3}{U}
	case 4:
		kenrithReanimate(gs, src, ctx) // {4}{B}
	}
}

// {R}: All creatures gain trample and haste until end of turn.
func kenrithGrantTrampleHaste(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "kenrith_r_trample_haste"
	count := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !p.IsCreature() {
				continue
			}
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["kw:trample"] = 1
			p.Flags["kw:haste"] = 1
			p.SummoningSick = false
			count++
		}
	}

	captured := []*gameengine.Permanent{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				captured = append(captured, p)
			}
		}
	}
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: "Kenrith, the Returned King",
		EffectFn: func(gs *gameengine.GameState) {
			for _, p := range captured {
				if p == nil || p.Flags == nil {
					continue
				}
				delete(p.Flags, "kw:trample")
				delete(p.Flags, "kw:haste")
			}
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             src.Controller,
		"creatures_buffed": count,
	})
}

// {1}{G}: Put a +1/+1 counter on target creature.
func kenrithPutPlusOneCounter(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kenrith_1g_plus_one_counter"
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil {
		// Fallback: best creature controller controls (highest power).
		s := gs.Seats[src.Controller]
		if s != nil {
			best := -1
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil || !p.IsCreature() {
					continue
				}
				score := p.Power()
				if score > best {
					best = score
					target = p
				}
			}
		}
	}
	if target == nil || !target.IsCreature() {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_target", nil)
		return
	}
	target.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": target.Card.DisplayName(),
	})
}

// {2}{W}: Target player gains 5 life.
func kenrithGainFiveLife(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kenrith_2w_gain_five_life"
	targetSeat := src.Controller
	if v, ok := ctx["target_seat"].(int); ok && v >= 0 && v < len(gs.Seats) {
		targetSeat = v
	}
	s := gs.Seats[targetSeat]
	if s == nil || s.Lost {
		emitFail(gs, slug, src.Card.DisplayName(), "invalid_target_seat", map[string]interface{}{
			"target_seat": targetSeat,
		})
		return
	}
	s.Life += 5
	gs.LogEvent(gameengine.Event{
		Kind:   "gain_life",
		Seat:   targetSeat,
		Target: targetSeat,
		Source: src.Card.DisplayName(),
		Amount: 5,
		Details: map[string]interface{}{
			"slug":   slug,
			"reason": "kenrith_2w",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":        src.Controller,
		"target_seat": targetSeat,
		"life":        5,
	})
}

// {3}{U}: Target player draws a card.
func kenrithDrawCard(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kenrith_3u_target_draws"
	targetSeat := src.Controller
	if v, ok := ctx["target_seat"].(int); ok && v >= 0 && v < len(gs.Seats) {
		targetSeat = v
	}
	s := gs.Seats[targetSeat]
	if s == nil || s.Lost {
		emitFail(gs, slug, src.Card.DisplayName(), "invalid_target_seat", map[string]interface{}{
			"target_seat": targetSeat,
		})
		return
	}
	drawOne(gs, targetSeat, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":        src.Controller,
		"target_seat": targetSeat,
	})
}

// {4}{B}: Put target creature card from a graveyard onto the battlefield
// under its owner's control.
func kenrithReanimate(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kenrith_4b_reanimate"
	// Find the best creature in any graveyard (highest CMC). The rebirth
	// target is the OWNER's battlefield (printed text — not Kenrith's
	// controller). Falls back to that owner's seat for placement.
	var bestCard *gameengine.Card
	bestCMC := -1
	bestSeat := -1
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil || !cardHasType(c, "creature") {
				continue
			}
			cmc := gameengine.ManaCostOf(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
				bestSeat = i
			}
		}
	}
	if bestCard == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_in_any_graveyard", nil)
		return
	}
	ownerSeat := bestCard.Owner
	if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
		ownerSeat = bestSeat
	}
	gameengine.MoveCard(gs, bestCard, bestSeat, "graveyard", "battlefield", "kenrith_reanimate")
	enterBattlefieldWithETB(gs, ownerSeat, bestCard, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       src.Controller,
		"reanimated": bestCard.DisplayName(),
		"from_seat":  bestSeat,
		"owner_seat": ownerSeat,
		"cmc":        bestCMC,
	})
	_ = gs.CheckEnd()
}
