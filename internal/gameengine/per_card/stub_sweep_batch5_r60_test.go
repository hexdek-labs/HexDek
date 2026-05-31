package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestAlesha_AttackReanimatesAndNoStalePartial pins the attack-trigger
// reanimation AND the batch-5 cleanup: no per_card_partial fires on
// the trigger path (the WB/WB optional cost is auto-paid by AI policy,
// not an engine gap).
func TestAlesha_AttackReanimatesAndNoStalePartial(t *testing.T) {
	gs := newGame(t, 2)
	alesha := addPerm(gs, 0, "Alesha, Who Smiles at Death", "legendary", "creature", "human", "warrior")
	target := addCard(gs, 0, "Goblin Guide", "creature", "goblin")
	target.BasePower = 2
	target.BaseToughness = 2
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": alesha,
		"seat":          0,
	})

	// Target should be on Alesha's battlefield.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == target {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected target reanimated onto battlefield")
	}
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == alesha.Card.DisplayName() {
			t.Errorf("expected no per_card_partial after batch 5 cleanup; got %+v", ev)
		}
	}
}

// TestToph_DiscardRebuyDoesNotEmitStalePartial pins the cleanup:
// when Toph's ETB-discard-rebuy ability opts in, no
// "earthbend_on_cast_trigger_not_implemented" partial fires (the
// earthbend wiring shipped in R52 batch K).
func TestToph_DiscardRebuyDoesNotEmitStalePartial(t *testing.T) {
	gs := newGame(t, 2)
	toph := addPerm(gs, 0, "Toph, Hardheaded Teacher", "legendary", "creature", "human")
	bolt := addCard(gs, 0, "Lightning Bolt", "instant")
	cheap := addCard(gs, 0, "Cheap Card", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, cheap)

	gameengine.InvokeETBHook(gs, toph)

	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == toph.Card.DisplayName() {
			if reason, _ := ev.Details["reason"].(string); reason == "earthbend_on_cast_trigger_not_implemented_separate_helper_needed" {
				t.Errorf("expected no stale earthbend partial after batch 5 cleanup; got %+v", ev)
			}
		}
	}
}

// TestNecromancy_LTBSacFiresFromTrigger pins the LTB-sac wiring
// (R52 batchM) that the batch 5 doc cleanup verifies is real, not a
// gap: when Necromancy itself leaves the battlefield, the bonded
// creature is sacrificed by its current controller.
func TestNecromancy_LTBSacFiresFromTrigger(t *testing.T) {
	gs := newGame(t, 2)
	creature := addCard(gs, 0, "Test Beater", "creature")
	creature.BasePower = 4
	creature.BaseToughness = 4
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, creature)

	necro := addPerm(gs, 0, "Necromancy", "enchantment")
	gameengine.InvokeETBHook(gs, necro)

	// Verify the reanimation landed.
	var reanimated *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == creature {
			reanimated = p
			break
		}
	}
	if reanimated == nil {
		t.Fatalf("setup: expected creature reanimated onto seat 0 battlefield")
	}

	// Fire Necromancy's LTB.
	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            necro,
		"card":            necro.Card,
		"to_zone":         "graveyard",
		"controller_seat": 0,
	})

	// Reanimated creature should have been sacrificed.
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == creature {
			t.Errorf("bonded creature should be sacrificed after Necromancy LTB; still on bf")
		}
	}
}

// TestAltair_NoAssassinInGraveyardEmitsFailNotPartial pins the
// reclassification: zero Assassins in graveyard is a legal "up to
// one target" zero-target case, not a partial implementation gap.
func TestAltair_NoAssassinInGraveyardEmitsFailNotPartial(t *testing.T) {
	gs := newGame(t, 2)
	altair := addPerm(gs, 0, "Altaïr Ibn-La'Ahad", "legendary", "creature", "human", "assassin")
	gs.Seats[1].Life = 40

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": altair,
		"seat":          0,
	})

	foundFail := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_failed" && ev.Source == altair.Card.DisplayName() {
			if reason, _ := ev.Details["reason"].(string); reason == "no_assassin_in_graveyard_at_attack_time" {
				foundFail = true
				break
			}
		}
		if ev.Kind == "per_card_partial" && ev.Source == altair.Card.DisplayName() {
			if reason, _ := ev.Details["reason"].(string); reason == "no_assassin_in_graveyard_at_attack_time" {
				t.Errorf("expected emitFail (not emitPartial) after batch 5 reclassification; got %+v", ev)
			}
		}
	}
	if !foundFail {
		t.Errorf("expected per_card_fail for no_assassin_in_graveyard_at_attack_time; events=%+v", gs.EventLog)
	}
}

// TestLluwen_PestTokenAttackGainsOneLife pins the wired Pest Token
// "Whenever this token attacks, you gain 1 life" trigger.
func TestLluwen_PestTokenAttackGainsOneLife(t *testing.T) {
	gs := newGame(t, 2)
	startLife := gs.Seats[0].Life
	pest := addPerm(gs, 0, "Pest Token", "creature", "token", "pest")
	pest.Flags["pest_attack_lifegain"] = 1

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": pest,
		"seat":          0,
	})

	if gs.Seats[0].Life != startLife+1 {
		t.Errorf("expected life +1 after pest attack; before=%d after=%d", startLife, gs.Seats[0].Life)
	}
}

// TestLluwen_NonLluwenPestTokenDoesNotGainLife pins the Flags gate:
// a Pest Token without the pest_attack_lifegain flag (minted by some
// other card) does NOT trigger the lifegain.
func TestLluwen_NonLluwenPestTokenDoesNotGainLife(t *testing.T) {
	gs := newGame(t, 2)
	startLife := gs.Seats[0].Life
	pest := addPerm(gs, 0, "Pest Token", "creature", "token", "pest")
	// No pest_attack_lifegain flag stamped.

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": pest,
		"seat":          0,
	})

	if gs.Seats[0].Life != startLife {
		t.Errorf("non-Lluwen pest must not gain life; before=%d after=%d", startLife, gs.Seats[0].Life)
	}
}
