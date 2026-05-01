package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPrimoTheUnbounded wires Primo, the Unbounded. Batch #33.
//
// Oracle text (Streets of New Capenna Commander, {X}{G}{G}{U}, Legendary
// Creature — Fractal Wolf, 0/0):
//
//	Trample
//	Primo enters with twice X +1/+1 counters on it.
//	Whenever one or more creatures you control with base power 0 deal
//	combat damage to a player, create a 0/0 green and blue Fractal
//	creature token. Put a number of +1/+1 counters on it equal to the
//	damage dealt.
//
// Implementation:
//   - Trample: AST keyword.
//   - OnCast: capture StackItem.ChosenX into gs.Flags["_primo_x_<seat>"]
//     so the ETB hook (which doesn't see the stack item) can read it.
//   - ETB: read the X flag (consume it), add 2X +1/+1 counters.
//   - "combat_damage_player": gate on source controlled by Primo's
//     controller and source's BasePower == 0. The ctx exposes
//     source_card (string name), so we look the source up on the
//     controller's battlefield. Accumulate the damage on Primo.Flags
//     ["primo_pending_damage"] and register a one-shot end_of_combat
//     delayed trigger (only on first accumulation per combat) that
//     drains the accumulator into a single Fractal token. This
//     captures the "one or more...deal combat damage...create a 0/0
//     Fractal" aggregation faithfully.
func registerPrimoTheUnbounded(r *Registry) {
	r.OnCast("Primo, the Unbounded", primoOnCast)
	r.OnETB("Primo, the Unbounded", primoETB)
	r.OnTrigger("Primo, the Unbounded", "combat_damage_player", primoCombatDamage)
}

func primoOnCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["_primo_x_"+intToStr(item.Controller)] = item.ChosenX
}

func primoETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "primo_the_unbounded_etb_with_2x_counters"
	if gs == nil || perm == nil {
		return
	}
	x := 0
	key := "_primo_x_" + intToStr(perm.Controller)
	if gs.Flags != nil {
		if v, ok := gs.Flags[key]; ok {
			x = v
			delete(gs.Flags, key)
		}
	}
	if x < 0 {
		x = 0
	}
	counters := 2 * x
	if counters > 0 {
		perm.AddCounter("+1/+1", counters)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"x_value":      x,
		"counters_added": counters,
	})
}

func primoCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "primo_the_unbounded_fractal_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceCard, _ := ctx["source_card"].(string)
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	if !primoSourceIsBasePower0(gs, perm.Controller, sourceCard) {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	priorPending := perm.Flags["primo_pending_damage"]
	perm.Flags["primo_pending_damage"] = priorPending + amount

	// Register the end_of_combat token-creation delayed trigger only on the
	// first accumulation in this combat. The trigger drains the accumulator
	// into a single Fractal token, faithfully capturing "one or more... deal
	// combat damage" → ONE token sized by TOTAL damage.
	if priorPending == 0 {
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "end_of_combat",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				if perm == nil || perm.Flags == nil {
					return
				}
				total := perm.Flags["primo_pending_damage"]
				perm.Flags["primo_pending_damage"] = 0
				if total <= 0 {
					return
				}
				primoCreateFractalToken(gs, perm, total)
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"source_card":      sourceCard,
		"damage":           amount,
		"pending_total":    perm.Flags["primo_pending_damage"],
	})
}

// primoSourceIsBasePower0 returns true if the named source card is on the
// controller's battlefield and has BasePower == 0. We look up by display
// name because combat_damage_player ctx exposes the source as a string.
func primoSourceIsBasePower0(gs *gameengine.GameState, seatIdx int, name string) bool {
	if gs == nil || name == "" {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Card.DisplayName() != name {
			continue
		}
		if p.Card.BasePower == 0 {
			return true
		}
	}
	return false
}

func primoCreateFractalToken(gs *gameengine.GameState, primo *gameengine.Permanent, counters int) {
	const slug = "primo_the_unbounded_fractal_token_create"
	if gs == nil || primo == nil {
		return
	}
	seat := gs.Seats[primo.Controller]
	if seat == nil {
		return
	}
	tokenCard := &gameengine.Card{
		Name:          "Fractal Token",
		Owner:         primo.Controller,
		Types:         []string{"creature", "token", "fractal", "pip:G", "pip:U"},
		Colors:        []string{"G", "U"},
		BasePower:     0,
		BaseToughness: 0,
	}
	token := &gameengine.Permanent{
		Card:          tokenCard,
		Controller:    primo.Controller,
		Owner:         primo.Controller,
		Tapped:        false,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{"primo_fractal_token": 1},
	}
	if counters > 0 {
		token.AddCounter("+1/+1", counters)
	}
	seat.Battlefield = append(seat.Battlefield, token)
	gameengine.RegisterReplacementsForPermanent(gs, token)
	gameengine.FirePermanentETBTriggers(gs, token)

	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   primo.Controller,
		Source: primo.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug":     slug,
			"token":    "Fractal Token",
			"power":    0,
			"tough":    0,
			"counters": counters,
			"reason":   "primo_combat_damage",
		},
	})
	emit(gs, slug, primo.Card.DisplayName(), map[string]interface{}{
		"seat":     primo.Controller,
		"counters": counters,
	})
}
