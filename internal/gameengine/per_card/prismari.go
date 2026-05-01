package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPrismari wires Prismari, the Inspiration.
//
// Oracle text:
//
//	Flying
//	Ward—Pay 5 life.
//	Instant and sorcery spells you cast have storm. (Whenever you cast
//	an instant or sorcery spell, copy it for each spell cast before it
//	this turn. You may choose new targets for the copies.)
//
// Engine support:
//   - Flying and Ward are AST keywords; ward triggers fire from the
//     stock targeting pipeline. No handler work needed.
//   - The storm grant is implemented via the "instant_or_sorcery_cast"
//     trigger. When the trigger handler runs, the spell that triggered
//     it sits at the top of the stack (PushPerCardTrigger pushes the
//     trigger above the spell, ResolveStackTop pops the trigger before
//     invoking the handler). We feed that StackItem into
//     gameengine.ApplyStormCopies, which appends (SpellsCastThisTurn-1)
//     copies above the spell — exactly the same path the engine uses
//     for printed-storm cards (see storm.go and zone_cast.go).
func registerPrismari(r *Registry) {
	r.OnTrigger("Prismari, the Inspiration", "instant_or_sorcery_cast", prismariStormGrant)
}

func prismariStormGrant(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "prismari_storm_grant"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	spellName, _ := ctx["spell_name"].(string)
	if len(gs.Stack) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "stack_empty", map[string]interface{}{
			"seat":  perm.Controller,
			"spell": spellName,
		})
		return
	}
	top := gs.Stack[len(gs.Stack)-1]
	if top == nil || top.Card == nil {
		return
	}
	if spellName != "" && top.Card.DisplayName() != spellName {
		emitFail(gs, slug, perm.Card.DisplayName(), "stack_top_mismatch", map[string]interface{}{
			"seat":      perm.Controller,
			"want":      spellName,
			"stack_top": top.Card.DisplayName(),
		})
		return
	}
	copies := gameengine.ApplyStormCopies(gs, top, casterSeat)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   casterSeat,
		"spell":  top.Card.DisplayName(),
		"copies": copies,
	})
}
