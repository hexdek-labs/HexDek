package tournament

import (
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// TestFeynman_ZoneAccounting_50Games runs 50 full games with real decks and
// asserts no seat exceeds deck_size+20 total cards. This catches the Feynman
// #3 hand-bloat bug: reanimate/cheat-into-play effects that duplicate cards
// via the MoveCard+enterBattlefieldWithETB double-call pattern.
func TestFeynman_ZoneAccounting_50Games(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test, skipped in -short mode")
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

	const numGames = 50
	const maxTurns = 50
	const nSeats = 4

	zoneViolations := 0

	for game := 0; game < numGames; game++ {
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
			commanderDecks[i] = &gameengine.CommanderDeck{
				CommanderCards: cmdrs,
				Library:        lib,
			}
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

		result := hat.CheckGame(gs)
		for _, v := range result.Violations {
			if v.Rule == "zone_accounting" {
				zoneViolations++
				t.Errorf("game %d: %s", game, v)
			}
		}
	}

	t.Logf("zone accounting stress: %d games, %d violations", numGames, zoneViolations)
}
