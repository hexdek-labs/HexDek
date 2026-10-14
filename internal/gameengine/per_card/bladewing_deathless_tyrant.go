package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBladewingDeathlessTyrant wires Bladewing, Deathless Tyrant.
//
// Oracle text:
//
//	Flying, haste
//	Whenever Bladewing deals combat damage to a player or planeswalker,
//	for each creature card in your graveyard, create a 2/2 black Zombie
//	Knight creature token with menace.
func registerBladewingDeathlessTyrant(r *Registry) {
	r.OnTrigger("Bladewing, Deathless Tyrant", "combat_damage_player", bladewingDeathlessCombat)
}

func bladewingDeathlessCombat(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bladewing_deathless_combat_zombies"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceName, _ := ctx["source_card"].(string)
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, c := range seat.Graveyard {
		if c != nil && cardHasType(c, "creature") {
			count++
		}
	}
	for i := 0; i < count; i++ {
		tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie Knight Token",
			[]string{"creature", "zombie", "knight", "pip:B"}, 2, 2)
		if tok != nil {
			if tok.Flags == nil {
				tok.Flags = map[string]int{}
			}
			gameengine.SetKeywordFlags(gs, tok, "menace")
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tokens": count,
	})
}
