package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CR §702.140d: when a merged (mutated) permanent leaves the battlefield,
// EVERY card in the pile goes to its owner's zone — not just the top card.
// Regression for the r63 mutate probe: the SBA death path (destroyPermSBA
// — lethal combat damage / 0-toughness, the most common way a creature
// dies) did NOT call UnmergeOnLeavePlay, so an under-card was stranded in
// "merged limbo" and vanished from the census (a ZoneConservation leak).
// DestroyPermanent / ExilePermanent already unmerged; the SBA path didn't.

func ms_mutateCardWithID(seat int, name, instID string, pow, tough int) *Card {
	return &Card{
		Name:          name,
		Owner:         seat,
		InstanceID:    instID,
		CMC:           4,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         []string{"creature", "elemental"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "mutate", Args: []any{float64(0)}}},
		},
	}
}

func TestMutate_SBADeath_UnmergesAllCards(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)

	// Target vanilla non-Human creature on the battlefield (survivor when
	// onTop=false), toughness 2 so lethal damage is easy to apply.
	targetCard := &Card{
		Name: "Pollywog Symbiote", Owner: 0, InstanceID: "ms-target-1",
		BasePower: 1, BaseToughness: 2, Types: []string{"creature", "frog"},
		AST: &gameast.CardAST{Name: "Pollywog Symbiote"},
	}
	target := &Permanent{
		Card: targetCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)

	// Mutating card in hand; slides UNDER the target.
	under := ms_mutateCardWithID(0, "Pondster", "ms-under-1", 3, 3)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, under)

	survivor, err := CastWithMutate(gs, 0, under, 0, target, false /*onTop*/)
	if err != nil {
		t.Fatalf("CastWithMutate failed: %v", err)
	}
	if survivor != target || survivor.MergeKind == MergeNone {
		t.Fatalf("expected merged target survivor; merge=%v", survivor.MergeKind)
	}

	// Precondition: under-card is in limbo (not in any visible zone yet).
	if inGraveyard(gs, 0, "Pondster") {
		t.Fatalf("under-card should be in merge limbo, not graveyard, pre-death")
	}

	// Kill the merged permanent via SBA lethal damage (§704.5g).
	survivor.MarkedDamage = survivor.Toughness()
	StateBasedActions(gs)

	// Both constituent cards must now be in seat 0's graveyard.
	if !inGraveyard(gs, 0, "Pollywog Symbiote") {
		t.Errorf("top card (Pollywog Symbiote) missing from graveyard after SBA death")
	}
	if !inGraveyard(gs, 0, "Pondster") {
		t.Errorf("under card (Pondster) leaked — not routed to graveyard on SBA death (CR §702.140d)")
	}
}

func inGraveyard(gs *GameState, seat int, name string) bool {
	for _, c := range gs.Seats[seat].Graveyard {
		if c != nil && c.DisplayName() == name {
			return true
		}
	}
	return false
}
