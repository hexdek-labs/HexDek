package main

import (
	"strings"
	"testing"
)

// countSentences counts how many `[.!?] ` (final-punct + space) boundaries
// the string contains plus 1 if the string ends with sentence punctuation.
// MTG card names contain commas ("Mikaeus, the Unhallowed") and
// apostrophes ("Thassa's Oracle") but NOT internal periods, so the
// punctuation-then-space heuristic does not false-fire on card names
// embedded in the blurb.
func countSentences(s string) int {
	n := 0
	for i := 0; i < len(s)-1; i++ {
		c := s[i]
		if (c == '.' || c == '!' || c == '?') && s[i+1] == ' ' {
			n++
		}
	}
	if s != "" {
		last := s[len(s)-1]
		if last == '.' || last == '!' || last == '?' {
			n++
		}
	}
	return n
}

// fixtureAristocratsProfile builds a DeckProfile + matching FreyaReport
// for a Bracket-3 Aristocrats deck with a named commander, star cards,
// pet cards, and a finisher win line. Used by every blurb-shape test
// so each test pins one specific aspect.
func fixtureAristocratsProfile() (*DeckProfile, *FreyaReport) {
	dp := &DeckProfile{
		DeckName:         "Yawgmoth Sacrifice",
		Commander:        "Yawgmoth, Thran Physician",
		PrimaryArchetype: "Aristocrats",
		AvgCMC:           2.4,
		RampCount:        10,
		WinLineCount:     2,
		Bracket:          3,
		BracketLabel:     "Upgraded",
		CommanderSynergy: 0.62,
		StarCards: []CardQuality{
			{Name: "Blood Artist", Tier: "star", Power: 78, PowerTier: "A"},
			{Name: "Pitiless Plunderer", Tier: "star", Power: 75, PowerTier: "A"},
			{Name: "Zulaport Cutthroat", Tier: "star", Power: 72, PowerTier: "A"},
		},
		PetCards: []PetCard{
			{Name: "Reassembling Skeleton", CMC: 2, Power: 30, PowerTier: "C", Reason: "personal-taste pick"},
		},
		ManaBaseGrade: "B",
	}
	report := &FreyaReport{
		NonlandCount:      63,
		NonLandTutorCount: 4,
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{
					Pieces: []string{"Blood Artist", "Phyrexian Altar", "Gravecrawler"},
					Type:   "infinite",
					Desc:   "Infinite drain via Gravecrawler recursion + Blood Artist trigger.",
				},
			},
		},
	}
	return dp, report
}

// TestBuildPersonalityBlurb_SentenceCountRange pins the contract: the
// blurb is between 4 and 6 sentences inclusive for any representative
// archetype. This is the headline kanban requirement — drop below 4 or
// climb above 6 and the structural promise to downstream consumers
// (deck-header UI, social-card renderer) is broken.
func TestBuildPersonalityBlurb_SentenceCountRange(t *testing.T) {
	cases := []struct {
		name  string
		setup func() (*DeckProfile, *FreyaReport)
	}{
		{"aristocrats", fixtureAristocratsProfile},
		{"combo-cedh", func() (*DeckProfile, *FreyaReport) {
			dp := &DeckProfile{
				Commander:        "Thrasios, Triton Hero",
				PrimaryArchetype: "Combo",
				AvgCMC:           1.9,
				WinLineCount:     4,
				Bracket:          5,
				BracketLabel:     "cEDH",
				HasTutorAccess:   true,
				StarCards: []CardQuality{
					{Name: "Thassa's Oracle", Tier: "star", Power: 92, PowerTier: "S"},
					{Name: "Demonic Consultation", Tier: "star", Power: 88, PowerTier: "S"},
				},
				ManaBaseGrade: "A",
			}
			report := &FreyaReport{
				NonlandCount:      63,
				NonLandTutorCount: 12,
				WinLines: &WinLineAnalysis{
					WinLines: []WinLine{{
						Pieces: []string{"Thassa's Oracle", "Demonic Consultation"},
						Type:   "infinite",
					}},
				},
			}
			return dp, report
		}},
		{"voltron-casual", func() (*DeckProfile, *FreyaReport) {
			dp := &DeckProfile{
				Commander:        "Uril, the Miststalker",
				PrimaryArchetype: "Voltron",
				AvgCMC:           3.1,
				WinLineCount:     1,
				Bracket:          2,
				BracketLabel:     "Core",
				CommanderSynergy: 0.55,
				StarCards: []CardQuality{
					{Name: "Shield of the Oversoul", Tier: "star", Power: 70, PowerTier: "A"},
				},
				PetCards: []PetCard{
					{Name: "Armadillo Cloak", CMC: 4, Reason: "personal-taste pick"},
				},
				ManaBaseGrade: "B",
			}
			report := &FreyaReport{NonlandCount: 64}
			return dp, report
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dp, report := c.setup()
			blurb := buildPersonalityBlurb(dp, report)
			n := countSentences(blurb)
			if n < 4 || n > 6 {
				t.Errorf("%s: blurb has %d sentences, want 4-6 inclusive — got: %q",
					c.name, n, blurb)
			}
		})
	}
}

// TestBuildPersonalityBlurb_NamesCommander verifies the opening
// sentence names the commander (the deck's flagship reference point).
// Without this we'd be flagging "this is a methodical Aristocrats deck"
// with no anchor for which Aristocrats deck out of the dozens of
// possible legendaries — readers want to know whose deck this is.
func TestBuildPersonalityBlurb_NamesCommander(t *testing.T) {
	dp, report := fixtureAristocratsProfile()
	blurb := buildPersonalityBlurb(dp, report)
	if !strings.Contains(blurb, dp.Commander) {
		t.Errorf("blurb does not name commander %q: %q", dp.Commander, blurb)
	}
}

// TestBuildPersonalityBlurb_NamesStarCards verifies the engine sentence
// names star cards from the deck. Drop this and the blurb regresses to
// generic descriptions that read the same on every deck of that
// archetype — the "specific cards driving the personality call"
// requirement from the kanban item.
func TestBuildPersonalityBlurb_NamesStarCards(t *testing.T) {
	dp, report := fixtureAristocratsProfile()
	blurb := buildPersonalityBlurb(dp, report)

	mentioned := 0
	for _, sc := range dp.StarCards {
		if strings.Contains(blurb, sc.Name) {
			mentioned++
		}
	}
	if mentioned < 1 {
		t.Errorf("blurb mentions zero star cards out of %d available: %q", len(dp.StarCards), blurb)
	}
}

// TestBuildPersonalityBlurb_PetCardsSurface verifies pet picks (flavor
// cards the builder kept despite off-archetype fit) appear in the
// texture sentence when present. These are the highest-signal
// personality markers in any deck — the deckbuilder is literally
// telling us what they care about by keeping a low-tier card.
func TestBuildPersonalityBlurb_PetCardsSurface(t *testing.T) {
	dp, report := fixtureAristocratsProfile()
	blurb := buildPersonalityBlurb(dp, report)
	if !strings.Contains(blurb, "Reassembling Skeleton") {
		t.Errorf("pet card not surfaced in blurb: %q", blurb)
	}
}

// TestBuildPersonalityBlurb_FinisherNamed verifies the closer sentence
// names finisher pieces from the primary win line. This was the only
// named-card signal in the pre-deepen blurb (via describeCloser /
// finisherNote), so this test also guards against regressions on the
// existing closer pipeline.
func TestBuildPersonalityBlurb_FinisherNamed(t *testing.T) {
	dp, report := fixtureAristocratsProfile()
	blurb := buildPersonalityBlurb(dp, report)

	// At least one finisher piece must appear. Blood Artist is also a
	// star card so it would land in the engine sentence too; the win
	// line has Phyrexian Altar and Gravecrawler that are NOT in
	// StarCards / PetCards, so one of those serves as a clean "only
	// the win-line path could have surfaced this card" check.
	hasAltar := strings.Contains(blurb, "Phyrexian Altar")
	hasCrawler := strings.Contains(blurb, "Gravecrawler")
	if !hasAltar && !hasCrawler {
		t.Errorf("finisher pieces (Phyrexian Altar / Gravecrawler) absent from blurb: %q", blurb)
	}
}

// TestBuildPersonalityBlurb_LengthRange pins the character-length
// envelope. Four ~90-char sentences gets us ~360 chars at minimum;
// six ~180-char sentences gets us ~1080 max. The 250-1300 envelope
// gives one sentence of slack on each end without admitting
// degenerate two-sentence outputs.
func TestBuildPersonalityBlurb_LengthRange(t *testing.T) {
	cases := []struct {
		name  string
		setup func() (*DeckProfile, *FreyaReport)
	}{
		{"aristocrats", fixtureAristocratsProfile},
		{"voltron", func() (*DeckProfile, *FreyaReport) {
			dp := &DeckProfile{
				Commander:        "Uril, the Miststalker",
				PrimaryArchetype: "Voltron",
				AvgCMC:           3.1,
				Bracket:          2,
				BracketLabel:     "Core",
				StarCards:        []CardQuality{{Name: "Shield of the Oversoul", Tier: "star"}},
				ManaBaseGrade:    "B",
			}
			return dp, &FreyaReport{NonlandCount: 64}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dp, report := c.setup()
			blurb := buildPersonalityBlurb(dp, report)
			if len(blurb) < 250 {
				t.Errorf("%s: blurb too short at %d chars (want >=250): %q", c.name, len(blurb), blurb)
			}
			if len(blurb) > 1300 {
				t.Errorf("%s: blurb too long at %d chars (want <=1300): %q", c.name, len(blurb), blurb)
			}
		})
	}
}

// TestBuildPersonalityTagline_NonEmpty verifies the tagline always
// emits something — readers should never see an empty deck-header
// subtitle. The default branch in buildPersonalityTagline guards this.
func TestBuildPersonalityTagline_NonEmpty(t *testing.T) {
	// Stress the default branch: no archetype, no win line, no commander.
	dp := &DeckProfile{}
	tag := buildPersonalityTagline(dp, &FreyaReport{})
	if tag == "" {
		t.Errorf("tagline is empty on minimal profile")
	}
}

// TestBuildPersonalityTagline_ArchetypeSpecific verifies different
// archetypes produce different taglines — without this, the entire
// tagline system could regress to a single generic line and the test
// suite wouldn't notice.
func TestBuildPersonalityTagline_ArchetypeSpecific(t *testing.T) {
	specimens := []string{"Aristocrats", "Combo", "Voltron", "Control", "Reanimator"}
	seen := map[string]string{}
	for _, arch := range specimens {
		dp := &DeckProfile{
			PrimaryArchetype: arch,
			Commander:        "Test Commander",
		}
		tag := buildPersonalityTagline(dp, &FreyaReport{})
		if existing, ok := seen[tag]; ok {
			t.Errorf("archetypes %q and %q produced identical taglines: %q", arch, existing, tag)
		}
		seen[tag] = arch
	}
}

// TestBuildPersonalityTagline_NamesCardOnSlotTemplates verifies templates
// that accept a card slot actually interpolate one when win-line data
// is available. Combo's slotted form mentions the win-line piece;
// Reanimator's commander form mentions the commander.
func TestBuildPersonalityTagline_NamesCardOnSlotTemplates(t *testing.T) {
	dpCombo := &DeckProfile{
		PrimaryArchetype: "Combo",
		Commander:        "Thrasios, Triton Hero",
	}
	reportCombo := &FreyaReport{
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{{
				Pieces: []string{"Thassa's Oracle", "Demonic Consultation"},
				Type:   "infinite",
			}},
		},
	}
	tag := buildPersonalityTagline(dpCombo, reportCombo)
	if !strings.Contains(tag, "Thassa's Oracle") {
		t.Errorf("combo tagline should name win-line piece, got %q", tag)
	}

	dpReanimator := &DeckProfile{
		PrimaryArchetype: "Reanimator",
		Commander:        "Meren of Clan Nel Toth",
	}
	tag = buildPersonalityTagline(dpReanimator, &FreyaReport{})
	if !strings.Contains(tag, "Meren of Clan Nel Toth") {
		t.Errorf("reanimator tagline should name commander, got %q", tag)
	}
}

// TestBuildPersonalityBlurb_AlwaysEndsInPunctuation defends against a
// failure mode where one of the helper functions returns a sentence
// fragment without terminal punctuation and the blurb ends mid-clause.
// The blurb-assembly loop adds a "." if the helper output lacks one;
// this test pins that contract end-to-end.
func TestBuildPersonalityBlurb_AlwaysEndsInPunctuation(t *testing.T) {
	dp, report := fixtureAristocratsProfile()
	blurb := buildPersonalityBlurb(dp, report)
	if !endsInSentencePunctuation(blurb) {
		t.Errorf("blurb does not end in sentence punctuation: %q", blurb)
	}
}

// TestBuildPersonalityBlurb_MinimalProfileStillEmits4Plus defends the
// always-at-least-4-sentences contract against degenerate input
// (no star cards, no pet cards, no win lines). Each helper has a
// fallback; this asserts the fallbacks compose correctly.
func TestBuildPersonalityBlurb_MinimalProfileStillEmits4Plus(t *testing.T) {
	dp := &DeckProfile{
		Commander:        "Anonymous Legend",
		PrimaryArchetype: "Midrange",
		AvgCMC:           3.0,
		Bracket:          3,
		BracketLabel:     "Upgraded",
	}
	blurb := buildPersonalityBlurb(dp, &FreyaReport{NonlandCount: 60})
	n := countSentences(blurb)
	if n < 4 {
		t.Errorf("minimal-profile blurb has only %d sentences (want >=4): %q", n, blurb)
	}
}
