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
}

// CardLine is the per-source-line view of a parsed deck. Section is one
// of "main", "commander". Comment is everything after a `//` token on
// the same line (TrimSpace'd, empty if no inline comment).
type CardLine struct {
	Qty     int
	Name    string
	Comment string
	Section string
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
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("deckparser: open %s: %w", path, err)
	}
	defer f.Close()
	deck, err := ParseDeckReader(f, corpus, meta)
	if err != nil {
		return nil, err
	}
	deck.Path = path
	return deck, nil
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

	// pendingCommanderHeader: set when the previous non-blank line was a
	// `// COMMANDER` directive-comment. The NEXT card line consumes the
	// flag and routes its card to commander slots instead of mainboard.
	// Mirrors the Archidekt category-marker convention.
	pendingCommanderHeader := false

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024), 1024*1024)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		// `// COMMANDER` / `// CMDR` directive-comment: flag the NEXT card
		// line as commander. Must run before the generic `//` drop below
		// or the directive would be silently swallowed.
		if cmdrHeaderCommentRE.MatchString(raw) {
			pendingCommanderHeader = true
			continue
		}
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
			continue
		}
		// COMMANDER: <name> — Moxfield's directive-style export uses two
		// COMMANDER: lines for partners. Distinct from the section-header
		// path below.
		if m := commanderLineRE.FindStringSubmatch(raw); m != nil {
			name := cleanCardName(strings.TrimSpace(m[1]))
			if explicitCommander == "" {
				explicitCommander = name
			} else if explicitPartner == "" {
				explicitPartner = name
			}
			continue
		}
		// PARTNER: <name>
		if m := partnerLineRE.FindStringSubmatch(raw); m != nil {
			explicitPartner = cleanCardName(strings.TrimSpace(m[1]))
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
			case label == "deck" || label == "mainboard" || strings.HasPrefix(label, "main"):
				section = "main"
			case sectionDrops[label]:
				section = "drop"
			default:
				section = "drop"
			}
			continue
		}
		if section == "drop" {
			continue
		}
		// MTGA / Aetherhub sideboard line prefix. These appear interleaved
		// with mainboard lines instead of under a Sideboard header, so the
		// section-based drop above doesn't catch them. Drop the line; the
		// rest of the file may still be mainboard.
		if sbPrefixRE.MatchString(raw) {
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
		var inlineComment string
		if cm := inlineCommentRE.FindStringSubmatchIndex(raw); cm != nil {
			inlineComment = strings.TrimSpace(raw[cm[2]:cm[3]])
			raw = strings.TrimSpace(raw[:cm[0]])
		}
		// Inline `*CMDR*` / `*Commander*` marker: peel + flag this line
		// as commander. Sibling to the foilMarkerRE strip but with
		// semantic meaning. Captured here so the marker doesn't leak
		// into cleanCardName's resolved name.
		lineIsCommander := false
		if cmdrInlineMarkerRE.MatchString(raw) {
			raw = strings.TrimSpace(cmdrInlineMarkerRE.ReplaceAllString(raw, " "))
			lineIsCommander = true
		}
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
		} else {
			qty = 1
			name = raw
		}
		name = cleanCardName(name)
		if name == "" {
			continue
		}
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
			})
			continue
		}
		lines = append(lines, lineEntry{qty: qty, name: name, comment: inlineComment, section: "main"})
		td.CardLines = append(td.CardLines, CardLine{
			Qty: qty, Name: name, Comment: inlineComment, Section: "main",
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
	if commanderName == "" {
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
		for j := 0; j < le.qty; j++ {
			c := buildCard(name, corpus, meta)
			if c == nil {
				if j == 0 {
					td.Unresolved = append(td.Unresolved, name)
				}
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
		return nil, fmt.Errorf("deckparser: no commander found (directive=%q)", explicitCommander)
	}
	// Final assembly — commander first, then partner. CommanderName stays
	// as the primary (commander) name for back-compat single-commander
	// consumers.
	td.CommanderName = commanderCard.DisplayName()
	td.CommanderCards = append(td.CommanderCards, commanderCard)
	if partnerCard != nil {
		td.CommanderCards = append(td.CommanderCards, partnerCard)
	}
	if partnerName != "" && !partnerTaken {
		// Partner directive given but the card couldn't be resolved.
		// Surface via Unresolved so the caller can decide whether to drop
		// the deck.
		td.Unresolved = append(td.Unresolved, partnerName)
	}
	return td, nil
}

// buildCard returns a fresh *gameengine.Card populated with AST + types
// + base P/T + CMC hint. Returns nil if both corpus and meta miss; a
// bare-bones fallback is used if only one side has the card.
//
// Handles DFC / split / adventure face lookups: if `name` is a single
// face, we also try `name // ...` and `... // name` to catch both
// halves of modal double-faced cards where Scryfall stores the full
// "A // B" under a single entry.
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
