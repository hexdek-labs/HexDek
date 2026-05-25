package hat

// Regressions for dev/hat-3rd-eye-r60 — recency-weighted action
// tracking and confidence-scaled archetype amplification in
// computeThreatLevel.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// TutoredWithin boundary cases
// ---------------------------------------------------------------------

// TestTutoredWithin_Boundaries pins the recency helper: a tutor on
// turn T is "within N" when the current turn is < T+N. The helper
// returns false for never-tutored profiles (LastTutorTurn == 0).
func TestTutoredWithin_Boundaries(t *testing.T) {
	cases := []struct {
		lastTutor, now, n int
		want              bool
	}{
		{lastTutor: 0, now: 5, n: 3, want: false},  // never tutored
		{lastTutor: 5, now: 5, n: 1, want: true},   // same turn
		{lastTutor: 4, now: 5, n: 2, want: true},   // 1 turn ago, within 2
		{lastTutor: 3, now: 5, n: 2, want: false},  // 2 turns ago, NOT within 2
		{lastTutor: 3, now: 5, n: 3, want: true},   // 2 turns ago, within 3
		{lastTutor: 1, now: 10, n: 4, want: false}, // long-ago tutor
		{lastTutor: 5, now: 5, n: 0, want: false},  // zero window is degenerate
	}
	for _, tc := range cases {
		p := &OpponentProfile{LastTutorTurn: tc.lastTutor}
		got := p.TutoredWithin(tc.now, tc.n)
		if got != tc.want {
			t.Errorf("TutoredWithin(last=%d, now=%d, n=%d) = %v, want %v",
				tc.lastTutor, tc.now, tc.n, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------
// Recency in computeThreatLevel
// ---------------------------------------------------------------------

// TestClassifyOpponent_RecentTutorRaisesThreatAboveStaleTutor: two
// combo-classified opponents identical in every observable except
// when their last tutor fired. The one who tutored last turn must
// read a noticeably higher ThreatLevel than the one who tutored 5
// turns ago. Pre-fix both produced identical threat (TutorsUsed >= 2
// was a flat flag).
func TestClassifyOpponent_RecentTutorRaisesThreatAboveStaleTutor(t *testing.T) {
	makeComboProfile := func(t *testing.T, lastTutorTurn, nowTurn int) float64 {
		t.Helper()
		gs := newTestGame(t, 2)
		gs.Turn = nowTurn
		gs.Seats[1].Life = 40
		h := primedYggdrasilHat(2)
		// Two tutors total — the second is the most recent.
		h.recordOpponentPlay("tutor", "Demonic Tutor", 1, nil, lastTutorTurn-1)
		h.recordOpponentPlay("tutor", "Vampiric Tutor", 1, nil, lastTutorTurn)
		h.opponentHeldMana[1] = 3
		// Anchor the combo classification with a known combo piece.
		h.comboPieceSet = map[string]bool{"Thassa's Oracle": true}
		thoracle := newTestCardMinimal("Thassa's Oracle", []string{"creature"}, 2, nil)
		h.recordOpponentPlay("cast", thoracle.DisplayName(), 1, thoracle, lastTutorTurn)
		prof := h.classifyOpponent(gs, 1)
		if prof.Archetype != "combo" {
			t.Fatalf("setup: archetype=%q, want combo", prof.Archetype)
		}
		return prof.ThreatLevel
	}

	recent := makeComboProfile(t, 5, 5) // tutored this turn
	stale := makeComboProfile(t, 1, 6)  // tutored 5 turns ago

	if recent <= stale {
		t.Errorf("recent-tutor combo threat (%.3f) should exceed stale-tutor (%.3f)",
			recent, stale)
	}
}

// TestClassifyOpponent_StaleTutorStillExceedsNoTutor: even after the
// recency window expires, a combo opponent with old tutors reads a
// higher threat than one with no tutors at all — the stale-tutor
// discount fires but doesn't zero out (we still know they're a combo
// deck with a tutor package).
func TestClassifyOpponent_StaleTutorStillExceedsNoTutor(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Turn = 12
	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 40

	hStale := primedYggdrasilHat(3)
	hStale.recordOpponentPlay("tutor", "Demonic Tutor", 1, nil, 1)
	hStale.recordOpponentPlay("tutor", "Vampiric Tutor", 1, nil, 2)
	hStale.opponentHeldMana[1] = 3
	hStale.comboPieceSet = map[string]bool{"Thassa's Oracle": true}
	oracle := newTestCardMinimal("Thassa's Oracle", []string{"creature"}, 2, nil)
	hStale.recordOpponentPlay("cast", oracle.DisplayName(), 1, oracle, 3)
	staleProf := hStale.classifyOpponent(gs, 1)

	hNone := primedYggdrasilHat(3)
	hNone.opponentHeldMana[1] = 3
	hNone.comboPieceSet = map[string]bool{"Thassa's Oracle": true}
	// Seed the classifier into combo via combo signals only (no tutor).
	hNone.recordOpponentPlay("cast", oracle.DisplayName(), 1, oracle, 3)
	hNone.recordOpponentPlay("cast", oracle.DisplayName(), 1, oracle, 4)
	noneProf := hNone.classifyOpponent(gs, 1)

	if noneProf.Archetype != "combo" || staleProf.Archetype != "combo" {
		t.Fatalf("setup: stale=%q none=%q (want both combo)", staleProf.Archetype, noneProf.Archetype)
	}
	if staleProf.ThreatLevel <= noneProf.ThreatLevel {
		t.Errorf("stale-tutor combo threat (%.3f) should exceed no-tutor (%.3f) — the +0.05 stale discount must still be positive",
			staleProf.ThreatLevel, noneProf.ThreatLevel)
	}
}

// ---------------------------------------------------------------------
// Confidence-scaled archetype amplification
// ---------------------------------------------------------------------

// TestComputeThreatLevel_ConfidenceScalesArchetypeBumps: a combo
// classification at 0.5 confidence should produce LESS archetype
// amplification than the same classification at 0.9 confidence. The
// board / hand / life observations are held constant; only confidence
// varies. Pre-fix the archetype bumps fired at full strength
// regardless of certainty.
func TestComputeThreatLevel_ConfidenceScalesArchetypeBumps(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	gs.Seats[1].Life = 40
	// Empty board / hand so archetype bumps dominate the delta.

	lowConf := &OpponentProfile{
		Archetype:     "combo",
		Confidence:    0.5,
		TutorsUsed:    2,
		LastTutorTurn: 5,
	}
	highConf := &OpponentProfile{
		Archetype:     "combo",
		Confidence:    0.9,
		TutorsUsed:    2,
		LastTutorTurn: 5,
	}

	lowThreat := computeThreatLevel(gs, 1, lowConf)
	highThreat := computeThreatLevel(gs, 1, highConf)

	if highThreat <= lowThreat {
		t.Errorf("higher-confidence combo threat (%.3f) should exceed lower-confidence (%.3f)",
			highThreat, lowThreat)
	}
	// Sanity: the bump should be material — at least 0.10 spread over
	// the 0.5 confidence gap (combo's archetype contribution is
	// 0.25 + 0.20 = 0.45 at full confidence; the delta between 0.5
	// and 0.9 is 0.40 * 0.45 = 0.18).
	if highThreat-lowThreat < 0.10 {
		t.Errorf("confidence scaling delta (%.3f) is too small — combo amplification should swing with confidence",
			highThreat-lowThreat)
	}
}

// TestComputeThreatLevel_ZeroConfidenceLeavesRawObservations: a
// zero-confidence classification (we have no idea what they are)
// should leave board / hand / life observations intact while
// archetype bumps drop to zero. Pins that the raw observations are
// not multiplied by confidence — only the archetype-specific bumps.
func TestComputeThreatLevel_ZeroConfidenceLeavesRawObservations(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	gs.Seats[1].Life = 40
	// Give them a creature on the board and a card in hand so the
	// raw-observation terms have non-zero magnitude.
	c := newTestCardMinimal("Bear", []string{"creature"}, 2, nil)
	newTestPermanent(gs.Seats[1], c, 2, 2)
	gs.Seats[1].Hand = []*gameengine.Card{newTestCardMinimal("Filler", nil, 0, nil)}

	prof := &OpponentProfile{
		Archetype:     "combo",
		Confidence:    0.0, // we have no idea
		TutorsUsed:    2,
		LastTutorTurn: 5,
	}
	threat := computeThreatLevel(gs, 1, prof)

	// Raw observations: 1 creature (*0.04) + 2 power (*0.02) + 1 hand (*0.02) = 0.10
	// Archetype bumps with confidence 0: 0 + 0 = 0
	// Expected total: 0.10.
	want := 0.10
	if delta := threat - want; delta < -0.001 || delta > 0.001 {
		t.Errorf("zero-confidence threat=%.3f, want ≈%.3f (raw observations only, no archetype amplification)",
			threat, want)
	}
}

// TestRecordOpponentPlay_LastTutorTurnPopulated verifies the
// recordOpponentPlay path actually writes the recency field on
// "tutor" / "search_library" events and on "cast" events that
// resolve to tutor oracle text.
func TestRecordOpponentPlay_LastTutorTurnPopulated(t *testing.T) {
	h := primedYggdrasilHat(2)
	h.recordOpponentPlay("tutor", "Demonic Tutor", 1, nil, 4)
	if got := h.opponentProfiles[1].LastTutorTurn; got != 4 {
		t.Errorf("tutor event: LastTutorTurn=%d, want 4", got)
	}
	// search_library is the alias the engine emits for fetchlands etc.
	h.recordOpponentPlay("search_library", "Cultivate", 1, nil, 7)
	if got := h.opponentProfiles[1].LastTutorTurn; got != 7 {
		t.Errorf("search_library event: LastTutorTurn=%d, want 7", got)
	}
}
