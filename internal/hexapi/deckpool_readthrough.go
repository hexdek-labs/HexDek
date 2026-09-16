package hexapi

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// deckpool_readthrough — make the engine deck pool self-healing, and
// make its refusals audible.
//
// THE BUG THIS REPLACES
//
// sm.deckPool was built exactly once, at startup, by buildDeckPool.
// findDeckInPool walked only that slice. So a deck imported after boot
// was invisible to the gauntlet forever, or until a human remembered
// to POST /api/showmatch/reload-pool.
//
// On 2026-09-15 that state had been live for ~50 hours. Every deck
// imported by anyone in that window was un-gauntletable. The user who
// found it had imported the same deck THREE times, because the error
// he was shown — "deck not in engine pool — re-import or check the
// deck id" — told him to do the one thing that cannot possibly work:
// each re-import produced another deck that also landed outside the
// pool. A message that blames the user converts a server-side bug into
// silence, which is why this went two days without a report.
//
// Three changes, because the bug had three independent causes:
//
//  1. READ-THROUGH (this file). On a pool miss, look for the deck file
//     on disk and parse it on demand, then add it to the live pool.
//     This removes the failure class rather than adding another thing
//     to remember: there is no staleness window because there is no
//     window. A scheduled or manual reload is a process control
//     guarding a failure that raises no error, and process controls
//     guarding silent failures get forgotten.
//
//  2. HONEST ERROR TEXT (showmatch.go). Say it's ours, not theirs.
//
//  3. A COUNTER (this file). Every rejection increments a counter that
//     /api/health reports. The system knew it was refusing users and
//     had no way to say so; one integer fixes that. This is the same
//     shape as the Freya cache serving stale reports and the
//     --all-decks walker skipping directories — something that knows
//     and tells nobody.
//
// The read-through deliberately applies the SAME four gates as
// buildDeckPool (parse / commander present / >= 80 cards / commander
// not banned). A deck that the startup path would have rejected must
// not sneak into the pool by a different door, or the two paths would
// disagree about what a legal deck is.

// poolMissStats counts deck-pool lookups that missed, split by what
// happened next. Exported through /api/health so a non-zero rejection
// count is visible without anyone running a query.
//
// These are process-lifetime counters, not rates. They exist to answer
// "is this happening at all", which is precisely the question nobody
// could answer for the 50 hours the pool bug was live.
var poolMissStats struct {
	// Misses is every lookup that did not find the deck in memory.
	Misses atomic.Int64
	// Recovered is misses the read-through resolved from disk. A high
	// Recovered with zero Rejected means the mechanism is working.
	Recovered atomic.Int64
	// Rejected is misses that could not be recovered — no file, parse
	// failure, or one of the pool gates. THIS is the number that means
	// a user was refused. Non-zero here deserves a look.
	Rejected atomic.Int64
}

// PoolMissCounts returns the current miss/recover/reject totals.
func PoolMissCounts() (misses, recovered, rejected int64) {
	return poolMissStats.Misses.Load(),
		poolMissStats.Recovered.Load(),
		poolMissStats.Rejected.Load()
}

// lookupDeckForPlay resolves a deck for gauntlet/tournament use.
//
// Fast path: the in-memory pool. Slow path: parse the deck file from
// disk, apply the pool gates, and splice it into the live pool so the
// next lookup is fast again.
//
// Returns a nil deck and a reason string when the deck genuinely
// cannot be played. The reason is for logs; callers give the user the
// message in showmatch.go, which deliberately does not repeat it —
// "commander missing" is useful to us and confusing to someone who can
// see their commander on the deck page.
func (sm *Showmatch) lookupDeckForPlay(owner, id string) (*deckparser.TournamentDeck, string) {
	if d := sm.findDeckInPool(owner, id); d != nil {
		return d, ""
	}
	poolMissStats.Misses.Add(1)

	d, reason := sm.parseDeckFromDisk(owner, id)
	if d == nil {
		poolMissStats.Rejected.Add(1)
		log.Printf("showmatch: pool read-through REJECTED %s/%s: %s", owner, id, reason)
		return nil, reason
	}

	// Splice into the live pool. Re-check under the write lock: a
	// concurrent reload (or another read-through for the same deck)
	// may have added it between our miss and here, and a duplicate
	// entry would give this deck two seats in the random-opponent
	// draw.
	target := owner + "/" + id
	sm.mu.Lock()
	for _, existing := range sm.deckPool {
		if deckKeyFromPath(existing.Path) == target {
			sm.mu.Unlock()
			poolMissStats.Recovered.Add(1)
			return existing, ""
		}
	}
	sm.deckPool = append(sm.deckPool, d)
	poolSize := len(sm.deckPool)
	sm.mu.Unlock()

	poolMissStats.Recovered.Add(1)
	log.Printf("showmatch: pool read-through recovered %s (pool now %d)", target, poolSize)
	return d, ""
}

// parseDeckFromDisk finds and parses a deck file for owner/id, applying
// the same gates buildDeckPool applies. Returns (nil, reason) when the
// deck can't be used.
func (sm *Showmatch) parseDeckFromDisk(owner, id string) (*deckparser.TournamentDeck, string) {
	sm.mu.RLock()
	corpus, meta, decksDir := sm.corpus, sm.meta, sm.decksDir
	sm.mu.RUnlock()

	if corpus == nil || meta == nil {
		return nil, "card corpus not loaded yet"
	}
	if decksDir == "" {
		return nil, "decks directory not configured"
	}

	path := findPlayableDeckFile(decksDir, owner, id)
	if path == "" {
		return nil, "no deck file on disk"
	}

	d, err := deckparser.ParseDeckFile(path, corpus, meta)
	if err != nil {
		return nil, fmt.Sprintf("parse failed: %v", err)
	}
	// Same four gates as buildDeckPool, in the same order. If these
	// ever diverge, a deck could be playable through one door and not
	// the other, which is a worse bug than the one being fixed.
	if len(d.CommanderCards) == 0 {
		return nil, "no commander"
	}
	if total := len(d.Library) + len(d.CommanderCards); total < 80 {
		return nil, fmt.Sprintf("only %d cards (need 80)", total)
	}
	if commanderBanned(d.CommanderName) {
		return nil, "commander is banned"
	}
	return d, ""
}

// findPlayableDeckFile locates owner/id's deck file, trying the same
// extensions findDeckFiles accepts. Returns "" when nothing is there.
//
// Note this does NOT honour findDeckFiles' directory skip-list
// (freya/benched/test/moxfield_300/.versions): those directories are
// excluded from BULK pool loading so scratch decks don't join the
// random-opponent draw, which is a different question from "may this
// specific deck the user explicitly named be played". A user asking
// for a gauntlet on a deck they own has named it explicitly.
func findPlayableDeckFile(decksDir, owner, id string) string {
	for _, ext := range []string{".txt", ".json"} {
		p := filepath.Join(decksDir, owner, id+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
