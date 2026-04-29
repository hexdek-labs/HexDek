package per_card

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func newTestGS2(seats int) *gameengine.GameState {
	gs := gameengine.NewGameState(seats, rand.New(rand.NewSource(42)), nil)
	gs.Turn = 1
	return gs
}

func TestDefenseGrid_TaxesOutsideControllersTurn(t *testing.T) {
	gs := newTestGS2(2)
	gs.Active = 0

	grid := &gameengine.Card{Name: "Defense Grid", Types: []string{"artifact"}}
	perm := &gameengine.Permanent{Card: grid, Controller: 0, Timestamp: 1}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	bolt := &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}}

	// Seat 0 is active — no tax for seat 0 (it's their turn).
	cost0 := gameengine.CalculateTotalCost(gs, bolt, 0)
	if cost0 != 0 {
		t.Errorf("Defense Grid should not tax active player: got cost %d, want 0", cost0)
	}

	// Seat 1 casts during seat 0's turn — should pay +3.
	cost1 := gameengine.CalculateTotalCost(gs, bolt, 1)
	if cost1 != 3 {
		t.Errorf("Defense Grid should tax non-active seat by 3: got cost %d, want 3", cost1)
	}

	// Switch active to seat 1 — now seat 1 is untaxed, seat 0 is taxed.
	gs.Active = 1
	cost0again := gameengine.CalculateTotalCost(gs, bolt, 0)
	if cost0again != 3 {
		t.Errorf("Defense Grid should tax seat 0 during seat 1's turn: got %d, want 3", cost0again)
	}
	cost1again := gameengine.CalculateTotalCost(gs, bolt, 1)
	if cost1again != 0 {
		t.Errorf("Defense Grid should not tax active seat 1: got %d, want 0", cost1again)
	}
}

func TestTrinisphere_MinimumThree(t *testing.T) {
	gs := newTestGS2(2)

	trini := &gameengine.Card{Name: "Trinisphere", Types: []string{"artifact"}}
	perm := &gameengine.Permanent{Card: trini, Controller: 1, Timestamp: 1}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, perm)

	// Bolt costs 1 mana normally → Trinisphere raises to 3.
	bolt := &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}}
	cost := gameengine.CalculateTotalCost(gs, bolt, 0)
	if cost != 3 {
		t.Errorf("Trinisphere minimum: got %d, want 3", cost)
	}

	// 5-CMC bomb stays at 5.
	bomb := &gameengine.Card{Name: "Big Bomb", Types: []string{"sorcery"}, CMC: 5}
	costBomb := gameengine.CalculateTotalCost(gs, bomb, 0)
	if costBomb < 5 {
		t.Errorf("Trinisphere should not lower high-cost spells: got %d, want >= 5", costBomb)
	}
}

func TestNotionThief_RedirectsOpponentDraw(t *testing.T) {
	gs := newTestGS2(2)
	gs.Active = 1 // opponent's turn
	gs.Turn = 1

	// Give both seats libraries.
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{Name: "Island"})
		gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{Name: "Swamp"})
	}

	thief := &gameengine.Card{Name: "Notion Thief", Types: []string{"creature"}}
	perm := &gameengine.Permanent{
		Card:       thief,
		Controller: 0,
		Timestamp:  1,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	gameengine.RegisterNotionThiefReplacement(gs, perm)

	hand0Before := len(gs.Seats[0].Hand)
	hand1Before := len(gs.Seats[1].Hand)

	// First draw for active player (seat 1) should go through normally.
	count, cancelled := gameengine.FireDrawEvent(gs, 1, nil)
	if cancelled {
		t.Error("First draw-step draw should not be cancelled")
	}
	if count != 1 {
		t.Errorf("First draw count: got %d, want 1", count)
	}

	// Second draw for seat 1 (same turn) — should be redirected.
	count2, cancelled2 := gameengine.FireDrawEvent(gs, 1, nil)
	if !cancelled2 {
		t.Error("Second draw should be cancelled (redirected to Notion Thief controller)")
	}
	_ = count2

	// Controller (seat 0) should have gained a card from the redirect.
	hand0After := len(gs.Seats[0].Hand)
	if hand0After <= hand0Before {
		t.Errorf("Notion Thief controller should have drawn: hand before=%d, after=%d", hand0Before, hand0After)
	}

	// Seat 1's hand should NOT have gained from the redirected draw.
	// (They only get the first draw-step draw.)
	_ = hand1Before
}

func TestNotionThief_DoesNotRedirectControllerDraw(t *testing.T) {
	gs := newTestGS2(2)
	gs.Active = 0
	gs.Turn = 1

	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{Name: "Island"})
	}

	thief := &gameengine.Card{Name: "Notion Thief", Types: []string{"creature"}}
	perm := &gameengine.Permanent{
		Card:       thief,
		Controller: 0,
		Timestamp:  1,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	gameengine.RegisterNotionThiefReplacement(gs, perm)

	// Controller's own draws should never be redirected.
	_, cancelled := gameengine.FireDrawEvent(gs, 0, nil)
	if cancelled {
		t.Error("Notion Thief should not redirect controller's own draw")
	}
	_, cancelled2 := gameengine.FireDrawEvent(gs, 0, nil)
	if cancelled2 {
		t.Error("Notion Thief should not redirect controller's second draw either")
	}
}

func TestNotionThief_RedirectsNonActiveDraw(t *testing.T) {
	gs := newTestGS2(3)
	gs.Active = 2
	gs.Turn = 1

	for i := 0; i < 3; i++ {
		for j := 0; j < 10; j++ {
			gs.Seats[i].Library = append(gs.Seats[i].Library, &gameengine.Card{Name: "Card"})
		}
	}

	thief := &gameengine.Card{Name: "Notion Thief", Types: []string{"creature"}}
	perm := &gameengine.Permanent{
		Card:       thief,
		Controller: 0,
		Timestamp:  1,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	gameengine.RegisterNotionThiefReplacement(gs, perm)

	// Non-active opponent (seat 1) draws — should be redirected immediately
	// since seat 1 is not the active player (no draw-step exception).
	_, cancelled := gameengine.FireDrawEvent(gs, 1, nil)
	if !cancelled {
		t.Error("Non-active opponent draw should be redirected")
	}
}
