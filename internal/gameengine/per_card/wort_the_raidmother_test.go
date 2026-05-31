package per_card

import (
	"testing"
)

func TestWort_ETBSpawnsTwoGoblinWarriors(t *testing.T) {
	gs := newGame(t, 2)
	wort := addPerm(gs, 0, "Wort, the Raidmother", "creature", "goblin")
	wortETB(gs, wort)
	count := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Goblin Warrior Token" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 Goblin Warrior tokens, got %d", count)
	}
}

func TestWort_ConspireGrantEmitsPartial(t *testing.T) {
	gs := newGame(t, 2)
	wort := addPerm(gs, 0, "Wort, the Raidmother", "creature")
	wortETB(gs, wort)
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" {
			if d, ok := ev.Details["slug"].(string); ok && d == "wort_etb_two_goblin_warriors" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_partial for the deferred conspire grant")
	}
}
