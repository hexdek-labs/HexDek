// InstanceID Phase 9 — spectator lineage HTTP endpoint per
// docs/instanceid-system-v2-r60.md §13.
//
// GET /api/spectator/lineage/{instance_id}
//
// Scans every live spectate room's most-recent snapshot for the
// requested InstanceID. When found, walks the lineage forest via
// heimdall.BuildLineageTree and returns the rendered tree as JSON.
// 404 when the ID is unknown to any room's current snapshot (e.g.
// the card has ceased, the game has ended and the room has been
// torn down, or the ID was never minted in any live game).

package hexapi

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/hexdek/hexdek/internal/heimdall"
)

// lineageInstanceIDPattern restricts the path parameter to the
// canonical InstanceID format. Rejects path traversal / log-injection
// attempts. Mirrors instanceid.FormatRegex without anchoring the cmc
// digit count (the path parameter is single-shot validation, not a
// format check — that responsibility stays with the minter).
var lineageInstanceIDPattern = regexp.MustCompile(`^[a-z][0-3](OG|TK|CP|AB)(V|H)(C|[WUBRG]{1,5})[0-9]+$`)

// lineageResponse is the JSON envelope returned by the lineage
// endpoint. Carries the walked tree plus a flat text render for
// clients that want a tooltip-ready string.
type lineageResponse struct {
	InstanceID string                 `json:"instance_id"`
	Tree       *heimdall.LineageNode  `json:"tree"`
	Text       string                 `json:"text,omitempty"`
}

func (sm *Showmatch) handleSpectatorLineage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("instance_id")
	if id == "" || !lineageInstanceIDPattern.MatchString(id) {
		http.Error(w, "invalid instance_id", http.StatusBadRequest)
		return
	}
	if sm == nil || sm.rooms == nil {
		http.Error(w, "spectator rooms unavailable", http.StatusServiceUnavailable)
		return
	}

	// Search every live room's most-recent snapshot. The InstanceID
	// system is per-game, so once the matching room is identified the
	// rest of the lineage forest comes from THAT room's snapshot — we
	// never cross-walk records between rooms.
	tree, found := sm.findLineageInRooms(id)
	if !found {
		http.Error(w, "instance_id not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lineageResponse{
		InstanceID: id,
		Tree:       tree,
		Text:       heimdall.RenderLineageText(tree),
	})
}

// findLineageInRooms scans the room manager for the first room whose
// most-recent snapshot contains instanceID, then builds and returns
// the walked tree. Returns (nil, false) when no room contains the ID.
func (sm *Showmatch) findLineageInRooms(instanceID string) (*heimdall.LineageNode, bool) {
	if sm == nil || sm.rooms == nil {
		return nil, false
	}
	sm.rooms.mu.RLock()
	defer sm.rooms.mu.RUnlock()
	for _, room := range sm.rooms.rooms {
		if room == nil {
			continue
		}
		room.mu.RLock()
		snap := room.snap
		room.mu.RUnlock()
		if snap == nil || len(snap.Lineage) == 0 {
			continue
		}
		if _, ok := snap.Lineage[instanceID]; !ok {
			continue
		}
		tree := heimdall.BuildLineageTree(snap.Lineage, instanceID)
		if tree != nil {
			return tree, true
		}
	}
	return nil, false
}
