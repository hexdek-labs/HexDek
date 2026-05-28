package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMercilessEviction wires Merciless Eviction.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Merciless%20Eviction):
//
//	Choose one —
//	  • Exile all artifacts.
//	  • Exile all creatures.
//	  • Exile all enchantments.
//	  • Exile all planeswalkers.
//
// {4}{W}{B} Sorcery. The modal mass-exile that hard-counters
// indestructible boards (Avacyn / Eldrazi titans / etched-Mondrak),
// hand-counters Heroic Intervention (exile beats indestructible),
// and answers planeswalker board states cleanly. The 6-mana cost
// keeps it out of the cEDH speed bracket, but in B4 / Bracket 4 it's
// premier WB attrition.
//
// Implementation:
//   - OnResolve. Mode picker: count opp permanents per type, pick
//     the type that exiles the most across all opps (the canonical
//     "do the most damage" policy). Tie-break: creatures > artifacts
//     > planeswalkers > enchantments (creatures usually represent
//     immediate damage; planeswalker exile is rarely needed).
//   - Mode may be overridden via item.CostMeta["eviction_mode"] when
//     the caster's hat pre-stamped a choice; otherwise use the
//     pick above.
//   - ExilePermanent through the canonical path so §614 would-be-
//     exiled replacements (Rest in Peace shifts irrelevant since
//     it's already exile, but §903.9b commander redirect applies),
//     LTB observers fire, and per_card detachment runs.
//   - Lands unaffected — printed types don't include "land."
func registerMercilessEviction(r *Registry) {
	r.OnResolve("Merciless Eviction", mercilessEvictionResolve)
}

func mercilessEvictionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "merciless_eviction"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Mode override from CostMeta, else pick from opp board census.
	mode := ""
	if item.CostMeta != nil {
		if v, ok := item.CostMeta["eviction_mode"]; ok {
			if s, ok := v.(string); ok {
				mode = s
			}
		}
	}
	if mode == "" {
		mode = pickMercilessEvictionMode(gs, seat)
	}

	// Build victim list per mode. Lands are excluded regardless.
	var victims []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsLand() {
				continue
			}
			match := false
			switch mode {
			case "creature":
				match = p.IsCreature()
			case "artifact":
				match = cardHasType(p.Card, "artifact")
			case "enchantment":
				match = cardHasType(p.Card, "enchantment")
			case "planeswalker":
				match = cardHasType(p.Card, "planeswalker")
			}
			if match {
				victims = append(victims, p)
			}
		}
	}

	exiled := 0
	for _, p := range victims {
		gameengine.ExilePermanent(gs, p, nil)
		exiled++
	}

	emit(gs, slug, "Merciless Eviction", map[string]interface{}{
		"seat":   seat,
		"mode":   mode,
		"exiled": exiled,
	})
}

// pickMercilessEvictionMode counts opp permanents by type and returns
// the mode that hits the most. Tie-break: creature > artifact >
// planeswalker > enchantment.
func pickMercilessEvictionMode(gs *gameengine.GameState, seat int) string {
	counts := map[string]int{}
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || p.IsLand() {
				continue
			}
			if p.IsCreature() {
				counts["creature"]++
			}
			if cardHasType(p.Card, "artifact") {
				counts["artifact"]++
			}
			if cardHasType(p.Card, "enchantment") {
				counts["enchantment"]++
			}
			if cardHasType(p.Card, "planeswalker") {
				counts["planeswalker"]++
			}
		}
	}
	best := "creature"
	bestN := -1
	// Tie-break order — first listed wins on ties.
	for _, m := range []string{"creature", "artifact", "planeswalker", "enchantment"} {
		if counts[m] > bestN {
			bestN = counts[m]
			best = m
		}
	}
	return best
}
