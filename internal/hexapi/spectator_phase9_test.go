package hexapi

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// Property: PermanentSnapshot's lineage fields are populated from
// Card.InstanceID + lineage fields by buildLineageIndex / captureSnapshot.
// We exercise buildLineageIndex directly (the simplest entry point) since
// the full captureSnapshot path requires a fully-initialized Showmatch.
func TestPhase9_SpectatorPayload_LineageFieldsPresent(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	// Stamp an OG card directly onto seat 0's hand.
	c := &gameengine.Card{
		Name:    "Sai of the Shining Sword",
		Owner:   0,
		Colors:  []string{"W"},
		CMC:     2,
	}
	gameengine.MintOGInstanceID(gs, c)
	if c.InstanceID == "" {
		t.Fatal("MintOGInstanceID failed to stamp ID")
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)

	idx := buildLineageIndex(gs)
	if len(idx) == 0 {
		t.Fatal("buildLineageIndex returned empty for a state with one minted card")
	}
	rec, ok := idx[c.InstanceID]
	if !ok {
		t.Fatalf("lineage index missing card %q", c.InstanceID)
	}
	if rec.Name != "Sai of the Shining Sword" {
		t.Errorf("record name = %q, want %q", rec.Name, "Sai of the Shining Sword")
	}
	if rec.Provenance != instanceid.ProvOG.String() {
		t.Errorf("record provenance = %q, want %q", rec.Provenance, instanceid.ProvOG.String())
	}
}

// Property: an InstanceID matching the canonical format is accepted by
// the endpoint's path-parameter validator; malformed IDs are rejected
// with 400 before any room scan happens.
func TestPhase9_LineageEndpoint_ValidatesInstanceIDFormat(t *testing.T) {
	cases := []struct {
		id       string
		wantPass bool
	}{
		{"h0OGVW20042", true},   // canonical
		{"h2TKVC11234", true},   // token form
		{"h0TKVWUBRG404071", true}, // multi-color (Atraxa-shape)
		{"", false},
		{"../../etc/passwd", false},
		{"h9OGVW20042", false},   // seat 9 illegal
		{"h0XXVW20042", false},   // bad provenance
		{"h0OGVX20042", false},   // bad color (not WUBRGC)
	}
	for _, tc := range cases {
		got := lineageInstanceIDPattern.MatchString(tc.id)
		if got != tc.wantPass {
			t.Errorf("pattern match(%q) = %v, want %v", tc.id, got, tc.wantPass)
		}
	}
}

// Property: lineage endpoint returns 404 when the InstanceID is
// well-formed but not found in any room's snapshot.
func TestPhase9_LineageEndpoint_404OnUnknownID(t *testing.T) {
	sm := &Showmatch{rooms: NewRoomManager()}
	req := httptest.NewRequest(http.MethodGet, "/api/spectator/lineage/h0OGVW20999", nil)
	req.SetPathValue("instance_id", "h0OGVW20999")
	w := httptest.NewRecorder()
	sm.handleSpectatorLineage(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// Property: lineage endpoint returns 400 on a malformed ID without
// touching the rooms manager.
func TestPhase9_LineageEndpoint_400OnBadFormat(t *testing.T) {
	sm := &Showmatch{rooms: NewRoomManager()}
	req := httptest.NewRequest(http.MethodGet, "/api/spectator/lineage/notavalidid", nil)
	req.SetPathValue("instance_id", "notavalidid")
	w := httptest.NewRecorder()
	sm.handleSpectatorLineage(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// Property: when a room holds a snapshot with the requested ID, the
// endpoint returns a valid JSON tree containing the ID plus a non-empty
// text rendering.
func TestPhase9_LineageEndpoint_ReturnsTreeFromLiveRoom(t *testing.T) {
	sm := &Showmatch{rooms: NewRoomManager()}
	rid := "test-room-lineage"
	room := &SpectateRoom{ID: rid, snap: &GameSnapshot{
		Lineage: map[string]heimdall.LineageRecord{
			"h0OGVW20042": {InstanceID: "h0OGVW20042", Name: "Sai of the Shining Sword", Provenance: "OG"},
		},
	}}
	sm.rooms.rooms[rid] = room

	req := httptest.NewRequest(http.MethodGet, "/api/spectator/lineage/h0OGVW20042", nil)
	req.SetPathValue("instance_id", "h0OGVW20042")
	w := httptest.NewRecorder()
	sm.handleSpectatorLineage(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp lineageResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.InstanceID != "h0OGVW20042" {
		t.Errorf("resp.InstanceID = %q, want %q", resp.InstanceID, "h0OGVW20042")
	}
	if resp.Tree == nil || resp.Tree.InstanceID != "h0OGVW20042" {
		t.Error("resp.Tree missing or wrong InstanceID")
	}
	if !strings.Contains(resp.Text, "Sai of the Shining Sword") {
		t.Errorf("resp.Text missing name; got %q", resp.Text)
	}
}

// Property: instanceIDFromDetails extracts the InstanceID from each
// canonical key in priority order.
func TestPhase9_InstanceIDFromDetails_PriorityOrder(t *testing.T) {
	cases := []struct {
		name    string
		details map[string]interface{}
		want    string
	}{
		{"empty", nil, ""},
		{"instance_id key", map[string]interface{}{"instance_id": "h0OGVW20042"}, "h0OGVW20042"},
		{"card_instance_id fallback", map[string]interface{}{"card_instance_id": "h0OGVW20002"}, "h0OGVW20002"},
		{"priority: instance_id wins over enabler", map[string]interface{}{"instance_id": "primary", "enabler_instance_id": "secondary"}, "primary"},
		{"non-string value ignored", map[string]interface{}{"instance_id": 42}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := instanceIDFromDetails(tc.details); got != tc.want {
				t.Errorf("instanceIDFromDetails(%v) = %q, want %q", tc.details, got, tc.want)
			}
		})
	}
}

// Property: Loki log lines include InstanceID stamping per Phase 9.
// Drive gameengine.RecentEvents with an event carrying instance_id in
// Details — the formatted line must include the stamp.
func TestPhase9_LokiLogStamp_RecentEventsIncludesInstanceID(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.EventPolicy = gameengine.EventLogFull
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   0,
		Target: -1,
		Source: "Sai of the Shining Sword",
		Details: map[string]interface{}{
			"instance_id":         "h0TKVC10001",
			"enabler_instance_id": "h0ABVW20456",
		},
	})
	lines := gameengine.RecentEvents(gs, 5)
	if len(lines) == 0 {
		t.Fatal("RecentEvents returned no lines")
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, "instance_id=h0TKVC10001") {
		t.Errorf("log line missing instance_id stamp; got %q", last)
	}
	if !strings.Contains(last, "enabler_instance_id=h0ABVW20456") {
		t.Errorf("log line missing enabler_instance_id stamp; got %q", last)
	}
}
