package judge

import (
	"strings"
	"testing"
)

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
		"turn_capped":      false,
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

// The context fields are necessary but NOT sufficient: the server log
// renders violations through ValidationViolation.String(), which emits
// severity/surface/name/seat/message and DROPS Context. A first version
// of r67 put the evidence only in Context and shipped to production,
// where the log line came out exactly as uninformative as before. This
// pins the evidence into the MESSAGE, which is the surface that
// actually reaches the log a human reads.
func TestStateIntegrityR67_EvidenceSurvivesStringRendering(t *testing.T) {
	g := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		seat704(0, -27, false),
		seat704(1, 0, true), seat704(2, 0, true), seat704(3, 0, true),
	}}
	var line string
	for _, v := range CheckStateIntegrity(g) {
		if v.Name == "704.5a" {
			line = v.String()
		}
	}
	if line == "" {
		t.Fatal("expected a 704.5a violation")
	}
	for _, want := range []string{
		"ended=true", "winner=true", "turn_capped=false", "alive=1",
		"sole_survivor=true", "shield=false", "sba_emitted=false",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("rendered log line is missing %q — it reads: %s", want, line)
		}
	}
}

// r67c — the turn-cap adjudicator (tournament/outcome_canon.go
// AdjudicateGameEnd) ends a capped game by crowning the living seat
// with the HIGHEST life, and never checks that life is above 0. It is
// the prime suspect for a recorded winner at <=0 life, so the
// violation must say outright which path produced the state rather
// than leaving it to be inferred from the absence of an unrelated
// showmatch log line.
func TestStateIntegrityR67c_TurnCappedIsReportedInTheLogLine(t *testing.T) {
	g := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true, TurnCapped: true, Seats: []SeatSnapshot{
		seat704(0, -44, false),
		seat704(1, 0, true), seat704(2, 0, true), seat704(3, 0, true),
	}}
	var line string
	for _, v := range CheckStateIntegrity(g) {
		if v.Name == "704.5a" {
			line = v.String()
		}
	}
	if !strings.Contains(line, "turn_capped=true") {
		t.Errorf("turn-cap origin missing from the rendered line: %s", line)
	}
}

// r68 — "unknown" must stay distinct from a number, and especially
// from 0. A seat whose decision-point life was never captured is not
// the same as a seat that was AT zero when the outcome was decided,
// and conflating them would answer the exact question this field
// exists to settle.
func TestStateIntegrityR68_UnknownLifeAtEndIsNotZero(t *testing.T) {
	unset := GameSnapshot{TotalSeats: 2, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		{Seat: 0, Life: -3, Lost: false}, // LifeAtEndKnown false
		{Seat: 1, Life: 0, Lost: true},
	}}
	atZero := GameSnapshot{TotalSeats: 2, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		{Seat: 0, Life: -3, Lost: false, LifeAtEnd: 0, LifeAtEndKnown: true},
		{Seat: 1, Life: 0, Lost: true},
	}}

	lineOf := func(g GameSnapshot) string {
		for _, v := range CheckStateIntegrity(g) {
			if v.Name == "704.5a" {
				return v.String()
			}
		}
		return ""
	}
	u, z := lineOf(unset), lineOf(atZero)
	if !strings.Contains(u, "life_at_end=unknown") {
		t.Errorf("uncaptured life must render as unknown, got: %s", u)
	}
	if !strings.Contains(z, "life_at_end=0") {
		t.Errorf("a captured 0 must render as 0, got: %s", z)
	}
	if u == z {
		t.Error("unknown and 0 rendered identically — the distinction this field exists for is lost")
	}
}

// A seat that was comfortably alive at the decision point and only went
// negative afterwards is the RESIDUE shape; one that was already at or
// below 0 when the outcome was decided is the alarming shape. Both must
// be readable straight off the line.
func TestStateIntegrityR68_ResidueAndAlreadyDeadAreDistinguishable(t *testing.T) {
	mk := func(lifeAtEnd int) string {
		g := GameSnapshot{TotalSeats: 2, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
			{Seat: 0, Life: -44, Lost: false, LifeAtEnd: lifeAtEnd, LifeAtEndKnown: true},
			{Seat: 1, Life: 0, Lost: true},
		}}
		for _, v := range CheckStateIntegrity(g) {
			if v.Name == "704.5a" {
				return v.String()
			}
		}
		return ""
	}
	if !strings.Contains(mk(17), "life_at_end=17") {
		t.Error("residue shape must report the healthy decision-point life")
	}
	if !strings.Contains(mk(-44), "life_at_end=-44") {
		t.Error("already-dead shape must report the negative decision-point life")
	}
}
