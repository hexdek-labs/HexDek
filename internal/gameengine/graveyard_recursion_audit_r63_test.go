package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// graveyard_recursion_audit_r63_test.go — CR §702.34 (flashback) / §702.148
// (escape) / §702.52 (dredge) / §702.131 (jump-start) graveyard-recursion
// audit. Full-pipeline verification (cast → resolve → final zone), with
// special focus on GRANTED flashback (Iroh-style), which the Issue Log
// flagged as inert — this suite proves it works end-to-end.

func gyAuditGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	return gs
}

func plainInstant(name string, owner, cmc int, types ...string) *Card {
	tl := []string{"instant"}
	if len(types) > 0 {
		tl = types
	}
	return &Card{
		Name: name, Owner: owner, Types: tl, CMC: cmc,
		AST: &gameast.CardAST{Name: name, Abilities: []gameast.Ability{}},
	}
}

func inExile(seat *Seat, card *Card) bool {
	for _, c := range seat.Exile {
		if c == card {
			return true
		}
	}
	return false
}
func inGrave(seat *Seat, card *Card) bool {
	for _, c := range seat.Graveyard {
		if c == card {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// GRANTED flashback (Iroh / Lier shape) — the Issue-Log-flagged case.
// A graveyard-wide grant makes a non-flashback instant castable from the
// graveyard, and on resolution the card is EXILED (§702.34c), not returned
// to the graveyard.
// ---------------------------------------------------------------------------

func TestGYAudit_GrantedFlashback_CastResolvesToExile(t *testing.T) {
	gs := gyAuditGame(t)
	gs.Seats[0].ManaPool = 10

	// A vanilla instant with NO intrinsic flashback — the grant is the only route.
	spell := plainInstant("Opt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, spell)

	// Iroh-shape grant: instants/sorceries flashback for their mana cost.
	RegisterGraveyardFlashbackGrant(gs, &GraveyardFlashbackGrant{
		Controller:      0,
		SourceTimestamp: 1,
		SourceName:      "Iroh, Grand Lotus",
		OnlyActiveTurn:  true,
		CostFor: func(c *Card) int {
			if c != nil && (cardHasType(c, "instant") || cardHasType(c, "sorcery")) {
				return ManaCostOf(c)
			}
			return -1
		},
	})

	if _, err := CastFlashback(gs, 0, spell, -1); err != nil {
		t.Fatalf("granted flashback cast failed: %v", err)
	}
	if inGrave(gs.Seats[0], spell) {
		t.Fatal("spell must leave the graveyard when cast")
	}
	if len(gs.Stack) != 1 || !ShouldExileOnResolve(gs.Stack[0]) {
		t.Fatal("granted-flashback cast must be flagged exile_on_resolve")
	}

	ResolveStackTop(gs)

	if inGrave(gs.Seats[0], spell) {
		t.Error("granted-flashback spell must NOT return to graveyard (§702.34c)")
	}
	if !inExile(gs.Seats[0], spell) {
		t.Error("granted-flashback spell must be EXILED after resolution (§702.34c)")
	}
}

// Off-turn the grant is inactive (Iroh "during your turn") — no cast.
func TestGYAudit_GrantedFlashback_InactiveOffTurn(t *testing.T) {
	gs := gyAuditGame(t)
	gs.Active = 1 // opponent's turn
	gs.Seats[0].ManaPool = 10
	spell := plainInstant("Opt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, spell)
	RegisterGraveyardFlashbackGrant(gs, &GraveyardFlashbackGrant{
		Controller: 0, SourceTimestamp: 1, SourceName: "Iroh", OnlyActiveTurn: true,
		CostFor: func(c *Card) int { return ManaCostOf(c) },
	})
	if _, err := CastFlashback(gs, 0, spell, -1); err == nil {
		t.Fatal("granted flashback must be inactive on the opponent's turn")
	}
}

// ---------------------------------------------------------------------------
// Intrinsic flashback — sanity that the printed keyword path also exiles.
// ---------------------------------------------------------------------------

func TestGYAudit_IntrinsicFlashback_ResolvesToExile(t *testing.T) {
	gs := gyAuditGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newFlashbackCard("Faithless Looting", 0, 1, "{2}{R}")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, spell)

	if _, err := CastFlashback(gs, 0, spell, -1); err != nil {
		t.Fatalf("intrinsic flashback cast failed: %v", err)
	}
	ResolveStackTop(gs)
	if !inExile(gs.Seats[0], spell) || inGrave(gs.Seats[0], spell) {
		t.Error("intrinsic flashback spell must end in exile")
	}
}

// ---------------------------------------------------------------------------
// Escape (§702.148) — exile-N additional cost; spell exiles on resolution.
// ---------------------------------------------------------------------------

func TestGYAudit_Escape_ExilesFodderAndResolvesToExile(t *testing.T) {
	gs := gyAuditGame(t)
	gs.Seats[0].ManaPool = 10
	spell := newEscapeCard("Bond of Flourishing", 0, 3, "{2}{R}", float64(2))
	f1 := plainInstant("Fodder1", 0, 1, "sorcery")
	f2 := plainInstant("Fodder2", 0, 1, "sorcery")
	f3 := plainInstant("Fodder3", 0, 1, "sorcery")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, spell, f1, f2, f3)

	res, err := CastWithEscape(gs, 0, spell, -1, []*Card{f1, f2})
	if err != nil {
		t.Fatalf("escape cast failed: %v", err)
	}
	if len(res.ExiledCards) != 2 {
		t.Errorf("escape must exile exactly 2 fodder cards; got %d", len(res.ExiledCards))
	}
	if !inExile(gs.Seats[0], f1) || !inExile(gs.Seats[0], f2) {
		t.Error("exiled fodder must be in exile")
	}
	if !inGrave(gs.Seats[0], f3) {
		t.Error("un-chosen fodder must stay in the graveyard")
	}
	ResolveStackTop(gs)
	if !inExile(gs.Seats[0], spell) || inGrave(gs.Seats[0], spell) {
		t.Error("escape spell must end in exile after resolution (§702.148b)")
	}
}

// The exile fodder can't include the spell itself (§702.148a "other").
func TestGYAudit_Escape_CannotExileSelf(t *testing.T) {
	gs := gyAuditGame(t)
	gs.Seats[0].ManaPool = 10
	spell := newEscapeCard("Kroxa", 0, 2, "{1}{B}", float64(1))
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, spell)
	if _, err := CastWithEscape(gs, 0, spell, -1, []*Card{spell}); err == nil {
		t.Fatal("escape must reject exiling the spell itself as fodder")
	}
}

// ---------------------------------------------------------------------------
// Dredge (§702.52) — mill N, return the card to hand; blocked if library < N.
// ---------------------------------------------------------------------------

func newDredgeCard(name string, owner, n int) *Card {
	return &Card{
		Name: name, Owner: owner, Types: []string{"creature"},
		AST: &gameast.CardAST{Name: name, Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "dredge", Args: []any{float64(n)}},
		}},
	}
}

func TestGYAudit_Dredge_MillsAndReturns(t *testing.T) {
	gs := gyAuditGame(t)
	card := newDredgeCard("Stinkweed Imp", 0, 5)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	for i := 0; i < 6; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, plainInstant("Lib", 0, 1))
	}
	gyBefore := len(gs.Seats[0].Graveyard)

	if !ActivateDredge(gs, 0, card) {
		t.Fatal("dredge should succeed with 6 cards in library (N=5)")
	}
	// Card returned to hand.
	inHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			inHand = true
		}
	}
	if !inHand {
		t.Error("dredged card must return to hand")
	}
	// 5 milled (library 6 -> 1), graveyard net: -1 (card left) +5 (milled) = +4.
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("dredge N=5 should leave 1 card in library; got %d", len(gs.Seats[0].Library))
	}
	if got := len(gs.Seats[0].Graveyard) - gyBefore; got != 4 {
		t.Errorf("graveyard delta should be +4 (5 milled - 1 returned); got %d", got)
	}
}

func TestGYAudit_Dredge_BlockedIfLibraryTooSmall(t *testing.T) {
	gs := gyAuditGame(t)
	card := newDredgeCard("Golgari Grave-Troll", 0, 6)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	for i := 0; i < 3; i++ { // only 3 < 6
		gs.Seats[0].Library = append(gs.Seats[0].Library, plainInstant("Lib", 0, 1))
	}
	if ActivateDredge(gs, 0, card) {
		t.Fatal("dredge must fail when library has fewer than N cards (§702.52a)")
	}
	if !inGrave(gs.Seats[0], card) {
		t.Error("a failed dredge must leave the card in the graveyard")
	}
}

// ---------------------------------------------------------------------------
// Jump-start (§702.131) — discard a card as additional cost; spell exiles.
// ---------------------------------------------------------------------------

func TestGYAudit_JumpStart_DiscardsAndExiles(t *testing.T) {
	gs := gyAuditGame(t)
	gs.Seats[0].ManaPool = 10
	spell := &Card{
		Name: "Chemister's Insight", Owner: 0, Types: []string{"instant"}, CMC: 4,
		AST: &gameast.CardAST{Name: "Chemister's Insight", Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "jump-start"},
		}},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, spell)
	discard := plainInstant("DiscardMe", 0, 1)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, discard)

	if !CastWithJumpStart(gs, 0, spell, discard) {
		t.Fatal("jump-start cast should succeed (discard a card from hand)")
	}
	// Discarded card is in graveyard; spell on stack flagged exile-on-resolve.
	if !inGrave(gs.Seats[0], discard) {
		t.Error("jump-start discard cost must put the discarded card in the graveyard")
	}
	if len(gs.Stack) != 1 || !ShouldExileOnResolve(gs.Stack[0]) {
		t.Fatal("jump-start spell must be flagged exile_on_resolve (§702.131a)")
	}
	ResolveStackTop(gs)
	if !inExile(gs.Seats[0], spell) || inGrave(gs.Seats[0], spell) {
		t.Error("jump-start spell must end in exile after resolution")
	}
}
