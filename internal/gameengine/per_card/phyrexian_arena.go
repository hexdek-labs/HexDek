package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPhyrexianArena wires Phyrexian Arena.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Phyrexian%20Arena):
//
//	At the beginning of your upkeep, you draw a card and you lose 1 life.
//
// {1}{B}{B} Enchantment. Black's canonical card-advantage engine — net
// +1 card per turn at a 1-life-per-turn tempo cost. B-pile staple
// alongside Necropotence / Bolas's Citadel / Sign in Blood. Pairs with
// any lifegain shell (Beseech the Mirror redirects, Vito etc.) to
// neutralize the drawback.
//
// Implementation:
//   - OnTrigger("upkeep_controller"). Filter: ctx["active_seat"] ==
//     perm.Controller (matches Black Market Connections / Thassa God of
//     the Sea pattern at batch17_sweep.go).
//   - Draw 1 + lose 1 life. Defensive guards: skip if seat is Lost or
//     library is empty (drawOne already handles empty-library by
//     setting AttemptedEmptyDraw; we still attempt and let the
//     standard helper log).
func registerPhyrexianArena(r *Registry) {
	r.OnTrigger("Phyrexian Arena", "upkeep_controller", phyrexianArenaUpkeep)
}

func phyrexianArenaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "phyrexian_arena"
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

	drawOne(gs, perm.Controller, "Phyrexian Arena")
	gameengine.LoseLife(gs, perm.Controller, 1, "Phyrexian Arena")

	emit(gs, slug, "Phyrexian Arena", map[string]interface{}{
		"seat":       perm.Controller,
		"life_after": seat.Life,
	})
}
