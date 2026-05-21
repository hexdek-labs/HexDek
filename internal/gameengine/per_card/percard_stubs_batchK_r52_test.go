package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// dev/percard-stubs-batchK-r52 — 10 fresh stub ports.
// One regression test per port.
// ---------------------------------------------------------------------------

// 1. Urabrask, Heretic Praetor — opponent's first draw on their turn is
//    replaced with exile-from-top instead.
func TestStubsBatchK_Urabrask_OppDrawReplacedWithExile(t *testing.T) {
	gs := newGame(t, 2)
	urabrask := addPerm(gs, 0, "Urabrask, Heretic Praetor", "creature", "legendary")
	urabrask.Card.BasePower = 4
	urabrask.Card.BaseToughness = 4
	addLibrary(gs, 1, "Forbidden Top", "Forest A", "Forest B")

	// Opponent's upkeep — registers the replacement.
	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{
		"active_seat": 1,
	})

	// Opponent attempts to draw.
	count, cancelled := gameengine.FireDrawEvent(gs, 1, urabrask)
	if !cancelled {
		t.Errorf("expected opp's draw to be cancelled by Urabrask, got count=%d cancelled=%v", count, cancelled)
	}
	// Top of opp's library should now be in their exile zone.
	if len(gs.Seats[1].Exile) == 0 || gs.Seats[1].Exile[0].DisplayName() != "Forbidden Top" {
		t.Errorf("expected Forbidden Top in opp's exile; exile=%v", cardNames(gs.Seats[1].Exile))
	}
}

// 2. Noctis, Prince of Lucis — finality counter stamped on artifact
//    permanents that ETB with the "cast_via_noctis" tag.
func TestStubsBatchK_Noctis_FinalityCounterOnGraveyardCast(t *testing.T) {
	gs := newGame(t, 2)
	noctis := addPerm(gs, 0, "Noctis, Prince of Lucis", "creature", "legendary")
	noctis.Card.BasePower = 4
	noctis.Card.BaseToughness = 4
	gameengine.InvokeETBHook(gs, noctis)

	// Simulate cast-via-noctis: stamp the tag on a fresh artifact and
	// let it ETB.
	relic := addPerm(gs, 0, "Tagged Relic", "artifact")
	relic.Card.Types = append(relic.Card.Types, "cast_via_noctis")
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            relic,
		"controller_seat": 0,
	})
	if relic.Counters["finality"] != 1 {
		t.Errorf("expected finality counter, got %v", relic.Counters)
	}
	// The tag should be consumed.
	for _, tag := range relic.Card.Types {
		if tag == "cast_via_noctis" {
			t.Errorf("expected cast_via_noctis tag consumed, got types=%v", relic.Card.Types)
		}
	}
}

// 3. Galea, Kindler of Hope — Equipment that ETBs with cast_via_galea tag
//    is auto-attached to best friendly creature.
func TestStubsBatchK_Galea_AutoAttachesEquipmentOnGaleaCast(t *testing.T) {
	gs := newGame(t, 2)
	galea := addPerm(gs, 0, "Galea, Kindler of Hope", "creature", "legendary")
	galea.Card.BasePower = 4
	galea.Card.BaseToughness = 4
	gameengine.InvokeETBHook(gs, galea)

	target := addPerm(gs, 0, "Hulking Brute", "creature")
	target.Card.BasePower = 5
	target.Card.BaseToughness = 5

	equip := addPerm(gs, 0, "Stoneforge Blade", "artifact", "equipment")
	equip.Card.Types = append(equip.Card.Types, "cast_via_galea")
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            equip,
		"controller_seat": 0,
	})
	if equip.AttachedTo != target {
		t.Errorf("expected equipment attached to hulking brute, got %v", equip.AttachedTo)
	}
}

// 4. The Master, Multiplied — own creature tokens get the
//    "trigger_protected" stamp at ETB and refresh on new tokens.
func TestStubsBatchK_TheMasterMultiplied_StampsOwnTokens(t *testing.T) {
	gs := newGame(t, 2)
	tok := addPerm(gs, 0, "Spawn Token", "creature", "token", "spawn")
	master := addPerm(gs, 0, "The Master, Multiplied", "creature", "legendary")
	master.Card.BasePower = 4
	master.Card.BaseToughness = 4
	gameengine.InvokeETBHook(gs, master)
	if tok.Flags["master_multiplied_trigger_protected"] != 1 {
		t.Errorf("expected existing token stamped, got flags=%v", tok.Flags)
	}
	// New token enters → refresh should stamp it.
	newTok := addPerm(gs, 0, "Soldier Token", "creature", "token", "soldier")
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            newTok,
		"controller_seat": 0,
	})
	if newTok.Flags["master_multiplied_trigger_protected"] != 1 {
		t.Errorf("expected new token stamped, got flags=%v", newTok.Flags)
	}
}

// 5. Alpharael, Stonechosen — ETB stamps Ward—Discard at random flags.
func TestStubsBatchK_Alpharael_WardDiscardRandomStamp(t *testing.T) {
	gs := newGame(t, 2)
	alpharael := addPerm(gs, 0, "Alpharael, Stonechosen", "creature", "legendary")
	gameengine.InvokeETBHook(gs, alpharael)
	if alpharael.Flags["ward"] != 1 {
		t.Errorf("expected ward flag set, got %v", alpharael.Flags)
	}
	if alpharael.Flags["ward_discard_random"] != 1 {
		t.Errorf("expected ward_discard_random flag set, got %v", alpharael.Flags)
	}
}

// 6. Infinite Guideline Station — Station activated ability taps target
//    creature and adds charge counters equal to its power.
func TestStubsBatchK_InfiniteGuidelineStation_StationActivate(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main1"
	station := addPerm(gs, 0, "Infinite Guideline Station", "artifact")
	tapTarget := addPerm(gs, 0, "Strong Worker", "creature")
	tapTarget.Card.BasePower = 5
	tapTarget.Card.BaseToughness = 5

	gameengine.InvokeActivatedHook(gs, station, 0, nil)

	if !tapTarget.Tapped {
		t.Errorf("expected tap target tapped")
	}
	if station.Counters["charge"] != 5 {
		t.Errorf("expected 5 charge counters on station, got %v", station.Counters)
	}
}

// 7. Cecily, Haunted Mage — when hand size ≥11 at attack, the
//    cecily_free_is_cast_pending seat flag is set.
func TestStubsBatchK_Cecily_FreeCastFlagWhenHandLarge(t *testing.T) {
	gs := newGame(t, 2)
	cecily := addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary")
	cecily.Card.BasePower = 4
	cecily.Card.BaseToughness = 4
	// Stack 12 cards in hand (post-draw will be 12 after the draw).
	for i := 0; i < 11; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{Name: "Filler"})
	}
	addLibrary(gs, 0, "Topdeck")
	gs.Seats[0].Life = 30

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": cecily,
	})

	if gs.Seats[0].Flags["cecily_free_is_cast_pending"] != 1 {
		t.Errorf("expected free-cast pending flag, got %v", gs.Seats[0].Flags)
	}
}

// 8. Zethi, Arcane Blademaster — exiles instants from the controller's
//    graveyard equal to kick count, tagging each with zethi_kick_counter.
func TestStubsBatchK_Zethi_ETBExilesInstantsTagged(t *testing.T) {
	gs := newGame(t, 2)
	zethi := addPerm(gs, 0, "Zethi, Arcane Blademaster", "creature", "legendary")
	zethi.Card.BasePower = 3
	zethi.Card.BaseToughness = 3
	zethi.Flags["multikick_count"] = 2
	bolt := &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}}
	cs := &gameengine.Card{Name: "Counterspell", Types: []string{"instant"}}
	bear := &gameengine.Card{Name: "Bear", Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, cs, bear)

	gameengine.InvokeETBHook(gs, zethi)

	exiled := 0
	for _, c := range gs.Seats[0].Exile {
		if c == nil {
			continue
		}
		hasTag := false
		for _, t := range c.Types {
			if t == "zethi_kick_counter" {
				hasTag = true
			}
		}
		if hasTag {
			exiled++
		}
	}
	if exiled != 2 {
		t.Errorf("expected 2 exiled+tagged instants, got %d", exiled)
	}
	// Creature should NOT be exiled.
	for _, c := range gs.Seats[0].Exile {
		if c == bear {
			t.Errorf("Bear should not be in exile")
		}
	}
}

// 9. Toph, Hardheaded Teacher — earthbend on each spell cast; Lesson
//    bonus adds an extra +1/+1 to the freshly earthbent land.
func TestStubsBatchK_TophHardheaded_SpellCastEarthbendAndLessonBonus(t *testing.T) {
	gs := newGame(t, 2)
	toph := addPerm(gs, 0, "Toph, Hardheaded Teacher", "creature", "legendary")
	toph.Card.BasePower = 3
	toph.Card.BaseToughness = 3
	land := addPerm(gs, 0, "Plains", "land")
	// Pre-stamp the earthbent flag so Toph's spell_cast trigger can
	// find it and apply the Lesson bonus.
	land.Flags["earthbent"] = 1
	land.Timestamp = gs.NextTimestamp()

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Test Lesson", Types: []string{"sorcery", "lesson"}},
	})

	if land.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 counter on earthbent land for Lesson cast, got %v", land.Counters)
	}
}

// 10. Rendmaw, Creaking Nest — multi-type land ETB fires the
//     bird-token spawn for each player.
func TestStubsBatchK_Rendmaw_MultiTypeLandETBSpawnsBirds(t *testing.T) {
	gs := newGame(t, 2)
	rendmaw := addPerm(gs, 0, "Rendmaw, Creaking Nest", "creature", "legendary")
	rendmaw.Card.BasePower = 5
	rendmaw.Card.BaseToughness = 5

	// A Dryad-Arbor-style land+creature multi-type land entering.
	multiLand := addPerm(gs, 0, "Dryad Arbor", "land", "creature")

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            multiLand,
		"controller_seat": 0,
	})

	// Each player should have a fresh Bird token.
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		hasBird := false
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.Card.DisplayName() == "Bird Token" {
				hasBird = true
			}
		}
		if !hasBird {
			t.Errorf("expected Bird Token on seat %d's battlefield", seatIdx)
		}
	}
}
