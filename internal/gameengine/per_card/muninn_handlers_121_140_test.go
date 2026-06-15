package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Smoke tests for the dev/muninn-handlers-121-140 wave. Each test
// exercises one handler's primary clause and verifies the canonical
// per_card_handler event fires with the expected effect.

func hasSlugTrue(gs *gameengine.GameState, slug string) bool {
	for _, ev := range gs.EventLog {
		if ev.Kind != "per_card_handler" {
			continue
		}
		if d, _ := ev.Details["slug"].(string); d != slug {
			continue
		}
		if triggered, ok := ev.Details["triggered"].(bool); ok {
			return triggered
		}
		return true
	}
	return false
}

func TestEmeritusOfWoe_ETBStampsPrepared(t *testing.T) {
	gs := newGame(t, 2)
	em := addPerm(gs, 0, "Emeritus of Woe", "creature")
	gameengine.InvokeETBHook(gs, em)
	if em.Flags["prepared"] != 1 {
		t.Errorf("expected prepared=1 after ETB, got %d", em.Flags["prepared"])
	}
}

func TestEmeritusOfWoe_EndStepStampsWhenTwoCreaturesDied(t *testing.T) {
	gs := newGame(t, 2)
	em := addPerm(gs, 0, "Emeritus of Woe", "creature")
	em.Flags["prepared"] = 0
	gs.Seats[0].Turn.CreaturesDied = 2

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})
	if em.Flags["prepared"] != 1 {
		t.Errorf("expected prepared=1 after 2 deaths, got %d", em.Flags["prepared"])
	}
}

func TestEmeritusOfWoe_NoStampWithFewDeaths(t *testing.T) {
	gs := newGame(t, 2)
	em := addPerm(gs, 0, "Emeritus of Woe", "creature")
	em.Flags["prepared"] = 0
	gs.Seats[0].Turn.CreaturesDied = 1

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})
	if em.Flags["prepared"] != 0 {
		t.Errorf("expected prepared=0 with only 1 death, got %d", em.Flags["prepared"])
	}
}

func TestArguelsBloodFast_ActivateDrawsForLife(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	addLibrary(gs, 0, "TopCard")
	bf := addPerm(gs, 0, "Arguel's Blood Fast", "enchantment", "legendary")

	gameengine.InvokeActivatedHook(gs, bf, 0, nil)

	if gs.Seats[0].Life != 18 {
		t.Errorf("expected life 18 after paying 2, got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected to draw 1, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestArguelsBloodFast_UpkeepTransformsAtLowLife(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 5
	addPerm(gs, 0, "Arguel's Blood Fast", "enchantment", "legendary")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})

	if !hasSlugTrue(gs, "arguels_blood_fast_upkeep") {
		t.Errorf("expected arguels_blood_fast_upkeep triggered=true at life<=5")
	}
}

func TestAradesh_AttackerWithEnlistGetsDoubleStrikeAndDraws(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "TopCard")
	addPerm(gs, 0, "Aradesh, the Founder", "creature", "legendary")

	att := addPerm(gs, 0, "Big Soldier", "creature")
	att.Card.BasePower = 3
	att.Flags["enlisted_this_combat"] = 1
	att.AddCounter("+1/+1", 2) // canonical power source → Power()=5 ≥ 4, draws
	// (was Flags["temp_power"]=2, a source Permanent.Power() never read — the
	// payoff now uses attacker.Power(), so model the boost the real way.)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": att,
		"defender_seat": 1,
	})

	if att.Flags["kw:double_strike"] != 1 {
		t.Errorf("expected double_strike on attacker, got %d", att.Flags["kw:double_strike"])
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected draw with power>=4, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestAradesh_NoTriggerWithoutEnlist(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Aradesh, the Founder", "creature", "legendary")
	att := addPerm(gs, 0, "Bear", "creature")
	att.Card.BasePower = 5

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": att,
		"defender_seat": 1,
	})

	if att.Flags["kw:double_strike"] != 0 {
		t.Errorf("expected no double_strike without enlist, got %d", att.Flags["kw:double_strike"])
	}
}

func TestEccentricPestfinder_EndStepStampsWhenLifeGained(t *testing.T) {
	gs := newGame(t, 2)
	pf := addPerm(gs, 0, "Eccentric Pestfinder", "creature")
	gs.Seats[0].Turn.LifeGained = 3

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})

	if pf.Flags["prepared"] != 1 {
		t.Errorf("expected prepared=1 after life gain, got %d", pf.Flags["prepared"])
	}
}

func TestTurnStones_ResolveMintsPestPerOpponent(t *testing.T) {
	gs := newGame(t, 4)
	card := addCard(gs, 0, "Turn Stones", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	pests := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "pest") {
			pests++
		}
	}
	if pests != 3 {
		t.Errorf("expected 3 pest tokens (one per opponent), got %d", pests)
	}
}

func TestSproutbackTrudge_ETBEmitsHandlerEvent(t *testing.T) {
	gs := newGame(t, 2)
	tr := addPerm(gs, 0, "Sproutback Trudge", "creature")
	gameengine.InvokeETBHook(gs, tr)

	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_handler" {
			if d, _ := ev.Details["slug"].(string); d == "sproutback_trudge_etb" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected sproutback_trudge_etb event")
	}
}

func TestSenu_ActivateExilesAndGainsLife(t *testing.T) {
	gs := newGame(t, 2)
	senu := addPerm(gs, 0, "Senu, Keen-Eyed Protector", "creature", "legendary")
	startLife := gs.Seats[0].Life

	gameengine.InvokeActivatedHook(gs, senu, 0, nil)

	if gs.Seats[0].Life != startLife+2 {
		t.Errorf("expected +2 life, got %d (was %d)", gs.Seats[0].Life, startLife)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == senu {
			t.Errorf("Senu should have left the battlefield")
		}
	}
	foundExile := false
	for _, c := range gs.Seats[0].Exile {
		if c.DisplayName() == "Senu, Keen-Eyed Protector" {
			foundExile = true
		}
	}
	if !foundExile {
		t.Errorf("expected Senu in exile")
	}
}

func TestSai_ArtifactCastCreatesThopter(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sai, Master Thopterist", "creature", "legendary")

	artifact := &gameengine.Card{
		Name:  "Sol Ring",
		Owner: 0,
		Types: []string{"artifact", "cmc:1"},
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        artifact,
		"spell_name":  "Sol Ring",
		"is_creature": false,
	})

	thopters := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "thopter") {
			thopters++
		}
	}
	if thopters != 1 {
		t.Errorf("expected 1 thopter from Sai, got %d", thopters)
	}
}

func TestSai_NoThopterOnNoncreatureNonArtifact(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sai, Master Thopterist", "creature", "legendary")

	sorcery := &gameengine.Card{
		Name:  "Wrath of God",
		Owner: 0,
		Types: []string{"sorcery", "cmc:4"},
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        sorcery,
		"spell_name":  "Wrath of God",
		"is_creature": false,
	})

	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "thopter") {
			t.Errorf("Sai should NOT make a thopter on non-artifact cast")
		}
	}
}

func TestPhyrexianDreadnought_SacsItselfWithoutFodder(t *testing.T) {
	gs := newGame(t, 2)
	dn := addPerm(gs, 0, "Phyrexian Dreadnought", "creature", "artifact")
	dn.Card.BasePower = 12
	dn.Card.BaseToughness = 12

	gameengine.InvokeETBHook(gs, dn)

	for _, p := range gs.Seats[0].Battlefield {
		if p == dn {
			t.Errorf("Dreadnought should have been sacrificed with no fodder")
		}
	}
}

func TestPhyrexianDreadnought_SurvivesWithSufficientFodder(t *testing.T) {
	gs := newGame(t, 2)
	fodder := addPerm(gs, 0, "Big Demon", "creature")
	fodder.Card.BasePower = 12

	dn := addPerm(gs, 0, "Phyrexian Dreadnought", "creature", "artifact")
	dn.Card.BasePower = 12
	dn.Card.BaseToughness = 12

	gameengine.InvokeETBHook(gs, dn)

	stillThere := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == dn {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("Dreadnought should survive when total power 12+ is available")
	}
}

func TestNoxiousGearhulk_DestroysAndGainsLife(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Big Beast", "creature")
	target.Card.BasePower = 4
	target.Card.BaseToughness = 5
	startLife := gs.Seats[0].Life

	hulk := addPerm(gs, 0, "Noxious Gearhulk", "creature", "artifact")
	gameengine.InvokeETBHook(gs, hulk)

	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			t.Errorf("target should be destroyed by Gearhulk")
		}
	}
	if gs.Seats[0].Life != startLife+5 {
		t.Errorf("expected +5 life from Gearhulk, got %d (was %d)", gs.Seats[0].Life, startLife)
	}
}

func TestMasterOfDeath_ETBSurveils2(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C")
	mod := addPerm(gs, 0, "Master of Death", "creature")
	gameengine.InvokeETBHook(gs, mod)

	if len(gs.Seats[0].Graveyard) != 2 {
		t.Errorf("expected 2 cards in graveyard after surveil 2, got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected 1 card left in library, got %d", len(gs.Seats[0].Library))
	}
}
