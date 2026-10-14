package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJensonCarthalionDruidExile wires Jenson Carthalion, Druid
// Exile.
//
// Oracle text:
//
//	Whenever you cast a multicolored spell, scry 1. If that spell was
//	all colors, create a 4/4 white Angel creature token with flying
//	and vigilance.
//	{5}, {T}: Add {W}{U}{B}{R}{G}.
//
// Implementation:
//   - spell_cast scoped to controller. If the spell has 5 colors,
//     create a 4/4 W Angel with flying+vigilance.
//   - The scry-1 leg is fired as a partial (no AI scry surface).
//   - Five-color mana ability is engine ramp territory — emitPartial
//     covers it via the ETB stub.
func registerJensonCarthalionDruidExile(r *Registry) {
	r.OnTrigger("Jenson Carthalion, Druid Exile", "spell_cast", jensonCarthalionSpellCast)
}

func jensonCarthalionSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jenson_carthalion_multicolor"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	colors := map[string]bool{}
	for _, c := range card.Colors {
		switch c {
		case "W", "U", "B", "R", "G":
			colors[c] = true
		}
	}
	if len(colors) < 2 {
		return
	}
	emit(gs, slug+"_scry", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	if len(colors) < 5 {
		return
	}
	token := &gameengine.Card{
		Name:          "Angel Token",
		Owner:         perm.Controller,
		BasePower:     4,
		BaseToughness: 4,
		Types:         []string{"token", "creature", "angel"},
		Colors:        []string{"W"},
		TypeLine:      "Token Creature — Angel",
	}
	tok := enterBattlefieldWithETB(gs, perm.Controller, token, false)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, tok, "flying")
		gameengine.SetKeywordFlags(gs, tok, "vigilance")
	}
	emit(gs, slug+"_angel", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
