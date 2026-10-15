package gameengine

// colored_cost_enforcement_r126.go — CR §202.1a coloured-pip enforcement
// on the real mana pool.
//
//	§202.1a "paying that mana cost requires MATCHING THE TYPE of any
//	         colored or colorless mana symbols as well as paying the
//	         generic mana indicated in the cost."
//	§107.4a "Colored mana in costs can be paid only with the appropriate
//	         color of mana."
//	§601.2h "Partial payments are not allowed."
//
// Until r126 the cast path compared a single integer — the reduced total
// cost against the pool's total — so a mono-green board could cast a
// {U}{U} counterspell. r124 is what made this fixable: it got the printed
// cost onto the runtime Card, where ParseCostRequirements could finally
// see it.
//
// SCOPE: this answers the COLOUR question only, never the quantity one.
// The caller's existing `cost > availableMana` check still owns magnitude,
// and deliberately so — CalculateTotalCost returns a cost that may have
// been REDUCED, while the printed string has not. Judging generic from the
// printed cost would over-demand against every cost reducer and wrongly
// block legal casts. Colour pips are not reduced by cost reduction, so
// they are safe to read off the printed cost.

// CanSupplyColoredPips reports whether `pool` can supply the coloured and
// colorless pips printed on `card`, per §202.1a.
//
// It is deliberately PERMISSIVE at every point of doubt, because a false
// negative here blocks a legal cast while a false positive merely leaves
// today's behaviour in place:
//
//   - no printed cost (tokens, engine-minted cards) -> true
//   - the pool holds restricted mana (§106.4a Food Chain, Powerstone) ->
//     true, because that mana may legally pay this very spell and this
//     function cannot tell
//   - Phyrexian and twobrid pips -> not required in colour (life and
//     generic pay them); CostRequirements.Soft carries the count
func CanSupplyColoredPips(pool *ColoredManaPool, card *Card) bool {
	if card == nil || pool == nil {
		return true
	}
	if card.ManaCostString == "" {
		return true
	}
	if len(pool.Restricted) > 0 {
		return true
	}
	req := ParseCostRequirements(card.ManaCostString)

	avail := [6]int{pool.W, pool.U, pool.B, pool.R, pool.G, pool.C}
	any := pool.Any

	// Pure pips first: only this colour, or Any, can pay them.
	for slot := 0; slot < 6; slot++ {
		need := req.Pure[slot] - req.Soft[slot]
		if need <= 0 {
			continue
		}
		use := need
		if avail[slot] < use {
			use = avail[slot]
		}
		avail[slot] -= use
		need -= use
		if need > any {
			return false
		}
		any -= need
	}

	// Hybrid pips: any colour in the mask, or Any. Satisfy the most
	// constrained mask first so a two-colour pip does not consume the
	// only source a one-colour pip needed.
	hybrids := append([]uint8(nil), req.Hybrid...)
	for len(hybrids) > 0 {
		best, bestCap := -1, 1<<30
		for i, mask := range hybrids {
			cap := 0
			for slot := 0; slot < 6; slot++ {
				if mask&slotToBit(slot) != 0 {
					cap += avail[slot]
				}
			}
			if cap < bestCap {
				best, bestCap = i, cap
			}
		}
		mask := hybrids[best]
		hybrids = append(hybrids[:best], hybrids[best+1:]...)

		paid := false
		for slot := 0; slot < 6 && !paid; slot++ {
			if mask&slotToBit(slot) != 0 && avail[slot] > 0 {
				avail[slot]--
				paid = true
			}
		}
		if paid {
			continue
		}
		if any <= 0 {
			return false
		}
		any--
	}
	return true
}
