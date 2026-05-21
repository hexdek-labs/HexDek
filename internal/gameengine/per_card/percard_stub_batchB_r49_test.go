package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R49 stub-batch B ports — ten gen_*.go pure-stub / partial-ETB handlers
// across the alphabet, complementing batch A (alphabetical first-half).
//
// Picks (oracle-text complexity, triggered + activated + multi-zone):
//   - Kwain, Itinerant Meddler         multi-player group-hug draw + life
//   - Oona, Queen of the Fae           {X}{U/B} mill + color-match tokens
//   - Silvar, Devourer of the Free     Partner-tutor ETB + sac-Human grow
//   - June, Bounty Hunter              draw-conditional unblockable + Clue sac
//   - The Earth King                   4/4 Bear ETB + attack-pow≥4 ramp
//   - Rat King, Verminister            Disappear end-step + sac-3-rats recursion
//   - The Wandering Minstrel           5-Town combat-begin + 5C team pump
//   - The Destined White Mage          lifegain counter (party-scaled) + lifelink grant
//   - Zidane, Tantalus Thief           ETB control-steal-UEOT
//   - Minwu, White Mage                fix: only Clerics get +1/+1 on lifegain

// ---------------------------------------------------------------------------
// Kwain
// ---------------------------------------------------------------------------

func TestKwain_EachPlayerDrawsAndDrewSeatsGainLife(t *testing.T) {
	gs := newGame(t, 3)
	kwain := addPerm(gs, 0, "Kwain, Itinerant Meddler", "creature", "legendary")
	addLibrary(gs, 0, "A1", "A2")
	addLibrary(gs, 1, "B1")
	// Seat 2: empty library.

	startLife := []int{gs.Seats[0].Life, gs.Seats[1].Life, gs.Seats[2].Life}

	kwainItinerantMeddlerActivate(gs, kwain, 0, nil)

	if !kwain.Tapped {
		t.Error("Kwain should be tapped after activation")
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("seat 0 hand = %d, want 1", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("seat 1 hand = %d, want 1", len(gs.Seats[1].Hand))
	}
	if len(gs.Seats[2].Hand) != 0 {
		t.Errorf("seat 2 hand = %d, want 0", len(gs.Seats[2].Hand))
	}
	if gs.Seats[0].Life != startLife[0]+1 {
		t.Errorf("seat 0 life delta = %d, want +1", gs.Seats[0].Life-startLife[0])
	}
	if gs.Seats[1].Life != startLife[1]+1 {
		t.Errorf("seat 1 life delta = %d, want +1", gs.Seats[1].Life-startLife[1])
	}
	if gs.Seats[2].Life != startLife[2] {
		t.Errorf("seat 2 should NOT have gained life (didn't draw); delta=%d", gs.Seats[2].Life-startLife[2])
	}
}

func TestKwain_AlreadyTappedFails(t *testing.T) {
	gs := newGame(t, 2)
	kwain := addPerm(gs, 0, "Kwain, Itinerant Meddler", "creature")
	kwain.Tapped = true
	addLibrary(gs, 0, "A")

	kwainItinerantMeddlerActivate(gs, kwain, 0, nil)

	if len(gs.Seats[0].Hand) != 0 {
		t.Error("should not draw when already tapped")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed event")
	}
}

// ---------------------------------------------------------------------------
// Oona
// ---------------------------------------------------------------------------

func TestOona_ExilesXAndSpawnsColorMatchedFaeries(t *testing.T) {
	gs := newGame(t, 2)
	oona := addPerm(gs, 0, "Oona, Queen of the Fae", "creature", "legendary")
	// Seed seat 1's library: 3 blues, 2 reds, 1 colorless.
	for i := 0; i < 3; i++ {
		c := &gameengine.Card{Name: "U" + string(rune('1'+i)), Owner: 1, Colors: []string{"U"}}
		gs.Seats[1].Library = append(gs.Seats[1].Library, c)
	}
	for i := 0; i < 2; i++ {
		c := &gameengine.Card{Name: "R" + string(rune('1'+i)), Owner: 1, Colors: []string{"R"}}
		gs.Seats[1].Library = append(gs.Seats[1].Library, c)
	}
	gs.Seats[1].Library = append(gs.Seats[1].Library,
		&gameengine.Card{Name: "C1", Owner: 1})

	preBF := len(gs.Seats[0].Battlefield)

	oonaQueenOfTheFaeActivate(gs, oona, 0, map[string]interface{}{
		"x": 6, // exile all 6 cards
	})

	if len(gs.Seats[1].Library) != 0 {
		t.Errorf("library should be empty, got %d", len(gs.Seats[1].Library))
	}
	if len(gs.Seats[1].Exile) != 6 {
		t.Errorf("expected 6 exiled, got %d", len(gs.Seats[1].Exile))
	}
	// Color choice should be U (3 cards). Tokens spawned = 3.
	tokens := len(gs.Seats[0].Battlefield) - preBF
	if tokens != 3 {
		t.Errorf("expected 3 Faerie tokens (matches U), got %d", tokens)
	}
}

func TestOona_NoOpponentWithLibraryFails(t *testing.T) {
	gs := newGame(t, 2)
	oona := addPerm(gs, 0, "Oona, Queen of the Fae", "creature")

	oonaQueenOfTheFaeActivate(gs, oona, 0, map[string]interface{}{"x": 5})

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed when no opponent has a library")
	}
}

// ---------------------------------------------------------------------------
// Silvar
// ---------------------------------------------------------------------------

func TestSilvar_PartnerETBTutorsTrynn(t *testing.T) {
	gs := newGame(t, 2)
	silvar := addPerm(gs, 0, "Silvar, Devourer of the Free", "creature", "legendary")
	trynn := &gameengine.Card{Name: "Trynn, Champion of Freedom", Owner: 0, Types: []string{"creature", "legendary"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, trynn,
		&gameengine.Card{Name: "Filler", Owner: 0})

	silvarPartnerETB(gs, silvar)

	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == trynn {
			foundInHand = true
			break
		}
	}
	if !foundInHand {
		t.Errorf("Trynn should be in hand after partner tutor; hand=%v", gs.Seats[0].Hand)
	}
}

func TestSilvar_SacHumanGivesCounterAndIndestructibleUEOT(t *testing.T) {
	gs := newGame(t, 2)
	silvar := addPerm(gs, 0, "Silvar, Devourer of the Free", "creature", "legendary")
	human := addPerm(gs, 0, "Soldier", "creature", "human")
	human.Card.BasePower = 1
	human.Card.BaseToughness = 1

	silvarDevourerOfTheFreeActivate(gs, silvar, 0, nil)

	if silvar.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 counter on Silvar, got %d", silvar.Counters["+1/+1"])
	}
	if silvar.Flags["kw:indestructible"] != 1 {
		t.Error("Silvar should have indestructible UEOT flag set")
	}
	// Human should be off the battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == human {
			t.Errorf("Human should have been sacrificed")
		}
	}
	if len(gs.DelayedTriggers) < 1 {
		t.Error("expected next_end_step delayed trigger for indestructible cleanup")
	}
}

func TestSilvar_NoHumanFails(t *testing.T) {
	gs := newGame(t, 2)
	silvar := addPerm(gs, 0, "Silvar, Devourer of the Free", "creature")
	// A non-Human creature shouldn't be eligible.
	addPerm(gs, 0, "Goblin", "creature", "goblin")

	silvarDevourerOfTheFreeActivate(gs, silvar, 0, nil)

	if silvar.Counters["+1/+1"] != 0 {
		t.Error("no counter should land without a Human to sac")
	}
}

// ---------------------------------------------------------------------------
// June, Bounty Hunter
// ---------------------------------------------------------------------------

func TestJune_DrawnTwoUnblockableFlagSet(t *testing.T) {
	gs := newGame(t, 2)
	june := addPerm(gs, 0, "June, Bounty Hunter", "creature")
	gs.Seats[0].Turn.CardsDrawn = 2

	juneBountyHunterETB(gs, june)

	if june.Flags["unblockable"] != 1 {
		t.Errorf("June should be unblockable with 2 cards drawn; flags=%v", june.Flags)
	}
}

func TestJune_DrawnZeroNoUnblockable(t *testing.T) {
	gs := newGame(t, 2)
	june := addPerm(gs, 0, "June, Bounty Hunter", "creature")

	juneBountyHunterETB(gs, june)

	if june.Flags["unblockable"] != 0 {
		t.Errorf("June should NOT be unblockable; flags=%v", june.Flags)
	}
}

func TestJune_SacCreatureSpawnsClue(t *testing.T) {
	gs := newGame(t, 2)
	june := addPerm(gs, 0, "June, Bounty Hunter", "creature")
	gs.Seats[0].ManaPool = 5
	addPerm(gs, 0, "Servo", "creature")
	preBF := len(gs.Seats[0].Battlefield)

	juneBountyHunterActivate(gs, june, 0, map[string]interface{}{
		"active_seat": 0,
	})

	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("mana pool should drop by 1, got %d", gs.Seats[0].ManaPool)
	}
	// One less perm (Servo sac'd) plus one more (Clue) = same count.
	// Verify a Clue is in play.
	foundClue := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && cardHasType(p.Card, "clue") {
			foundClue = true
		}
	}
	if !foundClue {
		t.Errorf("expected a Clue token; bf names=%v", bfNames(gs.Seats[0].Battlefield))
	}
	_ = preBF
}

func TestJune_OpponentTurnBlocksActivation(t *testing.T) {
	gs := newGame(t, 2)
	june := addPerm(gs, 0, "June, Bounty Hunter", "creature")
	gs.Seats[0].ManaPool = 5
	addPerm(gs, 0, "Servo", "creature")

	juneBountyHunterActivate(gs, june, 0, map[string]interface{}{
		"active_seat": 1, // opponent's turn
	})

	if gs.Seats[0].ManaPool != 5 {
		t.Error("activation should have been blocked; mana spent")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed: not_controllers_turn")
	}
}

// ---------------------------------------------------------------------------
// The Earth King
// ---------------------------------------------------------------------------

func TestEarthKing_ETBSpawns4_4GreenBear(t *testing.T) {
	gs := newGame(t, 2)
	ek := addPerm(gs, 0, "The Earth King", "creature", "legendary")

	preBF := len(gs.Seats[0].Battlefield)
	theEarthKingETB(gs, ek)

	foundBear := false
	for _, p := range gs.Seats[0].Battlefield[preBF:] {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "bear") {
			foundBear = true
			if p.Card.BasePower != 4 || p.Card.BaseToughness != 4 {
				t.Errorf("Bear should be 4/4, got %d/%d", p.Card.BasePower, p.Card.BaseToughness)
			}
		}
	}
	if !foundBear {
		t.Errorf("expected a Bear token; bf=%v", bfNames(gs.Seats[0].Battlefield))
	}
}

func TestEarthKing_AttackTriggerRampsBasics(t *testing.T) {
	gs := newGame(t, 2)
	ek := addPerm(gs, 0, "The Earth King", "creature", "legendary")
	// Big attacker
	big := addPerm(gs, 0, "Rhox", "creature")
	big.Card.BasePower = 5
	big.Card.BaseToughness = 5
	big.Flags["attacking"] = 1

	// Stack basics + non-basics in library.
	forest1 := &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"basic", "land", "forest"}}
	forest2 := &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"basic", "land", "forest"}}
	nonbasic := &gameengine.Card{Name: "Stomping Ground", Owner: 0, Types: []string{"land"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, forest1, nonbasic, forest2)

	theEarthKingAttackTrigger(gs, ek, map[string]interface{}{
		"attacker_seat": 0,
		"attacker_perm": big,
	})

	// One attacker w/ power 5 → search up to 1 basic. The non-basic
	// is skipped, the first Forest lands tapped.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Forest" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a Forest on battlefield after ramp; bf=%v",
			bfNames(gs.Seats[0].Battlefield))
	}
}

func TestEarthKing_DedupedWithinSameAttackStep(t *testing.T) {
	gs := newGame(t, 2)
	ek := addPerm(gs, 0, "The Earth King", "creature", "legendary")
	big := addPerm(gs, 0, "Rhox", "creature")
	big.Card.BasePower = 5
	big.Card.BaseToughness = 5
	big.Flags["attacking"] = 1
	for i := 0; i < 3; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"basic", "land"}})
	}

	theEarthKingAttackTrigger(gs, ek, map[string]interface{}{
		"attacker_seat": 0,
		"attacker_perm": big,
	})
	preBF := len(gs.Seats[0].Battlefield)
	// Second attacker in same step should be deduped.
	theEarthKingAttackTrigger(gs, ek, map[string]interface{}{
		"attacker_seat": 0,
		"attacker_perm": big,
	})
	if len(gs.Seats[0].Battlefield) != preBF {
		t.Errorf("second attack in same step should be deduped; before=%d after=%d",
			preBF, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Rat King, Verminister
// ---------------------------------------------------------------------------

func TestRatKing_DisappearEndStepSpawnsRatAndAddsCounter(t *testing.T) {
	gs := newGame(t, 2)
	rk := addPerm(gs, 0, "Rat King, Verminister", "creature", "legendary")
	gs.Seats[0].Turn.PermanentsLeft = 1
	preBF := len(gs.Seats[0].Battlefield)

	ratKingDisappearEndStep(gs, rk, map[string]interface{}{
		"active_seat": 0,
	})

	if rk.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 counter on Rat King, got %d", rk.Counters["+1/+1"])
	}
	if len(gs.Seats[0].Battlefield) != preBF+1 {
		t.Errorf("expected a Rat token spawned")
	}
}

func TestRatKing_DisappearSkipsIfNoPermsLeft(t *testing.T) {
	gs := newGame(t, 2)
	rk := addPerm(gs, 0, "Rat King, Verminister", "creature")
	gs.Seats[0].Turn.PermanentsLeft = 0
	preBF := len(gs.Seats[0].Battlefield)

	ratKingDisappearEndStep(gs, rk, map[string]interface{}{
		"active_seat": 0,
	})

	if rk.Counters["+1/+1"] != 0 {
		t.Error("no counter should land with PermanentsLeft=0")
	}
	if len(gs.Seats[0].Battlefield) != preBF {
		t.Error("no token should spawn with PermanentsLeft=0")
	}
}

func TestRatKing_SacThreeRatsReturnsSameName(t *testing.T) {
	gs := newGame(t, 2)
	rk := addPerm(gs, 0, "Rat King, Verminister", "creature", "legendary", "rat")
	// Three rats to sacrifice.
	for i := 0; i < 3; i++ {
		p := addPerm(gs, 0, "Rat", "creature", "rat")
		p.Card.BasePower = 1
		p.Card.BaseToughness = 1
	}
	// Two same-name creatures in graveyard + one other.
	gob1 := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"creature", "goblin"}, BasePower: 2, BaseToughness: 2}
	gob2 := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"creature", "goblin"}, BasePower: 2, BaseToughness: 2}
	other := &gameengine.Card{Name: "Squirrel", Owner: 0, Types: []string{"creature", "squirrel"}, BasePower: 1, BaseToughness: 1}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gob1, gob2, other)

	ratKingVerministerActivate(gs, rk, 0, nil)

	if !rk.Tapped {
		t.Error("Rat King should be tapped")
	}
	// Both Goblins should be on the battlefield, Squirrel stays in gy.
	goblinsBF := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Goblin" {
			goblinsBF++
		}
	}
	if goblinsBF != 2 {
		t.Errorf("expected 2 Goblins returned, got %d", goblinsBF)
	}
	stillInGY := 0
	for _, c := range gs.Seats[0].Graveyard {
		if c != nil && c.DisplayName() == "Goblin" {
			stillInGY++
		}
	}
	if stillInGY != 0 {
		t.Errorf("expected no Goblins left in graveyard, got %d", stillInGY)
	}
}

func TestRatKing_NotEnoughRatsFails(t *testing.T) {
	gs := newGame(t, 2)
	rk := addPerm(gs, 0, "Rat King, Verminister", "creature", "rat")
	addPerm(gs, 0, "Rat", "creature", "rat")
	// Only 2 rats total (including Rat King).

	ratKingVerministerActivate(gs, rk, 0, nil)

	if rk.Tapped {
		t.Error("should not have tapped when activation fails")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed: not_enough_rats")
	}
}

// ---------------------------------------------------------------------------
// The Wandering Minstrel
// ---------------------------------------------------------------------------

func TestWanderingMinstrel_BalladTriggersAt5Towns(t *testing.T) {
	gs := newGame(t, 2)
	wm := addPerm(gs, 0, "The Wandering Minstrel", "creature", "legendary")
	for i := 0; i < 5; i++ {
		t := addPerm(gs, 0, "Town", "land", "town")
		_ = t
	}
	preBF := len(gs.Seats[0].Battlefield)

	theWanderingMinstrelCombatBegin(gs, wm, map[string]interface{}{
		"active_seat": 0,
	})

	foundElem := false
	for _, p := range gs.Seats[0].Battlefield[preBF:] {
		if p != nil && p.Card != nil && cardHasType(p.Card, "elemental") {
			foundElem = true
		}
	}
	if !foundElem {
		t.Errorf("expected an Elemental token at 5 Towns; bf=%v",
			bfNames(gs.Seats[0].Battlefield))
	}
}

func TestWanderingMinstrel_BalladSkipsUnder5Towns(t *testing.T) {
	gs := newGame(t, 2)
	wm := addPerm(gs, 0, "The Wandering Minstrel", "creature")
	for i := 0; i < 4; i++ {
		addPerm(gs, 0, "Town", "land", "town")
	}
	preBF := len(gs.Seats[0].Battlefield)

	theWanderingMinstrelCombatBegin(gs, wm, map[string]interface{}{
		"active_seat": 0,
	})

	if len(gs.Seats[0].Battlefield) != preBF {
		t.Error("ballad should not trigger under 5 Towns")
	}
}

func TestWanderingMinstrel_ActivatedPumpsOthersByTownCount(t *testing.T) {
	gs := newGame(t, 2)
	wm := addPerm(gs, 0, "The Wandering Minstrel", "creature")
	other := addPerm(gs, 0, "Servo", "creature")
	for i := 0; i < 3; i++ {
		addPerm(gs, 0, "Town", "land", "town")
	}
	gs.Seats[0].ManaPool = 8

	preMods := len(other.Modifications)
	theWanderingMinstrelActivate(gs, wm, 0, nil)

	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana spent fully, got %d", gs.Seats[0].ManaPool)
	}
	if len(other.Modifications) != preMods+1 {
		t.Errorf("expected 1 Modification appended on other creature, got delta=%d",
			len(other.Modifications)-preMods)
	}
	if other.Modifications[len(other.Modifications)-1].Power != 3 {
		t.Errorf("expected +3 power buff (3 Towns), got %+v",
			other.Modifications[len(other.Modifications)-1])
	}
	// Source itself should NOT be pumped (only "other creatures").
	for _, m := range wm.Modifications {
		if m.Duration == "until_end_of_turn" {
			t.Error("source should not pump itself")
		}
	}
}

// ---------------------------------------------------------------------------
// The Destined White Mage
// ---------------------------------------------------------------------------

func TestDestinedWhiteMage_LifegainAddsOneCounterWithoutFullParty(t *testing.T) {
	gs := newGame(t, 2)
	dwm := addPerm(gs, 0, "The Destined White Mage", "creature", "legendary")
	target := addPerm(gs, 0, "Soldier", "creature")
	target.Card.BasePower = 2
	target.Card.BaseToughness = 2

	theDestinedWhiteMageTrigger(gs, dwm, map[string]interface{}{
		"seat": 0,
	})

	if target.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 counter without full party, got %d", target.Counters["+1/+1"])
	}
}

func TestDestinedWhiteMage_LifegainAddsThreeWithFullParty(t *testing.T) {
	gs := newGame(t, 2)
	dwm := addPerm(gs, 0, "The Destined White Mage", "creature", "legendary")
	addPerm(gs, 0, "C", "creature", "cleric")
	addPerm(gs, 0, "R", "creature", "rogue")
	addPerm(gs, 0, "W", "creature", "warrior")
	addPerm(gs, 0, "Z", "creature", "wizard")
	// Best target = strongest creature; make Wizard the biggest.
	wiz := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	wiz.Card.BasePower = 5
	wiz.Card.BaseToughness = 5

	theDestinedWhiteMageTrigger(gs, dwm, map[string]interface{}{
		"seat": 0,
	})

	if wiz.Counters["+1/+1"] != 3 {
		t.Errorf("expected 3 counters with full party, got %d", wiz.Counters["+1/+1"])
	}
}

func TestDestinedWhiteMage_GrantLifelink(t *testing.T) {
	gs := newGame(t, 2)
	dwm := addPerm(gs, 0, "The Destined White Mage", "creature")
	other := addPerm(gs, 0, "Soldier", "creature")
	other.Card.BasePower = 4
	other.Card.BaseToughness = 4
	gs.Seats[0].ManaPool = 1

	theDestinedWhiteMageActivate(gs, dwm, 0, nil)

	if other.Flags["kw:lifelink"] != 1 {
		t.Error("expected lifelink grant on other creature")
	}
	if !dwm.Tapped {
		t.Error("source should be tapped after activation")
	}
	if len(gs.DelayedTriggers) < 1 {
		t.Error("expected end-of-turn cleanup trigger")
	}
}

// ---------------------------------------------------------------------------
// Zidane, Tantalus Thief
// ---------------------------------------------------------------------------

func TestZidane_ETBStealsHighestPowerOpponentCreature(t *testing.T) {
	gs := newGame(t, 2)
	zidane := addPerm(gs, 0, "Zidane, Tantalus Thief", "creature", "legendary")
	weak := addPerm(gs, 1, "Goblin", "creature")
	weak.Card.BasePower = 1
	weak.Card.BaseToughness = 1
	strong := addPerm(gs, 1, "Dragon", "creature")
	strong.Card.BasePower = 5
	strong.Card.BaseToughness = 5
	strong.Tapped = true

	zidaneTantalusThiefETB(gs, zidane)

	// Strong should be on seat 0's battlefield.
	onSeat0 := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == strong {
			onSeat0 = true
		}
	}
	if !onSeat0 {
		t.Error("Dragon should have been stolen to seat 0")
	}
	if strong.Tapped {
		t.Error("stolen creature should be untapped")
	}
	if strong.Flags["kw:lifelink"] != 1 || strong.Flags["kw:haste"] != 1 {
		t.Errorf("expected lifelink+haste grant; flags=%v", strong.Flags)
	}
	// Weak should still be on seat 1.
	for _, p := range gs.Seats[1].Battlefield {
		if p == strong {
			t.Error("strong should no longer be on seat 1")
		}
	}
}

func TestZidane_ETBWithNoOpponentCreaturesEmitsNoTarget(t *testing.T) {
	gs := newGame(t, 2)
	zidane := addPerm(gs, 0, "Zidane, Tantalus Thief", "creature")

	zidaneTantalusThiefETB(gs, zidane)

	if hasEvent(gs, "per_card_partial") < 1 {
		t.Error("expected per_card_partial breadcrumb")
	}
}

// ---------------------------------------------------------------------------
// Minwu, White Mage
// ---------------------------------------------------------------------------

func TestMinwu_LifegainOnlyBuffsClerics(t *testing.T) {
	gs := newGame(t, 2)
	minwu := addPerm(gs, 0, "Minwu, White Mage", "creature", "legendary")
	cleric := addPerm(gs, 0, "Priest", "creature", "cleric")
	cleric.Card.BasePower = 1
	cleric.Card.BaseToughness = 1
	soldier := addPerm(gs, 0, "Soldier", "creature", "human")
	soldier.Card.BasePower = 2
	soldier.Card.BaseToughness = 2

	minwuWhiteMageTrigger(gs, minwu, map[string]interface{}{
		"seat": 0,
	})

	if cleric.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 on Cleric, got %d", cleric.Counters["+1/+1"])
	}
	if soldier.Counters["+1/+1"] != 0 {
		t.Errorf("non-Cleric should NOT be buffed, got %d", soldier.Counters["+1/+1"])
	}
}

func TestMinwu_LifegainByOpponentNoBuff(t *testing.T) {
	gs := newGame(t, 2)
	minwu := addPerm(gs, 0, "Minwu, White Mage", "creature")
	cleric := addPerm(gs, 0, "Priest", "creature", "cleric")

	minwuWhiteMageTrigger(gs, minwu, map[string]interface{}{
		"seat": 1, // opponent gained life
	})

	if cleric.Counters["+1/+1"] != 0 {
		t.Error("Cleric should not be buffed when opponent gains life")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func bfNames(perms []*gameengine.Permanent) []string {
	out := []string{}
	for _, p := range perms {
		if p != nil && p.Card != nil {
			out = append(out, p.Card.DisplayName())
		}
	}
	return out
}
