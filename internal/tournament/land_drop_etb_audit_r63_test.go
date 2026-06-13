package tournament

// land_drop_etb_audit_r63_test.go — CR-compliance audit of the
// LAND-DROP → ETB pipeline (CR §305 / §116.2a special action).
//
// Verifies the five properties of a land drop:
//  1. tapped/untapped state at entry (§614 ETB-tapped),
//  2. landfall fires for the LAND'S CONTROLLER and stays silent for an
//     opponent's land (controller-restricted "lands you control"),
//  3. the land's own ETB pipeline runs,
//  4. the land-drop allowance is counted and extra-land-drops
//     (Exploration/Azusa/Dryad/Gitrog + one-shot "additional land this
//     turn" effects) are honored,
//  5. playing a land does NOT use the stack.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

func basicForest() *gameengine.Card {
	return &gameengine.Card{
		Name: "Forest", Types: []string{"basic", "land", "forest"},
		TypeLine: "basic land — forest",
	}
}

func etbTappedLand() *gameengine.Card {
	// Path-1 detection: the "etb_tapped" type tag (set by AST/extensions
	// for guildgate/tap-dual/tri-land style unconditional ETB-tapped lands).
	return &gameengine.Card{
		Name: "Tap Dual", Types: []string{"land", "etb_tapped"},
		TypeLine: "land",
	}
}

func newLandTestGS(seats int) *gameengine.GameState {
	gs := gameengine.NewGameState(seats, rand.New(rand.NewSource(1)), nil)
	gs.EventPolicy = gameengine.EventLogFull
	for i := 0; i < seats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}
	return gs
}

// (1)+(3)+(5): a normal land drop enters UNTAPPED, on the battlefield, is
// counted, logs play_land, runs its ETB pipeline, and uses NO stack.
func TestLandDrop_Normal_UntappedNoStackCounted(t *testing.T) {
	gs := newLandTestGS(2)
	seat := gs.Seats[0]
	seat.Hand = append(seat.Hand, basicForest())

	stackBefore := len(gs.Stack)
	if !tryPlayLand(gs, 0) {
		t.Fatal("tryPlayLand returned false for a Forest in hand")
	}
	if len(seat.Battlefield) != 1 {
		t.Fatalf("expected Forest on battlefield, got %d perms", len(seat.Battlefield))
	}
	if seat.Battlefield[0].Tapped {
		t.Error("basic Forest entered TAPPED — should be untapped")
	}
	if got := landsPlayedThisTurn(gs, 0); got != 1 {
		t.Errorf("lands-played count = %d, want 1", got)
	}
	// (5) No stack window for the land itself.
	if len(gs.Stack) != stackBefore {
		t.Errorf("playing a land pushed to the stack (depth %d → %d) — land plays are a special action, not a spell",
			stackBefore, len(gs.Stack))
	}
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "play_land" && ev.Source == "Forest" {
			found = true
		}
	}
	if !found {
		t.Error("play_land event not logged")
	}
}

// (1): an "enters tapped" land enters TAPPED.
func TestLandDrop_EntersTapped(t *testing.T) {
	gs := newLandTestGS(2)
	seat := gs.Seats[0]
	seat.Hand = append(seat.Hand, etbTappedLand())
	if !tryPlayLand(gs, 0) {
		t.Fatal("tryPlayLand returned false")
	}
	if len(seat.Battlefield) != 1 {
		t.Fatalf("expected the land on battlefield, got %d", len(seat.Battlefield))
	}
	if !seat.Battlefield[0].Tapped {
		t.Error("etb_tapped land entered UNTAPPED — §614 ETB-tapped not applied")
	}
}

// (2) self: a controller-restricted landfall ("whenever a land YOU CONTROL
// enters") fires when the controller plays a land. Aesi, Tyrant of Gyre
// Strait draws a card on landfall — observed via the controller's library
// shrinking by one.
func TestLandDrop_Landfall_SelfFires(t *testing.T) {
	gs := newLandTestGS(2)
	seat := gs.Seats[0]
	// Stock the library so Aesi's landfall draw has a card to take.
	for i := 0; i < 5; i++ {
		seat.Library = append(seat.Library, basicForest())
	}
	libBefore := len(seat.Library)

	aesi := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Aesi, Tyrant of Gyre Strait", Types: []string{"legendary", "creature"}},
		Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, aesi)

	seat.Hand = append(seat.Hand, basicForest())
	if !tryPlayLand(gs, 0) {
		t.Fatal("tryPlayLand returned false")
	}
	if len(seat.Library) != libBefore-1 {
		t.Errorf("Aesi landfall draw did NOT fire on the controller's land: library %d → %d (want -1)",
			libBefore, len(seat.Library))
	}
}

// (2) opponent: that SAME controller-restricted landfall must STAY SILENT
// when an OPPONENT plays a land (CR: "lands you control"). This proves the
// ETB dispatch scopes landfall by controller across seats.
func TestLandDrop_Landfall_OpponentLandStaysSilent(t *testing.T) {
	gs := newLandTestGS(2)
	owner := gs.Seats[0]
	for i := 0; i < 5; i++ {
		owner.Library = append(owner.Library, basicForest())
	}
	libBefore := len(owner.Library)

	aesi := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Aesi, Tyrant of Gyre Strait", Types: []string{"legendary", "creature"}},
		Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	owner.Battlefield = append(owner.Battlefield, aesi)

	// Opponent (seat 1) plays the land.
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, basicForest())
	if !tryPlayLand(gs, 1) {
		t.Fatal("tryPlayLand returned false for seat 1")
	}
	if len(owner.Library) != libBefore {
		t.Errorf("Aesi (seat 0) landfall fired on an OPPONENT's land: library %d → %d — landfall must be controller-restricted",
			libBefore, len(owner.Library))
	}
}

// (4) direct: the land-drop allowance honors both buckets, and the
// one-shot bucket expires at turn start.
func TestLandDropAllowance(t *testing.T) {
	gs := newLandTestGS(2)
	if a := landDropAllowance(gs, 0); a != 1 {
		t.Fatalf("base allowance = %d, want 1", a)
	}
	gs.Seats[0].Flags["extra_land_drops"] = 1         // continuous (Dryad-style)
	gs.Seats[0].Flags["extra_land_drops_oneshot"] = 2 // one-shot spell grants
	if a := landDropAllowance(gs, 0); a != 4 {
		t.Fatalf("allowance with 1 continuous + 2 one-shot = %d, want 4", a)
	}
	// After two plays, still under a 4-allowance.
	setPlayedLand(gs, 0)
	setPlayedLand(gs, 0)
	if !canPlayAnotherLand(gs, 0) {
		t.Error("expected to still be allowed a 3rd land under allowance 4")
	}
	// Turn start expires the one-shot bucket (continuous persists).
	clearPlayedLand(gs, 0)
	clearOneShotLandDrops(gs, 0)
	if a := landDropAllowance(gs, 0); a != 2 {
		t.Errorf("after one-shot expiry, allowance = %d, want 2 (base 1 + continuous 1)", a)
	}
}

// (4) integration: with an extra land drop granted, the main phase plays
// TWO lands in one turn. Pre-fix the gate was a boolean and the second
// land was silently refused.
func TestLandDrop_ExtraLandDrop_PlaysTwoLands(t *testing.T) {
	gs := newLandTestGS(2)
	seat := gs.Seats[0]
	seat.Flags["extra_land_drops"] = 1 // e.g. Exploration / Dryad on board
	seat.Hand = append(seat.Hand, basicForest(), basicForest())

	runMainPhase(gs, 0, true)

	lands := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsLand() {
			lands++
		}
	}
	if lands != 2 {
		t.Fatalf("with an extra land drop, expected 2 lands played, got %d (allowance ignored?)", lands)
	}
	if got := landsPlayedThisTurn(gs, 0); got != 2 {
		t.Errorf("lands-played count = %d, want 2", got)
	}
}

// (4) regression: WITHOUT an extra drop, only ONE land is played even with
// two in hand (the §305.2 default limit still holds).
func TestLandDrop_NoExtraDrop_PlaysOneLand(t *testing.T) {
	gs := newLandTestGS(2)
	seat := gs.Seats[0]
	seat.Hand = append(seat.Hand, basicForest(), basicForest())

	runMainPhase(gs, 0, true)

	lands := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsLand() {
			lands++
		}
	}
	if lands != 1 {
		t.Fatalf("without an extra drop, expected exactly 1 land, got %d", lands)
	}
}
