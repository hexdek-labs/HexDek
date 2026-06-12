package gameengine

// r63 — seed 8675309 game 209 regression: the cumulative-validation
// sweep's one residual was a deterministic LifeConsistency +
// SBACompleteness false positive on a LEGAL Platinum Angel win.
//
// Sequence (from the full game-209 event log): seat 1 dropped to −6
// behind their own Platinum Angel (loss_prevented, CR §704.5a + §614);
// the Angel itself then attacked the last opponent to death; the
// §104.2a last-seat-standing win fired immediately; post-game cleanup
// (silent — LogEvent suppressed once ended=1) removed the Angel from
// the battlefield snapshot. The invariants then saw "life=−6,
// Lost=false, no loss-prevention on battlefield" and flagged a state
// that was rules-correct end to end.
//
// Fix: winners are exempt from both life checks. The r60 zombie-game
// pin (ended=1 without resolved outcomes — cap-draw seed 1337 game 465)
// is preserved by the negative control below: a seat that is neither
// Lost nor Won still flags.

import (
	"strings"
	"testing"
)

func violationsByName(gs *GameState) map[string]string {
	out := map[string]string{}
	for _, v := range RunAllInvariants(gs) {
		out[v.Name] = v.Message
	}
	return out
}

// The game-209 shape: winner at negative life, protection no longer
// visible on the battlefield, game ended. Must be clean.
func TestWinnerAtNegativeLife_PlatinumAngelWin_NotFlagged(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Flags["ended"] = 1
	gs.Seats[0].Life = -6
	gs.Seats[0].Won = true // §104.2a last-seat-standing
	gs.Seats[1].Life = -3
	gs.Seats[1].Lost = true

	got := violationsByName(gs)
	if msg, ok := got["LifeConsistency"]; ok {
		t.Errorf("legal Platinum Angel win flagged by LifeConsistency: %s", msg)
	}
	if msg, ok := got["SBACompleteness"]; ok && strings.Contains(msg, "704.5a") {
		t.Errorf("legal Platinum Angel win flagged by SBACompleteness: %s", msg)
	}
}

// Negative control — the r60 zombie-game pin must survive the winner
// exemption: a seat at negative life that is NEITHER Lost NOR Won (and
// has no protection) still flags both checks.
func TestZombieSeatAtNegativeLife_StillFlagged(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Flags["ended"] = 1
	gs.Seats[0].Life = -6 // zombie: not Lost, not Won
	gs.Seats[1].Life = -3
	gs.Seats[1].Lost = true

	got := violationsByName(gs)
	if _, ok := got["LifeConsistency"]; !ok {
		t.Error("zombie seat (neither Lost nor Won) at -6 must still flag LifeConsistency")
	}
	if msg, ok := got["SBACompleteness"]; !ok || !strings.Contains(msg, "704.5a") {
		t.Errorf("zombie seat at -6 must still flag SBACompleteness 704.5a; got %q", msg)
	}
}

// Mid-game protection path unchanged: a live (not Won) seat at negative
// life WITH Platinum Angel on the battlefield stays clean.
func TestLiveSeatAtNegativeLife_AngelOnBattlefield_NotFlagged(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Life = -6
	angel := &Permanent{
		Card:       &Card{Name: "Platinum Angel", Owner: 0, Types: []string{"artifact", "creature"}},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, angel)

	got := violationsByName(gs)
	if msg, ok := got["LifeConsistency"]; ok {
		t.Errorf("protected live seat flagged by LifeConsistency: %s", msg)
	}
}
