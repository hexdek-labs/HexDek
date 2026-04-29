package gameengine

// Generic counterspell resolver — replaces 200+ potential per-card handlers
// with a single AST-driven dispatch function. Reads the parser's CounterSpell
// node fields (target filter, unless-pay cost) and resolves them against the
// current game state.
//
// Architecture:
//
//   ResolveCounterSpellGeneric is called from resolve.go when the effect is a
//   *gameast.CounterSpell AND no per-card snowflake handler exists for the
//   card. Cards that DO have per-card handlers (Swan Song, Arcane Denial,
//   Mana Drain) continue to use those handlers for side-effect correctness;
//   the generic resolver handles the ~200 remaining counterspells that are
//   pure "counter target [filter] spell" with no exotic side effects.
//
// Filter matching:
//
//   The AST parser emits a Filter on the CounterSpell.Target field. The
//   Filter.Base discriminates the primary type:
//
//     "spell"     → any spell on the stack
//     "creature"  → creature spell
//     "instant"   → instant spell
//     "sorcery"   → sorcery spell
//     "artifact"  → artifact spell
//     "enchantment" → enchantment spell
//     "planeswalker" → planeswalker spell (rare)
//     "thing"     → any (parser fallback for "target spell" without a type)
//     "activated" / "activated_ability" → activated ability on stack
//     "triggered" → triggered ability on stack
//     "abilities" → all abilities (Kadena's Silencer)
//
//   The Filter.Extra slice carries adjectives like "non-creature" (Negate,
//   Dovin's Veto), "colorless" (Ceremonious Rejection), "multicolored"
//   (Neutralizing Blast, Trial // Error), "non-artifact" (Revolutionary
//   Rebuff).
//
//   The Filter.ColorFilter slice carries color restrictions (e.g. "black"
//   for Lifeforce, "blue" for certain color-hosers).

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ResolveCounterSpellGeneric is the engine-level generic counterspell
// resolver. It reads the CounterSpell AST node's target filter to find a
// legal target on the stack, handles the "unless pay" cost (if present),
// marks the target as countered, and moves it to the appropriate zone.
//
// Returns true if the counter succeeded (target found and countered),
// false if no legal target was found or the counter was blocked by
// "can't be countered".
//
// The caller (resolve.go) is responsible for checking whether a per-card
// snowflake handler exists before calling this function.
func ResolveCounterSpellGeneric(gs *GameState, casterSeat int, cs *gameast.CounterSpell) bool {
	if gs == nil || cs == nil {
		return false
	}

	// --- 1. Find target on the stack ---
	target := findGenericCounterTarget(gs, casterSeat, cs.Target)
	if target == nil {
		gs.LogEvent(Event{
			Kind:   "counter_spell_fizzle",
			Seat:   casterSeat,
			Source: "generic_counter",
			Details: map[string]interface{}{
				"reason":      "no_legal_target",
				"filter_base": cs.Target.Base,
			},
		})
		return false
	}

	// --- 2. Check "can't be countered" ---
	if spellCannotBeCountered(target) {
		gs.LogEvent(Event{
			Kind:   "counter_spell_blocked",
			Seat:   casterSeat,
			Target: target.Controller,
			Source: "generic_counter",
			Details: map[string]interface{}{
				"target_card": stackItemName(target),
				"reason":      "cannot_be_countered",
			},
		})
		return false
	}

	// --- 3. Handle "unless pay" condition ---
	if cs.Unless != nil && cs.Unless.Mana != nil {
		unlessCost := cs.Unless.Mana.CMC()
		if unlessCost > 0 && canPayUnlessCost(gs, target.Controller, unlessCost) {
			// Target's controller pays the cost.
			payUnlessCost(gs, target.Controller, unlessCost)
			gs.LogEvent(Event{
				Kind:   "counter_spell_paid",
				Seat:   target.Controller,
				Source: "generic_counter",
				Details: map[string]interface{}{
					"paid_amount": unlessCost,
					"target_card": stackItemName(target),
				},
			})
			return false
		}
	}

	// --- 4. Counter the spell ---
	target.Countered = true

	// Log the counter event.
	gs.LogEvent(Event{
		Kind:   "counter_spell",
		Seat:   casterSeat,
		Target: target.Controller,
		Source: "generic_counter",
		Details: map[string]interface{}{
			"target_card": stackItemName(target),
			"target_id":   target.ID,
			"filter_base": cs.Target.Base,
		},
	})

	return true
}

// findGenericCounterTarget searches the stack (top-down) for a spell or
// ability matching the CounterSpell's target filter. Only opponents' items
// are considered (you can't counter your own spells with a targeted counter).
func findGenericCounterTarget(gs *GameState, casterSeat int, filter gameast.Filter) *StackItem {
	if gs == nil {
		return nil
	}
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Countered {
			continue
		}
		// Must be an opponent's item.
		if si.Controller == casterSeat {
			continue
		}
		if matchesCounterFilter(si, filter) {
			return si
		}
	}
	return nil
}

// matchesCounterFilter checks whether a stack item matches a CounterSpell's
// target filter. The filter's Base field is the primary discriminator; the
// Extra slice carries adjectives.
func matchesCounterFilter(si *StackItem, filter gameast.Filter) bool {
	if si == nil {
		return false
	}

	base := strings.ToLower(filter.Base)

	// --- Ability filters (Stifle, Trickbind, etc.) ---
	switch base {
	case "activated", "activated_ability":
		return si.Kind == "activated"
	case "triggered":
		return si.Kind == "triggered"
	case "abilities":
		return si.Kind == "activated" || si.Kind == "triggered"
	}

	// --- Spell filters: the item must be a spell (not a triggered/activated ability) ---
	if !isStackItemSpell(si) {
		return false
	}

	// Check extras first for negative filters.
	for _, ex := range filter.Extra {
		exLow := strings.ToLower(ex)
		switch exLow {
		case "non-creature", "noncreature":
			if stackItemHasType(si, "creature") {
				return false
			}
		case "non-artifact", "nonartifact":
			if stackItemHasType(si, "artifact") {
				return false
			}
		case "non-land", "nonland":
			if stackItemHasType(si, "land") {
				return false
			}
		case "colorless":
			if si.Card != nil && len(si.Card.Colors) > 0 {
				return false
			}
		case "multicolored":
			if si.Card == nil || len(si.Card.Colors) < 2 {
				return false
			}
		}
	}

	// Check color filters.
	if len(filter.ColorFilter) > 0 && si.Card != nil {
		matchesColor := false
		for _, filterColor := range filter.ColorFilter {
			for _, cardColor := range si.Card.Colors {
				if strings.EqualFold(filterColor, cardColor) {
					matchesColor = true
					break
				}
			}
			if matchesColor {
				break
			}
		}
		if !matchesColor {
			return false
		}
	}

	// Check color exclusions.
	if len(filter.ColorExclude) > 0 && si.Card != nil {
		for _, excColor := range filter.ColorExclude {
			for _, cardColor := range si.Card.Colors {
				if strings.EqualFold(excColor, cardColor) {
					return false
				}
			}
		}
	}

	// Check base type filter.
	switch base {
	case "spell", "thing", "":
		// Any spell matches.
		return true
	case "creature":
		return stackItemHasType(si, "creature")
	case "instant":
		return stackItemHasType(si, "instant")
	case "sorcery":
		return stackItemHasType(si, "sorcery")
	case "artifact":
		return stackItemHasType(si, "artifact")
	case "enchantment":
		return stackItemHasType(si, "enchantment")
	case "planeswalker":
		return stackItemHasType(si, "planeswalker")
	case "battle":
		return stackItemHasType(si, "battle")
	case "non", "other", "or":
		// Parser fallback: "non" / "other" / "or" are edge-case base values.
		// Treat as "any spell" since the Extra slice carries the restriction.
		return true
	}

	// Color-based base values (e.g. "black" for Lifeforce).
	colorBases := map[string]string{
		"black": "B", "blue": "U", "white": "W",
		"red": "R", "green": "G",
	}
	if colorCode, ok := colorBases[base]; ok {
		if si.Card == nil {
			return false
		}
		for _, c := range si.Card.Colors {
			if strings.EqualFold(c, colorCode) || strings.EqualFold(c, base) {
				return true
			}
		}
		return false
	}

	// Tribal / creature-type based base (e.g. "spirit" for specific counters).
	if si.Card != nil {
		for _, t := range si.Card.Types {
			if strings.EqualFold(t, base) {
				return true
			}
		}
	}

	// Fallback: if we don't recognize the base, treat as "any spell".
	// This is conservative — better to counter too broadly than to miss.
	return true
}

// isStackItemSpell returns true if the stack item represents a spell
// (as opposed to a triggered or activated ability).
func isStackItemSpell(si *StackItem) bool {
	if si == nil {
		return false
	}
	// Explicit kind discrimination.
	switch si.Kind {
	case "activated", "triggered":
		return false
	case "spell":
		return true
	}
	// Legacy inference: Source != nil → triggered/activated ability.
	// Card != nil && Source == nil → spell.
	// Card == nil && Source == nil → bare test fixture, treat as spell
	// for backward compatibility.
	if si.Source != nil {
		return false
	}
	return true
}

// stackItemHasType checks if a stack item's card has a given type
// (case-insensitive). Checks Card.Types, Card.TypeLine, and Card.AST.
func stackItemHasType(si *StackItem, typeName string) bool {
	if si == nil || si.Card == nil {
		return false
	}
	want := strings.ToLower(typeName)
	for _, t := range si.Card.Types {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	// Fallback: check TypeLine string (e.g. "Legendary Creature — Elf Druid").
	if si.Card.TypeLine != "" {
		if strings.Contains(strings.ToLower(si.Card.TypeLine), want) {
			return true
		}
	}
	return false
}

// stackItemName returns the display name of a stack item's card (or "ability").
func stackItemName(si *StackItem) string {
	if si == nil {
		return ""
	}
	if si.Card != nil {
		return si.Card.DisplayName()
	}
	if si.Source != nil && si.Source.Card != nil {
		return si.Source.Card.DisplayName() + " (ability)"
	}
	return "unknown"
}

// spellCannotBeCountered checks if a stack item has the "can't be countered"
// flag. This can come from:
//   - CostMeta["cannot_be_countered"] (set by per-card cast hooks, e.g.
//     Dovin's Veto, Abrupt Decay)
//   - Card types containing "uncounterable" (test convention)
//   - AST containing a "this spell can't be countered" static
func spellCannotBeCountered(si *StackItem) bool {
	if si == nil {
		return false
	}
	// Check CostMeta flag (most reliable — set by cast hooks).
	if si.CostMeta != nil {
		if v, ok := si.CostMeta["cannot_be_countered"]; ok {
			if b, ok := v.(bool); ok && b {
				return true
			}
		}
	}
	// Check card types for test convention.
	if si.Card != nil {
		for _, t := range si.Card.Types {
			if strings.EqualFold(t, "uncounterable") {
				return true
			}
		}
	}
	// Check AST for "parsed_tail" modification with "can't be countered".
	if si.Card != nil && si.Card.AST != nil {
		for _, ab := range si.Card.AST.Abilities {
			s, ok := ab.(*gameast.Static)
			if !ok || s.Modification == nil {
				continue
			}
			if s.Modification.ModKind == "parsed_tail" && len(s.Modification.Args) > 0 {
				if text, ok := s.Modification.Args[0].(string); ok {
					if strings.Contains(strings.ToLower(text), "can't be countered") {
						return true
					}
				}
			}
		}
	}
	return false
}

// CounterCanTarget checks whether a counterspell's filter allows it to
// target a given stack item. Exported for use by Hat implementations
// (ChooseResponse needs to verify that a counterspell in hand can actually
// counter the spell on the stack before committing to cast it).
func CounterCanTarget(counter gameast.Effect, target *StackItem) bool {
	cs := ExtractCounterSpellNode(counter)
	if cs == nil || target == nil {
		return false
	}
	return matchesCounterFilter(target, cs.Target)
}

// canPayUnlessCost returns true if the target's controller has enough mana
// to pay the "unless" cost. In the current MVP, this uses the Hat's
// decision-making: the greedy policy always declines to pay (conservative
// assumption that maximizes counterspell effectiveness in simulation).
// A future enhancement would query the Hat for a pay/don't-pay decision.
func canPayUnlessCost(gs *GameState, controllerSeat int, amount int) bool {
	if gs == nil || controllerSeat < 0 || controllerSeat >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[controllerSeat]
	if seat == nil {
		return false
	}
	// MVP: check if they CAN pay (have enough mana). The Hat decides
	// whether they SHOULD pay. For greedy baseline: always pay if able
	// (it's almost always correct to pay to save your spell).
	return seat.ManaPool >= amount
}

// payUnlessCost deducts the unless-pay cost from the controller's mana pool.
func payUnlessCost(gs *GameState, controllerSeat int, amount int) {
	if gs == nil || controllerSeat < 0 || controllerSeat >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[controllerSeat]
	if seat == nil {
		return
	}
	seat.ManaPool -= amount
	if seat.ManaPool < 0 {
		seat.ManaPool = 0
	}
	SyncManaAfterSpend(seat)
}
