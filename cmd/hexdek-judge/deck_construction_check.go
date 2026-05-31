package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// CR §903.5 / §100.4 Commander deck construction probe.
//
// Third batch-mode probe in hexdek-judge after --check-mana-costs
// (CR §202.2) and --check-commander (CR §903.5/.5b/.6/.4 + banned).
// This one focuses on the structural deck-construction rules — the
// rules that determine whether a list of cards is a well-formed
// Commander deck, regardless of which specific commander or card pool
// the player picked:
//
//   - §903.5  Card count — exactly 100 cards including the commander
//             (or 98 + 2 for partner / background pairings).
//   - §903.5b Singleton — at most one copy of any non-basic card.
//             Basic lands AND cards with "a deck can have any number
//             of cards named …" rules text are exempt.
//   - §903.4  Color identity — re-checked here so the deck-construction
//             probe is self-contained; the --check-commander probe
//             covers the same rule from the commander-eligibility
//             angle, but a deck-construction CI gate shouldn't need
//             two separate probe runs.
//
// On top of the immutable structural rules, an optional --bracket N
// (1-5) layer applies the WotC 2024 Commander bracket-system
// "shape" expectations to the land base. Each bracket has a typical
// land-count range; a deck claiming a bracket whose land count falls
// outside that range gets a warning (not a hard violation — the
// bracket framework is advisory). The framework is itself out of CR
// scope, but tooling that auto-classifies a deck's bracket benefits
// from being able to flag "this deck claims B5 cEDH but plays 39
// lands" as a probable mis-tag.
//
// This split keeps the §903.5 / §903.5b / §903.4 hard rules separate
// from the §903.6 format-legality + banned-list logic already in
// --check-commander, so a CI hook can probe construction
// independently of legality (some workflows want to validate deck
// SHAPE before resolving cards against the live Commander banned
// list, which changes monthly).

// DeckConstructionReport is the top-level JSON shape for --check-deck-construction.
type DeckConstructionReport struct {
	Rule                   string                       `json:"rule"`
	DeckPath               string                       `json:"deck_path"`
	Commanders             []string                     `json:"commanders"`
	CommanderColorIdentity []string                     `json:"commander_color_identity"`
	DeckCardCount          int                          `json:"deck_card_count"`
	ExpectedCardCount      int                          `json:"expected_card_count"`
	Checks                 DeckConstructionChecks       `json:"checks"`
	BracketCheck           *BracketShapeCheck           `json:"bracket_check,omitempty"`
	Valid                  bool                         `json:"valid"`
	Warnings               []string                     `json:"warnings,omitempty"`
}

// DeckConstructionChecks groups the three structural verdicts.
type DeckConstructionChecks struct {
	CardCount     CardCountCheck     `json:"card_count"`
	Singleton     SingletonCheck     `json:"singleton"`
	ColorIdentity ColorIdentityCheck `json:"color_identity"`
}

// CardCountCheck reports §903.5 deck-size compliance.
type CardCountCheck struct {
	Valid           bool   `json:"valid"`
	ActualCount     int    `json:"actual_count"`
	ExpectedCount   int    `json:"expected_count"`
	HasPartner      bool   `json:"has_partner"`
	Reason          string `json:"reason,omitempty"`
}

// SingletonCheck reports §903.5b verdicts.
type SingletonCheck struct {
	Valid      bool                  `json:"valid"`
	Violations []SingletonViolation  `json:"violations"`
}

// SingletonViolation captures one non-basic / non-"any number" card
// that appears more than once.
type SingletonViolation struct {
	CardName string `json:"card_name"`
	Count    int    `json:"count"`
	Reason   string `json:"reason,omitempty"`
}

// BracketShapeCheck reports the per-bracket "shape" advisory layer.
type BracketShapeCheck struct {
	Bracket          int                       `json:"bracket"`
	BracketName      string                    `json:"bracket_name"`
	LandCount        int                       `json:"land_count"`
	BasicLandCount   int                       `json:"basic_land_count"`
	LandRange        BracketLandRange          `json:"expected_land_range"`
	Shape            string                    `json:"shape"` // "in_range" | "below_range" | "above_range"
	Warnings         []string                  `json:"warnings,omitempty"`
}

// BracketLandRange is the WotC 2024 framework's typical land-count
// expectation per bracket. Values are inclusive on both ends.
type BracketLandRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// bracketLandRanges encodes the WotC 2024 Commander bracket framework's
// typical land-count expectations.
//
// Anchored on community-published bracket framing:
//   - B1 (Exhibition / precon) — precons typically ship with 37-42
//     lands; the brand-new player who hasn't tuned ramp yet.
//   - B2 (Core / midpower) — 35-40 lands, ramp piece count
//     comfortable but not optimized.
//   - B3 (Upgraded / optimized) — 34-38 lands, ramp piece count
//     deliberately tuned so the curve drops a land or two.
//   - B4 (Optimized / high-power) — 31-36 lands, fast-mana density
//     replaces some land drops.
//   - B5 (cEDH) — 28-33 lands, ~10+ fast-mana density, every land
//     slot pulls its weight.
//
// These are advisory ranges, not hard rules — a B5 cEDH deck with 34
// lands is still legal, the warning just flags "land count is above
// the typical cEDH range; double-check the bracket tag."
var bracketLandRanges = map[int]BracketLandRange{
	1: {Min: 37, Max: 42},
	2: {Min: 35, Max: 40},
	3: {Min: 34, Max: 38},
	4: {Min: 31, Max: 36},
	5: {Min: 28, Max: 33},
}

var bracketNames = map[int]string{
	1: "Exhibition / precon (B1)",
	2: "Core / midpower (B2)",
	3: "Upgraded / optimized (B3)",
	4: "Optimized / high-power (B4)",
	5: "cEDH (B5)",
}

// runDeckConstructionCheck is the entry point. Loads oracle, parses
// the deck file, runs the three structural sub-checks plus the
// optional bracket-shape advisory.
func runDeckConstructionCheck(deckPath, oraclePath, outPath string, bracket int) (*DeckConstructionReport, error) {
	if deckPath == "" {
		return nil, fmt.Errorf("--check-deck-construction requires --deck <path>")
	}
	if bracket != 0 {
		if _, ok := bracketLandRanges[bracket]; !ok {
			return nil, fmt.Errorf("--bracket %d out of range (valid: 1-5)", bracket)
		}
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

	rep := &DeckConstructionReport{
		Rule:       "CR §903.5",
		DeckPath:   deckPath,
		Commanders: commanders,
		Valid:      true,
	}
	totalCount := 0
	for _, c := range deck {
		totalCount += c.Quantity
	}
	// Commander count is sum of named COMMANDER:/PARTNER: directives.
	// Deck file's mainboard does NOT include the commander(s) — those
	// are listed separately. Total deck size is mainboard + commanders.
	commanderCount := len(commanders)
	rep.DeckCardCount = totalCount + commanderCount
	rep.Checks.CardCount = checkCardCount(totalCount+commanderCount, commanderCount)
	rep.ExpectedCardCount = rep.Checks.CardCount.ExpectedCount
	if !rep.Checks.CardCount.Valid {
		rep.Valid = false
	}

	rep.Checks.Singleton = checkSingleton(deck, db)
	if !rep.Checks.Singleton.Valid {
		rep.Valid = false
	}

	// Color identity — union over all commanders, applied to the deck.
	allowed := map[string]bool{}
	allowedSlice := []string{}
	allowedSeen := map[string]bool{}
	for _, name := range commanders {
		e := db.get(name)
		if e == nil {
			continue
		}
		for _, col := range e.ColorIdentity {
			up := strings.ToUpper(col)
			if !allowed[up] {
				allowed[up] = true
				allowedSlice = append(allowedSlice, up)
			}
			allowedSeen[up] = true
		}
	}
	allowedSlice = sortWUBRG(allowedSlice)
	rep.CommanderColorIdentity = allowedSlice
	rep.Checks.ColorIdentity = validateDeckColorIdentity(deck, allowed, allowedSlice, db)
	if !rep.Checks.ColorIdentity.Valid {
		rep.Valid = false
	}

	// Optional bracket shape — advisory, never flips top-level Valid.
	if bracket != 0 {
		rep.BracketCheck = computeBracketShape(deck, db, bracket)
		if len(rep.BracketCheck.Warnings) > 0 {
			rep.Warnings = append(rep.Warnings, rep.BracketCheck.Warnings...)
		}
	}

	if len(commanders) == 0 {
		rep.Warnings = append(rep.Warnings, "no COMMANDER: directive in deck file — color identity check skipped")
	}

	return rep, writeDeckConstructionReport(rep, outPath)
}

// checkCardCount applies §903.5. For 0 commanders the expected is
// 100 (the deck file's mainboard alone is treated as the full deck —
// a deck listing without a COMMANDER: directive still has a
// well-defined card count check). For 1 commander it's 100. For 2
// commanders (partners / background / friends-forever) it's 100. The
// expectation is always 100 total; the partner case just has 2
// commanders + 98 in the mainboard rather than 1 + 99.
func checkCardCount(total, commanderCount int) CardCountCheck {
	out := CardCountCheck{
		ActualCount:   total,
		ExpectedCount: 100,
		HasPartner:    commanderCount >= 2,
	}
	if total == 100 {
		out.Valid = true
		if commanderCount >= 2 {
			out.Reason = fmt.Sprintf("100 cards (%d commanders + %d mainboard)",
				commanderCount, total-commanderCount)
		} else {
			out.Reason = fmt.Sprintf("100 cards (%d commander + %d mainboard)",
				commanderCount, total-commanderCount)
		}
		return out
	}
	out.Valid = false
	out.Reason = fmt.Sprintf("expected 100 cards, found %d (off by %+d)",
		total, total-100)
	return out
}

// basicLandNames is the canonical singleton-exempt basic-land set.
// Mirrors cmd/hexdek-freya/legality.go's singletonExemptBasics. Snow-
// covered basics share the same exemption per CR §903.5b ("any number
// of basic land cards with the same name").
var basicLandNames = map[string]bool{
	"plains":   true,
	"island":   true,
	"swamp":    true,
	"mountain": true,
	"forest":   true,
	"wastes":   true,
}

// isBasicLand returns true when the card name is a basic land subtype
// per CR §305.6 (Plains / Island / Swamp / Mountain / Forest /
// Wastes) or a snow-covered variant.
func isBasicLand(name string) bool {
	low := strings.ToLower(strings.TrimSpace(name))
	if basicLandNames[low] {
		return true
	}
	if strings.HasPrefix(low, "snow-covered ") {
		return basicLandNames[strings.TrimPrefix(low, "snow-covered ")]
	}
	return false
}

// hasAnyNumberOfText returns true when the card's oracle text contains
// "a deck can have any number of" — CR §903.5b's primary singleton
// exemption for Relentless Rats, Shadowborn Apostle, Persistent Petitioners,
// Rat Colony, Dragon's Approach, Hare Apparent, Templar Knight, etc.
func hasAnyNumberOfText(e *oracleCmdrEntry) bool {
	if e == nil {
		return false
	}
	ot := strings.ToLower(e.OracleText)
	if strings.Contains(ot, "a deck can have any number of") {
		return true
	}
	for _, f := range e.CardFaces {
		if strings.Contains(strings.ToLower(f.OracleText), "a deck can have any number of") {
			return true
		}
	}
	return false
}

// checkSingleton applies §903.5b. A card may appear more than once
// iff it's a basic land OR its oracle text contains "a deck can have
// any number of …" (Relentless Rats / Shadowborn Apostle / etc.).
// Cards not found in the oracle DB get treated as standard singleton
// — the unresolved-card case isn't a singleton violation but the
// quantity itself can still trip the check, which matches what the
// player sees.
func checkSingleton(deck []deckCard, db *oracleCmdrDB) SingletonCheck {
	out := SingletonCheck{Valid: true, Violations: []SingletonViolation{}}
	// Group by normalized name so "Sol Ring" and "sol ring" merge.
	type rolledUp struct {
		display string
		count   int
	}
	totals := map[string]*rolledUp{}
	for _, c := range deck {
		key := normCmdrName(c.Name)
		if r, ok := totals[key]; ok {
			r.count += c.Quantity
		} else {
			totals[key] = &rolledUp{display: c.Name, count: c.Quantity}
		}
	}
	for key, r := range totals {
		if r.count <= 1 {
			continue
		}
		if isBasicLand(r.display) {
			continue
		}
		if e := db.byName[key]; e != nil && hasAnyNumberOfText(e) {
			continue
		}
		out.Valid = false
		reason := "non-basic singleton violation"
		if db.byName[key] == nil {
			reason = "non-basic singleton violation (card not in oracle — verify name)"
		}
		out.Violations = append(out.Violations, SingletonViolation{
			CardName: r.display,
			Count:    r.count,
			Reason:   reason,
		})
	}
	sort.Slice(out.Violations, func(i, j int) bool {
		return out.Violations[i].CardName < out.Violations[j].CardName
	})
	return out
}

// computeBracketShape returns the bracket-shape advisory. Land count
// is computed from the oracle type_line — any card whose type_line
// contains "land" counts. Basic land count uses isBasicLand directly
// against the name (no oracle lookup needed).
func computeBracketShape(deck []deckCard, db *oracleCmdrDB, bracket int) *BracketShapeCheck {
	out := &BracketShapeCheck{
		Bracket:     bracket,
		BracketName: bracketNames[bracket],
		LandRange:   bracketLandRanges[bracket],
		Warnings:    []string{},
	}
	for _, c := range deck {
		if isBasicLand(c.Name) {
			out.BasicLandCount += c.Quantity
			out.LandCount += c.Quantity
			continue
		}
		entry := db.get(c.Name)
		if entry == nil {
			continue
		}
		tl := strings.ToLower(entry.TypeLine)
		if strings.Contains(tl, "land") {
			out.LandCount += c.Quantity
		}
	}
	switch {
	case out.LandCount < out.LandRange.Min:
		out.Shape = "below_range"
		out.Warnings = append(out.Warnings,
			fmt.Sprintf("land count %d is BELOW the typical %d-%d range for %s — deck may be miscategorized or land-starved",
				out.LandCount, out.LandRange.Min, out.LandRange.Max, out.BracketName))
	case out.LandCount > out.LandRange.Max:
		out.Shape = "above_range"
		out.Warnings = append(out.Warnings,
			fmt.Sprintf("land count %d is ABOVE the typical %d-%d range for %s — deck may be miscategorized or over-landed",
				out.LandCount, out.LandRange.Min, out.LandRange.Max, out.BracketName))
	default:
		out.Shape = "in_range"
	}
	return out
}

func writeDeckConstructionReport(rep *DeckConstructionReport, outPath string) error {
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
