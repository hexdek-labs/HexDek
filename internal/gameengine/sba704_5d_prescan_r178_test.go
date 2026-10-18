package gameengine

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// r178 — sba704_5d gained the same first-token pre-scan r176 gave its
// sibling 5e. 5d's COMMIT was already gated on `removed > 0`, so nothing
// observable was wrong; what was not gated was the per-element writes.
//
// SAME HONEST LIMIT AS r176: no unit test can distinguish "rewrote every
// element over itself" from "wrote nothing", because the old writes
// produced an identical slice. Correctness is carried by a DIFFERENTIAL
// against a verbatim copy of the previous implementation; the
// performance claim is structural (see the commit message).

// r178Legacy is the pre-r178 body of sba704_5d, verbatim.
func r178Legacy(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, zoneName := range []string{"hand", "graveyard", "exile", "library"} {
			var zone *[]*Card
			switch zoneName {
			case "hand":
				zone = &s.Hand
			case "graveyard":
				zone = &s.Graveyard
			case "exile":
				zone = &s.Exile
			case "library":
				zone = &s.Library
			}
			removed := 0
			kept := (*zone)[:0]
			src := *zone
			for _, c := range src {
				if cardIsToken(c) {
					if c != nil && c.InstanceID != "" {
						MarkInstanceIDCeased(gs, c.InstanceID)
					}
					removed++
					continue
				}
				kept = append(kept, c)
			}
			if removed > 0 {
				*zone = kept
				changed = true
				gs.LogEvent(Event{
					Kind: "sba_704_5d", Seat: s.Idx, Target: -1, Amount: removed,
					Details: map[string]interface{}{
						"rule": "704.5d", "reason": "token_in_nonbattlefield_zone",
						"zone": zoneName,
					},
				})
			}
		}
	}
	return changed
}

// r178Card builds one card from a spec char: 'k' keep, 't' token,
// 'i' token carrying an InstanceID, 'n' nil.
func r178Card(ch byte, tag string) *Card {
	switch ch {
	case 'n':
		return nil
	case 't':
		return &Card{Name: "tok-" + tag, Types: []string{"creature", "token"}}
	case 'i':
		return &Card{Name: "tok-" + tag, Types: []string{"creature", "token"},
			InstanceID: "id-" + tag}
	default:
		return &Card{Name: "keep-" + tag, Types: []string{"creature"}}
	}
}

// r178Fill populates all four hidden zones of all four seats from specs.
func r178Fill(gs *GameState, specs [4]string) {
	for si, s := range gs.Seats {
		if s == nil {
			continue
		}
		spec := specs[si%len(specs)]
		mk := func(zone string) []*Card {
			out := make([]*Card, 0, len(spec))
			for i := 0; i < len(spec); i++ {
				out = append(out, r178Card(spec[i], fmt.Sprintf("%d-%s-%d", si, zone, i)))
			}
			return out
		}
		s.Hand, s.Graveyard, s.Exile, s.Library = mk("h"), mk("g"), mk("e"), mk("l")
	}
}

func r178Snapshot(gs *GameState) []string {
	var out []string
	for si, s := range gs.Seats {
		if s == nil {
			continue
		}
		for zn, z := range map[string][]*Card{
			"hand": s.Hand, "graveyard": s.Graveyard, "exile": s.Exile, "library": s.Library,
		} {
			for i, c := range z {
				n := "<nil>"
				if c != nil {
					n = c.Name
				}
				out = append(out, fmt.Sprintf("%d/%s/%d=%s", si, zn, i, n))
			}
		}
	}
	sort.Strings(out)
	return out
}

func r178Ceased(gs *GameState) []string {
	out := make([]string, 0, len(gs.CeasedInstanceIDs))
	for id := range gs.CeasedInstanceIDs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func r178Eq(a, b []string) bool {
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

// The headline: identical surviving zones, identical return value and
// identical ceased-ID set as the implementation it replaced.
func TestR178_Sba704_5dMatchesThePreviousImplementation(t *testing.T) {
	specs := []string{
		// the common case this change exists for
		"", "k", "kkkk", "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk",
		// token at each position
		"t", "tk", "kt", "ktk", "tkk", "kkt",
		// instance-id bearing tokens
		"i", "ki", "ik", "kik", "iki",
		// all tokens
		"tt", "ttt", "iii",
		// nils, which are NOT tokens and must survive
		"n", "nk", "kn", "nt", "tn", "nkt", "knt", "nnn",
		// longer mixtures
		"kkktkkk", "kkkikkk", "ktiktikt", "nknknknk", "kkkkkkkkkt",
	}
	rng := rand.New(rand.NewSource(178))
	alphabet := []byte("kkkktin")
	for i := 0; i < 1200; i++ {
		n := rng.Intn(20)
		b := make([]byte, n)
		for j := range b {
			b[j] = alphabet[rng.Intn(len(alphabet))]
		}
		specs = append(specs, string(b))
	}

	for _, spec := range specs {
		got := NewGameState(4, rand.New(rand.NewSource(1)), nil)
		want := NewGameState(4, rand.New(rand.NewSource(1)), nil)
		four := [4]string{spec, spec, spec, spec}
		r178Fill(got, four)
		r178Fill(want, four)

		gotChanged := sba704_5d(got)
		wantChanged := r178Legacy(want)

		if gotChanged != wantChanged {
			t.Fatalf("spec %q: changed=%v, previous implementation gave %v",
				spec, gotChanged, wantChanged)
		}
		if g, w := r178Snapshot(got), r178Snapshot(want); !r178Eq(g, w) {
			t.Fatalf("spec %q: surviving zones differ from the previous implementation\n"+
				" got: %v\nwant: %v", spec, g, w)
		}
		if g, w := r178Ceased(got), r178Ceased(want); !r178Eq(g, w) {
			t.Fatalf("spec %q: ceased ids %v, previous implementation gave %v", spec, g, w)
		}
	}
}

func TestR178_Sba704_5d_BothDirections(t *testing.T) {
	t.Run("a token in a hidden zone IS removed and ceased", func(t *testing.T) {
		gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
		r178Fill(gs, [4]string{"kkikk", "kkikk", "kkikk", "kkikk"})
		if !sba704_5d(gs) {
			t.Fatal("CR 704.5d — a token in a hidden zone must be removed")
		}
		if n := len(gs.Seats[0].Library); n != 4 {
			t.Errorf("expected 4 survivors in the library, got %d", n)
		}
		for _, c := range gs.Seats[0].Library {
			if cardIsToken(c) {
				t.Error("a token survived the sweep")
			}
		}
		if len(gs.CeasedInstanceIDs) == 0 {
			t.Error("the removed token's InstanceID was not marked ceased")
		}
	})

	t.Run("a zone with no tokens is left exactly as it was", func(t *testing.T) {
		gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
		r178Fill(gs, [4]string{"kkkkk", "kkkkk", "kkkkk", "kkkkk"})
		before := r178Snapshot(gs)
		if sba704_5d(gs) {
			t.Fatal("no tokens present, so nothing should have changed")
		}
		if after := r178Snapshot(gs); !r178Eq(before, after) {
			t.Error("a zone with no tokens was modified")
		}
		if len(gs.CeasedInstanceIDs) != 0 {
			t.Errorf("nothing removed but %d ids ceased", len(gs.CeasedInstanceIDs))
		}
	})
}

// nil is not a token; the pre-scan must neither match it nor deref it.
func TestR178_NilEntriesSurviveAndDoNotPanic(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	r178Fill(gs, [4]string{"nnknn", "nnknn", "nnknn", "nnknn"})
	if sba704_5d(gs) {
		t.Fatal("nil entries are not tokens; nothing should have been removed")
	}
	if n := len(gs.Seats[0].Hand); n != 5 {
		t.Errorf("expected all 5 entries to survive, got %d", n)
	}
}

func benchR178(b *testing.B, spec string) {
	base := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	r178Fill(base, [4]string{spec, spec, spec, spec})
	snap := make([][]*Card, 0, 16)
	for _, s := range base.Seats {
		snap = append(snap, append([]*Card{}, s.Hand...), append([]*Card{}, s.Graveyard...),
			append([]*Card{}, s.Exile...), append([]*Card{}, s.Library...))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := 0
		for _, s := range base.Seats {
			s.Hand = append(s.Hand[:0], snap[k]...)
			s.Graveyard = append(s.Graveyard[:0], snap[k+1]...)
			s.Exile = append(s.Exile[:0], snap[k+2]...)
			s.Library = append(s.Library[:0], snap[k+3]...)
			k += 4
		}
		sba704_5d(base)
	}
}

// A real library is ~90 cards and essentially never holds a token.
func BenchmarkR178_Zones90_NoTokens(b *testing.B) {
	spec := ""
	for i := 0; i < 90; i++ {
		spec += "k"
	}
	benchR178(b, spec)
}

func BenchmarkR178_Zones90_OneTokenAtEnd(b *testing.B) {
	spec := ""
	for i := 0; i < 89; i++ {
		spec += "k"
	}
	benchR178(b, spec+"t")
}
