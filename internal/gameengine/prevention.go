package gameengine

// Damage prevention framework — CR §615.
//
// Comp-rules citations:
//
//   §615.1  — "Some continuous effects are prevention effects.
//             Like replacement effects, prevention effects apply
//             continuously as events happen."
//   §615.7  — "If damage would be dealt to a player or permanent
//             by two or more applicable prevention effects, the
//             player or controller of the permanent chooses which
//             one to apply."
//   §702.16d — Protection prevents damage from sources of the
//              protected quality.
//
// This file provides:
//
//   - PreventionShield struct — "prevent the next N damage"
//   - PreventDamage(gs, target, amount, source) — returns actual damage
//   - AddPreventionShield / RemovePreventionShield
//
// Integration: called from applyDamage (resolve.go) and
// applyCombatDamageToPlayer / applyCombatDamageToCreature (combat.go).

import "strings"

// PreventionShield represents a "prevent the next N damage" effect.
// Stored on Seat.Flags or Permanent.Flags as a structured shield.
// For simplicity in the MVP, we use a slice on GameState.
type PreventionShield struct {
	// TargetSeat is the seat being shielded (-1 for permanent shields).
	TargetSeat int
	// TargetPerm is the permanent being shielded (nil for seat shields).
	TargetPerm *Permanent
	// Amount is the remaining damage to prevent. 0 or negative means
	// the shield is exhausted. -1 means "prevent all damage" (infinite).
	Amount int
	// SourceFilter restricts which sources are prevented. Empty means all.
	// Valid values: "all", a color code ("R", "U", etc.).
	SourceFilter string
	// SourceCard is the card name that created this shield (for logging).
	SourceCard string
	// OneShot is true for shields consumed after a single application
	// (e.g. "prevent the next 3 damage"). False for persistent shields
	// like "prevent all damage that would be dealt to you" (Teferi's
	// Protection).
	OneShot bool
	// Consumed is set after a one-shot shield is fully used.
	Consumed bool
}

// PreventDamageToPlayer checks all prevention shields and protection
// effects on the target seat. Returns the actual damage to deal after
// prevention. Modifies shields in place (decrementing amounts).
func PreventDamageToPlayer(gs *GameState, targetSeat int, amount int, src *Permanent) int {
	if gs == nil || amount <= 0 || targetSeat < 0 || targetSeat >= len(gs.Seats) {
		return amount
	}
	seat := gs.Seats[targetSeat]
	if seat == nil {
		return amount
	}

	// Check seat-level protection flags (Teferi's Protection, etc.).
	if seat.Flags != nil {
		// "protection_from_everything" already handled in combat.go's
		// applyCombatDamageToPlayer. For non-combat damage, check here too.
		if seat.Flags["protection_from_everything"] > 0 {
			gs.LogEvent(Event{
				Kind:   "damage_prevented",
				Seat:   targetSeat,
				Source: permanentName(src),
				Amount: amount,
				Details: map[string]interface{}{
					"reason": "protection_from_everything",
					"rule":   "702.16d",
				},
			})
			return 0
		}
		// "prevent_all_damage" — blanket prevention on player.
		if seat.Flags["prevent_all_damage"] > 0 {
			gs.LogEvent(Event{
				Kind:   "damage_prevented",
				Seat:   targetSeat,
				Source: permanentName(src),
				Amount: amount,
				Details: map[string]interface{}{
					"reason": "prevent_all_damage",
					"rule":   "615.1",
				},
			})
			return 0
		}
	}

	// Walk prevention shields on GameState.
	remaining := amount
	for i := range gs.PreventionShields {
		shield := &gs.PreventionShields[i]
		if shield.Consumed || remaining <= 0 {
			continue
		}
		// Must match target.
		if shield.TargetSeat != targetSeat || shield.TargetPerm != nil {
			continue
		}
		// Source filter check.
		if shield.SourceFilter != "" && shield.SourceFilter != "all" {
			if !sourceMatchesFilter(src, shield.SourceFilter) {
				continue
			}
		}
		// Apply prevention.
		if shield.Amount < 0 {
			// Infinite prevention — prevent all.
			prevented := remaining
			remaining = 0
			gs.LogEvent(Event{
				Kind:   "damage_prevented",
				Seat:   targetSeat,
				Source: permanentName(src),
				Amount: prevented,
				Details: map[string]interface{}{
					"shield_source": shield.SourceCard,
					"rule":          "615.1",
				},
			})
		} else {
			prevented := shield.Amount
			if prevented > remaining {
				prevented = remaining
			}
			shield.Amount -= prevented
			remaining -= prevented
			if shield.Amount <= 0 && shield.OneShot {
				shield.Consumed = true
			}
			if prevented > 0 {
				gs.LogEvent(Event{
					Kind:   "damage_prevented",
					Seat:   targetSeat,
					Source: permanentName(src),
					Amount: prevented,
					Details: map[string]interface{}{
						"shield_source":    shield.SourceCard,
						"shield_remaining": shield.Amount,
						"rule":             "615.1",
					},
				})
			}
		}
	}

	// Clean up consumed shields.
	cleanupShields(gs)

	return remaining
}

// PreventDamageToPermanent checks all prevention shields on the target
// permanent. Returns the actual damage to deal after prevention.
func PreventDamageToPermanent(gs *GameState, target *Permanent, amount int, src *Permanent) int {
	if gs == nil || target == nil || amount <= 0 {
		return amount
	}

	// §122.1b: Shield counter — "If this permanent would be dealt damage,
	// remove a shield counter from it instead."
	if target.Counters != nil && target.Counters["shield"] > 0 {
		target.Counters["shield"]--
		if target.Counters["shield"] <= 0 {
			delete(target.Counters, "shield")
		}
		gs.LogEvent(Event{
			Kind:   "shield_counter_consumed",
			Seat:   target.Controller,
			Source: permanentName(src),
			Amount: amount,
			Details: map[string]interface{}{
				"target_card":       target.Card.DisplayName(),
				"shields_remaining": target.Counters["shield"],
				"prevented":         "damage",
				"rule":              "122.1b",
			},
		})
		return 0
	}

	// Check permanent-level protection (protection from color/quality).
	// This is already partially handled in combat.go via
	// attackerHasProtectionFrom. For completeness, check here.
	if target.Flags != nil {
		if target.Flags["prevent_all_damage"] > 0 {
			gs.LogEvent(Event{
				Kind:   "damage_prevented",
				Seat:   target.Controller,
				Source: permanentName(src),
				Amount: amount,
				Details: map[string]interface{}{
					"target_card": target.Card.DisplayName(),
					"reason":      "prevent_all_damage",
					"rule":        "615.1",
				},
			})
			return 0
		}
	}

	// Walk prevention shields.
	remaining := amount
	for i := range gs.PreventionShields {
		shield := &gs.PreventionShields[i]
		if shield.Consumed || remaining <= 0 {
			continue
		}
		if shield.TargetPerm != target {
			continue
		}
		// Source filter check.
		if shield.SourceFilter != "" && shield.SourceFilter != "all" {
			if !sourceMatchesFilter(src, shield.SourceFilter) {
				continue
			}
		}
		if shield.Amount < 0 {
			prevented := remaining
			remaining = 0
			gs.LogEvent(Event{
				Kind:   "damage_prevented",
				Seat:   target.Controller,
				Source: permanentName(src),
				Amount: prevented,
				Details: map[string]interface{}{
					"target_card":   target.Card.DisplayName(),
					"shield_source": shield.SourceCard,
					"rule":          "615.1",
				},
			})
		} else {
			prevented := shield.Amount
			if prevented > remaining {
				prevented = remaining
			}
			shield.Amount -= prevented
			remaining -= prevented
			if shield.Amount <= 0 && shield.OneShot {
				shield.Consumed = true
			}
			if prevented > 0 {
				gs.LogEvent(Event{
					Kind:   "damage_prevented",
					Seat:   target.Controller,
					Source: permanentName(src),
					Amount: prevented,
					Details: map[string]interface{}{
						"target_card":      target.Card.DisplayName(),
						"shield_source":    shield.SourceCard,
						"shield_remaining": shield.Amount,
						"rule":             "615.1",
					},
				})
			}
		}
	}

	cleanupShields(gs)
	return remaining
}

// AddPreventionShield registers a new prevention shield on the game state.
func AddPreventionShield(gs *GameState, shield PreventionShield) {
	if gs == nil {
		return
	}
	gs.PreventionShields = append(gs.PreventionShields, shield)
	gs.LogEvent(Event{
		Kind:   "prevention_shield_added",
		Seat:   shield.TargetSeat,
		Source: shield.SourceCard,
		Amount: shield.Amount,
		Details: map[string]interface{}{
			"source_filter": shield.SourceFilter,
			"one_shot":      shield.OneShot,
			"rule":          "615.1",
		},
	})
}

// cleanupShields removes consumed shields.
func cleanupShields(gs *GameState) {
	if gs == nil || len(gs.PreventionShields) == 0 {
		return
	}
	kept := gs.PreventionShields[:0]
	for _, s := range gs.PreventionShields {
		if !s.Consumed {
			kept = append(kept, s)
		}
	}
	gs.PreventionShields = kept
}

// sourceMatchesFilter checks if a damage source matches a filter string.
func sourceMatchesFilter(src *Permanent, filter string) bool {
	if src == nil || filter == "" || filter == "all" {
		return true
	}
	// Color filter — check if the source's card has the specified color.
	color := strings.ToUpper(filter)
	if len(color) == 1 && (color == "W" || color == "U" || color == "B" || color == "R" || color == "G") {
		if src.Card != nil {
			return CardHasColor(src.Card, color)
		}
		return false
	}
	return true
}

// permanentName returns the display name of a permanent for logging.
func permanentName(src *Permanent) string {
	if src == nil || src.Card == nil {
		return "<unknown>"
	}
	return src.Card.DisplayName()
}
