package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMrHousePresidentAndCEO wires Mr. House, President and CEO.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	{R}{W}{B}
//	Legendary Artifact Creature — Human (4/4 — printed)
//
//	Whenever you roll a 4 or higher, create a 3/3 colorless Robot
//	artifact creature token. If you rolled 6 or higher, instead create
//	that token and a Treasure token.
//
//	{4}, {T}: Roll a six-sided die plus an additional six-sided die
//	for each mana from Treasures spent to activate this ability.
//
// Implementation:
//   - Activated ability (idx 0): roll one d6 base, then approximate the
//     "extra die per Treasure mana" rider by counting untapped Treasure
//     tokens controlled at activation time and rolling one extra die per
//     such Treasure (capped at 6 to avoid runaway in fuzz). For each
//     resulting roll we evaluate the static rolled-trigger inline:
//       * roll >= 6  → 3/3 colorless Robot token + 1 Treasure
//       * roll 4-5   → 3/3 colorless Robot token
//       * roll 1-3   → nothing
//     The Treasure-mana cost rider isn't perfectly modeled — the engine
//     pays the {4} cost before the handler runs, so we can't observe
//     whether Treasures specifically funded it. This heuristic captures
//     the common case (Mr. House decks always activate with Treasures
//     ready to sac).
//
//   - "Whenever you roll a 4 or higher" trigger: registered as an
//     OnTrigger("die_rolled") for forward compatibility. The engine's
//     existing rolls log a "roll_die" event but don't dispatch a
//     FireCardTrigger, so foreign rolls won't fire this — emitPartial
//     to flag the gap.
//
// Robot token spec: 3/3 colorless artifact creature. We construct it
// via gameengine.CreateCreatureToken with explicit type tags.
func registerMrHousePresidentAndCEO(r *Registry) {
	r.OnActivated("Mr. House, President and CEO", mrHouseActivate)
	r.OnTrigger("Mr. House, President and CEO", "die_rolled", mrHouseDieRolledTrigger)
	r.OnETB("Mr. House, President and CEO", mrHouseETB)
}

func mrHouseETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, "mr_house_die_rolled_trigger", perm.Card.DisplayName(),
		"foreign_die_rolls_do_not_dispatch_die_rolled_event")
}

func mrHouseActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mr_house_activated_roll_dice"
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

	// Heuristic for the "additional die per Treasure mana spent" rider:
	// count Treasures controlled and assume the controller would sac them
	// for the extra dice (each one is +EV: 1/6 robot+treasure, 1/3 robot).
	// Cap to keep fuzz runs bounded.
	extra := countMrHouseTreasures(gs, seatIdx)
	if extra > 6 {
		extra = 6
	}
	dieCount := 1 + extra

	rolls := make([]int, 0, dieCount)
	robots := 0
	treasures := 0
	for i := 0; i < dieCount; i++ {
		v := mrHouseRollD6(gs)
		gs.LogEvent(gameengine.Event{
			Kind:   "roll_die",
			Seat:   seatIdx,
			Source: src.Card.DisplayName(),
			Amount: v,
			Details: map[string]interface{}{
				"sides":  6,
				"reason": "mr_house_activated",
			},
		})
		rolls = append(rolls, v)
		if v >= 6 {
			mrHouseCreateRobot(gs, seatIdx)
			gameengine.CreateTreasureToken(gs, seatIdx)
			robots++
			treasures++
		} else if v >= 4 {
			mrHouseCreateRobot(gs, seatIdx)
			robots++
		}
	}

	if extra > 0 {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"extra_die_per_treasure_mana_approximated_via_controlled_treasure_count")
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seatIdx,
		"dice_count":    dieCount,
		"rolls":         rolls,
		"robots_made":   robots,
		"treasures_made": treasures,
	})
}

// mrHouseDieRolledTrigger handles the static "Whenever you roll a 4 or
// higher" trigger. Today the engine doesn't fire die_rolled events, so
// this is a forward-compatible scaffold; the activated ability path
// inlines the same logic instead.
func mrHouseDieRolledTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mr_house_die_rolled_payoff"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	rollerSeat, _ := ctx["roller_seat"].(int)
	if rollerSeat != perm.Controller {
		return
	}
	result, _ := ctx["result"].(int)
	if result < 4 {
		return
	}
	mrHouseCreateRobot(gs, perm.Controller)
	if result >= 6 {
		gameengine.CreateTreasureToken(gs, perm.Controller)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"result": result,
	})
}

// countMrHouseTreasures returns the number of untapped Treasure tokens
// the seat controls. Used as a proxy for "Treasure mana available to
// fund the {4}, {T} cost."
func countMrHouseTreasures(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return 0
	}
	n := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || p.Tapped {
			continue
		}
		tl := strings.ToLower(p.Card.TypeLine)
		if strings.Contains(tl, "treasure") || p.Card.DisplayName() == "Treasure Token" {
			n++
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "treasure") {
				n++
				break
			}
		}
	}
	return n
}

// mrHouseRollD6 returns 1-6 using the GameState RNG when available,
// falling back to 1 when the RNG hasn't been seeded (deterministic for
// tests).
func mrHouseRollD6(gs *gameengine.GameState) int {
	if gs != nil && gs.Rng != nil {
		return gs.Rng.Intn(6) + 1
	}
	return 1
}

// mrHouseCreateRobot creates a 3/3 colorless artifact creature Robot
// token. Mirrors the printed token spec from Fallout commander set.
func mrHouseCreateRobot(gs *gameengine.GameState, seatIdx int) {
	gameengine.CreateCreatureToken(
		gs,
		seatIdx,
		"Robot Token",
		[]string{"artifact", "creature", "robot"},
		3, 3,
	)
}
