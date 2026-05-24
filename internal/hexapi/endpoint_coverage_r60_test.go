package hexapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// r60 — table-driven coverage for the three hexapi endpoints that
// reported 0% line coverage before this change:
//
//   - ArtCacheHandler           (artcache.go)
//   - Handler.handleCardSearch  (cards.go)
//   - Handler.handleCardByName  (cards.go)
//
// Each test exercises the happy path plus error branches. ErrorResponse
// is now uniform across hexapi (r60), so we can assert the JSON shape
// directly: {"error": "...", "status": <code>}.

// decodeErrorResponse decodes a hexapi error body and returns it,
// failing the test on a structural mismatch. Centralizing this lets
// the table cases stay terse.
func decodeErrorResponse(t *testing.T, rr *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: want application/json, got %q (body=%q)", ct, rr.Body.String())
	}
	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v (raw=%q)", err, rr.Body.String())
	}
	if body.Status != rr.Code {
		t.Errorf("body.status (%d) does not match HTTP status (%d)", body.Status, rr.Code)
	}
	return body
}

// -------------------------------------------------------------------
// ArtCacheHandler
// -------------------------------------------------------------------

// TestArtCacheHandler exercises the three exit paths of
// ArtCacheHandler: cache hit (200 image/jpeg), cache miss (302 to
// Scryfall), and missing path value (400 with unified error shape).
// The cache key normalization (lowercase + split on "//") is verified
// implicitly by seeding the cache under the expected hash and hitting
// the endpoint with mixed case / DFC-form names.
func TestArtCacheHandler(t *testing.T) {
	cacheDir := t.TempDir()

	// Seed two cache entries to drive the cache-hit cases.
	seed := func(displayName string, payload []byte) {
		key := strings.TrimSpace(strings.Split(displayName, "//")[0])
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(key))))
		if err := os.WriteFile(filepath.Join(cacheDir, hash+".jpg"), payload, 0o644); err != nil {
			t.Fatalf("seed %s: %v", displayName, err)
		}
	}
	cyclonicJPEG := []byte("\xff\xd8\xff\xe0CYCLONICRIFT")
	urAragornJPEG := []byte("\xff\xd8\xff\xe0URARAGORN")
	seed("Cyclonic Rift", cyclonicJPEG)
	seed("Ur-Aragorn // Reforged Blade", urAragornJPEG)

	handler := ArtCacheHandler(cacheDir)

	type wantHit struct {
		body        []byte
		contentType string
		cacheCtrl   string
	}
	cases := []struct {
		name         string
		pathName     string
		wantStatus   int
		wantHit      *wantHit
		wantRedirect bool // 302 to Scryfall
		wantErrMsg   string
	}{
		{
			name:       "cache hit — exact lowercase normalization",
			pathName:   "Cyclonic Rift",
			wantStatus: http.StatusOK,
			wantHit: &wantHit{
				body:        cyclonicJPEG,
				contentType: "image/jpeg",
				cacheCtrl:   "public, max-age=2592000",
			},
		},
		{
			name:       "cache hit — DFC name normalizes via // split",
			pathName:   "Ur-Aragorn // Reforged Blade",
			wantStatus: http.StatusOK,
			wantHit: &wantHit{
				body:        urAragornJPEG,
				contentType: "image/jpeg",
				cacheCtrl:   "public, max-age=2592000",
			},
		},
		{
			name:         "cache miss — redirects to Scryfall",
			pathName:     "Black Lotus",
			wantStatus:   http.StatusFound,
			wantRedirect: true,
		},
		{
			name:       "missing path value — unified error",
			pathName:   "",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "missing card name",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/card-art/"+url.PathEscape(tc.pathName), nil)
			req.SetPathValue("name", tc.pathName)
			rr := httptest.NewRecorder()

			handler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}

			switch {
			case tc.wantHit != nil:
				if ct := rr.Header().Get("Content-Type"); ct != tc.wantHit.contentType {
					t.Errorf("Content-Type: want %q, got %q", tc.wantHit.contentType, ct)
				}
				if cc := rr.Header().Get("Cache-Control"); cc != tc.wantHit.cacheCtrl {
					t.Errorf("Cache-Control: want %q, got %q", tc.wantHit.cacheCtrl, cc)
				}
				if got := rr.Body.Bytes(); string(got) != string(tc.wantHit.body) {
					t.Errorf("body: want %q, got %q", tc.wantHit.body, got)
				}
			case tc.wantRedirect:
				loc := rr.Header().Get("Location")
				if !strings.HasPrefix(loc, "https://api.scryfall.com/cards/named?exact=") {
					t.Errorf("Location: want Scryfall redirect, got %q", loc)
				}
				if cc := rr.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
					t.Errorf("Cache-Control: want short cache for miss, got %q", cc)
				}
			case tc.wantErrMsg != "":
				body := decodeErrorResponse(t, rr)
				if body.Error != tc.wantErrMsg {
					t.Errorf("body.error: want %q, got %q", tc.wantErrMsg, body.Error)
				}
			}
		})
	}
}

// -------------------------------------------------------------------
// Handler.handleCardSearch
// -------------------------------------------------------------------

// newTestHandlerWithCardDB returns a Handler whose cardDB contains a
// small fixed set of cards. Names are stored lowercased as the loader
// does — the case the handler receives in the query is irrelevant.
func newTestHandlerWithCardDB() *Handler {
	return &Handler{
		cardDB: map[string]oracleCard{
			"cyclonic rift": {
				CMC:      6, ManaCost: "{5}{U}", TypeLine: "Instant",
				OracleText: "Return all nonland permanents target opponent controls to their owners' hands.",
				Set:        "rtr",
			},
			"sol ring": {
				CMC: 1, ManaCost: "{1}", TypeLine: "Artifact",
				OracleText: "Add {C}{C}.", Set: "lea",
			},
			"counterspell": {
				CMC: 2, ManaCost: "{U}{U}", TypeLine: "Instant",
				OracleText: "Counter target spell.", Set: "lea",
			},
		},
	}
}

// TestHandleCardSearch exercises the happy path (matches found, sort
// order, image_uri shape) plus the empty-corpus and empty-query
// branches. Both empty branches must return 200 with an empty results
// array — they aren't error states.
func TestHandleCardSearch(t *testing.T) {
	h := newTestHandlerWithCardDB()
	hEmpty := &Handler{} // nil cardDB

	type cardSearchResp struct {
		Query   string          `json:"query"`
		Count   int             `json:"count"`
		Results []CardSearchHit `json:"results"`
	}

	cases := []struct {
		name        string
		handler     *Handler
		query       string
		limit       string
		wantStatus  int
		wantCount   int     // -1 = unchecked
		wantTopName string  // "" = unchecked
		wantEmpty   bool
	}{
		{
			name:        "happy path — prefix match returns hit with image_uri",
			handler:     h,
			query:       "cyclonic",
			wantStatus:  http.StatusOK,
			wantCount:   1,
			wantTopName: "Cyclonic Rift",
		},
		{
			name:       "substring match across multiple cards",
			handler:    h,
			query:      "n", // matches "Cyclonic Rift" and "Counterspell" and "Sol Ring"
			wantStatus: http.StatusOK,
			wantCount:  -1, // multi-match — count depends on matchScore, just assert >0
		},
		{
			name:        "case insensitive match",
			handler:     h,
			query:       "SOL RING",
			wantStatus:  http.StatusOK,
			wantCount:   1,
			wantTopName: "Sol Ring",
		},
		{
			name:       "limit clamps to top N",
			handler:    h,
			query:      "n",
			limit:      "1",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "empty query returns empty results, not error",
			handler:    h,
			query:      "",
			wantStatus: http.StatusOK,
			wantEmpty:  true,
		},
		{
			name:       "whitespace-only query treated as empty",
			handler:    h,
			query:      "   ",
			wantStatus: http.StatusOK,
			wantEmpty:  true,
		},
		{
			name:       "empty cardDB returns empty results, not error",
			handler:    hEmpty,
			query:      "anything",
			wantStatus: http.StatusOK,
			wantEmpty:  true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vals := url.Values{}
			vals.Set("q", tc.query)
			if tc.limit != "" {
				vals.Set("limit", tc.limit)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/cards/search?"+vals.Encode(), nil)
			rr := httptest.NewRecorder()

			tc.handler.handleCardSearch(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}

			var body cardSearchResp
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v (raw=%q)", err, rr.Body.String())
			}

			if tc.wantEmpty {
				if len(body.Results) != 0 {
					t.Errorf("want empty results, got %d", len(body.Results))
				}
				return
			}
			if tc.wantCount > 0 && len(body.Results) != tc.wantCount {
				t.Errorf("results count: want %d, got %d (results=%+v)", tc.wantCount, len(body.Results), body.Results)
			}
			if tc.wantCount == -1 && len(body.Results) == 0 {
				t.Errorf("expected at least one result for substring query, got none")
			}
			if tc.wantTopName != "" {
				if len(body.Results) == 0 || body.Results[0].Name != tc.wantTopName {
					t.Errorf("top result: want %q, got %+v", tc.wantTopName, body.Results)
				}
				if !strings.HasPrefix(body.Results[0].ImageURI, "/api/card-art/") {
					t.Errorf("image_uri: want /api/card-art/... prefix, got %q", body.Results[0].ImageURI)
				}
			}
		})
	}
}

// -------------------------------------------------------------------
// Handler.handleCardByName
// -------------------------------------------------------------------

// TestHandleCardByName covers the three exit paths of the /api/cards/{name}
// endpoint: happy path (200 + CardDetail JSON), invalid URL-escape (400),
// and unknown card (404). Both error paths must produce the unified
// ErrorResponse envelope.
func TestHandleCardByName(t *testing.T) {
	h := newTestHandlerWithCardDB()

	cases := []struct {
		name        string
		pathName    string
		wantStatus  int
		wantName    string
		wantErrMsg  string
	}{
		{
			name:       "happy path — known card returns CardDetail",
			pathName:   "Cyclonic Rift",
			wantStatus: http.StatusOK,
			wantName:   "Cyclonic Rift",
		},
		{
			name:       "case-insensitive lookup",
			pathName:   "sol ring",
			wantStatus: http.StatusOK,
			wantName:   "Sol Ring",
		},
		{
			name:       "URL-encoded path resolves",
			pathName:   "Sol%20Ring",
			wantStatus: http.StatusOK,
			wantName:   "Sol Ring",
		},
		{
			name:       "invalid percent-escape",
			pathName:   "%ZZ",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid name",
		},
		{
			name:       "empty name",
			pathName:   "",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid name",
		},
		{
			name:       "unknown card",
			pathName:   "Black Lotus",
			wantStatus: http.StatusNotFound,
			wantErrMsg: "card not found",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Path-escape so URL parsing succeeds; the handler decodes
			// via url.PathUnescape from r.PathValue, which we set
			// directly with the raw (already-encoded) path segment.
			req := httptest.NewRequest(http.MethodGet, "/api/cards/"+url.PathEscape(tc.pathName), nil)
			req.SetPathValue("name", tc.pathName)
			rr := httptest.NewRecorder()

			h.handleCardByName(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}

			if tc.wantErrMsg != "" {
				body := decodeErrorResponse(t, rr)
				if body.Error != tc.wantErrMsg {
					t.Errorf("body.error: want %q, got %q", tc.wantErrMsg, body.Error)
				}
				return
			}

			var detail CardDetail
			if err := json.NewDecoder(rr.Body).Decode(&detail); err != nil {
				t.Fatalf("decode CardDetail: %v (raw=%q)", err, rr.Body.String())
			}
			if detail.Name != tc.wantName {
				t.Errorf("detail.name: want %q, got %q", tc.wantName, detail.Name)
			}
			if !strings.HasPrefix(detail.ImageURI, "/api/card-art/") {
				t.Errorf("detail.image_uri: want /api/card-art/... prefix, got %q", detail.ImageURI)
			}
			if detail.RoleDistribution == nil {
				t.Errorf("detail.role_distribution must be non-nil even when empty")
			}
			if detail.DecksUsing == nil {
				t.Errorf("detail.decks_using must be non-nil even when empty")
			}
		})
	}
}
