package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// dev/layer7-ports-r55 — 10 variable-PT cards ported to Layer 7b CDAs
// via the RegisterDynamicSetPT / SetPower / SetToughness primitives.
// ---------------------------------------------------------------------------

// 1. Lord of Extinction — P/T = cards in all graveyards (moved here
//    from spot 10; Sandman port deferred pending isolation-fix r56).
func TestLayer7Ports_LordOfExtinction_AllGraveyards(t *testing.T) {
	gs := newGame(t, 2)
	loe := addPerm(gs, 0, "Lord of Extinction", "creature", "legendary")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "A"}, &gameengine.Card{Name: "B"},
	)
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard,
		&gameengine.Card{Name: "C"}, &gameengine.Card{Name: "D"},
	)
	gameengine.InvokeETBHook(gs, loe)

	chars := gameengine.GetEffectiveCharacteristics(gs, loe)
	if chars.Power != 4 || chars.Toughness != 4 {
		t.Errorf("expected 4/4 (2+2 cards in graveyards), got %d/%d", chars.Power, chars.Toughness)
	}
}

// 2. Greensleeves, Maro-Sorcerer — P/T = lands you control.
func TestLayer7Ports_Greensleeves_PTEqualsLands(t *testing.T) {
	gs := newGame(t, 2)
	greens := addPerm(gs, 0, "Greensleeves, Maro-Sorcerer", "creature", "legendary")
	addPerm(gs, 0, "Forest", "land", "basic", "forest")
	addPerm(gs, 0, "Forest 2", "land", "basic", "forest")
	gameengine.InvokeETBHook(gs, greens)

	chars := gameengine.GetEffectiveCharacteristics(gs, greens)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("expected 2/2 with 2 lands, got %d/%d", chars.Power, chars.Toughness)
	}
}

// 3. Eluge, the Shoreless Sea — P/T = Islands you control (incl. flood-counter lands).
func TestLayer7Ports_Eluge_PTEqualsIslands(t *testing.T) {
	gs := newGame(t, 2)
	eluge := addPerm(gs, 0, "Eluge, the Shoreless Sea", "creature", "legendary")
	addPerm(gs, 0, "Island A", "land", "basic", "island")
	addPerm(gs, 0, "Island B", "land", "basic", "island")
	mountain := addPerm(gs, 0, "Mountain", "land", "basic", "mountain")
	mountain.Counters = map[string]int{"flood": 1} // counts as Island via Eluge's static
	gameengine.InvokeETBHook(gs, eluge)

	chars := gameengine.GetEffectiveCharacteristics(gs, eluge)
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Errorf("expected 3/3 (2 Islands + 1 flood-counter land), got %d/%d", chars.Power, chars.Toughness)
	}
}

// 4. Ixidron — P/T = face-down creatures on battlefield (all seats).
func TestLayer7Ports_Ixidron_PTEqualsFaceDown(t *testing.T) {
	gs := newGame(t, 2)
	ixidron := addPerm(gs, 0, "Ixidron", "creature", "legendary")
	// Pre-existing creatures that Ixidron will flip face-down.
	c1 := addPerm(gs, 1, "Bear", "creature")
	c2 := addPerm(gs, 1, "Wolf", "creature")
	_ = c1
	_ = c2
	gameengine.InvokeETBHook(gs, ixidron)

	chars := gameengine.GetEffectiveCharacteristics(gs, ixidron)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("expected 2/2 (2 face-down creatures), got %d/%d", chars.Power, chars.Toughness)
	}
}

// 5. Uurg, Spawn of Turg — power = land cards in your graveyard.
func TestLayer7Ports_Uurg_PowerEqualsLandYard(t *testing.T) {
	gs := newGame(t, 2)
	uurg := addPerm(gs, 0, "Uurg, Spawn of Turg", "creature", "legendary")
	uurg.Card.BasePower = 0
	uurg.Card.BaseToughness = 4
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Forest", Types: []string{"land", "basic", "forest"}},
		&gameengine.Card{Name: "Swamp", Types: []string{"land", "basic", "swamp"}},
		&gameengine.Card{Name: "Bear", Types: []string{"creature"}}, // not a land
	)
	gameengine.InvokeETBHook(gs, uurg)

	chars := gameengine.GetEffectiveCharacteristics(gs, uurg)
	if chars.Power != 2 {
		t.Errorf("expected power 2 (2 land cards in yard), got %d", chars.Power)
	}
	// Toughness should remain at printed base (4) — power-only CDA.
	if chars.Toughness != 4 {
		t.Errorf("expected toughness preserved at printed 4, got %d", chars.Toughness)
	}
}


// 7. Daxos, Blessed by the Sun — toughness = devotion to white.
func TestLayer7Ports_Daxos_ToughnessEqualsDevotionWhite(t *testing.T) {
	gs := newGame(t, 2)
	daxos := addPerm(gs, 0, "Daxos, Blessed by the Sun", "creature", "legendary")
	daxos.Card.BasePower = 2
	daxos.Card.BaseToughness = 0
	w1 := addPerm(gs, 0, "Plains-walker A", "creature")
	w1.Card.Colors = []string{"W"}
	w2 := addPerm(gs, 0, "Plains-walker B", "creature")
	w2.Card.Colors = []string{"W"}
	gameengine.InvokeETBHook(gs, daxos)

	chars := gameengine.GetEffectiveCharacteristics(gs, daxos)
	if chars.Toughness != 2 {
		t.Errorf("expected toughness 2, got %d", chars.Toughness)
	}
}

// 7. Adeline, Resplendent Cathar — power = creatures you control.
func TestLayer7Ports_Adeline_PowerEqualsCreatures(t *testing.T) {
	gs := newGame(t, 2)
	adeline := addPerm(gs, 0, "Adeline, Resplendent Cathar", "creature", "legendary")
	adeline.Card.BasePower = 0
	adeline.Card.BaseToughness = 3
	addPerm(gs, 0, "Bear", "creature")
	addPerm(gs, 0, "Wolf", "creature")
	gameengine.InvokeETBHook(gs, adeline)

	chars := gameengine.GetEffectiveCharacteristics(gs, adeline)
	if chars.Power != 3 {
		t.Errorf("expected power 3 (Adeline + 2 others), got %d", chars.Power)
	}
}

// 8. Mendicant Core, Guidelight — power = artifacts you control.
func TestLayer7Ports_MendicantCore_PowerEqualsArtifacts(t *testing.T) {
	gs := newGame(t, 2)
	mc := addPerm(gs, 0, "Mendicant Core, Guidelight", "creature", "artifact")
	mc.Card.BasePower = 0
	mc.Card.BaseToughness = 4
	addPerm(gs, 0, "Sol Ring", "artifact")
	addPerm(gs, 0, "Mana Crypt", "artifact")
	addPerm(gs, 0, "Forest", "land", "basic", "forest")
	gameengine.InvokeETBHook(gs, mc)
	chars := gameengine.GetEffectiveCharacteristics(gs, mc)
	if chars.Power != 3 {
		t.Errorf("expected power 3, got %d", chars.Power)
	}
}

// 9. Mortivore — P/T = creature cards in all graveyards.
func TestLayer7Ports_Mortivore_CreatureCardsInGraveyards(t *testing.T) {
	gs := newGame(t, 2)
	mortivore := addPerm(gs, 0, "Mortivore", "creature", "legendary")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Bear", Types: []string{"creature"}},
		&gameengine.Card{Name: "Bolt", Types: []string{"instant"}},
	)
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard,
		&gameengine.Card{Name: "Wolf", Types: []string{"creature"}},
		&gameengine.Card{Name: "Elf", Types: []string{"creature"}},
	)
	gameengine.InvokeETBHook(gs, mortivore)
	chars := gameengine.GetEffectiveCharacteristics(gs, mortivore)
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Errorf("expected 3/3, got %d/%d", chars.Power, chars.Toughness)
	}
}

// 10. Multani, Maro-Sorcerer — P/T = cards in all hands.
func TestLayer7Ports_Multani_PTEqualsAllHands(t *testing.T) {
	gs := newGame(t, 2)
	multani := addPerm(gs, 0, "Multani, Maro-Sorcerer", "creature", "legendary")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		&gameengine.Card{Name: "A"}, &gameengine.Card{Name: "B"},
	)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand,
		&gameengine.Card{Name: "C"}, &gameengine.Card{Name: "D"}, &gameengine.Card{Name: "E"},
	)
	gameengine.InvokeETBHook(gs, multani)
	chars := gameengine.GetEffectiveCharacteristics(gs, multani)
	if chars.Power != 5 || chars.Toughness != 5 {
		t.Errorf("expected 5/5, got %d/%d", chars.Power, chars.Toughness)
	}
}

// 11. Sandman, Shifting Scoundrel — P/T = lands you control. R55
//     deferred this port due to a global-rand interaction with
//     TestKrark; r55's krark.go switched to gs.Rng, and r56 finishes
//     the port (custom_sandman_shifting_scoundrel.go).
func TestLayer7Ports_R56_Sandman_PTEqualsLands(t *testing.T) {
	gs := newGame(t, 2)
	sand := addPerm(gs, 0, "Sandman, Shifting Scoundrel", "creature", "legendary")
	for i := 0; i < 3; i++ {
		addPerm(gs, 0, "Forest", "land", "basic", "forest")
	}
	gameengine.InvokeETBHook(gs, sand)

	chars := gameengine.GetEffectiveCharacteristics(gs, sand)
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Errorf("expected 3/3 with 3 lands, got %d/%d", chars.Power, chars.Toughness)
	}
	// Tracking: add a 4th land; CDA should now read 4/4 on next layer pass.
	addPerm(gs, 0, "Mountain", "land", "basic", "mountain")
	gs.InvalidateCharacteristicsCache()
	chars = gameengine.GetEffectiveCharacteristics(gs, sand)
	if chars.Power != 4 || chars.Toughness != 4 {
		t.Errorf("expected 4/4 after 4th land, got %d/%d", chars.Power, chars.Toughness)
	}
}

// 12. Namor the Sub-Mariner — power = Merfolk you control. r55
//     deferred; r56 ports namor_sub_mariner.go to
//     RegisterDynamicSetPower.
func TestLayer7Ports_R56_Namor_PowerEqualsMerfolk(t *testing.T) {
	gs := newGame(t, 2)
	namor := addPerm(gs, 0, "Namor the Sub-Mariner", "creature", "legendary")
	namor.Card.BasePower = 0
	namor.Card.BaseToughness = 4
	addPerm(gs, 0, "Merrow Wavebreaker", "creature", "merfolk")
	addPerm(gs, 0, "Merfolk Looter", "creature", "merfolk")
	addPerm(gs, 0, "Plain Bear", "creature", "bear")
	gameengine.InvokeETBHook(gs, namor)

	chars := gameengine.GetEffectiveCharacteristics(gs, namor)
	if chars.Power != 2 {
		t.Errorf("expected power 2 (2 Merfolk; Namor isn't a Merfolk), got %d", chars.Power)
	}
	if chars.Toughness != 4 {
		t.Errorf("expected toughness preserved at printed 4, got %d", chars.Toughness)
	}
}
