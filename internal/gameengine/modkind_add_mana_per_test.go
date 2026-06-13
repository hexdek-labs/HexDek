package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// modkind_add_mana_per_test.go — pins the generic add_mana_per mana
// handler. Each scaled mana source added ZERO mana before this seam.

func ampPerm(gs *GameState, seat int, name string, types ...string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Owner: seat, Types: append([]string{}, types...)},
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func runAddManaPer(gs *GameState, src *Permanent, color, basis string) {
	resolveModificationEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "add_mana_per",
		Args:    []interface{}{color, basis},
	})
}

func TestAddManaPer_CreaturesYouControl_GaeasCradle(t *testing.T) {
	gs := newTestGameState(2)
	cradle := ampPerm(gs, 0, "Gaea's Cradle", "land")
	ampPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	ampPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	ampPerm(gs, 0, "Birds of Paradise", "creature", "bird")
	// An opponent creature must NOT count for "you control".
	ampPerm(gs, 1, "Goblin", "creature", "goblin")

	runAddManaPer(gs, cradle, "{G}", "creature you control")

	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("Gaea's Cradle with 3 creatures should add 3 mana, got %d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Mana == nil || gs.Seats[0].Mana.G != 3 {
		t.Errorf("expected 3 GREEN in typed pool, got %+v", gs.Seats[0].Mana)
	}
}

func TestAddManaPer_SwampsYouControl_CabalCoffers(t *testing.T) {
	gs := newTestGameState(2)
	coffers := ampPerm(gs, 0, "Cabal Coffers", "land")
	ampPerm(gs, 0, "Swamp", "land", "swamp")
	ampPerm(gs, 0, "Swamp", "land", "swamp")
	ampPerm(gs, 0, "Forest", "land", "forest") // not a Swamp

	runAddManaPer(gs, coffers, "{B}", "Swamp you control")

	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("Cabal Coffers with 2 Swamps should add 2 mana, got %d", gs.Seats[0].ManaPool)
	}
}

func TestAddManaPer_BasicSwampOnly_CabalStronghold(t *testing.T) {
	gs := newTestGameState(2)
	stronghold := ampPerm(gs, 0, "Cabal Stronghold", "land")
	ampPerm(gs, 0, "Swamp", "land", "basic", "swamp")
	ampPerm(gs, 0, "Swamp", "land", "basic", "swamp")
	ampPerm(gs, 0, "Watery Grave", "land", "swamp") // nonbasic Swamp — excluded

	runAddManaPer(gs, stronghold, "{B}", "basic Swamp you control")

	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("Cabal Stronghold should count only basic Swamps (2), got %d", gs.Seats[0].ManaPool)
	}
}

func TestAddManaPer_CountersOnSelf_EverflowingChalice(t *testing.T) {
	gs := newTestGameState(2)
	chalice := ampPerm(gs, 0, "Everflowing Chalice", "artifact")
	chalice.Counters["charge"] = 4

	runAddManaPer(gs, chalice, "{C}", "charge counter on this artifact")

	if gs.Seats[0].ManaPool != 4 {
		t.Fatalf("Everflowing Chalice with 4 charge counters should add 4, got %d", gs.Seats[0].ManaPool)
	}
}

func TestAddManaPer_GlobalType_PriestOfTitania(t *testing.T) {
	gs := newTestGameState(2)
	priest := ampPerm(gs, 0, "Priest of Titania", "creature", "elf", "druid")
	ampPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	ampPerm(gs, 1, "Elvish Mystic", "creature", "elf") // opponent's Elf counts (battlefield-wide)

	runAddManaPer(gs, priest, "{G}", "Elf on the battlefield")

	// Priest itself + Llanowar + opponent's Mystic = 3 Elves.
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("Priest of Titania should count all 3 Elves on the battlefield, got %d", gs.Seats[0].ManaPool)
	}
}

func TestAddManaPer_UnmatchedBasis_AddsNothing(t *testing.T) {
	gs := newTestGameState(2)
	src := ampPerm(gs, 0, "Inner Fire", "instant")
	// "card in your hand" is a deferred long-tail basis — must add nothing
	// (and must not panic / over-count).
	runAddManaPer(gs, src, "{R}", "card in your hand")

	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("unmatched basis must add no mana, got %d", gs.Seats[0].ManaPool)
	}
}

func TestAddManaPer_ComplexPredicateDegradesToZero(t *testing.T) {
	gs := newTestGameState(2)
	src := ampPerm(gs, 0, "Leafkin Avenger", "creature")
	ampPerm(gs, 0, "Big Dude", "creature")
	// "creature with power 4 or greater you control" isn't resolvable to
	// plain card types → must degrade to 0 (prior behavior), never
	// mis-count the plain creatures.
	runAddManaPer(gs, src, "{G}", "creature with power 4 or greater you control")

	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("complex predicate must degrade to 0, got %d", gs.Seats[0].ManaPool)
	}
}
