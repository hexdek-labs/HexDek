package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEsperSentinel wires Esper Sentinel.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Esper%20Sentinel):
//
//	Whenever an opponent casts their first noncreature spell each
//	turn, draw a card unless that player pays {X}, where X is this
//	creature's power.
//
// {W} 1/1 Artifact Creature — Human Soldier. cEDH white staple.
//
// Implementation mirrors Rhystic Study's "pay or draw" pattern. Two
// gates that make Sentinel different:
//   - Only the FIRST noncreature spell each turn per opponent fires
//     the trigger. seat.Turn.Casts is consulted for the cast log; the
//     trigger only fires when this cast is the first noncreature one
//     for the casting seat this turn. RecordCast appends to Casts
//     BEFORE fireCastTriggers per stack.go:436-447, so the just-cast
//     spell is already in the log when we count.
//   - The tax X scales with Sentinel's power (counters can grow it).
//     Default 1; an opponent with X+ mana available greedily pays.
//
// Pay policy matches the engine's existing Rhystic-style heuristic:
// opponent pays iff they can afford X. Real cEDH play often refuses
// the tax to deny card advantage, but the engine doesn't yet have a
// per-trigger decision hook — match the established Study/Remora
// policy.
func registerEsperSentinel(r *Registry) {
	r.OnTrigger("Esper Sentinel", "noncreature_spell_cast", esperSentinelOnCast)
}

func esperSentinelOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "esper_sentinel"
	if gs == nil || perm == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster == perm.Controller {
		return // opponent-only
	}
	if caster < 0 || caster >= len(gs.Seats) {
		return
	}
	opp := gs.Seats[caster]
	if opp == nil {
		return
	}

	// "First noncreature spell each turn" gate. Count noncreature
	// casts in this seat's per-turn cast log. The cast that just
	// fired this trigger is already recorded (RecordCast runs before
	// fireCastTriggers), so first-spell means count == 1.
	nonCreatureCount := 0
	for _, rec := range opp.Turn.Casts {
		isCreature := false
		for _, t := range rec.Types {
			if t == "creature" {
				isCreature = true
				break
			}
		}
		if !isCreature {
			nonCreatureCount++
		}
	}
	if nonCreatureCount != 1 {
		return
	}

	x := perm.Power()
	if x < 0 {
		x = 0
	}

	// Greedy: pay if affordable.
	if x > 0 && opp.ManaPool >= x {
		opp.ManaPool -= x
		gameengine.SyncManaAfterSpend(opp)
		gs.LogEvent(gameengine.Event{
			Kind:   "pay_mana",
			Seat:   caster,
			Source: perm.Card.DisplayName(),
			Amount: x,
			Details: map[string]interface{}{
				"reason": "esper_sentinel_tax",
				"x":      x,
			},
		})
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"caster_seat": caster,
			"x":           x,
			"paid_tax":    true,
		})
		return
	}

	// Didn't pay (or X==0) — Sentinel's controller draws.
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster_seat": caster,
		"x":           x,
		"paid_tax":    false,
	})
}
