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
		if !strings.Contains(low, "where x is") && !strings.Contains(low, "where x =") {
			continue
		}
		if n, ok := whereXCount(gs, src, low); ok {
			return n, true
		}
	}
	return 0, false
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
	return 0, false
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
