package hexapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// graphql_test.go — POC validation that the /graphql surface
// matches the design notes in graphql.go:
//
//   - Schema compiles + validates at construction time.
//   - `games(limit:)` returns history newest-first.
//   - `game(id:)` returns a single row, or null when missing.
//   - Field projection actually works (asking for {id winner} omits
//     deckKeys / commanders from the response).
//   - Variables thread through ($id, $limit).
//   - Aliasing works (one query returning two `game` lookups under
//     different aliases — the cross-resource shape GraphQL pays for).
//   - Bad query syntax / unknown field → 200 + populated `errors`,
//     per the GraphQL transport spec.
//   - The HTTP layer rejects malformed JSON + missing `query` with
//     the unified hexapi ErrorResponse envelope.
//   - GET /graphql works with ?query= for ad-hoc browser pokes.

// seedShowmatch builds a Showmatch with three deterministic
// CompletedGame rows so the order + selection assertions are
// reproducible.
func seedShowmatch(t *testing.T) *Showmatch {
	t.Helper()
	sm := &Showmatch{
		gameHistory: []CompletedGame{
			{
				GameID:     1,
				Winner:     0,
				WinnerName: "Korvold",
				Commanders: []string{"Korvold", "Atraxa", "Muldrotha", "Breya"},
				DeckKeys:   []string{"alice/k", "bob/a", "carol/m", "dave/b"},
				Turns:      11,
				EndReason:  "lethal_combat_damage",
				FinishedAt: time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
			},
			{
				GameID:     2,
				Winner:     2,
				WinnerName: "Muldrotha",
				Commanders: []string{"Korvold", "Atraxa", "Muldrotha", "Breya"},
				DeckKeys:   []string{"alice/k", "bob/a", "carol/m", "dave/b"},
				Turns:      18,
				EndReason:  "concede",
				FinishedAt: time.Date(2026, 5, 24, 11, 30, 0, 0, time.UTC),
			},
			{
				GameID:     3,
				Winner:     1,
				WinnerName: "Atraxa",
				Commanders: []string{"Korvold", "Atraxa", "Muldrotha", "Breya"},
				DeckKeys:   []string{"alice/k", "bob/a", "carol/m", "dave/b"},
				Turns:      22,
				EndReason:  "decking",
				FinishedAt: time.Date(2026, 5, 24, 13, 0, 0, 0, time.UTC),
			},
		},
	}
	return sm
}

// doGraphQL runs one POST /graphql request and returns the parsed
// envelope `{data, errors}`. Tests pull the response fields they
// care about; the helper just spares them the marshal-roundtrip.
func doGraphQL(t *testing.T, h *GraphQLHandler, query string, vars map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(graphqlRequest{Query: query, Variables: vars})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200 (GraphQL transport spec), got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, rr.Body.String())
	}
	return resp
}

// -------------------------------------------------------------------
// Schema construction
// -------------------------------------------------------------------

func TestNewGraphQLHandler_NilShowmatch(t *testing.T) {
	if _, err := NewGraphQLHandler(nil, ""); err == nil {
		t.Fatal("expected error for nil Showmatch, got nil")
	}
}

func TestNewGraphQLHandler_SchemaCompiles(t *testing.T) {
	h, err := NewGraphQLHandler(seedShowmatch(t), "")
	if err != nil {
		t.Fatalf("NewGraphQLHandler: %v", err)
	}
	if h == nil {
		t.Fatal("returned handler is nil")
	}
}

// -------------------------------------------------------------------
// games(limit:) — list query
// -------------------------------------------------------------------

func TestGraphQL_Games_ReturnsNewestFirst(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	resp := doGraphQL(t, h, `{ games(limit: 10) { id winnerName turns } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data := resp["data"].(map[string]any)
	games := data["games"].([]any)
	if len(games) != 3 {
		t.Fatalf("want 3 games, got %d", len(games))
	}
	// Newest first → ids 3, 2, 1.
	wantIDs := []int{3, 2, 1}
	for i, w := range wantIDs {
		g := games[i].(map[string]any)
		got := int(g["id"].(float64))
		if got != w {
			t.Errorf("games[%d].id: want %d, got %d", i, w, got)
		}
	}
}

func TestGraphQL_Games_LimitTruncates(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	resp := doGraphQL(t, h, `{ games(limit: 2) { id } }`, nil)
	games := resp["data"].(map[string]any)["games"].([]any)
	if len(games) != 2 {
		t.Errorf("want 2 games, got %d", len(games))
	}
}

func TestGraphQL_Games_DefaultLimit(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	// Default limit (20) is bigger than the 3 seed rows — should
	// return all of them.
	resp := doGraphQL(t, h, `{ games { id } }`, nil)
	games := resp["data"].(map[string]any)["games"].([]any)
	if len(games) != 3 {
		t.Errorf("default limit should return all 3 seed rows, got %d", len(games))
	}
}

func TestGraphQL_Games_NegativeLimitErrors(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	resp := doGraphQL(t, h, `{ games(limit: -1) { id } }`, nil)
	if _, ok := resp["errors"]; !ok {
		t.Errorf("expected errors for negative limit, got %v", resp)
	}
}

// -------------------------------------------------------------------
// game(id:) — single-row query
// -------------------------------------------------------------------

func TestGraphQL_GameByID_Found(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	resp := doGraphQL(t, h, `{ game(id: 2) { id winnerName endReason } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	g := resp["data"].(map[string]any)["game"].(map[string]any)
	if int(g["id"].(float64)) != 2 {
		t.Errorf("id: want 2, got %v", g["id"])
	}
	if g["winnerName"] != "Muldrotha" {
		t.Errorf("winnerName: want Muldrotha, got %v", g["winnerName"])
	}
	if g["endReason"] != "concede" {
		t.Errorf("endReason: want concede, got %v", g["endReason"])
	}
}

func TestGraphQL_GameByID_MissingReturnsNull(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	resp := doGraphQL(t, h, `{ game(id: 9999) { id } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data := resp["data"].(map[string]any)
	if data["game"] != nil {
		t.Errorf("game: want nil, got %v", data["game"])
	}
}

// -------------------------------------------------------------------
// Selective field projection (the POC's load-bearing claim)
// -------------------------------------------------------------------

func TestGraphQL_FieldProjection_OmitsUnaskedFields(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	resp := doGraphQL(t, h, `{ game(id: 1) { id winner } }`, nil)
	g := resp["data"].(map[string]any)["game"].(map[string]any)
	// Asked for these two:
	if _, ok := g["id"]; !ok {
		t.Errorf("id missing")
	}
	if _, ok := g["winner"]; !ok {
		t.Errorf("winner missing")
	}
	// Critical: deckKeys / commanders / turns must NOT appear.
	// This is the whole point of choosing GraphQL over REST here —
	// REST would have returned the full CompletedGame body.
	for _, omitted := range []string{"deckKeys", "commanders", "turns", "endReason", "finishedAt", "winnerName"} {
		if _, ok := g[omitted]; ok {
			t.Errorf("field projection failed: %s was returned despite not being requested (response=%v)",
				omitted, g)
		}
	}
}

// -------------------------------------------------------------------
// Variables + aliasing — the cross-resource shape GraphQL pays for
// -------------------------------------------------------------------

func TestGraphQL_Variables(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	q := `query Get($id: Int!) { game(id: $id) { id winnerName } }`
	resp := doGraphQL(t, h, q, map[string]any{"id": 3})
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	g := resp["data"].(map[string]any)["game"].(map[string]any)
	if int(g["id"].(float64)) != 3 {
		t.Errorf("id: want 3, got %v", g["id"])
	}
	if g["winnerName"] != "Atraxa" {
		t.Errorf("winnerName: want Atraxa, got %v", g["winnerName"])
	}
}

func TestGraphQL_Aliasing_BatchedGameLookup(t *testing.T) {
	// One round-trip pulls three game lookups in parallel under
	// distinct aliases. Today's REST equivalent is three /api/games/{id}
	// GETs. Pinning this shape demonstrates the load-bearing
	// argument for adding GraphQL to the surface.
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	q := `{
	  g1: game(id: 1) { winnerName endReason }
	  g2: game(id: 2) { winnerName endReason }
	  g3: game(id: 3) { winnerName endReason }
	}`
	resp := doGraphQL(t, h, q, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data := resp["data"].(map[string]any)
	cases := []struct {
		alias  string
		winner string
		reason string
	}{
		{"g1", "Korvold", "lethal_combat_damage"},
		{"g2", "Muldrotha", "concede"},
		{"g3", "Atraxa", "decking"},
	}
	for _, tc := range cases {
		entry, ok := data[tc.alias].(map[string]any)
		if !ok {
			t.Errorf("alias %s missing from response: %v", tc.alias, data)
			continue
		}
		if entry["winnerName"] != tc.winner {
			t.Errorf("%s.winnerName: want %s, got %v", tc.alias, tc.winner, entry["winnerName"])
		}
		if entry["endReason"] != tc.reason {
			t.Errorf("%s.endReason: want %s, got %v", tc.alias, tc.reason, entry["endReason"])
		}
	}
}

// -------------------------------------------------------------------
// Error cases — schema-level
// -------------------------------------------------------------------

func TestGraphQL_SyntaxError_Returns200WithErrors(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	// Missing closing brace → parse failure → GraphQL spec says 200
	// with errors populated, NOT a 400. (HTTP failure modes are
	// reserved for transport / auth — the spec calls this out
	// explicitly.)
	body, _ := json.Marshal(graphqlRequest{Query: `{ games(limit: 5) { id`})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.serve(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if _, ok := resp["errors"]; !ok {
		t.Errorf("expected errors in response, got %v", resp)
	}
}

func TestGraphQL_UnknownField_Returns200WithErrors(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	resp := doGraphQL(t, h, `{ games { id totallyMadeUpField } }`, nil)
	if _, ok := resp["errors"]; !ok {
		t.Errorf("expected errors for unknown field, got %v", resp)
	}
}

func TestGraphQL_UnknownQuery_Returns200WithErrors(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	resp := doGraphQL(t, h, `{ notARealQuery { id } }`, nil)
	if _, ok := resp["errors"]; !ok {
		t.Errorf("expected errors for unknown root query, got %v", resp)
	}
}

// -------------------------------------------------------------------
// Error cases — HTTP transport (these DO use writeError 4xx)
// -------------------------------------------------------------------

func TestGraphQL_MissingQuery_400(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	body, _ := json.Marshal(graphqlRequest{Query: ""})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.serve(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rr.Code)
	}
	var err ErrorResponse
	if e := json.NewDecoder(rr.Body).Decode(&err); e != nil {
		t.Fatalf("decode ErrorResponse: %v", e)
	}
	if err.Error != "query required" || err.Status != 400 {
		t.Errorf("envelope: want {query required, 400}, got %+v", err)
	}
}

func TestGraphQL_MalformedJSON_400(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{not-json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.serve(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
}

// -------------------------------------------------------------------
// GET /graphql?query=... browser-friendly form
// -------------------------------------------------------------------

func TestGraphQL_GET_QueryParamWorks(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	q := url.QueryEscape(`{ game(id: 1) { winnerName } }`)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query="+q, nil)
	rr := httptest.NewRecorder()
	h.serve(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	g := resp["data"].(map[string]any)["game"].(map[string]any)
	if g["winnerName"] != "Korvold" {
		t.Errorf("winnerName: want Korvold, got %v", g["winnerName"])
	}
}

func TestGraphQL_GET_WithVariables(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	q := url.QueryEscape(`query Get($id: Int!) { game(id: $id) { winnerName } }`)
	vars := url.QueryEscape(`{"id": 2}`)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query="+q+"&variables="+vars, nil)
	rr := httptest.NewRecorder()
	h.serve(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	g := resp["data"].(map[string]any)["game"].(map[string]any)
	if g["winnerName"] != "Muldrotha" {
		t.Errorf("winnerName: want Muldrotha, got %v", g["winnerName"])
	}
}

func TestGraphQL_GET_MalformedVariables_400(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	q := url.QueryEscape(`{ games { id } }`)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query="+q+"&variables=not-json", nil)
	rr := httptest.NewRecorder()
	h.serve(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
}

// -------------------------------------------------------------------
// Route registration
// -------------------------------------------------------------------

func TestGraphQL_RegisterMountsBothMethods(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	mux := http.NewServeMux()
	h.RegisterGraphQL(mux)

	// Both POST and GET should route to the same handler.
	for _, m := range []string{http.MethodPost, http.MethodGet} {
		var body *bytes.Reader
		var u string
		if m == http.MethodPost {
			b, _ := json.Marshal(graphqlRequest{Query: `{ games(limit:1){id} }`})
			body = bytes.NewReader(b)
			u = "/graphql"
		} else {
			body = bytes.NewReader(nil)
			u = "/graphql?query=" + url.QueryEscape(`{ games(limit:1){id} }`)
		}
		req := httptest.NewRequest(m, u, body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s /graphql: want 200, got %d (body=%q)", m, rr.Code, rr.Body.String())
		}
	}
}
