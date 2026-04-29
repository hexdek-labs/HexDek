package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// P1 #4: Ward Tests
// ---------------------------------------------------------------------------

func TestWard_CountersSpellWhenCannotPay(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Seat 1 has a creature with ward.
	wardCreature := &Permanent{
		Card:       &Card{Name: "Ward Creature", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
		Flags:      map[string]int{"kw:ward": 1, "ward_cost": 2},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, wardCreature)

	// Seat 0 targets it with a spell but has no mana.
	gs.Seats[0].ManaPool = 0
	item := &StackItem{
		Controller: 0,
		Card:       &Card{Name: "Murder"},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: wardCreature},
		},
	}

	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("spell targeting ward creature should be countered when caster can't pay")
	}

	// Verify event logged.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "ward_counter" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'ward_counter' event in log")
	}
}

func TestWard_PaysWardCost(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	wardCreature := &Permanent{
		Card:       &Card{Name: "Ward Creature", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
		Flags:      map[string]int{"kw:ward": 1, "ward_cost": 2},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, wardCreature)

	// Seat 0 has enough mana to pay ward.
	gs.Seats[0].ManaPool = 5
	item := &StackItem{
		Controller: 0,
		Card:       &Card{Name: "Murder"},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: wardCreature},
		},
	}

	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("spell should NOT be countered when caster pays ward cost")
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("expected 3 mana remaining (5 - 2 ward cost), got %d", gs.Seats[0].ManaPool)
	}

	// Verify ward_paid event.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "ward_paid" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'ward_paid' event in log")
	}
}

func TestWard_DoesNotTriggerOnOwnSpells(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	wardCreature := &Permanent{
		Card:       &Card{Name: "Ward Creature", Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{"kw:ward": 1, "ward_cost": 2},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, wardCreature)

	// Seat 0 targets own ward creature — no ward check.
	gs.Seats[0].ManaPool = 0
	item := &StackItem{
		Controller: 0,
		Card:       &Card{Name: "Giant Growth"},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: wardCreature},
		},
	}

	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward should not trigger on own spells")
	}
}

func TestWard_DefaultCostIsOne(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Ward creature with no explicit ward_cost flag — defaults to 1.
	wardCreature := &Permanent{
		Card:       &Card{Name: "Ward Creature", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
		Flags:      map[string]int{"kw:ward": 1},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, wardCreature)

	gs.Seats[0].ManaPool = 1
	item := &StackItem{
		Controller: 0,
		Card:       &Card{Name: "Shock"},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: wardCreature},
		},
	}

	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("spell should not be countered when paying default ward cost of 1")
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected 0 mana remaining after paying ward 1, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// P1 #5: Prowess Tests
// ---------------------------------------------------------------------------

func TestProwess_GetsBuffOnNoncreatureSpell(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Seat 0 has a creature with prowess.
	prowessCreature := &Permanent{
		Card: &Card{
			Name:          "Monastery Swiftspear",
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 2,
			AST: &gameast.CardAST{
				Name: "Monastery Swiftspear",
				Abilities: []gameast.Ability{
					&gameast.Keyword{Name: "prowess"},
				},
			},
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, prowessCreature)

	basePower := prowessCreature.Power()
	baseToughness := prowessCreature.Toughness()

	// Cast a noncreature spell.
	castCard := &Card{Name: "Lightning Bolt", Types: []string{"instant"}}
	FireCastTriggerObservers(gs, castCard, 0, false)

	// Prowess should have buffed +1/+1.
	if prowessCreature.Power() != basePower+1 {
		t.Fatalf("expected power %d after prowess, got %d", basePower+1, prowessCreature.Power())
	}
	if prowessCreature.Toughness() != baseToughness+1 {
		t.Fatalf("expected toughness %d after prowess, got %d", baseToughness+1, prowessCreature.Toughness())
	}

	// Verify prowess event.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "prowess" && ev.Source == "Monastery Swiftspear" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'prowess' event in log")
	}
}

func TestProwess_DoesNotTriggerOnCreatureSpell(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	prowessCreature := &Permanent{
		Card: &Card{
			Name:          "Monastery Swiftspear",
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 2,
			AST: &gameast.CardAST{
				Name: "Monastery Swiftspear",
				Abilities: []gameast.Ability{
					&gameast.Keyword{Name: "prowess"},
				},
			},
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, prowessCreature)

	basePower := prowessCreature.Power()

	// Cast a creature spell — prowess should NOT trigger.
	castCard := &Card{Name: "Grizzly Bears", Types: []string{"creature"}}
	FireCastTriggerObservers(gs, castCard, 0, false)

	if prowessCreature.Power() != basePower {
		t.Fatalf("prowess should not trigger on creature spells; power %d, expected %d",
			prowessCreature.Power(), basePower)
	}
}

func TestProwess_DoesNotTriggerOnOpponentSpell(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	prowessCreature := &Permanent{
		Card: &Card{
			Name:          "Monastery Swiftspear",
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 2,
			AST: &gameast.CardAST{
				Name: "Monastery Swiftspear",
				Abilities: []gameast.Ability{
					&gameast.Keyword{Name: "prowess"},
				},
			},
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, prowessCreature)

	basePower := prowessCreature.Power()

	// Opponent casts a noncreature spell — prowess should NOT trigger.
	castCard := &Card{Name: "Lightning Bolt", Types: []string{"instant"}}
	FireCastTriggerObservers(gs, castCard, 1, false)

	if prowessCreature.Power() != basePower {
		t.Fatalf("prowess should not trigger on opponent's spells; power %d, expected %d",
			prowessCreature.Power(), basePower)
	}
}

func TestProwess_StacksMultipleTriggers(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	prowessCreature := &Permanent{
		Card: &Card{
			Name:          "Monastery Swiftspear",
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 2,
			AST: &gameast.CardAST{
				Name: "Monastery Swiftspear",
				Abilities: []gameast.Ability{
					&gameast.Keyword{Name: "prowess"},
				},
			},
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, prowessCreature)

	// Cast 3 noncreature spells.
	for i := 0; i < 3; i++ {
		castCard := &Card{Name: "Spell", Types: []string{"instant"}}
		FireCastTriggerObservers(gs, castCard, 0, false)
	}

	// Should have +3/+3 from 3 prowess triggers.
	if prowessCreature.Power() != 4 {
		t.Fatalf("expected power 4 after 3 prowess triggers, got %d", prowessCreature.Power())
	}
	if prowessCreature.Toughness() != 5 {
		t.Fatalf("expected toughness 5 after 3 prowess triggers, got %d", prowessCreature.Toughness())
	}
}
