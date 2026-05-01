package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGristTheHungerTide wires Grist, the Hunger Tide (Modern Horizons 2).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary Planeswalker — Grist. Loyalty 3. Mana cost {1}{B}{G}.
//	As long as Grist isn't on the battlefield, it's a 1/1 Insect creature
//	in addition to its other types.
//	+1: Create a 1/1 black and green Insect creature token, then mill a
//	    card. If an Insect card was milled this way, put a loyalty counter
//	    on Grist and repeat this process.
//	−2: You may sacrifice a creature. When you do, destroy target creature
//	    or planeswalker.
//	−5: Each opponent loses life equal to the number of creature cards in
//	    your graveyard.
//
// Implementation:
//   - ETB: pin starting loyalty to 3 (CR §306.5b fallback may differ).
//   - Activated abilities (loyalty cost paid by activation pipeline):
//       * abilityIdx 0 (+1): mint a 1/1 B/G Insect token, then mill the top
//         of the controller's library. If that card had type "insect",
//         bump loyalty by 1 and repeat. Iterate up to 60 times as a hard
//         loop guard against pathological insect-only libraries.
//       * abilityIdx 1 (-2): if the controller has another creature to
//         sacrifice AND there's an opponent creature/planeswalker worth
//         destroying, sacrifice the cheapest creature and destroy the
//         best opponent target (highest CMC, prefer creature with highest
//         power; planeswalker preferred if loyalty <=3 and high impact).
//       * abilityIdx 2 (-5): tally creature cards in Grist's controller's
//         graveyard, drain each living opponent by that amount.
//   - Static "1/1 Insect outside battlefield": planeswalker-as-creature
//     in zones other than battlefield is purely an enabler for Grist as
//     EDH commander; in-engine, the command-zone characteristics aren't
//     consulted for combat, so we emitPartial rather than implement a
//     zone-conditional characteristic layer.
func registerGristTheHungerTide(r *Registry) {
	r.OnETB("Grist, the Hunger Tide", gristETB)
	r.OnActivated("Grist, the Hunger Tide", gristActivate)
}

func gristETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["loyalty"] = 3
	emitPartial(gs, "grist_static_outside_battlefield", perm.Card.DisplayName(),
		"command_zone_creature_characteristics_not_modeled")
}

func gristActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		gristPlusOne(gs, src)
	case 1:
		gristMinusTwo(gs, src)
	case 2:
		gristMinusFive(gs, src)
	}
}

func gristPlusOne(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "grist_plus_one_insect_token_mill"
	src.AddCounter("loyalty", 1)

	seat := src.Controller
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	tokens := 0
	milled := 0
	insects := 0
	for iter := 0; iter < 60; iter++ {
		// Mint a 1/1 B/G Insect token.
		tokenCard := &gameengine.Card{
			Name:          "Insect Token",
			Owner:         seat,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "insect"},
			Colors:        []string{"B", "G"},
			TypeLine:      "Token Creature — Insect",
		}
		enterBattlefieldWithETB(gs, seat, tokenCard, false)
		tokens++

		// Mill 1.
		if len(s.Library) == 0 {
			break
		}
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "graveyard", "grist_mill")
		milled++
		gs.LogEvent(gameengine.Event{
			Kind:   "mill",
			Seat:   seat,
			Source: src.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"card":   card.DisplayName(),
				"reason": "grist_plus_one",
			},
		})

		if !cardHasType(card, "insect") {
			break
		}
		insects++
		src.AddCounter("loyalty", 1)
		// Loop continues — repeat process.
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            seat,
		"loyalty":         src.Counters["loyalty"],
		"tokens_created":  tokens,
		"milled":          milled,
		"insects_milled":  insects,
	})
}

func gristMinusTwo(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "grist_minus_two_sac_destroy"
	src.AddCounter("loyalty", -2)

	seat := src.Controller
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Pick a target first — only sacrifice if there's something worth killing.
	target, targetSeat := gristPickDestroyTarget(gs, seat)
	if target == nil {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":    seat,
			"loyalty": src.Counters["loyalty"],
			"reason":  "no_valid_destroy_target",
		})
		return
	}

	// Pick the cheapest non-Grist creature to sacrifice.
	var victim *gameengine.Permanent
	bestCMC := 1<<31 - 1
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc < bestCMC {
			victim = p
			bestCMC = cmc
		}
	}
	if victim == nil {
		// "may sacrifice" — declined (no creatures to sac).
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":    seat,
			"loyalty": src.Counters["loyalty"],
			"reason":  "no_creature_to_sacrifice",
		})
		return
	}
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "grist_minus_two")

	// "When you do, destroy target creature or planeswalker." — reflexive
	// trigger; resolves the destroy now.
	targetName := target.Card.DisplayName()
	destroyed := gameengine.DestroyPermanent(gs, target, src)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"loyalty":      src.Counters["loyalty"],
		"sacrificed":   victimName,
		"target":       targetName,
		"target_seat":  targetSeat,
		"destroyed":    destroyed,
	})
}

// gristPickDestroyTarget picks the best opponent creature or planeswalker
// to destroy. Heuristic: highest CMC; planeswalkers prioritized over
// equally-priced creatures (planeswalkers typically have higher impact).
func gristPickDestroyTarget(gs *gameengine.GameState, seat int) (*gameengine.Permanent, int) {
	var best *gameengine.Permanent
	bestSeat := -1
	bestScore := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			isCreature := p.IsCreature()
			isPW := p.IsPlaneswalker()
			if !isCreature && !isPW {
				continue
			}
			score := cardCMC(p.Card) * 10
			if isPW {
				score += 5
			}
			if score > bestScore {
				best = p
				bestSeat = opp
				bestScore = score
			}
		}
	}
	return best, bestSeat
}

func gristMinusFive(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "grist_minus_five_creature_drain"
	src.AddCounter("loyalty", -5)

	seat := src.Controller
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	x := 0
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") {
			x++
		}
	}
	drained := 0
	for _, opp := range gs.LivingOpponents(seat) {
		os := gs.Seats[opp]
		if os == nil {
			continue
		}
		os.Life -= x
		gs.LogEvent(gameengine.Event{
			Kind:   "life_loss",
			Seat:   seat,
			Target: opp,
			Source: src.Card.DisplayName(),
			Amount: x,
			Details: map[string]interface{}{
				"reason": "grist_minus_five",
			},
		})
		drained++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            seat,
		"loyalty":         src.Counters["loyalty"],
		"creature_count":  x,
		"opponents_hit":   drained,
	})
}
