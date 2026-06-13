package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/judge"
)

// CR §903 Commander format legality probe.
//
// Second batch-mode probe in hexdek-judge, following the §202.2 mana
// cost check. This one operates on a deck text file (the canonical
// `COMMANDER: ...\n1 Card\n...` shape that hexdek-import emits) and
// validates the deck against the relevant Comprehensive Rules
// sub-sections of §903:
//
//   - §903.5a — A commander is a legendary creature.
//   - §903.5b — Some non-creature cards designate themselves as commanders
//               with "can be your commander" or analogous text (planeswalkers
//               like Daretti / Saheeli / Teferi; Backgrounds via §903.5e;
//               Doctor Who companion-pairing via §903.5g).
//   - §903.6  — The commander itself must be Commander-format-legal
//               (Scryfall's `legalities.commander == "legal"`).
//   - §903.4  — Every card in the deck must have a color identity that
//               is a subset of the commander's color identity.
//   - §903.5  — No card in the deck may be on the Commander banned list.
//
// Multi-commander shapes (partners / backgrounds / friends-forever)
// are recognized but treated as a single "primary commander" for the
// color-identity union — when more than one COMMANDER: line is
// present, the allowed identity is the union of all listed commanders'
// identities (CR §903.4).
//
// Out of scope for this probe (intentional):
//   - §903.5c partner-mechanic validation (does each named partner
//     ACTUALLY have "partner" / "partner with X" text?) — would
//     require a second oracle pass per commander pair.
//   - Singleton check — covered by freya/legality.go for full deck
//     analysis; the judge probe is a fast targeted check.
//   - Card count (CR §903.5 requires exactly 100 cards) — same
//     reason; freya's CardCountCheck handles it.
//
// This split keeps the judge probe a small, focused, CI-friendly
// rules-correctness probe that runs in well under a second on a real
// deck file, separate from Freya's heavier full-deck-analysis path.

// CommanderReport is the top-level JSON shape emitted by --check-commander.
type CommanderReport struct {
	Rule                   string                 `json:"rule"`
	DeckPath               string                 `json:"deck_path"`
	Commanders             []string               `json:"commanders"`
	CommanderColorIdentity []string               `json:"commander_color_identity"`
	DeckCardCount          int                    `json:"deck_card_count"`
	Checks                 CommanderChecks        `json:"checks"`
	Valid                  bool                   `json:"valid"`
	Warnings               []string               `json:"warnings,omitempty"`
}

// CommanderChecks groups the five sub-rule verdicts.
type CommanderChecks struct {
	CommanderLegality    CommanderLegalityCheck `json:"commander_legality"`
	CommanderFormatLegal FormatLegalityCheck    `json:"commander_format_legal"`
	ColorIdentity        ColorIdentityCheck     `json:"color_identity"`
	BannedCards          BannedCardsCheck       `json:"banned_cards"`
}

// CommanderLegalityCheck reports per-commander §903.5a / §903.5b status.
type CommanderLegalityCheck struct {
	Valid     bool                       `json:"valid"`
	PerCmdr   []CommanderLegalityVerdict `json:"per_commander"`
}

// CommanderLegalityVerdict is one commander's verdict.
type CommanderLegalityVerdict struct {
	Name   string `json:"name"`
	Valid  bool   `json:"valid"`
	Reason string `json:"reason"` // "legendary creature (§903.5a)" / "can be your commander (§903.5b)" / failure reason
	Rule   string `json:"rule,omitempty"`
}

// FormatLegalityCheck reports §903.6 Commander-format legality.
type FormatLegalityCheck struct {
	Valid      bool                     `json:"valid"`
	PerCmdr    []FormatLegalityVerdict  `json:"per_commander"`
}

// FormatLegalityVerdict captures Scryfall's `legalities.commander` for one commander.
type FormatLegalityVerdict struct {
	Name           string `json:"name"`
	Valid          bool   `json:"valid"`
	ScryfallStatus string `json:"scryfall_status"` // "legal" | "not_legal" | "banned" | "restricted" | "unknown"
}

// ColorIdentityCheck reports §903.4 color identity verdicts for the 99.
type ColorIdentityCheck struct {
	Valid          bool                 `json:"valid"`
	Allowed        []string             `json:"allowed_colors"`
	Violations     []ColorIDViolation   `json:"violations"`
}

// ColorIDViolation captures one card whose CI is not a subset of the
// commander union.
type ColorIDViolation struct {
	CardName       string   `json:"card_name"`
	CardCIdentity  []string `json:"card_color_identity"`
	AllowedColors  []string `json:"allowed_colors"`
	OutOfIdentity  []string `json:"out_of_identity"`
}

// BannedCardsCheck reports cards on the Commander banned list.
type BannedCardsCheck struct {
	Valid      bool               `json:"valid"`
	Violations []BannedViolation  `json:"violations"`
}

// BannedViolation captures one banned card found in the deck.
type BannedViolation struct {
	CardName string `json:"card_name"`
	Where    string `json:"where"` // "commander" | "mainboard"
}

// commanderBannedList — the official Commander banned list as of April
// 2026. Mirrors cmd/hexdek-freya/legality.go's commanderBannedList; kept
// in sync manually. Lowercased for case-insensitive matching.
//
// Sync invariant: when freya updates its list (which is checked into
// the same repo and reviewed alongside engine changes), update this
// one too. The duplication is intentional — judge can't import a cmd
// package, and a shared internal/ package for a 50-entry list would
// be over-engineering.
var commanderBannedList = map[string]bool{
	"ancestral recall":            true,
	"balance":                     true,
	"biorhythm":                   true,
	"black lotus":                 true,
	"braids, cabal minion":        true,
	"channel":                     true,
	"coalition victory":           true,
	"dockside extortionist":       true,
	"emrakul, the aeons torn":     true,
	"erayo, soratami ascendant":   true,
	"falling star":                true,
	"fastbond":                    true,
	"flash":                       true,
	"golos, tireless pilgrim":     true,
	"griselbrand":                 true,
	"hullbreacher":                true,
	"iona, shield of emeria":      true,
	"jeweled lotus":               true,
	"karakas":                     true,
	"leovold, emissary of trest":  true,
	"library of alexandria":       true,
	"limited resources":           true,
	"lutri, the spellchaser":      true,
	"mana crypt":                  true,
	"mox emerald":                 true,
	"mox jet":                     true,
	"mox pearl":                   true,
	"mox ruby":                    true,
	"mox sapphire":                true,
	"nadu, winged wisdom":         true,
	"panoptic mirror":             true,
	"paradox engine":              true,
	"primeval titan":              true,
	"prophet of kruphix":          true,
	"recurring nightmare":         true,
	"rofellos, llanowar emissary": true,
	"shahrazad":                   true,
	"sundering titan":             true,
	"sway of the stars":           true,
	"sylvan primordial":           true,
	"time vault":                  true,
	"time walk":                   true,
	"tinker":                      true,
	"tolarian academy":            true,
	"trade secrets":               true,
	"upheaval":                    true,
	"worldfire":                   true,
	"yawgmoth's bargain":          true,
}

// oracleCmdrEntry is the per-card slice we need from oracle-cards.json
// to evaluate §903 — enough for the commander-eligibility classification
// (type line + oracle text + back-face for MDFCs), the color identity
// (§903.4), and the commander-format-legality check (§903.6).
type oracleCmdrEntry struct {
	Name           string                 `json:"name"`
	TypeLine       string                 `json:"type_line"`
	OracleText     string                 `json:"oracle_text"`
	ColorIdentity  []string               `json:"color_identity"`
	Legalities     map[string]string      `json:"legalities"`
	Layout         string                 `json:"layout"`
	CardFaces      []oracleCmdrFace       `json:"card_faces"`
	BorderColor    string                 `json:"border_color"`
}

type oracleCmdrFace struct {
	Name       string   `json:"name"`
	TypeLine   string   `json:"type_line"`
	OracleText string   `json:"oracle_text"`
}

// oracleCmdrDB is the in-memory name→entry lookup. Names are stored
// normalized (lowercase, trimmed) for the resolver. For DFCs / MDFCs /
// splits the canonical "Front // Back" name is the lookup key Scryfall
// emits; we also index by front-face and back-face names so a
// deck listing either form resolves.
type oracleCmdrDB struct {
	byName map[string]*oracleCmdrEntry
}

func newOracleCmdrDB() *oracleCmdrDB {
	return &oracleCmdrDB{byName: map[string]*oracleCmdrEntry{}}
}

// normCmdrName canonicalizes a name for case-insensitive lookup. Folds
// curly quotes back to ASCII so " typed " from Scryfall and a plaintext
// deck file match.
func normCmdrName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.ReplaceAll(s, "‘", "'")
	return s
}

func (db *oracleCmdrDB) get(name string) *oracleCmdrEntry {
	if db == nil {
		return nil
	}
	return db.byName[normCmdrName(name)]
}

func (db *oracleCmdrDB) put(e *oracleCmdrEntry) {
	if e == nil || e.Name == "" {
		return
	}
	db.byName[normCmdrName(e.Name)] = e
	// Index each face name too — a deck file may list a DFC by either
	// "Brazen Borrower // Petty Theft" (canonical) or "Brazen Borrower"
	// (front face only). Both must resolve.
	for _, f := range e.CardFaces {
		if f.Name != "" {
			db.byName[normCmdrName(f.Name)] = e
		}
	}
}

// nonPlayableLayouts are Scryfall layout values for entries that
// shadow real card names but aren't actually playable cards. Caught
// by smoke-testing Edgar Markov: Scryfall ships an `art_series` entry
// named "Edgar Markov // Edgar Markov" with type_line "Card // Card",
// which (when the entry is indexed by face name) would overwrite the
// real Legendary Creature entry — leading the probe to report
// Edgar as "not a legendary creature." The same hazard applies to
// emblems, double_faced_tokens, schemes, planes, phenomena, and
// vanguard avatars.
var nonPlayableLayouts = map[string]bool{
	"art_series":         true,
	"emblem":             true,
	"double_faced_token": true,
	"scheme":             true,
	"plane":              true,
	"phenomenon":         true,
	"vanguard":           true,
	"token":              true,
}

// loadOracleCmdrDB streams oracle-cards.json and builds the lookup.
// Skips silver-border and non-playable layouts (art_series, emblems,
// tokens, …) to match other probes' filters; everything else is
// indexed so an Un-set commander attempt surfaces a clear failure
// reason instead of "commander not found".
func loadOracleCmdrDB(path string) (*oracleCmdrDB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open oracle: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("read array start: %w", err)
	}
	db := newOracleCmdrDB()
	for dec.More() {
		var e oracleCmdrEntry
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("decode card: %w", err)
		}
		if e.BorderColor == "silver" {
			continue
		}
		if nonPlayableLayouts[e.Layout] {
			continue
		}
		// Make a heap-allocated copy so the pointer stored in the map
		// outlives the loop iteration's stack slot.
		entry := e
		db.put(&entry)
	}
	return db, nil
}

// classifyCommander returns (valid, reason) for whether the entry can
// serve as a commander under §903.5a / §903.5b. A nil entry returns a
// "not found" failure.
//
// The rule set:
//   - Legendary creature → §903.5a pass
//   - Card text contains "can be your commander" → §903.5b pass.
//     Covers the planeswalker commander cycle (Saheeli, Teferi, Daretti,
//     Tibalt, etc.), Doctor Who companions ("can be your commander
//     when..."), and the various §903.5b-tagged legendary noncreatures.
//   - DFCs / MDFCs: either face may qualify; the front face's text +
//     type line is checked first, back face as fallback. Some Doctor
//     Who companion cards mark §903.5b on the front face only; some
//     transform commanders (Garruk Relentless / Veteran Hunter) mark
//     it via the back-face state.
//   - Anything else → fail with the actual type line.
func classifyCommander(e *oracleCmdrEntry) (bool, string) {
	if e == nil {
		return false, "not found in oracle corpus"
	}
	tl := strings.ToLower(e.TypeLine)
	ot := strings.ToLower(e.OracleText)
	if strings.Contains(tl, "legendary") && strings.Contains(tl, "creature") {
		return true, "legendary creature (CR §903.5a)"
	}
	if strings.Contains(ot, "can be your commander") {
		return true, "designated commander via oracle text (CR §903.5b)"
	}
	for _, f := range e.CardFaces {
		ftl := strings.ToLower(f.TypeLine)
		fot := strings.ToLower(f.OracleText)
		if strings.Contains(ftl, "legendary") && strings.Contains(ftl, "creature") {
			return true, "legendary creature face (CR §903.5a, via " + f.Name + ")"
		}
		if strings.Contains(fot, "can be your commander") {
			return true, "designated commander via face oracle text (CR §903.5b, via " + f.Name + ")"
		}
	}
	tlReport := strings.TrimSpace(e.TypeLine)
	if tlReport == "" {
		tlReport = "(unknown type)"
	}
	return false, "not a legendary creature and has no \"can be your commander\" text — type line: " + tlReport
}

// classifyFormatLegal returns (valid, scryfall_status). The status
// comes from Scryfall's `legalities.commander` field which has six
// canonical values: "legal" / "not_legal" / "banned" / "restricted" /
// "banned_as_commander" / "banned_in_deck". For §903.6 anything other
// than "legal" / "restricted" disqualifies; banned_as_commander
// flagged here is the specific §903.5d / §903.5e exclusion (e.g.
// Lutri before the 2020 banning was banned_as_commander only).
//
// A nil entry returns "unknown" to distinguish "card missing" from
// "card present but not legal".
func classifyFormatLegal(e *oracleCmdrEntry) (bool, string) {
	if e == nil {
		return false, "unknown"
	}
	// Shared stale-ban override: a recently-unbanned commander whose
	// cached Scryfall legality still reads "banned" reads legal here too
	// (consistent with the import + deck-analysis paths).
	if judge.IsUnbannedOverride("commander", e.Name) {
		return true, "legal"
	}
	if e.Legalities == nil {
		return true, "unknown"
	}
	status, ok := e.Legalities["commander"]
	if !ok {
		return true, "unknown"
	}
	switch status {
	case "legal", "restricted":
		return true, status
	default:
		return false, status
	}
}

// validateDeckColorIdentity walks the deck cards and flags any whose
// color identity is not a subset of `allowed`. Cards not found in
// oracle are silently skipped — the unresolved-card failure is a
// different rule than the §903.4 color-identity check, and the probe
// doesn't want a missing Scryfall entry to produce a false §903.4
// violation. (A future expansion could surface unresolved cards
// separately.)
func validateDeckColorIdentity(deck []deckCard, allowed map[string]bool, allowedSlice []string, db *oracleCmdrDB) ColorIdentityCheck {
	out := ColorIdentityCheck{
		Valid:      true,
		Allowed:    allowedSlice,
		Violations: []ColorIDViolation{},
	}
	for _, c := range deck {
		entry := db.get(c.Name)
		if entry == nil {
			continue
		}
		var outside []string
		for _, col := range entry.ColorIdentity {
			if !allowed[strings.ToUpper(col)] {
				outside = append(outside, col)
			}
		}
		if len(outside) > 0 {
			out.Valid = false
			out.Violations = append(out.Violations, ColorIDViolation{
				CardName:      entry.Name,
				CardCIdentity: entry.ColorIdentity,
				AllowedColors: allowedSlice,
				OutOfIdentity: outside,
			})
		}
	}
	sort.Slice(out.Violations, func(i, j int) bool {
		return out.Violations[i].CardName < out.Violations[j].CardName
	})
	return out
}

// validateBannedCards checks both commanders and deck cards against
// commanderBannedList. Same card found in both commander and mainboard
// is reported twice with distinct `where`.
func validateBannedCards(commanderNames []string, deck []deckCard) BannedCardsCheck {
	out := BannedCardsCheck{Valid: true, Violations: []BannedViolation{}}
	for _, name := range commanderNames {
		if commanderBannedList[normCmdrName(name)] && !judge.IsUnbannedOverride("commander", name) {
			out.Valid = false
			out.Violations = append(out.Violations, BannedViolation{
				CardName: name,
				Where:    "commander",
			})
		}
	}
	for _, c := range deck {
		if commanderBannedList[normCmdrName(c.Name)] && !judge.IsUnbannedOverride("commander", c.Name) {
			out.Valid = false
			out.Violations = append(out.Violations, BannedViolation{
				CardName: c.Name,
				Where:    "mainboard",
			})
		}
	}
	sort.Slice(out.Violations, func(i, j int) bool {
		if out.Violations[i].Where != out.Violations[j].Where {
			return out.Violations[i].Where < out.Violations[j].Where
		}
		return out.Violations[i].CardName < out.Violations[j].CardName
	})
	return out
}

// deckCard is one deck entry parsed from the text file.
type deckCard struct {
	Quantity int
	Name     string
}

// parseDeckFile reads the canonical hexdek-import deck format:
//
//	# Optional title header
//	# Source: ...
//	COMMANDER: Name
//	PARTNER: Optional Partner Name
//	1 Card Name
//	1 Another Card
//	// Sideboard: ...   (silently dropped)
//	// Companion: ...   (silently dropped)
//
// Returns commander names (preserves source order so partner pairs
// stay paired) and the parsed mainboard. Lines that don't match
// either marker are silently ignored — matches the lenient parsing
// stance of moxfield.ParseDecklist and internal/deckparser.
func parseDeckFile(r io.Reader) (commanders []string, deck []deckCard, err error) {
	cmdrRE := regexp.MustCompile(`(?i)^\s*COMMANDER\s*:\s*(.+?)\s*$`)
	partnerRE := regexp.MustCompile(`(?i)^\s*PARTNER\s*:\s*(.+?)\s*$`)
	qtyRE := regexp.MustCompile(`^\s*(\d+)\s*[xX]?\s+(.+?)\s*$`)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	skipSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			skipSection = false
			continue
		}
		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			lower := strings.ToLower(strings.TrimLeft(line, "/# \t"))
			switch {
			case strings.HasPrefix(lower, "sideboard"),
				strings.HasPrefix(lower, "companion"),
				strings.HasPrefix(lower, "maybeboard"),
				strings.HasPrefix(lower, "considering"),
				strings.HasPrefix(lower, "plane"),
				strings.HasPrefix(lower, "scheme"),
				strings.HasPrefix(lower, "signaturespell"),
				strings.HasPrefix(lower, "attraction"),
				strings.HasPrefix(lower, "sticker"),
				strings.HasPrefix(lower, "contraption"):
				skipSection = true
				continue
			}
			// Other comment lines (# header, # Source:, // Tags:) → ignored.
			continue
		}
		if m := cmdrRE.FindStringSubmatch(line); m != nil {
			commanders = append(commanders, strings.TrimSpace(m[1]))
			continue
		}
		if m := partnerRE.FindStringSubmatch(line); m != nil {
			commanders = append(commanders, strings.TrimSpace(m[1]))
			continue
		}
		if skipSection {
			continue
		}
		if m := qtyRE.FindStringSubmatch(line); m != nil {
			qty, _ := strconv.Atoi(m[1])
			if qty <= 0 {
				continue
			}
			deck = append(deck, deckCard{Quantity: qty, Name: strings.TrimSpace(m[2])})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read deck: %w", err)
	}
	return commanders, deck, nil
}

// runCommanderCheck is the entry point. Loads oracle, parses the deck
// file, runs the four sub-checks, emits JSON to outPath (or stdout
// when empty).
func runCommanderCheck(deckPath, oraclePath, outPath string) (*CommanderReport, error) {
	if deckPath == "" {
		return nil, fmt.Errorf("--check-commander requires --deck <path>")
	}
	db, err := loadOracleCmdrDB(oraclePath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(deckPath)
	if err != nil {
		return nil, fmt.Errorf("open deck: %w", err)
	}
	defer f.Close()
	commanders, deck, err := parseDeckFile(f)
	if err != nil {
		return nil, err
	}

	rep := &CommanderReport{
		Rule:       "CR §903",
		DeckPath:   deckPath,
		Commanders: commanders,
		Checks: CommanderChecks{
			CommanderLegality: CommanderLegalityCheck{
				Valid:   true,
				PerCmdr: []CommanderLegalityVerdict{},
			},
			CommanderFormatLegal: FormatLegalityCheck{
				Valid:   true,
				PerCmdr: []FormatLegalityVerdict{},
			},
		},
		Valid: true,
	}
	deckCardCount := 0
	for _, c := range deck {
		deckCardCount += c.Quantity
	}
	rep.DeckCardCount = deckCardCount

	if len(commanders) == 0 {
		rep.Valid = false
		rep.Checks.CommanderLegality.Valid = false
		rep.Checks.CommanderLegality.PerCmdr = append(rep.Checks.CommanderLegality.PerCmdr, CommanderLegalityVerdict{
			Name:   "",
			Valid:  false,
			Reason: "no COMMANDER: directive found in deck file",
			Rule:   "CR §903.5",
		})
		rep.Checks.CommanderFormatLegal.Valid = false
		// Still emit a final report — caller picks up Valid=false.
		return rep, writeCommanderReport(rep, outPath)
	}

	// §903.5 + §903.6 per-commander.
	allowed := map[string]bool{}
	allowedSlice := []string{}
	allowedSeen := map[string]bool{}
	for _, name := range commanders {
		entry := db.get(name)
		ok, reason := classifyCommander(entry)
		v := CommanderLegalityVerdict{Name: name, Valid: ok, Reason: reason}
		if !ok {
			rep.Checks.CommanderLegality.Valid = false
			if strings.Contains(reason, "§903.5a") {
				v.Rule = "CR §903.5a"
			} else if strings.Contains(reason, "§903.5b") {
				v.Rule = "CR §903.5b"
			} else {
				v.Rule = "CR §903.5"
			}
		} else if strings.Contains(reason, "§903.5a") {
			v.Rule = "CR §903.5a"
		} else if strings.Contains(reason, "§903.5b") {
			v.Rule = "CR §903.5b"
		}
		rep.Checks.CommanderLegality.PerCmdr = append(rep.Checks.CommanderLegality.PerCmdr, v)

		legalOK, status := classifyFormatLegal(entry)
		rep.Checks.CommanderFormatLegal.PerCmdr = append(rep.Checks.CommanderFormatLegal.PerCmdr, FormatLegalityVerdict{
			Name:           name,
			Valid:          legalOK,
			ScryfallStatus: status,
		})
		if !legalOK {
			rep.Checks.CommanderFormatLegal.Valid = false
		}

		// Union of color identities across all commanders for §903.4.
		if entry != nil {
			for _, col := range entry.ColorIdentity {
				up := strings.ToUpper(col)
				if !allowed[up] {
					allowed[up] = true
					allowedSlice = append(allowedSlice, up)
				}
				allowedSeen[up] = true
			}
		}
	}
	// Sort allowed identity in canonical WUBRG order so the JSON output
	// matches Scryfall's printed convention.
	allowedSlice = sortWUBRG(allowedSlice)
	rep.CommanderColorIdentity = allowedSlice

	// §903.4 color identity scan.
	rep.Checks.ColorIdentity = validateDeckColorIdentity(deck, allowed, allowedSlice, db)
	if !rep.Checks.ColorIdentity.Valid {
		rep.Valid = false
	}

	// Banned list scan (commanders + deck).
	rep.Checks.BannedCards = validateBannedCards(commanders, deck)
	if !rep.Checks.BannedCards.Valid {
		rep.Valid = false
	}

	if !rep.Checks.CommanderLegality.Valid || !rep.Checks.CommanderFormatLegal.Valid {
		rep.Valid = false
	}

	return rep, writeCommanderReport(rep, outPath)
}

// sortWUBRG returns the input colors in canonical WUBRG order.
func sortWUBRG(colors []string) []string {
	order := map[string]int{"W": 0, "U": 1, "B": 2, "R": 3, "G": 4}
	out := make([]string, 0, len(colors))
	seen := map[string]bool{}
	for _, c := range colors {
		up := strings.ToUpper(c)
		if !seen[up] {
			seen[up] = true
			out = append(out, up)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		oi, oki := order[out[i]]
		oj, okj := order[out[j]]
		if !oki {
			oi = 99
		}
		if !okj {
			oj = 99
		}
		if oi != oj {
			return oi < oj
		}
		return out[i] < out[j]
	})
	return out
}

func writeCommanderReport(rep *CommanderReport, outPath string) error {
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
