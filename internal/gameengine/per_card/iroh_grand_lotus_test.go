package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Iroh, Grand Lotus regression — graveyard-flashback grant primitive.
//
// Oracle behavior under test:
//
//   - During controller's turn, each non-Lesson instant/sorcery in the
//     controller's graveyard has flashback equal to its mana cost.
//   - During controller's turn, each Lesson in the controller's
//     graveyard has flashback {1}.
//   - Grant is inactive on other players' turns.
//   - Grant disappears when Iroh leaves the battlefield (LTB) and is
//     also swept by EOT cleanup of orphaned grants.

// newGraveyardCard builds a graveyard-eligible instant/sorcery card.
// The AST has no intrinsic flashback keyword — the grant is the only
// route to flashback in these tests.
func newGraveyardCard(name string, owner, cmc int, subtypes ...string) *gameengine.Card {
	types := append([]string{"sorcery"}, subtypes...)
	return &gameengine.Card{
		Name:  name,
		Owner: owner,
		Types: types,
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func TestIroh_RegistersGraveyardFlashbackGrantOnETB(t *testing.T) {
	gs := newGame(t, 2)
	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature", "legendary")

	gameengine.InvokeETBHook(gs, iroh)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("expected 1 graveyard-flashback grant after Iroh ETB, got %d",
			len(gs.GraveyardFlashbackGrants))
	}
	g := gs.GraveyardFlashbackGrants[0]
	if g.Controller != 0 {
		t.Errorf("grant Controller = %d, want 0", g.Controller)
	}
	if !g.OnlyActiveTurn {
		t.Errorf("Iroh grant should be active-turn-gated")
	}
	if g.SourceTimestamp != iroh.Timestamp {
		t.Errorf("grant SourceTimestamp = %d, want iroh.Timestamp %d",
			g.SourceTimestamp, iroh.Timestamp)
	}
}

func TestIroh_GrantsFlashbackToNonLessonInstantSorcery_DuringOwnTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10

	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature", "legendary")
	gameengine.InvokeETBHook(gs, iroh)

	// Non-Lesson sorcery, CMC 3 in seat 0's graveyard.
	gy := newGraveyardCard("Cathartic Reunion", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	cost, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy)
	if !ok {
		t.Fatal("expected Iroh grant to apply to non-Lesson sorcery on own turn")
	}
	if cost != 3 {
		t.Errorf("expected flashback cost = card CMC (3), got %d", cost)
	}

	// CastFlashback should succeed via the graveyard-wide grant.
	if _, err := gameengine.CastFlashback(gs, 0, gy, -1); err != nil {
		t.Fatalf("CastFlashback via Iroh grant failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 7 {
		t.Errorf("expected 10 - 3 = 7 mana left, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected stack item from flashback cast, got %d", len(gs.Stack))
	}
	if !gameengine.ShouldExileOnResolve(gs.Stack[0]) {
		t.Errorf("flashback cast should be flagged exile_on_resolve (§702.34c)")
	}
}

func TestIroh_LessonFlashbackCostIsOne(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature", "legendary")
	gameengine.InvokeETBHook(gs, iroh)

	// Lesson sorcery with mana cost 4 — should flashback for {1}.
	gy := newGraveyardCard("Environmental Sciences", 0, 4, "lesson")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	cost, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy)
	if !ok {
		t.Fatal("expected Iroh grant to apply to Lesson on own turn")
	}
	if cost != 1 {
		t.Errorf("expected Lesson flashback cost = 1, got %d", cost)
	}

	if _, err := gameengine.CastFlashback(gs, 0, gy, -1); err != nil {
		t.Fatalf("CastFlashback for Lesson failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("expected 5 - 1 = 4 mana left, got %d", gs.Seats[0].ManaPool)
	}
}

func TestIroh_DoesNotGrantFlashbackToCreatureCards(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature", "legendary")
	gameengine.InvokeETBHook(gs, iroh)

	creature := &gameengine.Card{
		Name:  "Llanowar Elves",
		Owner: 0,
		Types: []string{"creature"},
		CMC:   1,
		AST:   &gameast.CardAST{Name: "Llanowar Elves"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, creature)

	if _, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, creature); ok {
		t.Fatal("Iroh grant should NOT apply to non-instant/sorcery cards")
	}
}

func TestIroh_GrantInactiveOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1 // opponent's turn

	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature", "legendary")
	gameengine.InvokeETBHook(gs, iroh)

	gy := newGraveyardCard("Cathartic Reunion", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	if _, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy); ok {
		t.Fatal("Iroh's grant should be inactive when controller is not the active player")
	}

	// CastFlashback should also reject (no intrinsic, no per-card grant,
	// no graveyard grant applies off-turn).
	if _, err := gameengine.CastFlashback(gs, 0, gy, -1); err == nil {
		t.Fatal("CastFlashback should fail when Iroh's grant is inactive (opponent's turn)")
	}
}

func TestIroh_GrantOnlyAppliesToControllerGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature", "legendary")
	gameengine.InvokeETBHook(gs, iroh)

	// Opponent's graveyard card — must not get Iroh's flashback grant.
	gy := newGraveyardCard("Cathartic Reunion", 1, 3)
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, gy)

	if _, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 1, gy); ok {
		t.Fatal("Iroh's grant should only apply to the controller's graveyard")
	}
}

func TestIroh_LTBRemovesGrant(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature", "legendary")
	gameengine.InvokeETBHook(gs, iroh)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("setup: expected 1 grant after ETB, got %d", len(gs.GraveyardFlashbackGrants))
	}

	// Fire LTB trigger directly.
	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm": iroh,
	})

	if len(gs.GraveyardFlashbackGrants) != 0 {
		t.Fatalf("expected grant removed by LTB, got %d", len(gs.GraveyardFlashbackGrants))
	}
}

func TestIroh_OrphanedGrantSweptByEOTCleanup(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature", "legendary")
	gameengine.InvokeETBHook(gs, iroh)

	// Simulate Iroh vanishing without the LTB hook running (e.g.
	// a code path that bypasses permanent_ltb). The EOT sweep should
	// catch it because the source timestamp no longer matches any
	// battlefield permanent.
	gs.Seats[0].Battlefield = nil

	gameengine.ExpireOrphanedGraveyardFlashbackGrants(gs)

	if len(gs.GraveyardFlashbackGrants) != 0 {
		t.Fatalf("expected orphaned grant swept by EOT cleanup, got %d",
			len(gs.GraveyardFlashbackGrants))
	}
}
