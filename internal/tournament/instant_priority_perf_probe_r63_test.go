package tournament

// r63 — perf probe: measure the per-game wall-time cost of the non-active
// instant-speed windows on the SAME heavy moxfield decks + GreedyHat setup as
// TestFeynman_ZoneAccounting_50Games (the inherently slow stress test), in both
// A/B modes. This isolates whether the windows materially slow grinder-style
// games on mana-rich real decks, separate from the pre-existing slowness of
// that deck type.

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

func TestAPNAP_PerfProbe_MoxfieldGreedy(t *testing.T) {
	if testing.Short() {
		t.Skip("perf probe — skipped in -short")
	}
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	var moxDir string
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "data", "decks", "moxfield")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			moxDir = candidate
			break
		}
		dir = filepath.Dir(dir)
	}
	if moxDir == "" {
		t.Skip("moxfield deck directory not found")
	}
	corpus, meta := loadCorpus(t)
	entries, err := os.ReadDir(moxDir)
	if err != nil {
		t.Skipf("cannot read moxfield dir: %v", err)
	}
	var decks []*deckparser.TournamentDeck
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".txt" {
			continue
		}
		d, err := deckparser.ParseDeckFile(filepath.Join(moxDir, e.Name()), corpus, meta)
		if err != nil {
			continue
		}
		decks = append(decks, d)
		if len(decks) >= 8 {
			break
		}
	}
	if len(decks) < 4 {
		t.Skipf("need at least 4 decks, found %d", len(decks))
	}

	const nSeats = 4
	const maxTurns = 20
	const probeGames = 3

	playOne := func(game int) int {
		rng := rand.New(rand.NewSource(int64(game)*97 + 7))
		gs := gameengine.NewGameState(nSeats, rng, nil)
		gs.EventPolicy = gameengine.EventLogNone
		commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
		for i := 0; i < nSeats; i++ {
			tpl := decks[(game+i)%len(decks)]
			lib := deckparser.CloneLibrary(tpl.Library)
			cmdrs := deckparser.CloneCards(tpl.CommanderCards)
			for _, c := range cmdrs {
				c.Owner = i
			}
			for _, c := range lib {
				c.Owner = i
			}
			rng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
			commanderDecks[i] = &gameengine.CommanderDeck{CommanderCards: cmdrs, Library: lib}
		}
		gameengine.SetupCommanderGame(gs, commanderDecks)
		for i := 0; i < nSeats; i++ {
			gs.Seats[i].Hat = &hat.GreedyHat{}
		}
		for i := 0; i < nSeats; i++ {
			RunLondonMulligan(gs, i)
		}
		gs.Active = rng.Intn(nSeats)
		gs.Turn = 1
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		playGameMDFCTest(gs, maxTurns)
		return gs.Turn
	}

	timeMode := func(disabled bool) (time.Duration, int) {
		nonActiveInstantWindowsDisabled = disabled
		defer func() { nonActiveInstantWindowsDisabled = false }()
		start := time.Now()
		turns := 0
		for g := 0; g < probeGames; g++ {
			turns += playOne(g)
		}
		return time.Since(start), turns
	}

	baseDur, baseTurns := timeMode(true)
	fmt.Printf("PROBE BASELINE (windows OFF): %v for %d games (%.2fs/game), total_turns=%d\n",
		baseDur, probeGames, baseDur.Seconds()/probeGames, baseTurns)
	fixDur, fixTurns := timeMode(false)
	fmt.Printf("PROBE FIXED    (windows ON):  %v for %d games (%.2fs/game), total_turns=%d\n",
		fixDur, probeGames, fixDur.Seconds()/probeGames, fixTurns)
	if baseDur > 0 {
		ratio := fixDur.Seconds() / baseDur.Seconds()
		fmt.Printf("PROBE slowdown ratio (fixed/baseline): %.2fx\n", ratio)
		// Guard against a future perf regression in the non-active windows.
		// Measured ratio is ~1.0x (the windows are gated cheaply); allow
		// generous headroom for CI noise on small samples before failing.
		if baseDur.Seconds() > 0.2 && ratio > 2.5 {
			t.Errorf("non-active instant windows slowed heavy moxfield games %.2fx (>2.5x) — perf regression", ratio)
		}
	}
}
