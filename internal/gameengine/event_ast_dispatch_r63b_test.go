package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// event_ast_dispatch_r63b_test.go — PROGRESSION phase-3b engine
// findings:
//
//  1. gain_life / becomes_tapped AST triggers were silent — the aliases
//     mapped them to per_card-only FireCardTrigger events and no walk
//     consulted the AST (77 + 82 corpus shapes).
//  2. The §608.2b fizzle gate treated parser-mis-stamped zone-selector
//     bases (library_top) as required targets, fizzling every
//     impulse-exile trigger before resolveExile's library_top arm could
//     run (Abbot of Keral Keep class, ~20 audit divergences across five
//     trigger families, one root cause).

func dispGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(63)), nil)
}

func dispPerm(gs *GameState, seat int, name string, trig *gameast.Triggered) *Permanent {
	card := &Card{Name: name, Owner: seat, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	if trig != nil {
		card.AST = &gameast.CardAST{Name: name, Abilities: []gameast.Ability{trig}}
	}
	p := &Permanent{Card: card, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func drawOneTrigger(raw, event string) *gameast.Triggered {
	return &gameast.Triggered{
		Trigger: gameast.Trigger{Event: event},
		Effect: &gameast.Draw{
			Count:  *gameast.NumInt(1),
			Target: gameast.Filter{Base: "controller"},
		},
		Raw: raw,
	}
}

func TestGainLifeASTTrigger_FiresAndGates(t *testing.T) {
	gs := dispGame(t)
	dispPerm(gs, 0, "Test Pridemate",
		drawOneTrigger("whenever you gain life, draw a card", "gain_life"))
	gs.Seats[0].Library = []*Card{{Name: "L1", Owner: 0}, {Name: "L2", Owner: 0}}

	// FIRE: the controller gains life — once per gain EVENT.
	h0 := len(gs.Seats[0].Hand)
	GainLife(gs, 0, 3, "test")
	if got := len(gs.Seats[0].Hand) - h0; got != 1 {
		t.Fatalf("gain_life trigger drew %d, want 1 (was: NEVER fired pre-fix)", got)
	}

	// CONTROLLER GATE: the opponent gains — silent.
	h0 = len(gs.Seats[0].Hand)
	GainLife(gs, 1, 3, "test")
	if len(gs.Seats[0].Hand) != h0 {
		t.Fatalf("'you gain life' trigger fired for the opponent's gain")
	}
}

func TestBecomesTappedASTTrigger_SelfAndEnchanted(t *testing.T) {
	gs := dispGame(t)
	bearer := dispPerm(gs, 0, "Test Curiosity Bearer",
		drawOneTrigger("whenever this creature becomes tapped, draw a card", "becomes_tapped"))
	gs.Seats[0].Library = []*Card{{Name: "L1", Owner: 0}, {Name: "L2", Owner: 0}, {Name: "L3", Owner: 0}}

	h0 := len(gs.Seats[0].Hand)
	bearer.Tapped = true
	FireTapEventASTTriggers(gs, bearer)
	if got := len(gs.Seats[0].Hand) - h0; got != 1 {
		t.Fatalf("self becomes_tapped drew %d, want 1 (was: NEVER fired pre-fix)", got)
	}

	// Attached: aura on a host; host taps → aura's trigger fires.
	gs2 := dispGame(t)
	host := dispPerm(gs2, 0, "Host Bear", nil)
	aura := dispPerm(gs2, 0, "Test Tap Ward",
		drawOneTrigger("whenever enchanted creature becomes tapped, draw a card", "becomes_tapped"))
	aura.Card.Types = []string{"enchantment", "aura"}
	aura.AttachedTo = host
	gs2.Seats[0].Library = []*Card{{Name: "L1", Owner: 0}}
	h0 = len(gs2.Seats[0].Hand)
	host.Tapped = true
	FireTapEventASTTriggers(gs2, host)
	if got := len(gs2.Seats[0].Hand) - h0; got != 1 {
		t.Fatalf("enchanted becomes_tapped drew %d, want 1", got)
	}

	// A bystander tapping must not fire the bearer's self trigger.
	gs3 := dispGame(t)
	dispPerm(gs3, 0, "Test Curiosity Bearer",
		drawOneTrigger("whenever this creature becomes tapped, draw a card", "becomes_tapped"))
	v := dispPerm(gs3, 0, "Bystander", nil)
	gs3.Seats[0].Library = []*Card{{Name: "L1", Owner: 0}}
	h0 = len(gs3.Seats[0].Hand)
	v.Tapped = true
	FireTapEventASTTriggers(gs3, v)
	if len(gs3.Seats[0].Hand) != h0 {
		t.Fatalf("self becomes_tapped fired for a bystander's tap")
	}
}

func TestFizzleGate_LibraryTopIsNotATarget(t *testing.T) {
	gs := dispGame(t)
	// Abbot of Keral Keep shape: ETB "exile the top card of your
	// library" with the parser's erroneous Targeted=true on the
	// library_top base.
	trig := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "etb"},
		Effect: &gameast.Exile{
			Target: gameast.Filter{Base: "library_top", Targeted: true, Quantifier: "one"},
		},
		Raw: "when this creature enters, exile the top card of your library",
	}
	bearer := dispPerm(gs, 0, "Test Abbot", trig)
	gs.Seats[0].Library = []*Card{{Name: "Top", Owner: 0}, {Name: "Next", Owner: 0}}

	lib0, ex0 := len(gs.Seats[0].Library), len(gs.Seats[0].Exile)
	FirePermanentETBTriggers(gs, bearer)
	if len(gs.Seats[0].Exile)-ex0 != 1 || lib0-len(gs.Seats[0].Library) != 1 {
		t.Fatalf("impulse exile did not happen (lib %d->%d exile %d->%d) — §608.2b fizzled a zone selector",
			lib0, len(gs.Seats[0].Library), ex0, len(gs.Seats[0].Exile))
	}
	for _, ev := range gs.EventLog {
		if ev.Kind == "fizzle" {
			t.Fatalf("zone-selector effect fizzled: %v", ev.Details)
		}
	}
}
