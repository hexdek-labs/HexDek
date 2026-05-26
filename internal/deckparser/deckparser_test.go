package deckparser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func libraryNames(lib []*gameengine.Card) []string {
	out := make([]string, 0, len(lib))
	for _, c := range lib {
		if c == nil {
			continue
		}
		out = append(out, c.Name)
	}
	return out
}

// astDatasetPath walks up from this test file to find data/rules/ast_dataset.jsonl.
// The dataset is ~46 MB and gitignored (see CLAUDE.md "Data Files"). Tests
// that need real-corpus data t.Skip() when it's absent rather than failing,
// so fresh checkouts run the AST-free unit tests cleanly.
func astDatasetPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 6; i++ {
		c := filepath.Join(dir, "data", "rules", "ast_dataset.jsonl")
		if _, err := os.Stat(c); err == nil {
			return c
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

func TestLoadMeta(t *testing.T) {
	path := astDatasetPath()
	if path == "" {
		t.Skip("no AST dataset available")
	}
	meta, err := LoadMetaFromJSONL(path)
	if err != nil {
		t.Fatalf("LoadMetaFromJSONL: %v", err)
	}
	if meta.Count() < 10_000 {
		t.Fatalf("expected >=10k cards, got %d", meta.Count())
	}
	// Spot-check a well-known card.
	m := meta.Get("Lightning Bolt")
	if m == nil {
		t.Fatalf("Lightning Bolt not found")
	}
	if m.CMC != 1 {
		t.Errorf("Lightning Bolt CMC want 1, got %d", m.CMC)
	}
	if !strings.Contains(strings.Join(m.Types, " "), "instant") {
		t.Errorf("Lightning Bolt types want instant, got %v", m.Types)
	}
}

func TestParseTypes(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"Legendary Creature — Human Ninja", []string{"legendary", "creature", "human", "ninja"}},
		{"Land", []string{"land"}},
		{"Basic Land — Swamp", []string{"basic", "land", "swamp"}},
		{"", nil},
	}
	for _, tc := range cases {
		got := parseTypes(tc.line)
		if len(got) != len(tc.want) {
			t.Errorf("parseTypes(%q): len want %d, got %d (%v)", tc.line, len(tc.want), len(got), got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseTypes(%q) [%d]: want %q got %q", tc.line, i, tc.want[i], got[i])
			}
		}
	}
}

func TestNormalizeName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Lightning Bolt", "lightning bolt"},
		{"  Lightning   Bolt  ", "lightning bolt"},
		{"Jötun Grunt", "jotun grunt"},
		{"Æther Vial", "ether vial"},
	}
	for _, tc := range cases {
		got := normalizeName(tc.in)
		if got != tc.want {
			t.Errorf("normalizeName(%q): want %q got %q", tc.in, tc.want, got)
		}
	}
}

func TestParseDeckReader(t *testing.T) {
	path := astDatasetPath()
	if path == "" {
		t.Skip("no AST dataset")
	}
	meta, err := LoadMetaFromJSONL(path)
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	text := `1 Tergrid, God of Fright // Tergrid's Lantern
1 Swamp
1 Dark Ritual
1 Swamp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		// Tergrid is present in the production AST corpus. If parse fails
		// here, either the corpus has dropped a legendary creature (data
		// regression) or ParseDeckReader has a parser regression on DFC
		// commanders — both worth surfacing loudly instead of skipping.
		t.Fatalf("ParseDeckReader: %v", err)
	}
	if td.CommanderName == "" {
		t.Fatalf("no commander parsed")
	}
	if len(td.Library) < 1 {
		t.Fatalf("library empty")
	}
}

// TestParseDeckReader_PartnerDirective verifies PARTNER: footer parsing.
// Two commanders must both land in CommanderCards; the library excludes
// both. CR §702.124 / §903.3c partner support.
func TestParseDeckReader_PartnerDirective(t *testing.T) {
	text := `1 Kraum, Ludevic's Opus
1 Tymna the Weaver
1 Sol Ring
1 Command Tower

COMMANDER: Kraum, Ludevic's Opus
PARTNER: Tymna the Weaver
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, nil)
	// Without meta+corpus the builder returns nil cards, so we check
	// that the directive was recognised even if cards aren't resolved.
	_ = err
	// Re-run with a stub-friendly path: use a synthetic meta that resolves
	// our 4 lines.
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Kraum, Ludevic's Opus", "Tymna the Weaver", "Sol Ring", "Command Tower"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 3,
		}
	}
	td, err = ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse with synthetic meta: %v", err)
	}
	if len(td.CommanderCards) != 2 {
		t.Fatalf("partner deck: want 2 commanders, got %d (%v)",
			len(td.CommanderCards), td.CommanderNames())
	}
	names := td.CommanderNames()
	if names[0] != "Kraum, Ludevic's Opus" {
		t.Fatalf("first commander wrong: %q", names[0])
	}
	if names[1] != "Tymna the Weaver" {
		t.Fatalf("partner wrong: %q", names[1])
	}
	// Library should NOT contain either commander.
	for _, c := range td.Library {
		if c.Name == "Kraum, Ludevic's Opus" || c.Name == "Tymna the Weaver" {
			t.Fatalf("library still contains commander %q", c.Name)
		}
	}
	if len(td.Library) != 2 {
		t.Fatalf("library should have 2 non-commander entries, got %d", len(td.Library))
	}
}

// TestParseDeckFile_RealPartnerDecks loads the 3 cEDH partner decks on
// disk (Kraum+Tymna, Ardenn+Rograkh, Kinnan+Thrasios) and verifies each
// parses as a two-commander deck. Directly exercises the integration
// path Josh tracked in feature_gap_list.md Tier 1 #5.
func TestParseDeckFile_RealPartnerDecks(t *testing.T) {
	astPath := astDatasetPath()
	if astPath == "" {
		t.Skip("no AST dataset")
	}
	meta, err := LoadMetaFromJSONL(astPath)
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	// Hunt down the deck dir by walking up from this file.
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	var deckDir string
	for i := 0; i < 6; i++ {
		c := filepath.Join(dir, "data", "decks", "benched")
		if _, err := os.Stat(c); err == nil {
			deckDir = c
			break
		}
		dir = filepath.Dir(dir)
	}
	if deckDir == "" {
		t.Skip("benched deck dir not found")
	}

	cases := []struct {
		file  string
		want1 string
		want2 string
	}{
		{"cedh_combo_partner_b5_kraum_tymna.txt", "Kraum, Ludevic's Opus", "Tymna the Weaver"},
		{"cedh_big_stick_b5_ardenn_rograkh.txt", "Ardenn, Intrepid Archaeologist", "Rograkh, Son of Rohgahh"},
		// Kinnan+Thrasios removed: Kinnan doesn't have the Partner keyword,
		// so Kinnan is solo commander with Thrasios in the 99 (validated
		// by the partner agent 2026-04-16). See cedh_turbo_b5_kinnan.txt.
	}
	for _, tc := range cases {
		path := filepath.Join(deckDir, tc.file)
		if _, err := os.Stat(path); err != nil {
			t.Logf("skipping missing %s", tc.file)
			continue
		}
		td, err := ParseDeckFile(path, nil, meta)
		if err != nil {
			t.Errorf("ParseDeckFile(%s): %v", tc.file, err)
			continue
		}
		if len(td.CommanderCards) != 2 {
			t.Errorf("%s: want 2 commanders, got %d (names=%v, unresolved=%v)",
				tc.file, len(td.CommanderCards), td.CommanderNames(), td.Unresolved)
			continue
		}
		names := td.CommanderNames()
		if names[0] != tc.want1 {
			t.Errorf("%s: commander[0] want %q got %q", tc.file, tc.want1, names[0])
		}
		if names[1] != tc.want2 {
			t.Errorf("%s: commander[1] want %q got %q", tc.file, tc.want2, names[1])
		}
		// Library should be positive.
		if len(td.Library) < 50 {
			t.Errorf("%s: library suspiciously small (%d)", tc.file, len(td.Library))
		}
	}
}

func TestParseDeckReader_BareCardNames(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Tinybones, the Pickpocket", "Sol Ring", "Dark Ritual", "Swamp"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	text := `COMMANDER: Tinybones, the Pickpocket
Sol Ring
Dark Ritual
Swamp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse bare names: %v", err)
	}
	if td.CommanderName != "Tinybones, the Pickpocket" {
		t.Fatalf("commander want Tinybones, got %q", td.CommanderName)
	}
	if len(td.Library) != 3 {
		t.Fatalf("library want 3 cards, got %d", len(td.Library))
	}
}

func TestParseDeckReader_SideboardDropped(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Tinybones, the Pickpocket", "Sol Ring", "Dark Ritual", "Swamp", "Lightning Greaves", "Feed the Swarm"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	text := `COMMANDER: Tinybones, the Pickpocket
1 Sol Ring
1 Dark Ritual
1 Swamp

Sideboard
1 Lightning Greaves
1 Feed the Swarm
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse sideboard: %v", err)
	}
	if len(td.Library) != 3 {
		t.Fatalf("library want 3 (sideboard dropped), got %d", len(td.Library))
	}
	for _, c := range td.Library {
		if c.Name == "Lightning Greaves" || c.Name == "Feed the Swarm" {
			t.Fatalf("sideboard card %q leaked into library", c.Name)
		}
	}
}

// TestParseDeckReader_CommanderSectionHeader covers Moxfield's native
// plaintext export, which uses a `Commander` section header rather than
// the `COMMANDER:` directive footer. Prior to r60 this caused the literal
// word "Commander" to be treated as a bare card name (hitting Unresolved)
// while the deck still parsed by accident — the first resolvable line
// became commander. Verify the header-driven path now resolves cleanly
// with no spurious Unresolved entries.
func TestParseDeckReader_CommanderSectionHeader(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Tergrid, God of Fright", "Sol Ring", "Dark Ritual"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 4,
		}
	}
	text := `Commander
1 Tergrid, God of Fright

Deck
1 Sol Ring
1 Dark Ritual
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if td.CommanderName != "Tergrid, God of Fright" {
		t.Fatalf("commander want Tergrid, got %q", td.CommanderName)
	}
	if len(td.Library) != 2 {
		t.Fatalf("library want 2, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
	if len(td.Unresolved) != 0 {
		t.Fatalf("unresolved should be empty, got %v", td.Unresolved)
	}
}

// TestParseDeckReader_MoxfieldCommandersPartner covers Moxfield's native
// export for partner decks: a single `Commanders` section block lists
// both partners, with NO directive footer. Prior to r60 the second
// commander silently leaked into the 99 because partner detection
// required an explicit `PARTNER:` line. This is the worst gap the audit
// surfaced — silent corruption of any partner deck pasted directly out
// of Moxfield.
func TestParseDeckReader_MoxfieldCommandersPartner(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Kraum, Ludevic's Opus", "Tymna the Weaver", "Sol Ring", "Command Tower"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 2,
		}
	}
	text := `Commanders
1 Kraum, Ludevic's Opus
1 Tymna the Weaver

Deck
1 Sol Ring
1 Command Tower
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 2 {
		t.Fatalf("want 2 commanders, got %d (%v)", len(td.CommanderCards), td.CommanderNames())
	}
	names := td.CommanderNames()
	if names[0] != "Kraum, Ludevic's Opus" || names[1] != "Tymna the Weaver" {
		t.Fatalf("commander order wrong: %v", names)
	}
	for _, c := range td.Library {
		if c.Name == "Kraum, Ludevic's Opus" || c.Name == "Tymna the Weaver" {
			t.Fatalf("library still contains commander %q", c.Name)
		}
	}
	if len(td.Library) != 2 {
		t.Fatalf("library want 2 non-commander entries, got %d", len(td.Library))
	}
}

// TestParseDeckReader_FoilAndTagSuffixStripping covers the
// non-set-paren decoration shapes other exporters emit (Archidekt,
// TappedOut, hand-edited Moxfield lists): trailing `*F*`/`*E*` foil
// markers, `[Category]` bracket tags, and `#hashtag` annotations.
// Prior to r60 these survived through to the meta lookup and silently
// landed the card in Unresolved.
func TestParseDeckReader_FoilAndTagSuffixStripping(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Atraxa, Praetors' Voice", "Sol Ring", "Mana Crypt", "Demonic Tutor", "Cyclonic Rift"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring *F*
1 Mana Crypt [foil]
1 Demonic Tutor #wincon
1 Cyclonic Rift *E* [etched] #removal #blowout
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 4 {
		t.Fatalf("library want 4, got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Fatalf("unresolved should be empty, got %v", td.Unresolved)
	}
	for _, c := range td.Library {
		if strings.ContainsAny(c.Name, "*[#") {
			t.Fatalf("library card name still has decoration: %q", c.Name)
		}
	}
}

// TestParseDeckReader_DroppedSectionExpansion covers the new section
// drops: Tokens, Signature Spells, Stickers, Attractions, Outside the
// Game. Anything under those headers must be excluded from the library.
func TestParseDeckReader_DroppedSectionExpansion(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Tinybones, the Pickpocket", "Sol Ring", "Goblin Token", "Demonic Tutor", "Sticker Sheet", "Attraction"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	text := `COMMANDER: Tinybones, the Pickpocket
1 Sol Ring

Tokens
1 Goblin Token

Signature Spells
1 Demonic Tutor

Stickers
1 Sticker Sheet

Attractions
1 Attraction

Outside the Game
1 Demonic Tutor
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 1 {
		t.Fatalf("library want 1 (only Sol Ring), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
}

func TestParseDeckReader_SectionHeaderVariants(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Tinybones, the Pickpocket", "Sol Ring", "Mana Crypt", "Dark Ritual"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	text := `COMMANDER: Tinybones, the Pickpocket
Deck
1 Sol Ring

Sideboard
1 Mana Crypt

Maybeboard
Dark Ritual
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse sections: %v", err)
	}
	if len(td.Library) != 1 {
		t.Fatalf("library want 1 (only Sol Ring from Deck section), got %d", len(td.Library))
	}
}

// TestSupplementWithOracleJSON_MDFCInstantLand regression-tests the fix for
// the MDFC permanent_types bug: an MDFC whose front face is instant/sorcery
// and whose back face is a land has no P/T on either face. The supplement
// loop's pre-fix early-return on (pw==0 && tg==0) skipped these entries
// entirely, leaving BackFaceName empty so IsMDFC()/MDFCBackFaceIsLand()
// returned false at runtime. The non-cast battlefield-entry paths then
// failed to swap to the back face, and the §205 SBA fired permanent_types
// violations.
func TestSupplementWithOracleJSON_MDFCInstantLand(t *testing.T) {
	dir := t.TempDir()
	oraclePath := filepath.Join(dir, "oracle.json")
	// Minimal Scryfall-shaped record for an instant//land MDFC. No P/T
	// on either face — the regression case the fix targets.
	data := `[
		{
			"name": "Malakir Rebirth // Malakir Mire",
			"layout": "modal_dfc",
			"card_faces": [
				{"name": "Malakir Rebirth", "type_line": "Instant", "mana_cost": "{B}"},
				{"name": "Malakir Mire", "type_line": "Land", "mana_cost": ""}
			]
		}
	]`
	if err := os.WriteFile(oraclePath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-seed the MetaDB with the front-face name (mirrors what the
	// AST dataset load would do).
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	meta.byName[normalizeName("Malakir Rebirth")] = &CardMeta{
		Name:     "Malakir Rebirth",
		TypeLine: "Instant",
		Types:    []string{"instant"},
	}
	if err := meta.SupplementWithOracleJSON(oraclePath); err != nil {
		t.Fatalf("SupplementWithOracleJSON: %v", err)
	}
	cm := meta.Get("Malakir Rebirth")
	if cm == nil {
		t.Fatalf("CardMeta missing for Malakir Rebirth")
	}
	if cm.BackFaceName != "Malakir Mire" {
		t.Errorf("BackFaceName: got %q, want %q", cm.BackFaceName, "Malakir Mire")
	}
	if cm.BackFaceTypeLine != "Land" {
		t.Errorf("BackFaceTypeLine: got %q, want %q", cm.BackFaceTypeLine, "Land")
	}
	hasLand := false
	for _, t := range cm.BackFaceTypes {
		if t == "land" {
			hasLand = true
			break
		}
	}
	if !hasLand {
		t.Errorf("BackFaceTypes missing 'land': got %v", cm.BackFaceTypes)
	}
}
