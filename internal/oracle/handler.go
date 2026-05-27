package oracle

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

// Handler exposes HTTP endpoints for the card oracle.
type Handler struct {
	DB *sql.DB
	// Limiter throttles inbound /api/oracle/card/{name} requests per
	// client IP. When nil, Register installs a default 60-burst,
	// 1 req/sec refill (≈ 60 req/min steady-state) limiter so the
	// public endpoint is never deployed unprotected.
	Limiter *InboundRateLimiter
}

// Register adds oracle routes to the mux. If h.Limiter is nil, a
// default per-IP limiter (60 burst, 60 req/min steady) is installed —
// the oracle endpoint is unauthenticated and must never be exposed
// without throttling.
func (h *Handler) Register(mux *http.ServeMux) {
	if h.Limiter == nil {
		h.Limiter = NewInboundRateLimiter(60, 1.0)
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

	card, err := Lookup(r.Context(), h.DB, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSONErr(w, http.StatusNotFound, "card not found")
			return
		}
		writeJSONErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(card)
}

func writeJSONErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
