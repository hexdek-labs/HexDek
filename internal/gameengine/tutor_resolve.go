package gameengine

// Generic tutor resolver — replaces 350+ potential per-card handlers with
// a single AST-driven dispatch function. Reads the parser's Tutor node
// fields (query filter, destination, count, reveal, shuffle_after) and
// resolves them against the current game state.
//
// Architecture:
//
//   ResolveTutorGeneric is called from resolve.go when the effect is a
//   *gameast.Tutor AND no per-card snowflake handler exists for the card.
//   Cards that DO have per-card handlers (Vampiric Tutor, Demonic Tutor,
//   Mystical Tutor, Enlightened Tutor, Worldly Tutor) continue to use
//   those handlers for side-effect correctness (life payment, etc.); the
//   generic resolver handles the ~350 remaining tutors that are pure
//   "search your library for [filter], put it into [destination]" with
//   no exotic side effects.
//
// Tutor AST node fields:
//
//   Query        Filter       — what to search for (creature, land, basic,
//                               artifact, enchantment, instant/sorcery,
//                               unrestricted "card", etc.)
//   Destination  string       — where it goes: "hand", "top_of_library",
//                               "battlefield", "battlefield_tapped",
//                               "graveyard"
//   Count        NumberOrRef  — how many: 1, "up_to_1", "up_to_2", X, "all"
//   Optional     bool         — "you may" (fail-to-find is legal)
//   ShuffleAfter bool         — whether library is shuffled after
//   Reveal       bool         — whether the found card must be revealed
//   Rest         string       — what happens to remaining revealed cards:
//                               "bottom", "graveyard", "exile", ""
//
// Filter matching (library-side, via cardMatchesFilter):
//
//   The AST parser emits a Filter on the Tutor.Query field. Filter.Base
//   is the primary type: "creature", "land", "basic_land", "artifact",
//   "enchantment", "instant", "sorcery", "card" (any), etc.
//
//   Filter.Extra carries adjectives like "nonland", "nonartifact".
//   Filter.CreatureTypes carries creature subtypes (e.g. ["elf"]).
//   Filter.ManaValueOp / ManaValue carry CMC constraints.
//
// Opposition Agent interaction:
//
//   CR §701.19 — if an opponent controls an Opposition Agent, the searching
//   player's found cards are exiled to the Agent controller's exile zone
//   instead of going to the searcher's destination. This is handled by
//   checking oppositionAgentControlsSeat before placement.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ResolveTutorGeneric is the engine-level generic tutor resolver. It reads
// the Tutor AST node's query filter, scans the controller's library for
// matching cards, picks the best matches (via Hat heuristics), places them
// in the destination zone, and shuffles if required.
//
// Returns the number of cards found and placed.
//
// The caller (resolve.go) is responsible for checking whether a per-card
// snowflake handler exists before calling this function.
func ResolveTutorGeneric(gs *GameState, casterSeat int, tutor *gameast.Tutor) int {
	if gs == nil || tutor == nil || casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return 0
	}

	// --- 1. Determine search count ---
	count := 1
	isUpTo := false
	if n, ok := evalNumber(gs, nil, &tutor.Count); ok && n > 0 {
		count = n
	}
	// Check for "up_to" quantifier — the query's quantifier may encode this.
	if strings.HasPrefix(tutor.Query.Quantifier, "up_to") {
		isUpTo = true
	}
	// "all" quantifier → search for ALL matching cards.
	isAll := tutor.Query.Quantifier == "all"

	// --- 2. Check Opposition Agent ---
	agentController := oppositionAgentControlsSeat(gs, casterSeat)

	// --- 3. Scan library for matching cards ---
	lib := gs.Seats[casterSeat].Library
	matches := []match{}
	for i, c := range lib {
		if c == nil {
			continue
		}
		if cardMatchesTutorFilter(c, tutor.Query) {
			matches = append(matches, match{idx: i, card: c})
			if !isAll && len(matches) >= count {
				break
			}
		}
	}

	// If "up to" or "all", adjust count to actual matches found.
	if isUpTo || isAll {
		if len(matches) < count || isAll {
			count = len(matches)
		}
	}

	// Optional tutor with no matches — legal "fail to find".
	if len(matches) == 0 {
		if tutor.ShuffleAfter {
			shuffleLibrary(gs, casterSeat)
		}
		gs.LogEvent(Event{
			Kind:   "tutor",
			Seat:   casterSeat,
			Source: "generic_tutor",
			Amount: 0,
			Details: map[string]interface{}{
				"destination": tutor.Destination,
				"filter_base": tutor.Query.Base,
				"result":      "fail_to_find",
			},
		})
		return 0
	}

	// Limit to count. The selecting player depends on Opposition Agent:
	// normally the caster picks, but with Agent active the Agent's
	// CONTROLLER conducts the search (CR 701.19, "you control your
	// opponents while they're searching their libraries").
	if len(matches) > count {
		if agentController >= 0 {
			// Opposition Agent's controller picks — they choose the card
			// that best serves THEIR interests, not the searcher's.
			// For GreedyHat: pick the highest-CMC card (steal the best one).
			matches = pickBestTutorMatches(matches, count, tutor.Query.Base)
		} else {
			matches = pickBestTutorMatches(matches, count, tutor.Query.Base)
		}
	}

	// --- 4. Remove found cards from library ---
	// Build a removal set, then rebuild library without those cards.
	removeSet := map[int]bool{}
	for _, m := range matches {
		removeSet[m.idx] = true
	}
	newLib := make([]*Card, 0, len(lib)-len(matches))
	for i, c := range lib {
		if !removeSet[i] {
			newLib = append(newLib, c)
		}
	}
	gs.Seats[casterSeat].Library = newLib

	// --- 5. For "top_of_library" destination, shuffle FIRST then place ---
	// This matches the correct sequence for Vampiric Tutor et al:
	// "Search, shuffle, then put on top."
	dest := tutor.Destination
	if dest == "" {
		dest = "hand"
	}
	if dest == "top_of_library" && tutor.ShuffleAfter {
		shuffleLibrary(gs, casterSeat)
	}

	// --- 6. Place cards at destination ---
	found := 0
	for _, m := range matches {
		if agentController >= 0 {
			// Opposition Agent (CR 701.19): "You control your opponents
			// while they're searching their libraries." The Agent's
			// controller conducted the search (card selection above) and
			// found cards go to exile. The Agent's controller may PLAY
			// those exiled cards, spending mana as any color.
			gs.Seats[agentController].Exile = append(gs.Seats[agentController].Exile, m.card)
			// Grant zone-cast permission: Agent controller can play the
			// exiled card as though it had any color of mana.
			if gs.ZoneCastGrants == nil {
				gs.ZoneCastGrants = make(map[*Card]*ZoneCastPermission)
			}
			gs.ZoneCastGrants[m.card] = NewFreeCastFromExilePermission(agentController, "Opposition Agent")
			gs.LogEvent(Event{
				Kind:   "opposition_agent_exile",
				Seat:   agentController,
				Source: "Opposition Agent",
				Details: map[string]interface{}{
					"exiled_card":       m.card.DisplayName(),
					"searcher_seat":     casterSeat,
					"controller_seat":   agentController,
					"can_play_exiled":   true,
					"any_color_mana":    true,
					"rule":              "701.19",
				},
			})
		} else {
			placeTutoredCard(gs, casterSeat, m.card, dest)
		}
		found++
	}

	// --- 7. Shuffle library (if not already done for top_of_library) ---
	if tutor.ShuffleAfter && dest != "top_of_library" {
		shuffleLibrary(gs, casterSeat)
	}
	// Default: most tutors shuffle even if ShuffleAfter isn't explicitly set.
	// The AST parser defaults ShuffleAfter=true for most tutors, but if it's
	// missing and we found cards, shuffle anyway (conservative correctness).
	if !tutor.ShuffleAfter && found > 0 {
		// Only shuffle if ShuffleAfter wasn't explicitly set to false.
		// The zero-value (false) is ambiguous — most tutors DO shuffle.
		// We shuffle by default since failing to shuffle is a bigger error
		// than shuffling unnecessarily. Per-card handlers that need no-shuffle
		// (e.g., Worldly Tutor variants that don't shuffle) will be handled
		// by their own per-card hook.
		shuffleLibrary(gs, casterSeat)
	}

	// --- 8. Log the tutor event ---
	details := map[string]interface{}{
		"destination": dest,
		"filter_base": tutor.Query.Base,
	}
	if tutor.Reveal {
		details["revealed"] = true
	}
	if found > 0 && agentController >= 0 {
		details["intercepted_by"] = "opposition_agent"
	}
	// Log found card names for debugging/analysis.
	if found > 0 && found <= 3 {
		names := make([]string, 0, found)
		for _, m := range matches[:found] {
			names = append(names, m.card.DisplayName())
		}
		details["found_cards"] = names
	}
	gs.LogEvent(Event{
		Kind:   "tutor",
		Seat:   casterSeat,
		Source: "generic_tutor",
		Amount: found,
		Details: details,
	})

	return found
}

// cardMatchesTutorFilter checks if a library card matches the Tutor's query
// filter. This is an enhanced version of cardMatchesFilter that also handles:
//   - "basic_land" / "basic" base types
//   - "instant_or_sorcery" compound base
//   - "artifact_or_enchantment" compound base
//   - Extra adjectives: "nonland", "nonartifact", "noncreature", "nonbasic"
//   - ManaValue constraints
//   - Color filters
//   - CreatureType subtypes
func cardMatchesTutorFilter(c *Card, f gameast.Filter) bool {
	if c == nil {
		return false
	}

	base := strings.ToLower(f.Base)

	// --- Base type matching ---
	switch base {
	case "", "card", "any_target":
		// No type restriction — any card matches.

	case "creature":
		if !cardHasType(c, "creature") {
			return false
		}

	case "land":
		if !cardHasType(c, "land") {
			return false
		}

	case "basic_land", "basic land":
		if !cardHasType(c, "land") || !cardHasType(c, "basic") {
			return false
		}

	case "nonbasic_land":
		if !cardHasType(c, "land") || cardHasType(c, "basic") {
			return false
		}

	case "artifact":
		if !cardHasType(c, "artifact") {
			return false
		}

	case "enchantment":
		if !cardHasType(c, "enchantment") {
			return false
		}

	case "instant":
		if !cardHasType(c, "instant") {
			return false
		}

	case "sorcery":
		if !cardHasType(c, "sorcery") {
			return false
		}

	case "instant_or_sorcery", "instant/sorcery":
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			return false
		}

	case "artifact_or_enchantment", "artifact/enchantment":
		if !cardHasType(c, "artifact") && !cardHasType(c, "enchantment") {
			return false
		}

	case "planeswalker":
		if !cardHasType(c, "planeswalker") {
			return false
		}

	case "permanent", "thing", "non", "or":
		// Permanent = creature | artifact | enchantment | planeswalker | land | battle.
		// "thing", "non", "or" are parser fallbacks — treat as "any".
		if base == "permanent" {
			if !cardHasType(c, "creature") && !cardHasType(c, "artifact") &&
				!cardHasType(c, "enchantment") && !cardHasType(c, "planeswalker") &&
				!cardHasType(c, "land") && !cardHasType(c, "battle") {
				return false
			}
		}

	case "creature or planeswalker":
		if !cardHasType(c, "creature") && !cardHasType(c, "planeswalker") {
			return false
		}
	case "artifact or enchantment":
		if !cardHasType(c, "artifact") && !cardHasType(c, "enchantment") {
			return false
		}
	case "creature or land":
		if !cardHasType(c, "creature") && !cardHasType(c, "land") {
			return false
		}
	case "creature or enchantment":
		if !cardHasType(c, "creature") && !cardHasType(c, "enchantment") {
			return false
		}
	case "instant or sorcery":
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			return false
		}
	case "aura or equipment":
		if !cardHasType(c, "aura") && !cardHasType(c, "equipment") {
			return false
		}

	default:
		// Color-as-base: "black", "red", "white", "blue", "green".
		colorBases := map[string]string{
			"black": "B", "blue": "U", "white": "W", "red": "R", "green": "G",
		}
		if colorCode, ok := colorBases[base]; ok {
			hasColor := false
			for _, cc := range c.Colors {
				if strings.EqualFold(cc, colorCode) || strings.EqualFold(cc, base) {
					hasColor = true
					break
				}
			}
			if !hasColor {
				return false
			}
		} else if strings.HasPrefix(base, "non") && base != "nonbasic_land" {
		// Handle negation-as-base: "nonlegendary", "noncreature", "nonland", etc.
			parts := strings.SplitN(base, " ", 2)
			if len(parts) == 2 {
				// "nonland permanent" → check not-land + permanent type
				negPart := strings.TrimPrefix(parts[0], "non")
				negPart = strings.TrimPrefix(negPart, "-")
				if cardHasType(c, negPart) {
					return false
				}
				// Check the remaining base type.
				if parts[1] == "creature" && !cardHasType(c, "creature") {
					return false
				}
			} else {
				// Standalone "nonlegendary", "noncreature", etc.
				negPart := strings.TrimPrefix(base, "non")
				negPart = strings.TrimPrefix(negPart, "-")
				if cardHasType(c, negPart) {
					return false
				}
			}
		} else if strings.Contains(base, " or ") {
			// Generic compound types we didn't list above.
			parts := strings.SplitN(base, " or ", 2)
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			// Strip "creature_card" suffix.
			right = strings.TrimSuffix(right, " creature_card")
			right = strings.TrimSuffix(right, "_card")
			if !cardHasType(c, left) && !cardHasType(c, right) {
				return false
			}
		} else {
			// Fallback: try matching as a type string. Catches tribal types
			// like "elf", "wizard", "dragon", etc.
			// Strip "_card" suffix if present.
			cleaned := strings.TrimSuffix(base, "_card")
			cleaned = strings.TrimSuffix(cleaned, " card")
			// Handle "that_card" / self-reference — matches any card (the
			// resolver typically means "the card this effect refers to").
			if cleaned == "that" || cleaned == "that_card" || cleaned == "this" {
				// Self-reference — matches anything in graveyard.
			} else if strings.Contains(cleaned, " ") {
				// Compound base like "dragon permanent" or "red sorcery".
				// Split into words and check each meaningful word as a type
				// or color. Skip generic/noise words.
				parts := strings.Fields(cleaned)
				// Check if this is a very long parser description (more than
				// 3 words) — treat it as a single type check to avoid over-matching.
				if len(parts) <= 3 {
					colorBases := map[string]string{
						"black": "B", "blue": "U", "white": "W", "red": "R", "green": "G",
					}
					for _, part := range parts {
						// Skip generic words that don't represent types.
						if part == "permanent" || part == "nonland" || part == "target" {
							continue
						}
						// Check if it's a color word — verify card has that color.
						if code, isColor := colorBases[part]; isColor {
							hasColor := false
							for _, cc := range c.Colors {
								if strings.EqualFold(cc, code) {
									hasColor = true
									break
								}
							}
							if !hasColor {
								return false
							}
							continue
						}
						if !cardHasType(c, part) {
							return false
						}
					}
				} else {
					// Long parser description — try the whole string.
					if !cardHasType(c, cleaned) {
						return false
					}
				}
			} else if !cardHasType(c, cleaned) {
				return false
			}
		}
	}

	// --- Extra adjective filters (negative) ---
	for _, ex := range f.Extra {
		exLow := strings.ToLower(ex)
		switch exLow {
		case "nonland", "non-land", "non_land":
			if cardHasType(c, "land") {
				return false
			}
		case "noncreature", "non-creature", "non_creature":
			if cardHasType(c, "creature") {
				return false
			}
		case "nonartifact", "non-artifact", "non_artifact":
			if cardHasType(c, "artifact") {
				return false
			}
		case "nontoken", "non-token":
			// Library cards are never tokens — always passes.
		case "nonbasic", "non-basic":
			if cardHasType(c, "basic") {
				return false
			}
		case "nonlegendary", "non-legendary":
			if cardHasType(c, "legendary") {
				return false
			}
		case "legendary":
			if !cardHasType(c, "legendary") {
				return false
			}
		case "historic":
			if !cardHasType(c, "legendary") && !cardHasType(c, "artifact") && !cardHasType(c, "saga") {
				return false
			}
		}
	}

	// --- Creature type / subtype filter ---
	if len(f.CreatureTypes) > 0 {
		hit := false
		for _, want := range f.CreatureTypes {
			wantLow := strings.ToLower(want)
			for _, got := range c.Types {
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

	// --- Color filter ---
	if len(f.ColorFilter) > 0 {
		matchesColor := false
		for _, filterColor := range f.ColorFilter {
			for _, cardColor := range c.Colors {
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

	// --- Color exclusion ---
	if len(f.ColorExclude) > 0 {
		for _, excColor := range f.ColorExclude {
			for _, cardColor := range c.Colors {
				if strings.EqualFold(excColor, cardColor) {
					return false
				}
			}
		}
	}

	// --- Mana value constraint ---
	if f.ManaValueOp != "" && f.ManaValue != nil {
		if !compareInt(c.CMC, f.ManaValueOp, *f.ManaValue) {
			return false
		}
	}

	return true
}

// pickBestTutorMatches selects the best N cards from a larger match set.
// Hat heuristic (GreedyHat baseline):
//   - For creature tutors: pick highest CMC (biggest threat)
//   - For land tutors: pick first match (any land is fine)
//   - For unrestricted: pick highest CMC (Demonic Tutor wants the best card)
//   - For combo pieces: future Phase 10 policy will score by wincon proximity
func pickBestTutorMatches(matches []match, count int, baseType string) []match {
	if len(matches) <= count {
		return matches
	}

	// Sort by CMC descending (biggest = best for greedy baseline).
	// Simple selection sort since count is typically 1-3.
	for i := 0; i < count; i++ {
		bestIdx := i
		for j := i + 1; j < len(matches); j++ {
			if matches[j].card.CMC > matches[bestIdx].card.CMC {
				bestIdx = j
			}
		}
		if bestIdx != i {
			matches[i], matches[bestIdx] = matches[bestIdx], matches[i]
		}
	}
	return matches[:count]
}

// match is a helper type for tutor resolution — tracks a card and its
// original index in the library.
type match struct {
	idx  int
	card *Card
}
