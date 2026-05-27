package oracle

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Handler exposes HTTP endpoints for the card oracle.
type Handler struct {
	DB *sql.DB
	// Limiter throttles inbound /api/oracle/card/{name} requests per
	// client IP. When nil, Register installs a default 60-burst,
	// 1 req/sec refill (≈ 60 req/min steady-state) limiter so the
	// public endpoint is never deployed unprotected.
	Limiter *InboundRateLimiter
	// Cache is an in-memory LRU+TTL response cache for successful
	// lookups. When nil, Register installs a default 1000-entry / 1h
	// cache. Hot cards (basic lands, Sol Ring, fetches) skip the
	// SQLite roundtrip entirely.
	Cache *ResponseCache
}

// Register adds oracle routes to the mux. If h.Limiter is nil, a
// default per-IP limiter (60 burst, 60 req/min steady) is installed —
// the oracle endpoint is unauthenticated and must never be exposed
// without throttling. If h.Cache is nil, a default 1000-entry / 1h
// response cache is installed.
func (h *Handler) Register(mux *http.ServeMux) {
	if h.Limiter == nil {
		h.Limiter = NewInboundRateLimiter(60, 1.0)
	}
	if h.Cache == nil {
		h.Cache = NewResponseCache(1000, time.Hour)
	}
	mux.HandleFunc("GET /api/oracle/card/{name}", h.lookupCard)
}

func (h *Handler) lookupCard(w http.ResponseWriter, r *http.Request) {
	if h.Limiter.enforce(w, r, "oracle lookup") {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		writeJSONErr(w, http.StatusBadRequest, "name required")
		return
	}

	// Cache fast path: hit returns immediately without touching SQLite
	// or the Scryfall outbound limiter. X-Cache header lets ops verify
	// the cache is working from devtools / curl without poking internals.
	if card, ok := h.Cache.Get(name); ok {
		writeCardJSON(w, card, "HIT")
		return
	}

	card, err := Lookup(r.Context(), h.DB, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSONErr(w, http.StatusNotFound, "card not found")
			return
		}
		writeJSONErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Only cache successful (positive) lookups. NotFound + transient
	// errors stay uncached — see ResponseCache godoc for rationale.
	h.Cache.Put(name, card)
	writeCardJSON(w, card, "MISS")
}

// writeCardJSON centralizes the success-response shape so cache HIT
// and MISS paths can't drift apart (same headers, same indent, same
// encoder behavior).
func writeCardJSON(w http.ResponseWriter, card *Card, cacheStatus string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", cacheStatus)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(card)
}

func writeJSONErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
