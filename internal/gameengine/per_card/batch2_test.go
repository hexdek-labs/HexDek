package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Laboratory Maniac
// -----------------------------------------------------------------------------

func TestLaboratoryManiac_RegistersReplacement(t *testing.T) {
	gs := newGame(t, 2)
	lab := addPerm(gs, 0, "Laboratory Maniac", "creature")

	gameengine.InvokeETBHook(gs, lab)

	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler breadcrumb for Laboratory Maniac ETB")
	}
	// Draw from empty library should now trigger a WIN via the
	// replacement. The draw-replacement is registered in
	// gameengine.RegisterLaboratoryManiac during ETB.
	if !HasETB("Laboratory Maniac") {
		t.Errorf("Laboratory Maniac ETB handler should be registered")
	}
}

// -----------------------------------------------------------------------------
// Jace, Wielder of Mysteries
// -----------------------------------------------------------------------------

func TestJaceWielder_RegistersReplacement(t *testing.T) {
	gs := newGame(t, 2)
	j := addPerm(gs, 0, "Jace, Wielder of Mysteries", "creature", "planeswalker")
	gameengine.InvokeETBHook(gs, j)

	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler breadcrumb")
	}
	if !HasETB("Jace, Wielder of Mysteries") {
		t.Errorf("Jace ETB handler should be registered")
	}
}

// -----------------------------------------------------------------------------
// Ad Nauseam
// -----------------------------------------------------------------------------

func TestAdNauseam_SelfPreservationStopsEarly(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40
	// Library of mixed CMCs.
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		c := &gameengine.Card{Name: name, Owner: 0, Types: []string{"cmc:5"}}
		gs.Seats[0].Library = append(gs.Seats[0].Library, c)
	}
	card := addCard(gs, 0, "Ad Nauseam", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Without Angel's Grace, Ad Nauseam should stop once life ≤ 1.
	// Starting 40 life, each flip costs 5 life, so 8 flips would be 40
	// life loss → stops at 0. Handler should bail before hitting below
	// 1 without Grace.
	if gs.Seats[0].Life > 40 {
		t.Errorf("life should only decrease")
	}
	// Should have flipped at least 1 card.
	if len(gs.Seats[0].Hand) == 0 {
		t.Errorf("expected cards in hand after Ad Nauseam")
	}
	// Verify self-preservation: life should not go below 0 without Grace.
	if gs.Seats[0].Life < 0 && gs.Flags["angels_grace_eot_seat_0"] == 0 {
		t.Errorf("life went below 0 without Grace — self-preservation broken; life=%d", gs.Seats[0].Life)
	}
}

func TestAdNauseam_WithAngelsGraceKeepsFlipping(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 5
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["angels_grace_eot_seat_0"] = 1

	// 10 cards, each CMC 1.
	for i := 0; i < 10; i++ {
		c := &gameengine.Card{Name: "x", Owner: 0, Types: []string{"cmc:1"}}
		gs.Seats[0].Library = append(gs.Seats[0].Library, c)
	}
	card := addCard(gs, 0, "Ad Nauseam", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// All 10 cards should be in hand; life should be 5 - 10 = -5.
	if len(gs.Seats[0].Hand) != 10 {
		t.Errorf("with Grace, all cards should flip; hand size=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != -5 {
		t.Errorf("expected life -5 with Grace, got %d", gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Peregrine Drake
// -----------------------------------------------------------------------------

func TestPeregrineDrake_UntapsFiveLands(t *testing.T) {
	gs := newGame(t, 2)
	// Seven tapped lands.
	taps := []*gameengine.Permanent{}
	for i := 0; i < 7; i++ {
		p := addPerm(gs, 0, "Island", "land", "pip:U")
		p.Tapped = true
		taps = append(taps, p)
	}
	drake := addPerm(gs, 0, "Peregrine Drake", "creature")
	gameengine.InvokeETBHook(gs, drake)

	// Five should be untapped, two should remain tapped.
	untappedCount := 0
	for _, p := range taps {
		if !p.Tapped {
			untappedCount++
		}
	}
	if untappedCount != 5 {
		t.Errorf("expected exactly 5 untapped lands, got %d", untappedCount)
	}
}

// -----------------------------------------------------------------------------
// Palinchron
// -----------------------------------------------------------------------------

func TestPalinchron_UntapsSevenLandsAndBouncesSelf(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 9; i++ {
		p := addPerm(gs, 0, "Island", "land")
		p.Tapped = true
	}
	palinchron := addPerm(gs, 0, "Palinchron", "creature")
	gameengine.InvokeETBHook(gs, palinchron)

	// 7 of 9 should untap.
	untappedCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.IsLand() && !p.Tapped {
			untappedCount++
		}
	}
	if untappedCount != 7 {
		t.Errorf("expected 7 untapped lands, got %d", untappedCount)
	}
	// Activate: bounce self.
	gameengine.InvokeActivatedHook(gs, palinchron, 0, nil)
	// Palinchron should be gone from battlefield, in hand.
	for _, p := range gs.Seats[0].Battlefield {
		if p == palinchron {
			t.Errorf("Palinchron should have bounced off battlefield")
		}
	}
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Palinchron" {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Errorf("Palinchron should be in hand")
	}
}

// -----------------------------------------------------------------------------
// Deadeye Navigator
// -----------------------------------------------------------------------------

func TestDeadeyeNavigator_FlickersTarget(t *testing.T) {
	gs := newGame(t, 2)
	deadeye := addPerm(gs, 0, "Deadeye Navigator", "creature")
	_ = deadeye
	target := addPerm(gs, 0, "Peregrine Drake", "creature")
	preTs := target.Timestamp

	gameengine.InvokeActivatedHook(gs, deadeye, 0, map[string]interface{}{
		"target_perm": target,
	})

	if hasEvent(gs, "flicker") < 1 {
		t.Errorf("expected flicker event from Deadeye")
	}
	// New Drake on battlefield with higher timestamp.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.DisplayName() == "Peregrine Drake" && p.Timestamp > preTs {
			found = true
		}
	}
	if !found {
		t.Errorf("flickered Drake should have new higher timestamp")
	}
}

// -----------------------------------------------------------------------------
// Bolas's Citadel
// -----------------------------------------------------------------------------

func TestBolassCitadel_PlayTopPaysLifeEqualsCMC(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 30
	// Put a CMC-4 card on top of library.
	c := &gameengine.Card{Name: "Tendrils", Owner: 0, Types: []string{"sorcery", "cmc:4"}}
	gs.Seats[0].Library = []*gameengine.Card{c}
	citadel := addPerm(gs, 0, "Bolas's Citadel", "artifact")
	gameengine.InvokeETBHook(gs, citadel)

	gameengine.InvokeActivatedHook(gs, citadel, 0, nil)

	// Life should be 30 - 4 = 26.
	if gs.Seats[0].Life != 26 {
		t.Errorf("expected life 26 after paying CMC 4, got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected card in hand after play_top, got %d", len(gs.Seats[0].Hand))
	}
}

func TestBolassCitadel_SacrificeActivatesDamage(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 20
	citadel := addPerm(gs, 0, "Bolas's Citadel", "artifact")
	gameengine.InvokeETBHook(gs, citadel)

	gameengine.InvokeActivatedHook(gs, citadel, 1, nil)

	// Seat 1 should lose 10 life.
	if gs.Seats[1].Life != 10 {
		t.Errorf("expected seat 1 life 10, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Mana Crypt
// -----------------------------------------------------------------------------

func TestManaCrypt_TapForTwoMana(t *testing.T) {
	gs := newGame(t, 2)
	crypt := addPerm(gs, 0, "Mana Crypt", "artifact")
	gameengine.InvokeActivatedHook(gs, crypt, 0, nil)

	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected mana pool 2 after Mana Crypt tap, got %d", gs.Seats[0].ManaPool)
	}
	if !crypt.Tapped {
		t.Errorf("Mana Crypt should be tapped after activation")
	}
	// Cannot tap again.
	gameengine.InvokeActivatedHook(gs, crypt, 0, nil)
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("re-tap should not add mana, got %d", gs.Seats[0].ManaPool)
	}
}

// -----------------------------------------------------------------------------
// Dockside Extortionist
// -----------------------------------------------------------------------------

func TestDocksideExtortionist_MakesTreasurePerOpponentArtifactOrEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	// Seat 1 controls 2 artifacts and 1 enchantment.
	addPerm(gs, 1, "Sol Ring", "artifact")
	addPerm(gs, 1, "Lotus Petal", "artifact")
	addPerm(gs, 1, "Rhystic Study", "enchantment")
	// Seat 0 also has an artifact — should NOT count.
	addPerm(gs, 0, "Mana Crypt", "artifact")

	dockside := addPerm(gs, 0, "Dockside Extortionist", "creature")
	bfBefore := len(gs.Seats[0].Battlefield)
	gameengine.InvokeETBHook(gs, dockside)
	bfAfter := len(gs.Seats[0].Battlefield)

	// Expected: 3 new treasure tokens.
	diff := bfAfter - bfBefore
	if diff != 3 {
		t.Errorf("expected 3 new treasures on battlefield, got %d (before=%d after=%d)", diff, bfBefore, bfAfter)
	}
	// Verify names.
	treasureCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Treasure Token" {
			treasureCount++
		}
	}
	if treasureCount != 3 {
		t.Errorf("expected 3 treasures named 'Treasure Token', got %d", treasureCount)
	}
}

// -----------------------------------------------------------------------------
// Urza, Lord High Artificer
// -----------------------------------------------------------------------------

func TestUrzaETB_CreatesConstructToken(t *testing.T) {
	gs := newGame(t, 2)
	urza := addPerm(gs, 0, "Urza, Lord High Artificer", "creature", "legendary")
	bfBefore := len(gs.Seats[0].Battlefield)
	gameengine.InvokeETBHook(gs, urza)
	bfAfter := len(gs.Seats[0].Battlefield)

	if bfAfter-bfBefore != 1 {
		t.Errorf("expected 1 Construct token created, got %d", bfAfter-bfBefore)
	}
	foundConstruct := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Construct Token" {
			foundConstruct = true
		}
	}
	if !foundConstruct {
		t.Errorf("expected a Construct Token on battlefield")
	}
}

func TestUrzaActivateTapArtifactForU(t *testing.T) {
	gs := newGame(t, 2)
	urza := addPerm(gs, 0, "Urza, Lord High Artificer", "creature", "legendary")
	rock := addPerm(gs, 0, "Sol Ring", "artifact")
	gameengine.InvokeActivatedHook(gs, urza, 0, map[string]interface{}{
		"target_perm": rock,
	})
	if !rock.Tapped {
		t.Errorf("Sol Ring should be tapped after Urza activation")
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("expected +1 mana from Urza tap, got %d", gs.Seats[0].ManaPool)
	}
}

// -----------------------------------------------------------------------------
// Emry, Lurker of the Loch
// -----------------------------------------------------------------------------

func TestEmry_ETBMillsFour(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "a", "b", "c", "d", "e", "f")
	emry := addPerm(gs, 0, "Emry, Lurker of the Loch", "creature")
	gameengine.InvokeETBHook(gs, emry)

	if len(gs.Seats[0].Graveyard) != 4 {
		t.Errorf("expected 4 cards milled into graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards left in library, got %d", len(gs.Seats[0].Library))
	}
}

func TestEmry_ActivateCastFromGrave_OncePerTurn(t *testing.T) {
	gs := newGame(t, 2)
	emry := addPerm(gs, 0, "Emry, Lurker of the Loch", "creature")
	// Graveyard with an artifact.
	moxCard := &gameengine.Card{Name: "Mox Opal", Owner: 0, Types: []string{"artifact"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, moxCard)

	// First activation succeeds.
	gameengine.InvokeActivatedHook(gs, emry, 0, map[string]interface{}{
		"target_card": moxCard,
	})
	if len(gs.Seats[0].Exile) != 1 {
		t.Errorf("expected Mox Opal in exile after Emry activation, got %d", len(gs.Seats[0].Exile))
	}

	// Add a second artifact and try to activate again — should fail.
	mox2 := &gameengine.Card{Name: "Mox Jet", Owner: 0, Types: []string{"artifact"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, mox2)
	gameengine.InvokeActivatedHook(gs, emry, 0, map[string]interface{}{
		"target_card": mox2,
	})
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on second activation")
	}
}

// -----------------------------------------------------------------------------
// Sensei's Divining Top
// -----------------------------------------------------------------------------

func TestSenseisDiviningTop_LookReorder(t *testing.T) {
	gs := newGame(t, 2)
	top := addPerm(gs, 0, "Sensei's Divining Top", "artifact")
	addLibrary(gs, 0, "a", "b", "c", "d", "e")
	beforeLen := len(gs.Seats[0].Library)

	gameengine.InvokeActivatedHook(gs, top, 0, nil)

	// Library size unchanged; handler just reorders.
	if len(gs.Seats[0].Library) != beforeLen {
		t.Errorf("library size should not change, got %d", len(gs.Seats[0].Library))
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler breadcrumb")
	}
}

func TestSenseisDiviningTop_DrawAndSelfTop(t *testing.T) {
	gs := newGame(t, 2)
	top := addPerm(gs, 0, "Sensei's Divining Top", "artifact")
	addLibrary(gs, 0, "a", "b", "c")

	gameengine.InvokeActivatedHook(gs, top, 1, nil)

	// Drew a card ("a"), then Top goes on top of library.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) < 1 {
		t.Errorf("library should have cards including Top itself")
	}
	// Top should no longer be on battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == top {
			t.Errorf("Top should have moved off battlefield")
		}
	}
}

// -----------------------------------------------------------------------------
// Angel's Grace
// -----------------------------------------------------------------------------

func TestAngelsGrace_SetsFlagAndRegistersCleanup(t *testing.T) {
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Angel's Grace", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if gs.Flags["angels_grace_eot_seat_0"] != 1 {
		t.Errorf("expected angels_grace_eot_seat_0 flag set, got %d",
			gs.Flags["angels_grace_eot_seat_0"])
	}
	if len(gs.DelayedTriggers) < 1 {
		t.Errorf("expected end-of-turn cleanup trigger registered")
	}
}

// -----------------------------------------------------------------------------
// Sundial of the Infinite
// -----------------------------------------------------------------------------

func TestSundial_EndsTurnClearsStackAndTriggers(t *testing.T) {
	gs := newGame(t, 2)
	sundial := addPerm(gs, 0, "Sundial of the Infinite", "artifact")
	// Seed the stack + delayed triggers.
	gs.Stack = []*gameengine.StackItem{{ID: 1, Controller: 1}}
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: 0,
		SourceCardName: "Final Fortune",
		EffectFn:       func(*gameengine.GameState) {},
	})

	gameengine.InvokeActivatedHook(gs, sundial, 0, nil)

	if gs.Flags["turn_ending_now"] != 1 {
		t.Errorf("expected turn_ending_now flag = 1")
	}
	if len(gs.Stack) != 0 {
		t.Errorf("expected stack to be drained, got %d items", len(gs.Stack))
	}
	// The end-of-turn delayed trigger we registered should be removed.
	for _, dt := range gs.DelayedTriggers {
		if dt.TriggerAt == "end_of_turn" {
			t.Errorf("end_of_turn delayed trigger should have been removed")
		}
	}
}

// -----------------------------------------------------------------------------
// Grand Abolisher
// -----------------------------------------------------------------------------

func TestGrandAbolisher_SetsFlag(t *testing.T) {
	gs := newGame(t, 2)
	abolisher := addPerm(gs, 0, "Grand Abolisher", "creature")
	gameengine.InvokeETBHook(gs, abolisher)

	if !GrandAbolisherActive(gs, 0) {
		t.Errorf("GrandAbolisherActive should report true for seat 0")
	}
	if GrandAbolisherActive(gs, 1) {
		t.Errorf("GrandAbolisherActive should report false for seat 1")
	}
}

// -----------------------------------------------------------------------------
// Mystic Remora cumulative upkeep
// -----------------------------------------------------------------------------

func TestMysticRemora_UpkeepAgeCountersAccumulate(t *testing.T) {
	gs := newGame(t, 2)
	remora := addPerm(gs, 0, "Mystic Remora", "enchantment")
	gs.Seats[0].ManaPool = 100

	// Turn 1 upkeep: age=1 → pay 1.
	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})
	if remora.Counters["age"] != 1 {
		t.Errorf("expected 1 age counter after turn 1, got %d", remora.Counters["age"])
	}
	if gs.Seats[0].ManaPool != 99 {
		t.Errorf("expected mana pool 99 after paying 1, got %d", gs.Seats[0].ManaPool)
	}

	// Turn 2 upkeep: age=2 → pay 2.
	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})
	if remora.Counters["age"] != 2 {
		t.Errorf("expected 2 age counters, got %d", remora.Counters["age"])
	}
	if gs.Seats[0].ManaPool != 97 {
		t.Errorf("expected mana pool 97, got %d", gs.Seats[0].ManaPool)
	}

	// Turn 3 upkeep: age=3 → pay 3 (total 6 paid).
	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})
	if gs.Seats[0].ManaPool != 94 {
		t.Errorf("expected mana pool 94, got %d", gs.Seats[0].ManaPool)
	}

	// Turn 4 upkeep: age=4 → sacrifice per policy.
	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})
	// Remora should be in graveyard now.
	for _, p := range gs.Seats[0].Battlefield {
		if p == remora {
			t.Errorf("Remora should have been sacrificed at turn 4 upkeep")
		}
	}
	foundInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.DisplayName() == "Mystic Remora" {
			foundInGY = true
		}
	}
	if !foundInGY {
		t.Errorf("expected Remora in graveyard after sacrifice")
	}
	if hasEvent(gs, "sacrifice") < 1 {
		t.Errorf("expected sacrifice event")
	}
}

func TestMysticRemora_UpkeepSacrificesWhenCantPay(t *testing.T) {
	gs := newGame(t, 2)
	remora := addPerm(gs, 0, "Mystic Remora", "enchantment")
	gs.Seats[0].ManaPool = 0 // can't afford even 1

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})

	// Remora should be sacrificed on turn 1 (can't pay 1).
	for _, p := range gs.Seats[0].Battlefield {
		if p == remora {
			t.Errorf("Remora should have been sacrificed (couldn't pay)")
		}
	}
}

// -----------------------------------------------------------------------------
// Registry smoke test: all batch #2 cards registered
// -----------------------------------------------------------------------------

func TestRegistry_Batch2CardsRegistered(t *testing.T) {
	// ETB handlers.
	etbCards := []string{
		"Laboratory Maniac",
		"Jace, Wielder of Mysteries",
		"Peregrine Drake",
		"Palinchron",
		"Deadeye Navigator",
		"Bolas's Citadel",
		"Dockside Extortionist",
		"Urza, Lord High Artificer",
		"Emry, Lurker of the Loch",
		"Grand Abolisher",
	}
	for _, n := range etbCards {
		if !HasETB(n) {
			t.Errorf("expected ETB handler for %s", n)
		}
	}
	// Resolve handlers.
	resolveCards := []string{
		"Ad Nauseam",
		"Angel's Grace",
	}
	for _, n := range resolveCards {
		if !HasResolve(n) {
			t.Errorf("expected Resolve handler for %s", n)
		}
	}
	// Activated handlers.
	activatedCards := []string{
		"Palinchron",
		"Deadeye Navigator",
		"Bolas's Citadel",
		"Mana Crypt",
		"Urza, Lord High Artificer",
		"Emry, Lurker of the Loch",
		"Sensei's Divining Top",
		"Sundial of the Infinite",
	}
	for _, n := range activatedCards {
		if !HasActivated(n) {
			t.Errorf("expected Activated handler for %s", n)
		}
	}
}
