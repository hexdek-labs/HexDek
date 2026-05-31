package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestSekKuar_NontokenCreatureDeathSpawnsGraveborn(t *testing.T) {
	gs := newGame(t, 2)
	sek := addPerm(gs, 0, "Sek'Kuar, Deathkeeper", "creature")
	dying := addPerm(gs, 0, "Llanowar Elves", "creature")
	sekKuarCreatureDies(gs, sek, map[string]interface{}{"perm": dying})
	if got := countGravebornTokens(gs, 0); got != 1 {
		t.Errorf("expected 1 Graveborn token, got %d", got)
	}
}

func TestSekKuar_DoesNotFireOnTokenDeath(t *testing.T) {
	gs := newGame(t, 2)
	sek := addPerm(gs, 0, "Sek'Kuar, Deathkeeper", "creature")
	tokenDying := addPerm(gs, 0, "Some Token", "creature", "token")
	sekKuarCreatureDies(gs, sek, map[string]interface{}{"perm": tokenDying})
	if got := countGravebornTokens(gs, 0); got != 0 {
		t.Errorf("token death must not trigger Sek'Kuar, got %d Graveborn", got)
	}
}

func TestSekKuar_DoesNotFireOnOpponentCreatureDeath(t *testing.T) {
	gs := newGame(t, 2)
	sek := addPerm(gs, 0, "Sek'Kuar, Deathkeeper", "creature")
	oppDying := addPerm(gs, 1, "Opp Creature", "creature")
	sekKuarCreatureDies(gs, sek, map[string]interface{}{"perm": oppDying})
	if got := countGravebornTokens(gs, 0); got != 0 {
		t.Errorf("opponent's creature death must not trigger Sek'Kuar, got %d", got)
	}
}

func TestSekKuar_DoesNotFireOnSelfDeath(t *testing.T) {
	gs := newGame(t, 2)
	sek := addPerm(gs, 0, "Sek'Kuar, Deathkeeper", "creature")
	sekKuarCreatureDies(gs, sek, map[string]interface{}{"perm": sek})
	if got := countGravebornTokens(gs, 0); got != 0 {
		t.Errorf("Sek'Kuar must not fire on his own death (\"another\")")
	}
}

func TestSekKuar_GravebornHasHaste(t *testing.T) {
	gs := newGame(t, 2)
	sek := addPerm(gs, 0, "Sek'Kuar, Deathkeeper", "creature")
	dying := addPerm(gs, 0, "Llanowar Elves", "creature")
	sekKuarCreatureDies(gs, sek, map[string]interface{}{"perm": dying})
	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Graveborn Token" {
			token = p
		}
	}
	if token == nil || token.SummoningSick {
		t.Errorf("Graveborn must enter with haste (no summoning sickness)")
	}
}

func countGravebornTokens(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Graveborn Token" {
			n++
		}
	}
	return n
}
