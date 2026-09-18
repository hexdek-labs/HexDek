package gameengine

import "testing"

func countHumanSoldierTokens(gs *GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Human Soldier" {
			n++
		}
	}
	return n
}

// TestRecruit_701_70a pins the Recruit keyword action: draw, discard, and if a
// NONLAND was discarded, make a 1/1 white Human Soldier.
func TestRecruit_NonlandDiscardMakesToken(t *testing.T) {
	gs := newCombatGame(t)
	gs.Seats[0].Hat = nil // deterministic: discard fallback = Hand[0] = the drawn card
	gs.Seats[0].Hand = nil
	gs.Seats[0].Library = []*Card{{Name: "Shock", Owner: 0, Types: []string{"instant"}}}

	PerformRecruit(gs, 0)

	if got := countHumanSoldierTokens(gs, 0); got != 1 {
		t.Errorf("discarding a nonland should make 1 Human Soldier, got %d", got)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("hand should be empty after draw+discard, got %d", len(gs.Seats[0].Hand))
	}
}

func TestRecruit_LandDiscardMakesNoToken(t *testing.T) {
	gs := newCombatGame(t)
	gs.Seats[0].Hat = nil
	gs.Seats[0].Hand = nil
	gs.Seats[0].Library = []*Card{{Name: "Forest", Owner: 0, Types: []string{"land"}}}

	PerformRecruit(gs, 0)

	if got := countHumanSoldierTokens(gs, 0); got != 0 {
		t.Errorf("discarding a land should make no token, got %d", got)
	}
}
