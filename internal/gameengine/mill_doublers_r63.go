package gameengine

import "strings"

// Mill doublers — CR §614 replacement effects that change how many cards a mill
// puts into a graveyard (e.g. Bruvac the Grandiloquent: "If an opponent would
// mill one or more cards, they mill twice that many cards instead.").
//
// The parser emits Bruvac's clause as a generic `if_intervening_tail` node the
// engine never consumed, so mill doublers were UNWIRED — an opponent milled by
// a Bruvac controller milled the base amount, not double. This applies the
// doubler at the mill AMOUNT (CR: it replaces the NUMBER milled, so a mill of N
// becomes a mill of 2N — do 2N single-card mills, not N mills of 2 cards).

// isMillDoubler reports whether a card is a mill-amount doubler affecting its
// controller's opponents. Detected by name (a unique legendary; mirrors the
// engine's other name-keyed statics) since the parsed AST shape is only a
// generic if_intervening_tail.
func isMillDoubler(c *Card) bool {
	if c == nil {
		return false
	}
	return strings.EqualFold(c.Name, "Bruvac the Grandiloquent")
}

// millDoubleFactor returns the multiplier applied to a mill of `targetSeat`,
// accounting for every mill doubler on the battlefield. Bruvac doubles mills of
// its controller's OPPONENTS only (not the controller's own mills). Multiple
// doublers compound (each is a separate replacement: two Bruvacs → ×4).
func millDoubleFactor(gs *GameState, targetSeat int) int {
	if gs == nil || targetSeat < 0 || targetSeat >= len(gs.Seats) {
		return 1
	}
	factor := 1
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !isMillDoubler(p.Card) {
				continue
			}
			// Bruvac: doubles only the controller's opponents' mills.
			if targetSeat != p.Controller {
				factor *= 2
			}
		}
	}
	return factor
}
