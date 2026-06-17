package gameengine

import "strings"

// dynamic_x.go — resolution of the bare "x" amount when X is defined by a
// "where X is <count>" clause rather than a cast {X} cost.
//
// The parser (scripts/) emits the amount/count of "deals X damage" /
// "put X counters" / "create X tokens" effects as the literal string "x",
// but it does NOT structure the trailing "where X is the number of …"
// clause — that text survives only on the source ability's Raw. With no
// {X} in the mana cost, gs.Flags["x"] is 0 at resolution, so evalNumber's
// "x" branch returned 0 and the effect under-resolved (damage 0, counters/
// tokens floored to 1). Live-grinder signature (r63): Thundering Sparkmage
// 0 damage, Priest of the Crossing +1/+1:1, Siegfried +1/+1:1, plus the
// distinct permanent-ETB cast-X miss on Defenders of Humanity.
//
// evalWhereXClause recovers the count from the source ability's Raw at the
// moment the effect resolves (so post-mill / post-board state is live, e.g.
// Siegfried counts the graveyard AFTER its own mill). It returns (0, false)
// for any card without a recognized "where X is" clause, so the existing
// cast-X / preset behavior is unchanged for every other card.
func evalWhereXClause(gs *GameState, src *Permanent) (int, bool) {
	if gs == nil || src == nil || src.Card == nil || src.Card.AST == nil {
		return 0, false
	}
	for _, ab := range src.Card.AST.Abilities {
		raw := abilityRaw(ab)
		if raw == "" {
			continue
		}
		low := strings.ToLower(raw)
		// Scope to the BASIS clause (the text after "where X is"). The full
		// ability Raw also carries the effect's TARGET phrase — e.g. Voja's
		// "put X +1/+1 counters on each creature you control, where X is the
		// number of elves you control" — and matching against the whole string
		// let the generic "creatures you control" case grab the target clause
		// (counting all 5 creatures) instead of the real basis (3 elves). Only
		// the post-marker substring is the count source.
		basis, ok := extractWhereXBasis(low)
		if !ok {
			continue
		}
		if n, ok := whereXCount(gs, src, basis); ok {
			return n, true
		}
	}
	return 0, false
}

// extractWhereXBasis returns the count-basis substring that follows the
// "where X is" / "where X =" marker in an already-lowercased ability Raw.
// Returns ("", false) when no marker is present.
func extractWhereXBasis(low string) (string, bool) {
	for _, marker := range []string{"where x is", "where x ="} {
		if i := strings.Index(low, marker); i >= 0 {
			return strings.TrimSpace(low[i+len(marker):]), true
		}
	}
	return "", false
}

// (abilityRaw lives in flicker_r63.go — extracts .Raw from Static/Triggered/
// Activated ability nodes, which covers every "where X is" clause carrier.)

// whereXCount maps a recognized "where X is <count>" clause (already
// lowercased) to its value against the live game state. Returns (0, false)
// for unrecognized count sources so the caller falls through unchanged.
//
// Multiplier: "twice the number of …" → ×2, "three times …" → ×3, else ×1.
func whereXCount(gs *GameState, src *Permanent, low string) (int, bool) {
	seat := src.Controller

	mult := 1
	switch {
	case strings.Contains(low, "twice the number of"):
		mult = 2
	case strings.Contains(low, "three times the number of"):
		mult = 3
	}

	switch {
	// Thundering Sparkmage — "the number of creatures in your party".
	case strings.Contains(low, "creatures in your party"), strings.Contains(low, "creature in your party"):
		return mult * CountParty(gs, seat), true

	// Basalt Ravager — "the greatest number of creatures you control that have
	// a creature type in common" (the largest single-shared-creature-type
	// group). Checked BEFORE the generic "creatures you control" case, which
	// would otherwise match this clause's "creatures you control" substring and
	// return the TOTAL creature count instead of the largest shared-type group.
	case strings.Contains(low, "creature type in common"):
		return mult * greatestSharedCreatureTypeCount(gs, seat), true

	// Priest of the Crossing — "creatures that died under your control this
	// turn" (and the plain "creatures died this turn" phrasing).
	case strings.Contains(low, "died under your control this turn"),
		strings.Contains(low, "creatures that died this turn"),
		strings.Contains(low, "creatures died this turn"):
		if seat >= 0 && seat < len(gs.Seats) && gs.Seats[seat] != nil {
			return mult * gs.Seats[seat].Turn.CreaturesDied, true
		}
		return 0, true

	// Siegfried, Famed Swordsman — "twice the number of creature cards in
	// your graveyard" (computed live, after the preceding mill).
	case strings.Contains(low, "creature cards in your graveyard"),
		strings.Contains(low, "creature card in your graveyard"):
		return mult * countCardsInGraveyardByType(gs, seat, "creature"), true

	// Generic "the number of creatures you control".
	case strings.Contains(low, "creatures you control"), strings.Contains(low, "creature you control"):
		return mult * countCreaturesControlled(gs, seat), true
	}

	// Subtype-specific PERMANENT count: "the number of <subtype> you control"
	// (Voja — elves you control; zombies / vampires / goblins / … creature
	// subtypes; AND non-creature permanent subtypes — Southern Air Temple's
	// "shrines you control" [enchantment subtype], vehicles / equipment / auras
	// / sagas). Resolved by deterministic pluralization of each controlled
	// permanent's own subtypes (elf→elves, shrine→shrines, ally→allies), which
	// sidesteps the singular-from-plural ambiguity. Only takes effect when at
	// least one matching permanent exists, so an unrecognized basis counts zero
	// here and falls through to the unchanged gs.Flags["x"] behavior.
	if noun, ok := subtypeYouControlNoun(low); ok {
		if n := countPermanentsBySubtypeNoun(gs, seat, noun); n > 0 {
			return mult * n, true
		}
	}
	return 0, false
}

// greatestSharedCreatureTypeCount returns the largest number of creatures a
// seat controls that share a single creature type (CR — "creatures you control
// that have a creature type in common"). Basalt Ravager's X-basis.
func greatestSharedCreatureTypeCount(gs *GameState, seat int) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	counts := map[string]int{}
	best := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || !p.IsCreature() || p.Card == nil {
			continue
		}
		for sub := range creatureSubtypesOf(p.Card) {
			counts[sub]++
			if counts[sub] > best {
				best = counts[sub]
			}
		}
	}
	return best
}

// subtypeYouControlNoun extracts the type noun from a "(the) number of <noun>
// you control" basis clause (already lowercased). Returns the last word before
// "you control" — the type noun ("elves", "zombies") — or ("", false) when the
// clause shape isn't a "number of … you control" count, or the noun is a
// generic non-subtype word the dedicated cases / permanent-count paths own.
func subtypeYouControlNoun(low string) (string, bool) {
	i := strings.Index(low, "number of ")
	if i < 0 {
		return "", false
	}
	rest := low[i+len("number of "):]
	j := strings.Index(rest, " you control")
	if j < 0 {
		return "", false
	}
	fields := strings.Fields(rest[:j])
	if len(fields) == 0 {
		return "", false
	}
	noun := fields[len(fields)-1]
	switch noun {
	// Generic creature phrasings are owned by the case above; permanent /
	// land / characteristic counts are a different (unhandled) basis — leave
	// them to the unchanged path rather than miscount them as creatures.
	case "creature", "creatures", "permanent", "permanents", "land", "lands",
		"artifact", "artifacts", "enchantment", "enchantments",
		"planeswalker", "planeswalkers", "token", "tokens", "card", "cards",
		"types", "colors", "swamps", "mountains", "islands", "forests", "plains":
		return "", false
	}
	return noun, true
}

// countPermanentsBySubtypeNoun counts permanents a seat controls whose own
// subtypes pluralize to (or already equal) the given plural type noun. Covers
// creature subtypes (elves / zombies / goblins) AND non-creature permanent
// subtypes (shrines / vehicles / equipment / auras / sagas); creatureSubtypesOf
// returns the subtype set for any permanent (it strips card-type supertypes, so
// an enchantment "Shrine" yields {"shrine"}).
func countPermanentsBySubtypeNoun(gs *GameState, seat int, noun string) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for sub := range creatureSubtypesOf(p.Card) {
			if sub == noun || pluralizeCreatureType(sub) == noun {
				n++
				break
			}
		}
	}
	return n
}

// pluralizeCreatureType applies standard English pluralization to a creature
// subtype singular (elf→elves, ally→allies, zombie→zombies). Deterministic
// from the singular, so matching a controlled creature's subtype against a
// printed plural noun avoids the ambiguity of singularizing the noun
// (zombies→zombie vs allies→ally).
func pluralizeCreatureType(s string) string {
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "fe") {
		return s[:len(s)-2] + "ves" // knife→knives
	}
	if strings.HasSuffix(s, "f") {
		return s[:len(s)-1] + "ves" // elf→elves, wolf→wolves, dwarf→dwarves
	}
	if strings.HasSuffix(s, "y") && len(s) >= 2 && !isVowel(s[len(s)-2]) {
		return s[:len(s)-1] + "ies" // ally→allies
	}
	return s + "s"
}

func isVowel(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// countCardsInGraveyardByType counts cards of the given type in a seat's
// graveyard.
func countCardsInGraveyardByType(gs *GameState, seat int, typ string) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	n := 0
	for _, c := range gs.Seats[seat].Graveyard {
		if c != nil && cardHasType(c, typ) {
			n++
		}
	}
	return n
}

// countCreaturesControlled counts creatures on a seat's battlefield.
func countCreaturesControlled(gs *GameState, seat int) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsCreature() {
			n++
		}
	}
	return n
}
