package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — DELAYED-return flicker (Flickerwisp / Otherworldly Journey / Long Road
// Home): "exile … return … at the beginning of the next end step". Before r63
// the return clause was dropped and the permanent was exiled FOREVER.

func staticRawCard(name, raw string) *Card {
	return &Card{
		Name:  name,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{&gameast.Static{Raw: raw}},
		},
	}
}

func TestDelayedFlicker_SpecDetection(t *testing.T) {
	cases := []struct {
		name, raw                  string
		wantDelayed, wantYC, wantP bool
	}{
		{"Flickerwisp", "When this creature enters, exile another target permanent. Return that card to the battlefield under its owner's control at the beginning of the next end step.", true, false, false},
		{"Otherworldly Journey", "Exile target creature. At the beginning of the next end step, return that card to the battlefield under its owner's control with a +1/+1 counter on it.", true, false, true},
		{"Cloudshift (immediate, NOT delayed)", "Exile target creature you control, then return that card to the battlefield under your control.", false, false, false},
	}
	for _, c := range cases {
		src := &Permanent{Card: staticRawCard(c.name, c.raw)}
		gotD, gotYC, gotP := exileDelayedFlickerSpec(src)
		if gotD != c.wantDelayed || gotYC != c.wantYC || gotP != c.wantP {
			t.Errorf("%s: got (delayed=%v yourCtrl=%v plusOne=%v), want (%v %v %v)",
				c.name, gotD, gotYC, gotP, c.wantDelayed, c.wantYC, c.wantP)
		}
	}
}

// Returns at the next end step as a new object on the battlefield.
func TestDelayedFlicker_ReturnsAtEndStep(t *testing.T) {
	gs := newFixtureGame(t)
	bear := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	// Card sits in owner's exile (post-exile state).
	gs.Seats[0].Exile = []*Card{bear}

	RegisterDelayedFlickerReturn(gs, bear, 0, false, "Flickerwisp")
	if bfCount(gs, 0, "Grizzly Bears") != 0 {
		t.Fatal("should not be on battlefield before the end step")
	}
	FireDelayedTriggers(gs, "ending", "end")
	if bfCount(gs, 0, "Grizzly Bears") != 1 {
		t.Fatalf("card should have returned to battlefield at end step; got %d", bfCount(gs, 0, "Grizzly Bears"))
	}
	// And it left exile.
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("card should have left exile on return, exile=%d", len(gs.Seats[0].Exile))
	}
}

// Otherworldly Journey / Long Road Home return WITH a +1/+1 counter.
func TestDelayedFlicker_ReturnsWithPlusOneCounter(t *testing.T) {
	gs := newFixtureGame(t)
	bear := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	gs.Seats[0].Exile = []*Card{bear}

	RegisterDelayedFlickerReturn(gs, bear, 0, true, "Otherworldly Journey")
	FireDelayedTriggers(gs, "ending", "end")
	var np *Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Grizzly Bears" {
			np = p
		}
	}
	if np == nil {
		t.Fatal("creature did not return")
	}
	if np.Counters["+1/+1"] != 1 {
		t.Errorf("should return with one +1/+1 counter, got %d", np.Counters["+1/+1"])
	}
}

// If the card is no longer in exile (moved away / token ceased), it does NOT
// return (the delayed trigger is keyed to the exiled object — CR §603.7).
func TestDelayedFlicker_GoneFromExileDoesNotReturn(t *testing.T) {
	gs := newFixtureGame(t)
	bear := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	// NOT in exile (e.g. a token that ceased, or moved to graveyard).
	RegisterDelayedFlickerReturn(gs, bear, 0, false, "Flickerwisp")
	FireDelayedTriggers(gs, "ending", "end")
	if bfCount(gs, 0, "Grizzly Bears") != 0 {
		t.Fatalf("card not in exile must not return; got %d on battlefield", bfCount(gs, 0, "Grizzly Bears"))
	}
}
