package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/credits"
	"github.com/hexdek/hexdek/internal/deckparser"
)

// r60 — pin the "no credit charge for failed gauntlet launches" rule.
//
// Bug history: handleStartGauntlet (showmatch.go) used to charge
// credits via credits.Spend BEFORE checking that the deck existed in
// the engine pool and BEFORE acquiring the global gauntletSem. The
// inline comment explicitly admitted the gap: "we don't refund on a
// launch failure below — the spend is committed". Practical
// consequences for any user past their free-tier daily quota:
//
//   - A typo in the deck id returned 200 with credits_charged set,
//     but RunGauntlet then noticed the deck wasn't in sm.deckPool,
//     set status="error" on the in-memory map, and exited. The user's
//     credits were gone with nothing to show.
//   - Two concurrent paid runs against an already-saturated
//     gauntletSem also charged the third user before the 429.
//
// Fix: reorder so the cheap deck-existence + sem-availability gates
// run before the credit Spend. credits.Spend is atomic — either it
// succeeds (and the gauntlet really is going to launch) or it fails
// (and nothing is debited). Tests below pin both regression cases
// plus a paid-happy-path sanity check that the reorder didn't break
// the normal flow.

// gauntletRefundFixture builds a Showmatch instance suitable for the
// gauntlet handler tests:
//   - in-memory SQLite with credits schema applied
//   - credits.Store wired in (sm.credits)
//   - empty gauntlets / gauntletSubs / elo maps so the goroutine path
//     in RunGauntlet is nil-safe
//   - sm.sqlDB intentionally left nil so the goroutine's
//     persist-snapshot tail is skipped (avoids racing the DB close
//     in t.Cleanup if RunGauntlet outlives the test).
type gauntletRefundFixture struct {
	sm    *Showmatch
	store *credits.Store
	db    *sql.DB
	owner string
}

func newGauntletRefundFixture(t *testing.T) *gauntletRefundFixture {
	t.Helper()
	d, err := openCreditsDB(t)
	if err != nil {
		t.Fatalf("open credits db: %v", err)
	}
	store := credits.New(d)

	sm := &Showmatch{
		elo:          map[string]*eloState{},
		gauntlets:    map[string]*GauntletResult{},
		gauntletSubs: map[string]map[*gauntletSubscriber]struct{}{},
		credits:      store,
	}
	return &gauntletRefundFixture{sm: sm, store: store, db: d, owner: "alice"}
}

// openCreditsDB returns an in-memory SQLite with the credits schema
// applied. Independent of internal/db so this test doesn't drag in
// the full hexdek schema for what is a focused contract.
func openCreditsDB(t *testing.T) (*sql.DB, error) {
	t.Helper()
	// modernc.org/sqlite is already imported transitively by internal/db
	// when the package compiles; opening it directly here keeps the
	// fixture independent of the full hexdek schema.
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { d.Close() })
	if err := credits.EnsureSchema(context.Background(), d); err != nil {
		return nil, err
	}
	return d, nil
}

// putUserInPaidTier exhausts the free-tier quota for the day and tops
// up the user's balance so QuotaState reports CanRunPaid=true. This is
// the state in which the bug actually manifests — the handler's free
// path doesn't call Spend.
func (f *gauntletRefundFixture) putUserInPaidTier(t *testing.T, balance int64) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().Unix()
	// Three gauntlet_usage rows for today -> CanRunFree=false.
	for i := 0; i < credits.FreeGauntletsPerDay; i++ {
		if _, err := f.db.ExecContext(ctx, `
			INSERT INTO gauntlet_usage (owner, deck_key, games, free, credits_charged, started_at)
			VALUES (?, 'alice/seed', 100, 1, 0, ?)
		`, f.owner, now); err != nil {
			t.Fatalf("seed gauntlet_usage: %v", err)
		}
	}
	if balance > 0 {
		if _, err := f.store.Earn(ctx, f.owner, balance, "test_topup", ""); err != nil {
			t.Fatalf("Earn: %v", err)
		}
	}
}

// addDeckToPool injects a fake TournamentDeck into sm.deckPool so
// findDeckInPool(owner, id) returns non-nil. The path is constructed
// to round-trip through deckKeyFromPath as owner/id.
func (f *gauntletRefundFixture) addDeckToPool(owner, id, commanderName string) {
	path := filepath.Join("/decks", owner, id+".txt")
	f.sm.deckPool = append(f.sm.deckPool, &deckparser.TournamentDeck{
		Path:          path,
		CommanderName: commanderName,
	})
}

// gauntletUsageRowsFor returns the count of gauntlet_usage rows the
// handler wrote for owner. A successful charge always emits a row;
// the bug-regression tests assert this count stays at the pre-seeded
// baseline (FreeGauntletsPerDay rows from putUserInPaidTier) when
// the launch fails.
func (f *gauntletRefundFixture) gauntletUsageRowsFor(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM gauntlet_usage WHERE owner = ?`, f.owner).Scan(&n); err != nil {
		t.Fatalf("count gauntlet_usage: %v", err)
	}
	return n
}

func (f *gauntletRefundFixture) currentBalance(t *testing.T) int64 {
	t.Helper()
	bal, err := f.store.GetBalance(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	return bal
}

// makeStartReq builds a POST /api/showmatch/gauntlet/{owner}/{id}
// request with the X-HexDek-Owner identity header set so the credits
// gate engages.
func (f *gauntletRefundFixture) makeStartReq(owner, id string) *http.Request {
	req := httptest.NewRequest(http.MethodPost,
		"/api/showmatch/gauntlet/"+owner+"/"+id, nil)
	req.SetPathValue("owner", owner)
	req.SetPathValue("id", id)
	req.Header.Set("X-HexDek-Owner", f.owner)
	return req
}

// TestStartGauntlet_DeckNotInPool_DoesNotChargeCredits is the primary
// regression for the bug. Pre-fix, this test would observe a debited
// balance and a freshly-inserted gauntlet_usage row even though no
// gauntlet actually ran. Post-fix, the 404 short-circuits before
// credits.Spend, so the balance stays put.
func TestStartGauntlet_DeckNotInPool_DoesNotChargeCredits(t *testing.T) {
	f := newGauntletRefundFixture(t)
	f.putUserInPaidTier(t, 10) // 2x CreditsPerGauntlet

	baselineUsage := f.gauntletUsageRowsFor(t)
	baselineBalance := f.currentBalance(t)
	if baselineBalance != 10 {
		t.Fatalf("baseline balance: want 10, got %d", baselineBalance)
	}

	// Deck pool is empty — findDeckInPool returns nil.
	req := f.makeStartReq("alice", "no_such_deck")
	rr := httptest.NewRecorder()

	f.sm.handleStartGauntlet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != http.StatusNotFound {
		t.Errorf("body.status: want 404, got %d", body.Status)
	}

	// REGRESSION ASSERTIONS — these are the heart of the fix.
	if got := f.currentBalance(t); got != baselineBalance {
		t.Errorf("balance was debited despite launch failure: baseline=%d, after=%d (the bug)",
			baselineBalance, got)
	}
	if got := f.gauntletUsageRowsFor(t); got != baselineUsage {
		t.Errorf("gauntlet_usage gained a row despite launch failure: baseline=%d, after=%d",
			baselineUsage, got)
	}
}

// TestStartGauntlet_SemFull_DoesNotChargeCredits covers the second
// failure path the old ordering exposed: the global gauntletSem is
// saturated, so the handler returns 429, but the credit Spend already
// ran. Post-fix, sem acquisition happens before Spend.
func TestStartGauntlet_SemFull_DoesNotChargeCredits(t *testing.T) {
	f := newGauntletRefundFixture(t)
	f.putUserInPaidTier(t, 10)
	f.addDeckToPool("alice", "korvold", "Korvold")

	baselineUsage := f.gauntletUsageRowsFor(t)
	baselineBalance := f.currentBalance(t)

	// Saturate the package-level gauntletSem (capacity 2). t.Cleanup
	// drains so we don't pollute other tests.
	for i := 0; i < cap(gauntletSem); i++ {
		gauntletSem <- struct{}{}
	}
	t.Cleanup(func() {
		// Drain anything we acquired; tolerate other tests having
		// added or removed slots concurrently by draining at most
		// cap with a non-blocking select.
		for i := 0; i < cap(gauntletSem); i++ {
			select {
			case <-gauntletSem:
			default:
				return
			}
		}
	})

	req := f.makeStartReq("alice", "korvold")
	rr := httptest.NewRecorder()

	f.sm.handleStartGauntlet(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status: want 429, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	if got := f.currentBalance(t); got != baselineBalance {
		t.Errorf("balance was debited despite 429: baseline=%d, after=%d (the bug)",
			baselineBalance, got)
	}
	if got := f.gauntletUsageRowsFor(t); got != baselineUsage {
		t.Errorf("gauntlet_usage gained a row despite 429: baseline=%d, after=%d",
			baselineUsage, got)
	}
}

// TestStartGauntlet_PaidHappyPath_ChargesAndLogs is the positive
// counterpart: when everything lines up (deck present, sem available,
// balance sufficient), the handler must still debit credits and log
// the run. Pins that the reorder didn't break the working path.
func TestStartGauntlet_PaidHappyPath_ChargesAndLogs(t *testing.T) {
	f := newGauntletRefundFixture(t)
	f.putUserInPaidTier(t, 10)
	f.addDeckToPool("alice", "korvold", "Korvold")

	baselineUsage := f.gauntletUsageRowsFor(t)
	baselineBalance := f.currentBalance(t)

	req := f.makeStartReq("alice", "korvold")
	rr := httptest.NewRecorder()

	f.sm.handleStartGauntlet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if got, _ := resp["status"].(string); got != "started" {
		t.Errorf("response.status: want %q, got %q", "started", got)
	}
	if got, _ := resp["credits_charged"].(float64); int64(got) != credits.CreditsPerGauntlet {
		t.Errorf("response.credits_charged: want %d, got %v",
			credits.CreditsPerGauntlet, resp["credits_charged"])
	}

	wantBalance := baselineBalance - credits.CreditsPerGauntlet
	if got := f.currentBalance(t); got != wantBalance {
		t.Errorf("balance not debited as expected: want %d, got %d", wantBalance, got)
	}
	if got := f.gauntletUsageRowsFor(t); got != baselineUsage+1 {
		t.Errorf("gauntlet_usage should have one new row: want %d, got %d",
			baselineUsage+1, got)
	}

	// Wait briefly for the launched goroutine to drain so it doesn't
	// outlive the test (sm.sqlDB is nil, so it touches no DB — but
	// still polite to let it finish before t.Cleanup tears down the
	// credits store).
	waitForGauntletStatus(t, f.sm, "alice/korvold", "complete", 2*time.Second)
}

func waitForGauntletStatus(t *testing.T, sm *Showmatch, deckKey, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Copy the status field under the lock — RunGauntlet writes
		// to *GauntletResult.Status concurrent with this read.
		// Reading after releasing the RLock races against the
		// engine goroutine (caught by `go test -race`).
		var gotStatus string
		sm.gauntletMu.RLock()
		if g := sm.gauntlets[deckKey]; g != nil {
			gotStatus = g.Status
		}
		sm.gauntletMu.RUnlock()
		if gotStatus == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Logf("gauntlet did not reach status=%q within %s — leaving anyway (goroutine is nil-safe)", want, timeout)
}
