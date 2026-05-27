package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCrypticCommand wires Cryptic Command.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Cryptic%20Command):
//
//	Choose two —
//	  • Counter target spell.
//	  • Return target permanent to its owner's hand.
//	  • Tap all creatures your opponents control.
//	  • Draw a card.
//
// {1}{U}{U}{U} Instant. The original 4-mode flex spell. cEDH plays as
// a 1-card tempo swing; the canonical "draw + counter" or "tap +
// bounce" lines are commander-defining for U/x control shells.
//
// Implementation extends the Mystic Confluence pattern with two key
// changes:
//   - 4 modes instead of 3, choose 2 distinct (Cryptic Command does
//     NOT allow repeats — different from Mystic Confluence).
//   - Adds the "tap all opp creatures" mode (sweep all seats' battle-
//     fields and tap untapped opp creatures).
//
// Mode-scoring heuristic same shape as Mystic Confluence:
//   - counter: 30 if a counterable opp spell exists, 0 otherwise
//   - bounce:  10 + perm.Power() if a legal opp permanent exists
//   - tap:     2 * count of untapped opp creatures
//   - draw:    5 (steady value)
//
// Pick top 2 scored modes WITHOUT replacement. Modes that fizzle at
// resolve fall back to draw so the spell always lands 2 effects.
func registerCrypticCommand(r *Registry) {
	r.OnResolve("Cryptic Command", crypticCommandResolve)
}

func crypticCommandResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "cryptic_command"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Score all 4 modes.
	scores := map[string]int{}
	if target := findCounterableSpell(gs, seat, nil); target != nil {
		scores["counter"] = 30
	}
	if best := pickCrypticCommandBounceTarget(gs, seat); best != nil {
		scores["bounce"] = 10 + best.Power()
	}
	untapped := countUntappedOppCreatures(gs, seat)
	if untapped > 0 {
		scores["tap"] = 2 * untapped
	}
	scores["draw"] = 5

	// Pick top 2 distinct modes by score.
	picked := pickTop2Modes(scores)
	chosen := []string{}
	for _, mode := range picked {
		switch mode {
		case "counter":
			target := findCounterableSpell(gs, seat, nil)
			if target == nil {
				drawOne(gs, seat, "Cryptic Command")
				chosen = append(chosen, "draw_fallback")
				continue
			}
			target.Countered = true
			emitCounter(gs, slug, "Cryptic Command", seat, target)
			chosen = append(chosen, "counter")
		case "bounce":
			best := pickCrypticCommandBounceTarget(gs, seat)
			if best == nil {
				drawOne(gs, seat, "Cryptic Command")
				chosen = append(chosen, "draw_fallback")
				continue
			}
			gameengine.BouncePermanent(gs, best, nil, "hand")
			chosen = append(chosen, "bounce")
		case "tap":
			n := tapAllOppCreatures(gs, seat)
			chosen = append(chosen, "tap")
			gs.LogEvent(gameengine.Event{
				Kind:   "tap_all_opp_creatures",
				Seat:   seat,
				Source: "Cryptic Command",
				Amount: n,
			})
		case "draw":
			drawOne(gs, seat, "Cryptic Command")
			chosen = append(chosen, "draw")
		}
	}

	emit(gs, slug, "Cryptic Command", map[string]interface{}{
		"seat":  seat,
		"modes": chosen,
	})
}

// pickCrypticCommandBounceTarget — Cryptic bounces ANY permanent, not
// just creatures (differs from Mystic Confluence). Picks highest-value
// opponent permanent — prioritize commander then creature by power
// then any nonland permanent.
func pickCrypticCommandBounceTarget(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	if gs == nil {
		return nil
	}
	var bestCreature *gameengine.Permanent
	var bestOther *gameengine.Permanent
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.IsLand() {
				continue
			}
			if p.IsCreature() {
				if bestCreature == nil || p.Power() > bestCreature.Power() {
					bestCreature = p
				}
			} else if bestOther == nil {
				bestOther = p
			}
		}
	}
	if bestCreature != nil {
		return bestCreature
	}
	return bestOther
}

func countUntappedOppCreatures(gs *gameengine.GameState, seat int) int {
	n := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() || p.Tapped {
				continue
			}
			n++
		}
	}
	return n
}

func tapAllOppCreatures(gs *gameengine.GameState, seat int) int {
	tapped := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() || p.Tapped {
				continue
			}
			p.Tapped = true
			tapped++
		}
	}
	return tapped
}

// pickTop2Modes returns the 2 highest-scoring modes (distinct). Ties
// broken by stable mode order: counter > bounce > tap > draw.
func pickTop2Modes(scores map[string]int) []string {
	order := []string{"counter", "bounce", "tap", "draw"}
	// Selection sort descending by score with stable tie-break order.
	picks := []string{}
	used := map[string]bool{}
	for k := 0; k < 2; k++ {
		bestScore := -1
		bestMode := ""
		for _, m := range order {
			if used[m] {
				continue
			}
			s := scores[m]
			if s > bestScore {
				bestScore = s
				bestMode = m
			}
		}
		if bestMode == "" {
			break
		}
		picks = append(picks, bestMode)
		used[bestMode] = true
	}
	return picks
}
