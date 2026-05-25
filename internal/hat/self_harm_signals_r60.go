package hat

import (
	"strings"
	"unicode"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// self_harm_signals_r60.go — R60 round 16+ ChooseResponse extension.
// Builds on PR #410 (self_trigger_response_r60.go) which gated counter-
// ing-own-trigger on the draw-trigger + damage-punisher scenario. Two
// more self-harm scenarios share the same shape — "trigger I control,
// resolving it is lethal" — and deserve the same treatment:
//
//   Self-mill lethal: trigger I control mills me, mill count ≥ library
//     size → §704.5b deck-out loss. Mesmeric Orb upkeep, Hedron Crab
//     landfall, Phenax / Sphinx's Tutelage self-targeted variants.
//
//   Self-life-loss with stated cost: trigger I control says "you lose
//     N life" or "deals N damage to you" with a fixed number; N ≥ life
//     → §704.5a loss. Phyrexian Arena upkeep, Erebos Bleak-Hearted dies-
//     trigger, Bottomless Pit upkeep, generic upkeep-tax triggers.
//
// Both scenarios are wired into shouldCounterOwnTrigger via OR-chain so
// the existing draw-damage gate is unchanged and the new scenarios are
// strict additions. Each new helper is self-contained and follows the
// same "returns true iff lethal" contract for clean dispatcher
// composition.

// estimateMillTriggerCardCount returns the number of cards a mill
// trigger would put into the controller's graveyard. Heuristic — matches
// "mill N" / "mill a card" / "puts the top N cards" phrasings. Unknown
// or non-mill triggers return 0.
func estimateMillTriggerCardCount(top *gameengine.StackItem) int {
	if top == nil || top.Source == nil || top.Source.Card == nil {
		return 0
	}
	ot := gameengine.OracleTextLower(top.Source.Card)
	if ot == "" {
		return 0
	}
	// "mill N" / "mills N" — most common modern phrasings. Card oracle
	// text uses both forms depending on subject ("you mill 3" vs
	// "target player mills 3"). Check both.
	if n := extractNumberAfter(ot, "mills "); n > 0 {
		return n
	}
	if n := extractNumberAfter(ot, "mill "); n > 0 {
		return n
	}
	if strings.Contains(ot, "mill a card") ||
		strings.Contains(ot, "mills a card") {
		return 1
	}
	// "puts the top N cards of [their/your] library into [their/your]
	// graveyard" — legacy / Mesmeric Orb / Hedron Crab phrasing.
	if n := extractNumberAfter(ot, "puts the top "); n > 0 {
		return n
	}
	return 0
}

// triggerCausesLethalMill returns true when the trigger would mill the
// controller for at least their remaining library count. Bounded to
// self-controlled triggers; an opponent's mill targeting us is opp's
// problem to resolve, not ours to counter via this gate.
func triggerCausesLethalMill(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) bool {
	if gs == nil || top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if top.Controller != seatIdx {
		return false
	}
	count := estimateMillTriggerCardCount(top)
	if count == 0 {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	// Library size includes the top card we'd draw next turn. Lethal
	// projection: if the mill empties the library, the NEXT draw step
	// triggers §704.5b "draw from empty library = lose". Counter to
	// avoid the empty-library state.
	return count >= len(seat.Library)
}

// estimateLifeLossTriggerAmount returns the stated life-loss amount on
// a triggered ability's source. Matches "you lose N life" and
// "deals N damage to you" with explicit integer N. Returns 0 for
// variable amounts ("you lose life equal to ...") since we can't
// project those without per-card state — those are intentionally
// excluded so we don't counter Dark Confidant's trigger on a guess.
func estimateLifeLossTriggerAmount(top *gameengine.StackItem) int {
	if top == nil || top.Source == nil || top.Source.Card == nil {
		return 0
	}
	ot := gameengine.OracleTextLower(top.Source.Card)
	if ot == "" {
		return 0
	}
	// "you lose N life" — Phyrexian Arena, Erebos, generic upkeep tax.
	if n := extractNumberAfter(ot, "you lose "); n > 0 {
		// Validate the matched phrase ends with " life" so we don't
		// capture "you lose N cards" / "you lose N counters" etc.
		idx := strings.Index(ot, "you lose ")
		if idx >= 0 {
			tail := ot[idx+len("you lose "):]
			// Skip the number and any whitespace; check the next token is "life".
			for tail != "" && (unicode.IsDigit(rune(tail[0])) || tail[0] == ' ') {
				tail = tail[1:]
			}
			if strings.HasPrefix(tail, "life") {
				return n
			}
		}
		return 0
	}
	// "deals N damage to you" — symmetric phrasing (a trigger on a
	// permanent we control that hits us — Manabarbs is anthem-shape but
	// some upkeep-tax triggers use this wording).
	if n := extractNumberAfter(ot, "deals "); n > 0 {
		idx := strings.Index(ot, "deals ")
		if idx >= 0 {
			tail := ot[idx+len("deals "):]
			// Skip the number.
			for tail != "" && (unicode.IsDigit(rune(tail[0])) || tail[0] == ' ') {
				tail = tail[1:]
			}
			if strings.HasPrefix(tail, "damage to you") {
				return n
			}
		}
		return 0
	}
	return 0
}

// triggerCausesLethalLifeLoss returns true when a self-controlled
// trigger's stated life-loss / self-damage amount equals or exceeds
// current life. Variable-amount triggers (Dark Confidant's "equal to
// its CMC") return 0 from the estimator and don't fire this gate.
func triggerCausesLethalLifeLoss(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) bool {
	if gs == nil || top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if top.Controller != seatIdx {
		return false
	}
	loss := estimateLifeLossTriggerAmount(top)
	if loss == 0 {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	return loss >= seat.Life
}

// extractNumberAfter scans ot for the first occurrence of needle and
// reads the immediately-following integer token. Returns the integer
// or 0 if no number follows or the needle isn't present. Word-numbers
// ("two", "three") are mapped explicitly — Magic oracle text uses
// English words for small constants ("draw two cards", "mill three
// cards") but uses digits for larger ones ("mill 10 cards").
func extractNumberAfter(ot, needle string) int {
	idx := strings.Index(ot, needle)
	if idx < 0 {
		return 0
	}
	after := ot[idx+len(needle):]
	// Skip leading whitespace.
	for after != "" && after[0] == ' ' {
		after = after[1:]
	}
	if after == "" {
		return 0
	}
	// Digit prefix.
	if unicode.IsDigit(rune(after[0])) {
		n := 0
		for i := 0; i < len(after) && unicode.IsDigit(rune(after[i])); i++ {
			n = n*10 + int(after[i]-'0')
		}
		return n
	}
	// English word numbers — only the small constants oracle text uses.
	for word, val := range englishSmallNumbers {
		if strings.HasPrefix(after, word) {
			// Confirm the word ends at a non-letter boundary so "three"
			// doesn't match the prefix of "threefold" etc.
			next := len(word)
			if next == len(after) || !unicode.IsLetter(rune(after[next])) {
				return val
			}
		}
	}
	return 0
}

var englishSmallNumbers = map[string]int{
	"one":   1,
	"two":   2,
	"three": 3,
	"four":  4,
	"five":  5,
	"six":   6,
	"seven": 7,
	"eight": 8,
	"nine":  9,
	"ten":   10,
}
