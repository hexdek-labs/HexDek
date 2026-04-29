package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// PickTarget resolves a Filter into one or more runtime Targets against
// the current game state.
//
// MVP heuristics:
//   - "self"                        → the source permanent
//   - "you" / "controller"          → source's controlling seat
//   - "opponent"/"each opponent"    → gs.Opponents(srcController)
//   - "each_player"                 → every non-lost seat
//   - "player"                      → opponent (if targeted) or src's seat
//   - "creature" / "permanent"      → lowest-toughness opponent creature
//     (highest threat lives here too; full scoring is Phase 10)
//   - "any_target"                  → opponent seat
//
// When Filter.Quantifier is "each", the returned slice fans out; for
// single-target filters it holds exactly one Target.
//
// Returns nil on no-valid-target. Caller emits a "target illegal" event
// and the effect fizzles (§608.2b).
func PickTarget(gs *GameState, src *Permanent, f gameast.Filter) []Target {
	srcSeat := 0
	if src != nil {
		srcSeat = src.Controller
	}

	// Fan-out quantifiers first.
	switch f.Quantifier {
	case "each", "each_player":
		if isPlayerFilter(f) {
			return allPlayerTargets(gs, f, srcSeat)
		}
		return allPermanentTargets(gs, f, srcSeat)
	}

	// Equipped/enchanted creature — resolve to the attached-to permanent,
	// not the equipment/aura itself.
	switch f.Base {
	case "equipped_creature", "equipped creature",
		"enchanted_creature", "enchanted creature",
		"enchanted_permanent", "enchanted_land", "enchanted":
		if src == nil {
			return nil
		}
		// Aura/Equipment self-reference: "enchanted creature" on an Aura means
		// the permanent it's attached to.
		if src.AttachedTo != nil {
			return []Target{{Kind: TargetKindPermanent, Permanent: src.AttachedTo, Seat: src.AttachedTo.Controller}}
		}
		// Non-aura source with "target enchanted permanent/creature": find a
		// permanent that has at least one Aura attached (CR §303.4d).
		return pickEnchantedTargets(gs, f, srcSeat)
	}

	// Self-reference — covers "self", "it", "this", "that creature",
	// "that_thing", "pronoun", and their underscore/space variants.
	switch f.Base {
	case "self", "it", "this",
		"that_creature", "that creature",
		"this_creature", "this creature",
		"that_thing", "that", "them",
		"pronoun", "pronoun_it":
		if src == nil {
			return nil
		}
		return []Target{{Kind: TargetKindPermanent, Permanent: src, Seat: src.Controller}}
	}

	// Player-ish filters.
	if isPlayerFilter(f) {
		return pickPlayerTarget(gs, f, srcSeat)
	}

	// Permanent/creature/card filters on the battlefield.
	return pickPermanentTarget(gs, f, srcSeat, src)
}

// isPlayerFilter returns true if the filter targets a player (not a permanent).
func isPlayerFilter(f gameast.Filter) bool {
	switch f.Base {
	case "player", "opponent", "you", "controller", "each_player",
		"target_player", "target player",
		"each_opponent", "each opponent",
		"target_opponent", "target opponent",
		"that_player", "player_or_planeswalker":
		return true
	}
	return false
}

// allPlayerTargets fans out across every matching seat.
func allPlayerTargets(gs *GameState, f gameast.Filter, srcSeat int) []Target {
	out := make([]Target, 0, len(gs.Seats))
	isOpp := isOpponentBase(f.Base)
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if isOpp && i == srcSeat {
			continue
		}
		out = append(out, Target{Kind: TargetKindSeat, Seat: i})
	}
	return out
}

// isOpponentBase returns true if the base string refers to an opponent (not controller).
func isOpponentBase(base string) bool {
	switch base {
	case "opponent", "each_opponent", "each opponent",
		"target_opponent", "target opponent":
		return true
	}
	return false
}

// allPermanentTargets fans out across every battlefield permanent matching f.
func allPermanentTargets(gs *GameState, f gameast.Filter, srcSeat int) []Target {
	out := []Target{}
	for i, s := range gs.Seats {
		if !matchesControl(f, i, srcSeat) {
			continue
		}
		for _, p := range s.Battlefield {
			if matchesPermanent(f, p) {
				out = append(out, Target{Kind: TargetKindPermanent, Permanent: p, Seat: p.Controller})
			}
		}
	}
	return out
}

// allPermanentTargetsExcluding is like allPermanentTargets but excludes a
// specific permanent (for "each other creature" / "each other permanent").
func allPermanentTargetsExcluding(gs *GameState, f gameast.Filter, srcSeat int, exclude *Permanent) []Target {
	out := []Target{}
	for i, s := range gs.Seats {
		if !matchesControl(f, i, srcSeat) {
			continue
		}
		for _, p := range s.Battlefield {
			if p == exclude {
				continue
			}
			if matchesPermanent(f, p) {
				out = append(out, Target{Kind: TargetKindPermanent, Permanent: p, Seat: p.Controller})
			}
		}
	}
	return out
}

// pickPlayerTarget selects a single seat target.
func pickPlayerTarget(gs *GameState, f gameast.Filter, srcSeat int) []Target {
	switch f.Base {
	case "you", "controller":
		return []Target{{Kind: TargetKindSeat, Seat: srcSeat}}
	case "opponent", "target_opponent", "target opponent",
		"each_opponent", "each opponent":
		opps := gs.Opponents(srcSeat)
		if len(opps) == 0 {
			return nil
		}
		// MVP policy: pick lowest-life opponent (most threatened).
		best := opps[0]
		for _, o := range opps[1:] {
			if gs.Seats[o].Life < gs.Seats[best].Life {
				best = o
			}
		}
		return []Target{{Kind: TargetKindSeat, Seat: best}}
	case "player", "target_player", "target player",
		"that_player", "player_or_planeswalker":
		// Default to opponent when targeted, else controller.
		if f.Targeted {
			opps := gs.Opponents(srcSeat)
			if len(opps) > 0 {
				return []Target{{Kind: TargetKindSeat, Seat: opps[0]}}
			}
		}
		return []Target{{Kind: TargetKindSeat, Seat: srcSeat}}
	}
	return nil
}

// pickPermanentTarget selects a single battlefield target.
func pickPermanentTarget(gs *GameState, f gameast.Filter, srcSeat int, src *Permanent) []Target {
	// "any_target" / "any target" is the Lightning-Bolt classic — can be any
	// creature, player, or planeswalker. MVP picks an opponent seat.
	if f.Base == "any_target" || f.Base == "any target" || f.Base == "any" {
		opps := gs.Opponents(srcSeat)
		if len(opps) > 0 {
			return []Target{{Kind: TargetKindSeat, Seat: opps[0]}}
		}
		return []Target{{Kind: TargetKindSeat, Seat: srcSeat}}
	}

	// Check if "another"/"other" base — exclude source permanent.
	nb := normalizeBase(f.Base)
	excludeSrc := nb == "another" || nb == "other"

	// Candidate list, filtered by control + type.
	type candidate struct {
		p     *Permanent
		score int
	}
	candidates := []candidate{}
	for i, s := range gs.Seats {
		if !matchesControl(f, i, srcSeat) {
			continue
		}
		for _, p := range s.Battlefield {
			if excludeSrc && p == src {
				continue
			}
			if !matchesPermanent(f, p) {
				continue
			}
			// §702.18 (shroud) / §702.11 (hexproof) / §702.11d (hexproof
			// from [color]) — targeting check. Only applies when the filter
			// is "targeted" (i.e. "target creature"). Untargeted effects
			// ("each creature") bypass shroud/hexproof.
			if f.Targeted {
				var srcCard *Card
				if src != nil {
					srcCard = src.Card
				}
				if !CanBeTargetedByCombat(p, srcSeat, srcCard) {
					continue
				}
			}
			// Simple threat heuristic: for opponent permanents, prefer
			// higher power (= more threatening). For own permanents,
			// prefer lower toughness (= easier to save / wants a buff).
			score := p.Power()
			if i == srcSeat {
				score = -p.Toughness()
			}
			candidates = append(candidates, candidate{p, score})
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}
	return []Target{{Kind: TargetKindPermanent, Permanent: best.p, Seat: best.p.Controller}}
}

// matchesControl returns true if seat i is valid given f's control flags.
func matchesControl(f gameast.Filter, i, srcSeat int) bool {
	switch {
	case f.YouControl:
		return i == srcSeat
	case f.OpponentControls:
		return i != srcSeat
	}
	return true // no control constraint
}

// normalizeBase strips common targeting prefixes from a filter's Base string
// so that "target creature", "a creature", "an artifact" all resolve to the
// core type name used in the switch below.
func normalizeBase(base string) string {
	b := strings.ToLower(strings.TrimSpace(base))
	b = strings.TrimPrefix(b, "target ")
	b = strings.TrimPrefix(b, "a ")
	b = strings.TrimPrefix(b, "an ")
	// Handle "nonland permanent" → split into extra + base in the caller;
	// but we still strip the "target" prefix here.
	return b
}

// matchesPermanent returns true if p matches f (type/subtype/color/extra filters).
// This is the universal permanent-side filter matcher used by PickTarget for
// Destroy, Exile, Bounce, Tap, Untap, GainControl, Sacrifice, and other
// battlefield operations.
func matchesPermanent(f gameast.Filter, p *Permanent) bool {
	if p == nil || p.Card == nil {
		// Tokens/placeholders: accept if the filter isn't asking for a
		// specific subtype.
		if len(f.CreatureTypes) > 0 {
			return false
		}
	}

	// CR §707.2: Face-down permanents are 2/2 colorless creatures with no
	// name, no subtypes, and no abilities. Targeting checks must use the
	// face-down characteristics, not the real card data.
	if p != nil && p.Card != nil && p.Card.FaceDown {
		// Face-down = creature only. Reject any filter that requires a
		// specific type other than creature, any color, or any subtype.
		if len(f.CreatureTypes) > 0 {
			return false // no subtypes face-down
		}
		if len(f.ColorFilter) > 0 {
			return false // colorless face-down
		}
		nb := normalizeBase(f.Base)
		switch nb {
		case "creature", "permanent", "any_target", "any target", "any", "",
			"target", "another", "other", "thing", "creatures":
			// These match a face-down creature.
		default:
			// Color-as-base or specific type that isn't creature.
			switch nb {
			case "black", "red", "white", "blue", "green":
				return false // face-down is colorless
			case "artifact", "enchantment", "land", "planeswalker",
				"instant", "sorcery":
				return false // face-down is only creature
			}
			// Compound types: only match if "creature" is part of it.
			if !strings.Contains(nb, "creature") {
				return false
			}
		}
		// Face-down creature matches generic creature filters. Skip
		// all remaining type/color/subtype checks — face-down has none.
		return true
	}

	// Normalize the base — strips "target ", "a ", "an " prefixes.
	base := normalizeBase(f.Base)

	// Handle compound bases like "nonland permanent", "nonland creature" etc.
	// Extract the "non___" prefix as an extra adjective and use the remainder as
	// the effective base.
	if strings.HasPrefix(base, "non") && !strings.HasPrefix(base, "nonbasic") {
		parts := strings.SplitN(base, " ", 2)
		if len(parts) == 2 {
			// e.g. "nonland permanent" → extra="nonland", base="permanent"
			// The extra adjective is already handled below in the Extra loop,
			// so we inject it and update the base.
			f.Extra = append(f.Extra, parts[0])
			base = parts[1]
		}
		// Standalone negation bases without a space (e.g. "nontoken", "noncreature",
		// "nonland", "nonartifact") — treat as "any permanent" with the negation
		// applied as an Extra adjective.
		if len(parts) == 1 {
			f.Extra = append(f.Extra, base)
			base = "permanent"
		}
	}

	// Normalize compound bases: "creature with mana value 3 or less" →
	// strip everything after " with " and use the type prefix. Only
	// apply this when the prefix is a recognized MTG type to avoid
	// mangling parser-generated long descriptions.
	if idx := strings.Index(base, " with "); idx > 0 {
		prefix := base[:idx]
		// Check if the prefix is a recognized type.
		switch prefix {
		case "creature", "artifact", "enchantment", "land", "planeswalker",
			"permanent", "nonland creature", "nonland permanent",
			"noncommander creature", "noncreature permanent":
			base = prefix
		default:
			// Check for "non___" + type prefix.
			if strings.HasPrefix(prefix, "non") {
				base = prefix
			}
		}
	}

	// Normalize parser-generated long descriptions that include effect
	// text after a period/comma (e.g. "creature. if you do, create...")
	// → extract the leading type word.
	if dotIdx := strings.IndexAny(base, ".,;"); dotIdx > 0 && dotIdx < 20 {
		base = strings.TrimSpace(base[:dotIdx])
	}

	// Base type check.
	switch base {
	case "creature":
		if !p.IsCreature() {
			return false
		}
	case "land":
		if !hasType(p, "land") {
			return false
		}
	case "basic_land", "basic land":
		if !hasType(p, "land") || !hasType(p, "basic") {
			return false
		}
	case "artifact":
		if !hasType(p, "artifact") {
			return false
		}
	case "enchantment":
		if !hasType(p, "enchantment") {
			return false
		}
	case "planeswalker":
		if !hasType(p, "planeswalker") {
			return false
		}
	case "permanent", "any_target", "any target", "any", "",
		"target", "another", "other", "thing", "non", "or":
		// no type restriction — "another"/"other" exclusion is handled by the
		// caller (pickPermanentTarget skips src when f.Base is "another"/"other").
		// "non" is a parser fallback where the real restriction is in Extra.
		// "or" is a parser edge case for truncated compound types — treat as "any".
	case "creatures", "attacking_creatures":
		if !p.IsCreature() {
			return false
		}
	case "creature or planeswalker":
		if !p.IsCreature() && !hasType(p, "planeswalker") {
			return false
		}
	case "artifact or enchantment":
		if !hasType(p, "artifact") && !hasType(p, "enchantment") {
			return false
		}
	case "creature or land":
		if !p.IsCreature() && !hasType(p, "land") {
			return false
		}
	case "artifact or creature":
		if !hasType(p, "artifact") && !p.IsCreature() {
			return false
		}
	case "creature or enchantment":
		if !p.IsCreature() && !hasType(p, "enchantment") {
			return false
		}
	case "enchantment or planeswalker":
		if !hasType(p, "enchantment") && !hasType(p, "planeswalker") {
			return false
		}
	case "aura or equipment":
		if !hasType(p, "aura") && !hasType(p, "equipment") {
			return false
		}
	case "instant or sorcery":
		// Instants/sorceries are not permanents on battlefield.
		return false
	case "instant_or_sorcery_spell", "instant", "sorcery", "spell":
		// These are spell-type filters — permanents on the battlefield can't
		// match spell types. Return false so the effect fizzles cleanly.
		return false
	default:
		// Color-as-base: "black", "red", "white", "blue", "green" — the parser
		// sometimes puts a color name in Base instead of ColorFilter. Check if
		// the permanent has that color.
		if colorCode := colorNameToCode(base); colorCode != "" {
			if p.Card == nil || !cardHasColor(p.Card, colorCode) {
				return false
			}
			// Color matched — don't fall through to type check.
		} else if strings.Contains(base, " or ") || strings.Contains(base, "and/or") {
			// Generic compound "X or Y" / "X and/or Y" types we didn't
			// explicitly list above.
			sep := " or "
			if strings.Contains(base, "and/or") {
				sep = "and/or"
			}
			parts := strings.SplitN(base, sep, 2)
			leftStr := strings.TrimSpace(parts[0])
			rightStr := strings.TrimSpace(parts[1])
			// Strip trailing "s" for plurals ("creatures" → "creature").
			leftStr = singularize(leftStr)
			rightStr = singularize(rightStr)
			leftMatch := hasTypeOrCreature(p, leftStr)
			rightMatch := hasTypeOrCreature(p, rightStr)
			if !leftMatch && !rightMatch {
				return false
			}
		} else {
			// Fallback: try matching as a type/subtype string (catches "elf",
			// "zombie", "dragon", etc.).
			if p.Card != nil && !hasType(p, base) {
				return false
			}
		}
	}

	// Non-token filter.
	if f.NonToken {
		if p.IsToken() {
			return false
		}
	}

	// Extra adjective filters — handles "nonland permanent", "nonblack
	// creature", "nonartifact creature", etc.
	for _, ex := range f.Extra {
		exLow := strings.ToLower(ex)
		switch exLow {
		case "nonland", "non-land", "non_land":
			if hasType(p, "land") {
				return false
			}
		case "noncreature", "non-creature", "non_creature":
			if p.IsCreature() {
				return false
			}
		case "nonartifact", "non-artifact", "non_artifact":
			if hasType(p, "artifact") {
				return false
			}
		case "nonenchantment", "non-enchantment":
			if hasType(p, "enchantment") {
				return false
			}
		case "nontoken", "non-token":
			if p.IsToken() {
				return false
			}
		case "nonbasic", "non-basic":
			if hasType(p, "basic") {
				return false
			}
		case "nonlegendary", "non-legendary":
			if hasType(p, "legendary") {
				return false
			}
		case "legendary":
			if !hasType(p, "legendary") {
				return false
			}
		case "tapped":
			if !p.Tapped {
				return false
			}
		case "untapped":
			if p.Tapped {
				return false
			}
		case "attacking":
			if p.Flags == nil || p.Flags["attacking"] == 0 {
				return false
			}
		case "blocking":
			if p.Flags == nil || p.Flags["blocking"] == 0 {
				return false
			}
		default:
			// Color-negation extras: "nonblack", "nonwhite", "nonred", etc.
			// Also creature-type negation: "non-vampire", "non-zombie", etc.
			if strings.HasPrefix(exLow, "non") {
				negPart := strings.TrimPrefix(exLow, "non")
				negPart = strings.TrimPrefix(negPart, "-")
				negPart = strings.TrimPrefix(negPart, "_")
				if code := colorNameToCode(negPart); code != "" {
					// Color negation.
					if p.Card != nil && cardHasColor(p.Card, code) {
						return false
					}
				} else if p.Card != nil {
					// Creature type / supertype negation.
					if hasType(p, negPart) {
						return false
					}
				}
			}
		}
	}

	// Color filter — the permanent must have at least one of the listed colors.
	if len(f.ColorFilter) > 0 && p.Card != nil {
		matchesColor := false
		for _, filterColor := range f.ColorFilter {
			for _, cardColor := range p.Card.Colors {
				if strings.EqualFold(filterColor, cardColor) {
					matchesColor = true
					break
				}
			}
			if matchesColor {
				break
			}
		}
		if !matchesColor {
			return false
		}
	}

	// Color exclusion — the permanent must NOT have any of the excluded colors.
	if len(f.ColorExclude) > 0 && p.Card != nil {
		for _, excColor := range f.ColorExclude {
			for _, cardColor := range p.Card.Colors {
				if strings.EqualFold(excColor, cardColor) {
					return false
				}
			}
		}
	}

	// Mana value constraint.
	if f.ManaValueOp != "" && f.ManaValue != nil && p.Card != nil {
		if !compareIntTarget(p.Card.CMC, f.ManaValueOp, *f.ManaValue) {
			return false
		}
	}

	// Subtype filter (creature types).
	if len(f.CreatureTypes) > 0 {
		if p.Card == nil {
			return false
		}
		hit := false
		for _, want := range f.CreatureTypes {
			wantLow := strings.ToLower(want)
			for _, got := range p.Card.Types {
				if strings.ToLower(got) == wantLow {
					hit = true
					break
				}
			}
			if hit {
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// compareIntTarget is the same as compareInt in resolve.go — duplicated here
// to avoid circular cross-file dependencies within the package. Both live in
// the same package so the compiler is fine, but keeping them co-located with
// their callers improves readability.
func compareIntTarget(a int, op string, b int) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "==", "=":
		return a == b
	case "!=":
		return a != b
	}
	return false
}

// hasType returns true if p's card carries the given type string.
// Lenient on tokens with empty Types. Handles common plural/singular
// variations (e.g. "elves"→"elf", "goblins"→"goblin").
func hasType(p *Permanent, t string) bool {
	if p == nil || p.Card == nil {
		return false
	}
	for _, got := range p.Card.Types {
		if got == t {
			return true
		}
	}
	// Try singular/plural normalization.
	singular := singularize(t)
	if singular != t {
		for _, got := range p.Card.Types {
			if got == singular {
				return true
			}
		}
	}
	// Try the reverse: if t is singular, check against pluralized forms.
	for _, got := range p.Card.Types {
		if singularize(got) == t {
			return true
		}
	}
	return false
}

var singularIrregulars = map[string]string{
	"elves": "elf", "dwarves": "dwarf", "wolves": "wolf",
	"halves": "half", "thieves": "thief", "knives": "knife",
	"lives": "life", "leaves": "leaf", "selves": "self",
}

func singularize(s string) string {
	s = strings.ToLower(s)
	if sing, ok := singularIrregulars[s]; ok {
		return sing
	}
	// Regular -s/-es plurals.
	if strings.HasSuffix(s, "ies") && len(s) > 4 {
		return s[:len(s)-3] + "y" // "zombies" → "zombie" (wait, that's -ies→-y but zombie→zombies is -s)
	}
	if strings.HasSuffix(s, "ses") || strings.HasSuffix(s, "xes") || strings.HasSuffix(s, "zes") ||
		strings.HasSuffix(s, "ches") || strings.HasSuffix(s, "shes") {
		return s[:len(s)-2] // "witches" → "witch"
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") {
		return s[:len(s)-1] // "goblins" → "goblin", "creatures" → "creature"
	}
	return s
}

// colorNameToCode maps a color name to its single-letter code.
// Returns "" if the name isn't a recognized MTG color.
func colorNameToCode(name string) string {
	switch name {
	case "black":
		return "B"
	case "blue":
		return "U"
	case "white":
		return "W"
	case "red":
		return "R"
	case "green":
		return "G"
	}
	return ""
}

// cardHasColor returns true if the card has the given color code
// (e.g. "B", "U", "W", "R", "G") or color name.
func cardHasColor(c *Card, colorCode string) bool {
	if c == nil {
		return false
	}
	for _, cc := range c.Colors {
		if strings.EqualFold(cc, colorCode) {
			return true
		}
	}
	return false
}

// hasTypeOrCreature checks if a permanent has a given type, treating
// "creature" specially via IsCreature().
func hasTypeOrCreature(p *Permanent, typeName string) bool {
	if typeName == "creature" {
		return p.IsCreature()
	}
	return hasType(p, typeName)
}

// pickEnchantedTargets finds permanents that have at least one Aura attached.
// Used for non-aura spells like "Destroy target enchanted creature" (CR §303.4d).
func pickEnchantedTargets(gs *GameState, f gameast.Filter, srcSeat int) []Target {
	enchantedPerms := map[*Permanent]bool{}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.AttachedTo == nil {
				continue
			}
			if p.Card != nil && hasType(p, "aura") {
				enchantedPerms[p.AttachedTo] = true
			}
		}
	}
	if len(enchantedPerms) == 0 {
		return nil
	}
	wantCreature := strings.Contains(f.Base, "creature")
	wantPermanent := strings.Contains(f.Base, "permanent") || f.Base == "enchanted"
	preferOpponent := !f.YouControl
	for p := range enchantedPerms {
		if wantCreature && !p.IsCreature() {
			continue
		}
		if !wantPermanent && !wantCreature {
			continue
		}
		if preferOpponent && p.Controller == srcSeat {
			continue
		}
		if f.YouControl && p.Controller != srcSeat {
			continue
		}
		return []Target{{Kind: TargetKindPermanent, Permanent: p, Seat: p.Controller}}
	}
	for p := range enchantedPerms {
		if wantCreature && !p.IsCreature() {
			continue
		}
		return []Target{{Kind: TargetKindPermanent, Permanent: p, Seat: p.Controller}}
	}
	return nil
}
