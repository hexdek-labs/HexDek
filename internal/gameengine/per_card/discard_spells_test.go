package per_card

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func newTestGS(seats int) *gameengine.GameState {
	return gameengine.NewGameState(seats, rand.New(rand.NewSource(42)), nil)
}

func TestHymnToTourach_Resolves(t *testing.T) {
	gs := newTestGS(2)
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "Forest"},
		{Name: "Plains"},
		{Name: "Mountain"},
	}

	item := &gameengine.StackItem{
		Card:       &gameengine.Card{Name: "Hymn to Tourach"},
		Controller: 0,
	}
	hymnToTourachResolve(gs, item)

	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("opponent should have 1 card left, got %d", len(gs.Seats[1].Hand))
	}
	if len(gs.Seats[1].Graveyard) != 2 {
		t.Errorf("opponent graveyard should have 2 cards, got %d", len(gs.Seats[1].Graveyard))
	}
}

func TestDeliriumSkeins_AllPlayersDiscard3(t *testing.T) {
	gs := newTestGS(4)
	for i := 0; i < 4; i++ {
		gs.Seats[i].Hand = make([]*gameengine.Card, 5)
		for j := 0; j < 5; j++ {
			gs.Seats[i].Hand[j] = &gameengine.Card{Name: "Card"}
		}
	}

	item := &gameengine.StackItem{
		Card:       &gameengine.Card{Name: "Delirium Skeins"},
		Controller: 0,
	}
	deliriumSkeinsResolve(gs, item)

	for i := 0; i < 4; i++ {
		if len(gs.Seats[i].Hand) != 2 {
			t.Errorf("seat %d should have 2 cards, got %d", i, len(gs.Seats[i].Hand))
		}
	}
}

func TestSyphonMind_DrawsForEachDiscard(t *testing.T) {
	gs := newTestGS(4)
	for i := 0; i < 4; i++ {
		gs.Seats[i].Hand = make([]*gameengine.Card, 3)
		for j := 0; j < 3; j++ {
			gs.Seats[i].Hand[j] = &gameengine.Card{Name: "Card"}
		}
	}
	gs.Seats[0].Library = make([]*gameengine.Card, 10)
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library[i] = &gameengine.Card{Name: "LibCard"}
	}

	item := &gameengine.StackItem{
		Card:       &gameengine.Card{Name: "Syphon Mind"},
		Controller: 0,
	}
	syphonMindResolve(gs, item)

	for i := 1; i < 4; i++ {
		if len(gs.Seats[i].Hand) != 2 {
			t.Errorf("opponent %d should have 2 cards, got %d", i, len(gs.Seats[i].Hand))
		}
	}
	// Controller drew 3 (one per opponent who discarded).
	if len(gs.Seats[0].Hand) != 6 {
		t.Errorf("controller should have 6 cards (3 original + 3 drawn), got %d", len(gs.Seats[0].Hand))
	}
}

func TestDarkDeal_DiscardAndRedraw(t *testing.T) {
	gs := newTestGS(2)
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"},
	}
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "E"}, {Name: "F"},
	}
	gs.Seats[0].Library = make([]*gameengine.Card, 10)
	gs.Seats[1].Library = make([]*gameengine.Card, 10)
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library[i] = &gameengine.Card{Name: "Lib0"}
		gs.Seats[1].Library[i] = &gameengine.Card{Name: "Lib1"}
	}

	item := &gameengine.StackItem{
		Card:       &gameengine.Card{Name: "Dark Deal"},
		Controller: 0,
	}
	darkDealResolve(gs, item)

	// Seat 0: had 4, discarded 4, drew 3 (4-1).
	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("seat 0 should have 3 cards, got %d", len(gs.Seats[0].Hand))
	}
	// Seat 1: had 2, discarded 2, drew 1 (2-1).
	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("seat 1 should have 1 card, got %d", len(gs.Seats[1].Hand))
	}
}

func TestNecrogenMists_UpkeepDiscard(t *testing.T) {
	gs := newTestGS(2)
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "A"}, {Name: "B"},
	}

	perm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Necrogen Mists"},
		Controller: 1,
	}
	ctx := map[string]interface{}{
		"active_seat": 0,
	}
	necrogenMistsUpkeep(gs, perm, ctx)

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("active player should have 1 card after upkeep, got %d", len(gs.Seats[0].Hand))
	}
}

func TestTergridDiscard_StealsFromGraveyard(t *testing.T) {
	gs := newTestGS(2)
	creature := &gameengine.Card{
		Name:          "Grizzly Bears",
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
	}
	gs.Seats[1].Hand = []*gameengine.Card{creature}

	tergridPerm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Tergrid, God of Fright"},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{tergridPerm}

	// Force discard to fire the trigger.
	gameengine.DiscardN(gs, 1, 1, "")

	// Tergrid should have stolen the creature.
	foundOnBattlefield := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Grizzly Bears" {
			foundOnBattlefield = true
			break
		}
	}
	if !foundOnBattlefield {
		t.Error("Tergrid should have stolen Grizzly Bears to battlefield")
	}
	if len(gs.Seats[1].Graveyard) != 0 {
		t.Errorf("opponent graveyard should be empty after Tergrid steal, has %d cards", len(gs.Seats[1].Graveyard))
	}
}

func TestWasteNot_CreatureZombie(t *testing.T) {
	gs := newTestGS(2)
	wasteNotPerm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Waste Not"},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{wasteNotPerm}

	ctx := map[string]interface{}{
		"discarder_seat": 1,
		"card":           &gameengine.Card{Name: "Bear", Types: []string{"creature"}},
	}
	wasteNotTrigger(gs, wasteNotPerm, ctx)

	zombies := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Zombie" {
			zombies++
		}
	}
	if zombies != 1 {
		t.Errorf("expected 1 Zombie token, got %d", zombies)
	}
}

func TestWasteNot_LandMana(t *testing.T) {
	gs := newTestGS(2)
	wasteNotPerm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Waste Not"},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{wasteNotPerm}
	initialMana := gs.Seats[0].ManaPool

	ctx := map[string]interface{}{
		"discarder_seat": 1,
		"card":           &gameengine.Card{Name: "Forest", Types: []string{"land"}},
	}
	wasteNotTrigger(gs, wasteNotPerm, ctx)

	if gs.Seats[0].ManaPool != initialMana+2 {
		t.Errorf("expected +2 mana, got %d → %d", initialMana, gs.Seats[0].ManaPool)
	}
}

func TestWasteNot_NoncreatureNonlandDraw(t *testing.T) {
	gs := newTestGS(2)
	wasteNotPerm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Waste Not"},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{wasteNotPerm}
	gs.Seats[0].Library = []*gameengine.Card{{Name: "Lightning Bolt"}}

	ctx := map[string]interface{}{
		"discarder_seat": 1,
		"card":           &gameengine.Card{Name: "Counterspell", Types: []string{"instant"}},
	}
	wasteNotTrigger(gs, wasteNotPerm, ctx)

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 drawn card, got %d", len(gs.Seats[0].Hand))
	}
}

func TestLilianasCaress_Drain(t *testing.T) {
	gs := newTestGS(2)
	gs.Seats[1].Life = 40
	caressPerm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Liliana's Caress"},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{caressPerm}

	ctx := map[string]interface{}{
		"discarder_seat": 1,
		"card":           &gameengine.Card{Name: "Whatever"},
	}
	lilianasCaressTrigger(gs, caressPerm, ctx)

	if gs.Seats[1].Life != 38 {
		t.Errorf("expected life=38, got %d", gs.Seats[1].Life)
	}
}
