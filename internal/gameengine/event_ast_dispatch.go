package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// event_ast_dispatch.go — AST trigger dispatch for event families that
// previously fired per_card handlers ONLY (r63 PROGRESSION widening
// phase 3b). The aliases mapped them (gain_life → "life_gained",
// becomes_tapped → "tap_event") but FireCardTrigger reaches the
// per_card registry alone — no walk ever consulted the AST, so all 77
// corpus "whenever you gain life" triggers and all 82 "becomes tapped"
// triggers were silent unless a card had a bespoke handler.
//
// Same discipline as observer_raw_dispatch.go: the event name + the raw
// oracle wording carry the semantics (the parser drops actor phrases);
// unrecognized wordings fail closed; per_card-owned cards are skipped
// via HasTriggerHook so nothing double-fires.

// FireLifeGainedASTTriggers fires "whenever you gain life" /
// "whenever an opponent gains life" AST triggers after `seat` gained
// life. Called from GainLife alongside the per_card tap. Fires once per
// life-gain EVENT (CR §603.2: the whole gain is one event).
func FireLifeGainedASTTriggers(gs *GameState, seat int) {
	if gs == nil {
		return
	}
	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		perms := append([]*Permanent{}, s.Battlefield...)
		for _, p := range perms {
			if p == nil || p.Card == nil || p.Card.AST == nil {
				continue
			}
			for _, ab := range p.Card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}
				if !EventEquals(trig.Trigger.Event, "life_gained") {
					continue
				}
				raw := strings.ToLower(trig.Raw)
				switch {
				case strings.Contains(raw, "whenever you gain life,"):
					if p.Controller != seat {
						continue
					}
				case strings.Contains(raw, "whenever an opponent gains life,"):
					if p.Controller == seat {
						continue
					}
				default:
					// Threshold / first-time-each-turn / "that much"
					// riders with conditions fail closed.
					continue
				}
				if HasTriggerHook != nil && HasTriggerHook(p.Card.DisplayName(), "life_gained") {
					continue
				}
				gs.LogEvent(Event{
					Kind: "trigger_fires", Seat: p.Controller,
					Source: p.Card.DisplayName(),
					Details: map[string]interface{}{
						"event": "life_gained", "rule": "603.2",
					},
				})
				PushTriggeredAbilityWithIf(gs, p, trig.Effect, trig.InterveningIf)
			}
		}
	}
}

// FireTapEventASTTriggers fires becomes-tapped AST triggers for the
// permanent that just transitioned untapped → tapped: its own
// "this creature becomes tapped" triggers plus attached bearers'
// "enchanted/equipped creature becomes tapped". Called from every
// engine tap-transition site alongside the per_card tap_event.
func FireTapEventASTTriggers(gs *GameState, tapped *Permanent) {
	if gs == nil || tapped == nil || tapped.Card == nil {
		return
	}
	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))

	fire := func(bearer *Permanent, trig *gameast.Triggered) {
		if HasTriggerHook != nil && HasTriggerHook(bearer.Card.DisplayName(), "tap_event") {
			return
		}
		gs.LogEvent(Event{
			Kind: "trigger_fires", Seat: bearer.Controller,
			Source: bearer.Card.DisplayName(),
			Details: map[string]interface{}{
				"event": "becomes_tapped", "rule": "603.2",
			},
		})
		PushTriggeredAbilityWithIf(gs, bearer, trig.Effect, trig.InterveningIf)
	}

	// Self triggers on the tapped permanent.
	if tapped.Card.AST != nil {
		for _, ab := range tapped.Card.AST.Abilities {
			trig, ok := ab.(*gameast.Triggered)
			if !ok || trig.Effect == nil {
				continue
			}
			if !EventEquals(trig.Trigger.Event, "tap_event") {
				continue
			}
			raw := strings.ToLower(trig.Raw)
			if strings.Contains(raw, "this creature becomes tapped") ||
				strings.Contains(raw, "this land becomes tapped") ||
				strings.Contains(raw, "this artifact becomes tapped") ||
				strings.Contains(raw, "this permanent becomes tapped") ||
				strings.Contains(raw, "~ becomes tapped") {
				fire(tapped, trig)
			}
		}
	}

	// Attached bearers (auras/equipment on the tapped permanent).
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		perms := append([]*Permanent{}, s.Battlefield...)
		for _, p := range perms {
			if p == nil || p.Card == nil || p.Card.AST == nil || p.AttachedTo != tapped {
				continue
			}
			for _, ab := range p.Card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}
				if !EventEquals(trig.Trigger.Event, "tap_event") {
					continue
				}
				raw := strings.ToLower(trig.Raw)
				if strings.Contains(raw, "enchanted creature becomes tapped") ||
					strings.Contains(raw, "enchanted land becomes tapped") ||
					strings.Contains(raw, "enchanted permanent becomes tapped") ||
					strings.Contains(raw, "equipped creature becomes tapped") {
					fire(p, trig)
				}
			}
		}
	}
}
