// Command gen-handlers auto-generates per_card handler stubs for
// commanders in the deck pool that don't yet have handlers. It reads
// commander names from data/decks/moxfield/, checks which are already
// registered in registry.go, looks up their AST in ast_dataset.jsonl,
// classifies abilities, and emits .go handler files.
//
// Usage:
//
//	go run ./cmd/gen-handlers
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

// ---------------------------------------------------------------------------
// AST types (mirrors the Python parser output)
// ---------------------------------------------------------------------------

type ASTDataset struct {
	Name       string   `json:"name"`
	OracleText string   `json:"oracle_text"`
	TypeLine   string   `json:"type_line"`
	ManaCost   string   `json:"mana_cost"`
	CMC        float64  `json:"cmc"`
	Colors     []string `json:"colors"`
	AST        CardAST  `json:"ast"`
}

type CardAST struct {
	ASTType   string    `json:"__ast_type__"`
	Name      string    `json:"name"`
	Abilities []Ability `json:"abilities"`
}

type Ability struct {
	ASTType  string          `json:"__ast_type__"`
	Name     string          `json:"name"` // for Keyword
	Raw      string          `json:"raw"`
	Trigger  json.RawMessage `json:"trigger"`
	Effect   json.RawMessage `json:"effect"`
	Args     json.RawMessage `json:"args"`
	Costs    json.RawMessage `json:"costs"`
	Modifier json.RawMessage `json:"modification"`
}

type TriggerInfo struct {
	Event string `json:"event"`
}

type ModificationInfo struct {
	Kind string            `json:"kind"`
	Args []json.RawMessage `json:"args"`
}

// UnmarshalJSON lets us handle the polymorphic ability list.
func (a *Ability) UnmarshalJSON(data []byte) error {
	type plain Ability
	if err := json.Unmarshal(data, (*plain)(a)); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Classification categories
// ---------------------------------------------------------------------------

type Category string

const (
	CatSimpleETB     Category = "SIMPLE_ETB"
	CatSimpleTrigger Category = "SIMPLE_TRIGGER"
	CatSimpleActive  Category = "SIMPLE_ACTIVATED"
	CatStatic        Category = "STATIC"
	CatComplex       Category = "COMPLEX"
	CatKeywordOnly   Category = "KEYWORD_ONLY"
	CatNoAST         Category = "NO_AST"
)

// ClassifiedCard is the result of analyzing a commander.
type ClassifiedCard struct {
	Slug           string
	CardName       string
	OracleText     string
	TypeLine       string
	Category       Category
	AbilityTypes   []string // the __ast_type__ of each ability
	TriggerEvents  []string // event names from Triggered abilities
	EffectSummary  string   // short description for code comment
	HasActivated   bool
	HasTriggered   bool
	HasETB         bool
	HasStatic      bool
	HasKeyword     bool
	Keywords       []string
	NonKWAbilities int
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		fmt.Fprintln(os.Stderr, "ERROR: could not find repo root (looking for go.mod)")
		os.Exit(1)
	}

	fmt.Printf("Repo root: %s\n", repoRoot)

	// 0. Clean up old generated files so the tool is idempotent.
	perCardDir := filepath.Join(repoRoot, "internal", "gameengine", "per_card")
	cleanOldGenFiles(perCardDir)

	// 1. Discover commander slugs from deck filenames.
	slugs := discoverCommanders(filepath.Join(repoRoot, "data", "decks", "moxfield"))
	fmt.Printf("Unique commander slugs from deck pool: %d\n", len(slugs))

	// 2. Parse registry.go for already-registered names (excluding gen_ files).
	registered := parseRegistered(perCardDir)
	fmt.Printf("Already registered handler names: %d\n", len(registered))

	// 3. Load AST dataset.
	astDB := loadAST(filepath.Join(repoRoot, "data", "rules", "ast_dataset.jsonl"))
	fmt.Printf("Cards in AST dataset: %d\n", len(astDB))

	// 4. Build normalized + tight AST lookups.
	normAST := buildNormalizedAST(astDB)
	tightAST := buildTightAST(astDB)

	// 5. Filter to unhandled commanders only.
	var unhandled []string
	var alreadyHandled []string
	for _, slug := range slugs {
		normSlug := normalizeForMatch(slugToSpaces(slug))
		if registered[normSlug] {
			alreadyHandled = append(alreadyHandled, slug)
			continue
		}
		// Also check the actual card name if we can resolve it
		cardName := resolveCardName(slug, normAST, tightAST)
		if cardName != "" {
			normCard := normalizeForMatch(cardName)
			if registered[normCard] {
				alreadyHandled = append(alreadyHandled, slug)
				continue
			}
		}
		unhandled = append(unhandled, slug)
	}
	fmt.Printf("Already handled: %d\n", len(alreadyHandled))
	fmt.Printf("Need handlers: %d\n", len(unhandled))

	// 6. Classify each unhandled commander.
	var classified []ClassifiedCard
	var noAST []string
	for _, slug := range unhandled {
		cardName := resolveCardName(slug, normAST, tightAST)
		if cardName == "" {
			noAST = append(noAST, slug)
			classified = append(classified, ClassifiedCard{
				Slug:     slug,
				CardName: slugToDisplay(slug),
				Category: CatNoAST,
			})
			continue
		}
		ast := normAST[normalizeForMatch(cardName)]
		cc := classifyCard(slug, ast)
		classified = append(classified, cc)
	}

	// 7. Count by category.
	counts := map[Category]int{}
	for _, cc := range classified {
		counts[cc.Category]++
	}
	fmt.Println("\n=== Classification Summary ===")
	for _, cat := range []Category{CatSimpleETB, CatSimpleTrigger, CatSimpleActive, CatStatic, CatKeywordOnly, CatComplex, CatNoAST} {
		fmt.Printf("  %-20s %d\n", cat, counts[cat])
	}

	// 8. Generate handler files.
	outDir := filepath.Join(repoRoot, "internal", "gameengine", "per_card")
	generated := generateHandlers(classified, outDir)

	// 9. Generate batch_generated.go
	generateBatchFile(generated, outDir)

	// 10. Patch registry.go to call registerGeneratedHandlers
	patchRegistry(filepath.Join(outDir, "registry.go"))

	// 11. Print report.
	printReport(classified, generated, noAST)
}

// ---------------------------------------------------------------------------
// Step 1: Discover commander slugs from filenames
// ---------------------------------------------------------------------------

var deckFileRe = regexp.MustCompile(`^(.+?)_b\d+_`)

func discoverCommanders(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: cannot read deck dir %s: %v\n", dir, err)
		return nil
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		m := deckFileRe.FindStringSubmatch(e.Name())
		if len(m) > 1 {
			seen[m[1]] = true
		}
	}
	slugs := make([]string, 0, len(seen))
	for s := range seen {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	return slugs
}

// ---------------------------------------------------------------------------
// Step 2: Parse registry.go for registered card names
// ---------------------------------------------------------------------------

func parseRegistered(dir string) map[string]bool {
	registered := map[string]bool{}

	// Walk all .go files in per_card dir for OnETB/OnCast/OnResolve/OnActivated/OnTrigger calls
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: cannot read per_card dir: %v\n", err)
		return registered
	}

	// Patterns to extract card names from registration calls.
	// The (?:[^"\\]|\\.)+ branch tolerates Go-source escapes like
	// `\"` inside the string literal so card names that contain quotes
	// (e.g. `Henzie \"Toolbox\" Torre`) are captured in full.
	regPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\.OnETB\("((?:[^"\\]|\\.)+)"`),
		regexp.MustCompile(`\.OnCast\("((?:[^"\\]|\\.)+)"`),
		regexp.MustCompile(`\.OnResolve\("((?:[^"\\]|\\.)+)"`),
		regexp.MustCompile(`\.OnActivated\("((?:[^"\\]|\\.)+)"`),
		regexp.MustCompile(`\.OnTrigger\("((?:[^"\\]|\\.)+)"`),
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		// Skip the batch registration file outright.
		if e.Name() == "batch_generated.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		// For gen_*.go files, only treat them as authoritative registrations
		// when they have been hand-edited (no "Auto-generated" marker). Pure
		// auto-gen files will be regenerated this run, so they shouldn't
		// suppress themselves from the unhandled list.
		if strings.HasPrefix(e.Name(), "gen_") && strings.Contains(content, "Auto-generated") {
			continue
		}
		for _, pat := range regPatterns {
			matches := pat.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				if len(m) > 1 {
					registered[normalizeForMatch(m[1])] = true
				}
			}
		}
	}
	return registered
}

// ---------------------------------------------------------------------------
// Step 3: Load AST dataset
// ---------------------------------------------------------------------------

func loadAST(path string) []ASTDataset {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot open AST dataset: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	var cards []ASTDataset
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		var card ASTDataset
		if err := json.Unmarshal(scanner.Bytes(), &card); err != nil {
			continue
		}
		cards = append(cards, card)
	}
	return cards
}

// ---------------------------------------------------------------------------
// Step 4: Build normalized AST lookup
// ---------------------------------------------------------------------------

func buildNormalizedAST(cards []ASTDataset) map[string]ASTDataset {
	m := make(map[string]ASTDataset, len(cards))
	for _, c := range cards {
		key := normalizeForMatch(c.Name)
		if _, exists := m[key]; !exists {
			m[key] = c
		}
	}
	return m
}

// buildTightAST builds an alphanumeric-only index (no spaces, no punctuation)
// keyed off card names. Lets us resolve slugs whose underscores don't line
// up with the original spaces — e.g. "niv_mizzet_parun" → "Niv-Mizzet, Parun"
// or "abdel_adrian_gorion_s_ward" → "Abdel Adrian, Gorion's Ward". For DFCs
// we also index just the front face.
func buildTightAST(cards []ASTDataset) map[string]ASTDataset {
	m := make(map[string]ASTDataset, len(cards)*2)
	for _, c := range cards {
		key := tightenForMatch(c.Name)
		if _, exists := m[key]; !exists {
			m[key] = c
		}
		// DFC front face fallback: index just the part before "//".
		if idx := strings.Index(c.Name, "//"); idx > 0 {
			front := strings.TrimSpace(c.Name[:idx])
			fkey := tightenForMatch(front)
			if _, exists := m[fkey]; !exists {
				m[fkey] = c
			}
		}
	}
	return m
}

// tightenForMatch strips every non-alphanumeric character and lowercases the
// rest. Maps both "Niv-Mizzet, Parun" and "niv_mizzet_parun" to
// "nivmizzetparun".
func tightenForMatch(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Name normalization and resolution
// ---------------------------------------------------------------------------

func normalizeForMatch(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	b.Grow(len(name))
	prevSpace := false
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		} else if r == ' ' || r == '_' {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		}
		// Drop all other punctuation
	}
	return strings.TrimSpace(b.String())
}

func slugToSpaces(slug string) string {
	return strings.ReplaceAll(slug, "_", " ")
}

func slugToDisplay(slug string) string {
	parts := strings.Split(slug, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func resolveCardName(slug string, normAST map[string]ASTDataset, tightAST map[string]ASTDataset) string {
	norm := normalizeForMatch(slugToSpaces(slug))
	if card, ok := normAST[norm]; ok {
		return card.Name
	}
	// Tight (alphanumeric-only) index handles hyphen/apostrophe mismatches
	// like "niv_mizzet_parun" → "Niv-Mizzet, Parun".
	tight := tightenForMatch(slug)
	if card, ok := tightAST[tight]; ok {
		return card.Name
	}
	// Try DFC: slug might be "front_back" where card is "Front // Back"
	// Also try just the front face. Progressively try longer prefixes.
	parts := strings.SplitN(slug, "_", -1)
	for i := len(parts) - 1; i > 0; i-- {
		prefix := normalizeForMatch(strings.Join(parts[:i], " "))
		if card, ok := normAST[prefix]; ok {
			return card.Name
		}
		tightPrefix := tightenForMatch(strings.Join(parts[:i], ""))
		if card, ok := tightAST[tightPrefix]; ok {
			return card.Name
		}
	}
	// Last resort: fuzzy regex fallback for slugs where unicode characters
	// in the original card name became "_" separators (e.g. Altaïr →
	// "alta_r_..."). Each "_" stands in for >=1 dropped letters. Match the
	// slug against tightened card names with that allowance, anchored at
	// both ends, and accept only when exactly one card matches.
	if name := fuzzyResolveSlug(slug, tightAST); name != "" {
		return name
	}
	return ""
}

func fuzzyResolveSlug(slug string, tightAST map[string]ASTDataset) string {
	parts := strings.Split(slug, "_")
	// Drop empty parts (leading/trailing underscores or doubled separators).
	pruned := parts[:0]
	for _, p := range parts {
		if p != "" {
			pruned = append(pruned, p)
		}
	}
	if len(pruned) < 2 {
		return ""
	}
	// Underscores in the slug correspond to either word boundaries (zero
	// extra chars in the tightened name) or stripped unicode characters
	// (one or more chars). Use ".*?" so both cases match. False positives
	// are filtered by the unique-match requirement below.
	pattern := "^"
	if len(slug) > 0 && slug[0] == '_' {
		pattern += ".*?"
	}
	for i, p := range pruned {
		pattern += regexp.QuoteMeta(p)
		if i < len(pruned)-1 {
			pattern += ".*?"
		}
	}
	if len(slug) > 0 && slug[len(slug)-1] == '_' {
		pattern += ".*?"
	}
	pattern += "$"
	if name := uniqueRegexMatch(pattern, tightAST); name != "" {
		return name
	}
	// Loosen the front anchor: the slug may have lost a leading unicode
	// character (e.g. "owyn_shieldmaiden" ← "Éowyn, Shieldmaiden").
	if loosened := strings.Replace(pattern, "^", "^.*?", 1); loosened != pattern {
		if name := uniqueRegexMatch(loosened, tightAST); name != "" {
			return name
		}
	}
	return ""
}

func uniqueRegexMatch(pattern string, tightAST map[string]ASTDataset) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	var match string
	matches := 0
	for tight, card := range tightAST {
		if re.MatchString(tight) {
			matches++
			match = card.Name
			if matches > 1 {
				return ""
			}
		}
	}
	if matches == 1 {
		return match
	}
	return ""
}

// ---------------------------------------------------------------------------
// Step 6: Classify abilities
// ---------------------------------------------------------------------------

// Known simple keywords that the engine handles natively
var simpleKeywords = map[string]bool{
	"flying": true, "haste": true, "trample": true, "lifelink": true,
	"deathtouch": true, "vigilance": true, "menace": true, "reach": true,
	"first strike": true, "double strike": true, "hexproof": true,
	"indestructible": true, "flash": true, "defender": true, "ward": true,
	"partner": true, "partner with": true, "friends forever": true,
	"choose a background": true, "eminence": true, "companion": true,
	"prowess": true, "intimidate": true, "fear": true, "shadow": true,
	"shroud": true, "protection": true, "cascade": true, "undying": true,
	"persist": true, "infect": true, "wither": true, "flanking": true,
	"exalted": true, "skulk": true, "convoke": true,
}

func isSimpleKeyword(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if simpleKeywords[n] {
		return true
	}
	// "enchant creature", "protection from X", etc.
	for k := range simpleKeywords {
		if strings.HasPrefix(n, k) {
			return true
		}
	}
	return false
}

// triggerEventMap maps AST trigger events to engine event names.
var triggerEventMap = map[string]string{
	"etb":                           "ETB",
	"self_etb":                      "ETB",
	"tribe_you_control_etb":         "permanent_etb",
	"creature_you_control_etb":      "permanent_etb",
	"permanent_you_control_etb":     "permanent_etb",
	"artifact_etb":                  "permanent_etb",
	"enchantment_etb":               "permanent_etb",
	"creature_dies":                 "creature_dies",
	"nontoken_creature_dies":        "creature_dies",
	"creature_you_control_dies":     "creature_dies",
	"token_creature_dies":           "creature_dies",
	"self_attacks":                  "creature_attacks",
	"creature_attacks":              "creature_attacks",
	"creature_you_control_attacks":  "creature_attacks",
	"self_deals_combat_damage":      "combat_damage_player",
	"deals_combat_damage_to_player": "combat_damage_player",
	"spell_cast":                    "spell_cast",
	"you_cast_spell":               "spell_cast",
	"opponent_casts_spell":         "spell_cast",
	"noncreature_spell_cast":       "noncreature_spell_cast",
	"instant_or_sorcery_cast":      "instant_or_sorcery_cast",
	"upkeep":                       "upkeep_controller",
	"beginning_of_upkeep":          "upkeep_controller",
	"your_upkeep":                  "upkeep_controller",
	"end_step":                     "end_step",
	"beginning_of_end_step":        "end_step",
	"your_end_step":                "end_step",
	"card_drawn":                   "card_drawn",
	"player_draws":                 "card_drawn",
	"gain_life":                    "life_gained",
	"life_gained":                  "life_gained",
	"lose_life":                    "life_lost",
	"discard":                      "card_discarded",
	"card_discarded":               "card_discarded",
	"land_etb":                     "permanent_etb",
	"token_created":                "token_created",
}

func classifyCard(slug string, ast ASTDataset) ClassifiedCard {
	cc := ClassifiedCard{
		Slug:       slug,
		CardName:   ast.Name,
		OracleText: ast.OracleText,
		TypeLine:   ast.TypeLine,
	}

	if len(ast.AST.Abilities) == 0 {
		cc.Category = CatKeywordOnly
		return cc
	}

	for _, ab := range ast.AST.Abilities {
		cc.AbilityTypes = append(cc.AbilityTypes, ab.ASTType)
		switch ab.ASTType {
		case "Keyword":
			cc.HasKeyword = true
			cc.Keywords = append(cc.Keywords, ab.Name)
			if !isSimpleKeyword(ab.Name) {
				cc.NonKWAbilities++
			}
		case "Triggered":
			cc.HasTriggered = true
			cc.NonKWAbilities++
			// Extract trigger event
			var trig TriggerInfo
			if ab.Trigger != nil {
				_ = json.Unmarshal(ab.Trigger, &trig)
			}
			if trig.Event != "" {
				cc.TriggerEvents = append(cc.TriggerEvents, trig.Event)
			}
			// Check if it's an ETB trigger
			if trig.Event == "etb" || trig.Event == "self_etb" {
				cc.HasETB = true
			}
		case "Activated":
			cc.HasActivated = true
			cc.NonKWAbilities++
		case "Static":
			cc.HasStatic = true
			cc.NonKWAbilities++
		default:
			cc.NonKWAbilities++
		}
	}

	// Classify based on ability profile
	if cc.NonKWAbilities == 0 {
		cc.Category = CatKeywordOnly
		return cc
	}

	// Count non-keyword abilities by type
	numTriggered := 0
	numActivated := 0
	numStatic := 0
	numETB := 0
	for _, ab := range ast.AST.Abilities {
		switch ab.ASTType {
		case "Triggered":
			numTriggered++
			var trig TriggerInfo
			if ab.Trigger != nil {
				_ = json.Unmarshal(ab.Trigger, &trig)
			}
			if trig.Event == "etb" || trig.Event == "self_etb" {
				numETB++
			}
		case "Activated":
			numActivated++
		case "Static":
			numStatic++
		}
	}

	// Simple ETB: has exactly 1 ETB trigger, maybe keywords + statics
	if numETB == 1 && numTriggered == 1 && numActivated == 0 {
		cc.Category = CatSimpleETB
		cc.EffectSummary = summarizeTriggeredEffect(ast.AST.Abilities)
		return cc
	}

	// Simple ETB: has 1 ETB trigger + maybe 1 static ability
	if numETB == 1 && numTriggered <= 2 && numActivated == 0 && numStatic <= 2 {
		cc.Category = CatSimpleETB
		cc.EffectSummary = summarizeTriggeredEffect(ast.AST.Abilities)
		return cc
	}

	// Simple trigger: has 1-2 triggered abilities (non-ETB), no activated
	if numTriggered >= 1 && numTriggered <= 2 && numETB == 0 && numActivated == 0 {
		// Check if trigger events are ones we know
		allKnown := true
		for _, ev := range cc.TriggerEvents {
			if _, ok := triggerEventMap[ev]; !ok {
				allKnown = false
				break
			}
		}
		if allKnown && len(cc.TriggerEvents) > 0 {
			cc.Category = CatSimpleTrigger
			cc.EffectSummary = summarizeTriggeredEffect(ast.AST.Abilities)
			return cc
		}
	}

	// Simple activated: has exactly 1 activated ability, maybe keywords/statics
	if numActivated == 1 && numTriggered == 0 {
		cc.Category = CatSimpleActive
		cc.EffectSummary = summarizeActivatedEffect(ast.AST.Abilities)
		return cc
	}

	// Static only: only static abilities + keywords
	if numStatic > 0 && numTriggered == 0 && numActivated == 0 {
		cc.Category = CatStatic
		return cc
	}

	// Multi-trigger or trigger+activated or trigger+static = complex but
	// many of these are still doable. If they have known trigger events
	// and <= 3 total non-keyword abilities, treat them as triggers.
	if numTriggered >= 1 && numTriggered <= 3 && numActivated <= 1 && numETB == 0 {
		allKnown := true
		for _, ev := range cc.TriggerEvents {
			if _, ok := triggerEventMap[ev]; !ok {
				allKnown = false
				break
			}
		}
		if allKnown && len(cc.TriggerEvents) > 0 {
			cc.Category = CatSimpleTrigger
			cc.EffectSummary = summarizeTriggeredEffect(ast.AST.Abilities)
			return cc
		}
	}

	// ETB + other abilities = still generate ETB, mark partial for the rest
	if numETB >= 1 && cc.NonKWAbilities <= 4 {
		cc.Category = CatSimpleETB
		cc.EffectSummary = summarizeTriggeredEffect(ast.AST.Abilities)
		return cc
	}

	cc.Category = CatComplex
	return cc
}

func summarizeTriggeredEffect(abilities []Ability) string {
	for _, ab := range abilities {
		if ab.ASTType == "Triggered" {
			raw := ab.Raw
			if len(raw) > 120 {
				raw = raw[:120] + "..."
			}
			return raw
		}
	}
	return ""
}

func summarizeActivatedEffect(abilities []Ability) string {
	for _, ab := range abilities {
		if ab.ASTType == "Activated" {
			raw := ab.Raw
			if len(raw) > 120 {
				raw = raw[:120] + "..."
			}
			return raw
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Step 8: Generate handler files
// ---------------------------------------------------------------------------

type GeneratedHandler struct {
	Slug       string
	CardName   string
	FuncPrefix string // Go function name prefix (e.g., "aesiTyrantOfGyreStrait")
	FileName   string // filename without path
	Category   Category
	OracleText string
}

func generateHandlers(classified []ClassifiedCard, outDir string) []GeneratedHandler {
	var generated []GeneratedHandler

	// Group by category for batch generation
	var etbCards, triggerCards, activatedCards, staticCards []ClassifiedCard
	for _, cc := range classified {
		switch cc.Category {
		case CatSimpleETB:
			etbCards = append(etbCards, cc)
		case CatSimpleTrigger:
			triggerCards = append(triggerCards, cc)
		case CatSimpleActive:
			activatedCards = append(activatedCards, cc)
		case CatStatic:
			staticCards = append(staticCards, cc)
		}
	}

	// Generate ETB handlers
	for _, cc := range etbCards {
		gh := generateETBHandler(cc, outDir)
		if gh != nil {
			generated = append(generated, *gh)
		}
	}

	// Generate trigger handlers
	for _, cc := range triggerCards {
		gh := generateTriggerHandler(cc, outDir)
		if gh != nil {
			generated = append(generated, *gh)
		}
	}

	// Generate activated handlers
	for _, cc := range activatedCards {
		gh := generateActivatedHandler(cc, outDir)
		if gh != nil {
			generated = append(generated, *gh)
		}
	}

	// Generate static handlers (partial stubs)
	for _, cc := range staticCards {
		gh := generateStaticHandler(cc, outDir)
		if gh != nil {
			generated = append(generated, *gh)
		}
	}

	fmt.Printf("\nGenerated %d handler files.\n", len(generated))
	return generated
}

func slugToFuncName(slug string) string {
	parts := strings.Split(slug, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	result := b.String()
	// Ensure first char is lowercase for unexported name
	if len(result) > 0 {
		result = strings.ToLower(result[:1]) + result[1:]
	}
	return result
}

func slugToFileName(slug string) string {
	return "gen_" + slug + ".go"
}

// Escape a string for use in a Go string literal (double-quote delimited).
func goStringLit(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// wrapOracleComment formats oracle text as a Go block comment.
func wrapOracleComment(oracle string) string {
	lines := strings.Split(oracle, "\n")
	var out []string
	for _, l := range lines {
		out = append(out, "//   "+l)
	}
	return strings.Join(out, "\n")
}

// ---------------------------------------------------------------------------
// ETB handler generation
// ---------------------------------------------------------------------------

var etbTemplate = template.Must(template.New("etb").Parse(`package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// register{{.RegisterName}} wires {{.CardName}}.
//
// Oracle text:
//
{{.OracleComment}}
//
// Auto-generated ETB handler.
func register{{.RegisterName}}(r *Registry) {
	r.OnETB("{{.CardNameEscaped}}", {{.FuncPrefix}}ETB)
}

func {{.FuncPrefix}}ETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "{{.Slug}}_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
{{.Body}}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
`))

type etbTemplateData struct {
	RegisterName    string
	CardName        string
	CardNameEscaped string
	OracleComment   string
	FuncPrefix      string
	Slug            string
	Body            string
}

func generateETBHandler(cc ClassifiedCard, outDir string) *GeneratedHandler {
	funcPrefix := slugToFuncName(cc.Slug)
	registerName := strings.ToUpper(funcPrefix[:1]) + funcPrefix[1:]
	fileName := slugToFileName(cc.Slug)

	// Analyze the oracle text to determine what the ETB does
	body := generateETBBody(cc)

	data := etbTemplateData{
		RegisterName:    registerName,
		CardName:        cc.CardName,
		CardNameEscaped: goStringLit(cc.CardName),
		OracleComment:   wrapOracleComment(cc.OracleText),
		FuncPrefix:      funcPrefix,
		Slug:            cc.Slug,
		Body:            body,
	}

	outPath := filepath.Join(outDir, fileName)
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create %s: %v\n", outPath, err)
		return nil
	}
	defer f.Close()

	if err := etbTemplate.Execute(f, data); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: template exec for %s: %v\n", cc.CardName, err)
		return nil
	}

	return &GeneratedHandler{
		Slug:       cc.Slug,
		CardName:   cc.CardName,
		FuncPrefix: funcPrefix,
		FileName:   fileName,
		Category:   CatSimpleETB,
		OracleText: cc.OracleText,
	}
}

func generateETBBody(cc ClassifiedCard) string {
	oracle := strings.ToLower(cc.OracleText)
	var lines []string

	// Detect common ETB effects from oracle text
	if containsAny(oracle, "draw a card", "draws a card") {
		lines = append(lines, "\tdrawOne(gs, seat, perm.Card.DisplayName())")
	} else if matched, n := extractDrawN(oracle); matched {
		lines = append(lines, fmt.Sprintf("\tfor i := 0; i < %d; i++ {", n))
		lines = append(lines, "\t\tdrawOne(gs, seat, perm.Card.DisplayName())")
		lines = append(lines, "\t}")
	}

	if containsAny(oracle, "gain", "life") && containsAny(oracle, "enters") {
		n := extractNumber(oracle, "gain", "life")
		if n > 0 {
			lines = append(lines, fmt.Sprintf("\tgameengine.GainLife(gs, seat, %d, perm.Card.DisplayName())", n))
		}
	}

	if containsAny(oracle, "mill") && containsAny(oracle, "enters", "when") {
		n := extractMillN(oracle)
		if n > 0 {
			lines = append(lines, "\ts := gs.Seats[seat]")
			lines = append(lines, fmt.Sprintf("\tfor i := 0; i < %d; i++ {", n))
			lines = append(lines, "\t\tif len(s.Library) == 0 { break }")
			lines = append(lines, "\t\tcard := s.Library[0]")
			lines = append(lines, "\t\tgameengine.MoveCard(gs, card, seat, \"library\", \"graveyard\", \"mill\")")
			lines = append(lines, "\t}")
		}
	}

	if containsAny(oracle, "create") && containsAny(oracle, "token") && containsAny(oracle, "enters") {
		power, tough, tokenType := extractTokenInfo(oracle)
		lines = append(lines, "\ttoken := &gameengine.Card{")
		lines = append(lines, fmt.Sprintf("\t\tName:          \"%d/%d %s Token\",", power, tough, strings.Title(tokenType)))
		lines = append(lines, "\t\tOwner:         seat,")
		lines = append(lines, fmt.Sprintf("\t\tBasePower:     %d,", power))
		lines = append(lines, fmt.Sprintf("\t\tBaseToughness: %d,", tough))
		lines = append(lines, fmt.Sprintf("\t\tTypes:         []string{\"token\", \"creature\", \"%s\"},", strings.ToLower(tokenType)))
		lines = append(lines, "\t}")
		lines = append(lines, "\tenterBattlefieldWithETB(gs, seat, token, false)")
	}

	if containsAny(oracle, "scry") && containsAny(oracle, "enters", "when") {
		n := extractScryN(oracle)
		if n > 0 {
			lines = append(lines, fmt.Sprintf("\tgameengine.Scry(gs, seat, %d)", n))
		}
	}

	// If we couldn't parse any specific effect, emit a partial
	if len(lines) == 0 {
		lines = append(lines, "\temitPartial(gs, slug, perm.Card.DisplayName(), \"auto-gen: ETB effect not parsed from oracle text\")")
	}

	// Check for additional non-ETB abilities and mark partial
	hasOtherAbilities := false
	for _, ab := range cc.AbilityTypes {
		if ab == "Static" || ab == "Activated" {
			hasOtherAbilities = true
			break
		}
	}
	if hasOtherAbilities {
		lines = append(lines, "\temitPartial(gs, slug, perm.Card.DisplayName(), \"additional non-ETB abilities not implemented\")")
	}

	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Trigger handler generation
// ---------------------------------------------------------------------------

var triggerTemplate = template.Must(template.New("trigger").Parse(`package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// register{{.RegisterName}} wires {{.CardName}}.
//
// Oracle text:
//
{{.OracleComment}}
//
// Auto-generated trigger handler.
func register{{.RegisterName}}(r *Registry) {
{{.Registration}}
}
{{.Handlers}}
`))

type triggerTemplateData struct {
	RegisterName    string
	CardName        string
	CardNameEscaped string
	OracleComment   string
	FuncPrefix      string
	Slug            string
	Registration    string
	Handlers        string
}

func generateTriggerHandler(cc ClassifiedCard, outDir string) *GeneratedHandler {
	funcPrefix := slugToFuncName(cc.Slug)
	registerName := strings.ToUpper(funcPrefix[:1]) + funcPrefix[1:]
	fileName := slugToFileName(cc.Slug)

	// Build registration lines and handler functions
	var regLines []string
	var handlerFuncs []string

	for i, ev := range cc.TriggerEvents {
		engineEvent, ok := triggerEventMap[ev]
		if !ok {
			engineEvent = ev
		}
		handlerName := fmt.Sprintf("%sTrigger", funcPrefix)
		if len(cc.TriggerEvents) > 1 {
			handlerName = fmt.Sprintf("%sTrigger%d", funcPrefix, i+1)
		}

		regLines = append(regLines, fmt.Sprintf("\tr.OnTrigger(\"%s\", \"%s\", %s)",
			goStringLit(cc.CardName), engineEvent, handlerName))

		body := generateTriggerBody(cc, ev, engineEvent)
		handler := fmt.Sprintf(`
func %s(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "%s_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
%s
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}`, handlerName, cc.Slug, body)
		handlerFuncs = append(handlerFuncs, handler)
	}

	if len(regLines) == 0 {
		// No known trigger events, skip
		return nil
	}

	data := triggerTemplateData{
		RegisterName:    registerName,
		CardName:        cc.CardName,
		CardNameEscaped: goStringLit(cc.CardName),
		OracleComment:   wrapOracleComment(cc.OracleText),
		FuncPrefix:      funcPrefix,
		Slug:            cc.Slug,
		Registration:    strings.Join(regLines, "\n"),
		Handlers:        strings.Join(handlerFuncs, "\n"),
	}

	outPath := filepath.Join(outDir, fileName)
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create %s: %v\n", outPath, err)
		return nil
	}
	defer f.Close()

	if err := triggerTemplate.Execute(f, data); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: template exec for %s: %v\n", cc.CardName, err)
		return nil
	}

	return &GeneratedHandler{
		Slug:       cc.Slug,
		CardName:   cc.CardName,
		FuncPrefix: funcPrefix,
		FileName:   fileName,
		Category:   CatSimpleTrigger,
		OracleText: cc.OracleText,
	}
}

func generateTriggerBody(cc ClassifiedCard, astEvent, engineEvent string) string {
	var lines []string
	oracle := strings.ToLower(cc.OracleText)

	// Common trigger guard: check controller/caster seat
	switch engineEvent {
	case "spell_cast", "noncreature_spell_cast", "instant_or_sorcery_cast":
		lines = append(lines, "\tcasterSeat, _ := ctx[\"caster_seat\"].(int)")
		lines = append(lines, "\tif casterSeat != perm.Controller { return }")
	case "creature_attacks":
		lines = append(lines, "\tattackerPerm, _ := ctx[\"attacker_perm\"].(*gameengine.Permanent)")
		if containsAny(oracle, "whenever "+strings.ToLower(cc.CardName)+" attacks",
			"whenever ~ attacks", "whenever this creature attacks") {
			lines = append(lines, "\tif attackerPerm != perm { return }")
		} else {
			lines = append(lines, "\tif attackerPerm == nil || attackerPerm.Controller != perm.Controller { return }")
		}
	case "combat_damage_player":
		lines = append(lines, "\tsrcSeat, _ := ctx[\"source_seat\"].(int)")
		lines = append(lines, "\tif srcSeat != perm.Controller { return }")
	case "creature_dies":
		lines = append(lines, "\tcontrollerSeat, _ := ctx[\"controller_seat\"].(int)")
		if containsAny(oracle, "you don't control", "you don’t control", "an opponent controls") {
			lines = append(lines, "\tif controllerSeat == perm.Controller { return } // fires on opponent's creatures only")
		} else if containsAny(oracle, "you control", "your") {
			lines = append(lines, "\tif controllerSeat != perm.Controller { return }")
		} else {
			lines = append(lines, "\t_ = controllerSeat // available for filtering")
		}
	case "permanent_etb":
		lines = append(lines, "\tcontrollerSeat, _ := ctx[\"controller_seat\"].(int)")
		lines = append(lines, "\tif controllerSeat != perm.Controller { return }")
	case "upkeep_controller":
		lines = append(lines, "\tactiveSeat, _ := ctx[\"active_seat\"].(int)")
		lines = append(lines, "\tif activeSeat != perm.Controller { return }")
	case "end_step":
		lines = append(lines, "\tactiveSeat, _ := ctx[\"active_seat\"].(int)")
		lines = append(lines, "\tif activeSeat != perm.Controller { return }")
	case "life_gained":
		lines = append(lines, "\tgainSeat, _ := ctx[\"seat\"].(int)")
		lines = append(lines, "\tif gainSeat != perm.Controller { return }")
	case "card_discarded":
		lines = append(lines, "\tdiscarderSeat, _ := ctx[\"discarder_seat\"].(int)")
		if containsAny(oracle, "opponent") {
			lines = append(lines, "\tif discarderSeat == perm.Controller { return }")
		}
	case "token_created":
		lines = append(lines, "\tcreatorSeat, _ := ctx[\"seat\"].(int)")
		lines = append(lines, "\tif creatorSeat != perm.Controller { return }")
	}

	// Detect common effects
	effectLines := detectEffectsFromOracle(oracle, cc.CardName, "perm")
	lines = append(lines, effectLines...)

	if len(effectLines) == 0 {
		lines = append(lines, "\temitPartial(gs, slug, perm.Card.DisplayName(), \"auto-gen: trigger effect not parsed from oracle text\")")
	}

	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Activated handler generation
// ---------------------------------------------------------------------------

var activatedTemplate = template.Must(template.New("activated").Parse(`package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// register{{.RegisterName}} wires {{.CardName}}.
//
// Oracle text:
//
{{.OracleComment}}
//
// Auto-generated activated ability handler.
func register{{.RegisterName}}(r *Registry) {
	r.OnActivated("{{.CardNameEscaped}}", {{.FuncPrefix}}Activate)
}

func {{.FuncPrefix}}Activate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "{{.Slug}}_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
{{.Body}}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
`))

type activatedTemplateData struct {
	RegisterName    string
	CardName        string
	CardNameEscaped string
	OracleComment   string
	FuncPrefix      string
	Slug            string
	Body            string
}

func generateActivatedHandler(cc ClassifiedCard, outDir string) *GeneratedHandler {
	funcPrefix := slugToFuncName(cc.Slug)
	registerName := strings.ToUpper(funcPrefix[:1]) + funcPrefix[1:]
	fileName := slugToFileName(cc.Slug)

	body := generateActivatedBody(cc)

	data := activatedTemplateData{
		RegisterName:    registerName,
		CardName:        cc.CardName,
		CardNameEscaped: goStringLit(cc.CardName),
		OracleComment:   wrapOracleComment(cc.OracleText),
		FuncPrefix:      funcPrefix,
		Slug:            cc.Slug,
		Body:            body,
	}

	outPath := filepath.Join(outDir, fileName)
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create %s: %v\n", outPath, err)
		return nil
	}
	defer f.Close()

	if err := activatedTemplate.Execute(f, data); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: template exec for %s: %v\n", cc.CardName, err)
		return nil
	}

	return &GeneratedHandler{
		Slug:       cc.Slug,
		CardName:   cc.CardName,
		FuncPrefix: funcPrefix,
		FileName:   fileName,
		Category:   CatSimpleActive,
		OracleText: cc.OracleText,
	}
}

func generateActivatedBody(cc ClassifiedCard) string {
	oracle := strings.ToLower(cc.OracleText)
	var lines []string

	effectLines := detectEffectsFromOracle(oracle, cc.CardName, "src")
	lines = append(lines, effectLines...)

	if len(effectLines) == 0 {
		lines = append(lines, "\temitPartial(gs, slug, src.Card.DisplayName(), \"auto-gen: activated effect not parsed from oracle text\")")
	}

	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Static handler generation (partial stubs)
// ---------------------------------------------------------------------------

var staticTemplate = template.Must(template.New("static").Parse(`package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// register{{.RegisterName}} wires {{.CardName}}.
//
// Oracle text:
//
{{.OracleComment}}
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func register{{.RegisterName}}(r *Registry) {
	r.OnETB("{{.CardNameEscaped}}", {{.FuncPrefix}}StaticETB)
}

func {{.FuncPrefix}}StaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "{{.Slug}}_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
`))

type staticTemplateData struct {
	RegisterName    string
	CardName        string
	CardNameEscaped string
	OracleComment   string
	FuncPrefix      string
	Slug            string
}

func generateStaticHandler(cc ClassifiedCard, outDir string) *GeneratedHandler {
	funcPrefix := slugToFuncName(cc.Slug)
	registerName := strings.ToUpper(funcPrefix[:1]) + funcPrefix[1:]
	fileName := slugToFileName(cc.Slug)

	data := staticTemplateData{
		RegisterName:    registerName,
		CardName:        cc.CardName,
		CardNameEscaped: goStringLit(cc.CardName),
		OracleComment:   wrapOracleComment(cc.OracleText),
		FuncPrefix:      funcPrefix,
		Slug:            cc.Slug,
	}

	outPath := filepath.Join(outDir, fileName)
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create %s: %v\n", outPath, err)
		return nil
	}
	defer f.Close()

	if err := staticTemplate.Execute(f, data); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: template exec for %s: %v\n", cc.CardName, err)
		return nil
	}

	return &GeneratedHandler{
		Slug:       cc.Slug,
		CardName:   cc.CardName,
		FuncPrefix: funcPrefix,
		FileName:   fileName,
		Category:   CatStatic,
		OracleText: cc.OracleText,
	}
}

// ---------------------------------------------------------------------------
// Step 9: Generate batch_generated.go
// ---------------------------------------------------------------------------

func generateBatchFile(handlers []GeneratedHandler, outDir string) {
	// Collect hand-edited gen_*.go files (preserved by cleanOldGenFiles)
	// and append their register*() calls so they actually get wired into
	// the registry. Without this, hand-improved handlers become dead code.
	handEdited := collectHandEditedHandlers(outDir)

	if len(handlers) == 0 && len(handEdited) == 0 {
		return
	}

	outPath := filepath.Join(outDir, "batch_generated.go")
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create batch_generated.go: %v\n", err)
		return
	}
	defer f.Close()

	fmt.Fprintln(f, "package per_card")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "// registerGeneratedHandlers registers all auto-generated handlers.")
	fmt.Fprintln(f, "// Generated by cmd/gen-handlers.")
	fmt.Fprintln(f, "func registerGeneratedHandlers(r *Registry) {")

	// Sort by card name for deterministic output
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].CardName < handlers[j].CardName
	})

	// Build a unified, deduped, sorted list (auto-gen + hand-edited).
	type entry struct {
		registerName string
		comment      string
	}
	seen := map[string]bool{}
	var entries []entry
	for _, h := range handlers {
		registerName := strings.ToUpper(h.FuncPrefix[:1]) + h.FuncPrefix[1:]
		if seen["register"+registerName] {
			continue
		}
		seen["register"+registerName] = true
		entries = append(entries, entry{
			registerName: registerName,
			comment:      fmt.Sprintf("%s [%s]", h.CardName, h.Category),
		})
	}
	for _, he := range handEdited {
		if seen[he.registerCall] {
			continue
		}
		seen[he.registerCall] = true
		entries = append(entries, entry{
			registerName: strings.TrimPrefix(he.registerCall, "register"),
			comment:      fmt.Sprintf("%s [hand-edited]", he.cardName),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].comment < entries[j].comment
	})
	for _, e := range entries {
		fmt.Fprintf(f, "\tregister%s(r) // %s\n", e.registerName, e.comment)
	}

	fmt.Fprintln(f, "}")
}

type handEditedHandler struct {
	registerCall string // e.g. "registerAlaundoTheSeer"
	cardName     string // e.g. "Alaundo the Seer"
}

// collectHandEditedHandlers walks the per_card directory for gen_*.go files
// that lack the "Auto-generated" marker and extracts their register*() entry
// points so we can wire them into batch_generated.go.
func collectHandEditedHandlers(dir string) []handEditedHandler {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	funcRe := regexp.MustCompile(`func\s+(register[A-Za-z0-9_]+)\s*\(r \*Registry\)`)
	nameRe := regexp.MustCompile(`\.On(?:ETB|Cast|Resolve|Activated|Trigger)\("((?:[^"\\]|\\.)+)"`)
	var out []handEditedHandler
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "gen_") || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "Auto-generated") {
			continue
		}
		funcMatches := funcRe.FindStringSubmatch(content)
		if len(funcMatches) < 2 {
			continue
		}
		card := ""
		if nm := nameRe.FindStringSubmatch(content); len(nm) > 1 {
			card = nm[1]
		}
		out = append(out, handEditedHandler{
			registerCall: funcMatches[1],
			cardName:     card,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Step 10: Patch registry.go
// ---------------------------------------------------------------------------

func patchRegistry(registryPath string) {
	data, err := os.ReadFile(registryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot read registry.go: %v\n", err)
		return
	}
	content := string(data)

	// Check if already patched
	if strings.Contains(content, "registerGeneratedHandlers") {
		fmt.Println("registry.go already has registerGeneratedHandlers call.")
		return
	}

	// Find func init() and the closing "}" of registerDefaults just before it.
	initIdx := strings.Index(content, "func init() {")
	if initIdx < 0 {
		fmt.Fprintln(os.Stderr, "WARN: cannot find init() in registry.go, manual patch needed")
		return
	}

	// The "}" that closes registerDefaults is the last "}" before "func init()".
	braceIdx := strings.LastIndex(content[:initIdx], "}")
	if braceIdx < 0 {
		fmt.Fprintln(os.Stderr, "WARN: cannot find registerDefaults closing brace")
		return
	}

	// Insert BEFORE that closing brace (inside the function body).
	insertion := "\n\t// Auto-generated handlers (cmd/gen-handlers).\n\tregisterGeneratedHandlers(Global())\n"
	newContent := content[:braceIdx] + insertion + content[braceIdx:]
	if err := os.WriteFile(registryPath, []byte(newContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot write registry.go: %v\n", err)
		return
	}
	fmt.Println("Patched registry.go with registerGeneratedHandlers call.")
}

// ---------------------------------------------------------------------------
// Step 11: Report
// ---------------------------------------------------------------------------

func printReport(classified []ClassifiedCard, generated []GeneratedHandler, noAST []string) {
	fmt.Println("\n========================================")
	fmt.Println("           GENERATION REPORT")
	fmt.Println("========================================")

	genMap := map[string]bool{}
	for _, g := range generated {
		genMap[g.Slug] = true
	}

	fmt.Printf("\nTotal commanders in pool: %d\n", 652)

	// Already handled
	fmt.Println("\n--- GENERATED (auto) ---")
	for _, g := range generated {
		fmt.Printf("  [%s] %s -> %s\n", g.Category, g.CardName, g.FileName)
	}

	fmt.Println("\n--- KEYWORD-ONLY (skipped, engine handles) ---")
	for _, cc := range classified {
		if cc.Category == CatKeywordOnly {
			kw := strings.Join(cc.Keywords, ", ")
			if kw == "" {
				kw = "(no abilities)"
			}
			fmt.Printf("  %s [%s]\n", cc.CardName, kw)
		}
	}

	fmt.Println("\n--- COMPLEX (needs manual handler) ---")
	for _, cc := range classified {
		if cc.Category == CatComplex {
			fmt.Printf("  %s\n", cc.CardName)
			if cc.OracleText != "" {
				// Print first 120 chars of oracle
				oracle := cc.OracleText
				if len(oracle) > 120 {
					oracle = oracle[:120] + "..."
				}
				oracle = strings.ReplaceAll(oracle, "\n", " | ")
				fmt.Printf("    Oracle: %s\n", oracle)
			}
		}
	}

	if len(noAST) > 0 {
		fmt.Println("\n--- NO AST DATA (needs manual lookup) ---")
		for _, slug := range noAST {
			fmt.Printf("  %s\n", slug)
		}
	}

	// Summary
	counts := map[Category]int{}
	for _, cc := range classified {
		counts[cc.Category]++
	}
	fmt.Println("\n--- SUMMARY ---")
	fmt.Printf("  Generated:     %d\n", len(generated))
	fmt.Printf("  Keyword-only:  %d (skipped)\n", counts[CatKeywordOnly])
	fmt.Printf("  Complex:       %d (manual)\n", counts[CatComplex])
	fmt.Printf("  No AST:        %d (manual lookup)\n", counts[CatNoAST])
	total := len(generated) + counts[CatKeywordOnly]
	fmt.Printf("  Coverage:      %d / %d unhandled (%.0f%%)\n",
		total, len(classified), float64(total)/float64(max(len(classified), 1))*100)
}

// ---------------------------------------------------------------------------
// Oracle text parsing helpers
// ---------------------------------------------------------------------------

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func extractDrawN(oracle string) (bool, int) {
	patterns := []struct {
		pat string
		n   int
	}{
		{"draw two cards", 2},
		{"draw three cards", 3},
		{"draw four cards", 4},
		{"draw five cards", 5},
		{"draw six cards", 6},
		{"draw seven cards", 7},
		{"draws two cards", 2},
		{"draws three cards", 3},
		{"draw cards equal to", 0}, // variable, skip
	}
	for _, p := range patterns {
		if strings.Contains(oracle, p.pat) {
			if p.n > 0 {
				return true, p.n
			}
		}
	}
	return false, 0
}

// numberAlternatives is scanned in deterministic priority order — multi-char
// digits and named numerals before single-char fallbacks, with "a"/"an" last
// because they otherwise match inside words like "gain" or "ranger". Each
// entry is treated as a whole-token match against the oracle segment.
var numberAlternatives = []struct {
	tok string
	n   int
}{
	{"10", 10}, {"ten", 10},
	{"9", 9}, {"nine", 9},
	{"8", 8}, {"eight", 8},
	{"7", 7}, {"seven", 7},
	{"6", 6}, {"six", 6},
	{"5", 5}, {"five", 5},
	{"4", 4}, {"four", 4},
	{"3", 3}, {"three", 3},
	{"2", 2}, {"two", 2},
	{"1", 1}, {"one", 1},
	{"an", 1}, {"a", 1},
}

func extractNumber(oracle, before, after string) int {
	idx := strings.Index(oracle, before)
	if idx < 0 {
		return 0
	}
	afterIdx := strings.Index(oracle[idx:], after)
	if afterIdx < 0 {
		return 0
	}
	segment := oracle[idx : idx+afterIdx]
	for _, alt := range numberAlternatives {
		// Word-boundary match avoids "a" inside "gain", "1" inside "1/1", etc.
		re, err := regexp.Compile(`(^|[^a-z0-9])` + regexp.QuoteMeta(alt.tok) + `([^a-z0-9]|$)`)
		if err != nil {
			continue
		}
		if re.MatchString(segment) {
			return alt.n
		}
	}
	return 0
}

func extractMillN(oracle string) int {
	patterns := map[string]int{
		"mill a card":      1,
		"mill one card":    1,
		"mill two cards":   2,
		"mill three cards": 3,
		"mill four cards":  4,
		"mill five cards":  5,
		"mill six cards":   6,
		"mill eight cards": 8,
		"mill ten cards":   10,
	}
	for pat, n := range patterns {
		if strings.Contains(oracle, pat) {
			return n
		}
	}
	return 0
}

func extractScryN(oracle string) int {
	patterns := map[string]int{
		"scry 1": 1,
		"scry 2": 2,
		"scry 3": 3,
		"scry 4": 4,
		"scry 5": 5,
	}
	for pat, n := range patterns {
		if strings.Contains(oracle, pat) {
			return n
		}
	}
	return 0
}

func extractTokenInfo(oracle string) (int, int, string) {
	// Try common token patterns
	patterns := []struct {
		pat   string
		p, t  int
		ttype string
	}{
		{"1/1", 1, 1, "creature"},
		{"2/2", 2, 2, "creature"},
		{"3/3", 3, 3, "creature"},
		{"4/4", 4, 4, "creature"},
		{"0/1", 0, 1, "creature"},
		{"1/0", 1, 0, "creature"},
		{"0/0", 0, 0, "creature"},
	}
	for _, p := range patterns {
		if strings.Contains(oracle, p.pat) {
			// Try to find creature type near the P/T
			idx := strings.Index(oracle, p.pat)
			nearby := oracle[idx:]
			if len(nearby) > 60 {
				nearby = nearby[:60]
			}
			// Common creature types
			types := []string{"soldier", "spirit", "zombie", "goblin", "elf", "human",
				"warrior", "elemental", "vampire", "angel", "demon", "dragon",
				"beast", "saproling", "cat", "dog", "bird", "fish", "insect",
				"treasure", "food", "clue", "blood", "thopter", "servo", "myr",
				"knight", "wizard", "drake", "faerie", "rat", "skeleton", "token"}
			for _, tt := range types {
				if strings.Contains(nearby, tt) {
					return p.p, p.t, tt
				}
			}
			return p.p, p.t, p.ttype
		}
	}
	return 1, 1, "creature"
}

// detectEffectsFromOracle detects common effects from oracle text and
// generates Go code lines. varName is the permanent variable name
// ("perm" for ETB/trigger handlers, "src" for activated handlers).
func detectEffectsFromOracle(oracle, cardName, varName string) []string {
	var lines []string
	ctrl := varName + ".Controller"
	disp := varName + ".Card.DisplayName()"

	if containsAny(oracle, "draw a card", "draws a card") {
		lines = append(lines, fmt.Sprintf("\tdrawOne(gs, %s, %s)", ctrl, disp))
	} else if matched, n := extractDrawN(oracle); matched {
		lines = append(lines, fmt.Sprintf("\tfor i := 0; i < %d; i++ {", n))
		lines = append(lines, fmt.Sprintf("\t\tdrawOne(gs, %s, %s)", ctrl, disp))
		lines = append(lines, "\t}")
	}

	if containsAny(oracle, "gain") && containsAny(oracle, "life") &&
		!containsAny(oracle, "enters") { // ETB life gain is handled separately
		n := extractNumber(oracle, "gain", "life")
		if n > 0 {
			lines = append(lines, fmt.Sprintf("\tgameengine.GainLife(gs, %s, %d, %s)", ctrl, n, disp))
		}
	}

	if containsAny(oracle, "lose") && containsAny(oracle, "life") {
		n := extractNumber(oracle, "lose", "life")
		if n > 0 && !containsAny(oracle, "you lose") {
			// Opponents lose life
			lines = append(lines, fmt.Sprintf("\tfor _, opp := range gs.Opponents(%s) {", ctrl))
			lines = append(lines, "\t\tif gs.Seats[opp] != nil && !gs.Seats[opp].Lost {")
			lines = append(lines, fmt.Sprintf("\t\t\tgameengine.LoseLife(gs, opp, %d, %s)", n, disp))
			lines = append(lines, "\t\t}")
			lines = append(lines, "\t}")
			lines = append(lines, "\t_ = gs.CheckEnd()")
		}
	}

	if containsAny(oracle, "create") && containsAny(oracle, "token") {
		power, tough, tokenType := extractTokenInfo(oracle)
		lines = append(lines, "\ttoken := &gameengine.Card{")
		lines = append(lines, fmt.Sprintf("\t\tName:          \"%d/%d %s Token\",", power, tough, capitalize(tokenType)))
		lines = append(lines, fmt.Sprintf("\t\tOwner:         %s,", ctrl))
		lines = append(lines, fmt.Sprintf("\t\tBasePower:     %d,", power))
		lines = append(lines, fmt.Sprintf("\t\tBaseToughness: %d,", tough))
		lines = append(lines, fmt.Sprintf("\t\tTypes:         []string{\"token\", \"creature\", \"%s\"},", strings.ToLower(tokenType)))
		lines = append(lines, "\t}")
		lines = append(lines, fmt.Sprintf("\tenterBattlefieldWithETB(gs, %s, token, false)", ctrl))
	}

	if containsAny(oracle, "scry") {
		n := extractScryN(oracle)
		if n > 0 {
			lines = append(lines, fmt.Sprintf("\tgameengine.Scry(gs, %s, %d)", ctrl, n))
		}
	}

	if containsAny(oracle, "mill") {
		n := extractMillN(oracle)
		if n > 0 {
			lines = append(lines, fmt.Sprintf("\ts := gs.Seats[%s]", ctrl))
			lines = append(lines, fmt.Sprintf("\tfor i := 0; i < %d; i++ {", n))
			lines = append(lines, "\t\tif len(s.Library) == 0 { break }")
			lines = append(lines, "\t\tcard := s.Library[0]")
			lines = append(lines, fmt.Sprintf("\t\tgameengine.MoveCard(gs, card, %s, \"library\", \"graveyard\", \"mill\")", ctrl))
			lines = append(lines, "\t}")
		}
	}

	if containsAny(oracle, "+1/+1 counter") && containsAny(oracle, "put") {
		if containsAny(oracle, "on each", "on all") {
			lines = append(lines, fmt.Sprintf("\tfor _, p := range gs.Seats[%s].Battlefield {", ctrl))
			lines = append(lines, fmt.Sprintf("\t\tif p == nil || !p.IsCreature() || p == %s { continue }", varName))
			lines = append(lines, "\t\tp.AddCounter(\"+1/+1\", 1)")
			lines = append(lines, "\t}")
		} else {
			lines = append(lines, fmt.Sprintf("\t%s.AddCounter(\"+1/+1\", 1)", varName))
		}
	}

	return lines
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

// cleanOldGenFiles removes auto-generated handlers so the tool is idempotent.
// It preserves any gen_*.go file that does NOT contain the "Auto-generated"
// marker string (those are hand-edited handlers we want to keep) and never
// touches *_test.go files.
func cleanOldGenFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	removed := 0
	preserved := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if strings.HasPrefix(name, "gen_") && strings.HasSuffix(name, ".go") {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err == nil && !strings.Contains(string(data), "Auto-generated") {
				preserved++
				continue
			}
			os.Remove(path)
			removed++
		}
		if name == "batch_generated.go" {
			os.Remove(filepath.Join(dir, name))
			removed++
		}
	}
	if removed > 0 || preserved > 0 {
		fmt.Printf("Cleaned %d old generated files (preserved %d hand-edited).\n", removed, preserved)
	}
}

func findRepoRoot() string {
	// Try current directory, then walk up
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback: try the known path
	known := "/Users/joshuawiedeman/Documents/GitHub/HexDek"
	if _, err := os.Stat(filepath.Join(known, "go.mod")); err == nil {
		return known
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
