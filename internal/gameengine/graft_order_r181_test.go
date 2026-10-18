package gameengine

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r181 — ApplyGraft's controller test was the LAST of four predicates,
// after a map lookup and a full ability-list walk. Hoisting it to first
// is behaviour-preserving because all four are pure. These tests prove
// that over randomized boards rather than by assertion.
//
// ApplyGraft is the third member of the ungated per-ETB
// full-battlefield-scan class (see docs/etb-observer-audit-r180.md) and
// the only one that scans ALL seats. The scan itself is still there;
// this round only removes the work done per rejected permanent.
//
// Mutation controls run against this differential, with results:
//   drop the controller check entirely      -> RED (census differs)
//   drop the PhasedOut exclusion            -> RED (census differs)
//   counters test <= 0 becomes < 0          -> GREEN, and it is a NO-OP
//       mutation, not a gap: ApplyGraftTransfer re-checks
//       source.Counters["+1/+1"] <= 0 and returns false, so the change
//       cannot be observed.
//   drop the g == perm self-exclusion       -> GREEN, also a no-op:
//       ApplyGraftTransfer(gs, perm, perm) removes and adds one counter
//       on the same permanent, netting zero.
//
// Known limit of this differential: the census measures +1/+1 counters
// only. A change that alters the EVENT LOG without altering counters --
// the two no-op mutations above both do -- would not be caught here.

func r181GraftCard(name string, n int, creature bool) *Card {
	abs := []gameast.Ability{}
	if n > 0 {
		abs = append(abs, &gameast.Keyword{Name: "Graft", Args: []interface{}{n}})
	}
	types := []string{"artifact"}
	if creature {
		types = []string{"creature"}
	}
	return &Card{Name: name, Types: types, AST: &gameast.CardAST{Abilities: abs}}
}

// applyGraftLegacy is the pre-r181 predicate order, verbatim.
func applyGraftLegacy(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if n, ok := keywordArgCostStrict(perm.Card, "graft"); ok && n > 0 {
		ApplyGraftETB(gs, perm, n)
	}
	if !perm.IsCreature() {
		return
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, g := range seat.Battlefield {
			if g == nil || g == perm || g.Card == nil || g.PhasedOut {
				continue
			}
			if g.Counters == nil || g.Counters["+1/+1"] <= 0 {
				continue
			}
			if _, ok := keywordArgCostStrict(g.Card, "graft"); !ok {
				continue
			}
			if perm.Controller != g.Controller {
				continue
			}
			ApplyGraftTransfer(gs, g, perm)
		}
	}
}

// r181Board builds one randomized 4-seat board plus an entering creature.
// Returns a builder so the identical board can be constructed twice.
func r181Board(seed int64) (func() (*GameState, *Permanent), string) {
	desc := fmt.Sprintf("seed=%d", seed)
	return func() (*GameState, *Permanent) {
		rng := rand.New(rand.NewSource(seed))
		gs := NewGameState(4, rand.New(rand.NewSource(seed)), nil)
		for s := 0; s < 4; s++ {
			np := rng.Intn(7)
			for i := 0; i < np; i++ {
				graftN := 0
				if rng.Intn(3) == 0 {
					graftN = 1 + rng.Intn(3)
				}
				c := r181GraftCard(fmt.Sprintf("r181 s%d p%d", s, i), graftN, rng.Intn(4) > 0)
				p := &Permanent{Card: c, Controller: s, Owner: s}
				if rng.Intn(2) == 0 {
					p.AddCounter("+1/+1", 1+rng.Intn(3))
				}
				if rng.Intn(10) == 0 {
					p.PhasedOut = true
				}
				gs.Seats[s].Battlefield = append(gs.Seats[s].Battlefield, p)
			}
		}
		entN := 0
		if rng.Intn(4) == 0 {
			entN = 1 + rng.Intn(2)
		}
		ctrl := rng.Intn(4)
		ent := &Permanent{Card: r181GraftCard("r181 entering", entN, rng.Intn(5) > 0), Controller: ctrl, Owner: ctrl}
		gs.Seats[ctrl].Battlefield = append(gs.Seats[ctrl].Battlefield, ent)
		return gs, ent
	}, desc
}

// counterCensus is the full observable effect of ApplyGraft: who holds
// how many +1/+1 counters afterwards.
func r181Census(gs *GameState) []int {
	var out []int
	for _, seat := range gs.Seats {
		for _, p := range seat.Battlefield {
			n := 0
			if p.Counters != nil {
				n = p.Counters["+1/+1"]
			}
			out = append(out, n)
		}
	}
	return out
}

func TestR181_ApplyGraft_OrderIsBehaviourPreserving(t *testing.T) {
	const corpus = 3000
	transfers := 0
	for i := 0; i < corpus; i++ {
		build, desc := r181Board(int64(i) + 181000)

		gsA, entA := build()
		applyGraftLegacy(gsA, entA)
		want := r181Census(gsA)

		gsB, entB := build()
		ApplyGraft(gsB, entB)
		got := r181Census(gsB)

		if len(want) != len(got) {
			t.Fatalf("%s: census length %d vs %d", desc, len(want), len(got))
		}
		for j := range want {
			if want[j] != got[j] {
				t.Fatalf("%s: counter census differs at %d: legacy %v, new %v\nlegacy=%v\nnew=%v",
					desc, j, want[j], got[j], want, got)
			}
		}
		// Track how often a transfer actually happened, so a corpus that
		// never exercises the body cannot pass vacuously.
		for j := range want {
			if want[j] > 0 {
				transfers++
				break
			}
		}
	}
	if transfers < corpus/10 {
		t.Fatalf("degenerate corpus: only %d/%d boards had any counters — "+
			"the differential is not exercising the transfer body",
			transfers, corpus)
	}
	t.Logf("r181: %d boards agreed, %d had counters", corpus, transfers)
}

// A graft source must still move a counter to a creature entering under
// the SAME controller, and must not to one entering under another.
func TestR181_ApplyGraft_StillTransfers(t *testing.T) {
	mk := func(entCtrl int) (*GameState, *Permanent, *Permanent) {
		gs := NewGameState(4, rand.New(rand.NewSource(3)), nil)
		src := &Permanent{Card: r181GraftCard("r181 Graft Source", 2, true), Controller: 0, Owner: 0}
		src.AddCounter("+1/+1", 2)
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
		ent := &Permanent{Card: r181GraftCard("r181 Newcomer", 0, true), Controller: entCtrl, Owner: entCtrl}
		gs.Seats[entCtrl].Battlefield = append(gs.Seats[entCtrl].Battlefield, ent)
		return gs, src, ent
	}

	gs, src, ent := mk(0)
	ApplyGraft(gs, ent)
	if ent.Counters["+1/+1"] != 1 || src.Counters["+1/+1"] != 1 {
		t.Fatalf("same controller: want src 1 / ent 1, got src %d / ent %d",
			src.Counters["+1/+1"], ent.Counters["+1/+1"])
	}

	gs, src, ent = mk(2)
	ApplyGraft(gs, ent)
	if ent.Counters["+1/+1"] != 0 || src.Counters["+1/+1"] != 2 {
		t.Fatalf("different controller: want src 2 / ent 0, got src %d / ent %d",
			src.Counters["+1/+1"], ent.Counters["+1/+1"])
	}
}
