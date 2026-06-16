package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Typed-ETB observer dispatch (r63). "Whenever an artifact/enchantment you
// control enters, ..." parses to a top-level *Triggered with Actor==nil and
// event artifact_you_control_enters / enchantment_you_control_enters. These
// events were absent from observerOnlyEvents, so isSelfTrigger misclassified
// them as self-triggers and fireObserverETBTriggers skipped them — the
// payoff was inert despite observerETBMatchesByRaw matching their Raw
// correctly. These tests pin that they fire, your-control-only, by type.

func typedETBObserverPayoff(gs *GameState, seat int, event, raw string) {
	c := &Card{
		Name:          "Typed-ETB Payoff " + event,
		Owner:         seat,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
	}
	c.AST = &gameast.CardAST{
		Name: c.Name,
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: event},
				Effect: &gameast.GainLife{
					Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
					Target: gameast.Filter{Base: "controller"},
				},
				Raw: raw,
			},
		},
	}
	p := &Permanent{
		Card:       c,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
}

func enterTypedPermanent(gs *GameState, seat int, types ...string) {
	c := &Card{Name: "Typed Enterer", Owner: seat, Types: types}
	p := &Permanent{
		Card:       c,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	FirePermanentETBTriggers(gs, p)
	for i := 0; i < 50 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

func TestTypedETBObserver_Dispatch(t *testing.T) {
	cases := []struct {
		name       string
		event      string
		raw        string
		enterSeat  int
		enterTypes []string
		wantFire   bool
	}{
		{"own artifact fires", "artifact_you_control_enters",
			"whenever an artifact you control enters, you gain 1 life",
			0, []string{"artifact"}, true},
		{"opponent artifact gated out", "artifact_you_control_enters",
			"whenever an artifact you control enters, you gain 1 life",
			1, []string{"artifact"}, false},
		{"own enchantment fires", "enchantment_you_control_enters",
			"whenever an enchantment you control enters, you gain 1 life",
			0, []string{"enchantment"}, true},
		{"opponent enchantment gated out", "enchantment_you_control_enters",
			"whenever an enchantment you control enters, you gain 1 life",
			1, []string{"enchantment"}, false},
		{"own enchantment-creature token fires", "enchantment_you_control_enters",
			"whenever an enchantment you control enters, you gain 1 life",
			0, []string{"enchantment", "creature", "token"}, true},
		{"non-matching type inert", "artifact_you_control_enters",
			"whenever an artifact you control enters, you gain 1 life",
			0, []string{"creature"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewGameState(2, rand.New(rand.NewSource(5)), nil)
			for _, s := range gs.Seats {
				s.Life = 40
			}
			typedETBObserverPayoff(gs, 0, tc.event, tc.raw)
			before := gs.Seats[0].Life
			enterTypedPermanent(gs, tc.enterSeat, tc.enterTypes...)
			fired := gs.Seats[0].Life != before
			if fired != tc.wantFire {
				t.Fatalf("%s: fired=%v want=%v (lifeΔ=%d)",
					tc.name, fired, tc.wantFire, gs.Seats[0].Life-before)
			}
		})
	}
}

// Two qualifying permanents entering separately each trigger the observer
// once (per-permanent dispatch, CR §603.2).
func TestTypedETBObserver_OncePerPermanent(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(5)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	typedETBObserverPayoff(gs, 0, "artifact_you_control_enters",
		"whenever an artifact you control enters, you gain 1 life")
	before := gs.Seats[0].Life
	enterTypedPermanent(gs, 0, "artifact")
	enterTypedPermanent(gs, 0, "artifact")
	if got := gs.Seats[0].Life - before; got != 2 {
		t.Fatalf("two artifacts: lifeΔ=%d want=2", got)
	}
}
