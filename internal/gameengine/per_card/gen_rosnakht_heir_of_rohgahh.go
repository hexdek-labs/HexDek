package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRosnakhtHeirOfRohgahh wires Rosnakht, Heir of Rohgahh.
//
// Oracle text:
//
//   Battle cry (Whenever this creature attacks, each other attacking creature gets +1/+0 until end of turn.)
//   Heroic — Whenever you cast a spell that targets Rosnakht, create a 0/1 red Kobold creature token named Kobolds of Kher Keep.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerRosnakhtHeirOfRohgahh(r *Registry) {
	r.OnETB("Rosnakht, Heir of Rohgahh", rosnakhtHeirOfRohgahhStaticETB)
}

func rosnakhtHeirOfRohgahhStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rosnakht_heir_of_rohgahh_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
