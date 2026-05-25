package hexapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleDeckArchive implements GET /api/decks/{owner}/{id}/archive.
//
// Returns the consolidated DeckArchive blob:
//   - Identity (owner / deck_id / current commander / current archetype)
//   - Version timeline (every entry in the deck's version DAG with
//     per-version games / wins / losses / winrate, archetype when a
//     historical snapshot is available; otherwise empty)
//   - Aggregate game stats + the trailing N most-recent games
//     (?recent=N, default 20, max 100)
//   - Archetype evolution timeline (one step per archetype change)
//
// Empty payload (200) when the deck has no lineage / games yet —
// frontend renders this as "no history" rather than 404.
func (h *Handler) handleDeckArchive(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	recent := 20
	if q := r.URL.Query().Get("recent"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			recent = n
		}
	}

	// Lineage from the version DAG, augmented with per-version
	// archetype where a snapshot file exists.
	lineage := h.loadDeckArchiveLineage(owner, id)

	// Current commander + archetype from the live strategy file.
	currentArch, currentCmdr := h.loadCurrentDeckArchetype(owner, id)

	// Full game history (oldest first). DB returns empty when the
	// deck has no games — that's fine; archive just emits the
	// no-games block.
	var games []db.DeckGameOutcome
	if h.Showmatch != nil && h.Showmatch.sqlDB != nil {
		rows, err := db.LoadDeckGameOutcomes(r.Context(), h.Showmatch.sqlDB, owner+"/"+id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "deck archive query failed")
			return
		}
		games = rows
	}

	archive := heimdall.BuildDeckArchive(heimdall.DeckArchiveInput{
		Owner:            owner,
		DeckID:           id,
		CurrentCommander: currentCmdr,
		CurrentArchetype: currentArch,
		Lineage:          lineage,
		Games:            convertOutcomes(games),
		RecentLimit:      recent,
	})
	writeJSON(w, archive)
}

// loadDeckArchiveLineage builds the lineage list with optional
// per-version archetype. The base lineage comes from the versioning
// DAG (same path the version-trends endpoint uses). Per-version
// archetype is resolved by looking for an optional
// `<DecksDir>/<owner>/versions/<id>_v<N>.strategy.json` snapshot —
// these are NOT written by the engine today; the lookup is forward-
// compatible scaffolding so the archive can light up automatically
// once historical snapshots start being persisted. Until then, only
// the LATEST version's archetype gets backfilled (by BuildDeckArchive
// using CurrentArchetype).
func (h *Handler) loadDeckArchiveLineage(owner, id string) []heimdall.DeckArchiveLineageEntry {
	base := h.loadDeckLineageEntries(owner, id)
	out := make([]heimdall.DeckArchiveLineageEntry, 0, len(base))
	versionsDir := filepath.Join(h.DecksDir, owner, "versions")
	for _, e := range base {
		entry := heimdall.DeckArchiveLineageEntry{DeckVersionEntry: e}
		snapPath := filepath.Join(versionsDir, id+"_v"+strconv.Itoa(e.Version)+".strategy.json")
		if data, err := os.ReadFile(snapPath); err == nil {
			var snap struct {
				Archetype string `json:"archetype"`
			}
			if json.Unmarshal(data, &snap) == nil {
				entry.Archetype = snap.Archetype
			}
		}
		out = append(out, entry)
	}
	return out
}

// loadCurrentDeckArchetype reads <DecksDir>/<owner>/freya/<id>.strategy.json
// and returns (archetype, commander). Empty strings when the file is
// missing or malformed — that's expected for fresh decks that haven't
// been analyzed yet.
func (h *Handler) loadCurrentDeckArchetype(owner, id string) (archetype, commander string) {
	strategyFile := filepath.Join(h.DecksDir, owner, "freya", id+".strategy.json")
	data, err := os.ReadFile(strategyFile)
	if err != nil {
		return "", ""
	}
	var strat struct {
		Archetype     string `json:"archetype"`
		CommanderName string `json:"commander_name"`
	}
	if json.Unmarshal(data, &strat) != nil {
		return "", ""
	}
	return strat.Archetype, strat.CommanderName
}
