package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYshtola wires Y'shtola, Night's Blessed (Batch #30 rewrite).
//
// Oracle text (Scryfall, Final Fantasy Commander, verified 2026-05-01):
//
//	{1}{W}{U}{B}, 2/4 Legendary Creature — Cat Warlock
//	Vigilance
//	At the beginning of each end step, if a player lost 4 or more life
//	  this turn, you draw a card.
//	Whenever you cast a noncreature spell with mana value 3 or greater,
//	  Y'shtola deals 2 damage to each opponent and you gain 2 life.
//
// Implementation:
//   - Vigilance: AST keyword pipeline.
//   - life_lost listener: tally per-(player,turn) life loss into perm
//     flags. Each Y'shtola tracks its own counters so multiple copies
//     are independent.
//   - end_step: at every player's end step ("each end step"), check the
//     per-turn tally; if ANY player crossed the 4-life threshold this
//     turn, draw a card. Reset turn-scoped tallies after firing.
//   - spell_cast: gate on caster_seat == controller, noncreature,
//     MV >= 3. Deal 2 damage to each living opponent (raw Life subtract
//     since the engine has no DealDamageToPlayer helper) and gain 2
//     life.
func registerYshtola(r *Registry) {
	r.OnTrigger("Y'shtola, Night's Blessed", "life_lost", yshtolaTrackLifeLost)
	r.OnTrigger("Y'shtola, Night's Blessed", "end_step", yshtolaEndStep)
	r.OnTrigger("Y'shtola, Night's Blessed", "spell_cast", yshtolaSpellCast)
}

func yshtolaLossKey(turn, seat int) string {
	return "yshtola_loss_t" + strconv.Itoa(turn+1) + "_s" + strconv.Itoa(seat)
}

func yshtolaTrackLifeLost(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	lossSeat, ok := ctx["seat"].(int)
	if !ok || lossSeat < 0 || lossSeat >= len(gs.Seats) {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags[yshtolaLossKey(gs.Turn, lossSeat)] += amount
}

func yshtolaEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yshtola_end_step_draw"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}

	threshold := 0
	worstSeat := -1
	for i := range gs.Seats {
		key := yshtolaLossKey(gs.Turn, i)
		lost := perm.Flags[key]
		delete(perm.Flags, key)
		if lost > threshold {
			threshold = lost
			worstSeat = i
		}
	}
	yshtolaPruneLossKeys(perm, gs.Turn)

	if threshold < 4 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"max_loss": threshold,
			"drew":     0,
			"required": 4,
		})
		return
	}
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drewName := ""
	if drawn != nil {
		drewName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"max_loss":       threshold,
		"max_loss_seat":  worstSeat,
		"drew":           1,
		"card":           drewName,
	})
}

func yshtolaSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yshtola_noncreature_mv3_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if cardHasType(card, "creature") {
		return
	}
	if card == perm.Card {
		return
	}
	if gameengine.ManaCostOf(card) < 3 {
		return
	}

	// Damage 2 to each living opponent.
	hits := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		s.Life -= 2
		hits++
		gs.LogEvent(gameengine.Event{
			Kind:   "damage",
			Seat:   perm.Controller,
			Target: opp,
			Source: perm.Card.DisplayName(),
			Amount: 2,
			Details: map[string]interface{}{
				"slug":   slug,
				"reason": "yshtola_noncreature_mv3",
			},
		})
	}
	gameengine.GainLife(gs, perm.Controller, 2, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"spell":     card.DisplayName(),
		"opps_hit":  hits,
		"life_gain": 2,
	})
	_ = gs.CheckEnd()
}

func yshtolaPruneLossKeys(perm *gameengine.Permanent, currentTurn int) {
	if perm == nil || perm.Flags == nil {
		return
	}
	prefix := "yshtola_loss_t"
	cutoff := currentTurn + 1
	for k := range perm.Flags {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		rest := k[len(prefix):]
		under := -1
		for i, ch := range rest {
			if ch == '_' {
				under = i
				break
			}
		}
		if under <= 0 {
			continue
		}
		n, err := strconv.Atoi(rest[:under])
		if err != nil {
			continue
		}
		if n < cutoff {
			delete(perm.Flags, k)
		}
	}
}
