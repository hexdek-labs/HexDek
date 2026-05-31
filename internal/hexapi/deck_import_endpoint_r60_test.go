package hexapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// deck_import_endpoint_r60_test.go — pins POST /api/deck/import:
//   - auth (X-HexDek-Owner) gate, 401 on missing
//   - rate limit (shared bucket with other import endpoints)
//   - 503 when Showmatch / MetaDB not configured
//   - 400 on bad JSON, empty deck_list, parse error
//   - happy path response shape (deck summary + quick_analyze)
//   - type tallies + average-CMC (lands excluded)
//   - color identity ordering (canonical WUBRG)
//   - warnings: missing commander / no lands / low parse coverage
//   - body size cap rejects 1MiB+ paste
//   - no CSRF needed (route not on csrfGatedRoutes; passes test pin)

// importPreviewMetaJSONL is the minimal MetaDB the tests load. Covers
// every card name used in the test fixtures. Built from real Scryfall-
// shaped rows so the parser exercises the same code path it would in
// production.
const importPreviewMetaJSONL = `
{"name":"Sol Ring","type_line":"Artifact","mana_cost":"{1}","cmc":1,"colors":[]}
{"name":"Counterspell","type_line":"Instant","mana_cost":"{U}{U}","cmc":2,"colors":["U"]}
{"name":"Demonic Tutor","type_line":"Sorcery","mana_cost":"{1}{B}","cmc":2,"colors":["B"]}
{"name":"Llanowar Elves","type_line":"Creature — Elf Druid","mana_cost":"{G}","cmc":1,"colors":["G"],"power":"1","toughness":"1"}
{"name":"Atraxa, Praetors' Voice","type_line":"Legendary Creature — Phyrexian Angel Horror","mana_cost":"{G}{W}{U}{B}","cmc":4,"colors":["B","G","U","W"],"power":"4","toughness":"4"}
{"name":"Sylvan Library","type_line":"Enchantment","mana_cost":"{1}{G}","cmc":2,"colors":["G"]}
{"name":"Forest","type_line":"Basic Land — Forest","mana_cost":"","cmc":0,"colors":[]}
{"name":"Island","type_line":"Basic Land — Island","mana_cost":"","cmc":0,"colors":[]}
{"name":"Swamp","type_line":"Basic Land — Swamp","mana_cost":"","cmc":0,"colors":[]}
{"name":"Plains","type_line":"Basic Land — Plains","mana_cost":"","cmc":0,"colors":[]}
{"name":"Mountain","type_line":"Basic Land — Mountain","mana_cost":"","cmc":0,"colors":[]}
{"name":"Wrath of God","type_line":"Sorcery","mana_cost":"{2}{W}{W}","cmc":4,"colors":["W"]}
{"name":"Birds of Paradise","type_line":"Creature — Bird","mana_cost":"{G}","cmc":1,"colors":["G"],"power":"0","toughness":"1"}
`

// makeImportPreviewHandler builds a Handler with a Showmatch carrying
// only the test MetaDB (corpus left nil — the preview endpoint
// doesn't need oracle text resolution). Returns the handler + a real
// http.ServeMux with the route mounted so tests exercise the full
// http dispatch path (not the bare h.handleDeckImportPreview call).
func makeImportPreviewHandler(t *testing.T) (*Handler, *http.ServeMux) {
	t.Helper()
	meta, err := deckparser.LoadMetaReader(strings.NewReader(importPreviewMetaJSONL))
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	h := &Handler{
		Showmatch: &Showmatch{meta: meta},
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux
}

// postImport drives one POST /api/deck/import. Owner header optional
// (pass "" to test the 401 path).
func postImport(t *testing.T, mux *http.ServeMux, deckList, owner string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(DeckImportPreviewRequest{DeckList: deckList})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deck/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if owner != "" {
		req.Header.Set("X-HexDek-Owner", owner)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// --- Auth gate ----------------------------------------------------------

func TestDeckImportPreview_401WithoutOwnerHeader(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	rec := postImport(t, mux, "1 Sol Ring", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401 (body=%q)", rec.Code, rec.Body.String())
	}
	var body ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, rec.Body.String())
	}
	if body.Error.Code != "unauthorized" {
		t.Errorf("error.code=%q, want unauthorized", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "auth") {
		t.Errorf("error.message=%q must mention auth", body.Error.Message)
	}
}

// Owner-header whitespace-only counts as missing — defends the
// existing pattern at handler.go:699 from drifting on this endpoint.
func TestDeckImportPreview_401WithBlankOwnerHeader(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	rec := postImport(t, mux, "1 Sol Ring", "   ")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("blank header: status=%d, want 401", rec.Code)
	}
}

// --- Body validation -----------------------------------------------------

func TestDeckImportPreview_400OnInvalidJSON(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/deck/import",
		strings.NewReader(`{"deck_list": malformed`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HexDek-Owner", "alice")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (body=%q)", rec.Code, rec.Body.String())
	}
	var body ErrorResponse
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Error.Code != "bad_request" {
		t.Errorf("error.code=%q, want bad_request", body.Error.Code)
	}
}

func TestDeckImportPreview_400OnEmptyDeckList(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	rec := postImport(t, mux, "   \n\n   ", "alice")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (body=%q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "deck_list") {
		t.Errorf("body should mention deck_list, got %q", rec.Body.String())
	}
}

// 1 MiB cap enforced via http.MaxBytesReader — pasting a giant blob
// must 400 (the reader closes mid-parse so json.Decoder returns an
// error which becomes "invalid JSON body").
func TestDeckImportPreview_400OnOversizedPayload(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	// 2 MiB of "1 Sol Ring\n" lines — well over the 1 MiB cap.
	huge := strings.Repeat("1 Sol Ring\n", 200_000)
	rec := postImport(t, mux, huge, "alice")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized: status=%d, want 400", rec.Code)
	}
}

// --- 503 wiring ----------------------------------------------------------

func TestDeckImportPreview_503WhenShowmatchMissing(t *testing.T) {
	h := &Handler{} // no Showmatch
	mux := http.NewServeMux()
	h.Register(mux)
	rec := postImport(t, mux, "1 Sol Ring", "alice")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
	var body ErrorResponse
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Error.Code != "service_unavailable" {
		t.Errorf("error.code=%q, want service_unavailable", body.Error.Code)
	}
}

func TestDeckImportPreview_503WhenMetaMissing(t *testing.T) {
	// Showmatch present but meta nil — surfaces the same 503.
	h := &Handler{Showmatch: &Showmatch{}}
	mux := http.NewServeMux()
	h.Register(mux)
	rec := postImport(t, mux, "1 Sol Ring", "alice")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
}

// --- Happy path: response shape -----------------------------------------

// commanderDeckFixture is a 100-card Commander shape (1 commander +
// 7 spells + 92 basics). The card-count + format-detection thresholds
// at deckparser.go:1355-1367 require total ∈ [95,101] for FormatCommander,
// so the basics make up the bulk.
const commanderDeckFixture = `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Counterspell
1 Demonic Tutor
1 Llanowar Elves
1 Sylvan Library
1 Wrath of God
1 Birds of Paradise
23 Forest
23 Island
23 Swamp
23 Plains
`

func TestDeckImportPreview_HappyPath_DeckSummary(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	rec := postImport(t, mux, commanderDeckFixture, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type=%q, want JSON", ct)
	}
	var resp DeckImportPreviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}
	if resp.Deck.Commander != "Atraxa, Praetors' Voice" {
		t.Errorf("commander=%q, want %q", resp.Deck.Commander, "Atraxa, Praetors' Voice")
	}
	if resp.Deck.Format != string(deckparser.FormatCommander) {
		t.Errorf("format=%q, want commander", resp.Deck.Format)
	}
	// 1 commander + 7 spells + 92 basics = 100 cards.
	if resp.Deck.CardCount != 100 {
		t.Errorf("card_count=%d, want 100", resp.Deck.CardCount)
	}
	if len(resp.Deck.Lines) == 0 {
		t.Error("lines empty — should have one entry per source line")
	}
	if resp.Deck.ParseCoverage < 90 {
		t.Errorf("parse_coverage=%v, want ≥90 (all cards in meta)", resp.Deck.ParseCoverage)
	}
}

func TestDeckImportPreview_HappyPath_QuickAnalyze(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	rec := postImport(t, mux, commanderDeckFixture, "alice")
	var resp DeckImportPreviewResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	qa := resp.QuickAnalyze

	// Type tallies: 3 creatures (Atraxa + Llanowar + Birds), 1 instant,
	// 2 sorceries, 1 enchantment, 1 artifact, 40 lands.
	if qa.Creatures != 3 {
		t.Errorf("creatures=%d, want 3", qa.Creatures)
	}
	if qa.Instants != 1 {
		t.Errorf("instants=%d, want 1", qa.Instants)
	}
	if qa.Sorceries != 2 {
		t.Errorf("sorceries=%d, want 2", qa.Sorceries)
	}
	if qa.Enchantments != 1 {
		t.Errorf("enchantments=%d, want 1", qa.Enchantments)
	}
	if qa.Artifacts != 1 {
		t.Errorf("artifacts=%d, want 1", qa.Artifacts)
	}
	if qa.Lands != 92 {
		t.Errorf("lands=%d, want 92", qa.Lands)
	}

	// Average CMC excludes lands. Non-land CMCs:
	//   Sol Ring 1, Counterspell 2, Demonic Tutor 2, Llanowar Elves 1,
	//   Sylvan Library 2, Wrath of God 4, Birds of Paradise 1, Atraxa 4
	// = 17 / 8 = 2.125. Allow ±0.05 tolerance.
	if qa.AverageCMC < 2.0 || qa.AverageCMC > 2.25 {
		t.Errorf("average_cmc=%v, want ~2.125 (17/8 across 8 non-land cards)", qa.AverageCMC)
	}

	// Color identity = WUBG (canonical WUBRG ordering — no R).
	wantColors := []string{"W", "U", "B", "G"}
	if len(qa.ColorIdentity) != len(wantColors) {
		t.Errorf("color_identity=%v, want %v", qa.ColorIdentity, wantColors)
	} else {
		for i, c := range qa.ColorIdentity {
			if c != wantColors[i] {
				t.Errorf("color_identity[%d]=%q, want %q (canonical WUBRG order)", i, c, wantColors[i])
			}
		}
	}
}

// --- Warnings -----------------------------------------------------------

func TestDeckImportPreview_Warnings_NoCommander(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	// Constructed-shape deck (60+ cards, no COMMANDER:).
	deck := "1 Counterspell\n" + strings.Repeat("1 Island\n", 60)
	rec := postImport(t, mux, deck, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	var resp DeckImportPreviewResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	found := false
	for _, w := range resp.QuickAnalyze.Warnings {
		if strings.Contains(w, "no commander") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'no commander' warning, got %v", resp.QuickAnalyze.Warnings)
	}
}

func TestDeckImportPreview_Warnings_NoLands(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	rec := postImport(t, mux, "COMMANDER: Atraxa, Praetors' Voice\n1 Sol Ring\n1 Counterspell\n", "alice")
	var resp DeckImportPreviewResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	found := false
	for _, w := range resp.QuickAnalyze.Warnings {
		if strings.Contains(w, "no lands") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'no lands' warning, got %v", resp.QuickAnalyze.Warnings)
	}
}

// Low parse coverage triggers the warning. Mostly-unresolved card
// names hit the "low parse coverage" branch.
func TestDeckImportPreview_Warnings_LowParseCoverage(t *testing.T) {
	_, mux := makeImportPreviewHandler(t)
	deck := "COMMANDER: Atraxa, Praetors' Voice\n" +
		"1 Notarealcard\n1 Anotherfakecardname\n1 Yetanotherfake\n1 Sol Ring\n"
	rec := postImport(t, mux, deck, "alice")
	var resp DeckImportPreviewResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Deck.ParseCoverage >= 90 {
		t.Fatalf("setup: expected low coverage, got %v", resp.Deck.ParseCoverage)
	}
	found := false
	for _, w := range resp.QuickAnalyze.Warnings {
		if strings.Contains(w, "low parse coverage") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'low parse coverage' warning, got %v", resp.QuickAnalyze.Warnings)
	}
	// Unresolved names surface in the response.
	if len(resp.Deck.Unresolved) < 3 {
		t.Errorf("unresolved=%v, want >=3 fake-card entries", resp.Deck.Unresolved)
	}
}

// --- Rate-limit + envelope ---------------------------------------------

func TestDeckImportPreview_RateLimit_ShareImportBudget(t *testing.T) {
	h, mux := makeImportPreviewHandler(t)
	// Tight bucket so over-limit triggers fast.
	h.DeckImportLimiter = NewRateLimiter(2, 1.0/60.0)

	// 2 calls from one IP pass.
	for i := 0; i < 2; i++ {
		rec := postImport(t, mux, commanderDeckFixture, "alice")
		if rec.Code != http.StatusOK {
			t.Fatalf("burst %d: status=%d", i+1, rec.Code)
		}
	}
	// 3rd: 429.
	body, _ := json.Marshal(DeckImportPreviewRequest{DeckList: commanderDeckFixture})
	req := httptest.NewRequest(http.MethodPost, "/api/deck/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HexDek-Owner", "alice")
	req.Header.Set("X-Forwarded-For", "192.0.2.1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: status=%d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After missing on 429")
	}
	var er ErrorResponse
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Code != "too_many_requests" {
		t.Errorf("error.code=%q, want too_many_requests", er.Error.Code)
	}
	if !strings.Contains(er.Error.Message, "deck preview") {
		t.Errorf("error.message=%q must carry the endpoint label", er.Error.Message)
	}
}

// Per-owner axis also gates. Mirrors the deck-analyze pattern from
// PR #823 — IP rotation by one owner doesn't bypass throttling.
func TestDeckImportPreview_RateLimit_PerOwnerCatchesIPRotation(t *testing.T) {
	h, mux := makeImportPreviewHandler(t)
	h.DeckImportLimiter = NewRateLimiter(100, 100.0)     // loose: per-IP won't fire
	h.DeckAnalyzeOwnerLimiter = NewRateLimiter(2, 1.0/60.0) // tight per-owner

	for i, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		body, _ := json.Marshal(DeckImportPreviewRequest{DeckList: commanderDeckFixture})
		req := httptest.NewRequest(http.MethodPost, "/api/deck/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-HexDek-Owner", "rotating-alice")
		req.Header.Set("X-Forwarded-For", ip)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("rotation %d (%s): status=%d", i+1, ip, rec.Code)
		}
	}
	// 3rd from fresh IP: per-owner bucket catches it.
	body, _ := json.Marshal(DeckImportPreviewRequest{DeckList: commanderDeckFixture})
	req := httptest.NewRequest(http.MethodPost, "/api/deck/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HexDek-Owner", "rotating-alice")
	req.Header.Set("X-Forwarded-For", "3.3.3.3")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd IP-rotated request: status=%d, want 429", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-By"); got != "owner" {
		t.Errorf("X-RateLimit-By=%q, want owner", got)
	}
}

// --- CSRF: NOT gated ---------------------------------------------------

// The endpoint is idempotent (pure parse, no side-effect) so it
// deliberately skips RequireCSRF. Pin that contract via the existing
// csrfGatedRoutes list — if a future PR mistakenly adds RequireCSRF
// here, the gated-routes coverage test will trip on the new route
// being missing from the pin.
func TestDeckImportPreview_NotInCSRFGatedList(t *testing.T) {
	for _, route := range csrfGatedRoutes {
		if strings.Contains(route, "/api/deck/import") {
			t.Errorf("POST /api/deck/import should NOT be CSRF-gated (idempotent), but appears in csrfGatedRoutes: %q", route)
		}
	}
}
