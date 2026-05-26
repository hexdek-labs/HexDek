package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Magecraft consumers r60 — 8 cards wired in magecraft_consumers_r60.go.
//
// All cases drive the trigger via gameengine.FireCardTrigger("magecraft",
// ctx) — the same path the engine's FireMagecraftTriggers
// (keywords_magecraft.go:164) uses after CR §702.137a filtering.
// Tests assume the engine-side filter has already run (only instant/sorcery
// reaches here, only friendly perms get fanned out), and pin the per_card
// payoff itself.
// -----------------------------------------------------------------------------

// magecraftCtx assembles the ctx that FireMagecraftTriggers forwards.
func magecraftCtx(src *gameengine.Permanent, spell *gameengine.Card, isCopy bool) map[string]interface{} {
	return map[string]interface{}{
		"source":     src,
		"controller": src.Controller,
		"spell":      spell,
		"spell_name": spell.DisplayName(),
		"is_copy":    isCopy,
	}
}

// Archmage Emeritus — draw on every instant/sorcery cast or copy.
func TestArchmageEmeritus_DrawsOnMagecraft(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top", "Next")
	emeritus := addPerm(gs, 0, "Archmage Emeritus", "creature")
	emeritus.Card.BasePower, emeritus.Card.BaseToughness = 2, 2

	spell := addCard(gs, 0, "Lightning Bolt", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(emeritus, spell, false))

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn, got hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Hand[0].DisplayName() != "Top" {
		t.Errorf("expected top of library drawn, got %s", gs.Seats[0].Hand[0].DisplayName())
	}
}

func TestArchmageEmeritus_FiresOnCopyToo(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top")
	emeritus := addPerm(gs, 0, "Archmage Emeritus", "creature")
	emeritus.Card.BasePower, emeritus.Card.BaseToughness = 2, 2

	spell := addCard(gs, 0, "Lightning Bolt", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(emeritus, spell, true))

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("magecraft must fire on copies too; hand=%d", len(gs.Seats[0].Hand))
	}
}

// Witherbloom Apprentice — drain 1 from each opp + gain 1.
func TestWitherbloomApprentice_DrainsEachOpp(t *testing.T) {
	gs := newGame(t, 4)
	app := addPerm(gs, 0, "Witherbloom Apprentice", "creature")
	app.Card.BasePower, app.Card.BaseToughness = 1, 1
	for i := 0; i < 4; i++ {
		gs.Seats[i].Life = 20
	}

	spell := addCard(gs, 0, "Strangle", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(app, spell, false))

	if gs.Seats[0].Life != 21 {
		t.Errorf("controller should gain 1 life; got %d", gs.Seats[0].Life)
	}
	for i := 1; i < 4; i++ {
		if gs.Seats[i].Life != 19 {
			t.Errorf("opp %d should lose 1 life; got %d", i, gs.Seats[i].Life)
		}
	}
}

func TestWitherbloomApprentice_SkipsLostOpps(t *testing.T) {
	gs := newGame(t, 4)
	app := addPerm(gs, 0, "Witherbloom Apprentice", "creature")
	app.Card.BasePower, app.Card.BaseToughness = 1, 1
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Seats[2].Life = 20
	gs.Seats[3].Life = 0
	gs.Seats[3].Lost = true

	spell := addCard(gs, 0, "Strangle", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(app, spell, false))

	if gs.Seats[3].Life != 0 {
		t.Errorf("eliminated opp should not be drained (CR §800.4); got %d", gs.Seats[3].Life)
	}
	if gs.Seats[1].Life != 19 || gs.Seats[2].Life != 19 {
		t.Errorf("living opps should still be drained; got %d, %d", gs.Seats[1].Life, gs.Seats[2].Life)
	}
}

// Witherbloom Pledgemage — gain 1 life on every magecraft trigger.
func TestWitherbloomPledgemage_GainsOneLife(t *testing.T) {
	gs := newGame(t, 2)
	mage := addPerm(gs, 0, "Witherbloom Pledgemage", "creature")
	mage.Card.BasePower, mage.Card.BaseToughness = 3, 3
	gs.Seats[0].Life = 20

	spell := addCard(gs, 0, "Cantrip", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(mage, spell, false))

	if gs.Seats[0].Life != 21 {
		t.Errorf("expected 21 life, got %d", gs.Seats[0].Life)
	}
}

// Leonin Lightscribe — your creatures get +1/+1 UEOT.
func TestLeoninLightscribe_BuffsFriendlyCreatures(t *testing.T) {
	gs := newGame(t, 2)
	leonin := addPerm(gs, 0, "Leonin Lightscribe", "creature")
	leonin.Card.BasePower, leonin.Card.BaseToughness = 1, 2

	friend1 := addPerm(gs, 0, "Savannah Lions", "creature")
	friend1.Card.BasePower, friend1.Card.BaseToughness = 2, 1

	friend2 := addPerm(gs, 0, "Llanowar Elves", "creature")
	friend2.Card.BasePower, friend2.Card.BaseToughness = 1, 1

	opp := addPerm(gs, 1, "Goblin", "creature")
	opp.Card.BasePower, opp.Card.BaseToughness = 1, 1

	spell := addCard(gs, 0, "Cantrip", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(leonin, spell, false))

	if leonin.Power() != 2 || leonin.Toughness() != 3 {
		t.Errorf("Leonin should be 2/3 after self-buff (1/2+1/+1); got %d/%d", leonin.Power(), leonin.Toughness())
	}
	if friend1.Power() != 3 || friend1.Toughness() != 2 {
		t.Errorf("friend1 should be 3/2; got %d/%d", friend1.Power(), friend1.Toughness())
	}
	if friend2.Power() != 2 || friend2.Toughness() != 2 {
		t.Errorf("friend2 should be 2/2; got %d/%d", friend2.Power(), friend2.Toughness())
	}
	if opp.Power() != 1 || opp.Toughness() != 1 {
		t.Errorf("opponent creature should NOT be buffed; got %d/%d", opp.Power(), opp.Toughness())
	}
}

// Storm-Kiln Artist — create a Treasure token.
func TestStormKilnArtist_CreatesTreasure(t *testing.T) {
	gs := newGame(t, 2)
	artist := addPerm(gs, 0, "Storm-Kiln Artist", "creature")
	artist.Card.BasePower, artist.Card.BaseToughness = 1, 4
	pre := len(gs.Seats[0].Battlefield)

	spell := addCard(gs, 0, "Lightning Bolt", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(artist, spell, false))

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected battlefield +1 (Treasure token), pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, ty := range p.Card.Types {
			if ty == "treasure" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected a Treasure token on battlefield")
	}
}

// Dragonsguard Elite — +1/+1 counter on self.
func TestDragonsguardElite_AddsSelfCounter(t *testing.T) {
	gs := newGame(t, 2)
	elite := addPerm(gs, 0, "Dragonsguard Elite", "creature")
	elite.Card.BasePower, elite.Card.BaseToughness = 2, 2

	spell := addCard(gs, 0, "Cantrip", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(elite, spell, false))

	if elite.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 +1/+1 counter, got %d", elite.Counters["+1/+1"])
	}
	if elite.Power() != 3 || elite.Toughness() != 3 {
		t.Errorf("expected 3/3 after +1/+1 counter; got %d/%d", elite.Power(), elite.Toughness())
	}
}

func TestDragonsguardElite_StacksCounterAcrossMultipleCasts(t *testing.T) {
	gs := newGame(t, 2)
	elite := addPerm(gs, 0, "Dragonsguard Elite", "creature")
	elite.Card.BasePower, elite.Card.BaseToughness = 2, 2

	spell := addCard(gs, 0, "Cantrip", "instant")
	for i := 0; i < 3; i++ {
		gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(elite, spell, false))
	}
	if elite.Counters["+1/+1"] != 3 {
		t.Errorf("expected 3 counters after 3 magecraft fires, got %d", elite.Counters["+1/+1"])
	}
}

// Quandrix Pledgemage — +1/+1 counter on self.
func TestQuandrixPledgemage_AddsSelfCounter(t *testing.T) {
	gs := newGame(t, 2)
	mage := addPerm(gs, 0, "Quandrix Pledgemage", "creature")
	mage.Card.BasePower, mage.Card.BaseToughness = 3, 3

	spell := addCard(gs, 0, "Cantrip", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(mage, spell, false))

	if mage.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 +1/+1 counter, got %d", mage.Counters["+1/+1"])
	}
}

// Karok Wrangler — +1/+1 counter on highest-power friendly creature.
func TestKarokWrangler_TargetsHighestPowerFriendly(t *testing.T) {
	gs := newGame(t, 2)
	wrangler := addPerm(gs, 0, "Karok Wrangler", "creature")
	wrangler.Card.BasePower, wrangler.Card.BaseToughness = 3, 5

	weakling := addPerm(gs, 0, "Llanowar Elves", "creature")
	weakling.Card.BasePower, weakling.Card.BaseToughness = 1, 1

	bigfella := addPerm(gs, 0, "Craterhoof", "creature")
	bigfella.Card.BasePower, bigfella.Card.BaseToughness = 5, 5

	spell := addCard(gs, 0, "Giant Growth", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(wrangler, spell, false))

	if bigfella.Counters["+1/+1"] != 1 {
		t.Errorf("expected counter on highest-power Craterhoof, got %d", bigfella.Counters["+1/+1"])
	}
	if weakling.Counters["+1/+1"] != 0 {
		t.Errorf("weakling should not receive counter, got %d", weakling.Counters["+1/+1"])
	}
}

func TestKarokWrangler_DoesNotCounterOpponentCreatures(t *testing.T) {
	gs := newGame(t, 2)
	wrangler := addPerm(gs, 0, "Karok Wrangler", "creature")
	wrangler.Card.BasePower, wrangler.Card.BaseToughness = 3, 5

	oppBig := addPerm(gs, 1, "Massive Dragon", "creature")
	oppBig.Card.BasePower, oppBig.Card.BaseToughness = 10, 10

	myFriend := addPerm(gs, 0, "Llanowar Elves", "creature")
	myFriend.Card.BasePower, myFriend.Card.BaseToughness = 1, 1

	spell := addCard(gs, 0, "Cantrip", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(wrangler, spell, false))

	if oppBig.Counters["+1/+1"] != 0 {
		t.Errorf("opp creature must never get the counter, got %d", oppBig.Counters["+1/+1"])
	}
	// Wrangler (3) > Llanowar (1) → wrangler buffs itself.
	if wrangler.Counters["+1/+1"] != 1 && myFriend.Counters["+1/+1"] != 1 {
		t.Errorf("expected a friendly creature to receive counter; wrangler=%d friend=%d",
			wrangler.Counters["+1/+1"], myFriend.Counters["+1/+1"])
	}
}

func TestKarokWrangler_SelfReceivesCounterWhenLoneCreature(t *testing.T) {
	gs := newGame(t, 2)
	wrangler := addPerm(gs, 0, "Karok Wrangler", "creature")
	wrangler.Card.BasePower, wrangler.Card.BaseToughness = 3, 5
	// Sole friendly creature → handler picks Wrangler itself (the oracle
	// permits self-target for "target creature you control"). Pins that
	// the handler doesn't no-op when there's no other choice.

	spell := addCard(gs, 0, "Cantrip", "instant")
	gameengine.FireCardTrigger(gs, "magecraft", magecraftCtx(wrangler, spell, false))

	if wrangler.Counters["+1/+1"] != 1 {
		t.Errorf("Wrangler should self-target when only friendly creature; got counters=%d", wrangler.Counters["+1/+1"])
	}
}

// Registry smoke — every handler is reachable post-registerDefaults.
func TestMagecraftConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Archmage Emeritus",
		"Witherbloom Apprentice",
		"Witherbloom Pledgemage",
		"Leonin Lightscribe",
		"Storm-Kiln Artist",
		"Dragonsguard Elite",
		"Quandrix Pledgemage",
		"Karok Wrangler",
	}
	reg := Global()
	for _, name := range cards {
		hasCast, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		_ = hasCast
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler registered for %s", name)
		}
	}
}
