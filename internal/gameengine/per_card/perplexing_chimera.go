package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPerplexingChimera wires up Perplexing Chimera.
//
// Oracle text:
//
//	Whenever an opponent casts a spell, you may exchange control of
//	Perplexing Chimera and that spell. If you do, you may choose new
//	targets for the spell.
//
// This is a layer-2 control-change effect on a spell ON THE STACK,
// which is unusual -- most control changes target permanents. The
// implementation:
//   - Trigger on "spell_cast_by_opponent" game event
//   - Swap the stack item's Controller to the Chimera's controller
//   - Swap Chimera's Controller to the opponent who cast the spell
//   - The spell now resolves under the new controller's control
//
// This is a "may" ability -- in simulation, we always accept (aggressive
// policy: stealing spells is almost always correct in cEDH). A future
// Hat-driven policy can refine this.
func registerPerplexingChimera(r *Registry) {
	r.OnTrigger("Perplexing Chimera", "spell_cast_by_opponent", perplexingChimeraTrigger)
}

func perplexingChimeraTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "perplexing_chimera_exchange"
	if gs == nil || perm == nil {
		return
	}
	// Only fire if there's a spell on the stack to steal.
	if len(gs.Stack) == 0 {
		return
	}

	// Get the caster's seat from context.
	casterSeat := -1
	if v, ok := ctx["caster_seat"].(int); ok {
		casterSeat = v
	}
	if casterSeat < 0 || casterSeat == perm.Controller {
		return // Only opponents
	}

	// Find the top stack item (the spell that was just cast).
	topItem := gs.Stack[len(gs.Stack)-1]
	if topItem == nil {
		return
	}

	// Exchange control: Chimera goes to opponent, spell goes to us.
	originalChimeraController := perm.Controller
	originalSpellController := topItem.Controller

	// Swap spell controller.
	topItem.Controller = originalChimeraController

	// Swap Chimera controller -- move between battlefields.
	if originalChimeraController >= 0 && originalChimeraController < len(gs.Seats) &&
		originalSpellController >= 0 && originalSpellController < len(gs.Seats) {
		// Remove from current controller's battlefield.
		srcSeat := gs.Seats[originalChimeraController]
		for i, p := range srcSeat.Battlefield {
			if p == perm {
				srcSeat.Battlefield = append(srcSeat.Battlefield[:i], srcSeat.Battlefield[i+1:]...)
				break
			}
		}
		// Add to new controller's battlefield.
		perm.Controller = originalSpellController
		gs.Seats[originalSpellController].Battlefield = append(
			gs.Seats[originalSpellController].Battlefield, perm)
	}

	spellName := ""
	if topItem.Card != nil {
		spellName = topItem.Card.DisplayName()
	}

	emit(gs, slug, "Perplexing Chimera", map[string]interface{}{
		"chimera_new_controller": originalSpellController,
		"spell_new_controller":  originalChimeraController,
		"spell_name":            spellName,
		"rule":                  "exchange_control_stack_object",
	})
}
