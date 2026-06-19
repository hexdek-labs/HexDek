package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// amplify_r63_test.go — Amplify (CR §702.38, Legions). ApplyAmplify had no
// caller — amplify was inert (no reveal, no counters). These pin: enters with
// amplifyN counters per shared-creature-type hand card, only shared types
// count, the doubler chain, and the reveal-zero → zero-counters edge.

func amplifyCreature(gs *GameState, seat int, name string, n int, subtypes ...string) *Permanent {
	ast := &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "amplify", Args: []interface{}{n}}},
	}
	types := append([]string{"creature"}, subtypes...)
	return addBattlefieldWithAST(gs, seat, name, 5, 5, ast, types...)
}

func amplifyHandCard(gs *GameState, seat, count int, subtypes ...string) {
	for i := 0; i < count; i++ {
		c := &Card{Name: "Hand Creature", Owner: seat, Types: append([]string{"creature"}, subtypes...)}
		gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	}
}

// (1)+(2)+(3) Amplify N enters with N counters per shared-type hand card;
// non-sharing cards don't count.
func TestAmplify_CountersPerSharedTypeCard(t *testing.T) {
	gs := newCombatGame(t)
	amplifyHandCard(gs, 0, 2, "dragon") // 2 Dragons share the type
	amplifyHandCard(gs, 0, 3, "goblin") // 3 Goblins do NOT share

	dragon := amplifyCreature(gs, 0, "Kilnmouth Dragon", 3, "dragon")
	ApplyAmplifyETB(gs, dragon)

	if dragon.Counters["+1/+1"] != 6 {
		t.Fatalf("amplify 3 with 2 shared-type cards should place 6 counters, got %d", dragon.Counters["+1/+1"])
	}
}

// (3) Doubler chain: Doubling Season doubles the amplify counters (proves
// PutCountersTriggered, not raw AddCounter).
func TestAmplify_DoublerApplies(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	amplifyHandCard(gs, 0, 2, "dragon")

	dragon := amplifyCreature(gs, 0, "Kilnmouth Dragon", 3, "dragon")
	ApplyAmplifyETB(gs, dragon)

	if dragon.Counters["+1/+1"] != 12 {
		t.Fatalf("amplify 3 × 2 cards with Doubling Season should be 12, got %d", dragon.Counters["+1/+1"])
	}
}

// (4) Revealing zero (no shared-type cards) is legal and yields zero counters.
func TestAmplify_NoSharedTypeNoCounters(t *testing.T) {
	gs := newCombatGame(t)
	amplifyHandCard(gs, 0, 4, "goblin") // none share with a Dragon

	dragon := amplifyCreature(gs, 0, "Kilnmouth Dragon", 3, "dragon")
	ApplyAmplifyETB(gs, dragon)

	if dragon.Counters["+1/+1"] != 0 {
		t.Fatalf("no shared-type cards should yield zero counters, got %d", dragon.Counters["+1/+1"])
	}
}

// Multi-type sharing: a card sharing ANY of the creature's types counts.
func TestAmplify_SharesAnyType(t *testing.T) {
	gs := newCombatGame(t)
	amplifyHandCard(gs, 0, 1, "zombie") // shares the second type
	amplifyHandCard(gs, 0, 1, "dragon") // shares the first type

	hybrid := amplifyCreature(gs, 0, "Zombie Dragon", 2, "dragon", "zombie")
	ApplyAmplifyETB(gs, hybrid)

	if hybrid.Counters["+1/+1"] != 4 {
		t.Fatalf("amplify 2 with 2 cards each sharing a type should place 4, got %d", hybrid.Counters["+1/+1"])
	}
}

// (5) Integration: amplify fires via the ETB path (FirePermanentETBTriggers).
func TestAmplify_WiredIntoETBPath(t *testing.T) {
	gs := newCombatGame(t)
	amplifyHandCard(gs, 0, 2, "dragon")
	dragon := amplifyCreature(gs, 0, "Kilnmouth Dragon", 3, "dragon")
	FirePermanentETBTriggers(gs, dragon)
	if dragon.Counters["+1/+1"] != 6 {
		t.Fatalf("amplify via FirePermanentETBTriggers should place 6 counters, got %d", dragon.Counters["+1/+1"])
	}
}
