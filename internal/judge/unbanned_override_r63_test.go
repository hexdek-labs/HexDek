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

// staleUnbanSet is the full r63 stale-legality-audit wave: cards still on
// the hardcoded commanderBannedList whose actual Commander status is now
// LEGAL (verified against the official WotC announcements + the
// 2026-04-30 oracle snapshot, which marks every one of them legal).
// 2025-04-22 wave: Sway/Braids/Coalition Victory/Panoptic Mirror.
// 2026-02-09 wave: Biorhythm + Lutri (Lutri legal for inclusion; still
// banned only as a Companion, which the banlist does not model).
// Worldfire was unbanned pre-Panel. See
// /tmp/fable-review/stale-legality-audit-r63.md.
var staleUnbanSet = []string{
	"Gifts Ungiven",
	"Sway of the Stars",
	"Braids, Cabal Minion",
	"Coalition Victory",
	"Panoptic Mirror",
	"Biorhythm",
	"Lutri, the Spellchaser",
	"Worldfire",
}

// Every card in the stale-unban wave must read LEGAL through the canonical
// override AND through IsCommanderBanned even though it remains on the
// hardcoded commanderBannedList (the override gates the list).
func TestUnbannedOverride_StaleUnbanWaveReadsLegal(t *testing.T) {
	for _, name := range staleUnbanSet {
		if !IsUnbannedOverride("commander", name) {
			t.Errorf("%q missing from the canonical unbanned override", name)
		}
		if IsCommanderBanned(name) {
			t.Errorf("IsCommanderBanned(%q) = true; stale-unban override should gate the banlist", name)
		}
	}
}

// Defensive: the override must win even when the card is explicitly
// present on commanderBannedList (it still is for these — only Gifts was
// pulled in #997), so this pins the gate for the whole wave.
func TestUnbannedOverride_StaleUnbanGatesHardcodedBanlist(t *testing.T) {
	// Each wave card except Gifts is still in commanderBannedList; confirm
	// the override neutralizes those live entries.
	for _, key := range []string{
		"sway of the stars", "braids, cabal minion", "coalition victory",
		"panoptic mirror", "biorhythm", "lutri, the spellchaser", "worldfire",
	} {
		if !commanderBannedList[key] {
			continue // tolerate a future list cleanup that removes the entry
		}
		if !unbannedOverrides["commander"][key] {
			t.Errorf("banlist still bans %q but no override neutralizes it", key)
		}
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
