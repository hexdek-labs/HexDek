package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// cost_reduce_self.go — generic handler for the `this_spell_colored_cost_reduce`
// static ability KIND: "This spell costs {N} less to cast [if <condition> |
// for each <thing>]." ~169 spells parse this clause to an inert
// `this_spell_colored_cost_reduce` Modification (log-only static bucket in
// resolveModificationEffect), so every one of them was charged FULL price.
//
// selfSpellCostReductionMods scans the spell's OWN AST for this kind, evaluates
// the reduction, and returns CostModReduction entries. It is invoked from
// CalculateTotalCostWithContext (the §601.2f cost chokepoint that the cast /
// payment path uses), so the reduction actually lowers what is paid.
//
// CONSERVATIVE / UNDER-REDUCE policy (engine-100%-correctness mandate): a
// cost reduction that fires when it should NOT lets the caster pay too little
// — a real bug. So this handler only emits a reduction when it can evaluate
// the condition/count from current game state with confidence:
//   - "for each <countable>" → reduce by N × an ACCURATE game-state count.
//   - "if <state condition>" → reduce by N only when the condition is
//     provably met.
//   - target-dependent ("if it targets …"), cost-meta ("if it's bargained"),
//     and any unrecognized clause → NO reduction (full cost, the prior inert
//     behavior). Never over-reduces.
func selfSpellCostReductionMods(gs *GameState, card *Card, seatIdx int) []CostModifier {
	if gs == nil || card == nil || card.AST == nil {
		return nil
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return nil
	}
	var out []CostModifier
	for _, ab := range card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "this_spell_colored_cost_reduce" {
			continue
		}
		args := st.Modification.Args
		if len(args) == 0 {
			continue
		}
		amt := bracedGenericAmount(asString(args[0]))
		if amt <= 0 {
			continue
		}
		clause := ""
		if len(args) > 1 {
			clause = strings.ToLower(strings.TrimSpace(asString(args[1])))
		}
		reduction, known := evalSelfCostReduceClause(gs, card, seatIdx, amt, clause)
		if known && reduction > 0 {
			out = append(out, CostModifier{
				Kind:   CostModReduction,
				Amount: reduction,
				Source: card.DisplayName() + " (self cost reduction)",
			})
		}
	}
	return out
}

// evalSelfCostReduceClause returns (reduction, known). known=false means the
// clause was not recognized → caller adds no modifier (full cost).
func evalSelfCostReduceClause(gs *GameState, card *Card, seatIdx, amt int, clause string) (int, bool) {
	if clause == "" {
		// No condition parsed: an unconditional "costs {N} less". Safe flat.
		return amt, true
	}
	// Target-dependent and cost-meta clauses cannot be evaluated at cost time
	// without the spell's chosen targets / cast modes → skip (full cost).
	if strings.Contains(clause, "it targets") ||
		strings.Contains(clause, "bargained") ||
		strings.Contains(clause, "kicked") ||
		strings.Contains(clause, "this way") {
		return 0, false
	}

	if strings.HasPrefix(clause, "for each") || strings.Contains(clause, "for each") {
		count, ok := costReduceForEachCount(gs, seatIdx, clause)
		if !ok {
			return 0, false
		}
		return amt * count, true
	}

	if strings.HasPrefix(clause, "if") {
		met, ok := costReduceIfCondition(gs, seatIdx, clause)
		if !ok {
			return 0, false
		}
		if met {
			return amt, true
		}
		return 0, true // recognized, condition not met → no reduction
	}
	return 0, false
}

// costReduceForEachCount counts the "for each <thing>" subject from current
// state. Returns (count, recognized).
func costReduceForEachCount(gs *GameState, seatIdx int, clause string) (int, bool) {
	seat := gs.Seats[seatIdx]
	// Party (CR §700.4) — uses the engine's existing CountParty.
	if strings.Contains(clause, "party") {
		return CountParty(gs, seatIdx), true
	}
	// "this turn" trackers and combat-state counts are not reliably available
	// here → skip to stay conservative.
	if strings.Contains(clause, "this turn") || strings.Contains(clause, "attack") {
		return 0, false
	}
	// Graveyard-based counts (the dominant, accurately-computable family).
	if strings.Contains(clause, "graveyard") {
		instSorc := strings.Contains(clause, "instant") && strings.Contains(clause, "sorcery")
		switch {
		case instSorc:
			return countGYWhere(seat, func(c *Card) bool {
				return cardHasType(c, "instant") || cardHasType(c, "sorcery")
			}), true
		case strings.Contains(clause, "creature"):
			return countGYWhere(seat, func(c *Card) bool { return cardHasType(c, "creature") }), true
		case strings.Contains(clause, "land"):
			return countGYWhere(seat, func(c *Card) bool { return cardHasType(c, "land") }), true
		case strings.Contains(clause, "instant"):
			return countGYWhere(seat, func(c *Card) bool { return cardHasType(c, "instant") }), true
		case strings.Contains(clause, "sorcery"):
			return countGYWhere(seat, func(c *Card) bool { return cardHasType(c, "sorcery") }), true
		case strings.Contains(clause, "card"):
			return len(seat.Graveyard), true
		}
		return 0, false
	}
	// "for each opponent you have"
	if strings.Contains(clause, "opponent") {
		n := 0
		for _, o := range gs.Opponents(seatIdx) {
			if gs.Seats[o] != nil && !gs.Seats[o].Lost {
				n++
			}
		}
		return n, true
	}
	// Battlefield creature counts.
	if strings.Contains(clause, "creature on the battlefield") {
		n := 0
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p != nil && p.IsCreature() {
					n++
				}
			}
		}
		return n, true
	}
	if strings.Contains(clause, "creature you control") {
		n := 0
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				n++
			}
		}
		return n, true
	}
	return 0, false
}

// costReduceIfCondition evaluates a flat "if <state>" condition. Returns
// (met, recognized).
func costReduceIfCondition(gs *GameState, seatIdx int, clause string) (bool, bool) {
	seat := gs.Seats[seatIdx]
	// "if you've cast another spell this turn"
	if strings.Contains(clause, "cast another spell") || strings.Contains(clause, "cast a spell this turn") {
		return seat.Turn.SpellsCast >= 1, true
	}
	// "if you control a creature with <keyword>"
	if strings.Contains(clause, "creature with ") {
		kw := afterPhrase(clause, "creature with ")
		kw = strings.TrimSpace(strings.TrimSuffix(kw, "."))
		if kw == "" {
			return false, false
		}
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() && (CardHasKeyword(p.Card, kw) || p.HasKeyword(kw)) {
				return true, true
			}
		}
		return false, true
	}
	// "if you control a <subtype>" (e.g. wizard) — only when clearly a
	// creature-subtype gate.
	if strings.HasPrefix(clause, "if you control a ") || strings.HasPrefix(clause, "if you control an ") {
		noun := afterPhrase(clause, "you control a")
		noun = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(noun), "n")) // handle "an"
		noun = strings.TrimSpace(strings.TrimSuffix(noun, "."))
		// Only single-word nouns we can match as a subtype, to avoid
		// misreading complex clauses.
		if noun == "" || strings.Contains(noun, " ") {
			return false, false
		}
		for _, p := range seat.Battlefield {
			if p != nil && p.Card != nil && cardHasType(p.Card, noun) {
				return true, true
			}
		}
		return false, true
	}
	return false, false
}

// --- small helpers ------------------------------------------------------

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func countGYWhere(seat *Seat, pred func(*Card) bool) int {
	if seat == nil {
		return 0
	}
	n := 0
	for _, c := range seat.Graveyard {
		if c != nil && pred(c) {
			n++
		}
	}
	return n
}

// bracedGenericAmount parses the generic-mana amount from a "{N}" string.
// "{3}" → 3. A colored-only cost ("{W}{W}") counts each pip as 1. Returns 0
// when nothing parseable.
func bracedGenericAmount(s string) int {
	s = strings.ToLower(s)
	total := 0
	pips := 0
	for _, part := range strings.Split(s, "}") {
		part = strings.TrimPrefix(strings.TrimSpace(part), "{")
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, ok := atoiSafe(part); ok {
			total += n
		} else {
			// a colored / hybrid / phyrexian pip
			pips++
		}
	}
	if total > 0 {
		return total
	}
	return pips
}

// afterPhrase returns the substring after the first occurrence of phrase.
func afterPhrase(s, phrase string) string {
	i := strings.Index(s, phrase)
	if i < 0 {
		return ""
	}
	return s[i+len(phrase):]
}
