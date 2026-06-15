package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// end_step_intervening_if_family.go — generic handler for the
// "At the beginning of [each|your] end step, if <condition>, <effect>"
// family, for gates that are NOT covered by the lifegain-specific
// scaffold in lifegain_endstep_family.go.
//
// The dispatch is identical across every member:
//   1. Receive end_step trigger.
//   2. Optionally restrict to controller's own end step.
//   3. Evaluate an intervening-if gate that reads existing engine
//      bookkeeping (Turn.SpellsCast, gs.ActiveSeat, etc).
//   4. Run the body closure.
//
// Hand-rolled siblings that need bespoke side-flag tracking (Lord Jyscal
// Guado tracks counter_placed events, Phoenix Fleet Airship tracks
// sacrifice events) keep their own implementations — this file owns the
// gap cards whose gate is satisfied by engine-maintained counters.

type endStepGate int

const (
	// "if you've cast a noncreature spell this turn"
	endStepGateCastNoncreatureThisTurn endStepGate = iota
	// "if it's not your turn" — used with each-end-step triggers.
	endStepGateNotMyTurn
)

// endStepIntervScope controls which seats' end steps the listener fires on.
type endStepIntervScope int

const (
	// "At the beginning of your end step" — only fires when active seat
	// equals the controller.
	endStepScopeYours endStepIntervScope = iota
	// "At the beginning of each end step" — fires on every end step.
	endStepScopeEach
)

type endStepInterveningEntry struct {
	cardName string
	scope    endStepIntervScope
	gate     endStepGate
	effect   func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{})
	partial  string // optional emitPartial reason if approximate
}

// Hand-rolled siblings that also match the shape (Hurkyl, Master Wizard:
// scope=yours, gate=cast_noncreature_this_turn, with per-perm-flag tracking
// of distinct types; Phoenix Fleet Airship: scope=yours, gate=sacrificed_
// this_turn; Lord Jyscal Guado: scope=each, gate=counter_placed_on_creature
// _this_turn; Feast of the Victorious Dead: scope=yours, gate=creature_died
// _this_turn) keep their bespoke handlers — the per-perm flag tracking they
// need can't be replaced by reading engine-maintained turn counters alone.
// The family owns gap cards whose gate is satisfied by counters this
// scaffold already understands (currently only not_my_turn).
var endStepInterveningEntries = []endStepInterveningEntry{
	{
		// Lighthouse Chronologist — {1}{U}, 1/3 Human Wizard.
		//   Level up {U}.
		//   LEVEL 4-6: 2/4
		//   LEVEL 7+: 3/5; at the beginning of each end step, if it's
		//   not your turn, take an extra turn after this one.
		// We fire the extra-turn body whenever the gate opens regardless
		// of level — leveling is a separate counter mechanic we don't
		// model (emitPartial). Engine's extra-turn flag is a global
		// counter that hands the extra turn to the ACTIVE PLAYER's next
		// turn, which collapses correctly here: the extra turn is queued
		// during an opponent's end step → next active turn is ours.
		cardName: "Lighthouse Chronologist",
		scope:    endStepScopeEach,
		gate:     endStepGateNotMyTurn,
		effect:   lighthouseChronologistExtraTurn,
		partial:  "level_up_gating_not_modeled_fires_at_any_level",
	},
}

func registerEndStepInterveningIfFamily(r *Registry) {
	for _, e := range endStepInterveningEntries {
		e := e
		r.OnTrigger(e.cardName, "end_step", func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			runEndStepIntervening(gs, perm, e, ctx)
		})
	}
}

func runEndStepIntervening(gs *gameengine.GameState, perm *gameengine.Permanent, e endStepInterveningEntry, ctx map[string]interface{}) {
	slug := "end_step_intervening_if_family:" + landFetchSlug(e.cardName)
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	switch e.scope {
	case endStepScopeYours:
		if activeSeat != perm.Controller {
			return
		}
	case endStepScopeEach:
		// Fire regardless of whose end step it is.
	}
	if !endStepGateOpen(gs, perm, e.gate, activeSeat) {
		emitFail(gs, slug, perm.Card.DisplayName(), endStepGateFailReason(e.gate), map[string]interface{}{
			"seat":        perm.Controller,
			"active_seat": activeSeat,
		})
		return
	}
	if e.effect != nil {
		e.effect(gs, perm, ctx)
	}
	_ = gs.CheckEnd()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"active_seat": activeSeat,
		"gate":        endStepGateName(e.gate),
	})
	if e.partial != "" {
		emitPartial(gs, slug, perm.Card.DisplayName(), e.partial)
	}
}

func endStepGateOpen(gs *gameengine.GameState, perm *gameengine.Permanent, gate endStepGate, activeSeat int) bool {
	switch gate {
	case endStepGateCastNoncreatureThisTurn:
		s := gs.Seats[perm.Controller]
		if s == nil {
			return false
		}
		for _, c := range s.Turn.Casts {
			isCreature := false
			for _, t := range c.Types {
				if t == "creature" {
					isCreature = true
					break
				}
			}
			if !isCreature {
				return true
			}
		}
		return false
	case endStepGateNotMyTurn:
		return activeSeat != perm.Controller
	}
	return false
}

func endStepGateFailReason(gate endStepGate) string {
	switch gate {
	case endStepGateCastNoncreatureThisTurn:
		return "no_noncreature_spell_cast_this_turn"
	case endStepGateNotMyTurn:
		return "own_end_step"
	}
	return "gate_closed"
}

func endStepGateName(gate endStepGate) string {
	switch gate {
	case endStepGateCastNoncreatureThisTurn:
		return "cast_noncreature_this_turn"
	case endStepGateNotMyTurn:
		return "not_my_turn"
	}
	return "unknown"
}

// ---------------------------------------------------------------------------
// Card bodies.
// ---------------------------------------------------------------------------

func lighthouseChronologistExtraTurn(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gameengine.GrantExtraTurn(gs, perm.Controller)
	gs.LogEvent(gameengine.Event{
		Kind:   "extra_turn",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug": "end_step_intervening_if_family:lighthouse_chronologist",
		},
	})
}

