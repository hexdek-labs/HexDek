package per_card

import (
	"sort"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJonIrenicusShatteredOne wires Jon Irenicus, Shattered One.
//
// Oracle text (Scryfall, verified, {2}{B}{R}, 3/4 legendary):
//
//	At the beginning of your end step, target opponent gains control
//	of up to one target creature you control. Put two +1/+1 counters
//	on it and tap it. It's goaded for the rest of the game and it
//	gains "This creature can't be sacrificed."
//	Whenever a creature you own but don't control attacks, you draw
//	a card.
//
// Implementation (R42 stub port):
//   - end_step trigger (gated to active_seat == controller): pick the
//     lowest-life living opponent as the recipient and the lowest-
//     power non-Jon, non-token creature controlled by Jon's
//     controller as the gift. Strategic policy: minimise own value
//     given away, maximise board pressure on the recipient.
//   - On the chosen creature: add 2 +1/+1 counters, tap, goad with
//     a far-future expiry (oracle says "for the rest of the game",
//     which the goad surface models as goaded_until_turn far ahead),
//     set "cant_be_sacrificed" flag. The control-change side
//     (transferring the perm to the opponent's battlefield) requires
//     touch on the legend-rule / attachment / replacement pipeline
//     that goes beyond a per_card hook — emitPartial breadcrumb.
//   - creature_attacks trigger: when ctx attacker's Owner ==
//     controller and Controller != controller (i.e. a creature we
//     own but don't control attacked), draw a card. The original
//     printed line; the new owner doesn't even need to swing AT us
//     for it to fire.
func registerJonIrenicusShatteredOne(r *Registry) {
	r.OnTrigger("Jon Irenicus, Shattered One", "end_step", jonIrenicusDonate)
	r.OnTrigger("Jon Irenicus, Shattered One", "creature_attacks", jonIrenicusOwnNotControlDraw)
}

func jonIrenicusDonate(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jon_irenicus_donate_end_step"
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

	// Pick the lowest-life living opponent.
	recipient := -1
	bestLife := 1 << 30
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		opp := gs.Seats[oppIdx]
		if opp == nil || opp.Lost {
			continue
		}
		if opp.Life < bestLife {
			bestLife = opp.Life
			recipient = oppIdx
		}
	}
	if recipient < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_living_opponent", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}

	// Pick the lowest-power non-Jon, non-token creature we control.
	var gifts []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !p.IsCreature() || p.IsToken() {
			continue
		}
		gifts = append(gifts, p)
	}
	sort.SliceStable(gifts, func(i, j int) bool {
		return gifts[i].Power() < gifts[j].Power()
	})
	if len(gifts) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_donatable_creature", map[string]interface{}{
			"seat":      perm.Controller,
			"recipient": recipient,
		})
		return
	}
	gift := gifts[0]

	gift.AddCounter("+1/+1", 2)
	gift.Tapped = true
	gameengine.GoadCreature(gs, recipient, gift)
	if gift.Flags == nil {
		gift.Flags = map[string]int{}
	}
	// Pin the goad expiry well beyond any realistic game length so the
	// "for the rest of the game" clause sticks even if the goader's
	// turn order changes (multi-goader semantics aren't modelled yet).
	gift.Flags["goaded_until_turn"] = gs.Turn + 9999
	gift.Flags["cant_be_sacrificed"] = 1

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"recipient":        recipient,
		"gift":             gift.Card.DisplayName(),
		"counters_added":   2,
		"tapped":           true,
		"goaded":           true,
		"cant_be_sacrificed": true,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"control_change_to_opponent_battlefield_requires_legend_rule_and_attachment_pipeline_not_modeled_at_per_card_layer")
}

func jonIrenicusOwnNotControlDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jon_irenicus_own_not_control_attacks_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Card == nil {
		return
	}
	if atk.Owner != perm.Controller {
		return
	}
	if atk.Controller == perm.Controller {
		return
	}
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"attacker":      atk.Card.DisplayName(),
		"attacker_seat": atk.Controller,
		"drew":          drawn != nil,
	})
}
