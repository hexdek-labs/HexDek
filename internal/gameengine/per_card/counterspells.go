package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Counterspells — targeted stack interaction.
//
// Pattern: check target is on the stack + matches a filter (noncreature,
// instant, enchantment/instant/sorcery, etc.), mark as countered, apply
// side effects.
//
// The stock resolveCounterSpell in resolve.go marks the top of the stack
// as countered. These per_card handlers override that behavior to add
// target restrictions and side effects.
// ============================================================================

// --- Negate ---
//
// Oracle text:
//   Counter target noncreature spell.
//
// 1U instant.
func registerNegate(r *Registry) {
	r.OnResolve("Negate", negateResolve)
}

func negateResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "negate"
	if gs == nil || item == nil {
		return
	}

	target := findCounterableSpell(gs, item.Controller, func(si *gameengine.StackItem) bool {
		return !isCreatureSpell(si)
	})
	if target == nil {
		emitFail(gs, slug, "Negate", "no_noncreature_spell_on_stack", nil)
		return
	}

	target.Countered = true
	emitCounter(gs, slug, "Negate", item.Controller, target)
}

// --- Swan Song ---
//
// Oracle text:
//   Counter target enchantment, instant, or sorcery spell. Its controller
//   creates a 2/2 blue Bird creature token with flying.
//
// U instant.
func registerSwanSong(r *Registry) {
	r.OnResolve("Swan Song", swanSongResolve)
}

func swanSongResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "swan_song"
	if gs == nil || item == nil {
		return
	}

	target := findCounterableSpell(gs, item.Controller, func(si *gameengine.StackItem) bool {
		return isEnchantmentSpell(si) || isInstantSpell(si) || isSorcerySpell(si)
	})
	if target == nil {
		emitFail(gs, slug, "Swan Song", "no_valid_spell_on_stack", nil)
		return
	}

	targetController := target.Controller
	target.Countered = true

	// Controller creates a 2/2 blue Bird creature token with flying.
	if targetController >= 0 && targetController < len(gs.Seats) {
		bird := &gameengine.Card{
			Name:          "Bird",
			Types:         []string{"token", "creature", "bird"},
			Owner:         targetController,
			BasePower:     2,
			BaseToughness: 2,
		}
		enterBattlefieldWithETB(gs, targetController, bird, false)

		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   targetController,
			Source: "Swan Song",
			Details: map[string]interface{}{
				"token": "2/2 Bird with flying",
			},
		})
	}

	emitCounter(gs, slug, "Swan Song", item.Controller, target)
}

// --- Dovin's Veto ---
//
// Oracle text:
//   This spell can't be countered.
//   Counter target noncreature spell.
//
// WU instant.
func registerDovinsVeto(r *Registry) {
	r.OnCast("Dovin's Veto", dovinsVetoCast)
	r.OnResolve("Dovin's Veto", dovinsVetoResolve)
}

func dovinsVetoCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	// Mark the stack item as uncounterable via CostMeta. The priority
	// round doesn't check this yet (Phase 10+ concern), but the flag
	// provides the data for a future "can't be countered" check.
	if item != nil {
		if item.CostMeta == nil {
			item.CostMeta = map[string]interface{}{}
		}
		item.CostMeta["cannot_be_countered"] = true
	}
}

func dovinsVetoResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "dovins_veto"
	if gs == nil || item == nil {
		return
	}

	target := findCounterableSpell(gs, item.Controller, func(si *gameengine.StackItem) bool {
		return !isCreatureSpell(si)
	})
	if target == nil {
		emitFail(gs, slug, "Dovin's Veto", "no_noncreature_spell_on_stack", nil)
		return
	}

	target.Countered = true
	emitCounter(gs, slug, "Dovin's Veto", item.Controller, target)
}

// --- Arcane Denial ---
//
// Oracle text:
//   Counter target spell. Its controller may draw up to two cards at
//   the beginning of the next turn's upkeep.
//   You draw a card at the beginning of the next turn's upkeep.
//
// 1U instant. The "friendly" counter.
// MVP: immediate draw effects instead of delayed triggers for simplicity.
func registerArcaneDenial(r *Registry) {
	r.OnResolve("Arcane Denial", arcaneDenialResolve)
}

func arcaneDenialResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "arcane_denial"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller

	target := findCounterableSpell(gs, seat, nil)
	if target == nil {
		emitFail(gs, slug, "Arcane Denial", "no_spell_on_stack", nil)
		return
	}

	targetController := target.Controller
	target.Countered = true

	// Register delayed triggers for next upkeep draws.
	// Target's controller draws 2, caster draws 1.
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_upkeep",
		ControllerSeat: targetController,
		SourceCardName: "Arcane Denial",
		EffectFn: func(gs *gameengine.GameState) {
			if targetController < 0 || targetController >= len(gs.Seats) {
				return
			}
			s := gs.Seats[targetController]
			if s == nil || s.Lost {
				return
			}
			for i := 0; i < 2; i++ {
				if len(s.Library) > 0 {
					card := s.Library[0]
					gameengine.MoveCard(gs, card, targetController, "library", "hand", "draw")
				}
			}
			gs.LogEvent(gameengine.Event{
				Kind:   "draw",
				Seat:   targetController,
				Source: "Arcane Denial",
				Amount: 2,
			})
		},
	})
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_upkeep",
		ControllerSeat: seat,
		SourceCardName: "Arcane Denial",
		EffectFn: func(gs *gameengine.GameState) {
			if seat < 0 || seat >= len(gs.Seats) {
				return
			}
			s := gs.Seats[seat]
			if s == nil || s.Lost {
				return
			}
			if len(s.Library) > 0 {
				card := s.Library[0]
				gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
			}
			gs.LogEvent(gameengine.Event{
				Kind:   "draw",
				Seat:   seat,
				Source: "Arcane Denial",
				Amount: 1,
			})
		},
	})

	emitCounter(gs, slug, "Arcane Denial", seat, target)
}

// --- Dispel ---
//
// Oracle text:
//   Counter target instant spell.
//
// U instant.
func registerDispel(r *Registry) {
	r.OnResolve("Dispel", dispelResolve)
}

func dispelResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "dispel"
	if gs == nil || item == nil {
		return
	}

	target := findCounterableSpell(gs, item.Controller, func(si *gameengine.StackItem) bool {
		return isInstantSpell(si)
	})
	if target == nil {
		emitFail(gs, slug, "Dispel", "no_instant_on_stack", nil)
		return
	}

	target.Countered = true
	emitCounter(gs, slug, "Dispel", item.Controller, target)
}

// --- Mana Drain ---
//
// Oracle text:
//   Counter target spell. At the beginning of your next main phase,
//   add an amount of {C} equal to that spell's mana value.
//
// UU instant. Premium counterspell.
func registerManaDrain(r *Registry) {
	r.OnResolve("Mana Drain", manaDrainResolve)
}

func manaDrainResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "mana_drain"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller

	target := findCounterableSpell(gs, seat, nil)
	if target == nil {
		emitFail(gs, slug, "Mana Drain", "no_spell_on_stack", nil)
		return
	}

	// Capture the CMC before countering.
	cmc := 0
	if target.Card != nil {
		cmc = gameengine.ManaCostOf(target.Card)
	}

	target.Countered = true

	// Register delayed trigger: at next main phase, add colorless mana
	// equal to the countered spell's CMC.
	if cmc > 0 {
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "your_next_main_phase",
			ControllerSeat: seat,
			SourceCardName: "Mana Drain",
			EffectFn: func(gs *gameengine.GameState) {
				if seat < 0 || seat >= len(gs.Seats) {
					return
				}
				s := gs.Seats[seat]
				if s == nil || s.Lost {
					return
				}
				s.ManaPool += cmc
				gameengine.SyncManaAfterAdd(s, cmc)
				gs.LogEvent(gameengine.Event{
					Kind:   "add_mana",
					Seat:   seat,
					Source: "Mana Drain",
					Amount: cmc,
					Details: map[string]interface{}{
						"reason": "mana_drain_delayed",
					},
				})
			},
		})
	}

	emitCounter(gs, slug, "Mana Drain", seat, target)
}

// ============================================================================
// Shared counterspell helpers
// ============================================================================

// findCounterableSpell searches the stack (top-down) for an opponent's spell
// matching the given filter. If filter is nil, any opponent's spell matches.
// Returns nil if no legal target found.
func findCounterableSpell(gs *gameengine.GameState, casterSeat int, filter func(*gameengine.StackItem) bool) *gameengine.StackItem {
	if gs == nil {
		return nil
	}
	// Search from top of stack down.
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Countered {
			continue
		}
		// Must be an opponent's spell.
		if si.Controller == casterSeat {
			continue
		}
		// Apply filter.
		if filter != nil && !filter(si) {
			continue
		}
		return si
	}
	return nil
}

// isCreatureSpell returns true if the stack item represents a creature spell.
func isCreatureSpell(si *gameengine.StackItem) bool {
	if si == nil || si.Card == nil {
		return false
	}
	for _, t := range si.Card.Types {
		if strings.EqualFold(t, "creature") {
			return true
		}
	}
	return false
}

// isInstantSpell returns true if the stack item represents an instant spell.
func isInstantSpell(si *gameengine.StackItem) bool {
	if si == nil || si.Card == nil {
		return false
	}
	for _, t := range si.Card.Types {
		if strings.EqualFold(t, "instant") {
			return true
		}
	}
	return false
}

// isSorcerySpell returns true if the stack item represents a sorcery spell.
func isSorcerySpell(si *gameengine.StackItem) bool {
	if si == nil || si.Card == nil {
		return false
	}
	for _, t := range si.Card.Types {
		if strings.EqualFold(t, "sorcery") {
			return true
		}
	}
	return false
}

// isEnchantmentSpell returns true if the stack item represents an enchantment spell.
func isEnchantmentSpell(si *gameengine.StackItem) bool {
	if si == nil || si.Card == nil {
		return false
	}
	for _, t := range si.Card.Types {
		if strings.EqualFold(t, "enchantment") {
			return true
		}
	}
	return false
}

// emitCounter writes the standardized counter event.
func emitCounter(gs *gameengine.GameState, slug, cardName string, seat int, target *gameengine.StackItem) {
	targetName := ""
	if target != nil && target.Card != nil {
		targetName = target.Card.DisplayName()
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "counter_spell",
		Seat:   seat,
		Target: target.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"target_card": targetName,
			"target_id":   target.ID,
		},
	})
	emit(gs, slug, cardName, map[string]interface{}{
		"seat":        seat,
		"countered":   targetName,
		"target_seat": target.Controller,
	})
}
