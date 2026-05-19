package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAdmiralBrassUnsinkable wires Admiral Brass, Unsinkable.
//
// Oracle text (Scryfall, verified):
//
//	When Admiral Brass enters, mill four cards.
//	At the beginning of combat on your turn, you may return target
//	Pirate creature card from your graveyard to the battlefield with a
//	finality counter on it. It has base power and toughness 4/4. It
//	gains haste until end of turn.
//
// Implementation (R43 stub port):
//   - OnETB: mill 4 — top 4 cards from controller's library to
//     graveyard via MoveCard. Stops cleanly when library is empty.
//   - OnTrigger("combat_begin") gated on active_seat == controller:
//     AI policy is greedy-upside ("you may" → always do it when a
//     Pirate is available). Pick the highest-CMC Pirate creature
//     card in the controller's graveyard (best value to reanimate).
//     Move to battlefield via MoveCard, then on the resulting
//     permanent: stamp finality counter, overwrite BasePower /
//     BaseToughness to 4/4 (stub-level "base P/T" — Phase 8 layers
//     will supersede), set kw:haste flag, clear summoning sickness.
//   - Haste is until-end-of-turn; we capture the perm in a delayed
//     trigger to drop the kw:haste flag at next end step. The
//     finality counter and 4/4 base persist past EOT per oracle.
func registerAdmiralBrassUnsinkable(r *Registry) {
	r.OnETB("Admiral Brass, Unsinkable", admiralBrassETB)
	r.OnTrigger("Admiral Brass, Unsinkable", "combat_begin", admiralBrassBeginCombatReanim)
}

func admiralBrassETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "admiral_brass_mill_four"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	milled := 0
	for i := 0; i < 4; i++ {
		if len(seat.Library) == 0 {
			break
		}
		top := seat.Library[0]
		if top == nil {
			break
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "graveyard", "admiral_brass_mill")
		milled++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"milled": milled,
	})
}

func admiralBrassBeginCombatReanim(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "admiral_brass_pirate_reanim"
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

	bestIdx := -1
	bestCMC := -1
	for i, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if !cardHasSubtype(c, "pirate") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_pirate_creature_in_graveyard", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	card := seat.Graveyard[bestIdx]
	gameengine.MoveCard(gs, card, perm.Controller, "graveyard", "battlefield", "admiral_brass_reanim")

	var ent *gameengine.Permanent
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p != nil && p.Card == card {
			ent = p
			break
		}
	}
	if ent == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "post_move_perm_lookup_failed", map[string]interface{}{
			"card": card.DisplayName(),
		})
		return
	}
	ent.AddCounter("finality", 1)
	if ent.Card != nil {
		ent.Card.BasePower = 4
		ent.Card.BaseToughness = 4
	}
	if ent.Flags == nil {
		ent.Flags = map[string]int{}
	}
	ent.Flags["kw:haste"] = 1
	ent.SummoningSick = false

	captured := ent
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			delete(captured.Flags, "kw:haste")
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"returned": card.DisplayName(),
		"cmc":      bestCMC,
		"base_pt":  "4/4",
	})
}
