package hexapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// mustTournamentDeck builds a minimal pool entry. Only Path matters
// here — findDeckInPool keys on deckKeyFromPath(d.Path).
func mustTournamentDeck(t *testing.T, path string) *deckparser.TournamentDeck {
	t.Helper()
	return &deckparser.TournamentDeck{Path: path}
}

// These tests pin the behaviour that was missing for ~50 hours in
// 2026-09: a deck imported after server boot was invisible to the
// gauntlet, permanently, and the only signal was a user complaining.

func TestFindPlayableDeckFile(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string) string {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	txt := mk("alice/deck_a.txt")
	jsn := mk("bob/deck_b.json")

	if got := findPlayableDeckFile(dir, "alice", "deck_a"); got != txt {
		t.Errorf(".txt deck: got %q want %q", got, txt)
	}
	if got := findPlayableDeckFile(dir, "bob", "deck_b"); got != jsn {
		t.Errorf(".json deck: got %q want %q — .json decks are real decks; "+
			"the --all-decks walker's .txt-only glob was a separate bug today", got, jsn)
	}
	if got := findPlayableDeckFile(dir, "alice", "nope"); got != "" {
		t.Errorf("missing deck: got %q want \"\"", got)
	}
	if got := findPlayableDeckFile(dir, "nobody", "deck_a"); got != "" {
		t.Errorf("wrong owner must not resolve: got %q", got)
	}
}

// TestFindPlayableDeckFile_DirectoryIsNotADeck guards a subtle failure:
// os.Stat succeeds on a directory, so without the IsDir check a folder
// named like a deck would be handed to the parser.
func TestFindPlayableDeckFile_DirectoryIsNotADeck(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "alice", "trap.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findPlayableDeckFile(dir, "alice", "trap"); got != "" {
		t.Errorf("a directory was returned as a deck file: %q", got)
	}
}

// TestParseDeckFromDisk_RefusesWithoutCorpus pins the degraded case.
// During startup corpus/meta are nil; the read-through must decline
// rather than panic, and must not report the deck as missing (the
// reason text is what lands in the log a human reads at 2am).
func TestParseDeckFromDisk_RefusesWithoutCorpus(t *testing.T) {
	sm := &Showmatch{decksDir: t.TempDir()}
	d, reason := sm.parseDeckFromDisk("alice", "deck")
	if d != nil {
		t.Fatal("expected no deck with a nil corpus")
	}
	if reason != "card corpus not loaded yet" {
		t.Errorf("reason = %q, want the corpus-not-loaded reason; a misleading "+
			"reason here is what sent a user to re-import three times", reason)
	}
}

func TestParseDeckFromDisk_RefusesWithoutDecksDir(t *testing.T) {
	sm := &Showmatch{}
	if _, reason := sm.parseDeckFromDisk("alice", "deck"); reason != "card corpus not loaded yet" {
		// corpus is checked first; assert the ordering is stable so the
		// operator sees the most actionable cause.
		t.Errorf("reason = %q, want corpus check to fire first", reason)
	}
}

// TestLookupDeckForPlay_HitsPoolWithoutTouchingDisk is the fast path:
// a deck already in the pool must not incur a parse, and must not be
// counted as a miss.
func TestLookupDeckForPlay_HitsPoolWithoutTouchingDisk(t *testing.T) {
	missesBefore, _, rejectedBefore := PoolMissCounts()

	sm := &Showmatch{}
	// decksDir deliberately empty: if the fast path is broken this
	// falls through to disk and fails, rather than silently passing.
	sm.deckPool = append(sm.deckPool, mustTournamentDeck(t, "alice/known.txt"))

	d, reason := sm.lookupDeckForPlay("alice", "known")
	if d == nil {
		t.Fatalf("pool hit returned nil (reason %q) — the in-memory fast path is broken", reason)
	}

	misses, _, rejected := PoolMissCounts()
	if misses != missesBefore {
		t.Errorf("a pool HIT incremented the miss counter (%d → %d)", missesBefore, misses)
	}
	if rejected != rejectedBefore {
		t.Errorf("a pool HIT incremented the rejected counter (%d → %d)", rejectedBefore, rejected)
	}
}

// TestLookupDeckForPlay_MissWithNoFileIsRejectedAndCounted is the
// honest-failure path. A deck that genuinely doesn't exist must be
// refused AND must show up in the counter — the whole point of the
// counter is that a refusal can no longer be invisible.
func TestLookupDeckForPlay_MissWithNoFileIsRejectedAndCounted(t *testing.T) {
	_, _, rejectedBefore := PoolMissCounts()

	sm := &Showmatch{decksDir: t.TempDir()}
	d, reason := sm.lookupDeckForPlay("ghost", "nothing_here")
	if d != nil {
		t.Fatal("a nonexistent deck resolved to something")
	}
	if reason == "" {
		t.Error("rejection carried no reason — the log line would say nothing")
	}

	_, _, rejected := PoolMissCounts()
	if rejected != rejectedBefore+1 {
		t.Errorf("rejected counter %d → %d, want +1. A refused user who "+
			"increments no counter is exactly the 50-hour bug: the system "+
			"knows it said no and has no way to tell anyone",
			rejectedBefore, rejected)
	}
}

// TestPoolMissCounts_Monotonic pins that the accessor reports the live
// values rather than a snapshot taken once.
func TestPoolMissCounts_Monotonic(t *testing.T) {
	_, _, r0 := PoolMissCounts()
	sm := &Showmatch{decksDir: t.TempDir()}
	sm.lookupDeckForPlay("ghost", "a")
	sm.lookupDeckForPlay("ghost", "b")
	_, _, r1 := PoolMissCounts()
	if r1 < r0+2 {
		t.Errorf("counters did not advance across two rejections: %d → %d", r0, r1)
	}
}
