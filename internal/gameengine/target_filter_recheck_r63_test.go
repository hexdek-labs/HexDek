package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CR §608.2b — at resolution, each target's legality is re-checked against
// the spell/ability's target FILTER, not just shroud/hexproof/protection.
// Regression for the r63 target-legality probe: isTargetStillLegal checked
// only zone-presence + protection, so a target that became an illegal TYPE
// or lost a required STATE / control restriction since targeting still
// resolved.

func tfr_game() *GameState { return NewGameState(2, rand.New(rand.NewSource(2)), nil) }

func tfr_perm(gs *GameState, seat int, name string, types ...string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Owner: seat, Types: append([]string(nil), types...), BasePower: 2, BaseToughness: 2},
		Controller: seat, Owner: seat, Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func tfr_item(controller int, filter gameast.Filter, tgt *Permanent) *StackItem {
	return &StackItem{
		Controller: controller,
		Effect:     &gameast.Destroy{Target: filter},
		Targets:    []Target{{Kind: TargetKindPermanent, Permanent: tgt}},
	}
}

// (d) "target tapped creature" that untapped since targeting → illegal.
func TestTargetRecheck_TappedRestriction_FizzlesWhenUntapped(t *testing.T) {
	gs := tfr_game()
	tgt := tfr_perm(gs, 1, "Bear", "creature")
	f := gameast.Filter{Base: "creature", Targeted: true, Extra: []string{"tapped"}}

	// Still tapped → legal.
	tgt.Tapped = true
	if ai, _ := CheckTargetLegality(gs, tfr_item(0, f, tgt)); ai {
		t.Error("a still-tapped creature must remain a legal 'target tapped creature'")
	}
	// Untapped in response → illegal at resolution.
	tgt.Tapped = false
	ai, legal := CheckTargetLegality(gs, tfr_item(0, f, tgt))
	if !ai || len(legal) != 0 {
		t.Errorf("'target tapped creature' must fizzle once the creature untaps; allIllegal=%v legal=%d", ai, len(legal))
	}
}

// (c) "target creature" on a permanent that's no longer a creature → illegal.
func TestTargetRecheck_TypeChange_FizzlesWhenNoLongerCreature(t *testing.T) {
	gs := tfr_game()
	tgt := tfr_perm(gs, 1, "Mishra's Bauble Golem", "creature", "artifact")
	f := gameast.Filter{Base: "creature", Targeted: true}

	// Still a creature → legal.
	if ai, _ := CheckTargetLegality(gs, tfr_item(0, f, tgt)); ai {
		t.Error("a creature must remain a legal 'target creature'")
	}
	// Loses its creature type (e.g. animation wore off) → illegal.
	tgt.Card.Types = []string{"artifact"}
	if ai, legal := CheckTargetLegality(gs, tfr_item(0, f, tgt)); !ai || len(legal) != 0 {
		t.Errorf("'target creature' must fizzle once the target is no longer a creature; allIllegal=%v legal=%d", ai, len(legal))
	}
}

// (d) "target creature you don't control" that you gained control of → illegal.
func TestTargetRecheck_ControlRestriction_FizzlesWhenControlGained(t *testing.T) {
	gs := tfr_game()
	tgt := tfr_perm(gs, 1, "Stolen Bear", "creature")
	f := gameast.Filter{Base: "creature", Targeted: true, OpponentControls: true}

	// Opponent controls → legal target for seat 0.
	if ai, _ := CheckTargetLegality(gs, tfr_item(0, f, tgt)); ai {
		t.Error("an opponent's creature must be a legal 'target you don't control'")
	}
	// Seat 0 gains control → 'you don't control' no longer holds → illegal.
	tgt.Controller = 0
	if ai, legal := CheckTargetLegality(gs, tfr_item(0, f, tgt)); !ai || len(legal) != 0 {
		t.Errorf("'target you don't control' must fizzle once you control it; allIllegal=%v legal=%d", ai, len(legal))
	}
}

// (b) partial legality: one target keeps its required state, the other lost
// it → resolves on the still-legal subset only.
func TestTargetRecheck_PartialLegality_KeepsLegalSubset(t *testing.T) {
	gs := tfr_game()
	a := tfr_perm(gs, 1, "Tapped One", "creature")
	b := tfr_perm(gs, 1, "Untapped One", "creature")
	a.Tapped = true
	b.Tapped = false // untapped since targeting → illegal
	f := gameast.Filter{Base: "creature", Targeted: true, Quantifier: "n", Extra: []string{"tapped"}}
	item := &StackItem{
		Controller: 0,
		Effect:     &gameast.Destroy{Target: f},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: a},
			{Kind: TargetKindPermanent, Permanent: b},
		},
	}
	ai, legal := CheckTargetLegality(gs, item)
	if ai {
		t.Fatal("not all targets illegal — should resolve on the legal subset")
	}
	if len(legal) != 1 || legal[0].Permanent != a {
		t.Errorf("only the still-tapped target should remain legal; got %d targets", len(legal))
	}
}
