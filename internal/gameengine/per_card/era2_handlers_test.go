package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// dev/era2-unification test suite. One focused test per handler plus a
// registration smoke check.

// ---------------------------------------------------------------------
// Sliver Gravemother — encore activated ability
// ---------------------------------------------------------------------

func TestSliverGravemother_EncoreSpawnsTokenPerOpponent(t *testing.T) {
	gs := newGame(t, 4)
	mother := addPerm(gs, 0, "Sliver Gravemother", "creature", "sliver", "legendary")
	gs.Seats[0].ManaPool = 5
	gs.Active = 0
	gs.Phase = "precombat_main" // sorcery-speed gate

	// Place a Sliver creature card in graveyard with CMC 3.
	dead := &gameengine.Card{
		Name:          "Two-Headed Sliver",
		Owner:         0,
		Types:         []string{"creature", "sliver", "cmc:3"},
		BasePower:     2,
		BaseToughness: 2,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, dead)

	bfBefore := len(gs.Seats[0].Battlefield)
	sliverGravemotherEncore(gs, mother, 0, nil)

	// 3 opponents → 3 tokens spawned.
	bfAfter := len(gs.Seats[0].Battlefield)
	if bfAfter-bfBefore != 3 {
		t.Errorf("expected 3 encore tokens (one per opponent); battlefield grew by %d", bfAfter-bfBefore)
	}
	// Mana pool spent.
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected 5 - 3 = 2 mana remaining; got %d", gs.Seats[0].ManaPool)
	}
	// Sliver moved from graveyard to exile.
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected graveyard empty; got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Exile) != 1 {
		t.Errorf("expected sliver exiled; exile=%d", len(gs.Seats[0].Exile))
	}
}

// ---------------------------------------------------------------------
// Yenna, Redtooth Regent — copy enchantment
// ---------------------------------------------------------------------

func TestYenna_CopyEnchantmentSpawnsToken(t *testing.T) {
	gs := newGame(t, 2)
	yenna := addPerm(gs, 0, "Yenna, Redtooth Regent", "creature")
	addPerm(gs, 0, "Rhystic Study", "enchantment")
	gs.Seats[0].ManaPool = 2
	gs.Active = 0
	gs.Phase = "precombat_main"

	bfBefore := len(gs.Seats[0].Battlefield)
	yennaCopyEnchantment(gs, yenna, 0, nil)

	if len(gs.Seats[0].Battlefield) <= bfBefore {
		t.Errorf("Yenna should have spawned an enchantment token; before=%d after=%d",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
}

func TestYenna_AuraTargetUntapsAndScrys(t *testing.T) {
	gs := newGame(t, 2)
	yenna := addPerm(gs, 0, "Yenna, Redtooth Regent", "creature")
	// Yenna must start untapped — the {T} cost gate rejects pre-tapped sources.
	addPerm(gs, 0, "Rancor", "enchantment", "aura")
	addLibrary(gs, 0, "A", "B", "C")
	gs.Seats[0].ManaPool = 2
	gs.Active = 0
	gs.Phase = "precombat_main"

	yennaCopyEnchantment(gs, yenna, 0, nil)

	if yenna.Tapped {
		t.Errorf("Yenna should have untapped after Aura copy")
	}
}

// ---------------------------------------------------------------------
// Amalia Benavides — explore + power-20 wipe
// ---------------------------------------------------------------------

func TestAmalia_ExploreLandToHand(t *testing.T) {
	gs := newGame(t, 2)
	amalia := addPerm(gs, 0, "Amalia Benavides Aguirre", "creature")
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Forest", Owner: 0, Types: []string{"land", "basic"}},
	}

	amaliaExploreOnLifeGain(gs, amalia, map[string]interface{}{
		"seat":   0,
		"amount": 2,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Amalia explore should have put Forest in hand on land top; hand=%d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("library should be empty after taking Forest; got %d", len(gs.Seats[0].Library))
	}
}

func TestAmalia_ExploreNonlandAddsCounter(t *testing.T) {
	gs := newGame(t, 2)
	amalia := addPerm(gs, 0, "Amalia Benavides Aguirre", "creature")
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}},
	}

	amaliaExploreOnLifeGain(gs, amalia, map[string]interface{}{
		"seat":   0,
		"amount": 1,
	})

	if amalia.Counters["+1/+1"] != 1 {
		t.Errorf("Amalia should get a +1/+1 counter on nonland top; got %d", amalia.Counters["+1/+1"])
	}
}

// ---------------------------------------------------------------------
// Saheeli, Radiant Creator — energy on artifact cast
// ---------------------------------------------------------------------

func TestSaheeli_ArtifactCastGrantsEnergy(t *testing.T) {
	gs := newGame(t, 2)
	saheeli := addPerm(gs, 0, "Saheeli, Radiant Creator", "creature")

	saheeliEnergyOnArtifactCast(gs, saheeli, map[string]interface{}{
		"caster_seat": 0,
		"is_artifact": true,
		"spell_name":  "Sol Ring",
	})

	if gameengine.GetEnergy(gs, 0) != 1 {
		t.Errorf("Saheeli should grant 1 energy on artifact cast; got %d", gameengine.GetEnergy(gs, 0))
	}
}

func TestSaheeli_NonArtifactNoEnergy(t *testing.T) {
	gs := newGame(t, 2)
	saheeli := addPerm(gs, 0, "Saheeli, Radiant Creator", "creature")

	saheeliEnergyOnArtifactCast(gs, saheeli, map[string]interface{}{
		"caster_seat": 0,
		"is_artifact": false,
		"spell_name":  "Lightning Bolt",
	})

	if gameengine.GetEnergy(gs, 0) != 0 {
		t.Errorf("Saheeli should NOT grant energy on non-artifact spell; got %d", gameengine.GetEnergy(gs, 0))
	}
}

// ---------------------------------------------------------------------
// Kardur, Doomscourge — attacker-dies drain
// ---------------------------------------------------------------------

func TestKardur_AttackerDeathDrainsAndGains(t *testing.T) {
	gs := newGame(t, 4)
	kardur := addPerm(gs, 0, "Kardur, Doomscourge", "creature")
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 30
	gs.Seats[2].Life = 30
	gs.Seats[3].Life = 30

	dying := addPerm(gs, 1, "Goblin Guide", "creature")
	dying.Flags["attacking"] = 1

	kardurAttackerDeathDrain(gs, kardur, map[string]interface{}{
		"perm": dying,
	})

	if gs.Seats[0].Life != 31 {
		t.Errorf("Kardur should gain 1 life; got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 29 {
		t.Errorf("opp seat 1 should lose 1 life; got %d", gs.Seats[1].Life)
	}
	if gs.Seats[2].Life != 29 {
		t.Errorf("opp seat 2 should lose 1 life; got %d", gs.Seats[2].Life)
	}
}

func TestKardur_NonAttackerDeathDoesNothing(t *testing.T) {
	gs := newGame(t, 2)
	kardur := addPerm(gs, 0, "Kardur, Doomscourge", "creature")
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 30

	dying := addPerm(gs, 1, "Llanowar Elves", "creature")
	// Not attacking.

	kardurAttackerDeathDrain(gs, kardur, map[string]interface{}{
		"perm": dying,
	})

	if gs.Seats[0].Life != 30 || gs.Seats[1].Life != 30 {
		t.Errorf("Kardur should not fire on non-attacker death")
	}
}

// ---------------------------------------------------------------------
// Felothar — sacrifice, draw=toughness, discard=power
// ---------------------------------------------------------------------

func TestFelothar_SacrificeDrawsAndDiscards(t *testing.T) {
	gs := newGame(t, 2)
	felothar := addPerm(gs, 0, "Felothar the Steadfast", "creature")
	gs.Seats[0].ManaPool = 3

	// Sac target: 2-power, 4-toughness creature.
	sac := addPerm(gs, 0, "Wall of Mulch", "creature")
	sac.Card.BasePower = 2
	sac.Card.BaseToughness = 4

	addLibrary(gs, 0, "A", "B", "C", "D", "E")
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Big", Owner: 0, Types: []string{"creature"}, BasePower: 8, BaseToughness: 8},
		{Name: "Small", Owner: 0, Types: []string{"creature"}, BasePower: 1, BaseToughness: 1},
		{Name: "Smaller", Owner: 0, Types: []string{"creature"}, BasePower: 1, BaseToughness: 1},
	}

	felotharSacDrawDiscard(gs, felothar, 0, nil)

	// Drew 4 from library; library now has 5-4=1, hand grew by 4.
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 3 mana spent; pool=%d", gs.Seats[0].ManaPool)
	}
	// Originally 3 in hand + drew 4 - discarded 2 = 5 in hand.
	if len(gs.Seats[0].Hand) != 5 {
		t.Errorf("expected hand size 5 (3 + 4 drew - 2 discarded); got %d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------
// Mondrak — token doubler flag + activated indestructible
// ---------------------------------------------------------------------

func TestMondrak_ETBSetsTokenDoublerFlag(t *testing.T) {
	gs := newGame(t, 2)
	mondrak := addPerm(gs, 0, "Mondrak, Glory Dominus", "creature")

	mondrakSetTokenDoubler(gs, mondrak)

	if gs.Seats[0].Flags["token_doubler_count"] != 1 {
		t.Errorf("Mondrak ETB should set token_doubler_count=1; got %d", gs.Seats[0].Flags["token_doubler_count"])
	}
}

func TestMondrak_ActivationSacrificesTwoAndAddsIndestructibleCounter(t *testing.T) {
	gs := newGame(t, 2)
	mondrak := addPerm(gs, 0, "Mondrak, Glory Dominus", "creature")
	gs.Seats[0].ManaPool = 3
	fodder1 := addPerm(gs, 0, "Llanowar Elves", "creature")
	fodder2 := addPerm(gs, 0, "Birds of Paradise", "creature")

	mondrakIndestructibleActivate(gs, mondrak, 0, nil)

	if mondrak.Counters["indestructible"] != 1 {
		t.Errorf("expected 1 indestructible counter on Mondrak (printed oracle); got %d", mondrak.Counters["indestructible"])
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 3 mana spent ({1}{W/P}{W/P}); pool=%d", gs.Seats[0].ManaPool)
	}
	// r51 leak regression: sacrificed creatures must NOT be in both
	// battlefield and exile (Loki r48/r50 game-59 CardIdentity surface).
	// Sacrifice routes victims to graveyard via SacrificePermanent.
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("sac cost should NOT route victims to exile; exile=%d (r48/r50 leak signature)", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Graveyard) != 2 {
		t.Errorf("expected 2 sacrificed creatures in graveyard; got %d", len(gs.Seats[0].Graveyard))
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && (p.Card == fodder1.Card || p.Card == fodder2.Card) {
			t.Errorf("sacrificed creature still on battlefield — CardIdentity leak regression")
		}
	}
}

// r51 regression: pin the exact game-59 / Avatar Enthusiasts leak shape.
// The pre-r51 handler exiled creatures with MoveCard(battlefield→exile),
// which left the Permanent on the battlefield AND added the *Card pointer
// to seat.Exile. The fix uses SacrificePermanent (which drops the
// Permanent properly).
func TestMondrak_ActivationDoesNotLeakAvatarEnthusiastsIntoExile(t *testing.T) {
	gs := newGame(t, 2)
	mondrak := addPerm(gs, 0, "Mondrak, Glory Dominus", "creature")
	avatar := addPerm(gs, 0, "Avatar Enthusiasts", "creature")
	addPerm(gs, 0, "Llanowar Elves", "creature")
	gs.Seats[0].ManaPool = 3

	avatarCard := avatar.Card
	mondrakIndestructibleActivate(gs, mondrak, 0, nil)

	// The Avatar Enthusiasts Card pointer must NOT appear in seat.Exile.
	for _, c := range gs.Seats[0].Exile {
		if c == avatarCard {
			t.Fatalf("REGRESSION: Avatar Enthusiasts *Card pointer in exile (r48/r50 game-59 leak)")
		}
	}
	// It also must NOT still be on the battlefield as a Permanent
	// (the leak's other half — Permanent retained while Card duped to exile).
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == avatarCard {
			t.Fatalf("REGRESSION: Avatar Enthusiasts Permanent still on battlefield after sac")
		}
	}
	if mondrak.Counters["indestructible"] != 1 {
		t.Errorf("expected 1 indestructible counter; got %d", mondrak.Counters["indestructible"])
	}
}

// ---------------------------------------------------------------------
// Solphim — noncombat dmg doubler flag
// ---------------------------------------------------------------------

func TestSolphim_ETBSetsDamageDoublerFlag(t *testing.T) {
	gs := newGame(t, 2)
	solphim := addPerm(gs, 0, "Solphim, Mayhem Dominus", "creature")

	solphimSetDamageDoublerFlag(gs, solphim)

	if gs.Seats[0].Flags["noncombat_damage_doubler_count"] != 1 {
		t.Errorf("Solphim ETB should set noncombat_damage_doubler_count=1; got %d",
			gs.Seats[0].Flags["noncombat_damage_doubler_count"])
	}
}

// ---------------------------------------------------------------------
// Drivnod — death-trigger doubler flag
// ---------------------------------------------------------------------

func TestDrivnod_ETBSetsDeathTriggerDoublerFlag(t *testing.T) {
	gs := newGame(t, 2)
	drivnod := addPerm(gs, 0, "Drivnod, Carnage Dominus", "creature")

	drivnodSetDeathDoublerFlag(gs, drivnod)

	if gs.Seats[0].Flags["death_trigger_doubler_count"] != 1 {
		t.Errorf("Drivnod ETB should set death_trigger_doubler_count=1; got %d",
			gs.Seats[0].Flags["death_trigger_doubler_count"])
	}
}

// ---------------------------------------------------------------------
// Zopandrel — combat damage to player draws + gains life
// ---------------------------------------------------------------------

func TestZopandrel_CombatDamageDrawsAndGains(t *testing.T) {
	gs := newGame(t, 2)
	zop := addPerm(gs, 0, "Zopandrel, Hunger Dominus", "creature")
	addLibrary(gs, 0, "A", "B", "C")
	gs.Seats[0].Life = 30

	zopandrelDrawAndGain(gs, zop, map[string]interface{}{
		"source_seat": 0,
		"target_seat": 1,
		"amount":      4,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Zopandrel should have drawn 1 card; hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != 34 {
		t.Errorf("Zopandrel should have gained 4 life; life=%d", gs.Seats[0].Life)
	}
}

// ---------------------------------------------------------------------
// Ixhel — exile top of poisoned opp libraries
// ---------------------------------------------------------------------

func TestIxhel_ExilesTopOfPoisonedOpponents(t *testing.T) {
	gs := newGame(t, 2)
	ixhel := addPerm(gs, 0, "Ixhel, Scion of Atraxa", "creature")
	gs.Active = 0
	// Opponent has 4 poison counters.
	if gs.Seats[1].Flags == nil {
		gs.Seats[1].Flags = map[string]int{}
	}
	gs.Seats[1].Flags["poison_counters"] = 4
	addLibrary(gs, 1, "Poison Pill")

	exBefore := len(gs.Seats[1].Exile)
	ixhelEndStepExile(gs, ixhel, map[string]interface{}{"active_seat": 0})

	if len(gs.Seats[1].Exile) <= exBefore {
		t.Errorf("Ixhel should have exiled top of poisoned opp library; exile before=%d after=%d",
			exBefore, len(gs.Seats[1].Exile))
	}
}

func TestIxhel_NonPoisonedOpponentSkipped(t *testing.T) {
	gs := newGame(t, 2)
	ixhel := addPerm(gs, 0, "Ixhel, Scion of Atraxa", "creature")
	gs.Active = 0
	if gs.Seats[1].Flags == nil {
		gs.Seats[1].Flags = map[string]int{}
	}
	gs.Seats[1].Flags["poison_counters"] = 2 // < 3
	addLibrary(gs, 1, "Card")

	ixhelEndStepExile(gs, ixhel, map[string]interface{}{"active_seat": 0})

	if len(gs.Seats[1].Exile) != 0 {
		t.Errorf("Ixhel should NOT exile from <3-poison opponent; exile=%d", len(gs.Seats[1].Exile))
	}
}

// ---------------------------------------------------------------------
// Registry smoke check — every era 2 commander has at least one handler
// ---------------------------------------------------------------------

func TestEra2_AllHandlersRegistered(t *testing.T) {
	cards := []string{
		"Sliver Gravemother",
		"Yenna, Redtooth Regent",
		"Amalia Benavides Aguirre",
		"Saheeli, Radiant Creator",
		"Kardur, Doomscourge",
		"Felothar the Steadfast",
		"Mondrak, Glory Dominus",
		"Solphim, Mayhem Dominus",
		"Drivnod, Carnage Dominus",
		"Zopandrel, Hunger Dominus",
		"Ixhel, Scion of Atraxa",
	}
	for _, name := range cards {
		hasAny := HasETB(name) || HasResolve(name) || HasActivated(name) || hasAnyEra2Trigger(name)
		if !hasAny {
			t.Errorf("%q should have at least one registered handler", name)
		}
	}
}

func hasAnyEra2Trigger(name string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	byEvent, ok := reg.onTrigger[normalizeName(name)]
	if !ok {
		return false
	}
	for _, hs := range byEvent {
		if len(hs) > 0 {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Cost-gate coverage: Erebos / Tasigur / Azami
// ---------------------------------------------------------------------

func TestErebosGodOfTheDead_PaysManaAndLifeForDraw(t *testing.T) {
	gs := newGame(t, 2)
	erebos := addPerm(gs, 0, "Erebos, God of the Dead", "creature")
	gs.Seats[0].ManaPool = 5
	gs.Seats[0].Life = 40
	addLibrary(gs, 0, "A", "B")

	erebosGodOfTheDeadActivate(gs, erebos, 0, nil)

	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected mana 5 - 2 = 3 after Erebos; got %d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Life != 38 {
		t.Errorf("expected life 40 - 2 = 38; got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestErebosGodOfTheDead_FailsWithoutMana(t *testing.T) {
	gs := newGame(t, 2)
	erebos := addPerm(gs, 0, "Erebos, God of the Dead", "creature")
	gs.Seats[0].ManaPool = 1 // can't pay {1}{B} (need 2)
	gs.Seats[0].Life = 40
	addLibrary(gs, 0, "A")

	erebosGodOfTheDeadActivate(gs, erebos, 0, nil)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("should NOT draw without mana; hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != 40 {
		t.Errorf("life should be unchanged when mana cost fails; got %d", gs.Seats[0].Life)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event")
	}
}

func TestErebosGodOfTheDead_FailsWithoutLife(t *testing.T) {
	gs := newGame(t, 2)
	erebos := addPerm(gs, 0, "Erebos, God of the Dead", "creature")
	gs.Seats[0].ManaPool = 5
	gs.Seats[0].Life = 2 // not strictly more than 2
	addLibrary(gs, 0, "A")

	erebosGodOfTheDeadActivate(gs, erebos, 0, nil)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("should NOT draw without enough life; hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("mana should be unchanged when life cost fails; got %d", gs.Seats[0].ManaPool)
	}
}

func TestTasigurTheGoldenFang_PaysManaAndMills(t *testing.T) {
	gs := newGame(t, 2)
	tasigur := addPerm(gs, 0, "Tasigur, the Golden Fang", "creature")
	gs.Seats[0].ManaPool = 6
	addLibrary(gs, 0, "A", "B", "C")

	tasigurTheGoldenFangActivateCustom(gs, tasigur, 0, nil)

	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected mana 6 - 4 = 2; got %d", gs.Seats[0].ManaPool)
	}
	// R60: gen mill-only stub replaced by full handler. Mill 2 → opponent
	// picks one milled card → returns it to seat 0's hand. Net zone
	// distribution: library 1 (untouched), graveyard 1 (kept), hand 1
	// (returned). Pre-r60 expectation was graveyard=2 (mill-only); the
	// new contract subtracts the returned card from the post-mill
	// graveyard count.
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected 1 left in library; got %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("expected 1 in graveyard after mill 2 + return 1; got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 in hand after opponent-picks-return; got %d", len(gs.Seats[0].Hand))
	}
}

func TestTasigurTheGoldenFang_FailsWithoutMana(t *testing.T) {
	gs := newGame(t, 2)
	tasigur := addPerm(gs, 0, "Tasigur, the Golden Fang", "creature")
	gs.Seats[0].ManaPool = 3 // can't pay {2}{G/U}{G/U} = 4
	addLibrary(gs, 0, "A", "B", "C")

	tasigurTheGoldenFangActivateCustom(gs, tasigur, 0, nil)

	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("should NOT mill without mana; graveyard=%d", len(gs.Seats[0].Graveyard))
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("mana should be unchanged on cost-fail; got %d", gs.Seats[0].ManaPool)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event")
	}
}

func TestAzamiLadyOfScrolls_TapsWizardForDraw(t *testing.T) {
	gs := newGame(t, 2)
	azami := addPerm(gs, 0, "Azami, Lady of Scrolls", "creature", "wizard", "legendary")
	wizard := addPerm(gs, 0, "Merfolk Looter", "creature", "wizard")
	addLibrary(gs, 0, "A")

	azamiLadyOfScrollsActivate(gs, azami, 0, nil)

	if !wizard.Tapped {
		t.Errorf("expected non-Azami Wizard to be tapped first")
	}
	if azami.Tapped {
		t.Errorf("Azami should remain untapped when another Wizard is available")
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestAzamiLadyOfScrolls_FallsBackToSelf(t *testing.T) {
	gs := newGame(t, 2)
	azami := addPerm(gs, 0, "Azami, Lady of Scrolls", "creature", "wizard", "legendary")
	addLibrary(gs, 0, "A")

	azamiLadyOfScrollsActivate(gs, azami, 0, nil)

	if !azami.Tapped {
		t.Errorf("Azami should tap herself when no other Wizard is available")
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestAzamiLadyOfScrolls_FailsWithoutAnyUntappedWizard(t *testing.T) {
	gs := newGame(t, 2)
	azami := addPerm(gs, 0, "Azami, Lady of Scrolls", "creature", "wizard", "legendary")
	azami.Tapped = true // can't tap her either
	tappedWiz := addPerm(gs, 0, "Merfolk Looter", "creature", "wizard")
	tappedWiz.Tapped = true
	// A non-Wizard shouldn't satisfy the cost.
	addPerm(gs, 0, "Grizzly Bears", "creature")
	addLibrary(gs, 0, "A")

	azamiLadyOfScrollsActivate(gs, azami, 0, nil)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("should NOT draw with no untapped Wizard; hand=%d", len(gs.Seats[0].Hand))
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event")
	}
}

func TestAzamiLadyOfScrolls_NonWizardCreatureDoesNotPay(t *testing.T) {
	gs := newGame(t, 2)
	azami := addPerm(gs, 0, "Azami, Lady of Scrolls", "creature", "wizard", "legendary")
	azami.Tapped = true
	// Untapped creature, but not a Wizard — must NOT pay the cost.
	addPerm(gs, 0, "Grizzly Bears", "creature")
	addLibrary(gs, 0, "A")

	azamiLadyOfScrollsActivate(gs, azami, 0, nil)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("non-Wizard creature shouldn't satisfy tap-a-Wizard cost; hand=%d", len(gs.Seats[0].Hand))
	}
}
