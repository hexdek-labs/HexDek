// hexdek-artfetch prefetches Scryfall card art for every unique card name
// found across all deck files in data/decks/. Images are stored in
// data/cache/art/ using content-addressable filenames (SHA-256 of the
// lowercase card name → .jpg). Already-cached images are skipped.
//
// Scryfall rate limit: 50-100ms between requests. We use 75ms.
//
// Usage:
//
//	hexdek-artfetch                    # prefetch all decks
//	hexdek-artfetch -decks data/decks  # custom decks dir
//	hexdek-artfetch -cache data/cache/art  # custom cache dir
//	hexdek-artfetch -dry-run           # list uncached cards without fetching
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	decksDir = flag.String("decks", "data/decks", "path to decks directory")
	cacheDir = flag.String("cache", "data/cache/art", "path to art cache directory")
	dryRun   = flag.Bool("dry-run", false, "list uncached cards without downloading")
	workers  = flag.Int("workers", 1, "concurrent download workers (keep 1 to respect rate limit)")
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		host := req.URL.Hostname()
		if !strings.HasSuffix(host, "scryfall.com") && !strings.HasSuffix(host, "scryfall.io") {
			return fmt.Errorf("redirect to disallowed host: %s", host)
		}
		return nil
	},
}

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime)

	// 1. Scan all deck files and collect unique card names.
	cards := scanDecks(*decksDir)
	log.Printf("found %d unique card names across all decks", len(cards))

	// 2. Determine which are already cached.
	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		log.Fatalf("cannot create cache dir %s: %v", *cacheDir, err)
	}

	var uncached []string
	cached := 0
	for _, name := range cards {
		hash := hashName(name)
		path := filepath.Join(*cacheDir, hash+".jpg")
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			cached++
		} else {
			uncached = append(uncached, name)
		}
	}

	log.Printf("%d already cached, %d to fetch", cached, len(uncached))

	if *dryRun {
		for _, name := range uncached {
			fmt.Println(name)
		}
		return
	}

	if len(uncached) == 0 {
		log.Println("nothing to do")
		return
	}

	// 3. Download uncached art. Workers share a single rate-limit ticker so
	// the total request rate across all goroutines stays under Scryfall's
	// 50–100 ms cadence regardless of --workers.
	n := *workers
	if n < 1 {
		n = 1
	}
	if n > len(uncached) {
		n = len(uncached)
	}

	ticker := time.NewTicker(75 * time.Millisecond)
	defer ticker.Stop()

	work := make(chan struct {
		idx  int
		name string
	}, n)

	var errors int64
	var wg sync.WaitGroup
	for w := 0; w < n; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				<-ticker.C
				if err := fetchArt(item.name, *cacheDir); err != nil {
					log.Printf("[%d/%d] FAIL  %s: %v", item.idx+1, len(uncached), item.name, err)
					atomic.AddInt64(&errors, 1)
				} else {
					log.Printf("[%d/%d] OK    %s", item.idx+1, len(uncached), item.name)
				}
			}
		}()
	}
	for i, name := range uncached {
		work <- struct {
			idx  int
			name string
		}{i, name}
	}
	close(work)
	wg.Wait()

	errCount := int(atomic.LoadInt64(&errors))
	log.Printf("done: %d fetched, %d errors, %d total cached",
		len(uncached)-errCount, errCount, cached+len(uncached)-errCount)
}

// scanDecks walks the decks directory and returns a sorted, deduplicated
// list of card names found across all .txt and .json deck files.
func scanDecks(root string) []string {
	seen := make(map[string]struct{})

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip version/analysis/metadata directories.
		if info.IsDir() {
			base := info.Name()
			if base == "versions" || base == "freya" || base == ".versions" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".txt" && ext != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var names []string
		if ext == ".json" {
			names = parseJSONDeck(data)
		} else {
			names = parseTXTDeck(string(data))
		}

		for _, n := range names {
			// Normalize: take the front face for double-faced cards,
			// trim whitespace, lowercase for dedup.
			clean := cleanCardName(n)
			if clean != "" {
				seen[clean] = struct{}{}
			}
		}
		return nil
	})

	cards := make([]string, 0, len(seen))
	for name := range seen {
		cards = append(cards, name)
	}
	sort.Strings(cards)
	return cards
}

// parseJSONDeck extracts card names from a JSON deck file.
func parseJSONDeck(data []byte) []string {
	var deck struct {
		Commander string `json:"commander"`
		Mainboard []struct {
			Name string `json:"name"`
		} `json:"mainboard"`
	}
	if err := json.Unmarshal(data, &deck); err != nil {
		return nil
	}
	var names []string
	if deck.Commander != "" {
		names = append(names, deck.Commander)
	}
	for _, c := range deck.Mainboard {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	return names
}

// parseTXTDeck extracts card names from a text deck file.
// Format: "N CardName" or "COMMANDER: CardName" or just "CardName".
func parseTXTDeck(content string) []string {
	var names []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "COMMANDER:") {
			name := strings.TrimSpace(line[len("COMMANDER:"):])
			if name != "" {
				names = append(names, name)
			}
			continue
		}
		if strings.HasPrefix(upper, "PARTNER:") {
			name := strings.TrimSpace(line[len("PARTNER:"):])
			if name != "" {
				names = append(names, name)
			}
			continue
		}

		// Try "N CardName" format.
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 && isNumber(parts[0]) {
			name := parts[1]
			// Strip set code suffix like "(NEO)" or "(NEO 123)".
			if idx := strings.Index(name, "("); idx > 0 {
				name = strings.TrimSpace(name[:idx])
			}
			if name != "" {
				names = append(names, name)
			}
		} else {
			// Bare card name.
			if idx := strings.Index(line, "("); idx > 0 {
				line = strings.TrimSpace(line[:idx])
			}
			if line != "" {
				names = append(names, line)
			}
		}
	}
	return names
}

// cleanCardName normalizes a card name for cache keying.
// For double-faced cards like "Tergrid, God of Fright // Tergrid's Lantern",
// we use the front face only (matching the existing handleCardArt behavior).
func cleanCardName(name string) string {
	// Take front face of DFC.
	if idx := strings.Index(name, "//"); idx >= 0 {
		name = name[:idx]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return name
}

// hashName returns the SHA-256 hex digest of the lowercase card name.
// This matches the cache key used by handleCardArt in hexapi/handler.go.
func hashName(name string) string {
	// The existing handler hashes the raw URL-decoded path value (lowercased).
	// We match that: sha256(lowercase(name)).
	h := sha256.Sum256([]byte(strings.ToLower(name)))
	return fmt.Sprintf("%x", h)
}

// fetchArt downloads a card's art_crop image from Scryfall and writes it
// to the cache directory.
func fetchArt(name, cacheDir string) error {
	// Use exact match (not fuzzy) to avoid wrong-card matches.
	scryfallURL := "https://api.scryfall.com/cards/named?exact=" +
		url.QueryEscape(name) + "&format=image&version=art_crop"

	req, err := http.NewRequest("GET", scryfallURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "HexDek-ArtFetch/1.0 (hexdek bulk art prefetcher; https://hexdek.dev)")
	req.Header.Set("Accept", "image/*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Card not found — try fuzzy as fallback for names with slight
		// variations (e.g., accented characters).
		return fetchArtFuzzy(name, cacheDir)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("scryfall returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("empty response")
	}

	hash := hashName(name)
	cachePath := filepath.Join(cacheDir, hash+".jpg")
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return nil
}

// fetchArtFuzzy retries with fuzzy matching for cards that fail exact lookup.
func fetchArtFuzzy(name, cacheDir string) error {
	scryfallURL := "https://api.scryfall.com/cards/named?fuzzy=" +
		url.QueryEscape(name) + "&format=image&version=art_crop"

	// Extra delay for the retry request.
	time.Sleep(75 * time.Millisecond)

	req, err := http.NewRequest("GET", scryfallURL, nil)
	if err != nil {
		return fmt.Errorf("build fuzzy request: %w", err)
	}
	req.Header.Set("User-Agent", "HexDek-ArtFetch/1.0 (hexdek bulk art prefetcher; https://hexdek.dev)")
	req.Header.Set("Accept", "image/*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fuzzy fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("scryfall returned %d (fuzzy)", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return fmt.Errorf("read fuzzy body: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("empty fuzzy response")
	}

	hash := hashName(name)
	cachePath := filepath.Join(cacheDir, hash+".jpg")
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return nil
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
