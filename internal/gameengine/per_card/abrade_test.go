package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Abrade: modal removal, inert (parsed_tail). Handler prefers destroying the
// best opponent artifact, else 3 damage to the best opponent creature.

func countArtifacts(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && cardHasType(p.Card, "artifact") {
			n++
		}
	}
	return n
}

func TestAbrade_DestroysOpponentArtifact(t *testing.T) {
	gs := newGame(t, 2)
	rock := addPerm(gs, 1, "Mind Stone", "artifact")
	rock.Card.CMC = 2
	_ = rock

	card := addCard(gs, 0, "Abrade", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if countArtifacts(gs, 1) != 0 {
		t.Fatalf("expected opponent artifact destroyed, %d remain", countArtifacts(gs, 1))
	}
}

func TestAbrade_DamagesCreatureWhenNoArtifact(t *testing.T) {
	gs := newGame(t, 2)
	bear := addPerm(gs, 1, "Grizzly Bears", "creature")
	bear.Card.BasePower, bear.Card.BaseToughness = 2, 2

	card := addCard(gs, 0, "Abrade", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if bear.MarkedDamage != 3 {
		t.Fatalf("expected 3 damage marked on opponent creature, got %d", bear.MarkedDamage)
	}
}

func TestAbrade_PrefersArtifactOverCreature(t *testing.T) {
	gs := newGame(t, 2)
	art := addPerm(gs, 1, "Sol Ring", "artifact")
	art.Card.CMC = 1
	bear := addPerm(gs, 1, "Grizzly Bears", "creature")
	bear.Card.BasePower, bear.Card.BaseToughness = 2, 2

	card := addCard(gs, 0, "Abrade", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if countArtifacts(gs, 1) != 0 {
		t.Fatalf("Abrade should prefer destroying the artifact")
	}
	if bear.MarkedDamage != 0 {
		t.Fatalf("creature should be untouched when artifact mode chosen, got %d", bear.MarkedDamage)
	}
}
