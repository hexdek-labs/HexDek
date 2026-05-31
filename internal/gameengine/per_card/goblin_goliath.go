package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGoblinGoliath wires Goblin Goliath (Muninn parser-gap #61,
// ~12K hits).
//
// Oracle text (Scryfall, verified 2026-05-30, {4}{R}{R}, 5/4 Creature
// — Goblin Mutant):
//
//	When this creature enters, create a number of 1/1 red Goblin
//	creature tokens equal to the number of opponents you have.
//	{3}{R}, {T}: If a source you control would deal damage to an
//	opponent this turn, it deals double that damage to that player
//	instead.
//
// Implementation (R60 stub sweep):
//   - ETB: count living opponents, mint that many 1/1 red Goblin tokens.
//   - OnActivated: tap; register a DamageReplacement that doubles
//     Amount when ctx.Source.Controller == Goblin Goliath's controller
//     AND the target is an opponent (ctx.Kind in {DamageCombatPlayer,
//     DamageNonCombatPlayer}). Register a delayed trigger at next
//     end_step to unregister via UnregisterDamageReplacementsForPermanent
//     (the "this turn" gating).
func registerGoblinGoliath(r *Registry) {
	r.OnETB("Goblin Goliath", goblinGoliathETB)
	r.OnActivated("Goblin Goliath", goblinGoliathActivate)
}

func goblinGoliathETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "goblin_goliath_etb_goblin_tokens"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	living := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		living++
	}
	for i := 0; i < living; i++ {
		token := &gameengine.Card{
			Name:          "Goblin Token",
			Owner:         perm.Controller,
			Types:         []string{"creature", "token", "goblin", "pip:R"},
			Colors:        []string{"R"},
			BasePower:     1,
			BaseToughness: 1,
		}
		enterBattlefieldWithETB(gs, perm.Controller, token, false)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"opponents": living,
		"tokens":    living,
	})
}

func goblinGoliathActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "goblin_goliath_double_damage"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	src.Tapped = true
	controller := src.Controller
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: src,
		HandlerID:  "goblin_goliath_double_to_opponent",
		Fn: func(gs *gameengine.GameState, dctx *gameengine.DamageContext) {
			if dctx == nil || dctx.Source == nil {
				return
			}
			if dctx.Source.Controller != controller {
				return
			}
			// Only affect damage to PLAYERS (CR §612 damage redirection
			// to creatures is a different surface). Both combat and
			// non-combat player damage count as "deal damage to an
			// opponent" per oracle text.
			if dctx.Kind != gameengine.DamageCombatPlayer && dctx.Kind != gameengine.DamageNonCombatPlayer {
				return
			}
			// Opponent filter: TargetSeat must not be controller.
			if dctx.TargetSeat == controller {
				return
			}
			if dctx.Amount <= 0 {
				return
			}
			dctx.Amount *= 2
		},
	})
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: controller,
		SourceCardName: src.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			gs.UnregisterDamageReplacementsForPermanent(src)
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     controller,
		"duration": "until_end_of_turn",
	})
}
