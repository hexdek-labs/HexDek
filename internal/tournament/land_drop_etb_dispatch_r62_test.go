package tournament

// Regression tests for the land-drop ETB-dispatcher bypass (r62, fleet
// review report 02 C-1).
//
// Background: tryPlayLand hand-rolled its own battlefield entry — it
// appended the Permanent and called only InvokeETBHook plus a bare
// FireCardTrigger("permanent_etb"). It never called
// RegisterReplacementsForPermanent or FirePermanentETBTriggers, so a
// land played from hand skipped RegisterContinuousEffectsForPermanent
// (named layer handlers AND generic AST statics), ApplyStaticETBCounters,
// self-AST ETB triggers, and the AST observer fan-out. Urborg, Tomb of
// Yawgmoth — whose "each land is a Swamp in addition to its other land
// types" lives in the named layer dispatch (layers.go RegisterUrborg) —
// was therefore silently inert whenever it was played from hand: the
// most common land-entry path, ~30+ times per simulated game. This is
// the same bug class PR #999 (ec3c555c) closed for the reanimate /
// token-mint / blink entry paths, surviving on the land-drop path.
//
// The fix routes tryPlayLand through the same two-call entry pattern
// every other path uses (see resolve.go placeTutoredCard):
// RegisterReplacementsForPermanent + FirePermanentETBTriggers.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// urborgCard builds a synthetic Urborg, Tomb of Yawgmoth. The named
// layer dispatch keys on DisplayName(), so Name is the load-bearing
// field; the real card's type line is "Legendary Land" with no printed
// land subtypes.
func urborgCard() *gameengine.Card {
	return &gameengine.Card{
		Name:     "Urborg, Tomb of Yawgmoth",
		Types:    []string{"legendary", "land"},
		TypeLine: "legendary land",
	}
}

// plainMountain builds a basic Mountain — printed subtype "mountain",
// no swamp anywhere. Proves the layer-4 subtype ADD (not a printed
// type) is what makes it a Swamp.
func plainMountain() *gameengine.Card {
	return &gameengine.Card{
		Name:     "Mountain",
		Types:    []string{"basic", "land", "mountain"},
		TypeLine: "basic land — mountain",
	}
}

// TestLandDrop_Urborg_RegistersContinuousEffect is the headline
// regression: playing Urborg FROM HAND must register its layer-4
// continuous effect, making every land on the battlefield a Swamp.
// Pre-fix, tryPlayLand bypassed RegisterContinuousEffectsForPermanent
// and both assertions below fail.
func TestLandDrop_Urborg_RegistersContinuousEffect(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	seat := gs.Seats[0]
	seat.Hat = &hat.GreedyHat{}

	// A Mountain already on the battlefield (entered by direct
	// construction — its own registration path is not under test).
	mountain := &gameengine.Permanent{
		Card:       plainMountain(),
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, mountain)

	if gs.HasSubtypeOf(mountain, "swamp") {
		t.Fatal("sanity: Mountain must not be a Swamp before Urborg is played")
	}

	// Play Urborg from hand via the land-drop path.
	seat.Hand = append(seat.Hand, urborgCard())
	tryPlayLand(gs, 0)

	if len(seat.Battlefield) != 2 {
		t.Fatalf("expected Urborg on battlefield (2 perms), got %d", len(seat.Battlefield))
	}
	urborg := seat.Battlefield[1]
	if urborg.Card.DisplayName() != "Urborg, Tomb of Yawgmoth" {
		t.Fatalf("expected Urborg as second permanent, got %q", urborg.Card.DisplayName())
	}

	// The continuous effect must apply to OTHER lands…
	if !gs.HasSubtypeOf(mountain, "swamp") {
		t.Error("Mountain is not a Swamp after Urborg was played from hand — " +
			"RegisterContinuousEffectsForPermanent was bypassed by the land-drop path")
	}
	// …and to Urborg itself ("each land", including the source).
	if !gs.HasSubtypeOf(urborg, "swamp") {
		t.Error("Urborg itself is not a Swamp after being played from hand")
	}
	// The Mountain keeps its own land type ("in addition to its other
	// land types") — the effect is a layer-4 ADD, not a replacement.
	// Note the engine's representational split: printed land types ride
	// in the flat Types list (parseTypes model), while layer-granted
	// subtypes land in Characteristics.Subtypes — so probe Types here.
	mountainStillMountain := false
	for _, ty := range gameengine.GetEffectiveCharacteristics(gs, mountain).Types {
		if ty == "mountain" {
			mountainStillMountain = true
			break
		}
	}
	if !mountainStillMountain {
		t.Error("Mountain lost its printed land type — Urborg should ADD swamp, not replace")
	}
}

// TestLandDrop_Urborg_AppliesToLandsPlayedLater pins the other
// direction: the registered effect is predicate-driven ("as long as"),
// so a land played AFTER Urborg must also be a Swamp the moment it
// enters via the land-drop path.
func TestLandDrop_Urborg_AppliesToLandsPlayedLater(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	seat := gs.Seats[0]
	seat.Hat = &hat.GreedyHat{}

	seat.Hand = append(seat.Hand, urborgCard())
	tryPlayLand(gs, 0)
	if len(seat.Battlefield) != 1 {
		t.Fatalf("expected Urborg on battlefield, got %d perms", len(seat.Battlefield))
	}

	// New turn-equivalent: clear the land-drop marker, then play a
	// Mountain from hand.
	clearPlayedLand(gs, 0)
	seat.Hand = append(seat.Hand, plainMountain())
	tryPlayLand(gs, 0)
	if len(seat.Battlefield) != 2 {
		t.Fatalf("expected 2 permanents after second land drop, got %d", len(seat.Battlefield))
	}
	mountain := seat.Battlefield[1]
	if !gs.HasSubtypeOf(mountain, "swamp") {
		t.Error("Mountain played after Urborg is not a Swamp — predicate-driven layer effect missing")
	}
}

// TestLandDrop_StillMarksLandDropAndLogs pins the parts of tryPlayLand
// that must SURVIVE the dispatcher routing: the §305.1 one-land-per-turn
// marker and the play_land event. (The pre-existing MDFC tests in
// mdfc_play_test.go pin face cleanup and ETB-tapped detection.)
func TestLandDrop_StillMarksLandDropAndLogs(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.RetainEvents = true
	seat := gs.Seats[0]
	seat.Hat = &hat.GreedyHat{}
	seat.Hand = append(seat.Hand, plainMountain())

	tryPlayLand(gs, 0)

	if len(seat.Battlefield) != 1 {
		t.Fatalf("expected the Mountain on battlefield, got %d perms", len(seat.Battlefield))
	}
	// The §305.1 marker the turn loop gates on (turn.go gates the
	// tryPlayLand CALL on !playedLandThisTurn) must still be set.
	if !playedLandThisTurn(gs, 0) {
		t.Error("land-drop marker not set after tryPlayLand")
	}
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "play_land" && ev.Source == "Mountain" {
			found = true
			break
		}
	}
	if !found {
		t.Error("play_land event not logged")
	}
}
