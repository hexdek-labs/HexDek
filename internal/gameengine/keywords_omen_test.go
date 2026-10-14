package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Omen tests — CR §702.176
// ---------------------------------------------------------------------------

func newOmenCard(name string, owner, cmc int, omenArg string) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "omen", Args: []any{omenArg}},
			},
		},
	}
}

func newOmenGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	return NewGameState(2, rng, nil)
}

// ---------------------------------------------------------------------------
// HasOmen
// ---------------------------------------------------------------------------

func TestHasOmen_Detects(t *testing.T) {
	card := newOmenCard("Augur's Sight", 0, 3, "{1}{U}")
	if !HasOmen(card) {
		t.Fatal("HasOmen returned false for a card with omen")
	}
}

func TestHasOmen_NegativeNoKeyword(t *testing.T) {
	card := &Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant"},
		AST:   &gameast.CardAST{Name: "Lightning Bolt"},
	}
	if HasOmen(card) {
		t.Fatal("HasOmen returned true for a card with no omen keyword")
	}
}

func TestHasOmen_NilCard(t *testing.T) {
	if HasOmen(nil) {
		t.Fatal("HasOmen(nil) should return false")
	}
}

// ---------------------------------------------------------------------------
// OmenCost — mana-string parsing
// ---------------------------------------------------------------------------

func TestOmenCost_ParsesManaString(t *testing.T) {
	card := newOmenCard("Augur's Sight", 0, 3, "{1}{U}")
	if got := OmenCost(card); got != 2 {
		t.Fatalf("OmenCost = %d, want 2 (for {1}{U})", got)
	}
}

func TestOmenCost_NumericArg(t *testing.T) {
	card := &Card{
		Name:  "Numeric Omen",
		Owner: 0,
		Types: []string{"sorcery"},
		CMC:   4,
		AST: &gameast.CardAST{
			Name: "Numeric Omen",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "omen", Args: []any{3}},
			},
		},
	}
	if got := OmenCost(card); got != 3 {
		t.Fatalf("OmenCost = %d, want 3 (numeric arg)", got)
	}
}

func TestOmenCost_NoArgsFallsBackToCMC(t *testing.T) {
	card := &Card{
		Name:  "Bare Omen",
		Owner: 0,
		Types: []string{"sorcery"},
		CMC:   4,
		AST: &gameast.CardAST{
			Name: "Bare Omen",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "omen"},
			},
		},
	}
	if got := OmenCost(card); got != 4 {
		t.Fatalf("OmenCost = %d, want 4 (fallback to CMC when no args)", got)
	}
}

// ---------------------------------------------------------------------------
// (a) CastOmen — pays cost, stamps CostMeta, registers exile-cast grant
// ---------------------------------------------------------------------------

func TestCastOmen_PaysAndStampsCostMeta(t *testing.T) {
	gs := newOmenGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	card := newOmenCard("Augur's Sight", 0, 3, "{1}{U}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	result, err := CastOmen(gs, 0, card, 2)
	if err != nil {
		t.Fatalf("CastOmen returned error: %v", err)
	}
	if result == nil {
		t.Fatal("CastOmen returned nil CostPaymentResult on success")
	}

	// Mana paid (omen cost was 2; pool was 5; expect 3 left).
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("expected 3 mana left, got %d", gs.Seats[0].ManaPool)
	}
	// Card removed from hand.
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Fatal("omen-cast card should not be in hand any more")
		}
	}
	// Stack item present with the expected CostMeta stamps.
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
	if v, ok := item.CostMeta["omen"]; !ok || v != true {
		t.Fatalf("CostMeta[\"omen\"] = %v, want true", item.CostMeta["omen"])
	}
	if v, ok := item.CostMeta["omen_cost"]; !ok || v != 2 {
		t.Fatalf("CostMeta[\"omen_cost\"] = %v, want 2", item.CostMeta["omen_cost"])
	}
	if v, ok := item.CostMeta["zone_cast_keyword"]; !ok || v != "omen" {
		t.Fatalf("CostMeta[\"zone_cast_keyword\"] = %v, want \"omen\"", item.CostMeta["zone_cast_keyword"])
	}
	// r99 — INVERTED. §720.3d: "As an Omen spell resolves, its
	// controller shuffles it into its owner's library instead of
	// putting it into its owner's graveyard as it resolves." Not exile.
	if _, present := item.CostMeta["exile_on_resolve"]; present {
		t.Fatalf("omen cast carries exile_on_resolve; §720.3d sends it to the library "+
			"(CostMeta=%+v)", item.CostMeta)
	}
	if ShouldExileOnResolve(item) {
		t.Fatal("ShouldExileOnResolve is true for an omen item")
	}
	if !ShouldShuffleIntoLibraryOnResolve(item) {
		t.Fatalf("omen cast should stamp shuffle_into_library_on_resolve; CostMeta=%+v",
			item.CostMeta)
	}

	// Seat-level "spell omen-cast this turn" flag set.
	if !SpellOmenCastThisTurn(gs, 0) {
		t.Fatal("SpellOmenCastThisTurn(0) returned false after an omen cast")
	}

	// r99 — NO cast-from-exile grant. This block used to assert one was
	// registered (Zone=exile, ManaCost=-1). §720 grants nothing of the
	// kind; the card is shuffled into the library, so there is never a
	// card in exile for such a grant to apply to. Asserted positively
	// as "no grant exists", not by deleting the old checks.
	if g := GetZoneCastGrant(gs, card); g != nil {
		t.Errorf("CastOmen registered a zone-cast grant (%+v); §720 has no "+
			"cast-the-omen-face-later ability", g)
	}
}

// ---------------------------------------------------------------------------
// CastOmen — failure paths
// ---------------------------------------------------------------------------

func TestCastOmen_NoKeyword(t *testing.T) {
	gs := newOmenGame(t)
	gs.Seats[0].ManaPool = 5
	card := &Card{
		Name:  "Plain Sorcery",
		Owner: 0,
		Types: []string{"sorcery"},
		AST:   &gameast.CardAST{Name: "Plain Sorcery"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	if _, err := CastOmen(gs, 0, card, 2); err == nil {
		t.Fatal("CastOmen should fail for a card without the omen keyword")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("mana should be unchanged on failure, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastOmen_NotInHand(t *testing.T) {
	gs := newOmenGame(t)
	gs.Seats[0].ManaPool = 5
	card := newOmenCard("Augur's Sight", 0, 3, "{1}{U}")
	// Card NOT placed in hand.
	if _, err := CastOmen(gs, 0, card, 2); err == nil {
		t.Fatal("CastOmen should fail when card is not in hand")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("mana should be unchanged on failure, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastOmen_InsufficientMana(t *testing.T) {
	gs := newOmenGame(t)
	gs.Seats[0].ManaPool = 1
	card := newOmenCard("Augur's Sight", 0, 3, "{1}{U}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	if _, err := CastOmen(gs, 0, card, 2); err == nil {
		t.Fatal("CastOmen should fail with insufficient mana")
	}
	// Card still in hand.
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("card should remain in hand after a failed omen cast")
	}
}

// ---------------------------------------------------------------------------
// (b) Resolve goes to exile, not graveyard
// ---------------------------------------------------------------------------

// r99 — was TestOmen_ResolveExilesInsteadOfGraveyard, which pinned the
// wrong zone. Re-pointed at the POSITIVE destination (the owner's
// library) rather than having its exile assertion deleted.
func TestOmen_ResolveShufflesIntoOwnersLibrary(t *testing.T) {
	gs := newOmenGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	card := newOmenCard("Augur's Sight", 0, 3, "{1}{U}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	if _, err := CastOmen(gs, 0, card, 2); err != nil {
		t.Fatalf("CastOmen failed: %v", err)
	}

	// Resolve — stack.go's ShouldExileOnResolve branch routes to exile.
	ResolveStackTop(gs)

	// Must NOT be in graveyard (the §702.176 "exile it" clause replaces
	// the default §608.2g graveyard destination).
	// CR §720.3d — the OWNER's library, shuffled in.
	foundInLibrary := false
	for _, c := range gs.Seats[0].Library {
		if c == card {
			foundInLibrary = true
			break
		}
	}
	if !foundInLibrary {
		t.Fatal("omen card should be shuffled into its owner's library after resolution (§720.3d)")
	}
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("omen card went to the graveyard; §720.3d replaces that with the library")
		}
	}
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			t.Fatal("omen card was exiled; §720 has no exile clause at all")
		}
	}
}

// ---------------------------------------------------------------------------
// (c) Omen face castable from exile via the registered ZoneCastPermission
// ---------------------------------------------------------------------------

// r99 — replaces TestOmen_Face2_CastableFromExileViaGrant, which drove
// an ability the card does not have. Nothing in §720 lets a player cast
// the omen face later from exile; that is foretell/plot-shaped reminder
// text which the old implementation carried in its header, beside a
// rule number that points at Impending.
//
// Asserted as the POSITIVE fact — after resolution the card is in the
// library and no grant exists — rather than by deleting the old test,
// so this also fails if omen stops resolving at all.
func TestOmen_NoCastFromExileGrant_r99(t *testing.T) {
	gs := newOmenGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	card := newOmenCard("Augur's Sight", 0, 3, "{1}{U}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	if _, err := CastOmen(gs, 0, card, 2); err != nil {
		t.Fatalf("CastOmen failed: %v", err)
	}
	ResolveStackTop(gs)

	inLibrary := false
	for _, c := range gs.Seats[0].Library {
		if c == card {
			inLibrary = true
			break
		}
	}
	if !inLibrary {
		t.Fatalf("omen card should be in its owner's library after resolution (§720.3d)")
	}
	if g := GetZoneCastGrant(gs, card); g != nil {
		t.Errorf("a zone-cast grant survives for an omen card (%+v); §720 grants no "+
			"later cast of the omen face", g)
	}
	if got := CanCastFromZone(gs, 0, card, ZoneExile, nil); got != nil {
		t.Errorf("CanCastFromZone returned a permission for an omen card in exile: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// NewOmenCastFromExilePermission — shape sanity
// ---------------------------------------------------------------------------

func TestNewOmenCastFromExilePermission_Shape(t *testing.T) {
	p := NewOmenCastFromExilePermission(2)
	if p == nil {
		t.Fatal("NewOmenCastFromExilePermission returned nil")
	}
	if p.Zone != ZoneExile {
		t.Fatalf("Zone = %q, want %q", p.Zone, ZoneExile)
	}
	if p.Keyword != "omen" {
		t.Fatalf("Keyword = %q, want \"omen\"", p.Keyword)
	}
	if p.ManaCost != -1 {
		t.Fatalf("ManaCost = %d, want -1", p.ManaCost)
	}
	if p.RequireController != 2 {
		t.Fatalf("RequireController = %d, want 2 (owner)", p.RequireController)
	}
	if p.Duration != "" {
		t.Fatalf("Duration = %q, want \"\" (permanent until cast)", p.Duration)
	}
	if p.ExileOnResolve {
		t.Fatal("face2 grant should NOT set ExileOnResolve — second cast resolves normally")
	}
}

// ---------------------------------------------------------------------------
// r99 — the SCOPE of §720.3d, and an over-suppression guard.
// ---------------------------------------------------------------------------

// §720.3d replaces the destination only "as an Omen spell resolves". A
// COUNTERED Omen spell is not covered by it and goes to its owner's
// graveyard under the ordinary rule. This pins that the new branch did
// not over-reach into the off-board path the way escape's exile flag
// did before r96.
func TestOmen_CounteredGoesToGraveyardNotLibrary_r99(t *testing.T) {
	gs := newOmenGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	card := newOmenCard("Augur's Sight", 0, 3, "{1}{U}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	if _, err := CastOmen(gs, 0, card, 2); err != nil {
		t.Fatalf("CastOmen failed: %v", err)
	}
	item := gs.Stack[len(gs.Stack)-1]
	item.Countered = true
	ResolveStackTop(gs)

	inGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			inGrave = true
			break
		}
	}
	if !inGrave {
		t.Errorf("a COUNTERED omen spell should go to its owner's graveyard; " +
			"§720.3d is a resolution replacement only")
	}
	for _, c := range gs.Seats[0].Library {
		if c == card {
			t.Errorf("a countered omen spell was shuffled into the library; " +
				"the §720.3d branch over-reached into the off-board path")
		}
	}
}

// Over-suppression guard: the new branch must not divert spells that
// are not Omens. A flashback item still exiles (§702.34a), and an
// ordinary item still reaches the graveyard.
func TestOmen_ShuffleBranchDoesNotCatchOtherSpells_r99(t *testing.T) {
	flashback := &StackItem{CostMeta: map[string]interface{}{"exile_on_resolve": true}}
	if ShouldShuffleIntoLibraryOnResolve(flashback) {
		t.Error("a flashback item matched the omen shuffle predicate")
	}
	if zone, rule := postStackZoneForOffboard(flashback, "graveyard", "701.6a"); zone != "exile" {
		t.Errorf("flashback off-board routing = %q (%s), want exile", zone, rule)
	}
	plain := &StackItem{}
	if ShouldShuffleIntoLibraryOnResolve(plain) {
		t.Error("an item with no CostMeta matched the omen shuffle predicate")
	}
	omen := &StackItem{CostMeta: map[string]interface{}{"shuffle_into_library_on_resolve": true}}
	if zone, _ := postStackZoneForOffboard(omen, "graveyard", "701.6a"); zone != "graveyard" {
		t.Errorf("countered omen off-board routing = %q, want graveyard", zone)
	}
}
