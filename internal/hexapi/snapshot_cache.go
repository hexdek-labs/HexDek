package hexapi

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// errMalformedSnapshot wraps payload-parse failures so callers
// can errors.Is-discriminate them from sql errors. Keeps the
// "real DB error → 500 / malformed → graceful fallback" contract
// honest even when the cache layer sits between the handler and
// the DB.
var errMalformedSnapshot = errors.New("malformed snapshot payload")

// IsMalformedSnapshot reports whether err originated from a
// snapshot-parse failure inside loadCachedSnapshot.
func IsMalformedSnapshot(err error) bool {
	return errors.Is(err, errMalformedSnapshot)
}

// snapshotCache caches parsed heimdall.GameObservationSnapshot
// values keyed by showmatch_game.game_id. Versioned by the
// underlying row's created_at so a fresh InsertGameObservation
// upsert (which bumps created_at via unixepoch()) naturally
// invalidates the cache on the next request.
//
// The cache eliminates the JSON unmarshal (~110μs/op for a
// realistic 14KB payload) plus the payload column fetch
// (~22μs/op) on a hit, leaving just one cheap version probe
// (~5μs/op) + a deep-copy of the cached struct. Net win on the
// hit path is ~125μs per request — over half the baseline
// /api/games/{id}/summary cost.
//
// Size-capped LRU via container/list to bound memory: ~14KB per
// entry × cap = ceiling. Default cap of 1024 → ~14MB worst case,
// which is fine for the hexdek-server process budget.
type snapshotCache struct {
	mu    sync.Mutex
	cap   int
	items map[int64]*list.Element
	order *list.List
}

type snapshotCacheEntry struct {
	gameID  int64
	version int64                          // showmatch_game_observation.created_at
	snap    heimdall.GameObservationSnapshot
}

func newSnapshotCache(cap int) *snapshotCache {
	if cap <= 0 {
		cap = 1024
	}
	return &snapshotCache{
		cap:   cap,
		items: make(map[int64]*list.Element, cap),
		order: list.New(),
	}
}

// get returns the cached snapshot for gameID if the cached entry's
// version matches the provided version. Returns (snap, true) on
// hit, (zero, false) on miss or version mismatch.
//
// On hit the entry moves to the front of the LRU list. The
// returned snapshot is the same struct stored in the cache — the
// underlying CommanderZoneVisits / RegretCards / MVPCards slices
// are shared by reference. Callers MUST treat the returned value
// as read-only (see snapshotPersistAdapter for the only write
// path, which marshals a fresh struct rather than mutating).
func (c *snapshotCache) get(gameID, version int64) (heimdall.GameObservationSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[gameID]
	if !ok {
		return heimdall.GameObservationSnapshot{}, false
	}
	entry := el.Value.(*snapshotCacheEntry)
	if entry.version != version {
		// Stale — drop it. The handler will re-fetch the payload
		// and re-prime the cache with the current version below.
		c.order.Remove(el)
		delete(c.items, gameID)
		return heimdall.GameObservationSnapshot{}, false
	}
	c.order.MoveToFront(el)
	return entry.snap, true
}

// put inserts (gameID, version, snap), evicting the LRU tail
// when the cache is at capacity. Idempotent — re-putting a
// gameID overwrites the prior entry and bumps it to the front.
func (c *snapshotCache) put(gameID, version int64, snap heimdall.GameObservationSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[gameID]; ok {
		entry := el.Value.(*snapshotCacheEntry)
		entry.version = version
		entry.snap = snap
		c.order.MoveToFront(el)
		return
	}
	entry := &snapshotCacheEntry{gameID: gameID, version: version, snap: snap}
	el := c.order.PushFront(entry)
	c.items[gameID] = el
	if c.order.Len() > c.cap {
		tail := c.order.Back()
		if tail != nil {
			te := tail.Value.(*snapshotCacheEntry)
			delete(c.items, te.gameID)
			c.order.Remove(tail)
		}
	}
}

// invalidate removes the entry for gameID if present. Called by
// write-path code (e.g., the live observation persistence
// adapter) when it knows a row has just been overwritten. Not
// strictly required — get() also self-heals on version mismatch
// — but skipping a stale entry is a tiny win on the next read.
func (c *snapshotCache) invalidate(gameID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[gameID]; ok {
		c.order.Remove(el)
		delete(c.items, gameID)
	}
}

// len returns the current entry count. Useful for tests + an
// eventual /debug/cache surface.
func (c *snapshotCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// loadCachedSnapshot is the shared snapshot-load helper used by
// both /api/games/{id}/summary and /api/games/{id}/summary.pdf.
// Returns the parsed snapshot, the row's version, and any error
// from the DB / unmarshal pipeline. sql.ErrNoRows is propagated
// verbatim for the missing-snapshot case so callers can branch
// to the db_only fallback.
//
// Hit path: 1 cheap version SELECT + cache lookup + struct
// return.
// Miss path: version SELECT + payload SELECT + JSON unmarshal +
// cache insert.
func (h *Handler) loadCachedSnapshot(ctx context.Context, gameID int64) (heimdall.GameObservationSnapshot, int64, error) {
	version, err := db.LoadGameObservationVersion(ctx, h.db, gameID)
	if err != nil {
		return heimdall.GameObservationSnapshot{}, 0, err
	}
	if h.snapshotCache != nil {
		if snap, ok := h.snapshotCache.get(gameID, version); ok {
			return snap, version, nil
		}
	}
	payload, err := db.LoadGameObservation(ctx, h.db, gameID)
	if err != nil {
		// Race: version row existed during the first SELECT but
		// got DELETE'd before the payload SELECT. Surface as
		// sql.ErrNoRows so the caller routes to db_only.
		return heimdall.GameObservationSnapshot{}, 0, err
	}
	snap, err := heimdall.UnmarshalSnapshot(payload)
	if err != nil {
		return heimdall.GameObservationSnapshot{}, version,
			fmt.Errorf("%w: %v", errMalformedSnapshot, err)
	}
	if h.snapshotCache != nil {
		h.snapshotCache.put(gameID, version, snap)
	}
	return snap, version, nil
}

// ensureSnapshotCache lazily wires the default cache. Handlers
// call this before reaching for h.snapshotCache so the field is
// optional at construction time but always populated by the
// first read request. Concurrent-safe via the read-then-write-
// under-mutex check.
func (h *Handler) ensureSnapshotCache() {
	h.snapshotCacheOnce.Do(func() {
		h.snapshotCache = newSnapshotCache(1024)
	})
}

