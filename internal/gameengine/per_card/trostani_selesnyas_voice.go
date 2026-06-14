package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTrostaniSelesnyasVoice wires Trostani, Selesnya's Voice
// (Muninn parser-gap #174, single-game hit on 2026-05-17).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{G}{G}{W}{W}
//	Legendary Creature — Dryad
//	Whenever another creature you control enters, you gain life equal
//	to that creature's toughness.
//	{1}{G}{W}, {T}: Populate. (Create a token that's a copy of a
//	creature token you control.)
//
// Implementation:
//   - "permanent_etb" observer: when another creature enters under
//     Trostani's controller, gain life equal to the entering
//     creature's current Toughness() (post-replacement, post-counter).
//     Self-trigger is suppressed via the entered != perm check; CR
//     §603.6e — "another" excludes the source.
//   - OnActivated index 0: populate. The engine's generic populate
//     path (resolve_helpers.go:2849) is a log-only stub, and copying a
//     creature token via per_card requires picking which token to copy
//     plus a faithful Permanent clone (timestamp + counters + flags).
//     We pick the highest-toughness creature-token the controller
//     controls and mint a token Card mirroring its printed P/T,
//     subtypes, and colors via enterBattlefieldWithETB(false). If no
//     creature token is on the battlefield, populate is a legal no-op
//     (CR §701.30b).
func registerTrostaniSelesnyasVoice(r *Registry) {
	r.OnTrigger("Trostani, Selesnya's Voice", "permanent_etb", trostaniVoiceCreatureETB)
	r.OnActivated("Trostani, Selesnya's Voice", trostaniVoicePopulate)
}

func trostaniVoiceCreatureETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "trostani_voice_etb_lifegain"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered == perm || !entered.IsCreature() {
		return
	}
	tough := entered.Toughness()
	if tough <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"gained":   0,
			"entered":  entered.Card.DisplayName(),
			"toughness": tough,
		})
		return
	}
	gameengine.GainLife(gs, perm.Controller, tough, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"gained":    tough,
		"entered":   entered.Card.DisplayName(),
		"toughness": tough,
	})
}

func trostaniVoicePopulate(gs *gameengine.GameState, src *gameengine.Permanent, idx int, ctx map[string]interface{}) {
	const slug = "trostani_voice_populate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	var best *gameengine.Permanent
	bestScore := -1
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "token") {
			continue
		}
		score := p.Power()*2 + p.Toughness()
		if score > bestScore {
			best = p
			bestScore = score
		}
	}
	if best == nil {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"copied": "",
		})
		return
	}

	// CR §707 populate: "create a token that's a copy of a creature token
	// you control." Route through MintTokenAsCopyOf so the copy carries the
	// source token's FULL copiable values — crucially its ABILITIES (AST) —
	// not just P/T+types+colors. The old hand-built Card dropped the AST, so
	// the populated copy had none of the source's rules text and its ETB
	// triggers were blank. MintTokenAsCopyOf DeepCopies the source card
	// (abilities included) and mints a fresh InstanceID; enterBattlefieldWithETB
	// then fires the copied creature's ETB triggers.
	token := gameengine.MintTokenAsCopyOf(gs, best.Card, seat, "")
	if token == nil {
		token = &gameengine.Card{
			Name:          best.Card.Name,
			Owner:         seat,
			Types:         append([]string{}, best.Card.Types...),
			Colors:        append([]string{}, best.Card.Colors...),
			BasePower:     best.Card.BasePower,
			BaseToughness: best.Card.BaseToughness,
			TypeLine:      best.Card.TypeLine,
		}
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"copied": best.Card.DisplayName(),
		"pt":     []int{best.Card.BasePower, best.Card.BaseToughness},
	})
}
