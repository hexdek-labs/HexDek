package gameengine

import "sort"

// mvl.go — the Minimize-Value-Lost (MVL) chooser, the cross-cutting cost-choice
// system. Many mechanics ask the same question: "give up the least-valuable
// resources that still satisfy a requirement." Sacrifice N, discard N,
// tap-for-Teamwork-N (CR §702.194), convoke's which-creatures, and the CBC
// pay-vs-sacrifice are all instances. Historically each seam re-implemented its
// own argmin over the Hat's evaluator; this is the shared primitive they can
// delegate to.
//
// STRATEGY STAYS PER-PLAYER: the caller supplies valueFn — typically the
// calling Hat's archetype-weighted per-permanent evaluator — so an aristocrats
// deck, a voltron deck, and a Teamwork deck each compute "value lost"
// differently. This helper only runs the comparison; it never decides what
// matters.

// ChooseLeastValuableSet picks a minimal-value subset of candidates whose
// summed contribution meets or exceeds need.
//
//   - contribFn: how much a candidate counts toward `need` (power for Teamwork;
//     return 1 for count-based costs like "sacrifice N").
//   - valueFn: the loss from giving up that candidate (lower = cheaper to give
//     up). Pass the Hat's evaluator so the choice reflects the deck's strategy.
//
// Returns the chosen subset, or nil if the candidates cannot meet `need`
// (fail-closed — the caller then declines an optional cost, or aborts a
// mandatory one, rather than committing a partial/illegal payment).
//
// Heuristic: greedy by value-per-contribution ascending — commit the most
// value-efficient candidates first until the threshold clears. Optimal for
// count-based costs (contrib == 1 → cheapest-value first) and near-optimal for
// power thresholds; not a full min-value knapsack, which can be swapped in
// later behind this same signature without touching callers.
func ChooseLeastValuableSet(candidates []*Permanent, need int, contribFn func(*Permanent) int, valueFn func(*Permanent) int) []*Permanent {
	if need <= 0 || contribFn == nil || valueFn == nil {
		return nil
	}
	type entry struct {
		p        *Permanent
		val, con int
	}
	var pool []entry
	total := 0
	for _, p := range candidates {
		if p == nil {
			continue
		}
		con := contribFn(p)
		if con <= 0 {
			continue
		}
		pool = append(pool, entry{p, valueFn(p), con})
		total += con
	}
	if total < need {
		return nil // requirement unreachable — fail closed
	}
	// value-per-contribution ascending: val_i/con_i < val_j/con_j, cross-
	// multiplied to avoid floats. Ties broken by higher contribution (reach
	// the floor with fewer commitments).
	sort.SliceStable(pool, func(i, j int) bool {
		li := pool[i].val * pool[j].con
		lj := pool[j].val * pool[i].con
		if li != lj {
			return li < lj
		}
		return pool[i].con > pool[j].con
	})
	var chosen []*Permanent
	got := 0
	for _, e := range pool {
		if got >= need {
			break
		}
		chosen = append(chosen, e.p)
		got += e.con
	}
	return chosen
}
