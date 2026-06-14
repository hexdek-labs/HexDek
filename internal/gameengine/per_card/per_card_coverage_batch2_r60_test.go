package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Per-card coverage sweep — batch 2 (R60)
//
// Focused regression tests for 10 previously-untested handlers, pinning the
// canonical happy path + at least one gating/scope branch per card. All
// tests use the package's standard fixture helpers (newGame, addPerm,
// addLibrary, hasEvent, addCardToGraveyard).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Aristocrats — Blood Artist / Zulaport Cutthroat / Bastion / Cruel Celebrant
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_BloodArtist_DrainsOneOpponentOnAnyDeath(t *testing.T) {
	gs := newGame(t, 3)
	ba := addPerm(gs, 0, "Blood Artist", "creature")
	// Non-zero P/T so SBA doesn't kill the bearer and recursively re-fire.
	ba.Card.BasePower = 1
	ba.Card.BaseToughness = 1
	dying := addPerm(gs, 2, "Goblin Guide", "creature") // opponent's creature dying
	dying.Card.BasePower = 2
	dying.Card.BaseToughness = 2
	startLife0 := gs.Seats[0].Life
	startLife1 := gs.Seats[1].Life
	startLife2 := gs.Seats[2].Life

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"controller_seat": 2,
	})

	if gs.Seats[0].Life != startLife0+1 {
		t.Errorf("Blood Artist controller should gain 1 life, %d → %d", startLife0, gs.Seats[0].Life)
	}
	// One (and only one) opponent should have lost 1 life — first alive opponent.
	dropped := 0
	if gs.Seats[1].Life == startLife1-1 {
		dropped++
	}
	if gs.Seats[2].Life == startLife2-1 {
		dropped++
	}
	if dropped != 1 {
		t.Errorf("expected exactly 1 opponent drained, got %d (seat1=%d seat2=%d)",
			dropped, gs.Seats[1].Life, gs.Seats[2].Life)
	}
}

func TestPerCardCoverageBatch2_ZulaportCutthroat_DrainsAllOnOwnCreatureDeath(t *testing.T) {
	gs := newGame(t, 3)
	zc := addPerm(gs, 0, "Zulaport Cutthroat", "creature")
	zc.Card.BasePower = 1
	zc.Card.BaseToughness = 1
	dying := addPerm(gs, 0, "Goblin Guide", "creature") // own creature dying
	dying.Card.BasePower = 2
	dying.Card.BaseToughness = 2
	startLife0 := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"controller_seat": 0,
	})

	if gs.Seats[0].Life != startLife0+1 {
		t.Errorf("Zulaport controller should gain 1, got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 19 || gs.Seats[2].Life != 19 {
		t.Errorf("each opponent should lose 1, got seat1=%d seat2=%d",
			gs.Seats[1].Life, gs.Seats[2].Life)
	}
}

func TestPerCardCoverageBatch2_ZulaportCutthroat_DoesNotFireOnOpponentCreatureDeath(t *testing.T) {
	gs := newGame(t, 2)
	zc := addPerm(gs, 0, "Zulaport Cutthroat", "creature")
	zc.Card.BasePower = 1
	zc.Card.BaseToughness = 1
	dying := addPerm(gs, 1, "Goblin Guide", "creature") // opp's creature
	dying.Card.BasePower = 2
	dying.Card.BaseToughness = 2
	startLife1 := gs.Seats[1].Life

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"controller_seat": 1,
	})

	if gs.Seats[1].Life != startLife1 {
		t.Errorf("Zulaport must require OWN creature dying, opp life dropped %d → %d",
			startLife1, gs.Seats[1].Life)
	}
}

func TestPerCardCoverageBatch2_CruelCelebrant_DrainsOnOwnCreatureDeath(t *testing.T) {
	gs := newGame(t, 3)
	cc := addPerm(gs, 0, "Cruel Celebrant", "creature")
	cc.Card.BasePower = 1
	cc.Card.BaseToughness = 1
	dying := addPerm(gs, 0, "Knight Token", "creature", "token")
	dying.Card.BasePower = 2
	dying.Card.BaseToughness = 2
	startLife0 := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"controller_seat": 0,
	})

	if gs.Seats[0].Life != startLife0+1 {
		t.Errorf("Cruel Celebrant should gain 1 on own creature death, got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 19 || gs.Seats[2].Life != 19 {
		t.Errorf("each opp should lose 1, got %d / %d", gs.Seats[1].Life, gs.Seats[2].Life)
	}
}

// ---------------------------------------------------------------------------
// Animar, Soul of Elements
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_Animar_ETBStampsProtection(t *testing.T) {
	gs := newGame(t, 2)
	animar := addPerm(gs, 0, "Animar, Soul of Elements", "creature", "legendary")

	gameengine.InvokeETBHook(gs, animar)

	if animar.Flags["prot:W"] != 1 || animar.Flags["prot:B"] != 1 {
		t.Errorf("expected prot:W and prot:B flags, got %v", animar.Flags)
	}
}

func TestPerCardCoverageBatch2_Animar_AddsCounterOnOwnCreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	animar := addPerm(gs, 0, "Animar, Soul of Elements", "creature", "legendary")
	spell := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}

	gameengine.FireCardTrigger(gs, "creature_spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Grizzly Bears",
		"is_creature": true,
		"card":        spell,
	})

	if animar.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 counter on Animar, got %v", animar.Counters)
	}
}

func TestPerCardCoverageBatch2_Animar_NoCounterOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	animar := addPerm(gs, 0, "Animar, Soul of Elements", "creature", "legendary")
	spell := &gameengine.Card{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}}

	gameengine.FireCardTrigger(gs, "creature_spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Grizzly Bears",
		"is_creature": true,
		"card":        spell,
	})

	if animar.Counters["+1/+1"] != 0 {
		t.Errorf("opponent cast should NOT pump Animar, got %v", animar.Counters)
	}
}

// ---------------------------------------------------------------------------
// Anje Falkenrath
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_AnjeFalkenrath_LootDiscardsAndDraws(t *testing.T) {
	gs := newGame(t, 2)
	anje := addPerm(gs, 0, "Anje Falkenrath", "creature", "legendary")
	// Seat 0 hand: a fodder card to discard.
	fodder := &gameengine.Card{Name: "Mountain", Owner: 0, Types: []string{"land"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, fodder)
	addLibrary(gs, 0, "Draw1")

	gameengine.InvokeActivatedHook(gs, anje, 0, nil)

	if !anje.Tapped {
		t.Errorf("Anje should be tapped after loot activation")
	}
	// Fodder discarded (now in graveyard).
	foundInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == fodder {
			foundInGY = true
		}
	}
	if !foundInGY {
		t.Errorf("expected fodder discarded to graveyard")
	}
	// Drew one (Draw1 should be in hand).
	drewOne := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Draw1" {
			drewOne = true
		}
	}
	if !drewOne {
		t.Errorf("expected Draw1 in hand after loot")
	}
}

func TestPerCardCoverageBatch2_AnjeFalkenrath_FailsWhenAlreadyTapped(t *testing.T) {
	gs := newGame(t, 2)
	anje := addPerm(gs, 0, "Anje Falkenrath", "creature", "legendary")
	anje.Tapped = true
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{Name: "X", Owner: 0})
	addLibrary(gs, 0, "Y")

	gameengine.InvokeActivatedHook(gs, anje, 0, nil)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when Anje is already tapped")
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("no discard should occur on tapped Anje")
	}
}

func TestPerCardCoverageBatch2_AnjeFalkenrath_DoesNotUntapOnNonMadnessDiscard(t *testing.T) {
	// OracleTextLower returns "" for AST-less cards (see hat.go:667), so a
	// plain-Card discard will never match "madness" and Anje stays tapped.
	// This pins the negative-path gate (no false-positive untap on plain
	// discards) without requiring an AST fixture.
	gs := newGame(t, 2)
	anje := addPerm(gs, 0, "Anje Falkenrath", "creature", "legendary")
	anje.Tapped = true
	plain := &gameengine.Card{Name: "Mountain", Owner: 0, Types: []string{"land"}}

	gameengine.FireCardTrigger(gs, "card_discarded", map[string]interface{}{
		"discarder_seat": 0,
		"card":           plain,
	})

	if !anje.Tapped {
		t.Errorf("Anje should stay tapped on non-madness discard")
	}
}

// ---------------------------------------------------------------------------
// Arcum Dagsson
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_ArcumDagsson_SacsArtifactCreatureTutorsArtifact(t *testing.T) {
	gs := newGame(t, 2)
	arcum := addPerm(gs, 0, "Arcum Dagsson", "creature", "legendary", "artificer")
	thopter := addPerm(gs, 0, "Ornithopter", "creature", "artifact")
	thopter.Card.BasePower = 0
	thopter.Card.BaseToughness = 2
	// Library has a noncreature artifact to tutor up.
	addLibrary(gs, 0, "Sol Ring")
	gs.Seats[0].Library[0].Types = []string{"artifact"}

	gameengine.InvokeActivatedHook(gs, arcum, 0, nil)

	if !arcum.Tapped {
		t.Errorf("Arcum should be tapped on activation")
	}
	// Ornithopter should be gone from the battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == thopter {
			t.Errorf("Ornithopter should have been sacrificed")
		}
	}
	// Sol Ring should be on the battlefield.
	foundSol := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Sol Ring" {
			foundSol = true
		}
	}
	if !foundSol {
		t.Errorf("expected Sol Ring tutored to battlefield post-Arcum")
	}
}

func TestPerCardCoverageBatch2_ArcumDagsson_FailsWithNoArtifactCreatureTarget(t *testing.T) {
	gs := newGame(t, 2)
	arcum := addPerm(gs, 0, "Arcum Dagsson", "creature", "legendary", "artificer")
	// No artifact creature to sac (only a non-artifact creature available).
	addPerm(gs, 0, "Grizzly Bears", "creature")
	addLibrary(gs, 0, "Sol Ring")

	gameengine.InvokeActivatedHook(gs, arcum, 0, nil)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no artifact-creature target")
	}
}

// ---------------------------------------------------------------------------
// Anowon, the Ruin Thief
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_Anowon_MillsOnRogueCombatDamage(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Anowon, the Ruin Thief", "creature", "legendary", "vampire", "rogue")
	// A Rogue creature on the battlefield dealing the damage.
	addPerm(gs, 0, "Goblin Rogue", "creature", "goblin", "rogue")
	// Defender library: 3 cards including one creature so the draw rider fires.
	addLibrary(gs, 1, "Plains", "Goblin Guide", "Island")
	gs.Seats[1].Library[1].Types = []string{"creature"}
	addLibrary(gs, 0, "DrawTarget") // for the rider draw

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Goblin Rogue",
		"defender_seat": 1,
		"amount":        3,
	})

	if len(gs.Seats[1].Graveyard) != 3 {
		t.Errorf("expected 3 milled cards, got %d", len(gs.Seats[1].Graveyard))
	}
	// Draw rider should have fired since one milled card was a creature.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Anowon controller to draw 1, got hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestPerCardCoverageBatch2_Anowon_DoesNotMillOnNonRogueDamage(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Anowon, the Ruin Thief", "creature", "legendary", "vampire", "rogue")
	// Non-Rogue source on battlefield.
	addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	addLibrary(gs, 1, "A", "B", "C")

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Grizzly Bears",
		"defender_seat": 1,
		"amount":        3,
	})

	if len(gs.Seats[1].Graveyard) != 0 {
		t.Errorf("non-Rogue source should not mill, got %d", len(gs.Seats[1].Graveyard))
	}
}

// ---------------------------------------------------------------------------
// Archangel of Tithes — ETB stamps deterrent flags, LTB clears them
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_ArchangelOfTithes_ETBStampsTaxFlags(t *testing.T) {
	gs := newGame(t, 2)
	angel := addPerm(gs, 0, "Archangel of Tithes", "creature", "angel")

	gameengine.InvokeETBHook(gs, angel)

	if angel.Flags["tax_attack"] != 1 || angel.Flags["tax_block_when_attacking"] != 1 {
		t.Errorf("expected tax flags stamped on ETB, got %v", angel.Flags)
	}
}

func TestPerCardCoverageBatch2_ArchangelOfTithes_LTBClearsTaxFlags(t *testing.T) {
	gs := newGame(t, 2)
	angel := addPerm(gs, 0, "Archangel of Tithes", "creature", "angel")
	gameengine.InvokeETBHook(gs, angel)

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm": angel,
	})

	if angel.Flags["tax_attack"] != 0 || angel.Flags["tax_block_when_attacking"] != 0 {
		t.Errorf("LTB should clear tax flags, got %v", angel.Flags)
	}
}

// ---------------------------------------------------------------------------
// Akroma's Will — mass anthem + keyword grant
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_AkromasWill_GrantsKeywordsAndProtectionToOwnCreatures(t *testing.T) {
	gs := newGame(t, 2)
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	rock := addPerm(gs, 0, "Sol Ring", "artifact") // non-creature, should NOT get grants
	oppBear := addPerm(gs, 1, "Goblin Guide", "creature")

	card := &gameengine.Card{Name: "Akroma's Will", Owner: 0, Types: []string{"sorcery"}}
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	expected := []string{"flying", "vigilance", "double_strike", "lifelink", "indestructible"}
	for _, kw := range expected {
		found := false
		for _, g := range bear.GrantedAbilities {
			if g == kw {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected Grizzly Bears to gain %s, got %v", kw, bear.GrantedAbilities)
		}
	}
	if bear.Flags["protection_from_black"] != 1 || bear.Flags["protection_from_red"] != 1 {
		t.Errorf("expected protection flags on bear, got %v", bear.Flags)
	}
	if len(rock.GrantedAbilities) != 0 {
		t.Errorf("non-creature should not get grants, got %v", rock.GrantedAbilities)
	}
	if len(oppBear.GrantedAbilities) != 0 {
		t.Errorf("opponent creature should not get grants, got %v", oppBear.GrantedAbilities)
	}
}

// ---------------------------------------------------------------------------
// Adeline, Resplendent Cathar — per-opponent Human token on attack
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_Adeline_MintsTokenPerOpponentOnAttack(t *testing.T) {
	gs := newGame(t, 4)
	adeline := addPerm(gs, 0, "Adeline, Resplendent Cathar", "creature", "legendary", "human", "knight")
	adeline.Card.BasePower = 3
	adeline.Card.BaseToughness = 3
	beforeOwn := len(gs.Seats[0].Battlefield)

	// r63: the engine dispatches the attack event with attacker_seat (NOT
	// active_seat — never populated for attacks). Adeline now keys on the
	// canonical attacker_seat. Seat-0 creature attacking → Adeline's combat.
	gameengine.FireCardTrigger(gs, "declare_attackers", map[string]interface{}{
		"attacker_seat": 0,
	})

	// 3 opponents → 3 tokens minted to seat 0.
	if len(gs.Seats[0].Battlefield) != beforeOwn+3 {
		t.Errorf("expected +3 tokens for 3 opponents, battlefield %d → %d",
			beforeOwn, len(gs.Seats[0].Battlefield))
	}
	tokens := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Human Token" {
			tokens++
		}
	}
	if tokens != 3 {
		t.Errorf("expected 3 Human Tokens, got %d", tokens)
	}
}

func TestPerCardCoverageBatch2_Adeline_DoesNotMintOnOpponentAttack(t *testing.T) {
	gs := newGame(t, 2)
	adel := addPerm(gs, 0, "Adeline, Resplendent Cathar", "creature", "legendary", "human", "knight")
	adel.Card.BasePower = 3
	adel.Card.BaseToughness = 3
	beforeOwn := len(gs.Seats[0].Battlefield)

	// r63: an OPPONENT's creature attacking → attacker_seat 1, not Adeline's
	// controller (0). Adeline must not mint. (The old test used active_seat,
	// a key the attack event never populates, so it tested nothing real.)
	gameengine.FireCardTrigger(gs, "declare_attackers", map[string]interface{}{
		"attacker_seat": 1,
	})

	if len(gs.Seats[0].Battlefield) != beforeOwn {
		t.Errorf("Adeline should not mint when not her controller's combat, %d → %d",
			beforeOwn, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Arahbo, Roar of the World
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_Arahbo_CombatBeginPumpsBestCat(t *testing.T) {
	gs := newGame(t, 2)
	arahbo := addPerm(gs, 0, "Arahbo, Roar of the World", "creature", "legendary", "cat")
	arahbo.Card.BasePower = 5
	arahbo.Card.BaseToughness = 5

	// Two cats: a runt and a beefy one. The beefier one should get +3/+3.
	runt := addPerm(gs, 0, "Felidar Cub", "creature", "cat")
	runt.Card.BasePower = 1
	runt.Card.BaseToughness = 2
	beefy := addPerm(gs, 0, "Regal Caracal", "creature", "cat")
	beefy.Card.BasePower = 3
	beefy.Card.BaseToughness = 3

	gameengine.FireCardTrigger(gs, "combat_begin", map[string]interface{}{
		"active_seat": 0,
	})

	// Beefy should have a +3/+3 modification.
	pumped := 0
	for _, m := range beefy.Modifications {
		if m.Power == 3 && m.Toughness == 3 {
			pumped++
		}
	}
	if pumped != 1 {
		t.Errorf("expected beefy cat to gain +3/+3, mods=%v", beefy.Modifications)
	}
	for _, m := range runt.Modifications {
		if m.Power == 3 {
			t.Errorf("runt should not be pumped (Arahbo picks highest-power)")
		}
	}
}

func TestPerCardCoverageBatch2_Arahbo_CatAttackTrampleAndPump(t *testing.T) {
	gs := newGame(t, 2)
	arahbo := addPerm(gs, 0, "Arahbo, Roar of the World", "creature", "legendary", "cat")
	cat := addPerm(gs, 0, "Leonin Warleader", "creature", "cat", "soldier")
	cat.Card.BasePower = 4
	cat.Card.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": cat,
		"attacker_seat": 0,
		"attacker_card": cat.Card,
	})

	// Cat should have +X/+X (X=4 = its power) and gain trample flag.
	foundBoost := false
	for _, m := range cat.Modifications {
		if m.Power == 4 && m.Toughness == 4 {
			foundBoost = true
		}
	}
	if !foundBoost {
		t.Errorf("expected +X/+X boost (X=power=4), mods=%v", cat.Modifications)
	}
	if cat.Flags["kw:trample"] != 1 {
		t.Errorf("expected trample flag, got %v", cat.Flags)
	}
	// Arahbo himself should NOT trigger his own attack.
	_ = arahbo
}

// ---------------------------------------------------------------------------
// Adriana, Captain of the Guard — melee anthem
// ---------------------------------------------------------------------------

func TestPerCardCoverageBatch2_Adriana_MeleeAnthemPumpsByOpponentsAttacked(t *testing.T) {
	gs := newGame(t, 4)
	adriana := addPerm(gs, 0, "Adriana, Captain of the Guard", "creature", "legendary", "human", "knight")
	adriana.Card.BasePower = 2
	adriana.Card.BaseToughness = 4

	// Two of our creatures attacking two different opponents.
	atk1 := addPerm(gs, 0, "Knight Token A", "creature", "knight")
	atk1.Card.BasePower = 2
	atk1.Card.BaseToughness = 2
	atk1.Flags = map[string]int{"attacking": 1}
	gameengine.SetAttackerDefender(atk1, 1)

	atk2 := addPerm(gs, 0, "Knight Token B", "creature", "knight")
	atk2.Card.BasePower = 2
	atk2.Card.BaseToughness = 2
	atk2.Flags = map[string]int{"attacking": 1}
	gameengine.SetAttackerDefender(atk2, 2)

	gameengine.FireCardTrigger(gs, "declare_attackers", map[string]interface{}{
		"active_seat": 0,
	})

	if atk1.Flags["temp_power"] != 2 || atk1.Flags["temp_toughness"] != 2 {
		t.Errorf("expected +2/+2 (two opponents attacked) on atk1, got P=%d T=%d",
			atk1.Flags["temp_power"], atk1.Flags["temp_toughness"])
	}
	if atk2.Flags["temp_power"] != 2 || atk2.Flags["temp_toughness"] != 2 {
		t.Errorf("expected +2/+2 on atk2, got P=%d T=%d",
			atk2.Flags["temp_power"], atk2.Flags["temp_toughness"])
	}
}

func TestPerCardCoverageBatch2_Adriana_DoesNotPumpOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Adriana, Captain of the Guard", "creature", "legendary")
	atk := addPerm(gs, 0, "Knight Token", "creature", "knight")
	atk.Card.BasePower = 2
	atk.Card.BaseToughness = 2
	atk.Flags = map[string]int{"attacking": 1}
	gameengine.SetAttackerDefender(atk, 1)

	gameengine.FireCardTrigger(gs, "declare_attackers", map[string]interface{}{
		"active_seat": 1, // not Adriana's controller
	})

	if atk.Flags["temp_power"] != 0 {
		t.Errorf("Adriana should not pump on opponent's combat, got %d", atk.Flags["temp_power"])
	}
}
