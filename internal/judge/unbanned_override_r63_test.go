package judge

import "testing"

// unbanned_override_r63_test.go — owner-reported bug (7174n1c): Gifts
// Ungiven, unbanned in Commander (Sept 2024), still showed ILLEGAL in
// the deck-legality UI. #996 added an import-path override and #997
// pulled it from the hardcoded lists; r63 lifts the override to ONE
// canonical helper (judge.IsUnbannedOverride) consulted by every
// banned-determination site. These pin the analysis/display path (the
// one feeding the UI's deck `legal` flag via freya → CheckDeckLegality).

// giftsDeck builds a structurally-legal 100-card Commander deck that
// contains Gifts Ungiven. A 5-color commander keeps the color-identity
// check from firing so the banned check is isolated.
func giftsDeck() DeckSubmission {
	cmdr := DeckCard{
		Name: "Kenrith, the Returned King", CanonicalName: "Kenrith, the Returned King",
		Qty: 1, ColorIdentity: []string{"W", "U", "B", "R", "G"},
		OracleText: "Activated abilities.", TypeLine: "Legendary Creature — Human Noble",
		Resolved: true,
	}
	cards := []DeckCard{
		{Name: "Island", Qty: 60, Resolved: true, TypeLine: "Basic Land — Island"},
		{Name: "Gifts Ungiven", CanonicalName: "Gifts Ungiven", Qty: 1,
			ColorIdentity: []string{"U"}, Resolved: true, TypeLine: "Instant"},
	}
	for i := 0; i < 38; i++ {
		cards = append(cards, DeckCard{
			Name: "Filler Spell " + string(rune('A'+i)), CanonicalName: "Filler Spell " + string(rune('A'+i)),
			Qty: 1, ColorIdentity: []string{"U"}, Resolved: true, TypeLine: "Creature — Bird",
		})
	}
	return DeckSubmission{Commander: cmdr, Cards: cards, TotalCards: 100}
}

// The analysis/display path must read a deck with Gifts Ungiven as LEGAL.
func TestUnbannedOverride_GiftsUngivenLegalThroughAnalysisPath(t *testing.T) {
	lr := CheckDeckLegality(giftsDeck())
	if !lr.BannedCards.Valid {
		t.Fatalf("Gifts Ungiven deck flagged banned cards: %v", lr.BannedCards.BannedFound)
	}
	for _, n := range lr.BannedCards.BannedFound {
		if n == "Gifts Ungiven" {
			t.Fatal("Gifts Ungiven reported as banned through the deck-analysis path")
		}
	}
	if !lr.Valid {
		t.Fatalf("Gifts Ungiven deck reported ILLEGAL: %v", lr.Errors)
	}
}

// Load-bearing: the override must GATE the banned list — even if a card
// is (still / again) on commanderBannedList, an unbanned-override entry
// reads it legal. Proves the override actually protects the display path
// rather than relying on the card merely being absent from the list.
func TestUnbannedOverride_GatesBannedListEntry(t *testing.T) {
	commanderBannedList["gifts ungiven"] = true
	defer delete(commanderBannedList, "gifts ungiven")

	bc := checkDeckBanned(giftsDeck().Cards)
	if !bc.Valid {
		t.Fatalf("override failed to gate a banned-list entry: %v", bc.BannedFound)
	}
	if IsCommanderBanned("Gifts Ungiven") {
		t.Fatal("IsCommanderBanned ignored the unbanned override")
	}
}

func TestIsUnbannedOverride_Normalization(t *testing.T) {
	cases := []struct {
		format, name string
		want         bool
	}{
		{"commander", "Gifts Ungiven", true},
		{"Commander", "  gifts ungiven  ", true},
		{"commander", "GIFTS UNGIVEN", true},
		{"commander", "Sol Ring", false},
		{"modern", "Gifts Ungiven", false}, // override is commander-scoped
		{"", "Gifts Ungiven", false},
	}
	for _, c := range cases {
		if got := IsUnbannedOverride(c.format, c.name); got != c.want {
			t.Errorf("IsUnbannedOverride(%q, %q) = %v, want %v", c.format, c.name, got, c.want)
		}
	}
}
