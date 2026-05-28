package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch X (R60) — tests for 5 new board-wipe handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Supreme Verdict
// -----------------------------------------------------------------------------

func TestSupremeVerdict_StampsCannotBeCounteredAtCast(t *testing.T) {
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Supreme Verdict", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	if item.CostMeta == nil {
		t.Fatalf("OnCast did not initialize CostMeta")
	}
	v, ok := item.CostMeta["cannot_be_countered"].(bool)
	if !ok || !v {
		t.Errorf("Supreme Verdict should stamp cannot_be_countered=true after OnCast, got %+v", item.CostMeta)
	}
}

func TestSupremeVerdict_DestroysAllCreatures(t *testing.T) {
	gs := newGame(t, 2)
	c1 := addPerm(gs, 0, "Mine", "creature")
	c2 := addPerm(gs, 1, "Opp1", "creature")
	c3 := addPerm(gs, 1, "Opp2", "creature")
	c1.Card.BaseToughness = 2
	c2.Card.BaseToughness = 2
	c3.Card.BaseToughness = 2

	card := addCard(gs, 0, "Supreme Verdict", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// All three creatures should be in graveyard, not battlefield.
	for _, perm := range []*gameengine.Permanent{c1, c2, c3} {
		stillOnBF := false
		for _, p := range gs.Seats[perm.Controller].Battlefield {
			if p == perm {
				stillOnBF = true
			}
		}
		if stillOnBF {
			t.Errorf("%s should not be on battlefield after Supreme Verdict", perm.Card.Name)
		}
	}
}

// -----------------------------------------------------------------------------
// Pernicious Deed
// -----------------------------------------------------------------------------

func TestPerniciousDeed_DestroysAtMVAndUnder(t *testing.T) {
	gs := newGame(t, 2)
	deed := addPerm(gs, 0, "Pernicious Deed", "enchantment")
	deed.Card.Types = append(deed.Card.Types, "cmc:3")
	small := addPerm(gs, 1, "Small", "creature", "cmc:2")
	small.Card.BaseToughness = 2
	big := addPerm(gs, 1, "Big", "creature", "cmc:5")
	big.Card.BaseToughness = 5
	rock := addPerm(gs, 1, "Sol Ring", "artifact", "cmc:1")

	ctx := map[string]interface{}{"x": 3}
	gameengine.InvokeActivatedHook(gs, deed, 0, ctx)

	// Deed in graveyard (sacrificed).
	foundDeedInGy := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == deed.Card {
			foundDeedInGy = true
		}
	}
	if !foundDeedInGy {
		t.Errorf("Pernicious Deed should be sacrificed (in graveyard)")
	}
	// Small (CMC 2) destroyed, Big (CMC 5) survives, rock (CMC 1) destroyed.
	for _, p := range gs.Seats[1].Battlefield {
		if p == small {
			t.Errorf("CMC 2 creature should be destroyed at X=3")
		}
		if p == rock {
			t.Errorf("CMC 1 artifact should be destroyed at X=3")
		}
	}
	foundBigOnBF := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == big {
			foundBigOnBF = true
		}
	}
	if !foundBigOnBF {
		t.Errorf("CMC 5 creature should survive X=3")
	}
}

// -----------------------------------------------------------------------------
// Vandalblast
// -----------------------------------------------------------------------------

func TestVandalblast_SingleTargetDestroysOneOppArtifact(t *testing.T) {
	gs := newGame(t, 2)
	solRing := addPerm(gs, 1, "Sol Ring", "artifact", "cmc:1")
	_ = solRing

	card := addCard(gs, 0, "Vandalblast", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	for _, p := range gs.Seats[1].Battlefield {
		if p == solRing {
			t.Errorf("Sol Ring should be destroyed by Vandalblast")
		}
	}
}

func TestVandalblast_OverloadWipesAllOppArtifactsKeepsOwn(t *testing.T) {
	gs := newGame(t, 2)
	ownRock := addPerm(gs, 0, "Own Sol Ring", "artifact")
	opp1 := addPerm(gs, 1, "Opp Mana Crypt", "artifact")
	opp2 := addPerm(gs, 1, "Opp Skullclamp", "artifact", "equipment")

	card := addCard(gs, 0, "Vandalblast", "sorcery")
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		CostMeta:   map[string]interface{}{"overloaded": true},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Own rock survives.
	foundOwn := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == ownRock {
			foundOwn = true
		}
	}
	if !foundOwn {
		t.Errorf("own artifact should survive overload Vandalblast")
	}
	// Opp artifacts gone.
	for _, p := range gs.Seats[1].Battlefield {
		if p == opp1 || p == opp2 {
			t.Errorf("opp artifact should be destroyed by overload")
		}
	}
}

// -----------------------------------------------------------------------------
// Anger of the Gods
// -----------------------------------------------------------------------------

func TestAngerOfTheGods_StampsExileFlagAndDamages(t *testing.T) {
	gs := newGame(t, 2)
	c1 := addPerm(gs, 0, "Friend", "creature")
	c1.Card.BasePower = 2
	c1.Card.BaseToughness = 2
	c2 := addPerm(gs, 1, "Opp", "creature")
	c2.Card.BasePower = 3
	c2.Card.BaseToughness = 3

	card := addCard(gs, 0, "Anger of the Gods", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Both creatures should have exile_instead_of_graveyard_this_turn = 1.
	if c1.Flags["exile_instead_of_graveyard_this_turn"] != 1 {
		t.Errorf("c1 should have exile-rider flag, got %v", c1.Flags)
	}
	if c2.Flags["exile_instead_of_graveyard_this_turn"] != 1 {
		t.Errorf("c2 should have exile-rider flag, got %v", c2.Flags)
	}
	// Both should have 3 marked damage.
	if c1.MarkedDamage != 3 {
		t.Errorf("c1 should have 3 marked damage, got %d", c1.MarkedDamage)
	}
	if c2.MarkedDamage != 3 {
		t.Errorf("c2 should have 3 marked damage, got %d", c2.MarkedDamage)
	}
}

// -----------------------------------------------------------------------------
// Merciless Eviction
// -----------------------------------------------------------------------------

func TestMercilessEviction_PicksModeByOppBoardCensus(t *testing.T) {
	gs := newGame(t, 2)
	// Opp has 3 creatures, 1 artifact. Mode picker → creature.
	c1 := addPerm(gs, 1, "C1", "creature")
	c2 := addPerm(gs, 1, "C2", "creature")
	c3 := addPerm(gs, 1, "C3", "creature")
	rock := addPerm(gs, 1, "Rock", "artifact")

	card := addCard(gs, 0, "Merciless Eviction", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// All creatures exiled; rock survives.
	for _, p := range gs.Seats[1].Battlefield {
		if p == c1 || p == c2 || p == c3 {
			t.Errorf("creature should be exiled, found on opp battlefield: %s", p.Card.Name)
		}
		if p == rock {
			// good — rock should survive creature-mode
		}
	}
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == rock {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("rock should survive creature-mode Merciless Eviction")
	}
}

func TestMercilessEviction_ExplicitModeOverride(t *testing.T) {
	gs := newGame(t, 2)
	c1 := addPerm(gs, 1, "Opp Creature", "creature")
	enc := addPerm(gs, 1, "Rhystic Study", "enchantment")

	card := addCard(gs, 0, "Merciless Eviction", "sorcery")
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		CostMeta:   map[string]interface{}{"eviction_mode": "enchantment"},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Enchantment exiled, creature survives despite there being only 1
	// of each — the override forced enchantment mode.
	for _, p := range gs.Seats[1].Battlefield {
		if p == enc {
			t.Errorf("enchantment should be exiled with explicit override")
		}
	}
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == c1 {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("creature should survive enchantment-mode override")
	}
}
