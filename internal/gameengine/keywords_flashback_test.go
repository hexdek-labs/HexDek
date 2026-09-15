package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Flashback tests — CR §702.34
// ---------------------------------------------------------------------------

func newFlashbackCard(name string, owner, cmc int, flashbackArg string) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "flashback", Args: []any{flashbackArg}},
			},
		},
	}
}

func newInstantCard(name string, owner, cmc int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func newFlashbackGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(2, rng, nil)
}

// ---------------------------------------------------------------------------
// HasFlashback
// ---------------------------------------------------------------------------

func TestHasFlashback_Detects(t *testing.T) {
	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	if !HasFlashback(card) {
		t.Fatal("HasFlashback returned false for a card with flashback keyword")
	}
}

func TestHasFlashback_NegativeNoKeyword(t *testing.T) {
	card := newInstantCard("Lightning Bolt", 0, 1)
	if HasFlashback(card) {
		t.Fatal("HasFlashback returned true for a card with no flashback keyword")
	}
}

func TestHasFlashback_NilCard(t *testing.T) {
	if HasFlashback(nil) {
		t.Fatal("HasFlashback(nil) should return false")
	}
}

// ---------------------------------------------------------------------------
// FlashbackCost — mana-string parsing
// ---------------------------------------------------------------------------

func TestFlashbackCost_ParsesManaString(t *testing.T) {
	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	if got := FlashbackCost(card); got != 5 {
		t.Fatalf("FlashbackCost = %d, want 5 (for {4}{R})", got)
	}
}

func TestFlashbackCost_NumericArg(t *testing.T) {
	card := &Card{
		Name:  "Numeric Flashback",
		Owner: 0,
		Types: []string{"sorcery"},
		CMC:   3,
		AST: &gameast.CardAST{
			Name: "Numeric Flashback",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "flashback", Args: []any{5}},
			},
		},
	}
	if got := FlashbackCost(card); got != 5 {
		t.Fatalf("FlashbackCost with numeric arg = %d, want 5", got)
	}
}

func TestFlashbackCost_NoArgsFallsBackToCMC(t *testing.T) {
	card := &Card{
		Name:  "Argless Flashback",
		Owner: 0,
		Types: []string{"sorcery"},
		CMC:   3,
		AST: &gameast.CardAST{
			Name: "Argless Flashback",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "flashback"},
			},
		},
	}
	if got := FlashbackCost(card); got != 3 {
		t.Fatalf("FlashbackCost with no args = %d, want 3 (CMC fallback)", got)
	}
}

func TestFlashbackCost_NoKeyword(t *testing.T) {
	card := newInstantCard("Lightning Bolt", 0, 1)
	if got := FlashbackCost(card); got != 0 {
		t.Fatalf("FlashbackCost on non-flashback card = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// CastFlashback — happy path (Past in Flames style)
// ---------------------------------------------------------------------------

func TestCastFlashback_PaysCostAndPushesStack(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 6

	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	result, err := CastFlashback(gs, 0, card, 5)
	if err != nil {
		t.Fatalf("CastFlashback returned error: %v", err)
	}
	if result == nil {
		t.Fatal("CastFlashback returned nil CostPaymentResult on success")
	}

	// Mana paid (cost 5; pool was 6; expect 1 left).
	if gs.Seats[0].ManaPool != 1 {
		t.Fatalf("expected 1 mana left, got %d", gs.Seats[0].ManaPool)
	}
	// Card removed from graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("flashback card should no longer be in graveyard")
		}
	}
	// Stack item present with exile_on_resolve CostMeta.
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]
	if item.Card != card {
		t.Fatal("stack item Card should reference the cast card")
	}
	if item.Controller != 0 {
		t.Fatalf("stack item Controller=%d, want 0", item.Controller)
	}
	if item.CastZone != ZoneGraveyard {
		t.Fatalf("stack item CastZone=%q, want %q", item.CastZone, ZoneGraveyard)
	}
	if !ShouldExileOnResolve(item) {
		t.Fatal("flashback stack item should be tagged exile_on_resolve")
	}
	if v, ok := item.CostMeta["flashback"]; !ok || v != true {
		t.Fatalf("stack item CostMeta[\"flashback\"] = %v, want true", item.CostMeta["flashback"])
	}
	if v, ok := item.CostMeta["flashback_cost"]; !ok || v != 5 {
		t.Fatalf("stack item CostMeta[\"flashback_cost\"] = %v, want 5", item.CostMeta["flashback_cost"])
	}
	if v, ok := item.CostMeta["zone_cast_keyword"]; !ok || v != "flashback" {
		t.Fatalf("stack item CostMeta[\"zone_cast_keyword\"] = %v, want \"flashback\"", item.CostMeta["zone_cast_keyword"])
	}
	if !SpellFlashbackedThisTurn(gs, 0) {
		t.Fatal("SpellFlashbackedThisTurn(0) returned false after a flashback cast")
	}
}

// ---------------------------------------------------------------------------
// CastFlashback — defaults flashbackCost from keyword args when -1 is passed
// ---------------------------------------------------------------------------

func TestCastFlashback_NegOneUsesPrintedCost(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastFlashback(gs, 0, card, -1); err != nil {
		t.Fatalf("CastFlashback(-1) failed: %v", err)
	}
	// Printed flashback cost is {4}{R} = 5 mana.
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected 0 mana left after paying printed {4}{R}, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// CastFlashback — failure paths
// ---------------------------------------------------------------------------

func TestCastFlashback_NoKeyword(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Seats[0].ManaPool = 5
	card := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	if _, err := CastFlashback(gs, 0, card, 1); err == nil {
		t.Fatal("CastFlashback should fail for a card without flashback")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("mana should be unchanged, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Fatal("card should remain in graveyard after failed CastFlashback")
	}
}

func TestCastFlashback_NotInGraveyard(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Seats[0].ManaPool = 5
	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	// Card NOT placed in graveyard.
	if _, err := CastFlashback(gs, 0, card, 5); err == nil {
		t.Fatal("CastFlashback should fail when card is not in graveyard")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("mana should be unchanged, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastFlashback_InsufficientMana(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Seats[0].ManaPool = 2
	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	if _, err := CastFlashback(gs, 0, card, 5); err == nil {
		t.Fatal("CastFlashback should fail with insufficient mana")
	}
	// Card still in graveyard, mana untouched.
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Fatal("card should remain in graveyard after failed CastFlashback")
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("mana should be unchanged, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastFlashback_InvalidSeat(t *testing.T) {
	gs := newFlashbackGame(t)
	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	if _, err := CastFlashback(gs, 99, card, 5); err == nil {
		t.Fatal("CastFlashback should fail on invalid seat index")
	}
}

// ---------------------------------------------------------------------------
// End-to-end: flashback cast → resolve → exile (Past in Flames pattern)
// ---------------------------------------------------------------------------

func TestFlashback_CastResolveExile_EndToEnd(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	// 1. Cast from graveyard for flashback cost.
	if _, err := CastFlashback(gs, 0, card, 5); err != nil {
		t.Fatalf("CastFlashback failed: %v", err)
	}
	// Sanity: card not in graveyard, sits on stack.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("card should be on the stack, not in graveyard")
		}
	}

	// 2. Resolve — stack.go's ShouldExileOnResolve branch routes to exile
	//    per CR §702.34a (flashback cards exile instead of graveyard).
	ResolveStackTop(gs)

	// 3. Card should be in exile, not graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("flashback card should be in exile after resolution, not graveyard (§702.34a)")
		}
	}
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			foundInExile = true
			break
		}
	}
	if !foundInExile {
		t.Fatal("flashback card should be in owner's exile after resolution")
	}
}

// ---------------------------------------------------------------------------
// Snapcaster Mage — GrantFlashbackUntilEOT
// ---------------------------------------------------------------------------

func TestGrantFlashbackUntilEOT_RegistersGrant(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Turn = 5
	target := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)

	GrantFlashbackUntilEOT(gs, target, 0, "Snapcaster Mage")

	grant := GetZoneCastGrant(gs, target)
	if grant == nil {
		t.Fatal("expected a ZoneCastPermission registered for the Snapcaster target")
	}
	if grant.Zone != ZoneGraveyard {
		t.Fatalf("grant.Zone = %q, want %q", grant.Zone, ZoneGraveyard)
	}
	if grant.Keyword != "flashback" {
		t.Fatalf("grant.Keyword = %q, want \"flashback\"", grant.Keyword)
	}
	if !grant.ExileOnResolve {
		t.Fatal("grant.ExileOnResolve should be true")
	}
	if grant.RequireController != 0 {
		t.Fatalf("grant.RequireController = %d, want 0", grant.RequireController)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Fatalf("grant.Duration = %q, want \"until_end_of_turn\"", grant.Duration)
	}
	if grant.GrantTurn != 5 {
		t.Fatalf("grant.GrantTurn = %d, want 5", grant.GrantTurn)
	}
	if grant.ManaCost != target.CMC {
		t.Fatalf("grant.ManaCost = %d, want %d (target CMC)", grant.ManaCost, target.CMC)
	}
}

func TestGrantFlashbackUntilEOT_RejectsCreature(t *testing.T) {
	gs := newFlashbackGame(t)
	creature := &Card{
		Name:  "Grizzly Bears",
		Owner: 0,
		Types: []string{"creature"},
		CMC:   2,
		AST: &gameast.CardAST{
			Name:      "Grizzly Bears",
			Abilities: []gameast.Ability{},
		},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, creature)

	GrantFlashbackUntilEOT(gs, creature, 0, "Snapcaster Mage")

	if grant := GetZoneCastGrant(gs, creature); grant != nil {
		t.Fatal("Snapcaster grant should be rejected for a creature (§702.34a, Snapcaster targets instant/sorcery)")
	}
}

func TestSnapcaster_CastViaGrant_EndToEnd(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Active = 0
	gs.Turn = 3
	gs.Seats[0].ManaPool = 1

	// Lightning Bolt in graveyard (no intrinsic flashback).
	bolt := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	// Snapcaster Mage enters and grants flashback to bolt.
	GrantFlashbackUntilEOT(gs, bolt, 0, "Snapcaster Mage")

	// 1. CastFlashback succeeds via the grant (passing -1 to use the
	//    grant's cost = Bolt's printed CMC of 1).
	if _, err := CastFlashback(gs, 0, bolt, -1); err != nil {
		t.Fatalf("CastFlashback via Snapcaster grant failed: %v", err)
	}
	// Mana spent.
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected 0 mana left, got %d", gs.Seats[0].ManaPool)
	}
	// Stack item tagged exile_on_resolve.
	if len(gs.Stack) != 1 || !ShouldExileOnResolve(gs.Stack[0]) {
		t.Fatal("Snapcaster-granted cast should push a stack item tagged exile_on_resolve")
	}
	// Grant consumed.
	if grant := GetZoneCastGrant(gs, bolt); grant != nil {
		t.Fatal("Snapcaster grant should be removed after the cast consumes it")
	}

	// 2. Resolve → bolt in exile, not graveyard.
	ResolveStackTop(gs)
	for _, c := range gs.Seats[0].Graveyard {
		if c == bolt {
			t.Fatal("Snapcaster-cast Bolt should be in exile after resolution, not graveyard")
		}
	}
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == bolt {
			foundInExile = true
			break
		}
	}
	if !foundInExile {
		t.Fatal("Snapcaster-cast Bolt should be in exile after resolution")
	}
}

func TestSnapcaster_GrantExpiresAtEndOfTurn(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Turn = 3
	bolt := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	GrantFlashbackUntilEOT(gs, bolt, 0, "Snapcaster Mage")
	if GetZoneCastGrant(gs, bolt) == nil {
		t.Fatal("grant should exist immediately after GrantFlashbackUntilEOT")
	}

	// End-of-turn cleanup on the same turn expires it.
	ExpireZoneCastGrants(gs)
	if GetZoneCastGrant(gs, bolt) != nil {
		t.Fatal("Snapcaster grant should expire at end-of-turn cleanup of grant turn")
	}
}

func TestSnapcaster_WrongControllerRejected(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Active = 1
	gs.Seats[1].ManaPool = 5

	// Bolt in seat 0's graveyard, Snapcaster grants flashback restricted
	// to seat 0. Seat 1 tries to cast it — must be rejected.
	bolt := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)
	GrantFlashbackUntilEOT(gs, bolt, 0, "Snapcaster Mage")

	if _, err := CastFlashback(gs, 1, bolt, -1); err == nil {
		t.Fatal("CastFlashback by wrong seat should be rejected by grant.RequireController")
	}
	// Bolt remains in seat 0's graveyard.
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == bolt {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Bolt should remain in seat 0's graveyard after rejected cast")
	}
}
