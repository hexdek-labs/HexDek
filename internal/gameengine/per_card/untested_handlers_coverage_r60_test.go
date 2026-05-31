package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Untested per_card handler coverage sweep (R60)
//
// Adds focused regression tests for 7 high-traffic Commander handlers that
// shipped without dedicated test coverage. Each test pins the happy path
// plus 1-2 edge cases (gating, controller-scope, mana/cost handling).
//
// Three handlers in this sweep (Adrix and Nev, Anafenza Kin-Tree, Genesis
// Chamber) were also silently inert before this PR because they read
// ctx["permanent"] while the engine fires permanent_etb with ctx["perm"]
// (verified at etb_dispatch.go:126 and stack.go:1640). The handler bodies
// in this PR add the canonical ctx["perm"] read with the ctx["permanent"]
// fallback (same pattern as Wild Pair / Crown of Gondor / Preston the
// Vanisher). Tests fire with the engine-canonical "perm" key.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Aggravated Assault — {5}: untap your creatures + extra combat
// ---------------------------------------------------------------------------

func TestAggravatedAssault_HappyPathUntapsAndQueuesCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	aa := addPerm(gs, 0, "Aggravated Assault", "enchantment")
	gs.Seats[0].ManaPool = 5

	atkr := addPerm(gs, 0, "Llanowar Elves", "creature")
	atkr.Tapped = true
	other := addPerm(gs, 0, "Sol Ring", "artifact")
	other.Tapped = true // non-creature, should NOT be untapped

	gameengine.InvokeActivatedHook(gs, aa, 0, nil)

	if atkr.Tapped {
		t.Errorf("expected creature untapped after Aggravated Assault, still tapped")
	}
	if !other.Tapped {
		t.Errorf("non-creature artifact should NOT be untapped")
	}
	if len(gs.PendingExtraCombats) != 1 {
		t.Errorf("expected 1 pending extra combat, got %d", len(gs.PendingExtraCombats))
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana pool drained to 0, got %d", gs.Seats[0].ManaPool)
	}
}

func TestAggravatedAssault_FailsOnInsufficientMana(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	aa := addPerm(gs, 0, "Aggravated Assault", "enchantment")
	gs.Seats[0].ManaPool = 4 // short by 1

	gameengine.InvokeActivatedHook(gs, aa, 0, nil)

	if len(gs.PendingExtraCombats) != 0 {
		t.Errorf("should not queue extra combat when mana insufficient")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event on insufficient mana")
	}
}

func TestAggravatedAssault_FailsOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1 // opponent's turn — restriction violated
	aa := addPerm(gs, 0, "Aggravated Assault", "enchantment")
	gs.Seats[0].ManaPool = 10

	gameengine.InvokeActivatedHook(gs, aa, 0, nil)

	if len(gs.PendingExtraCombats) != 0 {
		t.Errorf("should not activate during opponent's turn")
	}
	if gs.Seats[0].ManaPool != 10 {
		t.Errorf("mana should not be spent on failed activation, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Adrix and Nev, Twincasters — token-doubling on ETB
// ---------------------------------------------------------------------------

func TestAdrixAndNev_DoublesOwnTokenETB(t *testing.T) {
	gs := newGame(t, 2)
	adrix := addPerm(gs, 0, "Adrix and Nev, Twincasters", "creature", "legendary", "merfolk", "wizard")
	adrix.Card.BasePower = 3
	adrix.Card.BaseToughness = 3

	tokenPerm := addPerm(gs, 0, "Faerie Rogue", "token", "creature", "faerie", "rogue")
	tokenPerm.Card.BasePower = 1
	tokenPerm.Card.BaseToughness = 1

	before := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            tokenPerm,
		"card":            tokenPerm.Card,
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Errorf("expected one doubled token added, battlefield went %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
	foundCopy := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Flags["adrix_copy"] == 1 {
			foundCopy = true
		}
	}
	if !foundCopy {
		t.Errorf("expected a permanent with Flags[adrix_copy]=1 on battlefield")
	}
}

func TestAdrixAndNev_DoesNotDoubleNonTokenETB(t *testing.T) {
	gs := newGame(t, 2)
	adrix := addPerm(gs, 0, "Adrix and Nev, Twincasters", "creature", "legendary", "merfolk", "wizard")
	adrix.Card.BasePower = 3
	adrix.Card.BaseToughness = 3

	// Nontoken creature ETB — must NOT trigger doubling.
	real := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	real.Card.BasePower = 1
	real.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            real,
		"card":            real.Card,
	})

	if len(gs.Seats[0].Battlefield) != before {
		t.Errorf("nontoken ETB should not mint copy, battlefield grew %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
}

func TestAdrixAndNev_DoesNotDoubleOpponentToken(t *testing.T) {
	gs := newGame(t, 2)
	adrix := addPerm(gs, 0, "Adrix and Nev, Twincasters", "creature", "legendary", "merfolk", "wizard")
	adrix.Card.BasePower = 3
	adrix.Card.BaseToughness = 3

	// Opponent mints a token — Adrix's "tokens YOU control" gate must block.
	oppTok := addPerm(gs, 1, "Treasure Token", "token", "artifact")
	beforeOpp := len(gs.Seats[1].Battlefield)
	beforeOwn := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 1,
		"perm":            oppTok,
		"card":            oppTok.Card,
	})

	if len(gs.Seats[1].Battlefield) != beforeOpp {
		t.Errorf("opponent's token should not be doubled, opp battlefield %d → %d",
			beforeOpp, len(gs.Seats[1].Battlefield))
	}
	if len(gs.Seats[0].Battlefield) != beforeOwn {
		t.Errorf("Adrix should not mint to own battlefield from opp's ETB, own %d → %d",
			beforeOwn, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Adeliz, the Cinder Wind — pump Wizards on instant/sorcery cast
// ---------------------------------------------------------------------------

func TestAdeliz_PumpsAllWizardsOnInstantCast(t *testing.T) {
	gs := newGame(t, 2)
	adeliz := addPerm(gs, 0, "Adeliz, the Cinder Wind", "creature", "legendary", "human", "wizard")
	adeliz.Card.BasePower = 2
	adeliz.Card.BaseToughness = 2
	otherWiz := addPerm(gs, 0, "Soothsaying Sage", "creature", "wizard")
	otherWiz.Card.BasePower = 1
	otherWiz.Card.BaseToughness = 3
	notWiz := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	notWiz.Card.BasePower = 1
	notWiz.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 0,
	})

	if adeliz.Flags["temp_power"] != 1 || adeliz.Flags["temp_toughness"] != 1 {
		t.Errorf("Adeliz herself (wizard) should be pumped +1/+1, got P=%d T=%d",
			adeliz.Flags["temp_power"], adeliz.Flags["temp_toughness"])
	}
	if otherWiz.Flags["temp_power"] != 1 || otherWiz.Flags["temp_toughness"] != 1 {
		t.Errorf("other wizard should be pumped, got P=%d T=%d",
			otherWiz.Flags["temp_power"], otherWiz.Flags["temp_toughness"])
	}
	if notWiz.Flags["temp_power"] != 0 || notWiz.Flags["temp_toughness"] != 0 {
		t.Errorf("non-wizard should NOT be pumped, got P=%d T=%d",
			notWiz.Flags["temp_power"], notWiz.Flags["temp_toughness"])
	}
}

func TestAdeliz_DoesNotPumpOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	adeliz := addPerm(gs, 0, "Adeliz, the Cinder Wind", "creature", "legendary", "human", "wizard")
	adeliz.Card.BasePower = 2
	adeliz.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 1, // opponent casts
	})

	if adeliz.Flags["temp_power"] != 0 {
		t.Errorf("opponent's cast should not pump our wizards; temp_power=%d",
			adeliz.Flags["temp_power"])
	}
}

// ---------------------------------------------------------------------------
// Alela, Cunning Conqueror — first-spell-each-opp-turn Faerie token + tap
// ---------------------------------------------------------------------------

func TestAlela_MintsFaerieOnFirstOppTurnSpell(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1 // opponent's turn
	gs.Turn = 5   // non-zero so the "first cast this turn" flag check is meaningful
	alela := addPerm(gs, 0, "Alela, Cunning Conqueror", "creature", "legendary", "faerie", "warlock")
	alela.Card.BasePower = 2
	alela.Card.BaseToughness = 3
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Errorf("expected one Faerie token minted, battlefield %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
	if alela.Flags["alela_cast_this_turn"] != gs.Turn {
		t.Errorf("expected first-spell flag stamped, got %d", alela.Flags["alela_cast_this_turn"])
	}

	// Second cast same opp turn should NOT mint again.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
	})
	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Errorf("second spell same opp turn should be gated; battlefield grew to %d",
			len(gs.Seats[0].Battlefield))
	}
}

func TestAlela_DoesNotMintOnOwnTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0 // Alela's own turn
	gs.Turn = 5
	alela := addPerm(gs, 0, "Alela, Cunning Conqueror", "creature", "legendary", "faerie", "warlock")
	alela.Card.BasePower = 2
	alela.Card.BaseToughness = 3
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
	})

	if len(gs.Seats[0].Battlefield) != before {
		t.Errorf("should not mint on own turn; battlefield %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
}

func TestAlela_TapsOppPermOnFaerieCombatDamage(t *testing.T) {
	gs := newGame(t, 2)
	alela := addPerm(gs, 0, "Alela, Cunning Conqueror", "creature", "legendary", "faerie", "warlock")
	alela.Card.BasePower = 2
	alela.Card.BaseToughness = 3

	// Alela's own Faerie attacker on the battlefield.
	faerie := addPerm(gs, 0, "Faerie Rogue", "creature", "faerie", "rogue")
	faerie.Card.BasePower = 1
	faerie.Card.BaseToughness = 1

	// Opponent has a stronger and a weaker untapped nonland perm.
	weak := addPerm(gs, 1, "Birds of Paradise", "creature")
	weak.Card.BasePower = 0
	weak.Card.BaseToughness = 1
	strong := addPerm(gs, 1, "Grizzly Bears", "creature")
	strong.Card.BasePower = 2
	strong.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Faerie Rogue",
		"defender_seat": 1,
	})

	if !strong.Tapped {
		t.Errorf("expected highest-power opp perm tapped, but it's still untapped")
	}
	if weak.Tapped {
		t.Errorf("weaker opp perm should NOT have been chosen")
	}
}

// ---------------------------------------------------------------------------
// Anafenza, Kin-Tree Spirit — bolster 1 on nontoken creature ETB
// ---------------------------------------------------------------------------

func TestAnafenza_BolstersLowestToughnessCreature(t *testing.T) {
	gs := newGame(t, 2)
	anaf := addPerm(gs, 0, "Anafenza, Kin-Tree Spirit", "creature", "legendary", "spirit", "soldier")
	anaf.Card.BasePower = 2
	anaf.Card.BaseToughness = 2

	runt := addPerm(gs, 0, "Squire", "creature", "human")
	runt.Card.BasePower = 1
	runt.Card.BaseToughness = 1
	beefy := addPerm(gs, 0, "Serra Angel", "creature", "angel")
	beefy.Card.BasePower = 4
	beefy.Card.BaseToughness = 4

	// Another non-token creature enters and triggers bolster.
	newby := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	newby.Card.BasePower = 2
	newby.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            newby,
		"card":            newby.Card,
	})

	if runt.Counters["+1/+1"] != 1 {
		t.Errorf("expected runt bolstered (+1/+1 counter), got counters=%v", runt.Counters)
	}
	if beefy.Counters["+1/+1"] != 0 {
		t.Errorf("beefy creature should NOT be bolstered, got counters=%v", beefy.Counters)
	}
}

func TestAnafenza_DoesNotTriggerOnToken(t *testing.T) {
	gs := newGame(t, 2)
	anaf := addPerm(gs, 0, "Anafenza, Kin-Tree Spirit", "creature", "legendary", "spirit", "soldier")
	anaf.Card.BasePower = 2
	anaf.Card.BaseToughness = 2

	runt := addPerm(gs, 0, "Squire", "creature", "human")
	runt.Card.BasePower = 1
	runt.Card.BaseToughness = 1

	// Token ETB — Anafenza's "nontoken" gate must block.
	tok := addPerm(gs, 0, "Saproling Token", "token", "creature", "saproling")
	tok.Card.BasePower = 1
	tok.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            tok,
		"card":            tok.Card,
	})

	if runt.Counters["+1/+1"] != 0 {
		t.Errorf("token ETB should NOT bolster, got runt counters=%v", runt.Counters)
	}
}

func TestAnafenza_DoesNotTriggerOnOpponentETB(t *testing.T) {
	gs := newGame(t, 2)
	anaf := addPerm(gs, 0, "Anafenza, Kin-Tree Spirit", "creature", "legendary", "spirit", "soldier")
	anaf.Card.BasePower = 2
	anaf.Card.BaseToughness = 2
	runt := addPerm(gs, 0, "Squire", "creature", "human")
	runt.Card.BasePower = 1
	runt.Card.BaseToughness = 1

	// Opponent creature enters — "creature YOU control" gate must block.
	opp := addPerm(gs, 1, "Goblin Guide", "creature", "goblin")
	opp.Card.BasePower = 2
	opp.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 1,
		"perm":            opp,
		"card":            opp.Card,
	})

	if runt.Counters["+1/+1"] != 0 {
		t.Errorf("opponent's creature ETB should not bolster us, got %v", runt.Counters)
	}
}

// ---------------------------------------------------------------------------
// Alesha, Who Smiles at Death — attack-trigger reanimate ≤2 power
// ---------------------------------------------------------------------------

func TestAlesha_ReanimatesPower2FromGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	alesha := addPerm(gs, 0, "Alesha, Who Smiles at Death", "creature", "legendary", "human", "warrior")
	alesha.Card.BasePower = 3
	alesha.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(alesha, 1)

	// Two creatures in yard — one too big (3 power), one valid (2 power).
	target := addCardToGraveyard(gs, 0, "Mardu Skullhunter", "creature", "human", "warrior")
	target.BasePower = 2
	target.BaseToughness = 1
	tooBig := addCardToGraveyard(gs, 0, "Serra Angel", "creature", "angel")
	tooBig.BasePower = 4
	tooBig.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": alesha,
	})

	// Skullhunter should now be on the battlefield.
	foundTarget := false
	foundTooBig := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == target {
			foundTarget = true
			if !p.Tapped {
				t.Errorf("reanimated creature should be tapped (attacking)")
			}
		}
		if p.Card == tooBig {
			foundTooBig = true
		}
	}
	if !foundTarget {
		t.Errorf("expected Mardu Skullhunter on battlefield post-Alesha")
	}
	if foundTooBig {
		t.Errorf("Serra Angel (4 power) should NOT be reanimable by Alesha")
	}
}

func TestAlesha_FailsGracefullyWithNoEligibleTarget(t *testing.T) {
	gs := newGame(t, 2)
	alesha := addPerm(gs, 0, "Alesha, Who Smiles at Death", "creature", "legendary", "human", "warrior")
	alesha.Card.BasePower = 3
	alesha.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(alesha, 1)
	// Only a too-big creature in graveyard.
	tooBig := addCardToGraveyard(gs, 0, "Akroma, Angel of Wrath", "creature", "angel")
	tooBig.BasePower = 6

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": alesha,
	})

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event when no eligible target in yard")
	}
}

func TestAlesha_DoesNotTriggerOnOtherAttacker(t *testing.T) {
	gs := newGame(t, 2)
	alesha := addPerm(gs, 0, "Alesha, Who Smiles at Death", "creature", "legendary", "human", "warrior")
	alesha.Card.BasePower = 3
	alesha.Card.BaseToughness = 2
	other := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	other.Card.BasePower = 2
	other.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(other, 1)

	in := addCardToGraveyard(gs, 0, "Mardu Skullhunter", "creature", "human", "warrior")
	in.BasePower = 2

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": other, // a non-Alesha attacker
	})

	// Skullhunter should stay in graveyard.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == in {
			t.Errorf("Alesha should only trigger on her own attack, not on %s", other.Card.DisplayName())
		}
	}
}

// ---------------------------------------------------------------------------
// Genesis Chamber — symmetric Myr token on nontoken creature ETB
// ---------------------------------------------------------------------------

func TestGenesisChamber_MintsMyrForOwnCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	gc := addPerm(gs, 0, "Genesis Chamber", "artifact")
	_ = gc

	newby := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	newby.Card.BasePower = 1
	newby.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            newby,
		"card":            newby.Card,
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Errorf("expected Myr token minted to own seat, battlefield %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
	foundMyr := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Myr Token" {
			foundMyr = true
		}
	}
	if !foundMyr {
		t.Errorf("expected Myr Token among own permanents")
	}
}

func TestGenesisChamber_MintsToOppForOppCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Genesis Chamber", "artifact")

	// Opponent's nontoken creature ETBs — token goes to OPPONENT (symmetric).
	oppNewby := addPerm(gs, 1, "Goblin Guide", "creature", "goblin")
	oppNewby.Card.BasePower = 2
	oppNewby.Card.BaseToughness = 2
	beforeOwn := len(gs.Seats[0].Battlefield)
	beforeOpp := len(gs.Seats[1].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 1,
		"perm":            oppNewby,
		"card":            oppNewby.Card,
	})

	if len(gs.Seats[1].Battlefield) != beforeOpp+1 {
		t.Errorf("symmetric Genesis Chamber should mint to opp, opp BF %d → %d",
			beforeOpp, len(gs.Seats[1].Battlefield))
	}
	if len(gs.Seats[0].Battlefield) != beforeOwn {
		t.Errorf("token must NOT go to Genesis Chamber controller on opp's ETB, own %d → %d",
			beforeOwn, len(gs.Seats[0].Battlefield))
	}
}

func TestGenesisChamber_DoesNotTriggerWhenTapped(t *testing.T) {
	gs := newGame(t, 2)
	gc := addPerm(gs, 0, "Genesis Chamber", "artifact")
	gc.Tapped = true // "if this artifact is untapped" gate

	newby := addPerm(gs, 0, "Llanowar Elves", "creature")
	newby.Card.BasePower = 1
	newby.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            newby,
		"card":            newby.Card,
	})

	if len(gs.Seats[0].Battlefield) != before {
		t.Errorf("tapped Genesis Chamber should not mint, BF %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
}
