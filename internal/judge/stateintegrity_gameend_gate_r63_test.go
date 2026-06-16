package judge

import "testing"

// stateintegrity_gameend_gate_r63_test.go — firespot r63: pins the §104.2a
// game_end exactly-one-winner gate (stateintegrity.go).
//
// The grinder's feynman bracket check was reported firing "0 of 4 seats
// lost (expected 3)" and "2 of 4 seats lost (expected 3)". Those are
// §104.2c win-by-effect / premature-end shapes: a "you win the game" effect
// ends the game with a winner but leaves opponents un-marked-Lost, so >1
// seat is still alive. The `alive == 1` gate (added 44ef5859) deliberately
// suppresses these — the unconditional form produced ~8,764 fishtank FPs.
// These tests pin that gate so the FP class cannot regress, and document
// that the message can ONLY fire when the snapshot has fewer seats than
// TotalSeats (nil/vacated seats) — never in a normal 4-non-nil-seat game.

func gameEndSeat(idx, life int, lost bool) SeatSnapshot {
	return SeatSnapshot{Seat: idx, Life: life, Lost: lost, CommanderDamage: map[string]int{}}
}

func gameEndHitCount(vs []ValidationViolation) int {
	n := 0
	for _, v := range vs {
		if v.Name == "game_end" {
			n++
		}
	}
	return n
}

// §104.2c win-by-effect: winner Won (!Lost), all 3 opponents still alive
// (the engine doesn't mark opponents Lost on a win-effect end). Server
// shape "0 of 4 lost (expected 3)" — must be GATED (alive=4 != 1).
func TestGameEndGate_WinEffect_AllOpponentsAlive_NoFP(t *testing.T) {
	g := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		gameEndSeat(0, 12, false), // winner
		gameEndSeat(1, 8, false),
		gameEndSeat(2, 15, false),
		gameEndSeat(3, 3, false),
	}}
	if hits := gameEndHitCount(CheckStateIntegrity(g)); hits != 0 {
		t.Fatalf("win-effect end with all opponents alive must NOT fire game_end (alive!=1); got %d", hits)
	}
}

// Win with 2 eliminated + 1 still alive. Server shape "2 of 4 lost" — must
// be GATED (alive=2 != 1).
func TestGameEndGate_WinEffect_TwoLostOneAlive_NoFP(t *testing.T) {
	g := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		gameEndSeat(0, 12, false), // winner
		gameEndSeat(1, 0, true),
		gameEndSeat(2, 0, true),
		gameEndSeat(3, 5, false), // still alive
	}}
	if hits := gameEndHitCount(CheckStateIntegrity(g)); hits != 0 {
		t.Fatalf("win with 2 lost + 2 alive must NOT fire game_end (alive!=1); got %d", hits)
	}
}

// True last-seat-standing: 3 lost, 1 alive → clean (lost == expected).
func TestGameEndGate_LastSeatStanding_Clean(t *testing.T) {
	g := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		gameEndSeat(0, 12, false),
		gameEndSeat(1, 0, true),
		gameEndSeat(2, 0, true),
		gameEndSeat(3, 0, true),
	}}
	if hits := gameEndHitCount(CheckStateIntegrity(g)); hits != 0 {
		t.Fatalf("clean last-seat-standing (3 lost, 1 alive) must not fire game_end; got %d", hits)
	}
}

// The ONLY way the message fires with the gate: a snapshot missing seats
// relative to TotalSeats (nil/vacated). Pins that this is the sole firing
// path — the diagnostic for any future "N of 4" report.
func TestGameEndGate_FiresOnlyWithMissingSnapshotSeats(t *testing.T) {
	g := GameSnapshot{TotalSeats: 4, Ended: true, HasWinner: true, Seats: []SeatSnapshot{
		gameEndSeat(0, 12, false), // winner only; 3 seats absent from snapshot
	}}
	if hits := gameEndHitCount(CheckStateIntegrity(g)); hits != 1 {
		t.Fatalf("missing-snapshot-seats case should fire exactly one game_end; got %d", hits)
	}
}
