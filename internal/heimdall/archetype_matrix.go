package heimdall

// ArchetypeMatrix is the all-pairs archetype-vs-archetype winrate
// matrix returned by /api/meta/archetype-vs-archetype.
//
// Archetypes is the canonical row/column key list — the index into
// this slice is the row/column index in Cells. Cells is a square
// 2-D slice of size len(Archetypes) × len(Archetypes); Cells[i][j]
// holds the winrate of Archetypes[i] when Archetypes[j] was also
// at the table.
//
// Cell semantics:
//   - Games: count of (game, seat) observations where the seat
//     piloted Archetypes[i] AND some other seat at the same game
//     piloted Archetypes[j]. Self-pairs (i == j) ARE counted —
//     two stax decks at the same table is a real matchup.
//   - Wins: subset of Games where the Archetypes[i] seat won.
//   - WinRate: Wins / Games, 0 when Games == 0.
//
// Symmetry: Cells[i][j].Games == Cells[j][i].Games is the typical
// case, but draws and third-party wins make Cells[i][j].Wins +
// Cells[j][i].Wins ≤ Cells[i][j].Games (i.e., not all games end
// with one of these two archetypes winning).
type ArchetypeMatrix struct {
	Archetypes []string              `json:"archetypes"`
	Cells      [][]ArchetypeMatrixCell `json:"cells"`
	TotalGames int                   `json:"total_games"` // number of distinct games walked
	TotalCells int                   `json:"total_cells"` // len(Archetypes) ^ 2
}

// ArchetypeMatrixCell is one cell in the matrix.
type ArchetypeMatrixCell struct {
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	WinRate float64 `json:"win_rate"`
}

// MatrixGameInput is one game's worth of seat archetypes for the
// pure-computation builder. The caller (hexapi handler) resolves
// each seat's deck_key → archetype before invoking BuildArchetypeMatrix.
//
// Seats whose archetype slug isn't in the canonical Archetypes list
// passed into BuildArchetypeMatrix are silently skipped — the matrix
// only surfaces known archetypes; "unknown" / unclassified seats
// don't pollute cells.
type MatrixGameInput struct {
	Seats []MatrixGameSeat
}

// MatrixGameSeat is one seat within a MatrixGameInput.
type MatrixGameSeat struct {
	Archetype string
	Won       bool
}

// CanonicalArchetypes is the 22-entry canonical archetype list used
// by the meta matrix. Mirrors the slugs in internal/hat/poker.go
// (kept duplicated here to avoid the heimdall ↔ hat dependency).
//
// The order is the same as the hat constant block, which is the
// order Freya's archetype classifier emits and the order the meta
// dashboard renders rows/columns in.
var CanonicalArchetypes = []string{
	"aggro",
	"midrange",
	"control",
	"ramp",
	"combo",
	"stax",
	"reanimator",
	"spellslinger",
	"tribal",
	"aristocrats",
	"selfmill",
	"enchantress",
	"artifacts",
	"lifegain",
	"voltron",
	"lands matter",
	"counters matter",
	"mill",
	"storm",
	"superfriends",
	"blink",
	"extra combats",
}

// BuildArchetypeMatrix builds the all-pairs matchup matrix from
// per-game seat lists. archetypes is the canonical row/column key
// list — pass CanonicalArchetypes for the standard 22.
//
// For each game, for each ordered (i, j) seat pair WHERE both seats'
// archetypes are in the canonical list, the cell
// matrix[arch_i][arch_j] gets Games+1 and (if seat i won) Wins+1.
// Self-archetype pairs (i and j are the same archetype) are
// permitted and counted — two "stax" decks at the same table is a
// valid mirror-matchup observation.
//
// The matrix is dense (every cell exists, with zeros when empty) so
// frontends can render a fixed-size grid without nil-guarding.
func BuildArchetypeMatrix(games []MatrixGameInput, archetypes []string) ArchetypeMatrix {
	n := len(archetypes)
	cells := make([][]ArchetypeMatrixCell, n)
	for i := range cells {
		cells[i] = make([]ArchetypeMatrixCell, n)
	}

	out := ArchetypeMatrix{
		Archetypes: append([]string(nil), archetypes...),
		Cells:      cells,
		TotalCells: n * n,
	}

	if n == 0 {
		return out
	}

	archIdx := make(map[string]int, n)
	for i, a := range archetypes {
		archIdx[a] = i
	}

	for _, g := range games {
		if len(g.Seats) < 2 {
			continue
		}
		// Resolve each seat's archetype index up front (skipping
		// unknown / non-canonical seats) so the inner loop is O(k²)
		// where k is the count of canonical seats in this game (at
		// most 4 in standard Commander).
		type seatRow struct {
			idx int
			won bool
		}
		rows := make([]seatRow, 0, len(g.Seats))
		for _, s := range g.Seats {
			if idx, ok := archIdx[s.Archetype]; ok {
				rows = append(rows, seatRow{idx: idx, won: s.Won})
			}
		}
		if len(rows) < 2 {
			// Game has fewer than 2 canonical-archetype seats — no
			// matrix cell would be populated, so it doesn't count
			// toward TotalGames either.
			continue
		}
		out.TotalGames++
		for i := range rows {
			for j := range rows {
				if i == j {
					continue
				}
				cell := &cells[rows[i].idx][rows[j].idx]
				cell.Games++
				if rows[i].won {
					cell.Wins++
				}
			}
		}
	}

	// Populate WinRate. The dense matrix layout means iterating every
	// cell is O(n²) ≈ 484 ops for the canonical 22 — trivial.
	for i := range cells {
		for j := range cells[i] {
			if cells[i][j].Games > 0 {
				cells[i][j].WinRate = float64(cells[i][j].Wins) / float64(cells[i][j].Games)
			}
		}
	}

	return out
}
