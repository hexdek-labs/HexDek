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
}

// Register adds oracle routes to the mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/oracle/card/{name}", h.lookupCard)
}

func (h *Handler) lookupCard(w http.ResponseWriter, r *http.Request) {
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
