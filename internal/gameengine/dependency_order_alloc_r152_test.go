package gameengine

import "testing"

// r152Effects builds n mutually independent continuous effects — no layer
// pairing in effectDependsOn can fire, so DependencyOrder must take the
// "timestamp order is already correct" path. Shape borrowed from the
// existing TestLayerDependency_DependencyOrderUnit.
func r152Effects(n int) []*ContinuousEffect {
	out := make([]*ContinuousEffect, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, &ContinuousEffect{
			Layer:      LayerColor,
			Timestamp:  i + 1,
			SourcePerm: &Permanent{Card: &Card{Name: "ind"}, Flags: map[string]int{}},
		})
	}
	return out
}

// r152Dependent builds the §613.8 ability-strip shape: b removes abilities
// at layer 6 and its predicate matches a's source, so a depends on b and b
// must be applied first.
func r152Dependent() (a, b *ContinuousEffect) {
	pa := &Permanent{Card: &Card{Name: "A"}, Flags: map[string]int{}}
	pb := &Permanent{Card: &Card{Name: "B"}, Flags: map[string]int{}}
	a = &ContinuousEffect{Layer: LayerColor, Timestamp: 1, SourcePerm: pa}
	b = &ContinuousEffect{
		Layer: LayerAbility, Timestamp: 2, SourcePerm: pb,
		Predicate: func(gs *GameState, p *Permanent) bool { return p == pa },
	}
	return a, b
}

// THE HEADLINE. With no dependencies, DependencyOrder must not allocate at
// all. It used to build an n x n bool matrix as n separate allocations and
// then throw it away.
func TestDependencyOrderR152_NoDependenciesAllocatesNothing(t *testing.T) {
	gs := newFixtureGame(t)
	effects := r152Effects(24)

	got := testing.AllocsPerRun(50, func() {
		DependencyOrder(effects, gs)
	})
	if got != 0 {
		t.Fatalf("no-dependency path allocated %v times, want 0", got)
	}
}

// CONTROL — the pre-r152 implementation, verbatim, so the test states the
// number it is preventing instead of asserting against a bare 0. If this
// stops allocating, the test above proves nothing.
func r152EagerDependencyOrder(effects []*ContinuousEffect, gs *GameState) []*ContinuousEffect {
	if len(effects) <= 1 {
		return effects
	}
	n := len(effects)
	depends := make([][]bool, n)
	for i := range depends {
		depends[i] = make([]bool, n)
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			if effectDependsOn(effects[i], effects[j], gs) {
				depends[i][j] = true
			}
		}
	}
	hasDeps := false
	for i := 0; i < n && !hasDeps; i++ {
		for j := 0; j < n && !hasDeps; j++ {
			if depends[i][j] {
				hasDeps = true
			}
		}
	}
	if !hasDeps {
		return effects
	}
	return effects
}

func TestDependencyOrderR152_ControlEagerMatrixDoesAllocate(t *testing.T) {
	gs := newFixtureGame(t)
	effects := r152Effects(24)

	got := testing.AllocsPerRun(50, func() {
		r152EagerDependencyOrder(effects, gs)
	})
	// n row allocations + the outer [][]bool.
	if got < 24 {
		t.Fatalf("CONTROL DID NOT GO RED: the eager matrix allocated only %v times for n=24, so the zero-alloc assertion above is not measuring the matrix", got)
	}
	t.Logf("control allocates %v times per call at n=24 (one per row plus the outer slice)", got)
}

// The other direction — a REAL dependency must still reorder. The existing
// TestLayerDependency_DependencyOrderUnit only covers independent effects,
// so this path had no regression at all before r152.
func TestDependencyOrderR152_RealDependencyStillReorders(t *testing.T) {
	gs := newFixtureGame(t)
	a, b := r152Dependent()

	// Precondition: the dependency really is detected, or this is vacuous.
	if !effectDependsOn(a, b, gs) {
		t.Fatalf("precondition: a should depend on b")
	}

	ordered := DependencyOrder([]*ContinuousEffect{a, b}, gs)
	if len(ordered) != 2 {
		t.Fatalf("want 2 effects, got %d", len(ordered))
	}
	if ordered[0] != b || ordered[1] != a {
		t.Fatalf("dependency ignored: b must be applied before a, got [%v %v] want [b a]",
			ordered[0].SourcePerm.Card.Name, ordered[1].SourcePerm.Card.Name)
	}
}

// §613.8b — a circular dependency falls back to timestamp order rather than
// producing a partial or reordered result.
func TestDependencyOrderR152_CycleFallsBackToTimestampOrder(t *testing.T) {
	gs := newFixtureGame(t)
	pa := &Permanent{Card: &Card{Name: "A"}, Flags: map[string]int{}}
	pb := &Permanent{Card: &Card{Name: "B"}, Flags: map[string]int{}}
	a := &ContinuousEffect{
		Layer: LayerAbility, Timestamp: 1, SourcePerm: pa,
		Predicate: func(gs *GameState, p *Permanent) bool { return p == pb },
	}
	b := &ContinuousEffect{
		Layer: LayerAbility, Timestamp: 2, SourcePerm: pb,
		Predicate: func(gs *GameState, p *Permanent) bool { return p == pa },
	}
	if !effectDependsOn(a, b, gs) || !effectDependsOn(b, a, gs) {
		t.Fatalf("precondition: the pair should be mutually dependent")
	}

	in := []*ContinuousEffect{a, b}
	ordered := DependencyOrder(in, gs)
	if len(ordered) != 2 || ordered[0] != a || ordered[1] != b {
		t.Fatalf("cycle should fall back to timestamp order [a b]")
	}
}

// The flat index must address the same cells the nested one did. A
// transposed index would silently reverse every dependency, which no
// ordering assertion on a 2-element set can catch.
func TestDependencyOrderR152_FlatIndexIsNotTransposed(t *testing.T) {
	gs := newFixtureGame(t)
	a, b := r152Dependent()
	// Pad with independents so a and b sit at non-symmetric indices: a
	// transposed i*n+j would read a different cell entirely.
	ind := r152Effects(3)
	in := []*ContinuousEffect{ind[0], a, ind[1], b, ind[2]}

	ordered := DependencyOrder(in, gs)
	if len(ordered) != 5 {
		t.Fatalf("want 5 effects, got %d", len(ordered))
	}
	posA, posB := -1, -1
	for i, e := range ordered {
		if e == a {
			posA = i
		}
		if e == b {
			posB = i
		}
	}
	if posA < 0 || posB < 0 {
		t.Fatalf("effects dropped: posA=%d posB=%d", posA, posB)
	}
	if posB > posA {
		t.Fatalf("b must precede a; got posB=%d posA=%d", posB, posA)
	}
}
