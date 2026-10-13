package judge

import "testing"

// r67 — pins the TRIAGE CONTEXT on the §704.5a violation.
//
// Live grinder output carried 190 of these at critical severity with a
// message that said only "seat N has -M life but is not marked lost".
// That is not triageable: it cannot distinguish a genuine SBA miss from
// the two known-benign shapes (the game ended in the same window, so the
// final SBA pass never ran; or the seat is the sole survivor winning at
// <=0 life under a shield). These tests pin the evidence fields so the
// class can actually be classified from a log line.
//
// Severity is deliberately NOT asserted to have changed — this pass is
// diagnostic only.

func ctx704(vs []ValidationViolation) map[string]interface{} {
	for _, v := range vs {
		if v.Name == "704.5a" {
			return v.Context
		}
	}
	return nil
}

func seat704(idx, life int, lost bool) SeatSnapshot {
	return SeatSnapshot{Seat: idx, Life: life, Lost: lost, CommanderDamage: map[string]int{}}
}

// The field that nearly shipped wrong: `alive` is a RUNNING counter
// accumulated inside the same loop that performs the §704.5a check, so
// reading it there yields "seats walked so far", not the survivor count.
// Here the only negative-life seat is seat 0 — the FIRST seat visited —
// so a partial count would report 1 ("sole survivor") when three other
// seats are in fact still alive.
func TestStateIntegrityR67_AliveCountIsTotalNotPartial(t *testing.T) {
	g := GameSnapshot{TotalSeats: 4, Ended: false, HasWinner: false, Seats: []SeatSnapshot{
		seat704(0, -5, false), // negative, checked first
		seat704(1, 20, false),
		seat704(2, 20, false),
		seat704(3, 20, false),
	}}
	c := ctx704(CheckStateIntegrity(g))
	if c == nil {
		t.Fatal("expected a 704.5a violation for a seat at -5 life")
	}
	if got := c["alive_seats"]; got != 4 {
		t.Errorf("alive_seats = %v, want 4 — a partial in-loop count would report 1", got)
	}
	if got := c["sole_survivor"]; got != false {
		t.Errorf("sole_survivor = %v, want false — three opponents are still alive", got)
	}
}

// The sole-survivor shape: a seat WINS at negative life. This is the
// case the severity branch is meant to downgrade when a shield is
// detected; the context must make it identifiable even when it is not.
func TestStateIntegrityR67_SoleSurvivorAtNegativeLifeIsIdentifiable(t *testing.T) {
	g := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		seat704(0, -9, false), // survivor, negative life, NO shield detected
		seat704(1, 0, true),
		seat704(2, 0, true),
		seat704(3, 0, true),
	}}
	c := ctx704(CheckStateIntegrity(g))
	if c == nil {
		t.Fatal("expected a 704.5a violation")
	}
	for field, want := range map[string]interface{}{
		"ended":            true,
		"has_winner":       true,
		"alive_seats":      1,
		"sole_survivor":    true,
		"cant_lose_shield": false,
		"life":             -9,
	} {
		if got := c[field]; got != want {
			t.Errorf("context[%q] = %v, want %v", field, got, want)
		}
	}
}

// The ended-in-the-same-window shape: StateBasedActions returns early
// once the game has ended (sba.go), so the pass that would mark the seat
// never runs. Must be distinguishable from a mid-game miss.
func TestStateIntegrityR67_EndedFlagDistinguishesFromMidGameMiss(t *testing.T) {
	mid := GameSnapshot{TotalSeats: 4, Ended: false, HasWinner: false, Seats: []SeatSnapshot{
		seat704(0, -3, false), seat704(1, 20, false), seat704(2, 20, false), seat704(3, 20, false),
	}}
	ended := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		seat704(0, -3, false), seat704(1, 20, false), seat704(2, 20, false), seat704(3, 20, false),
	}}

	cm, ce := ctx704(CheckStateIntegrity(mid)), ctx704(CheckStateIntegrity(ended))
	if cm == nil || ce == nil {
		t.Fatal("expected a 704.5a violation in both snapshots")
	}
	if cm["ended"] != false || ce["ended"] != true {
		t.Errorf("ended must mirror the snapshot: mid=%v ended=%v", cm["ended"], ce["ended"])
	}
	// A mid-game miss is the shape that is genuinely alarming; it must
	// not be silently downgraded by this diagnostic pass.
	if sev := severityOf704(CheckStateIntegrity(mid)); sev != SeverityCritical {
		t.Errorf("a mid-game §704.5a miss must stay critical, got %v", sev)
	}
}

func severityOf704(vs []ValidationViolation) string {
	for _, v := range vs {
		if v.Name == "704.5a" {
			return v.Severity
		}
	}
	return SeverityInfo
}
