package judge

import "testing"

// Pins for the STATE-INTEGRITY end-of-game checks (fold of the Feynman
// §704.5 SBA-compliance + §104.2a exactly-one-winner originals).

func siCollect(t *testing.T) (*[]ValidationViolation, func()) {
	t.Helper()
	var got []ValidationViolation
	un := RegisterSink(func(v ValidationViolation) { got = append(got, v) })
	return &got, un
}

func TestStateIntegrity_LifePoisonCommanderDamage(t *testing.T) {
	routed, done := siCollect(t)
	defer done()

	vs := CheckStateIntegrity(GameSnapshot{
		TotalSeats: 4,
		Seats: []SeatSnapshot{
			{Seat: 0, Life: -3},                                                  // 704.5a critical
			{Seat: 1, Life: 40, PoisonCounters: 11},                              // 704.5c
			{Seat: 2, Life: 40, CommanderDamage: map[string]int{"Edgar": 23}},    // 704.5v
			{Seat: 3, Life: -5, Lost: true, PoisonCounters: 12},                  // lost: no violations
		},
	})

	want := map[string]int{"704.5a": 0, "704.5c": 1, "704.5v": 2}
	if len(vs) != 3 {
		t.Fatalf("expected 3 violations, got %d: %v", len(vs), vs)
	}
	for _, v := range vs {
		seat, ok := want[v.Name]
		if !ok {
			t.Errorf("unexpected violation %q", v.Name)
			continue
		}
		if v.Seat != seat {
			t.Errorf("%s attributed to seat %d, want %d", v.Name, v.Seat, seat)
		}
		if v.Dimension != DimensionStateIntegrity || v.Surface != SurfaceFeynman {
			t.Errorf("%s must carry feynman/state_integrity tags; got %s/%s", v.Name, v.Surface, v.Dimension)
		}
	}
	if len(*routed) != 3 {
		t.Errorf("all violations must route through LogViolation; routed %d", len(*routed))
	}
}

func TestStateIntegrity_CantLoseShieldDowngradesToInfo(t *testing.T) {
	_, done := siCollect(t)
	defer done()

	vs := CheckStateIntegrity(GameSnapshot{
		TotalSeats: 2,
		Seats: []SeatSnapshot{
			{Seat: 0, Life: -2, CantLoseShield: true}, // Platinum Angel class
			{Seat: 1, Life: 40, Lost: false},
		},
	})
	if len(vs) != 1 || vs[0].Severity != SeverityInfo {
		t.Fatalf("shielded life<=0 must downgrade to info; got %v", vs)
	}
}

// Regression: seed 13230778 game 1323 — seat 3 WON as the last seat
// standing at -11 life under a live "can't lose the game" shield
// (Platinum Angel class). The §704.5a SBA's shield branch sets
// SBALossEmitted while declining to mark Lost, so a shielded seat is
// (CantLoseShield=true, SBALossEmitted=true). That combination must stay
// INFO — the old `!SBALossEmitted && CantLoseShield` gate wrongly
// re-escalated it to a critical state-integrity violation.
func TestStateIntegrity_ShieldedWinnerWithSBAEmittedStaysInfo(t *testing.T) {
	_, done := siCollect(t)
	defer done()

	vs := CheckStateIntegrity(GameSnapshot{
		TotalSeats: 4, Ended: true, HasWinner: true,
		Seats: []SeatSnapshot{
			{Seat: 0, Life: -4, Lost: true},
			{Seat: 1, Life: -1, Lost: true},
			{Seat: 2, Life: 0, Lost: true},
			// The shielded winner at <=0 life, SBA already processed it.
			{Seat: 3, Life: -11, Lost: false, CantLoseShield: true, SBALossEmitted: true},
		},
	})
	for _, v := range vs {
		if v.Name == "704.5a" && v.Seat == 3 && v.Severity != SeverityInfo {
			t.Fatalf("shielded winner at <=0 life (SBALossEmitted=true) must be info, got %s", v.Severity)
		}
		if v.Severity == SeverityCritical {
			t.Fatalf("unexpected critical violation for a clean shielded-winner end: %+v", v)
		}
	}
}

// A seat at <=0 life with NO shield is still a real bug even if the SBA
// flag is set — the loss-marking genuinely failed.
func TestStateIntegrity_UnshieldedLifeZeroStaysCritical(t *testing.T) {
	_, done := siCollect(t)
	defer done()

	vs := CheckStateIntegrity(GameSnapshot{
		TotalSeats: 2,
		Seats: []SeatSnapshot{
			{Seat: 0, Life: -3, Lost: false, CantLoseShield: false, SBALossEmitted: true},
			{Seat: 1, Life: 40, Lost: false},
		},
	})
	found := false
	for _, v := range vs {
		if v.Name == "704.5a" && v.Seat == 0 {
			found = true
			if v.Severity != SeverityCritical {
				t.Fatalf("unshielded life<=0 must stay critical, got %s", v.Severity)
			}
		}
	}
	if !found {
		t.Fatal("expected a 704.5a violation for the unshielded -3-life seat")
	}
}

func TestStateIntegrity_ExactlyOneWinnerGating(t *testing.T) {
	_, done := siCollect(t)
	defer done()

	mk := func(ended, hasWinner bool, lostSeats int) GameSnapshot {
		g := GameSnapshot{TotalSeats: 4, Ended: ended, HasWinner: hasWinner}
		for i := 0; i < 4; i++ {
			g.Seats = append(g.Seats, SeatSnapshot{Seat: i, Life: 40, Lost: i < lostSeats})
		}
		return g
	}

	// Clean §104.2a win: 3 lost, 1 alive — no violation.
	if vs := CheckStateIntegrity(mk(true, true, 3)); len(vs) != 0 {
		t.Errorf("clean last-seat-standing must not flag: %v", vs)
	}
	// Turn-cap leader finish (not ended): 1 lost, 3 alive — gated out.
	if vs := CheckStateIntegrity(mk(false, true, 1)); len(vs) != 0 {
		t.Errorf("turn-cap finish must be gated out: %v", vs)
	}
	// Win-effect (alive>1): gated out.
	if vs := CheckStateIntegrity(mk(true, true, 1)); len(vs) != 0 {
		t.Errorf("win-effect shape (alive>1) must be gated out: %v", vs)
	}
	// Genuine marking bug: ended + winner + 1 alive but a vacated seat
	// makes lost==2 < expected 3.
	g := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true}
	g.Seats = []SeatSnapshot{
		{Seat: 0, Lost: true}, {Seat: 1, Lost: true}, {Seat: 2, Life: 40},
	}
	vs := CheckStateIntegrity(g)
	if len(vs) != 1 || vs[0].Name != "game_end" {
		t.Fatalf("loser-marking bug must flag game_end; got %v", vs)
	}
}
