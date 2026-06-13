package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Generic self_enters_tapped coverage: an unconditional "enters tapped"
// permanent (Guildgate, Temple, tapped mana rock) now enters tapped. The
// mis-bucketed "enters prepared" form (Args=["prepared"]) is left alone.

func mkSelfTapped(gs *GameState, seat string, kind string, args []interface{}, types ...string) *Permanent {
	card := &Card{
		Name:  "Test Tapped Perm",
		Owner: 0,
		Types: append([]string{}, types...),
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Modification: &gameast.Modification{ModKind: kind, Args: args}},
			},
		},
	}
	return &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
}

func TestSelfEntersTapped_LandTaps(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkSelfTapped(gs, "0", "self_enters_tapped", nil, "land")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	FirePermanentETBTriggers(gs, p)
	if !p.Tapped {
		t.Fatalf("unconditional self_enters_tapped land must enter TAPPED")
	}
	if p.Flags["enters_tapped"] != 1 {
		t.Fatalf("expected enters_tapped flag")
	}
}

func TestSelfEntersTapped_ArtifactRockTaps(t *testing.T) {
	gs := newTestGame(t, 2)
	// Moss-Diamond-style tapped mana rock.
	p := mkSelfTapped(gs, "0", "enters_tapped", nil, "artifact")
	ApplySelfEntersTapped(gs, p)
	if !p.Tapped {
		t.Fatalf("tapped mana rock must enter TAPPED")
	}
}

func TestSelfEntersTapped_PreparedFormSkipped(t *testing.T) {
	gs := newTestGame(t, 2)
	// "This creature enters prepared" is mis-bucketed under this kind with
	// Args=["prepared"] — must NOT be tapped by this handler.
	p := mkSelfTapped(gs, "0", "self_enters_tapped", []interface{}{"prepared"}, "creature")
	p.Card.BasePower = 2
	p.Card.BaseToughness = 2
	ApplySelfEntersTapped(gs, p)
	if p.Tapped {
		t.Fatalf("'enters prepared' form must NOT be tapped by self_enters_tapped handler")
	}
}

func TestSelfEntersTapped_NoStaticNoTap(t *testing.T) {
	gs := newTestGame(t, 2)
	// A plain land with no enters-tapped static stays untapped.
	card := &Card{Name: "Plains", Owner: 0, Types: []string{"land", "basic"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}}}
	p := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{}}
	ApplySelfEntersTapped(gs, p)
	if p.Tapped {
		t.Fatalf("a permanent without an enters-tapped static must NOT be tapped")
	}
}
