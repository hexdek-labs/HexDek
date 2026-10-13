package judge

import "testing"

// r69 — the post-end residue carve-out, and the three shapes it must
// NOT touch.
//
// Measured on the live grinder before this was written: every §704.5a
// winner-at-negative-life sample had the winner comfortably alive at
// the decision point (final -5 with life_at_end=5; final -1 with
// life_at_end=9), going negative only after the game was over. That is
// residue and the recorded result is correct.
//
// The danger here is over-suppression, not under-suppression. These
// tests exist mostly to pin what stays CRITICAL.

func sev704(g GameSnapshot) string {
	for _, v := range CheckStateIntegrity(g) {
		if v.Name == "704.5a" {
			return v.Severity
		}
	}
	return ""
}

func residueSnap(ended bool, finalLife, lifeAtEnd int, known bool) GameSnapshot {
	return GameSnapshot{TotalSeats: 4, Ended: ended, HasWinner: ended, Seats: []SeatSnapshot{
		{Seat: 0, Life: finalLife, Lost: false, LifeAtEnd: lifeAtEnd, LifeAtEndKnown: known},
		{Seat: 1, Life: 0, Lost: true},
		{Seat: 2, Life: 0, Lost: true},
		{Seat: 3, Life: 0, Lost: true},
	}}
}

// The shape actually observed in production: alive at the decision
// point, negative afterwards.
func TestResidueR69_HealthyAtDecisionPointIsNotCritical(t *testing.T) {
	if got := sev704(residueSnap(true, -5, 5, true)); got != SeverityInfo {
		t.Errorf("post-end residue should be info, got %q", got)
	}
	if got := sev704(residueSnap(true, -1, 9, true)); got != SeverityInfo {
		t.Errorf("post-end residue should be info, got %q", got)
	}
}

// THE ALARMING SHAPE. The engine declared a winner who was already at
// or below 0 when the outcome was decided — a real CR §104.4a
// draw-detection miss. This must never be downgraded.
func TestResidueR69_AlreadyDeadAtDecisionPointStaysCritical(t *testing.T) {
	for _, lifeAtEnd := range []int{0, -1, -44} {
		if got := sev704(residueSnap(true, -44, lifeAtEnd, true)); got != SeverityCritical {
			t.Errorf("life_at_end=%d: a winner who had already lost must stay critical, got %q",
				lifeAtEnd, got)
		}
	}
}

// A mid-game miss is the original reason this check exists. The game
// has not ended, so there is no residue explanation available.
func TestResidueR69_MidGameMissStaysCritical(t *testing.T) {
	g := residueSnap(false, -3, 20, true)
	if got := sev704(g); got != SeverityCritical {
		t.Errorf("mid-game §704.5a miss must stay critical, got %q", got)
	}
}

// No decision-point data means an end path that bypasses CheckEnd.
// Absence of evidence is not evidence of residue.
func TestResidueR69_UnknownDecisionPointStaysCritical(t *testing.T) {
	if got := sev704(residueSnap(true, -7, 0, false)); got != SeverityCritical {
		t.Errorf("unknown decision-point life must stay critical, got %q", got)
	}
}

// The shield downgrade predates this and must survive it: a seat
// winning at <=0 under a live "can't lose the game" effect is
// rules-correct regardless of the decision-point life.
func TestResidueR69_ShieldDowngradeStillApplies(t *testing.T) {
	g := residueSnap(true, -11, -11, true)
	g.Seats[0].CantLoseShield = true
	if got := sev704(g); got != SeverityInfo {
		t.Errorf("a shielded win at negative life should be info, got %q", got)
	}
}
