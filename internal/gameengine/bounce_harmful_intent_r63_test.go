package gameengine

// Regression for the own-land bounce policy bug (r63 OUTCOME phase-4
// residual, fixed phase 6).
//
// resolveBounce used the NEUTRAL PickTarget: own permanents score
// -toughness and opponents score power, so 0/0 categories (lands,
// artifacts, enchantments) tied at 0 and candidate order picked the
// caster's OWN permanent first — "return target land to its owner's
// hand" (Aven Fogbringer, Glowing Anemone) bounced the controller's
// own tapped land. Same root cause as the destroy-own-land cluster
// (finding #2) that was already fixed by routing destroy/exile/damage
// through PickTargetHarmful; bounce had been left on the neutral path.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func bounceTestState(t *testing.T) (*GameState, *Permanent) {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(63)), nil)
	src := &Permanent{
		Card:       &Card{Name: "Fogbringer", Owner: 0, Types: []string{"creature"}, BasePower: 1, BaseToughness: 3},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	ownLand := &Permanent{
		Card:       &Card{Name: "Own Wastes", Owner: 0, Types: []string{"land", "basic"}},
		Controller: 0, Owner: 0, Tapped: true,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	oppLand := &Permanent{
		Card:       &Card{Name: "Opp Wastes", Owner: 1, Types: []string{"land", "basic"}},
		Controller: 1, Owner: 1,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src, ownLand)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, oppLand)
	return gs, src
}

func TestBounce_TargetLand_PicksOpponentLand(t *testing.T) {
	gs, src := bounceTestState(t)

	ResolveEffect(gs, src, &gameast.Bounce{
		Target: gameast.Filter{Base: "land", Quantifier: "one", Targeted: true},
		To:     "owners_hand",
	})

	if len(gs.Seats[0].Battlefield) != 2 {
		t.Errorf("controller's battlefield changed (%d perms, want 2) — harmful bounce hit its own land",
			len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("opponent's land not bounced (%d perms on their battlefield, want 0)",
			len(gs.Seats[1].Battlefield))
	}
	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("bounced land not in its owner's hand (opp hand = %d, want 1)", len(gs.Seats[1].Hand))
	}
}

func TestBounce_YouControlFilter_StillResolvesOwnSide(t *testing.T) {
	// Harmful intent must not break "return target creature you
	// control" — the control filter restricts candidates to the own
	// side before scoring.
	gs, src := bounceTestState(t)
	ownBear := &Permanent{
		Card:       &Card{Name: "Own Bear", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, ownBear)

	ResolveEffect(gs, src, &gameast.Bounce{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true, YouControl: true},
		To:     "owners_hand",
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("own-side bounce did not return a controller creature to hand (hand = %d, want 1)",
			len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[1].Battlefield) != 1 {
		t.Errorf("you-control bounce crossed to the opponent's battlefield (%d perms, want 1)",
			len(gs.Seats[1].Battlefield))
	}
}
