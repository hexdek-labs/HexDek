package gameengine

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// r176 — removeCopiesFromZone gained a first-copy pre-scan so the
// overwhelmingly common no-copy case does no writes at all.
//
// HONEST LIMIT, STATED UP FRONT: no unit test can distinguish "wrote
// every element back over itself" from "wrote nothing", because the old
// writes were `src[i] = src[i]` and produced an identical slice. The
// correctness claim is therefore carried by a DIFFERENTIAL against a
// verbatim copy of the previous implementation, and the performance
// claim by the benchmark at the bottom. A test asserting "the fast path
// was taken" would pass under both and prove nothing — r174 shipped
// exactly that mistake and it is not repeated here.

// r176Legacy is the pre-r176 body, verbatim, as the differential oracle.
func r176Legacy(gs *GameState, zone *[]*Card) int {
	if zone == nil || len(*zone) == 0 {
		return 0
	}
	removed := 0
	kept := (*zone)[:0]
	for _, c := range *zone {
		if c != nil && c.IsCopy {
			if c.InstanceID != "" {
				MarkInstanceIDCeased(gs, c.InstanceID)
			}
			removed++
			continue
		}
		kept = append(kept, c)
	}
	*zone = kept
	return removed
}

// r176Zone builds a zone from a compact spec: 'k' keep, 'c' copy,
// 'i' copy carrying an InstanceID, 'n' a nil entry.
func r176Zone(spec string) []*Card {
	out := make([]*Card, 0, len(spec))
	for i, ch := range spec {
		switch ch {
		case 'n':
			out = append(out, nil)
		case 'k':
			out = append(out, &Card{Name: fmt.Sprintf("keep-%d", i)})
		case 'c':
			out = append(out, &Card{Name: fmt.Sprintf("copy-%d", i), IsCopy: true})
		case 'i':
			out = append(out, &Card{
				Name: fmt.Sprintf("copy-%d", i), IsCopy: true,
				InstanceID: fmt.Sprintf("id-%d", i),
			})
		}
	}
	return out
}

func r176Names(zone []*Card) []string {
	out := make([]string, 0, len(zone))
	for _, c := range zone {
		if c == nil {
			out = append(out, "<nil>")
			continue
		}
		out = append(out, c.Name)
	}
	return out
}

func r176Ceased(gs *GameState) []string {
	out := make([]string, 0, len(gs.CeasedInstanceIDs))
	for id := range gs.CeasedInstanceIDs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func r176Equal(a, b []string) bool {
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

// The headline: for every shape, the new implementation must produce the
// same surviving zone, the same return value, and the same set of ceased
// instance IDs as the code it replaced.
func TestR176_RemoveCopiesMatchesThePreviousImplementation(t *testing.T) {
	specs := []string{
		// the common case this change exists for
		"", "k", "kkkk", "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk",
		// copy at each position
		"c", "ck", "kc", "kck", "ckk", "kkc",
		// instance-id bearing copies
		"i", "ki", "ik", "kik", "iki",
		// all copies
		"cc", "ccc", "iii",
		// nils, which the old code KEPT (nil is not a copy)
		"n", "nk", "kn", "nc", "cn", "nkc", "knc", "nnn",
		// longer mixtures
		"kkkckkk", "kkkikkk", "kcikcikc", "nknknknk", "kkkkkkkkkc",
	}
	// plus randomised shapes, which is where an off-by-one in the prefix
	// handling would actually show up
	rng := rand.New(rand.NewSource(176))
	alphabet := []byte("kkkkcin")
	for i := 0; i < 2000; i++ {
		n := rng.Intn(24)
		b := make([]byte, n)
		for j := range b {
			b[j] = alphabet[rng.Intn(len(alphabet))]
		}
		specs = append(specs, string(b))
	}

	for _, spec := range specs {
		gotGS := NewGameState(4, rand.New(rand.NewSource(1)), nil)
		wantGS := NewGameState(4, rand.New(rand.NewSource(1)), nil)
		gotZone := r176Zone(spec)
		wantZone := r176Zone(spec)

		gotN := removeCopiesFromZone(gotGS, &gotZone)
		wantN := r176Legacy(wantGS, &wantZone)

		if gotN != wantN {
			t.Fatalf("spec %q: removed count %d, previous implementation gave %d",
				spec, gotN, wantN)
		}
		if g, w := r176Names(gotZone), r176Names(wantZone); !r176Equal(g, w) {
			t.Fatalf("spec %q: surviving zone %v, previous implementation gave %v",
				spec, g, w)
		}
		if g, w := r176Ceased(gotGS), r176Ceased(wantGS); !r176Equal(g, w) {
			t.Fatalf("spec %q: ceased ids %v, previous implementation gave %v",
				spec, g, w)
		}
	}
}

// Both directions, named explicitly so a failure says which half broke.
func TestR176_RemoveCopies_BothDirections(t *testing.T) {
	t.Run("a copy IS removed and ceased", func(t *testing.T) {
		gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
		zone := r176Zone("kkikk")
		if n := removeCopiesFromZone(gs, &zone); n != 1 {
			t.Fatalf("expected 1 copy removed, got %d", n)
		}
		if len(zone) != 4 {
			t.Errorf("expected 4 survivors, got %d (%v)", len(zone), r176Names(zone))
		}
		for _, c := range zone {
			if c != nil && c.IsCopy {
				t.Error("a copy survived the sweep (CR 707.10)")
			}
		}
		if _, ok := gs.CeasedInstanceIDs["id-2"]; !ok {
			t.Error("the removed copy's InstanceID was not marked ceased")
		}
	})

	t.Run("a zone with no copies is left exactly as it was", func(t *testing.T) {
		gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
		zone := r176Zone("kkkkk")
		before := r176Names(zone)
		if n := removeCopiesFromZone(gs, &zone); n != 0 {
			t.Fatalf("expected 0 removed from a zone with no copies, got %d", n)
		}
		if after := r176Names(zone); !r176Equal(before, after) {
			t.Errorf("zone changed: %v -> %v", before, after)
		}
		if len(gs.CeasedInstanceIDs) != 0 {
			t.Errorf("nothing was removed but %d ids were ceased", len(gs.CeasedInstanceIDs))
		}
	})
}

// A nil entry is not a copy and must survive — the pre-scan must not
// mistake it for one, and must not deref it.
func TestR176_NilEntriesSurviveAndDoNotPanic(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	zone := r176Zone("nnknn")
	if n := removeCopiesFromZone(gs, &zone); n != 0 {
		t.Fatalf("nil entries are not copies; expected 0 removed, got %d", n)
	}
	if len(zone) != 5 {
		t.Errorf("expected all 5 entries to survive, got %d", len(zone))
	}
}

func benchRemoveCopies(b *testing.B, spec string) {
	gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	base := r176Zone(spec)
	zone := make([]*Card, len(base))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(zone, base)
		zone = zone[:len(base)]
		removeCopiesFromZone(gs, &zone)
	}
}

// A real library is ~90 cards and almost never holds a copy. That is the
// case this change is for.
func BenchmarkR176_Library90_NoCopies(b *testing.B) {
	spec := ""
	for i := 0; i < 90; i++ {
		spec += "k"
	}
	benchRemoveCopies(b, spec)
}

func BenchmarkR176_Library90_OneCopyAtEnd(b *testing.B) {
	spec := ""
	for i := 0; i < 89; i++ {
		spec += "k"
	}
	benchRemoveCopies(b, spec+"c")
}
