package heimdall

import (
	"encoding/json"
	"strings"
	"testing"
)

// narrative_r60_test.go — pins SummarizeGame + SummarizeReplayEvents
// output. The narrative is the consumer-facing surface, so the tests
// pin SHAPE (sentences in expected order) rather than exact strings —
// future polish to phrasing should not break tests.

// mkBaselineSnap builds a 4-seat snapshot with realistic data:
// commander cast turns, MVPs, mulligan stats. Tests mutate it to
// exercise specific branches.
func mkBaselineSnap() GameObservationSnapshot {
	return GameObservationSnapshot{
		Seed: GameSeed{
			RNGSeed:    42,
			DeckKeys:   [4]string{"alice/tergrid", "bob/atraxa", "carol/edric", "dave/krenko"},
			Winner:     2,
			Turns:      9,
			KillMethod: "commander",
		},
		FinalTurn: 9,
		CommanderNamesBySeat: [][]string{
			{"Atraxa, Praetors' Voice"},
			{"Edric, Spymaster of Trest"},
			{"Tergrid, God of Fright"},
			{"Krenko, Mob Boss"},
		},
		CardFirstPlayed: map[string]int{
			"Atraxa, Praetors' Voice":  6,
			"Edric, Spymaster of Trest": 4,
			"Tergrid, God of Fright":   4,
			"Krenko, Mob Boss":         5,
			"Sol Ring":                 1,
			"Liliana the Last Hope":    5,
			"Bolas's Citadel":          7,
		},
		CommanderZoneVisits: []CommanderZoneVisit{
			{Seat: 2, CommanderName: "Tergrid, God of Fright", CastCount: 1, InZoneAtEnd: false, Visits: 1},
		},
		MVPCards: []MVPCard{
			{Seat: 2, CardName: "Tergrid, God of Fright", CMC: 5, TurnPlayed: 4, Score: 11, IsCommander: true},
			{Seat: 2, CardName: "Liliana the Last Hope", CMC: 3, TurnPlayed: 5, Score: 8, IsCommander: false},
			{Seat: 2, CardName: "Bolas's Citadel", CMC: 7, TurnPlayed: 7, Score: 12, IsCommander: false},
			{Seat: 0, CardName: "Doubling Season", CMC: 5, TurnPlayed: 6, Score: 9, IsCommander: false},
		},
		RegretCards: []RegretCard{
			{Seat: 1, CardName: "Praetor's Counsel", CMC: 8, Reason: "stranded_in_hand"},
			{Seat: 3, CardName: "Goblin Charbelcher", CMC: 4, Reason: "stranded_in_hand"},
		},
		MulliganStats: []MulliganStat{
			{Seat: 0, MulligansTaken: 0, OpeningHandSize: 7, Lands: 3, ActionCards: 4, Keepable: true},
			{Seat: 1, MulligansTaken: 1, OpeningHandSize: 6, Lands: 3, ActionCards: 3, Keepable: true},
			{Seat: 2, MulligansTaken: 0, OpeningHandSize: 7, Lands: 3, ActionCards: 4, Keepable: true},
			{Seat: 3, MulligansTaken: 2, OpeningHandSize: 5, Lands: 2, ActionCards: 3, Keepable: false},
		},
	}
}

// TestSummarizeGame_HappyPathStructure pins the canonical 4-sentence
// shape: outcome + mulligan + commander cast + MVPs + regret.
func TestSummarizeGame_HappyPathStructure(t *testing.T) {
	out := SummarizeGame(mkBaselineSnap())

	// Opening: winner identified by seat + commander, turn, kill method.
	if !strings.Contains(out, "Seat 2 (Tergrid, God of Fright) won on turn 9 via commander damage.") {
		t.Errorf("opening sentence missing/malformed; got:\n%s", out)
	}
	// Mulligan: seats 1 and 3 mulliganed.
	if !strings.Contains(out, "Seat 1 mulliganed to 6") {
		t.Errorf("missing seat 1 mulligan; got:\n%s", out)
	}
	if !strings.Contains(out, "Seat 3 mulliganed to 5") {
		t.Errorf("missing seat 3 mulligan; got:\n%s", out)
	}
	// Commander cast turn.
	if !strings.Contains(out, `They cast "Tergrid, God of Fright" on turn 4.`) {
		t.Errorf("missing commander cast sentence; got:\n%s", out)
	}
	// Top MVPs (Bolas's Citadel score 12 > Liliana score 8; commander excluded).
	if !strings.Contains(out, `Their key supporting pieces were "Bolas's Citadel" and "Liliana the Last Hope".`) {
		t.Errorf("missing/malformed MVP sentence; got:\n%s", out)
	}
	// Regret: Seat 1's 8-cost Praetor's Counsel beats seat 3's 4-cost (below threshold anyway).
	if !strings.Contains(out, `Seat 1 never resolved their 8-cost "Praetor's Counsel".`) {
		t.Errorf("missing regret sentence; got:\n%s", out)
	}
}

// TestSummarizeGame_Draw pins the draw-framed opening.
func TestSummarizeGame_Draw(t *testing.T) {
	snap := mkBaselineSnap()
	snap.Seed.Winner = -1
	snap.Seed.KillMethod = "timeout"
	out := SummarizeGame(snap)
	if !strings.Contains(out, "The game ended without a winner on turn 9.") {
		t.Errorf("draw opening missing; got:\n%s", out)
	}
	// No "Seat X won" anywhere.
	if strings.Contains(out, "won on turn") {
		t.Errorf("draw should NOT contain 'won on turn'; got:\n%s", out)
	}
}

// TestSummarizeGame_NoMulligans pins that the mulligan sentence is
// dropped when no seat mulliganed (modal case — silence is golden).
func TestSummarizeGame_NoMulligans(t *testing.T) {
	snap := mkBaselineSnap()
	for i := range snap.MulliganStats {
		snap.MulliganStats[i].MulligansTaken = 0
		snap.MulliganStats[i].OpeningHandSize = 7
	}
	out := SummarizeGame(snap)
	if strings.Contains(out, "mulliganed") {
		t.Errorf("no-mulligan case should not mention mulligans; got:\n%s", out)
	}
}

// TestSummarizeGame_KillMethodRemap pins the friendly-clause remap
// across all known KillMethod values.
func TestSummarizeGame_KillMethodRemap(t *testing.T) {
	cases := []struct {
		killMethod string
		want       string
	}{
		{"combat", "combat damage"},
		{"commander", "commander damage"},
		{"combo", "a combo finish"},
		{"mill", "decking out an opponent"},
		{"poison", "infect / poison counters"},
		{"", ""},
	}
	for _, c := range cases {
		snap := mkBaselineSnap()
		snap.Seed.KillMethod = c.killMethod
		out := SummarizeGame(snap)
		if c.want == "" {
			// Empty / unknown → no "via …" tail at all.
			if strings.Contains(out, "won on turn 9 via") {
				t.Errorf("empty kill method should drop 'via' clause; got:\n%s", out)
			}
		} else if !strings.Contains(out, "via "+c.want) {
			t.Errorf("kill method %q: expected 'via %s'; got:\n%s", c.killMethod, c.want, out)
		}
	}
}

// TestSummarizeGame_RecastClause pins the recast-count branches in
// the commander sentence. CastCount=2 → "recast it once"; >=3 →
// "recast it N times".
func TestSummarizeGame_RecastClause(t *testing.T) {
	snap := mkBaselineSnap()
	// 2 casts → "once".
	snap.CommanderZoneVisits[0].CastCount = 2
	out := SummarizeGame(snap)
	if !strings.Contains(out, "recast it once after answering removal.") {
		t.Errorf("CastCount=2 should mention 'recast it once'; got:\n%s", out)
	}
	// 4 casts → "4 times".
	snap.CommanderZoneVisits[0].CastCount = 4
	out = SummarizeGame(snap)
	if !strings.Contains(out, "recast it 4 times across the game.") {
		t.Errorf("CastCount=4 should mention 'recast it 4 times'; got:\n%s", out)
	}
}

// TestSummarizeGame_NoMVPsForWinner pins that the MVP sentence is
// dropped when the winner's only MVP entry is the commander itself.
func TestSummarizeGame_NoMVPsForWinner(t *testing.T) {
	snap := mkBaselineSnap()
	// Keep only the commander entry on seat 2.
	filtered := snap.MVPCards[:0]
	for _, m := range snap.MVPCards {
		if m.Seat == 2 && !m.IsCommander {
			continue
		}
		filtered = append(filtered, m)
	}
	snap.MVPCards = filtered
	out := SummarizeGame(snap)
	if strings.Contains(out, "key supporting piece") {
		t.Errorf("commander-only MVP set should skip MVP sentence; got:\n%s", out)
	}
}

// TestSummarizeGame_RegretBelowThresholdHidden pins the 5-CMC floor
// on the regret sentence. A 3-CMC stranded card shouldn't surface.
func TestSummarizeGame_RegretBelowThresholdHidden(t *testing.T) {
	snap := mkBaselineSnap()
	snap.RegretCards = []RegretCard{
		{Seat: 1, CardName: "Cheap Spell", CMC: 3, Reason: "stranded_in_hand"},
	}
	out := SummarizeGame(snap)
	if strings.Contains(out, "never resolved") {
		t.Errorf("3-CMC regret should be hidden; got:\n%s", out)
	}
}

// TestSummarizeGame_EmptyInput pins the unavailable-data guard.
func TestSummarizeGame_EmptyInput(t *testing.T) {
	out := SummarizeGame(GameObservationSnapshot{})
	if out != "Game data unavailable." {
		t.Errorf("empty snapshot → %q, want 'Game data unavailable.'", out)
	}
}

// TestSummarizeGame_SinglePeriodPerSentence pins that the output
// joins sentences with a single space and each sentence ends with a
// period. Defends against double-period bugs from concatenation.
func TestSummarizeGame_SinglePeriodPerSentence(t *testing.T) {
	out := SummarizeGame(mkBaselineSnap())
	if strings.Contains(out, "..") {
		t.Errorf("output contains double period; got:\n%s", out)
	}
	if strings.Contains(out, " .") {
		t.Errorf("output contains space before period; got:\n%s", out)
	}
	if !strings.HasSuffix(out, ".") {
		t.Errorf("output should end with a period; got:\n%s", out)
	}
}

// TestSummarizeReplayEvents_HappyPath pins SummarizeReplayEvents
// against a synthetic chronological event stream.
func TestSummarizeReplayEvents_HappyPath(t *testing.T) {
	events := []ReplayEvent{
		{Turn: 1, Kind: "card_first_played", Seat: 2, CardName: "Sol Ring", Description: ""},
		{Turn: 4, Kind: "card_first_played", Seat: 2, CardName: "Tergrid, God of Fright", IsCommander: true, Description: ""},
		{Turn: 5, Kind: "mvp_emerged", Seat: 2, CardName: "Bolas's Citadel", Score: 12, Description: ""},
		{Turn: 6, Kind: "mvp_emerged", Seat: 0, CardName: "Doubling Season", Score: 9, Description: ""},
		{Turn: 9, Kind: "game_end", Seat: 2, Description: "seat 2 won on turn 9"},
	}
	out := SummarizeReplayEvents(events, [][]string{
		{"Atraxa, Praetors' Voice"},
		{"Edric, Spymaster of Trest"},
		{"Tergrid, God of Fright"},
		{"Krenko, Mob Boss"},
	})
	if !strings.Contains(out, "Seat 2 (Tergrid, God of Fright) won on turn 9.") {
		t.Errorf("missing/malformed opening; got:\n%s", out)
	}
	if !strings.Contains(out, `Their commander "Tergrid, God of Fright" first resolved on turn 4.`) {
		t.Errorf("missing commander cast turn; got:\n%s", out)
	}
	if !strings.Contains(out, `Top MVPs at game end: "Bolas's Citadel" and "Doubling Season".`) {
		t.Errorf("missing/malformed MVP list; got:\n%s", out)
	}
}

// TestSummarizeReplayEvents_DegradesWithoutCommanderNames pins the
// graceful fallback when commanderNamesBySeat is nil.
func TestSummarizeReplayEvents_DegradesWithoutCommanderNames(t *testing.T) {
	events := []ReplayEvent{
		{Turn: 9, Kind: "game_end", Seat: 1, Description: ""},
	}
	out := SummarizeReplayEvents(events, nil)
	if !strings.Contains(out, "Seat 1 won on turn 9.") {
		t.Errorf("expected generic seat label; got:\n%s", out)
	}
}

// TestSummarizeReplayEvents_NoEnd pins the missing-game_end guard.
func TestSummarizeReplayEvents_NoEnd(t *testing.T) {
	out := SummarizeReplayEvents([]ReplayEvent{
		{Turn: 1, Kind: "card_first_played", Seat: 0, CardName: "Sol Ring"},
	}, nil)
	if !strings.Contains(out, "missing a game_end record") {
		t.Errorf("expected missing-end guard message; got:\n%s", out)
	}
}

// TestSummarizeReplayEvents_Empty pins the empty-input guard.
func TestSummarizeReplayEvents_Empty(t *testing.T) {
	if out := SummarizeReplayEvents(nil, nil); !strings.Contains(out, "Replay event stream is empty.") {
		t.Errorf("empty input: got %q", out)
	}
}

// TestSummarizeGame_RoundTripsThroughMarshalSnapshot pins the
// canonical persistence path: a snapshot that's been
// Marshal/Unmarshal'd through the JSON store should produce the same
// narrative as one summarized directly. Defends against the
// hypothetical bug where a field gets dropped by omitempty.
func TestSummarizeGame_RoundTripsThroughMarshalSnapshot(t *testing.T) {
	snap := mkBaselineSnap()
	directOut := SummarizeGame(snap)

	payload, err := MarshalSnapshot(snap)
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}
	var roundTripped GameObservationSnapshot
	if err := json.Unmarshal([]byte(payload), &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	roundTripOut := SummarizeGame(roundTripped)

	if directOut != roundTripOut {
		t.Errorf("round-trip drift:\n direct:    %s\n roundtrip: %s", directOut, roundTripOut)
	}
}

// TestSummarizeGame_BuildReplayLogParity is the cross-API parity
// check: SummarizeReplayEvents(BuildReplayLog(snap)) should produce a
// narrative consistent with SummarizeGame(snap) on the overlapping
// facts (opening sentence + commander cast turn). The two surfaces
// don't have to be identical — SummarizeReplayEvents is thinner — but
// they shouldn't disagree about basic facts like winner/turn.
func TestSummarizeGame_BuildReplayLogParity(t *testing.T) {
	snap := mkBaselineSnap()
	events := BuildReplayLog(snap)
	streamOut := SummarizeReplayEvents(events, snap.CommanderNamesBySeat)
	snapOut := SummarizeGame(snap)

	// Both narratives must reference seat 2 winning on turn 9.
	for _, fragment := range []string{"Seat 2 (Tergrid, God of Fright)", "turn 9"} {
		if !strings.Contains(streamOut, fragment) {
			t.Errorf("event-stream narrative missing %q; got:\n%s", fragment, streamOut)
		}
		if !strings.Contains(snapOut, fragment) {
			t.Errorf("snapshot narrative missing %q; got:\n%s", fragment, snapOut)
		}
	}
}
