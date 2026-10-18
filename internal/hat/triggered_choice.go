package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Triggered-ability and modal-effect scoring intelligence.
//
// This file holds the helpers that turn ChooseMode / ShouldTriggerOptional
// from a fixed-score lookup into a board-aware decision. Each helper
// answers one question about the current board state — "what's the best
// thing we could destroy?", "do we have an aristocrats payoff up?",
// "how close is the closest opponent to lethal?" — and the upgraded
// scoreModeEffect composes them into the per-mode score.
//
// CurseDNA axes are folded in at the end so personality nudges shift
// the picks of evolved hats: high DrainAffinity boosts sacrifice and
// damage modes, high TokenPressure boosts create_token, etc.

// -----------------------------------------------------------------------------
// Board-state helpers
// -----------------------------------------------------------------------------

// bestOpponentRemovalScore returns a 0..1 score reflecting the value of
// the best legal opponent permanent we could destroy or exile.
//   - 0.95 when the opponent has a combo-relevant card on board
//   - 0.80 when they have a value engine / commander
//   - 0.60 when they have a big creature (power ≥ 4) or planeswalker
//   - 0.50 when they have any non-vanilla creature
//   - 0.40 vanilla creatures only
//   - 0.00 when no legal opponent permanent exists
func (h *YggdrasilHat) bestOpponentRemovalScore(gs *gameengine.GameState, seatIdx int) float64 {
	if gs == nil {
		return 0
	}
	best := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			score := 0.40
			if p.IsCreature() {
				if gs.PowerOf(p) >= 4 {
					score = 0.60
				}
			}
			if p.IsPlaneswalker() {
				score = 0.60
			}
			if h.isComboRelevant(p.Card) {
				score = 0.95
			} else if h.isValueEngineKey(p.Card) {
				score = 0.80
			} else if isCommanderCard(gs, i, p.Card) {
				if score < 0.75 {
					score = 0.75
				}
			} else if p.Card.AST != nil {
				// Has triggered abilities → engine piece.
				// r180a — the disjunction is a pure function of the lowered
				// oracle text, so it is memoized per Card. The AST != nil
				// guard above is kept verbatim: OracleTextLower returns ""
				// for a nil AST so the predicate would be false anyway, but
				// keeping the guard makes the diff behaviour-preserving on
				// its face.
				if cachedHasTriggerWording(p.Card) {
					if score < 0.55 {
						score = 0.55
					}
				}
			}
			if score > best {
				best = score
			}
		}
	}
	return best
}

// hasAristocratsPayoff reports whether seat has at least one
// "creature-dies → drain / pump / counter" trigger on the battlefield.
// Used to up-rate sacrifice modes when the death matters to us.
func (h *YggdrasilHat) hasAristocratsPayoff(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if !strings.Contains(ot, "whenever") && !strings.Contains(ot, "when ") {
			continue
		}
		if !strings.Contains(ot, "creature dies") &&
			!strings.Contains(ot, "creature you control dies") &&
			!strings.Contains(ot, "another creature") {
			continue
		}
		// Death trigger that produces value: drain, pump, draw, counter.
		if strings.Contains(ot, "loses") || strings.Contains(ot, "drain") ||
			strings.Contains(ot, "+1/+1") || strings.Contains(ot, "draw") ||
			strings.Contains(ot, "create") || strings.Contains(ot, "each opponent") {
			return true
		}
	}
	return false
}

// lethalIncomingScore returns a 0..1 score reflecting how lethal a damage
// or lose_life effect of the given amount could be against the closest-
// to-dead opponent. ≥ amount = lethal → 1.0; otherwise scaled.
func lethalIncomingScore(gs *gameengine.GameState, seatIdx int, amount int) float64 {
	if gs == nil || amount <= 0 {
		return 0.4
	}
	bestCloseness := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		life := s.Life
		if life <= 0 {
			continue
		}
		var c float64
		if amount >= life {
			c = 1.0
		} else {
			c = float64(amount) / float64(life)
		}
		if c > bestCloseness {
			bestCloseness = c
		}
	}
	// Compress: even non-lethal pings have some value.
	return 0.35 + 0.65*bestCloseness
}

// minOpponentLibrarySize returns the smallest non-zero library count
// across opponents — used to scale mill effects.
func minOpponentLibrarySize(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil {
		return 0
	}
	best := -1
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		n := len(s.Library)
		if best < 0 || n < best {
			best = n
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

// bestBounceCMC returns the highest CMC permanent we could legally
// return — covers bouncing our own ETB-triggered piece (Cloudstone-
// style) or an opponent's bomb. Higher CMC = more tempo per swap.
func bestBounceCMC(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil {
		return 0
	}
	best := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			c := p.Card.CMC
			if c > best {
				best = c
			}
		}
	}
	return best
}

// hasCounterPayoff reports whether we have a creature on the battlefield
// that benefits from +1/+1 counters (any creature is a baseline target;
// we look for proliferate / counter-payoff text for the bonus weight).
func (h *YggdrasilHat) hasCounterPayoff(gs *gameengine.GameState, seatIdx int) (any bool, bonus bool) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false, false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false, false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsCreature() {
			any = true
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "+1/+1 counter") ||
				strings.Contains(ot, "proliferate") ||
				strings.Contains(ot, "evolve") || strings.Contains(ot, "adapt") {
				bonus = true
			}
		}
	}
	return any, bonus
}

// ourCreatureCount returns the number of creatures we control. Used
// for token / go-wide scoring.
func ourCreatureCount(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			n++
		}
	}
	return n
}

// commanderCaresAboutCreatures reports whether seat's commander oracle
// text mentions creature-count / sacrifice / die / +1/+1 — heuristics
// for "this deck wants more creatures".
func commanderCaresAboutCreatures(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, c := range seat.CommandZone {
		if c == nil {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		if strings.Contains(ot, "creature") &&
			(strings.Contains(ot, "you control") || strings.Contains(ot, "dies") ||
				strings.Contains(ot, "+1/+1") || strings.Contains(ot, "sacrifice") ||
				strings.Contains(ot, "attacks") || strings.Contains(ot, "create")) {
			return true
		}
	}
	return false
}

// effectAmount best-effort extracts a small integer from a NumberOrRef
// for scoring purposes. Falls back to 1 for X / scaling / unknown.
func effectAmount(n gameast.NumberOrRef) int {
	if v, ok := n.IntVal(); ok {
		return v
	}
	return 1
}

// -----------------------------------------------------------------------------
// CurseDNA axis nudges
// -----------------------------------------------------------------------------

// applyDNANudge folds CurseDNA personality axes into the mode score.
// Axes range over [0,1]; the nudge size is bounded so a single axis
// can shift a score by at most ~0.15. Multiple axes can stack — a
// drain-affinity + token-pressure deck slamming a sacrifice-then-
// create-token mode gets both pushes.
func (h *YggdrasilHat) applyDNANudge(eff gameast.Effect, score float64) float64 {
	if h == nil || h.DNA == nil {
		return score
	}
	const maxAxisShift = 0.15
	axisShift := func(axis float64) float64 {
		// axis [0,1] → centered on 0.5, scaled to [-maxAxisShift, +maxAxisShift].
		return (axis - 0.5) * 2 * maxAxisShift
	}
	switch eff.Kind() {
	case "sacrifice":
		score += axisShift(h.DNA.DrainAffinity)
	case "damage", "lose_life":
		score += axisShift(h.DNA.DrainAffinity)
	case "draw":
		score += axisShift(h.DNA.ResourceGreed)
	case "add_mana":
		score += axisShift(h.DNA.ResourceGreed)
	case "create_token":
		score += axisShift(h.DNA.TokenPressure)
	case "reanimate", "recurse":
		score += axisShift(h.DNA.GraveyardExploitation)
	case "mill":
		// Self-mill is value to graveyard decks; opp-mill is incidental.
		// We don't know the target side here without inspecting the
		// effect's target filter, but in modal contexts the controller
		// usually has the choice — boost if they're a graveyard deck.
		score += axisShift(h.DNA.GraveyardExploitation)
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// -----------------------------------------------------------------------------
// ShouldTriggerOptional — "you may" net-positive evaluator.
// -----------------------------------------------------------------------------

// ShouldTriggerOptional returns true when an optional ("you may") trigger
// is net-positive for the controller. Engine plumbing for this hook is
// pending — when the optional_effect resolver in resolve_helpers.go
// learns to consult a hat method here, this evaluator already gates
// the choice. Until then it's exercised by tests and provides a clear
// API surface.
//
// Decision rule:
//   - score the effect via scoreModeEffect (board-aware)
//   - require ≥ 0.50 baseline; lower the bar to 0.35 when behind
//   - reject under 0.20 even when desperate (small chance of accepting
//     for variance)
func (h *YggdrasilHat) ShouldTriggerOptional(gs *gameengine.GameState, seatIdx int, effect gameast.Effect) bool {
	if effect == nil {
		return false
	}
	pos := h.evalPosition(gs, seatIdx)
	score := h.scoreModeEffect(gs, seatIdx, effect, pos)
	threshold := 0.50
	if pos < -0.2 {
		threshold = 0.35
	}
	if pos > 0.3 {
		threshold = 0.55
	}
	return score >= threshold
}
