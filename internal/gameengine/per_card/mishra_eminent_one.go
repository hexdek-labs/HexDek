package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Mishra, Eminent One — {1}{R} 2/2 Legendary Creature — Phyrexian
// Artificer.
//
//   At the beginning of combat on your turn, create a token that's a
//   copy of target noncreature artifact you control, except its name
//   is Mishra's Warform and it's a 4/4 Construct artifact creature in
//   addition to its other types. It gains haste until end of turn.
//   Sacrifice it at the beginning of the next end step.
//
// Implementation: OnTrigger combat_begin (alias for begin_combat) with
// active_seat == controller filter. Pick highest-CMC noncreature artifact
// controlled; MintTokenAsCopyOf (Calix/Anikthea pattern); rename to
// "Mishra's Warform"; stamp BasePower=4 / BaseToughness=4; append
// "creature" + "construct" if missing (preserve original types per "in
// addition to"); strip Legendary; mark haste via Flags; register
// next_end_step delayed sac trigger.

func init() {
	registerMishraEminentOne(Global())
	AddResetHook(registerMishraEminentOne)
}

func registerMishraEminentOne(r *Registry) {
	r.OnTrigger("Mishra, Eminent One", "combat_begin", mishraEminentOneCombatBegin)
}

func mishraEminentOneCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mishra_eminent_warform_copy"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	var pick *gameengine.Permanent
	bestCMC := -1
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "artifact") {
			continue
		}
		if cardHasType(p.Card, "creature") {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc > bestCMC {
			bestCMC = cmc
			pick = p
		}
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"reason": "no_noncreature_artifact_to_copy",
		})
		return
	}
	copyCard := gameengine.MintTokenAsCopyOf(gs, pick.Card, seat, gameengine.CurrentMintEnablerID(gs))
	if copyCard == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "mint_token_returned_nil", nil)
		return
	}
	copyCard.IsCopy = true
	copyCard.Name = "Mishra's Warform"
	copyCard.BasePower = 4
	copyCard.BaseToughness = 4
	if !cardHasType(copyCard, "creature") {
		copyCard.Types = append(copyCard.Types, "creature")
	}
	if !cardHasSubtype(copyCard, "construct") {
		copyCard.Types = append(copyCard.Types, "construct")
	}
	filtered := copyCard.Types[:0]
	for _, t := range copyCard.Types {
		if t == "legendary" || t == "Legendary" {
			continue
		}
		filtered = append(filtered, t)
	}
	copyCard.Types = filtered
	token := enterBattlefieldWithETB(gs, seat, copyCard, false)
	if token == nil {
		return
	}
	if token.Flags == nil {
		token.Flags = map[string]int{}
	}
	token.Flags["kw:haste"] = 1
	token.Flags["haste_eot"] = 1
	token.SummoningSick = false
	tokenRef := token
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seat,
		SourceCardName: perm.Card.DisplayName() + " (warform sac)",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.SacrificePermanent(gs, tokenRef, "mishra_eminent_warform_eot_sac")
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            seat,
		"copied_from":     pick.Card.DisplayName(),
		"warform_pt":      [2]int{4, 4},
	})
}
