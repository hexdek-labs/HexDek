package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCarpetOfFlowers wires Carpet of Flowers.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Carpet%20of%20Flowers):
//
//	At the beginning of each of your main phases, if you haven't added
//	mana with this ability this turn, you may add X mana of any one
//	color, where X is the number of Islands target opponent controls.
//
// {G} Enchantment. Premier sideboard ramp against blue meta — a free
// 2-5 mana per turn for {G} is absurdly efficient. Carpet's typical
// cEDH read is "if there's an Islands deck at the table, Carpet
// generates more mana than it costs every turn forever."
//
// Implementation note on "each main phase" timing: the engine doesn't
// yet fire a "main_phase_begin" event. Carpet's once-per-turn gate
// ("if you haven't added mana with this ability this turn") makes the
// distinction moot — at most one activation per turn — so this
// handler fires on `upkeep_controller` (just before precombat main)
// as an approximation. The once-per-turn flag prevents double-fire
// from any future main-phase events that get plumbed in.
//
// Pick policy:
//   - Scan all opponents and pick the one with the most Islands.
//   - X = count of Islands controlled by that opponent.
//   - Add X mana of one color. Default to green (the Carpet's
//     color) since green sources are scarce in mono-color shells.
//     If controller's commander identity includes other colors,
//     a more sophisticated picker could optimize for color demand,
//     but green is the safe default — extending to color-demand
//     reads needs a Freya/hat color hook which lives elsewhere.
func registerCarpetOfFlowers(r *Registry) {
	r.OnTrigger("Carpet of Flowers", "upkeep", carpetOfFlowersUpkeep)
}

func carpetOfFlowersUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "carpet_of_flowers"
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
	// Once-per-turn gate.
	if perm.Flags["carpet_added_mana_turn"] == gs.Turn {
		return
	}

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Find the opponent with the most Islands.
	bestSeat := -1
	bestIslands := 0
	for i, opp := range gs.Seats {
		if opp == nil || opp.Lost || i == seat {
			continue
		}
		count := 0
		for _, p := range opp.Battlefield {
			if p == nil || !p.IsLand() {
				continue
			}
			if cardHasSubtype(p.Card, "island") {
				count++
			}
		}
		if count > bestIslands {
			bestIslands = count
			bestSeat = i
		}
	}
	if bestIslands == 0 {
		// No opponent controls an Island; skip without consuming the gate.
		emitFail(gs, slug, "Carpet of Flowers", "no_opponent_islands", nil)
		return
	}

	perm.Flags["carpet_added_mana_turn"] = gs.Turn
	gameengine.AddMana(gs, s, "G", bestIslands, "Carpet of Flowers")
	emit(gs, slug, "Carpet of Flowers", map[string]interface{}{
		"seat":       seat,
		"target_opp": bestSeat,
		"islands":    bestIslands,
		"color":      "G",
		"mana_added": bestIslands,
	})
}
