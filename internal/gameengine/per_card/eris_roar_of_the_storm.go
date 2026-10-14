package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerErisRoarOfTheStorm wires Eris, Roar of the Storm.
//
// Oracle text:
//
//	This spell costs {2} less to cast for each different mana value
//	among instant and sorcery cards in your graveyard.
//	Flying, prowess
//	Whenever you cast your second spell each turn, create a 4/4 red
//	Dragon Elemental creature token with flying and prowess.
//
// Implementation: track per-turn cast count via perm flag; on the
// second instant/sorcery cast each turn, create a 4/4 dragon.
func registerErisRoarOfTheStorm(r *Registry) {
	r.OnTrigger("Eris, Roar of the Storm", "spell_cast", erisSpellCast)
}

func erisSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "eris_second_spell_dragon"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnCountKey := "eris_count_turn"
	turnNoKey := "eris_turn_no"
	if perm.Flags[turnNoKey] != gs.Turn+1 {
		perm.Flags[turnNoKey] = gs.Turn + 1
		perm.Flags[turnCountKey] = 0
	}
	perm.Flags[turnCountKey]++
	if perm.Flags[turnCountKey] != 2 {
		return
	}
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Dragon Elemental Token",
		[]string{"creature", "dragon", "elemental", "pip:R"}, 4, 4)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, tok, "flying")
		gameengine.SetKeywordFlags(gs, tok, "prowess")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "Dragon Elemental",
	})
}
