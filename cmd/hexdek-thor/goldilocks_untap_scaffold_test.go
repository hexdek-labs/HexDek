package main

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 goldilocks re-baseline regression: 31 of 131 dead-effect failures
// were self/target-untap abilities (Morphling "{U}: Untap Morphling",
// Glimmerbell, Maze of Ith, ...) whose scaffold placed the SOURCE — and
// the opponent board — untapped, so resolveUntap's effect produced no
// observable change. The scaffold now taps the world for untap-kind
// effects, EXCEPT a source whose activated ability needs {T} (tapping
// it would make the activation itself illegal as already_tapped).
// cardHasTapCostActivated is the gate for that exception.
func TestCardHasTapCostActivated(t *testing.T) {
	tapCost := &gameengine.Card{
		Name: "Maze-Shape",
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost:   gameast.Cost{Tap: true},
				Effect: &gameast.UntapEffect{},
				Raw:    "{T}: untap target attacking creature",
			},
		}},
	}
	if !cardHasTapCostActivated(tapCost) {
		t.Error("tap-cost activated ability not detected — source would be scaffolded tapped and the activation would fail as already_tapped")
	}

	manaCost := &gameengine.Card{
		Name: "Morphling-Shape",
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{Mana: &gameast.ManaCost{
					Symbols: []gameast.ManaSymbol{{Color: []string{"U"}}},
				}},
				Effect: &gameast.UntapEffect{},
				Raw:    "{U}: untap this creature",
			},
		}},
	}
	if cardHasTapCostActivated(manaCost) {
		t.Error("mana-only activated ability misread as tap-cost — self-untap source would be scaffolded untapped and read as a dead effect")
	}

	if cardHasTapCostActivated(nil) || cardHasTapCostActivated(&gameengine.Card{Name: "no-ast"}) {
		t.Error("nil/AST-less card must report no tap cost")
	}
}

// Scaffold-behavior pin through the real setupForEffect: an untap-kind
// effect must scaffold a mana-cost self-untap source TAPPED (Morphling
// shape — pre-fix it started untapped and the effect read as dead) and
// a {T}-cost source UNTAPPED (Maze of Ith shape — tapping it would
// break the activation itself). The opponent board must end tapped so
// any "untap target" pick is observable.
func TestSetupForEffect_UntapScaffoldTapsTheWorld(t *testing.T) {
	mk := func(tapCost bool) (*gameengine.GameState, *oracleCard) {
		gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
		cost := gameast.Cost{}
		if tapCost {
			cost.Tap = true
		} else {
			cost.Mana = &gameast.ManaCost{Symbols: []gameast.ManaSymbol{{Color: []string{"U"}}}}
		}
		oc := &oracleCard{
			Name: "Scaffold Probe", Types: []string{"creature"},
			Power: 3, Toughness: 3,
			ast: &gameast.CardAST{Name: "Scaffold Probe", Abilities: []gameast.Ability{
				&gameast.Activated{Cost: cost, Effect: &gameast.UntapEffect{}},
			}},
		}
		return gs, oc
	}
	info := &effectInfo{kind: "untap", abilityKind: "activated", effect: &gameast.UntapEffect{}, fullEffect: &gameast.UntapEffect{}}

	gs, oc := mk(false) // Morphling shape
	setupForEffect(gs, oc, info)
	src := findPermByName(gs, "Scaffold Probe")
	if src == nil {
		t.Fatal("source not placed")
	}
	if !src.Tapped {
		t.Error("mana-cost self-untap source scaffolded UNTAPPED — effect unobservable (the r63 dead-effect class)")
	}
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && !p.Tapped {
			t.Errorf("opponent permanent %q untapped — 'untap target' picks may be unobservable", p.Card.DisplayName())
		}
	}

	gs2, oc2 := mk(true) // Maze of Ith shape
	setupForEffect(gs2, oc2, info)
	src2 := findPermByName(gs2, "Scaffold Probe")
	if src2 == nil {
		t.Fatal("source not placed")
	}
	if src2.Tapped {
		t.Error("{T}-cost source scaffolded TAPPED — its own activation would fail as already_tapped")
	}
}

func findPermByName(gs *gameengine.GameState, name string) *gameengine.Permanent {
	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.Card.Name == name {
				return p
			}
		}
	}
	return nil
}
