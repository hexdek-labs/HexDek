package gameengine

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Loki r60 mega-stress / seeds 31415 + 271828 / nightmare boards
// surfaced 4 CardIdentity violations across 50,000 boards. The
// bit-stable signature on board 9751 (seed 31415):
//
//   CardIdentity: card "Necrogen Communion" (ptr 0xc009bc8360)
//   appears in both seat 2 graveyard and seat 2 stack
//
// Sequence: a nightmare board placed Necrogen Communion (an Aura)
// directly on seat 2's battlefield with no AttachedTo (the nightmare
// path skips `attachAuraOnETB`). SBA 704.5n then destroyed it as an
// aura with no legal target, sending the *Card to seat 2's graveyard.
// Necrogen Communion's AST has a `Triggered{event: "die"}` ability
// (parser approximation of "when enchanted creature dies"); when the
// aura itself died, the trigger fired and `PushTriggeredAbility` in
// stack.go:194-201 set `item.Card = src.Card` on the StackItem — a
// log-label, not a real zone reference. The CardIdentity invariant
// counted the *Card pointer in both seat 2 graveyard (real) AND seat
// 2 stack (label) and fired a false-positive duplication.
//
// Same r41-era "Card pointer in two zones" cluster Adric, Oketra,
// and Abuelo previously closed — but those were genuine engine
// duplications. This signature is an invariant false-positive: the
// stack ability item's Card field is documented as a log label, not
// a zone occupant.
//
// Fix in `checkCardIdentity` skips stack items whose Source != nil OR
// Kind is "triggered" / "activated" — those are abilities of a
// permanent that lives somewhere else; only Kind=="spell" + Source==
// nil stack items reflect a card actually moving through the stack
// zone (CR §405).

// TestCardIdentity_TriggeredAbilityStackItemDoesNotFalsePositive pins
// the bit-stable signature: a triggered ability with `item.Card` set
// for logging must not race the underlying card's true zone location.
func TestCardIdentity_TriggeredAbilityStackItemDoesNotFalsePositive(t *testing.T) {
	rng := rand.New(rand.NewSource(31415))
	gs := NewGameState(4, rng, nil)

	// Necrogen Communion's *Card lives in seat 2's graveyard (where SBA
	// 704.5n put it after destruction).
	communion := &Card{
		Name:  "Necrogen Communion",
		Owner: 2,
		Types: []string{"enchantment"},
	}
	gs.Seats[2].Graveyard = append(gs.Seats[2].Graveyard, communion)

	// A residual triggered-ability stack item references the same *Card
	// pointer as a log label. Source is the (now-destroyed) permanent.
	srcPerm := &Permanent{
		Card:       communion,
		Controller: 2,
		Owner:      2,
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Stack = append(gs.Stack, &StackItem{
		Controller: 2,
		Source:     srcPerm,
		Card:       communion, // log-label, not zone reference
		Effect:     &gameast.Reanimate{},
		Kind:       "triggered",
	})

	if err := checkCardIdentity(gs); err != nil {
		t.Fatalf("CardIdentity should not false-positive on a triggered ability's log-label Card; got: %v", err)
	}
}

// TestCardIdentity_ActivatedAbilityStackItemDoesNotFalsePositive
// confirms the same handling extends to activated-ability stack items.
// Activated abilities (Kind=="activated") set item.Card the same way
// for the same reason.
func TestCardIdentity_ActivatedAbilityStackItemDoesNotFalsePositive(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gs := NewGameState(2, rng, nil)

	card := &Card{Name: "Mishra's Bauble", Owner: 0, Types: []string{"artifact"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	srcPerm := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}
	gs.Stack = append(gs.Stack, &StackItem{
		Controller: 0,
		Source:     srcPerm,
		Card:       card,
		Kind:       "activated",
	})

	if err := checkCardIdentity(gs); err != nil {
		t.Fatalf("CardIdentity must not false-positive on activated ability stack item; got: %v", err)
	}
}

// TestCardIdentity_SpellStackItemStillDetectsRealDuplication pins the
// other direction: a SPELL stack item (Kind=="spell" with Source==nil)
// reflects a card genuinely moving through the stack zone. If the same
// pointer also appears in a graveyard, that's a real engine bug —
// the invariant must still fire.
//
// Defends against the regression where I narrowed the stack check too
// far and broke detection of the original r41-era duplications
// (Cerulean Sphinx, Adric, Oketra family).
func TestCardIdentity_SpellStackItemStillDetectsRealDuplication(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gs := NewGameState(2, rng, nil)

	dup := &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, dup)
	gs.Stack = append(gs.Stack, &StackItem{
		Controller: 0,
		Source:     nil, // genuine spell, not an ability
		Card:       dup,
		Kind:       "spell",
	})

	err := checkCardIdentity(gs)
	if err == nil {
		t.Fatal("CardIdentity must still fire on real card duplication across graveyard + stack (spell item)")
	}
	if !strings.Contains(err.Error(), "Lightning Bolt") {
		t.Fatalf("error should name the duplicated card; got: %v", err)
	}
}

// TestCardIdentity_MultipleAbilityStackItemsSameCard pins a stronger
// shape: even if N ability items on the stack all reference the same
// log-label *Card, the invariant still passes (because the underlying
// card lives in exactly one zone — graveyard — and the stack items
// are abstract abilities). The pre-fix invariant would have fired N
// times.
func TestCardIdentity_MultipleAbilityStackItemsSameCard(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	gs := NewGameState(2, rng, nil)

	card := &Card{Name: "Echoing Trigger", Owner: 0, Types: []string{"enchantment"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	srcPerm := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}

	for i := 0; i < 3; i++ {
		gs.Stack = append(gs.Stack, &StackItem{
			Controller: 0,
			Source:     srcPerm,
			Card:       card,
			Kind:       "triggered",
		})
	}

	if err := checkCardIdentity(gs); err != nil {
		t.Fatalf("CardIdentity must accept multiple ability stack items pointing at the same source card; got: %v", err)
	}
}
