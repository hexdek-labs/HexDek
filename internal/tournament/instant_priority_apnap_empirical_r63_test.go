package tournament

// r63 — EMPIRICAL A/B for the non-active instant-speed priority windows.
//
// The orchestrator asked for ground-truth from real games, not just unit
// proofs. This test plays the SAME real 4-player Commander games twice — once
// with the non-active windows DISABLED (pre-r63 behavior) and once ENABLED
// (the fix) — and measures, via process-global counters, how often a NON-ACTIVE
// player actually takes an instant-speed action on an opponent's turn.
//
// Expected ground truth:
//   - baseline (disabled): non-active instant acts == 0  (the gap is total —
//     no code path let a non-active player act on an empty stack)
//   - fixed (enabled):     non-active instant acts  > 0  (real interaction)
//   - the fix must not destabilize games (crash rate stays low; avg turns sane)

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// findDecksAcrossDirs gathers up to n .txt deck files by walking data/decks
// recursively, so we can reach 4 distinct decks even though no single
// subdirectory holds that many.
func findDecksAcrossDirs(tb testing.TB, n int) []string {
	tb.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 6; i++ {
		root := filepath.Join(dir, "data", "decks")
		var out []string
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || len(out) >= n {
				return nil
			}
			if strings.HasSuffix(p, ".txt") {
				out = append(out, p)
			}
			return nil
		})
		if len(out) >= n {
			return out[:n]
		}
		dir = filepath.Dir(dir)
	}
	tb.Skip("could not find 4 deck files under data/decks")
	return nil
}

func TestAPNAP_Empirical_NonActiveInteractionDelta(t *testing.T) {
	if testing.Short() {
		t.Skip("empirical multi-game A/B — skipped in -short")
	}
	corpus, meta := loadCorpus(t)
	paths := findDecksAcrossDirs(t, 4)
	if len(paths) < 4 {
		t.Skipf("need 4 decks, got %d", len(paths))
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:4] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}

	// Production-representative hat mix (Yggdrasil = the deployed AI).
	hatFns := []HatFactory{
		func() gameengine.Hat { return hat.NewYggdrasilHat(nil, 0) },
		func() gameengine.Hat { return hat.NewPokerHat() },
		func() gameengine.Hat { return hat.NewYggdrasilHat(nil, 0) },
		func() gameengine.Hat { return &hat.GreedyHat{} },
	}
	const nGames = 120

	mkCfg := func() TournamentConfig {
		return TournamentConfig{
			Decks:         decks,
			NSeats:        4,
			NGames:        nGames,
			Seed:          77,
			HatFactories:  hatFns,
			Workers:       4,
			CommanderMode: true,
		}
	}

	run := func(disabled bool) (acts, offered int64, crashes, games int, avgTurns float64) {
		nonActiveInstantWindowsDisabled = disabled
		defer func() { nonActiveInstantWindowsDisabled = false }()
		atomic.StoreInt64(&metricNonActiveInstantActs, 0)
		atomic.StoreInt64(&metricNonActiveWindowsOffered, 0)
		r, err := Run(mkCfg())
		if err != nil {
			t.Fatalf("Run (disabled=%v): %v", disabled, err)
		}
		return atomic.LoadInt64(&metricNonActiveInstantActs),
			atomic.LoadInt64(&metricNonActiveWindowsOffered),
			r.Crashes, r.Games, r.AvgTurns
	}

	baseActs, baseOffered, baseCrash, baseGames, baseTurns := run(true)
	fixActs, fixOffered, fixCrash, fixGames, fixTurns := run(false)

	t.Logf("BASELINE (non-active windows OFF): non_active_acts=%d offered=%d games=%d crashes=%d avg_turns=%.1f",
		baseActs, baseOffered, baseGames, baseCrash, baseTurns)
	t.Logf("FIXED    (non-active windows ON):  non_active_acts=%d offered=%d games=%d crashes=%d avg_turns=%.1f",
		fixActs, fixOffered, fixGames, fixCrash, fixTurns)
	if fixGames > 0 {
		t.Logf("FIXED rate: %.3f non-active instant acts/game, %.3f windows-with-a-play offered/game",
			float64(fixActs)/float64(fixGames), float64(fixOffered)/float64(fixGames))
	}

	// Ground truth #1: the gap was TOTAL before the fix.
	if baseActs != 0 {
		t.Errorf("baseline should have ZERO non-active instant acts (no code path enabled them); got %d", baseActs)
	}
	if baseOffered != 0 {
		t.Errorf("baseline should never even offer a non-active window; got %d", baseOffered)
	}
	// Ground truth #2: the fix produces REAL interaction in real games.
	if fixActs == 0 {
		t.Errorf("fix should produce >0 non-active instant acts across %d real games; got 0 (window never fired)", nGames)
	}
	// Ground truth #3: the fix doesn't destabilize games.
	if fixGames > 0 {
		crashRate := float64(fixCrash) / float64(fixGames+fixCrash)
		baseRate := float64(baseCrash) / float64(baseGames+baseCrash)
		if crashRate > 0.05 {
			t.Errorf("fix crash rate %.1f%% exceeds 5%% (crashes=%d)", crashRate*100, fixCrash)
		}
		// The fix shouldn't materially worsen stability vs baseline.
		if crashRate > baseRate+0.03 {
			t.Errorf("fix crash rate %.1f%% materially worse than baseline %.1f%%", crashRate*100, baseRate*100)
		}
	}
}
