package judge

import (
	"strings"
	"testing"
)

// legality_test.go — LEGALITY dimension fold (r63 phase 3).
//
// Pins: (1) each of the five deck-construction checks fires on its
// failure shape and emits a canonical violation tagged
// Dimension=legality through LogViolation; (2) a legal deck emits
// nothing; (3) the freya-era check semantics survived the promotion
// (basic-land/snow/any-number singleton exemptions, unresolved-card
// skips, partner card count message).

func captureViolations(t *testing.T) (*[]ValidationViolation, func()) {
	t.Helper()
	var got []ValidationViolation
	unreg := RegisterSink(func(v ValidationViolation) {
		got = append(got, v)
	})
	return &got, unreg
}

func legalDeck() DeckSubmission {
	cmdr := DeckCard{
		Name: "Klauth, Unrivaled Ancient", CanonicalName: "Klauth, Unrivaled Ancient",
		Qty: 1, ColorIdentity: []string{"R", "G"},
		OracleText: "Flying. Whenever Klauth attacks, add mana.",
		TypeLine:   "Legendary Creature — Dragon", Resolved: true,
	}
	cards := []DeckCard{
		{Name: "Forest", Qty: 49, Resolved: true, TypeLine: "Basic Land — Forest"},
		{Name: "Mountain", Qty: 30, Resolved: true, TypeLine: "Basic Land — Mountain"},
	}
	// 20 distinct legal singletons to reach 100 total.
	for i := 0; i < 20; i++ {
		cards = append(cards, DeckCard{
			Name: "Filler Card " + string(rune('A'+i)), CanonicalName: "Filler Card " + string(rune('A'+i)),
			Qty: 1, ColorIdentity: []string{"G"}, Resolved: true,
			TypeLine: "Creature — Bear",
		})
	}
	return DeckSubmission{Commander: cmdr, Cards: cards, TotalCards: 100}
}

func TestDeckLegality_LegalDeckCleanAndSilent(t *testing.T) {
	got, unreg := captureViolations(t)
	defer unreg()

	lr := CheckDeckLegality(legalDeck())
	if !lr.Valid {
		t.Fatalf("legal deck reported invalid: %v", lr.Errors)
	}
	if len(*got) != 0 {
		t.Fatalf("legal deck emitted %d violations: %+v", len(*got), (*got)[0])
	}
	if !lr.CardCount.Valid || !lr.ColorID.Valid || !lr.Singleton.Valid ||
		!lr.BannedCards.Valid || !lr.CommanderOK.Valid {
		t.Fatalf("sub-check invalid on legal deck: %+v", lr)
	}
}

func TestDeckLegality_EachCheckFiresTagged(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*DeckSubmission)
		wantRule string
		wantMsg  string
	}{
		{"card_count", func(d *DeckSubmission) { d.TotalCards = 99 }, "903.5a", "expected 100 cards, found 99"},
		{"color_identity", func(d *DeckSubmission) {
			d.Cards = append(d.Cards, DeckCard{
				Name: "Counterspell", CanonicalName: "Counterspell", Qty: 1,
				ColorIdentity: []string{"U"}, Resolved: true,
			})
		}, "903.5c", "color identity violation: Counterspell"},
		{"singleton", func(d *DeckSubmission) {
			d.Cards = append(d.Cards, DeckCard{
				Name: "Lightning Greaves", CanonicalName: "Lightning Greaves", Qty: 2, Resolved: true,
			})
		}, "903.5b", "singleton violation: Lightning Greaves appears 2 times"},
		{"banned", func(d *DeckSubmission) {
			d.Cards = append(d.Cards, DeckCard{
				Name: "Dockside Extortionist", CanonicalName: "Dockside Extortionist",
				Qty: 1, ColorIdentity: []string{"R"}, Resolved: true,
			})
		}, "banlist", "banned card: Dockside Extortionist"},
		{"commander", func(d *DeckSubmission) {
			d.Commander = DeckCard{
				Name: "Llanowar Elves", CanonicalName: "Llanowar Elves", Qty: 1,
				ColorIdentity: []string{"G"}, TypeLine: "Creature — Elf Druid", Resolved: true,
			}
		}, "903.3", "not a legendary creature"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, unreg := captureViolations(t)
			defer unreg()

			d := legalDeck()
			tc.mutate(&d)
			lr := CheckDeckLegality(d)

			if lr.Valid {
				t.Fatalf("deck reported valid despite %s failure", tc.name)
			}
			var hit *ValidationViolation
			for i := range *got {
				if (*got)[i].Name == tc.wantRule {
					hit = &(*got)[i]
					break
				}
			}
			if hit == nil {
				t.Fatalf("no violation with rule %q emitted; got %+v", tc.wantRule, *got)
			}
			if hit.Dimension != DimensionLegality {
				t.Fatalf("Dimension = %q, want %q", hit.Dimension, DimensionLegality)
			}
			if hit.Surface != SurfaceLegality {
				t.Fatalf("Surface = %q, want %q", hit.Surface, SurfaceLegality)
			}
			if !strings.Contains(hit.Message, strings.Split(tc.wantMsg, ": ")[0]) {
				t.Fatalf("Message %q does not contain %q", hit.Message, tc.wantMsg)
			}
		})
	}
}

func TestDeckLegality_SingletonExemptions(t *testing.T) {
	d := legalDeck() // already has Forest x49 + Mountain x30
	d.Cards = append(d.Cards,
		DeckCard{Name: "Snow-Covered Island", Qty: 5, Resolved: true},
		DeckCard{Name: "Relentless Rats", Qty: 4, Resolved: true,
			OracleText: "A deck can have any number of cards named Relentless Rats."},
	)
	lr := CheckDeckLegality(d)
	if !lr.Singleton.Valid {
		t.Fatalf("exempt multiples flagged: %+v", lr.Singleton.Violations)
	}
}

func TestDeckLegality_UnresolvedCardsSkipDataChecks(t *testing.T) {
	got, unreg := captureViolations(t)
	defer unreg()

	d := legalDeck()
	// Unresolved card: color identity + banned canonical lookup must skip
	// (raw-name banned matching still applies — pin BOTH behaviors).
	d.Cards = append(d.Cards,
		DeckCard{Name: "Some Custom Card", Qty: 1, Resolved: false},
		DeckCard{Name: "Mana Crypt", Qty: 1, Resolved: false}, // banned by raw name
	)
	lr := CheckDeckLegality(d)
	if !lr.ColorID.Valid {
		t.Fatalf("unresolved card tripped color identity")
	}
	if lr.BannedCards.Valid {
		t.Fatalf("raw-name banned match missed for unresolved Mana Crypt")
	}
	_ = got
}

func TestIsCommanderBanned(t *testing.T) {
	if !IsCommanderBanned("Mana Crypt") || !IsCommanderBanned("dockside extortionist") {
		t.Fatalf("known banned cards not flagged")
	}
	if IsCommanderBanned("Lightning Greaves") {
		t.Fatalf("legal card flagged as banned")
	}
	// Typographic apostrophe folds to ASCII.
	if !IsCommanderBanned("Yawgmoth’s Bargain") {
		t.Fatalf("curly-quote banned name not folded")
	}
}

func TestDeckLegality_PartnerCardCountMessage(t *testing.T) {
	d := legalDeck()
	d.Commander.OracleText = "Partner (You can have two commanders if both have partner.)"
	lr := CheckDeckLegality(d)
	if !lr.CardCount.Valid || !lr.CardCount.HasPartner {
		t.Fatalf("partner detection failed: %+v", lr.CardCount)
	}
	if !strings.Contains(lr.CardCount.Message, "partner") {
		t.Fatalf("partner message missing: %q", lr.CardCount.Message)
	}
}
