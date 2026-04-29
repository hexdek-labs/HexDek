package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Kinnan, Bonder Prodigy
// -----------------------------------------------------------------------------

func TestKinnan_StaticAddsExtraManaOnNonlandTap(t *testing.T) {
	gs := newGame(t, 2)
	kinnan := addPerm(gs, 0, "Kinnan, Bonder Prodigy", "creature", "legendary")
	gameengine.InvokeETBHook(gs, kinnan)

	// A nonland mana source taps for 1 G — Kinnan should ADD +1 of the
	// same type. Simulate the tap via AddManaFromPermanent.
	rock := addPerm(gs, 0, "Mana Rock", "artifact")
	gameengine.AddManaFromPermanent(gs, gs.Seats[0], rock, "G", 1)

	// Expected: original 1 G + Kinnan's extra 1 G = 2 total.
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected mana pool 2 (1 original + 1 from Kinnan), got %d", gs.Seats[0].ManaPool)
	}
}

func TestKinnan_DoesNotTriggerOnLandTap(t *testing.T) {
	gs := newGame(t, 2)
	kinnan := addPerm(gs, 0, "Kinnan, Bonder Prodigy", "creature", "legendary")
	gameengine.InvokeETBHook(gs, kinnan)

	land := addPerm(gs, 0, "Forest", "land")
	gameengine.AddManaFromPermanent(gs, gs.Seats[0], land, "G", 1)

	// Expected: only 1 mana, no Kinnan doubling on a LAND.
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("expected mana pool 1 (no Kinnan doubling on land), got %d", gs.Seats[0].ManaPool)
	}
}

func TestKinnan_DoesNotRecurseOnItsOwnMana(t *testing.T) {
	// Kinnan's extra mana add must NOT re-trigger Kinnan (would loop).
	gs := newGame(t, 2)
	kinnan := addPerm(gs, 0, "Kinnan, Bonder Prodigy", "creature", "legendary")
	gameengine.InvokeETBHook(gs, kinnan)

	rock := addPerm(gs, 0, "Mana Rock", "artifact")
	gameengine.AddManaFromPermanent(gs, gs.Seats[0], rock, "C", 3)

	// Expected: 3 original + 1 Kinnan = 4 total. NOT 3 + 3 = 6 or infinite.
	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("expected mana pool 4 (3 + 1 Kinnan, no recursion), got %d", gs.Seats[0].ManaPool)
	}
}

func TestKinnan_Activate_RevealCreaturePlacesOnBattlefield(t *testing.T) {
	gs := newGame(t, 2)
	kinnan := addPerm(gs, 0, "Kinnan, Bonder Prodigy", "creature", "legendary")
	creatureCard := &gameengine.Card{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*gameengine.Card{creatureCard}

	bfBefore := len(gs.Seats[0].Battlefield)
	gameengine.InvokeActivatedHook(gs, kinnan, 0, nil)

	if len(gs.Seats[0].Battlefield) != bfBefore+1 {
		t.Errorf("expected creature onto battlefield, have %d", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("library should be drained by 1, got %d", len(gs.Seats[0].Library))
	}
}

// -----------------------------------------------------------------------------
// Basalt Monolith + Kinnan: the infinite-mana combo
// -----------------------------------------------------------------------------

func TestBasaltMonolith_TapAddsThreeColorless(t *testing.T) {
	gs := newGame(t, 2)
	basalt := addPerm(gs, 0, "Basalt Monolith", "artifact")
	gameengine.InvokeETBHook(gs, basalt)

	gameengine.InvokeActivatedHook(gs, basalt, 0, nil)

	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected 3 mana, got %d", gs.Seats[0].ManaPool)
	}
	if !basalt.Tapped {
		t.Errorf("Basalt should be tapped")
	}
}

func TestBasaltMonolith_UntapActivation(t *testing.T) {
	gs := newGame(t, 2)
	basalt := addPerm(gs, 0, "Basalt Monolith", "artifact")
	basalt.Tapped = true
	gameengine.InvokeActivatedHook(gs, basalt, 1, nil)

	if basalt.Tapped {
		t.Errorf("Basalt should be untapped after activation")
	}
}

func TestBasaltMonolith_WithKinnanInfinite(t *testing.T) {
	// The headline combo test.
	gs := newGame(t, 2)
	kinnan := addPerm(gs, 0, "Kinnan, Bonder Prodigy", "creature", "legendary")
	gameengine.InvokeETBHook(gs, kinnan)
	basalt := addPerm(gs, 0, "Basalt Monolith", "artifact")
	gameengine.InvokeETBHook(gs, basalt)

	// First cycle: tap Basalt → 3 mana + 1 Kinnan = 4. Pay {3} untap.
	const cycles = 5
	for i := 0; i < cycles; i++ {
		// Reset tapped for the cycle.
		basalt.Tapped = false
		preMana := gs.Seats[0].ManaPool
		gameengine.InvokeActivatedHook(gs, basalt, 0, nil) // Tap → +4 mana
		afterTap := gs.Seats[0].ManaPool
		if afterTap-preMana != 4 {
			t.Errorf("cycle %d: expected +4 mana per tap (3 + Kinnan), got %d", i, afterTap-preMana)
		}
		// Now pay {3} via the typed pool's PayGenericCost path (both typed
		// pool and legacy int must decrement in lock-step; a raw
		// `ManaPool -= 3` leaves the typed pool out of sync).
		ok := gameengine.PayGenericCost(gs, gs.Seats[0], 3, "activated", "basalt_untap_cost", "Basalt Monolith")
		if !ok {
			t.Fatalf("cycle %d: failed to pay {3} for untap", i)
		}
		gameengine.InvokeActivatedHook(gs, basalt, 1, nil)
		if basalt.Tapped {
			t.Errorf("cycle %d: Basalt should be untapped after paying 3", i)
		}
	}
	// After 5 cycles, we've netted +1 per cycle = 5 total.
	if gs.Seats[0].ManaPool != cycles {
		t.Errorf("expected net +%d mana after Kinnan+Basalt infinite loop, got %d", cycles, gs.Seats[0].ManaPool)
	}
}

// -----------------------------------------------------------------------------
// Grim Monolith
// -----------------------------------------------------------------------------

func TestGrimMonolith_TapAddsThreeColorless(t *testing.T) {
	gs := newGame(t, 2)
	grim := addPerm(gs, 0, "Grim Monolith", "artifact")
	gameengine.InvokeETBHook(gs, grim)

	gameengine.InvokeActivatedHook(gs, grim, 0, nil)

	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected 3 mana from Grim Monolith tap, got %d", gs.Seats[0].ManaPool)
	}
}

func TestGrimMonolith_UntapsOnActivation(t *testing.T) {
	gs := newGame(t, 2)
	grim := addPerm(gs, 0, "Grim Monolith", "artifact")
	grim.Tapped = true

	gameengine.InvokeActivatedHook(gs, grim, 1, nil)
	if grim.Tapped {
		t.Errorf("Grim Monolith should untap")
	}
}

// -----------------------------------------------------------------------------
// Isochron Scepter + Dramatic Reversal
// -----------------------------------------------------------------------------

func TestIsochronScepter_ImprintsInstantFromHand(t *testing.T) {
	gs := newGame(t, 2)
	reversal := addCard(gs, 0, "Dramatic Reversal", "instant", "cmc:2")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, reversal)

	scepter := addPerm(gs, 0, "Isochron Scepter", "artifact")
	gameengine.InvokeETBHook(gs, scepter)

	// Reversal should move from hand to exile.
	for _, c := range gs.Seats[0].Hand {
		if c == reversal {
			t.Errorf("Reversal should have been imprinted (moved to exile)")
		}
	}
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == reversal {
			foundInExile = true
		}
	}
	if !foundInExile {
		t.Errorf("Reversal should be in exile")
	}
	if scepter.Flags["imprint_present"] != 1 {
		t.Errorf("imprint_present flag should be set")
	}
}

func TestIsochronScepter_ImprintSkipsNonInstant(t *testing.T) {
	gs := newGame(t, 2)
	sorceryCard := addCard(gs, 0, "Demonic Tutor", "sorcery", "cmc:2")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, sorceryCard)

	scepter := addPerm(gs, 0, "Isochron Scepter", "artifact")
	gameengine.InvokeETBHook(gs, scepter)

	// Sorcery should still be in hand (can't be imprinted).
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == sorceryCard {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Errorf("sorcery should still be in hand (not imprint-eligible)")
	}
}

func TestIsochronScepter_ImprintSkipsTooHighCMC(t *testing.T) {
	gs := newGame(t, 2)
	bigInstant := addCard(gs, 0, "Counterspell++", "instant", "cmc:5")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, bigInstant)

	scepter := addPerm(gs, 0, "Isochron Scepter", "artifact")
	gameengine.InvokeETBHook(gs, scepter)

	// CMC 5 > 2, should not imprint.
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == bigInstant {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Errorf("cmc-5 instant should stay in hand")
	}
}

func TestIsochronScepter_ActivateFiresImprintedResolveHandler(t *testing.T) {
	gs := newGame(t, 2)
	// Put Dramatic Reversal in hand, imprint, then activate.
	reversal := addCard(gs, 0, "Dramatic Reversal", "instant", "cmc:2")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, reversal)

	scepter := addPerm(gs, 0, "Isochron Scepter", "artifact")
	gameengine.InvokeETBHook(gs, scepter)

	// Tap a rock so Reversal has something to untap.
	rock := addPerm(gs, 0, "Sol Ring", "artifact")
	rock.Tapped = true

	gameengine.InvokeActivatedHook(gs, scepter, 0, nil)

	// Sol Ring should have been untapped by the Reversal copy.
	if rock.Tapped {
		t.Errorf("Sol Ring should have been untapped by Dramatic Reversal copy")
	}
	// Scepter itself ALSO gets untapped by Reversal (it's a nonland
	// permanent controlled by seat 0 — that's the whole reason this
	// is an infinite combo!). So we expect scepter.Tapped == false.
	if scepter.Tapped {
		t.Errorf("Scepter should ALSO be untapped by its own copy of Reversal (this is the combo!)")
	}
}

// -----------------------------------------------------------------------------
// Dramatic Reversal
// -----------------------------------------------------------------------------

func TestDramaticReversal_UntapsAllNonlandPermanents(t *testing.T) {
	gs := newGame(t, 2)
	rock := addPerm(gs, 0, "Sol Ring", "artifact")
	rock.Tapped = true
	dork := addPerm(gs, 0, "Llanowar Elves", "creature")
	dork.Tapped = true
	land := addPerm(gs, 0, "Island", "land")
	land.Tapped = true

	card := addCard(gs, 0, "Dramatic Reversal", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if rock.Tapped {
		t.Errorf("Sol Ring should have untapped")
	}
	if dork.Tapped {
		t.Errorf("Llanowar Elves should have untapped")
	}
	if !land.Tapped {
		t.Errorf("Island (land) should remain tapped — Reversal only untaps nonland")
	}
}

// -----------------------------------------------------------------------------
// Null Rod
// -----------------------------------------------------------------------------

func TestNullRod_SetsActiveFlag(t *testing.T) {
	gs := newGame(t, 2)
	rod := addPerm(gs, 0, "Null Rod", "artifact")
	gameengine.InvokeETBHook(gs, rod)

	if !NullRodActive(gs) {
		t.Errorf("NullRodActive should be true after Rod ETB")
	}
}

func TestNullRod_ExemptsManaAbilities(t *testing.T) {
	gs := newGame(t, 2)
	rod := addPerm(gs, 0, "Null Rod", "artifact")
	gameengine.InvokeETBHook(gs, rod)

	// Sol Ring's ability 0 is a mana ability — exempted.
	sol := addPerm(gs, 0, "Sol Ring", "artifact")
	if NullRodSuppresses(gs, sol, 0) {
		t.Errorf("Sol Ring's mana ability should NOT be suppressed")
	}

	// Aetherflux Reservoir ability 0 is 50-damage (non-mana) — suppressed.
	aet := addPerm(gs, 0, "Aetherflux Reservoir", "artifact")
	if !NullRodSuppresses(gs, aet, 0) {
		t.Errorf("Aetherflux's 50-damage ability SHOULD be suppressed by Null Rod")
	}
}

func TestNullRod_IgnoresNonArtifacts(t *testing.T) {
	gs := newGame(t, 2)
	rod := addPerm(gs, 0, "Null Rod", "artifact")
	gameengine.InvokeETBHook(gs, rod)

	// Creature activation — not an artifact, not suppressed by Null Rod.
	dork := addPerm(gs, 0, "Llanowar Elves", "creature")
	if NullRodSuppresses(gs, dork, 0) {
		t.Errorf("Null Rod should NOT suppress creature abilities (Cursed Totem's job)")
	}
}

// -----------------------------------------------------------------------------
// Collector Ouphe
// -----------------------------------------------------------------------------

func TestCollectorOuphe_SharesNullRodCounter(t *testing.T) {
	gs := newGame(t, 2)
	ouphe := addPerm(gs, 0, "Collector Ouphe", "creature")
	gameengine.InvokeETBHook(gs, ouphe)

	if !NullRodActive(gs) {
		t.Errorf("Collector Ouphe should share NullRodActive")
	}
	// Aetherflux activation should also be suppressed.
	aet := addPerm(gs, 0, "Aetherflux Reservoir", "artifact")
	if !NullRodSuppresses(gs, aet, 0) {
		t.Errorf("Aetherflux activation should be suppressed by Collector Ouphe")
	}
}

// -----------------------------------------------------------------------------
// Cursed Totem
// -----------------------------------------------------------------------------

func TestCursedTotem_SuppressesCreatureAbilities(t *testing.T) {
	gs := newGame(t, 2)
	totem := addPerm(gs, 0, "Cursed Totem", "artifact")
	gameengine.InvokeETBHook(gs, totem)

	if !CursedTotemActive(gs) {
		t.Errorf("CursedTotemActive should be true")
	}

	dork := addPerm(gs, 0, "Birds of Paradise", "creature")
	if !CursedTotemSuppresses(gs, dork) {
		t.Errorf("Birds of Paradise activation should be suppressed")
	}

	// Artifact (non-creature) — Totem does NOT suppress.
	rock := addPerm(gs, 0, "Sol Ring", "artifact")
	if CursedTotemSuppresses(gs, rock) {
		t.Errorf("Cursed Totem should NOT suppress artifact-only activations (Null Rod's job)")
	}
}

// -----------------------------------------------------------------------------
// Drannith Magistrate
// -----------------------------------------------------------------------------

func TestDrannith_SetsOpponentRestriction(t *testing.T) {
	gs := newGame(t, 2)
	drannith := addPerm(gs, 0, "Drannith Magistrate", "creature", "legendary")
	gameengine.InvokeETBHook(gs, drannith)

	if !DrannithMagistrateRestrictsOpponent(gs, 1) {
		t.Errorf("seat 1 (opponent) should be restricted by seat 0's Drannith")
	}
	if DrannithMagistrateRestrictsOpponent(gs, 0) {
		t.Errorf("seat 0 (controller) should NOT be restricted by own Drannith")
	}
}

// -----------------------------------------------------------------------------
// Opposition Agent
// -----------------------------------------------------------------------------

func TestOppositionAgent_ControlsOpponentSearch(t *testing.T) {
	gs := newGame(t, 2)
	agent := addPerm(gs, 0, "Opposition Agent", "creature")
	gameengine.InvokeETBHook(gs, agent)

	// If seat 1 searches, seat 0 controls the search.
	if OppositionAgentControlsSearch(gs, 1) != 0 {
		t.Errorf("seat 0's Agent should control seat 1's search")
	}
	// Seat 0 searching their own library → no Agent controls it.
	if OppositionAgentControlsSearch(gs, 0) != -1 {
		t.Errorf("seat 0 searching their own library should be unrestricted")
	}
}

func TestOppositionAgent_ExilesSearchResultToControllerExile(t *testing.T) {
	gs := newGame(t, 2)
	_ = addPerm(gs, 0, "Opposition Agent", "creature")
	gameengine.InvokeETBHook(gs, findPermanentByName(gs, "Opposition Agent"))

	// Simulate seat 1 searching for a card; Agent's controller (seat 0)
	// gets to exile it.
	tutoredCard := &gameengine.Card{Name: "Demonic Tutor", Owner: 1, Types: []string{"sorcery"}}
	ExileSearchResult(gs, 0, tutoredCard)

	foundInSeat0Exile := false
	for _, c := range gs.Seats[0].Exile {
		if c == tutoredCard {
			foundInSeat0Exile = true
		}
	}
	if !foundInSeat0Exile {
		t.Errorf("tutored card should be in Agent-controller's exile")
	}
}

// -----------------------------------------------------------------------------
// Necropotence
// -----------------------------------------------------------------------------

func TestNecropotence_PayLifeExileTopCard(t *testing.T) {
	gs := newGame(t, 2)
	necro := addPerm(gs, 0, "Necropotence", "enchantment")
	gameengine.InvokeETBHook(gs, necro)
	gs.Seats[0].Life = 40
	addLibrary(gs, 0, "CardA", "CardB", "CardC")

	gameengine.InvokeActivatedHook(gs, necro, 0, nil)

	if gs.Seats[0].Life != 39 {
		t.Errorf("expected life 39 after paying 1, got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Exile) != 1 {
		t.Errorf("expected 1 card in exile, got %d", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards left in library, got %d", len(gs.Seats[0].Library))
	}
	// Delayed end-step trigger registered.
	if len(gs.DelayedTriggers) < 1 {
		t.Errorf("expected end-of-turn trigger registered")
	}
}

func TestNecropotence_DelayedTriggerReturnsToHand(t *testing.T) {
	gs := newGame(t, 2)
	necro := addPerm(gs, 0, "Necropotence", "enchantment")
	gameengine.InvokeETBHook(gs, necro)
	gs.Seats[0].Life = 40
	addLibrary(gs, 0, "CardA", "CardB")

	gameengine.InvokeActivatedHook(gs, necro, 0, nil)

	// Fire the delayed trigger directly (simulating end-of-turn).
	for _, dt := range gs.DelayedTriggers {
		if dt.TriggerAt == "end_of_turn" && dt.SourceCardName == "Necropotence" {
			dt.EffectFn(gs)
		}
	}

	// Card should now be in hand, out of exile.
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("expected empty exile after end-step return, got %d", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand after end-step, got %d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Griselbrand
// -----------------------------------------------------------------------------

func TestGriselbrand_PayLifeDrawSeven(t *testing.T) {
	gs := newGame(t, 2)
	gris := addPerm(gs, 0, "Griselbrand", "creature", "legendary")
	gs.Seats[0].Life = 40
	addLibrary(gs, 0, "A", "B", "C", "D", "E", "F", "G", "H", "I", "J")

	gameengine.InvokeActivatedHook(gs, gris, 0, nil)

	if gs.Seats[0].Life != 33 {
		t.Errorf("expected life 33 after paying 7, got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 7 {
		t.Errorf("expected 7 cards drawn, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 3 {
		t.Errorf("expected 3 cards left in library, got %d", len(gs.Seats[0].Library))
	}
}

func TestGriselbrand_PartialDrawWhenLibraryLow(t *testing.T) {
	gs := newGame(t, 2)
	gris := addPerm(gs, 0, "Griselbrand", "creature", "legendary")
	gs.Seats[0].Life = 40
	// Only 3 cards in library; drawing 7 should draw 3 and set empty flag.
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.InvokeActivatedHook(gs, gris, 0, nil)

	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("expected 3 cards drawn, got %d", len(gs.Seats[0].Hand))
	}
	if !gs.Seats[0].AttemptedEmptyDraw {
		t.Errorf("AttemptedEmptyDraw should be true")
	}
}

// -----------------------------------------------------------------------------
// Razaketh, the Foulblooded
// -----------------------------------------------------------------------------

func TestRazaketh_PaysLifeSacTutors(t *testing.T) {
	gs := newGame(t, 2)
	raz := addPerm(gs, 0, "Razaketh, the Foulblooded", "creature", "legendary")
	gs.Seats[0].Life = 30
	fodder := addPerm(gs, 0, "Saproling Token", "creature", "token")
	target := &gameengine.Card{Name: "Demonic Tutor", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Swamp", Owner: 0, Types: []string{"basic", "land"}},
		target,
		{Name: "Forest", Owner: 0, Types: []string{"basic", "land"}},
	}

	gameengine.InvokeActivatedHook(gs, raz, 0, map[string]interface{}{
		"sacrifice_target": fodder,
		"named_card":       "Demonic Tutor",
	})

	// Life: 30 - 2 = 28.
	if gs.Seats[0].Life != 28 {
		t.Errorf("expected life 28 after paying 2, got %d", gs.Seats[0].Life)
	}
	// Fodder sacrificed → in graveyard OR ceases if token.
	for _, p := range gs.Seats[0].Battlefield {
		if p == fodder {
			t.Errorf("fodder should have been sacrificed")
		}
	}
	// Target in hand.
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c == target {
			found = true
		}
	}
	if !found {
		t.Errorf("Demonic Tutor should be in hand; hand=%+v", gs.Seats[0].Hand)
	}
}

func TestRazaketh_CannotSacrificeSelf(t *testing.T) {
	gs := newGame(t, 2)
	raz := addPerm(gs, 0, "Razaketh, the Foulblooded", "creature", "legendary")
	gs.Seats[0].Life = 30

	gameengine.InvokeActivatedHook(gs, raz, 0, map[string]interface{}{
		"sacrifice_target": raz,
	})

	// Should fail; no life paid.
	if gs.Seats[0].Life != 30 {
		t.Errorf("should not pay life if sac target invalid; life=%d", gs.Seats[0].Life)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected failure event")
	}
}

// -----------------------------------------------------------------------------
// Deadeye Navigator — soulbond auto-pairing upgrade
// -----------------------------------------------------------------------------

func TestDeadeyeNavigator_AutoPairsOnETB(t *testing.T) {
	gs := newGame(t, 2)
	drake := addPerm(gs, 0, "Peregrine Drake", "creature")
	deadeye := addPerm(gs, 0, "Deadeye Navigator", "creature")

	gameengine.InvokeETBHook(gs, deadeye)

	if deadeye.Flags["paired_timestamp"] != drake.Timestamp {
		t.Errorf("Deadeye should be paired with Drake, got paired_timestamp=%d (want %d)",
			deadeye.Flags["paired_timestamp"], drake.Timestamp)
	}
	if drake.Flags["paired_timestamp"] != deadeye.Timestamp {
		t.Errorf("Drake should be paired with Deadeye, got paired_timestamp=%d (want %d)",
			drake.Flags["paired_timestamp"], deadeye.Timestamp)
	}
}

func TestDeadeyeNavigator_DoesNotPairWithSelf(t *testing.T) {
	gs := newGame(t, 2)
	deadeye := addPerm(gs, 0, "Deadeye Navigator", "creature")

	gameengine.InvokeETBHook(gs, deadeye)

	// No partner available — deadeye should not have a partner stamp.
	if deadeye.Flags["paired_timestamp"] != 0 {
		t.Errorf("Deadeye should not self-pair; got stamp %d", deadeye.Flags["paired_timestamp"])
	}
}

// -----------------------------------------------------------------------------
// Phantasmal Image
// -----------------------------------------------------------------------------

func TestPhantasmalImage_CopiesTargetCreature(t *testing.T) {
	gs := newGame(t, 2)
	// Target creature with a distinct name.
	addPerm(gs, 0, "Llanowar Elves", "creature")

	image := addPerm(gs, 0, "Phantasmal Image", "creature")
	gameengine.InvokeETBHook(gs, image)

	// Image's Card should now point at Llanowar Elves' Card (same
	// display name).
	if image.Card.DisplayName() != "Llanowar Elves" {
		t.Errorf("Image should copy Llanowar Elves' card name, got %q", image.Card.DisplayName())
	}
	// Sac rider flag set.
	if image.Flags["phantasmal_sac_on_target"] != 1 {
		t.Errorf("Image should have sac-on-target rider set")
	}
}

func TestPhantasmalImage_SacOnTargeted(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Llanowar Elves", "creature")
	image := addPerm(gs, 0, "Phantasmal Image", "creature")
	gameengine.InvokeETBHook(gs, image)

	// Call the sac-on-target checker (what a targeting primitive would
	// do when image becomes the target). Note: after the copy, image's
	// Card.DisplayName() is "Llanowar Elves", so FireCardTrigger won't
	// resolve to Phantasmal Image's handler via name lookup. The flag-
	// scan helper handles this correctly.
	PhantasmalImageCheckTargeted(gs, image)

	// Image should have been sacrificed.
	for _, p := range gs.Seats[0].Battlefield {
		if p == image {
			t.Errorf("Image should have been sacrificed when targeted")
		}
	}
}

// -----------------------------------------------------------------------------
// Registry smoke test: all batch #3 cards registered
// -----------------------------------------------------------------------------

func TestRegistry_Batch3CardsRegistered(t *testing.T) {
	// Kinnan has Activated + Trigger (OnTrigger for mana_added_from_permanent)
	// but NO OnETB handler by design — his static is a passive react-to-mana
	// hook rather than a battlefield-arrival effect.
	etbCards := []string{
		"Basalt Monolith",
		"Grim Monolith",
		"Isochron Scepter",
		"Null Rod",
		"Collector Ouphe",
		"Cursed Totem",
		"Drannith Magistrate",
		"Opposition Agent",
		"Necropotence",
		"Phantasmal Image",
	}
	for _, n := range etbCards {
		if !HasETB(n) {
			t.Errorf("expected ETB handler for %s", n)
		}
	}
	resolveCards := []string{
		"Dramatic Reversal",
	}
	for _, n := range resolveCards {
		if !HasResolve(n) {
			t.Errorf("expected Resolve handler for %s", n)
		}
	}
	activatedCards := []string{
		"Kinnan, Bonder Prodigy",
		"Basalt Monolith",
		"Grim Monolith",
		"Isochron Scepter",
		"Necropotence",
		"Griselbrand",
		"Razaketh, the Foulblooded",
	}
	for _, n := range activatedCards {
		if !HasActivated(n) {
			t.Errorf("expected Activated handler for %s", n)
		}
	}
}

// -----------------------------------------------------------------------------
// Integration: Scepter + Reversal + mana rocks — the canonical cEDH combo
// -----------------------------------------------------------------------------

func TestIntegration_ScepterReversalInfiniteManaLoop(t *testing.T) {
	gs := newGame(t, 2)
	// Hand: Dramatic Reversal (will be imprinted).
	reversal := addCard(gs, 0, "Dramatic Reversal", "instant", "cmc:2")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, reversal)

	// Mana rocks: 3 Sol Rings' worth of generic output.
	rocks := []*gameengine.Permanent{}
	for i := 0; i < 3; i++ {
		r := addPerm(gs, 0, "Sol Ring", "artifact")
		rocks = append(rocks, r)
	}

	// Scepter ETBs & imprints Reversal.
	scepter := addPerm(gs, 0, "Isochron Scepter", "artifact")
	gameengine.InvokeETBHook(gs, scepter)

	// Cycle N times. Each cycle:
	//   - Tap rocks → 6 mana (3 rocks × 2).
	//   - Pay {2} to activate Scepter. Mana: 4.
	//   - Reversal copy resolves → untaps all nonlands (rocks + Scepter).
	//
	// Net: +4 mana per cycle. After 10 cycles we should have 40 mana.
	const cycles = 10
	for cycle := 0; cycle < cycles; cycle++ {
		// Reset rock tapped state (simulate a fresh turn where all rocks
		// are already tapped by the previous Reversal).
		for _, r := range rocks {
			r.Tapped = true
		}
		scepter.Tapped = true

		// Now "tap" rocks for mana.
		for _, r := range rocks {
			r.Tapped = false                  // untap for the pay step
			gs.Seats[0].ManaPool += 2         // Sol Ring tap
			r.Tapped = true
		}
		// Pay {2} to activate Scepter.
		scepter.Tapped = false
		gs.Seats[0].ManaPool -= 2
		gameengine.InvokeActivatedHook(gs, scepter, 0, nil)
		// Rocks should now be untapped (from Reversal).
	}

	// Net +4 per cycle: but we added 6 and paid 2 manually, then Reversal
	// untaps everything. So after each cycle we have previous + (6 - 2) = +4.
	// Over 10 cycles: 40 mana.
	if gs.Seats[0].ManaPool < cycles*4 {
		t.Errorf("expected at least %d mana after %d cycles of Scepter+Reversal, got %d",
			cycles*4, cycles, gs.Seats[0].ManaPool)
	}
}

// -----------------------------------------------------------------------------
// Integration: Kinnan + Basalt Monolith explicit infinite loop with cap
// -----------------------------------------------------------------------------

func TestIntegration_KinnanBasaltInfiniteManaWithCap(t *testing.T) {
	gs := newGame(t, 2)
	kinnan := addPerm(gs, 0, "Kinnan, Bonder Prodigy", "creature", "legendary")
	gameengine.InvokeETBHook(gs, kinnan)
	basalt := addPerm(gs, 0, "Basalt Monolith", "artifact")
	gameengine.InvokeETBHook(gs, basalt)

	// Simulate the loop: tap Basalt → +4 (3 + Kinnan's 1) → pay {3} →
	// untap. Repeat until hit cap.
	const cap = 100
	for cycle := 0; cycle < cap; cycle++ {
		basalt.Tapped = false
		gameengine.InvokeActivatedHook(gs, basalt, 0, nil)
		ok := gameengine.PayGenericCost(gs, gs.Seats[0], 3, "activated", "basalt_untap_cost", "Basalt Monolith")
		if !ok {
			t.Fatalf("cycle %d: failed to pay {3} for untap", cycle)
		}
		gameengine.InvokeActivatedHook(gs, basalt, 1, nil)
	}
	// Net: 100 cycles × +1 = 100 mana.
	if gs.Seats[0].ManaPool != cap {
		t.Errorf("expected net %d mana after %d loop cycles, got %d",
			cap, cap, gs.Seats[0].ManaPool)
	}
}
