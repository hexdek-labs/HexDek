package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Kykar, Zephyr Awakener — {1}{R}{W}{U} 3/3 Legendary Creature — Bird
// Cleric.
//
//   Flying
//   Whenever you cast a noncreature spell, choose one —
//   • Exile another target creature you control. Return that card to
//     the battlefield under its owner's control at the beginning of
//     the next end step.
//   • Create a 1/1 white Spirit creature token with flying.
//
// Greedy mode: blink if another creature is available, else Spirit token.

func init() {
	registerKykarZephyrAwakener(Global())
	AddResetHook(registerKykarZephyrAwakener)
}

func registerKykarZephyrAwakener(r *Registry) {
	r.OnTrigger("Kykar, Zephyr Awakener", "noncreature_spell_cast", kykarZephyrNoncreatureCast)
}

func kykarZephyrNoncreatureCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kykar_zephyr_noncreature_cast"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || s.Lost {
		return
	}
	var blinkTarget *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		blinkTarget = p
		break
	}
	if blinkTarget != nil {
		ownerSeat := blinkTarget.Card.Owner
		targetCard := blinkTarget.Card
		gameengine.ExilePermanent(gs, blinkTarget, perm)
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName() + " (blink return)",
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
					return
				}
				if !gameengine.CardCanEnterBattlefield(targetCard) {
					return
				}
				moveCardBetweenZones(gs, ownerSeat, targetCard, "exile", "battlefield", "kykar_zephyr_return")
				enterBattlefieldWithETB(gs, ownerSeat, targetCard, false)
			},
		})
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":        perm.Controller,
			"mode":        "blink",
			"target_card": targetCard.DisplayName(),
			"owner_seat":  ownerSeat,
		})
		return
	}
	token := gameengine.CreateCreatureToken(gs, perm.Controller, "Spirit Token",
		[]string{"creature", "spirit", "pip:W", "flying"}, 1, 1)
	tokenName := ""
	if token != nil {
		tokenName = token.Card.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"mode":  "spirit_token",
		"token": tokenName,
	})
}
