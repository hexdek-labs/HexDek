package gameengine

import (
	"regexp"
	"strconv"
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

// r109 — addManaEffectVariable is a COLOUR-blind quantity guard with a
// colour phrase in its marker list: "any combination" describes how the
// mana may be SPLIT, not whether the amount is variable. Relic of Sauron
// prints "add TWO mana in any combination of {U}, {B}, and/or {R}" — a
// literal count — and was bailing to the one-generic fallback because of
// it (CR §107.4c pips; lesson 60).
//
// resolveAddManaVariableCount resolves the quantity for the shapes that
// ARE derivable at resolution time. ok=false means "still not derivable"
// and the caller keeps the legacy fallback.
//
// NOTE ON ORDER (r109): the count must be resolved BEFORE any colour is
// emitted. The fallback's single mana lands in the typed pool's `Any`
// bucket, and `Any` is a UNIVERSAL WILDCARD — CanPayColored adds it to
// every colour's availability and PayColoredPip spends it for any pip.
// So emitting the printed colours while leaving the count at 1 makes
// these cards STRICTLY WORSE than the bug does. Colour is only an
// improvement when it arrives with the count.
var reAddManaCountWord = regexp.MustCompile(`add (one|two|three|four|five|six|seven|eight|nine|ten|[0-9]+) mana`)

var addManaCountWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

func resolveAddManaVariableCount(gs *GameState, src *Permanent, text string) (int, bool) {
	// (a) A literal count sitting in the text ("add two mana in any
	// combination of …"). Not variable at all — only the colour phrase
	// dragged it into the bailout.
	if m := reAddManaCountWord.FindStringSubmatch(text); m != nil {
		if n, ok := addManaCountWords[m[1]]; ok {
			return n, true
		}
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n, true
		}
	}
	if !strings.Contains(text, "where x is") {
		return 0, false
	}
	// (b) "where X is <self>'s power" (Vivi Ornitier, A-Vivi Ornitier).
	// Require the possessive to name THIS permanent — "the greatest power
	// among creatures you control" (Selvala) and "the total power of
	// attacking creatures" (Klauth) are different bases and must not match.
	//
	// The Alchemy rebalances keep the ORIGINAL card's name in their own
	// oracle text — A-Vivi Ornitier reads "where X is Vivi Ornitier's
	// power" — so strip the "a-" prefix before comparing. Measured: the
	// corpus coverage ledger caught A-Vivi falling through without this.
	self := strings.TrimPrefix(strings.ToLower(sourceName(src)), "a-") + "'s power"
	if strings.Contains(text, self) || strings.Contains(text, "~'s power") {
		pow := gs.PowerOf(src)
		if pow < 0 {
			pow = 0
		}
		return pow, true
	}
	// (c) "where X is the number of creatures you control" (Xenagos).
	if strings.Contains(text, "the number of creatures you control") {
		return countCreaturesControlled(gs, controllerSeat(src)), true
	}
	return 0, false
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
	if text == "" {
		return false
	}
	if !reAddManaEffectSymbol.MatchString(text) {
		return false
	}

	width := 0
	if addManaEffectVariable(text) {
		// r109 — a variable-quantity shape. Resolve the COUNT first; if it
		// can't be resolved, keep the legacy fallback rather than narrowing
		// the colour (see resolveAddManaVariableCount's order note).
		//
		// A printed spend restriction (CR §106.6) is not modelled on this
		// path, so decline rather than mint a larger amount of UNRESTRICTED
		// mana than the fallback already does. Leyline Immersion is the
		// corpus case.
		if strings.Contains(text, "spend this mana only") {
			return false
		}
		n, ok := resolveAddManaVariableCount(gs, src, text)
		if !ok || n <= 0 {
			return false
		}
		width = n
	} else {
		// Mode width = the most pips in any one comma/"or"-separated segment.
		// "add {w}{w}, {w}{u}, or {u}{u}" → 3 segments, each 2 pips → 2 mana.
		segments := regexp.MustCompile(`,| or | and/or `).Split(text, -1)
		for _, seg := range segments {
			if n := len(reAddManaEffectSymbol.FindAllString(seg, -1)); n > width {
				width = n
			}
		}
		if width <= 0 {
			return false
		}
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
