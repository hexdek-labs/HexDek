package hat

import (
	"math/rand"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

const (
	rolloutDepth    = 3
	rolloutBudgetGe = 200
)

// TurnRunnerFunc advances a GameState by one full turn including SBAs.
// Injected by the tournament package to avoid a circular import.
type TurnRunnerFunc func(gs *gameengine.GameState)

func (h *MCTSHat) canRollout() bool {
	return h.Budget >= rolloutBudgetGe && h.TurnRunner != nil
}

// rolloutSeedCounter is bumped per-rollout within a decision to give each
// candidate a different RNG stream.
var rolloutSeedCounter int64

// simulateRollout clones gs, applies actionFn to the clone, then runs
// rolloutDepth turns and evaluates the resulting position for seatIdx.
// Returns the evaluator score of the terminal clone state.
func (h *MCTSHat) simulateRollout(gs *gameengine.GameState, seatIdx int, actionFn func(clone *gameengine.GameState)) float64 {
	rolloutSeedCounter++
	rng := rand.New(rand.NewSource(int64(gs.Turn)*1000 + int64(seatIdx)*100 + rolloutSeedCounter))
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

	actionFn(clone)

	// Resolve any items we just pushed onto the stack before running
	// turns — otherwise the cast action never actually takes effect.
	resolveStack(clone)
	gameengine.StateBasedActions(clone)

	for i := 0; i < rolloutDepth; i++ {
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
