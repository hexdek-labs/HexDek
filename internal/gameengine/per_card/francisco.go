package per_card

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFrancisco wires Francisco, Fowl Marauder.
//
// Oracle text:
//
//	Flying
//	Francisco can't block.
//	Whenever one or more Pirates you control deal damage to a player,
//	Francisco explores.
//	Partner
//
// Flying / Partner are parsed by the AST. "Can't block" is parsed as a
// static restriction (gameast Static modification) — combat-side
// enforcement reads the AST keyword via canBlockGS already, but as a
// belt-and-suspenders measure we don't model that here.
//
// The "one or more Pirates ... deal damage to a player" trigger fires
// once per (turn, defending player). combat_damage_player is dispatched
// per (attacker, defender) pair, so we de-dupe via a per-permanent flag
// keyed on (turn, defenderSeat).
func registerFrancisco(r *Registry) {
	r.OnTrigger("Francisco, Fowl Marauder", "combat_damage_player", franciscoTrigger)
}

func franciscoTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "francisco_fowl_marauder_explore"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	defenderSeat, _ := ctx["defender_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	if sourceName == "" {
		return
	}

	// Source must be a Pirate the controller controls. Francisco himself
	// is a Pirate, so self-damage (rare — he can't block, only attack)
	// also counts.
	isPirate := false
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !strings.EqualFold(p.Card.DisplayName(), sourceName) {
			continue
		}
		if cardHasType(p.Card, "pirate") {
			isPirate = true
		}
		break
	}
	if !isPirate {
		return
	}

	// De-dupe: "one or more Pirates" fires once per damage event. The
	// engine emits combat_damage_player per (attacker, defender) pair,
	// so we coalesce per (turn, defenderSeat) on Francisco's flag.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := "francisco_explored_" + strconv.Itoa(gs.Turn) + "_" + strconv.Itoa(defenderSeat)
	if perm.Flags[key] > 0 {
		return
	}
	perm.Flags[key] = 1

	gameengine.PerformExplore(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"source_card":   sourceName,
		"defender_seat": defenderSeat,
	})
}
