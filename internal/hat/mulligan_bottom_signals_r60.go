package hat

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// mulligan_bottom_signals_r60.go — R60 round 6+ London-mulligan bottom-pile
// signals. ChooseBottomCards (yggdrasil.go:7422) already runs cardHeuristic
// + combo / value-engine / star bumps + cuttable penalty + a 2-land floor,
// but two well-known London-mulligan instincts were missing from the
// ranking:
//
//   1. Color-deadness — a {U}{U} Counterspell in a hand with no Island
//      can't be cast for several turns, while we still draw 1/turn. The
//      heuristic treated such cards by their inherent quality only. We
//      now subtract a small bias from a card's keep-value when the lands
//      in the keep hand can't satisfy its pure / hybrid colored pips.
//
//   2. Early-play anchor — when the hand contains exactly one non-land
//      card with CMC ≤ 2 (the only T1-T2 play), bottoming it leaves the
//      hand doing nothing for the first two turns. We bump that card's
//      keep-value so the heuristic preserves it.
//
// Both signals are intentionally small (0.4 / 0.6) so they nudge the
// ranking on tiebreakers without overriding the combo / star / cuttable
// scaffolding that's already pulling its weight in the gauntlet.

// colorBitsByPureSlot maps the gameengine `manaSlot{W,U,B,R,G}` indices
// (0..4) to the hat-package color bits. Engine convention: W=0, U=1,
// B=2, R=3, G=4, C=5. We don't include the C slot because colorless
// generic pips are paid from any land and never color-dead.
var colorBitsByPureSlot = [5]uint8{
	colorBitW, colorBitU, colorBitB, colorBitR, colorBitG,
}

// handProducedColorMask returns the bitmask of mana colors lands in the
// supplied hand can produce. Builds on landProducesColorsMask (yggdrasil.go:8750)
// which works for cards in any zone — it inspects the printed type line
// + "add {X}" oracle text. Non-land cards are ignored.
func handProducedColorMask(hand []*gameengine.Card) uint8 {
	var mask uint8
	for _, c := range hand {
		if c == nil {
			continue
		}
		if !typeLineContains(c, "land") {
			continue
		}
		mask |= landProducesColorsMask(c)
	}
	return mask
}

// cardColorDeadAgainstHand reports whether `card` has at least one pure
// or hybrid colored pip that the supplied `produces` mask can't satisfy.
// Conservative defaults:
//   - lands and engine-minted cards (ManaCostString == "") return false
//   - hybrid pips count as satisfied when ANY of their colors is available
//   - colorless / generic-only cards are never color-dead
//
// Used as a small keep-value penalty in ChooseBottomCards so spells the
// current land base can't cast drift toward the bottom pile.
func cardColorDeadAgainstHand(card *gameengine.Card, produces uint8) bool {
	if card == nil || card.ManaCostString == "" {
		return false
	}
	if typeLineContains(card, "land") {
		return false
	}
	req := gameengine.ParseCostRequirements(card.ManaCostString)
	for slot, n := range req.Pure[:5] {
		if n > 0 && produces&colorBitsByPureSlot[slot] == 0 {
			return true
		}
	}
	for _, mask := range req.Hybrid {
		if mask&produces == 0 {
			return true
		}
	}
	return false
}

// countCheapPlayables counts non-land cards in the hand with CMC ≤ 2.
// "Cheap playables" stand in for "things you can do on turn 1 or 2"
// without trying to predict castability — color-fixing and ramp can
// usually unblock cheap colored spells within the cheap window, so the
// raw CMC count is a robust proxy.
func countCheapPlayables(hand []*gameengine.Card) int {
	n := 0
	for _, c := range hand {
		if c == nil {
			continue
		}
		if typeLineContains(c, "land") {
			continue
		}
		if gameengine.ManaCostOf(c) <= 2 {
			n++
		}
	}
	return n
}

// isEarlyPlayAnchor returns true when `card` is the hand's only cheap
// non-land playable (CMC ≤ 2). Caller passes a pre-computed
// `cheapPlayables` count from countCheapPlayables to avoid an O(n²)
// inner loop inside ChooseBottomCards. The helper does its own
// type / CMC check on the candidate card so a caller miscount doesn't
// accidentally anchor a 5-mana bomb.
func isEarlyPlayAnchor(card *gameengine.Card, cheapPlayables int) bool {
	if cheapPlayables != 1 {
		return false
	}
	if card == nil || typeLineContains(card, "land") {
		return false
	}
	return gameengine.ManaCostOf(card) <= 2
}
