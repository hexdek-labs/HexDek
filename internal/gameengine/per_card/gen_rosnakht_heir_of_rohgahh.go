package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRosnakhtHeirOfRohgahh wires Rosnakht, Heir of Rohgahh.
//
// Oracle text (Scryfall, verified, {R}, 1/2 legendary creature — Kobold):
//
//	Battle cry (Whenever this creature attacks, each other attacking
//	creature gets +1/+0 until end of turn.)
//	Heroic — Whenever you cast a spell that targets Rosnakht, Heir of
//	Rohgahh, create a 0/1 red Kobold creature token named Kobolds of
//	Kher Keep.
//
// Implementation (R42 stub port):
//   - Battle cry: AST keyword pipeline (it pumps other attackers
//     during the declare-attackers step).
//   - Heroic: CR §702.123a fires a "heroic" trigger from
//     keywords_heroic.go when a spell cast by Rosnakht's controller
//     locks targets on Rosnakht. We gate on ctx["source"] == perm
//     (the dispatcher already enforces same-controller and the
//     heroic keyword on the source's card, so this handler trusts
//     the surface and just mints the Kobold).
//   - Kobold token: 0/1 red creature named "Kobolds of Kher Keep".
//     The token is technically also a Kobold creature type, which
//     matters for Rohgahh/Crookshank tribal payoffs.
func registerRosnakhtHeirOfRohgahh(r *Registry) {
	r.OnTrigger("Rosnakht, Heir of Rohgahh", "heroic", rosnakhtHeroicSpawn)
}

func rosnakhtHeroicSpawn(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rosnakht_heroic_kobold_token"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	src, _ := ctx["source"].(*gameengine.Permanent)
	if src != perm {
		return
	}
	tok := gameengine.CreateCreatureToken(
		gs,
		perm.Controller,
		"Kobolds of Kher Keep",
		[]string{"creature", "kobold"},
		0, 1,
	)
	if tok == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_creation_failed", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	if tok.Card != nil {
		tok.Card.Colors = []string{"R"}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "Kobolds of Kher Keep",
		"color": "R",
	})
}
