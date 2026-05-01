package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOjerAxonil wires Ojer Axonil, Deepest Might // Temple of Power.
//
// Front face — Ojer Axonil, Deepest Might:
//
//	Trample
//	If a red source you control would deal an amount of noncombat damage
//	less than Ojer Axonil's power to an opponent, it deals damage equal
//	to Ojer Axonil's power instead.
//	When Ojer Axonil dies, return it to the battlefield tapped and
//	transformed under its owner's control.
//
// Back face — Temple of Power (land):
//
//	{T}: Add {R}.
//	{2}{R}, {T}: Transform Temple of Power. Activate only if you've
//	dealt 4 or more noncombat damage this turn.
//
// The damage-replacement clause is the central engine: it scales every
// red ping (Guttersnipe, Impact Tremors, Bonecrusher Giant adventures,
// even Lightning Bolt) up to Ojer's power. We register a §614
// `would_be_dealt_damage` replacement on ETB. Combat damage flows
// through applyCombatDamageToPlayer/Creature directly without firing
// FireDamageEvent, so the noncombat-only filter is implicit.
//
// The death-transform clause and Temple-of-Power back face activations
// are flagged via emitPartial — the per_card framework doesn't yet have
// hooks for self-die replacements (would need to hook into the AST
// self-die path) or activated-only-after-damage-this-turn conditions.
func registerOjerAxonil(r *Registry) {
	r.OnETB("Ojer Axonil, Deepest Might", ojerAxonilETB)
}

func ojerAxonilETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ojer_axonil_noncombat_damage_floor"
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_be_dealt_damage",
		HandlerID:      "Ojer Axonil, Deepest Might:noncombat_damage_floor:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.Source == nil || ev.Source.Card == nil {
				return false
			}
			// Source must be controlled by Ojer's controller.
			if ev.Source.Controller != controller {
				return false
			}
			// Source must be red.
			if !gameengine.CardHasColor(ev.Source.Card, "R") {
				return false
			}
			// Target must be an opponent (player target only — permanent
			// damage is not what the oracle floors, since it specifies
			// "to an opponent").
			if ev.TargetPerm != nil {
				return false
			}
			if ev.TargetSeat == controller || ev.TargetSeat < 0 || ev.TargetSeat >= len(gs.Seats) {
				return false
			}
			// Only floor amounts strictly less than Ojer's power.
			power := perm.Power()
			if power <= 0 {
				return false
			}
			return ev.Count() < power
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			power := perm.Power()
			before := ev.Count()
			ev.SetCount(power)
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Target: ev.TargetSeat,
				Source: "Ojer Axonil, Deepest Might",
				Amount: power,
				Details: map[string]interface{}{
					"slug":   slug,
					"rule":   "614",
					"effect": "noncombat_damage_floor",
					"before": before,
					"after":  power,
				},
			})
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           controller,
		"power":          perm.Power(),
		"replaces":       "would_be_dealt_damage",
		"scope":          "red_source_controller_to_opponent_noncombat",
	})

	// Death-transform clause and Temple-of-Power back-face activations
	// are not wired: the per_card framework lacks a self-die replacement
	// hook (would need AST self-trigger integration), and the activated
	// "only if you've dealt 4+ noncombat damage this turn" gate isn't
	// trackable without a per-turn damage-by-color counter.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"death_return_transformed_and_temple_of_power_back_face_unimplemented")
}

