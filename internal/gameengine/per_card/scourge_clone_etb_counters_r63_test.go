package per_card

// scourge_clone_etb_counters_r63_test.go — regression for the r63
// long-tail bug (longtail-sweep cluster #2, seed 39340043): a permanent
// that ENTERS as a copy of a creature with an "enters with N +1/+1
// counters" self-replacement (Scourge of Skola Vale) must enter WITH those
// counters (CR §706.2 copiable values + §614.1c as-enters self-replacement).
//
// Root cause: the "enters as a copy" paths swap perm.Card to the copy
// AFTER the stock cast-path ApplyStaticETBCounters (stack.go) already ran
// on the original (vanilla) clone body, so the COPIED card's as-enters
// counters were never placed. fireETBOnCopy — the shared chokepoint every
// clone path reaches after the swap — now applies them.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// scourgeLikeCard builds a creature whose AST carries the
// "enters with two +1/+1 counters" self-replacement (Scourge of Skola
// Vale's shape).
func scourgeLikeCard(owner int) *gameengine.Card {
	return &gameengine.Card{
		Name:          "Scourge of Skola Vale",
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     0,
		BaseToughness: 0,
		AST: &gameast.CardAST{
			Name: "Scourge of Skola Vale",
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "etb_with_counters",
						Args:    []interface{}{float64(2), "+1/+1"},
					},
				},
			},
		},
	}
}

func TestPhantasmalImage_CopyOfEntersWithCounters(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)

	// Copy target: a creature that enters with two +1/+1 counters.
	target := &gameengine.Permanent{
		Card: scourgeLikeCard(0), Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)

	// Phantasmal Image enters and auto-copies the target creature.
	image := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Phantasmal Image", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, image)

	phantasmalImageETB(gs, image)

	if image.Card.DisplayName() != "Scourge of Skola Vale" {
		t.Fatalf("Image did not become a copy of Scourge, got %q", image.Card.DisplayName())
	}
	if got := image.Counters["+1/+1"]; got != 2 {
		t.Fatalf("copy of an enters-with-2-counters creature has %d +1/+1 counter(s), want 2 (CR 706.2/614.1c)", got)
	}
}

// TestCloneVariant_CopyOfEntersWithCounters exercises the same fix through
// the generic cloneVariantETB path (Clone / Sakashima family) rather than
// Phantasmal Image — both route through fireETBOnCopy.
func TestCloneVariant_CopyOfEntersWithCounters(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)

	target := &gameengine.Permanent{
		Card: scourgeLikeCard(0), Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)

	clone := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Clone", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, clone)

	// cloneVariantETB: copy any creature (allow-all predicate).
	cloneVariantETB(gs, clone, "test_clone", func(cand *gameengine.Permanent) bool {
		return cand != nil && cand.IsCreature()
	}, nil)

	if clone.Card.DisplayName() != "Scourge of Skola Vale" {
		t.Fatalf("Clone did not become a copy of Scourge, got %q", clone.Card.DisplayName())
	}
	if got := clone.Counters["+1/+1"]; got != 2 {
		t.Fatalf("clone of an enters-with-2-counters creature has %d +1/+1 counter(s), want 2 (CR 706.2/614.1c)", got)
	}
}
