package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJaxisTheTroublemaker wires Jaxis, the Troublemaker.
//
// Oracle text (Scryfall, verified):
//
//	{R}, {T}, Discard a card: Create a token that's a copy of another
//	target creature you control. It gains haste and "When this token
//	dies, draw a card." Sacrifice it at the beginning of the next end
//	step. Activate only as a sorcery.
//	Blitz {1}{R}
//
// Implementation (R44 stub port):
//   - Activated ability: caller-paid cost ({R}, {T}, discard,
//     sorcery-speed). Target picker: highest-CMC own creature other
//     than Jaxis (mirrors the saheeli combat-copy heuristic). Build
//     a token-card copy with the target's printed Types / Base P/T,
//     prepend "token" so IsToken() reads true, append a
//     "jaxis_token" type tag so the die-draw observer can identify
//     us. Token enters with kw:haste set and summoning sickness
//     cleared.
//   - Delayed sacrifice at the next end step via RegisterDelayedTrigger
//     (same pattern as Saheeli's combat copy).
//   - "When this token dies, draw a card" rider: OnTrigger
//     ("creature_dies") on Jaxis herself checks the dying perm's
//     "jaxis_token" type tag; if set and the token's controller is
//     Jaxis's controller, drawOne.
//   - Blitz alt-cost is a cast-pipeline path and emitPartial'd here
//     (per_card layer doesn't intercept blitz costs).
func registerJaxisTheTroublemaker(r *Registry) {
	r.OnETB("Jaxis, the Troublemaker", jaxisTheTroublemakerETB)
	r.OnActivated("Jaxis, the Troublemaker", jaxisActivate)
	r.OnTrigger("Jaxis, the Troublemaker", "creature_dies", jaxisTokenDiesDraw)
}

func jaxisTheTroublemakerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "jaxis_the_troublemaker_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"blitz_alt_cost_cast_pipeline_not_intercepted_at_per_card_layer")
}

func jaxisActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jaxis_copy_creature"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Allow ctx["creature_perm"] to override target pick (matches the
	// chooseSacVictimNotSelf convention for caller-supplied targets).
	var pick *gameengine.Permanent
	if p, ok := ctx["creature_perm"].(*gameengine.Permanent); ok && p != nil && p != src && p.IsCreature() && p.Controller == src.Controller {
		pick = p
	}
	if pick == nil {
		bestCMC := -1
		for _, p := range seat.Battlefield {
			if p == nil || p == src || p.Card == nil {
				continue
			}
			if !p.IsCreature() {
				continue
			}
			c := cardCMC(p.Card)
			if c > bestCMC {
				bestCMC = c
				pick = p
			}
		}
	}
	if pick == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_other_creature_to_copy", map[string]interface{}{
			"seat": src.Controller,
		})
		return
	}

	tokenCard := &gameengine.Card{
		Name:          pick.Card.DisplayName() + " (Jaxis token)",
		Owner:         src.Controller,
		Types:         append([]string{"token", "jaxis_token"}, pick.Card.Types...),
		Colors:        append([]string{}, pick.Card.Colors...),
		BasePower:     pick.Card.BasePower,
		BaseToughness: pick.Card.BaseToughness,
	}
	tokenPerm := &gameengine.Permanent{
		Card:          tokenCard,
		Controller:    src.Controller,
		Owner:         src.Controller,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{"kw:haste": 1, "jaxis_token": 1},
		SummoningSick: false,
	}
	seat.Battlefield = append(seat.Battlefield, tokenPerm)

	capturedCard := tokenCard
	owner := src.Controller
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: owner,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.MoveCard(gs, capturedCard, owner, "battlefield", "graveyard", "jaxis_eos_sacrifice")
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         src.Controller,
		"copied":       pick.Card.DisplayName(),
		"token_pt":     []int{tokenCard.BasePower, tokenCard.BaseToughness},
		"haste":        true,
		"sac_at_eot":   true,
	})
}

// jaxisTokenDiesDraw fires when any creature dies. If the dying perm
// is a Jaxis-tagged token controlled by Jaxis's controller, drawOne.
func jaxisTokenDiesDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jaxis_token_dies_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil || dying.Card == nil {
		return
	}
	if dying.Flags == nil || dying.Flags["jaxis_token"] == 0 {
		return
	}
	if dying.Controller != perm.Controller {
		return
	}
	c := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drewName := ""
	if c != nil {
		drewName = c.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": dying.Card.DisplayName(),
		"drew":  drewName,
	})
}
