package hexapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse is the single shape every hexapi handler returns when it
// signals a non-success HTTP status. Clients (the React frontend in
// hexdek/, plus any third-party caller of the public HTTP surface) can
// rely on this contract: Content-Type is "application/json", the body
// decodes into ErrorResponse, and Status echoes the HTTP status code so
// it survives intermediaries that strip the header.
type ErrorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

// writeError writes a JSON ErrorResponse with the given HTTP status.
// It is the unified replacement for scattered `http.Error(w, msg,
// status)` calls (text/plain bodies) and `writeJSON(w,
// map[string]string{"error": ...})` + late `WriteHeader` sequences
// (which silently dropped the status because the body write committed
// the default 200 first).
//
// Headers are written before the body, the response is flushed by
// json.Encoder, and the status is mirrored into the body so clients
// without easy access to the response status can still surface it.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message, Status: status}); err != nil {
		log.Printf("hexapi writeError encode: %v", err)
	}
}

// writeErrorWithDetails is the extended form of writeError for
// handlers that need to ship additional structured fields alongside
// the unified envelope — e.g. the gauntlet credits gate has to surface
// the user's current balance and required credit count so the
// frontend can render an actionable "earn or wait" prompt.
//
// The emitted body always carries the unified `error` + `status`
// fields; any key in `details` is merged in. If `details` reuses the
// `error` or `status` keys they are silently dropped — the unified
// contract wins, so existing decoders that bind ErrorResponse stay
// compatible. Headers match writeError exactly.
func writeErrorWithDetails(w http.ResponseWriter, status int, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	body := make(map[string]any, len(details)+2)
	for k, v := range details {
		if k == "error" || k == "status" {
			continue
		}
		body[k] = v
	}
	body["error"] = message
	body["status"] = status
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("hexapi writeErrorWithDetails encode: %v", err)
	}
}
