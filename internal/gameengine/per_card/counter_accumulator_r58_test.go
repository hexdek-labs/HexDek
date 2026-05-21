package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R58 — counter-accumulator-threshold primitive (gs.CounterThresholds)
// + 4 ports: Helix Pinnacle, Darksteel Reactor, Azor's Elocutors,
// Quest for Ula's Temple.

// ---------------------------------------------------------------------------
// Engine primitive smoke
// ---------------------------------------------------------------------------

func TestCounterThreshold_RegisterAndUnregister(t *testing.T) {
	gs := newGame(t, 2)
	p := addPerm(gs, 0, "Source", "creature")
	called := 0
	gs.RegisterCounterThreshold(&gameengine.CounterThresholdEffect{
		SourcePerm: p,
		HandlerID:  "test",
		Counter:    "test_counter",
		Threshold:  5,
		OnReach:    func(_ *gameengine.GameState, _ *gameengine.Permanent) { called++ },
	})
	if len(gs.CounterThresholds) != 1 {
		t.Fatalf("expected 1 threshold registered; got %d", len(gs.CounterThresholds))
	}
	gs.UnregisterCounterThresholdsForPermanent(p)
	if len(gs.CounterThresholds) != 0 {
		t.Errorf("expected 0 after unregister; got %d", len(gs.CounterThresholds))
	}
}

func TestCounterThreshold_OneShotFiresOnce(t *testing.T) {
	gs := newGame(t, 2)
	p := addPerm(gs, 0, "Source", "creature")
	called := 0
	gs.RegisterCounterThreshold(&gameengine.CounterThresholdEffect{
		SourcePerm: p,
		HandlerID:  "test",
		Counter:    "tower",
		Threshold:  3,
		Repeatable: false,
		OnReach:    func(_ *gameengine.GameState, _ *gameengine.Permanent) { called++ },
	})
	p.AddCounter("tower", 5)
	gs.EvaluateCounterThresholds()
	gs.EvaluateCounterThresholds()
	gs.EvaluateCounterThresholds()
	if called != 1 {
		t.Errorf("one-shot should fire exactly once; got %d", called)
	}
}

func TestCounterThreshold_RepeatableFiresEachEval(t *testing.T) {
	gs := newGame(t, 2)
	p := addPerm(gs, 0, "Source", "creature")
	called := 0
	gs.RegisterCounterThreshold(&gameengine.CounterThresholdEffect{
		SourcePerm: p,
		HandlerID:  "test",
		Counter:    "quest",
		Threshold:  3,
		Repeatable: true,
		OnReach:    func(_ *gameengine.GameState, _ *gameengine.Permanent) { called++ },
	})
	p.AddCounter("quest", 3)
	gs.EvaluateCounterThresholds()
	gs.EvaluateCounterThresholds()
	gs.EvaluateCounterThresholds()
	if called != 3 {
		t.Errorf("repeatable should fire each eval; got %d", called)
	}
}

func TestCounterThreshold_BelowThresholdDoesNotFire(t *testing.T) {
	gs := newGame(t, 2)
	p := addPerm(gs, 0, "Source", "creature")
	called := 0
	gs.RegisterCounterThreshold(&gameengine.CounterThresholdEffect{
		SourcePerm: p,
		HandlerID:  "test",
		Counter:    "charge",
		Threshold:  20,
		OnReach:    func(_ *gameengine.GameState, _ *gameengine.Permanent) { called++ },
	})
	p.AddCounter("charge", 19) // one short
	gs.EvaluateCounterThresholds()
	if called != 0 {
		t.Errorf("below-threshold should not fire; got %d", called)
	}
}

// ---------------------------------------------------------------------------
// Helix Pinnacle
// ---------------------------------------------------------------------------

func TestHelixPinnacle_ETBRegistersThresholdAndShroud(t *testing.T) {
	gs := newGame(t, 2)
	helix := addPerm(gs, 0, "Helix Pinnacle", "enchantment")
	helixPinnacleETB(gs, helix)
	if len(gs.CounterThresholds) != 1 {
		t.Fatalf("expected 1 threshold; got %d", len(gs.CounterThresholds))
	}
	if helix.Flags["kw:shroud"] != 1 {
		t.Errorf("Helix should have shroud while under 100 tower counters")
	}
}

func TestHelixPinnacle_PourXAddsTowerCounters(t *testing.T) {
	gs := newGame(t, 2)
	helix := addPerm(gs, 0, "Helix Pinnacle", "enchantment")
	helixPinnacleETB(gs, helix)
	gs.Seats[0].ManaPool = 10
	helixPinnacleActivate(gs, helix, 0, map[string]interface{}{"x": 7})
	if gameengine.CounterThresholdCount(helix, "tower") != 7 {
		t.Errorf("expected 7 tower counters; got %d",
			gameengine.CounterThresholdCount(helix, "tower"))
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected pool 3 (10 - 7); got %d", gs.Seats[0].ManaPool)
	}
	if !helix.Tapped {
		t.Errorf("Helix should be tapped after activation")
	}
}

func TestHelixPinnacle_UpkeepAtOrAbove100Wins(t *testing.T) {
	gs := newGame(t, 2)
	helix := addPerm(gs, 0, "Helix Pinnacle", "enchantment")
	helixPinnacleETB(gs, helix)
	helix.AddCounter("tower", 100)
	helixPinnacleUpkeep(gs, helix, map[string]interface{}{"active_seat": 0})
	if !gs.Seats[0].Won {
		t.Errorf("controller should win at 100 tower counters")
	}
	if !gs.Seats[1].Lost {
		t.Errorf("opponent should be marked Lost")
	}
}

func TestHelixPinnacle_ShroudDropsAt100(t *testing.T) {
	gs := newGame(t, 2)
	helix := addPerm(gs, 0, "Helix Pinnacle", "enchantment")
	helixPinnacleETB(gs, helix)
	helix.AddCounter("tower", 100)
	helixPinnacleRefreshShroud(helix)
	if helix.Flags["kw:shroud"] == 1 {
		t.Errorf("shroud should clear at 100 tower counters")
	}
}

func TestHelixPinnacle_UpkeepOnOpponentTurnNoFire(t *testing.T) {
	gs := newGame(t, 2)
	helix := addPerm(gs, 0, "Helix Pinnacle", "enchantment")
	helixPinnacleETB(gs, helix)
	helix.AddCounter("tower", 100)
	helixPinnacleUpkeep(gs, helix, map[string]interface{}{"active_seat": 1})
	if gs.Seats[0].Won {
		t.Errorf("Helix should only check on controller's upkeep")
	}
}

// ---------------------------------------------------------------------------
// Darksteel Reactor
// ---------------------------------------------------------------------------

func TestDarksteelReactor_ETBPlacesFirstCharge(t *testing.T) {
	gs := newGame(t, 2)
	reactor := addPerm(gs, 0, "Darksteel Reactor", "artifact", "legendary")
	darksteelReactorETB(gs, reactor)
	if gameengine.CounterThresholdCount(reactor, "charge") != 1 {
		t.Errorf("ETB should place 1 charge counter; got %d",
			gameengine.CounterThresholdCount(reactor, "charge"))
	}
}

func TestDarksteelReactor_NineteenUpkeepsToWin(t *testing.T) {
	gs := newGame(t, 2)
	reactor := addPerm(gs, 0, "Darksteel Reactor", "artifact", "legendary")
	darksteelReactorETB(gs, reactor)
	// Already at 1 from ETB; need 19 more upkeeps to hit 20.
	for i := 0; i < 19; i++ {
		darksteelReactorUpkeep(gs, reactor, map[string]interface{}{"active_seat": 0})
		if gs.Seats[0].Won {
			if i < 18 {
				t.Errorf("Reactor shouldn't win before reaching 20 charge (at iter %d)", i)
			}
		}
	}
	if !gs.Seats[0].Won {
		t.Errorf("controller should win after 20 charge counters")
	}
}

// ---------------------------------------------------------------------------
// Azor's Elocutors
// ---------------------------------------------------------------------------

func TestAzorsElocutors_FiveUpkeepsToWin(t *testing.T) {
	gs := newGame(t, 2)
	azor := addPerm(gs, 0, "Azor's Elocutors", "creature", "legendary")
	azorsElocutorsETB(gs, azor)
	for i := 0; i < 5; i++ {
		azorsElocutorsUpkeep(gs, azor, map[string]interface{}{"active_seat": 0})
	}
	if !gs.Seats[0].Won {
		t.Errorf("controller should win after 5 filibuster counters")
	}
}

func TestAzorsElocutors_DamageResetsFilibusterCounters(t *testing.T) {
	gs := newGame(t, 2)
	azor := addPerm(gs, 0, "Azor's Elocutors", "creature", "legendary")
	azorsElocutorsETB(gs, azor)
	azor.AddCounter("filibuster", 4) // one short of win
	azorsElocutorsDamageReset(gs, azor, map[string]interface{}{
		"seat":   0,
		"amount": 3,
	})
	if gameengine.CounterThresholdCount(azor, "filibuster") != 0 {
		t.Errorf("damage should remove all filibuster counters; got %d",
			gameengine.CounterThresholdCount(azor, "filibuster"))
	}
}

func TestAzorsElocutors_OpponentDamageIgnored(t *testing.T) {
	gs := newGame(t, 2)
	azor := addPerm(gs, 0, "Azor's Elocutors", "creature", "legendary")
	azorsElocutorsETB(gs, azor)
	azor.AddCounter("filibuster", 4)
	azorsElocutorsDamageReset(gs, azor, map[string]interface{}{
		"seat":   1, // opponent damaged, not Azor's controller
		"amount": 3,
	})
	if gameengine.CounterThresholdCount(azor, "filibuster") != 4 {
		t.Errorf("opp damage should NOT clear Azor's counters; got %d",
			gameengine.CounterThresholdCount(azor, "filibuster"))
	}
}

// ---------------------------------------------------------------------------
// Quest for Ula's Temple
// ---------------------------------------------------------------------------

func TestQuestForUlasTemple_BelowThresholdNoCheat(t *testing.T) {
	gs := newGame(t, 2)
	quest := addPerm(gs, 0, "Quest for Ula's Temple", "enchantment")
	questForUlasTempleETB(gs, quest)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
		Name: "Kraken", Owner: 0, Types: []string{"creature", "kraken", "cmc:9"},
	})
	// Two upkeeps — not enough.
	questForUlasTempleUpkeep(gs, quest, map[string]interface{}{"active_seat": 0})
	questForUlasTempleUpkeep(gs, quest, map[string]interface{}{"active_seat": 0})
	// Kraken should still be in hand.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Kraken still in hand at 2 counters; got hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestQuestForUlasTemple_AtThresholdCheatsKraken(t *testing.T) {
	gs := newGame(t, 2)
	quest := addPerm(gs, 0, "Quest for Ula's Temple", "enchantment")
	questForUlasTempleETB(gs, quest)
	kraken := &gameengine.Card{
		Name: "Inkwell Leviathan", Owner: 0, Types: []string{"creature", "leviathan", "cmc:7"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, kraken)
	for i := 0; i < 3; i++ {
		questForUlasTempleUpkeep(gs, quest, map[string]interface{}{"active_seat": 0})
	}
	// At 3 quest counters, OnReach should have cheated the Leviathan
	// onto the battlefield.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected Leviathan moved from hand; hand=%d", len(gs.Seats[0].Hand))
	}
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == kraken {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Leviathan should be on the battlefield")
	}
}

func TestQuestForUlasTemple_RepeatableFiresEveryUpkeep(t *testing.T) {
	gs := newGame(t, 2)
	quest := addPerm(gs, 0, "Quest for Ula's Temple", "enchantment")
	questForUlasTempleETB(gs, quest)
	// Stock two eligible creatures.
	c1 := &gameengine.Card{Name: "Octopus 1", Owner: 0, Types: []string{"creature", "octopus", "cmc:4"}}
	c2 := &gameengine.Card{Name: "Serpent 1", Owner: 0, Types: []string{"creature", "serpent", "cmc:5"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, c1, c2)
	// Reach threshold (3 upkeeps), then run two more — each should
	// cheat another creature.
	for i := 0; i < 3; i++ {
		questForUlasTempleUpkeep(gs, quest, map[string]interface{}{"active_seat": 0})
	}
	// At iter 3: 1 creature cheated, 1 remaining.
	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("expected 1 left in hand after 3 upkeeps; got %d", len(gs.Seats[0].Hand))
	}
	// 4th upkeep — still at >= 3 counters → repeatable fires again.
	questForUlasTempleUpkeep(gs, quest, map[string]interface{}{"active_seat": 0})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("repeatable should fire on 4th upkeep; expected hand empty, got %d",
			len(gs.Seats[0].Hand))
	}
}

func TestQuestForUlasTemple_NonEligibleCreatureSkipped(t *testing.T) {
	gs := newGame(t, 2)
	quest := addPerm(gs, 0, "Quest for Ula's Temple", "enchantment")
	questForUlasTempleETB(gs, quest)
	// Bear is creature but not a Kraken/Leviathan/Octopus/Serpent.
	bear := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature", "bear"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, bear)
	for i := 0; i < 3; i++ {
		questForUlasTempleUpkeep(gs, quest, map[string]interface{}{"active_seat": 0})
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("non-eligible creature should stay in hand; got %d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// LTB unregisters
// ---------------------------------------------------------------------------

func TestCounterAccum_LTBUnregistersAllFour(t *testing.T) {
	gs := newGame(t, 2)
	helix := addPerm(gs, 0, "Helix Pinnacle", "enchantment")
	reactor := addPerm(gs, 0, "Darksteel Reactor", "artifact")
	azor := addPerm(gs, 0, "Azor's Elocutors", "creature")
	quest := addPerm(gs, 0, "Quest for Ula's Temple", "enchantment")
	helixPinnacleETB(gs, helix)
	darksteelReactorETB(gs, reactor)
	azorsElocutorsETB(gs, azor)
	questForUlasTempleETB(gs, quest)
	if len(gs.CounterThresholds) != 4 {
		t.Fatalf("expected 4 thresholds registered; got %d", len(gs.CounterThresholds))
	}
	helixPinnacleLTB(gs, helix, map[string]interface{}{"perm": helix})
	darksteelReactorLTB(gs, reactor, map[string]interface{}{"perm": reactor})
	azorsElocutorsLTB(gs, azor, map[string]interface{}{"perm": azor})
	questForUlasTempleLTB(gs, quest, map[string]interface{}{"perm": quest})
	if len(gs.CounterThresholds) != 0 {
		t.Errorf("all LTBs should unregister; got %d", len(gs.CounterThresholds))
	}
}
