// Package oracle provides Scryfall card lookup with SQLite caching.
//
// Scryfall API (https://scryfall.com/docs/api) is the canonical card data
// source. We use the /cards/named?fuzzy= endpoint for individual lookups.
// Results are cached in SQLite so repeat lookups don't hit the network.
//
// Scryfall's rate limit is ~10 req/s. We don't hit that ceiling under
// normal use because each card is fetched at most once per server lifetime.
package oracle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

// scryfallBase is a var (not const) so tests can stub the API via
// SetScryfallBaseForTest. Production code never mutates it.
var scryfallBase = "https://api.scryfall.com"

// scryfallHTTPClient is shared across Lookup / LookupMany /
// fetchScryfallCollection so tests can inject a custom transport.
var scryfallHTTPClient = &http.Client{Timeout: 30 * time.Second}

// scryfallLimit gates the dispatch rate. A leaky bucket: callers may
// run in parallel but their request START times are spaced by interval.
// Default 120ms ≈ 8.3 req/s — under Scryfall's ~10 req/s ceiling.
var scryfallLimit = newScryfallLimiter(120 * time.Millisecond)

// scryfallParallelism caps how many Scryfall requests may be in flight
// concurrently from LookupMany. 8 workers + 120ms interval keeps us
// well under the 10 req/s ceiling while letting per-request latency
// (~150-400ms) be overlapped instead of stacked.
var scryfallParallelism = 8

// SetScryfallBaseForTest swaps the API base URL and returns a cleanup
// that restores it. Test-only escape hatch — production code does not
// call this. Pair with the test's t.Cleanup so other tests don't
// inherit the stub.
func SetScryfallBaseForTest(base string) func() {
	orig := scryfallBase
	scryfallBase = base
	return func() { scryfallBase = orig }
}

// SetScryfallLimitIntervalForTest swaps the dispatch interval and
// returns a cleanup. Test-only.
func SetScryfallLimitIntervalForTest(d time.Duration) func() {
	return scryfallLimit.setIntervalForTest(d)
}

// SetScryfallParallelismForTest swaps the worker-pool cap and returns
// a cleanup. Test-only.
func SetScryfallParallelismForTest(n int) func() {
	orig := scryfallParallelism
	scryfallParallelism = n
	return func() { scryfallParallelism = orig }
}

// Card is the slim subset of Scryfall data we use.
type Card struct {
	Name           string `json:"name"`
	ScryfallID     string `json:"scryfall_id"`
	ManaCost       string `json:"mana_cost"`
	CMC            int    `json:"cmc"`
	TypeLine       string `json:"type_line"`
	OracleText     string `json:"oracle_text"`
	ImageURINormal string `json:"image_uri_normal"`
	ImageURIArt    string `json:"image_uri_art"`
	SetCode        string `json:"set_code"`
	CachedAt       int64  `json:"cached_at"`

	// Legalities maps format slug (commander, brawl, modern, legacy,
	// vintage, pauper, pioneer, standard, etc.) to Scryfall's status
	// for that format: "legal", "not_legal", "banned", or "restricted".
	// May be nil/empty on cache hits written before the r60 legalities
	// migration — callers that need legality data should treat
	// nil/empty as "unknown" rather than as "all legal."
	Legalities map[string]string `json:"legalities,omitempty"`

	// Prices is Scryfall's prices block. Keys observed in the wild:
	// "usd", "usd_foil", "usd_etched", "eur", "eur_foil", "tix". Values
	// are strings (Scryfall returns dollar amounts with two decimal
	// places, or null for unpriced cards — represented here as an
	// absent key, NOT the literal string "null"). May be nil/empty on
	// cache hits written before the r60 prices migration; the deck
	// budget endpoint treats nil/empty as "needs refresh" and triggers
	// a fresh Scryfall fetch.
	Prices map[string]string `json:"prices,omitempty"`
}

// HasPrices reports whether the card carries at least one non-empty
// price entry. Pre-migration cache hits and Scryfall responses for
// unpriced cards both fail this check; callers use it to decide
// whether a re-fetch is worth a network round-trip.
func (c *Card) HasPrices() bool {
	if c == nil || len(c.Prices) == 0 {
		return false
	}
	for _, v := range c.Prices {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// Lookup returns card data for the given name. Tries the SQLite cache
// first, falling back to Scryfall API on miss. Returns ErrNotFound if
// Scryfall has no match.
//
// Scryfall's public API asks for ~10 req/s ceiling. We dispatch through
// scryfallLimit (default 120ms between request starts) which lets
// parallel callers run concurrently while keeping the rate well under
// the ceiling. The cache is re-checked AFTER limiter Wait — another
// goroutine may have filled it while we were queued.
func Lookup(ctx context.Context, database *sql.DB, name string) (*Card, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil, fmt.Errorf("empty name")
	}

	// Cache hit?
	if c, err := getCached(ctx, database, key); err == nil {
		return c, nil
	}

	if err := scryfallLimit.Wait(ctx); err != nil {
		return nil, err
	}

	// Re-check cache — a sibling worker may have filled it while we
	// queued at the limiter.
	if c, err := getCached(ctx, database, key); err == nil {
		return c, nil
	}

	// Fetch from Scryfall, with one quick retry on transient failure.
	// The retry observes the limiter so its dispatch is rate-counted.
	c, err := fetchScryfall(ctx, name)
	if err != nil && err != ErrNotFound {
		if werr := scryfallLimit.Wait(ctx); werr == nil {
			c, err = fetchScryfall(ctx, name)
		}
	}
	if err != nil {
		return nil, err
	}
	if err := saveToCache(ctx, database, key, c); err != nil {
		// A persistent cache-write failure (disk full, schema mismatch,
		// locked db) silently disables the cache and causes Scryfall API
		// hammering on every subsequent lookup. Surface it so an operator
		// notices instead of silently burning through the ~10 req/s
		// ceiling. The fresh card is still returned to the caller.
		log.Printf("oracle: cache write for %q failed: %v", key, err)
		cacheWriteFailures.Add(1)
	}
	return c, nil
}

// cacheWriteFailures is an atomic counter of saveToCache errors observed
// by Lookup / LookupMany. Exported via CacheWriteFailures so tests and
// any future /healthz probe can read it. Reset via test code only.
var cacheWriteFailures atomicCounter

// CacheWriteFailures returns the number of saveToCache errors observed by
// Lookup / LookupMany since process start.
func CacheWriteFailures() int64 { return cacheWriteFailures.Load() }

// atomicCounter is a tiny int64 counter using sync/atomic semantics; kept
// inline to avoid a one-off package dependency.
type atomicCounter struct {
	mu sync.Mutex
	n  int64
}

func (c *atomicCounter) Add(d int64) {
	c.mu.Lock()
	c.n += d
	c.mu.Unlock()
}

func (c *atomicCounter) Load() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func (c *atomicCounter) reset() {
	c.mu.Lock()
	c.n = 0
	c.mu.Unlock()
}

// LookupMany fetches multiple cards. Missing ones are fetched via
// Scryfall's bulk /cards/collection endpoint (up to 75 cards per
// request, one API call for the whole deck in the common case) rather
// than 67 serial named lookups. Returns a map keyed by lowercased
// name → card.
//
// Two layers of concurrency:
//
//  1. Bulk chunks (every 75 missing names) dispatched in parallel,
//     bounded by scryfallParallelism. Common case (≤75 misses) is
//     still a single request.
//  2. Fuzzy fallback for names the bulk endpoint can't resolve (DFC
//     edge cases, alt spellings) runs in a worker pool sharing the
//     same parallelism cap.
//
// Both layers feed through scryfallLimit so dispatch stays under the
// ~10 req/s Scryfall ceiling. Wall time for a 99-card first-import
// drops from "all chunks + all fallbacks serial" to roughly
// (num_chunks + num_fallbacks) * 120ms + max_RTT.
func LookupMany(ctx context.Context, database *sql.DB, names []string) map[string]*Card {
	out := make(map[string]*Card, len(names))
	var outMu sync.Mutex

	// Dedupe missing names too: duplicates in `names` (which happens
	// when a caller passes a raw decklist with repeated basic lands)
	// would otherwise schedule two concurrent fetches for the same name.
	seen := map[string]bool{}
	missing := []string{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if c, err := getCached(ctx, database, key); err == nil {
			out[key] = c
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return out
	}

	// Build chunks of 75 (Scryfall's documented ceiling).
	var chunks [][]string
	for i := 0; i < len(missing); i += 75 {
		end := i + 75
		if end > len(missing) {
			end = len(missing)
		}
		chunks = append(chunks, missing[i:end])
	}

	// Layer 1: parallel bulk-chunk fetches.
	var stillMissingMu sync.Mutex
	stillMissing := []string{}
	parallelism := scryfallParallelism
	if parallelism < 1 {
		parallelism = 1
	}
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	for _, chunk := range chunks {
		chunk := chunk
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fetched, err := fetchScryfallCollection(ctx, chunk)
			if err != nil {
				stillMissingMu.Lock()
				stillMissing = append(stillMissing, chunk...)
				stillMissingMu.Unlock()
				return
			}
			for name, card := range fetched {
				outMu.Lock()
				out[name] = card
				outMu.Unlock()
				if err := saveToCache(ctx, database, name, card); err != nil {
					log.Printf("oracle: cache write for %q failed: %v", name, err)
					cacheWriteFailures.Add(1)
				}
			}
			for _, n := range chunk {
				if _, ok := fetched[strings.ToLower(strings.TrimSpace(n))]; !ok {
					stillMissingMu.Lock()
					stillMissing = append(stillMissing, n)
					stillMissingMu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	// Layer 2: parallel fuzzy fallback for unresolved names.
	for _, n := range stillMissing {
		n := n
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			c, lerr := Lookup(ctx, database, n)
			if lerr != nil {
				return
			}
			outMu.Lock()
			// Index by the lookup-key the caller asked for AND by the
			// canonical name Scryfall returned, since they differ for
			// DFCs ("Wedding Announcement" → "Wedding Announcement //
			// Wedding Festivity").
			out[strings.ToLower(strings.TrimSpace(n))] = c
			out[strings.ToLower(strings.TrimSpace(c.Name))] = c
			outMu.Unlock()
		}()
	}
	wg.Wait()

	return out
}

// RefreshPrices ensures every name in `names` has a cached card with
// at least one non-empty price entry. Cards already priced are
// returned from cache untouched; cards with missing or empty prices
// (typically pre-r60-migration cache rows) are re-fetched from
// Scryfall's collection endpoint and saved back into cache.
//
// Used by the deck budget endpoint to backfill prices on demand without
// thrashing the rate-limit budget on the legality-only LookupMany
// path. Returns the merged map of name → Card (lowercased name keys,
// matching LookupMany's convention).
func RefreshPrices(ctx context.Context, database *sql.DB, names []string) map[string]*Card {
	out := make(map[string]*Card, len(names))
	if database == nil {
		return out
	}
	var needs []string
	seen := map[string]bool{}
	for _, n := range names {
		key := strings.ToLower(strings.TrimSpace(n))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if c, err := getCached(ctx, database, key); err == nil && c.HasPrices() {
			out[key] = c
			continue
		}
		needs = append(needs, n)
	}
	if len(needs) == 0 {
		return out
	}

	// Chunk by 75 (Scryfall's limit). Use the same parallelism as
	// LookupMany so a 99-card deck refresh runs in ≤ 2 round-trips.
	var chunks [][]string
	for i := 0; i < len(needs); i += 75 {
		end := i + 75
		if end > len(needs) {
			end = len(needs)
		}
		chunks = append(chunks, needs[i:end])
	}
	parallelism := scryfallParallelism
	if parallelism < 1 {
		parallelism = 1
	}
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	var outMu sync.Mutex
	for _, chunk := range chunks {
		chunk := chunk
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fetched, err := fetchScryfallCollection(ctx, chunk)
			if err != nil {
				return
			}
			for key, card := range fetched {
				outMu.Lock()
				out[key] = card
				outMu.Unlock()
				if err := saveToCache(ctx, database, key, card); err != nil {
					log.Printf("oracle: cache write for %q failed: %v", key, err)
					cacheWriteFailures.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	return out
}

// fetchScryfallCollection POSTs a batch of card identifiers to Scryfall's
// /cards/collection endpoint. Goes through scryfallLimit for dispatch
// rate-limiting; concurrent callers run in parallel but their request
// START times are spaced by the limiter's interval.
func fetchScryfallCollection(ctx context.Context, names []string) (map[string]*Card, error) {
	if err := scryfallLimit.Wait(ctx); err != nil {
		return nil, err
	}

	type ident struct {
		Name string `json:"name"`
	}
	body := struct {
		Identifiers []ident `json:"identifiers"`
	}{}
	for _, n := range names {
		body.Identifiers = append(body.Identifiers, ident{Name: n})
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", scryfallBase+"/cards/collection", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "hexdek/0.1 (https://github.com/hexdek/hexdek)")

	resp, err := scryfallHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scryfall collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		retry := 5 * time.Second
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, perr := strconv.Atoi(ra); perr == nil && secs > 0 && secs < 120 {
				retry = time.Duration(secs) * time.Second
			}
		}
		select {
		case <-time.After(retry):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("scryfall collection: rate limited")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("scryfall collection: HTTP %d", resp.StatusCode)
	}

	var cr struct {
		Data     []scryfallNamedResp `json:"data"`
		NotFound []map[string]string `json:"not_found"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("scryfall collection decode: %w", err)
	}

	out := map[string]*Card{}
	for _, sr := range cr.Data {
		imgNormal := sr.ImageURIs.Normal
		imgArt := sr.ImageURIs.ArtCrop
		if imgNormal == "" && len(sr.CardFaces) > 0 {
			imgNormal = sr.CardFaces[0].ImageURIs.Normal
			imgArt = sr.CardFaces[0].ImageURIs.ArtCrop
		}
		key := strings.ToLower(strings.TrimSpace(sr.Name))
		out[key] = &Card{
			Name:           sr.Name,
			ScryfallID:     sr.ID,
			ManaCost:       sr.ManaCost,
			CMC:            int(sr.CMC),
			TypeLine:       sr.TypeLine,
			OracleText:     sr.OracleText,
			ImageURINormal: imgNormal,
			ImageURIArt:    imgArt,
			SetCode:        sr.Set,
			CachedAt:       db.Now(),
			Legalities:     sr.Legalities,
			Prices:         filterEmptyPrices(sr.Prices),
		}
	}
	return out, nil
}

// ----- internals -----

type scryfallNamedResp struct {
	Object     string `json:"object"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	ManaCost   string `json:"mana_cost"`
	CMC        float64 `json:"cmc"`
	TypeLine   string `json:"type_line"`
	OracleText string `json:"oracle_text"`
	Set        string `json:"set"`
	ImageURIs  struct {
		Normal  string `json:"normal"`
		ArtCrop string `json:"art_crop"`
	} `json:"image_uris"`
	CardFaces []struct {
		ImageURIs struct {
			Normal  string `json:"normal"`
			ArtCrop string `json:"art_crop"`
		} `json:"image_uris"`
	} `json:"card_faces"`
	Legalities map[string]string `json:"legalities"`
	// Scryfall serializes per-currency prices as a string-keyed object;
	// entries may be JSON null (omitted from this map after decode) or
	// strings like "0.50" / "12.34" / "1289.99".
	Prices map[string]string `json:"prices"`
}

// ErrNotFound is returned when Scryfall has no match for the queried name.
var ErrNotFound = fmt.Errorf("oracle: card not found")

func fetchScryfall(ctx context.Context, name string) (*Card, error) {
	endpoint := fmt.Sprintf("%s/cards/named?fuzzy=%s", scryfallBase, url.QueryEscape(name))
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "hexdek/0.1 (https://github.com/hexdek/hexdek)")

	resp, err := scryfallHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scryfall: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if resp.StatusCode == 429 {
		// Rate limited — respect Retry-After (in seconds), then bubble up the
		// error so the caller can retry. Default 5s if header missing.
		retry := 5 * time.Second
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, perr := strconv.Atoi(ra); perr == nil && secs > 0 && secs < 120 {
				retry = time.Duration(secs) * time.Second
			}
		}
		select {
		case <-time.After(retry):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("scryfall: rate limited (retried after %s)", retry)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("scryfall: HTTP %d", resp.StatusCode)
	}

	var sr scryfallNamedResp
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("scryfall decode: %w", err)
	}

	imgNormal := sr.ImageURIs.Normal
	imgArt := sr.ImageURIs.ArtCrop
	if imgNormal == "" && len(sr.CardFaces) > 0 {
		imgNormal = sr.CardFaces[0].ImageURIs.Normal
		imgArt = sr.CardFaces[0].ImageURIs.ArtCrop
	}

	return &Card{
		Name:           sr.Name,
		ScryfallID:     sr.ID,
		ManaCost:       sr.ManaCost,
		CMC:            int(sr.CMC),
		TypeLine:       sr.TypeLine,
		OracleText:     sr.OracleText,
		ImageURINormal: imgNormal,
		ImageURIArt:    imgArt,
		SetCode:        sr.Set,
		CachedAt:       db.Now(),
		Legalities:     sr.Legalities,
		Prices:         filterEmptyPrices(sr.Prices),
	}, nil
}

// filterEmptyPrices drops zero-value entries from Scryfall's prices
// map so HasPrices doesn't false-positive on a {"usd":""} entry that
// indicates the card has no recorded market price.
func filterEmptyPrices(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func getCached(ctx context.Context, database *sql.DB, key string) (*Card, error) {
	c := &Card{}
	var legalitiesJSON, pricesJSON string
	err := database.QueryRowContext(ctx,
		`SELECT display_name, scryfall_id, mana_cost, cmc, type_line, oracle_text,
		        image_uri_normal, image_uri_art, set_code, cached_at, legalities, prices
		 FROM card_oracle WHERE name = ?`, key,
	).Scan(&c.Name, &c.ScryfallID, &c.ManaCost, &c.CMC, &c.TypeLine, &c.OracleText,
		&c.ImageURINormal, &c.ImageURIArt, &c.SetCode, &c.CachedAt, &legalitiesJSON, &pricesJSON)
	if err != nil {
		return c, err
	}
	// Empty string is the pre-migration default; treat it as "no
	// legality data" without erroring. Malformed JSON also degrades to
	// nil rather than failing the whole cache read — losing legality
	// info beats losing the card.
	if legalitiesJSON != "" {
		var leg map[string]string
		if jerr := json.Unmarshal([]byte(legalitiesJSON), &leg); jerr == nil {
			c.Legalities = leg
		}
	}
	// Same fault tolerance for prices: empty = pre-migration row (deck
	// budget endpoint will trigger a fresh fetch); malformed JSON
	// degrades to nil rather than dropping the cache hit.
	if pricesJSON != "" {
		var pr map[string]string
		if jerr := json.Unmarshal([]byte(pricesJSON), &pr); jerr == nil {
			c.Prices = pr
		}
	}
	return c, nil
}

func saveToCache(ctx context.Context, database *sql.DB, key string, c *Card) error {
	legalitiesJSON := ""
	if len(c.Legalities) > 0 {
		if b, jerr := json.Marshal(c.Legalities); jerr == nil {
			legalitiesJSON = string(b)
		}
	}
	pricesJSON := ""
	if len(c.Prices) > 0 {
		if b, jerr := json.Marshal(c.Prices); jerr == nil {
			pricesJSON = string(b)
		}
	}
	_, err := database.ExecContext(ctx,
		`INSERT OR REPLACE INTO card_oracle
		 (name, display_name, scryfall_id, mana_cost, cmc, type_line, oracle_text,
		  image_uri_normal, image_uri_art, set_code, cached_at, legalities, prices)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key, c.Name, c.ScryfallID, c.ManaCost, c.CMC, c.TypeLine, c.OracleText,
		c.ImageURINormal, c.ImageURIArt, c.SetCode, c.CachedAt, legalitiesJSON, pricesJSON)
	return err
}
