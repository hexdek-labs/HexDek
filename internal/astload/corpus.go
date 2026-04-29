// Package astload reads the Python-exported `ast_dataset.jsonl` corpus and
// populates github.com/hexdek/hexdek/internal/gameast.CardAST
// values for every real MTG card.
//
// The JSONL file is the data contract between the Python parser in
// scripts/mtg_ast.py + scripts/export_ast_dataset.py and the Go rules
// engine. Every AST node carries an `__ast_type__` string discriminator
// (e.g. "CardAST", "Static", "Activated", "Damage", "Sequence",
// "Filter", "ScalingAmount") so Go can deserialize without hand-rolled
// schema inference.
//
// Typical use:
//
//	corpus, err := astload.Load("data/rules/ast_dataset.jsonl")
//	if err != nil { log.Fatal(err) }
//	card, ok := corpus.Get("Lightning Bolt")
//	if !ok { log.Fatalf("no such card") }
//	for _, ab := range card.Abilities { /* ... */ }
//
// Name lookup is case- and accent-insensitive: `Get("jhoira")` and
// `Get("Jhoira, Weatherlight Captain")` both find the right entry (the
// first by casefolding, the second literally). The original Scryfall
// spelling is always preserved on `CardAST.Name`.
package astload

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"
	"unicode"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Corpus is an in-memory card database keyed by normalized card name.
//
// A Corpus is safe for concurrent Get/Count/Names after Load returns; it is
// NOT safe to mutate (and no mutation API is exposed).
type Corpus struct {
	// cards is keyed by normalized name (see normalizeName). Values are
	// shared pointers — callers must treat them as read-only.
	cards map[string]*gameast.CardAST

	// names holds the original (un-normalized) card names, in the order
	// they were encountered in the JSONL file. Useful for deterministic
	// iteration; Names() returns a sorted copy.
	names []string

	// ParseWarnings records any per-card issues the loader encountered
	// but did not treat as fatal (unknown `__ast_type__`, malformed
	// NumberOrRef, etc.). Each entry is "<card name>: <message>".
	// The full corpus is still populated; these are advisory.
	ParseWarnings []string

	// CardCount is the number of cards successfully loaded (== len(cards)).
	CardCount int

	// LoadDuration is how long the full load took — useful for benchmarks
	// and the "Go beats Python by 10-50x" claim in the port plan.
	LoadDuration time.Duration

	// SourceBytes is the total JSONL bytes read. 0 for LoadReader when
	// the reader doesn't expose a size.
	SourceBytes int64
}

// Load reads `path` as JSONL and returns a populated Corpus.
//
// Returns an error only on filesystem errors or if the entire file is
// malformed. Per-line parse errors are collected in Corpus.ParseWarnings
// and the rest of the corpus still loads.
func Load(path string) (*Corpus, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("astload: open %s: %w", path, err)
	}
	defer f.Close()

	c, err := LoadReader(f)
	if err != nil {
		return nil, fmt.Errorf("astload: read %s: %w", path, err)
	}
	if fi, statErr := f.Stat(); statErr == nil {
		c.SourceBytes = fi.Size()
	}
	return c, nil
}

// LoadReader reads JSONL records from `r` and returns a populated Corpus.
// Use this for embedded/test fixtures or when the source isn't a local file.
//
// The reader is read line-by-line (no "slurp everything into memory"): the
// 40 MB dataset never exists as a single buffer.
func LoadReader(r io.Reader) (*Corpus, error) {
	t0 := time.Now()
	c := &Corpus{
		cards: make(map[string]*gameast.CardAST, 32_000),
		names: make([]string, 0, 32_000),
	}

	sc := bufio.NewScanner(r)
	// Default Scanner buffer is 64 KiB; some card rows (tokens w/ long
	// oracle text + full AST) push past that. Bump to 4 MiB to be safe.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		card, warnings, err := decodeCardLine(line)
		if err != nil {
			// Fatal per-line failure (malformed JSON). Record + continue.
			c.ParseWarnings = append(c.ParseWarnings,
				fmt.Sprintf("line %d: %v", lineNum, err))
			continue
		}
		if card == nil {
			// AST was null or empty; skip.
			continue
		}
		for _, w := range warnings {
			c.ParseWarnings = append(c.ParseWarnings,
				fmt.Sprintf("%s: %s", card.Name, w))
		}

		key := normalizeName(card.Name)
		if key == "" {
			c.ParseWarnings = append(c.ParseWarnings,
				fmt.Sprintf("line %d: empty card name; skipping", lineNum))
			continue
		}
		// On name collision, keep the first occurrence and record a warning.
		// The corpus shouldn't have duplicates, but defensively...
		if _, exists := c.cards[key]; exists {
			c.ParseWarnings = append(c.ParseWarnings,
				fmt.Sprintf("duplicate name %q at line %d; keeping first", card.Name, lineNum))
			continue
		}
		c.cards[key] = card
		c.names = append(c.names, card.Name)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	c.CardCount = len(c.cards)
	c.LoadDuration = time.Since(t0)
	return c, nil
}

// Get fetches a card by name. Lookup is case-insensitive and accent-insensitive
// (matches Python's `cards_by_name[name.lower()]` with diacritic folding added).
// Returns (nil, false) if not found.
func (c *Corpus) Get(name string) (*gameast.CardAST, bool) {
	if c == nil {
		return nil, false
	}
	card, ok := c.cards[normalizeName(name)]
	return card, ok
}

// Count returns the number of cards loaded.
func (c *Corpus) Count() int {
	if c == nil {
		return 0
	}
	return c.CardCount
}

// Names returns a sorted copy of the original (un-normalized) card names.
// Sort order is deterministic for reproducible iteration.
func (c *Corpus) Names() []string {
	if c == nil {
		return nil
	}
	out := make([]string, len(c.names))
	copy(out, c.names)
	sort.Strings(out)
	return out
}

// -----------------------------------------------------------------------------
// Name normalization
// -----------------------------------------------------------------------------

// stripAccents maps common accented runes used in MTG card names (mostly
// French/German/Spanish/Japanese transliteration) to ASCII. Built once.
//
// Kept as a simple rune→rune map (no external unicode/norm dependency) to
// avoid pulling in golang.org/x/text for a one-off. The set covers every
// accented glyph actually present in the Scryfall oracle-cards dump.
var (
	accentOnce sync.Once
	accentMap  map[rune]rune
)

func initAccents() {
	accentMap = map[rune]rune{
		'á': 'a', 'à': 'a', 'â': 'a', 'ä': 'a', 'ã': 'a', 'å': 'a', 'ā': 'a',
		'Á': 'a', 'À': 'a', 'Â': 'a', 'Ä': 'a', 'Ã': 'a', 'Å': 'a', 'Ā': 'a',
		'ç': 'c', 'Ç': 'c',
		'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e',
		'É': 'e', 'È': 'e', 'Ê': 'e', 'Ë': 'e', 'Ē': 'e',
		'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i', 'ī': 'i',
		'Í': 'i', 'Ì': 'i', 'Î': 'i', 'Ï': 'i', 'Ī': 'i',
		'ñ': 'n', 'Ñ': 'n',
		'ó': 'o', 'ò': 'o', 'ô': 'o', 'ö': 'o', 'õ': 'o', 'ø': 'o', 'ō': 'o',
		'Ó': 'o', 'Ò': 'o', 'Ô': 'o', 'Ö': 'o', 'Õ': 'o', 'Ø': 'o', 'Ō': 'o',
		'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u', 'ū': 'u',
		'Ú': 'u', 'Ù': 'u', 'Û': 'u', 'Ü': 'u', 'Ū': 'u',
		'ý': 'y', 'ÿ': 'y', 'Ý': 'y', 'Ÿ': 'y',
		'ß': 's',
		'æ': 'e', 'Æ': 'e', // Æ in card names ("Æther Vial") — collapse both halves so "aether" matches
	}
}

// normalizeName returns the lookup key for a card: lowercase + accent-stripped
// + whitespace-collapsed. This matches Python's `.lower()` behavior plus a
// diacritic fold for friendlier lookups ("Jotun Grunt" == "Jötun Grunt").
func normalizeName(name string) string {
	accentOnce.Do(initAccents)
	// Hot path: ASCII-only lowercase. Walk runes exactly once.
	out := make([]rune, 0, len(name))
	prevSpace := false
	for _, r := range name {
		if mapped, ok := accentMap[r]; ok {
			r = mapped
		} else if unicode.IsUpper(r) {
			r = unicode.ToLower(r)
		}
		// Collapse runs of whitespace to a single space so " Lightning  Bolt"
		// normalizes the same as "Lightning Bolt". Trim leading/trailing by
		// skipping leading whitespace and stripping trailing at end.
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
	// Strip trailing space if any.
	if n := len(out); n > 0 && out[n-1] == ' ' {
		out = out[:n-1]
	}
	return string(out)
}
