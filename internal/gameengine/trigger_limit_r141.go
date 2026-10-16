package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// Once-each-turn trigger limiter (r141).
//
// r139 filed this as blocked on "no chokepoint — 31 type-assertion sites
// across 18 files". That count was of the sites that RECOGNISE a
// *gameast.Triggered. The sites that FIRE one all reach the stack through
// pushTriggeredAbility, and always did. What was genuinely missing is the
// ability's identity: every caller passed trig.Effect and trig.InterveningIf
// rather than the node, so the chokepoint could not tell which ability had
// fired and therefore could not count per-ability. r141 threads the node
// through (PushTriggeredAbilityNode) and counts here.
//
// Identity is (permanent, ability node):
//
//   - The *Permanent pointer is the object. A permanent that leaves the
//     battlefield and returns is a NEW object, and its limiter resets with
//     it — which is the behaviour the rules want, and which an identity
//     keyed on card name would get wrong.
//   - The *gameast.Triggered pointer is the ability. The AST is shared per
//     card name, so two copies of the same card share the node — but they
//     are different Permanents, so their counts stay separate.
//
// The per-turn reset is SELF-INVALIDATING rather than a reset call. The
// engine's existing per-turn trigger counter (_trigger_fires_this_turn) is
// zeroed at internal/tournament/turn.go:146 — inside the tournament driver,
// not the engine — so a limiter keyed the same way would silently never
// reset under showmatch or paritycheck. Storing the turn the counts belong
// to and discarding them when gs.Turn moves on removes the dependency on
// any caller remembering to reset, and bounds the map to one turn's worth
// of entries.

type triggerLimitKey struct {
	perm *Permanent
	ab   *gameast.Triggered
}

// triggerLimitAllows reports whether this trigger may fire, and records the
// firing when it may. A nil node, or one carrying no once-per-turn
// restriction, is always allowed — the limiter never invents a limit.
func triggerLimitAllows(gs *GameState, src *Permanent, trig *gameast.Triggered) bool {
	if gs == nil || src == nil || trig == nil {
		return true
	}
	if trig.TimingRestriction != "once_per_turn" {
		return true
	}
	if gs.triggerLimitTurn != gs.Turn || gs.triggerLimitFired == nil {
		gs.triggerLimitTurn = gs.Turn
		gs.triggerLimitFired = map[triggerLimitKey]int{}
	}
	key := triggerLimitKey{perm: src, ab: trig}
	if gs.triggerLimitFired[key] > 0 {
		name := ""
		if src.Card != nil {
			name = src.Card.DisplayName()
		}
		gs.LogEvent(Event{
			Kind:   "trigger_suppressed",
			Seat:   src.Controller,
			Target: -1,
			Source: name,
			Details: map[string]interface{}{
				"reason":      "once_per_turn_limit",
				"restriction": "once_per_turn",
				"turn":        gs.Turn,
			},
		})
		return false
	}
	gs.triggerLimitFired[key]++
	return true
}
