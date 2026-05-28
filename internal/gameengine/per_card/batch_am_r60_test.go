package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// batch_am_r60_test.go — smoke tests for the 5 batch-AM handlers:
// Kroxa, Uro, Lurrus, Mizzix's Mastery, and the §400.7 / §110.2a
// reparenting fix to the existing Reanimate handler.

// -----------------------------------------------------------------------------
// Kroxa, Titan of Death's Hunger
// -----------------------------------------------------------------------------

func TestKroxa_ETBSacUnlessEscaped_HardCastSacrifices(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Hand = []*gameengine.Card{{Name: "Forest"}, {Name: "Mountain"}}
	kroxa := addPerm(gs, 0, "Kroxa, Titan of Death's Hunger", "creature", "legendary")
	// No escape_cast flag → hard cast → sac fires.

	gameengine.InvokeETBHook(gs, kroxa)

	// Kroxa sacrificed.
	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p == kroxa {
			stillOnBF = true
		}
	}
	if stillOnBF {
		t.Error("hard-cast Kroxa should be sacrificed on ETB")
	}
	// Seat 1 discarded 1.
	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("opponent should have discarded 1 (hand was 2); got hand=%d", len(gs.Seats[1].Hand))
	}
}

func TestKroxa_ETBSacUnlessEscaped_EscapedStaysOnBattlefield(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Hand = []*gameengine.Card{{Name: "Forest"}}
	kroxa := addPerm(gs, 0, "Kroxa, Titan of Death's Hunger", "creature", "legendary")
	kroxa.Flags["escape_cast"] = 1

	gameengine.InvokeETBHook(gs, kroxa)

	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == kroxa {
			stillOnBF = true
		}
	}
	if !stillOnBF {
		t.Error("escape-cast Kroxa should stay on battlefield (no sac trigger)")
	}
	// Discard still fires regardless of escape.
	if len(gs.Seats[1].Hand) != 0 {
		t.Errorf("opp should still discard on escape ETB; hand=%d", len(gs.Seats[1].Hand))
	}
}

func TestKroxa_UpkeepSacUnlessOppLostLife_NoLifeLossSacrifices(t *testing.T) {
	gs := newGame(t, 2)
	kroxa := addPerm(gs, 0, "Kroxa, Titan of Death's Hunger", "creature", "legendary")
	// Opp lost no life this turn.

	kroxaUpkeep(gs, kroxa, map[string]interface{}{"active_seat": 0})

	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == kroxa {
			stillOnBF = true
		}
	}
	if stillOnBF {
		t.Error("Kroxa should sac on upkeep when no opp lost life")
	}
}

func TestKroxa_UpkeepSacUnlessOppLostLife_OppDamagedSurvives(t *testing.T) {
	gs := newGame(t, 2)
	kroxa := addPerm(gs, 0, "Kroxa, Titan of Death's Hunger", "creature", "legendary")
	gs.Seats[1].Turn.LifeLost = 3

	kroxaUpkeep(gs, kroxa, map[string]interface{}{"active_seat": 0})

	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == kroxa {
			stillOnBF = true
		}
	}
	if !stillOnBF {
		t.Error("Kroxa should NOT sac when an opponent lost life this turn")
	}
}

// -----------------------------------------------------------------------------
// Uro, Titan of Nature's Wrath
// -----------------------------------------------------------------------------

func TestUro_ETBValueClauseGainLifeDrawLand(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40
	addLibrary(gs, 0, "DrawnCard")
	// Land in hand for the "may play extra land" clause.
	gs.Seats[0].Hand = []*gameengine.Card{{Name: "Forest", Owner: 0, Types: []string{"land"}}}
	uro := addPerm(gs, 0, "Uro, Titan of Nature's Wrath", "creature", "legendary")
	uro.Flags["escape_cast"] = 1 // escape so it survives the sac

	gameengine.InvokeETBHook(gs, uro)

	if gs.Seats[0].Life != 43 {
		t.Errorf("Uro should gain 3 life; life=%d", gs.Seats[0].Life)
	}
	// Drew DrawnCard → into hand. Note hand started with 1 land, gained 1 draw,
	// then placed the land onto battlefield → hand ends at 1 (the drawn card).
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("hand should have 1 card (Forest played, DrawnCard drawn); got %d", len(gs.Seats[0].Hand))
	}
	// Land on battlefield.
	landBF := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && cardHasType(p.Card, "land") {
			landBF++
		}
	}
	if landBF != 1 {
		t.Errorf("Forest should be on battlefield; land count=%d", landBF)
	}
}

func TestUro_AttackTriggerRunsValueClause(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 30
	addLibrary(gs, 0, "Card1")
	uro := addPerm(gs, 0, "Uro, Titan of Nature's Wrath", "creature", "legendary")
	uro.Flags["escape_cast"] = 1 // already on BF; flag stays

	uroAttack(gs, uro, map[string]interface{}{"attacker_perm": uro})

	if gs.Seats[0].Life != 33 {
		t.Errorf("attack should gain 3 life; life=%d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("attack should draw; hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Lurrus of the Dream-Den
// -----------------------------------------------------------------------------

func TestLurrus_ETBGrantsCastFromGraveyardOnLowCMCPermanents(t *testing.T) {
	gs := newGame(t, 2)
	// Graveyard mix: 1 creature CMC 1 (eligible), 1 creature CMC 3 (too big),
	// 1 instant CMC 1 (wrong type), 1 land CMC 0 (eligible by CMC but cast-illegal).
	smolCreature := &gameengine.Card{Name: "Mox Diamond", Owner: 0, Types: []string{"artifact", "cmc:0"}}
	bigCreature := &gameengine.Card{Name: "Big Beast", Owner: 0, Types: []string{"creature", "cmc:3"}}
	instant := &gameengine.Card{Name: "Brainstorm", Owner: 0, Types: []string{"instant", "cmc:1"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, smolCreature, bigCreature, instant)

	lurrus := addPerm(gs, 0, "Lurrus of the Dream-Den", "creature", "legendary")
	gameengine.InvokeETBHook(gs, lurrus)

	// Mox Diamond gets a grant; Big Beast does not; Brainstorm does not.
	if g := gameengine.GetZoneCastGrant(gs, smolCreature); g == nil {
		t.Error("Mox Diamond (CMC 0 artifact) should have a Lurrus grant")
	}
	if g := gameengine.GetZoneCastGrant(gs, bigCreature); g != nil {
		t.Error("Big Beast (CMC 3) should NOT have a Lurrus grant")
	}
	if g := gameengine.GetZoneCastGrant(gs, instant); g != nil {
		t.Error("Brainstorm (instant — not a permanent) should NOT have a Lurrus grant")
	}
}

// -----------------------------------------------------------------------------
// Mizzix's Mastery
// -----------------------------------------------------------------------------

func TestMizzixsMastery_RegistersFreeCastGrantsOnAllInstantsAndSorceries(t *testing.T) {
	gs := newGame(t, 2)
	instant := &gameengine.Card{Name: "Counterspell", Owner: 0, Types: []string{"instant", "cmc:2"}}
	sorcery := &gameengine.Card{Name: "Wrath of God", Owner: 0, Types: []string{"sorcery", "cmc:4"}}
	creature := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature", "cmc:2"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, instant, sorcery, creature)

	item := &gameengine.StackItem{Card: &gameengine.Card{Name: "Mizzix's Mastery"}, Controller: 0}
	mizzixsMasteryResolve(gs, item)

	if g := gameengine.GetZoneCastGrant(gs, instant); g == nil {
		t.Error("Counterspell should have a Mizzix grant")
	}
	if g := gameengine.GetZoneCastGrant(gs, sorcery); g == nil {
		t.Error("Wrath (sorcery) should have a Mizzix grant (overload covers any CMC)")
	}
	if g := gameengine.GetZoneCastGrant(gs, creature); g != nil {
		t.Error("Bear (creature) should NOT have a Mizzix grant")
	}
	// Grants are controller-restricted + exile-on-resolve.
	if g := gameengine.GetZoneCastGrant(gs, instant); g != nil {
		if g.RequireController != 0 {
			t.Errorf("Mizzix grant should restrict to seat 0; got %d", g.RequireController)
		}
		if !g.ExileOnResolve {
			t.Error("Mizzix grant should exile-on-resolve per 'Then exile it' oracle")
		}
		if g.ManaCost != 0 {
			t.Errorf("Mizzix grant should be free-cast; ManaCost=%d", g.ManaCost)
		}
	}
}

// -----------------------------------------------------------------------------
// Reanimate (§400.7 / §110.2a reparenting fix)
// -----------------------------------------------------------------------------

func TestReanimate_R60_ReparentsReanimatedCreatureToCaster(t *testing.T) {
	// Pre-r60 the reanimateResolve in commander_staples.go moved the
	// creature to the OWNER's battlefield via MoveCard but didn't
	// reparent to the caster. Result: opp owned AND controlled their
	// own creature back. The fix runs the same reparenting pattern
	// used by Nicol Bolas's −4 ultimate.
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40
	bigCreature := &gameengine.Card{Name: "Griselbrand", Owner: 1, Types: []string{"creature", "legendary", "cmc:8"}}
	bigCreature.BasePower = 7
	bigCreature.BaseToughness = 7
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, bigCreature)

	item := &gameengine.StackItem{Card: &gameengine.Card{Name: "Reanimate"}, Controller: 0}
	reanimateResolve(gs, item)

	// Should be on seat 0's battlefield, controlled by seat 0.
	foundOnSeat0 := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == bigCreature {
			foundOnSeat0 = true
			if p.Controller != 0 {
				t.Errorf("reanimated creature should be controlled by caster (seat 0); got %d", p.Controller)
			}
		}
	}
	if !foundOnSeat0 {
		t.Errorf("reanimated creature should be on caster's battlefield (seat 0)")
	}
	foundOnSeat1 := false
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.Card == bigCreature {
			foundOnSeat1 = true
		}
	}
	if foundOnSeat1 {
		t.Error("reanimated creature should NOT remain on owner's battlefield (seat 1) — reparent fix")
	}
	// Life paid = 8 (Griselbrand's CMC).
	if gs.Seats[0].Life != 32 {
		t.Errorf("caster should pay life=CMC=8; life=%d", gs.Seats[0].Life)
	}
}
