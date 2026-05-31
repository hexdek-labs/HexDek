package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestHapatra_CreatesSnakeOnMinusCounterByController(t *testing.T) {
	gs := newGame(t, 2)
	hapatra := addPerm(gs, 0, "Hapatra, Vizier of Poisons", "creature")
	victim := addPerm(gs, 1, "Grizzly Bears", "creature")

	preTokens := countSnakeTokens(gs, 0)
	hapatraCounterPlaced(gs, hapatra, map[string]interface{}{
		"counter_kind": "-1/-1",
		"target_perm":  victim,
		"source_seat":  0,
		"amount":       1,
	})
	if got := countSnakeTokens(gs, 0); got != preTokens+1 {
		t.Errorf("expected 1 new Snake token, got %d (total before %d)", got-preTokens, preTokens)
	}
}

func TestHapatra_DoesNotFireOnOpponentMinusCounter(t *testing.T) {
	gs := newGame(t, 2)
	hapatra := addPerm(gs, 0, "Hapatra, Vizier of Poisons", "creature")
	victim := addPerm(gs, 0, "Llanowar Elves", "creature")
	hapatraCounterPlaced(gs, hapatra, map[string]interface{}{
		"counter_kind": "-1/-1",
		"target_perm":  victim,
		"source_seat":  1, // opponent placed the counter
	})
	if got := countSnakeTokens(gs, 0); got != 0 {
		t.Errorf("Hapatra must not fire on opponent-placed counters, got %d snakes", got)
	}
}

func TestHapatra_DoesNotFireOnNonMinusCounter(t *testing.T) {
	gs := newGame(t, 2)
	hapatra := addPerm(gs, 0, "Hapatra, Vizier of Poisons", "creature")
	victim := addPerm(gs, 1, "Grizzly Bears", "creature")
	hapatraCounterPlaced(gs, hapatra, map[string]interface{}{
		"counter_kind": "+1/+1",
		"target_perm":  victim,
		"source_seat":  0,
	})
	if got := countSnakeTokens(gs, 0); got != 0 {
		t.Errorf("Hapatra must only react to -1/-1 counters, got %d snakes", got)
	}
}

func TestHapatra_DoesNotFireOnMinusCounterOnNoncreature(t *testing.T) {
	gs := newGame(t, 2)
	hapatra := addPerm(gs, 0, "Hapatra, Vizier of Poisons", "creature")
	// Artifact, not a creature.
	target := addPerm(gs, 1, "Mox Opal", "artifact")
	hapatraCounterPlaced(gs, hapatra, map[string]interface{}{
		"counter_kind": "-1/-1",
		"target_perm":  target,
		"source_seat":  0,
	})
	if got := countSnakeTokens(gs, 0); got != 0 {
		t.Errorf("Hapatra fires only on creatures, got %d snakes", got)
	}
}

func countSnakeTokens(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Snake Token" {
			n++
		}
	}
	return n
}
