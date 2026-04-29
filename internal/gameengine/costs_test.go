package gameengine

// Wave 2 tests — alternative and additional cost enforcement.
//
// These tests verify the cost infrastructure (costs.go) and its
// integration with CastSpellWithCosts. Uses synthetic fixture cards
// built inline; no corpus dependency.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// 1. Force of Will — pitch a blue card + pay 1 life → cast for free
// ---------------------------------------------------------------------------

func TestCosts_ForceOfWill_PitchCast(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 1 // opponent's turn (FoW can be cast on any turn)
	gs.Seats[0].ManaPool = 0
	gs.Seats[0].Life = 20

	// Put Force of Will in hand (normally costs 5).
	fow := addHandCardWithEffect(gs, 0, "Force of Will", 5,
		&gameast.CounterSpell{
			Target: gameast.Filter{Base: "spell", Targeted: true},
		}, "instant")
	fow.Colors = []string{"U"}

	// Put a blue card in hand to exile.
	blueCard := addHandCard(gs, 0, "Brainstorm", 1, "instant")
	blueCard.Colors = []string{"U"}

	// Put a spell from opponent on the stack so the counter has something
	// to target (otherwise it fizzles).
	oppSpell := addHandCardWithEffect(gs, 1, "Lightning Bolt", 1,
		&gameast.Damage{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		}, "instant")
	gs.Seats[1].ManaPool = 1
	// Manually push the opponent's spell to the stack.
	removeFromHand(gs.Seats[1], oppSpell)
	gs.Seats[1].ManaPool -= 1
	oppItem := &StackItem{
		Controller: 1,
		Card:       oppSpell,
		Effect:     collectSpellEffect(oppSpell),
	}
	PushStackItem(gs, oppItem)

	// Define the pitch alt cost.
	pitchCost := &AlternativeCost{
		Kind:       AltCostKindPitch,
		Label:      "Force of Will pitch",
		ExileColor: "U",
		LifeCost:   1,
	}

	// Verify it's payable.
	if !CanPayAlternativeCost(gs, 0, fow, pitchCost) {
		t.Fatal("Force of Will pitch cost should be payable")
	}

	// Cast with alt cost.
	result, err := CastSpellWithCosts(gs, 0, fow, nil, pitchCost, nil, true)
	if err != nil {
		t.Fatalf("CastSpellWithCosts failed: %v", err)
	}

	// Verify results.
	if !result.UsedAlternativeCost {
		t.Error("expected UsedAlternativeCost to be true")
	}
	if result.AltCostKind != AltCostKindPitch {
		t.Errorf("expected AltCostKind %q, got %q", AltCostKindPitch, result.AltCostKind)
	}
	if result.LifePaid != 1 {
		t.Errorf("expected 1 life paid, got %d", result.LifePaid)
	}
	if len(result.ExiledCards) != 1 {
		t.Errorf("expected 1 exiled card, got %d", len(result.ExiledCards))
	}

	// Life should be 19 (20 - 1).
	if gs.Seats[0].Life != 19 {
		t.Errorf("expected life 19, got %d", gs.Seats[0].Life)
	}

	// Mana should be 0 (we paid alt cost, not mana).
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana 0, got %d", gs.Seats[0].ManaPool)
	}

	// Brainstorm should be in exile.
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c.Name == "Brainstorm" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Brainstorm should be in exile")
	}

	// Force of Will should NOT be in hand (it was cast).
	for _, c := range gs.Seats[0].Hand {
		if c.Name == "Force of Will" {
			t.Error("Force of Will should not be in hand after casting")
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Force of Will — cannot pitch if no blue card in hand
// ---------------------------------------------------------------------------

func TestCosts_ForceOfWill_CannotPitchWithoutBlueCard(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 0
	gs.Seats[0].Life = 20

	fow := addHandCard(gs, 0, "Force of Will", 5, "instant")
	fow.Colors = []string{"U"}

	// Only a red card in hand (not blue).
	redCard := addHandCard(gs, 0, "Mountain", 0, "land")
	redCard.Colors = []string{"R"}

	pitchCost := &AlternativeCost{
		Kind:       AltCostKindPitch,
		Label:      "Force of Will pitch",
		ExileColor: "U",
		LifeCost:   1,
	}

	if CanPayAlternativeCost(gs, 0, fow, pitchCost) {
		t.Fatal("should NOT be able to pitch without a blue card")
	}
}

// ---------------------------------------------------------------------------
// 3. Force of Will — cannot pitch if life would drop to 0
// ---------------------------------------------------------------------------

func TestCosts_ForceOfWill_CannotPitchAtOneLife(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 0
	gs.Seats[0].Life = 1

	fow := addHandCard(gs, 0, "Force of Will", 5, "instant")
	fow.Colors = []string{"U"}

	blueCard := addHandCard(gs, 0, "Ponder", 1, "sorcery")
	blueCard.Colors = []string{"U"}

	pitchCost := &AlternativeCost{
		Kind:       AltCostKindPitch,
		Label:      "Force of Will pitch",
		ExileColor: "U",
		LifeCost:   1,
	}

	if CanPayAlternativeCost(gs, 0, fow, pitchCost) {
		t.Fatal("should NOT be able to pitch at 1 life (would die)")
	}
}

// ---------------------------------------------------------------------------
// 4. Fierce Guardianship — free when commander on battlefield
// ---------------------------------------------------------------------------

func TestCosts_FierceGuardianship_FreeWithCommander(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = true
	gs.Active = 1 // opponent's turn
	gs.Seats[0].ManaPool = 0
	gs.Seats[0].CommanderNames = []string{"Kess, Dissident Mage"}

	// Put commander on battlefield.
	addBattlefield(gs, 0, "Kess, Dissident Mage", 3, 4,
		"legendary", "creature")

	// Put Fierce Guardianship in hand (normally costs 3).
	fg := addHandCardWithEffect(gs, 0, "Fierce Guardianship", 3,
		&gameast.CounterSpell{
			Target: gameast.Filter{Base: "spell", Targeted: true},
		}, "instant")

	// Put a spell from opponent on the stack.
	oppSpell := addHandCardWithEffect(gs, 1, "Demonic Tutor", 2,
		&gameast.Tutor{
			Count:       *gameast.NumInt(1),
			Destination: "hand",
		}, "sorcery")
	gs.Seats[1].ManaPool = 2
	removeFromHand(gs.Seats[1], oppSpell)
	gs.Seats[1].ManaPool -= 2
	oppItem := &StackItem{
		Controller: 1,
		Card:       oppSpell,
		Effect:     collectSpellEffect(oppSpell),
	}
	PushStackItem(gs, oppItem)

	cmdrFree := &AlternativeCost{
		Kind:  AltCostKindCommanderFree,
		Label: "Fierce Guardianship commander free",
	}

	if !CanPayAlternativeCost(gs, 0, fg, cmdrFree) {
		t.Fatal("Fierce Guardianship should be free with commander on battlefield")
	}

	result, err := CastSpellWithCosts(gs, 0, fg, nil, cmdrFree, nil, true)
	if err != nil {
		t.Fatalf("CastSpellWithCosts failed: %v", err)
	}

	if !result.UsedAlternativeCost {
		t.Error("expected UsedAlternativeCost true")
	}
	if result.AltCostKind != AltCostKindCommanderFree {
		t.Errorf("expected AltCostKind %q, got %q", AltCostKindCommanderFree, result.AltCostKind)
	}

	// Mana should still be 0.
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana 0, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// 5. Fierce Guardianship — costs 3 when commander NOT on battlefield
// ---------------------------------------------------------------------------

func TestCosts_FierceGuardianship_CostsManWithoutCommander(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = true
	gs.Active = 1
	gs.Seats[0].ManaPool = 3
	gs.Seats[0].CommanderNames = []string{"Kess, Dissident Mage"}
	// Commander NOT on battlefield (in command zone or graveyard).

	fg := addHandCardWithEffect(gs, 0, "Fierce Guardianship", 3,
		&gameast.CounterSpell{
			Target: gameast.Filter{Base: "spell", Targeted: true},
		}, "instant")

	// Put opponent spell on stack.
	oppSpell := addHandCardWithEffect(gs, 1, "Opt", 1,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")
	gs.Seats[1].ManaPool = 1
	removeFromHand(gs.Seats[1], oppSpell)
	gs.Seats[1].ManaPool -= 1
	oppItem := &StackItem{
		Controller: 1,
		Card:       oppSpell,
		Effect:     collectSpellEffect(oppSpell),
	}
	PushStackItem(gs, oppItem)

	cmdrFree := &AlternativeCost{
		Kind:  AltCostKindCommanderFree,
		Label: "Fierce Guardianship commander free",
	}

	// Commander free should NOT be available.
	if CanPayAlternativeCost(gs, 0, fg, cmdrFree) {
		t.Fatal("commander_free should not be available without commander on battlefield")
	}

	// Cast normally for 3 mana.
	result, err := CastSpellWithCosts(gs, 0, fg, nil, cmdrFree, nil, false)
	if err != nil {
		t.Fatalf("normal cast failed: %v", err)
	}

	if result.UsedAlternativeCost {
		t.Error("should not have used alternative cost")
	}

	// Mana should be 0 (paid 3).
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana 0 after paying 3, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// 6. Culling the Weak — sacrifice a creature, add {B}{B}{B}{B}
// ---------------------------------------------------------------------------

func TestCosts_CullingTheWeak_SacrificeForMana(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1 // 1 mana for the spell itself

	// Put a creature on battlefield to sacrifice.
	creature := addBattlefield(gs, 0, "Birds of Paradise", 0, 1, "creature")
	_ = creature

	// Culling the Weak costs {B} (1 mana for MVP) and has an additional
	// cost: sacrifice a creature. Its effect adds {B}{B}{B}{B} (4 mana).
	cw := addHandCardWithEffect(gs, 0, "Culling the Weak", 1,
		&gameast.AddMana{
			Pool: []gameast.ManaSymbol{
				{Raw: "{B}", Color: []string{"B"}},
				{Raw: "{B}", Color: []string{"B"}},
				{Raw: "{B}", Color: []string{"B"}},
				{Raw: "{B}", Color: []string{"B"}},
			},
		}, "instant")

	sacCost := &AdditionalCost{
		Kind:            AddCostKindSacrifice,
		Label:           "sacrifice a creature",
		SacrificeFilter: "creature",
	}

	// Verify the sac cost is payable.
	if !CanPayAdditionalCost(gs, 0, sacCost) {
		t.Fatal("should be able to sacrifice a creature")
	}

	result, err := CastSpellWithCosts(gs, 0, cw, nil, nil, []*AdditionalCost{sacCost}, false)
	if err != nil {
		t.Fatalf("CastSpellWithCosts failed: %v", err)
	}

	// Creature should have been sacrificed.
	if len(result.SacrificedPermanents) != 1 {
		t.Errorf("expected 1 sacrificed permanent, got %d", len(result.SacrificedPermanents))
	}

	// Creature should NOT be on the battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Birds of Paradise" {
			t.Error("Birds of Paradise should not be on battlefield after sacrifice")
		}
	}

	// Birds of Paradise card should be in graveyard.
	bopInGraveyard := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Birds of Paradise" {
			bopInGraveyard = true
			break
		}
	}
	if !bopInGraveyard {
		t.Error("Birds of Paradise should be in graveyard after sacrifice")
	}

	// Culling the Weak should have resolved and added 4 mana. The spell
	// costs 1, so net mana = 0 - 1 + 4 = 3 (the mana from the AddMana
	// effect). However, since the resolve uses ResolveEffect which adds
	// to ManaPool, we check the pool is >= 4 (the added mana).
	// Actually: started with 1, paid 1 for the spell, resolve adds 4.
	// ManaPool should be 4 after resolution.
	if gs.Seats[0].ManaPool < 4 {
		t.Errorf("expected at least 4 mana after Culling the Weak, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// 7. Sacrifice-as-cost — cannot cast without a creature to sacrifice
// ---------------------------------------------------------------------------

func TestCosts_SacrificeCost_NoCreature(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	cw := addHandCard(gs, 0, "Culling the Weak", 1, "instant")

	sacCost := &AdditionalCost{
		Kind:            AddCostKindSacrifice,
		Label:           "sacrifice a creature",
		SacrificeFilter: "creature",
	}

	if CanPayAdditionalCost(gs, 0, sacCost) {
		t.Fatal("should NOT be able to sacrifice a creature with no creatures on battlefield")
	}

	_, err := CastSpellWithCosts(gs, 0, cw, nil, nil, []*AdditionalCost{sacCost}, false)
	if err == nil {
		t.Fatal("expected error when no creature to sacrifice")
	}
}

// ---------------------------------------------------------------------------
// 8. Mindbreak Trap — free when 3+ spells cast this turn
// ---------------------------------------------------------------------------

func TestCosts_MindbreakTrap_SpellCountThreshold(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 1 // opponent's turn
	gs.Seats[0].ManaPool = 0
	// Simulate 3 opponent spells cast this turn.
	gs.SpellsCastThisTurn = 3
	gs.Seats[1].SpellsCastThisTurn = 3

	mb := addHandCard(gs, 0, "Mindbreak Trap", 4, "instant")

	spellCountAlt := &AlternativeCost{
		Kind:                   AltCostKindSpellCount,
		Label:                  "Mindbreak Trap trap cost",
		SpellCountThreshold:    3,
		SpellCountOpponentOnly: true,
	}

	if !CanPayAlternativeCost(gs, 0, mb, spellCountAlt) {
		t.Fatal("Mindbreak Trap should be free when 3+ opponent spells cast")
	}
}

// ---------------------------------------------------------------------------
// 9. Mindbreak Trap — NOT free when < 3 opponent spells
// ---------------------------------------------------------------------------

func TestCosts_MindbreakTrap_BelowThreshold(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 1
	gs.Seats[0].ManaPool = 0
	// Only 2 opponent spells.
	gs.SpellsCastThisTurn = 2
	gs.Seats[1].SpellsCastThisTurn = 2

	mb := addHandCard(gs, 0, "Mindbreak Trap", 4, "instant")

	spellCountAlt := &AlternativeCost{
		Kind:                   AltCostKindSpellCount,
		Label:                  "Mindbreak Trap trap cost",
		SpellCountThreshold:    3,
		SpellCountOpponentOnly: true,
	}

	if CanPayAlternativeCost(gs, 0, mb, spellCountAlt) {
		t.Fatal("Mindbreak Trap should NOT be free with only 2 opponent spells")
	}
}

// ---------------------------------------------------------------------------
// 10. Evoke — Solitude cast for evoke cost, sacrificed on ETB
// ---------------------------------------------------------------------------

func TestCosts_Evoke_SolitudeSacrificeOnETB(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1 // evoke cost {W} = 1 for MVP

	// Solitude is a creature with evoke.
	sol := addHandCard(gs, 0, "Solitude", 5, "creature")
	sol.Colors = []string{"W"}

	evokeCost := &AlternativeCost{
		Kind:      AltCostKindEvoke,
		Label:     "evoke {W}",
		EvokeMana: 1,
	}

	if !CanPayAlternativeCost(gs, 0, sol, evokeCost) {
		t.Fatal("should be able to pay evoke cost with 1 mana")
	}

	result, err := CastSpellWithCosts(gs, 0, sol, nil, evokeCost, nil, true)
	if err != nil {
		t.Fatalf("CastSpellWithCosts failed: %v", err)
	}

	if !result.EvokeUsed {
		t.Error("expected EvokeUsed to be true")
	}

	// After resolution, Solitude should have entered the battlefield and
	// been sacrificed (evoke trigger). It should be in graveyard.
	solOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Solitude" {
			solOnBF = true
		}
	}
	if solOnBF {
		t.Error("Solitude should NOT be on battlefield after evoke sacrifice")
	}

	solInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Solitude" {
			solInGY = true
			break
		}
	}
	if !solInGY {
		t.Error("Solitude should be in graveyard after evoke sacrifice")
	}

	// Check for evoke_sacrifice_trigger event.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "evoke_sacrifice_trigger" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected evoke_sacrifice_trigger event in log")
	}
}

// ---------------------------------------------------------------------------
// 11. ControlsCommander — false when commander not on battlefield
// ---------------------------------------------------------------------------

func TestCosts_ControlsCommander_NotOnBattlefield(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"Kenrith, the Returned King"}
	// Commander is in command zone, not on battlefield.
	gs.Seats[0].CommandZone = []*Card{
		{Name: "Kenrith, the Returned King", Owner: 0},
	}

	if ControlsCommander(gs, 0) {
		t.Fatal("should not control commander when it's in command zone")
	}
}

// ---------------------------------------------------------------------------
// 12. ControlsCommander — true when commander is on battlefield
// ---------------------------------------------------------------------------

func TestCosts_ControlsCommander_OnBattlefield(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"Kenrith, the Returned King"}

	addBattlefield(gs, 0, "Kenrith, the Returned King", 5, 5,
		"legendary", "creature")

	if !ControlsCommander(gs, 0) {
		t.Fatal("should control commander when it's on battlefield")
	}
}

// ---------------------------------------------------------------------------
// 13. CardHasColor — basic checks
// ---------------------------------------------------------------------------

func TestCosts_CardHasColor(t *testing.T) {
	card := &Card{Name: "Test", Colors: []string{"U", "B"}}
	if !CardHasColor(card, "U") {
		t.Error("card should have blue")
	}
	if !CardHasColor(card, "B") {
		t.Error("card should have black")
	}
	if CardHasColor(card, "R") {
		t.Error("card should not have red")
	}
	if CardHasColor(nil, "U") {
		t.Error("nil card should not have any color")
	}
}

// ---------------------------------------------------------------------------
// 14. AdditionalCost — pay life
// ---------------------------------------------------------------------------

func TestCosts_PayLifeAdditional(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 20

	add := &AdditionalCost{
		Kind:       AddCostKindPayLife,
		Label:      "pay 3 life",
		LifeAmount: 3,
	}

	if !CanPayAdditionalCost(gs, 0, add) {
		t.Fatal("should be able to pay 3 life at 20 life")
	}

	card := &Card{Name: "Test Spell"}
	result := PayAdditionalCost(gs, 0, card, add)
	if result == nil {
		t.Fatal("PayAdditionalCost returned nil")
	}
	if result.LifePaid != 3 {
		t.Errorf("expected 3 life paid, got %d", result.LifePaid)
	}
	if gs.Seats[0].Life != 17 {
		t.Errorf("expected life 17, got %d", gs.Seats[0].Life)
	}
}

// ---------------------------------------------------------------------------
// 15. AdditionalCost — cannot pay life when it would kill you
// ---------------------------------------------------------------------------

func TestCosts_PayLifeAdditional_WouldKill(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 3

	add := &AdditionalCost{
		Kind:       AddCostKindPayLife,
		Label:      "pay 3 life",
		LifeAmount: 3,
	}

	// Life is exactly 3, paying 3 would drop to 0 — not allowed (must survive).
	if CanPayAdditionalCost(gs, 0, add) {
		t.Fatal("should NOT be able to pay 3 life at exactly 3 life")
	}
}
