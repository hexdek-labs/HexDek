// hexdek-import — Deck import CLI for Moxfield and Archidekt.
//
// Fetches a decklist from a Moxfield or Archidekt URL and writes it as
// a .txt file in the standard deckparser format:
//
//	# Deck Name
//	# Source: https://moxfield.com/decks/XXXXX
//	COMMANDER: Commander Name
//	1 Card Name
//	1 Another Card
//	...
//
// Usage:
//
//	hexdek-import --moxfield https://www.moxfield.com/decks/XXXXXX
//	hexdek-import --archidekt https://archidekt.com/decks/12345/deckname
//	hexdek-import --moxfield URL --output data/decks/josh
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/moxfield"
)

func main() {
	moxfieldURL := flag.String("moxfield", "", "Moxfield deck URL to import")
	archidektURL := flag.String("archidekt", "", "Archidekt deck URL to import")
	outputDir := flag.String("output", "", "output directory for deck files (overrides --owner)")
	owner := flag.String("owner", "imported", "owner/group folder name for the deck (saves to data/decks/<owner>/)")
	refresh := flag.Bool("refresh", false, "invalidate the cached Moxfield deck JSON before fetching (default: serve from ~/.cache/hexdek/moxfield if fresher than HEXDEK_MOXFIELD_CACHE_TTL)")
	validateFormat := flag.Bool("validate-format", false, "after import, check each card against the deck's declared format (commander/brawl/modern/...) via Scryfall and print any banned/not-legal violations as warnings (non-fatal)")
	listImports := flag.Bool("list-imports", false, "dump the import-tracking registry (every Moxfield deck this user has imported through hexdek-import); doesn't trigger any HTTP")
	checkStale := flag.Bool("check-stale", false, "for each deck in the import registry, re-fetch from Moxfield and report which ones have changed upstream since last import (useful for delta re-imports)")
	flag.Parse()

	if *listImports {
		runListImports()
		return
	}
	if *checkStale {
		runCheckStale()
		return
	}

	if *validateFormat {
		// Env-gated so runImportMoxfield (testable core) can pick it up
		// without changing its signature.
		_ = os.Setenv("HEXDEK_IMPORT_VALIDATE_FORMAT", "1")
	}

	if *moxfieldURL == "" && *archidektURL == "" {
		fmt.Println("hexdek-import — import decks from Moxfield and Archidekt")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  hexdek-import --moxfield https://www.moxfield.com/decks/XXXXXX")
		fmt.Println("  hexdek-import --archidekt https://archidekt.com/decks/12345/deckname")
		fmt.Println("  hexdek-import --archidekt URL --owner lyon")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --moxfield URL    Moxfield deck URL to import")
		fmt.Println("  --archidekt URL   Archidekt deck URL to import")
		fmt.Println("  --owner NAME      Owner/group folder (default: imported)")
		fmt.Println("  --output DIR      Output directory (overrides --owner)")
		fmt.Println("  --refresh         Invalidate the Moxfield deck cache before fetching")
		fmt.Println("  --validate-format Check each card's Scryfall legality against the deck's declared format")
		fmt.Println("  --list-imports    Dump the import-tracking registry; no HTTP")
		fmt.Println("  --check-stale     Compare each imported deck against upstream; flag any that have changed")
		os.Exit(1)
	}

	// Resolve output directory: --output takes precedence, otherwise use data/decks/<owner>/
	resolvedOutputDir := *outputDir
	if resolvedOutputDir == "" {
		resolvedOutputDir = filepath.Join("data", "decks", *owner)
	}

	if err := os.MkdirAll(resolvedOutputDir, 0755); err != nil {
		log.Fatalf("create output dir %s: %v", resolvedOutputDir, err)
	}

	if *moxfieldURL != "" {
		if *refresh {
			if id := moxfield.ExtractDeckID(*moxfieldURL); id != "" {
				moxfield.Refresh(id)
			}
		}
		importMoxfield(*moxfieldURL, resolvedOutputDir)
	}
	if *archidektURL != "" {
		importArchidekt(*archidektURL, resolvedOutputDir)
	}
}

// ---------------------------------------------------------------------------
// Moxfield import
// ---------------------------------------------------------------------------

func importMoxfield(url, outputDir string) {
	outPath, err := runImportMoxfield(url, outputDir)
	if err != nil {
		log.Fatalf("%v", err)
	}
	// Auto-run Freya analysis (gated by env so tests skip the shell-out).
	if os.Getenv("HEXDEK_IMPORT_SKIP_FREYA") == "" {
		runFreyaOnDeck(outPath)
	}
}

// runImportMoxfield is the testable core of importMoxfield: fetch the
// Moxfield deck, write it to disk under outputDir with the standard
// header comments, log the saved path + summary, and return the
// written file's path. Returns error instead of log.Fatal so tests
// can exercise the failure modes (bad URL, API error, write failure).
func runImportMoxfield(url, outputDir string) (string, error) {
	log.Printf("importing from Moxfield: %s", url)

	deckID := moxfield.ExtractDeckID(url)
	if deckID == "" {
		return "", fmt.Errorf("could not extract deck ID from URL: %s", url)
	}

	// Capture the prior-snapshot count BEFORE fetching so we can tell
	// after the fetch whether a new snapshot was created (cache miss
	// → new file landed) or this is a no-op re-import (cache hit →
	// snapshot count unchanged). Only the new-snapshot case produces
	// a meaningful version diff.
	priorSnapCount := 0
	if snaps, err := moxfield.ListSnapshots(deckID); err == nil {
		priorSnapCount = len(snaps)
	}

	// Both FetchDeckName and FetchDeck share the in-process cache, so
	// the back-to-back pair only hits the API once.
	deckName, nameErr := moxfield.FetchDeckName(url)
	if nameErr != nil {
		deckName = deckID // fallback to ID
	}

	text, err := moxfield.FetchDeck(url)
	if err != nil {
		return "", fmt.Errorf("fetch Moxfield deck: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n", deckName))
	sb.WriteString(fmt.Sprintf("# Source: %s\n", url))
	sb.WriteString(text)

	filename := sanitizeFilename(deckName) + ".txt"
	outPath := filepath.Join(outputDir, filename)

	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", outPath, err)
	}
	log.Printf("saved %s (%d bytes)", outPath, len(sb.String()))
	printDeckSummary(text)

	// Record the import in the per-user registry so --list-imports /
	// --check-stale + future delta-import tooling have a truth set of
	// "what this user has imported". Best-effort: log and continue on
	// IO failure rather than failing the import.
	recordImportRegistry(deckID, url, deckName, outPath)

	// Re-import diff: if this fetch produced a new snapshot AND there
	// was a prior snapshot to compare against, show what changed
	// upstream since last import. Skipped silently when:
	//   - first import (priorSnapCount == 0)
	//   - cache hit (no new snapshot created)
	//   - HEXDEK_IMPORT_NO_DIFF=1 (opt-out for scripted bulk imports)
	if os.Getenv("HEXDEK_IMPORT_NO_DIFF") == "" {
		showImportDiff(deckID, priorSnapCount)
	}

	if os.Getenv("HEXDEK_IMPORT_VALIDATE_FORMAT") != "" {
		runFormatValidation(deckID)
	}
	return outPath, nil
}

// showImportDiff compares the just-fetched snapshot against the
// previously-newest one and prints the deck-level changes. Quiet
// when this is the first import or when the fetch hit cache (no new
// snapshot landed).
func showImportDiff(deckID string, priorSnapCount int) {
	snaps, err := moxfield.ListSnapshots(deckID)
	if err != nil {
		log.Printf("  [diff] could not list snapshots: %v", err)
		return
	}
	if len(snaps) <= priorSnapCount {
		// Cache hit — no new snapshot, nothing to diff against this
		// import.
		return
	}
	if priorSnapCount == 0 {
		log.Printf("  [diff] first import of deck %s (no prior version to diff)", deckID)
		return
	}
	d, err := moxfield.DiffLatestSnapshots(deckID)
	if err != nil {
		log.Printf("  [diff] could not compute diff: %v", err)
		return
	}
	if d == nil || d.IsEmpty() {
		log.Printf("  [diff] no upstream changes vs previous import")
		return
	}
	log.Printf("  [diff] changes since previous import:")
	for _, line := range strings.Split(d.String(), "\n") {
		log.Printf("  [diff] %s", line)
	}
}

// recordImportRegistry upserts an entry in the moxfield import
// registry, pulling the most-recent snapshot's hash + path (set by
// the fetchDeckRaw snapshot wiring) so --check-stale has a baseline
// to compare against later. Snapshot lookup is best-effort: if
// snapshots are disabled the registry entry still lands, just
// without a hash baseline (FindStale skips no-baseline entries).
func recordImportRegistry(deckID, url, deckName, outputPath string) {
	rec := moxfield.ImportRecord{
		DeckID:     deckID,
		URL:        url,
		DeckName:   deckName,
		OutputPath: outputPath,
	}
	if snaps, err := moxfield.ListSnapshots(deckID); err == nil && len(snaps) > 0 {
		rec.LastSnapshotHash = snaps[0].Hash
		rec.LastSnapshotPath = snaps[0].Path
	}
	if err := moxfield.RecordImport(rec); err != nil {
		log.Printf("  [imports] could not record in registry: %v", err)
	}
}

// runListImports prints the import registry to stdout in a stable
// human-readable form. Doesn't trigger any HTTP. Exits with status 0
// even when the registry is empty.
func runListImports() {
	records, err := moxfield.ListImports()
	if err != nil {
		log.Fatalf("list-imports: %v", err)
	}
	if len(records) == 0 {
		fmt.Println("(import registry is empty — no decks recorded)")
		return
	}
	fmt.Printf("# Moxfield import registry: %d deck(s)\n", len(records))
	for _, r := range records {
		fmt.Printf("\n%s\n", r.DeckName)
		fmt.Printf("  deck_id:        %s\n", r.DeckID)
		fmt.Printf("  url:            %s\n", r.URL)
		fmt.Printf("  output:         %s\n", r.OutputPath)
		fmt.Printf("  first_imported: %s\n", r.FirstImportedAt.Format(time.RFC3339))
		fmt.Printf("  last_imported:  %s\n", r.LastImportedAt.Format(time.RFC3339))
		fmt.Printf("  import_count:   %d\n", r.ImportCount)
		if r.LastSnapshotHash != "" {
			fmt.Printf("  snapshot_hash:  %s\n", r.LastSnapshotHash)
		}
	}
}

// runCheckStale re-fetches each registered deck and reports those
// whose upstream content has changed since the recorded snapshot.
// Hits the Moxfield API for every registered deck — expensive; the
// user opted in by setting --check-stale.
func runCheckStale() {
	stale, err := moxfield.FindStale(context.Background(), 0)
	if err != nil {
		log.Fatalf("check-stale: %v", err)
	}
	if len(stale) == 0 {
		fmt.Println("(no stale decks — every imported deck matches its upstream Moxfield content)")
		return
	}
	fmt.Printf("# %d deck(s) have changed upstream since last import:\n\n", len(stale))
	for _, r := range stale {
		fmt.Printf("- %s\n", r.DeckName)
		fmt.Printf("    %s\n", r.URL)
		fmt.Printf("    last imported: %s\n", r.LastImportedAt.Format(time.RFC3339))
		fmt.Printf("    recorded hash: %s\n", r.LastSnapshotHash)
		fmt.Println()
	}
	fmt.Println("Re-import each with `hexdek-import --refresh --moxfield <url>` to pick up the new state.")
}

// runFormatValidation hits Scryfall (via the rate-limited oracle
// package) for every unique card in the deck's commander+mainboard and
// reports any cards whose declared-format legality is banned /
// not_legal / restricted. Best-effort: on any setup error (DB open
// fail, format unknown) the function logs a notice and returns
// without failing the import. The flag is opt-in (--validate-format
// or HEXDEK_IMPORT_VALIDATE_FORMAT=1) so the default fast-path stays
// network-free beyond the one Moxfield deck fetch.
func runFormatValidation(deckID string) {
	// Use an in-memory SQLite cache so the validation is self-contained
	// and doesn't depend on (or pollute) the user's persistent oracle
	// cache. The cost is that legalities won't be cached across imports
	// — fine for an opt-in flag that already accepts the Scryfall RTT.
	d, err := db.Open(":memory:")
	if err != nil {
		log.Printf("  [validate] could not open temp oracle cache: %v (skipping)", err)
		return
	}
	defer d.Close()

	data, err := moxfield.FetchDeckMeta("https://moxfield.com/decks/" + deckID)
	if err != nil {
		log.Printf("  [validate] could not fetch deck meta for %s: %v (skipping)", deckID, err)
		return
	}
	// FetchDeckMeta primes the in-process moxfield cache; reach into it
	// for the full deck data via the public FetchDeckByID path so we
	// have the actual card list. ValidateFormatFromDeck takes the raw
	// apiResponse — but that type is unexported. Use the exported
	// helper ValidateFormatFromURL instead (added in this branch).
	report := moxfield.ValidateFormatFromURL(context.Background(), d, "https://moxfield.com/decks/"+deckID)
	if report == nil {
		log.Printf("  [validate] no validation report produced (skipping)")
		return
	}

	log.Printf("  [validate] format=%q  violations=%d  skipped=%d  (deck name: %q)",
		report.Format, len(report.Violations), len(report.SkippedUnknown), data.Name)
	for _, v := range report.Violations {
		log.Printf("  [validate]   ! %s", v.String())
	}
	if len(report.SkippedUnknown) > 0 && len(report.SkippedUnknown) <= 5 {
		log.Printf("  [validate]   (skipped legality check for %d card(s) with no Scryfall data: %v)",
			len(report.SkippedUnknown), report.SkippedUnknown)
	} else if len(report.SkippedUnknown) > 5 {
		log.Printf("  [validate]   (skipped legality check for %d card(s) with no Scryfall data)",
			len(report.SkippedUnknown))
	}
}

// ---------------------------------------------------------------------------
// Archidekt import
// ---------------------------------------------------------------------------

// archidektURLRE extracts the deck ID from an Archidekt URL.
// Supports:
//
//	https://archidekt.com/decks/12345/deckname
//	https://www.archidekt.com/decks/12345
//	archidekt.com/decks/12345/name
var archidektURLRE = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?archidekt\.com/decks/(\d+)`)

func extractArchidektID(url string) string {
	m := archidektURLRE.FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1]
}

// archidektResponse mirrors the subset of the Archidekt API response we need.
type archidektResponse struct {
	Name  string          `json:"name"`
	Cards []archidektCard `json:"cards"`
}

type archidektCard struct {
	Quantity   int                 `json:"quantity"`
	Card       archidektCardDetail `json:"card"`
	Categories []string           `json:"categories"`
}

type archidektCardDetail struct {
	OracleCard archidektOracle `json:"oracleCard"`
}

type archidektOracle struct {
	Name string `json:"name"`
}

func importArchidekt(url, outputDir string) {
	log.Printf("importing from Archidekt: %s", url)

	deckID := extractArchidektID(url)
	if deckID == "" {
		log.Fatalf("could not extract deck ID from URL: %s", url)
	}

	apiURL := fmt.Sprintf("https://archidekt.com/api/decks/%s/", deckID)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Fatalf("build request: %v", err)
	}
	req.Header.Set("User-Agent", "hexdek/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("fetch Archidekt API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Fatalf("Archidekt API returned %d for deck %s: %s", resp.StatusCode, deckID, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("read Archidekt response: %v", err)
	}

	var data archidektResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		log.Fatalf("parse Archidekt JSON: %v", err)
	}

	if data.Name == "" {
		data.Name = fmt.Sprintf("archidekt_%s", deckID)
	}

	// Separate commanders from mainboard.
	var commanders []string
	type cardLine struct {
		qty  int
		name string
	}
	var mainboard []cardLine
	var sideboard []cardLine

	for _, c := range data.Cards {
		name := c.Card.OracleCard.Name
		if name == "" {
			continue
		}
		if c.Quantity < 1 {
			c.Quantity = 1
		}

		isCommander := false
		isSideboard := false
		isToken := false
		for _, cat := range c.Categories {
			lower := strings.ToLower(cat)
			if lower == "commander" {
				isCommander = true
			}
			if lower == "sideboard" || lower == "maybeboard" {
				isSideboard = true
			}
			if lower == "tokens & extras" {
				isToken = true
			}
		}

		if isToken {
			continue // skip tokens and extras
		}
		if isCommander {
			commanders = append(commanders, name)
		} else if isSideboard {
			sideboard = append(sideboard, cardLine{c.Quantity, name})
		} else {
			mainboard = append(mainboard, cardLine{c.Quantity, name})
		}
	}

	// Sort mainboard alphabetically for consistent output.
	sort.Slice(mainboard, func(i, j int) bool {
		return mainboard[i].name < mainboard[j].name
	})

	// Build the output text.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n", data.Name))
	sb.WriteString(fmt.Sprintf("# Source: %s\n", url))

	for _, cmd := range commanders {
		sb.WriteString(fmt.Sprintf("COMMANDER: %s\n", cmd))
	}

	for _, c := range mainboard {
		sb.WriteString(fmt.Sprintf("%d %s\n", c.qty, c.name))
	}

	for _, c := range sideboard {
		sb.WriteString(fmt.Sprintf("// Sideboard: %d %s\n", c.qty, c.name))
	}

	text := sb.String()
	if strings.TrimSpace(text) == "" {
		log.Fatalf("Archidekt deck %q is empty", data.Name)
	}

	filename := sanitizeFilename(data.Name) + ".txt"
	outPath := filepath.Join(outputDir, filename)

	if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}
	log.Printf("saved %s (%d bytes)", outPath, len(text))

	// Print summary.
	mbCount := 0
	for _, c := range mainboard {
		mbCount += c.qty
	}
	log.Printf("  deck:       %s", data.Name)
	log.Printf("  commanders: %s", strings.Join(commanders, ", "))
	log.Printf("  mainboard:  %d cards", mbCount)
	log.Printf("  sideboard:  %d cards", len(sideboard))

	// Auto-run Freya analysis.
	runFreyaOnDeck(outPath)
}

// ---------------------------------------------------------------------------
// Freya auto-analysis
// ---------------------------------------------------------------------------

// runFreyaOnDeck shells out to the hexdek-freya binary (or falls back to
// go run) to auto-generate combo/synergy analysis for the imported deck.
// Freya saves its output to a freya/ subfolder next to the deck file.
func runFreyaOnDeck(deckPath string) {
	log.Printf("  [freya] running auto-analysis on %s ...", filepath.Base(deckPath))

	repoRoot := findRepoRoot()

	// Prefer a pre-built binary for speed; fall back to go run.
	freyaBin := filepath.Join(repoRoot, "cmd", "hexdek-freya", "hexdek-freya")
	var cmd *exec.Cmd
	if _, err := os.Stat(freyaBin); err == nil {
		cmd = exec.Command(freyaBin, "--deck", deckPath)
	} else {
		cmd = exec.Command("go", "run", "./cmd/hexdek-freya/", "--deck", deckPath)
	}

	cmd.Dir = repoRoot
	cmd.Stdout = os.Stderr // pipe Freya logs to stderr so they're visible
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("  [freya] auto-analysis failed: %v", err)
		return
	}
	log.Printf("  [freya] auto-analysis complete for %s", filepath.Base(deckPath))
}

// findRepoRoot walks up from the current working directory (or the
// executable's directory) looking for go.mod to locate the repo root.
func findRepoRoot() string {
	// Start from CWD.
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod.
			return "."
		}
		dir = parent
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// sanitizeFilename produces a safe filename from a deck name.
func sanitizeFilename(name string) string {
	name = strings.ToLower(name)
	// Replace common separators with underscores.
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	// Collapse runs of underscores.
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	name = strings.Trim(name, "_")
	if name == "" {
		name = "imported_deck"
	}
	// Truncate to something reasonable.
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}

func printDeckSummary(text string) {
	commanders := 0
	mainboard := 0
	sideboard := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COMMANDER:") {
			commanders++
		} else if strings.HasPrefix(line, "// Sideboard:") {
			sideboard++
		} else if strings.HasPrefix(line, "// Companion:") {
			// skip
		} else if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
			mainboard++
		}
	}
	log.Printf("  commanders: %d", commanders)
	log.Printf("  mainboard:  %d entries", mainboard)
	if sideboard > 0 {
		log.Printf("  sideboard:  %d entries", sideboard)
	}
}
