package hexapi

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ArtCache serves card art images from a local disk cache, falling back to
// a 302 redirect to Scryfall when the image is not cached. This is intended
// to replace the inline handleCardArt once the prefetcher has warmed the
// cache, avoiding runtime Scryfall fetches from the server process.
//
// Not yet wired to a route — call ArtCacheHandler(cacheDir) to get the
// handler, then register it on your mux as needed.
//
// Cache layout: {cacheDir}/{sha256(lowercase(cardName))}.jpg
//
// Response headers:
//   - Cache-Control: public, max-age=2592000 (30 days)
//   - Content-Type: image/jpeg (for cached hits)
//
// URL pattern: GET /api/card-art/{name}
func ArtCacheHandler(cacheDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(w, http.StatusBadRequest, "missing card name")
			return
		}

		// Normalize the same way the existing handler and prefetcher do.
		clean := strings.TrimSpace(strings.Split(name, "//")[0])
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(clean))))

		// Try disk cache.
		cachePath := filepath.Join(cacheDir, hash+".jpg")
		data, err := os.ReadFile(cachePath)
		if err == nil && len(data) > 0 {
			// Also populate the in-memory cache if available.
			artMemCache.Store(hash, data)

			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "public, max-age=2592000")
			_, _ = w.Write(data) // ResponseWriter.Write error means client closed the connection — nothing to do.
			return
		}

		// Not cached — redirect to Scryfall so the client still gets art.
		// The prefetcher will eventually fill this gap.
		scryfallURL := "https://api.scryfall.com/cards/named?exact=" +
			url.QueryEscape(clean) + "&format=image&version=art_crop"
		w.Header().Set("Cache-Control", "public, max-age=3600") // short cache for redirect
		http.Redirect(w, r, scryfallURL, http.StatusFound)
	}
}
