package gameengine

import (
	"testing"
	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 STORM audit (CR §702.40).

func auditStormCard(name string) *Card {
	return &Card{
		Name: name, Owner: 0, Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "storm"},
				&gameast.Static{Modification: &gameast.Modification{ModKind: "typed_spell_effect",
					Args: []interface{}{&gameast.Damage{
						Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
						Target: gameast.Filter{Base: "any", Quantifier: "any"}}}}},
			},
		},
	}
}

// (a) the storm count = OTHER spells cast this turn by ALL players, not self.
func TestStorm_CountsAllPlayersNotSelf(t *testing.T) {
	gs := NewGameState(3, nil, nil)
	IncrementCastCount(gs, 0) // spell A — seat 0
	IncrementCastCount(gs, 1) // spell B — seat 1 (opponent)
	IncrementCastCount(gs, 2) // spell C — seat 2 (opponent)
	IncrementCastCount(gs, 0) // the storm spell itself — seat 0
	// 3 other spells (A by self + B + C by opponents); storm spell excluded.
	if got := StormCount(gs, 0); got != 3 {
		t.Fatalf("storm count must be 3 (all players' prior spells, minus self), got %d", got)
	}
	item := &StackItem{Card: auditStormCard("Storm Bolt"), Controller: 0}
	item.ID = nextStackID(gs)
	gs.Stack = append(gs.Stack, item)
	if n := ApplyStormCopies(gs, item, 0); n != 3 {
		t.Fatalf("ApplyStormCopies must make 3 copies across players, got %d", n)
	}
}

// (b) a spell that was later COUNTERED still counts (increment is at cast time).
func TestStorm_CounteredSpellStillCounts(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	IncrementCastCount(gs, 0)          // spell A cast...
	gs.Stack = nil                     // ...then countered / left the stack
	IncrementCastCount(gs, 0)          // storm spell
	if got := StormCount(gs, 0); got != 1 {
		t.Fatalf("a countered spell still counts toward storm: want 1, got %d", got)
	}
}

// (c) copies are COPIES — IsCopy set, they do NOT increment the cast count and
// do NOT re-fire storm; each copy carries an independent target slice.
func TestStorm_CopiesAreCopiesNotCasts(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.SpellsCastThisTurn = 3 // 2 priors + storm self
	item := &StackItem{
		Card:    auditStormCard("Storm Bolt"),
		Controller: 0,
		Targets: []Target{{Kind: TargetKindSeat, Seat: 1}},
	}
	item.ID = nextStackID(gs)
	gs.Stack = append(gs.Stack, item)
	before := gs.SpellsCastThisTurn
	ApplyStormCopies(gs, item, 0)
	if gs.SpellsCastThisTurn != before {
		t.Fatalf("copies must NOT increment the cast count: %d -> %d", before, gs.SpellsCastThisTurn)
	}
	copies := 0
	for _, it := range gs.Stack {
		if it.IsCopy {
			copies++
			if !it.Card.IsCopy {
				t.Error("copy item's Card must also be marked IsCopy (CR §707.10)")
			}
			// independent slice (not aliasing the original's).
			if len(it.Targets) > 0 && &it.Targets[0] == &item.Targets[0] {
				t.Error("each copy must own its target slice (independent new-target choice)")
			}
		}
	}
	if copies != 2 {
		t.Fatalf("want 2 copies on stack, got %d", copies)
	}
}

// (d) a copy of a non-permanent spell ceases to exist after resolving (§707.10).
func TestStorm_CopyCeasesOnResolve(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[1].Life = 20
	card := auditStormCard("Storm Bolt")
	card.IsCopy = true
	copyItem := &StackItem{
		Card: card, Controller: 0, IsCopy: true,
		Effect: CollectSpellEffectOf(card),
	}
	copyItem.ID = nextStackID(gs)
	gs.Stack = append(gs.Stack, copyItem)
	ResolveStackTop(gs)
	// The copy must not linger in any graveyard (it ceased to exist).
	for si := range gs.Seats {
		for _, c := range gs.Seats[si].Graveyard {
			if c == card {
				t.Fatalf("a resolved storm copy must cease, not go to seat %d graveyard", si)
			}
		}
	}
}
