package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIvyGleefulSpellthief wires Ivy, Gleeful Spellthief.
//
// Oracle text:
//
//	Flying
//	Whenever a player casts a spell that targets only a single
//	creature other than Ivy, you may copy that spell. The copy
//	targets Ivy. (A copy of an Aura spell becomes a token.)
//
// Implementation (R46 stub port):
//   - Flying: handled by the AST keyword pipeline.
//   - "spell_cast" trigger: inspect the cast spell's target list.
//     Require exactly one creature target that isn't Ivy. We locate
//     the originating StackItem by Card pointer (passed via
//     ctx["card"]), then push a copy onto the stack with IsCopy=true
//     and a single retargeted creature target (Ivy). Pattern mirrors
//     Strionic Resonator / resolve.go's CR §707.2 copy primitive.
//   - The Aura→token conversion rider (CR §707.10g) on a copied Aura
//     spell isn't handled here; if the original is an Aura spell the
//     copy will resolve normally but won't be converted into a token
//     by this handler — partial.
func registerIvyGleefulSpellthief(r *Registry) {
	r.OnTrigger("Ivy, Gleeful Spellthief", "spell_cast", ivySpellCast)
}

func ivySpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ivy_copy_single_creature_target"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// "any player" — no caster_seat filter.
	targets, _ := ctx["targets"].([]interface{})
	if len(targets) == 0 {
		// Try alternate keys some emitters use.
		if tperm, ok := ctx["target_perm"].(*gameengine.Permanent); ok && tperm != nil {
			targets = []interface{}{tperm}
		}
	}
	if len(targets) != 1 {
		return
	}
	tperm, ok := targets[0].(*gameengine.Permanent)
	if !ok || tperm == nil || !tperm.IsCreature() {
		return
	}
	if tperm == perm {
		return // can't be Ivy herself
	}
	// Find the originating spell on the stack. Prefer the most recent
	// item whose Card matches ctx["card"]; fall back to top instant/
	// sorcery if ctx is missing the pointer (older event emitters).
	castCard, _ := ctx["card"].(*gameengine.Card)
	var originItem *gameengine.StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Card == nil {
			continue
		}
		if castCard != nil && si.Card != castCard {
			continue
		}
		originItem = si
		break
	}
	if originItem == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"original_target": tperm.Card.DisplayName(),
			"reason":          "origin_spell_not_on_stack",
		})
		return
	}
	copyCard := originItem.Card.DeepCopy()
	copyCard.IsCopy = true
	copyItem := &gameengine.StackItem{
		Controller: perm.Controller,
		Card:       copyCard,
		Effect:     originItem.Effect,
		Kind:       originItem.Kind,
		IsCopy:     true,
		Targets: []gameengine.Target{
			{Kind: gameengine.TargetKindPermanent, Permanent: perm},
		},
	}
	gameengine.PushStackItem(gs, copyItem)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"original_target": tperm.Card.DisplayName(),
		"new_target":      perm.Card.DisplayName(),
		"copied":          originItem.Card.DisplayName(),
		"rule":            "707.2",
	})
}
