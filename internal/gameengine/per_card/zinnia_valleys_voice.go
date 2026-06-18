package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZinniaValleysVoice wires Zinnia, Valley's Voice.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{U}{R}{W}
//	Legendary Creature — Bird Bard
//	1/3
//	Flying
//	Zinnia gets +X/+0, where X is the number of other creatures you
//	control with base power 1.
//	Creature spells you cast gain offspring {2} as you cast them. (You
//	may pay an additional {2} as you cast a creature spell. If you do,
//	when that creature enters, create a 1/1 token copy of it.)
//
// Implementation:
//   - ETB: count other creatures you control with base power 1, set
//     Zinnia's temp_power buff. Like Jarad, this is a static buff that
//     should layer continuously; we refresh on ETB only and on each
//     creature_etb trigger. emitPartial flags incomplete coverage.
//   - creature_etb (gated on controller_seat == perm.Controller): mint a
//     1/1 token copy of the entering creature. We can't see the "did the
//     player pay {2}" signal, so we always mint — biased toward upside
//     for the controller. The cost is hidden in the AI's mana model.
func registerZinniaValleysVoice(r *Registry) {
	r.OnETB("Zinnia, Valley's Voice", zinniaValleysVoiceETB)
	r.OnTrigger("Zinnia, Valley's Voice", "permanent_etb", zinniaValleysVoiceOffspring)
}

func zinniaValleysVoiceETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zinnia_valleys_voice_etb_buff"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Card.BasePower == 1 {
			count++
		}
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["temp_power"] += count
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"power_1_count": count,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"static_buff_only_refreshed_on_etb_not_layered_continuously")
}

func zinniaValleysVoiceOffspring(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zinnia_valleys_voice_offspring_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil {
		enteringPerm, _ = ctx["permanent"].(*gameengine.Permanent)
	}
	if enteringPerm == nil || enteringPerm == perm || enteringPerm.Card == nil {
		return
	}
	if enteringPerm.Controller != perm.Controller {
		return
	}
	if !enteringPerm.IsCreature() {
		return
	}
	if enteringPerm.IsToken() {
		return
	}
	src := enteringPerm.Card
	// Route through the canonical offspring primitive: a faithful 1/1 token
	// copy minted via the create-a-copy path (token doublers apply,
	// token_created fires, fresh *Card per token — no aliasing). Replaces the
	// old hand-rolled name/types/colors-only token, which dropped the copied
	// abilities and bypassed both doublers and the token_created trigger.
	gameengine.CreateOffspringToken(gs, enteringPerm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"original": src.DisplayName(),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"offspring_2_cost_not_modeled_token_always_minted")
}
