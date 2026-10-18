package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r180 — cardHasObserverETBCandidate short-circuits the observer-ETB scan
// for cards that can never match it. These tests pin the equivalence.
//
// The risk being tested is NOT "does the memo cache" — it is "does the
// candidate test accept everything the real loop could act on". A
// candidate test that is too narrow silently makes real triggers stop
// firing, which is an ELO-corrupting rules bug that no crash would reveal.

// legacyObserverETBCandidate re-derives the acceptance condition directly
// from the shape of the loop body in fireObserverETBTriggers, independently
// of the production helper. The two must agree on every input.
func legacyObserverETBCandidate(c *Card) bool {
	if c == nil || c.AST == nil {
		return false
	}
	for _, ab := range c.AST.Abilities {
		trig, landfall := observerTriggeredFromAbility(ab)
		if trig == nil || trig.Effect == nil {
			continue
		}
		if landfall {
			return true
		}
		if EventEquals(trig.Trigger.Event, "etb") {
			return true
		}
	}
	return false
}

// r180EventVocab mixes events that alias to "etb" with events that do not,
// so the corpus contains both true positives and near misses.
var r180EventVocab = []string{
	"etb", "etb_as", "comes_into_play", "put_onto_bf",
	"another_typed_enters", "tribe_you_control_etb", "ally_etb",
	"etb_or_another", "another_creature_enters", "creature_etb_any",
	"die", "ltb", "attack", "block", "cast", "untap", "transform",
	"phase", "deal_combat_damage", "discover", "", "ETB", "  etb  ",
	"not_an_event", "etb_like_but_not", "enters_the_battlefield",
}

func r180Effect() gameast.Effect {
	return &gameast.Draw{Count: gameast.NumberOrRef{}}
}

// r180RandomAbility builds one ability of a randomly chosen shape: a
// Triggered, a Triggered with a nil Effect, a landfall ability_word
// wrapper, a non-landfall ability_word wrapper, a Keyword, or a Static.
func r180RandomAbility(rng *rand.Rand) gameast.Ability {
	ev := r180EventVocab[rng.Intn(len(r180EventVocab))]
	switch rng.Intn(7) {
	case 0:
		return &gameast.Triggered{Trigger: gameast.Trigger{Event: ev}, Effect: r180Effect()}
	case 1:
		// Nil effect — must never be a candidate on its own.
		return &gameast.Triggered{Trigger: gameast.Trigger{Event: ev}}
	case 2:
		// Landfall wrapper: a candidate regardless of the inner event.
		inner := &gameast.Triggered{Trigger: gameast.Trigger{Event: ev}, Effect: r180Effect()}
		return &gameast.Static{Modification: &gameast.Modification{
			ModKind: "ability_word",
			Args:    []interface{}{"landfall", "triggered", inner},
		}}
	case 3:
		// Landfall wrapper whose inner trigger has NO effect.
		inner := &gameast.Triggered{Trigger: gameast.Trigger{Event: ev}}
		return &gameast.Static{Modification: &gameast.Modification{
			ModKind: "ability_word",
			Args:    []interface{}{"landfall", "triggered", inner},
		}}
	case 4:
		// A DIFFERENT ability word — must not be unwrapped as landfall.
		inner := &gameast.Triggered{Trigger: gameast.Trigger{Event: ev}, Effect: r180Effect()}
		word := []string{"constellation", "magecraft", "heroic", "raid"}[rng.Intn(4)]
		return &gameast.Static{Modification: &gameast.Modification{
			ModKind: "ability_word",
			Args:    []interface{}{word, "triggered", inner},
		}}
	case 5:
		return &gameast.Keyword{Name: "flying"}
	default:
		return &gameast.Static{Raw: "r180 filler text"}
	}
}

func r180RandomCard(rng *rand.Rand, name string) *Card {
	n := rng.Intn(6) // 0..5 abilities, so empty ASTs are covered
	abs := make([]gameast.Ability, 0, n)
	for i := 0; i < n; i++ {
		abs = append(abs, r180RandomAbility(rng))
	}
	return &Card{Name: name, Types: []string{"creature"}, AST: &gameast.CardAST{Abilities: abs}}
}

// The headline test: over a large random corpus the memoized helper and
// the independently re-derived condition must agree EXACTLY.
func TestR180_ObserverETBCandidate_MatchesLoopCondition(t *testing.T) {
	rng := rand.New(rand.NewSource(1180))
	const corpus = 5000
	agreed := 0
	trues := 0
	for i := 0; i < corpus; i++ {
		c := r180RandomCard(rng, "r180 card")
		want := legacyObserverETBCandidate(c)
		got := cardHasObserverETBCandidate(c)
		if got != want {
			t.Fatalf("card %d: cardHasObserverETBCandidate=%v, loop condition=%v\nAST=%#v",
				i, got, want, c.AST.Abilities)
		}
		// Second call must come off the memo with the same answer.
		if again := cardHasObserverETBCandidate(c); again != got {
			t.Fatalf("card %d: memo returned %v then %v", i, got, again)
		}
		agreed++
		if want {
			trues++
		}
	}
	// A corpus that is all-false (or all-true) would pass vacuously and
	// prove nothing. Demand both directions be well represented.
	if trues < corpus/10 || trues > corpus*9/10 {
		t.Fatalf("degenerate corpus: %d/%d candidates — the test is not "+
			"exercising both directions", trues, corpus)
	}
	t.Logf("r180: %d cards agreed, %d candidates (%.1f%%)",
		agreed, trues, 100*float64(trues)/float64(corpus))
}

// The memo is keyed on the AST pointer, not a ready bool, so a transform
// that reassigns Card.AST must change the answer. This is the r157 defect
// and it must stay impossible to reintroduce.
func TestR180_ObserverETBCandidate_IsASTKeyed(t *testing.T) {
	c := &Card{Name: "r180 transformer", AST: &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Keyword{Name: "flying"},
	}}}
	if cardHasObserverETBCandidate(c) {
		t.Fatal("front face has no triggered ability; want false")
	}
	// Transform: reassign the AST in place, as dfc.go does.
	c.AST = &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Triggered{
			Trigger: gameast.Trigger{Event: "another_creature_enters"},
			Effect:  r180Effect(),
		},
	}}
	if !cardHasObserverETBCandidate(c) {
		t.Fatal("back face observes creature ETBs, but the memo returned " +
			"the front face's answer — the AST key is not working")
	}
	// And back again.
	c.AST = &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "flying"}}}
	if cardHasObserverETBCandidate(c) {
		t.Fatal("reverted to front face; want false")
	}
}

// A nil card and a card with no AST must be false, not a panic: the scan
// already skips them, but the helper is the thing being trusted now.
func TestR180_ObserverETBCandidate_NilSafe(t *testing.T) {
	if cardHasObserverETBCandidate(nil) {
		t.Fatal("nil card: want false")
	}
	if cardHasObserverETBCandidate(&Card{Name: "no ast"}) {
		t.Fatal("nil AST: want false")
	}
}

// End-to-end: a real observer must still fire through the gated scan.
// This is the test that would catch a candidate check that is too narrow.
func TestR180_ObserverStillFiresThroughGate(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	// EventPolicy zero value retains everything.

	observerCard := &Card{
		Name:  "R180 Soul Watcher",
		Types: []string{"creature"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event: "another_creature_enters",
					Actor: &gameast.Filter{Base: "another_creature"},
				},
				Effect: r180Effect(),
			},
		}},
	}
	observer := &Permanent{Card: observerCard, Controller: 0, Owner: 0}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, observer)

	enteringCard := &Card{Name: "R180 Bear", Types: []string{"creature"}, AST: &gameast.CardAST{}}
	entering := &Permanent{Card: enteringCard, Controller: 0, Owner: 0}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, entering)

	fireObserverETBTriggers(gs, entering)

	for _, ev := range gs.EventLog {
		if ev.Kind == "trigger_fires" && ev.Source == "R180 Soul Watcher" {
			return
		}
	}
	t.Fatal("observer ETB trigger did not fire through the r180 gate — " +
		"the candidate check is too narrow")
}
