package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTeval wires Teval, the Balanced Scale.
//
// Oracle text:
//
//	At the beginning of combat on your turn, if an opponent controls the
//	most creatures, create a 3/3 green Beast creature token. If you
//	control the most creatures, creatures you control get +1/+1 until
//	end of turn.
//
// Implementation:
//   - "combat_begin": gate on active_seat == controller. Count creatures
//     per seat.
//   - Token branch: fires when the highest opponent creature count
//     strictly exceeds the controller's count.
//   - Anthem branch: fires when the controller's count strictly exceeds
//     every opponent's. Ties produce no effect (matches the strict
//     "the most" reading).
//   - De-dupe per turn so any extra-combat phase doesn't double-fire.
func registerTeval(r *Registry) {
	r.OnTrigger("Teval, the Balanced Scale", "combat_begin", tevalCombatBegin)
}

func tevalCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "teval_combat_begin"
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
	dedupe := "teval_combat_t" + strconv.Itoa(gs.Turn)
	if perm.Flags[dedupe] > 0 {
		return
	}
	perm.Flags[dedupe] = 1

	youCount := tevalCreatureCount(gs, perm.Controller)
	oppMax := -1
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		c := tevalCreatureCount(gs, oppIdx)
		if c > oppMax {
			oppMax = c
		}
	}

	switch {
	case oppMax > youCount:
		gameengine.CreateCreatureToken(gs, perm.Controller, "Beast Token",
			[]string{"creature", "beast", "pip:G"}, 3, 3)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"branch":    "opponent_most",
			"you_count": youCount,
			"opp_max":   oppMax,
			"token":     "3/3 Beast",
		})
	case youCount > oppMax:
		ts := gs.NextTimestamp()
		buffed := 0
		seat := gs.Seats[perm.Controller]
		if seat != nil {
			for _, p := range seat.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				p.Modifications = append(p.Modifications, gameengine.Modification{
					Power:     1,
					Toughness: 1,
					Duration:  "until_end_of_turn",
					Timestamp: ts,
				})
				buffed++
			}
		}
		if buffed > 0 {
			gs.InvalidateCharacteristicsCache()
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"branch":    "you_most",
			"you_count": youCount,
			"opp_max":   oppMax,
			"buffed":    buffed,
		})
	default:
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"branch":    "tie_no_effect",
			"you_count": youCount,
			"opp_max":   oppMax,
		})
	}
}

func tevalCreatureCount(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			n++
		}
	}
	return n
}
