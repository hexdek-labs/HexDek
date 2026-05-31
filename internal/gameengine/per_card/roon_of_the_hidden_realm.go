package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Roon of the Hidden Realm — {2}{G}{W}{U} 3/3 Legendary Creature — Rhino
// Soldier.
//
//   Vigilance, trample
//   {2}, {T}: Exile another target creature. Return that card to the
//   battlefield under its owner's control at the beginning of the next
//   end step.
//
// Implementation: OnActivated, target picked from ctx["target_perm"];
// must be a creature, must be != Roon ("another"). Use ExilePermanent
// for the canonical battlefield-exit (handles §614 + DetachAll +
// replacement-unregister); register a next_end_step delayed trigger
// that calls enterBattlefieldWithETB for the owner's seat (Ardbert
// pattern at ardbert_warrior_of_darkness.go:121).
//
// Vigilance/trample are keyword-pipeline.

func init() {
	registerRoonOfTheHiddenRealm(Global())
	AddResetHook(registerRoonOfTheHiddenRealm)
}

func registerRoonOfTheHiddenRealm(r *Registry) {
	r.OnActivated("Roon of the Hidden Realm", roonActivate)
}

func roonActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "roon_exile_and_return"
	if gs == nil || src == nil || src.Card == nil || ctx == nil {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_in_ctx", nil)
		return
	}
	if !target.IsCreature() {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_creature",
			map[string]interface{}{"card": target.Card.DisplayName()})
		return
	}
	if target == src {
		emitFail(gs, slug, src.Card.DisplayName(), "self_target_refused", nil)
		return
	}
	ownerSeat := target.Card.Owner
	targetCard := target.Card
	gameengine.ExilePermanent(gs, target, src)
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: src.Card.DisplayName() + " (return blink)",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
				return
			}
			if !gameengine.CardCanEnterBattlefield(targetCard) {
				return
			}
			moveCardBetweenZones(gs, ownerSeat, targetCard, "exile", "battlefield", "roon_return_from_exile")
			enterBattlefieldWithETB(gs, ownerSeat, targetCard, false)
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         src.Controller,
		"target_card":  targetCard.DisplayName(),
		"owner_seat":   ownerSeat,
		"return_at":    "next_end_step",
	})
}
