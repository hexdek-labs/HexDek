package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Visit tests — CR §702.177
// ---------------------------------------------------------------------------

func newVisitGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(45))
	return NewGameState(2, rng, nil)
}

func newRoomPermanent(name string, owner int) *Permanent {
	card := &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"enchantment", "room"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "visit"},
			},
		},
	}
	return &Permanent{
		Card:       card,
		Controller: owner,
		Owner:      owner,
		Flags:      map[string]int{},
	}
}

// ---------------------------------------------------------------------------
// HasVisit
// ---------------------------------------------------------------------------

func TestHasVisit_Detects(t *testing.T) {
	card := newRoomPermanent("Cozy Den", 0).Card
	if !HasVisit(card) {
		t.Fatal("HasVisit should detect visit keyword on the card")
	}
}

func TestHasVisit_Negative(t *testing.T) {
	card := newInstantCard("Lightning Bolt", 0, 1)
	if HasVisit(card) {
		t.Fatal("HasVisit should return false for a card without the visit keyword")
	}
}

// ---------------------------------------------------------------------------
// ApplyVisit / VisitedThisTurn
// ---------------------------------------------------------------------------

func TestApplyVisit_SetsFlagAndReturnsCount(t *testing.T) {
	gs := newVisitGame(t)
	perm := newRoomPermanent("Cozy Den", 0)

	count := ApplyVisit(gs, 0, perm)
	if count != 1 {
		t.Fatalf("first visit should return 1, got %d", count)
	}
	if !VisitedThisTurn(perm) {
		t.Fatal("VisitedThisTurn should be true after first ApplyVisit")
	}

	count = ApplyVisit(gs, 1, perm)
	if count != 2 {
		t.Fatalf("second visit should return 2, got %d", count)
	}
	if perm.Flags["visited_this_turn"] != 2 {
		t.Fatalf("visited_this_turn counter = %d, want 2", perm.Flags["visited_this_turn"])
	}
}

func TestApplyVisit_EmitsEvent(t *testing.T) {
	gs := newVisitGame(t)
	perm := newRoomPermanent("Cozy Den", 0)
	before := len(gs.EventLog)
	ApplyVisit(gs, 1, perm)
	var found *Event
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "visit" {
			found = &gs.EventLog[i]
			break
		}
	}
	if found == nil {
		t.Fatal("ApplyVisit should emit a visit event")
	}
	if found.Seat != 1 {
		t.Fatalf("event Seat = %d, want 1 (visitor)", found.Seat)
	}
	if found.Source != "Cozy Den" {
		t.Fatalf("event Source = %q, want \"Cozy Den\"", found.Source)
	}
	if found.Amount != 1 {
		t.Fatalf("event Amount = %d, want 1 (first visit count)", found.Amount)
	}
}

func TestApplyVisit_FiresTriggerHook(t *testing.T) {
	gs := newVisitGame(t)
	perm := newRoomPermanent("Cozy Den", 0)

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var sawEvent string
	var sawPerm *Permanent
	var sawVisitor int = -1
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "visited" {
			sawEvent = ev
			if p, ok := ctx["perm"].(*Permanent); ok {
				sawPerm = p
			}
			if v, ok := ctx["visitor_seat"].(int); ok {
				sawVisitor = v
			}
		}
	}

	ApplyVisit(gs, 1, perm)
	if sawEvent != "visited" {
		t.Fatal("TriggerHook did not observe visited event")
	}
	if sawPerm != perm {
		t.Fatal("trigger ctx[\"perm\"] should reference the visited permanent")
	}
	if sawVisitor != 1 {
		t.Fatalf("trigger ctx[\"visitor_seat\"] = %d, want 1", sawVisitor)
	}
}

func TestApplyVisit_NilSafe(t *testing.T) {
	gs := newVisitGame(t)
	if got := ApplyVisit(gs, 0, nil); got != 0 {
		t.Fatalf("ApplyVisit(nil perm) should return 0, got %d", got)
	}
	if got := ApplyVisit(nil, 0, newRoomPermanent("Cozy Den", 0)); got != 0 {
		t.Fatalf("ApplyVisit(nil gs) should return 0, got %d", got)
	}
}

func TestVisitedThisTurn_Default(t *testing.T) {
	perm := newRoomPermanent("Cozy Den", 0)
	if VisitedThisTurn(perm) {
		t.Fatal("VisitedThisTurn should be false on a freshly created perm")
	}
	if VisitedThisTurn(nil) {
		t.Fatal("VisitedThisTurn(nil) should be false")
	}
}

// ---------------------------------------------------------------------------
// ClearVisitFlags — EOT cleanup
// ---------------------------------------------------------------------------

func TestClearVisitFlags_ResetsAcrossBattlefields(t *testing.T) {
	gs := newVisitGame(t)
	pA := newRoomPermanent("Cozy Den", 0)
	pB := newRoomPermanent("Bramble Burrow", 1)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, pA)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, pB)

	ApplyVisit(gs, 0, pA)
	ApplyVisit(gs, 0, pA)
	ApplyVisit(gs, 1, pB)
	if !VisitedThisTurn(pA) || !VisitedThisTurn(pB) {
		t.Fatal("expected both perms to be visited before cleanup")
	}

	ClearVisitFlags(gs)
	if VisitedThisTurn(pA) {
		t.Fatal("pA should no longer be visited after ClearVisitFlags")
	}
	if VisitedThisTurn(pB) {
		t.Fatal("pB should no longer be visited after ClearVisitFlags")
	}
	if _, ok := pA.Flags["visited_this_turn"]; ok {
		t.Fatal("ClearVisitFlags should delete the flag, not zero it")
	}
}

func TestClearVisitFlags_NilSafe(t *testing.T) {
	// "Doesn't panic" assertion made explicit via recover() — pre-Phase-2B
	// the test had no runtime check; the assertion was implicit.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ClearVisitFlags(nil) must not panic; got %v", r)
		}
	}()
	ClearVisitFlags(nil)
}
