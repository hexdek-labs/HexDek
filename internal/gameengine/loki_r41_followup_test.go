package gameengine

// Loki r41 follow-up: regression tests pinning the Cerulean Sphinx zone-leak
// cluster (1,622 of 1,652 r41 chaos-game invariant hits) — see
// docs/loki-r41-report.md + docs/loki-r41-followup-report.md.
//
// Two bugs collaborated:
//
//   1. collectSpellEffect treated the first Activated ability of any card
//      as the cast-time spell effect, even on permanent spells. Cerulean
//      Sphinx ({U}: Its owner shuffles it into their library) therefore
//      resolved its activated ability AT CAST TIME, then proceeded to ETB
//      normally — duplicating the *Card pointer across "owner's library"
//      and "controller's battlefield".
//
//   2. ResolveStackTop / resolveActivatedAbility synthesize a transient
//      Permanent when the stack item has no on-battlefield source (spells,
//      or activated abilities of a not-yet-on-battlefield source). Those
//      synthetic Permanents omitted the Owner field, so handlers keying off
//      src.Owner (shuffle_into_owner_library, etc.) routed effects to seat
//      0 even when the card was owned by seat 1+.
//
// Both fixes have to be in place for the duplication to clear; this file
// pins the surfaces independently.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestCollectSpellEffect_PermanentSpellsHaveNoEffect — permanent spells
// (creature / artifact / enchantment / planeswalker / battle) must NEVER
// expose a printed Activated ability as a cast-time spell effect. That was
// the bug behind the r41 Loki Cerulean Sphinx cluster.
func TestCollectSpellEffect_PermanentSpellsHaveNoEffect(t *testing.T) {
	cases := []struct {
		name  string
		types []string
	}{
		{"creature", []string{"creature"}},
		{"artifact", []string{"artifact"}},
		{"enchantment", []string{"enchantment"}},
		{"planeswalker", []string{"planeswalker"}},
		{"battle", []string{"battle"}},
		{"legendary creature", []string{"legendary", "creature"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card := &Card{
				Name:  "Cerulean Sphinx",
				Types: tc.types,
				AST: &gameast.CardAST{
					Name: "Cerulean Sphinx",
					Abilities: []gameast.Ability{
						&gameast.Keyword{Name: "flying"},
						&gameast.Activated{
							Cost: gameast.Cost{
								Mana: &gameast.ManaCost{
									Symbols: []gameast.ManaSymbol{{Raw: "{u}", Color: []string{"U"}}},
								},
							},
							Effect: &gameast.ModificationEffect{
								ModKind: "shuffle_self_into_library",
							},
						},
					},
				},
			}
			if got := collectSpellEffect(card); got != nil {
				t.Fatalf("expected nil spell effect for %q permanent spell, got %T", tc.name, got)
			}
		})
	}
}

// TestCollectSpellEffect_InstantsKeepEmptyCostActivatedBody — instants and
// sorceries are allowed to expose an Activated AST node as their spell
// body when (and only when) the cost is empty (parser artifact for cards
// like Summon the School / Divergent Growth).
func TestCollectSpellEffect_InstantsKeepEmptyCostActivatedBody(t *testing.T) {
	body := &gameast.Damage{Amount: *gameast.NumInt(3), Target: gameast.TargetOpponent()}
	card := &Card{
		Name:  "Fake Bolt",
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Fake Bolt",
			Abilities: []gameast.Ability{
				&gameast.Activated{Cost: gameast.Cost{}, Effect: body},
			},
		},
	}
	if got := collectSpellEffect(card); got == nil {
		t.Fatal("empty-cost Activated on an instant should be returned as spell body")
	}
}

// TestCollectSpellEffect_InstantsSkipNonEmptyCostActivated — even on an
// instant, a real activated-ability AST node (one with a mana / tap / etc.
// cost) is NOT the spell body. It's the printed ability that would function
// if the card were on the battlefield (rare in practice for instants, but
// the filter has to be correct or the bug class returns).
func TestCollectSpellEffect_InstantsSkipNonEmptyCostActivated(t *testing.T) {
	card := &Card{
		Name:  "Hypothetical Instant With Activated",
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Hypothetical Instant With Activated",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost: gameast.Cost{
						Mana: &gameast.ManaCost{
							Symbols: []gameast.ManaSymbol{{Raw: "{1}", Generic: 1}},
						},
					},
					Effect: &gameast.Damage{Amount: *gameast.NumInt(3), Target: gameast.TargetOpponent()},
				},
			},
		},
	}
	if got := collectSpellEffect(card); got != nil {
		t.Fatalf("non-empty cost Activated must not be returned as spell body; got %T", got)
	}
}

// TestShuffleSelfIntoLibrary_RoutesToCardOwnerNotSeatZero — when the
// shuffle_self_into_library handler runs through the ResolveEffect
// synthetic-Permanent path (i.e. spell cast or activated ability where the
// stack item has no battlefield source), it must read the destination seat
// from the Card's Owner — not from the zero-valued Owner of the synthetic
// Permanent. This is the second half of the r41 duplication bug.
//
// We exercise the path directly: build a transient Permanent that mirrors
// what ResolveStackTop synthesizes when Owner is properly threaded, then
// invoke the resolver. The card's owner is seat 1, so the card must land
// in seat 1's library — never seat 0's.
func TestShuffleSelfIntoLibrary_RoutesToCardOwnerNotSeatZero(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{
		Name:  "Cerulean Sphinx",
		Owner: 1,
		Types: []string{"creature"},
	}
	// Put the card on seat 1's battlefield first so removePermanent has
	// something to clean up — mirrors the real activated-ability path.
	perm := &Permanent{
		Card:       card,
		Controller: 1,
		Owner:      1,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, perm)

	ResolveEffect(gs, perm, &gameast.ModificationEffect{
		ModKind: "shuffle_self_into_library",
	})

	if len(gs.Seats[0].Library) != 0 {
		t.Fatalf("seat 0 library must stay empty; got %d cards (regression: r41 bug routed everything to seat 0)",
			len(gs.Seats[0].Library))
	}
	found := false
	for _, c := range gs.Seats[1].Library {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("card must land in its owner's (seat 1) library after shuffle_self_into_library")
	}
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Fatalf("seat 1 battlefield must be empty after shuffle; got %d perms (CardIdentity regression risk)",
			len(gs.Seats[1].Battlefield))
	}
}
