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

// FireTurnedFaceUpTriggers fires "when this creature is turned face up,"
// AST triggers on the permanent that just transitioned face-down → face-up
// (CR §702.36e — morph; also megamorph / disguise / cloak / manifest /
// "turn ~ face up" effects, which all route through TurnFaceUp). Before
// r63d the engine flipped the face and logged the "turn_face_up" event but
// NEVER consulted these AST triggers — the same alias-to-nowhere shape as
// the gain_life / becomes_tapped families (the alias maps turned_face_up →
// "face_up", but no walk reached the AST), so all 109 corpus "is turned
// face up" triggers were silent. Driven from the single TurnFaceUp
// chokepoint so every face-up path is covered. per_card-owned cards are
// skipped via HasTriggerHook so nothing double-fires.
func FireTurnedFaceUpTriggers(gs *GameState, p *Permanent) {
	if gs == nil || p == nil || p.Card == nil || p.Card.AST == nil {
		return
	}
	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))
	for _, ab := range p.Card.AST.Abilities {
		trig, ok := ab.(*gameast.Triggered)
		if !ok || trig.Effect == nil {
			continue
		}
		if !EventEquals(trig.Trigger.Event, "face_up") {
			continue
		}
		raw := strings.ToLower(trig.Raw)
		// Self wordings only — "this creature/permanent is turned face up".
		// Observer forms ("whenever a permanent is turned face up") carry an
		// actor the parser drops; they stay out until carried.
		if !strings.Contains(raw, "this creature is turned face up") &&
			!strings.Contains(raw, "this permanent is turned face up") &&
			!strings.Contains(raw, "~ is turned face up") {
			continue
		}
		if HasTriggerHook != nil && HasTriggerHook(p.Card.DisplayName(), "face_up") {
			continue
		}
		gs.LogEvent(Event{
			Kind: "trigger_fires", Seat: p.Controller,
			Source: p.Card.DisplayName(),
			Details: map[string]interface{}{
				"event": "turned_face_up", "rule": "603.2",
			},
		})
		PushTriggeredAbilityWithIf(gs, p, trig.Effect, trig.InterveningIf)
	}
}

// FireDrawCardASTTriggers fires "whenever you draw a card," AST triggers
// after `drawerSeat` drew a card. Same alias-to-nowhere shape as the
// gain_life / becomes_tapped / turned_face_up families: the alias maps
// draw_card → "card_drawn", but the drawOne chokepoint only ever called
// FireCardTrigger (the per_card registry) — no walk consulted the AST, so
// every "whenever you draw a card" AST payoff (35 corpus shapes) was silent
// unless the card had a bespoke per_card handler. Fires once per draw EVENT.
// per_card-owned cards are skipped via HasTriggerHook so nothing double-fires.
func FireDrawCardASTTriggers(gs *GameState, drawerSeat int) {
	if gs == nil || drawerSeat < 0 || drawerSeat >= len(gs.Seats) {
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
				if !EventEquals(trig.Trigger.Event, "card_drawn") {
					continue
				}
				raw := strings.ToLower(trig.Raw)
				// CR §614.6 "except the first ... in each draw step" rider is
				// dropped by the parser (Orcish Bowmasters class) — fail closed.
				// "your second/third card each turn" is the distinct you_whenever
				// event, not this one; threshold / "that much" riders also out.
				if strings.Contains(raw, "except") || strings.Contains(raw, "that much") {
					continue
				}
				switch {
				case strings.Contains(raw, "whenever you draw a card,"):
					if p.Controller != drawerSeat {
						continue
					}
				case strings.Contains(raw, "whenever an opponent draws a card,"):
					if p.Controller == drawerSeat {
						continue
					}
				case strings.Contains(raw, "whenever a player draws a card,"):
					// any drawer — no gate
				default:
					continue
				}
				if HasTriggerHook != nil && HasTriggerHook(p.Card.DisplayName(), "card_drawn") {
					continue
				}
				gs.LogEvent(Event{
					Kind: "trigger_fires", Seat: p.Controller,
					Source: p.Card.DisplayName(),
					Details: map[string]interface{}{
						"event": "card_drawn", "rule": "603.2",
					},
				})
				PushTriggeredAbilityWithIf(gs, p, trig.Effect, trig.InterveningIf)
			}
		}
	}
}

// sacrificeFilterMatches reports whether a sacrificed victim satisfies the
// "whenever you sacrifice a <filter>" wording, returning (matched, ok) where
// ok=false means the wording is not a recognized bare self-sacrifice form.
func sacrificeFilterMatches(raw string, victim *Permanent, bearer *Permanent) (bool, bool) {
	// "another" excludes the bearer itself as the victim.
	another := strings.Contains(raw, "sacrifice another")
	if another && victim == bearer {
		return false, true
	}
	switch {
	case strings.Contains(raw, "sacrifice a creature,"), strings.Contains(raw, "sacrifice another creature,"):
		return victim.IsCreature(), true
	case strings.Contains(raw, "sacrifice an artifact,"), strings.Contains(raw, "sacrifice another artifact,"):
		return cardHasType(victim.Card, "artifact"), true
	case strings.Contains(raw, "sacrifice an enchantment,"), strings.Contains(raw, "sacrifice another enchantment,"):
		return cardHasType(victim.Card, "enchantment"), true
	case strings.Contains(raw, "sacrifice a land,"), strings.Contains(raw, "sacrifice another land,"):
		return cardHasType(victim.Card, "land"), true
	case strings.Contains(raw, "sacrifice a permanent,"), strings.Contains(raw, "sacrifice another permanent,"):
		return true, true
	}
	return false, false
}

// FireSacrificeASTTriggers fires "whenever you sacrifice a <filter>," AST
// triggers after `sacSeat` sacrificed `victim`. Same alias-to-nowhere shape
// as the draw/gain_life families: sacrificePermanentImpl fired only the
// per_card registry (artifact_sacrificed / creature_sacrificed /
// permanent_sacrificed), so the parsed "whenever you sacrifice a X" AST
// payoffs (70 corpus shapes) were silent unless a card had a bespoke
// handler. Controller-gated ("you sacrifice" = the victim's controller);
// per_card-owned cards are skipped via HasTriggerHook.
func FireSacrificeASTTriggers(gs *GameState, sacSeat int, victim *Permanent) {
	if gs == nil || victim == nil || victim.Card == nil {
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
			// "you sacrifice" — the bearer's controller must be the sacrificer.
			if p.Controller != sacSeat {
				continue
			}
			for _, ab := range p.Card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}
				if !EventEquals(trig.Trigger.Event, "sacrifice") {
					continue
				}
				raw := strings.ToLower(trig.Raw)
				if !strings.Contains(raw, "whenever you sacrifice") {
					continue
				}
				matched, recognized := sacrificeFilterMatches(raw, victim, p)
				if !recognized || !matched {
					continue
				}
				if HasTriggerHook != nil && HasTriggerHook(p.Card.DisplayName(), "sacrifice") {
					continue
				}
				gs.LogEvent(Event{
					Kind: "trigger_fires", Seat: p.Controller,
					Source: p.Card.DisplayName(),
					Details: map[string]interface{}{
						"event": "sacrifice", "rule": "603.2",
					},
				})
				PushTriggeredAbilityWithIf(gs, p, trig.Effect, trig.InterveningIf)
			}
		}
	}
}

// FireDiscardASTTriggers fires "whenever you discard a card," AST triggers
// after `discarderSeat` discarded a card. Same alias-to-nowhere shape as the
// draw family: DiscardCard fired card_discarded only through the per_card
// registry, so parsed discard payoffs (27 corpus shapes) were silent unless
// a card had a bespoke handler. Controller-gated; per_card HasTriggerHook
// guard prevents double-fire.
func FireDiscardASTTriggers(gs *GameState, discarderSeat int) {
	if gs == nil || discarderSeat < 0 || discarderSeat >= len(gs.Seats) {
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
				if !EventEquals(trig.Trigger.Event, "discard") {
					continue
				}
				raw := strings.ToLower(trig.Raw)
				if strings.Contains(raw, "that much") {
					continue
				}
				switch {
				case strings.Contains(raw, "whenever you discard a card,"):
					if p.Controller != discarderSeat {
						continue
					}
				case strings.Contains(raw, "whenever an opponent discards a card,"):
					if p.Controller == discarderSeat {
						continue
					}
				case strings.Contains(raw, "whenever a player discards a card,"):
					// any discarder
				default:
					continue
				}
				if HasTriggerHook != nil && HasTriggerHook(p.Card.DisplayName(), "card_discarded") {
					continue
				}
				gs.LogEvent(Event{
					Kind: "trigger_fires", Seat: p.Controller,
					Source: p.Card.DisplayName(),
					Details: map[string]interface{}{
						"event": "card_discarded", "rule": "603.2",
					},
				})
				PushTriggeredAbilityWithIf(gs, p, trig.Effect, trig.InterveningIf)
			}
		}
	}
}

// FireCommitCrimeASTTriggers fires "whenever you commit a crime" AST
// triggers for the seat that just committed a crime (CR §701.71). Same
// per_card-only gap the dispatch-widening closed for life-gain / tap /
// draw / sacrifice / discard: FireCommitsCrimeTriggers reaches the
// per_card registry alone (FireCardTrigger "crime"), so the ~18 corpus
// "whenever you commit a crime" AST triggers (Hardbristle Bandit, Marauding
// Sphinx, Vadmir, Bandit's Haul, Rattleback Apothecary, …) were silent
// unless a card had a bespoke handler — the crime DETECTION fired and bumped
// the per-turn counter, but the triggered EFFECT never resolved.
//
// Fires once per crime EVENT (CR §701.71b — one crime per qualifying
// spell/ability regardless of how many opponent objects it targets; the
// caller already deduped). per_card-owned commit_crime handlers (Gisa) are
// skipped via HasTriggerHook so nothing double-fires.
func FireCommitCrimeASTTriggers(gs *GameState, seat int) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))
	// "you commit a crime" — only the crime-committing player's own
	// permanents have the trigger.
	s := gs.Seats[seat]
	if s == nil {
		return
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
			if !EventEquals(trig.Trigger.Event, "commit_crime") {
				continue
			}
			raw := strings.ToLower(trig.Raw)
			if !strings.Contains(raw, "commit a crime") {
				// Fail closed on unrecognized wording (e.g. opponent-crime
				// variants carry their own event/handler).
				continue
			}
			// "during your turn" rider (Overzealous Muscle): only fires on
			// the controller's own turn.
			if strings.Contains(raw, "during your turn") && gs.Active != p.Controller {
				continue
			}
			if HasTriggerHook != nil && HasTriggerHook(p.Card.DisplayName(), "commit_crime") {
				continue
			}
			gs.LogEvent(Event{
				Kind: "trigger_fires", Seat: p.Controller,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"event": "commit_crime", "rule": "701.71b",
				},
			})
			PushTriggeredAbilityWithIf(gs, p, trig.Effect, trig.InterveningIf)
		}
	}
}
