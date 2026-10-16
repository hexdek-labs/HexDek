package gameengine

// countCardsForSweep returns an upper bound on the number of distinct
// InstanceIDs collectPresentInstanceIDsForSweep will insert.
//
// r149: it is a capacity hint, nothing more. Over-counting (a nil card, an
// empty InstanceID, the same ID in two places) costs a slightly larger map
// and never changes the result; under-counting costs a rehash. It walks
// slice LENGTHS only, never elements, so it is effectively free next to the
// insertion loop it is sizing.
//
// It deliberately mirrors the zone list in collectPresentInstanceIDsForSweep.
// If a zone is added there and not here the hint is merely low — the sweep
// stays correct, which is why this is a hint and not a contract.
func countCardsForSweep(gs *GameState) int {
	if gs == nil {
		return 0
	}
	n := 0
	for _, s := range gs.Seats {
		if s == nil || s.LeftGame {
			continue
		}
		n += len(s.Library) + len(s.Hand) + len(s.Graveyard) + len(s.Exile) +
			len(s.CommandZone) + len(s.ForetellExile)
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			n += 1 + len(p.MergedCardPtrs)
		}
		if s.Companion != nil {
			n++
		}
	}
	n += len(gs.ZoneCastGrants)
	return n
}
