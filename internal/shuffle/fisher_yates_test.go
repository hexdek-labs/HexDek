package shuffle

import (
	"sort"
	"testing"
)

func TestShuffleProducesPermutation(t *testing.T) {
	original := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cards := make([]int, len(original))
	copy(cards, original)

	if err := Shuffle(cards); err != nil {
		t.Fatalf("shuffle failed: %v", err)
	}

	// Result must be a permutation of original (same multiset)
	got := make([]int, len(cards))
	copy(got, cards)
	sort.Ints(got)

	want := make([]int, len(original))
	copy(want, original)
	sort.Ints(want)

	for i := range got {
		if got[i] != want[i] {
			t.Errorf("shuffle changed contents: got %v, want %v", got, want)
		}
	}
}

func TestShuffleEmptySlice(t *testing.T) {
	cards := []int{}
	if err := Shuffle(cards); err != nil {
		t.Errorf("shuffle of empty slice should succeed, got: %v", err)
	}
}

func TestShuffleSingleElement(t *testing.T) {
	cards := []int{42}
	if err := Shuffle(cards); err != nil {
		t.Errorf("shuffle of single-element slice should succeed, got: %v", err)
	}
	if cards[0] != 42 {
		t.Errorf("single-element shuffle changed value: got %d, want 42", cards[0])
	}
}

func TestShuffleDistribution(t *testing.T) {
	// Statistical sanity check: shuffle a 4-element array many times,
	// the position of each element should appear at each index roughly
	// equally often (~25% per position).
	const trials = 10000
	const n = 4

	// counts[element][position] = how many times element ended up at position
	counts := make([][]int, n)
	for i := range counts {
		counts[i] = make([]int, n)
	}

	for trial := 0; trial < trials; trial++ {
		cards := []int{0, 1, 2, 3}
		if err := Shuffle(cards); err != nil {
			t.Fatalf("trial %d: shuffle failed: %v", trial, err)
		}
		for pos, elem := range cards {
			counts[elem][pos]++
		}
	}

	expected := trials / n
	tolerance := expected / 10 // allow 10% variance

	for elem := 0; elem < n; elem++ {
		for pos := 0; pos < n; pos++ {
			diff := counts[elem][pos] - expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tolerance {
				t.Errorf("element %d at position %d: %d times (expected ~%d, tolerance %d)",
					elem, pos, counts[elem][pos], expected, tolerance)
			}
		}
	}
}

func TestRandIntNRange(t *testing.T) {
	for n := 1; n <= 100; n++ {
		for trial := 0; trial < 100; trial++ {
			v, err := randIntN(n)
			if err != nil {
				t.Fatalf("randIntN(%d) error: %v", n, err)
			}
			if v < 0 || v >= n {
				t.Errorf("randIntN(%d) returned %d, want in [0, %d)", n, v, n)
			}
		}
	}
}
