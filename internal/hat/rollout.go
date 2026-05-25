package hat

import (
	"math/rand"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

const (
	rolloutDepth    = 3 // baseline depth + fallback for nil / unknown archetype
	rolloutBudgetGe = 200
)

// rolloutDepthFor returns the calibrated MCTS rollout depth for a given
// archetype. R60 round 5+ calibration: with all 22 archetypes now custom-
// tuned, the uniform depth-3 rollout window was leaving slow-plan
// archetypes (Control, Stax, Mill, Lifegain, Superfriends, LandsMatter)
// myopic — Smokestack-style lock pieces and Counterspell-style reactive
// plans only show their value across 5+ turns. Compound-plan archetypes
// (Combo, Storm, Aristocrats, Spellslinger, etc.) sit between: depth 4
// captures one tutor+win cycle or one sac chain compounding into a drain
// trigger. Fast-clock archetypes (Aggro, Voltron, Reanimator) stay at 3
// — their kill window fits a 3-turn look-ahead, and any deeper just
// burns budget on attacks that resolve identically.
//
// Cost: rollouts scale linearly with depth. Across the corpus the
// weighted average per-rollout cost rises ~25-30%, but rollouts only
// fire at Budget >= 200 (already the deep-think path), so the absolute
// throughput hit on a tournament gauntlet is bounded.
//
// Unknown / empty archetype falls back to the depth-3 baseline so the
// pre-calibration behavior is preserved for any external caller that
// builds a StrategyProfile without an archetype string.
func rolloutDepthFor(archetype string) int {
	switch archetype {
	case ArchetypeControl,
		ArchetypeStax,
		ArchetypeMill,
		ArchetypeLifegain,
		ArchetypeSuperfriends,
		ArchetypeLandsMatter:
		return 5
	case ArchetypeCombo,
		ArchetypeSpellslinger,
		ArchetypeStorm,
		ArchetypeAristocrats,
		ArchetypeSelfmill,
		ArchetypeEnchantress,
		ArchetypeArtifacts,
		ArchetypeRamp:
		return 4
	default:
		// Fast-clock + baseline: Aggro, Voltron, Reanimator, Midrange,
		// Tribal, Blink, ExtraCombats, GroupHug, CountersMatter, Unknown,
		// and any string not in the explicit lists above.
		return rolloutDepth
	}
}

// TurnRunnerFunc advances a GameState by one full turn including SBAs.
// Injected by the tournament package to avoid a circular import.
type TurnRunnerFunc func(gs *gameengine.GameState)

func (h *MCTSHat) canRollout() bool {
	return h.Budget >= rolloutBudgetGe && h.TurnRunner != nil
}

// simulateRollout clones gs, applies actionFn to the clone, then runs
// rolloutDepth turns and evaluates the resulting position for seatIdx.
// Returns the evaluator score of the terminal clone state.
//
// The per-rollout RNG seed is derived from a HAT-LOCAL counter
// (h.rolloutSeed) — not a package global. Pre-R60r5 this was a shared
// `var rolloutSeedCounter int64` mutated by every hat in the process,
// which (a) caused test-order dependence (one test's rollouts shifted
// the next test's seeds), (b) was an unsynchronized data race under
// parallel tournament runners, and (c) leaked seed state across game
// boundaries within a process. The per-hat counter is reset on
// game_start in ObserveEvent so each game starts from a deterministic
// seed sequence given (Turn, seatIdx, decision-order).
func (h *MCTSHat) simulateRollout(gs *gameengine.GameState, seatIdx int, actionFn func(clone *gameengine.GameState)) float64 {
	h.rolloutSeed++
	rng := rand.New(rand.NewSource(int64(gs.Turn)*1000 + int64(seatIdx)*100 + h.rolloutSeed))
	clone := gs.CloneForRollout(rng)
	if clone == nil {
		return 0
	}

	// Replace hats on the clone with the inner policy (lightweight).
	for _, s := range clone.Seats {
		if s != nil && s.Hat != nil {
			if mh, ok := s.Hat.(*MCTSHat); ok {
				s.Hat = mh.Inner
			}
		}
	}

	// Rollouts are approximations — use a tighter trigger cap so
	// pathological cascades don't burn 30s per rollout turn.
	if clone.Flags == nil {
		clone.Flags = map[string]int{}
	}
	clone.Flags["_rollout_trigger_cap"] = 200

	actionFn(clone)

	// Resolve any items we just pushed onto the stack before running
	// turns — otherwise the cast action never actually takes effect.
	resolveStack(clone)
	gameengine.StateBasedActions(clone)

	depth := rolloutDepth
	if h.Evaluator != nil && h.Evaluator.Strategy != nil {
		depth = rolloutDepthFor(h.Evaluator.Strategy.Archetype)
	}
	for i := 0; i < depth; i++ {
		if clone.CheckEnd() {
			break
		}
		clone.Active = advanceActive(clone)
		h.TurnRunner(clone)
		gameengine.StateBasedActions(clone)
	}

	return h.Evaluator.Evaluate(clone, seatIdx)
}

// resolveStack pops and resolves all stack items in LIFO order. This is a
// simplified resolution — it handles permanent spells landing on the
// battlefield and instant/sorceries going to graveyard. Effects that
// require complex targeting or modes are approximated.
func resolveStack(gs *gameengine.GameState) {
	for len(gs.Stack) > 0 {
		top := gs.Stack[len(gs.Stack)-1]
		gs.Stack = gs.Stack[:len(gs.Stack)-1]

		if top == nil || top.Card == nil || top.Countered {
			continue
		}

		card := top.Card
		ctrl := top.Controller
		if ctrl < 0 || ctrl >= len(gs.Seats) {
			continue
		}
		seat := gs.Seats[ctrl]
		if seat == nil {
			continue
		}

		if isPermanentCard(card) {
			p := &gameengine.Permanent{
				Card:       card,
				Controller: ctrl,
				Owner:      card.Owner,
				Counters:   map[string]int{},
				Flags:      map[string]int{},
			}
			seat.Battlefield = append(seat.Battlefield, p)
		} else {
			seat.Graveyard = append(seat.Graveyard, card)
		}
	}
}

func isPermanentCard(c *gameengine.Card) bool {
	for _, t := range c.Types {
		t = strings.ToLower(t)
		if t == "instant" || t == "sorcery" {
			return false
		}
	}
	return true
}

func advanceActive(gs *gameengine.GameState) int {
	n := len(gs.Seats)
	if n == 0 {
		return 0
	}
	for offset := 1; offset <= n; offset++ {
		next := (gs.Active + offset) % n
		if s := gs.Seats[next]; s != nil && !s.Lost {
			return next
		}
	}
	return gs.Active
}
