package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOldOneEye wires Old One Eye.
//
// Oracle text (relevant clauses):
//
//	Trample
//	Other creatures you control have trample.
//	When Old One Eye enters, create a 5/5 green Tyranid creature token.
//	Fast Healing — At the beginning of your first main phase, you may
//	               discard two cards. If you do, return this card from
//	               your graveyard to your hand.
//
// R37 port:
//
//   - ETB token creation is fully wired here using CreateCreatureToken.
//     The prior stub created a 1/1 with name "1/1 Creature Token" and
//     duplicate "creature" type tags — those bugs are fixed inline as
//     part of the port.
//   - "Other creatures you control have trample" is a layer-6/static
//     grant; the AST keyword pipeline handles trample distribution.
//     emitPartial flags that gap so audits can catch the missing
//     anthem coverage.
//   - "Fast Healing" first-main-phase recursion is its own delayed
//     trigger; deferred (a separate per-card hook would have to
//     register a "first_main_phase" observer on the controller's turn
//     and check for Old One Eye in their graveyard).
func registerOldOneEye(r *Registry) {
	r.OnETB("Old One Eye", oldOneEyeETB)
	r.OwnsETBTrigger("Old One Eye")
}

func oldOneEyeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "old_one_eye_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	tok := gameengine.CreateCreatureToken(gs, seat,
		"Tyranid Token",
		[]string{"creature", "tyranid"},
		5, 5)
	if tok != nil && tok.Card != nil {
		tok.Card.Colors = []string{"G"}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"token_created": "Tyranid Token",
		"power":         5,
		"toughness":     5,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"static anthem (other creatures gain trample) handled by AST engine; Fast Healing first-main-phase recursion not modeled")
}
