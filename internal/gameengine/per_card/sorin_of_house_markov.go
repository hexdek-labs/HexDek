package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSorinOfHouseMarkov wires Sorin of House Markov // Sorin, Ravenous Neonate.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
// Front — Sorin of House Markov ({1}{B}, Human Noble, 1/4):
//
//	Lifelink
//	Extort
//	At the beginning of each of your postcombat main phases, if you
//	gained 3 or more life this turn, exile Sorin, then return him to
//	the battlefield transformed under his owner's control.
//
// Back — Sorin, Ravenous Neonate (Planeswalker — Sorin):
//
//	Extort
//	+2: Create a Food token.
//	−1: Sorin deals damage equal to the amount of life you gained
//	    this turn to any target.
//	−6: Gain control of target creature. It becomes a Vampire in
//	    addition to its other types. Put a lifelink counter on it if
//	    you control a white permanent other than that creature or
//	    Sorin.
//
// Implementation:
//   - Lifelink + extort: AST keyword pipeline.
//   - "postcombat_main_controller": if seat.Flags["life_gained_this_turn"]
//     >= 3, transform Sorin.
//   - Back-face activated abilities (loyalty pipeline pays cost):
//       0 (+2): Create Food.
//       1 (-1): Damage to lowest-life opponent equal to life gained.
//       2 (-6): Gain control of opponent's biggest creature.
func registerSorinOfHouseMarkov(r *Registry) {
	r.OnTrigger("Sorin of House Markov // Sorin, Ravenous Neonate", "postcombat_main_controller", sorinPostcombatTransform)
	r.OnTrigger("Sorin of House Markov", "postcombat_main_controller", sorinPostcombatTransform)
	r.OnActivated("Sorin of House Markov // Sorin, Ravenous Neonate", sorinNeonateActivate)
	r.OnActivated("Sorin, Ravenous Neonate", sorinNeonateActivate)
}

func sorinPostcombatTransform(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sorin_postcombat_transform"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if perm.Transformed {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	gained := seat.Turn.LifeGained
	if gained < 3 {
		return
	}
	if !gameengine.TransformPermanent(gs, perm, "sorin_three_life_gain") {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"transform_failed_face_data_missing")
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"gained": gained,
		"to":     "Sorin, Ravenous Neonate",
	})
}

func sorinNeonateActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	switch abilityIdx {
	case 0:
		sorinNeonatePlusTwo(gs, src)
	case 1:
		sorinNeonateMinusOne(gs, src)
	case 2:
		sorinNeonateMinusSix(gs, src)
	}
}

func sorinNeonatePlusTwo(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "sorin_neonate_plus_two"
	gameengine.CreateFoodToken(gs, src.Controller)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

func sorinNeonateMinusOne(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "sorin_neonate_minus_one"
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	gained := seat.Turn.LifeGained
	if gained <= 0 {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":   src.Controller,
			"damage": 0,
		})
		return
	}
	tgt := -1
	bestLife := 1 << 30
	for _, oppIdx := range gs.Opponents(src.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			tgt = oppIdx
		}
	}
	if tgt < 0 {
		return
	}
	gameengine.DealDamage(gs, tgt, gained, src.Card.DisplayName())
	_ = gs.CheckEnd()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": tgt,
		"damage": gained,
	})
}

func sorinNeonateMinusSix(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "sorin_neonate_minus_six"
	var target *gameengine.Permanent
	bestPow := -1
	for _, oppIdx := range gs.Opponents(src.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if p.Power() > bestPow {
				bestPow = p.Power()
				target = p
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_opponent_creature", nil)
		return
	}
	priorOwner := target.Controller
	prevSeat := gs.Seats[priorOwner]
	for i, p := range prevSeat.Battlefield {
		if p == target {
			prevSeat.Battlefield = append(prevSeat.Battlefield[:i], prevSeat.Battlefield[i+1:]...)
			break
		}
	}
	target.Controller = src.Controller
	gs.Seats[src.Controller].Battlefield = append(gs.Seats[src.Controller].Battlefield, target)
	if !cardHasType(target.Card, "vampire") {
		target.Card.Types = append(target.Card.Types, "vampire")
	}
	// Lifelink counter if controller has another white permanent.
	hasWhite := false
	for _, p := range gs.Seats[src.Controller].Battlefield {
		if p == nil || p == src || p == target || p.Card == nil {
			continue
		}
		for _, c := range p.Card.Colors {
			if c == "W" {
				hasWhite = true
				break
			}
		}
		if hasWhite {
			break
		}
	}
	if hasWhite {
		target.AddCounter("lifelink", 1)
		// r92: a keyword counter is a layer-6 grant (CR 122.1b), and
		// AddCounter is the raw primitive — it takes no gs and cannot
		// invalidate. Without this the layer-aware authority keeps
		// answering from a cached entry taken before the counter went
		// on; the canonical placer (counter_place.go) invalidates, and
		// every raw keyword-counter site in the tree had skipped it.
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            src.Controller,
		"stolen":          target.Card.DisplayName(),
		"prev_owner":      priorOwner,
		"lifelink_added":  hasWhite,
	})
}
