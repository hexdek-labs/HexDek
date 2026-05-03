package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOvikaEnigmaGoliath wires Ovika, Enigma Goliath.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Flying
//	Ward—{3}, Pay 3 life.
//	Whenever you cast a noncreature spell, create X 1/1 red Phyrexian
//	Goblin creature tokens, where X is that spell's mana value.
//
// Implementation:
//   - Flying and ward are handled by the AST keyword pipeline.
//   - OnTrigger("noncreature_spell_cast"): fires when any noncreature spell
//     is cast (creature type already filtered by the engine). Gate on
//     caster_seat == perm.Controller (only Ovika's controller triggers).
//     Read the spell's CMC via gameengine.ManaCostOf(card). Create CMC
//     1/1 red Phyrexian Goblin creature tokens in a loop.
//
// Coverage gaps:
//   - Ward cost (pay 3 life) is handled generically by the AST keyword
//     pipeline; no per-card hook needed.
//   - If CMC == 0 (free spell, e.g. Ornithopter cast as sorcery, Force of
//     Will via alternate cost) no tokens are created. This matches the oracle
//     since "X is that spell's mana value" — mana value of a {0} spell is 0.
//
// Token spec:
//
//	Name="Phyrexian Goblin", Power=1, Toughness=1,
//	Types=["creature","phyrexian","goblin"], Colors=["R"].
func registerOvikaEnigmaGoliath(r *Registry) {
	r.OnTrigger("Ovika, Enigma Goliath", "noncreature_spell_cast", ovikaEnigmaGoliathSpellCast)
}

func ovikaEnigmaGoliathSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ovika_enigma_goliath_goblin_tokens"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Only trigger when Ovika's controller casts the noncreature spell.
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}

	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}

	// X = mana value of the cast spell.
	x := gameengine.ManaCostOf(card)
	if x <= 0 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"zero_mana_value_spell_creates_no_tokens")
		return
	}

	// Create X 1/1 red Phyrexian Goblin creature tokens.
	for i := 0; i < x; i++ {
		tok := gameengine.CreateCreatureToken(
			gs,
			perm.Controller,
			"Phyrexian Goblin",
			[]string{"creature", "phyrexian", "goblin"},
			1, 1,
		)
		if tok != nil {
			if tok.Card != nil {
				tok.Card.Colors = []string{"R"}
			}
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"spell_name":  card.DisplayName(),
		"spell_cmc":   x,
		"tokens":      x,
		"token_name":  "Phyrexian Goblin",
		"token_pt":    "1/1",
		"token_color": "R",
	})
}
