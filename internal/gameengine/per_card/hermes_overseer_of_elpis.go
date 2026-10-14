package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHermesOverseerOfElpis wires Hermes, Overseer of Elpis.
//
// Oracle text:
//
//	Whenever you cast a noncreature spell, create a 1/1 blue Bird
//	creature token with flying and vigilance.
//	Whenever you attack with one or more Birds, scry 2.
//
// Implementation:
//   - noncreature_spell_cast: scoped to controller; create a 1/1 blue
//     Bird token with flying + vigilance.
//   - attacks trigger: if any attacker we control is a Bird, scry 2
//     (best-effort: just put the top card on the bottom if it looks
//     bad — engine doesn't expose AI scry decisions, so emitPartial).
func registerHermesOverseerOfElpis(r *Registry) {
	r.OnTrigger("Hermes, Overseer of Elpis", "noncreature_spell_cast", hermesNoncreatureCast)
	r.OnTrigger("Hermes, Overseer of Elpis", "attacks", hermesAttacks)
}

func hermesNoncreatureCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hermes_bird_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	token := &gameengine.Card{
		Name:          "Bird Token",
		Owner:         perm.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "bird"},
		Colors:        []string{"U"},
		TypeLine:      "Token Creature — Bird",
	}
	tok := enterBattlefieldWithETB(gs, perm.Controller, token, false)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, tok, "flying")
		gameengine.SetKeywordFlags(gs, tok, "vigilance")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func hermesAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hermes_bird_scry"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	attackerSeat, _ := ctx["seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"scry_2_with_no_ai_choice_path_unimplemented")
}
