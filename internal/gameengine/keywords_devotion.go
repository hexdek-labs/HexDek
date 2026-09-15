package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_devotion.go — CR §700.5 Devotion (Theros, 2013).
//
// "Devotion to [color]" is a count: how many mana symbols of that
// color appear in the mana costs of permanents you control. Cards
// like Gray Merchant of Asphodel, Master of Waves, Nykthos, Shrine to
// Nyx scale their effects by this count.
//
// CountDevotion (keywords_misc.go) does the count; this file adds the
// rest of the devotion surface:
//
//   - DevotionPipsFromManaCost   shared pip parser. Walks "{...}"
//                                symbols in a mana-cost string and
//                                returns the pip contribution toward
//                                a given color. Handles single-color,
//                                hybrid {B/R} (counts BOTH colors),
//                                twobrid {2/B} (+1 color), and
//                                Phyrexian {B/P} (+1 color) per
//                                §700.5a. Generic / X / snow /
//                                colorless contribute 0.
//   - DevotionToWhite/Blue/      per-color convenience wrappers
//     Black/Red/Green            around CountDevotion. The full
//                                set of five matches the canonical
//                                §107.4a color set.
//   - HasDevotionRider(card, c)  oracle-text / AST detector for
//                                "devotion to [color]" clauses on a
//                                card. Returns true when the card's
//                                text references devotion to color c
//                                (e.g. Gray Merchant "your devotion
//                                to black", Master of Waves "your
//                                devotion to blue").
//   - ApplyDevotionRider         resolver-side rider executor.
//                                Mirrors ApplyThresholdRider /
//                                ApplyMaxSpeedRider. Logs a
//                                devotion_rider event with the color
//                                and current count, runs the tagged
//                                payload if present, otherwise logs
//                                devotion_rider_pending for per_card
//                                handler / corpus backfill.
//
// Wired into resolveSequence (resolve.go) alongside threshold and
// max-speed. Each spell resolution fires the devotion rider at most
// once per color whose rider is printed on the card. Depth-counter
// gated so nested Sequence nodes don't re-fire.

// DevotionColors is the canonical color list used by per-color
// devotion APIs and the resolveSequence rider fan-out. Order matches
// WUBRG (the standard MTG color order).
var DevotionColors = []string{"W", "U", "B", "R", "G"}

// isColorLetter returns true if s is exactly one uppercase color
// letter ("W", "U", "B", "R", "G").
func isColorLetter(s string) bool {
	if len(s) != 1 {
		return false
	}
	switch s[0] {
	case 'W', 'U', 'B', 'R', 'G':
		return true
	}
	return false
}

// DevotionPipsFromManaCost parses `cost` (a Scryfall-style
// "{2}{B}{B}" / "{B/R}" string) and returns the number of mana
// symbols that contribute to devotion to `color`. CR §700.5.
//
// Counting rules per §700.5a-b:
//   - Single-color symbol `{C}`: +1 to color C.
//   - Hybrid `{A/B}` where both halves are colors: +1 to BOTH A and B
//     independently.
//   - Twobrid `{2/C}`: +1 to color C.
//   - Phyrexian `{C/P}`: +1 to color C.
//   - Generic `{N}`, X `{X}`, snow `{S}`, colorless `{C}` (the
//     colorless-mana symbol, distinct from a color letter), and
//     anything else not matching the above: 0.
//
// `color` is matched case-insensitively. Empty cost or unrecognized
// color returns 0.
//
// The parser is tolerant: malformed strings (unclosed braces,
// unknown contents) are skipped without panicking. We accept both
// uppercase and lowercase color letters inside the braces.
func DevotionPipsFromManaCost(cost, color string) int {
	if cost == "" || color == "" {
		return 0
	}
	want := strings.ToUpper(strings.TrimSpace(color))
	if !isColorLetter(want) {
		return 0
	}

	total := 0
	i := 0
	for i < len(cost) {
		if cost[i] != '{' {
			i++
			continue
		}
		end := strings.IndexByte(cost[i:], '}')
		if end < 0 {
			break
		}
		inner := strings.ToUpper(cost[i+1 : i+end])
		i += end + 1
		total += pipsForSymbol(inner, want)
	}
	return total
}

// pipsForSymbol returns the devotion contribution toward `want` for a
// single mana symbol's inner text (the part between the braces, already
// uppercased). See DevotionPipsFromManaCost for the rules.
func pipsForSymbol(inner, want string) int {
	switch {
	case inner == "":
		return 0
	case isColorLetter(inner):
		if inner == want {
			return 1
		}
		return 0
	case strings.Contains(inner, "/"):
		parts := strings.Split(inner, "/")
		// Hybrid (color/color), twobrid (2/color), Phyrexian (color/P).
		count := 0
		for _, part := range parts {
			if part == want {
				count++
			}
		}
		return count
	}
	return 0
}

// DevotionToWhite returns devotion to white for `seatIdx`.
func DevotionToWhite(gs *GameState, seatIdx int) int {
	return CountDevotion(gs, seatIdx, "W")
}

// DevotionToBlue returns devotion to blue for `seatIdx`.
func DevotionToBlue(gs *GameState, seatIdx int) int {
	return CountDevotion(gs, seatIdx, "U")
}

// DevotionToBlack returns devotion to black for `seatIdx`.
func DevotionToBlack(gs *GameState, seatIdx int) int {
	return CountDevotion(gs, seatIdx, "B")
}

// DevotionToRed returns devotion to red for `seatIdx`.
func DevotionToRed(gs *GameState, seatIdx int) int {
	return CountDevotion(gs, seatIdx, "R")
}

// DevotionToGreen returns devotion to green for `seatIdx`.
func DevotionToGreen(gs *GameState, seatIdx int) int {
	return CountDevotion(gs, seatIdx, "G")
}

// HasDevotionRider reports whether `card`'s printed text references
// "devotion to <color>" for the given color. Two detection paths
// mirror the other rider keywords:
//
//  1. cardHasKeywordByName(card, "devotion") — if a corpus dump
//     tags the card with the devotion keyword, treat any color
//     query as a positive match (the per-color filter happens at
//     resolution-time via the payload, not at the keyword level).
//  2. Oracle text matches "devotion to <colorWord>" where colorWord
//     is the spelled-out color name (white/blue/black/red/green).
//
// Case-insensitive. Returns false for nil cards, empty oracle text,
// or unrecognized color codes.
func HasDevotionRider(card *Card, color string) bool {
	if card == nil || color == "" {
		return false
	}
	want := strings.ToUpper(strings.TrimSpace(color))
	if !isColorLetter(want) {
		return false
	}
	if cardHasKeywordByName(card, "devotion") {
		return true
	}
	text := OracleTextLower(card)
	if text == "" {
		return false
	}
	colorWord := colorWordFor(want)
	if colorWord == "" {
		return false
	}
	return strings.Contains(text, "devotion to "+colorWord)
}

// colorWordFor returns the lowercase spelled-out color name for a
// color letter ("B" → "black"). Returns "" for unrecognized input.
func colorWordFor(color string) string {
	switch strings.ToUpper(strings.TrimSpace(color)) {
	case "W":
		return "white"
	case "U":
		return "blue"
	case "B":
		return "black"
	case "R":
		return "red"
	case "G":
		return "green"
	}
	return ""
}

// findDevotionRiderEffect returns the Effect of the first Activated
// ability tagged as a devotion-rider payload, or nil. Tagging
// convention mirrors threshold / max-speed: Cost.Extra contains
// "devotion_rider" OR Activated.Raw begins with "devotion".
func findDevotionRiderEffect(card *Card) gameast.Effect {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		a, ok := ab.(*gameast.Activated)
		if !ok || a == nil || a.Effect == nil {
			continue
		}
		if abilityIsTaggedDevotionRider(a) {
			return a.Effect
		}
	}
	return nil
}

func abilityIsTaggedDevotionRider(a *gameast.Activated) bool {
	if a == nil {
		return false
	}
	for _, extra := range a.Cost.Extra {
		if strings.EqualFold(extra, "devotion_rider") {
			return true
		}
	}
	raw := strings.ToLower(strings.TrimSpace(a.Raw))
	return strings.HasPrefix(raw, "devotion")
}

// ApplyDevotionRider executes the devotion rider for `src` against
// `color`, if any. Returns true if the rider fired.
//
// Conditions to fire:
//   - src is non-nil and has a Card.
//   - HasDevotionRider(src.Card, color) — the card prints a devotion
//     rider for this color.
//
// Note we DO NOT gate on a minimum devotion count — Gray Merchant
// resolves even at devotion 0 (it just deals 0 damage and drains 0
// life). The payload reads the current count and scales accordingly.
//
// On fire we log a devotion_rider event recording the color and the
// current devotion count. If the AST carries a tagged payload, we
// run it with ResolveEffect; otherwise we log
// devotion_rider_pending so per_card handlers + corpus backfill can
// find affected cards.
func ApplyDevotionRider(gs *GameState, src *Permanent, color string) bool {
	if gs == nil || src == nil || src.Card == nil {
		return false
	}
	if !HasDevotionRider(src.Card, color) {
		return false
	}
	count := CountDevotion(gs, src.Controller, color)

	gs.LogEvent(Event{
		Kind:   "devotion_rider",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Amount: count,
		Details: map[string]interface{}{
			"rule":     "700.5",
			"color":    strings.ToUpper(strings.TrimSpace(color)),
			"devotion": count,
		},
	})

	if eff := findDevotionRiderEffect(src.Card); eff != nil {
		ResolveEffect(gs, src, eff)
		return true
	}

	gs.LogEvent(Event{
		Kind:   "devotion_rider_pending",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "700.5",
			"color":  strings.ToUpper(strings.TrimSpace(color)),
			"reason": "rider_payload_not_in_ast",
		},
	})
	return true
}

// ApplyDevotionRidersAllColors fires the devotion rider once per color
// for which `src.Card` prints one. Called from resolveSequence
// alongside the threshold / max-speed riders so spells with a printed
// devotion clause auto-fire at the right time.
func ApplyDevotionRidersAllColors(gs *GameState, src *Permanent) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	for _, c := range DevotionColors {
		if HasDevotionRider(src.Card, c) {
			ApplyDevotionRider(gs, src, c)
		}
	}
}
