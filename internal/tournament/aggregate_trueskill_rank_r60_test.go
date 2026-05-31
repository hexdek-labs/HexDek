package tournament

import (
	"testing"
)

// R60 regression: aggregate()'s TrueSkill rank attribution must use
// the deck-indexed EliminationOrder via the participant deck index,
// NOT the loop seat index. Pre-fix the code read
// `outcome.EliminationOrder[i]` (seat) instead of [idx] (deck), so
// every rot != 0 game (75% of standard 4-seat rotate mode) fed
// scrambled ranks to TrueSkill — the wrong commander was credited
// with the win.
//
// The synthetic-outcome harness here lets us bypass the engine and
// directly exercise the aggregator's index math.

// makeOutcome constructs a GameOutcome describing a game where the
// deck index `winnerDeckIdx` placed in slot N-1 (winner) and the
// remaining decks placed 0..N-2 in increasing deck-index order. The
// rotation determines which seat each deck played from
// (participants[seat] = (seat + rot) % nSeats).
func makeOutcome(gameIdx, nSeats, rot, winnerDeckIdx int) GameOutcome {
	o := GameOutcome{
		GameIdx:                  gameIdx,
		Rot:                      rot,
		Turns:                    10,
		EndReason:                "last_seat_standing",
		ParticipantCommanderIdxs: make([]int, nSeats),
		EliminationOrder:         make([]int, nSeats),
	}
	for seat := 0; seat < nSeats; seat++ {
		deckIdx := (seat + rot) % nSeats
		o.ParticipantCommanderIdxs[seat] = deckIdx
		if deckIdx == winnerDeckIdx {
			o.Winner = seat
			o.WinnerCommanderIdx = deckIdx
		}
	}
	// Fill EliminationOrder: losers in deck-index order get slots
	// 0..N-2, winner gets N-1.
	slot := 0
	for d := 0; d < nSeats; d++ {
		if d == winnerDeckIdx {
			continue
		}
		o.EliminationOrder[d] = slot
		slot++
	}
	o.EliminationOrder[winnerDeckIdx] = nSeats - 1
	return o
}

// feedOne pushes one outcome through aggregate() and returns the
// result. nGames=1 to keep the harness terse.
func feedOne(o GameOutcome, names []string) *TournamentResult {
	ch := make(chan GameOutcome, 1)
	ch <- o
	close(ch)
	return aggregate(ch, 1, len(names), names)
}

func TestAggregate_TrueSkillRankUsesDeckIndex_Rot0(t *testing.T) {
	// Sanity: rot=0 is the only case the pre-fix code happened to
	// handle correctly (because deck_idx == seat_idx). Pin it
	// independently so a future fix that breaks this case is
	// caught.
	names := []string{"A", "B", "C", "D"}
	o := makeOutcome(0, 4, 0, 2) // deck C (idx 2) wins
	r := feedOne(o, names)
	if r.WinsByCommander["C"] != 1 {
		t.Fatalf("rot=0: expected C to win 1 game, got %v", r.WinsByCommander)
	}
	// Highest mu must belong to the winner.
	top := r.TrueSkill[0]
	if top.Commander != "C" {
		t.Errorf("rot=0: TrueSkill top should be the winning deck (C), got %s (full: %+v)", top.Commander, r.TrueSkill)
	}
}

func TestAggregate_TrueSkillRankUsesDeckIndex_RotNonZero(t *testing.T) {
	// This is the pre-fix bug surface. With rot=1 and deck 0 winning
	// from seat 3, EliminationOrder = [3, 0, 1, 2] (deck-indexed).
	// Pre-fix the aggregator read EliminationOrder[seat], which for
	// seat 0 (= deck 1) returned slot 3 → rank 0 (winner). TrueSkill
	// would then credit deck B with the win, not deck A.
	names := []string{"A", "B", "C", "D"}
	o := makeOutcome(0, 4, 1, 0) // deck A (idx 0) wins; seat 3 plays A
	r := feedOne(o, names)

	if r.WinsByCommander["A"] != 1 {
		t.Fatalf("rot=1 winner accounting wrong: %v", r.WinsByCommander)
	}
	if got := r.TrueSkill[0].Commander; got != "A" {
		t.Errorf("rot=1: TrueSkill top should be the winning deck (A), got %s. "+
			"Pre-fix this returned B (the deck physically seated at seat 0 where the "+
			"wrong elimination slot was read). Full ranking: %+v", got, r.TrueSkill)
	}
	// Last place (lowest mu) must be the deck that was eliminated
	// first. EliminationOrder = [3, 0, 1, 2] → deck idx 1 = B is
	// slot 0 = first out.
	last := r.TrueSkill[len(r.TrueSkill)-1]
	if last.Commander != "B" {
		t.Errorf("rot=1: TrueSkill last should be first-out deck (B), got %s. Full: %+v",
			last.Commander, r.TrueSkill)
	}
}

func TestAggregate_TrueSkillRankUsesDeckIndex_RotAllNonZero(t *testing.T) {
	// Sweep all three non-zero rotations to confirm the fix isn't
	// load-bearing on one specific rotation modulus. In every case
	// deck D wins; TrueSkill must place D first regardless of which
	// physical seat D was assigned to.
	for rot := 1; rot < 4; rot++ {
		names := []string{"A", "B", "C", "D"}
		o := makeOutcome(0, 4, rot, 3) // deck D always wins
		r := feedOne(o, names)
		if got := r.TrueSkill[0].Commander; got != "D" {
			t.Errorf("rot=%d: TrueSkill top should be winning deck (D), got %s. Full: %+v",
				rot, got, r.TrueSkill)
		}
	}
}

func TestAggregate_TrueSkillSigmaDropsForClearWinner(t *testing.T) {
	// Operational consequence: across a sequence of games where the
	// same deck always wins (and rotation cycles through every
	// position), the winning deck's TrueSkill sigma must drop AND
	// its mu must end above the default. Pre-fix, the scrambled
	// ranks meant the "always-wins" deck got credited correctly
	// only 25% of the time — sigma still drops (any update tightens
	// the posterior) but mu drift relative to peers is muted.
	names := []string{"A", "B", "C", "D"}
	ch := make(chan GameOutcome, 40)
	for g := 0; g < 40; g++ {
		rot := g % 4
		ch <- makeOutcome(g, 4, rot, 0) // deck A wins every game
	}
	close(ch)
	r := aggregate(ch, 40, 4, names)

	var muA, sigmaA float64
	found := false
	for _, e := range r.TrueSkill {
		if e.Commander == "A" {
			muA, sigmaA = e.Mu, e.Sigma
			found = true
		}
	}
	if !found {
		t.Fatal("deck A missing from TrueSkill snapshot")
	}
	if muA <= 25.0 {
		t.Errorf("clear winner mu should rise above default (25), got %f", muA)
	}
	if sigmaA >= 25.0/3.0 {
		t.Errorf("clear winner sigma should drop below default (8.33), got %f", sigmaA)
	}
	// And A must be the top-ranked deck.
	if r.TrueSkill[0].Commander != "A" {
		t.Errorf("after 40 wins by A, A must be top of TrueSkill; got %s. Full: %+v",
			r.TrueSkill[0].Commander, r.TrueSkill)
	}
}
