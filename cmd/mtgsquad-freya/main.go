// mtgsquad-freya -- Automatic combo and synergy detector for MTG decklists.
//
// Freya reads a decklist, resolves each card's oracle text from the
// Scryfall oracle-cards.json dump, classifies each card's effects as
// PRODUCES/CONSUMES/TRIGGERS, builds a resource graph, finds cycles
// (combo loops), and reports combos, finishers, and synergies.
//
// Usage:
//
//	go run ./cmd/mtgsquad-freya/ --deck data/decks/benched/ragost_artifact_engine_b4_deft_gastronaut.txt
//	go run ./cmd/mtgsquad-freya/ --all-decks data/decks/benched/ --format markdown
//	go run ./cmd/mtgsquad-freya/ --deck my_deck.txt --format json
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// oracleEntry mirrors the subset of Scryfall oracle-cards.json we need.
type oracleEntry struct {
	Name          string   `json:"name"`
	OracleText    string   `json:"oracle_text"`
	TypeLine      string   `json:"type_line"`
	Layout        string   `json:"layout"`
	ManaCost      string   `json:"mana_cost"`
	CMC           float64  `json:"cmc"`
	Power         string   `json:"power"`
	Toughness     string   `json:"toughness"`
	Keywords      []string `json:"keywords"`
	ColorIdentity []string `json:"color_identity"`
	CardFaces     []struct {
		Name       string `json:"name"`
		OracleText string `json:"oracle_text"`
		TypeLine   string `json:"type_line"`
		ManaCost   string `json:"mana_cost"`
	} `json:"card_faces"`
}

// oracleDB maps normalized card names to oracle entries.
type oracleDB struct {
	byName map[string]*oracleEntry
}

func main() {
	var deckPath string
	var deckDir string
	var format string

	flag.StringVar(&deckPath, "deck", "", "path to decklist file")
	flag.StringVar(&deckDir, "all-decks", "", "analyze all decks in directory")
	flag.StringVar(&format, "format", "text", "output format: text, markdown, json")
	flag.Parse()

	if deckPath == "" && deckDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: mtgsquad-freya --deck <path> | --all-decks <dir>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load oracle cards.
	log.Println("mtgsquad-freya starting")
	log.Println("loading oracle corpus from data/rules/oracle-cards.json ...")
	t0 := time.Now()
	oracle, err := loadOracle("data/rules/oracle-cards.json")
	if err != nil {
		log.Fatalf("load oracle: %v", err)
	}
	log.Printf("  %d cards in %s", len(oracle.byName), time.Since(t0))

	// Load mechanic reference database.
	log.Println("building mechanic reference DB ...")
	t1 := time.Now()
	mechDB, err := BuildMechanicDB("data/rules/oracle-cards.json")
	if err != nil {
		log.Fatalf("build mechanic DB: %v", err)
	}
	log.Printf("  mechanic DB built in %s", time.Since(t1))
	mechDB.LogStats()

	if deckPath != "" {
		// Single deck mode.
		report, err := analyzeDeckFile(deckPath, oracle, mechDB)
		if err != nil {
			log.Fatalf("analyze deck: %v", err)
		}
		PrintReport(os.Stdout, report, format)
		// Auto-save to freya/ subfolder alongside the deck file.
		saveFreyaData(deckPath, report)
	} else {
		// All decks mode.
		files, err := listDeckFiles(deckDir)
		if err != nil {
			log.Fatalf("list decks: %v", err)
		}
		if len(files) == 0 {
			log.Fatalf("no .txt deck files found in %s", deckDir)
		}
		log.Printf("found %d deck files in %s", len(files), deckDir)

		var reports []*FreyaReport
		for _, f := range files {
			report, err := analyzeDeckFile(f, oracle, mechDB)
			if err != nil {
				log.Printf("  SKIP %s: %v", filepath.Base(f), err)
				continue
			}
			reports = append(reports, report)
			if format != "json" {
				PrintReport(os.Stdout, report, format)
			}
			// Auto-save to freya/ subfolder.
			saveFreyaData(f, report)
		}

		if format == "json" {
			// Print all reports as a JSON array.
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(reports)
		} else {
			PrintAllDecksSummary(os.Stdout, reports)
		}
	}
}

// ---------------------------------------------------------------------------
// Core pipeline
// ---------------------------------------------------------------------------

func analyzeDeckFile(path string, oracle *oracleDB, mechDB *MechanicDB) (*FreyaReport, error) {
	// Parse the decklist — two passes:
	// 1. Unique cards for synergy/combo detection (no basics, deduplicated)
	// 2. Full card list with quantities for mana curve and land counting
	cards, commander, err := parseDeckList(path)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cardQtys, err := parseDeckListWithQuantities(path)
	if err != nil {
		cardQtys = nil // non-fatal
	}

	deckName := filepath.Base(path)
	deckName = strings.TrimSuffix(deckName, filepath.Ext(deckName))

	// Classify each unique card for synergy detection.
	var profiles []CardProfile
	resolved := 0
	for _, cardName := range cards {
		entry := oracle.lookup(cardName)
		if entry == nil {
			continue
		}
		resolved++

		oracleText := entry.OracleText
		typeLine := entry.TypeLine
		manaCost := entry.ManaCost
		cmc := int(entry.CMC)
		if oracleText == "" && len(entry.CardFaces) > 0 {
			oracleText = entry.CardFaces[0].OracleText
			typeLine = entry.CardFaces[0].TypeLine
			manaCost = entry.CardFaces[0].ManaCost
		}

		profile := ClassifyCard(entry.Name, oracleText, typeLine, manaCost, cmc, entry.Power)
		profiles = append(profiles, profile)
	}

	// Build quantity-aware profiles for mana curve + land counting.
	// This includes basics and respects quantities.
	basicLandProfiles := map[string]CardProfile{
		"plains":                {Name: "Plains", IsLand: true, LandColors: []string{"W"}},
		"island":                {Name: "Island", IsLand: true, LandColors: []string{"U"}},
		"swamp":                 {Name: "Swamp", IsLand: true, LandColors: []string{"B"}},
		"mountain":              {Name: "Mountain", IsLand: true, LandColors: []string{"R"}},
		"forest":                {Name: "Forest", IsLand: true, LandColors: []string{"G"}},
		"snow-covered plains":   {Name: "Snow-Covered Plains", IsLand: true, LandColors: []string{"W"}},
		"snow-covered island":   {Name: "Snow-Covered Island", IsLand: true, LandColors: []string{"U"}},
		"snow-covered swamp":    {Name: "Snow-Covered Swamp", IsLand: true, LandColors: []string{"B"}},
		"snow-covered mountain": {Name: "Snow-Covered Mountain", IsLand: true, LandColors: []string{"R"}},
		"snow-covered forest":   {Name: "Snow-Covered Forest", IsLand: true, LandColors: []string{"G"}},
		"wastes":                {Name: "Wastes", IsLand: true, LandColors: []string{}},
	}
	var qtyProfiles []CardProfileQty
	if cardQtys != nil {
		for name, qty := range cardQtys {
			// Check basic land shortcut first
			if bp, ok := basicLandProfiles[strings.ToLower(strings.TrimSpace(name))]; ok {
				qtyProfiles = append(qtyProfiles, CardProfileQty{Profile: bp, Qty: qty})
				continue
			}
			entry := oracle.lookup(name)
			if entry == nil {
				log.Printf("    [qty-pass] UNRESOLVED: %q (qty=%d)", name, qty)
				qtyProfiles = append(qtyProfiles, CardProfileQty{
					Profile: CardProfile{Name: name},
					Qty:     qty,
				})
				continue
			}
			oracleText := entry.OracleText
			typeLine := entry.TypeLine
			manaCost := entry.ManaCost
			cmc := int(entry.CMC)
			if oracleText == "" && len(entry.CardFaces) > 0 {
				oracleText = entry.CardFaces[0].OracleText
				typeLine = entry.CardFaces[0].TypeLine
				manaCost = entry.CardFaces[0].ManaCost
			}
			p := ClassifyCard(entry.Name, oracleText, typeLine, manaCost, cmc, entry.Power)
			// Ensure land classification uses the top-level type line (not DFC face)
			if !p.IsLand && strings.Contains(strings.ToLower(entry.TypeLine), "land") {
				p.IsLand = true
			}
			qtyProfiles = append(qtyProfiles, CardProfileQty{Profile: p, Qty: qty})
		}
	}

	log.Printf("  %s: %d cards parsed, %d resolved, %d unresolved, %d qtyEntries",
		filepath.Base(path), len(cards), resolved, len(cards)-resolved, len(qtyProfiles))

	// Run analysis — pass both unique profiles (synergy) and qty profiles (curve).
	report := AnalyzeDeck(profiles, deckName, path, commander)

	// Filter combo potential notes by commander color identity.
	if commander != "" {
		cmdrEntry := oracle.lookup(commander)
		if cmdrEntry != nil && len(cmdrEntry.ColorIdentity) > 0 {
			cmdrColors := map[string]bool{}
			for _, c := range cmdrEntry.ColorIdentity {
				cmdrColors[c] = true
			}
			var filtered []string
			for _, note := range report.ComboNotes {
				// Extract the missing card name and check its color identity
				// Format: "Name: have X, missing Y for type"
				parts := strings.SplitN(note, "missing ", 2)
				if len(parts) < 2 {
					filtered = append(filtered, note)
					continue
				}
				missingPart := strings.SplitN(parts[1], " for ", 2)
				missingCards := strings.Split(missingPart[0], " + ")
				legal := true
				for _, mc := range missingCards {
					mcEntry := oracle.lookup(strings.TrimSpace(mc))
					if mcEntry == nil {
						continue
					}
					for _, ci := range mcEntry.ColorIdentity {
						if !cmdrColors[ci] {
							legal = false
							break
						}
					}
					if !legal {
						break
					}
				}
				if legal {
					filtered = append(filtered, note)
				} else {
					filtered = append(filtered, note+" [ILLEGAL — outside color identity]")
				}
			}
			report.ComboNotes = filtered
		}
	}

	// Run mechanic-based synergy detection using the full oracle corpus DB.
	mechSynergies := FindMechanicSynergies(profiles, cardQtys, mechDB)
	report.Synergies = append(report.Synergies, mechSynergies...)
	report.Synergies = deduplicateCombos(report.Synergies)

	// Override mana curve and land count with quantity-aware data.
	if len(qtyProfiles) > 0 {
		report.ManaCurve = [8]int{}
		report.LandCount = 0
		report.NonlandCount = 0
		report.ColorDemand = map[string]int{"W": 0, "U": 0, "B": 0, "R": 0, "G": 0}
		report.ColorSupply = map[string]int{"W": 0, "U": 0, "B": 0, "R": 0, "G": 0}
		totalCMC := 0
		for _, qp := range qtyProfiles {
			for q := 0; q < qp.Qty; q++ {
				if qp.Profile.IsLand {
					report.LandCount++
					for _, c := range qp.Profile.LandColors {
						report.ColorSupply[c]++
					}
				} else {
					report.NonlandCount++
					cmc := qp.Profile.CMC
					if cmc >= 7 {
						report.ManaCurve[7]++
					} else if cmc >= 0 {
						report.ManaCurve[cmc]++
					}
					totalCMC += cmc
					countManaCostPips(qp.Profile.ManaCost, report.ColorDemand)
				}
			}
		}
		report.TotalCards = report.LandCount + report.NonlandCount
		if commander != "" && cardQtys != nil && cardQtys[commander] == 0 {
			report.TotalCards++
		}
		if report.NonlandCount > 0 {
			report.AvgCMC = float64(totalCMC) / float64(report.NonlandCount)
		}
		// Recalculate color mismatches
		report.ColorMismatch = nil
		totalDemand := 0
		totalSupply := 0
		for _, v := range report.ColorDemand {
			totalDemand += v
		}
		for _, v := range report.ColorSupply {
			totalSupply += v
		}
		if totalDemand > 0 && totalSupply > 0 {
			for _, color := range []string{"W", "U", "B", "R", "G"} {
				dPct := float64(report.ColorDemand[color]) / float64(totalDemand) * 100
				sPct := float64(report.ColorSupply[color]) / float64(totalSupply) * 100
				if dPct-sPct > 5 {
					report.ColorMismatch = append(report.ColorMismatch,
						fmt.Sprintf("%s underrepresented: %.0f%% demand vs %.0f%% supply", color, dPct, sPct))
				}
			}
		}

		// Phase 1 statistics module.
		report.Stats = ComputeDeckStatistics(qtyProfiles)

		// Phase 2 role tagging.
		report.Roles = ComputeRoleAnalysis(qtyProfiles, oracle)

		// Phase 3 archetype classification.
		report.Archetype = ClassifyArchetype(report, qtyProfiles, oracle)

		// Phase 4 win line mapping.
		report.WinLines = ComputeWinLines(report, qtyProfiles, oracle)

		// Phase 5 unified deck profile.
		report.Profile = BuildDeckProfile(report, oracle)

		// Legality validation (after TotalCards is finalized).
		report.Legality = CheckLegality(report, qtyProfiles, oracle)
	}

	return report, nil
}

// ---------------------------------------------------------------------------
// Oracle loading
// ---------------------------------------------------------------------------

func loadOracle(path string) (*oracleDB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []oracleEntry
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode oracle: %w", err)
	}

	db := &oracleDB{byName: make(map[string]*oracleEntry, len(entries))}
	for i := range entries {
		e := &entries[i]
		if e.Name == "" {
			continue
		}
		// Skip art_series, token, and other non-playable card layouts
		if e.Layout == "art_series" || e.Layout == "token" || e.Layout == "double_faced_token" || e.Layout == "emblem" {
			continue
		}
		norm := normalizeName(e.Name)
		// First entry wins (deduplicate reprints).
		if _, exists := db.byName[norm]; !exists {
			db.byName[norm] = e
		}
		// Also index by simple lowercase with curly-quote folding for fallback matching.
		lower := foldQuotes(strings.ToLower(strings.TrimSpace(e.Name)))
		if _, exists := db.byName[lower]; !exists {
			db.byName[lower] = e
		}
		// Also index each face of DFC/split cards.
		for _, face := range e.CardFaces {
			if face.Name == "" {
				continue
			}
			faceNorm := normalizeName(face.Name)
			if _, exists := db.byName[faceNorm]; !exists {
				faceCopy := &oracleEntry{
					Name:       face.Name,
					OracleText: face.OracleText,
					TypeLine:   face.TypeLine,
				}
				db.byName[faceNorm] = faceCopy
			}
		}
	}

	return db, nil
}

func (db *oracleDB) lookup(name string) *oracleEntry {
	if db == nil {
		return nil
	}
	norm := normalizeName(name)
	if e, ok := db.byName[norm]; ok {
		return e
	}
	// Fallback: simple lowercase match with quote folding
	lower := foldQuotes(strings.ToLower(strings.TrimSpace(name)))
	if e, ok := db.byName[lower]; ok {
		return e
	}
	// Try splitting DFC name "A // B" and looking up each face.
	if strings.Contains(name, " // ") {
		for _, face := range strings.Split(name, " // ") {
			faceNorm := normalizeName(strings.TrimSpace(face))
			if e, ok := db.byName[faceNorm]; ok {
				return e
			}
			faceLower := strings.ToLower(strings.TrimSpace(face))
			if e, ok := db.byName[faceLower]; ok {
				return e
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Decklist parsing (simplified -- we only need card names)
// ---------------------------------------------------------------------------

var deckLineRE = regexp.MustCompile(`^\s*(\d+)\s*[xX]?\s+(.+?)\s*$`)
var commanderLineRE = regexp.MustCompile(`(?i)^\s*COMMANDER\s*:\s*(.+?)\s*$`)
var partnerLineRE = regexp.MustCompile(`(?i)^\s*PARTNER\s*:\s*(.+?)\s*$`)

// parseDeckList reads a Moxfield-format decklist and returns card names +
// the commander name. Basic lands are skipped since they add noise to
// the combo/synergy analysis.
func parseDeckList(path string) (cards []string, commander string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	basicLands := map[string]bool{
		"plains": true, "island": true, "swamp": true,
		"mountain": true, "forest": true,
		"snow-covered plains": true, "snow-covered island": true,
		"snow-covered swamp": true, "snow-covered mountain": true,
		"snow-covered forest": true,
		"wastes":              true,
	}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
			continue
		}

		// COMMANDER: <name>
		if m := commanderLineRE.FindStringSubmatch(raw); m != nil {
			commander = strings.TrimSpace(m[1])
			continue
		}
		// PARTNER: <name> (treat as part of the deck too)
		if m := partnerLineRE.FindStringSubmatch(raw); m != nil {
			cards = append(cards, strings.TrimSpace(m[1]))
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

		// Skip basic lands.
		if basicLands[strings.ToLower(name)] {
			continue
		}

		// Only add the card once (regardless of qty) for combo analysis.
		cards = append(cards, name)
	}
	if err := sc.Err(); err != nil {
		return nil, "", err
	}

	// If commander was specified, ensure it's in the card list.
	if commander != "" {
		found := false
		for _, c := range cards {
			if normalizeName(c) == normalizeName(commander) {
				found = true
				break
			}
		}
		if !found {
			cards = append(cards, commander)
		}
	}

	return cards, commander, nil
}

// CardProfileQty pairs a profile with its deck quantity.
type CardProfileQty struct {
	Profile CardProfile
	Qty     int
}

// parseDeckListWithQuantities returns a map of card name → quantity,
// INCLUDING basic lands. Used for mana curve and land counting.
func parseDeckListWithQuantities(path string) (map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := map[string]int{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
			continue
		}
		if commanderLineRE.MatchString(raw) || partnerLineRE.MatchString(raw) {
			continue
		}
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
		result[name] += qty
	}
	return result, sc.Err()
}

// ---------------------------------------------------------------------------
// Auto-save Freya data alongside the deck
// ---------------------------------------------------------------------------

func saveFreyaData(deckPath string, report *FreyaReport) {
	dir := filepath.Dir(deckPath)
	freyaDir := filepath.Join(dir, "freya")
	os.MkdirAll(freyaDir, 0755)

	base := filepath.Base(deckPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	// Human-readable report.
	mdPath := filepath.Join(freyaDir, base+"_freya.md")
	f, err := os.Create(mdPath)
	if err != nil {
		log.Printf("  [freya] failed to save %s: %v", mdPath, err)
		return
	}
	PrintReport(f, report, "markdown")
	f.Close()
	log.Printf("  [freya] saved %s", mdPath)

	// Machine-consumable strategy JSON for the hat/MCTS evaluator.
	stratPath := filepath.Join(freyaDir, base+".strategy.json")
	saveStrategyJSON(stratPath, report)
}

// strategyJSON is the compact machine-consumable format read by
// hat.LoadStrategyFromFreya. Contains only what the evaluator needs.
type strategyJSON struct {
	Archetype       string            `json:"archetype"`
	Bracket         int               `json:"bracket"`
	GameplanSummary string            `json:"gameplan_summary"`
	WinLines        []strategyWinLine `json:"win_lines"`
	ValueEngineKeys []string          `json:"value_engine_keys,omitempty"`
	TutorTargets    []string          `json:"tutor_targets,omitempty"`
	Weights         *jsonEvalWeights  `json:"eval_weights,omitempty"`
}

type strategyWinLine struct {
	Pieces     []string         `json:"pieces"`
	Type       string           `json:"type"`
	TutorPaths []jsonTutorChain `json:"tutor_paths,omitempty"`
}

func saveStrategyJSON(path string, report *FreyaReport) {
	sj := strategyJSON{}

	if report.Profile != nil {
		sj.Archetype = strings.ToLower(report.Profile.PrimaryArchetype)
		sj.Bracket = report.Profile.Bracket
		sj.GameplanSummary = report.Profile.GameplanSummary
	} else if report.Archetype != nil {
		sj.Archetype = strings.ToLower(report.Archetype.Primary)
		sj.Bracket = report.Archetype.Bracket
	}

	if report.WinLines != nil {
		for _, wl := range report.WinLines.WinLines {
			swl := strategyWinLine{
				Pieces: wl.Pieces,
				Type:   wl.Type,
			}
			seen := map[string]bool{}
			for _, tp := range wl.TutorPaths {
				key := tp.Tutor + "→" + tp.Finds
				if seen[key] {
					continue
				}
				seen[key] = true
				swl.TutorPaths = append(swl.TutorPaths, jsonTutorChain{
					Tutor:    tp.Tutor,
					Finds:    tp.Finds,
					Delivery: tp.Delivery,
				})
			}
			sj.WinLines = append(sj.WinLines, swl)
		}
	}

	// Value engine keys from value chains.
	seenKeys := map[string]bool{}
	for _, vc := range report.ValueChains {
		for _, step := range vc.Steps {
			for _, card := range step.Cards {
				if !seenKeys[card] {
					seenKeys[card] = true
					sj.ValueEngineKeys = append(sj.ValueEngineKeys, card)
				}
			}
		}
		for _, card := range vc.BridgeCards {
			if !seenKeys[card] {
				seenKeys[card] = true
				sj.ValueEngineKeys = append(sj.ValueEngineKeys, card)
			}
		}
	}

	// Tutor targets: all win line pieces (ordered by win line priority).
	seenTargets := map[string]bool{}
	for _, wl := range sj.WinLines {
		for _, p := range wl.Pieces {
			if !seenTargets[p] {
				seenTargets[p] = true
				sj.TutorTargets = append(sj.TutorTargets, p)
			}
		}
	}

	// Compute eval weights from deck profile.
	if report.Profile != nil {
		sj.Weights = ComputeEvalWeights(report.Profile, report)
	}

	f, err := os.Create(path)
	if err != nil {
		log.Printf("  [freya] failed to save %s: %v", path, err)
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(sj)
	log.Printf("  [freya] saved %s", path)
}

// ---------------------------------------------------------------------------
// File listing
// ---------------------------------------------------------------------------

func listDeckFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "freya" || d.Name() == "benched" || d.Name() == "test" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".txt") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// ---------------------------------------------------------------------------
// Name normalization (mirrors deckparser.normalizeName)
// ---------------------------------------------------------------------------

// foldQuotes replaces Unicode curly quotes/dashes with ASCII equivalents.
func foldQuotes(s string) string {
	r := strings.NewReplacer(
		"\u2018", "'", "\u2019", "'", "\u2032", "'",
		"\u201c", "\"", "\u201d", "\"",
		"\u2013", "-", "\u2014", "-",
	)
	return r.Replace(s)
}

func normalizeName(name string) string {
	out := make([]rune, 0, len(name))
	prevSpace := false
	for _, r := range name {
		// Normalize curly quotes/apostrophes to ASCII equivalents
		switch r {
		case '\u2018', '\u2019', '\u2032': // left/right single quote, prime
			r = '\''
		case '\u201c', '\u201d': // left/right double quote
			r = '"'
		case '\u2013', '\u2014': // en-dash, em-dash
			r = '-'
		}
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
	case '\u00e1', '\u00e0', '\u00e2', '\u00e4', '\u00e3', '\u00e5', '\u0101',
		'\u00c1', '\u00c0', '\u00c2', '\u00c4', '\u00c3', '\u00c5', '\u0100':
		return 'a'
	case '\u00e7', '\u00c7':
		return 'c'
	case '\u00e9', '\u00e8', '\u00ea', '\u00eb', '\u0113',
		'\u00c9', '\u00c8', '\u00ca', '\u00cb', '\u0112':
		return 'e'
	case '\u00ed', '\u00ec', '\u00ee', '\u00ef', '\u012b',
		'\u00cd', '\u00cc', '\u00ce', '\u00cf', '\u012a':
		return 'i'
	case '\u00f1', '\u00d1':
		return 'n'
	case '\u00f3', '\u00f2', '\u00f4', '\u00f6', '\u00f5', '\u00f8', '\u014d',
		'\u00d3', '\u00d2', '\u00d4', '\u00d6', '\u00d5', '\u00d8', '\u014c':
		return 'o'
	case '\u00fa', '\u00f9', '\u00fb', '\u00fc', '\u016b',
		'\u00da', '\u00d9', '\u00db', '\u00dc', '\u016a':
		return 'u'
	case '\u00fd', '\u00ff', '\u00dd', '\u0178':
		return 'y'
	case '\u00df':
		return 's'
	case '\u00e6', '\u00c6':
		return 'e'
	}
	return r
}
