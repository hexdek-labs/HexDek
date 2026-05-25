package hat

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

const (
	isRolloutsPerCandidate = 3
	isRolloutBudgetGe      = 200
	isRolloutCost          = 25
)

// determinize performs an information-set determinization on a cloned game
// state. For each opponent seat, it shuffles their hand back into their
// library and redeals the same number of cards. This samples one possible
// world from the hat's perspective, allowing the rollout to reason over
// the space of opponent hands it doesn't know.
//
// Must be called on a CLONE, never the live game state.
func determinize(gs *gameengine.GameState, perspectiveSeat int, rng *rand.Rand) {
	for i, s := range gs.Seats {
		if i == perspectiveSeat || s == nil || s.Lost || s.LeftGame {
			continue
		}
		handSize := len(s.Hand)
		if handSize == 0 {
			continue
		}

		pool := make([]*gameengine.Card, 0, handSize+len(s.Library))
		pool = append(pool, s.Hand...)
		pool = append(pool, s.Library...)

		rng.Shuffle(len(pool), func(a, b int) {
			pool[a], pool[b] = pool[b], pool[a]
		})

		s.Hand = pool[:handSize]
		s.Library = pool[handSize:]
	}
}

// multiRolloutForCard runs multiple determinized rollouts for a candidate
// cast action and returns the mean evaluation score. Each rollout samples
// a different possible world (opponent hands), producing a more robust
// estimate than a single rollout.
func (h *YggdrasilHat) multiRolloutForCard(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, n int) float64 {
	if n <= 0 {
		n = 1
	}
	var total float64
	for i := 0; i < n; i++ {
		rolloutSeedCounter++
		rng := rand.New(rand.NewSource(int64(gs.Turn)*1000 + int64(seatIdx)*100 + rolloutSeedCounter + int64(i)*7))
		clone := gs.CloneForRollout(rng)
		if clone == nil {
			continue
		}

		determinize(clone, seatIdx, rng)

		for _, s := range clone.Seats {
			if s != nil && s.Hat != nil {
				if yh, ok := s.Hat.(*YggdrasilHat); ok {
					s.Hat = NewYggdrasilHat(yh.Strategy, 0)
				} else if mh, ok := s.Hat.(*MCTSHat); ok {
					s.Hat = mh.Inner
				}
			}
		}

		castIntoClone(clone, seatIdx, card)
		resolveStack(clone)
		gameengine.StateBasedActions(clone)

		depth := rolloutDepth
		if h.Strategy != nil {
			depth = rolloutDepthFor(h.Strategy.Archetype)
		}
		for t := 0; t < depth; t++ {
			if clone.CheckEnd() {
				break
			}
			clone.Active = advanceActive(clone)
			h.TurnRunner(clone)
			gameengine.StateBasedActions(clone)
		}

		total += h.Evaluator.Evaluate(clone, seatIdx)
	}
	return total / float64(n)
}

// castIntoClone pushes a card onto the clone's stack from the casting
// seat's hand. A simplified version of the full cast path — it finds
// the card in hand by name and pushes a stack item.
func castIntoClone(clone *gameengine.GameState, seatIdx int, card *gameengine.Card) {
	seat := clone.Seats[seatIdx]
	for i, c := range seat.Hand {
		if c != nil && c.DisplayName() == card.DisplayName() {
			seat.Hand = append(seat.Hand[:i], seat.Hand[i+1:]...)
			clone.Stack = append(clone.Stack, &gameengine.StackItem{
				Card:       c,
				Controller: seatIdx,
			})
			return
		}
	}
}
