package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// One-shot EOT flashback-grant regressions: Past in Flames, Will of the
// Jeskai, Flashback (instant).
//
// Primitive under test: gameengine.RegisterEOTGraveyardFlashbackGrant +
// gameengine.ExpireEOTGraveyardFlashbackGrants, with the existing
// EffectiveFlashbackCostFromGraveyardGrants + CastFlashback hot path.

func newInstantSorceryGYCard(name string, owner, cmc int, typeLine string) *gameengine.Card {
	// Use the "cmc:N" Type token so per_card's cardCMC() sees the value
	// — it doesn't fall back to Card.CMC in the current implementation.
	return &gameengine.Card{
		Name:  name,
		Owner: owner,
		Types: []string{typeLine, "cmc:" + itoaSmall(cmc)},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func itoaSmall(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [12]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestPastInFlames_RegistersEOTMassGrant(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Turn = 5
	gs.Seats[0].ManaPool = 10

	// Two i/s and one non-i/s in graveyard.
	gy1 := newInstantSorceryGYCard("Cathartic Reunion", 0, 3, "sorcery")
	gy2 := newInstantSorceryGYCard("Lightning Bolt", 0, 1, "instant")
	creature := &gameengine.Card{
		Name:  "Grizzly Bears",
		Owner: 0,
		Types: []string{"creature"},
		CMC:   2,
		AST:   &gameast.CardAST{Name: "Grizzly Bears"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy1, gy2, creature)

	pif := addCard(gs, 0, "Past in Flames", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: pif}
	if fired := gameengine.InvokeResolveHook(gs, item); fired < 1 {
		t.Fatalf("expected Past in Flames OnResolve to fire, got %d", fired)
	}
	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("expected 1 EOT grant after Past in Flames, got %d", len(gs.GraveyardFlashbackGrants))
	}
	g := gs.GraveyardFlashbackGrants[0]
	if !g.ExpiresAtCleanup {
		t.Errorf("expected ExpiresAtCleanup=true on Past in Flames grant")
	}
	if g.SourceTimestamp != 0 {
		t.Errorf("expected SourceTimestamp=0 (no permanent source), got %d", g.SourceTimestamp)
	}
	if g.GrantTurn != 5 {
		t.Errorf("expected GrantTurn=5, got %d", g.GrantTurn)
	}

	// Both i/s eligible at printed CMC; creature not eligible.
	if c, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy1); !ok || c != 3 {
		t.Errorf("expected sorcery flashback cost=3 ok=true, got %d,%v", c, ok)
	}
	if c, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy2); !ok || c != 1 {
		t.Errorf("expected instant flashback cost=1 ok=true, got %d,%v", c, ok)
	}
	if _, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, creature); ok {
		t.Errorf("creature should NOT get flashback from mass grant")
	}
}

func TestPastInFlames_GrantActiveOnOpponentTurn(t *testing.T) {
	// One-shot mass grant is NOT active-turn-gated (unlike Iroh).
	// Past in Flames is cast on your turn but the grant only lasts
	// until end of turn, so it's still controller's turn when it
	// matters. Verify OnlyActiveTurn is false.
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	gy := newInstantSorceryGYCard("Lightning Bolt", 0, 1, "instant")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	pif := addCard(gs, 0, "Past in Flames", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: pif}
	gameengine.InvokeResolveHook(gs, item)

	if gs.GraveyardFlashbackGrants[0].OnlyActiveTurn {
		t.Errorf("one-shot EOT grant should not be active-turn-gated")
	}
	// Flip active seat — grant should still apply.
	gs.Active = 1
	if _, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy); !ok {
		t.Errorf("EOT mass grant should still apply when controller is not active")
	}
}

func TestPastInFlames_ExpiresAtCleanup(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Turn = 3

	gy := newInstantSorceryGYCard("Lightning Bolt", 0, 1, "instant")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	pif := addCard(gs, 0, "Past in Flames", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: pif}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("setup: expected 1 grant, got %d", len(gs.GraveyardFlashbackGrants))
	}

	gameengine.ExpireEOTGraveyardFlashbackGrants(gs)
	if len(gs.GraveyardFlashbackGrants) != 0 {
		t.Fatalf("expected EOT sweep to remove grant, got %d", len(gs.GraveyardFlashbackGrants))
	}
}

func TestPastInFlames_OrphanSweepLeavesEOTGrantsAlone(t *testing.T) {
	// ExpireOrphanedGraveyardFlashbackGrants must skip EOT grants
	// (they have SourceTimestamp=0 but should not be confused with
	// "orphan from a permanent that left").
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Turn = 2

	gy := newInstantSorceryGYCard("Lightning Bolt", 0, 1, "instant")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	pif := addCard(gs, 0, "Past in Flames", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: pif}
	gameengine.InvokeResolveHook(gs, item)

	gameengine.ExpireOrphanedGraveyardFlashbackGrants(gs)
	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("orphan sweep should NOT remove EOT grants, got %d", len(gs.GraveyardFlashbackGrants))
	}
}

func TestWillOfTheJeskai_FlashbackOnlyWithoutCommander(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Turn = 4
	// Seed a hand so we can detect whether mode-1 ran.
	hand := &gameengine.Card{Name: "Filler", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, hand)
	addLibrary(gs, 0, "L1", "L2", "L3", "L4", "L5", "L6")

	will := addCard(gs, 0, "Will of the Jeskai", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: will}
	if fired := gameengine.InvokeResolveHook(gs, item); fired < 1 {
		t.Fatalf("expected Will of the Jeskai OnResolve to fire, got %d", fired)
	}

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("expected mass grant registered, got %d", len(gs.GraveyardFlashbackGrants))
	}
	// Mode 1 (discard + draw 5) should NOT have run — no commander present.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected hand size unchanged (no commander → no draw 5), got %d", len(gs.Seats[0].Hand))
	}
}

func TestWillOfTheJeskai_BothModesWithCommander(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Turn = 4
	gs.Seats[0].CommanderNames = []string{"Narset, Enlightened Master"}
	addPerm(gs, 0, "Narset, Enlightened Master", "creature", "legendary")
	// Hand has 2 cards (less than 5 → controller takes the redraw).
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		&gameengine.Card{Name: "A", Owner: 0, Types: []string{"sorcery"}},
		&gameengine.Card{Name: "B", Owner: 0, Types: []string{"sorcery"}},
	)
	addLibrary(gs, 0, "L1", "L2", "L3", "L4", "L5", "L6", "L7")

	will := addCard(gs, 0, "Will of the Jeskai", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: will}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("expected mass grant from mode 2, got %d", len(gs.GraveyardFlashbackGrants))
	}
	// Mode 1 fired: hand replaced with 5 fresh cards.
	if len(gs.Seats[0].Hand) != 5 {
		t.Errorf("expected hand size 5 after mode 1, got %d", len(gs.Seats[0].Hand))
	}
	// Discarded 2 → graveyard should include them (named A, B).
	gyNames := map[string]bool{}
	for _, c := range gs.Seats[0].Graveyard {
		gyNames[c.DisplayName()] = true
	}
	if !gyNames["A"] || !gyNames["B"] {
		t.Errorf("expected discarded A and B in graveyard, got %v", gyNames)
	}
}

func TestFlashbackInstant_GrantsToHighestCMCTarget(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	cheap := newInstantSorceryGYCard("Lightning Bolt", 0, 1, "instant")
	expensive := newInstantSorceryGYCard("Time Spiral", 0, 6, "sorcery")
	creature := &gameengine.Card{
		Name:  "Grizzly Bears",
		Owner: 0,
		Types: []string{"creature"},
		CMC:   2,
		AST:   &gameast.CardAST{Name: "Grizzly Bears"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, cheap, creature, expensive)

	fb := addCard(gs, 0, "Flashback", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: fb}
	if fired := gameengine.InvokeResolveHook(gs, item); fired < 1 {
		t.Fatalf("expected Flashback OnResolve to fire, got %d", fired)
	}

	// Per-card ZoneCastGrant should be registered for the expensive target.
	grant := gameengine.GetZoneCastGrant(gs, expensive)
	if grant == nil {
		t.Fatalf("expected ZoneCastGrant on Time Spiral, got nil")
	}
	if grant.Keyword != "flashback" {
		t.Errorf("expected grant keyword=flashback, got %q", grant.Keyword)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("expected duration=until_end_of_turn, got %q", grant.Duration)
	}
	// Cheap card should NOT have received the grant.
	if gameengine.GetZoneCastGrant(gs, cheap) != nil {
		t.Errorf("expected no grant on cheap card; Flashback targets the picked card only")
	}
}

func TestRecoup_GrantsToSorceryNotInstant(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	bolt := newInstantSorceryGYCard("Lightning Bolt", 0, 1, "instant")
	wrath := newInstantSorceryGYCard("Wrath of God", 0, 4, "sorcery")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, wrath)

	rec := addCard(gs, 0, "Recoup", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: rec}
	gameengine.InvokeResolveHook(gs, item)

	if gameengine.GetZoneCastGrant(gs, wrath) == nil {
		t.Errorf("Recoup should grant flashback to the sorcery target")
	}
	if gameengine.GetZoneCastGrant(gs, bolt) != nil {
		t.Errorf("Recoup must NOT target instants")
	}
}

func TestFlashbackInstant_NoEligibleTargetFailsGracefully(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	// Empty graveyard.
	fb := addCard(gs, 0, "Flashback", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: fb}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event on Flashback with no target; events: %+v", gs.EventLog)
	}
}
