package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSharkTyphoon wires Shark Typhoon (the on-battlefield enchantment
// mode).
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Whenever you cast a noncreature spell, create an X/X blue Shark creature
//	token with flying, where X is that spell's mana value.
//	Cycling {X}{1}{U}. When you cycle this card, create an X/X blue Shark...
//
// Why a per-card handler: the trigger parses to an inert
// `parsed_effect_residual` ("create an x/x blue shark creature token with
// flying, where x is that spell's mana value") — the generic dispatch makes
// no token. Shark Typhoon is a premier blue staple; leaving it inert removes
// the whole payoff. (The cycling mode is a separate activated/cycling path
// and is not covered here; this handler implements the resolved-enchantment
// static trigger.)
//
// Implementation (CR §603 triggered ability):
//   - Listen on "noncreature_spell_cast"; gate caster == controller.
//   - X = the cast spell's mana value (ctx card ManaValue, fallback CMC).
//   - Mint an X/X blue Shark with flying. A 0-mana-value spell makes a 0/0
//     that dies to SBA — correct per rules.
//
// Self-registers via init() + AddResetHook (no shared registry edits).
func init() {
	registerSharkTyphoon(Global())
	AddResetHook(registerSharkTyphoon)
}

func registerSharkTyphoon(r *Registry) {
	r.OnTrigger("Shark Typhoon", "noncreature_spell_cast", sharkTyphoonOnNoncreatureCast)
}

func sharkTyphoonOnNoncreatureCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "shark_typhoon_shark_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, ok := ctx["caster_seat"].(int)
	if !ok || casterSeat != perm.Controller {
		return
	}

	x := 0
	if c, ok := ctx["card"].(*gameengine.Card); ok && c != nil {
		x = c.CMC
	}

	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Shark",
		[]string{"creature", "shark"}, x, x)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["kw:flying"] = 1
		if tok.Card != nil {
			tok.Card.Colors = []string{"U"}
		}
	}

	spellName := ""
	if c, ok := ctx["card"].(*gameengine.Card); ok && c != nil {
		spellName = c.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"x":          x,
		"spell_name": spellName,
		"token":      "Shark",
		"flying":     true,
	})
}
