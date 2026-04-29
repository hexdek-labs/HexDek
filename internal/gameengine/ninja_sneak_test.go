package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Shared helper tests
// ---------------------------------------------------------------------------

func TestFindUnblockedAttacker_ReturnsSmallest(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	big := &Permanent{
		Card:       &Card{Name: "Giant", Owner: 0, BasePower: 5, BaseToughness: 5, Types: []string{"creature"}},
		Controller: 0,
		Flags:      map[string]int{},
	}
	small := &Permanent{
		Card:       &Card{Name: "Ornithopter", Owner: 0, BasePower: 0, BaseToughness: 2, Types: []string{"creature"}},
		Controller: 0,
		Flags:      map[string]int{},
	}
	setPermFlag(big, flagAttacking, true)
	setPermFlag(small, flagAttacking, true)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, big, small)

	attackers := []*Permanent{big, small}
	blockerMap := map[*Permanent][]*Permanent{
		big:   nil,
		small: nil,
	}

	result := FindUnblockedAttacker(gs, 0, attackers, blockerMap)
	if result != small {
		t.Fatalf("expected smallest-power attacker (Ornithopter), got %v", result)
	}
}

func TestFindUnblockedAttacker_NoneWhenAllBlocked(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	atk := &Permanent{
		Card:       &Card{Name: "Bear", Owner: 0, BasePower: 2, BaseToughness: 2, Types: []string{"creature"}},
		Controller: 0,
		Flags:      map[string]int{},
	}
	setPermFlag(atk, flagAttacking, true)
	blocker := &Permanent{Card: &Card{Name: "Wall", Types: []string{"creature"}}, Controller: 1}

	attackers := []*Permanent{atk}
	blockerMap := map[*Permanent][]*Permanent{
		atk: {blocker},
	}

	result := FindUnblockedAttacker(gs, 0, attackers, blockerMap)
	if result != nil {
		t.Fatal("expected nil when all attackers are blocked")
	}
}

func TestBounceUnblockedAttacker_ClearsFlags(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	atk := &Permanent{
		Card:       &Card{Name: "Ornithopter", Owner: 0, BasePower: 0, BaseToughness: 2, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(atk, flagAttacking, true)
	setPermFlag(atk, flagDeclaredAttacker, true)
	setAttackerDefender(atk, 1)
	seat.Battlefield = append(seat.Battlefield, atk)

	defSeat := BounceUnblockedAttacker(gs, atk)
	if defSeat != 1 {
		t.Fatalf("expected defender seat 1, got %d", defSeat)
	}

	// Attacker should be in owner's hand.
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c != nil && c.Name == "Ornithopter" {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Fatal("bounced attacker should be in hand")
	}
}

// ---------------------------------------------------------------------------
// Ninjutsu tests (refactored path)
// ---------------------------------------------------------------------------

func TestNinjutsuRefactored_SwapsUnblockedAttacker(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card: &Card{
			Name:          "Ornithopter",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 2,
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	ninja := &Card{
		Name:          "Ninja of the Deep Hours",
		Owner:         0,
		Types:         []string{"creature", "cost:4"},
		BasePower:     2,
		BaseToughness: 2,
		AST: &gameast.CardAST{
			Name: "Ninja of the Deep Hours",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
			},
		},
	}
	seat.Hand = append(seat.Hand, ninja)
	seat.ManaPool = 5

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{
		attacker: nil,
	}

	result := CheckNinjutsuRefactored(gs, 0, attackers, blockerMap)

	// Attacker should be bounced to hand.
	foundBounced := false
	for _, c := range seat.Hand {
		if c != nil && c.Name == "Ornithopter" {
			foundBounced = true
		}
	}
	if !foundBounced {
		t.Fatal("bounced attacker should be in hand")
	}

	// Ninja should be on battlefield.
	foundNinja := false
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Ninja of the Deep Hours" {
			foundNinja = true
			if !permFlag(p, flagAttacking) {
				t.Fatal("ninja should be attacking")
			}
			if permFlag(p, flagDeclaredAttacker) {
				t.Fatal("ninja should NOT be a declared attacker (CR 702.49b)")
			}
			if !p.Tapped {
				t.Fatal("ninja should be tapped")
			}
			def, ok := AttackerDefender(p)
			if !ok || def != 1 {
				t.Fatalf("ninja should be attacking seat 1, got %d", def)
			}
			// Verify ninjutsu_entry flag is set (for Yuriko).
			if _, ok := p.Flags["ninjutsu_entry"]; !ok {
				t.Fatal("ninja should have ninjutsu_entry flag set")
			}
		}
	}
	if !foundNinja {
		t.Fatal("ninja should be on battlefield")
	}

	// Result should contain the ninja and not the old attacker.
	foundNinjaInResult := false
	foundOldInResult := false
	for _, p := range result {
		if p.Card.Name == "Ninja of the Deep Hours" {
			foundNinjaInResult = true
		}
		if p.Card.Name == "Ornithopter" {
			foundOldInResult = true
		}
	}
	if !foundNinjaInResult {
		t.Fatal("result attackers should contain the ninja")
	}
	if foundOldInResult {
		t.Fatal("result attackers should NOT contain the bounced attacker")
	}

	// Mana should have been paid.
	if seat.ManaPool >= 5 {
		t.Fatal("mana should have been spent on ninjutsu cost")
	}

	// Verify ninjutsu event.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "ninjutsu" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'ninjutsu' event in log")
	}
}

func TestNinjutsuRefactored_NoActivationWithoutUnblocked(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card:       &Card{Name: "Ornithopter", Owner: 0, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	seat.Battlefield = append(seat.Battlefield, attacker)

	blocker := &Permanent{
		Card:       &Card{Name: "Bear", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, blocker)

	ninja := &Card{
		Name:  "Ninja",
		Owner: 0,
		Types: []string{"creature", "cost:4"},
		AST: &gameast.CardAST{
			Name: "Ninja",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
			},
		},
	}
	seat.Hand = append(seat.Hand, ninja)
	seat.ManaPool = 5

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{
		attacker: {blocker},
	}

	result := CheckNinjutsuRefactored(gs, 0, attackers, blockerMap)

	if len(result) != 1 || result[0] != attacker {
		t.Fatal("no ninjutsu should have activated with blocked attacker")
	}

	foundInHand := false
	for _, c := range seat.Hand {
		if c == ninja {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Fatal("ninja should still be in hand when no unblocked attackers")
	}
}

func TestNinjutsuRefactored_NoActivationWithoutMana(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card:       &Card{Name: "Ornithopter", Owner: 0, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	ninja := &Card{
		Name:  "Ninja",
		Owner: 0,
		Types: []string{"creature", "cost:4"},
		AST: &gameast.CardAST{
			Name: "Ninja",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
			},
		},
	}
	seat.Hand = append(seat.Hand, ninja)
	seat.ManaPool = 0

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{attacker: nil}

	result := CheckNinjutsuRefactored(gs, 0, attackers, blockerMap)

	if len(result) != 1 || result[0] != attacker {
		t.Fatal("no ninjutsu should have activated without mana")
	}
}

// ---------------------------------------------------------------------------
// Commander ninjutsu tests
// ---------------------------------------------------------------------------

func TestCommanderNinjutsu_FromCommandZone(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card: &Card{
			Name:          "Ornithopter",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 2,
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	yuriko := &Card{
		Name:          "Yuriko, the Tiger's Shadow",
		Owner:         0,
		Types:         []string{"creature", "cost:3"},
		BasePower:     1,
		BaseToughness: 3,
		AST: &gameast.CardAST{
			Name: "Yuriko, the Tiger's Shadow",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "commander_ninjutsu"},
			},
		},
	}
	// Put in command zone, NOT in hand.
	seat.CommandZone = append(seat.CommandZone, yuriko)
	seat.ManaPool = 5

	// Initialize commander cast counts to verify NO tax increment.
	if seat.CommanderCastCounts == nil {
		seat.CommanderCastCounts = map[string]int{}
	}
	seat.CommanderCastCounts["Yuriko, the Tiger's Shadow"] = 0

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{attacker: nil}

	result := CheckNinjutsuRefactored(gs, 0, attackers, blockerMap)

	// Yuriko should be on battlefield.
	foundYuriko := false
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Yuriko, the Tiger's Shadow" {
			foundYuriko = true
			if !permFlag(p, flagAttacking) {
				t.Fatal("Yuriko should be attacking")
			}
			if permFlag(p, flagDeclaredAttacker) {
				t.Fatal("Yuriko should NOT be a declared attacker")
			}
			if !p.Tapped {
				t.Fatal("Yuriko should be tapped")
			}
		}
	}
	if !foundYuriko {
		t.Fatal("Yuriko should be on battlefield from command zone")
	}

	// Should NOT have incremented commander tax.
	if seat.CommanderCastCounts["Yuriko, the Tiger's Shadow"] != 0 {
		t.Fatalf("commander ninjutsu should NOT increment commander tax, got %d",
			seat.CommanderCastCounts["Yuriko, the Tiger's Shadow"])
	}

	// Result should contain Yuriko, not Ornithopter.
	if len(result) != 1 || result[0].Card.Name != "Yuriko, the Tiger's Shadow" {
		t.Fatal("result should contain Yuriko as the new attacker")
	}

	// Yuriko should be removed from command zone.
	for _, c := range seat.CommandZone {
		if c == yuriko {
			t.Fatal("Yuriko should no longer be in command zone")
		}
	}

	// Should have commander_ninjutsu event.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "commander_ninjutsu" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'commander_ninjutsu' event in log")
	}
}

// ---------------------------------------------------------------------------
// Sneak tests
// ---------------------------------------------------------------------------

func TestSneak_CastsCreatureTappedAttacking(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card: &Card{
			Name:          "Ornithopter",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 2,
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	sneakCard := &Card{
		Name:          "Foot Ninjas",
		Owner:         0,
		Types:         []string{"creature", "cost:3"},
		BasePower:     3,
		BaseToughness: 3,
		AST: &gameast.CardAST{
			Name: "Foot Ninjas",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "sneak"},
			},
		},
	}
	seat.Hand = append(seat.Hand, sneakCard)
	seat.ManaPool = 10

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{
		attacker: nil,
	}

	result := CheckSneak(gs, 0, attackers, blockerMap)

	// Ornithopter should be bounced to hand.
	foundBounced := false
	for _, c := range seat.Hand {
		if c != nil && c.Name == "Ornithopter" {
			foundBounced = true
		}
	}
	if !foundBounced {
		t.Fatal("bounced attacker should be in hand")
	}

	// Foot Ninjas should be on battlefield.
	foundSneak := false
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Foot Ninjas" {
			foundSneak = true
			if !permFlag(p, flagAttacking) {
				t.Fatal("sneak creature should be attacking")
			}
			if !p.Tapped {
				t.Fatal("sneak creature should be tapped")
			}
			if permFlag(p, flagDeclaredAttacker) {
				t.Fatal("sneak creature should NOT be a declared attacker")
			}
			if _, ok := p.Flags["sneak_entry"]; !ok {
				t.Fatal("sneak creature should have sneak_entry flag")
			}
		}
	}
	if !foundSneak {
		t.Fatal("sneak creature should be on battlefield")
	}

	// Result should include the sneak creature.
	foundInResult := false
	for _, p := range result {
		if p.Card.Name == "Foot Ninjas" {
			foundInResult = true
		}
	}
	if !foundInResult {
		t.Fatal("result attackers should contain the sneak creature")
	}

	// Verify that sneak IS a cast (should have a "cast" event).
	foundCast := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "cast" && ev.Source == "Foot Ninjas" {
			foundCast = true
		}
	}
	if !foundCast {
		t.Fatal("sneak should be a cast (expected 'cast' event for Foot Ninjas)")
	}
}

func TestSneak_NoActivationWithoutUnblocked(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card:       &Card{Name: "Ornithopter", Owner: 0, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	seat.Battlefield = append(seat.Battlefield, attacker)

	blocker := &Permanent{
		Card:       &Card{Name: "Wall", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, blocker)

	sneakCard := &Card{
		Name:  "Foot Ninjas",
		Owner: 0,
		Types: []string{"creature", "cost:3"},
		AST: &gameast.CardAST{
			Name: "Foot Ninjas",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "sneak"},
			},
		},
	}
	seat.Hand = append(seat.Hand, sneakCard)
	seat.ManaPool = 10

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{
		attacker: {blocker},
	}

	result := CheckSneak(gs, 0, attackers, blockerMap)

	if len(result) != 1 || result[0] != attacker {
		t.Fatal("no sneak should have activated with blocked attacker")
	}
}

func TestSneak_NoActivationWithoutMana(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card:       &Card{Name: "Ornithopter", Owner: 0, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	sneakCard := &Card{
		Name:  "Foot Ninjas",
		Owner: 0,
		Types: []string{"creature", "cost:5"},
		AST: &gameast.CardAST{
			Name: "Foot Ninjas",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "sneak"},
			},
		},
	}
	seat.Hand = append(seat.Hand, sneakCard)
	seat.ManaPool = 0

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{attacker: nil}

	result := CheckSneak(gs, 0, attackers, blockerMap)

	if len(result) != 1 || result[0] != attacker {
		t.Fatal("no sneak should have activated without mana")
	}
}

func TestSneak_IncrementsStormCount(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card: &Card{
			Name:          "Ornithopter",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 2,
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	sneakCard := &Card{
		Name:          "Sneaky Ninja",
		Owner:         0,
		Types:         []string{"creature", "cost:2"},
		BasePower:     2,
		BaseToughness: 2,
		AST: &gameast.CardAST{
			Name: "Sneaky Ninja",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "sneak"},
			},
		},
	}
	seat.Hand = append(seat.Hand, sneakCard)
	seat.ManaPool = 10

	initialStormCount := gs.SpellsCastThisTurn

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{attacker: nil}

	CheckSneak(gs, 0, attackers, blockerMap)

	// Sneak IS a cast, so storm count should have incremented.
	if gs.SpellsCastThisTurn <= initialStormCount {
		t.Fatalf("sneak should increment storm count; was %d, now %d",
			initialStormCount, gs.SpellsCastThisTurn)
	}
}

// ---------------------------------------------------------------------------
// Legacy wrapper test
// ---------------------------------------------------------------------------

func TestCheckNinjutsu_LegacyWrapper_Delegates(t *testing.T) {
	// Verify the old CheckNinjutsu function still works identically.
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card: &Card{
			Name:          "Ornithopter",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 2,
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	ninja := &Card{
		Name:          "Ninja of the Deep Hours",
		Owner:         0,
		Types:         []string{"creature", "cost:4"},
		BasePower:     2,
		BaseToughness: 2,
		AST: &gameast.CardAST{
			Name: "Ninja of the Deep Hours",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
			},
		},
	}
	seat.Hand = append(seat.Hand, ninja)
	seat.ManaPool = 5

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{attacker: nil}

	result := CheckNinjutsu(gs, 0, attackers, blockerMap)

	// Should work identically to the refactored version.
	foundNinja := false
	for _, p := range result {
		if p.Card.Name == "Ninja of the Deep Hours" {
			foundNinja = true
		}
	}
	if !foundNinja {
		t.Fatal("legacy CheckNinjutsu wrapper should still swap in the ninja")
	}

	foundEvent := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "ninjutsu" {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Fatal("legacy wrapper should emit ninjutsu event")
	}
}

// ---------------------------------------------------------------------------
// cardHasSneak / cardHasNinjutsu detection tests
// ---------------------------------------------------------------------------

func TestCardHasSneak_Detection(t *testing.T) {
	// AST keyword detection.
	c := &Card{
		Name: "Foot Ninjas",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "sneak"},
			},
		},
	}
	if !cardHasSneak(c) {
		t.Fatal("should detect sneak keyword from AST")
	}

	// Type-list fallback (test convention).
	c2 := &Card{
		Name:  "Test Sneak",
		Types: []string{"creature", "sneak"},
	}
	if !cardHasSneak(c2) {
		t.Fatal("should detect sneak from Types (test convention)")
	}

	// Negative case.
	c3 := &Card{
		Name:  "Grizzly Bears",
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "trample"},
			},
		},
	}
	if cardHasSneak(c3) {
		t.Fatal("should not detect sneak on card without it")
	}
}

func TestCardHasNinjutsuOrCommanderNinjutsu(t *testing.T) {
	// Regular ninjutsu.
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
			},
		},
	}
	if !cardHasNinjutsuOrCommanderNinjutsu(c, false) {
		t.Fatal("should detect ninjutsu")
	}

	// Commander ninjutsu (from hand = false).
	c2 := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "commander_ninjutsu"},
			},
		},
	}
	if cardHasNinjutsuOrCommanderNinjutsu(c2, false) {
		t.Fatal("should NOT detect commander_ninjutsu when fromCommandZone=false")
	}
	if !cardHasNinjutsuOrCommanderNinjutsu(c2, true) {
		t.Fatal("should detect commander_ninjutsu when fromCommandZone=true")
	}
}
