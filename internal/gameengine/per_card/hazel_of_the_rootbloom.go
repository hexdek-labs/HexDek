package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHazelOfTheRootbloom wires Hazel of the Rootbloom.
//
// Oracle text (Scryfall, verified 2026-05-30, {2}{B}{G}, 3/5
// Legendary Creature — Squirrel Druid):
//
//	{T}, Pay 2 life, Tap X untapped tokens you control: Add X mana
//	in any combination of colors.
//	At the beginning of your end step, create a token that's a copy
//	of target token you control. If that token is a Squirrel, instead
//	create two tokens that are copies of it.
//
// Implementation (R60 stub sweep):
//   - end_step (controller's own end step): find first token controlled
//     by Hazel's controller, build a token copy via Card.DeepCopy + IsCopy
//     stamp, mint via enterBattlefieldWithETB. If the chosen token has
//     "squirrel" in its types, mint TWO copies instead of one.
//   - The {T}, Pay 2 life, Tap X tokens activated mana ability is a
//     cost-pipeline gap — emitPartial. Mana sources that consume X
//     tokens as cost aren't expressed at the per_card layer.
func registerHazelOfTheRootbloom(r *Registry) {
	r.OnTrigger("Hazel of the Rootbloom", "end_step", hazelRootbloomEndStep)
}

func hazelRootbloomEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hazel_rootbloom_end_step_copy_token"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Pick first token controlled by Hazel's controller.
	var target *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "token") {
			continue
		}
		target = p
		break
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_token_to_copy", nil)
		return
	}

	count := 1
	if cardSubtypeMatches(target.Card, "squirrel") {
		count = 2
	}
	enablerID := gameengine.CurrentMintEnablerID(gs)
	for i := 0; i < count; i++ {
		// Phase 5 chokepoint: MintTokenAsCopyOf clears the inherited
		// InstanceID before stamping a fresh TK ID, preventing the
		// same-ID-in-two-perms duplication shape (PR #853 / Drafna fix).
		cp := gameengine.MintTokenAsCopyOf(gs, target.Card, perm.Controller, enablerID)
		if cp == nil {
			emitFail(gs, slug, perm.Card.DisplayName(), "mint_token_returned_nil", nil)
			continue
		}
		cp.IsCopy = true
		enterBattlefieldWithETB(gs, perm.Controller, cp, false)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copied": target.Card.DisplayName(),
		"count":  count,
	})
	emitPartial(gs, "hazel_rootbloom_activated_mana", perm.Card.DisplayName(),
		"tap_x_tokens_for_x_mana_activated_cost_pipeline_unwired_at_per_card_layer")
}
