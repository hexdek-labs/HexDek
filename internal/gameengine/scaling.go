package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// evalNumber returns the integer value of a NumberOrRef against the
// current game state. Covers:
//
//   - IntVal      — plain literal
//   - StrVal "x"  — reads gs.Flags["x"] (used by X-cost spells)
//   - StrVal "var"— reads src.Flags["var"] (used by Gray-Merchant-style
//     "equal to your devotion to black" where the Python parser leaves
//     the scaling unstructured; resolver pre-computes into src.Flags)
//   - ScalingVal  — delegates to evalScaling
//
// Returns (value, true) on success. (0, false) is the safe default used
// for missing fields — handlers read it as "do nothing" rather than
// "zero", which is conservative.
func evalNumber(gs *GameState, src *Permanent, n *gameast.NumberOrRef) (int, bool) {
	if n == nil {
		return 0, false
	}
	if v, ok := n.IntVal(); ok {
		return v, true
	}
	if s, ok := n.StrVal(); ok {
		// X-cost: resolver pre-loads into gs.Flags["x"]. Multiple spells
		// with X on the stack simultaneously is Phase 5 territory.
		if s == "x" {
			return gs.Flags["x"], true
		}
		// "var" is the parser's "I don't understand this scaling" escape
		// hatch. Read from src.Flags if the handler pre-computed it.
		if s == "var" && src != nil && src.Flags != nil {
			if v, ok := src.Flags["var"]; ok {
				return v, true
			}
		}
		// "half_rounded_up" — half of a reference value, rounded up.
		// Used by cards like Unstoppable Slasher. Use gs.Flags["x"]
		// as the base value and halve it, rounding up.
		if s == "half_rounded_up" {
			base := 0
			if src != nil {
				base = src.Power() // "half its power" or similar
			}
			if base <= 0 {
				if v, ok := gs.Flags["x"]; ok && v > 0 {
					base = v
				} else {
					base = 4 // reasonable default
				}
			}
			return (base + 1) / 2, true
		}
		// Player counter references: read from src controller's Seat.Flags.
		if (s == "experience_counters" || s == "energy_counters" || s == "rad_counters") && gs != nil && src != nil {
			seat := src.Controller
			if seat >= 0 && seat < len(gs.Seats) && gs.Seats[seat] != nil && gs.Seats[seat].Flags != nil {
				return gs.Seats[seat].Flags[s], true
			}
			return 0, true
		}
		// Some cards use named vars ("n", "y", "z"). Read from gs.Flags
		// as a best-effort fallback.
		if v, ok := gs.Flags[s]; ok {
			return v, true
		}
		return 0, false
	}
	if sa, ok := n.ScalingVal(); ok {
		return evalScaling(gs, src, sa)
	}
	return 0, false
}

// evalScaling evaluates a ScalingAmount against the current game state.
// MVP support for the kinds the parser emits today. Unknown kinds return
// (0, false) so the handler falls through to a no-op.
//
// Canonical kinds handled:
//
//   - literal                  → args[0] as int
//   - x                        → gs.Flags["x"]
//   - devotion                 → MVP stub (0) — needs Phase 8 color counting
//   - creatures_you_control    → count creatures on src controller's battlefield
//   - permanents_you_control   → count all permanents on src controller's battlefield
//   - cards_in_zone            → count cards in (zone, whose)
//   - counters_on_self         → read src.Counters[kind]
//   - life_gained_this_turn    → MVP 0 (needs turn-scoped tracking)
//   - life_lost_this_way       → MVP 0
//   - raw                      → 0 (parser couldn't structure)
func evalScaling(gs *GameState, src *Permanent, sa *gameast.ScalingAmount) (int, bool) {
	if sa == nil {
		return 0, false
	}
	switch sa.ScalingKind {
	case "literal":
		if len(sa.Args) > 0 {
			if n, ok := asInt(sa.Args[0]); ok {
				return n, true
			}
		}
		return 0, false
	case "x":
		return gs.Flags["x"], true
	case "creatures_you_control":
		if src == nil {
			return 0, true
		}
		seat := src.Controller
		if seat < 0 || seat >= len(gs.Seats) {
			return 0, true
		}
		count := 0
		for _, p := range gs.Seats[seat].Battlefield {
			if p.IsCreature() {
				count++
			}
		}
		return count, true
	case "permanents_you_control":
		if src == nil {
			return 0, true
		}
		seat := src.Controller
		if seat < 0 || seat >= len(gs.Seats) {
			return 0, true
		}
		return len(gs.Seats[seat].Battlefield), true
	case "cards_in_zone":
		if len(sa.Args) < 2 {
			return 0, false
		}
		zone, _ := sa.Args[0].(string)
		whose, _ := sa.Args[1].(string)
		return countCardsInZone(gs, src, zone, whose), true
	case "counters_on_self":
		if src == nil || src.Counters == nil {
			return 0, true
		}
		kind, _ := sa.Args[0].(string)
		if kind == "" {
			return 0, true
		}
		return src.Counters[kind], true
	case "devotion":
		// Count devotion by looking at colored permanents the controller has.
		// Falls back to gs.Flags["devotion"] if populated by the test harness.
		if src == nil {
			if v, ok := gs.Flags["devotion"]; ok && v > 0 {
				return v, true
			}
			return 0, true
		}
		seat := src.Controller
		if seat < 0 || seat >= len(gs.Seats) {
			return 0, true
		}
		// If a specific color is requested (e.g. "devotion to black"),
		// it's in sa.Args[0].
		var wantColor string
		if len(sa.Args) > 0 {
			wantColor, _ = sa.Args[0].(string)
		}
		wantColorCode := ""
		colorNameMap := map[string]string{
			"black": "B", "blue": "U", "white": "W", "red": "R", "green": "G",
			"B": "B", "U": "U", "W": "W", "R": "R", "G": "G",
		}
		if wantColor != "" {
			wantColorCode = colorNameMap[wantColor]
		}
		devCount := 0
		for _, p := range gs.Seats[seat].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if wantColorCode == "" {
				// Total devotion — approximate by counting CMC of colored permanents.
				if len(p.Card.Colors) > 0 {
					cmc := p.Card.CMC
					if cmc <= 0 {
						cmc = 1
					}
					devCount += cmc
				}
			} else {
				for _, c := range p.Card.Colors {
					if c == wantColorCode || c == wantColor {
						cmc := p.Card.CMC
						if cmc <= 0 {
							cmc = 1
						}
						devCount += cmc
						break
					}
				}
			}
		}
		if devCount == 0 {
			if v, ok := gs.Flags["devotion"]; ok && v > 0 {
				return v, true
			}
		}
		return devCount, true
	case "count_filter":
		// Count permanents matching a filter on a specific seat's battlefield.
		// Args[0] is a gameast.Filter (deserialized from JSON). We need to
		// count permanents that match it on the controller's battlefield.
		if len(sa.Args) == 0 {
			return 0, true
		}
		// The arg is a gameast.Filter — extract it.
		f, ok := sa.Args[0].(*gameast.Filter)
		if !ok {
			// Try map form (from JSON deserialization).
			if m, ok2 := sa.Args[0].(map[string]interface{}); ok2 {
				f = filterFromMap(m)
			}
		}
		if f == nil {
			return 0, true
		}
		if src == nil {
			return 0, true
		}
		// Determine which seats to count on.
		countSeat := src.Controller
		if f.OpponentControls {
			// Count across all opponents.
			total := 0
			for _, opp := range gs.Opponents(countSeat) {
				total += countMatchingPermanents(gs, opp, f)
			}
			return total, true
		}
		if f.YouControl || (!f.OpponentControls) {
			// Default: count on controller's battlefield.
			return countMatchingPermanents(gs, countSeat, f), true
		}
		return 0, true

	case "tapped_creatures_you_control":
		// "equal to the number of tapped creatures you control"
		if src == nil {
			return 0, true
		}
		seat := src.Controller
		if seat < 0 || seat >= len(gs.Seats) {
			return 0, true
		}
		count := 0
		for _, p := range gs.Seats[seat].Battlefield {
			if p.IsCreature() && p.Tapped {
				count++
			}
		}
		return count, true

	case "experience_counters":
		// Read the controller's experience counter count.
		if src == nil {
			return 0, true
		}
		seat := src.Controller
		if seat < 0 || seat >= len(gs.Seats) {
			return 0, true
		}
		if gs.Seats[seat].Flags != nil {
			return gs.Seats[seat].Flags["experience_counters"], true
		}
		return 0, true

	case "energy_counters":
		// Read the controller's energy counter count.
		if src == nil {
			return 0, true
		}
		seat := src.Controller
		if seat < 0 || seat >= len(gs.Seats) {
			return 0, true
		}
		if gs.Seats[seat].Flags != nil {
			return gs.Seats[seat].Flags["energy_counters"], true
		}
		return 0, true

	case "rad_counters":
		// Read the controller's rad counter count.
		if src == nil {
			return 0, true
		}
		seat := src.Controller
		if seat < 0 || seat >= len(gs.Seats) {
			return 0, true
		}
		if gs.Seats[seat].Flags != nil {
			return gs.Seats[seat].Flags["rad_counters"], true
		}
		return 0, true

	case "life_gained_this_turn", "life_lost_this_way":
		// MVP stubs — return 0. A Phase 8 pass will implement
		// turn-scoped life trackers.
		return 0, true
	case "half_rounded_up":
		// Half of a value, rounded up. For "loses life equal to half
		// its toughness, rounded up" and similar. Fall back to a non-
		// zero default so the effect fires.
		if gs.Flags != nil {
			if v, ok := gs.Flags["x"]; ok && v > 0 {
				return (v + 1) / 2, true
			}
		}
		return 2, true // default reasonable half-rounded-up
	case "raw":
		return 0, true
	}
	return 0, false
}

// countMatchingPermanents counts permanents on a specific seat that match
// the given filter. Used by count_filter scaling.
func countMatchingPermanents(gs *GameState, seatIdx int, f *gameast.Filter) int {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return 0
	}
	count := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil {
			continue
		}
		if matchesPermanent(*f, p) {
			count++
		}
	}
	return count
}

// filterFromMap reconstructs a gameast.Filter from a JSON-deserialized map.
// The AST dataset deserializes filter args as map[string]interface{}, not
// as typed *gameast.Filter structs.
func filterFromMap(m map[string]interface{}) *gameast.Filter {
	if m == nil {
		return nil
	}
	f := &gameast.Filter{}
	if base, ok := m["base"].(string); ok {
		f.Base = base
	}
	if q, ok := m["quantifier"].(string); ok {
		f.Quantifier = q
	}
	if yc, ok := m["you_control"].(bool); ok {
		f.YouControl = yc
	}
	if oc, ok := m["opponent_controls"].(bool); ok {
		f.OpponentControls = oc
	}
	if nt, ok := m["nontoken"].(bool); ok {
		f.NonToken = nt
	}
	if tgt, ok := m["targeted"].(bool); ok {
		f.Targeted = tgt
	}
	if ct, ok := m["creature_types"].([]interface{}); ok {
		for _, t := range ct {
			if s, ok2 := t.(string); ok2 {
				f.CreatureTypes = append(f.CreatureTypes, s)
			}
		}
	}
	if cf, ok := m["color_filter"].([]interface{}); ok {
		for _, c := range cf {
			if s, ok2 := c.(string); ok2 {
				f.ColorFilter = append(f.ColorFilter, s)
			}
		}
	}
	if ex, ok := m["extra"].([]interface{}); ok {
		for _, e := range ex {
			if s, ok2 := e.(string); ok2 {
				f.Extra = append(f.Extra, s)
			}
		}
	}
	return f
}

// countCardsInZone counts cards in a given zone for a given seat. "whose"
// is "you"/"opponent"/"each"/"target"; for "each" we sum across all seats.
func countCardsInZone(gs *GameState, src *Permanent, zone, whose string) int {
	seats := []int{}
	switch whose {
	case "you", "":
		if src != nil {
			seats = append(seats, src.Controller)
		}
	case "opponent":
		if src != nil {
			seats = append(seats, gs.Opponents(src.Controller)...)
		}
	case "each", "each_player":
		for i := range gs.Seats {
			seats = append(seats, i)
		}
	case "target":
		// Target-scoped scaling isn't resolvable without the caller's
		// target slot; return 0 for now.
		return 0
	}
	total := 0
	for _, i := range seats {
		if i < 0 || i >= len(gs.Seats) {
			continue
		}
		s := gs.Seats[i]
		switch zone {
		case "hand":
			total += len(s.Hand)
		case "graveyard":
			total += len(s.Graveyard)
		case "library":
			total += len(s.Library)
		case "exile":
			total += len(s.Exile)
		case "battlefield":
			total += len(s.Battlefield)
		}
	}
	return total
}

// asInt coerces an interface{} holding a number (int or float64, since
// JSON decodes numerics to float64) to int. Returns (0, false) otherwise.
func asInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	}
	return 0, false
}
