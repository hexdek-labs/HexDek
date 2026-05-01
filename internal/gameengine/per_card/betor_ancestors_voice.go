package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBetorAncestorsVoice wires Betor, Ancestor's Voice.
//
// Oracle text (TDC, {2}{W}{B}{G}, 3/5 Legendary Spirit Dragon):
//
//	Flying, lifelink
//	At the beginning of your end step, put a number of +1/+1 counters
//	on up to one other target creature you control equal to the amount
//	of life you gained this turn. Return up to one target creature card
//	with mana value less than or equal to the amount of life you lost
//	this turn from your graveyard to the battlefield.
//
// Implementation:
//   - Flying/lifelink: AST keyword pipeline.
//   - life_gained / life_lost listeners maintain per-Betor turn-scoped
//     counters on perm.Flags. Betor's controller's gains/losses are the
//     only ones that count. Multiple Betors are safe because each tallies
//     into its own perm.Flags.
//   - end_step (gated to active_seat == controller): apply +1/+1 counters
//     to the best other creature we control (highest power, earliest
//     timestamp) equal to gained, then reanimate the highest-CMC creature
//     in graveyard whose CMC <= lost. Both halves are independent and
//     each silently does nothing when its budget is zero or no target
//     exists.
func registerBetorAncestorsVoice(r *Registry) {
	r.OnTrigger("Betor, Ancestor's Voice", "life_gained", betorTrackLifeGained)
	r.OnTrigger("Betor, Ancestor's Voice", "life_lost", betorTrackLifeLost)
	r.OnTrigger("Betor, Ancestor's Voice", "end_step", betorEndStep)
}

func betorGainKey(turn int) string { return "betor_gained_t" + strconv.Itoa(turn+1) }
func betorLossKey(turn int) string { return "betor_lost_t" + strconv.Itoa(turn+1) }

func betorTrackLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
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
	perm.Flags[betorGainKey(gs.Turn)] += amount
}

func betorTrackLifeLost(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
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
	perm.Flags[betorLossKey(gs.Turn)] += amount
}

func betorEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "betor_ancestors_voice_end_step"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	gainKey := betorGainKey(gs.Turn)
	lossKey := betorLossKey(gs.Turn)
	gained := perm.Flags[gainKey]
	lost := perm.Flags[lossKey]

	// Reset turn-scoped counters and prune any older entries.
	delete(perm.Flags, gainKey)
	delete(perm.Flags, lossKey)
	clearStaleBetorKeys(perm, gs.Turn)

	// Half 1: +1/+1 counters on best other creature.
	counterTarget := pickBetorCounterTarget(seat, perm)
	counterTargetName := ""
	if gained > 0 && counterTarget != nil {
		counterTarget.AddCounter("+1/+1", gained)
		counterTargetName = counterTarget.Card.DisplayName()
		gs.InvalidateCharacteristicsCache()
	}

	// Half 2: reanimate highest-CMC creature card with CMC <= lost.
	reanimated := ""
	reanimatedCMC := -1
	if lost > 0 {
		var bestCard *gameengine.Card
		bestCMC := -1
		for _, c := range seat.Graveyard {
			if c == nil || !cardHasType(c, "creature") {
				continue
			}
			cmc := gameengine.ManaCostOf(c)
			if cmc <= lost && cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
			}
		}
		if bestCard != nil {
			gameengine.MoveCard(gs, bestCard, perm.Controller, "graveyard", "battlefield", "betor_reanimate")
			ent := enterBattlefieldWithETB(gs, perm.Controller, bestCard, false)
			if ent != nil {
				reanimated = bestCard.DisplayName()
				reanimatedCMC = bestCMC
			}
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"life_gained":     gained,
		"life_lost":       lost,
		"counter_target":  counterTargetName,
		"counters_added":  gained,
		"reanimated":      reanimated,
		"reanimated_cmc":  reanimatedCMC,
	})
}

func pickBetorCounterTarget(seat *gameengine.Seat, perm *gameengine.Permanent) *gameengine.Permanent {
	if seat == nil || perm == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		pow := p.Power()
		if pow > bestPow {
			bestPow = pow
			best = p
			continue
		}
		if pow == bestPow && best != nil && p.Timestamp < best.Timestamp {
			best = p
		}
	}
	return best
}

func clearStaleBetorKeys(perm *gameengine.Permanent, currentTurn int) {
	if perm == nil || perm.Flags == nil {
		return
	}
	for _, prefix := range []string{"betor_gained_t", "betor_lost_t"} {
		for k := range perm.Flags {
			if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
				continue
			}
			n, err := strconv.Atoi(k[len(prefix):])
			if err != nil {
				continue
			}
			if n <= currentTurn {
				delete(perm.Flags, k)
			}
		}
	}
}
