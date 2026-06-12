package heimdall

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/validation"
)

// Consolidation step 2 — ClassifyKillFinal unification pins.
//
// The structured LossDetail path (every engine loss writer stamps it
// since step 2) must classify identically through ClassifyKill,
// ClassifyKillWithMaxTurns, and ClassifyKillFinal; the legacy
// freeform-string path must keep its pre-step-2 behavior for old
// replays and fixtures.

func s2Game(t *testing.T) *gameengine.GameState {
	t.Helper()
	return gameengine.NewGameState(4, rand.New(rand.NewSource(7)), nil)
}

// s2Eliminate marks a loss with BOTH the freeform string and the
// structured detail (what the engine writers do post-step-2), then runs
// the real §800.4a pipeline.
func s2Eliminate(t *testing.T, gs *gameengine.GameState, seatIdx int, reason string, detail *validation.LossReason) {
	t.Helper()
	gs.Seats[seatIdx].Lost = true
	gs.Seats[seatIdx].LossReason = reason
	gs.Seats[seatIdx].LossDetail = detail
	gameengine.HandleSeatElimination(gs, seatIdx)
}

func TestClassifyKillFinal_StructuredPerKillType(t *testing.T) {
	cases := []struct {
		name       string
		detail     *validation.LossReason
		reason     string
		wantMethod string
	}{
		{"poison", &validation.LossReason{Category: validation.LossCategoryPoison, Rule: "704.5c"}, "ten or more poison counters (CR 704.5c)", "poison"},
		{"commander", &validation.LossReason{Category: validation.LossCategoryCommanderDamage, Rule: "704.6c", SourceCard: "Edgar Markov"}, "21+ commander damage from Edgar Markov (CR 704.6c)", "commander"},
		{"mill", &validation.LossReason{Category: validation.LossCategoryEmptyLibrary, Rule: "704.5b"}, "drew from empty library (CR 704.5b)", "mill"},
		{"life_combat", &validation.LossReason{Category: validation.LossCategoryLife, Rule: "704.5a"}, "life total 0 or less (CR 704.5a)", "combat"},
		{"concession", &validation.LossReason{Category: validation.LossCategoryConcession, Rule: "104.3a"}, "concession", "concession"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs := s2Game(t)
			winner := 3
			if c.detail.Category == validation.LossCategoryLife {
				// The life category deliberately falls through to the
				// game-state heuristics — the victim's life must
				// actually be ≤0 for the combat read.
				gs.Seats[1].Life = 0
				gs.Seats[2].Life = -2
			}
			s2Eliminate(t, gs, 0, "ten or more poison counters (CR 704.5c)",
				&validation.LossReason{Category: validation.LossCategoryPoison, Rule: "704.5c"}) // decoy early kill
			s2Eliminate(t, gs, 1, c.reason, c.detail)
			s2Eliminate(t, gs, 2, c.reason, c.detail) // final elimination

			if got := ClassifyKill(gs, winner); got != c.wantMethod {
				t.Errorf("ClassifyKill = %q, want %q", got, c.wantMethod)
			}
			if got := ClassifyKillWithMaxTurns(gs, winner, 0); got != c.wantMethod {
				t.Errorf("ClassifyKillWithMaxTurns = %q, want %q", got, c.wantMethod)
			}
			fin := ClassifyKillFinal(gs, winner, 0)
			if fin.Method != c.wantMethod {
				t.Errorf("ClassifyKillFinal.Method = %q, want %q", fin.Method, c.wantMethod)
			}
			if fin.VictimSeat != 2 {
				t.Errorf("ClassifyKillFinal.VictimSeat = %d, want 2 (final elimination)", fin.VictimSeat)
			}
			if fin.Category != c.detail.Category || fin.Rule != c.detail.Rule || fin.SourceCard != c.detail.SourceCard {
				t.Errorf("ClassifyKillFinal detail = {%s %s %s}, want {%s %s %s}",
					fin.Category, fin.Rule, fin.SourceCard, c.detail.Category, c.detail.Rule, c.detail.SourceCard)
			}
		})
	}
}

// TestClassifyKillFinal_CommanderKillerSeat pins killer attribution: a
// commander-damage detail naming a commander resolves KillerSeat to the
// seat owning that commander.
func TestClassifyKillFinal_CommanderKillerSeat(t *testing.T) {
	gs := s2Game(t)
	gs.Seats[1].CommanderNames = []string{"Edgar Markov"}
	s2Eliminate(t, gs, 2, "21+ commander damage from Edgar Markov (CR 704.6c)",
		&validation.LossReason{Category: validation.LossCategoryCommanderDamage, Rule: "704.6c", SourceCard: "Edgar Markov"})

	fin := ClassifyKillFinal(gs, 1, 0)
	if fin.Method != "commander" {
		t.Fatalf("Method = %q, want commander", fin.Method)
	}
	if fin.KillerSeat != 1 {
		t.Errorf("KillerSeat = %d, want 1 (Edgar Markov's owner)", fin.KillerSeat)
	}
}

// TestClassifyKill_LegacyStringPathUnchanged pins the pre-step-2
// fallback: string-only states (old replays, hand-built fixtures)
// classify exactly as before. NOTE the concession case: string-only
// concession heuristically reads as "mill" (Lost with positive life) —
// that misclassification is exactly what LossDetail fixes for new
// games, but the legacy path must stay stable for replay comparisons.
func TestClassifyKill_LegacyStringPathUnchanged(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		want   string
	}{
		{"poison", "ten or more poison counters (CR 704.5c)", "poison"},
		{"commander", "21+ commander damage from X (CR 704.6c)", "commander"},
		{"mill", "drew from empty library (CR 704.5b)", "mill"},
		{"concession_legacy_misread", "concession", "mill"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs := s2Game(t)
			gs.Seats[2].Lost = true
			gs.Seats[2].LossReason = c.reason
			gameengine.HandleSeatElimination(gs, 2)
			if got := ClassifyKill(gs, 3); got != c.want {
				t.Errorf("legacy ClassifyKill = %q, want %q", got, c.want)
			}
		})
	}
}

// TestClassifyKillFinal_Timeout pins the turn-cap shape.
func TestClassifyKillFinal_Timeout(t *testing.T) {
	gs := s2Game(t)
	gs.Turn = 50
	fin := ClassifyKillFinal(gs, 0, 50)
	if fin.Method != "timeout" {
		t.Errorf("Method = %q, want timeout", fin.Method)
	}
	if fin.VictimSeat != -1 || fin.KillerSeat != -1 {
		t.Errorf("timeout shape must not attribute victim/killer; got %d/%d", fin.VictimSeat, fin.KillerSeat)
	}
}

// TestSeedBinary_ConcessionDrawRoundTrip pins the append-only enum
// extension: the new methods survive the 1-byte encode/decode.
func TestSeedBinary_ConcessionDrawRoundTrip(t *testing.T) {
	for _, m := range []string{"concession", "draw"} {
		if got := killMethodString(killMethodEnum(m)); got != m {
			t.Errorf("round-trip(%q) = %q", m, got)
		}
	}
	// Old values keep their bytes (persisted records).
	if killMethodEnum("combat") != 0 || killMethodEnum("timeout") != 5 {
		t.Error("legacy enum values renumbered — persisted seed records would misdecode")
	}
}
