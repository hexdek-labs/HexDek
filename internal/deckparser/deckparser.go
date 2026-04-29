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
	Power     string `json:"power"`
	Toughness string `json:"toughness"`
	Loyalty   string `json:"loyalty"`
	Defense   string `json:"defense"`
}

// oracleCard mirrors the top-level Scryfall oracle-cards.json row.
type oracleCard struct {
	Name      string       `json:"name"`
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
	merged := 0
	for dec.More() {
		var e oracleCard
		if err := dec.Decode(&e); err != nil {
			continue
		}
		if e.Name == "" {
			continue
		}
		pw, tg := pickOraclePT(&e)
		if pw == 0 && tg == 0 {
			continue
		}
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
		if cm.Power == 0 {
			cm.Power = pw
		}
		if cm.Toughness == 0 {
			cm.Toughness = tg
		}
		merged++
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
		qty  int
		name string
	}
	var explicitCommander string
	var explicitPartner string
	var lines []lineEntry

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024), 1024*1024)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
			continue
		}
		// COMMANDER: <name>
		if m := commanderLineRE.FindStringSubmatch(raw); m != nil {
			explicitCommander = strings.TrimSpace(m[1])
			continue
		}
		// PARTNER: <name>
		if m := partnerLineRE.FindStringSubmatch(raw); m != nil {
			explicitPartner = strings.TrimSpace(m[1])
			continue
		}
		// Strip "(SET) 123" suffix.
		if idx := strings.Index(raw, "("); idx > 0 {
			raw = strings.TrimSpace(raw[:idx])
		}
		m := deckLineRE.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		qty, _ := strconv.Atoi(m[1])
		if qty < 1 {
			continue
		}
		name := strings.TrimSpace(m[2])
		if name == "" {
			continue
		}
		lines = append(lines, lineEntry{qty, name})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("deckparser: scan: %w", err)
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

// parseTypes splits a Scryfall type_line into the engine's lower-case
// type tokens. "Legendary Creature — Human Ninja" becomes
// ["legendary", "creature", "human", "ninja"].
//
// We split on the em-dash or hyphen the same way moxfield.splitTypeLine
// does; both halves are lowercased and whitespace-trimmed. Any side can
// be empty.
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

// normalizeName mirrors astload.normalizeName (accent-folded + lowercase
// + whitespace-collapsed). Keep in sync; unit tests exercise both.
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
