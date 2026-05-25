package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/oracle"
)

// deck_budget_r60_test.go — HTTP-level integration tests for
// GET /api/decks/{owner}/{id}/budget. Stubs the Scryfall API via
// oracle.SetScryfallBaseForTest so the bulk /cards/collection call
// resolves with deterministic prices.

// budgetStub is a minimal Scryfall stub that answers /cards/collection
// with the canned price map keyed by lowercased card name. Tracks the
// number of HTTP hits so we can assert the cache short-circuits a
// second budget request.
type budgetStub struct {
	server *httptest.Server
	hits   int32
	prices map[string]map[string]string // lowercased name → {usd:"1.23",...}
}

func newBudgetStub(t *testing.T, prices map[string]map[string]string) *budgetStub {
	t.Helper()
	b := &budgetStub{prices: prices}
	b.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&b.hits, 1)
		switch {
		case strings.Contains(r.URL.Path, "/cards/collection"):
			var req struct {
				Identifiers []struct{ Name string } `json:"identifiers"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			data := []map[string]any{}
			notFound := []map[string]string{}
			for _, ident := range req.Identifiers {
				k := strings.ToLower(strings.TrimSpace(ident.Name))
				p, ok := b.prices[k]
				if !ok {
					notFound = append(notFound, map[string]string{"name": ident.Name})
					continue
				}
				data = append(data, map[string]any{
					"object":    "card",
					"id":        k + "-id",
					"name":      ident.Name,
					"type_line": "Test",
					"prices":    p,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":      data,
				"not_found": notFound,
			})
		case strings.Contains(r.URL.Path, "/cards/named"):
			name := r.URL.Query().Get("fuzzy")
			k := strings.ToLower(strings.TrimSpace(name))
			p := b.prices[k]
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": "card", "id": k + "-id", "name": name,
				"type_line": "Test", "prices": p,
			})
		default:
			w.WriteHeader(404)
		}
	}))
	return b
}

func writeDeckFile(t *testing.T, decksDir, owner, id, body string) {
	t.Helper()
	dir := filepath.Join(decksDir, owner)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir owner: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write deck: %v", err)
	}
}

func TestHandleDeckBudget_SumsPricesAndExcludesBasics(t *testing.T) {
	defer oracle.SetScryfallLimitIntervalForTest(5 * time.Millisecond)()
	stub := newBudgetStub(t, map[string]map[string]string{
		"mana crypt":   {"usd": "89.99"},
		"sol ring":     {"usd": "1.50"},
		"lightning bolt": {"usd": "0.25"},
		// Forest deliberately absent — should never be queried (basic).
	})
	defer stub.server.Close()
	defer oracle.SetScryfallBaseForTest(stub.server.URL)()

	h, decksDir := newVersionTrendsEnv(t)
	writeDeckFile(t, decksDir, "alice", "burn", strings.Join([]string{
		"1 Mana Crypt",
		"2 Lightning Bolt",
		"1 Sol Ring",
		"30 Forest", // basic — skipped, not part of total
	}, "\n"))

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/burn/budget", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}

	if resp["currency"] != "USD" {
		t.Errorf("currency: want USD, got %v", resp["currency"])
	}
	// 89.99 + 1.50 + (0.25 * 2) = 91.99
	if got := resp["total"].(float64); got != 91.99 {
		t.Errorf("total: want 91.99, got %v", got)
	}
	if got := resp["card_count"].(float64); int(got) != 34 {
		t.Errorf("card_count: want 34, got %v", got)
	}
	if got := resp["unique_count"].(float64); int(got) != 4 {
		t.Errorf("unique_count: want 4, got %v", got)
	}
	if got := resp["priced_count"].(float64); int(got) != 4 {
		// Mana Crypt + Bolt + Sol Ring + Forest (basic flagged priced=true at $0).
		t.Errorf("priced_count: want 4, got %v", got)
	}
	if got := resp["missing_count"].(float64); int(got) != 0 {
		t.Errorf("missing_count: want 0, got %v", got)
	}

	// top_expensive must order by line value desc.
	top := resp["top_expensive"].([]any)
	if len(top) == 0 {
		t.Fatal("top_expensive is empty")
	}
	first := top[0].(map[string]any)
	if first["name"] != "Mana Crypt" {
		t.Errorf("top_expensive[0]: want Mana Crypt, got %v", first["name"])
	}
	if first["line"].(float64) != 89.99 {
		t.Errorf("top_expensive[0].line: want 89.99, got %v", first["line"])
	}
}

func TestHandleDeckBudget_MissingPricesAreFlagged(t *testing.T) {
	defer oracle.SetScryfallLimitIntervalForTest(5 * time.Millisecond)()
	stub := newBudgetStub(t, map[string]map[string]string{
		"sol ring": {"usd": "1.50"},
		// Mystery Card has no prices entry — Scryfall returns it in
		// not_found, so RefreshPrices won't surface it either.
	})
	defer stub.server.Close()
	defer oracle.SetScryfallBaseForTest(stub.server.URL)()

	h, decksDir := newVersionTrendsEnv(t)
	writeDeckFile(t, decksDir, "alice", "mystery", strings.Join([]string{
		"1 Sol Ring",
		"1 Mystery Card",
	}, "\n"))

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/mystery/budget", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)

	if got := resp["total"].(float64); got != 1.50 {
		t.Errorf("total: want 1.50, got %v", got)
	}
	if got := resp["priced_count"].(float64); int(got) != 1 {
		t.Errorf("priced_count: want 1, got %v", got)
	}
	if got := resp["missing_count"].(float64); int(got) != 1 {
		t.Errorf("missing_count: want 1, got %v", got)
	}

	// The per-card row for Mystery Card must carry priced=false.
	cards := resp["cards"].([]any)
	var mystery map[string]any
	for _, c := range cards {
		m := c.(map[string]any)
		if m["name"] == "Mystery Card" {
			mystery = m
			break
		}
	}
	if mystery == nil {
		t.Fatal("Mystery Card missing from cards[]")
	}
	if mystery["priced"].(bool) {
		t.Errorf("Mystery Card should be priced=false")
	}
	if mystery["line"].(float64) != 0 {
		t.Errorf("Mystery Card line: want 0, got %v", mystery["line"])
	}
}

func TestHandleDeckBudget_EmptyDeckRendersZero(t *testing.T) {
	defer oracle.SetScryfallLimitIntervalForTest(5 * time.Millisecond)()
	stub := newBudgetStub(t, map[string]map[string]string{})
	defer stub.server.Close()
	defer oracle.SetScryfallBaseForTest(stub.server.URL)()

	h, decksDir := newVersionTrendsEnv(t)
	writeDeckFile(t, decksDir, "alice", "empty", "")

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/empty/budget", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if got := resp["total"].(float64); got != 0 {
		t.Errorf("empty total: want 0, got %v", got)
	}
}

func TestHandleDeckBudget_DeckNotFound(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/nope/budget", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleDeckBudget_SecondRequestHitsCache(t *testing.T) {
	defer oracle.SetScryfallLimitIntervalForTest(5 * time.Millisecond)()
	stub := newBudgetStub(t, map[string]map[string]string{
		"sol ring": {"usd": "1.50"},
	})
	defer stub.server.Close()
	defer oracle.SetScryfallBaseForTest(stub.server.URL)()

	h, decksDir := newVersionTrendsEnv(t)
	writeDeckFile(t, decksDir, "alice", "ring", "1 Sol Ring\n")

	mux := http.NewServeMux()
	h.Register(mux)

	req1 := httptest.NewRequest("GET", "/api/decks/alice/ring/budget", nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status: want 200, got %d (%s)", rec1.Code, rec1.Body.String())
	}
	hitsAfterFirst := atomic.LoadInt32(&stub.hits)
	if hitsAfterFirst == 0 {
		t.Fatal("first request should have hit Scryfall")
	}

	req2 := httptest.NewRequest("GET", "/api/decks/alice/ring/budget", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second request status: want 200, got %d", rec2.Code)
	}
	if atomic.LoadInt32(&stub.hits) != hitsAfterFirst {
		t.Errorf("second request should have served entirely from cache; hits went %d → %d",
			hitsAfterFirst, atomic.LoadInt32(&stub.hits))
	}
}

func TestHandleDeckBudget_InvalidOwnerOrID(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/..%2Fetc/foo/budget", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("path traversal should not 200, got %d", rec.Code)
	}
}
