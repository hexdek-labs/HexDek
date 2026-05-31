package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// per_card test coverage batch 3 (R60)
//
// Sibling to the previous coverage batches (PR #815 + the wave-2 patch).
// 11 high-traffic Commander-staple handlers that shipped without
// dedicated tests. Each test pins the happy path + 1-2 gating edges.
//
// Cards covered:
//   - Marwyn, the Nurturer       (elf-counter ETB)
//   - Purphoros, God of the Forge (creature ETB ping)
//   - Talrand, Sky Summoner       (instant/sorcery cast drake)
//   - Mizzix of the Izmagnus      (xp counter on big instant/sorcery)
//   - Sram, Senior Edificer       (aura/equipment/vehicle cast draw)
//   - Sheoldred, the Apocalypse   (symmetric card_drawn lifegain/loss)
//   - Selvala, Heart of the Wilds (strongest-creature ETB cantrip)
//   - Savra, Queen of the Golgari (sac-color triggers)
//   - Karlach, Fury of Avernus    (first-combat extra combat + untap)
//   - Pia Nalaar, Consul of Revival (artifact-creature-dies → Thopter)
//   - Yawgmoth, Thran Physician   (sac+life-pay → counter+draw activation)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Marwyn, the Nurturer
// ---------------------------------------------------------------------------

func TestMarwyn_AddsCounterOnAnotherElfETB(t *testing.T) {
	gs := newGame(t, 2)
	marwyn := addPerm(gs, 0, "Marwyn, the Nurturer", "creature", "legendary", "elf", "druid")
	marwyn.Card.BasePower = 1
	marwyn.Card.BaseToughness = 1

	elf := &gameengine.Card{
		Name:          "Llanowar Elves",
		Owner:         0,
		Types:         []string{"creature", "elf", "druid"},
		BasePower:     1,
		BaseToughness: 1,
	}
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"card":            elf,
	})

	if marwyn.Counters["+1/+1"] != 1 {
		t.Errorf("expected Marwyn +1/+1 after elf ETB, got %v", marwyn.Counters)
	}
}

func TestMarwyn_DoesNotCounterNonElfETB(t *testing.T) {
	gs := newGame(t, 2)
	marwyn := addPerm(gs, 0, "Marwyn, the Nurturer", "creature", "legendary", "elf", "druid")
	marwyn.Card.BasePower = 1
	marwyn.Card.BaseToughness = 1

	notElf := &gameengine.Card{
		Name:  "Grizzly Bears",
		Owner: 0,
		Types: []string{"creature", "bear"},
	}
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"card":            notElf,
	})

	if marwyn.Counters["+1/+1"] != 0 {
		t.Errorf("non-elf ETB should not trigger Marwyn, got %v", marwyn.Counters)
	}
}

// ---------------------------------------------------------------------------
// Purphoros, God of the Forge
// ---------------------------------------------------------------------------

func TestPurphoros_PingsOpponentsOnOwnCreatureETB(t *testing.T) {
	gs := newGame(t, 3)
	purph := addPerm(gs, 0, "Purphoros, God of the Forge", "creature", "legendary", "god")
	purph.Card.BasePower = 4
	purph.Card.BaseToughness = 4

	startOpp1 := gs.Seats[1].Life
	startOpp2 := gs.Seats[2].Life

	other := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	other.Card.BasePower = 1
	other.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            other,
		"card":            other.Card,
	})

	if gs.Seats[1].Life != startOpp1-2 {
		t.Errorf("expected seat 1 to take 2 dmg, %d → %d", startOpp1, gs.Seats[1].Life)
	}
	if gs.Seats[2].Life != startOpp2-2 {
		t.Errorf("expected seat 2 to take 2 dmg, %d → %d", startOpp2, gs.Seats[2].Life)
	}
	_ = purph
}

func TestPurphoros_NoPingOnOpponentCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Purphoros, God of the Forge", "creature", "legendary", "god")
	startOpp := gs.Seats[1].Life

	oppCreature := addPerm(gs, 1, "Goblin Guide", "creature", "goblin")
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 1,
		"perm":            oppCreature,
		"card":            oppCreature.Card,
	})

	if gs.Seats[1].Life != startOpp {
		t.Errorf("opp's creature ETB should not ping; life %d → %d", startOpp, gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// Talrand, Sky Summoner
// ---------------------------------------------------------------------------

func TestTalrand_MintsDrakeOnOwnInstantCast(t *testing.T) {
	gs := newGame(t, 2)
	tal := addPerm(gs, 0, "Talrand, Sky Summoner", "creature", "legendary", "merfolk", "wizard")
	tal.Card.BasePower = 2
	tal.Card.BaseToughness = 2
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 0,
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Errorf("expected 1 Drake minted, battlefield %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
	foundDrake := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Drake Token" {
			foundDrake = true
		}
	}
	if !foundDrake {
		t.Errorf("expected Drake Token on battlefield")
	}
}

func TestTalrand_NoDrakeOnOppCast(t *testing.T) {
	gs := newGame(t, 2)
	tal := addPerm(gs, 0, "Talrand, Sky Summoner", "creature", "legendary", "merfolk", "wizard")
	tal.Card.BasePower = 2
	tal.Card.BaseToughness = 2
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 1, // opp cast
	})

	if len(gs.Seats[0].Battlefield) != before {
		t.Errorf("opp instant cast should not mint Drake for Talrand; bf %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Mizzix of the Izmagnus
// ---------------------------------------------------------------------------

func TestMizzix_GainsXPCounterOnBigInstantCast(t *testing.T) {
	gs := newGame(t, 2)
	mz := addPerm(gs, 0, "Mizzix of the Izmagnus", "creature", "legendary", "wizard")
	mz.Card.BasePower = 2
	mz.Card.BaseToughness = 2

	spell := &gameengine.Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant", "cost:1"},
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        spell,
	})

	if gs.Seats[0].Flags["experience_counters"] != 1 {
		t.Errorf("expected 1 xp counter from CMC-1 instant > 0 xp, got %d",
			gs.Seats[0].Flags["experience_counters"])
	}
}

func TestMizzix_DoesNotGainXPWhenCMCLeXP(t *testing.T) {
	gs := newGame(t, 2)
	mz := addPerm(gs, 0, "Mizzix of the Izmagnus", "creature", "legendary", "wizard")
	mz.Card.BasePower = 2
	mz.Card.BaseToughness = 2
	gs.Seats[0].Flags = map[string]int{"experience_counters": 3}

	spell := &gameengine.Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant", "cost:2"},
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        spell,
	})

	if gs.Seats[0].Flags["experience_counters"] != 3 {
		t.Errorf("xp 3 ≥ cmc 2 — should NOT bump, got %d", gs.Seats[0].Flags["experience_counters"])
	}
}

func TestMizzix_IgnoresCreatureSpells(t *testing.T) {
	gs := newGame(t, 2)
	mz := addPerm(gs, 0, "Mizzix of the Izmagnus", "creature", "legendary", "wizard")
	mz.Card.BasePower = 2
	mz.Card.BaseToughness = 2

	creature := &gameengine.Card{
		Name:  "Grizzly Bears",
		Owner: 0,
		Types: []string{"creature", "bear", "cost:2"},
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        creature,
	})

	if gs.Seats[0].Flags["experience_counters"] != 0 {
		t.Errorf("creature cast should not trigger Mizzix xp, got %d",
			gs.Seats[0].Flags["experience_counters"])
	}
}

// ---------------------------------------------------------------------------
// Sram, Senior Edificer
// ---------------------------------------------------------------------------

func TestSram_DrawsOnOwnEquipmentCast(t *testing.T) {
	gs := newGame(t, 2)
	sram := addPerm(gs, 0, "Sram, Senior Edificer", "creature", "legendary", "dwarf", "advisor")
	sram.Card.BasePower = 1
	sram.Card.BaseToughness = 2
	addLibrary(gs, 0, "A", "B", "C")
	startHand := len(gs.Seats[0].Hand)

	equipment := &gameengine.Card{
		Name:  "Skullclamp",
		Owner: 0,
		Types: []string{"artifact", "equipment"},
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        equipment,
	})

	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Errorf("expected Sram to draw 1 on equipment cast, hand %d → %d",
			startHand, len(gs.Seats[0].Hand))
	}
}

func TestSram_NoDrawOnPlainCreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	sram := addPerm(gs, 0, "Sram, Senior Edificer", "creature", "legendary", "dwarf", "advisor")
	sram.Card.BasePower = 1
	sram.Card.BaseToughness = 2
	addLibrary(gs, 0, "A", "B", "C")
	startHand := len(gs.Seats[0].Hand)

	creature := &gameengine.Card{
		Name:  "Grizzly Bears",
		Owner: 0,
		Types: []string{"creature", "bear"},
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        creature,
	})

	if len(gs.Seats[0].Hand) != startHand {
		t.Errorf("plain creature cast should not trigger Sram, hand %d → %d",
			startHand, len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Sheoldred, the Apocalypse
// ---------------------------------------------------------------------------

func TestSheoldred_GainsLifeOnControllerDraw(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sheoldred, the Apocalypse", "creature", "legendary", "phyrexian", "praetor")
	startLife := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"seat":          0,
		"drawer_seat":   0,
		"card":          "Brainstorm",
		"nth_this_turn": 1,
		"source":        "draw",
	})

	if gs.Seats[0].Life != startLife+2 {
		t.Errorf("expected +2 life for controller draw, %d → %d", startLife, gs.Seats[0].Life)
	}
}

func TestSheoldred_DrainsLifeOnOpponentDraw(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sheoldred, the Apocalypse", "creature", "legendary", "phyrexian", "praetor")
	startOpp := gs.Seats[1].Life

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"seat":          1,
		"drawer_seat":   1,
		"card":          "Brainstorm",
		"nth_this_turn": 1,
		"source":        "draw",
	})

	if gs.Seats[1].Life != startOpp-2 {
		t.Errorf("expected -2 life for opp draw, %d → %d", startOpp, gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// Selvala, Heart of the Wilds
// ---------------------------------------------------------------------------

func TestSelvala_DrawsWhenStrongestCreatureEnters(t *testing.T) {
	gs := newGame(t, 2)
	sel := addPerm(gs, 0, "Selvala, Heart of the Wilds", "creature", "legendary", "elf", "scout")
	sel.Card.BasePower = 2
	sel.Card.BaseToughness = 4

	weak := addPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	weak.Card.BasePower = 1
	weak.Card.BaseToughness = 1
	oppCreature := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")
	oppCreature.Card.BasePower = 2
	oppCreature.Card.BaseToughness = 2
	_ = weak

	addLibrary(gs, 0, "A", "B")
	startHand := len(gs.Seats[0].Hand)

	// New 5/5 creature enters seat 0 — strictly bigger than every other.
	titan := addPerm(gs, 0, "Hill Giant", "creature", "giant")
	titan.Card.BasePower = 5
	titan.Card.BaseToughness = 5
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            titan,
		"card":            titan.Card,
	})

	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Errorf("expected Selvala to draw for new biggest, hand %d → %d",
			startHand, len(gs.Seats[0].Hand))
	}
}

func TestSelvala_NoDrawWhenTiedOrSmaller(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Selvala, Heart of the Wilds", "creature", "legendary", "elf", "scout")

	existing := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")
	existing.Card.BasePower = 4
	existing.Card.BaseToughness = 4

	addLibrary(gs, 0, "A", "B")
	startHand := len(gs.Seats[0].Hand)

	// New creature at power 4 — tied with existing, not strictly greater.
	mid := addPerm(gs, 0, "Air Elemental", "creature", "elemental")
	mid.Card.BasePower = 4
	mid.Card.BaseToughness = 4
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            mid,
		"card":            mid.Card,
	})

	if len(gs.Seats[0].Hand) != startHand {
		t.Errorf("tied power should NOT draw; hand %d → %d", startHand, len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Savra, Queen of the Golgari
// ---------------------------------------------------------------------------

func TestSavra_GainsLifeOnGreenSac(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Savra, Queen of the Golgari", "creature", "legendary", "zombie", "elf")
	startLife := gs.Seats[0].Life

	greenCreature := &gameengine.Card{
		Name:   "Yavimaya Elder",
		Owner:  0,
		Types:  []string{"creature", "human", "druid"},
		Colors: []string{"G"},
	}
	gameengine.FireCardTrigger(gs, "permanent_sacrificed", map[string]interface{}{
		"controller_seat": 0,
		"card":            greenCreature,
		"card_name":       "Yavimaya Elder",
	})

	if gs.Seats[0].Life != startLife+2 {
		t.Errorf("expected +2 life on green sac, %d → %d", startLife, gs.Seats[0].Life)
	}
}

func TestSavra_NoTriggerOnOpponentSac(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Savra, Queen of the Golgari", "creature", "legendary", "zombie", "elf")
	startLife := gs.Seats[0].Life

	greenCreature := &gameengine.Card{
		Name:   "Llanowar Elves",
		Owner:  1,
		Types:  []string{"creature", "elf"},
		Colors: []string{"G"},
	}
	gameengine.FireCardTrigger(gs, "permanent_sacrificed", map[string]interface{}{
		"controller_seat": 1, // opp sac
		"card":            greenCreature,
		"card_name":       "Llanowar Elves",
	})

	if gs.Seats[0].Life != startLife {
		t.Errorf("opp's green sac should not trigger Savra; life %d → %d", startLife, gs.Seats[0].Life)
	}
}

// ---------------------------------------------------------------------------
// Karlach, Fury of Avernus
// ---------------------------------------------------------------------------

func TestKarlach_GrantsExtraCombatAndUntapsOnFirstCombatAttack(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Turn = 1
	karl := addPerm(gs, 0, "Karlach, Fury of Avernus", "creature", "legendary", "tiefling", "barbarian")
	karl.Card.BasePower = 5
	karl.Card.BaseToughness = 4
	gameengine.SetAttackerDefender(karl, 1)
	karl.Flags["attacking"] = 1
	karl.Tapped = true // attacking creatures are usually tapped

	// combat_begin seeds the combat-idx counter.
	gameengine.FireCardTrigger(gs, "combat_begin", map[string]interface{}{
		"active_seat": 0,
	})
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": karl,
		"attacker_seat": 0,
		"attacker_card": karl.Card,
	})

	if karl.Tapped {
		t.Errorf("expected Karlach untapped after first-combat trigger")
	}
	if karl.Flags["kw:first strike"] != 1 {
		t.Errorf("expected first-strike grant flag on Karlach")
	}
	if len(gs.PendingExtraCombats) != 1 {
		t.Errorf("expected 1 pending extra combat, got %d", len(gs.PendingExtraCombats))
	}
}

func TestKarlach_DoesNotFireTwiceInSameFirstCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Turn = 1
	karl := addPerm(gs, 0, "Karlach, Fury of Avernus", "creature", "legendary", "tiefling", "barbarian")
	karl.Card.BasePower = 5
	karl.Card.BaseToughness = 4
	gameengine.SetAttackerDefender(karl, 1)
	karl.Flags["attacking"] = 1
	ally := addPerm(gs, 0, "Goblin Guide", "creature", "goblin")
	ally.Card.BasePower = 2
	ally.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(ally, 1)
	ally.Flags["attacking"] = 1

	gameengine.FireCardTrigger(gs, "combat_begin", map[string]interface{}{
		"active_seat": 0,
	})
	// Two attackers fire creature_attacks separately — Karlach's gate is
	// per (turn, combat-idx), not per attacker.
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": karl,
		"attacker_seat": 0,
	})
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": ally,
		"attacker_seat": 0,
	})

	if len(gs.PendingExtraCombats) != 1 {
		t.Errorf("Karlach should add ONE extra combat per first-combat regardless of attacker count, got %d",
			len(gs.PendingExtraCombats))
	}
}

// ---------------------------------------------------------------------------
// Pia Nalaar, Consul of Revival
// ---------------------------------------------------------------------------

func TestPiaNalaar_MintsThopterOnOwnArtifactCreatureDies(t *testing.T) {
	gs := newGame(t, 2)
	pia := addPerm(gs, 0, "Pia Nalaar, Consul of Revival", "creature", "legendary", "human", "artificer")
	pia.Card.BasePower = 2
	pia.Card.BaseToughness = 2
	before := len(gs.Seats[0].Battlefield)

	dying := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  "Etched Champion",
			Owner: 0,
			Types: []string{"creature", "artifact", "soldier"},
		},
		Controller: 0,
		Owner:      0,
	}
	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"card":            dying.Card,
		"controller_seat": 0,
		"to_zone":         "graveyard",
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Errorf("expected Thopter minted, bf %d → %d", before, len(gs.Seats[0].Battlefield))
	}
	foundThopter := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Thopter Token" {
			foundThopter = true
			if !p.Tapped {
				t.Errorf("Pia Nalaar Thopter should enter tapped")
			}
		}
	}
	if !foundThopter {
		t.Errorf("expected Thopter Token on battlefield")
	}
}

func TestPiaNalaar_NoThopterOnNonArtifactCreatureDies(t *testing.T) {
	gs := newGame(t, 2)
	pia := addPerm(gs, 0, "Pia Nalaar, Consul of Revival", "creature", "legendary", "human", "artificer")
	pia.Card.BasePower = 2
	pia.Card.BaseToughness = 2
	before := len(gs.Seats[0].Battlefield)

	dying := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  "Llanowar Elves",
			Owner: 0,
			Types: []string{"creature", "elf"},
		},
		Controller: 0,
		Owner:      0,
	}
	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"card":            dying.Card,
		"controller_seat": 0,
		"to_zone":         "graveyard",
	})

	if len(gs.Seats[0].Battlefield) != before {
		t.Errorf("non-artifact death should not mint Thopter; bf %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Yawgmoth, Thran Physician — ability 0: sac creature, draw, -1/-1 counter
// ---------------------------------------------------------------------------

func TestYawgmoth_SacsCreatureDrawsAndCountersOpponent(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40 // Commander life so Yawg can pay 1.
	yawg := addPerm(gs, 0, "Yawgmoth, Thran Physician", "creature", "legendary", "phyrexian", "praetor")
	yawg.Card.BasePower = 2
	yawg.Card.BaseToughness = 4

	// Sac fodder.
	fodder := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	fodder.Card.BasePower = 1
	fodder.Card.BaseToughness = 1

	// Opponent creature to -1/-1.
	oppCreature := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")
	oppCreature.Card.BasePower = 2
	oppCreature.Card.BaseToughness = 2

	addLibrary(gs, 0, "Top1", "Top2")
	startHand := len(gs.Seats[0].Hand)
	startLife := gs.Seats[0].Life

	gameengine.InvokeActivatedHook(gs, yawg, 0, nil)

	// Yawg paid 1 life.
	if gs.Seats[0].Life != startLife-1 {
		t.Errorf("expected Yawg controller -1 life, %d → %d", startLife, gs.Seats[0].Life)
	}
	// Drew 1.
	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Errorf("expected Yawg controller +1 card, hand %d → %d", startHand, len(gs.Seats[0].Hand))
	}
	// Fodder should be off the battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == fodder {
			t.Errorf("Llanowar Elves should have been sacrificed")
		}
	}
	// Opp creature should have a -1/-1 counter.
	if oppCreature.Counters["-1/-1"] != 1 {
		t.Errorf("expected -1/-1 on opp creature, got %v", oppCreature.Counters)
	}
}

func TestYawgmoth_FailsWhenLifeTooLow(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 1
	yawg := addPerm(gs, 0, "Yawgmoth, Thran Physician", "creature", "legendary", "phyrexian", "praetor")
	yawg.Card.BasePower = 2
	yawg.Card.BaseToughness = 4
	fodder := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	fodder.Card.BasePower = 1
	fodder.Card.BaseToughness = 1

	gameengine.InvokeActivatedHook(gs, yawg, 0, nil)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when life ≤ 1")
	}
	// Fodder should still be on the battlefield.
	stillThere := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == fodder {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("Yawg should not have sacced when life-pay would self-kill")
	}
}
