package tournament

// Mini-grinder integration test for the MDFC permanent_types fix.
// Loads real decks containing the Final Fantasy reverse-MDFC land cycle
// (Midgar/Lindblum/Ishgard/Jidoor/Zanarkand), plays a batch of full games
// with the same setup the showmatch grinder uses, and runs hat.CheckGame
// after each game. Asserts the post-fix permanent_types violation count
// for FF MDFC cards is zero.

import (
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// FF MDFC reverse-land cycle (front face = land, back face = sorcery).
// These are the cards the user identified as still leaking 339
// permanent_types violations after the prior SwapToBackFace fixes.
var ffMDFCFrontNames = []string{
	"Midgar, City of Mako",
	"Lindblum, Home of Theater Ship",
	"Ishgard, Holy See",
	"Jidoor, Opera Capital",
	"Zanarkand, Forgotten Ruins",
}

// findDecksContainingFFMDFC scans data/decks/moxfield for deck files that
// reference at least one card from the FF MDFC reverse-land cycle, returns
// up to `n` paths. Used to seed the mini-grinder so the actual buggy code
// path (tryPlayLand → reverse-MDFC) gets exercised.
func findDecksContainingFFMDFC(tb testing.TB, n int) []string {
	tb.Helper()
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
		tb.Skip("no data/decks/moxfield directory")
		return nil
	}
	all, err := deckparser.ListDeckFiles(moxDir)
	if err != nil || len(all) == 0 {
		tb.Skipf("ListDeckFiles: %v", err)
		return nil
	}
	var hits []string
	for _, p := range all {
		body, err := readFileBytes(p)
		if err != nil {
			continue
		}
		text := string(body)
		for _, name := range ffMDFCFrontNames {
			if strings.Contains(text, name) {
				hits = append(hits, p)
				break
			}
		}
		if len(hits) >= n {
			break
		}
	}
	return hits
}

// TestMDFC_GrinderIntegration_NoFFMDFCPermanentTypeViolations is the
// real-data validation of the tryPlayLand reverse-MDFC fix. Plays a
// modest batch of games using decks that contain FF reverse-MDFC lands
// and asserts hat.CheckGame produces zero permanent_types violations
// where the offending card is one of the FF MDFC cycle.
//
// Pre-fix: the grinder reported 339 such violations across its sample.
// Post-fix: should be zero.
//
// Skipped if the AST corpus or the FF-deck pool is unavailable
// (gitignored data files).
func TestMDFC_GrinderIntegration_NoFFMDFCPermanentTypeViolations(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test, skipped in -short mode")
	}
	corpus, meta := loadCorpus(t)
	paths := findDecksContainingFFMDFC(t, 4)
	if len(paths) < 4 {
		t.Skipf("need 4 FF-MDFC decks, found %d", len(paths))
	}
	decks := make([]*deckparser.TournamentDeck, 0, 4)
	for _, p := range paths[:4] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Logf("skip parse error %s: %v", filepath.Base(p), err)
			continue
		}
		decks = append(decks, d)
	}
	if len(decks) < 4 {
		t.Skipf("only %d FF-MDFC decks parsed cleanly", len(decks))
	}

	// 80 games keeps the FF-MDFC hit rate ample (~30 games with the cycle
	// reaching a battlefield) while letting this integration test fit
	// inside the per-package 300s timeout when `go test ./...` saturates
	// cores with the gameengine/astload binaries in parallel.
	const numGames = 80
	const maxTurns = 50
	const nSeats = 4

	ffViolations := 0
	totalPermViolations := 0
	gamesWithFFCardOnBattlefield := 0
	ffNameSet := map[string]bool{}
	for _, n := range ffMDFCFrontNames {
		ffNameSet[n] = true
	}

	for game := 0; game < numGames; game++ {
		gs := setupGameStateForMDFCTest(decks, nSeats, int64(game)*101+1)
		playGameMDFCTest(gs, maxTurns)

		// Did any FF MDFC card actually land on a battlefield this game?
		// (If not, the test is vacuous for that game.)
		sawFF := false
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				name := p.Card.Name
				front := name
				if i := strings.Index(name, " // "); i >= 0 {
					front = name[:i]
				}
				if ffNameSet[name] || ffNameSet[front] {
					sawFF = true
					break
				}
			}
			if sawFF {
				break
			}
		}
		if sawFF {
			gamesWithFFCardOnBattlefield++
		}

		result := hat.CheckGame(gs)
		for _, v := range result.Violations {
			if v.Rule != "permanent_types" {
				continue
			}
			totalPermViolations++
			cardField, _ := v.Details["card"].(string)
			front := cardField
			if i := strings.Index(cardField, " // "); i >= 0 {
				front = cardField[:i]
			}
			if ffNameSet[cardField] || ffNameSet[front] {
				ffViolations++
				t.Errorf("game %d: FF MDFC permanent_types violation: card=%q type=%v",
					game, cardField, v.Details["type"])
			}
		}
	}

	t.Logf("grinder summary: %d games, %d games with an FF-MDFC card reaching the battlefield, "+
		"%d total permanent_types violations, %d FF-MDFC permanent_types violations",
		numGames, gamesWithFFCardOnBattlefield, totalPermViolations, ffViolations)

	if gamesWithFFCardOnBattlefield == 0 {
		// Hard-fail rather than skip: a vacuous integration test is a
		// silent false-pass. If this fires, increase numGames/maxTurns
		// or update the FF-MDFC deck pool until at least one card from
		// the cycle reaches the battlefield.
		t.Errorf("no FF MDFC card reached a battlefield in any game — test was vacuous, " +
			"increase numGames/maxTurns or pick decks more likely to play these lands")
	}
}

// setupGameStateForMDFCTest builds a fresh GameState seeded for the
// integration test. Mirrors the relevant subset of runOneGame's setup
// (deck cloning, library shuffle, commander placement, mulligan, hat
// attachment) without the outcome bookkeeping.
func setupGameStateForMDFCTest(decks []*deckparser.TournamentDeck, nSeats int, seed int64) *gameengine.GameState {
	rng := rand.New(rand.NewSource(seed))
	gs := gameengine.NewGameState(nSeats, rng, nil)
	gs.RetainEvents = false

	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		tpl := decks[i%len(decks)]
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
	return gs
}

// playGameMDFCTest runs the turn loop until natural end or maxTurns.
// Mirrors runOneGame's inner loop minus the elimination/outcome bookkeeping.
func playGameMDFCTest(gs *gameengine.GameState, maxTurns int) {
	for turn := 1; turn <= maxTurns; turn++ {
		gs.Turn = turn
		takeTurnImpl(gs, nil)
		gameengine.StateBasedActions(gs)
		if gs.CheckEnd() {
			return
		}
		gs.Active = nextLivingSeat(gs)
		if gs.Active < 0 {
			return
		}
	}
}

func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}
