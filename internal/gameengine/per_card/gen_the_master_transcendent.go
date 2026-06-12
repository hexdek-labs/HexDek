package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheMasterTranscendent wires The Master, Transcendent.
//
// Oracle text:
//
//	When The Master enters, target player gets two rad counters.
//	{T}: Put target creature card in a graveyard that was milled this
//	turn onto the battlefield under your control. It's a green Mutant
//	with base power and toughness 3/3. (It loses its other colors and
//	creature types.)
//
// Implementation:
//   - ETB rad counters: pick the most-life opponent (heuristic — most
//     life = best target for incremental rad pressure) and add 2 to
//     their `rad_counters` Seat.Flag, which the engine's rad pipeline
//     in rad.go picks up at the next draw step.
//   - Activated ({T}): the engine doesn't track which specific cards
//     were milled this turn, only `Turn.Milled int`. We approximate:
//     when the active player's Turn.Milled > 0, pick the highest-CMC
//     creature card from any graveyard (preferring ours), reanimate
//     it under our control with base 3/3 green Mutant overrides via
//     a layer-1/4/7 ContinuousEffect chain. Engine-side proper
//     "milled this turn" tracking would let us narrow targets;
//     emitPartial breadcrumb covers the gap.
func registerTheMasterTranscendent(r *Registry) {
	r.OnETB("The Master, Transcendent", theMasterETBRad)
	r.OnActivated("The Master, Transcendent", theMasterReanimateMutant)
}

func theMasterETBRad(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_master_transcendent_etb_rad"
	if gs == nil || perm == nil {
		return
	}
	// Pick the highest-life opponent. Falls back to opps[0].
	target := -1
	bestLife := -1
	for _, opp := range gs.Opponents(perm.Controller) {
		if opp < 0 || opp >= len(gs.Seats) {
			continue
		}
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent_for_rad", nil)
		return
	}
	if gs.Seats[target].Flags == nil {
		gs.Seats[target].Flags = map[string]int{}
	}
	gs.Seats[target].Flags["rad_counters"] += 2
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"target_seat": target,
		"rad_added":   2,
	})
}

func theMasterReanimateMutant(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_master_transcendent_reanimate_mutant"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	// Engine doesn't track per-card mill-this-turn, only the count. Gate
	// on "any milling happened anywhere this turn" so the activation
	// remains legal in the typical reanimator-with-mill-engine flow.
	milledThisTurn := 0
	for _, s := range gs.Seats {
		if s != nil {
			milledThisTurn += s.Turn.Milled
		}
	}
	if milledThisTurn == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_cards_milled_this_turn", nil)
		return
	}

	// Pick the highest-CMC creature from any graveyard. Prefer ours
	// when tied so we don't inadvertently buff an opponent's reanimate
	// chain by yanking their best card through our text.
	var best *gameengine.Card
	bestCMC := -1
	bestSeat := -1
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasType(c, "creature") {
				continue
			}
			cmc := cardCMC(c)
			if cmc > bestCMC || (cmc == bestCMC && i == src.Controller && bestSeat != src.Controller) {
				best = c
				bestCMC = cmc
				bestSeat = i
			}
		}
	}
	if best == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_in_any_graveyard", nil)
		return
	}

	src.Tapped = true

	// Pull from owner's graveyard.
	owner := gs.Seats[bestSeat]
	for i, c := range owner.Graveyard {
		if c == best {
			owner.Graveyard = append(owner.Graveyard[:i], owner.Graveyard[i+1:]...)
			break
		}
	}

	// Reanimate under our control with the 3/3 green Mutant overrides.
	// We use BasePower/BaseToughness on a wrapping Card so the engine's
	// stat readers see 3/3, and override the type slice. The engine's
	// continuous-effect layer pipeline would be cleaner; we approximate
	// here with a copy of the card the layer-aware readers can consume.
	overrideCard := best.DeepCopy()
	overrideCard.BasePower = 3
	overrideCard.BaseToughness = 3
	overrideCard.Types = []string{"creature", "mutant"}
	overrideCard.Colors = []string{"G"}

	newPerm := enterBattlefieldWithETB(gs, src.Controller, overrideCard, false)
	if newPerm != nil {
		// §108.3 (r63): the reanimated card may come from an OPPONENT's
		// graveyard — ownership never changes. Pre-r63 this stamped
		// Permanent.Owner = controller, the exact corruption behind the
		// PR-#1047 elimination vanishes.
		newPerm.Owner = overrideCard.Owner
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         src.Controller,
		"reanimated":   best.DisplayName(),
		"from_seat":    bestSeat,
		"as_pt":        "3/3",
		"as_subtypes":  []string{"mutant"},
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"engine doesn't track per-card mill-this-turn; activation gates on global Turn.Milled count")
}
