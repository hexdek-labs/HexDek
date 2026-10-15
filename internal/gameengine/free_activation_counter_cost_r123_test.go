package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r123 — a counter-removal cost the parser cannot read is a cost that is
// never paid, which makes the ability FREE.
//
// activation.go pays a "remove N <kind> counters" activation cost (CR
// §602.1b) via RemoveCounterCostSpec, which returns ok=false for any shape
// it cannot parse with confidence. That decline is deliberate — enforcement
// must never wrongly BLOCK a legal activation — but it means the boundary of
// the parser is the boundary of the enforcement, and the parser's number-word
// list stopped at "ten".
//
// Fire Nation Turret's ONLY cost is "remove fifty charge counters", with no
// mana and no tap. Before r123 it accepted 5 of 5 activations holding zero
// counters and took an opponent from 20 to -30 life.

// r123Turret builds the real shape: a single activated ability whose only
// cost is a fixed counter removal expressed as an English number word.
func r123Turret(t *testing.T, word string, held int) (*GameState, *Permanent, []Target) {
	t.Helper()
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Fire Nation Turret", 0, 0, "artifact")
	p.Counters = map[string]int{"charge": held}
	p.Card.AST = &gameast.CardAST{Name: "Fire Nation Turret", Abilities: []gameast.Ability{
		&gameast.Activated{
			Cost: gameast.Cost{Extra: []string{
				"remove " + word + " charge counters from this artifact"}},
			Effect: &gameast.Damage{
				Amount: gameast.NumberOrRef{IsInt: true, Int: 50},
				Target: gameast.Filter{Base: "target", Quantifier: "any"},
			},
		},
	}}
	gs.Active = 0
	victim := addCreature(gs, 1, "r123 Victim", 5, 5)
	return gs, p, []Target{{Kind: TargetKindPermanent, Permanent: victim, Seat: 1}}
}

// Without the counters, the activation must be rejected — not silently free.
func TestFreeActivationR123_FiftyIsUnaffordableAtZero(t *testing.T) {
	gs, p, tg := r123Turret(t, "fifty", 0)
	if err := ActivateAbility(gs, 0, p, 0, tg); err == nil {
		t.Fatalf("activation ACCEPTED with 0 of 50 charge counters — the cost is free")
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("opponent life = %d, want 20 (no damage should have been dealt)", gs.Seats[1].Life)
	}
}

// With the counters, it must still be payable, and must actually spend them.
func TestFreeActivationR123_FiftyIsPayableAndSpends(t *testing.T) {
	gs, p, tg := r123Turret(t, "fifty", 50)
	if err := ActivateAbility(gs, 0, p, 0, tg); err != nil {
		t.Fatalf("activation REJECTED holding exactly 50 counters: %v", err)
	}
	if got := p.Counters["charge"]; got != 0 {
		t.Errorf("charge counters after = %d, want 0 — the cost must be spent", got)
	}
	// Second activation must now fail: the resource is gone.
	if err := ActivateAbility(gs, 0, p, 0, tg); err == nil {
		t.Errorf("second activation ACCEPTED after the counters were spent — still free")
	}
}

// "twelve" is the other corpus word outside the old list (Trenzalore
// Clocktower). Pinning the boundary itself rather than one card.
func TestFreeActivationR123_TwelveIsUnaffordableAtEleven(t *testing.T) {
	gs, p, tg := r123Turret(t, "twelve", 11)
	if err := ActivateAbility(gs, 0, p, 0, tg); err == nil {
		t.Fatalf("activation ACCEPTED with 11 of 12 counters")
	}
}

// The shapes the parser still declines must keep declining — removing zero
// is a legal payment and these effects scale off "counters removed this way",
// so enforcement here would wrongly block a legal activation.
func TestFreeActivationR123_VariableShapesStillDecline(t *testing.T) {
	for _, w := range []string{"x", "any", "all"} {
		if _, _, ok := RemoveCounterCostSpec(gameast.Cost{Extra: []string{
			"remove " + w + " charge counters from this artifact"}}); ok {
			t.Errorf("%q now parses as a fixed count — it is a player choice, not a number", w)
		}
	}
}
