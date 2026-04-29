// Package party exposes the HTTP API for the lobby/party-creation layer:
// devices register themselves, create parties, and join parties via a
// 6-character code.
package party

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/auth"
	"github.com/hexdek/hexdek/internal/db"
	gameengine "github.com/hexdek/hexdek/internal/game"
	"github.com/hexdek/hexdek/internal/moxfield"
)

// Handler holds the HTTP handlers for party / device / deck endpoints.
type Handler struct {
	DB *sql.DB
	// OnGameStart, if set, is called after StartGame succeeds with the new
	// game's id, its party id, and player count. Used by main.go to kick off
	// the AI autopilot without this package importing internal/ws or
	// internal/ai directly.
	OnGameStart func(gameID, partyID string, numPlayers int)
}

var safeFilenameRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.json$`)

// Register attaches all party-related routes to the given mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/device/register", h.registerDevice)
	mux.HandleFunc("GET /api/device/{id}", h.getDevice)

	mux.HandleFunc("POST /api/deck/import", h.importDeck)
	mux.HandleFunc("POST /api/deck/import_decklist", h.importDecklist)
	mux.HandleFunc("POST /api/deck/import_moxfield", h.importMoxfield)
	mux.HandleFunc("GET /api/deck/{id}", h.getDeck)
	mux.HandleFunc("GET /api/device/{id}/decks", h.listDevicesDecks)
	mux.HandleFunc("GET /api/decks/premade", h.listPremadeDecks)
	mux.HandleFunc("GET /api/decks/premade/{file}", h.getPremadeDeck)

	mux.HandleFunc("POST /api/party/create", h.createParty)
	mux.HandleFunc("POST /api/party/{id}/join", h.joinParty)
	mux.HandleFunc("GET /api/party/{id}", h.getParty)
	mux.HandleFunc("POST /api/party/{id}/start_game", h.startGame)
	mux.HandleFunc("POST /api/party/{id}/set_deck", h.setDeck)
	mux.HandleFunc("POST /api/party/{id}/add_ai", h.addAI)
	mux.HandleFunc("POST /api/deck/{id}/save_as_premade", h.saveAsPremade)
	mux.HandleFunc("GET /api/game/{id}/snapshot/{seat}", h.gameSnapshot)
}

// setDeck updates the deck for a player already in the party.
func (h *Handler) setDeck(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	partyID := r.PathValue("id")
	var req struct {
		DeviceID string `json:"device_id"`
		DeckID   string `json:"deck_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.DeviceID == "" || req.DeckID == "" {
		writeErr(w, http.StatusBadRequest, "device_id and deck_id required")
		return
	}
	if err := db.SetMemberDeck(r.Context(), h.DB, partyID, req.DeviceID, req.DeckID); err != nil {
		writeErr(w, http.StatusBadRequest, "failed to set deck")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ----- device endpoints -----

type registerDeviceRequest struct {
	DisplayName string `json:"display_name"`
}

func (h *Handler) registerDevice(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	var req registerDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		writeErr(w, http.StatusBadRequest, "display_name required")
		return
	}

	d, err := db.CreateDevice(r.Context(), h.DB, req.DisplayName)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create device")
		return
	}
	// Issue a session token alongside the device record so clients can
	// immediately connect to WebSocket endpoints. Default 30-day expiry.
	const sessionTTL = 30 * 24 * 60 * 60
	session, err := auth.IssueSession(r.Context(), h.DB, d.ID, sessionTTL)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "device created but session issue failed")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		Device  *db.Device    `json:"device"`
		Session *auth.Session `json:"session"`
	}{Device: d, Session: session})
}

func (h *Handler) getDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := db.GetDevice(r.Context(), h.DB, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "device not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to fetch device")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// ----- deck endpoints -----

type importDeckRequest struct {
	OwnerDeviceID string          `json:"owner_device_id"`
	Name          string          `json:"name"`           // optional override
	MoxfieldURL   string          `json:"moxfield_url"`   // optional reference
	Deck          json.RawMessage `json:"deck"`           // full deck JSON in our internal format
}

func (h *Handler) importDeck(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	var req importDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.OwnerDeviceID == "" {
		writeErr(w, http.StatusBadRequest, "owner_device_id required")
		return
	}
	if len(req.Deck) == 0 {
		writeErr(w, http.StatusBadRequest, "deck required")
		return
	}

	// Validate the deck JSON parses
	var deck moxfield.Deck
	if err := json.Unmarshal(req.Deck, &deck); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("deck JSON parse: %v", err))
		return
	}
	if deck.Name == "" || deck.Commander == "" || len(deck.Mainboard) == 0 {
		writeErr(w, http.StatusBadRequest, "deck JSON missing required fields")
		return
	}

	name := req.Name
	if name == "" {
		name = deck.Name
	}

	d := &db.Deck{
		OwnerDeviceID: req.OwnerDeviceID,
		Name:          name,
		CommanderName: deck.Commander,
		Format:        deck.Format,
		MoxfieldURL:   req.MoxfieldURL,
		RawJSON:       string(req.Deck),
	}
	if err := db.CreateDeck(r.Context(), h.DB, d); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save deck")
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

// importDecklistRequest accepts a plain-text decklist (Moxfield / Archidekt /
// Goldfish / plain text — all use the same `{qty} {name}` shape). We parse
// it, resolve every card via our Scryfall-backed oracle cache, and save a
// fully-enriched deck to the DB.
type importDecklistRequest struct {
	OwnerDeviceID string `json:"owner_device_id"`
	Name          string `json:"name"`
	MoxfieldURL   string `json:"moxfield_url"`   // optional reference only
	Decklist      string `json:"decklist"`       // required: raw text
	CommanderName string `json:"commander_name"` // optional override
	Format        string `json:"format"`         // defaults to "commander"
}

func (h *Handler) importDecklist(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	var req importDecklistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.OwnerDeviceID == "" {
		writeErr(w, http.StatusBadRequest, "owner_device_id required")
		return
	}
	if strings.TrimSpace(req.Decklist) == "" {
		writeErr(w, http.StatusBadRequest, "decklist required")
		return
	}

	parsed, err := moxfield.ParseDecklist(req.Decklist)
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("parse: %v", err))
		return
	}
	result, err := moxfield.Resolve(r.Context(), h.DB, parsed, req.Name, req.CommanderName, req.Format)
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("resolve: %v", err))
		return
	}

	rawJSON, err := json.Marshal(result.Deck)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("marshal: %v", err))
		return
	}

	d := &db.Deck{
		OwnerDeviceID: req.OwnerDeviceID,
		Name:          result.Deck.Name,
		CommanderName: result.Deck.Commander,
		Format:        result.Deck.Format,
		MoxfieldURL:   req.MoxfieldURL,
		RawJSON:       string(rawJSON),
	}
	if err := db.CreateDeck(r.Context(), h.DB, d); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save deck")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		*db.Deck
		CardCount  int      `json:"card_count"`
		Unresolved []string `json:"unresolved"`
	}{
		Deck:       d,
		CardCount:  result.Deck.CardCount(),
		Unresolved: result.Unresolved,
	})
}

// importMoxfield fetches a deck from a Moxfield URL, resolves all cards via
// the oracle cache, saves the imported .txt to data/decks/imported/, and stores
// the fully-enriched deck to the DB. This is the "paste a Moxfield link" flow.
type importMoxfieldRequest struct {
	OwnerDeviceID string `json:"owner_device_id"`
	URL           string `json:"url"`  // required: Moxfield deck URL
	Name          string `json:"name"` // optional override
}

func (h *Handler) importMoxfield(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	var req importMoxfieldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.OwnerDeviceID == "" {
		writeErr(w, http.StatusBadRequest, "owner_device_id required")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeErr(w, http.StatusBadRequest, "url required")
		return
	}

	// Fetch the decklist from Moxfield.
	deckID := moxfield.ExtractDeckID(req.URL)
	if deckID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("could not extract deck ID from URL %q", req.URL))
		return
	}
	text, err := moxfield.FetchDeck(req.URL)
	if err != nil {
		writeErr(w, http.StatusBadGateway, fmt.Sprintf("moxfield fetch: %v", err))
		return
	}

	// Save raw text to data/decks/imported/ for reuse.
	importDir := "data/decks/imported"
	_ = os.MkdirAll(importDir, 0755)
	txtPath := importDir + "/" + deckID + ".txt"
	_ = os.WriteFile(txtPath, []byte(text), 0644)

	// Parse + resolve via oracle.
	parsed, err := moxfield.ParseDecklist(text)
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("parse fetched deck: %v", err))
		return
	}
	name := req.Name
	if name == "" {
		// Try to get the deck name from the API response.
		if fetchedName, err := moxfield.FetchDeckName(req.URL); err == nil && fetchedName != "" {
			name = fetchedName
		}
	}
	result, err := moxfield.Resolve(r.Context(), h.DB, parsed, name, "", "commander")
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("resolve: %v", err))
		return
	}

	rawJSON, err := json.Marshal(result.Deck)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("marshal: %v", err))
		return
	}

	d := &db.Deck{
		OwnerDeviceID: req.OwnerDeviceID,
		Name:          result.Deck.Name,
		CommanderName: result.Deck.Commander,
		Format:        result.Deck.Format,
		MoxfieldURL:   req.URL,
		RawJSON:       string(rawJSON),
	}
	if err := db.CreateDeck(r.Context(), h.DB, d); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save deck")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		*db.Deck
		CardCount  int      `json:"card_count"`
		Unresolved []string `json:"unresolved"`
	}{
		Deck:       d,
		CardCount:  result.Deck.CardCount(),
		Unresolved: result.Unresolved,
	})
}

// listPremadeDecks returns metadata for every deck JSON file under
// data/decks/. The UI shows these as a dropdown of pre-baked decks users can
// import without pasting text.
func (h *Handler) listPremadeDecks(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir("data/decks")
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	type premade struct {
		File      string `json:"file"`
		Name      string `json:"name"`
		Commander string `json:"commander"`
		Format    string `json:"format"`
		Cards     int    `json:"cards"`
	}
	out := []premade{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		d, err := moxfield.LoadDeckFromFile("data/decks/" + entry.Name())
		if err != nil {
			continue
		}
		out = append(out, premade{
			File:      entry.Name(),
			Name:      d.Name,
			Commander: d.Commander,
			Format:    d.Format,
			Cards:     d.CardCount(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// getPremadeDeck serves the raw JSON body of a deck file under data/decks/.
// Filename is restricted to a whitelist character set to prevent traversal.
func (h *Handler) getPremadeDeck(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if !safeFilenameRE.MatchString(file) {
		writeErr(w, http.StatusBadRequest, "invalid filename")
		return
	}
	path := "data/decks/" + file
	if _, err := os.Stat(path); err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	http.ServeFile(w, r, path)
}

func (h *Handler) getDeck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := db.GetDeck(r.Context(), h.DB, id)
	if err != nil {
		writeErr(w, http.StatusNotFound, fmt.Sprintf("deck not found: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) listDevicesDecks(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	decks, err := db.ListDecksByDevice(r.Context(), h.DB, deviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list decks")
		return
	}
	if decks == nil {
		decks = []*db.Deck{}
	}
	writeJSON(w, http.StatusOK, decks)
}

// ----- party endpoints -----

type createPartyRequest struct {
	HostDeviceID string `json:"host_device_id"`
	MaxPlayers   int    `json:"max_players"` // 2-4
}

func (h *Handler) createParty(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	var req createPartyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.HostDeviceID == "" {
		writeErr(w, http.StatusBadRequest, "host_device_id required")
		return
	}
	if req.MaxPlayers == 0 {
		req.MaxPlayers = 4 // default to 4-player commander
	}

	// Verify host device exists
	if _, err := db.GetDevice(r.Context(), h.DB, req.HostDeviceID); err != nil {
		writeErr(w, http.StatusNotFound, "host device not found")
		return
	}

	p, err := db.CreateParty(r.Context(), h.DB, req.HostDeviceID, req.MaxPlayers)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create party")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

type joinPartyRequest struct {
	DeviceID string `json:"device_id"`
	DeckID   string `json:"deck_id"`
	IsAI     bool   `json:"is_ai"`
}

func (h *Handler) joinParty(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	partyID := r.PathValue("id")
	var req joinPartyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.DeviceID == "" {
		writeErr(w, http.StatusBadRequest, "device_id required")
		return
	}

	if _, err := db.GetDevice(r.Context(), h.DB, req.DeviceID); err != nil {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}

	m, err := db.JoinParty(r.Context(), h.DB, partyID, req.DeviceID, req.DeckID, req.IsAI)
	if err != nil {
		if errors.Is(err, db.ErrPartyFull) {
			writeErr(w, http.StatusConflict, "party is full")
			return
		}
		writeErr(w, http.StatusBadRequest, "failed to join party")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) getParty(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := db.GetParty(r.Context(), h.DB, id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "party not found")
		return
	}
	members, err := db.ListPartyMembers(r.Context(), h.DB, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		*db.Party
		Members []*db.PartyMember `json:"members"`
	}{
		Party:   p,
		Members: members,
	})
}

// startGame transitions a party from lobby state to a running game,
// initializing each player's library, hand, and turn state.
func (h *Handler) startGame(w http.ResponseWriter, r *http.Request) {
	partyID := r.PathValue("id")
	members, err := db.ListPartyMembers(r.Context(), h.DB, partyID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "party not found")
		return
	}
	if len(members) < 2 {
		writeErr(w, http.StatusBadRequest, "party needs at least 2 members")
		return
	}
	// Each member must have selected a deck
	in := gameengine.StartGameInput{PartyID: partyID}
	for _, m := range members {
		if !m.DeckID.Valid || m.DeckID.String == "" {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("seat %d has no deck selected", m.SeatPosition))
			return
		}
		in.Players = append(in.Players, gameengine.StartGamePlayer{
			SeatPosition: m.SeatPosition,
			DeviceID:     m.DeviceID,
			DeckID:       m.DeckID.String,
		})
	}

	g, err := gameengine.StartGame(r.Context(), h.DB, in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("start game: %v", err))
		return
	}

	// Mark party as playing
	_, _ = h.DB.ExecContext(r.Context(),
		`UPDATE party SET state = 'playing' WHERE id = ?`, partyID)

	if h.OnGameStart != nil {
		h.OnGameStart(g.ID, partyID, len(members))
	}

	writeJSON(w, http.StatusCreated, g)
}

// addAI attaches a synthetic AI-controlled seat to the party. On success,
// returns the party member record. The AI gets a dedicated device row
// (marked visibly with the "AI:" prefix so humans can tell who's across the
// table) and joins the party with the requested premade deck (or Yuriko by
// default if unspecified).
func (h *Handler) addAI(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	partyID := r.PathValue("id")
	var req struct {
		Name     string `json:"name"`      // display name for the AI (defaults to "Hex")
		DeckFile string `json:"deck_file"` // premade file under data/decks/
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // body optional

	if req.Name == "" {
		req.Name = "Hex (AI)"
	}
	if req.DeckFile == "" {
		req.DeckFile = "yuriko_v1.json"
	}
	if !safeFilenameRE.MatchString(req.DeckFile) {
		writeErr(w, http.StatusBadRequest, "invalid deck_file")
		return
	}

	// Register the AI device
	aiDevice, err := db.CreateDevice(r.Context(), h.DB, req.Name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create AI device")
		return
	}

	// Import the deck into the AI's deck list
	deckRaw, err := os.ReadFile("data/decks/" + req.DeckFile)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "deck file not found")
		return
	}
	var parsedDeck moxfield.Deck
	if err := json.Unmarshal(deckRaw, &parsedDeck); err != nil {
		writeErr(w, http.StatusInternalServerError, "invalid deck format")
		return
	}
	d := &db.Deck{
		OwnerDeviceID: aiDevice.ID,
		Name:          parsedDeck.Name,
		CommanderName: parsedDeck.Commander,
		Format:        parsedDeck.Format,
		RawJSON:       string(deckRaw),
	}
	if err := db.CreateDeck(r.Context(), h.DB, d); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save AI deck")
		return
	}

	// Join party with is_ai=1
	m, err := db.JoinParty(r.Context(), h.DB, partyID, aiDevice.ID, d.ID, true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "AI failed to join party")
		return
	}

	writeJSON(w, http.StatusCreated, struct {
		*db.PartyMember
		DisplayName   string `json:"display_name"`
		DeckName      string `json:"deck_name"`
		CommanderName string `json:"commander_name"`
	}{
		PartyMember:   m,
		DisplayName:   aiDevice.DisplayName,
		DeckName:      d.Name,
		CommanderName: d.CommanderName,
	})
}

// saveAsPremade copies an imported deck's raw JSON to the data/decks/
// directory so it shows up in the premade list for everyone else.
// Filename is derived from commander name; caller can override.
func (h *Handler) saveAsPremade(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	deckID := r.PathValue("id")
	var req struct {
		Filename string `json:"filename"` // optional override (must be *.json)
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	d, err := db.GetDeck(r.Context(), h.DB, deckID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "deck not found")
		return
	}

	filename := req.Filename
	if filename == "" {
		filename = slugify(d.Name) + ".json"
	}
	if !safeFilenameRE.MatchString(filename) {
		writeErr(w, http.StatusBadRequest, "invalid filename")
		return
	}

	if err := os.MkdirAll("data/decks", 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot create directory")
		return
	}
	path := "data/decks/" + filename
	if _, err := os.Stat(path); err == nil {
		writeErr(w, http.StatusConflict, "premade deck already exists with that filename")
		return
	}
	if err := os.WriteFile(path, []byte(d.RawJSON), 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot write file")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"filename": filename,
	})
}

// slugify produces a filesystem-safe stem from an arbitrary deck name.
//
//	"Gandhi Oloro Peacekeeper" → "gandhi_oloro_peacekeeper"
func slugify(s string) string {
	var b strings.Builder
	last := '_'
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			last = r
		default:
			if last != '_' {
				b.WriteRune('_')
				last = '_'
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// gameSnapshot returns a per-seat view of game state with hidden info redacted.
func (h *Handler) gameSnapshot(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	seatStr := r.PathValue("seat")
	seat, err := strconv.Atoi(seatStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid seat")
		return
	}
	snap, err := gameengine.Snapshot(r.Context(), h.DB, gameID, seat)
	if err != nil {
		writeErr(w, http.StatusNotFound, fmt.Sprintf("snapshot: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// ----- helpers -----

func limitBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		// best-effort; headers already sent
		_ = err
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Bind exposes a context-helper for tests.
var _ = context.Background
