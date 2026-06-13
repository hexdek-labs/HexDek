package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Generic etb_tapped_unless coverage: a land with this static enters tapped
// when its "unless" condition is not met, untapped when it is. Before this
// handler all ~105 such lands entered untapped for free.

// mkTaplandUnless builds an unattached land permanent (seat 0) carrying an
// etb_tapped_unless static with the given raw condition. It is NOT placed on
// the battlefield (so it is not counted as an "other land" against itself).
func mkTaplandUnless(gs *GameState, seat int, cond string) *Permanent {
	card := &Card{
		Name:  "Test Tapland",
		Owner: seat,
		Types: []string{"land"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Modification: &gameast.Modification{
					ModKind: "etb_tapped_unless",
					Args:    []interface{}{cond},
				}},
			},
		},
	}
	return &Permanent{
		Card: card, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
}

func TestEtbTappedUnless_Checkland_ConditionMet(t *testing.T) {
	gs := newTestGame(t, 2)
	// Controller already controls an Island → Glacial-Fortress-style checkland
	// enters untapped.
	isl := addTestPerm(gs, 0, "Island", "land", "island")
	isl.Card.TypeLine = "Basic Land — Island"

	p := mkTaplandUnless(gs, 0, "you control a plains or an island")
	ApplyEntersTappedUnless(gs, p)
	if p.Tapped {
		t.Fatalf("checkland should enter UNTAPPED when you control an Island")
	}
}

func TestEtbTappedUnless_Checkland_ConditionNotMet(t *testing.T) {
	gs := newTestGame(t, 2)
	// No Plains/Island controlled → enters tapped.
	p := mkTaplandUnless(gs, 0, "you control a plains or an island")
	ApplyEntersTappedUnless(gs, p)
	if !p.Tapped {
		t.Fatalf("checkland should enter TAPPED when you control no Plains/Island")
	}
	if p.Flags["enters_tapped"] != 1 {
		t.Fatalf("expected enters_tapped flag set")
	}
}

func TestEtbTappedUnless_Fastland_TwoOrFewerOtherLands(t *testing.T) {
	gs := newTestGame(t, 2)
	// Fastland: enters untapped if you control two or fewer OTHER lands.
	addTestPerm(gs, 0, "Forest", "land")
	addTestPerm(gs, 0, "Forest", "land") // 2 other lands → untapped
	p := mkTaplandUnless(gs, 0, "you control two or fewer other lands")
	ApplyEntersTappedUnless(gs, p)
	if p.Tapped {
		t.Fatalf("fastland with 2 other lands should enter UNTAPPED")
	}

	// 3 other lands → enters tapped.
	gs2 := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		addTestPerm(gs2, 0, "Forest", "land")
	}
	p2 := mkTaplandUnless(gs2, 0, "you control two or fewer other lands")
	ApplyEntersTappedUnless(gs2, p2)
	if !p2.Tapped {
		t.Fatalf("fastland with 3 other lands should enter TAPPED")
	}
}

func TestEtbTappedUnless_LifeTotalLand(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Life = 12 // a player has 13 or less life → untapped
	p := mkTaplandUnless(gs, 0, "a player has 13 or less life")
	ApplyEntersTappedUnless(gs, p)
	if p.Tapped {
		t.Fatalf("life-total land should enter UNTAPPED when a player has <=13 life")
	}

	gs2 := newTestGame(t, 2) // everyone at starting life (40) → tapped
	p2 := mkTaplandUnless(gs2, 0, "a player has 13 or less life")
	ApplyEntersTappedUnless(gs2, p2)
	if !p2.Tapped {
		t.Fatalf("life-total land should enter TAPPED when nobody is at <=13 life")
	}
}

func TestEtbTappedUnless_TwoOrMoreOpponents(t *testing.T) {
	gs4 := newTestGame(t, 4)
	p := mkTaplandUnless(gs4, 0, "you have two or more opponents")
	ApplyEntersTappedUnless(gs4, p)
	if p.Tapped {
		t.Fatalf("with 3 opponents the land should enter UNTAPPED")
	}

	gs2 := newTestGame(t, 2) // only 1 opponent → tapped
	p2 := mkTaplandUnless(gs2, 0, "you have two or more opponents")
	ApplyEntersTappedUnless(gs2, p2)
	if !p2.Tapped {
		t.Fatalf("with 1 opponent the land should enter TAPPED")
	}
}

func TestEtbTappedUnless_LegendaryCreature(t *testing.T) {
	gs := newTestGame(t, 2)
	c := addTestPerm(gs, 0, "Some Commander", "creature", "legendary")
	c.Card.BasePower = 2
	c.Card.BaseToughness = 2
	p := mkTaplandUnless(gs, 0, "you control a legendary creature")
	ApplyEntersTappedUnless(gs, p)
	if p.Tapped {
		t.Fatalf("should enter UNTAPPED when you control a legendary creature")
	}
}

func TestEtbTappedUnless_BasicLandCount(t *testing.T) {
	gs := newTestGame(t, 2)
	b1 := addTestPerm(gs, 0, "Plains", "land", "basic")
	b1.Card.TypeLine = "Basic Land — Plains"
	b2 := addTestPerm(gs, 0, "Island", "land", "basic")
	b2.Card.TypeLine = "Basic Land — Island"
	p := mkTaplandUnless(gs, 0, "you control two or more basic lands")
	ApplyEntersTappedUnless(gs, p)
	if p.Tapped {
		t.Fatalf("should enter UNTAPPED with two basic lands")
	}
}

func TestEtbTappedUnless_UnparseableDefaultsTapped(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkTaplandUnless(gs, 0, "some bizarre condition the parser does not know")
	ApplyEntersTappedUnless(gs, p)
	if !p.Tapped {
		t.Fatalf("unparseable condition must conservatively default to TAPPED")
	}
}

func TestEtbTappedUnless_WiredIntoETBPath(t *testing.T) {
	// End-to-end through FirePermanentETBTriggers: no Plains/Island → tapped.
	gs := newTestGame(t, 2)
	p := mkTaplandUnless(gs, 0, "you control a plains or an island")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	FirePermanentETBTriggers(gs, p)
	if !p.Tapped {
		t.Fatalf("etb_tapped_unless must fire through the ETB dispatch path")
	}
}
