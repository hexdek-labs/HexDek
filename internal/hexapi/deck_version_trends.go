package hexapi

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
	"github.com/hexdek/hexdek/internal/versioning"
)

// handleDeckVersionTrends implements GET /api/decks/{owner}/{id}/version-trends.
//
// Returns per-version winrate rollup for the named deck. The version
// timeline is read from the versioning DAG at
// <DecksDir>/.versions/deck_dag.json; per-game outcomes come from the
// showmatch DB. ComputeDeckVersionTrends buckets each game into the
// version that was HEAD at its finished_at timestamp.
//
// Empty payload (200) when the deck has no version history yet —
// frontend renders this as "no versions tracked" rather than a 404.
func (h *Handler) handleDeckVersionTrends(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	lineage := h.loadDeckLineageEntries(owner, id)

	var games []db.DeckGameOutcome
	if h.Showmatch != nil && h.Showmatch.sqlDB != nil {
		rows, err := db.LoadDeckGameOutcomes(r.Context(), h.Showmatch.sqlDB, owner+"/"+id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "version trends query failed")
			return
		}
		games = rows
	}

	out := heimdall.ComputeDeckVersionTrends(owner, id, lineage, convertOutcomes(games))
	writeJSON(w, out)
}

// loadDeckLineageEntries reads the versioning DAG and translates the
// owner/id deck's lineage into the heimdall.DeckVersionEntry shape
// (parsing the RFC3339 CreatedAt into a Unix epoch). Returns nil
// when the DAG file is missing or the deck has no recorded versions.
func (h *Handler) loadDeckLineageEntries(owner, id string) []heimdall.DeckVersionEntry {
	dagDir := filepath.Join(h.DecksDir, ".versions")
	dag, err := versioning.LoadDAG(dagDir)
	if err != nil {
		return nil
	}
	lineage := dag.GetLineage(owner, id)
	if len(lineage) == 0 {
		return nil
	}
	out := make([]heimdall.DeckVersionEntry, 0, len(lineage))
	for _, n := range lineage {
		ts := int64(0)
		if t, err := time.Parse(time.RFC3339, n.CreatedAt); err == nil {
			ts = t.Unix()
		}
		out = append(out, heimdall.DeckVersionEntry{
			Hash:      n.Hash,
			Version:   n.Version,
			CardCount: n.CardCount,
			CardDelta: n.CardDelta,
			CreatedAt: ts,
		})
	}
	return out
}

// convertOutcomes adapts db.DeckGameOutcome rows into the heimdall
// pure-computation shape. Keeps heimdall import-free of internal/db.
func convertOutcomes(rows []db.DeckGameOutcome) []heimdall.DeckVersionOutcome {
	out := make([]heimdall.DeckVersionOutcome, 0, len(rows))
	for _, r := range rows {
		out = append(out, heimdall.DeckVersionOutcome{
			FinishedAt: r.FinishedAt,
			Won:        r.Won,
			Draw:       r.Draw,
		})
	}
	return out
}
