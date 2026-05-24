package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// choose_discard_r60_test.go — regressions for the R60 ChooseDiscard
// additions:
//
//   1. Commander-in-hand protection — discarding our own commander is
//      almost never correct. ChooseDiscard now applies a dominating
//      +10 bonus to commander cards in hand, so they're the last
//      considered for discard.
//
//   2. Mana-starvation land protection — with fewer than three mana
//      sources, the next land drop matters more than any non-land in
//      hand; ChooseDiscard now applies a +3 bonus to lands when
//      sources < 3 (mirror of the existing -0.5 sources>=5 flood
//      penalty).

// -----------------------------------------------------------------------------
// Commander protection
// -----------------------------------------------------------------------------

func TestChooseDiscard_DoesNotDiscardCommanderFromHand(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Seats[0].CommanderNames = []string{"Korvold, Fae-Cursed King"}

	commander := newTestCardMinimal("Korvold, Fae-Cursed King", []string{"creature", "legendary", "dragon"}, 5, nil)
	filler := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)

	hand := []*gameengine.Card{commander, filler}
	got := h.ChooseDiscard(gs, 0, hand, 1)
	if len(got) != 1 {
		t.Fatalf("want 1 discard, got %d", len(got))
	}
	if got[0] == commander {
		t.Fatalf("commander should be the LAST card discarded; got %s",
			got[0].DisplayName())
	}
}

func TestChooseDiscard_DiscardsCommanderWhenForcedByHandSize(t *testing.T) {
	// If n >= len(hand), ChooseDiscard returns the whole hand by design.
	// Verify commander protection doesn't change that contract — when
	// you're forced to discard everything, the commander goes too.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Seats[0].CommanderNames = []string{"Korvold, Fae-Cursed King"}

	commander := newTestCardMinimal("Korvold, Fae-Cursed King", []string{"creature", "legendary", "dragon"}, 5, nil)
	hand := []*gameengine.Card{commander}

	got := h.ChooseDiscard(gs, 0, hand, 1)
	if len(got) != 1 || got[0] != commander {
		t.Fatalf("forced full-hand discard should still return the commander; got %v", got)
	}
}

func TestChooseDiscard_CommanderProtectionDoesNotBleedToOpponentCards(t *testing.T) {
	// Our seat's commander-name list shouldn't affect a card with the
	// same name in another context (defense against false positives in
	// shared-name cases, e.g. partner pairs across decks).
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	// Seat 0 has no commander; seat 1 has Korvold (opponent).
	gs.Seats[1].CommanderNames = []string{"Korvold, Fae-Cursed King"}

	korvoldLike := newTestCardMinimal("Korvold, Fae-Cursed King", []string{"creature", "legendary", "dragon"}, 5, nil)
	filler := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)

	hand := []*gameengine.Card{korvoldLike, filler}
	got := h.ChooseDiscard(gs, 0, hand, 1)
	if len(got) != 1 {
		t.Fatalf("want 1 discard, got %d", len(got))
	}
	// Without seat-0 commander protection the higher-CMC Korvold card
	// should be discarded — the protection must read OUR seat's
	// commander list, not the opponent's.
	if got[0] != korvoldLike {
		t.Fatalf("without our-seat commander protection, expected the 5-mv card discarded first; got %s",
			got[0].DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Mana-starvation land protection
// -----------------------------------------------------------------------------

func TestChooseDiscard_ProtectsLandWhenManaStarved(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	// Seat 0 has just 1 land in play — mana-starved (sources < 3).
	loneLand := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)
	newTestPermanent(gs.Seats[0], loneLand, 0, 0)

	land := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)
	creature := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)

	hand := []*gameengine.Card{land, creature}
	got := h.ChooseDiscard(gs, 0, hand, 1)
	if len(got) != 1 {
		t.Fatalf("want 1 discard, got %d", len(got))
	}
	if got[0] == land {
		t.Fatalf("with sources<3, mana-starvation should protect the land; got land discarded")
	}
}

func TestChooseDiscard_NoStarvationProtectionAtThreeSources(t *testing.T) {
	// At sources=3 we're exactly at the threshold (sources<3 is the
	// trigger), so the starvation bonus must NOT fire. We pin this by
	// asserting the existing default behavior: with a land and a
	// vanilla creature in hand, the land's lower base cardHeuristic
	// makes it the discard pick — same outcome as at any non-starved
	// non-flooded source count.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		basic := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)
		newTestPermanent(gs.Seats[0], basic, 0, 0)
	}

	land := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)
	creature := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)

	hand := []*gameengine.Card{land, creature}
	got := h.ChooseDiscard(gs, 0, hand, 1)
	if len(got) != 1 {
		t.Fatalf("want 1 discard, got %d", len(got))
	}
	// With sources=3 no starvation bonus fires; the land's natural
	// lower heuristic means it still gets discarded over the creature.
	// (At sources<3 the +3 starvation bonus would invert this.)
	if got[0] != land {
		t.Fatalf("at sources=3 (boundary), starvation should not fire — expected land discarded; got %s",
			got[0].DisplayName())
	}
}

func TestChooseDiscard_LandFloodPenaltyStillFiresAtFivePlus(t *testing.T) {
	// Sanity: the existing sources>=5 land-flood penalty must remain
	// active and not be overridden by the new starvation bonus.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	for i := 0; i < 5; i++ {
		basic := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)
		newTestPermanent(gs.Seats[0], basic, 0, 0)
	}

	land := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)
	creature := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)

	hand := []*gameengine.Card{land, creature}
	got := h.ChooseDiscard(gs, 0, hand, 1)
	if len(got) != 1 {
		t.Fatalf("want 1 discard, got %d", len(got))
	}
	if got[0] != land {
		t.Fatalf("with sources>=5, flooded land should be discarded first; got %s",
			got[0].DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Interaction
// -----------------------------------------------------------------------------

func TestChooseDiscard_CommanderProtectionBeatsManaStarvationProtection(t *testing.T) {
	// Mana-starved seat, both a land and the commander in hand. The
	// commander's +10 bonus dominates the land's +3 starvation bonus,
	// so when forced to keep one, the commander wins — the land is
	// the discard. This pins the relative weight ordering.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Seats[0].CommanderNames = []string{"Korvold, Fae-Cursed King"}
	// 1 land in play → sources < 3.
	soleLand := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)
	newTestPermanent(gs.Seats[0], soleLand, 0, 0)

	commander := newTestCardMinimal("Korvold, Fae-Cursed King", []string{"creature", "legendary", "dragon"}, 5, nil)
	land := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)

	hand := []*gameengine.Card{commander, land}
	got := h.ChooseDiscard(gs, 0, hand, 1)
	if len(got) != 1 {
		t.Fatalf("want 1 discard, got %d", len(got))
	}
	if got[0] != land {
		t.Fatalf("commander protection should dominate starvation protection — expected land discarded, got %s",
			got[0].DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Helper-function unit tests
// -----------------------------------------------------------------------------

func TestIsCommanderCardName_Matches(t *testing.T) {
	commander := newTestCardMinimal("Atraxa, Praetors' Voice", []string{"creature", "legendary"}, 4, nil)
	if !isCommanderCardName([]string{"Atraxa, Praetors' Voice"}, commander) {
		t.Fatal("expected match against commander name")
	}
}

func TestIsCommanderCardName_NoMatch(t *testing.T) {
	other := newTestCardMinimal("Llanowar Elves", []string{"creature"}, 1, nil)
	if isCommanderCardName([]string{"Atraxa, Praetors' Voice"}, other) {
		t.Fatal("non-commander card should not match")
	}
}

func TestIsCommanderCardName_NilSafe(t *testing.T) {
	if isCommanderCardName(nil, nil) {
		t.Fatal("nil card with nil list should not panic or match")
	}
	if isCommanderCardName([]string{"Korvold, Fae-Cursed King"}, nil) {
		t.Fatal("nil card should not match any name")
	}
	bear := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	if isCommanderCardName(nil, bear) {
		t.Fatal("empty commander list should match nothing")
	}
}

func TestCommanderNamesForSeat_ReturnsCopy(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].CommanderNames = []string{"Krenko, Mob Boss"}

	got := commanderNamesForSeat(gs, 0)
	if len(got) != 1 || got[0] != "Krenko, Mob Boss" {
		t.Fatalf("want [Krenko, Mob Boss], got %v", got)
	}
	// Mutating the returned slice must not affect the seat.
	got[0] = "Mutated"
	if gs.Seats[0].CommanderNames[0] != "Krenko, Mob Boss" {
		t.Fatal("commanderNamesForSeat returned an alias instead of a copy")
	}
}

func TestCommanderNamesForSeat_OutOfRange(t *testing.T) {
	gs := newTestGame(t, 2)
	if commanderNamesForSeat(gs, -1) != nil {
		t.Fatal("negative seat should return nil")
	}
	if commanderNamesForSeat(gs, 99) != nil {
		t.Fatal("out-of-range seat should return nil")
	}
	if commanderNamesForSeat(nil, 0) != nil {
		t.Fatal("nil game should return nil")
	}
}
