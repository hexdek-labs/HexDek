package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CR §202.2 mana cost format legality probe.
//
// hexdek-judge's interactive REPL is fine for hand-crafted board states, but
// the engine has no batch mode to spot-check a corpus or a deck for
// rules-correctness. This file adds the first such probe: a CR §202.2
// scan that walks the Scryfall oracle corpus, parses each card's printed
// `mana_cost` field, and flags any cost containing a mana symbol that
// doesn't conform to the Comprehensive Rules grammar.
//
// CR §202.2 mana symbols recognized as legal (Constructed-legal subset —
// silver-border / Un-set half-pips like {HW} are out of scope and the
// surrounding `is_real_card` filter in cmd/parser-coverage already drops
// those cards from the audit population):
//
//   - Generic numeric: {0}, {1}, ..., {20}   (CR §107.4 caps printed
//     numerals — Draco is the canonical {16}, Gleemax is {1,000,000}
//     but that's a silver-border outlier the audit skips)
//   - Variable: {X}, {Y}, {Z}                (CR §107.3, §107.3b)
//   - Colored: {W}, {U}, {B}, {R}, {G}        (CR §107.4a)
//   - Colorless: {C}                          (CR §107.4c)
//   - Snow: {S}                               (CR §107.4f)
//   - Hybrid colored: {W/U}, {W/B}, {U/B},
//       {U/R}, {B/R}, {B/G}, {R/G}, {R/W},
//       {G/W}, {G/U}                         (CR §107.4e — 10 enemy
//                                            + allied pairs only)
//   - Monocolored hybrid (generic-or-colored):
//       {2/W}, {2/U}, {2/B}, {2/R}, {2/G}    (CR §107.4e)
//   - Phyrexian: {W/P}, {U/P}, {B/P}, {R/P},
//       {G/P}                                (CR §107.4i)
//   - Hybrid Phyrexian: {W/U/P}, {U/B/P},
//       {B/R/P}, {R/G/P}, {G/W/P},
//       {W/B/P}, {B/G/P}, {G/U/P},
//       {U/R/P}, {R/W/P}                    (CR §107.4i — appears on
//                                            Phyrexia: All Will Be One
//                                            phyrexian cycle)
//
// The probe is opt-in via --check-mana-costs. When set, the binary
// skips the REPL and runs the scan in batch mode against --oracle
// (defaults to the same path the REPL uses). Output is a JSON report
// suitable for CI consumption — exit status 0 on a clean scan, 1 when
// violations are found (so a CI hook can gate on it).
//
// Why this is hexdek-judge's job (not freya's): Freya's legality.go
// runs deck-level Commander format checks (singleton, color identity,
// banned list, commander legitimacy). The CR §202.2 mana cost grammar
// is a card-level rule and applies to every card in every format. A
// failure here typically means either a Scryfall data oddity OR a real
// printing typo — both are signals the rules engine wants to surface
// before it hits a malformed token at simulation time.

// ManaCostReport is the top-level JSON shape emitted by --check-mana-costs.
type ManaCostReport struct {
	OraclePath  string                `json:"oracle_path"`
	TotalCards  int                   `json:"total_cards"`
	Scanned     int                   `json:"scanned"`         // cards with non-empty mana_cost
	Vanilla     int                   `json:"vanilla"`         // cards skipped (empty mana_cost — lands, MDFC back faces, etc.)
	Violations  []ManaCostViolation   `json:"violations"`
	ViolationsByKind map[string]int   `json:"violations_by_kind"`
	Valid       bool                  `json:"valid"`
}

// ManaCostViolation captures one card whose printed mana cost contains a
// symbol outside the CR §202.2 grammar.
type ManaCostViolation struct {
	CardName  string `json:"card_name"`
	SetCode   string `json:"set_code,omitempty"`
	ManaCost  string `json:"mana_cost"`
	BadSymbol string `json:"bad_symbol"`
	Kind      string `json:"kind"` // "unknown_letter" | "malformed_brace" | "out_of_grammar"
	Reason    string `json:"reason"`
}

// oracleManaEntry mirrors only the fields ManaCost check reads from
// oracle-cards.json. Streaming-decode keeps memory bounded on the
// 163MB corpus.
type oracleManaEntry struct {
	Name       string                 `json:"name"`
	SetCode    string                 `json:"set"`
	Layout     string                 `json:"layout"`
	ManaCost   string                 `json:"mana_cost"`
	BorderColor string                `json:"border_color"`
	SetType    string                 `json:"set_type"`
	CardFaces  []oracleManaCardFace   `json:"card_faces"`
}

type oracleManaCardFace struct {
	Name     string `json:"name"`
	ManaCost string `json:"mana_cost"`
}

// excludedSetTypes mirrors the filter used by cmd/parser-coverage —
// Un-set / memorabilia / Mystery Booster minigame / Alchemy entries
// have their own grammar (half-pips, infinite mana, perpetual costs)
// that CR §202.2 doesn't cover. Including them would flood the
// violation list with cards whose printing is rules-correct for
// their format.
var manaCheckExcludedSetTypes = map[string]bool{
	"memorabilia": true,
	"token":       true,
	"minigame":    true,
	"funny":       true,
	"alchemy":     true,
}

// manaSymbolRE matches one mana symbol — a {...} group with non-empty
// content. Anything between braces is captured for grammar checking.
var manaSymbolRE = regexp.MustCompile(`\{([^{}]*)\}`)

// validManaSymbols is the CR §202.2 grammar as an exact-match set.
// Hybrid orderings are normalized via canonicalSymbol; the canonical
// form for each hybrid pair is the WUBRG-cycle order Scryfall already
// emits (W/U, U/B, ..., G/W for allied; W/B, U/R, ..., G/U for enemy;
// W/U/P, U/B/P, ..., G/W/P for hybrid Phyrexian; 2/W, 2/U, ... for
// generic-or-colored).
var validManaSymbols = map[string]bool{
	// Generic numeric — sized to cover the full printed range without
	// special-casing each integer. 0..20 covers every modern printing;
	// CR §107.4 explicitly mentions up to {15} as the canonical large
	// generic cost; the {16} on Draco and {17} on Khalni Hydra reach
	// just past that.
	"0": true, "1": true, "2": true, "3": true, "4": true, "5": true,
	"6": true, "7": true, "8": true, "9": true, "10": true, "11": true,
	"12": true, "13": true, "14": true, "15": true, "16": true, "17": true,
	"18": true, "19": true, "20": true,

	// Variable.
	"X": true, "Y": true, "Z": true,

	// Colored / colorless / snow.
	"W": true, "U": true, "B": true, "R": true, "G": true, "C": true, "S": true,

	// Hybrid colored — 10 pairs (5 allied + 5 enemy). Both Scryfall's
	// canonical printed order AND the reversed order are listed so
	// canonicalSymbol doesn't need WUBRG-cycle awareness — a simple
	// alphabetical sort of letters with /P at the tail is enough to
	// land on one of the 20 entries below.
	"W/U": true, "U/B": true, "B/R": true, "R/G": true, "G/W": true,
	"W/B": true, "U/R": true, "B/G": true, "R/W": true, "G/U": true,
	"U/W": true, "B/U": true, "R/B": true, "G/R": true, "W/G": true,
	"B/W": true, "R/U": true, "G/B": true, "W/R": true, "U/G": true,

	// Monocolored hybrid (generic-or-colored). Only the {2}-first
	// canonical form prints — Scryfall never emits {W/2}.
	"2/W": true, "2/U": true, "2/B": true, "2/R": true, "2/G": true,

	// Phyrexian (mono).
	"W/P": true, "U/P": true, "B/P": true, "R/P": true, "G/P": true,

	// Colorless-hybrid (CR §107.4 cycle introduced on Modern Horizons 3's
	// Eldrazi Titans — Ulalek, Fused Atrocity costs {C/W}{C/U}{C/B}{C/R}{C/G}).
	// The /C side stays at canonical-tail under canonicalSymbol (numbers
	// lead, letters alphabetized, P at tail) — letter-order canonicalSymbol
	// produces "C/W" for {W/C}, so list both orderings.
	"C/W": true, "C/U": true, "C/B": true, "C/R": true, "C/G": true,
	"W/C": true, "U/C": true, "B/C": true, "R/C": true, "G/C": true,

	// Colorless Phyrexian (CR §107.4i extension — Kozilek, Compleated
	// costs {8}{C/P}{C/P} in MB2 / MH3).
	"C/P": true,

	// Hybrid Phyrexian (Phyrexia: All Will Be One cycle — 10 pairs).
	// As with the plain hybrid set, both orderings are listed so the
	// canonical key lookup doesn't need cycle awareness.
	"W/U/P": true, "U/B/P": true, "B/R/P": true, "R/G/P": true, "G/W/P": true,
	"W/B/P": true, "U/R/P": true, "B/G/P": true, "R/W/P": true, "G/U/P": true,
	"U/W/P": true, "B/U/P": true, "R/B/P": true, "G/R/P": true, "W/G/P": true,
	"B/W/P": true, "R/U/P": true, "G/B/P": true, "W/R/P": true, "U/G/P": true,
}

// canonicalSymbol uppercases and sorts a hybrid symbol into a stable
// form so {U/W} matches {W/U}. The Phyrexian suffix /P is preserved at
// the tail of hybrid Phyrexian forms; generic-or-colored {2/X} keeps
// the 2 leading. This matches Scryfall's printed canonical form.
func canonicalSymbol(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !strings.Contains(s, "/") {
		return s
	}
	parts := strings.Split(s, "/")
	// Phyrexian marker — always sorts to the tail.
	hasP := false
	hasNumeric := ""
	var letters []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "P" {
			hasP = true
			continue
		}
		if n, err := strconv.Atoi(p); err == nil && n >= 2 {
			hasNumeric = strconv.Itoa(n)
			continue
		}
		letters = append(letters, p)
	}
	sort.Strings(letters)
	var out []string
	if hasNumeric != "" {
		out = append(out, hasNumeric)
	}
	out = append(out, letters...)
	if hasP {
		out = append(out, "P")
	}
	return strings.Join(out, "/")
}

// validateManaCost parses a printed mana_cost string and returns
// (symbol, reason) for the first invalid symbol it finds, or "" if
// the cost is valid (or empty).
//
// The two failure modes:
//  1. malformed_brace — unbalanced "{" / "}" — the symbol couldn't
//     be extracted at all
//  2. out_of_grammar — symbol extracted but not in validManaSymbols
//     (could be an unknown letter, a hybrid combination CR doesn't
//     define, a too-large integer, etc.)
//
// Empty input is treated as valid (lands print with no mana cost).
func validateManaCost(cost string) (bad string, kind string, reason string) {
	if strings.TrimSpace(cost) == "" {
		return "", "", ""
	}
	// Quick balance check: every "{" must be matched by a later "}".
	depth := 0
	for _, r := range cost {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return cost, "malformed_brace", "unbalanced closing brace"
			}
		}
	}
	if depth != 0 {
		return cost, "malformed_brace", "unbalanced opening brace"
	}
	// Strip out every well-formed {…} symbol; if anything's left over
	// (other than whitespace), the cost contains characters outside
	// the brace-symbol grammar — Scryfall would never emit that, but
	// flag defensively in case a custom format slips in.
	residue := manaSymbolRE.ReplaceAllString(cost, "")
	if strings.TrimSpace(residue) != "" {
		return strings.TrimSpace(residue), "malformed_brace", "characters outside any {symbol} group"
	}
	// Now check each extracted symbol against the grammar.
	for _, m := range manaSymbolRE.FindAllStringSubmatch(cost, -1) {
		raw := m[1]
		canon := canonicalSymbol(raw)
		if !validManaSymbols[canon] {
			return "{" + raw + "}", "out_of_grammar",
				"symbol {" + raw + "} not in CR §202.2 grammar (canonical=" + canon + ")"
		}
	}
	return "", "", ""
}

// runManaCostCheck streams oracle-cards.json, validates each card's
// printed mana_cost, and writes a JSON report to outPath (or stdout
// when outPath is empty). Returns the report and an error suitable for
// fatal-logging.
func runManaCostCheck(oraclePath, outPath string) (*ManaCostReport, error) {
	f, err := os.Open(oraclePath)
	if err != nil {
		return nil, fmt.Errorf("open oracle: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	// Read the opening "["
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("read array start: %w", err)
	}
	rep := &ManaCostReport{
		OraclePath:       oraclePath,
		Violations:       []ManaCostViolation{},
		ViolationsByKind: map[string]int{},
		Valid:            true,
	}

	checkOne := func(name, setCode, cost string) {
		bad, kind, reason := validateManaCost(cost)
		if bad == "" {
			return
		}
		rep.Valid = false
		rep.ViolationsByKind[kind]++
		rep.Violations = append(rep.Violations, ManaCostViolation{
			CardName:  name,
			SetCode:   setCode,
			ManaCost:  cost,
			BadSymbol: bad,
			Kind:      kind,
			Reason:    reason,
		})
	}

	for dec.More() {
		var e oracleManaEntry
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("decode card: %w", err)
		}
		rep.TotalCards++
		if e.BorderColor == "silver" || manaCheckExcludedSetTypes[e.SetType] {
			continue
		}
		// Multi-face cards (DFCs, MDFCs, splits, flips, adventures)
		// have per-face mana_cost; the top-level mana_cost may be
		// empty. Walk each face explicitly so back-face spells aren't
		// silently skipped, then also check the top-level field when
		// non-empty.
		if len(e.CardFaces) > 0 {
			for _, face := range e.CardFaces {
				if strings.TrimSpace(face.ManaCost) == "" {
					rep.Vanilla++
					continue
				}
				rep.Scanned++
				checkOne(face.Name, e.SetCode, face.ManaCost)
			}
			continue
		}
		if strings.TrimSpace(e.ManaCost) == "" {
			rep.Vanilla++
			continue
		}
		rep.Scanned++
		checkOne(e.Name, e.SetCode, e.ManaCost)
	}

	// Stable violation order so a CI hook can diff reports across runs.
	sort.Slice(rep.Violations, func(i, j int) bool {
		if rep.Violations[i].CardName != rep.Violations[j].CardName {
			return rep.Violations[i].CardName < rep.Violations[j].CardName
		}
		return rep.Violations[i].BadSymbol < rep.Violations[j].BadSymbol
	})

	if err := writeManaCostReport(rep, outPath); err != nil {
		return rep, err
	}
	return rep, nil
}

// writeManaCostReport emits the JSON to outPath or stdout when empty.
func writeManaCostReport(rep *ManaCostReport, outPath string) error {
	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
