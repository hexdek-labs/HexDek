package seedcontract

import "testing"

// r62 — Seal canonicalization (review 08, C-H2). The seal path and the
// replay-verify path previously computed KillMethod through two
// drifted mappers; Seal now derives it from (Winner, EndReason) so any
// two callers that agree on the game agree on the digest.

func TestSeal_DerivesKillMethod(t *testing.T) {
	cases := []struct {
		name      string
		winner    int
		endReason string
		want      string
	}{
		{"natural win", 1, "last_seat_standing", "last_seat_standing"},
		{"turn cap leader", 0, "turn_cap_leader", "timeout"},
		{"turn cap tie", 2, "turn_cap_tie", "timeout"},
		{"turn cap", 0, "turn_cap", "timeout"},
		{"draw has no kill", -1, "draw", ""},
		{"all dead at cap has no kill", -1, "turn_cap_all_dead", ""},
		{"crash has no kill", -1, "crash", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New(Inputs{RNGSeed: 7, NSeats: 4})
			c.Seal(Outcome{
				Winner:     tc.winner,
				EndReason:  tc.endReason,
				KillMethod: "attacker-controlled-garbage", // must be ignored
			})
			if c.Outcome.KillMethod != tc.want {
				t.Errorf("KillMethod = %q, want %q", c.Outcome.KillMethod, tc.want)
			}
		})
	}
}

// Two callers disagreeing only on the (derived) KillMethod field must
// produce identical digests — this is the exact drift class that made
// the verifier reject honest draws (seal said "", replay said "draw").
func TestSeal_KillMethodCannotDesyncDigest(t *testing.T) {
	base := Outcome{
		Winner:           -1,
		Turns:            44,
		EndReason:        "draw",
		EliminationOrder: [4]int{0, 1, 2, 3},
		FinalLife:        [4]int{0, 0, 0, 0},
	}
	sealSide := base
	sealSide.KillMethod = "" // what the pre-r62 tournament seal produced
	replaySide := base
	replaySide.KillMethod = "draw" // what the pre-r62 replay produced

	a := New(Inputs{})
	a.Seal(sealSide)
	b := New(Inputs{})
	b.Seal(replaySide)
	if a.OutcomeDigest != b.OutcomeDigest {
		t.Fatalf("digest desync on caller-supplied KillMethod:\n  seal   %s\n  replay %s",
			a.OutcomeDigest, b.OutcomeDigest)
	}
}

// Contracts sealed by pre-r62 builds stored KillMethod values that the
// new canonicalization derives identically (winner<0 → "", else the
// shared end-reason mapper — the old seal-side rule). CheckIntegrity
// must therefore keep passing on the existing signed corpus without a
// schema bump.
func TestSeal_BackCompat_OldCanonicalFormRedigests(t *testing.T) {
	key := DeriveContractKey([]byte("master"), "tournament:bc")
	for _, out := range []Outcome{
		{Winner: 2, Turns: 19, KillMethod: "last_seat_standing",
			EndReason:        "last_seat_standing",
			EliminationOrder: [4]int{1, 0, 3, 2}, FinalLife: [4]int{-2, 0, 31, 0}},
		{Winner: -1, Turns: 80, KillMethod: "",
			EndReason:        "draw",
			EliminationOrder: [4]int{0, 1, 2, 3}},
		{Winner: 0, Turns: 80, KillMethod: "timeout",
			EndReason:        "turn_cap_leader",
			EliminationOrder: [4]int{3, 0, 1, 2}, FinalLife: [4]int{40, 12, 8, 1}},
	} {
		c := New(Inputs{RNGSeed: 99, NSeats: 4, EngineVersion: "old"})
		c.Seal(out)
		c.Sign(key)
		// Simulate reading the contract back from disk: the stored
		// Outcome must re-digest to the stored OutcomeDigest.
		if err := c.CheckIntegrity(key); err != nil {
			t.Errorf("old-canonical outcome failed integrity after r62 Seal change: %v (outcome %+v)", err, out)
		}
		if c.Outcome.KillMethod != out.KillMethod {
			t.Errorf("canonicalization changed a previously-stored KillMethod %q -> %q — old contracts would break",
				out.KillMethod, c.Outcome.KillMethod)
		}
	}
}

func TestKillMethodFromEndReason(t *testing.T) {
	for in, want := range map[string]string{
		"turn_cap":           "timeout",
		"turn_cap_leader":    "timeout",
		"turn_cap_tie":       "timeout",
		"turn_cap_all_dead":  "timeout",
		"draw":               "draw",
		"crash":              "crash",
		"last_seat_standing": "last_seat_standing",
		"":                   "",
	} {
		if got := KillMethodFromEndReason(in); got != want {
			t.Errorf("KillMethodFromEndReason(%q) = %q, want %q", in, got, want)
		}
	}
}
