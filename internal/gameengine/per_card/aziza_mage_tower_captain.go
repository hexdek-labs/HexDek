package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAzizaMageTowerCaptain wires Aziza, Mage Tower Captain.
//
// Oracle text:
//
//	Whenever you cast an instant or sorcery spell, you may tap three
//	untapped creatures you control. If you do, copy that spell. You
//	may choose new targets for the copy.
//
// R51 batch J port (cheap CMC=2 commander; pure-stub spell-copy port):
//
//   - Trigger on instant_or_sorcery_cast filtered to controller's casts.
//   - Heuristic cost-pay: tap up to 3 untapped friendly creatures
//     (excluding Aziza herself). If at least 3 are available, pay the
//     cost and push a copy of the originating spell onto the stack
//     with IsCopy=true (Strionic Resonator / Ivy precedent — CR §707.2).
//   - When fewer than 3 untapped creatures are available, the optional
//     "may" goes unpaid and we emitFail so the trigger fires but the
//     copy is skipped (matches CR §603 optional trigger semantics).
//   - "Choose new targets for the copy" — the engine doesn't yet
//     dispatch target reselection on copies; the StackItem.Targets
//     inherits from the original Card and the engine resolves it
//     against the same targets. This is the standard partial for
//     spell-copy ports and is shared with Ivy / Riku.
func registerAzizaMageTowerCaptain(r *Registry) {
	r.OnTrigger("Aziza, Mage Tower Captain", "instant_or_sorcery_cast", azizaSpellCopy)
}

func azizaSpellCopy(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aziza_spell_copy"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	castCard, _ := ctx["card"].(*gameengine.Card)
	if castCard == nil {
		return
	}

	// Collect 3 untapped friendly creatures (excluding Aziza).
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var toTap []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !p.IsCreature() || p.Tapped {
			continue
		}
		toTap = append(toTap, p)
		if len(toTap) == 3 {
			break
		}
	}
	if len(toTap) < 3 {
		emitFail(gs, slug, perm.Card.DisplayName(), "insufficient_untapped_creatures", map[string]interface{}{
			"have":     len(toTap),
			"required": 3,
		})
		return
	}

	// Pay the tap cost.
	for _, p := range toTap {
		p.Tapped = true
	}

	// Locate the originating StackItem to copy its targets.
	var originatingTargets []gameengine.Target
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil {
			continue
		}
		if si.Card == castCard && si.Controller == casterSeat {
			originatingTargets = append(originatingTargets, si.Targets...)
			break
		}
	}

	// Push the copy. Route through MintSpellCopy so the copy gets a
	// fresh CP-provenance InstanceID with the source ID recorded as
	// SourceInstanceID. The pre-r60 shape aliased `castCard` directly
	// into the StackItem; on §707.10 cease at resolve, stack.go:1312
	// fired MarkInstanceIDCeased(item.Card.InstanceID) which retired the
	// ORIGINAL's ID because the StackItem.Card pointer WAS the original.
	// Every invariant tick thereafter walked seat 1's hand/graveyard,
	// found the original *Card present, and flagged it as fabricated —
	// the dominant ZoneConservation residual on the post-#851 5K seed-42
	// sweep (34 hits on Lash Out / game 2762 / Aziza pod; closed here).
	copyCard := gameengine.MintSpellCopy(gs, castCard)
	copyItem := &gameengine.StackItem{
		Controller: casterSeat,
		Card:       copyCard,
		IsCopy:     true,
		Targets:    originatingTargets,
		CostMeta:   map[string]interface{}{},
	}
	gs.Stack = append(gs.Stack, copyItem)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       casterSeat,
		"copied":     castCard.DisplayName(),
		"tapped":     len(toTap),
		"stack_size": len(gs.Stack),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"choose_new_targets_for_copy_not_modelled_targets_inherited_from_original")
}
