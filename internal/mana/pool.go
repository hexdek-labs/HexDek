// Pool tracks per-color available mana for a single player.
package mana

import "fmt"

// Pool is a per-color mana count. Negative values are not permitted.
type Pool struct {
	W int
	U int
	B int
	R int
	G int
	C int // colorless
}

// Add increments the pool by another pool's values.
func (p *Pool) Add(other Pool) {
	p.W += other.W
	p.U += other.U
	p.B += other.B
	p.R += other.R
	p.G += other.G
	p.C += other.C
}

// Total returns the sum of mana of all colors in the pool.
func (p Pool) Total() int {
	return p.W + p.U + p.B + p.R + p.G + p.C
}

// Empty zeroes the pool. Mana pools empty between phases in MTG; the game
// state machine calls this at phase boundaries.
func (p *Pool) Empty() {
	p.W = 0
	p.U = 0
	p.B = 0
	p.R = 0
	p.G = 0
	p.C = 0
}

// CanPay returns true if the pool contains enough mana to satisfy the cost.
//
// Colored requirements must be satisfied with the matching color. The
// generic portion can be paid with any color. We compute it greedily: try
// to satisfy each colored pip first, then check that what remains is at
// least the generic cost.
//
// For X-cost spells, pass the resolved X value (Cost.Variable * x).
func (p Pool) CanPay(cost Cost, xValue int) bool {
	// Track remaining colored requirements.
	required := map[Color]int{
		White:     cost.Colored[White],
		Blue:      cost.Colored[Blue],
		Black:     cost.Colored[Black],
		Red:       cost.Colored[Red],
		Green:     cost.Colored[Green],
		Colorless: cost.Colored[Colorless],
	}
	available := map[Color]int{
		White: p.W, Blue: p.U, Black: p.B, Red: p.R, Green: p.G, Colorless: p.C,
	}

	for color, need := range required {
		if available[color] < need {
			return false
		}
		available[color] -= need
	}

	// Generic cost can be paid with any remaining mana of any color (except
	// Colorless mana is fungible into generic too, since the C symbol just
	// means "specifically uncolored mana required" which is the rare case).
	totalGeneric := cost.Generic + xValue*cost.Variable
	totalAvailable := available[White] + available[Blue] + available[Black] +
		available[Red] + available[Green] + available[Colorless]
	return totalAvailable >= totalGeneric
}

// Pay deducts the cost from the pool, choosing colored mana for colored
// pips and any-color mana for generic. Returns an error if the pool can't
// satisfy the cost (caller should CanPay first).
//
// For generic cost, we deduct from colors in priority order
// (W → U → B → R → G → C) which is conventional but arbitrary. The game
// loop should expose explicit "tap this land" choice for advanced players.
func (p *Pool) Pay(cost Cost, xValue int) error {
	if !p.CanPay(cost, xValue) {
		return fmt.Errorf("insufficient mana to pay cost %s (pool: %s)", cost.String(), p.String())
	}

	p.W -= cost.Colored[White]
	p.U -= cost.Colored[Blue]
	p.B -= cost.Colored[Black]
	p.R -= cost.Colored[Red]
	p.G -= cost.Colored[Green]
	p.C -= cost.Colored[Colorless]

	remaining := cost.Generic + xValue*cost.Variable
	for _, slot := range []*int{&p.W, &p.U, &p.B, &p.R, &p.G, &p.C} {
		if remaining == 0 {
			break
		}
		take := *slot
		if take > remaining {
			take = remaining
		}
		*slot -= take
		remaining -= take
	}
	return nil
}

// String returns a human-readable rendering of the pool, e.g. "U:2 B:1".
func (p Pool) String() string {
	if p.Total() == 0 {
		return "(empty)"
	}
	out := ""
	for _, named := range []struct {
		Label string
		Count int
	}{
		{"W", p.W}, {"U", p.U}, {"B", p.B},
		{"R", p.R}, {"G", p.G}, {"C", p.C},
	} {
		if named.Count > 0 {
			if out != "" {
				out += " "
			}
			out += fmt.Sprintf("%s:%d", named.Label, named.Count)
		}
	}
	return out
}
