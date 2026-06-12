package hat_test

// dev/hat-self-play-cohesion-r60 — after a stack of hat improvements
// (eval, ChooseAttacks, ChooseDiscard, AssignBlockers, 3rd Eye) the
// individual decision paths each have focused unit tests, but nothing
// asserts they cohere at game level. This is the smallest test that
// drives the full turn loop through four YggdrasilHats and pins the
// most embarrassing failure modes a regression could introduce:
//
//   - infinite loop / non-termination
//   - all-pass turns (a seat is alive with cards in hand but never
//     plays anything)
//   - degenerate game length (every game ending on turn 1)
//   - panic anywhere in the decision stack
//
// It lives in package hat_test (external test) to break the hat ↔
// tournament cycle — tournament imports hat, so a test inside package
// hat can't import tournament directly. hat_test can import both.
//
// The deck is synthetic — 1 vanilla 3/3 legendary commander, 36 basic
// Forests, 63 vanilla creatures of varied CMC. That's enough to keep
// the engine busy across all the hat hot paths (mulligan, land drops,
// cast loop, combat, blocks, end step) without depending on the
// gitignored AST corpus or any per-card handlers.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

// synthCard builds a minimal *gameengine.Card with the fields the engine
// reads off Card directly (Name, Types, BasePower, BaseToughness, AST).
// CMC encoded as a "cost:N" type tag matches the pattern newEnginePerm
// in tournament/engine_test.go uses.
func synthCard(name string, types []string, power, toughness, cmc int) *gameengine.Card {
	t := append([]string(nil), types...)
	if cmc > 0 {
		t = append(t, costTag(cmc))
	}
	return &gameengine.Card{
		Name:          name,
		Types:         t,
		BasePower:     power,
		BaseToughness: toughness,
		CMC:           cmc,
		AST:           &gameast.CardAST{Name: name},
	}
}

func costTag(n int) string {
	// Match tournament/engine_test.go's "cost:N" convention.
	return "cost:" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := make([]byte, 0, 4)
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}

// buildSyntheticDeck returns a CommanderDeck shaped like a real
// commander 99: one legendary commander + a 99-card library of basic
// lands and vanilla creatures. nameSeed lets each seat have unique
// card identity so cardsSeen / opponent profiling has something to
// chew on (otherwise every seat's "Forest" collides).
func buildSyntheticDeck(nameSeed string) *gameengine.CommanderDeck {
	commander := synthCard(
		"Cmdr "+nameSeed,
		[]string{"creature", "legendary"},
		3, 3, 3,
	)
	library := make([]*gameengine.Card, 0, 99)
	// 36 Forests — enough mana density for a 4-player game to actually
	// progress past the early-mulligan window.
	for i := 0; i < 36; i++ {
		library = append(library, synthCard(
			"Forest "+nameSeed,
			[]string{"land", "forest"},
			0, 0, 0,
		))
	}
	// 63 vanilla creatures spread across CMC 1-5 so each seat has
	// playable spells every turn from turn 1 forward.
	for cmc := 1; cmc <= 5; cmc++ {
		count := []int{14, 14, 14, 12, 9}[cmc-1] // sums to 63
		for i := 0; i < count; i++ {
			library = append(library, synthCard(
				"Beast "+nameSeed+"-"+itoa(cmc)+"-"+itoa(i),
				[]string{"creature"},
				cmc, cmc, cmc,
			))
		}
	}
	return &gameengine.CommanderDeck{
		CommanderCards: []*gameengine.Card{commander},
		Library:        library,
	}
}

// runSelfPlay sets up a 4-seat synthetic-deck game with four
// YggdrasilHats and runs the turn loop until natural end or turnCap.
// Returns the final GameState so the caller can assert on it.
func runSelfPlay(t *testing.T, seed int64, turnCap int) *gameengine.GameState {
	t.Helper()
	const nSeats = 4

	rng := rand.New(rand.NewSource(seed))
	gs := gameengine.NewGameState(nSeats, rng, nil)
	gs.EventPolicy = gameengine.EventLogFull // we read the event log to assert per-seat activity

	decks := make([]*gameengine.CommanderDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		decks[i] = buildSyntheticDeck(itoa(i))
		for _, c := range decks[i].CommanderCards {
			c.Owner = i
		}
		for _, c := range decks[i].Library {
			c.Owner = i
		}
		rng.Shuffle(len(decks[i].Library), func(a, b int) {
			decks[i].Library[a], decks[i].Library[b] = decks[i].Library[b], decks[i].Library[a]
		})
	}
	gameengine.SetupCommanderGame(gs, decks)

	for i := 0; i < nSeats; i++ {
		gs.Seats[i].Hat = hat.NewYggdrasilHat(nil, 0)
		if yh, ok := gs.Seats[i].Hat.(*hat.YggdrasilHat); ok {
			yh.Noise = 0
		}
		tournament.RunLondonMulligan(gs, i)
	}

	gs.Active = rng.Intn(nSeats)
	gs.Turn = 1

	for turn := 1; turn <= turnCap; turn++ {
		gs.Turn = turn
		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)
		if gs.CheckEnd() {
			break
		}
		next := nextLivingSeat(gs)
		if next < 0 {
			break
		}
		gs.Active = next
	}
	return gs
}

// nextLivingSeat is a thin copy of the tournament-internal helper so
// the test doesn't need an exported version. Returns the next seat
// index clockwise that isn't Lost/LeftGame, or -1 if none.
func nextLivingSeat(gs *gameengine.GameState) int {
	n := len(gs.Seats)
	for off := 1; off <= n; off++ {
		idx := (gs.Active + off) % n
		s := gs.Seats[idx]
		if s != nil && !s.Lost && !s.LeftGame {
			return idx
		}
	}
	return -1
}

// TestSelfPlay_FourYggdrasilCohesion is the top-level cohesion test.
// Four YggdrasilHats play a synthetic 4-seat game. We assert four
// things any healthy run satisfies:
//
//  1. Termination — the game ended (winner declared, or turn cap hit
//     gracefully) without panic. A non-terminating loop would fail
//     test timeout; an unhandled panic would crash the test.
//  2. Per-seat activity — every seat that survived past turn 2 cast
//     or played at least one card. A seat that goes the whole game
//     without a single play_land or cast event means a decision path
//     is silently returning nil (the all-pass bug).
//  3. Game length — at least 3 turns of play. A 4-seat 40-life game
//     ending on turn 1 or 2 would mean something is killing seats
//     instantly (e.g. conviction over-eagerly conceding, or a
//     state-based action loop draining life).
//  4. No "every turn was empty" — across the whole game, at least one
//     attack was attempted, at least one land was played per seat
//     that survived, and the game's total event count is non-trivial.
func TestSelfPlay_FourYggdrasilCohesion(t *testing.T) {
	if testing.Short() {
		t.Skip("self-play game loop, skipped in -short mode")
	}

	const turnCap = 30
	gs := runSelfPlay(t, 42, turnCap)

	// (1) Termination is implicit: if the loop hadn't terminated the
	// test would have hit Go's test timeout. We log the outcome for
	// triage if a later assertion fails.
	winner := -1
	if gs.Flags != nil {
		if w, ok := gs.Flags["winner"]; ok && gs.Flags["ended"] == 1 {
			winner = w
		}
	}
	t.Logf("self-play finished: turn=%d winner=%d ended=%v",
		gs.Turn, winner, gs.Flags["ended"] == 1)

	// (3) Game length sanity. A 4-seat 40-life game ending on turn 1
	// indicates an instant-loss bug (over-eager conviction concession,
	// a damage-state-based-action loop, etc.).
	if gs.Turn < 3 {
		t.Errorf("game ended on turn %d — 4-player 40-life shouldn't resolve that fast", gs.Turn)
	}

	// (2) + (4) Per-seat activity from the event log. Count play_land
	// and cast events per seat. A seat that survived past turn 2 with
	// zero plays means a decision path silently returned nil
	// somewhere — the all-pass regression we're guarding against.
	plays := make([]int, len(gs.Seats))
	casts := make([]int, len(gs.Seats))
	attacks := 0
	eventKinds := map[string]int{}
	for _, ev := range gs.EventLog {
		eventKinds[ev.Kind]++
		if ev.Seat < 0 || ev.Seat >= len(plays) {
			continue
		}
		switch ev.Kind {
		case "play_land":
			plays[ev.Seat]++
		case "cast":
			casts[ev.Seat]++
		case "declare_attackers":
			// declare_attackers only fires when at least one attacker
			// is declared (combat.go:504 gates the emit on
			// len(declared) > 0), so any occurrence counts.
			attacks++
		}
	}
	t.Logf("per-seat activity: plays=%v casts=%v attacks=%d events=%d",
		plays, casts, attacks, len(gs.EventLog))

	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Seats that died on turn 1 or 2 wouldn't have had time to do
		// much; allow them silence. A seat that survived past turn 2
		// must have at least one play_land or cast.
		eliminatedEarly := s.Lost && s.Flags != nil && s.Flags["lost_turn"] > 0 && s.Flags["lost_turn"] <= 2
		if eliminatedEarly {
			continue
		}
		if plays[i]+casts[i] == 0 {
			t.Errorf("seat %d survived past turn 2 with 0 plays (lands=%d casts=%d lost=%v) — all-pass regression?",
				i, plays[i], casts[i], s.Lost)
		}
	}

	// Total event volume sanity. A live 4-seat game across 3+ turns
	// emits hundreds of events (turn_start, phase transitions, draws,
	// triggers). A handful indicates the loop bailed silently.
	if len(gs.EventLog) < 50 {
		t.Errorf("event log has only %d events across %d turns — engine likely short-circuited", len(gs.EventLog), gs.Turn)
	}

	// Attack signal: across the whole game, at least ONE declare_attackers
	// event should fire — otherwise the table is stuck at 40 life with no
	// combat ever happening, which is a clear cohesion failure even at
	// budget=0 heuristic mode. This assertion is wrapped in a soft-log
	// rather than a hard failure when the game is short (turn cap < 8),
	// because heuristic hats might legitimately hold creatures back early
	// while ramping. Past 8 turns of vanilla-creature self-play across
	// four seats, zero attacks is a real signal.
	if attacks == 0 && gs.Turn >= 8 {
		t.Errorf("zero attacks across %d turns of 4-seat self-play — ChooseAttackers may be returning empty unconditionally (event kinds: %v)",
			gs.Turn, eventKinds)
	}
}

// TestSelfPlay_DifferentSeedsTerminate runs three different seeds to
// catch a non-termination bug that's seed-specific (a particular
// decision path going circular under one shuffle but not another).
// Lighter assertions than the cohesion test — primarily a no-panic
// no-hang sweep.
func TestSelfPlay_DifferentSeedsTerminate(t *testing.T) {
	if testing.Short() {
		t.Skip("self-play game loop, skipped in -short mode")
	}
	const turnCap = 25
	for _, seed := range []int64{1, 17, 99} {
		seed := seed
		t.Run("seed="+itoa(int(seed)), func(t *testing.T) {
			gs := runSelfPlay(t, seed, turnCap)
			if gs.Turn < 2 {
				t.Errorf("game stopped at turn %d — likely degenerate", gs.Turn)
			}
		})
	}
}
