package heimdall

import (
	"math"
	"testing"
)

func matrixApprox(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func mkMatrixGame(seats ...MatrixGameSeat) MatrixGameInput {
	return MatrixGameInput{Seats: seats}
}
func seat(arch string, won bool) MatrixGameSeat {
	return MatrixGameSeat{Archetype: arch, Won: won}
}

// archIdx looks up the row/column index for an archetype in an
// ArchetypeMatrix. Test-only helper.
func archIdx(m ArchetypeMatrix, name string) int {
	for i, a := range m.Archetypes {
		if a == name {
			return i
		}
	}
	return -1
}

// TestBuildArchetypeMatrix_CanonicalIsTwentyTwo pins the canonical
// list size — this is the "22 × 22" the task spec calls out.
func TestBuildArchetypeMatrix_CanonicalIsTwentyTwo(t *testing.T) {
	if got := len(CanonicalArchetypes); got != 22 {
		t.Errorf("CanonicalArchetypes len = %d, want 22", got)
	}
	m := BuildArchetypeMatrix(nil, CanonicalArchetypes)
	if len(m.Cells) != 22 {
		t.Errorf("rows = %d, want 22", len(m.Cells))
	}
	for i := range m.Cells {
		if len(m.Cells[i]) != 22 {
			t.Errorf("row[%d] cols = %d, want 22", i, len(m.Cells[i]))
		}
	}
	if m.TotalCells != 22*22 {
		t.Errorf("TotalCells = %d, want %d", m.TotalCells, 22*22)
	}
}

// TestBuildArchetypeMatrix_EmptyInput pins the zero-game shape: every
// cell is zero-valued (non-nil), no panics.
func TestBuildArchetypeMatrix_EmptyInput(t *testing.T) {
	m := BuildArchetypeMatrix(nil, CanonicalArchetypes)
	if m.TotalGames != 0 {
		t.Errorf("TotalGames = %d, want 0", m.TotalGames)
	}
	for i := range m.Cells {
		for j := range m.Cells[i] {
			if m.Cells[i][j].Games != 0 || m.Cells[i][j].Wins != 0 || m.Cells[i][j].WinRate != 0 {
				t.Errorf("cells[%d][%d] = %+v, want zero", i, j, m.Cells[i][j])
			}
		}
	}
}

// TestBuildArchetypeMatrix_BasicPairCounts pins the per-pair counting:
// in one 4-seat game with archetypes A,B,C,D where A wins,
// A's row gets +1 game vs each of B, C, D and +1 win in each;
// B's row gets +1 game vs A, C, D with 0 wins; etc.
func TestBuildArchetypeMatrix_BasicPairCounts(t *testing.T) {
	games := []MatrixGameInput{mkMatrixGame(
		seat("control", true),  // winner
		seat("combo", false),
		seat("voltron", false),
		seat("aristocrats", false),
	)}
	m := BuildArchetypeMatrix(games, CanonicalArchetypes)

	ic := archIdx(m, "control")
	icc := archIdx(m, "combo")
	iv := archIdx(m, "voltron")
	ia := archIdx(m, "aristocrats")

	// control row: 3 games (one vs each opponent), 3 wins
	for _, opp := range []int{icc, iv, ia} {
		c := m.Cells[ic][opp]
		if c.Games != 1 || c.Wins != 1 || !matrixApprox(c.WinRate, 1.0) {
			t.Errorf("cells[control][%s] = %+v, want 1g/1w", m.Archetypes[opp], c)
		}
	}
	// combo row vs control: 1 game, 0 wins
	c := m.Cells[icc][ic]
	if c.Games != 1 || c.Wins != 0 || c.WinRate != 0 {
		t.Errorf("cells[combo][control] = %+v, want 1g/0w/0wr", c)
	}
	// combo row vs voltron: 1 game, 0 wins (neither won this game)
	c = m.Cells[icc][iv]
	if c.Games != 1 || c.Wins != 0 {
		t.Errorf("cells[combo][voltron] = %+v, want 1g/0w", c)
	}
	if m.TotalGames != 1 {
		t.Errorf("TotalGames = %d, want 1", m.TotalGames)
	}
}

// TestBuildArchetypeMatrix_SelfMirrorCounted pins that two seats with
// the same archetype contribute to the diagonal cell.
func TestBuildArchetypeMatrix_SelfMirrorCounted(t *testing.T) {
	// 4-seat game: two control decks (one wins), and two voltrons.
	games := []MatrixGameInput{mkMatrixGame(
		seat("control", true),
		seat("control", false),
		seat("voltron", false),
		seat("voltron", false),
	)}
	m := BuildArchetypeMatrix(games, CanonicalArchetypes)
	ic := archIdx(m, "control")
	iv := archIdx(m, "voltron")

	// Diagonal control vs control: 2 ordered pairs (seat0→seat1, seat1→seat0).
	// Seat 0 won so 1 of those 2 has Wins=1; seat 1 lost so the
	// other has Wins=0. Aggregated: 2 games, 1 win.
	cc := m.Cells[ic][ic]
	if cc.Games != 2 || cc.Wins != 1 || !matrixApprox(cc.WinRate, 0.5) {
		t.Errorf("cells[control][control] = %+v, want 2g/1w/0.5", cc)
	}
	// Diagonal voltron vs voltron: 2 games, 0 wins.
	vv := m.Cells[iv][iv]
	if vv.Games != 2 || vv.Wins != 0 {
		t.Errorf("cells[voltron][voltron] = %+v, want 2g/0w", vv)
	}
}

// TestBuildArchetypeMatrix_UnknownArchetypesSkipped pins that seats
// whose archetype isn't in the canonical list don't pollute the
// matrix — but other seats in the same game still pair off normally.
func TestBuildArchetypeMatrix_UnknownArchetypesSkipped(t *testing.T) {
	games := []MatrixGameInput{mkMatrixGame(
		seat("control", true),
		seat("combo", false),
		seat("bogus_homebrew", false),
		seat("", false),
	)}
	m := BuildArchetypeMatrix(games, CanonicalArchetypes)
	ic := archIdx(m, "control")
	icc := archIdx(m, "combo")

	// control × combo: 1 game, control won
	c := m.Cells[ic][icc]
	if c.Games != 1 || c.Wins != 1 {
		t.Errorf("cells[control][combo] = %+v, want 1g/1w", c)
	}
	// control should NOT have games vs the unknowns — verify by
	// summing control's row.
	rowSum := 0
	for j := range m.Cells[ic] {
		rowSum += m.Cells[ic][j].Games
	}
	if rowSum != 1 {
		t.Errorf("control row total Games = %d, want 1 (only combo opponent counted)", rowSum)
	}
}

// TestBuildArchetypeMatrix_GameWithFewerThanTwoCanonicalSeatsSkipped
// pins that a game where 0 or 1 seats have a canonical archetype
// doesn't contribute to TotalGames or any cell.
func TestBuildArchetypeMatrix_GameWithFewerThanTwoCanonicalSeatsSkipped(t *testing.T) {
	games := []MatrixGameInput{
		// 1 canonical seat only
		mkMatrixGame(
			seat("control", true),
			seat("bogus", false),
			seat("", false),
		),
		// 0 canonical seats
		mkMatrixGame(
			seat("nope", true),
			seat("alsonope", false),
		),
	}
	m := BuildArchetypeMatrix(games, CanonicalArchetypes)
	if m.TotalGames != 0 {
		t.Errorf("TotalGames = %d, want 0 (no game had ≥ 2 canonical seats)", m.TotalGames)
	}
	for i := range m.Cells {
		for j := range m.Cells[i] {
			if m.Cells[i][j].Games != 0 {
				t.Errorf("cells[%d][%d] = %+v, want zero", i, j, m.Cells[i][j])
			}
		}
	}
}

// TestBuildArchetypeMatrix_MultipleGamesAccumulate pins that counts
// accumulate across many games.
func TestBuildArchetypeMatrix_MultipleGamesAccumulate(t *testing.T) {
	games := []MatrixGameInput{
		mkMatrixGame(seat("control", true), seat("combo", false)),
		mkMatrixGame(seat("control", true), seat("combo", false)),
		mkMatrixGame(seat("control", false), seat("combo", true)),
		mkMatrixGame(seat("control", false), seat("combo", true)),
		mkMatrixGame(seat("control", false), seat("combo", false)), // draw / third-party
	}
	m := BuildArchetypeMatrix(games, CanonicalArchetypes)
	ic := archIdx(m, "control")
	icc := archIdx(m, "combo")

	cb := m.Cells[ic][icc]
	if cb.Games != 5 || cb.Wins != 2 || !matrixApprox(cb.WinRate, 0.4) {
		t.Errorf("cells[control][combo] = %+v, want 5g/2w/0.4", cb)
	}
	// Symmetry: combo's row vs control should also have 5 games but
	// only 2 wins (the games combo won).
	bc := m.Cells[icc][ic]
	if bc.Games != 5 || bc.Wins != 2 {
		t.Errorf("cells[combo][control] = %+v, want 5g/2w", bc)
	}
	if m.TotalGames != 5 {
		t.Errorf("TotalGames = %d, want 5", m.TotalGames)
	}
}

// TestBuildArchetypeMatrix_SymmetricGamesCount pins that for a single
// game, cells[A][B].Games == cells[B][A].Games (the pair is observed
// from both sides). Win counts diverge based on who won.
func TestBuildArchetypeMatrix_SymmetricGamesCount(t *testing.T) {
	games := []MatrixGameInput{mkMatrixGame(
		seat("control", true),
		seat("voltron", false),
		seat("midrange", false),
	)}
	m := BuildArchetypeMatrix(games, CanonicalArchetypes)
	for _, pair := range [][2]string{
		{"control", "voltron"},
		{"control", "midrange"},
		{"voltron", "midrange"},
	} {
		i := archIdx(m, pair[0])
		j := archIdx(m, pair[1])
		if m.Cells[i][j].Games != m.Cells[j][i].Games {
			t.Errorf("Games asymmetric for %s vs %s: %d vs %d",
				pair[0], pair[1], m.Cells[i][j].Games, m.Cells[j][i].Games)
		}
	}
}

// TestBuildArchetypeMatrix_CustomArchetypesList pins that a caller
// can pass their own archetype list (smaller / different ordering)
// and the matrix dimensions follow.
func TestBuildArchetypeMatrix_CustomArchetypesList(t *testing.T) {
	custom := []string{"alpha", "beta", "gamma"}
	games := []MatrixGameInput{mkMatrixGame(
		seat("alpha", true),
		seat("beta", false),
		seat("gamma", false),
	)}
	m := BuildArchetypeMatrix(games, custom)
	if len(m.Cells) != 3 || m.TotalCells != 9 {
		t.Errorf("dims: rows=%d cells=%d, want 3 / 9", len(m.Cells), m.TotalCells)
	}
	if m.Cells[0][1].Wins != 1 || m.Cells[0][2].Wins != 1 {
		t.Errorf("alpha row wins not set: %+v", m.Cells[0])
	}
}
