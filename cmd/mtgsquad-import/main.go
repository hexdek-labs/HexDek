// mtgsquad-import — Deck import CLI for Moxfield and Archidekt.
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
//	mtgsquad-import --moxfield https://www.moxfield.com/decks/XXXXXX
//	mtgsquad-import --archidekt https://archidekt.com/decks/12345/deckname
//	mtgsquad-import --moxfield URL --output data/decks/josh
package main

import (
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

	"github.com/hexdek/hexdek/internal/moxfield"
)

func main() {
	moxfieldURL := flag.String("moxfield", "", "Moxfield deck URL to import")
	archidektURL := flag.String("archidekt", "", "Archidekt deck URL to import")
	outputDir := flag.String("output", "", "output directory for deck files (overrides --owner)")
	owner := flag.String("owner", "imported", "owner/group folder name for the deck (saves to data/decks/<owner>/)")
	flag.Parse()

	if *moxfieldURL == "" && *archidektURL == "" {
		fmt.Println("mtgsquad-import — import decks from Moxfield and Archidekt")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  mtgsquad-import --moxfield https://www.moxfield.com/decks/XXXXXX")
		fmt.Println("  mtgsquad-import --archidekt https://archidekt.com/decks/12345/deckname")
		fmt.Println("  mtgsquad-import --archidekt URL --owner lyon")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --moxfield URL    Moxfield deck URL to import")
		fmt.Println("  --archidekt URL   Archidekt deck URL to import")
		fmt.Println("  --owner NAME      Owner/group folder (default: imported)")
		fmt.Println("  --output DIR      Output directory (overrides --owner)")
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
	log.Printf("importing from Moxfield: %s", url)

	deckID := moxfield.ExtractDeckID(url)
	if deckID == "" {
		log.Fatalf("could not extract deck ID from URL: %s", url)
	}

	// Fetch the deck name separately for the header comment.
	deckName, nameErr := moxfield.FetchDeckName(url)
	if nameErr != nil {
		deckName = deckID // fallback to ID
	}

	// Fetch the full decklist text (COMMANDER: + mainboard lines).
	text, err := moxfield.FetchDeck(url)
	if err != nil {
		log.Fatalf("fetch Moxfield deck: %v", err)
	}

	// Build the output with header comments.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n", deckName))
	sb.WriteString(fmt.Sprintf("# Source: %s\n", url))
	sb.WriteString(text)

	// Sanitize name for filename.
	filename := sanitizeFilename(deckName) + ".txt"
	outPath := filepath.Join(outputDir, filename)

	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}
	log.Printf("saved %s (%d bytes)", outPath, len(sb.String()))
	printDeckSummary(text)

	// Auto-run Freya analysis.
	runFreyaOnDeck(outPath)
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
	req.Header.Set("User-Agent", "mtgsquad/1.0")
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

// runFreyaOnDeck shells out to the mtgsquad-freya binary (or falls back to
// go run) to auto-generate combo/synergy analysis for the imported deck.
// Freya saves its output to a freya/ subfolder next to the deck file.
func runFreyaOnDeck(deckPath string) {
	log.Printf("  [freya] running auto-analysis on %s ...", filepath.Base(deckPath))

	repoRoot := findRepoRoot()

	// Prefer a pre-built binary for speed; fall back to go run.
	freyaBin := filepath.Join(repoRoot, "cmd", "mtgsquad-freya", "mtgsquad-freya")
	var cmd *exec.Cmd
	if _, err := os.Stat(freyaBin); err == nil {
		cmd = exec.Command(freyaBin, "--deck", deckPath)
	} else {
		cmd = exec.Command("go", "run", "./cmd/mtgsquad-freya/", "--deck", deckPath)
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
