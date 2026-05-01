package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerQuintorius wires Quintorius, History Chaser.
//
// Oracle text (Legendary Planeswalker — Quintorius, loyalty 4):
//
//	Whenever one or more cards leave your graveyard, create a 3/2 red
//	and white Spirit creature token.
//	+1: You may discard a card. If you do, draw two cards, then mill a
//	    card.
//	−4: Spirits you control gain double strike and vigilance until end
//	    of turn.
//	Quintorius, History Chaser can be your commander.
//
// Implementation:
//   - ETB: pin starting loyalty to 4. The engine's stack.go falls back
//     to BaseToughness when no explicit loyalty is set; we set it
//     explicitly so the handler is independent of card-load source
//     (mirrors Tevesh Szat's approach).
//   - The graveyard-leave trigger is at AST index 0. There is no
//     "cards_left_graveyard" engine event yet — same gap as Tormod, the
//     Desecrator (see batch17_sweep.go). emitPartial flags it so audits
//     can find it.
//   - +1 (AST index 1): discard the worst card in hand (highest-CMC
//     land if any, else highest-CMC card), draw two, mill one.
//   - −4 (AST index 2): grant double strike + vigilance via runtime
//     keyword flags to every Spirit we control, with a delayed
//     end-of-turn cleanup.
func registerQuintorius(r *Registry) {
	r.OnETB("Quintorius, History Chaser", quintoriusETB)
	r.OnActivated("Quintorius, History Chaser", quintoriusActivate)
}

func quintoriusETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "quintorius_loyalty_init"
	if gs == nil || perm == nil {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["loyalty"] = 4
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"loyalty": 4,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"graveyard_leave_trigger_observer_unimplemented")
}

func quintoriusActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 1:
		quintoriusPlusOne(gs, src)
	case 2:
		quintoriusMinusFour(gs, src)
	}
}

func quintoriusPlusOne(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "quintorius_plus_one_loot_mill"
	src.AddCounter("loyalty", 1)

	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Discard is optional; only skip when hand is empty.
	discarded := ""
	if len(s.Hand) > 0 {
		discardIdx := quintoriusPickDiscard(s.Hand)
		card := s.Hand[discardIdx]
		discarded = card.DisplayName()
		gameengine.MoveCard(gs, card, seat, "hand", "graveyard", "quintorius_plus_one_discard")
		gs.LogEvent(gameengine.Event{
			Kind:   "discard",
			Seat:   seat,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug": slug,
				"card": discarded,
			},
		})
	}

	// "If you do, draw two, then mill one."
	drawn := 0
	milled := ""
	if discarded != "" {
		for i := 0; i < 2 && len(s.Library) > 0; i++ {
			top := s.Library[0]
			gameengine.MoveCard(gs, top, seat, "library", "hand", "draw")
			drawn++
		}
		if len(s.Library) > 0 {
			top := s.Library[0]
			milled = top.DisplayName()
			gameengine.MoveCard(gs, top, seat, "library", "graveyard", "quintorius_plus_one_mill")
			gs.LogEvent(gameengine.Event{
				Kind:   "mill",
				Seat:   seat,
				Source: src.Card.DisplayName(),
				Details: map[string]interface{}{
					"slug": slug,
					"card": milled,
				},
			})
		}
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"loyalty":   src.Counters["loyalty"],
		"discarded": discarded,
		"drew":      drawn,
		"milled":    milled,
	})
}

// quintoriusPickDiscard returns the index of the card we'd rather lose:
// prefer the highest-CMC land (excess mana), else the highest-CMC card
// in hand. Falls back to index 0.
func quintoriusPickDiscard(hand []*gameengine.Card) int {
	bestLandIdx := -1
	bestLandCMC := -1
	bestAnyIdx := 0
	bestAnyCMC := -1
	for i, c := range hand {
		if c == nil {
			continue
		}
		cmc := cardCMC(c)
		if cardHasType(c, "land") && cmc >= bestLandCMC {
			bestLandCMC = cmc
			bestLandIdx = i
		}
		if cmc > bestAnyCMC {
			bestAnyCMC = cmc
			bestAnyIdx = i
		}
	}
	if bestLandIdx >= 0 {
		return bestLandIdx
	}
	return bestAnyIdx
}

func quintoriusMinusFour(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "quintorius_minus_four_spirit_anthem"
	src.AddCounter("loyalty", -4)

	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	var buffed []*gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "spirit") {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		// Record prior keyword state so the cleanup doesn't strip a
		// flag the creature already had from its own AST.
		hadDS := p.Flags["kw:double_strike"] == 1
		hadVig := p.Flags["kw:vigilance"] == 1
		p.Flags["kw:double_strike"] = 1
		p.Flags["kw:vigilance"] = 1
		buffed = append(buffed, p)

		captured := p
		hadDSCap := hadDS
		hadVigCap := hadVig
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: seat,
			SourceCardName: "Quintorius, History Chaser",
			EffectFn: func(gs *gameengine.GameState) {
				if captured == nil || captured.Flags == nil {
					return
				}
				if !hadDSCap {
					delete(captured.Flags, "kw:double_strike")
				}
				if !hadVigCap {
					delete(captured.Flags, "kw:vigilance")
				}
			},
		})
	}
	if len(buffed) > 0 {
		gs.InvalidateCharacteristicsCache()
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"loyalty":        src.Counters["loyalty"],
		"spirits_buffed": len(buffed),
		"keywords":       []string{"double_strike", "vigilance"},
		"duration":       "until_end_of_turn",
	})
}
