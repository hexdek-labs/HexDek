package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// card_cycled_consumers_r60 — 6 handlers wired in
// card_cycled_consumers_r60.go covering the cycling-payoff family
// (CR §702.29).
// -----------------------------------------------------------------------------

// cycledCtx mirrors fireCyclingTriggers's ctx shape.
func cycledCtx(seat int, cycledName string) map[string]interface{} {
	return map[string]interface{}{
		"cycled_card":      &gameengine.Card{Name: cycledName, Owner: seat, Types: []string{"creature"}},
		"cycled_card_name": cycledName,
		"controller_seat":  seat,
	}
}

// -----------------------------------------------------------------------------
// Astral Slide
// -----------------------------------------------------------------------------

func TestAstralSlide_ExilesAndReturnsAtEOT(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Astral Slide", "enchantment")
	target := addPerm(gs, 1, "Hill Giant", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 3, 3
	targetCard := target.Card

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Slogurk, the Overslime"))

	// Hill Giant should be in opp's exile, not battlefield.
	stillBF := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			stillBF = true
		}
	}
	if stillBF {
		t.Errorf("Hill Giant should be exiled by Astral Slide")
	}
	inExile := false
	for _, c := range gs.Seats[1].Exile {
		if c == targetCard {
			inExile = true
		}
	}
	if !inExile {
		t.Errorf("Hill Giant card should be in opp's exile zone")
	}

	// Fire the delayed-trigger drain at next end step.
	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.TriggerAt == "next_end_step" && !dt.Consumed {
			dt.EffectFn(gs)
			dt.Consumed = true
		}
	}

	// Hill Giant should be back on opp's battlefield.
	backOnBF := false
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.Card == targetCard {
			backOnBF = true
		}
	}
	if !backOnBF {
		t.Errorf("Hill Giant should return to opp's battlefield at next end step")
	}
}

func TestAstralSlide_FiresOnAnyPlayerCycle(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Astral Slide", "enchantment")
	target := addPerm(gs, 1, "Hill Giant", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 3, 3

	// Opponent (seat 1) cycles — Astral Slide says "a player," so this
	// must fire even though seat 1 isn't Astral Slide's controller.
	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(1, "Renegade Map"))

	stillBF := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			stillBF = true
		}
	}
	if stillBF {
		t.Errorf("Astral Slide should trigger on opp cycle too (oracle: 'a player')")
	}
}

// -----------------------------------------------------------------------------
// Lightning Rift
// -----------------------------------------------------------------------------

func TestLightningRift_PingsOpponentOnCycle(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 1
	gs.Seats[1].Life = 20
	addPerm(gs, 0, "Lightning Rift", "enchantment")

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Some Cycler"))

	if gs.Seats[1].Life != 18 {
		t.Errorf("expected 2 dmg to opp (life 20→18), got %d", gs.Seats[1].Life)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 1 mana spent, got %d", gs.Seats[0].ManaPool)
	}
}

func TestLightningRift_NoManaNoFire(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 0
	gs.Seats[1].Life = 20
	addPerm(gs, 0, "Lightning Rift", "enchantment")

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Some Cycler"))

	if gs.Seats[1].Life != 20 {
		t.Errorf("no mana → no damage; got %d", gs.Seats[1].Life)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed for no_mana_to_pay")
	}
}

func TestLightningRift_FiresOnAnyPlayerCycle(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 1
	gs.Seats[1].Life = 20
	addPerm(gs, 0, "Lightning Rift", "enchantment")

	// Opponent cycles — Rift triggers because "a player" not "you."
	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(1, "Opp Cycler"))

	if gs.Seats[1].Life != 18 {
		t.Errorf("expected ping on opp cycle, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Faith of the Devoted
// -----------------------------------------------------------------------------

func TestFaithOfTheDevoted_DrainsOnYourCycle(t *testing.T) {
	gs := newGame(t, 4)
	gs.Seats[0].ManaPool = 1
	gs.Seats[0].Life = 20
	for i := 1; i < 4; i++ {
		gs.Seats[i].Life = 20
	}
	addPerm(gs, 0, "Faith of the Devoted", "enchantment")

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Some Cycler"))

	if gs.Seats[0].Life != 22 {
		t.Errorf("expected +2 life to controller, got %d", gs.Seats[0].Life)
	}
	for i := 1; i < 4; i++ {
		if gs.Seats[i].Life != 18 {
			t.Errorf("opp %d expected 18 life, got %d", i, gs.Seats[i].Life)
		}
	}
}

func TestFaithOfTheDevoted_IgnoresOpponentCycle(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 1
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	addPerm(gs, 0, "Faith of the Devoted", "enchantment")

	// Opp cycles — "you" filter rejects.
	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(1, "Opp Cycler"))

	if gs.Seats[0].Life != 20 || gs.Seats[1].Life != 20 {
		t.Errorf("opp cycle must not trigger Faith; seat0=%d seat1=%d",
			gs.Seats[0].Life, gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Drake Haven
// -----------------------------------------------------------------------------

func TestDrakeHaven_CreatesDrakeOnYourCycle(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 1
	addPerm(gs, 0, "Drake Haven", "enchantment")
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Some Cycler"))

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected battlefield +1 (Drake token); pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	foundDrake := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Drake" {
			foundDrake = true
			break
		}
	}
	if !foundDrake {
		t.Errorf("expected a Drake token")
	}
}

func TestDrakeHaven_NoManaNoFire(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 0
	addPerm(gs, 0, "Drake Haven", "enchantment")
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Some Cycler"))

	if len(gs.Seats[0].Battlefield) != pre {
		t.Errorf("no mana → no Drake; pre=%d post=%d", pre, len(gs.Seats[0].Battlefield))
	}
}

// -----------------------------------------------------------------------------
// Curator of Mysteries
// -----------------------------------------------------------------------------

func TestCuratorOfMysteries_ScriesOnYourCycle(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "TopCard")
	curator := addPerm(gs, 0, "Curator of Mysteries", "creature")
	curator.Card.BasePower, curator.Card.BaseToughness = 4, 4

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Some Cycler"))

	if hasEvent(gs, "surveil") < 1 && hasEvent(gs, "scry") < 1 && hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected scry / per_card_handler event from Curator on cycle; events=%+v", gs.EventLog)
	}
}

func TestCuratorOfMysteries_IgnoresSelfCycle(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "TopCard")
	curator := addPerm(gs, 0, "Curator of Mysteries", "creature")
	curator.Card.BasePower, curator.Card.BaseToughness = 4, 4

	// Cycling Curator itself ("another card" filter).
	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Curator of Mysteries"))

	if hasEvent(gs, "per_card_handler") > 0 {
		t.Errorf("Curator's 'another card' filter should reject self-cycle; events=%+v", gs.EventLog)
	}
}

func TestCuratorOfMysteries_IgnoresOpponentCycle(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "TopCard")
	curator := addPerm(gs, 0, "Curator of Mysteries", "creature")
	curator.Card.BasePower, curator.Card.BaseToughness = 4, 4

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(1, "Opp Cycler"))

	if hasEvent(gs, "per_card_handler") > 0 {
		t.Errorf("Curator must not fire on opp cycle (oracle: 'you cycle')")
	}
}

// -----------------------------------------------------------------------------
// Archfiend of Ifnir
// -----------------------------------------------------------------------------

func TestArchfiendOfIfnir_MarksOppCreaturesOnYourCycle(t *testing.T) {
	gs := newGame(t, 2)
	arch := addPerm(gs, 0, "Archfiend of Ifnir", "creature")
	arch.Card.BasePower, arch.Card.BaseToughness = 5, 4

	opp1 := addPerm(gs, 1, "Hill Giant", "creature")
	opp1.Card.BasePower, opp1.Card.BaseToughness = 3, 3
	opp2 := addPerm(gs, 1, "Llanowar Elves", "creature")
	opp2.Card.BasePower, opp2.Card.BaseToughness = 1, 1

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Some Cycler"))

	if opp1.Counters["-1/-1"] != 1 {
		t.Errorf("expected -1/-1 counter on Hill Giant, got %d", opp1.Counters["-1/-1"])
	}
	if opp2.Counters["-1/-1"] != 1 {
		t.Errorf("expected -1/-1 counter on Llanowar Elves, got %d", opp2.Counters["-1/-1"])
	}
}

func TestArchfiendOfIfnir_IgnoresOwnCreatures(t *testing.T) {
	gs := newGame(t, 2)
	arch := addPerm(gs, 0, "Archfiend of Ifnir", "creature")
	arch.Card.BasePower, arch.Card.BaseToughness = 5, 4
	friend := addPerm(gs, 0, "Llanowar Elves", "creature")
	friend.Card.BasePower, friend.Card.BaseToughness = 1, 1

	gameengine.FireCardTrigger(gs, "card_cycled", cycledCtx(0, "Some Cycler"))

	if friend.Counters["-1/-1"] != 0 {
		t.Errorf("own creature must not get -1/-1 counter; got %d", friend.Counters["-1/-1"])
	}
}

// -----------------------------------------------------------------------------
// Registry smoke
// -----------------------------------------------------------------------------

func TestCardCycledConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Astral Slide", "Lightning Rift", "Faith of the Devoted",
		"Drake Haven", "Curator of Mysteries", "Archfiend of Ifnir",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler for %s", name)
		}
	}
}
