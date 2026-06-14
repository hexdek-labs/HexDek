package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Theros God creature-toggle (CR 711.2 / devotion CDA-like state check).
//
// Gods (Erebos, Heliod, Thassa, Phenax, …) are printed "Legendary Enchantment
// Creature — God", so their Card.Types carry "creature" after deck-load. But
// each god has a static ability — parsed to the `devotion_gated_not_creature`
// AST modkind — reading "as long as your devotion to <color(s)> is less than
// <N>, ~ isn't a creature." Before r63 the engine had ZERO consumers of that
// modkind: the toggle was UNWIRED, so a god was ALWAYS a creature (could attack
// / block / be targeted by creature-only effects) regardless of devotion.
//
// IsCreature() reads p.Card.Types DIRECTLY (it does not consult the §613 layer
// cache), and the engine's working "becomes a creature" effects (crew, animate,
// Start Your Engines) all mutate Card.Types. So this toggle does the same:
// RefreshDevotionCreatureToggles re-evaluates each god's devotion and
// adds/removes "creature" from Card.Types accordingly. It is called from
// StateBasedActions, which runs after every action and before priority, so the
// status tracks devotion continuously (property b) — it flips the instant
// devotion crosses the threshold and reverts when it drops.

// colorWordToLetter maps a devotion color word (or letter) to its WUBRG letter.
func colorWordToLetter(word string) string {
	switch strings.ToLower(strings.TrimSpace(word)) {
	case "white":
		return "W"
	case "blue":
		return "U"
	case "black":
		return "B"
	case "red":
		return "R"
	case "green":
		return "G"
	}
	u := strings.ToUpper(strings.TrimSpace(word))
	if isColorLetter(u) {
		return u
	}
	return ""
}

// parseDevotionColorArg extracts WUBRG letters from the modkind's color
// argument, which the parser emits as a nested array of color words
// (["blue","black"]) — decoded here as []interface{} or []string.
func parseDevotionColorArg(arg interface{}) []string {
	var words []interface{}
	switch v := arg.(type) {
	case []interface{}:
		words = v
	case []string:
		var out []string
		for _, s := range v {
			if l := colorWordToLetter(s); l != "" {
				out = append(out, l)
			}
		}
		return out
	default:
		return nil
	}
	var out []string
	for _, w := range words {
		if s, ok := w.(string); ok {
			if l := colorWordToLetter(s); l != "" {
				out = append(out, l)
			}
		}
	}
	return out
}

// parseDevotionThresholdArg extracts the integer threshold from the modkind's
// threshold argument ("five"/"seven" word, or a numeric value).
func parseDevotionThresholdArg(arg interface{}) int {
	switch v := arg.(type) {
	case string:
		if n, ok := wordToInt(v); ok {
			return n
		}
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// findDevotionGatedNotCreature returns the WUBRG color letters and the
// devotion threshold for a card's `devotion_gated_not_creature` static, or
// ok=false if the card has none.
func findDevotionGatedNotCreature(ast *gameast.CardAST) (letters []string, threshold int, ok bool) {
	if ast == nil {
		return nil, 0, false
	}
	for _, ab := range ast.Abilities {
		st, isStatic := ab.(*gameast.Static)
		if !isStatic || st.Modification == nil {
			continue
		}
		if st.Modification.ModKind != "devotion_gated_not_creature" {
			continue
		}
		args := st.Modification.Args
		if len(args) < 2 {
			continue
		}
		ls := parseDevotionColorArg(args[0])
		th := parseDevotionThresholdArg(args[1])
		if len(ls) == 0 || th <= 0 {
			continue
		}
		return ls, th, true
	}
	return nil, 0, false
}

// devotionForColorLetters returns devotion to the given color set. For a single
// color it defers to the canonical CountDevotion; for multiple colors it walks
// each controlled permanent's mana-cost symbols ONCE, counting a symbol that
// contributes to ANY of the colors (so a single hybrid symbol counts once, not
// once per color — CR multi-color devotion).
func devotionForColorLetters(gs *GameState, seatIdx int, letters []string) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || len(letters) == 0 {
		return 0
	}
	if len(letters) == 1 {
		return CountDevotion(gs, seatIdx, letters[0])
	}
	total := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		cost := p.Card.ManaCostString
		if cost == "" {
			if p.IsToken() {
				continue
			}
			// Fixture fallback (Colors set, no printed cost): count the
			// permanent once if it carries any of the gate's colors.
			for _, c := range p.Card.Colors {
				match := false
				for _, l := range letters {
					if strings.EqualFold(c, l) {
						match = true
						break
					}
				}
				if match {
					total++
					break
				}
			}
			continue
		}
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
			for _, l := range letters {
				if pipsForSymbol(inner, strings.ToUpper(l)) > 0 {
					total++
					break // each symbol counts at most once
				}
			}
		}
	}
	return total
}

// applyOneDevotionGate sets p's creature-ness to match its current devotion:
// "creature" present in Card.Types iff devotion >= threshold. Returns true if
// it changed the type set.
func applyOneDevotionGate(gs *GameState, p *Permanent, letters []string, th int) bool {
	if th <= 0 || p.Card == nil || len(letters) == 0 {
		return false
	}
	wantCreature := devotionForColorLetters(gs, p.Controller, letters) >= th
	hasCreature := false
	for _, t := range p.Card.Types {
		if strings.EqualFold(t, "creature") {
			hasCreature = true
			break
		}
	}
	if wantCreature == hasCreature {
		return false
	}
	if wantCreature {
		p.Card.Types = append(p.Card.Types, "creature")
	} else {
		kept := make([]string, 0, len(p.Card.Types))
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "creature") {
				continue
			}
			kept = append(kept, t)
		}
		p.Card.Types = kept
	}
	gs.InvalidateCharacteristicsCache()
	gs.LogEvent(Event{
		Kind:   "devotion_creature_toggle",
		Seat:   p.Controller,
		Source: p.Card.DisplayName(),
		Details: map[string]interface{}{
			"is_creature": wantCreature,
			"threshold":   th,
			"rule":        "711.2",
		},
	})
	return true
}

// RefreshDevotionCreatureToggles re-evaluates every Theros-God-style permanent's
// creature-ness against current devotion and toggles "creature" in Card.Types.
// Detection is read-only (it inspects Card.AST without mutating any Flags — so
// it does not perturb non-god permanents' observable state), and the only
// mutation is on an actual god whose creature-ness crossed the threshold.
// Returns true if any permanent changed.
func RefreshDevotionCreatureToggles(gs *GameState) bool {
	if gs == nil {
		return false
	}
	changed := false
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || p.Card.AST == nil {
				continue
			}
			letters, th, ok := findDevotionGatedNotCreature(p.Card.AST)
			if !ok {
				continue
			}
			if applyOneDevotionGate(gs, p, letters, th) {
				changed = true
			}
		}
	}
	return changed
}
