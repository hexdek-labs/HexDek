package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// etb_ownership_double_fire_r63_test.go — regressions for the r63
// PROGRESSION double-fire fix: per_card families that FULLY implement a
// card's printed self-ETB triggered ability registered via OnETB but
// never declared OwnsETBTrigger, so the engine's generic AST push ALSO
// resolved the printed ability — applying the effect twice in real games
// (Bloodborn Scoundrels drained 4, Rune-Scarred Demon tutored 2, …).

// TestEtbOwnership_DeclaredForFullImplementers pins the ownership
// declaration so the etb-dispatch double-fire gate suppresses the AST push.
func TestEtbOwnership_DeclaredForFullImplementers(t *testing.T) {
	Reset() // exercise the reset-hook path — ownership must survive Reset()
	owned := []string{
		// drain family
		"Bloodborn Scoundrels", "Skymarch Bloodletter", "Vampire Sovereign",
		"Highway Robber", "Dakmor Ghoul",
		// tutor family + dedicated tutor
		"Trophy Mage", "Treasure Mage", "Trinket Mage", "Rune-Scarred Demon",
		// enters-or-attacks token, escape titans
		"Slimefoot and Squee", "Kroxa, Titan of Death's Hunger",
		"Uro, Titan of Nature's Wrath",
	}
	for _, name := range owned {
		if !OwnsETB(name) {
			t.Errorf("OwnsETB(%q) = false; the full-implementing OnETB handler must declare OwnsETBTrigger", name)
		}
	}
}

// drainETBAST is the printed "loses N / gain N" trigger the drain family's
// cards carry — exactly the AST that would double-resolve without the gate.
func drainETBAST(n int) *gameast.CardAST {
	return &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "etb"},
				Effect: &gameast.Sequence{Items: []gameast.Effect{
					&gameast.LoseLife{
						Amount: gameast.NumberOrRef{IsInt: true, Int: n},
						Target: gameast.Filter{Base: "opponent", Targeted: true},
					},
					&gameast.GainLife{
						Amount: gameast.NumberOrRef{IsInt: true, Int: n},
						Target: gameast.Filter{Base: "self"},
					},
				}},
				Raw: "when this creature enters, target opponent loses 2 life and you gain 2 life",
			},
		},
	}
}

// TestEtbDrain_FiresOnceNotTwice: Bloodborn Scoundrels carries the printed
// AST drain AND the per_card handler. With ownership declared, only the
// handler resolves — the opponent loses 2, not 4.
func TestEtbDrain_FiresOnceNotTwice(t *testing.T) {
	gs := newGame(t, 2)
	oppBefore := gs.Seats[1].Life
	meBefore := gs.Seats[0].Life

	bearer := addPerm(gs, 0, "Bloodborn Scoundrels", "creature")
	bearer.Card.AST = drainETBAST(2)

	gameengine.FirePermanentETBTriggers(gs, bearer)

	if got := oppBefore - gs.Seats[1].Life; got != 2 {
		t.Fatalf("opponent should lose 2 (single fire), lost %d — AST push double-fired with the per_card handler", got)
	}
	if got := gs.Seats[0].Life - meBefore; got != 2 {
		t.Fatalf("controller should gain 2 (single fire), gained %d", got)
	}
}
