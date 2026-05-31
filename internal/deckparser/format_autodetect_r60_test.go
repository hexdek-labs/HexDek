package deckparser

import (
	"fmt"
	"strings"
	"testing"
)

// format_autodetect_r60_test.go — DetectedFormat heuristic test suite.
//
// Verifies the structural format detector across the 6 buckets
// (Commander / Brawl / Oathbreaker / Constructed / Precon / Casual)
// with realistic deck shapes pulled from Wizards-published precons,
// canonical EDHREC commanders, Standard / Modern / Pioneer maindecks,
// Legacy / Vintage maindecks, Pauper exports, and Oathbreaker primer
// lists. Detection is structural — Standard / Modern / Pioneer /
// Legacy / Vintage / Pauper share the same 60+ mainboard + sideboard +
// multiples-allowed shape, so they collapse to FormatConstructed;
// further refinement requires a card-legality DB the deckparser
// package doesn't own. The tests document this collapse with a
// dedicated table per family so the bucket-mapping intent stays
// readable.

// fmtMeta returns a stub MetaDB seeded with enough card names to
// resolve every card cited in this file. The cards intentionally
// span all formats: cEDH staples (Atraxa, Sol Ring), Standard
// creatures (Llanowar Elves, Lightning Bolt), Modern staples
// (Snapcaster Mage, Tarmogoyf), Legacy/Vintage power (Force of Will,
// Black Lotus), Pauper commons (Lava Spike, Mulldrifter), Brawl
// commanders (Chulane, Yenna), and Oathbreaker planeswalkers
// (Ajani Steadfast, Saheeli Sublime Artificer).
func fmtMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	add := func(name string, types ...string) {
		meta.byName[normalizeName(name)] = &CardMeta{
			Name: name, Types: types, CMC: 1,
		}
	}
	// Commanders + signature spells.
	add("Atraxa, Praetors' Voice", "legendary", "creature")
	add("Kinnan, Bonder Prodigy", "legendary", "creature")
	add("Thrasios, Triton Hero", "legendary", "creature")
	add("Bello, Bard of the Brambles", "legendary", "creature")
	add("Chulane, Teller of Tales", "legendary", "creature")
	add("Yenna, Redtooth Regent", "legendary", "creature")
	add("Ajani, Steadfast", "legendary", "planeswalker")
	add("Saheeli, Sublime Artificer", "legendary", "planeswalker")
	add("Path to Exile", "instant")
	add("Lightning Bolt", "instant")
	// Mainboard staples used as fillers.
	for _, n := range []string{
		"Sol Ring", "Command Tower", "Arcane Signet", "Lightning Greaves",
		"Cyclonic Rift", "Demonic Tutor", "Counterspell", "Force of Will",
		"Black Lotus", "Mox Sapphire", "Mox Ruby", "Mox Pearl",
		"Snapcaster Mage", "Tarmogoyf", "Llanowar Elves",
		"Lava Spike", "Mulldrifter", "Counterspell", "Brainstorm",
		"Ponder", "Preordain", "Birds of Paradise", "Wood Elves",
		"Solemn Simulacrum", "Sylvan Library", "Eternal Witness",
		"Cultivate", "Kodama's Reach", "Rampant Growth", "Farseek",
		"Reflecting Pool", "Mana Confluence", "City of Brass",
		"Forest", "Swamp", "Plains", "Island", "Mountain",
		"Snow-Covered Forest", "Snow-Covered Island", "Snow-Covered Plains",
	} {
		add(n, "generic")
	}
	return meta
}

// repeatLines emits `qty Name\n` n times — helper for building
// realistic-sized maindecks without writing 60+ literal lines per case.
func repeatLines(name string, qty int) string {
	var sb strings.Builder
	for i := 0; i < qty; i++ {
		fmt.Fprintf(&sb, "1 %s\n", name)
	}
	return sb.String()
}

// quotaFill builds n distinct card lines from the fmtMeta filler pool.
// Cycles through the filler-name list so each line uses a real meta
// entry and the parser exercises real resolution. Qty per line stays
// at 1 (singleton) so the same helper drives Commander, Brawl, and
// Oathbreaker construction.
func quotaFill(n int) string {
	fillers := []string{
		"Sol Ring", "Command Tower", "Arcane Signet", "Lightning Greaves",
		"Cyclonic Rift", "Demonic Tutor", "Counterspell", "Force of Will",
		"Snapcaster Mage", "Tarmogoyf", "Llanowar Elves",
		"Mulldrifter", "Brainstorm", "Ponder", "Preordain",
		"Birds of Paradise", "Wood Elves", "Solemn Simulacrum",
		"Sylvan Library", "Eternal Witness", "Cultivate",
		"Kodama's Reach", "Rampant Growth", "Farseek",
		"Reflecting Pool", "Mana Confluence", "City of Brass",
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "1 %s\n", fillers[i%len(fillers)])
	}
	return sb.String()
}

// TestDetectFormat_Commander — eight realistic Commander deck shapes
// (cEDH, casual, partner pair, partner via Commander section, Moxfield
// directive form, Moxfield section form, B2 / B5 bracket variants).
// All must resolve to FormatCommander.
func TestDetectFormat_Commander(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{
			"directive form, 99+1 cEDH",
			"COMMANDER: Atraxa, Praetors' Voice\n" + quotaFill(99),
		},
		{
			"directive form, 99+1 casual",
			"COMMANDER: Bello, Bard of the Brambles\n" + quotaFill(99),
		},
		{
			"partner pair via directive",
			"COMMANDER: Kinnan, Bonder Prodigy\nPARTNER: Thrasios, Triton Hero\n" + quotaFill(98),
		},
		{
			"Commander section header, 99+1",
			"Commander\n1 Atraxa, Praetors' Voice\n\nDeck\n" + quotaFill(99),
		},
		{
			"Commanders section header, partner pair",
			"Commanders (2)\n1 Kinnan, Bonder Prodigy\n1 Thrasios, Triton Hero\n\nDeck\n" + quotaFill(98),
		},
		{
			"inline *CMDR* marker",
			"1 Atraxa, Praetors' Voice *CMDR*\n" + quotaFill(99),
		},
		{
			"// COMMANDER header comment",
			"// COMMANDER\n1 Bello, Bard of the Brambles\n" + quotaFill(99),
		},
		{
			"directive form, 100+1 (some users over-count basics)",
			"COMMANDER: Atraxa, Praetors' Voice\n" + quotaFill(100),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td, err := ParseDeckReader(strings.NewReader(tc.text), nil, fmtMeta())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if td.DetectedFormat != FormatCommander {
				t.Errorf("DetectedFormat = %q, want %q (cmdr=%d lib=%d sig=%d sb=%d hints=%v)",
					td.DetectedFormat, FormatCommander,
					len(td.CommanderCards), len(td.Library),
					td.SignatureSpellCount, td.SideboardCount, td.SourceHints)
			}
		})
	}
}

// TestDetectFormat_Brawl — eight realistic Brawl shapes (1 commander +
// 59 maindeck, no sideboard). Brawl is structurally Commander-minus-
// 40-cards, so detection rides on the total-cards range.
func TestDetectFormat_Brawl(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"directive, 59+1", "COMMANDER: Chulane, Teller of Tales\n" + quotaFill(59)},
		{"directive, 60+1 (rare 60-deck variant)", "COMMANDER: Chulane, Teller of Tales\n" + quotaFill(60)},
		{"Commander section, 59+1", "Commander (1)\n1 Yenna, Redtooth Regent\n\nDeck (59)\n" + quotaFill(59)},
		{"directive Yenna 59+1", "COMMANDER: Yenna, Redtooth Regent\n" + quotaFill(59)},
		{"inline *CMDR* 59+1", "1 Chulane, Teller of Tales *CMDR*\n" + quotaFill(59)},
		{"// COMMANDER 59+1", "// COMMANDER\n1 Yenna, Redtooth Regent\n" + quotaFill(59)},
		{"directive 58+1 (just under)", "COMMANDER: Chulane, Teller of Tales\n" + quotaFill(58)},
		{"Commander section 60+1", "Commander\n1 Yenna, Redtooth Regent\n\nDeck\n" + quotaFill(60)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td, err := ParseDeckReader(strings.NewReader(tc.text), nil, fmtMeta())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if td.DetectedFormat != FormatBrawl {
				t.Errorf("DetectedFormat = %q, want %q (cmdr=%d lib=%d total=%d)",
					td.DetectedFormat, FormatBrawl,
					len(td.CommanderCards), len(td.Library),
					len(td.CommanderCards)+len(td.Library))
			}
		})
	}
}

// TestDetectFormat_Oathbreaker — Oathbreaker = 1 planeswalker commander
// ("the Oathbreaker") + 1 instant/sorcery ("the Signature Spell") + 58
// mainboard. Signature Spells live under a `Signature Spells` section
// header — pre-fix the parser silently dropped them; new
// SignatureSpellCount tracks them for detection.
func TestDetectFormat_Oathbreaker(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{
			"Ajani + Path to Exile, 58 mainboard",
			"COMMANDER: Ajani, Steadfast\n\nSignature Spells (1)\n1 Path to Exile\n\nDeck (58)\n" + quotaFill(58),
		},
		{
			"Saheeli + Lightning Bolt, 58 mainboard",
			"COMMANDER: Saheeli, Sublime Artificer\n\nSignature Spells\n1 Lightning Bolt\n\nDeck\n" + quotaFill(58),
		},
		{
			"Ajani + signature, Commander section form",
			"Commander\n1 Ajani, Steadfast\n\nSignature Spells\n1 Path to Exile\n\nDeck\n" + quotaFill(58),
		},
		{
			"60 mainboard variant (some primers over-count)",
			"COMMANDER: Saheeli, Sublime Artificer\n\nSignature Spells\n1 Lightning Bolt\n\nDeck\n" + quotaFill(60),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td, err := ParseDeckReader(strings.NewReader(tc.text), nil, fmtMeta())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if td.DetectedFormat != FormatOathbreaker {
				t.Errorf("DetectedFormat = %q, want %q (sig=%d lib=%d)",
					td.DetectedFormat, FormatOathbreaker,
					td.SignatureSpellCount, len(td.Library))
			}
		})
	}
}

// TestDetectFormat_Constructed — Standard / Modern / Pioneer / Legacy /
// Vintage / Pauper all collapse to FormatConstructed. Each entry below
// represents one realistic export shape from the named format; the
// detector cannot distinguish them without legality data, so the table
// documents the collapse intentionally.
func TestDetectFormat_Constructed(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"Standard 60+15", repeatLines("Llanowar Elves", 4) + repeatLines("Lightning Bolt", 4) + repeatLines("Forest", 22) + repeatLines("Mountain", 30) + "\nSideboard (15)\n" + repeatLines("Counterspell", 15)},
		{"Modern 60+15", repeatLines("Snapcaster Mage", 4) + repeatLines("Tarmogoyf", 4) + repeatLines("Lightning Bolt", 4) + repeatLines("Force of Will", 4) + repeatLines("Brainstorm", 4) + repeatLines("Ponder", 4) + repeatLines("Forest", 18) + repeatLines("Island", 18) + "\nSideboard (15)\n" + repeatLines("Path to Exile", 15)},
		{"Pioneer 60+15", repeatLines("Lightning Bolt", 4) + repeatLines("Llanowar Elves", 4) + repeatLines("Forest", 26) + repeatLines("Plains", 26) + "\nSideboard\n" + repeatLines("Counterspell", 15)},
		{"Legacy 60+15", repeatLines("Force of Will", 4) + repeatLines("Brainstorm", 4) + repeatLines("Ponder", 4) + repeatLines("Snapcaster Mage", 4) + repeatLines("Island", 22) + repeatLines("Mountain", 22) + "\nSideboard\n" + repeatLines("Counterspell", 15)},
		{"Vintage 60+15 (with Power 9)", "1 Black Lotus\n1 Mox Sapphire\n1 Mox Ruby\n1 Mox Pearl\n" + repeatLines("Force of Will", 4) + repeatLines("Brainstorm", 4) + repeatLines("Snapcaster Mage", 4) + repeatLines("Island", 22) + repeatLines("Tarmogoyf", 20) + "\nSideboard\n" + repeatLines("Counterspell", 15)},
		{"Pauper 60+15 (commons only)", repeatLines("Lava Spike", 4) + repeatLines("Lightning Bolt", 4) + repeatLines("Mulldrifter", 4) + repeatLines("Counterspell", 4) + repeatLines("Mountain", 22) + repeatLines("Island", 22) + "\nSideboard (15)\n" + repeatLines("Ponder", 15)},
		{"Standard 60-no-sideboard (user truncated paste)", repeatLines("Llanowar Elves", 4) + repeatLines("Lightning Bolt", 4) + repeatLines("Forest", 26) + repeatLines("Mountain", 26)},
		{"Modern 75-card export (60 main + 15 SB summed by user)", repeatLines("Snapcaster Mage", 4) + repeatLines("Tarmogoyf", 4) + repeatLines("Lightning Bolt", 4) + repeatLines("Brainstorm", 4) + repeatLines("Ponder", 4) + repeatLines("Force of Will", 4) + repeatLines("Forest", 18) + repeatLines("Island", 18) + "\nSideboard\n" + repeatLines("Path to Exile", 15)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td, err := ParseDeckReader(strings.NewReader(tc.text), nil, fmtMeta())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if td.DetectedFormat != FormatConstructed {
				t.Errorf("DetectedFormat = %q, want %q (cmdr=%d lib=%d sb=%d)",
					td.DetectedFormat, FormatConstructed,
					len(td.CommanderCards), len(td.Library), td.SideboardCount)
			}
		})
	}
}

// TestDetectFormat_Precon — source-comment hint detection. A leading
// `# ... precon ...` or `# ... preconstructed ...` line shadows the
// rules-format (most precons are Commander format but the precon
// distinction is high-signal for UI / build-coaching).
func TestDetectFormat_Precon(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{
			"Animated Army Bloomburrow precon",
			"# Animated Army (Bloomburrow Commander Precon Decklist)\n# Source: https://moxfield.com/decks/GAnCfVPj7EGXBf4ftLgn-A\nCOMMANDER: Bello, Bard of the Brambles\n" + quotaFill(99),
		},
		{
			"Aura of Courage AFR precon",
			"# Aura of Courage (Adventures in the Forgotten Realms Commander Precon Decklist)\nCOMMANDER: Atraxa, Praetors' Voice\n" + quotaFill(99),
		},
		{
			"capitalized PRECON marker",
			"# PRECON: Veloci-Ramp-Tor\nCOMMANDER: Atraxa, Praetors' Voice\n" + quotaFill(99),
		},
		{
			"preconstructed word variant",
			"# Pioneer Challenger Deck (preconstructed)\n" + repeatLines("Lightning Bolt", 4) + repeatLines("Forest", 28) + repeatLines("Mountain", 28),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td, err := ParseDeckReader(strings.NewReader(tc.text), nil, fmtMeta())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if td.DetectedFormat != FormatPrecon {
				t.Errorf("DetectedFormat = %q, want %q (hints=%v)",
					td.DetectedFormat, FormatPrecon, td.SourceHints)
			}
		})
	}
}

// TestDetectFormat_Casual — catch-all for shapes that don't fit any
// other bucket: tiny decks, commander-shape-but-wrong-count, and
// other irregular pastes.
func TestDetectFormat_Casual(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"tiny 5-card deck", "COMMANDER: Atraxa, Praetors' Voice\n" + quotaFill(5)},
		{"commander signal, only 40 cards", "COMMANDER: Atraxa, Praetors' Voice\n" + quotaFill(40)},
		{"commander signal, 70 cards (off-spec)", "COMMANDER: Atraxa, Praetors' Voice\n" + quotaFill(70)},
		{"no commander, 30 cards (kitchen-table half-built)", quotaFill(30)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td, err := ParseDeckReader(strings.NewReader(tc.text), nil, fmtMeta())
			if err != nil {
				// Tiny no-commander deck legitimately errors with the
				// existing "no commander found" guard. That's OK —
				// nothing to assert.
				return
			}
			if td.DetectedFormat != FormatCasual {
				t.Errorf("DetectedFormat = %q, want %q (cmdr=%d lib=%d)",
					td.DetectedFormat, FormatCasual,
					len(td.CommanderCards), len(td.Library))
			}
		})
	}
}

// TestDetectFormat_SideboardCountAccurate — the SideboardCount field
// must reflect the actual cards under a Sideboard header (drives the
// Constructed branch of the detector). Verifies the count plumbing
// independent of the format verdict.
func TestDetectFormat_SideboardCountAccurate(t *testing.T) {
	text := repeatLines("Llanowar Elves", 4) +
		repeatLines("Forest", 56) +
		"\nSideboard (15)\n" +
		repeatLines("Counterspell", 4) +
		repeatLines("Ponder", 4) +
		repeatLines("Brainstorm", 4) +
		repeatLines("Path to Exile", 3)
	td, err := ParseDeckReader(strings.NewReader(text), nil, fmtMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if td.SideboardCount != 15 {
		t.Errorf("SideboardCount = %d, want 15", td.SideboardCount)
	}
}

// TestDetectFormat_SignatureSpellCountAccurate — same shape check for
// SignatureSpellCount. Oathbreaker decks always ship exactly 1 but
// the field should tolerate user errors (2+) without crashing.
func TestDetectFormat_SignatureSpellCountAccurate(t *testing.T) {
	text := "COMMANDER: Ajani, Steadfast\n\nSignature Spells (2)\n1 Path to Exile\n1 Lightning Bolt\n\nDeck\n" + quotaFill(58)
	td, err := ParseDeckReader(strings.NewReader(text), nil, fmtMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if td.SignatureSpellCount != 2 {
		t.Errorf("SignatureSpellCount = %d, want 2", td.SignatureSpellCount)
	}
}

// TestDetectFormat_SourceHintsCapture — leading `#` comments before any
// card content land in SourceHints; mid-deck `#` comments do not (they
// shouldn't false-trigger precon detection).
func TestDetectFormat_SourceHintsCapture(t *testing.T) {
	text := `# Top-level comment line one
# Source: https://moxfield.com/decks/abc
COMMANDER: Atraxa, Praetors' Voice
# this mid-deck comment should NOT land in SourceHints
1 Sol Ring
1 Lightning Bolt
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, fmtMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.SourceHints) != 2 {
		t.Errorf("SourceHints: want 2 (only the leading two), got %d (%v)",
			len(td.SourceHints), td.SourceHints)
	}
	for _, h := range td.SourceHints {
		if strings.Contains(h, "mid-deck") {
			t.Errorf("mid-deck comment leaked into SourceHints: %q", h)
		}
	}
}

// TestDetectFormat_NonConstructedDeckNoLongerErrors — pre-fix a 60-card
// Modern-shape deck (no commander signal) errored out with "no
// commander found". The fix makes ParseDeckReader succeed when no
// commander signal is present AND the library is ≥ 60 cards
// (Constructed-shape). Below 60, the legacy error still fires (those
// are probably truncated pastes, not real Constructed decks).
func TestDetectFormat_NonConstructedDeckNoLongerErrors(t *testing.T) {
	text := repeatLines("Lightning Bolt", 4) +
		repeatLines("Snapcaster Mage", 4) +
		repeatLines("Forest", 26) +
		repeatLines("Mountain", 26)
	td, err := ParseDeckReader(strings.NewReader(text), nil, fmtMeta())
	if err != nil {
		t.Fatalf("60-card no-cmdr deck should not error post-fix: %v", err)
	}
	if td.DetectedFormat != FormatConstructed {
		t.Errorf("DetectedFormat = %q, want %q", td.DetectedFormat, FormatConstructed)
	}
	if len(td.CommanderCards) != 0 {
		t.Errorf("CommanderCards: want 0 (no commander signal), got %d (%v) — auto-pick fallback fired when it shouldn't have",
			len(td.CommanderCards), commanderNames(td))
	}
}
