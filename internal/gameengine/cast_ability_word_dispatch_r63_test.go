package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// cast_ability_word_dispatch_r63_test.go — dead-dispatch follow-up #4
// (CAST class). The cast spell's OWN ability-word cast trigger
// ("Morbid/Undergrowth — When you cast this spell, …") was inert: it parses
// to a Static{ability_word → Triggered} wrapper the top-level scans skip, and
// the spell being cast isn't a battlefield permanent so the observer cast
// scan misses it. Fixed in fireCastTriggersFromZone with a transient source.
// heroic/magecraft (their own cast hooks) are excluded — no double-fire.

func castGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(29)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func castDrain(gs *GameState) {
	for i := 0; i < 80 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

// castAbilityWordCard builds a sorcery whose AST is a Static{ability_word
// word → Triggered{cast_spell, GainLife controller+1}} ("when you cast this
// spell, …") wrapper.
func castAbilityWordCard(word string) *Card {
	inner := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "cast_spell"},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "controller"}},
		Raw:     word + " — when you cast this spell, you gain 1 life",
	}
	c := &Card{Name: "Probe " + word, Owner: 0, Types: []string{"sorcery"}}
	c.AST = &gameast.CardAST{Name: c.Name, Abilities: []gameast.Ability{
		&gameast.Static{Modification: &gameast.Modification{ModKind: "ability_word", Args: []interface{}{word, "triggered", inner}}},
	}}
	return c
}

// Cast ability words (morbid/undergrowth) fire when their spell is cast.
func TestCastAbilityWord_FiresOnCast(t *testing.T) {
	for _, word := range []string{"morbid", "undergrowth"} {
		gs := castGame(t)
		life := gs.Seats[0].Life
		fireCastTriggers(gs, 0, castAbilityWordCard(word))
		castDrain(gs)
		if got := gs.Seats[0].Life - life; got != 1 {
			t.Fatalf("cast ability word %q did not fire on cast (lifeΔ=%d, want 1)", word, got)
		}
	}
}

// heroic / magecraft are NOT fired by this new self-cast dispatch (they own
// dedicated cast hooks) — so a heroic/magecraft-worded inner GainLife is not
// resolved by the generic path → no double-fire.
func TestCastAbilityWord_DedicatedWordsExcluded(t *testing.T) {
	for _, word := range []string{"heroic", "magecraft"} {
		gs := castGame(t)
		life := gs.Seats[0].Life
		fireCastTriggers(gs, 0, castAbilityWordCard(word))
		castDrain(gs)
		if gs.Seats[0].Life != life {
			t.Fatalf("dedicated-dispatch word %q was fired by the generic cast dispatch (double-fire risk): lifeΔ=%d", word, gs.Seats[0].Life-life)
		}
	}
}

// Per_card double-fire guard: when a per_card handler owns the card's
// spell_cast event, the generic self-cast dispatch is skipped.
func TestCastAbilityWord_PerCardGuard(t *testing.T) {
	gs := castGame(t)
	card := castAbilityWordCard("morbid")

	old := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == card.DisplayName() && event == "spell_cast"
	}
	defer func() { HasTriggerHook = old }()

	life := gs.Seats[0].Life
	fireCastTriggers(gs, 0, card)
	castDrain(gs)
	if gs.Seats[0].Life != life {
		t.Fatalf("generic cast dispatch fired despite a per_card spell_cast handler (double-fire): lifeΔ=%d", gs.Seats[0].Life-life)
	}
}
