package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBaralAndKariZev wires Baral and Kari Zev.
//
// Oracle text:
//
//	First strike, menace
//	Whenever you cast your first instant or sorcery spell each turn,
//	you may cast a spell with lesser mana value that shares a card type
//	with it from your hand without paying its mana cost. If you don't,
//	create First Mate Ragavan, a legendary 2/1 red Monkey Pirate
//	creature token. It gains haste until end of turn.
//
// Implementation: track first-instant-or-sorcery-each-turn via perm
// flag. Free-cast resolution path is non-trivial — we always make the
// Ragavan token instead (the safer side of the choice).
func registerBaralAndKariZev(r *Registry) {
	r.OnTrigger("Baral and Kari Zev", "instant_or_sorcery_cast", baralKariFirstSpell)
}

func baralKariFirstSpell(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "baral_kari_first_spell"
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
	turnKey := "baral_kari_fired_turn"
	if perm.Flags[turnKey] == gs.Turn+1 {
		return
	}
	perm.Flags[turnKey] = gs.Turn + 1

	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "First Mate Ragavan",
		[]string{"legendary", "creature", "monkey", "pirate", "pip:R"}, 2, 1)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, tok, "haste")
		tok.SummoningSick = false
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "First Mate Ragavan",
	})
}
