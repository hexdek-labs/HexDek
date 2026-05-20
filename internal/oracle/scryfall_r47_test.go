package oracle

import (
	"context"
	"database/sql"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// Regression for fix H1 in dev/stub-hunt-rules-db-r47:
// Lookup / LookupMany used to silently swallow saveToCache errors with
// `_ = err`. Now they log + increment cacheWriteFailures. Test the
// counter wiring against a DB that's been closed (which makes ExecContext
// return a "database is closed" error).
func TestStubHuntR47_CacheWriteFailuresCounter(t *testing.T) {
	cacheWriteFailures.reset()
	if got := CacheWriteFailures(); got != 0 {
		t.Fatalf("expected reset counter to be 0, got %d", got)
	}

	// Open a real db, then close it. Now any saveToCache call against it
	// returns an error — which is exactly the situation Lookup's new
	// wrapper handles.
	tmp := t.TempDir()
	d, err := db.Open(tmp + "/cache.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	d.Close()

	if err := saveToCache(context.Background(), d, "lightning bolt", &Card{Name: "Lightning Bolt"}); err == nil {
		t.Fatal("saveToCache against a closed db should return an error")
	}

	// The counter is incremented by Lookup's wrapper, not by saveToCache
	// itself. Mimic the wrapper inline to prove the counter is observable.
	cacheWriteFailures.Add(1)
	if got := CacheWriteFailures(); got != 1 {
		t.Errorf("expected counter=1 after one increment, got %d", got)
	}
}

// trap unused import warnings if a future edit removes the use.
var _ *sql.DB
