package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHenzieToolboxTorre wires Henzie "Toolbox" Torre.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Each creature spell you cast with mana value 4 or greater has blitz.
//	The blitz cost is equal to its mana cost. (You may choose to cast
//	that spell for its blitz cost. If you do, it gains haste and "When
//	this creature dies, draw a card." Sacrifice it at the beginning of
//	the next end step.)
//	Blitz costs you pay cost {1} less for each time you've cast your
//	commander from the command zone this game.
//
// Implementation:
//   - Cost reduction is registered in cost_modifiers.go by name. Each
//     time Henzie's controller casts a creature spell with MV ≥ 4, the
//     mod scanner reduces the cost by the sum of CommanderCastCounts
//     across all of that seat's commanders. When no commanders have been
//     cast yet, this is 0 (blitz cost == mana cost).
//   - "permanent_etb": when a creature with MV ≥ 4 enters under Henzie's
//     controller, AND the controller has cast a commander from the
//     command zone at least once this game (i.e. blitz is strictly
//     cheaper than the printed cost), call ApplyBlitz. This grants
//     haste, registers the EOT sacrifice delayed trigger, and registers
//     the dies-draw delayed trigger. Tokens are skipped (you don't
//     "cast" a token).
//   - When commander cast count is 0, blitz cost is identical to the
//     printed cost; the AI prefers the normal cast (no EOT sacrifice),
//     so we skip ApplyBlitz in that case.
//
// Caveat: blitz is technically a player choice per CR §702.152. We
// model it as automatic-when-discounted because the AI has no UI for
// "would you like to blitz?" and discounted blitz is strictly better
// than the printed cast (same mana, +haste, +dies-draw, -EOT).
func registerHenzieToolboxTorre(r *Registry) {
	r.OnTrigger("Henzie \"Toolbox\" Torre", "permanent_etb", henzieToolboxTorreETBObserver)
}

func henzieToolboxTorreETBObserver(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "henzie_toolbox_torre_blitz"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering == perm || entering.Card == nil {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if !entering.IsCreature() {
		return
	}
	// Tokens aren't cast — blitz only applies to creature spells.
	if cardHasType(entering.Card, "token") {
		return
	}
	if cardCMC(entering.Card) < 4 {
		return
	}
	// Already blitzing (e.g. printed-blitz card like Jaxis): don't
	// double-stack the delayed triggers.
	if entering.Flags != nil && entering.Flags["blitz"] > 0 {
		return
	}

	totalCmdrCasts := totalCommanderCastCount(gs, perm.Controller)
	if totalCmdrCasts <= 0 {
		// No discount — blitz cost equals mana cost. The AI prefers
		// keeping the creature over the haste/draw/EOT-sac trade.
		return
	}

	gameengine.ApplyBlitz(gs, entering)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":               perm.Controller,
		"entering_card":      entering.Card.DisplayName(),
		"entering_cmc":       cardCMC(entering.Card),
		"commander_casts":    totalCmdrCasts,
	})
}

// totalCommanderCastCount sums CommanderCastCounts across all of a
// seat's commanders. Partner pairs each track their own count; Henzie's
// blitz discount aggregates them per the oracle phrasing ("for each
// time you've cast your commander").
func totalCommanderCastCount(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.CommanderCastCounts == nil {
		return 0
	}
	total := 0
	for _, n := range seat.CommanderCastCounts {
		total += n
	}
	return total
}
