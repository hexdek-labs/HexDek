package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLeonardoTheBalance wires Leonardo, the Balance.
//
// Oracle text (Scryfall, verified 2026-05-01; TMNT crossover):
//
//	Whenever a token you control enters, you may put a +1/+1 counter on
//	each creature you control. Do this only once each turn.
//	{W}{U}{B}{R}{G}: Creatures you control gain menace, trample, and
//	lifelink until end of turn.
//	Partner—Character select (You can have two commanders if both have
//	this ability.)
//
// Implementation:
//   - OnTrigger("token_created"): when a token enters under Leonardo's
//     controller, place a +1/+1 counter on each creature they control.
//     "Once each turn" — gated by a per-turn flag on Leonardo's own
//     permanent so the counter spread fires at most once per turn even
//     if multiple tokens enter. The "may" is auto-yes (always upside).
//   - OnActivated: grant menace, trample, and lifelink to every creature
//     Leonardo's controller controls until end of turn. Cleanup happens
//     via a delayed "next_end_step" trigger that strips the kw:* flags
//     from the captured set.
//   - "Partner—Character select" is a deck-construction restriction with
//     no in-game effect; no handler needed.
func registerLeonardoTheBalance(r *Registry) {
	r.OnTrigger("Leonardo, the Balance", "token_created", leonardoTokenCounterSpread)
	r.OnActivated("Leonardo, the Balance", leonardoFiveColorGrant)
}

func leonardoTokenCounterSpread(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "leonardo_the_balance_token_counter_spread"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}

	// "Do this only once each turn." Per-turn lockout on Leonardo herself.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupeKey := fmt.Sprintf("leonardo_balance_t%d", gs.Turn+1)
	if perm.Flags[dedupeKey] == 1 {
		return
	}
	perm.Flags[dedupeKey] = 1

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.AddCounter("+1/+1", 1)
		count++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"creatures_buffed": count,
	})
}

func leonardoFiveColorGrant(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "leonardo_the_balance_wubrg_grant"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}

	captured := []*gameengine.Permanent{}
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:menace"] = 1
		p.Flags["kw:trample"] = 1
		p.Flags["kw:lifelink"] = 1
		captured = append(captured, p)
	}
	if len(captured) > 0 {
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: src.Controller,
			SourceCardName: src.Card.DisplayName(),
			EffectFn: func(gs *gameengine.GameState) {
				for _, p := range captured {
					if p == nil || p.Flags == nil {
						continue
					}
					delete(p.Flags, "kw:menace")
					delete(p.Flags, "kw:trample")
					delete(p.Flags, "kw:lifelink")
				}
			},
		})
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             src.Controller,
		"creatures_buffed": len(captured),
		"keywords":         []string{"menace", "trample", "lifelink"},
		"duration":         "until_end_of_turn",
	})
}
