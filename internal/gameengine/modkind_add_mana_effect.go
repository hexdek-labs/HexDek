package gameengine

import (
	"regexp"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// modkind_add_mana_effect.go — improves the `add_mana_effect` scaffold
// ModKind, the catch-all the parser emits for add-mana text it couldn't
// fold into a structured AddMana node. The prior behavior added exactly
// ONE generic mana for every such card, which badly under-produced the
// large, regular family of dual FILTER LANDS:
//
//	{W/U}, {T}: Add {W}{W}, {W}{U}, or {U}{U}.   (Mystic Gate)
//	{W/B}, {T}: Add {W}{W}, {W}{B}, or {B}{B}.   (Fetid Heath)
//	… and Graven Cairns, Cascade Bluffs, Sunken Ruins, Flooded Grove,
//	Twilight Mire, Fire-Lit Thicket, Rugged Prairie, Wooded Bastion —
//	all 10 Eventide/Shadowmoor filter lands.
//
// These should add TWO mana of the listed colors, not one generic. The
// args[0] text carries the explicit mana symbols, so we parse the
// high-confidence shape — comma/"or"-separated modes of literal {X}
// pips — and add the mode-width count of mana distributed across the
// distinct colors. Variable-quantity text ("add X mana in any
// combination …" storage lands, "for each …") and symbol-free text
// ("add two mana of the chosen color", "add one mana of any color …")
// stay on the conservative one-generic fallback rather than guess.

var reAddManaEffectSymbol = regexp.MustCompile(`\{([WUBRGCwubrgc])\}`)

// addManaEffectVariable reports text whose mana quantity isn't a fixed
// literal pip count and therefore can't be safely parsed from symbols.
func addManaEffectVariable(text string) bool {
	for _, marker := range []string{"x mana", "any combination", "any number", "for each", "equal to"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// resolveAddManaEffect handles one add_mana_effect ModificationEffect.
// Returns true if it produced structured colored mana from the args text;
// false to signal the caller to fall back to the legacy one-generic add.
func resolveAddManaEffect(gs *GameState, src *Permanent, e *gameast.ModificationEffect) bool {
	if gs == nil || src == nil {
		return false
	}
	seat := controllerSeat(src)
	if seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(modArgString(e.Args, 0)))
	if text == "" || addManaEffectVariable(text) {
		return false
	}
	if !reAddManaEffectSymbol.MatchString(text) {
		return false
	}

	// Mode width = the most pips in any one comma/"or"-separated segment.
	// "add {w}{w}, {w}{u}, or {u}{u}" → 3 segments, each 2 pips → 2 mana.
	segments := regexp.MustCompile(`,| or | and/or `).Split(text, -1)
	width := 0
	for _, seg := range segments {
		if n := len(reAddManaEffectSymbol.FindAllString(seg, -1)); n > width {
			width = n
		}
	}
	if width <= 0 {
		return false
	}

	// Distinct colors in first-appearance order — deterministic output.
	var colors []string
	seen := map[string]bool{}
	for _, m := range reAddManaEffectSymbol.FindAllStringSubmatch(text, -1) {
		c := strings.ToUpper(m[1])
		if !seen[c] {
			seen[c] = true
			colors = append(colors, c)
		}
	}
	if len(colors) == 0 {
		return false
	}

	// Distribute `width` mana round-robin across the distinct colors
	// (filter lands → 1 of each; single-color modes → all of that color).
	added := map[string]int{}
	for i := 0; i < width; i++ {
		added[colors[i%len(colors)]]++
	}
	for _, c := range colors {
		if added[c] > 0 {
			AddMana(gs, gs.Seats[seat], c, added[c], sourceName(src))
		}
	}
	gs.LogEvent(Event{
		Kind:   "add_mana_effect",
		Seat:   seat,
		Source: sourceName(src),
		Amount: width,
		Details: map[string]interface{}{
			"colors": colors,
			"text":   text,
		},
	})
	return true
}
