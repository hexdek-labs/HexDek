package per_card

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Fixture helpers
// -----------------------------------------------------------------------------

func newGame(t *testing.T, seats int) *gameengine.GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return gameengine.NewGameState(seats, rng, nil)
}

func addCard(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Card {
	c := &gameengine.Card{
		Name:  name,
		Owner: seat,
		Types: append([]string{}, types...),
	}
	return c
}

func addPerm(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Permanent {
	card := addCard(gs, seat, name, types...)
	p := &gameengine.Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func addLibrary(gs *gameengine.GameState, seat int, names ...string) {
	for _, n := range names {
		c := &gameengine.Card{
			Name:  n,
			Owner: seat,
		}
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, c)
	}
}

// hasEvent returns the count of events with a matching Kind.
func hasEvent(gs *gameengine.GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// -----------------------------------------------------------------------------
// Thassa's Oracle
// -----------------------------------------------------------------------------

func TestThassasOracle_WinsWithEmptyLibrary(t *testing.T) {
	gs := newGame(t, 2)
	// Seat 0: play Thassa's Oracle with devotion = 1 (her own blue pip).
	// Library is empty → 0 <= 1 → win.
	oracle := addPerm(gs, 0, "Thassa's Oracle", "creature", "pip:U", "pip:U")
	// Seat 0's library is empty.
	// Invoke the ETB handler directly (no stack needed for this unit test).
	gameengine.InvokeETBHook(gs, oracle)

	if hasEvent(gs, "per_card_win") < 1 {
		t.Errorf("expected per_card_win event, got %d; events: %+v",
			hasEvent(gs, "per_card_win"), gs.EventLog)
	}
	if !gs.Seats[0].Won {
		t.Errorf("expected seat 0 to be marked Won, got %+v", gs.Seats[0])
	}
	if !gs.Seats[1].Lost {
		t.Errorf("expected seat 1 to be marked Lost")
	}
}

func TestThassasOracle_LosesWithLibraryLargerThanDevotion(t *testing.T) {
	gs := newGame(t, 2)
	// Devotion = 2 (two U pips on the Oracle card).
	// Library has 5 cards → 5 > 2 → no win.
	oracle := addPerm(gs, 0, "Thassa's Oracle", "creature", "pip:U", "pip:U")
	addLibrary(gs, 0, "A", "B", "C", "D", "E")

	gameengine.InvokeETBHook(gs, oracle)

	if hasEvent(gs, "per_card_win") > 0 {
		t.Errorf("should NOT have won, library > devotion")
	}
	if gs.Seats[0].Won {
		t.Errorf("seat 0 should not be Won")
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler breadcrumb")
	}
}

func TestThassasOracle_WinsWithLibraryEqualDevotion(t *testing.T) {
	gs := newGame(t, 2)
	// Devotion = 3 (two pips on Oracle + 1 pip on another blue perm).
	// Library has 3 cards → 3 <= 3 → win.
	oracle := addPerm(gs, 0, "Thassa's Oracle", "creature", "pip:U", "pip:U")
	addPerm(gs, 0, "Island", "land", "pip:U") // devotion bump
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.InvokeETBHook(gs, oracle)

	if hasEvent(gs, "per_card_win") < 1 {
		t.Errorf("expected per_card_win event at library == devotion boundary")
	}
}

// -----------------------------------------------------------------------------
// Demonic Consultation
// -----------------------------------------------------------------------------

func TestDemonicConsultation_EmptiesLibrary(t *testing.T) {
	gs := newGame(t, 2)
	// Seat 0 casts Consultation with no named card in the deck.
	addLibrary(gs, 0, "A", "B", "C", "D", "E", "F", "G", "H", "I", "J")
	card := addCard(gs, 0, "Demonic Consultation", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}

	ok := gameengine.InvokeResolveHook(gs, item)
	if ok < 1 {
		t.Errorf("expected ResolveHook to fire, got %d", ok)
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("expected library empty after Consultation combo line, got %d",
			len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Exile) != 10 {
		t.Errorf("expected 10 cards in exile, got %d", len(gs.Seats[0].Exile))
	}
}

func TestDemonicConsultation_FindsNamedCard(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "D", "E", "F", "G", "H", "Target", "Z")
	card := addCard(gs, 0, "Demonic Consultation", "sorcery", "named:Target")
	item := &gameengine.StackItem{Controller: 0, Card: card}

	gameengine.InvokeResolveHook(gs, item)

	// Top 6 exiled, then reveal until "Target" — positions 6,7 exiled,
	// position 8 is "Target" → to hand, position 9 "Z" stays.
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Target" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Target in hand; hand = %+v", handNames(gs.Seats[0].Hand))
	}
}

func handNames(hand []*gameengine.Card) []string {
	out := []string{}
	for _, c := range hand {
		out = append(out, c.DisplayName())
	}
	return out
}

// -----------------------------------------------------------------------------
// Tainted Pact
// -----------------------------------------------------------------------------

func TestTaintedPact_SingletonDeckEmpties(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "D", "E")
	card := addCard(gs, 0, "Tainted Pact", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}

	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("singleton deck should empty; library has %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Exile) != 5 {
		t.Errorf("expected 5 exiled, got %d", len(gs.Seats[0].Exile))
	}
}

func TestTaintedPact_StopsOnDuplicate(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "A", "D", "E") // "A" duplicate at index 3
	card := addCard(gs, 0, "Tainted Pact", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}

	gameengine.InvokeResolveHook(gs, item)

	// Should exile A, B, C (3 unique), then A again (duplicate) → stop.
	// 4 total exiled, 2 remain.
	if len(gs.Seats[0].Exile) != 4 {
		t.Errorf("expected 4 exiled on duplicate hit, got %d", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(gs.Seats[0].Library))
	}
}

// -----------------------------------------------------------------------------
// Thoracle + Consultation end-to-end
// -----------------------------------------------------------------------------

func TestThoracleConsultationCombo(t *testing.T) {
	gs := newGame(t, 2)
	// Oracle is ALREADY in play.
	oracle := addPerm(gs, 0, "Thassa's Oracle", "creature", "pip:U", "pip:U")
	_ = oracle
	addLibrary(gs, 0, "A", "B", "C", "D", "E", "F", "G", "H")

	// Cast Demonic Consultation (no named card → empties library).
	card := addCard(gs, 0, "Demonic Consultation", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Library now empty. Now re-fire Oracle ETB (simulating a Cloudstone
	// flicker or a fresh cast) — which triggers the win check.
	gameengine.InvokeETBHook(gs, oracle)

	if !gs.Seats[0].Won {
		t.Errorf("Thoracle + Consultation should win; seat 0 state = %+v", gs.Seats[0])
	}
}

// -----------------------------------------------------------------------------
// Doomsday
// -----------------------------------------------------------------------------

func TestDoomsday_PicksFiveExilesRest(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40 // commander format
	addLibrary(gs, 0, "A", "B", "C", "D", "E", "F", "G", "H", "I", "J")

	card := addCard(gs, 0, "Doomsday", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Library) != 5 {
		t.Errorf("expected 5-card library post-Doomsday, got %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Exile) != 5 {
		t.Errorf("expected 5 exiled, got %d", len(gs.Seats[0].Exile))
	}
	// Life: 40 → 40 - ceil(40/2) = 40 - 20 = 20.
	if gs.Seats[0].Life != 20 {
		t.Errorf("expected life 20 after Doomsday, got %d", gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Underworld Breach
// -----------------------------------------------------------------------------

func TestUnderworldBreach_FlagsGraveyardAndRegistersEndStep(t *testing.T) {
	gs := newGame(t, 2)
	breach := addPerm(gs, 0, "Underworld Breach", "enchantment")

	gameengine.InvokeETBHook(gs, breach)

	if breach.Flags["escape_grants_to_graveyard"] != 1 {
		t.Errorf("expected escape flag set on Breach permanent")
	}
	if len(gs.DelayedTriggers) < 1 {
		t.Errorf("expected end-step sacrifice trigger registered")
	}
	if hasEvent(gs, "per_card_partial") < 1 {
		t.Errorf("expected per_card_partial breadcrumb for zone-cast stub")
	}
}

// -----------------------------------------------------------------------------
// Aetherflux Reservoir
// -----------------------------------------------------------------------------

func TestAetherflux_GainsLifeOnCast(t *testing.T) {
	gs := newGame(t, 2)
	reservoir := addPerm(gs, 0, "Aetherflux Reservoir", "artifact")
	_ = reservoir
	startLife := gs.Seats[0].Life

	// Log a cast event manually (mirroring what CastSpell would do).
	gs.LogEvent(gameengine.Event{Kind: "cast", Seat: 0, Source: "Ponder"})
	// Then fire the spell_cast trigger.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	if gs.Seats[0].Life <= startLife {
		t.Errorf("Aetherflux should have gained life; before=%d after=%d", startLife, gs.Seats[0].Life)
	}
}

func TestAetherflux_50DamageActivation(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 60 // enough to pay 50
	gs.Seats[1].Life = 40
	reservoir := addPerm(gs, 0, "Aetherflux Reservoir", "artifact")

	gameengine.InvokeActivatedHook(gs, reservoir, 0, map[string]interface{}{
		"target_seat": 1,
	})

	if gs.Seats[0].Life != 10 {
		t.Errorf("expected seat 0 life 10 after paying 50, got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != -10 {
		t.Errorf("expected seat 1 life -10 after 50 dmg, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Food Chain
// -----------------------------------------------------------------------------

func TestFoodChain_ExileForMana(t *testing.T) {
	gs := newGame(t, 2)
	foodChain := addPerm(gs, 0, "Food Chain", "enchantment")
	// Misthollow Griffin as the creature to exile. Use BasePower/Tough
	// to give it CMC 4 (via cardCMC proxy).
	griffin := &gameengine.Card{
		Name:          "Misthollow Griffin",
		Owner:         0,
		BasePower:     3,
		BaseToughness: 3,
		Types:         []string{"creature", "cmc:4"},
	}
	griffinPerm := &gameengine.Permanent{
		Card:       griffin,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, griffinPerm)

	// Activate Food Chain targeting Griffin.
	gameengine.InvokeActivatedHook(gs, foodChain, 0, map[string]interface{}{
		"creature_perm": griffinPerm,
	})

	// Expected: Griffin exiled; mana pool gets 4+1 = 5.
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("expected mana pool 5, got %d", gs.Seats[0].ManaPool)
	}
	// Griffin should be in exile, not battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == griffinPerm {
			t.Errorf("Griffin should have been removed from battlefield")
		}
	}
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == griffin {
			foundInExile = true
			break
		}
	}
	if !foundInExile {
		t.Errorf("Griffin should be in exile")
	}
}

// -----------------------------------------------------------------------------
// Displacer Kitten
// -----------------------------------------------------------------------------

func TestDisplacerKitten_FlickersOnNoncreatureSpell(t *testing.T) {
	gs := newGame(t, 2)
	kitten := addPerm(gs, 0, "Displacer Kitten", "creature")
	_ = kitten
	target := addPerm(gs, 0, "Sol Ring", "artifact")
	preTimestamp := target.Timestamp

	gameengine.FireCardTrigger(gs, "noncreature_spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	// Target should have been flickered — look for either:
	//   a) a "flicker" event
	//   b) the permanent now has a NEW timestamp (fresh Permanent)
	if hasEvent(gs, "flicker") < 1 {
		t.Errorf("expected flicker event, got %d; events: %+v",
			hasEvent(gs, "flicker"), gs.EventLog)
	}
	// After flicker: original target is no longer on battlefield, a new
	// one with a higher timestamp took its place.
	foundNew := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Sol Ring" && p.Timestamp > preTimestamp {
			foundNew = true
			break
		}
	}
	if !foundNew {
		t.Errorf("expected Sol Ring to have a new, higher timestamp after flicker")
	}
}

// -----------------------------------------------------------------------------
// Rhystic Study
// -----------------------------------------------------------------------------

func TestRhysticStudy_OpponentPaysTax(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Rhystic Study", "enchantment")
	gs.Seats[1].ManaPool = 5
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	// Opponent paid 1, no draw.
	if gs.Seats[1].ManaPool != 4 {
		t.Errorf("expected opponent mana 4 after paying tax, got %d", gs.Seats[1].ManaPool)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("should NOT have drawn if opponent paid; hand size %d", len(gs.Seats[0].Hand))
	}
}

func TestRhysticStudy_OpponentCantPayWeDraw(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Rhystic Study", "enchantment")
	gs.Seats[1].ManaPool = 0
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected to draw 1 when opponent can't pay, got hand %d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Mystic Remora
// -----------------------------------------------------------------------------

func TestMysticRemora_DrawsOnUnpaidNoncreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mystic Remora", "enchantment")
	gs.Seats[1].ManaPool = 3 // can't afford {4}
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.FireCardTrigger(gs, "noncreature_spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Remora to draw when tax unpaid, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestMysticRemora_IgnoresCreatureSpells(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mystic Remora", "enchantment")
	gs.Seats[1].ManaPool = 0
	addLibrary(gs, 0, "A", "B")

	gameengine.FireCardTrigger(gs, "creature_spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Grizzly Bears",
		"is_creature": true,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Remora should NOT fire on creature spells, hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Cloudstone Curio
// -----------------------------------------------------------------------------

func TestCloudstoneCurio_BouncesOldestOnETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Cloudstone Curio", "artifact")
	oldRock := addPerm(gs, 0, "Sol Ring", "artifact")

	// Simulate a fresh ETB event for a new nonland permanent.
	newCreature := addPerm(gs, 0, "Llanowar Elves", "creature")
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            newCreature,
		"controller_seat": 0,
		"card":            newCreature.Card,
	})

	// oldRock should have been bounced.
	for _, p := range gs.Seats[0].Battlefield {
		if p == oldRock {
			t.Errorf("Sol Ring should have been bounced by Cloudstone Curio")
		}
	}
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Sol Ring" {
			foundInHand = true
			break
		}
	}
	if !foundInHand {
		t.Errorf("Sol Ring should be in hand after Cloudstone bounce")
	}
}

// -----------------------------------------------------------------------------
// Hullbreaker Horror
// -----------------------------------------------------------------------------

func TestHullbreakerHorror_BouncesOpponentOnCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Hullbreaker Horror", "creature")
	oppThreat := addPerm(gs, 1, "Lotus Petal", "artifact")

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	// Opponent's Lotus Petal should be bounced.
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppThreat {
			t.Errorf("opponent's Lotus Petal should have been bounced")
		}
	}
}

// -----------------------------------------------------------------------------
// Necrotic Ooze
// -----------------------------------------------------------------------------

func TestNecroticOoze_CollectsActivatedAbilitiesFromGraveyards(t *testing.T) {
	gs := newGame(t, 2)
	// Put a creature card with an activated ability into a graveyard.
	// Without a real AST, we check that the handler runs end-to-end and
	// produces a granted_count of 0 (empty graveyards give 0 grants).
	ooze := addPerm(gs, 0, "Necrotic Ooze", "creature")
	gameengine.InvokeETBHook(gs, ooze)

	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler event")
	}
	// Without AST-bearing graveyard cards, grants should be 0.
	if ooze.Flags["granted_activated_count"] != 0 {
		t.Errorf("expected 0 grants with empty graveyards, got %d",
			ooze.Flags["granted_activated_count"])
	}
}

// -----------------------------------------------------------------------------
// Hermit Druid
// -----------------------------------------------------------------------------

func TestHermitDruid_MillsUntilBasicLand(t *testing.T) {
	gs := newGame(t, 2)
	druid := addPerm(gs, 0, "Hermit Druid", "creature")
	// Library: 3 non-basics then a basic.
	addLibrary(gs, 0, "Wheel of Fortune", "Ad Nauseam", "Brainstorm")
	// Append a basic land.
	basic := &gameengine.Card{
		Name:  "Island",
		Owner: 0,
		Types: []string{"basic", "land"},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, basic)

	gameengine.InvokeActivatedHook(gs, druid, 0, nil)

	// 3 non-basics milled, 1 basic to hand.
	if len(gs.Seats[0].Graveyard) != 3 {
		t.Errorf("expected 3 milled, got %d", len(gs.Seats[0].Graveyard))
	}
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Island" {
			foundInHand = true
			break
		}
	}
	if !foundInHand {
		t.Errorf("expected Island in hand")
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("library should be empty after Hermit Druid, got %d", len(gs.Seats[0].Library))
	}
}

func TestHermitDruid_NoBasicsMillsEverything(t *testing.T) {
	gs := newGame(t, 2)
	druid := addPerm(gs, 0, "Hermit Druid", "creature")
	addLibrary(gs, 0, "A", "B", "C", "D", "E") // no basics

	gameengine.InvokeActivatedHook(gs, druid, 0, nil)

	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("no-basics deck should empty library, got %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Graveyard) != 5 {
		t.Errorf("expected 5 milled, got %d", len(gs.Seats[0].Graveyard))
	}
}

// -----------------------------------------------------------------------------
// Walking Ballista
// -----------------------------------------------------------------------------

func TestWalkingBallista_ETBWithXCounters(t *testing.T) {
	gs := newGame(t, 2)
	// Flag X=4 for the ETB.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["_ballista_x_0"] = 4

	ballista := addPerm(gs, 0, "Walking Ballista", "creature", "artifact")
	gameengine.InvokeETBHook(gs, ballista)

	if ballista.Counters["+1/+1"] != 4 {
		t.Errorf("expected 4 +1/+1 counters on Ballista, got %d", ballista.Counters["+1/+1"])
	}
}

func TestWalkingBallista_RemoveCounterDealsDamage(t *testing.T) {
	gs := newGame(t, 2)
	ballista := addPerm(gs, 0, "Walking Ballista", "creature", "artifact")
	ballista.AddCounter("+1/+1", 5)

	// Activate ability 1 (remove counter → 1 dmg to seat 1).
	gameengine.InvokeActivatedHook(gs, ballista, 1, map[string]interface{}{
		"target_seat": 1,
	})

	if ballista.Counters["+1/+1"] != 4 {
		t.Errorf("expected 4 counters after removal, got %d", ballista.Counters["+1/+1"])
	}
	if gs.Seats[1].Life != 19 {
		t.Errorf("expected seat 1 life 19 after 1 dmg, got %d", gs.Seats[1].Life)
	}
}

func TestWalkingBallista_CantShootWithNoCounters(t *testing.T) {
	gs := newGame(t, 2)
	ballista := addPerm(gs, 0, "Walking Ballista", "creature", "artifact")

	gameengine.InvokeActivatedHook(gs, ballista, 1, map[string]interface{}{
		"target_seat": 1,
	})

	// Should fail gracefully.
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no counters; events: %+v", gs.EventLog)
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("seat 1 should not take damage with no counters")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke test
// -----------------------------------------------------------------------------

func TestRegistry_AllFifteenCardsRegistered(t *testing.T) {
	cards := []struct {
		name    string
		kind    string // "etb", "resolve", "activated"
	}{
		{"Thassa's Oracle", "etb"},
		{"Demonic Consultation", "resolve"},
		{"Tainted Pact", "resolve"},
		{"Underworld Breach", "etb"},
		{"Aetherflux Reservoir", "activated"},
		{"Food Chain", "activated"},
		{"Doomsday", "resolve"},
		{"Displacer Kitten", "etb"}, // registered as trigger but we check etb/trigger below
		{"Necrotic Ooze", "etb"},
		{"Walking Ballista", "etb"},
	}
	// Check ETB / resolve / activated directly.
	for _, c := range cards {
		switch c.kind {
		case "etb":
			if c.name == "Displacer Kitten" {
				// kitten has trigger, not etb
				continue
			}
			if !HasETB(c.name) {
				t.Errorf("expected ETB handler for %s", c.name)
			}
		case "resolve":
			if !HasResolve(c.name) {
				t.Errorf("expected Resolve handler for %s", c.name)
			}
		case "activated":
			if !HasActivated(c.name) {
				t.Errorf("expected Activated handler for %s", c.name)
			}
		}
	}
	// Trigger-only cards: Rhystic Study, Mystic Remora, Cloudstone Curio,
	// Hullbreaker Horror, Displacer Kitten. Check via a cast event +
	// hasEvent on per_card_handler/per_card_failed.
	triggerCards := []string{
		"Rhystic Study", "Mystic Remora", "Cloudstone Curio",
		"Hullbreaker Horror", "Displacer Kitten",
	}
	for _, name := range triggerCards {
		// Not checkable via HasETB/HasResolve/HasActivated — assert via
		// a dummy game that the handler fires.
		gs := newGame(t, 2)
		addPerm(gs, 0, name, "artifact", "creature", "enchantment") // fuzzy types
		// Pick an event likely to fire each.
		var event string
		switch name {
		case "Rhystic Study":
			event = "spell_cast_by_opponent"
		case "Mystic Remora":
			event = "noncreature_spell_cast"
		case "Cloudstone Curio":
			event = "nonland_permanent_etb"
		case "Hullbreaker Horror":
			event = "spell_cast"
		case "Displacer Kitten":
			event = "noncreature_spell_cast"
		}
		before := len(gs.EventLog)
		// Set up mana / extra perms so handlers don't early-bail too hard.
		gs.Seats[1].ManaPool = 0 // cheapskate opponent
		addLibrary(gs, 0, "A")   // for Rhystic/Remora draws
		extra := addPerm(gs, 0, "Sol Ring", "artifact")
		_ = extra

		// Default caster: opponent (seat 1) for opponent-scoped triggers
		// (Rhystic, Remora). Self (seat 0) for own-cast triggers
		// (Hullbreaker, Displacer, Aetherflux).
		casterSeat := 1
		switch name {
		case "Hullbreaker Horror", "Displacer Kitten", "Aetherflux Reservoir":
			casterSeat = 0
		}
		ctx := map[string]interface{}{
			"caster_seat": casterSeat,
			"spell_name":  "X",
			"is_creature": false,
		}
		if event == "nonland_permanent_etb" {
			// Cloudstone/Kitten look for a "perm" in ctx.
			newbie := addPerm(gs, 0, "Grizzly Bears", "creature")
			ctx["perm"] = newbie
			ctx["controller_seat"] = 0
			ctx["caster_seat"] = 0
		}
		gameengine.FireCardTrigger(gs, event, ctx)
		after := len(gs.EventLog)
		if after <= before {
			t.Errorf("%s: no events fired on %q trigger (before=%d after=%d)",
				name, event, before, after)
		}
	}
}

func TestSimicBasilisk_GrantsBasiliskAbility(t *testing.T) {
	gs := newGame(t, 4)

	basilisk := addPerm(gs, 0, "Simic Basilisk", "creature")
	basilisk.Card.BasePower = 0
	basilisk.Card.BaseToughness = 0
	basilisk.Counters["counter:+1/+1"] = 3
	basilisk.Flags["counter:+1/+1"] = 3

	recipient := addPerm(gs, 0, "Grizzly Bears", "creature")
	recipient.Card.BasePower = 2
	recipient.Card.BaseToughness = 2
	recipient.Flags["counter:+1/+1"] = 1

	gameengine.InvokeActivatedHook(gs, basilisk, 0, map[string]interface{}{
		"controller": 0,
	})

	if recipient.Flags["basilisk_granted"] != 1 {
		t.Fatal("Simic Basilisk ability should grant basilisk_granted flag to creature with +1/+1 counter")
	}

	if hasEvent(gs, "basilisk_ability_granted") == 0 {
		t.Fatal("expected basilisk_ability_granted event")
	}
}

func TestSimicBasilisk_CombatDamageMarksDestroy(t *testing.T) {
	gs := newGame(t, 4)

	attacker := addPerm(gs, 0, "Grizzly Bears", "creature")
	attacker.Card.BasePower = 3
	attacker.Card.BaseToughness = 3
	attacker.Flags["basilisk_granted"] = 1

	blocker := addPerm(gs, 1, "Hill Giant", "creature")
	blocker.Card.BasePower = 3
	blocker.Card.BaseToughness = 3

	gameengine.DealCombatDamageStep(gs, []*gameengine.Permanent{attacker},
		map[*gameengine.Permanent][]*gameengine.Permanent{attacker: {blocker}}, false)

	if blocker.Flags["basilisk_marked_destroy"] != 1 {
		t.Fatal("basilisk-granted creature should mark blocker for destruction on combat damage")
	}
	if attacker.Flags["basilisk_combat_hit"] != 1 {
		t.Fatal("basilisk-granted creature should have basilisk_combat_hit flag set")
	}
}
