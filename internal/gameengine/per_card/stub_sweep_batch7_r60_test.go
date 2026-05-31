package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestValakutExploration_LandfallExilesTopAndRegistersGrant pins the
// batch 7 wiring: landfall exiles top of library AND registers a
// play-from-exile grant tied to Valakut's timestamp.
func TestValakutExploration_LandfallExilesTopAndRegistersGrant(t *testing.T) {
	gs := newGame(t, 2)
	valakut := addPerm(gs, 0, "Valakut Exploration", "enchantment")
	land := addPerm(gs, 0, "Mountain", "land", "basic", "mountain")
	exileTarget := addCard(gs, 0, "Lightning Bolt", "instant")
	gs.Seats[0].Library = append([]*gameengine.Card{exileTarget}, gs.Seats[0].Library...)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            land,
		"controller_seat": 0,
	})

	// Verify exiled.
	foundExiled := false
	for _, c := range gs.Seats[0].Exile {
		if c == exileTarget {
			foundExiled = true
			break
		}
	}
	if !foundExiled {
		t.Errorf("expected card exiled by Valakut landfall; exile=%+v", gs.Seats[0].Exile)
	}
	// Verify grant registered with correct shape.
	grant, ok := gs.ZoneCastGrants[exileTarget]
	if !ok {
		t.Fatalf("expected ZoneCastGrant registered for exiled card")
	}
	if grant.ManaCost != -1 {
		t.Errorf("expected ManaCost=-1 (normal cost); got %d", grant.ManaCost)
	}
	if grant.Duration != "while_source_on_bf" {
		t.Errorf("expected Duration=while_source_on_bf; got %q", grant.Duration)
	}
	if grant.SourceTimestamp != valakut.Timestamp {
		t.Errorf("expected SourceTimestamp=Valakut.Timestamp; got %d vs %d", grant.SourceTimestamp, valakut.Timestamp)
	}
	if grant.RequireController != 0 {
		t.Errorf("expected RequireController=0; got %d", grant.RequireController)
	}
}

// TestValakutExploration_NoETBPartialAfterCleanup pins the cleanup:
// ETB no longer emits the per-ETB "play_from_exile" partial.
func TestValakutExploration_NoETBPartialAfterCleanup(t *testing.T) {
	gs := newGame(t, 2)
	valakut := addPerm(gs, 0, "Valakut Exploration", "enchantment")

	gameengine.InvokeETBHook(gs, valakut)

	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == valakut.Card.DisplayName() {
			if slug, _ := ev.Details["slug"].(string); slug == "valakut_exploration_play_from_exile" {
				t.Errorf("expected no per_card_partial after batch 7 cleanup; got %+v", ev)
			}
		}
	}
}

// TestEtali_NoStaleDeferredAICastPartial pins the cleanup: the attack
// trigger no longer emits the "auto-cast deferred to AI/Hat" partial.
func TestEtali_NoStaleDeferredAICastPartial(t *testing.T) {
	gs := newGame(t, 2)
	etali := addPerm(gs, 0, "Etali, Primal Storm", "legendary", "creature", "elemental")
	etali.Card.BasePower = 6
	etali.Card.BaseToughness = 6
	// Put a nonland on top of opponent's library so a grant would be registered.
	gs.Seats[1].Library = append([]*gameengine.Card{
		addCard(gs, 1, "Counterspell", "instant"),
	}, gs.Seats[1].Library...)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": etali,
		"seat":          0,
	})

	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == etali.Card.DisplayName() {
			t.Errorf("expected no per_card_partial after batch 7 cleanup; got %+v", ev)
		}
	}
}

// TestXyris_NoETBCoverageGapPartial pins the cleanup: ETB no longer
// emits the per-ETB coverage-gap partial.
func TestXyris_NoETBCoverageGapPartial(t *testing.T) {
	gs := newGame(t, 2)
	xyris := addPerm(gs, 0, "Xyris, the Writhing Storm", "legendary", "creature", "snake")

	gameengine.InvokeETBHook(gs, xyris)

	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == xyris.Card.DisplayName() {
			t.Errorf("expected no per_card_partial on Xyris ETB after batch 7 cleanup; got %+v", ev)
		}
	}
}

// TestStormForceOfNature_NoStaleConsumptionPartial pins the cleanup:
// combat-damage trigger no longer emits the "consumption handled by
// cast pipeline" partial.
func TestStormForceOfNature_NoStaleConsumptionPartial(t *testing.T) {
	gs := newGame(t, 2)
	storm := addPerm(gs, 0, "Storm, Force of Nature", "legendary", "creature", "elemental", "dragon")
	// Real P/T so SBA 704.5f doesn't kill Storm mid-trigger and clear
	// the storm_grant_pending flag via the LTB cleanup.
	storm.Card.BasePower = 7
	storm.Card.BaseToughness = 7

	gameengine.FireCardTrigger(gs, "combat_damage_to_player", map[string]interface{}{
		"source_perm": storm,
		"source_seat": 0,
		"source_card": storm.Card.DisplayName(),
		"target_seat": 1,
		"amount":      5,
	})

	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == storm.Card.DisplayName() {
			t.Errorf("expected no per_card_partial after batch 7 cleanup; got %+v", ev)
		}
	}
	// Verify the storm_grant_pending flag was set (the substantive
	// behavior survives the cleanup).
	if gs.Seats[0].Flags["storm_grant_pending"] != 1 {
		t.Errorf("expected storm_grant_pending flag set after combat damage; flags=%+v", gs.Seats[0].Flags)
	}
}

// TestValakutExploration_EndStepBurnsAndYards pins the existing
// end-step behavior is preserved after the batch 7 cleanup.
func TestValakutExploration_EndStepBurnsAndYards(t *testing.T) {
	gs := newGame(t, 2)
	valakut := addPerm(gs, 0, "Valakut Exploration", "enchantment")
	// Two tagged exile cards.
	c1 := addCard(gs, 0, "Card 1", valakutExplorationTag)
	c2 := addCard(gs, 0, "Card 2", valakutExplorationTag)
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, c1, c2)
	startOppLife := gs.Seats[1].Life

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})

	if gs.Seats[1].Life != startOppLife-2 {
		t.Errorf("expected opp life -2 from Valakut burn (2 yarded cards); before=%d after=%d", startOppLife, gs.Seats[1].Life)
	}
	// Tagged cards should have moved to graveyard.
	for _, c := range gs.Seats[0].Exile {
		if c == c1 || c == c2 {
			t.Errorf("yarded card should not still be in exile: %s", c.DisplayName())
		}
	}
	_ = valakut
}
