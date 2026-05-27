package oracle

import (
	"container/list"
	"strings"
	"sync"
	"time"
)

// ResponseCache is an in-memory LRU+TTL cache for successful
// /api/oracle/card/{name} responses. Sits in FRONT of Lookup so hot
// cards skip the SQLite roundtrip entirely — SQLite hits are cheap
// (~tens of µs) but not free, and the public endpoint sees the same
// few hundred staples (basic lands, fetches, Sol Ring, etc.) on every
// deck-builder page load. The default 1000-entry / 1h cache covers
// that working set with sub-µs hits and bounded memory (~1 MB at typical
// Scryfall card-JSON sizes).
//
// Only successful Lookup results are cached. NotFound and error
// responses are NOT cached:
//   - NotFound: an attacker could enumerate junk names to pin them in
//     the cache, blocking legitimate keys (cache poisoning).
//   - Errors: usually transient (DB timeout, Scryfall flake). Caching
//     them would extend the outage past root-cause resolution.
//
// Keys are canonicalized (lowercase + trim) to match how Lookup keys
// the SQLite cache — "Sol Ring" and "sol ring" share one entry.
type ResponseCache struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	// items maps key → *list.Element wrapping an rcEntry.
	items map[string]*list.Element
	// order is a doubly-linked list with front = most recently used,
	// back = least recently used. O(1) move-to-front + back-eviction.
	order *list.List
	now   func() time.Time // injectable for tests
}

type rcEntry struct {
	key     string
	card    *Card
	expires time.Time
}

// NewResponseCache returns an LRU+TTL cache with the given capacity
// and entry TTL. Production default: capacity=1000, ttl=1 hour.
// capacity ≤ 0 is treated as 1 (degenerate cache always evicts on
// insert), and ttl ≤ 0 makes every entry expire instantly — both are
// no-op-equivalent defenses against misconfiguration.
func NewResponseCache(capacity int, ttl time.Duration) *ResponseCache {
	if capacity < 1 {
		capacity = 1
	}
	return &ResponseCache{
		capacity: capacity,
		ttl:      ttl,
		items:    map[string]*list.Element{},
		order:    list.New(),
		now:      time.Now,
	}
}

// canonKey lowercases + trims so casing/whitespace variants share an
// entry. Mirrors Lookup's SQLite-key normalization at scryfall.go.
func canonKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Get returns the cached card for name (canonicalized) and true on
// hit. A stale entry (expires ≤ now) is treated as a miss AND removed
// in the same call — lazy expiration keeps the hot path lock-free of
// background sweepers.
func (rc *ResponseCache) Get(name string) (*Card, bool) {
	if rc == nil {
		return nil, false
	}
	key := canonKey(name)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	el, ok := rc.items[key]
	if !ok {
		return nil, false
	}
	ent := el.Value.(*rcEntry)
	if !rc.now().Before(ent.expires) {
		rc.order.Remove(el)
		delete(rc.items, key)
		return nil, false
	}
	rc.order.MoveToFront(el)
	return ent.card, true
}

// Put inserts (or refreshes) name → card. If the cache is at capacity
// the LRU entry is evicted. Refreshing an existing key extends its
// TTL — matches the "any access resets the timer" intuition.
//
// Put silently no-ops on nil card or nil cache to keep callers free of
// per-call guards.
func (rc *ResponseCache) Put(name string, card *Card) {
	if rc == nil || card == nil {
		return
	}
	key := canonKey(name)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	expires := rc.now().Add(rc.ttl)
	if el, ok := rc.items[key]; ok {
		ent := el.Value.(*rcEntry)
		ent.card = card
		ent.expires = expires
		rc.order.MoveToFront(el)
		return
	}
	ent := &rcEntry{key: key, card: card, expires: expires}
	el := rc.order.PushFront(ent)
	rc.items[key] = el
	if rc.order.Len() > rc.capacity {
		oldest := rc.order.Back()
		if oldest != nil {
			rc.order.Remove(oldest)
			delete(rc.items, oldest.Value.(*rcEntry).key)
		}
	}
}

// Len reports the current entry count (including not-yet-swept stale
// entries). Mostly for tests + future observability.
func (rc *ResponseCache) Len() int {
	if rc == nil {
		return 0
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.order.Len()
}
