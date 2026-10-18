package heimdall

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// r179 — DiffBattlefield used to append one entry PER COPY, making the
// result length equal to the number of permanents that entered that
// turn. A token swarm therefore produced an enormous slice of one
// repeated string, which the caller retained for the whole game.
//
// Found from a SIGQUIT goroutine dump of a wedged production process
// that was on-CPU inside this function.
//
// Correctness here is EQUIVALENCE UNDER THE CONSUMERS, not equality:
// every production consumer either calls distinctNames() first or keeps
// only a name's first occurrence, so emitting each name once must give
// every consumer the same answer as before.

// r179Legacy is the pre-r179 body, verbatim.
func r179Legacy(pre, post map[string]int) []string {
	var newCards []string
	for name, postCount := range post {
		preCount := pre[name]
		for i := 0; i < postCount-preCount; i++ {
			newCards = append(newCards, name)
		}
	}
	return newCards
}

func r179Sorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func r179Eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The headline: identical to the old implementation once de-duplicated,
// which is exactly how every consumer reads it.
func TestR179_EquivalentToLegacyUnderDistinctNames(t *testing.T) {
	type pair struct{ pre, post map[string]int }
	cases := []pair{
		{nil, nil},
		{map[string]int{}, map[string]int{}},
		{map[string]int{}, map[string]int{"Llanowar Elves": 1}},
		{map[string]int{"Llanowar Elves": 1}, map[string]int{"Llanowar Elves": 1}},
		{map[string]int{"Llanowar Elves": 1}, map[string]int{"Llanowar Elves": 4}},
		{map[string]int{"Llanowar Elves": 4}, map[string]int{"Llanowar Elves": 1}}, // shrank
		{map[string]int{"A": 2, "B": 1}, map[string]int{"A": 3, "B": 1, "C": 7}},
		{map[string]int{"Saproling Token": 3}, map[string]int{"Saproling Token": 90210}},
	}
	rng := rand.New(rand.NewSource(179))
	names := []string{"A", "B", "C", "Goblin Token", "Treasure Token", "Forest"}
	for i := 0; i < 1500; i++ {
		pre, post := map[string]int{}, map[string]int{}
		for _, n := range names {
			if rng.Intn(3) > 0 {
				pre[n] = rng.Intn(6)
			}
			if rng.Intn(3) > 0 {
				post[n] = rng.Intn(60)
			}
		}
		cases = append(cases, pair{pre, post})
	}

	for i, c := range cases {
		gotNew := distinctNames(DiffBattlefield(c.pre, c.post))
		wantNew := distinctNames(r179Legacy(c.pre, c.post))
		if !r179Eq(r179Sorted(gotNew), r179Sorted(wantNew)) {
			t.Fatalf("case %d: distinct set differs from the previous implementation\n got: %v\nwant: %v",
				i, r179Sorted(gotNew), r179Sorted(wantNew))
		}
	}
}

// The property the fix exists for: the result is bounded by the number
// of DISTINCT names, never by the number of permanents.
func TestR179_ResultBoundedByDistinctNamesNotByCopies(t *testing.T) {
	pre := map[string]int{"Saproling Token": 1}
	post := map[string]int{"Saproling Token": 2_000_000}

	got := DiffBattlefield(pre, post)
	if len(got) != 1 {
		t.Fatalf("a swarm of one token name must yield 1 entry, got %d", len(got))
	}
	if got[0] != "Saproling Token" {
		t.Fatalf("wrong name: %q", got[0])
	}
	// and the legacy shape is what we are escaping
	if n := len(r179Legacy(pre, post)); n != 1_999_999 {
		t.Fatalf("sanity: the previous implementation should have produced "+
			"1,999,999 entries here, got %d", n)
	}
}

// Both directions, so the test cannot pass by returning nothing.
func TestR179_BothDirections(t *testing.T) {
	t.Run("a genuinely new name IS reported", func(t *testing.T) {
		got := DiffBattlefield(map[string]int{"A": 1}, map[string]int{"A": 1, "B": 1})
		if !r179Eq(got, []string{"B"}) {
			t.Fatalf("expected [B], got %v", got)
		}
	})
	t.Run("an unchanged or shrinking name is NOT reported", func(t *testing.T) {
		got := DiffBattlefield(map[string]int{"A": 5, "B": 2}, map[string]int{"A": 5, "B": 1})
		if len(got) != 0 {
			t.Fatalf("nothing entered, expected no names, got %v", got)
		}
	})
}

func benchR179(b *testing.B, copies int, fn func(a, c map[string]int) []string) {
	pre := map[string]int{"Saproling Token": 1, "Forest": 8}
	post := map[string]int{"Saproling Token": copies, "Forest": 8}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// the consumers' real shape: diff, then de-duplicate
		_ = distinctNames(fn(pre, post))
	}
}

func BenchmarkR179_Swarm10k_Before(b *testing.B) { benchR179(b, 10000, r179Legacy) }
func BenchmarkR179_Swarm10k_After(b *testing.B)  { benchR179(b, 10000, DiffBattlefield) }
func BenchmarkR179_Typical8_Before(b *testing.B) { benchR179(b, 8, r179Legacy) }
func BenchmarkR179_Typical8_After(b *testing.B)  { benchR179(b, 8, DiffBattlefield) }

var _ = fmt.Sprintf
