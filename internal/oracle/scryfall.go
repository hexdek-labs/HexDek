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
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

const scryfallBase = "https://api.scryfall.com"

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
}

// Lookup returns card data for the given name. Tries the SQLite cache
// first, falling back to Scryfall API on miss. Returns ErrNotFound if
// Scryfall has no match.
//
// Scryfall's public API asks for ~10 req/s ceiling. We serialize misses
// through a mutex + 120ms gate so a burst (e.g., importing a 99-card deck
// for the first time) can't trip their rate limiter.
func Lookup(ctx context.Context, database *sql.DB, name string) (*Card, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil, fmt.Errorf("empty name")
	}

	// Cache hit?
	if c, err := getCached(ctx, database, key); err == nil {
		return c, nil
	}

	scryfallGate.Lock()
	defer scryfallGate.Unlock()

	// Re-check cache under lock — another caller may have filled it while
	// we waited.
	if c, err := getCached(ctx, database, key); err == nil {
		return c, nil
	}

	// Honour 120ms since last miss. On a fresh process this is a no-op.
	if elapsed := time.Since(lastScryfallHit); elapsed < scryfallInterval {
		select {
		case <-time.After(scryfallInterval - elapsed):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	lastScryfallHit = time.Now()

	// Fetch from Scryfall, with one quick retry on transient failure.
	c, err := fetchScryfall(ctx, name)
	if err != nil && err != ErrNotFound {
		time.Sleep(500 * time.Millisecond)
		c, err = fetchScryfall(ctx, name)
	}
	if err != nil {
		return nil, err
	}
	if err := saveToCache(ctx, database, key, c); err != nil {
		_ = err
	}
	return c, nil
}

var (
	scryfallGate     sync.Mutex
	lastScryfallHit  time.Time
	scryfallInterval = 120 * time.Millisecond
)

// LookupMany fetches multiple cards. Missing ones are fetched via Scryfall's
// bulk /cards/collection endpoint (up to 75 cards per request, one API call
// for the whole deck in the common case) rather than 67 serial named
// lookups. Returns a map keyed by lowercased name → card.
func LookupMany(ctx context.Context, database *sql.DB, names []string) map[string]*Card {
	out := make(map[string]*Card, len(names))
	missing := []string{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if c, err := getCached(ctx, database, key); err == nil {
			out[key] = c
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return out
	}

	// Batch misses in chunks of 75 (Scryfall's documented ceiling).
	// The collection endpoint does exact-name matching only — for anything
	// it returns as not_found (DFCs, alt spellings, partial names), fall
	// back to the fuzzy /cards/named endpoint which is what Lookup uses.
	stillMissing := []string{}
	for i := 0; i < len(missing); i += 75 {
		end := i + 75
		if end > len(missing) {
			end = len(missing)
		}
		chunk := missing[i:end]
		fetched, err := fetchScryfallCollection(ctx, chunk)
		if err != nil {
			stillMissing = append(stillMissing, chunk...)
			continue
		}
		for name, card := range fetched {
			out[name] = card
			_ = saveToCache(ctx, database, name, card)
		}
		// Detect which names in this chunk didn't come back.
		for _, n := range chunk {
			if _, ok := fetched[strings.ToLower(strings.TrimSpace(n))]; !ok {
				stillMissing = append(stillMissing, n)
			}
		}
	}
	// Fuzzy fallback for any still missing.
	for _, n := range stillMissing {
		if c, lerr := Lookup(ctx, database, n); lerr == nil {
			out[strings.ToLower(strings.TrimSpace(n))] = c
			// Also index the lookup-name key if it differs from the canonical
			// name (e.g., "Wedding Announcement" → canonical "Wedding
			// Announcement // Wedding Festivity"). Callers may look up by
			// either.
			out[strings.ToLower(strings.TrimSpace(c.Name))] = c
		}
	}
	return out
}

// fetchScryfallCollection POSTs a batch of card identifiers to Scryfall's
// /cards/collection endpoint. Respects the shared scryfallGate and
// Retry-After on 429.
func fetchScryfallCollection(ctx context.Context, names []string) (map[string]*Card, error) {
	scryfallGate.Lock()
	defer scryfallGate.Unlock()

	if elapsed := time.Since(lastScryfallHit); elapsed < scryfallInterval {
		select {
		case <-time.After(scryfallInterval - elapsed):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	lastScryfallHit = time.Now()

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
	req.Header.Set("User-Agent", "mtgsquad/0.1 (https://github.com/hexdek/hexdek)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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
	req.Header.Set("User-Agent", "mtgsquad/0.1 (https://github.com/hexdek/hexdek)")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
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
	}, nil
}

func getCached(ctx context.Context, database *sql.DB, key string) (*Card, error) {
	c := &Card{}
	err := database.QueryRowContext(ctx,
		`SELECT display_name, scryfall_id, mana_cost, cmc, type_line, oracle_text,
		        image_uri_normal, image_uri_art, set_code, cached_at
		 FROM card_oracle WHERE name = ?`, key,
	).Scan(&c.Name, &c.ScryfallID, &c.ManaCost, &c.CMC, &c.TypeLine, &c.OracleText,
		&c.ImageURINormal, &c.ImageURIArt, &c.SetCode, &c.CachedAt)
	return c, err
}

func saveToCache(ctx context.Context, database *sql.DB, key string, c *Card) error {
	_, err := database.ExecContext(ctx,
		`INSERT OR REPLACE INTO card_oracle
		 (name, display_name, scryfall_id, mana_cost, cmc, type_line, oracle_text,
		  image_uri_normal, image_uri_art, set_code, cached_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key, c.Name, c.ScryfallID, c.ManaCost, c.CMC, c.TypeLine, c.OracleText,
		c.ImageURINormal, c.ImageURIArt, c.SetCode, c.CachedAt)
	return err
}
