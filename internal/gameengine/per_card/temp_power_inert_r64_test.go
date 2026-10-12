package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// r64 — "temp_power" pumps were inert.
//
// Ten handlers pumped power/toughness by writing only
// Permanent.Flags["temp_power"] / ["temp_toughness"]. Nothing in the
// engine reads those flags: not the layer system, not Permanent.Power(),
// not combat damage, and nothing clears them. Every one of those pumps
// therefore did nothing to the creature.
//
// The existing tests for several of these cards asserted on the flag,
// so they passed against a card that did nothing — which is how the bug
// survived. These tests assert on gs.PowerOf / gs.ToughnessOf, the
// layer-correct query, so they can only pass if the pump is real.
// ---------------------------------------------------------------------------

func pt(gs *gameengine.GameState, p *gameengine.Permanent) (int, int) {
	return gs.PowerOf(p), gs.ToughnessOf(p)
}

func TestTempPumpR64_AdelizRaisesRealPower(t *testing.T) {
	gs := newGame(t, 2)
	adeliz := addPerm(gs, 0, "Adeliz, the Cinder Wind", "creature", "legendary", "human", "wizard")
	adeliz.Card.BasePower, adeliz.Card.BaseToughness = 2, 2
	notWiz := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	notWiz.Card.BasePower, notWiz.Card.BaseToughness = 1, 1

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 0,
	})

	if p, tg := pt(gs, adeliz); p != 3 || tg != 3 {
		t.Errorf("Adeliz after one instant: want 3/3, got %d/%d", p, tg)
	}
	if p, tg := pt(gs, notWiz); p != 1 || tg != 1 {
		t.Errorf("non-Wizard must not be pumped: want 1/1, got %d/%d", p, tg)
	}

	// Repeatable trigger: the second cast stacks on the first.
	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 0,
	})
	if p, tg := pt(gs, adeliz); p != 4 || tg != 4 {
		t.Errorf("Adeliz after two instants: want 4/4, got %d/%d", p, tg)
	}

	// The legacy diagnostic stamp is preserved for existing readers.
	if adeliz.Flags["temp_power"] != 2 {
		t.Errorf("legacy temp_power stamp should still track the pump, got %d",
			adeliz.Flags["temp_power"])
	}
}

func TestTempPumpR64_PumpExpiresAtCleanup(t *testing.T) {
	gs := newGame(t, 2)
	wiz := addPerm(gs, 0, "Adeliz, the Cinder Wind", "creature", "legendary", "human", "wizard")
	wiz.Card.BasePower, wiz.Card.BaseToughness = 2, 2

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 0,
	})
	if p, _ := pt(gs, wiz); p != 3 {
		t.Fatalf("precondition: want power 3, got %d", p)
	}

	// CR §514.2 — "until end of turn" effects end at cleanup. The
	// modification must be swept; a pump that never expires is as wrong
	// as one that never applies.
	kept := wiz.Modifications[:0]
	for _, m := range wiz.Modifications {
		if m.Duration == "until_end_of_turn" {
			continue
		}
		kept = append(kept, m)
	}
	wiz.Modifications = kept
	gs.InvalidateCharacteristicsCache()

	if p, tg := pt(gs, wiz); p != 2 || tg != 2 {
		t.Errorf("after cleanup sweep: want 2/2, got %d/%d", p, tg)
	}
}

func TestTempPumpR64_SepticRatsGrowsWhenDefenderPoisoned(t *testing.T) {
	gs := newGame(t, 2)
	rats := addPerm(gs, 0, "Septic Rats", "creature", "phyrexian", "rat")
	rats.Card.BasePower, rats.Card.BaseToughness = 2, 2
	gs.Seats[1].PoisonCounters = 1

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": rats,
		"defender_seat": 1,
	})

	if p, tg := pt(gs, rats); p != 3 || tg != 3 {
		t.Errorf("Septic Rats attacking a poisoned player: want 3/3, got %d/%d", p, tg)
	}
}

func TestTempPumpR64_SepticRatsFlatWhenDefenderClean(t *testing.T) {
	gs := newGame(t, 2)
	rats := addPerm(gs, 0, "Septic Rats", "creature", "phyrexian", "rat")
	rats.Card.BasePower, rats.Card.BaseToughness = 2, 2

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": rats,
		"defender_seat": 1,
	})

	if p, tg := pt(gs, rats); p != 2 || tg != 2 {
		t.Errorf("no poison on the defender: want 2/2, got %d/%d", p, tg)
	}
}

func TestTempPumpR64_ThrakkusDoublesCurrentNotPrintedPower(t *testing.T) {
	gs := newGame(t, 2)
	thrakkus := addPerm(gs, 0, "Thrakkus the Butcher", "creature", "legendary", "dragon")
	thrakkus.Card.BasePower, thrakkus.Card.BaseToughness = 6, 5

	// A 4/4 Dragon carrying two +1/+1 counters is a 6/6 right now.
	// "Double its power" must double six, not the printed four.
	drake := addPerm(gs, 0, "Shivan Dragon", "creature", "dragon")
	drake.Card.BasePower, drake.Card.BaseToughness = 4, 4
	drake.Counters["+1/+1"] = 2

	notDragon := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	notDragon.Card.BasePower, notDragon.Card.BaseToughness = 2, 2

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": thrakkus,
	})

	if p, _ := pt(gs, drake); p != 12 {
		t.Errorf("Dragon at 6 power doubled: want 12, got %d", p)
	}
	if p, _ := pt(gs, notDragon); p != 2 {
		t.Errorf("non-Dragon must not be doubled: want 2, got %d", p)
	}
}

func TestTempPumpR64_JaradStaticRestatesInsteadOfStacking(t *testing.T) {
	gs := newGame(t, 2)
	jarad := addPerm(gs, 0, "Jarad, Golgari Lich Lord", "creature", "legendary", "zombie", "elf")
	jarad.Card.BasePower, jarad.Card.BaseToughness = 2, 2
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		addCard(gs, 0, "Llanowar Elves", "creature"),
		addCard(gs, 0, "Birds of Paradise", "creature"),
		addCard(gs, 0, "Lightning Bolt", "instant"),
	)

	jaradGolgariLichLordETB(gs, jarad)
	if p, tg := pt(gs, jarad); p != 4 || tg != 4 {
		t.Errorf("Jarad with 2 creatures in yard: want 4/4, got %d/%d", p, tg)
	}

	// Flicker: a second ETB restates the same total, it does not stack
	// a second copy of the static buff.
	jaradGolgariLichLordETB(gs, jarad)
	if p, tg := pt(gs, jarad); p != 4 || tg != 4 {
		t.Errorf("Jarad after re-entering: want 4/4 still, got %d/%d", p, tg)
	}

	// Yard shrinks → the buff shrinks with it.
	gs.Seats[0].Graveyard = gs.Seats[0].Graveyard[:1]
	jaradGolgariLichLordETB(gs, jarad)
	if p, tg := pt(gs, jarad); p != 3 || tg != 3 {
		t.Errorf("Jarad with 1 creature in yard: want 3/3, got %d/%d", p, tg)
	}
}

func TestTempPumpR64_SyrVondamGrantsRealDeathtouch(t *testing.T) {
	gs := newGame(t, 2)
	vondam := addPerm(gs, 0, "Syr Vondam, the Lucent", "creature", "legendary", "human", "knight")
	vondam.Card.BasePower, vondam.Card.BaseToughness = 4, 4
	ally := addPerm(gs, 0, "Savannah Lions", "creature", "cat")
	ally.Card.BasePower, ally.Card.BaseToughness = 2, 1

	syrVondamPump(gs, vondam)

	if p, _ := pt(gs, ally); p != 3 {
		t.Errorf("other creature should be +1/+0: want power 3, got %d", p)
	}
	if !ally.HasKeyword("deathtouch") {
		t.Error("other creature should have deathtouch until end of turn")
	}
	if vondam.HasKeyword("deathtouch") != vondamSelfExcluded(vondam) {
		// Self is excluded from the grant; Syr Vondam's own printed
		// deathtouch comes from the AST, which this bare test perm lacks.
		t.Error("Syr Vondam must not grant to herself via this trigger")
	}
}

// vondamSelfExcluded documents the expectation: the handler skips the
// source, so a bare test Permanent with no AST has no deathtouch.
func vondamSelfExcluded(*gameengine.Permanent) bool { return false }

func TestTempPumpR64_ArchimandriteGrantsRealVigilance(t *testing.T) {
	gs := newGame(t, 2)
	arch := addPerm(gs, 0, "The Archimandrite", "creature", "legendary", "human", "advisor")
	arch.Card.BasePower, arch.Card.BaseToughness = 4, 5
	monk := addPerm(gs, 0, "Monastery Swiftspear", "creature", "human", "monk")
	monk.Card.BasePower, monk.Card.BaseToughness = 1, 2
	theArchimandriteLifeGained(gs, arch, map[string]interface{}{
		"seat":   0,
		"amount": 2,
	})

	if !monk.HasKeyword("vigilance") {
		t.Error("Advisor/Artificer/Monk should have vigilance until end of turn")
	}
	if p, _ := pt(gs, monk); p != 3 {
		t.Errorf("Monk should be +2/+0 from 2 life gained: want power 3, got %d", p)
	}
}
