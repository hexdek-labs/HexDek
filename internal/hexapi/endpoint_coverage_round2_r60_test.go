package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// r60 round 2 — table-driven coverage for the next three zero-coverage
// hexapi endpoint surfaces:
//
//   - Handler.handleDeckSharePage           (sharepage.go)
//   - Handler.handleCardSharePage           (sharepage_more.go)
//   - Handler.handleOperatorSharePage       (sharepage_more.go)
//   - Handler.handleCardStatsOverview       (card_stats_extended.go)
//   - Handler.handleCardStatsByCommander    (card_stats_extended.go)
//   - Handler.handleDeckUpgrade             (upgrade.go)
//
// Happy + error paths each. Error envelopes use the unified
// ErrorResponse shape (PR #93); HTML share pages assert on the rendered
// OG meta tags. Where a DB is required, an in-memory SQLite is seeded
// via the existing db package helpers.

// -------------------------------------------------------------------
// Test helpers
// -------------------------------------------------------------------

// writeDeckTxt drops a minimal deck list at decksDir/<owner>/<id>.txt
// suitable for findDeckFile + extractCommander + parseDeckList.
func writeDeckTxt(t *testing.T, decksDir, owner, id, commander string, mainboard []string) {
	t.Helper()
	dir := filepath.Join(decksDir, owner)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	lines := []string{"COMMANDER: " + commander}
	lines = append(lines, mainboard...)
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, id+".txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write deck: %v", err)
	}
}

// writeStrategyJSON drops a Freya strategy.json next to a deck so the
// share-page summary path lights up.
func writeStrategyJSON(t *testing.T, decksDir, owner, id string, payload map[string]any) {
	t.Helper()
	dir := filepath.Join(decksDir, owner, "freya")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir freya: %v", err)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal strategy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".strategy.json"), data, 0o644); err != nil {
		t.Fatalf("write strategy: %v", err)
	}
}

// openSeededDB returns an in-memory SQLite with card_stats / ELO /
// card_win_stats seeded for Sol Ring under two commanders. Used by the
// /api/card-stats happy-path cases.
func openSeededDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()

	if err := db.EnsureCardStatsSchema(ctx, d); err != nil {
		t.Fatalf("EnsureCardStatsSchema: %v", err)
	}

	// Three decks logged.
	for _, rec := range []db.ELORecord{
		{DeckKey: "alice/korvold", Commander: "Korvold", Owner: "alice", Rating: 1500, Games: 10, Wins: 6, Losses: 4, Bracket: 3},
		{DeckKey: "bob/muldrotha", Commander: "Muldrotha", Owner: "bob", Rating: 1450, Games: 8, Wins: 3, Losses: 5, Bracket: 4},
		{DeckKey: "cara/breya", Commander: "Breya", Owner: "cara", Rating: 1400, Games: 12, Wins: 6, Losses: 6, Bracket: 3},
	} {
		if err := db.UpsertELO(ctx, d, rec); err != nil {
			t.Fatalf("UpsertELO: %v", err)
		}
	}

	// BatchUpsertCardWinStats treats each row as a single game (games
	// += 1 per call, wins += excluded.wins). To produce 3 games for
	// Korvold and 4 for Muldrotha we feed individual per-game rows.
	// LoadCardStatsByCommander then filters games >= 3 — both
	// commanders clear that threshold.
	perGame := []db.CardWinStat{
		{CardName: "Sol Ring", Commander: "Korvold", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Korvold", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Korvold", Wins: 0, OnBoardAtWin: 0},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 0, OnBoardAtWin: 0},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 0, OnBoardAtWin: 0},
	}
	for _, row := range perGame {
		if err := db.BatchUpsertCardWinStats(ctx, d, []db.CardWinStat{row}); err != nil {
			t.Fatalf("BatchUpsertCardWinStats: %v", err)
		}
	}

	// Cross-commander aggregate for the win_rate side of the overview.
	if err := db.BatchUpsertCardStats(ctx, d, []db.CardStatDelta{
		{CardName: "Sol Ring", Win: 1, Loss: 0},
		{CardName: "Sol Ring", Win: 1, Loss: 0},
		{CardName: "Sol Ring", Win: 0, Loss: 1},
	}); err != nil {
		t.Fatalf("BatchUpsertCardStats: %v", err)
	}
	return d
}

// -------------------------------------------------------------------
// handleDeckSharePage
// -------------------------------------------------------------------

func TestHandleDeckSharePage(t *testing.T) {
	decksDir := t.TempDir()
	writeDeckTxt(t, decksDir, "alice", "korvold_combo", "Korvold, Fae-Cursed King", []string{"1 Sol Ring", "1 Command Tower"})
	writeStrategyJSON(t, decksDir, "alice", "korvold_combo", map[string]any{
		"gameplan_summary": "Sacrifice value chains into infinite combo.",
		"archetype":        "Combo",
	})
	// Same deck without strategy.json so the archetype-only fallback can
	// also be exercised via a separate case.
	writeDeckTxt(t, decksDir, "alice", "bare_deck", "Atraxa, Praetors' Voice", nil)

	h := &Handler{DecksDir: decksDir}

	cases := []struct {
		name             string
		owner            string
		id               string
		wantStatus       int
		wantContentType  string
		wantBodyContains []string // substrings that must appear
		wantErrMsg       string
	}{
		{
			name:            "happy path — known deck with strategy.json fills OG summary",
			owner:           "alice",
			id:              "korvold_combo",
			wantStatus:      http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
			wantBodyContains: []string{
				`<meta property="og:title" content="Korvold, Fae-Cursed King"`,
				"Sacrifice value chains into infinite combo.",
				ogMetaStartMarker,
				ogMetaEndMarker,
				"hexdek.dev/decks/alice/korvold_combo",
				"api/card-art/Korvold",
			},
		},
		{
			name:            "happy path — deck without strategy.json gets default summary",
			owner:           "alice",
			id:              "bare_deck",
			wantStatus:      http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
			wantBodyContains: []string{
				`og:title" content="Atraxa, Praetors&#39; Voice"`,
				"Commander deck on HEXDEK.",
			},
		},
		{
			name:       "invalid owner — path traversal rejected",
			owner:      "..",
			id:         "korvold_combo",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "invalid id — slash rejected",
			owner:      "alice",
			id:         "korvold/combo",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "deck not found",
			owner:      "alice",
			id:         "no_such_deck",
			wantStatus: http.StatusNotFound,
			wantErrMsg: "deck not found",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/decks/"+url.PathEscape(tc.owner)+"/"+url.PathEscape(tc.id), nil)
			req.SetPathValue("owner", tc.owner)
			req.SetPathValue("id", tc.id)
			rr := httptest.NewRecorder()

			h.handleDeckSharePage(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantErrMsg != "" {
				body := decodeErrorResponse(t, rr)
				if body.Error.Message != tc.wantErrMsg {
					t.Errorf("body.error.message: want %q, got %q", tc.wantErrMsg, body.Error.Message)
				}
				return
			}
			if ct := rr.Header().Get("Content-Type"); ct != tc.wantContentType {
				t.Errorf("Content-Type: want %q, got %q", tc.wantContentType, ct)
			}
			if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=300") {
				t.Errorf("Cache-Control: want max-age=300, got %q", cc)
			}
			body := rr.Body.String()
			for _, want := range tc.wantBodyContains {
				if !strings.Contains(body, want) {
					t.Errorf("body missing %q\nbody=%s", want, body)
				}
			}
		})
	}
}

// TestHandleDeckSharePage_IndexHTMLInjection covers the IndexHTMLPath
// branch: when the SPA's built index.html exists and contains the OG
// markers, the handler must inject deck-specific OG tags into it
// rather than serve the stub.
func TestHandleDeckSharePage_IndexHTMLInjection(t *testing.T) {
	decksDir := t.TempDir()
	writeDeckTxt(t, decksDir, "alice", "deck1", "Atraxa, Praetors' Voice", nil)

	// Synthesize a tiny index.html with the OG markers in place.
	tmpHTML := filepath.Join(t.TempDir(), "index.html")
	indexBody := "<!doctype html><html><head>" +
		ogMetaStartMarker + "\n<!-- placeholder -->\n" + ogMetaEndMarker +
		"</head><body>SPA</body></html>"
	if err := os.WriteFile(tmpHTML, []byte(indexBody), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	h := &Handler{DecksDir: decksDir, IndexHTMLPath: tmpHTML}

	req := httptest.NewRequest(http.MethodGet, "/decks/alice/deck1", nil)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "deck1")
	rr := httptest.NewRecorder()

	h.handleDeckSharePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "SPA</body>") {
		t.Errorf("expected SPA shell to survive injection: body=%q", body)
	}
	if !strings.Contains(body, "og:title") {
		t.Errorf("expected injected OG block: body=%q", body)
	}
	if strings.Contains(body, "<!-- placeholder -->") {
		t.Errorf("placeholder should have been replaced: body=%q", body)
	}
}

// -------------------------------------------------------------------
// handleCardSharePage
// -------------------------------------------------------------------

func TestHandleCardSharePage(t *testing.T) {
	h := &Handler{
		cardDB: map[string]oracleCard{
			"sol ring": {
				CMC: 1, ManaCost: "{1}", TypeLine: "Artifact",
				OracleText: "{T}: Add {C}{C}.",
			},
		},
	}

	cases := []struct {
		name             string
		pathName         string
		wantStatus       int
		wantBodyContains []string
		wantErrMsg       string
	}{
		{
			name:       "happy path — known card uses oracle text in summary",
			pathName:   "Sol Ring",
			wantStatus: http.StatusOK,
			wantBodyContains: []string{
				`og:title" content="Sol Ring"`,
				"Add {C}{C}.",
				// PathEscape encodes spaces as %20 in OG image URLs.
				"api/card-art/Sol%20Ring",
			},
		},
		{
			name:       "happy path — unknown card still renders (generic summary)",
			pathName:   "Mythic Mystery",
			wantStatus: http.StatusOK,
			wantBodyContains: []string{
				`og:title" content="Mythic Mystery"`,
				"Magic: The Gathering card on HEXDEK.",
			},
		},
		{
			name:       "empty name",
			pathName:   "",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid name",
		},
		{
			name:       "invalid percent-escape",
			pathName:   "%ZZ",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid name",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/cards/"+url.PathEscape(tc.pathName), nil)
			req.SetPathValue("name", tc.pathName)
			rr := httptest.NewRecorder()

			h.handleCardSharePage(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantErrMsg != "" {
				body := decodeErrorResponse(t, rr)
				if body.Error.Message != tc.wantErrMsg {
					t.Errorf("body.error.message: want %q, got %q", tc.wantErrMsg, body.Error.Message)
				}
				return
			}
			body := rr.Body.String()
			for _, want := range tc.wantBodyContains {
				if !strings.Contains(body, want) {
					t.Errorf("body missing %q\nbody=%s", want, body)
				}
			}
		})
	}
}

// -------------------------------------------------------------------
// handleOperatorSharePage
// -------------------------------------------------------------------

func TestHandleOperatorSharePage(t *testing.T) {
	// No DAG / ELO data on disk — handler must still render the generic
	// "operator profile on HEXDEK" stub (the loop over Leaderboard()
	// just yields nothing).
	h := &Handler{DecksDir: t.TempDir()}

	cases := []struct {
		name             string
		owner            string
		wantStatus       int
		wantBodyContains []string
		wantErrMsg       string
	}{
		{
			name:       "happy path — no DAG data still renders generic profile",
			owner:      "alice",
			wantStatus: http.StatusOK,
			wantBodyContains: []string{
				`og:title" content="ALICE"`,
				"operator profile on HEXDEK.",
			},
		},
		{
			name:       "invalid owner",
			owner:      "..",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner",
		},
		{
			name:       "owner with slash rejected",
			owner:      "alice/etc",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/operator/"+url.PathEscape(tc.owner), nil)
			req.SetPathValue("owner", tc.owner)
			rr := httptest.NewRecorder()

			h.handleOperatorSharePage(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantErrMsg != "" {
				body := decodeErrorResponse(t, rr)
				if body.Error.Message != tc.wantErrMsg {
					t.Errorf("body.error.message: want %q, got %q", tc.wantErrMsg, body.Error.Message)
				}
				return
			}
			body := rr.Body.String()
			for _, want := range tc.wantBodyContains {
				if !strings.Contains(body, want) {
					t.Errorf("body missing %q\nbody=%s", want, body)
				}
			}
		})
	}
}

// -------------------------------------------------------------------
// handleCardStatsOverview
// -------------------------------------------------------------------

func TestHandleCardStatsOverview(t *testing.T) {
	wrapper := openSeededDB(t)
	hSeeded := &Handler{}
	hSeeded.SetDB(wrapper)
	hNilDB := &Handler{}

	cases := []struct {
		name        string
		handler     *Handler
		pathName    string
		wantStatus  int
		assertResp  func(t *testing.T, resp db.CardStatsOverview)
		wantErrMsg  string
	}{
		{
			name:       "happy path — seeded DB populates win_rate, inclusion, commanders",
			handler:    hSeeded,
			pathName:   "Sol Ring",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, resp db.CardStatsOverview) {
				if resp.CardName != "Sol Ring" {
					t.Errorf("card_name: want Sol Ring, got %q", resp.CardName)
				}
				if resp.GamesPlayed != 3 {
					t.Errorf("games_played: want 3, got %d", resp.GamesPlayed)
				}
				if resp.Wins != 2 {
					t.Errorf("wins: want 2, got %d", resp.Wins)
				}
				if resp.WinRate <= 0 {
					t.Errorf("win_rate should be > 0, got %v", resp.WinRate)
				}
				if resp.DecksUsing != 2 {
					t.Errorf("decks_using: want 2 (Korvold + Muldrotha), got %d", resp.DecksUsing)
				}
				if resp.TotalDecks != 3 {
					t.Errorf("total_decks: want 3, got %d", resp.TotalDecks)
				}
				if len(resp.TopCommanders) == 0 {
					t.Errorf("top_commanders should be non-empty")
				}
				if resp.BracketDist == nil {
					t.Errorf("bracket_distribution must be non-nil even when empty")
				}
			},
		},
		{
			name:       "happy path — nil DB returns zero-value overview, not error",
			handler:    hNilDB,
			pathName:   "Sol Ring",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, resp db.CardStatsOverview) {
				if resp.CardName != "Sol Ring" {
					t.Errorf("card_name: want Sol Ring, got %q", resp.CardName)
				}
				if resp.GamesPlayed != 0 || resp.Wins != 0 {
					t.Errorf("nil DB should produce zero stats, got games=%d wins=%d", resp.GamesPlayed, resp.Wins)
				}
				if resp.TopCommanders == nil {
					t.Errorf("top_commanders must be a non-nil slice even on nil DB")
				}
				if resp.BracketDist == nil {
					t.Errorf("bracket_distribution must be a non-nil slice even on nil DB")
				}
			},
		},
		{
			name:       "empty card name",
			handler:    hSeeded,
			pathName:   "",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "missing card name",
		},
		{
			name:       "invalid percent-escape",
			handler:    hSeeded,
			pathName:   "%ZZ",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "missing card name",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/card-stats/"+url.PathEscape(tc.pathName), nil)
			req.SetPathValue("cardName", tc.pathName)
			rr := httptest.NewRecorder()

			tc.handler.handleCardStatsOverview(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantErrMsg != "" {
				body := decodeErrorResponse(t, rr)
				if body.Error.Message != tc.wantErrMsg {
					t.Errorf("body.error.message: want %q, got %q", tc.wantErrMsg, body.Error.Message)
				}
				return
			}
			var resp db.CardStatsOverview
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("decode CardStatsOverview: %v (raw=%q)", err, rr.Body.String())
			}
			tc.assertResp(t, resp)
		})
	}
}

// -------------------------------------------------------------------
// handleCardStatsByCommander
// -------------------------------------------------------------------

func TestHandleCardStatsByCommander(t *testing.T) {
	wrapper := openSeededDB(t)
	hSeeded := &Handler{}
	hSeeded.SetDB(wrapper)
	hNilDB := &Handler{}

	type byCmdrResp struct {
		CardName   string                   `json:"card_name"`
		Commanders []db.CardCommanderStat   `json:"commanders"`
	}

	cases := []struct {
		name        string
		handler     *Handler
		pathName    string
		wantStatus  int
		assertResp  func(t *testing.T, resp byCmdrResp)
		wantErrMsg  string
	}{
		{
			name:       "happy path — seeded DB returns ordered commanders",
			handler:    hSeeded,
			pathName:   "Sol Ring",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, resp byCmdrResp) {
				if resp.CardName != "Sol Ring" {
					t.Errorf("card_name: want Sol Ring, got %q", resp.CardName)
				}
				if len(resp.Commanders) != 2 {
					t.Fatalf("commanders: want 2, got %d (resp=%+v)", len(resp.Commanders), resp)
				}
				// Muldrotha has 4 games > Korvold 3, must sort first.
				if resp.Commanders[0].Commander != "Muldrotha" {
					t.Errorf("top commander: want Muldrotha, got %s", resp.Commanders[0].Commander)
				}
			},
		},
		{
			name:       "happy path — nil DB returns empty commanders list (not nil)",
			handler:    hNilDB,
			pathName:   "Sol Ring",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, resp byCmdrResp) {
				if resp.Commanders == nil {
					t.Errorf("commanders must be a non-nil slice")
				}
				if len(resp.Commanders) != 0 {
					t.Errorf("nil DB should produce empty commanders, got %d", len(resp.Commanders))
				}
			},
		},
		{
			name:       "happy path — unknown card returns empty list, not 404",
			handler:    hSeeded,
			pathName:   "Mythic Mystery",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, resp byCmdrResp) {
				if len(resp.Commanders) != 0 {
					t.Errorf("unknown card: want empty, got %d", len(resp.Commanders))
				}
			},
		},
		{
			name:       "empty card name",
			handler:    hSeeded,
			pathName:   "",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "missing card name",
		},
		{
			name:       "invalid percent-escape",
			handler:    hSeeded,
			pathName:   "%ZZ",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "missing card name",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/card-stats/"+url.PathEscape(tc.pathName)+"/by-commander", nil)
			req.SetPathValue("cardName", tc.pathName)
			rr := httptest.NewRecorder()

			tc.handler.handleCardStatsByCommander(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantErrMsg != "" {
				body := decodeErrorResponse(t, rr)
				if body.Error.Message != tc.wantErrMsg {
					t.Errorf("body.error.message: want %q, got %q", tc.wantErrMsg, body.Error.Message)
				}
				return
			}
			var resp byCmdrResp
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("decode by-commander: %v (raw=%q)", err, rr.Body.String())
			}
			tc.assertResp(t, resp)
		})
	}
}

// -------------------------------------------------------------------
// handleDeckUpgrade
// -------------------------------------------------------------------

func TestHandleDeckUpgrade(t *testing.T) {
	decksDir := t.TempDir()
	writeDeckTxt(t, decksDir, "alice", "deck1", "Korvold", []string{
		"1 Sol Ring",
		"1 Command Tower",
	})

	cardDB := map[string]oracleCard{
		"sol ring":      {CMC: 1, ManaCost: "{1}", TypeLine: "Artifact"},
		"command tower": {CMC: 0, ManaCost: "", TypeLine: "Land"},
	}

	type upgradeResp struct {
		Suggestions []UpgradeSuggestion `json:"suggestions"`
		Meta        map[string]any      `json:"meta"`
	}

	cases := []struct {
		name        string
		setup       func(t *testing.T) *Handler
		owner       string
		id          string
		wantStatus  int
		assertResp  func(t *testing.T, resp upgradeResp)
		wantErrMsg  string
	}{
		{
			name: "no card oracle DB loaded — empty suggestions with meta reason",
			setup: func(t *testing.T) *Handler {
				return &Handler{DecksDir: decksDir} // nil cardDB
			},
			owner:      "alice",
			id:         "deck1",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, resp upgradeResp) {
				if len(resp.Suggestions) != 0 {
					t.Errorf("suggestions: want empty, got %+v", resp.Suggestions)
				}
				reason, _ := resp.Meta["reason"].(string)
				if !strings.Contains(reason, "card oracle DB") {
					t.Errorf("meta.reason: want oracle-DB reason, got %q", reason)
				}
			},
		},
		{
			name: "no card_stats data — empty suggestions with meta reason",
			setup: func(t *testing.T) *Handler {
				wrapper := openSeededDB(t)
				// Drop the seeded card_stats rows so the handler sees an empty
				// stats map after loadUpgradeCardStats.
				if _, err := wrapper.Exec("DELETE FROM card_stats"); err != nil {
					t.Fatalf("clear card_stats: %v", err)
				}
				h := &Handler{DecksDir: decksDir, cardDB: cardDB}
				h.SetDB(wrapper)
				return h
			},
			owner:      "alice",
			id:         "deck1",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, resp upgradeResp) {
				if len(resp.Suggestions) != 0 {
					t.Errorf("suggestions: want empty, got %+v", resp.Suggestions)
				}
				reason, _ := resp.Meta["reason"].(string)
				if !strings.Contains(reason, "no card_stats") {
					t.Errorf("meta.reason: want no-stats reason, got %q", reason)
				}
			},
		},
		{
			name: "happy path — with stats coverage, Mana Crypt suggested over Sol Ring",
			setup: func(t *testing.T) *Handler {
				// Seed enough card_stats games for both Sol Ring (current
				// card in the deck, needs >= upgradeMinCurrentGames=10) and
				// Mana Crypt (candidate, needs >= upgradeMinCandidateGames=
				// 25) so the inner loop produces an actual suggestion.
				// Sol Ring: 12 games, 4 wins -> ~33% WR.
				// Mana Crypt: 30 games, 24 wins -> 80% WR. Delta 47 > 5pp.
				d, err := db.Open(":memory:")
				if err != nil {
					t.Fatalf("db.Open: %v", err)
				}
				t.Cleanup(func() { d.Close() })
				ctx := context.Background()
				if err := db.EnsureCardStatsSchema(ctx, d); err != nil {
					t.Fatalf("EnsureCardStatsSchema: %v", err)
				}
				solRing := make([]db.CardStatDelta, 0, 12)
				for i := 0; i < 12; i++ {
					win := 0
					if i < 4 {
						win = 1
					}
					solRing = append(solRing, db.CardStatDelta{CardName: "Sol Ring", Win: win, Loss: 1 - win})
				}
				manaCrypt := make([]db.CardStatDelta, 0, 30)
				for i := 0; i < 30; i++ {
					win := 0
					if i < 24 {
						win = 1
					}
					manaCrypt = append(manaCrypt, db.CardStatDelta{CardName: "Mana Crypt", Win: win, Loss: 1 - win})
				}
				if err := db.BatchUpsertCardStats(ctx, d, solRing); err != nil {
					t.Fatalf("seed Sol Ring stats: %v", err)
				}
				if err := db.BatchUpsertCardStats(ctx, d, manaCrypt); err != nil {
					t.Fatalf("seed Mana Crypt stats: %v", err)
				}
				cardDBWithCrypt := map[string]oracleCard{
					"sol ring":      {CMC: 1, ManaCost: "{1}", TypeLine: "Artifact"},
					"mana crypt":    {CMC: 0, ManaCost: "", TypeLine: "Artifact"},
					"command tower": {CMC: 0, ManaCost: "", TypeLine: "Land"},
				}
				h := &Handler{DecksDir: decksDir, cardDB: cardDBWithCrypt}
				h.SetDB(d)
				return h
			},
			owner:      "alice",
			id:         "deck1",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, resp upgradeResp) {
				if resp.Meta == nil {
					t.Fatalf("meta must be populated on the stats-coverage path")
				}
				if _, ok := resp.Meta["min_current_games"]; !ok {
					t.Errorf("meta missing min_current_games: %+v", resp.Meta)
				}
				if _, ok := resp.Meta["deck_size"]; !ok {
					t.Errorf("meta missing deck_size: %+v", resp.Meta)
				}
				// Mana Crypt should be the suggested upgrade for Sol Ring
				// (same type, colorless, CMC 0 <= 1, win-rate delta well
				// above the 5pp threshold).
				found := false
				for _, s := range resp.Suggestions {
					if s.CurrentCard == "Sol Ring" && s.SuggestedCard == "Mana Crypt" {
						found = true
						if s.WinRateDelta < upgradeMinDeltaPct {
							t.Errorf("win_rate_delta below threshold: %v", s.WinRateDelta)
						}
					}
				}
				if !found {
					t.Errorf("expected Sol Ring -> Mana Crypt suggestion, got %+v", resp.Suggestions)
				}
			},
		},
		{
			name:       "invalid owner — path traversal rejected",
			setup:      func(t *testing.T) *Handler { return &Handler{DecksDir: decksDir} },
			owner:      "..",
			id:         "deck1",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "invalid id — slash rejected",
			setup:      func(t *testing.T) *Handler { return &Handler{DecksDir: decksDir} },
			owner:      "alice",
			id:         "deck/1",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "deck not found",
			setup:      func(t *testing.T) *Handler { return &Handler{DecksDir: decksDir} },
			owner:      "alice",
			id:         "no_such_deck",
			wantStatus: http.StatusNotFound,
			wantErrMsg: "deck not found",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := tc.setup(t)
			req := httptest.NewRequest(http.MethodGet, "/api/decks/"+url.PathEscape(tc.owner)+"/"+url.PathEscape(tc.id)+"/upgrade", nil)
			req.SetPathValue("owner", tc.owner)
			req.SetPathValue("id", tc.id)
			rr := httptest.NewRecorder()

			h.handleDeckUpgrade(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantErrMsg != "" {
				body := decodeErrorResponse(t, rr)
				if body.Error.Message != tc.wantErrMsg {
					t.Errorf("body.error.message: want %q, got %q", tc.wantErrMsg, body.Error.Message)
				}
				return
			}
			var resp upgradeResp
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("decode upgrade resp: %v (raw=%q)", err, rr.Body.String())
			}
			tc.assertResp(t, resp)
		})
	}
}
