package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAuntieOol wires Auntie Ool, Cursewretch.
//
// Oracle text:
//
//	Ward—Blight 2. (To blight 2, a player puts two -1/-1 counters on a
//	creature they control.)
//	Whenever one or more -1/-1 counters are put on a creature, draw a
//	card if you control that creature. If you don't control it, its
//	controller loses 1 life.
//
// Implementation:
//   - ETB: emitPartial for ward—blight 2 (a non-mana alternative ward
//     payment that itself places counters; engine ward grants only
//     model mana costs cleanly).
//   - counter_placed (custom engine event fired from resolveCounterMod
//     when "put" succeeds): if -1/-1 went on a creature, either Auntie's
//     controller draws (creature is theirs) or the creature's controller
//     loses 1 life.
//
// Note: this trigger is "one or more counters at once", which is a
// per-EVENT trigger rather than per-counter. resolveCounterMod fires
// counter_placed once per target with the aggregate amount, which
// matches the rules wording for AST-driven counter applications.
// Counter placements outside resolveCounterMod (combat infect/wither)
// don't currently fire counter_placed; emitPartial flags the gap.
func registerAuntieOol(r *Registry) {
	r.OnETB("Auntie Ool, Cursewretch", auntieOolETB)
	r.OnTrigger("Auntie Ool, Cursewretch", "counter_placed", auntieOolCounterPlaced)
}

func auntieOolETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "auntie_ool_ward_blight"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"ward_blight_2_alt_payment_unimplemented")
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"counter_placed_not_fired_from_combat_infect_wither")
}

func auntieOolCounterPlaced(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "auntie_ool_minus_counter_response"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	kind, _ := ctx["counter_kind"].(string)
	if kind != "-1/-1" {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		return
	}
	if !target.IsCreature() {
		return
	}
	targetSeat, _ := ctx["target_seat"].(int)
	if targetSeat == perm.Controller {
		drawOne(gs, perm.Controller, perm.Card.DisplayName())
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"target_card":   target.Card.DisplayName(),
			"effect":        "draw",
		})
		return
	}
	if targetSeat < 0 || targetSeat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[targetSeat]
	if s == nil || s.Lost {
		return
	}
	s.Life--
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   targetSeat,
		Target: targetSeat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason": "auntie_ool_counter_drain",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"target_card":  target.Card.DisplayName(),
		"target_seat":  targetSeat,
		"effect":       "drain",
	})
	_ = gs.CheckEnd()
}
