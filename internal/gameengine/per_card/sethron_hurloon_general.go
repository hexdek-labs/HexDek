package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSethronHurloonGeneral wires Sethron, Hurloon General's tribal
// token trigger.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Whenever Sethron or another nontoken Minotaur you control enters, create a
//	2/3 red Minotaur creature token.
//	{2}{B/R}: Minotaurs you control get +1/+0 and gain menace and haste...
//
// Why a per-card handler: the trigger parses to an inert `untyped_effect`
// ("another nontoken minotaur you control enters, create a 2/3 red minotaur
// creature token") — generic dispatch makes no token. The Minotaur tribal
// payoff is the whole point of the card.
//
// Implementation (CR §603 triggered ability):
//   - Listen on "permanent_etb". Fire when the entering permanent is a
//     NONTOKEN Minotaur controlled by Sethron's controller (this includes
//     Sethron itself). Token Minotaurs are excluded by the "nontoken" clause,
//     which also prevents the created 2/3 tokens from looping.
//   - Mint a 2/3 red Minotaur token.
//   - The activated anthem ability is NOT covered here (separate path).
//
// Self-registers via init() + AddResetHook (no shared registry edits).
func init() {
	registerSethronHurloonGeneral(Global())
	AddResetHook(registerSethronHurloonGeneral)
}

func registerSethronHurloonGeneral(r *Registry) {
	r.OnTrigger("Sethron, Hurloon General", "permanent_etb", sethronOnMinotaurETB)
}

func sethronOnMinotaurETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sethron_minotaur_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	// Must be controlled by Sethron's controller.
	if entering.Controller != perm.Controller {
		return
	}
	// Must be a nontoken Minotaur.
	if entering.IsToken() {
		return
	}
	if !cardHasType(entering.Card, "minotaur") {
		return
	}

	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Minotaur",
		[]string{"creature", "minotaur"}, 2, 3)
	if tok != nil && tok.Card != nil {
		tok.Card.Colors = []string{"R"}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"trigger":  entering.Card.DisplayName(),
		"token":    "Minotaur",
		"token_pt": "2/3",
	})
}
