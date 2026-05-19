package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestResolveLoseLife_HalfPerTarget verifies that LoseLife with
// Amount="half" computes the loss per-target as half the target's
// current life. Used by Pox Plague / Fraying Omnipotence ("each
// player loses half their life"). Before this fix, evalNumber returned
// 0 for the "half" string and the resolver early-returned.
func TestResolveLoseLife_HalfPerTarget(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Pox Plague Stand-In", 0, 0, "sorcery")
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 13

	eff := &gameast.LoseLife{
		Amount: *gameast.NumStr("half"),
		// "each_player" is the AST shape that exercises the fan-out path;
		// pickPlayerTarget returns nil for it, and resolveLoseLife's
		// empty-target fallback now expands to every seat when the
		// filter base is each_player / player / "each player".
		Target: gameast.Filter{Base: "each_player"},
	}
	ResolveEffect(gs, src, eff)

	// Round-down semantics (Pox Plague). 20/2=10, 13/2=6.
	if gs.Seats[0].Life != 10 {
		t.Errorf("seat 0 life=%d, want 10", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 7 {
		t.Errorf("seat 1 life=%d, want 7", gs.Seats[1].Life)
	}
}

// TestResolveLoseLife_HalfSingleTarget verifies that LoseLife with
// Amount="half" and Filter.Base="player" (which pickPlayerTarget
// resolves to the source's controller for an untargeted "player")
// still applies the per-target half computation — half of THAT
// player's current life, not zero.
func TestResolveLoseLife_HalfSingleTarget(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Half-Life Source", 0, 0, "sorcery")
	gs.Seats[0].Life = 14

	eff := &gameast.LoseLife{
		Amount: *gameast.NumStr("half"),
		Target: gameast.Filter{Base: "player"},
	}
	ResolveEffect(gs, src, eff)

	if gs.Seats[0].Life != 7 {
		t.Errorf("seat 0 life=%d, want 7", gs.Seats[0].Life)
	}
}

// TestPickPlayerTarget_ThatPlayerNeverController verifies that
// Filter.Base="that_player" with targeted=false picks an opponent, not
// the controller. Fishing Gear ("exile the top card of that player's
// library") previously resolved against the controller's empty graveyard
// because pickPlayerTarget collapsed untargeted "that_player" to srcSeat.
func TestPickPlayerTarget_ThatPlayerNeverController(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Fishing Gear Stand-In", 0, 0, "artifact")

	for _, targeted := range []bool{true, false} {
		f := gameast.Filter{Base: "that_player", Targeted: targeted}
		targets := PickTarget(gs, src, f)
		if len(targets) == 0 {
			t.Fatalf("targeted=%v: PickTarget returned no targets", targeted)
		}
		seat, ok := seatFromTarget(targets[0])
		if !ok {
			t.Fatalf("targeted=%v: target has no seat", targeted)
		}
		if seat == 0 {
			t.Errorf("targeted=%v: that_player resolved to controller seat 0; want opponent", targeted)
		}
	}
}

// TestResolveTurnFaceUp_FallsBackToFaceDownCreature verifies that when
// Filter.Base="self" but the source isn't a face-down creature (the
// parser-confused shape used for Expose the Culprit's "turn target
// face-down creature face up"), the resolver searches the battlefield
// for any face-down creature and turns it face up.
func TestResolveTurnFaceUp_FallsBackToFaceDownCreature(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Expose the Culprit Stand-In", 0, 0, "instant")

	hidden := addBattlefield(gs, 0, "Hidden Creature", 2, 2, "creature")
	hidden.Card.FaceDown = true

	eff := &gameast.TurnFaceUp{
		Target: gameast.Filter{Base: "self"},
	}
	ResolveEffect(gs, src, eff)

	if hidden.Card.FaceDown {
		t.Fatalf("expected hidden creature to be turned face up; still face-down")
	}
}
