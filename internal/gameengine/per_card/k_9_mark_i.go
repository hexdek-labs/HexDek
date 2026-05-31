package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerK9MarkI wires K-9, Mark I.
//
// Oracle text (Scryfall, verified 2026-05-30, {U}, 1/1 Legendary
// Artifact Creature — Robot Dog):
//
//	Negative — As long as K-9 is untapped, other legendary creatures you
//	  control have ward {1}.
//	Affirmative — {1}{U}, {T}: Target legendary creature can't be
//	  blocked this turn.
//	Doctor's companion (You can have two commanders if the other is the
//	  Doctor.)
//
// Implementation (R60 stub sweep):
//   - OnActivated: tap, then stamp Flags["unblockable_until_eot"] = 1
//     and Flags["unblockable"] = 1 on the target legendary creature (the
//     canonical flag combat.go's blocker-legality check reads — same
//     surface used by Wraith / Norman Osborn). Register a delayed
//     trigger to clear both flags at next_end_step. ctx["target_perm"]
//     is the chosen target (legality check — must be a legendary
//     creature). If no valid target supplied, emitFail.
//   - The Negative ward grant is a continuous-effect static gated on
//     tap state — left as emitPartial on ETB.
func registerK9MarkI(r *Registry) {
	r.OnETB("K-9, Mark I", k9MarkIETB)
	r.OnActivated("K-9, Mark I", k9MarkIActivate)
}

func k9MarkIETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, "k9_mark_i_negative_static", perm.Card.DisplayName(),
		"ward_grant_to_legendary_creatures_when_untapped_not_modeled")
}

func k9MarkIActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "k9_mark_i_affirmative"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target", nil)
		return
	}
	if !target.IsCreature() || !cardHasType(target.Card, "legendary") {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_legendary_creature", nil)
		return
	}
	src.Tapped = true
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["unblockable"] = 1
	target.Flags["unblockable_until_eot"] = 1
	gs.InvalidateCharacteristicsCache()
	captured := target
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: src.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			if captured.Flags["unblockable_until_eot"] == 1 {
				delete(captured.Flags, "unblockable")
				delete(captured.Flags, "unblockable_until_eot")
				gs.InvalidateCharacteristicsCache()
			}
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": target.Card.DisplayName(),
		"effect": "unblockable_until_eot",
	})
}
