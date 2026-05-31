package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// wave2_multistep_batch2_r60_test.go — Wave 2 sac/graveyard-cycle batch.
//
// 7 per_card handlers in the "sac-to-reanimate / exile-cards-as-cost"
// family pre-r60 carried manual graveyard / hand / exile splice +
// append pairs that bypassed §614 replacement (Rest in Peace, Leyline
// of the Void), §903.9b commander redirect, card_exiled / zone_change /
// graveyard_leave observers, and Madness / Mayhem / Necropotence
// redirection. This batch routes every such site through MoveCard,
// DiscardCard, or enterBattlefieldWithETB chokepoints.
//
// Migrated:
//   1. Araumi of the Dead Tide      — encore cost (graveyard → exile, N+1 cards)
//   2. Felothar the Steadfast       — discard cost via DiscardCard
//   3. Sliver Gravemother           — encore cost (graveyard → exile)
//   4. Karmic Guide                 — ETB reanimate (graveyard → battlefield)
//   5. Karador, Ghost Chieftain     — activated reanimate (graveyard → battlefield)
//   6. Alaundo the Seer             — suspend (hand → exile) + release (exile → bf)
//   7. Mairsil, the Pretender       — cage (hand/graveyard → exile)
//
// Each regression pins destination-zone count = 1 and source-zone
// count = 0 for the migrated move (no double-add, no orphan).

// ---------------------------------------------------------------------------
// 1. Araumi
// ---------------------------------------------------------------------------

func TestWave2MS2_Araumi_EncoreExilesTargetAndCostCleanly(t *testing.T) {
	gs := newGame(t, 4) // 3 opponents
	araumi := addPerm(gs, 0, "Araumi of the Dead Tide", "creature")

	target := &gameengine.Card{
		Name: "Big Creature", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     5, BaseToughness: 5,
	}
	cost1 := &gameengine.Card{Name: "Cost1", Owner: 0, Types: []string{"sorcery"}}
	cost2 := &gameengine.Card{Name: "Cost2", Owner: 0, Types: []string{"sorcery"}}
	cost3 := &gameengine.Card{Name: "Cost3", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target, cost1, cost2, cost3)

	araumiEncore(gs, araumi, 0, nil)

	// Target + 3 cost cards must all be in exile exactly once each.
	for _, c := range []*gameengine.Card{target, cost1, cost2, cost3} {
		if countCardIn(gs.Seats[0].Exile, c) != 1 {
			t.Errorf("Araumi: %q must be in exile once; got %d",
				c.Name, countCardIn(gs.Seats[0].Exile, c))
		}
		if countCardIn(gs.Seats[0].Graveyard, c) != 0 {
			t.Errorf("Araumi: %q must NOT be in graveyard; got %d",
				c.Name, countCardIn(gs.Seats[0].Graveyard, c))
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Felothar
// ---------------------------------------------------------------------------

func TestWave2MS2_Felothar_DiscardRoutesThroughDiscardCard(t *testing.T) {
	gs := newGame(t, 2)
	felothar := stampCreaturePT(addPerm(gs, 0, "Felothar the Steadfast", "creature"), 3, 3)
	// Sac target with toughness 1, power 1 → draw 1, discard 1.
	sac := stampCreaturePT(addPerm(gs, 0, "Sac Target", "creature"), 1, 1)
	_ = sac
	// Library cards for the draw.
	gs.Seats[0].Library = append(gs.Seats[0].Library,
		&gameengine.Card{Name: "Drawn", Owner: 0, Types: []string{"sorcery"}})
	// One card in hand to discard.
	discardTarget := &gameengine.Card{Name: "Discard Me", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, discardTarget)
	gs.Seats[0].ManaPool = 3
	preDiscarded := gs.Seats[0].Turn.Discarded

	felotharSacDrawDiscard(gs, felothar, 0, nil)

	if countCardIn(gs.Seats[0].Hand, discardTarget) != 0 {
		t.Errorf("Felothar: discardTarget still in hand")
	}
	if countCardIn(gs.Seats[0].Graveyard, discardTarget) != 1 {
		t.Errorf("Felothar: discardTarget must be in graveyard exactly once; got %d",
			countCardIn(gs.Seats[0].Graveyard, discardTarget))
	}
	// DiscardCard contract: Turn.Discarded increments.
	if gs.Seats[0].Turn.Discarded <= preDiscarded {
		t.Errorf("Felothar: Turn.Discarded must increment via DiscardCard (got %d → %d)",
			preDiscarded, gs.Seats[0].Turn.Discarded)
	}
}

// ---------------------------------------------------------------------------
// 3. Sliver Gravemother
// ---------------------------------------------------------------------------

func TestWave2MS2_SliverGravemother_EncoreExilesGYSliverCleanly(t *testing.T) {
	gs := newGame(t, 4) // 3 opponents
	gravemother := addPerm(gs, 0, "Sliver Gravemother", "creature", "sliver")
	gs.Seats[0].ManaPool = 10
	// Sorcery-speed gate.
	gs.Active = 0
	gs.Phase = "main1"
	gs.Stack = nil

	sliver := &gameengine.Card{
		Name: "Muscle Sliver", Owner: 0,
		Types:         []string{"creature", "sliver"},
		BasePower:     1, BaseToughness: 1,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, sliver)

	sliverGravemotherEncore(gs, gravemother, 0, nil)

	if countCardIn(gs.Seats[0].Graveyard, sliver) != 0 {
		t.Errorf("Gravemother: sliver still in graveyard after encore cost; got %d",
			countCardIn(gs.Seats[0].Graveyard, sliver))
	}
	if countCardIn(gs.Seats[0].Exile, sliver) != 1 {
		t.Errorf("Gravemother: sliver must be in exile exactly once; got %d",
			countCardIn(gs.Seats[0].Exile, sliver))
	}
}

// ---------------------------------------------------------------------------
// 4. Karmic Guide
// ---------------------------------------------------------------------------

func TestWave2MS2_KarmicGuide_ReanimateNoDoubleRefs(t *testing.T) {
	gs := newGame(t, 2)
	guide := addPerm(gs, 0, "Karmic Guide", "creature")
	target := &gameengine.Card{
		Name: "Reveillark", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     4, BaseToughness: 3,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)

	karmicGuideETB(gs, guide)

	if countCardIn(gs.Seats[0].Graveyard, target) != 0 {
		t.Errorf("Karmic Guide: Reveillark still in graveyard after reanimate")
	}
	if countBFCard(gs.Seats[0].Battlefield, target) != 1 {
		t.Errorf("Karmic Guide: Reveillark must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
}

// ---------------------------------------------------------------------------
// 5. Karador
// ---------------------------------------------------------------------------

func TestWave2MS2_Karador_GraveyardCheatNoDoubleRefs(t *testing.T) {
	gs := newGame(t, 2)
	karador := addPerm(gs, 0, "Karador, Ghost Chieftain", "creature")
	karador.Flags = map[string]int{}
	target := &gameengine.Card{
		Name: "Grave Creature", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     2, BaseToughness: 2,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)
	gs.Seats[0].ManaPool = 10

	karadorCastFromGY(gs, karador, 0, nil)

	if countCardIn(gs.Seats[0].Graveyard, target) != 0 {
		t.Errorf("Karador: target still in graveyard after cheat")
	}
	if countBFCard(gs.Seats[0].Battlefield, target) != 1 {
		t.Errorf("Karador: target must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
}

// ---------------------------------------------------------------------------
// 6. Alaundo
// ---------------------------------------------------------------------------

func TestWave2MS2_Alaundo_SuspendHandToExileCleanly(t *testing.T) {
	gs := newGame(t, 2)
	alaundo := addPerm(gs, 0, "Alaundo the Seer", "creature")
	target := &gameengine.Card{
		Name: "To Suspend", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     3, BaseToughness: 3,
		CMC: 3,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, target)
	// Trigger 2nd activation (release every 2 activations) WITHOUT
	// triggering yet — just test the suspend half. Activations starts at 0.

	alaundoTheSeerActivate(gs, alaundo, 0, nil)

	if countCardIn(gs.Seats[0].Hand, target) != 0 {
		t.Errorf("Alaundo: suspend target still in hand")
	}
	if countCardIn(gs.Seats[0].Exile, target) != 1 {
		t.Errorf("Alaundo: suspend target must be in exile once; got %d",
			countCardIn(gs.Seats[0].Exile, target))
	}
}

// ---------------------------------------------------------------------------
// 7. Mairsil
// ---------------------------------------------------------------------------

func TestWave2MS2_Mairsil_CageFromHandRoutesThroughMoveCard(t *testing.T) {
	gs := newGame(t, 2)
	mairsil := addPerm(gs, 0, "Mairsil, the Pretender", "creature")
	target := &gameengine.Card{
		Name: "Caged Artifact", Owner: 0,
		Types: []string{"artifact"},
		CMC:   3,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, target)

	mairsilETB(gs, mairsil)

	if countCardIn(gs.Seats[0].Hand, target) != 0 {
		t.Errorf("Mairsil: cage target still in hand")
	}
	if countCardIn(gs.Seats[0].Exile, target) != 1 {
		t.Errorf("Mairsil: cage target must be in exile once; got %d",
			countCardIn(gs.Seats[0].Exile, target))
	}
	// The cage_counter tag must be on the exiled card.
	foundTag := false
	for _, tt := range target.Types {
		if tt == "cage_counter" {
			foundTag = true
			break
		}
	}
	if !foundTag {
		t.Errorf("Mairsil: cage_counter type tag must be applied to caged card")
	}
}

func TestWave2MS2_Mairsil_CageFromGraveyardRoutesThroughMoveCard(t *testing.T) {
	gs := newGame(t, 2)
	mairsil := addPerm(gs, 0, "Mairsil, the Pretender", "creature")
	// Empty hand → falls back to graveyard.
	target := &gameengine.Card{
		Name: "Caged Creature", Owner: 0,
		Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)

	mairsilETB(gs, mairsil)

	if countCardIn(gs.Seats[0].Graveyard, target) != 0 {
		t.Errorf("Mairsil: cage target still in graveyard")
	}
	if countCardIn(gs.Seats[0].Exile, target) != 1 {
		t.Errorf("Mairsil: cage target must be in exile once; got %d",
			countCardIn(gs.Seats[0].Exile, target))
	}
}
