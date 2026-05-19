package gameengine

// Loki r44: regression tests pinning the Adric, Mathematical Genius
// CardIdentity leak (562 of 564 CardIdentity hits in r43 chaos game 170 /
// seed 1700042 — see docs/loki-r43-postfix.md).
//
// Root cause: a contract mismatch between Hat.ChooseResponse and
// PriorityRound. The Hat returned a *StackItem as advice without
// removing the card from hand (the comment said the engine would do
// it); PriorityRound paid mana, logged the cast event, and pushed the
// item onto the stack — but never removed the card from hand. The
// permanent then resolved to the battlefield while its *Card pointer
// remained in seat.Hand, tripping CardIdentity "card X in both hand
// and battlefield".
//
// Adric surfaced this because his second activated ability ("Ultimate
// Sacrifice — {1}{U}, Sacrifice ~: Counter target activated or
// triggered ability") makes counterSpellEffect return a CounterSpell
// effect when scanning his AST. The Hat saw him as a counter-response
// card and selected him for every opposing trigger that hit the stack.
//
// Fix (two layers, both pinned here):
//
//  1. counterSpellEffect rejects permanent spells. A permanent's printed
//     activated counter ability is only usable from the battlefield —
//     the card-in-hand is not a counterspell candidate.
//
//  2. PriorityRound centralizes the hand-removal side-effect. Every
//     successful response (Hat or fallback) now leaves hand via the
//     same removeFromHand call after the cost check passes.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestCounterSpellEffect_PermanentSpellNeverCounterCandidate — a creature
// with a printed activated counter ability (Adric, Mathematical Genius)
// must not be selectable as an instant-speed counter response. Same
// reasoning as collectSpellEffect rejecting permanent spells: the
// counter clause functions only on the battlefield (CR §112.6 / §603.5).
func TestCounterSpellEffect_PermanentSpellNeverCounterCandidate(t *testing.T) {
	adric := &Card{
		Name:  "Adric, Mathematical Genius",
		Types: []string{"legendary", "creature"},
		AST: &gameast.CardAST{
			Name: "Adric, Mathematical Genius",
			Abilities: []gameast.Ability{
				// First activated — copy-ability, not counter.
				&gameast.Activated{
					Cost: gameast.Cost{
						Mana: &gameast.ManaCost{
							Symbols: []gameast.ManaSymbol{
								{Raw: "{2}", Generic: 2},
								{Raw: "{u}", Color: []string{"U"}},
							},
						},
						Tap: true,
					},
					Effect: &gameast.ModificationEffect{ModKind: "parsed_effect_residual"},
				},
				// Second activated — the bait. CounterSpell effect with
				// a sacrifice-self cost. Without the fix, this makes
				// counterSpellEffect return non-nil and Adric's *Card
				// gets pulled out of hand at opponents' priority rounds.
				&gameast.Activated{
					Cost: gameast.Cost{
						Sacrifice: &gameast.Filter{Base: "thing"},
					},
					Effect: &gameast.CounterSpell{
						Target: gameast.Filter{Base: "abilities", Targeted: true},
					},
				},
			},
		},
	}
	if got := counterSpellEffect(adric); got != nil {
		t.Fatalf("permanent spell exposed as counter-response candidate: %T", got)
	}
	if got := CounterSpellEffectOf(adric); got != nil {
		t.Fatalf("public CounterSpellEffectOf wrapper also leaks permanent spell: %T", got)
	}
}

// TestCounterSpellEffect_InstantStillSelectable — refinement check:
// real counterspells (instants) must still be detected as counter
// candidates. Without this, the r44 fix would over-correct and break
// every actual counterspell.
func TestCounterSpellEffect_InstantStillSelectable(t *testing.T) {
	cancel := &Card{
		Name:  "Cancel",
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Cancel",
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "spell_effect",
						Args: []interface{}{
							&gameast.CounterSpell{
								Target: gameast.Filter{Base: "spell", Targeted: true},
							},
						},
					},
				},
			},
		},
	}
	if got := counterSpellEffect(cancel); got == nil {
		t.Fatal("Cancel-shape instant must remain a counter candidate")
	}
}

// TestPriorityRound_ResponseRemovesFromHand — end-to-end check that
// when a defender's response is selected via the Hat path, the card
// leaves hand before being pushed to the stack. The bug shape was:
// card on stack + card in hand simultaneously, which CardIdentity
// caught at every subsequent invariant tick until the spell resolved
// and the same *Card landed on the battlefield as well.
func TestPriorityRound_ResponseRemovesFromHand(t *testing.T) {
	gs := newFixtureGame(t)

	// Seat 0 owns the incoming spell ("Lightning Bolt" — proxy).
	bolt := &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Stack = append(gs.Stack, &StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       bolt,
	})

	// Seat 1 holds a counterspell (instant) with mana to cast it.
	counter := &Card{
		Name:  "Counterspell",
		Owner: 1,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "spell_effect",
						Args: []interface{}{
							&gameast.CounterSpell{
								Target: gameast.Filter{Base: "spell", Targeted: true},
							},
						},
					},
				},
			},
		},
	}
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, counter)
	gs.Seats[1].ManaPool = 10
	EnsureTypedPool(gs.Seats[1])

	// Drive PriorityRound — seat 1 should respond by casting Counterspell.
	gs.Active = 0
	PriorityRound(gs)

	// Counterspell must NOT be in seat 1's hand anymore.
	for _, c := range gs.Seats[1].Hand {
		if c == counter {
			t.Fatal("Counterspell pointer still in seat 1's hand after being pushed as response — CardIdentity will trip")
		}
	}

	// And the stack should have at least the counter on top (the bolt
	// may have been resolved-and-countered by DrainStack, but the
	// response must have been ATTEMPTED, which is what matters here).
	// The strict check is the hand invariant above.
	_ = gs.Stack
}
