// Package deckparser loads Magic decklists in the Moxfield text format
// used by scripts/gauntlet.py + scripts/gauntlet_poker.py and enriches
// each entry with the metadata the Go gameengine needs (Types slice +
// BasePower / BaseToughness + CMC) drawn from the same AST JSONL file
// the astload.Corpus reads.
//
// This is the semantic peer of Python's gauntlet.parse_deck_file — same
// line format, same "first entry is the commander" convention, same
// silent drop of unresolved names. Type-line parsing matches
// moxfield.splitTypeLine.
//
// The package is a consumer of gameengine + gameast + astload only; it
// does not modify them.
package deckparser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// CardMeta holds the supplementary per-card facts we pull from the
// AST JSONL dataset (type_line, mana_cost, cmc, P/T) that don't live
// on gameast.CardAST itself. Keyed by the same normalizeName logic
// as astload.Corpus so lookups are case/accent-insensitive.
type CardMeta struct {
	Name      string
	TypeLine  string
	Types     []string // folded lowercase, lands get "land", creatures "creature", etc.
	ManaCost  string
	CMC       int
	Colors    []string
	Power     int
	Toughness int

	// MDFC back-face data (populated by SupplementWithOracleJSON for
	// layout=="modal_dfc"). Zero-valued for non-MDFC cards.
	BackFaceName     string
	BackFaceCMC      int
	BackFaceTypeLine string
	BackFaceTypes    []string
}

// MetaDB maps normalized card name -> CardMeta. Built alongside the
// Corpus by LoadMetaFromJSONL.
type MetaDB struct {
	byName map[string]*CardMeta
}

// Count returns how many cards have metadata.
func (m *MetaDB) Count() int {
	if m == nil {
		return 0
	}
	return len(m.byName)
}

// Get returns the CardMeta for `name`, or nil if unknown.
func (m *MetaDB) Get(name string) *CardMeta {
	if m == nil {
		return nil
	}
	return m.byName[normalizeName(name)]
}

// All returns every CardMeta entry in the database. The returned slice
// is a snapshot; callers may filter/sort without affecting the MetaDB.
// Used by the chaos gauntlet to enumerate the full oracle corpus for
// random deck generation.
func (m *MetaDB) All() []*CardMeta {
	if m == nil {
		return nil
	}
	out := make([]*CardMeta, 0, len(m.byName))
	for _, cm := range m.byName {
		out = append(out, cm)
	}
	return out
}

// oracleFace mirrors the subset of Scryfall oracle-cards.json face JSON
// we consume for P/T supplementation.
type oracleFace struct {
	Name      string `json:"name"`
	ManaCost  string `json:"mana_cost"`
	TypeLine  string `json:"type_line"`
	Power     string `json:"power"`
	Toughness string `json:"toughness"`
	Loyalty   string `json:"loyalty"`
	Defense   string `json:"defense"`
}

// oracleCard mirrors the top-level Scryfall oracle-cards.json row.
type oracleCard struct {
	Name      string       `json:"name"`
	Layout    string       `json:"layout"`
	Power     string       `json:"power"`
	Toughness string       `json:"toughness"`
	Loyalty   string       `json:"loyalty"`
	Defense   string       `json:"defense"`
	CardFaces []oracleFace `json:"card_faces"`
}

// SupplementWithOracleJSON augments an existing MetaDB with power/
// toughness values from Scryfall's oracle-cards.json dump. The ast
// dataset does not carry P/T; without this supplement, every creature
// that resolves to the battlefield immediately dies to §704.5f (0
// toughness). Mirrors the data path Python takes through
// playloop.load_card_by_name (which reads directly from
// oracle-cards.json).
//
// For MDFC / split / adventure cards, P/T lives inside the
// per-face card_faces[].power / .toughness fields; we take the
// first face with non-empty P/T. Loyalty (planeswalkers) and defense
// (battles) are slotted into Toughness so the engine's ETB
// initialization has a positive number.
//
// Silently no-ops if path doesn't exist — MetaDB stays functional,
// just with lossy creature combat.
func (m *MetaDB) SupplementWithOracleJSON(path string) error {
	if m == nil {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("deckparser: open oracle %s: %w", path, err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("deckparser: oracle decode start: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return fmt.Errorf("deckparser: oracle expected [, got %v", tok)
	}
	for dec.More() {
		var e oracleCard
		if err := dec.Decode(&e); err != nil {
			continue
		}
		if e.Name == "" {
			continue
		}
		// Resolve the CardMeta entry first — both the P/T merge and the
		// MDFC back-face extraction need it, and an entry with no P/T
		// on either face (instant//land MDFCs like Malakir Rebirth)
		// must NOT be skipped: its back-face metadata is the only way
		// the engine learns the card has a land back face for the
		// EnsureMDFCBackFaceForBattlefield swap to fire.
		norm := normalizeName(e.Name)
		cm := m.byName[norm]
		if cm == nil {
			for _, f := range e.CardFaces {
				if cm = m.byName[normalizeName(f.Name)]; cm != nil {
					break
				}
			}
			if cm == nil {
				continue
			}
		}
		// MDFC back-face data: extract cost + type from face[1] for
		// modal DFCs so the casting system can offer both faces and the
		// battlefield-entry path can swap to the land back face.
		// Independent of P/T — instant//land MDFCs have no P/T on
		// either face but still need this metadata wired.
		if e.Layout == "modal_dfc" && len(e.CardFaces) >= 2 && cm.BackFaceName == "" {
			bf := e.CardFaces[1]
			cm.BackFaceName = bf.Name
			cm.BackFaceCMC = parseMDFCManaCost(bf.ManaCost)
			cm.BackFaceTypeLine = bf.TypeLine
			cm.BackFaceTypes = parseTypes(bf.TypeLine)
		}
		pw, tg := pickOraclePT(&e)
		if pw == 0 && tg == 0 {
			continue
		}
		if cm.Power == 0 {
			cm.Power = pw
		}
		if cm.Toughness == 0 {
			cm.Toughness = tg
		}
	}
	return nil
}

// pickOraclePT picks (power, toughness) from an oracle-cards entry,
// falling back to the first face with numeric P/T. Loyalty / defense
// are slotted into Toughness when Power stays zero (planeswalkers +
// battles don't have P/T, but §704.5i / §704.5s need non-zero
// toughness to avoid immediate SBA death).
func pickOraclePT(e *oracleCard) (int, int) {
	if e == nil {
		return 0, 0
	}
	if pw, ok := atoiStrict(e.Power); ok {
		if tg, ok2 := atoiStrict(e.Toughness); ok2 {
			return pw, tg
		}
	}
	for _, f := range e.CardFaces {
		if pw, ok := atoiStrict(f.Power); ok {
			if tg, ok2 := atoiStrict(f.Toughness); ok2 {
				return pw, tg
			}
		}
	}
	// Planeswalker loyalty or battle defense as a toughness surrogate.
	if loy, ok := atoiStrict(e.Loyalty); ok {
		return 0, loy
	}
	if def, ok := atoiStrict(e.Defense); ok {
		return 0, def
	}
	for _, f := range e.CardFaces {
		if loy, ok := atoiStrict(f.Loyalty); ok {
			return 0, loy
		}
		if def, ok := atoiStrict(f.Defense); ok {
			return 0, def
		}
	}
	return 0, 0
}

// atoiStrict parses an int but rejects empty / X / negative (*, 1+*, etc.)
// entries. Returns (n, true) on success; (0, false) otherwise. Scryfall
// emits "*" / "1+*" for variable P/T — we don't try to resolve those at
// load time; a real engine would defer to CDA.
func atoiStrict(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	if n < 0 {
		return 0, false
	}
	return n, true
}

// LoadMetaFromJSONL reads the same ast_dataset.jsonl file that
// astload.Load reads and builds a CardMeta entry per row. We only
// need top-level fields so we skip the full AST decode.
func LoadMetaFromJSONL(path string) (*MetaDB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("deckparser: open %s: %w", path, err)
	}
	defer f.Close()
	return LoadMetaReader(f)
}

// LoadMetaReader reads JSONL rows from r and builds a MetaDB.
func LoadMetaReader(r io.Reader) (*MetaDB, error) {
	db := &MetaDB{byName: make(map[string]*CardMeta, 32_000)}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var row struct {
			Name      string   `json:"name"`
			TypeLine  string   `json:"type_line"`
			ManaCost  string   `json:"mana_cost"`
			CMC       float64  `json:"cmc"`
			Colors    []string `json:"colors"`
			Power     string   `json:"power"`
			Toughness string   `json:"toughness"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			continue
		}
		if row.Name == "" {
			continue
		}
		types := parseTypes(row.TypeLine)
		pw, _ := strconv.Atoi(row.Power)
		tg, _ := strconv.Atoi(row.Toughness)
		meta := &CardMeta{
			Name:      row.Name,
			TypeLine:  row.TypeLine,
			Types:     types,
			ManaCost:  row.ManaCost,
			CMC:       int(row.CMC + 0.5),
			Colors:    row.Colors,
			Power:     pw,
			Toughness: tg,
		}
		db.byName[normalizeName(row.Name)] = meta
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("deckparser: scan: %w", err)
	}
	return db, nil
}

// TournamentDeck is a deck ready to drop into the tournament runner:
// one commander card (or two for partners) + a pre-built library slice
// of gameengine.Card pointers the runner shuffles per game.
//
// For partner decks (CR §702.124 / §903.3c), CommanderCards has length 2;
// CommanderCards[0] is the COMMANDER: directive target and [1] is the
// PARTNER: directive target. CommanderName is still the primary (index 0)
// name for back-compat with single-commander consumers; iterate
// CommanderCards to get all names.
type TournamentDeck struct {
	Path           string
	CommanderName  string
	CommanderCards []*gameengine.Card
	Library        []*gameengine.Card
	Unresolved     []string // names that weren't in the MetaDB; dropped
	// CardLines preserves the per-line view of the deck source: one entry
	// per source line (with quantity, cleaned name, optional inline `//`
	// comment, and the section the line was parsed under). Library is
	// flattened across copies and loses the line annotations; CardLines
	// is what UI / build-coaching layers consume when they need to
	// preserve a deckbuilder's intent notes ("// mvp", "// flex slot —
	// swap for Cyclonic Rift if available"). Order matches source-line
	// order; commander / partner lines are included with Section set
	// accordingly. Unresolved lines DO appear here so the UI can still
	// surface the deckbuilder's note next to the missing card.
	CardLines []CardLine

	// DetectedFormat is the format autodetector's best categorization.
	// Set at the end of ParseDeckReader from the deck's shape (commander
	// count, mainboard count, sideboard count, signature-spell presence,
	// source-comment hints). Empty (FormatUnknown) only when the deck
	// shipped no usable signal. See detectFormat for the full decision
	// tree.
	DetectedFormat DetectedFormat

	// SideboardCount counts cards seen under a Sideboard section header.
	// The cards themselves are still dropped from Library (preserving
	// the existing single-zone tournament-runner contract); this just
	// tracks the count so the format detector can distinguish Constructed
	// (sideboard ≥ ~5) from Casual.
	SideboardCount int

	// SignatureSpellCount counts cards seen under a "Signature Spells"
	// section header — the Oathbreaker format's instant/sorcery slot
	// (1 per deck). Cards still dropped from Library, count tracked for
	// format detection.
	SignatureSpellCount int

	// SourceHints captures leading `#` comments before any card content
	// (e.g. `# Source: https://moxfield.com/decks/...` or
	// `# Precon Decklist`). Used by detectFormat for precon recognition;
	// also useful for UI provenance display.
	SourceHints []string

	// ParseReport is the structured per-line resolution coverage report.
	// Populated at the end of every successful ParseDeckReader call (not
	// populated on parse error). Drives the hexdek-judge --report-parse
	// output and any UI that wants to surface "X / N lines resolved" to
	// the deckbuilder so they can spot typos, renamed cards, and meta
	// gaps. CardLines carries the per-line status; this struct is the
	// roll-up + unresolved-detail summary.
	ParseReport ParseReport
}

// DetectedFormat is the format-detector's verdict for a parsed deck.
// Detection is structural / count-based, not legality-aware — Standard /
// Modern / Pioneer / Legacy / Vintage / Pauper all share the same shape
// (60+ mainboard, ≤15 sideboard, multiples allowed) and collapse to a
// single FormatConstructed bucket; refining further requires a card-
// legality database that the deckparser package deliberately does not
// own. FormatPrecon shadows the rules-format when a source comment
// flags the deck as a published preconstructed product — most precons
// are Commander, but distinguishing "literal precon decklist" from
// "user-built Commander deck" is high-signal for UI / build-coaching.
type DetectedFormat string

const (
	FormatUnknown     DetectedFormat = ""
	FormatCommander   DetectedFormat = "commander"
	FormatBrawl       DetectedFormat = "brawl"
	FormatOathbreaker DetectedFormat = "oathbreaker"
	FormatConstructed DetectedFormat = "constructed"
	FormatPrecon      DetectedFormat = "precon"
	FormatCasual      DetectedFormat = "casual"
)

// CardLine is the per-source-line view of a parsed deck. Section is one
// of "main", "commander". Comment is everything after a `//` token on
// the same line (TrimSpace'd, empty if no inline comment).
//
// Status is the parser's verdict on the line: LineStatusResolved when
// the name hit meta on the first lookup, LineStatusFallbackResolved
// when buildCard had to walk the face-match / DFC-canonicalize
// fallback (the card is still in Library but the source name was
// non-canonical), LineStatusUnresolved when buildCard returned nil
// (card dropped from Library, name surfaced in Unresolved). Drives the
// --report output in hexdek-judge.
//
// LineNumber is the 1-based source line in the original file; used by
// the report renderer to point the user at the broken line.
type CardLine struct {
	Qty        int
	Name       string
	Comment    string
	Section    string
	Status     LineStatus
	LineNumber int

	// HashTags preserves trailing `#tag1 #tag2 ...` annotations from
	// the source line. Pre-fix these were stripped by hashTagRE
	// during cleanCardName and the tag content was lost entirely —
	// the parser resolved the card correctly but UIs trying to round-
	// trip the deck (and any tooling consuming deckbuilder intent
	// like "#wincon", "#ramp", "#flex") had no way to recover them.
	// Stored without the leading `#` (so `#wincon` → "wincon") and
	// in source order. Drives the WriteText round-trip path: hashtags
	// re-emit verbatim with their leading `#` restored.
	HashTags []string
}

// LineStatus is the resolution verdict for a CardLine. Drives the
// per-line status column of the --report output.
type LineStatus int

const (
	LineStatusUnknown          LineStatus = iota // not yet resolved (commander section lines before final assembly)
	LineStatusResolved                           // meta direct hit
	LineStatusFallbackResolved                   // resolved via face-match / DFC canonicalization / corpus-only path
	LineStatusUnresolved                         // buildCard returned nil; card dropped from Library
)

// String returns a stable short label per status, used as the report's
// per-line status column.
func (s LineStatus) String() string {
	switch s {
	case LineStatusResolved:
		return "resolved"
	case LineStatusFallbackResolved:
		return "fallback"
	case LineStatusUnresolved:
		return "unresolved"
	default:
		return "unknown"
	}
}

// ParseReport is the parser's structured coverage report — every
// CardLine's resolution status rolled up, plus per-failure detail for
// the unresolved set. Populated in TournamentDeck.ParseReport at the
// end of every successful ParseDeckReader call. Empty / zero on parse
// error.
type ParseReport struct {
	TotalLines        int              // total card-shaped source lines (commander + main + dropped card lines)
	ResolvedLines     int              // direct-meta-hit count
	FallbackResolved  int              // face-match / DFC-canonicalize / corpus-only path count
	UnresolvedLines   int              // buildCard returned nil
	DroppedLines      int              // sideboard / signature-spells / etc. (counted but not resolved)
	UnresolvedDetails []UnresolvedLine // per-failure detail (LineNumber + raw context + reason)
}

// CoveragePercent returns the percentage of card-shaped lines (excl.
// intentionally-dropped sections) that resolved cleanly. 100% means
// every mainboard / commander line resolved. Returns 0 when there are
// no resolvable lines (defensive — avoids divide-by-zero).
func (r ParseReport) CoveragePercent() float64 {
	resolvable := r.ResolvedLines + r.FallbackResolved + r.UnresolvedLines
	if resolvable == 0 {
		return 0
	}
	return float64(r.ResolvedLines+r.FallbackResolved) * 100.0 / float64(resolvable)
}

// UnresolvedLine is the per-failure detail row in a ParseReport.
//
// Suggestions carries the autosuggest output from MetaDB.SuggestSimilarNames
// (up to 3 closest matches by Levenshtein distance). Empty when no
// candidate is within the per-input distance threshold or when the
// parser ran without a usable meta. Drives the "Did you mean X?" line
// in PrintReport's per-failure block.
type UnresolvedLine struct {
	LineNumber  int              // 1-based source line in the original file
	Raw         string           // the original raw line (TrimSpace'd, post-HTML-strip)
	Name        string           // best-effort extracted name (post-clean, post-DFC-normalize)
	Section     string           // "commander" / "main" — where the line was routed
	Reason      string           // human-readable failure reason
	Suggestions []NameSuggestion // top-N closest meta names by Levenshtein (empty when none qualify)
}

// CommanderNames returns the display names of every commander in the
// deck (1 for single commander, 2 for partner pairs). Convenience for
// callers that need to feed gameengine.SetupCommanderGame or check
// partner legality.
func (td *TournamentDeck) CommanderNames() []string {
	if td == nil {
		return nil
	}
	out := make([]string, 0, len(td.CommanderCards))
	for _, c := range td.CommanderCards {
		if c == nil {
			continue
		}
		out = append(out, c.DisplayName())
	}
	return out
}

// ParseDeckFile reads a Moxfield-format text decklist and enriches
// every entry with metadata from corpus + meta. First entry is treated
// as the commander (matches Python gauntlet.parse_deck_file).
//
// The deck is stored by-value so callers can rebuild per-game libraries
// (deep-copying cards) without disturbing the parse result.
func ParseDeckFile(path string, corpus *astload.Corpus, meta *MetaDB) (*TournamentDeck, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("deckparser: open %s: %w", path, err)
	}
	// Structured .json decks (the UI/import-served format) are converted
	// to the plaintext decklist the text parser consumes, so JSON and
	// plaintext share one parse path + corpus resolution. Detection is by
	// content, not extension, so a mislabeled file still parses correctly.
	text := string(data)
	if looksLikeJSONDeck(data) {
		list, jerr := jsonDeckToDecklist(data)
		if jerr != nil {
			return nil, fmt.Errorf("deckparser: parse json deck %s: %w", path, jerr)
		}
		text = list
	}
	deck, err := ParseDeckReader(strings.NewReader(text), corpus, meta)
	if err != nil {
		return nil, err
	}
	deck.Path = path
	return deck, nil
}

// looksLikeJSONDeck reports whether data is a JSON object (first
// non-whitespace byte is '{') — the structured deck format. Plaintext
// decklists start with a quantity digit or a card name.
func looksLikeJSONDeck(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// jsonDeckToDecklist converts the structured deck JSON
// ({"commander":..., "mainboard":[{"name","quantity"}...]}) into the
// `<qty> <name>` lines + trailing `COMMANDER:` directive that
// ParseDeckReader consumes. Reusing the text path means JSON decks get
// identical corpus resolution, commander stripping, and metadata
// enrichment as plaintext decks — one source of truth.
func jsonDeckToDecklist(data []byte) (string, error) {
	var d struct {
		Commander string `json:"commander"`
		Mainboard []struct {
			Name     string `json:"name"`
			Quantity int    `json:"quantity"`
		} `json:"mainboard"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return "", err
	}
	if len(d.Mainboard) == 0 {
		return "", fmt.Errorf("json deck has empty mainboard")
	}
	var b strings.Builder
	for _, c := range d.Mainboard {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			continue
		}
		q := c.Quantity
		if q < 1 {
			q = 1
		}
		fmt.Fprintf(&b, "%d %s\n", q, name)
	}
	// COMMANDER: directive last (footer form). ParseDeckReader strips the
	// matching card from the library when it also appears in the mainboard.
	if cmd := strings.TrimSpace(d.Commander); cmd != "" {
		fmt.Fprintf(&b, "COMMANDER: %s\n", cmd)
	}
	return b.String(), nil
}

// ParseDeckReader is the stream version of ParseDeckFile.
//
// Supports the `COMMANDER: <name>` directive (case-insensitive; mirrors
// scripts/gauntlet.py _COMMANDER_RE) AND the `PARTNER: <name>` directive
// for two-commander decks (CR §702.124 / §903.3c). When no directive is
// present, the first resolvable card is treated as the commander.
//
// Partner handling: if a PARTNER: footer line is present, the named card
// becomes a second commander, added to TournamentDeck.CommanderCards. The
// library is built from the non-commander entries (both commander copies
// stripped when present in the main list). Legality is NOT enforced here;
// call gameengine.ValidatePartnerPair on the resulting CommanderCards to
// verify both cards have the Partner keyword or a valid partner-with pair.
func ParseDeckReader(r io.Reader, corpus *astload.Corpus, meta *MetaDB) (*TournamentDeck, error) {
	td := &TournamentDeck{}

	type lineEntry struct {
		qty     int
		name    string
		comment string
		section string
		lineNum int    // 1-based source line — used by ParseReport
		raw     string // post-HTML-strip raw line, for unresolved detail
	}
	var explicitCommander string
	var explicitPartner string
	var lines []lineEntry
	// commanderSectionNames: cards seen under a Moxfield "Commander" /
	// "Commanders" section header. These are an alternative to the
	// COMMANDER:/PARTNER: directive footers — Moxfield's native plaintext
	// export uses section headers, not directives, and partner decks list
	// both commanders under "Commanders" with no footer at all.
	var commanderSectionNames []string
	section := "main"
	// dropSubtype tracks WHICH dropped section we're inside: "sideboard"
	// / "signature_spells" / "other". Used by the format detector — a
	// 15-card sideboard distinguishes Constructed from Casual, and the
	// presence of any signature spells flags Oathbreaker. Cards
	// themselves are still dropped from Library; only counts are kept.
	dropSubtype := ""
	sectionDrops := map[string]bool{
		"sideboard": true, "maybeboard": true, "considering": true,
		"companion": true, "tokens": true, "signature spells": true,
		"stickers": true, "attractions": true, "outside the game": true,
		// Aetherhub preamble — `About` precedes a `Name <DeckName>` line
		// (and sometimes a `Format Commander` / `Description ...` block)
		// before the real `Commander` / `Deck` section headers. Drop the
		// whole block until a recognized section header reasserts state.
		"about": true,
	}
	// sawAnyCardContent: SourceHints captures leading `#` comments only
	// — once a card line / directive has been seen, subsequent `#` lines
	// are mid-deck annotations and shouldn't pollute the precon hint
	// scan. Flipped true on the first real content line.
	sawAnyCardContent := false
	// otherDropCardLines counts card-shaped lines dropped under sections
	// OTHER than Sideboard / Signature Spells (Maybeboard, Companion,
	// Tokens, Stickers, Attractions, etc.). Sums into ParseReport.DroppedLines
	// alongside SideboardCount / SignatureSpellCount.
	otherDropCardLines := 0
	// unresolvedDetails accumulates per-failure context for the ParseReport.
	// Each entry records the source line number, raw text, extracted name,
	// section, and a human-readable reason — the data the --report-parse
	// CLI surfaces so the deckbuilder can find the broken line.
	var unresolvedDetails []UnresolvedLine
	// hasExplicitCommanderSignal: true when the deck shipped any of the
	// explicit "this is a Commander-format deck" markers — COMMANDER: /
	// PARTNER: directive, a `Commander[s]` section header that ate at
	// least one card, a `// COMMANDER` directive comment, or an inline
	// `*CMDR*` marker. The format detector uses this to distinguish a
	// genuine Commander deck (commander signal + total ≈ 100) from a
	// 60-card Constructed deck where the legacy auto-pick fallback
	// pulled the first card into the commander slot. Auto-pick alone
	// does NOT flip this flag.
	hasExplicitCommanderSignal := false

	// pendingCommanderHeader: set when the previous non-blank line was a
	// `// COMMANDER` directive-comment. The NEXT card line consumes the
	// flag and routes its card to commander slots instead of mainboard.
	// Mirrors the Archidekt category-marker convention.
	pendingCommanderHeader := false

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024), 1024*1024)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		raw := strings.TrimSpace(sc.Text())
		rawOriginal := raw // preserved for ParseReport.UnresolvedDetails
		// MTGGoldfish HTML strip — `<br>`, `<td>`, `<div class="...">`,
		// `</p>`, etc. leak in from "Save as HTML" exports and browser
		// drag-drop pastes. Replace every well-formed `<...>` tag with
		// a SPACE (not empty) so adjacent words don't collide:
		// `<td>4</td><td>Lightning Bolt</td>` must become `4 Lightning
		// Bolt`, not `4Lightning Bolt`. The Fields/Join pass collapses
		// the resulting runs of whitespace back to single spaces. Lines
		// that were ENTIRELY HTML (e.g. `<div class="deck">`) collapse
		// to empty and short-circuit at the blank check below.
		if strings.ContainsRune(raw, '<') {
			raw = strings.Join(strings.Fields(htmlTagRE.ReplaceAllString(raw, " ")), " ")
		}
		// `// COMMANDER` / `// CMDR` directive-comment: flag the NEXT card
		// line as commander. Must run before the generic `//` drop below
		// or the directive would be silently swallowed.
		if cmdrHeaderCommentRE.MatchString(raw) {
			pendingCommanderHeader = true
			hasExplicitCommanderSignal = true
			continue
		}
		if raw == "" {
			continue
		}
		// Tappedout `#!<section>` directive — sets the current section
		// the same way a `Sideboard` / `Commander` header line does, but
		// uses Tappedout's hash-bang convention instead of a bare label.
		// Must run BEFORE the generic `#` comment drop below or the
		// directive would be silently swallowed. Recognized sections:
		// Commander, Mainboard / Maindeck / Deck, Sideboard, Maybeboard,
		// Companion. Anything else routes to drop with subtype "other".
		if m := tappedoutDirectiveRE.FindStringSubmatch(raw); m != nil {
			label := strings.ToLower(strings.TrimSpace(m[1]))
			switch label {
			case "commander", "commanders":
				section = "commander"
				dropSubtype = ""
			case "mainboard", "maindeck", "deck", "main":
				section = "main"
				dropSubtype = ""
			case "sideboard":
				section = "drop"
				dropSubtype = "sideboard"
			case "signature spells", "signaturespells":
				section = "drop"
				dropSubtype = "signature_spells"
			default:
				section = "drop"
				dropSubtype = "other"
			}
			continue
		}
		if strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
			// Leading `#` comments (before any card content) become
			// SourceHints — detectFormat scans them for "precon" markers
			// and UIs can surface them as provenance. Mid-deck comments
			// are still dropped silently. MTGTop8 / MWS exports use
			// `// NAME:` / `// FORMAT:` / `// CREATOR:` / `// DATE:`
			// preamble lines; capture those into hints too so the
			// format-detector can pick up format hints (`// FORMAT:
			// Legacy` could later refine FormatConstructed → Legacy when
			// a legality DB is wired in).
			if !sawAnyCardContent {
				if strings.HasPrefix(raw, "#") {
					hint := strings.TrimSpace(strings.TrimLeft(raw, "#"))
					if hint != "" {
						td.SourceHints = append(td.SourceHints, hint)
					}
				} else if mtgtop8MetaCommentRE.MatchString(raw) {
					// `// NAME: Storm`, `// FORMAT: Legacy`, etc.
					hint := strings.TrimSpace(strings.TrimLeft(raw, "/"))
					if hint != "" {
						td.SourceHints = append(td.SourceHints, hint)
					}
				}
			}
			continue
		}
		// Tappedout `**Markdown Bold**` section header wrapper —
		// `**Commanders (1):**` / `**Lands (38):**`. Strip the `**` so
		// sectionHeaderRE / typeCategoryHeaderRE see the bare label.
		if m := markdownBoldHeaderRE.FindStringSubmatch(raw); m != nil {
			raw = strings.TrimSpace(m[1])
		}
		// Tappedout bullet-list line prefix — `* 1 Lightning Bolt`,
		// `- 1 Sol Ring`, `• 1 Forest`. Strip the bullet so the
		// downstream qty + name extraction sees a normal `1 Card` line.
		if loc := bulletPrefixRE.FindStringIndex(raw); loc != nil {
			raw = strings.TrimSpace(raw[loc[1]:])
		}
		// Trailing price tag strip — `$1.50` / `€1,50` / `£2.00` /
		// `$0.50 USD`. Tappedout and some Aetherhub exports append
		// per-card prices to the line tail. No real card name contains
		// these patterns so the strip is safe.
		if loc := priceTagRE.FindStringIndex(raw); loc != nil {
			raw = strings.TrimSpace(raw[:loc[0]])
		}
		// COMMANDER: <name> — Moxfield's directive-style export uses two
		// COMMANDER: lines for partners. Distinct from the section-header
		// path below.
		if m := commanderLineRE.FindStringSubmatch(raw); m != nil {
			name := normalizeDFCSeparator(cleanCardName(strings.TrimSpace(m[1])), meta)
			if explicitCommander == "" {
				explicitCommander = name
			} else if explicitPartner == "" {
				explicitPartner = name
			}
			sawAnyCardContent = true
			hasExplicitCommanderSignal = true
			continue
		}
		// PARTNER: <name>
		if m := partnerLineRE.FindStringSubmatch(raw); m != nil {
			explicitPartner = normalizeDFCSeparator(cleanCardName(strings.TrimSpace(m[1])), meta)
			sawAnyCardContent = true
			hasExplicitCommanderSignal = true
			continue
		}
		// Section headers: Sideboard, Maybeboard, Companion, Commander,
		// Commanders, Tokens, etc. Commander section feeds the commander
		// slots directly so Moxfield's native export (no directive footer)
		// preserves partner pairs.
		if sm := sectionHeaderRE.FindStringSubmatch(raw); sm != nil {
			label := strings.ToLower(strings.TrimSpace(sm[1]))
			label = strings.Join(strings.Fields(label), " ") // collapse whitespace
			switch {
			case label == "commander" || label == "commanders":
				section = "commander"
				dropSubtype = ""
			case label == "deck" || label == "mainboard" || strings.HasPrefix(label, "main"):
				section = "main"
				dropSubtype = ""
			case sectionDrops[label]:
				section = "drop"
				switch label {
				case "sideboard":
					dropSubtype = "sideboard"
				case "signature spells":
					dropSubtype = "signature_spells"
				default:
					dropSubtype = "other"
				}
			default:
				section = "drop"
				dropSubtype = "other"
			}
			continue
		}
		// Type-category headers from Moxfield's "Card View" copy-paste and
		// Archidekt's "By Type" export: `Creatures (24)`, `Sorceries (5)`,
		// `Lands (38)`, etc. These are NOT alternate boards — they're
		// mainboard sub-categorizations. Pre-fix they fell through to the
		// fallback as qty=1 fake cards, AND (worse) they stayed glued to
		// the prior `Commander` section so the next 23 creature lines all
		// routed to commander slots (second one becoming a bogus partner)
		// and the entire library landed empty. Drop the line, and if we
		// were still in the commander section, transition to mainboard —
		// these headers reliably mark the end of the commander block.
		// Don't touch `section == "drop"` (we might be sub-categorizing
		// inside a sideboard that has its own type buckets — staying
		// dropped is safer than re-promoting cards into main).
		if typeCategoryHeaderRE.MatchString(raw) {
			if section == "commander" {
				section = "main"
			}
			continue
		}
		if section == "drop" {
			// Count card-shaped lines inside Sideboard / Signature Spells
			// drops so the format detector can distinguish Constructed
			// (sideboard) and Oathbreaker (signature spell) from the
			// commander / casual variants. Card pointer stays dropped;
			// only the count is tracked. Other drop sub-sections
			// (Maybeboard, Companion, Tokens, etc.) feed
			// otherDropCardLines for the report's DroppedLines roll-up.
			if dm := deckLineRE.FindStringSubmatch(raw); dm != nil {
				if q, err := strconv.Atoi(dm[1]); err == nil && q > 0 {
					switch dropSubtype {
					case "sideboard":
						td.SideboardCount += q
					case "signature_spells":
						td.SignatureSpellCount += q
					default:
						otherDropCardLines += q
					}
				}
			}
			sawAnyCardContent = true
			continue
		}
		// MTGA / Aetherhub sideboard line prefix. These appear interleaved
		// with mainboard lines instead of under a Sideboard header, so the
		// section-based drop above doesn't catch them. Drop the line; the
		// rest of the file may still be mainboard. Count the line against
		// SideboardCount so the format detector still sees a sideboard.
		if loc := sbPrefixRE.FindStringIndex(raw); loc != nil {
			tail := strings.TrimSpace(raw[loc[1]:])
			if dm := deckLineRE.FindStringSubmatch(tail); dm != nil {
				if q, err := strconv.Atoi(dm[1]); err == nil && q > 0 {
					td.SideboardCount += q
				}
			}
			sawAnyCardContent = true
			continue
		}
		// MTGO / Tournament-Ready metadata header lines (`Deck name: ...`,
		// `Format: Commander`, etc.). Drop entirely; they're informational
		// and have no card content.
		if mtgoMetadataRE.MatchString(raw) {
			continue
		}
		// Strip inline `// comment` and capture it for CardLine.Comment.
		// Runs before the paren strip so a Moxfield-style line with both
		// a `(SET) 123` tail and a `// comment` (rare but possible)
		// peels in the right order: comment first, then printing tail.
		//
		// CRITICAL: DFC / split / adventure card names also use ` // ` as
		// the face separator (Moxfield's canonical export format —
		// `1 Aang, Swift Savior // Aang and La, Ocean's Fury`). The naive
		// strip would treat the back-face name as a "comment", losing
		// the DFC suffix from CardLine.Name and polluting CardLine.Comment
		// with a fake user note. ~3K lines across the curated corpus hit
		// this shape. Disambiguate by reconstructing the full DFC name
		// (LHS + " // " + RHS, each with qty / set-parens trimmed) and
		// probing meta: a known card means the ` // ` is the DFC face
		// separator and the strip is skipped.
		var inlineComment string
		if cm := inlineCommentRE.FindStringSubmatchIndex(raw); cm != nil {
			lhs := raw[:cm[0]]
			if dm := deckLineRE.FindStringSubmatch(lhs); dm != nil {
				lhs = strings.TrimSpace(dm[2])
			}
			if pi := strings.Index(lhs, "("); pi > 0 {
				lhs = strings.TrimSpace(lhs[:pi])
			}
			rhs := strings.TrimSpace(raw[cm[2]:cm[3]])
			if pi := strings.Index(rhs, "("); pi > 0 {
				rhs = strings.TrimSpace(rhs[:pi])
			}
			fullDFCProbe := cleanCardName(lhs + " // " + rhs)
			if meta == nil || meta.Get(fullDFCProbe) == nil {
				inlineComment = strings.TrimSpace(raw[cm[2]:cm[3]])
				raw = strings.TrimSpace(raw[:cm[0]])
			}
		}
		// Inline `*CMDR*` / `*Commander*` marker: peel + flag this line
		// as commander. Sibling to the foilMarkerRE strip but with
		// semantic meaning. Captured here so the marker doesn't leak
		// into cleanCardName's resolved name.
		lineIsCommander := false
		if cmdrInlineMarkerRE.MatchString(raw) {
			raw = strings.TrimSpace(cmdrInlineMarkerRE.ReplaceAllString(raw, " "))
			lineIsCommander = true
			hasExplicitCommanderSignal = true
		}
		// Archidekt per-line category bracket — `1 Atraxa (CMR) 222 [Commander{top}]`,
		// `1 Sol Ring [Ramp]`, `1 Counterspell [Sideboard]`. Each line carries
		// its own category annotation. Distinguished from MTGO-style set-code
		// brackets (`[LEA]`, `[KHM]`) by the case heuristic in
		// isArchidektCategoryLabel — Archidekt categories always contain at
		// least one lowercase letter (Commander / Sideboard / Ramp / Wincons
		// / etc.); all-uppercase content is left for bracketTagRE's set-code
		// strip. Must run before the set-parens strip below: set-parens
		// always come BEFORE the bracket in real Archidekt exports
		// (`...(CMR) 222 [Cat]`), so peeling the trailing bracket first
		// lets the set-parens strip fire normally afterward.
		lineRouteDrop := false
		lineRouteDropSubtype := ""
		if am := archidektCategoryRE.FindStringSubmatch(raw); am != nil {
			label := strings.TrimSpace(am[1])
			if isArchidektCategoryLabel(label) {
				cat := strings.ToLower(label)
				cat = strings.Join(strings.Fields(cat), " ")
				switch {
				case cat == "commander" || cat == "commanders":
					lineIsCommander = true
					hasExplicitCommanderSignal = true
				case cat == "sideboard":
					lineRouteDrop = true
					lineRouteDropSubtype = "sideboard"
				case cat == "signature spells":
					lineRouteDrop = true
					lineRouteDropSubtype = "signature_spells"
				case sectionDrops[cat]:
					lineRouteDrop = true
					lineRouteDropSubtype = "other"
				}
				// Strip the bracket regardless of routing: mainboard sub-
				// categories (Ramp / Removal / Wincons / Creatures / Lands)
				// fall through with the bracket peeled so name resolution
				// sees a clean line.
				raw = strings.TrimSpace(archidektCategoryRE.ReplaceAllString(raw, ""))
			}
		}
		// Fuzzy quantity normalization — transform alternate qty syntaxes
		// (`x4 Card`, `Card x4`, `Card (4)`) into the canonical
		// `<qty> Card` form before set-parens stripping eats `(4)`. Only
		// fires when the line doesn't already start with a digit so the
		// canonical `4 Card` path stays a no-op and the existing
		// deckLineRE keeps owning the primary qty extraction.
		raw = normalizeFuzzyQuantity(raw)
		// Extract trailing `#tag1 #tag2 ...` hashtags into a per-line
		// list before set-parens strip eats them. Pre-fix hashTagRE
		// inside cleanCardName silently dropped the tags; round-trip
		// tooling (UI build-coaching, deck-export) had no way to
		// recover deckbuilder intent like "#ramp", "#wincon", "#flex".
		// Stored on CardLine.HashTags without the leading `#`.
		var lineHashTags []string
		raw, lineHashTags = extractHashTags(raw)
		// Strip "(SET) 123" suffix (set code + collector number + foil flag).
		if idx := strings.Index(raw, "("); idx > 0 {
			raw = strings.TrimSpace(raw[:idx])
		}
		m := deckLineRE.FindStringSubmatch(raw)
		var qty int
		var name string
		if m != nil {
			qty, _ = strconv.Atoi(m[1])
			if qty < 1 {
				continue
			}
			name = m[2]
			// Deckbox multi-tab metadata strip — Deckbox's full-inventory
			// CSV/TSV export appends edition / condition / language / foil
			// fields after the name on the same line, tab-separated. The
			// first tab-separated field after qty IS the name; everything
			// from the second tab onward is metadata that pollutes meta
			// lookups. `\s+` in deckLineRE already collapses the qty-name
			// tab boundary, so this strip only removes the second-and-
			// beyond tabs. No real card name contains tabs.
			name = deckboxTabExtraRE.ReplaceAllString(name, "")
		} else {
			qty = 1
			name = raw
		}
		// Per-line Archidekt drop (sideboard / signature spells / other):
		// count and continue. Cards still dropped from Library; the count
		// is what the format detector needs.
		if lineRouteDrop {
			switch lineRouteDropSubtype {
			case "sideboard":
				td.SideboardCount += qty
			case "signature_spells":
				td.SignatureSpellCount += qty
			default:
				otherDropCardLines += qty
			}
			sawAnyCardContent = true
			continue
		}
		name = normalizeDFCSeparator(cleanCardName(name), meta)
		if name == "" {
			continue
		}
		sawAnyCardContent = true
		// A `// COMMANDER` header on the previous line OR an inline
		// `*CMDR*` marker on this line routes the card to commander
		// slots regardless of the current Section state. The pending
		// flag consumes on the first non-blank card line that follows.
		if pendingCommanderHeader || lineIsCommander {
			pendingCommanderHeader = false
			for i := 0; i < qty; i++ {
				commanderSectionNames = append(commanderSectionNames, name)
			}
			td.CardLines = append(td.CardLines, CardLine{
				Qty: qty, Name: name, Comment: inlineComment, Section: "commander",
				LineNumber: lineNum, HashTags: lineHashTags,
			})
			continue
		}
		if section == "commander" {
			// Each card under a Commander section becomes a commander slot
			// (up to 2 for partner pairs). Extras are silently dropped per
			// CR §903.5b.
			for i := 0; i < qty; i++ {
				commanderSectionNames = append(commanderSectionNames, name)
			}
			td.CardLines = append(td.CardLines, CardLine{
				Qty: qty, Name: name, Comment: inlineComment, Section: "commander",
				LineNumber: lineNum, HashTags: lineHashTags,
			})
			hasExplicitCommanderSignal = true
			continue
		}
		lines = append(lines, lineEntry{
			qty: qty, name: name, comment: inlineComment, section: "main",
			lineNum: lineNum, raw: rawOriginal,
		})
		td.CardLines = append(td.CardLines, CardLine{
			Qty: qty, Name: name, Comment: inlineComment, Section: "main",
			LineNumber: lineNum, HashTags: lineHashTags,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("deckparser: scan: %w", err)
	}

	// Promote `Commander` / `Commanders` section entries into the
	// explicit commander slots so the directive-style and section-style
	// Moxfield exports share one resolution path.
	if explicitCommander == "" && len(commanderSectionNames) > 0 {
		explicitCommander = commanderSectionNames[0]
		if explicitPartner == "" && len(commanderSectionNames) > 1 {
			explicitPartner = commanderSectionNames[1]
		}
	} else if explicitPartner == "" && len(commanderSectionNames) > 0 {
		// Directive named the primary; section may still carry a partner.
		for _, n := range commanderSectionNames {
			if normalizeName(n) != normalizeName(explicitCommander) {
				explicitPartner = n
				break
			}
		}
	}

	// Commander + partner resolution. Names to pull out of the main
	// library slice: the COMMANDER: entry (one copy), plus the PARTNER:
	// entry (one copy) if present. CR §903.5b forbids duplicates so
	// any extras in the main list are ignored.
	//
	// We resolve cards into `commanderCard` / `partnerCard` local slots
	// first and append them to td.CommanderCards in a deterministic
	// commander-then-partner order at the end. That way iteration order
	// through the main list doesn't flip which card is the primary
	// commander (a real bug hit by the Kinnan deck where Thrasios appears
	// in the main list before Kinnan's COMMANDER: footer is parsed).
	commanderName := explicitCommander
	partnerName := explicitPartner
	var commanderCard, partnerCard *gameengine.Card
	commanderTaken := false
	partnerTaken := partnerName == ""

	commanderMatch := func(name string) bool {
		if commanderName == "" {
			return false
		}
		return normalizeName(name) == normalizeName(commanderName)
	}
	partnerMatch := func(name string) bool {
		if partnerName == "" {
			return false
		}
		return normalizeName(name) == normalizeName(partnerName)
	}

	// Pass 1: if no explicit commander, take the first resolvable entry
	// as the commander. Partner requires the explicit directive — we
	// won't guess partner from the main list.
	//
	// Gated on mainboard size: the legacy auto-pick fallback is the
	// right thing for small / hand-edited decks (and the existing test
	// corpus relies on it) but WRONG for 60+ Constructed-shape decks
	// where stealing the first card into the commander slot misclassifies
	// the deck. Skip auto-pick when the mainboard has ≥ 60 cards AND no
	// explicit commander signal — let detectFormat surface FormatConstructed
	// with CommanderCards empty.
	mainboardQty := 0
	for _, le := range lines {
		mainboardQty += le.qty
	}
	if commanderName == "" && (mainboardQty < 60 || hasExplicitCommanderSignal) {
		for i, le := range lines {
			if c := buildCard(le.name, corpus, meta); c != nil {
				commanderName = c.DisplayName()
				lines[i].qty--
				c.Owner = -1
				commanderCard = c
				commanderTaken = true
				break
			}
		}
	}

	// lineStatusByNum records the resolution verdict per source line so
	// the post-loop CardLines update can stamp CardLine.Status without
	// re-running buildCard. Keyed by source line number (1-based).
	lineStatusByNum := map[int]LineStatus{}
	for i := range lines {
		le := &lines[i]
		if le.qty < 1 {
			continue
		}
		name := le.name
		// Pull out one copy of commander if the directive matched this entry.
		if !commanderTaken && commanderMatch(name) {
			c := buildCard(name, corpus, meta)
			if c != nil {
				commanderName = c.DisplayName()
				commanderCard = c
				commanderTaken = true
				le.qty--
			}
		}
		// Pull out one copy of partner if the directive matched this entry.
		if !partnerTaken && partnerMatch(name) {
			c := buildCard(name, corpus, meta)
			if c != nil {
				partnerCard = c
				partnerTaken = true
				le.qty--
			}
		}
		// If commander/partner extraction took the only copy, le.qty has
		// dropped to 0 — stamp Resolved (the commander resolved cleanly)
		// and skip the library probe. Pre-fix this branch was unreachable
		// because the original loop's `j := 0; j < le.qty` short-circuited
		// at qty=0; the new refactor pulled the probe out of the loop and
		// has to re-check.
		if le.qty < 1 {
			lineStatusByNum[le.lineNum] = LineStatusResolved
			continue
		}
		// First-copy resolution probes whether the name builds AT ALL and
		// captures the resolution path (Resolved / FallbackResolved /
		// Unresolved). Subsequent copies in the qty loop just re-call
		// buildCard for fresh Card pointers; status is identical for all
		// copies of the same name.
		probe, probeStatus := buildCardWithStatus(name, corpus, meta)
		lineStatusByNum[le.lineNum] = probeStatus
		if probe == nil {
			td.Unresolved = append(td.Unresolved, name)
			unresolvedDetails = append(unresolvedDetails, UnresolvedLine{
				LineNumber:  le.lineNum,
				Raw:         le.raw,
				Name:        name,
				Section:     le.section,
				Reason:      "name not found in meta (corpus + DFC face-match all missed)",
				Suggestions: meta.SuggestSimilarNames(name, 3),
			})
			continue
		}
		td.Library = append(td.Library, probe)
		for j := 1; j < le.qty; j++ {
			c := buildCard(name, corpus, meta)
			if c == nil {
				// Defensive — probe succeeded but a subsequent build
				// failed (shouldn't happen for a deterministic builder).
				continue
			}
			td.Library = append(td.Library, c)
		}
	}

	// Fallback lookups — directive set but card wasn't in the main list.
	if !commanderTaken && commanderName != "" {
		c := buildCard(commanderName, corpus, meta)
		if c != nil {
			commanderCard = c
			commanderTaken = true
		}
	}
	if !partnerTaken && partnerName != "" {
		c := buildCard(partnerName, corpus, meta)
		if c != nil {
			partnerCard = c
			partnerTaken = true
		}
	}

	if !commanderTaken {
		// Only error out when the deck SHIPPED an explicit commander
		// signal but the card couldn't be resolved — that's a real bug
		// the caller wants to know about. For 60-card Constructed-shape
		// decks with no commander signal at all (no COMMANDER: directive,
		// no Commander section header, no `// COMMANDER` / `*CMDR*` marker)
		// just leave CommanderCards empty and let detectFormat surface
		// FormatConstructed / FormatCasual. Pre-fix this errored out
		// on every non-Commander deck.
		if hasExplicitCommanderSignal || len(td.Library) < 60 {
			return nil, fmt.Errorf("deckparser: no commander found (directive=%q)", explicitCommander)
		}
	} else {
		// Final assembly — commander first, then partner. CommanderName
		// stays as the primary (commander) name for back-compat
		// single-commander consumers.
		td.CommanderName = commanderCard.DisplayName()
		td.CommanderCards = append(td.CommanderCards, commanderCard)
		if partnerCard != nil {
			td.CommanderCards = append(td.CommanderCards, partnerCard)
		}
	}
	if partnerName != "" && !partnerTaken {
		// Partner directive given but the card couldn't be resolved.
		// Surface via Unresolved so the caller can decide whether to drop
		// the deck.
		td.Unresolved = append(td.Unresolved, partnerName)
	}
	td.DetectedFormat = detectFormat(td, hasExplicitCommanderSignal)
	// Build the ParseReport — stamp per-line status on each CardLine
	// (mainboard via lineStatusByNum; commander/partner sections via
	// final commanderTaken/partnerTaken state) and roll up the counts.
	commanderResolved := commanderTaken
	partnerResolved := partnerName != "" && partnerTaken
	partnerExpected := partnerName != ""
	for i := range td.CardLines {
		cl := &td.CardLines[i]
		switch cl.Section {
		case "main":
			if st, ok := lineStatusByNum[cl.LineNumber]; ok {
				cl.Status = st
			} else {
				// Mainboard line not in lineStatusByNum means the loop
				// drained it (commanderMatch / partnerMatch took the
				// only copy, qty dropped to zero pre-probe). That copy
				// resolved cleanly into the commander slot.
				cl.Status = LineStatusResolved
			}
		case "commander":
			// Commander-section lines: status reflects whether final
			// commander resolution succeeded. For partner-pair decks
			// where both lines route to the commander section, the first
			// line corresponds to the primary commander and subsequent
			// lines to the partner — assume the first commander-section
			// line maps to commanderResolved, the second to partnerResolved.
			// Conservative: any commander-section line gets Resolved if
			// commanderTaken (the parser succeeded in resolving at least
			// the primary); Unresolved otherwise.
			switch {
			case commanderResolved && (!partnerExpected || partnerResolved):
				cl.Status = LineStatusResolved
			case commanderResolved:
				// Primary resolved but partner directive failed.
				cl.Status = LineStatusFallbackResolved
			default:
				cl.Status = LineStatusUnresolved
			}
		}
	}
	report := ParseReport{
		DroppedLines:      td.SideboardCount + td.SignatureSpellCount + otherDropCardLines,
		UnresolvedDetails: unresolvedDetails,
	}
	for _, cl := range td.CardLines {
		report.TotalLines += cl.Qty
		switch cl.Status {
		case LineStatusResolved:
			report.ResolvedLines += cl.Qty
		case LineStatusFallbackResolved:
			report.FallbackResolved += cl.Qty
		case LineStatusUnresolved:
			report.UnresolvedLines += cl.Qty
		}
	}
	report.TotalLines += report.DroppedLines
	td.ParseReport = report
	return td, nil
}

// PrintReport writes a human-readable parse coverage report to w.
// Surfaces the per-line resolution roll-up, format detection verdict,
// and the unresolved-detail list pointing at the broken source lines.
// Driven by hexdek-judge's --report-parse flag; safe to call from any
// caller that wants the textual report (UI build-coaching, CI deck-
// audit, etc.).
//
// Format:
//
//	==== Parse coverage report ====
//	Source:    <td.Path>
//	Format:    <DetectedFormat>
//	Total lines:       N
//	Resolved (clean):  X
//	Resolved (fallback): Y
//	Unresolved:        Z
//	Dropped (SB/SS):   D
//	Coverage:          PP.P% (Y/X resolved / unresolved)
//
//	Unresolved details:
//	  line N: <name> [<section>] — <reason>
//	    raw: <raw line>
//	  ...
func (td *TournamentDeck) PrintReport(w io.Writer) error {
	if td == nil {
		_, err := fmt.Fprintln(w, "==== Parse coverage report ==== (no deck)")
		return err
	}
	r := td.ParseReport
	pathLine := td.Path
	if pathLine == "" {
		pathLine = "(stream)"
	}
	fmt.Fprintln(w, "==== Parse coverage report ====")
	fmt.Fprintf(w, "Source:              %s\n", pathLine)
	fmt.Fprintf(w, "Format:              %s\n", td.DetectedFormat)
	if td.CommanderName != "" {
		fmt.Fprintf(w, "Commander:           %s\n", td.CommanderName)
	}
	fmt.Fprintf(w, "Total lines:         %d\n", r.TotalLines)
	fmt.Fprintf(w, "Resolved (clean):    %d\n", r.ResolvedLines)
	fmt.Fprintf(w, "Resolved (fallback): %d\n", r.FallbackResolved)
	fmt.Fprintf(w, "Unresolved:          %d\n", r.UnresolvedLines)
	fmt.Fprintf(w, "Dropped (SB/SS/etc): %d\n", r.DroppedLines)
	fmt.Fprintf(w, "Coverage:            %.1f%%\n", r.CoveragePercent())
	if len(r.UnresolvedDetails) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Unresolved details (%d):\n", len(r.UnresolvedDetails))
		for _, u := range r.UnresolvedDetails {
			fmt.Fprintf(w, "  line %d: %q [%s] — %s\n", u.LineNumber, u.Name, u.Section, u.Reason)
			if u.Raw != "" && u.Raw != u.Name {
				fmt.Fprintf(w, "    raw: %s\n", u.Raw)
			}
			for _, s := range u.Suggestions {
				fmt.Fprintf(w, "    %s\n", suggestionPhrase(s, u.Name))
			}
		}
	}
	return nil
}

// detectFormat is the deck shape → format heuristic. Pure structural
// classification; no card-legality lookups. Standard / Modern / Pioneer
// / Legacy / Vintage / Pauper all share the 60+ mainboard + ≤15
// sideboard + multiples-allowed shape and collapse to FormatConstructed
// — refining further requires a legality database that this package
// deliberately doesn't own. Decision tree (priority order):
//
//  1. Source hint contains "precon" / "preconstructed" → FormatPrecon
//     (shadows the rules-format; precons are usually Commander but
//     distinguishing literal-precon from user-built is high-signal).
//  2. Commander signal + ≥1 signature spell → FormatOathbreaker.
//  3. Commander signal + total cards in [95, 101] → FormatCommander.
//  4. Commander signal + total cards in [58, 61] → FormatBrawl.
//  5. Commander signal + count off-spec → FormatCasual (commander deck
//     with non-standard size).
//  6. No commander signal + total ≥ 58 + sideboard ≥ 5 → FormatConstructed.
//  7. No commander signal + total ≥ 60 → FormatConstructed (some users
//     don't paste a sideboard with the maindeck).
//  8. Anything else → FormatCasual.
//
// hasCommanderSignal is true iff the parser saw at least one explicit
// commander marker (COMMANDER:/PARTNER: directive, Commander[s] section
// header that ate ≥1 card, `// COMMANDER` directive comment, or `*CMDR*`
// inline marker). The legacy "auto-pick first resolvable card as
// commander" fallback does NOT count — without it a 60-card Constructed
// deck (where the legacy fallback would steal the first card into the
// commander slot) gets correctly classified as Constructed instead of
// Casual.
func detectFormat(td *TournamentDeck, hasCommanderSignal bool) DetectedFormat {
	if td == nil {
		return FormatUnknown
	}
	for _, h := range td.SourceHints {
		lh := strings.ToLower(h)
		if strings.Contains(lh, "precon") || strings.Contains(lh, "preconstructed") {
			return FormatPrecon
		}
	}
	cmdrCount := len(td.CommanderCards)
	total := cmdrCount + len(td.Library)
	if hasCommanderSignal && cmdrCount >= 1 {
		if td.SignatureSpellCount >= 1 {
			return FormatOathbreaker
		}
		switch {
		case total >= 95 && total <= 101:
			return FormatCommander
		case total >= 58 && total <= 61:
			return FormatBrawl
		default:
			return FormatCasual
		}
	}
	if total >= 58 && td.SideboardCount >= 5 {
		return FormatConstructed
	}
	if total >= 60 {
		return FormatConstructed
	}
	return FormatCasual
}

// buildCard returns a fresh *gameengine.Card populated with AST + types
// + base P/T + CMC hint. Returns nil if both corpus and meta miss; a
// bare-bones fallback is used if only one side has the card.
//
// Handles DFC / split / adventure face lookups: if `name` is a single
// face, we also try `name // ...` and `... // name` to catch both
// halves of modal double-faced cards where Scryfall stores the full
// "A // B" under a single entry.
// buildCardWithStatus is the report-instrumented wrapper around
// buildCard. Returns the resolution path taken: LineStatusResolved when
// meta.Get(name) is the direct hit, LineStatusFallbackResolved when
// the card came back via face-match / DFC-canonicalize / corpus-only,
// LineStatusUnresolved on a nil card. Used by ParseDeckReader so each
// CardLine carries an accurate Status without buildCard having to
// thread a status return value through every caller.
func buildCardWithStatus(name string, corpus *astload.Corpus, meta *MetaDB) (*gameengine.Card, LineStatus) {
	directHit := meta != nil && meta.Get(name) != nil
	c := buildCard(name, corpus, meta)
	if c == nil {
		return nil, LineStatusUnresolved
	}
	if directHit {
		return c, LineStatusResolved
	}
	return c, LineStatusFallbackResolved
}

func buildCard(name string, corpus *astload.Corpus, meta *MetaDB) *gameengine.Card {
	var ast *gameast.CardAST
	if corpus != nil {
		ast, _ = corpus.Get(name)
		if ast == nil {
			// Face-match: look up any card whose split/DFC name contains
			// `name` as a face. Meta holds the normalized-name index we
			// can scan for "//" entries.
			if meta != nil {
				for _, m := range meta.byName {
					if strings.Contains(m.Name, " // ") {
						for _, face := range strings.Split(m.Name, " // ") {
							if normalizeName(face) == normalizeName(name) {
								ast, _ = corpus.Get(m.Name)
								if ast != nil {
									name = m.Name
									break
								}
							}
						}
						if ast != nil {
							break
						}
					}
				}
			}
		}
	}
	var md *CardMeta
	if meta != nil {
		md = meta.Get(name)
		if md == nil {
			// DFC face-match on meta too.
			for _, m := range meta.byName {
				if strings.Contains(m.Name, " // ") {
					for _, face := range strings.Split(m.Name, " // ") {
						if normalizeName(face) == normalizeName(name) {
							md = m
							name = m.Name
							break
						}
					}
					if md != nil {
						break
					}
				}
			}
		}
	}
	if ast == nil && md == nil {
		return nil
	}
	c := &gameengine.Card{
		AST:   ast,
		Name:  name,
		Owner: -1, // runner sets per seat
	}
	if md != nil {
		c.Name = md.Name
		// Copy to avoid sharing the MetaDB's Types slice.
		if len(md.Types) > 0 {
			c.Types = append([]string(nil), md.Types...)
		}
		c.BasePower = md.Power
		c.BaseToughness = md.Toughness
		// Copy Colors for Runaway Steam-Kin / red-spell cast-triggers.
		if len(md.Colors) > 0 {
			c.Colors = append([]string(nil), md.Colors...)
		}
		c.CMC = md.CMC
		c.TypeLine = strings.ToLower(md.TypeLine)
		// MDFC back-face data.
		if md.BackFaceName != "" {
			c.BackFaceName = md.BackFaceName
			c.BackFaceCMC = md.BackFaceCMC
			c.BackFaceTypeLine = strings.ToLower(md.BackFaceTypeLine)
			if len(md.BackFaceTypes) > 0 {
				c.BackFaceTypes = append([]string(nil), md.BackFaceTypes...)
			}
		}
	}
	return c
}

// CloneLibrary returns a deep copy of a source library. Callers create
// a fresh library per game to avoid cross-goroutine mutation.
func CloneLibrary(src []*gameengine.Card) []*gameengine.Card {
	if len(src) == 0 {
		return nil
	}
	out := make([]*gameengine.Card, len(src))
	for i, c := range src {
		if c == nil {
			continue
		}
		cp := *c
		// Types slice should be copied so game-side mutations don't
		// cross-leak. Most games don't mutate Types but we pay the
		// tiny cost for safety.
		if len(c.Types) > 0 {
			cp.Types = append([]string(nil), c.Types...)
		}
		if len(c.BackFaceTypes) > 0 {
			cp.BackFaceTypes = append([]string(nil), c.BackFaceTypes...)
		}
		cp.CastingBackFace = false
		out[i] = &cp
	}
	return out
}

// CloneCards is the commander-variant clone.
func CloneCards(src []*gameengine.Card) []*gameengine.Card {
	return CloneLibrary(src)
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

var deckLineRE = regexp.MustCompile(`^\s*(\d+)\s*[xX]?\s+(.+?)\s*$`)
var commanderLineRE = regexp.MustCompile(`(?i)^\s*COMMANDER\s*:\s*(.+?)\s*$`)
var partnerLineRE = regexp.MustCompile(`(?i)^\s*PARTNER\s*:\s*(.+?)\s*$`)

// sectionHeaderRE matches section banners in any of the export-format
// shapes the parser sees in the wild:
//
//	Sideboard
//	Sideboard:
//	Sideboard (5)         ← Moxfield's native plaintext export
//	Sideboard (5):        ← hand-edited variant
//	Sideboard: (5)        ← Archidekt occasionally
//
// The trailing `(\d+)` count is the load-bearing addition vs the older
// label-only regex: Moxfield's most-used export mode emits headers with
// the parenthesized count, and a missed match here silently leaks every
// sideboard / companion / token card into the library (see
// section_count_r60_test.go).
var sectionHeaderRE = regexp.MustCompile(`(?i)^\s*(Sideboard|Maybeboard|Companion|Considering|Deck|Main\s*Deck|Mainboard|Commanders?|Tokens|Signature\s*Spells|Stickers|Attractions|Outside\s*the\s*Game|About)\s*:?\s*(?:\(\s*\d+\s*\))?\s*:?\s*$`)

// hashTagExtractRE matches the trailing `#tag1 #tag2 ...` block on a
// card line. Tighter than the cleanCardName-internal hashTagRE: each
// tag must start with `#` followed by a letter and then word chars
// (`-` allowed for hyphenated tags like `#turn-1-ramp`), so URL
// fragments (`https://...#anchor` — no real card line has a URL but
// defensive) and standalone `#` chars are left alone. Anchored to
// end-of-line. Captures the entire tag block in group 1 for
// per-token splitting in extractHashTags.
var hashTagExtractRE = regexp.MustCompile(`\s+(#[A-Za-z][\w\-]*(?:\s+#[A-Za-z][\w\-]*)*)\s*$`)

// extractHashTags pulls trailing `#tag1 #tag2 ...` annotations off the
// raw line and returns the stripped line + a slice of tag strings
// (without leading `#`, in source order). Drives CardLine.HashTags
// for round-trip preservation: pre-fix cleanCardName's hashTagRE
// silently dropped the tags so any tooling consuming deckbuilder
// intent ("#wincon", "#ramp", "#flex") had no way to recover them.
//
// No-op when no trailing hashtag block is present. Inner-line `#`
// chars (defensive — no real card name has them, but URLs in
// comments could) stay untouched because the regex anchors to `$`.
func extractHashTags(raw string) (string, []string) {
	m := hashTagExtractRE.FindStringSubmatchIndex(raw)
	if m == nil {
		return raw, nil
	}
	block := raw[m[2]:m[3]]
	var tags []string
	for _, tok := range strings.Fields(block) {
		if strings.HasPrefix(tok, "#") {
			tags = append(tags, strings.TrimPrefix(tok, "#"))
		}
	}
	return strings.TrimSpace(raw[:m[0]]), tags
}

// WriteText emits the deck as a Moxfield-format text decklist back to
// w, preserving Comment + HashTags from each CardLine so users can
// round-trip annotated decks (parse → modify → write → re-parse).
// Output format:
//
//	COMMANDER: <commander name>
//	[PARTNER: <partner name>]
//
//	<qty> <name> [// comment] [#tag1 #tag2 ...]
//	...
//
// Commander lines flow through the directive form (not the
// `Commander` section header form) because the directive resolves
// unambiguously regardless of meta-DB presence. Sections other than
// "commander" / "main" (legacy "drop" entries — there shouldn't be
// any in a freshly-parsed deck since drops bypass CardLines) are
// silently elided. Round-trip identity is verified by
// TestWriteText_RoundTripPreservesCommentsAndTags.
func (td *TournamentDeck) WriteText(w io.Writer) error {
	if td == nil {
		return nil
	}
	for i, c := range td.CommanderCards {
		var label string
		if i == 0 {
			label = "COMMANDER"
		} else {
			label = "PARTNER"
		}
		if _, err := fmt.Fprintf(w, "%s: %s\n", label, c.DisplayName()); err != nil {
			return err
		}
	}
	if len(td.CommanderCards) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	for _, cl := range td.CardLines {
		if cl.Section != "main" {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d %s", cl.Qty, cl.Name)
		if cl.Comment != "" {
			b.WriteString(" // ")
			b.WriteString(cl.Comment)
		}
		for _, tag := range cl.HashTags {
			b.WriteString(" #")
			b.WriteString(tag)
		}
		b.WriteByte('\n')
		if _, err := io.WriteString(w, b.String()); err != nil {
			return err
		}
	}
	return nil
}

// fuzzyLeadingXQtyRE matches the leading-x quantity form `x4 Card` /
// `X4 Card`. Captures (qty, name). Distinct from the canonical
// leading-digit `4 Card` form that deckLineRE already handles —
// fuzzy normalization only fires when the line lacks a leading digit.
var fuzzyLeadingXQtyRE = regexp.MustCompile(`^\s*[xX](\d+)\s+(.+?)\s*$`)

// fuzzyTrailingXQtyRE matches the trailing-x form `Card x4` /
// `Card X4`. Captures (name, qty). The `\s+` before `[xX]` requires
// whitespace separation so card names containing inner `x` chars
// (e.g. "Xanthic Statue", "Naya Hexproof") don't accidentally match;
// only an isolated `x<digits>` token at end-of-line qualifies.
var fuzzyTrailingXQtyRE = regexp.MustCompile(`^\s*(.+?)\s+[xX](\d+)\s*$`)

// fuzzyTrailingParenQtyRE matches the trailing parens-qty form
// `Card (4)`. Captures (name, qty). Anchored to end-of-line with
// digit-only parens content — `(M11)` (set code) and `(SET) 123`
// (printing tail) have non-digit content and won't match, preserving
// the existing set-parens strip path. Defensive against real Magic
// card names: no card contains `(<digits>)` literal in its name, so
// a positive match is unambiguously a quantity annotation.
var fuzzyTrailingParenQtyRE = regexp.MustCompile(`^\s*(.+?)\s+\((\d+)\)\s*$`)

// normalizeFuzzyQuantity converts alternate quantity syntaxes to the
// canonical leading `<qty> <name>` form that deckLineRE expects. The
// three supported alternate shapes:
//
//   - `x4 Lightning Bolt`        → `4 Lightning Bolt`  (leading x)
//   - `Lightning Bolt x4`        → `4 Lightning Bolt`  (trailing x)
//   - `Lightning Bolt (4)`       → `4 Lightning Bolt`  (trailing parens)
//
// No-op when the line already starts with a digit (`4 Card` /
// `4x Card` — deckLineRE already handles those). Each alternate is
// tried in order; the first match wins.
//
// Trailing-parens form must run BEFORE the set-parens strip in the
// caller (the `(4)` would otherwise be eaten as a "(SET) 123" tail
// and the quantity lost).
func normalizeFuzzyQuantity(raw string) string {
	trimmed := strings.TrimLeft(raw, " \t")
	if len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' {
		return raw
	}
	if m := fuzzyLeadingXQtyRE.FindStringSubmatch(raw); m != nil {
		return m[1] + " " + m[2]
	}
	if m := fuzzyTrailingXQtyRE.FindStringSubmatch(raw); m != nil {
		return m[2] + " " + m[1]
	}
	if m := fuzzyTrailingParenQtyRE.FindStringSubmatch(raw); m != nil {
		return m[2] + " " + m[1]
	}
	return raw
}

// tappedoutDirectiveRE matches Tappedout's `#!<section>` directive
// shape: `#!Commander`, `#!Mainboard`, `#!Sideboard`, `#!Maybeboard`,
// `#!Companion`. Routes the following lines to the named section the
// same way a bare-label section header would. Must be matched BEFORE
// the generic `#` comment drop or the directive is silently swallowed.
// Case-insensitive; tolerant of surrounding whitespace.
var tappedoutDirectiveRE = regexp.MustCompile(`(?i)^\s*#!\s*(Commander|Commanders|Mainboard|Maindeck|Main|Deck|Sideboard|Maybeboard|Companion|Signature\s*Spells|Tokens)\s*$`)

// markdownBoldHeaderRE matches Tappedout's markdown-style section
// header wrapping: `**Commanders (1):**` / `**Lands (38):**`. Captures
// the un-wrapped label (group 1) so sectionHeaderRE /
// typeCategoryHeaderRE can match the bare form.
var markdownBoldHeaderRE = regexp.MustCompile(`^\*\*\s*(.+?)\s*\*\*\s*$`)

// bulletPrefixRE matches a leading bullet character (`* `, `- `, `• `)
// that Tappedout's text export prepends to every card line. Strips
// only when the bullet is followed by whitespace — `*CMDR*` and
// similar inline markers are NOT bulleted (no trailing space) and
// stay intact for cmdrInlineMarkerRE to handle later.
var bulletPrefixRE = regexp.MustCompile(`^\s*[*\-\x{2022}]\s+`)

// priceTagRE matches a trailing per-card price annotation that some
// Tappedout / Aetherhub exports append: `$1.50`, `€1,50`, `£2.00`,
// `$0.50 USD`. Anchored to end-of-line with a leading whitespace
// requirement so card names containing `$` (none in real Magic, but
// defensive) aren't truncated mid-name. Currency symbols restricted
// to $ / € / £ — the three Tappedout actually emits.
var priceTagRE = regexp.MustCompile(`\s+[\$€£][0-9]+(?:[.,][0-9]+)?(?:\s+(?:USD|EUR|GBP))?\s*$`)

// mtgtop8MetaCommentRE matches MTGTop8 / MWS / Apprentice export
// preamble lines: `// NAME: ...`, `// CREATOR: ...`, `// FORMAT: ...`,
// `// DATE: ...`. These appear before any card content and carry
// useful provenance — captured into SourceHints. Distinct from
// cmdrHeaderCommentRE (which steals `// COMMANDER`) and from the
// generic `//` comment drop (which loses the information).
var mtgtop8MetaCommentRE = regexp.MustCompile(`(?i)^\s*//\s*(NAME|CREATOR|FORMAT|DATE|AUTHOR|DESCRIPTION|SOURCE|URL)\s*:\s*\S`)

// htmlTagRE matches any HTML tag like `<br>`, `</p>`, `<div class="x">`,
// or `<td>`. MTGGoldfish's "Save as HTML" export and various browser
// drag-drop pastes leak these inline with card lines. Pre-fix every
// HTML-bearing line landed in Unresolved because `Lightning Bolt<br>`
// doesn't match any meta entry. Stripped inline at the top of the
// per-line loop before any other tokenization runs. Conservative —
// only strips well-formed `<...>` tags; never touches `<` or `>`
// inside otherwise-valid card content (no real card name has them).
var htmlTagRE = regexp.MustCompile(`<[^>]*>`)

// deckboxTabExtraRE matches the second-and-beyond tab-separated fields
// that Deckbox's full-inventory CSV/TSV export appends after the card
// name (`4\tLightning Bolt\tM11\tNM\tEN\tEnglish\t...`). The first
// tab-separated field IS the name; everything from the second tab
// onward is metadata (edition, condition, language, foil flag, etc.)
// that pollutes meta lookups when it leaks into the name. Pre-fix
// `Lightning Bolt\tM11` failed to resolve. Stripped after deckLineRE
// has matched (so the qty-name tab boundary is preserved); never
// applied to the qty-name boundary itself, which `deckLineRE`'s
// `\s+` already handles.
var deckboxTabExtraRE = regexp.MustCompile(`\t.*$`)

// archidektCategoryRE matches Archidekt's per-line trailing
// `[Category]` or `[Category{modifier}]` annotation. Captures the
// category name in group 1 (sans modifier). Distinguished from
// generic `[SET]` set-code tags by the routing helper
// `isArchidektCategoryLabel`: any bracket content with a lowercase
// letter is a category word (Commander, Sideboard, Ramp, Wincons);
// all-uppercase is a set code and falls through to bracketTagRE.
// Anchored to end-of-line because Archidekt always emits the category
// last on the line (after any `(SET) NUM` printing tail).
var archidektCategoryRE = regexp.MustCompile(`\s*\[([A-Za-z][A-Za-z ]*?)(?:\{[^}]*\})?\]\s*$`)

// isArchidektCategoryLabel returns true when a bracket-captured label
// looks like an Archidekt category name (Commander / Sideboard / Ramp
// / user-defined like "Wincons") rather than a Magic set code (LEA /
// KHM / MM3). Heuristic: any lowercase letter implies a category
// word, since real set codes are always all-uppercase. All-uppercase
// content falls through to bracketTagRE's existing set-code strip.
func isArchidektCategoryLabel(label string) bool {
	for _, r := range label {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

// typeCategoryHeaderRE matches Moxfield's "Card View" copy-paste and
// Archidekt's "By Type" export sub-section headers: the type-line
// category names with optional trailing count. Distinct from
// `sectionHeaderRE` (which whitelists ALTERNATE-BOARD labels like
// Sideboard/Maybeboard/Companion); these are MAINBOARD subdivisions
// that the parser was treating as garbage card lines.
//
// Coverage of the canonical Magic permanent + spell types plus the
// occasional "Tribal" / "Other" buckets some exports use. Plurals are
// authoritative because both Moxfield and Archidekt emit plurals;
// singulars added for safety against hand-edited / TappedOut variants.
// The `(N)` count is optional so a bare `Lands` line (no count) also
// matches.
//
// Safety: no real Magic card is named exactly any of these labels
// (`Lands`, `Sorceries`, etc. would be invalid card names by Magic's
// naming convention) so a positive match is unambiguously a header,
// not a card line — and the parser already requires a leading qty for
// card lines anyway, so even a typo card-by-this-name wouldn't reach
// this regex (it has no digit prefix).
var typeCategoryHeaderRE = regexp.MustCompile(`(?i)^\s*(Creatures?|Planeswalkers?|Battles?|Sorceries|Sorcery|Instants?|Artifacts?|Enchantments?|Lands?|Tribal|Other|Spells?)\s*:?\s*(?:\(\s*\d+\s*\))?\s*:?\s*$`)

// mtgoMetadataRE matches MTGO / Tournament-Ready export metadata header
// lines: `Deck name: <name>`, `Created by: <author>`, `Format: <format>`,
// `Layout: <layout>`, `Description: <blurb>`, `Tags: <list>`, `Author:
// <name>`. MTGO's text export emits these before the card list; pre-fix
// they fell through the fallback as qty=1 cards named e.g. `Deck name:
// My MTGO Deck` and landed in the Unresolved report. Real Magic cards
// never contain a colon, so the generic "leading alphanumeric word(s)
// followed by colon followed by content" pattern is safe — checked
// AFTER COMMANDER: / PARTNER: / SB: directives (which match earlier and
// short-circuit) so it doesn't shadow them.
var mtgoMetadataRE = regexp.MustCompile(`(?i)^\s*(deck\s*name|created\s*by|format|layout|author|description|tags?|source|owner|notes?)\s*:\s*\S`)

// inlineCommentRE splits a card line on the first `//` token (preceded
// by whitespace), capturing whatever follows as the line's inline
// comment. Deckbuilders use this for intent notes: `1 Sol Ring // mvp`,
// `1 Cyclonic Rift // wincon — never cut`. Pre-fix, every such line
// landed in Unresolved because cleanCardName had no handler for the
// `//` token and `Sol Ring // mvp` doesn't match the meta. Whole-line
// `//` comments (line starting with `//`) are still dropped earlier in
// the loop — this only catches the inline form.
var inlineCommentRE = regexp.MustCompile(`\s+//\s*(.*)$`)

// cmdrInlineMarkerRE matches an inline commander tag on a card line:
// `1 Sigarda, Host of Herons *CMDR*` / `*Commander*` / `*COMMANDER*`.
// Sibling to the foilMarkerRE strip but with semantic meaning: when
// matched, the line's card is promoted to commander instead of mainboard.
// The marker is peeled before cleanCardName runs so it doesn't leak
// into the resolved card name.
var cmdrInlineMarkerRE = regexp.MustCompile(`(?i)\s*\*(?:CMDR|Commander)\*\s*`)

// cmdrHeaderCommentRE matches a `// COMMANDER` / `// CMDR` directive-
// comment that flags the NEXT card line as commander. Distinct from the
// generic whole-line `//` comment drop because we need to peek at this
// before the comment is silently swallowed at the top of the parse loop.
// Mirrors the Archidekt category-marker convention where a comment-style
// header annotates the role of the following card.
var cmdrHeaderCommentRE = regexp.MustCompile(`(?i)^\s*//\s*(?:COMMANDER|CMDR)\s*$`)

// sbPrefixRE matches MTGA / Aetherhub sideboard line prefix: `SB: 1 Card`.
// Pre-fix, every `SB:` line dropped into the fallback "qty=1, name=<raw>"
// path — most became Unresolved noise, but the surface bug is that an
// `SB:` line whose suffix happened to match a known card (e.g.
// `SB: 1 Sol Ring`) silently duplicated the card into the mainboard.
// Aetherhub's text export uses this form interleaved with mainboard
// lines instead of a Sideboard section header.
var sbPrefixRE = regexp.MustCompile(`(?i)^\s*SB\s*:\s*`)

// trailing-suffix patterns mirrored from internal/moxfield/textlist.go so
// the gauntlet-side parser strips the same Moxfield / Archidekt / TappedOut
// noise the import-side already handles.
//
// foilMarkerRE matches `*F*` / `*Etched*` / `*F-Etched*` — the etched-foil
// variant carries a hyphen, which the original `[A-Za-z]+` class rejected.
// The leadBracketTagRE pair handles the MTGO/Deckstats format where the set
// code appears between the quantity and the name (`4 [LEA] Sol Ring`); the
// original bracketTagRE only matched trailing brackets, so leading set
// codes survived through to the meta lookup and silently dropped the card
// into Unresolved — destroying the entire library for any deck pasted from
// MTGO. That was the worst gap surfaced by the audit.
// emojiTrimRE peels leading-or-trailing runs of emoji + whitespace from
// a card name. Moxfield deck tags sometimes carry pictographic markers
// like 🔥 / ⚔️ / 💀 as a visual annotation (similar in spirit to the
// `*F*` foil marker but without the asterisk wrapping). Pre-fix, every
// such line landed in Unresolved because the emoji bled into the meta
// lookup name (e.g. `1 Sol Ring 🔥` resolved as `Sol Ring 🔥`, miss).
//
// Strip is conservative: leading + trailing runs only, never interior
// characters. No real Magic card name contains emoji, but card names
// DO contain non-ASCII letters (Lim-Dûl's Vault, Jötun Grunt) so the
// strip targets symbol/format categories only:
//
//   - \p{So} — Symbol, Other (most pictographic emoji)
//   - \p{Sk} — Symbol, Modifier (skin-tone modifiers)
//   - \p{Cf} — Format (ZWJ, BOM, etc.)
//   - U+FE00-U+FE0F — variation selectors (the second codepoint in
//     two-codepoint emoji like ⚔️ = U+2694 + U+FE0F). These are
//     categorized as \p{Mn} (Mark, Nonspacing) which would over-match
//     combining accents on letter chars; the explicit hex range
//     targets ONLY the VS block.
//   - U+200D — Zero-Width Joiner used in emoji sequences like
//     👨‍👩‍👧 (man + ZWJ + woman + ZWJ + girl). Already covered by
//     \p{Cf} but listed for clarity.
//
// Letter-category chars (\p{L}) are intentionally left alone, so
// Lim-Dûl / Jötun / Æther / Séance survive.
var emojiTrimRE = regexp.MustCompile(`(?:^[\s\p{So}\p{Sk}\p{Cf}\x{FE00}-\x{FE0F}\x{200D}]+|[\s\p{So}\p{Sk}\p{Cf}\x{FE00}-\x{FE0F}\x{200D}]+$)`)

var (
	foilMarkerRE = regexp.MustCompile(`\s*\*[A-Za-z][A-Za-z\-]*\*\s*$`)
	// bracketTagRE peels a trailing `[Set]` tag, optionally followed by a
	// TCGplayer-style collector token (`[M11] 146`, `[Modern Masters
	// 2017] 36`, `[Commander 2014] 263`, `[Zendikar] ★114`). The original
	// `\s*\[[^\]]+\]\s*$` only matched a bare bracket at end-of-line, so
	// TCGplayer's documented mass-entry format `{qty} {name} [{set}]
	// {collector}` left the collector tail in the name → resolver
	// dropped the card into Unresolved despite the meta lookup being
	// trivially possible. The `\S+` token form (vs `\d+\S*`) covers
	// promo prefixes like ★ and ⊕ that some collector numbers use.
	// Safe because no real Magic card name contains `[...]`, so a
	// post-bracket token can't be card-name content.
	bracketTagRE     = regexp.MustCompile(`\s*\[[^\]]+\](?:\s+\S+)?\s*$`)
	leadBracketTagRE = regexp.MustCompile(`^\[[^\]]+\]\s*`)
	hashTagRE        = regexp.MustCompile(`\s+#\S+(?:\s+#\S+)*\s*$`)
)

// cleanCardName trims a card name of Moxfield-style decoration that real
// exports include after the card name: foil markers like `*F*` / `*E*`,
// bracket category tags like `[Burn]`, and hash tags like `#wincon #combo`.
// The "(SET) 123" set/collector tail is already stripped by the caller
// before this runs; this handles the suffixes that survive when no set
// parens are emitted (Archidekt, TappedOut, custom exports).
func cleanCardName(name string) string {
	s := strings.TrimSpace(name)
	// Repeat-strip because a card may carry several trailing decorations
	// (e.g. `Sol Ring *F* #ramp`); each pass peels one off. The leading
	// bracket strip runs in the same loop so a line like
	// `[LEA] Sol Ring *F*` peels both ends.
	for i := 0; i < 4; i++ {
		before := s
		s = leadBracketTagRE.ReplaceAllString(s, "")
		s = foilMarkerRE.ReplaceAllString(s, "")
		s = bracketTagRE.ReplaceAllString(s, "")
		s = hashTagRE.ReplaceAllString(s, "")
		s = emojiTrimRE.ReplaceAllString(s, "")
		s = strings.TrimSpace(s)
		if s == before {
			break
		}
	}
	return s
}

// singleSlashDFCRE matches a single ` / ` separator (with surrounding
// whitespace) when it is NOT part of an existing ` // ` double-slash
// pair. Used by normalizeDFCSeparator to detect the Aetherhub /
// TappedOut / legacy-Moxfield DFC export shape `Eirdu, Carrier of Dawn
// / Isilu, Carrier of Twilight` and canonicalize to the ` // ` form
// that meta lookups expect. No real Magic card name contains a single
// `/`, so a positive match is unambiguously a DFC face separator.
var singleSlashDFCRE = regexp.MustCompile(`\s+/\s+`)

// normalizeDFCSeparator canonicalizes single-slash DFC face separators
// (` / `) to the meta-canonical double-slash (` // `) form when the
// substituted name resolves in meta. Handles the Aetherhub /
// TappedOut export quirk where DFC / split / adventure / meld cards
// are rendered with a single slash instead of Moxfield's canonical
// ` // ` — e.g. `Eirdu, Carrier of Dawn / Isilu, Carrier of Twilight`
// (Edge of Eternities Commander) vs the meta entry
// `Eirdu, Carrier of Dawn // Isilu, Carrier of Twilight`. Without
// canonicalization the buildCard face-match would miss (it scans for
// ` // ` entries and tests each face against the input string, but
// the input here contains ` / ` so neither half ever matches a single
// face name on its own).
//
// Sibling to PR #785's ` // ` vs ` // comment` disambiguation: that
// fix runs only when inlineCommentRE (double-slash) matches; this one
// runs only when a single ` / ` is present AND ` // ` is NOT, so the
// two paths never overlap.
//
// Returns the input unchanged when: (1) name already contains ` // `
// (canonical form, no work needed); (2) meta is nil (no probe
// possible — caller's face-match in buildCard is the only remaining
// fallback); (3) the substituted ` // ` form misses meta (input wasn't
// a DFC after all — leave the original alone rather than mangling).
func normalizeDFCSeparator(name string, meta *MetaDB) string {
	if name == "" || meta == nil {
		return name
	}
	if strings.Contains(name, " // ") {
		return name
	}
	if !singleSlashDFCRE.MatchString(name) {
		return name
	}
	candidate := singleSlashDFCRE.ReplaceAllString(name, " // ")
	if meta.Get(candidate) != nil {
		return candidate
	}
	return name
}

// parseTypes splits a Scryfall type_line into the engine's lower-case
// type tokens. "Legendary Creature — Human Ninja" becomes
// ["legendary", "creature", "human", "ninja"].
//
// We split on the em-dash or hyphen the same way moxfield.splitTypeLine
// does; both halves are lowercased and whitespace-trimmed. Any side can
// be empty.
// parseMDFCManaCost converts a Scryfall mana cost string like "{W}{U}{B}{R}{G}"
// or "{3}{U}{U}" into a total CMC integer. Each {W}/{U}/{B}/{R}/{G}/{C} = 1,
// {N} = N, {X} = 0, hybrid {W/U} = 1.
func parseMDFCManaCost(cost string) int {
	cmc := 0
	i := 0
	for i < len(cost) {
		if cost[i] == '{' {
			j := strings.IndexByte(cost[i:], '}')
			if j < 0 {
				break
			}
			sym := cost[i+1 : i+j]
			switch {
			case sym == "X":
				// X contributes 0 to CMC
			case len(sym) == 1 && sym[0] >= '0' && sym[0] <= '9':
				cmc += int(sym[0] - '0')
			case len(sym) == 2 && sym[0] >= '1' && sym[1] >= '0' && sym[0] <= '9' && sym[1] <= '9':
				cmc += int(sym[0]-'0')*10 + int(sym[1]-'0')
			case strings.Contains(sym, "/"):
				cmc++ // hybrid symbols count as 1 CMC
			default:
				cmc++ // single color symbol
			}
			i += j + 1
		} else {
			i++
		}
	}
	return cmc
}

func parseTypes(typeLine string) []string {
	if typeLine == "" {
		return nil
	}
	normalized := strings.ReplaceAll(typeLine, "—", "-")
	out := []string{}
	for _, f := range strings.Fields(normalized) {
		f = strings.TrimSpace(f)
		if f == "" || f == "-" {
			continue
		}
		out = append(out, strings.ToLower(f))
	}
	return out
}

// NormalizeName accent-folds, lowercases, and collapses whitespace in a
// card name. Exported (R60 Phase 2C consolidation) so other internal
// packages — specifically `internal/deckid`, which already imports
// `deckparser` — can share the canonical implementation rather than
// keeping a verbatim copy. Mirrors astload.normalizeName; unit tests
// exercise both.
func NormalizeName(name string) string { return normalizeName(name) }

// CleanCardName strips Moxfield / Archidekt / MTGO printing decoration
// (set codes, collector numbers, foil markers, hash tags, bracket tags)
// from a card name. Exported so content-addressing layers like
// internal/deckid can defensively re-clean Card.Name values that bypass
// the parser path (DB rows, manual construction, external imports) —
// otherwise "Forest (THB) 270" and "Forest" hash to different deck IDs.
func CleanCardName(name string) string {
	s := strings.TrimSpace(name)
	if idx := strings.Index(s, "("); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	return cleanCardName(s)
}

func normalizeName(name string) string {
	out := make([]rune, 0, len(name))
	prevSpace := false
	for _, r := range name {
		r = foldAccent(r)
		if unicode.IsUpper(r) {
			r = unicode.ToLower(r)
		}
		if unicode.IsSpace(r) {
			if prevSpace || len(out) == 0 {
				continue
			}
			out = append(out, ' ')
			prevSpace = true
			continue
		}
		prevSpace = false
		out = append(out, r)
	}
	if n := len(out); n > 0 && out[n-1] == ' ' {
		out = out[:n-1]
	}
	return string(out)
}

func foldAccent(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å', 'ā',
		'Á', 'À', 'Â', 'Ä', 'Ã', 'Å', 'Ā':
		return 'a'
	case 'ç', 'Ç':
		return 'c'
	case 'é', 'è', 'ê', 'ë', 'ē',
		'É', 'È', 'Ê', 'Ë', 'Ē':
		return 'e'
	case 'í', 'ì', 'î', 'ï', 'ī',
		'Í', 'Ì', 'Î', 'Ï', 'Ī':
		return 'i'
	case 'ñ', 'Ñ':
		return 'n'
	case 'ó', 'ò', 'ô', 'ö', 'õ', 'ø', 'ō',
		'Ó', 'Ò', 'Ô', 'Ö', 'Õ', 'Ø', 'Ō':
		return 'o'
	case 'ú', 'ù', 'û', 'ü', 'ū',
		'Ú', 'Ù', 'Û', 'Ü', 'Ū':
		return 'u'
	case 'ý', 'ÿ', 'Ý', 'Ÿ':
		return 'y'
	case 'ß':
		return 's'
	case 'æ', 'Æ':
		return 'e'
	}
	return r
}

// ListDeckFiles returns absolute paths to every `*.txt` decklist in
// dir, sorted alphabetically. Callers typically pass this straight
// into parallel ParseDeckFile invocations.
func ListDeckFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("deckparser: read dir %s: %w", dir, err)
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out, nil
}
