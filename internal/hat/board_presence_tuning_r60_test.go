package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// board_presence_tuning_r60_test.go — regressions for the BoardPresence
// dimension tuning. Two targeted improvements (see poker.go::effectiveBoardPower
// and evaluator.go::scoreBoard):
//
//   1. Tapped + summoning-sick creatures derate their power contribution
//      (tapped × 0.7, sick-without-haste × 0.8) so a fully-committed
//      board scores below a fresh untapped board with equal raw power.
//
//   2. scoreBoard adds a small width bonus (0.05 × creature-count
//      differential vs opponent average) so 6 untapped 1/1 tokens score
//      modestly above one untapped 6/6 (same raw power, but multiple
//      independent bodies for block / sac / crew / equip / ETB-doubler).

// -----------------------------------------------------------------------------
// effectiveBoardPower
// -----------------------------------------------------------------------------

// Baseline: untapped, no summoning sickness — full raw power.
func TestEffectiveBoardPower_UntappedFullCredit(t *testing.T) {
	gs := newTestGame(t, 2)
	_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 3 {
		t.Errorf("untapped 3/3 should contribute full power 3; got %d", got)
	}
}

// Tapped creature de-rates to 70% — 3 power × 0.7 = 2.1 → rounds to 2.
func TestEffectiveBoardPower_TappedDerated(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
	p.Tapped = true

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 2 {
		t.Errorf("tapped 3/3 should derate to 0.7*3 = 2.1 → 2; got %d", got)
	}
	// Sanity: raw boardPower must still return 3 (the helper is non-mutating).
	if raw := boardPower(gs, gs.Seats[0]); raw != 3 {
		t.Errorf("raw boardPower must remain 3 for tapped 3/3; got %d", raw)
	}
}

// Summoning-sick creature without haste de-rates to 80%.
func TestEffectiveBoardPower_SickWithoutHasteDerated(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Fatty", []string{"creature"}, 5, nil), 5, 5)
	p.SummoningSick = true

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 4 {
		t.Errorf("sick 5/5 should derate to 0.8*5 = 4; got %d", got)
	}
}

// Summoning-sick WITH haste — no derate (haste lets it attack regardless).
func TestEffectiveBoardPower_HasteCancelsSickDerate(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin Guide", []string{"creature"}, 1, nil), 2, 2)
	p.SummoningSick = true
	addKeyword(p, "haste")

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 2 {
		t.Errorf("sick+haste 2/2 should be 2 (no derate); got %d", got)
	}
}

// Both tapped AND sick (rare — e.g. a vehicle crewed the turn it ETB'd):
// 0.7 × 0.8 = 0.56 → 5 × 0.56 = 2.8 → rounds to 3.
func TestEffectiveBoardPower_TappedAndSickStacks(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Fatty", []string{"creature"}, 5, nil), 5, 5)
	p.Tapped = true
	p.SummoningSick = true

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 3 {
		t.Errorf("tapped+sick 5/5 should be 0.7*0.8*5 = 2.8 → 3; got %d", got)
	}
}

// Planeswalkers contribute (loyalty + 2) regardless of board-state
// availability — loyalty is a persistent threat clock, not a per-turn
// availability question. Tapped/sick discounts only apply to creatures.
func TestEffectiveBoardPower_PlaneswalkerUndiscounted(t *testing.T) {
	gs := newTestGame(t, 2)
	pw := newTestPermanent(gs.Seats[0], newTestCardMinimal("Liliana", []string{"planeswalker", "legendary"}, 4, nil), 0, 0)
	pw.Counters = map[string]int{"loyalty": 4}
	pw.Tapped = true // simulate any state — should be irrelevant

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 6 {
		t.Errorf("Liliana loy=4 should contribute 4+2 = 6 regardless of Tapped; got %d", got)
	}
}

// -----------------------------------------------------------------------------
// liveCreatureCount
// -----------------------------------------------------------------------------

func TestLiveCreatureCount_CountsOnlyCreatures(t *testing.T) {
	gs := newTestGame(t, 2)
	_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
	_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil), 0, 0)
	_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Liliana", []string{"planeswalker", "legendary"}, 4, nil), 0, 0)
	_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Token", []string{"creature", "token"}, 0, nil), 1, 1)

	got := liveCreatureCount(gs.Seats[0])
	if got != 2 {
		t.Errorf("expected 2 creatures (Bear + Token); got %d (excludes Sol Ring + Liliana)", got)
	}
}

// -----------------------------------------------------------------------------
// scoreBoard integration
// -----------------------------------------------------------------------------

// The headline ask: 4 tapped 3/3s should score below 4 untapped 3/3s,
// because the tapped board can't defend this round and the untapped one
// can. Pre-r60 both scored identically (raw boardPower = 12 for both).
func TestScoreBoard_TappedBoardScoresBelowUntapped(t *testing.T) {
	// Two parallel games: seat 0 has 4 untapped 3/3s in one; 4 tapped 3/3s
	// in the other. seat 1 (opp) has 4 untapped 3/3s in both (constant).
	mkGame := func(t *testing.T, ourTapped bool) (*GameStateEvaluator, *gameengine.GameState) {
		gs := newTestGame(t, 2)
		for i := 0; i < 4; i++ {
			p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
			if ourTapped {
				p.Tapped = true
			}
		}
		for i := 0; i < 4; i++ {
			_ = newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
		}
		return &GameStateEvaluator{}, gs
	}

	untapped, ugs := mkGame(t, false)
	tapped, tgs := mkGame(t, true)

	untappedScore := untapped.scoreBoard(ugs, 0)
	tappedScore := tapped.scoreBoard(tgs, 0)

	if !(untappedScore > tappedScore) {
		t.Errorf("untapped board (%.3f) should score above tapped board (%.3f)",
			untappedScore, tappedScore)
	}
}

// 6 untapped 1/1 tokens vs 1 untapped 6/6 — same raw power but wide board
// gets the small width bonus.
func TestScoreBoard_WidthBonusFavorsWideOverTall(t *testing.T) {
	mkGame := func(t *testing.T, wide bool) (*GameStateEvaluator, *gameengine.GameState) {
		gs := newTestGame(t, 2)
		if wide {
			for i := 0; i < 6; i++ {
				_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 1, 1)
			}
		} else {
			_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Fatty", []string{"creature"}, 6, nil), 6, 6)
		}
		// Constant opponent: one 3/3.
		_ = newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
		return &GameStateEvaluator{}, gs
	}

	wide, wgs := mkGame(t, true)
	tall, tgs := mkGame(t, false)

	wideScore := wide.scoreBoard(wgs, 0)
	tallScore := tall.scoreBoard(tgs, 0)

	if !(wideScore > tallScore) {
		t.Errorf("wide board (6×1/1, %.3f) should score above tall board (1×6/6, %.3f) "+
			"with the r60 width bonus", wideScore, tallScore)
	}
}

// Symmetric sanity: with no opponents, scoreBoard returns 1 when we have
// any creatures, 0 otherwise (unchanged by the tuning).
func TestScoreBoard_NoOpponentsUnchanged(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Lost = true
	_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)

	e := &GameStateEvaluator{}
	got := e.scoreBoard(gs, 0)
	if got != 1 {
		t.Errorf("no-opponent board with creature should return 1; got %.3f", got)
	}
}

