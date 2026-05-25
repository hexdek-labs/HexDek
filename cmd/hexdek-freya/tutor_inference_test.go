package main

import (
	"strings"
	"testing"
)

// tutorCase pins one card's expected classification.
type tutorCase struct {
	name        string
	oracleText  string
	typeLine    string
	wantTutor   bool
	wantLand    bool
	wantWish    bool
	wantRestrict string // "" means don't assert
	wantDeliver  string // "" means don't assert
}

// realCardTutorCases covers the cards the task explicitly calls out plus a
// few representative false positives. Oracle text is taken from the most
// recent printing (Scryfall, lowercased). Reminder text and flavor text are
// omitted — the matcher only sees oracle bodies, and adding extra wording
// would make the test brittle without making it more honest.
var realCardTutorCases = []tutorCase{
	// --- Canonical tutors -------------------------------------------------
	{
		name:        "Demonic Tutor",
		oracleText:  "search your library for a card, put that card into your hand, then shuffle.",
		typeLine:    "sorcery",
		wantTutor:   true,
		wantRestrict: "any",
		wantDeliver:  "hand",
	},
	{
		name:        "Vampiric Tutor",
		oracleText:  "search your library for a card, then shuffle and put that card on top of it. you lose 2 life.",
		typeLine:    "instant",
		wantTutor:   true,
		wantRestrict: "any",
		wantDeliver:  "top",
	},
	{
		name:        "Worldly Tutor",
		oracleText:  "search your library for a creature card, reveal that card, then shuffle and put that card on top.",
		typeLine:    "instant",
		wantTutor:   true,
		wantRestrict: "creature",
		wantDeliver:  "top",
	},
	{
		name:        "Eldritch Evolution",
		oracleText:  "as an additional cost to cast this spell, sacrifice a creature. search your library for a creature card with converted mana cost equal to or less than 2 plus the sacrificed creature's converted mana cost, put that card onto the battlefield, then shuffle. exile this spell.",
		typeLine:    "sorcery",
		wantTutor:   true,
		// r60: detectVariableCmcSearch now picks up "equal to or less than
		// 2 plus the sacrificed creature's mana value" and promotes the
		// plain "creature" classification to the tighter "cmc_variable" —
		// honest about the game-state-dependent bound that the plain
		// "creature" bucket dropped.
		wantRestrict: "cmc_variable",
		wantDeliver:  "battlefield",
	},

	// --- Conditional CMC tutors ------------------------------------------
	{
		name:        "Bring to Light",
		oracleText:  "sunburst (this enters with a charge counter on it for each color of mana spent to cast it.) search your library for an instant, sorcery, creature, artifact, enchantment, or planeswalker card with converted mana cost 5 or less, exile that card, then shuffle. you may cast that card without paying its mana cost as long as it remains exiled. if it has sunburst, it enters with a charge counter on it for each color of mana spent to cast it that way.",
		typeLine:    "sorcery",
		wantTutor:   true,
		wantRestrict: "cmcle5",
	},
	{
		name:        "Diabolic Revelation",
		oracleText:  "search your library for up to x cards, reveal those cards, put them into your hand, then shuffle.",
		typeLine:    "sorcery",
		wantTutor:   true,
		// Diabolic Revelation has unbounded X so we accept the broad "any"
		// bucket. The matcher should at least flag it as a tutor.
	},

	// --- Tribal subtype tutors -------------------------------------------
	{
		name:        "Goblin Matron",
		oracleText:  "when this creature enters, you may search your library for a goblin card, reveal that card, put it into your hand, then shuffle.",
		typeLine:    "creature — goblin",
		wantTutor:   true,
		wantRestrict: "tribe:goblin",
		wantDeliver:  "hand",
	},

	// --- Wish-style / outside-the-game tutors ----------------------------
	{
		name:        "Burning Wish",
		oracleText:  "you may choose a sorcery card you own from outside the game, reveal that card, and put it into your hand. exile this spell.",
		typeLine:    "sorcery",
		wantTutor:   true,
		wantWish:    true,
		wantRestrict: "wish",
	},
	{
		name:        "Karn, the Great Creator",
		oracleText:  "artifacts your opponents control can't be activated. +1: until your next turn, up to one target noncreature artifact becomes a 0/0 artifact creature. −2: you may reveal an artifact card you own from outside the game or in exile and put it into your hand.",
		typeLine:    "legendary planeswalker — karn",
		wantTutor:   true,
		wantWish:    true,
		wantRestrict: "wish",
	},

	// --- Land tutors / ramp ----------------------------------------------
	{
		name:        "Cultivate",
		oracleText:  "search your library for up to two basic land cards, reveal those cards, put one onto the battlefield tapped and the other into your hand, then shuffle.",
		typeLine:    "sorcery",
		wantTutor:   true,
		wantLand:    true,
		wantRestrict: "basic_land",
	},
	{
		name:        "Rampant Growth",
		oracleText:  "search your library for a basic land card, put that card onto the battlefield tapped, then shuffle.",
		typeLine:    "sorcery",
		wantTutor:   true,
		wantLand:    true,
		wantRestrict: "basic_land",
		wantDeliver:  "battlefield",
	},
	{
		name:        "Crop Rotation",
		oracleText:  "as an additional cost to cast this spell, sacrifice a land. search your library for a land card, put that card onto the battlefield, then shuffle.",
		typeLine:    "instant",
		wantTutor:   true,
		wantLand:    true,
		wantRestrict: "land",
		wantDeliver:  "battlefield",
	},

	// --- False positives (must NOT flag as tutors) -----------------------
	{
		name:        "Aven Mindcensor",
		oracleText:  "flash. flying. if an opponent would search a library, that player searches the top four cards of that library instead.",
		typeLine:    "creature — bird wizard",
		wantTutor:   false,
	},
	{
		name:        "Stranglehold",
		oracleText:  "if an opponent would search a library, that player exiles the top four cards of that library instead. your opponents can't take extra turns.",
		typeLine:    "enchantment",
		wantTutor:   false,
	},
	{
		name:        "Searing Touch",
		oracleText:  "this spell deals 2 damage to any target. cycling {2} ({2}, discard this card: draw a card.)",
		typeLine:    "sorcery",
		wantTutor:   false,
	},
	{
		name:        "Krosan Tusker",
		oracleText:  "cycling {2}{g} ({2}{g}, discard this card: draw a card.) when you cycle this card, you may search your library for a basic land card, reveal it, put it into your hand, then shuffle.",
		typeLine:    "creature — beast",
		// Cycling-search hybrid: my classifier currently classifies this as
		// a tutor when the cycling clause also contains "search your library"
		// (the cycling-exclusion is overridden by the "search your library
		// for a card" hint). Krosan Tusker says "for a basic land card", so
		// the cycling-exclusion fires and we treat it as NOT a tutor — the
		// effect is gated behind paying cycling cost, which is closer to
		// ramp than to a free-cast tutor.
		wantTutor: false,
	},

	// --- Pure non-search false positives ---------------------------------
	{
		name:        "Lightning Bolt",
		oracleText:  "this spell deals 3 damage to any target.",
		typeLine:    "instant",
		wantTutor:   false,
	},
	{
		name:        "Sol Ring",
		oracleText:  "{t}: add {c}{c}.",
		typeLine:    "artifact",
		wantTutor:   false,
	},
}

func TestClassifyTutor_RealCards(t *testing.T) {
	for _, tc := range realCardTutorCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := CardProfile{Name: tc.name}
			classifyTutorInto(&p, strings.ToLower(tc.oracleText),
				strings.ToLower(tc.typeLine), tc.name)
			if p.IsTutor != tc.wantTutor {
				t.Errorf("%s: IsTutor=%v, want %v", tc.name, p.IsTutor, tc.wantTutor)
			}
			if p.IsLandTutor != tc.wantLand {
				t.Errorf("%s: IsLandTutor=%v, want %v", tc.name, p.IsLandTutor, tc.wantLand)
			}
			if p.IsWishTutor != tc.wantWish {
				t.Errorf("%s: IsWishTutor=%v, want %v", tc.name, p.IsWishTutor, tc.wantWish)
			}

			if tc.wantRestrict != "" {
				gotR, gotD := inferTutorRestriction(strings.ToLower(tc.oracleText),
					strings.ToLower(tc.typeLine))
				if gotR != tc.wantRestrict {
					t.Errorf("%s: inferTutorRestriction restricted=%q, want %q",
						tc.name, gotR, tc.wantRestrict)
				}
				if tc.wantDeliver != "" && gotD != tc.wantDeliver {
					t.Errorf("%s: inferTutorRestriction delivery=%q, want %q",
						tc.name, gotD, tc.wantDeliver)
				}
			}
		})
	}
}

// TestAnalyzeDeck_NonLandTutorCount verifies the split: a deck mixing
// canonical card tutors with basic-land ramp should report the two
// categories separately. The scoring sites now consume NonLandTutorCount,
// so this is the regression that keeps "8 Cultivates ≠ deep tutor package".
func TestAnalyzeDeck_NonLandTutorCount(t *testing.T) {
	mkProfile := func(name, oracle, typeLine string) CardProfile {
		p := CardProfile{Name: name, TypeLine: typeLine}
		classifyTutorInto(&p, strings.ToLower(oracle), strings.ToLower(typeLine), name)
		return p
	}

	demonic := mkProfile("Demonic Tutor",
		"search your library for a card, put that card into your hand, then shuffle.",
		"sorcery")
	vampiric := mkProfile("Vampiric Tutor",
		"search your library for a card, then shuffle and put that card on top.",
		"instant")
	cultivate := mkProfile("Cultivate",
		"search your library for up to two basic land cards, put one onto the battlefield tapped and the other into your hand, then shuffle.",
		"sorcery")
	rampant := mkProfile("Rampant Growth",
		"search your library for a basic land card, put that card onto the battlefield tapped, then shuffle.",
		"sorcery")
	bolt := mkProfile("Lightning Bolt", "deals 3 damage to any target.", "instant")
	burningWish := mkProfile("Burning Wish",
		"you may choose a sorcery card you own from outside the game, reveal that card, and put it into your hand. exile this spell.",
		"sorcery")

	profiles := []CardProfile{demonic, vampiric, cultivate, rampant, bolt, burningWish}
	report := AnalyzeDeck(profiles, "test", "/dev/null", "test commander")

	// 2 real tutors (Demonic, Vampiric) + 1 wish (Burning) + 2 land tutors.
	if report.TutorCount != 5 {
		t.Errorf("TutorCount=%d, want 5", report.TutorCount)
	}
	if report.NonLandTutorCount != 3 {
		t.Errorf("NonLandTutorCount=%d, want 3 (Demonic, Vampiric, Burning Wish)", report.NonLandTutorCount)
	}
	if report.LandTutorCount != 2 {
		t.Errorf("LandTutorCount=%d, want 2 (Cultivate, Rampant Growth)", report.LandTutorCount)
	}
	if report.WishTutorCount != 1 {
		t.Errorf("WishTutorCount=%d, want 1 (Burning Wish)", report.WishTutorCount)
	}
}

// TestTutorCanFind_NewRestrictions exercises the additional restriction
// keys the matcher now emits. Without these passing, inferTutorRestriction
// would produce restriction strings that downstream tutor-coverage walks
// silently treat as "uncoverable", suppressing tutor paths in win lines.
func TestTutorCanFind_NewRestrictions(t *testing.T) {
	typeByName := map[string]string{
		"thassa's oracle":      "creature — merfolk wizard",
		"demonic consultation": "instant",
		"goblin lackey":        "creature — goblin",
		"sol ring":             "artifact",
		"karn liberated":       "legendary planeswalker — karn",
		"forest":               "basic land — forest",
		"ancient tomb":         "land",
	}

	cases := []struct {
		restricted string
		card       string
		want       bool
	}{
		// Wish: always findable (Freya doesn't model sideboards).
		{"wish", "thassa's oracle", true},

		// Tribal: matches by type-line subtype.
		{"tribe:goblin", "goblin lackey", true},
		{"tribe:goblin", "thassa's oracle", false},
		{"tribe:merfolk", "thassa's oracle", true},

		// Planeswalker.
		{"planeswalker", "karn liberated", true},
		{"planeswalker", "sol ring", false},

		// Basic land.
		{"basic_land", "forest", true},
		{"basic_land", "ancient tomb", false},

		// Plain land still matches any land (basic or not).
		{"land", "forest", true},
		{"land", "ancient tomb", true},
	}
	for _, tc := range cases {
		got := tutorCanFind(TutorInfo{Restricted: tc.restricted},
			tc.card, typeByName, nil)
		if got != tc.want {
			t.Errorf("tutorCanFind(restricted=%q, card=%q) = %v, want %v",
				tc.restricted, tc.card, got, tc.want)
		}
	}
}

// TestInferTutorRestriction_CmcLeBoundary pins the cmcle parsing — both
// "mana value" and "converted mana cost" phrasings must land in the same
// bucket so old-frame and new-frame printings tutor identically.
func TestInferTutorRestriction_CmcLeBoundary(t *testing.T) {
	cases := []struct {
		ot   string
		want string
	}{
		{
			ot:   "search your library for a creature card with mana value 3 or less, put it onto the battlefield, then shuffle.",
			want: "cmcle3",
		},
		{
			ot:   "search your library for a creature card with converted mana cost 3 or less, put it onto the battlefield, then shuffle.",
			want: "cmcle3",
		},
		{
			ot:   "search your library for an instant or sorcery card with mana value 2 or less, reveal it, put it into your hand, then shuffle.",
			want: "cmcle2",
		},
	}
	for _, tc := range cases {
		got, _ := inferTutorRestriction(tc.ot, "")
		if got != tc.want {
			t.Errorf("inferTutorRestriction(%q) restricted=%q, want %q",
				tc.ot, got, tc.want)
		}
	}
}
