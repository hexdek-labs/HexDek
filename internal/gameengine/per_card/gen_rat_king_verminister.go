package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRatKingVerminister wires Rat King, Verminister.
//
// Oracle text (Scryfall, verified):
//
//	Disappear — At the beginning of your end step, if a permanent
//	left the battlefield under your control this turn, create a 1/1
//	black Rat creature token and put a +1/+1 counter on Rat King.
//	{T}, Sacrifice three Rats: Return target creature card and all
//	other cards with the same name as that card from your graveyard
//	to the battlefield tapped.
//
// Implementation (R49 stub port):
//   - Disappear (end_step trigger): gate on active_seat == controller
//     ("your end step"). Check seat.Turn.PermanentsLeft > 0 — true iff
//     any permanent left this seat's battlefield this turn (covers
//     dies, exile, bounce, sac, top-deck shuffle). On hit: 1/1 black
//     Rat token + a +1/+1 counter on Rat King.
//   - Activated {T} + Sacrifice three Rats: pick the largest target
//     creature card in the controller's graveyard (no UI for target
//     selection at the per_card layer — go by power+toughness as the
//     "best return" heuristic, falling back to most recent), then
//     return that card AND every other card with the same name from
//     the graveyard, each tapped. SacrificePermanent for the three
//     Rat-cost picks (lowest-PT Rats, never Rat King itself unless
//     it's the only Rat).
func registerRatKingVerminister(r *Registry) {
	r.OnTrigger("Rat King, Verminister", "end_step", ratKingDisappearEndStep)
	r.OnActivated("Rat King, Verminister", ratKingVerministerActivate)
}

func ratKingDisappearEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rat_king_disappear_endstep"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Turn.PermanentsLeft <= 0 {
		return
	}
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Rat Token",
		[]string{"creature", "rat"}, 1, 1)
	if tok != nil && tok.Card != nil {
		tok.Card.Colors = []string{"B"}
	}
	perm.AddCounter("+1/+1", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func ratKingVerministerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "rat_king_sac_three_rats_return"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}

	// Pick three smallest-PT Rats to sacrifice. Skip Rat King unless we
	// have no other rats — sacrificing the source is allowed but a poor
	// trade (loses the +1/+1 counter and unblockable progress).
	rats := []*gameengine.Permanent{}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if cardHasType(p.Card, "rat") {
			rats = append(rats, p)
		}
	}
	if len(rats) < 3 {
		emitFail(gs, slug, src.Card.DisplayName(), "not_enough_rats", map[string]interface{}{
			"rats_on_bf": len(rats),
		})
		return
	}
	// Sort by ascending PT, with src (Rat King) pushed to the end so
	// it's only picked if absolutely needed.
	for i := 0; i < len(rats); i++ {
		for j := i + 1; j < len(rats); j++ {
			ai := gs.PowerOf(rats[i]) + gs.ToughnessOf(rats[i])
			aj := gs.PowerOf(rats[j]) + gs.ToughnessOf(rats[j])
			// src always sorts last regardless of PT.
			if rats[i] == src && rats[j] != src {
				rats[i], rats[j] = rats[j], rats[i]
				continue
			}
			if rats[j] == src {
				continue
			}
			if aj < ai {
				rats[i], rats[j] = rats[j], rats[i]
			}
		}
	}

	// Pick a target: largest-PT creature card in graveyard. Tiebreak:
	// most recent (last index).
	var target *gameengine.Card
	bestPT := -1
	for _, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		pt := c.BasePower + c.BaseToughness
		if pt >= bestPT {
			bestPT = pt
			target = c
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_in_graveyard", nil)
		return
	}
	targetName := target.DisplayName()

	src.Tapped = true
	for i := 0; i < 3; i++ {
		gameengine.SacrificePermanent(gs, rats[i], "rat_king_sac_cost")
	}

	// Return target + all other same-name creature cards from graveyard.
	returned := []string{}
	// Snapshot first since MoveCard mutates the slice we're iterating.
	picks := []*gameengine.Card{}
	for _, c := range seat.Graveyard {
		if c != nil && c.DisplayName() == targetName && cardHasType(c, "creature") {
			picks = append(picks, c)
		}
	}
	for _, c := range picks {
		gameengine.MoveCard(gs, c, seatIdx, "graveyard", "battlefield_tapped", "rat_king_return")
		returned = append(returned, c.DisplayName())
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seatIdx,
		"target":   targetName,
		"returned": returned,
	})
}
