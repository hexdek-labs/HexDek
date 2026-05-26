package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 — closes half-finished-features-r48 #7: Tasigur's auto-gen body
// was mill-only, dropping the return-from-graveyard half (the entire
// reason to play Tasigur). The custom handler in
// custom_tasigur_the_golden_fang.go now: pays {4}, mills 2, picks the
// first living opponent clockwise, has that opponent adversarially
// choose the LOWEST-CMC nonland card from the activator's graveyard,
// and returns it to the activator's hand. Lands and empty graveyards
// produce a no-legal-return partial.
//
// The delve cast cost reduction is owned by
// internal/gameengine/keywords_delve_cast.go::CastWithDelve (tested
// alongside in the same PR; see TestCastWithDelve_* in this file).

// -----------------------------------------------------------------------
// Activated ability — return-from-graveyard heuristic
// -----------------------------------------------------------------------

func TestTasigur_Activate_OpponentReturnsLowestCMCNonland(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 4
	// Library has 2 fillers to mill.
	for i := 0; i < 2; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
			Name:      "Filler",
			Owner:     0,
			Types:     []string{"creature"},
			CMC:       2,
		})
	}
	// Graveyard already has three nonland candidates + one land.
	cheap := &gameengine.Card{Name: "Brainstorm", Owner: 0, Types: []string{"instant"}, CMC: 1}
	mid := &gameengine.Card{Name: "Eternal Witness", Owner: 0, Types: []string{"creature"}, CMC: 3}
	expensive := &gameengine.Card{Name: "Cataclysmic Gearhulk", Owner: 0, Types: []string{"creature"}, CMC: 5}
	land := &gameengine.Card{Name: "Swamp", Owner: 0, Types: []string{"land"}, CMC: 0}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, expensive, mid, cheap, land)

	tasigur := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Tasigur, the Golden Fang", Owner: 0, Types: []string{"creature", "legendary"}, CMC: 5},
		Controller: 0,
		Owner:      0,
	}

	tasigurTheGoldenFangActivateCustom(gs, tasigur, 0, nil)

	// Cheap (CMC 1) is the lowest-CMC nonland → opponent picks it →
	// returns to seat 0's hand.
	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("expected 1 card returned to hand, got %d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Hand[0].Name != "Brainstorm" {
		t.Fatalf("opponent should pick lowest-CMC nonland (Brainstorm, CMC 1); got %q",
			gs.Seats[0].Hand[0].Name)
	}
	// Land must stay in graveyard — ineligible.
	foundLand := false
	for _, c := range gs.Seats[0].Graveyard {
		if c != nil && c.Name == "Swamp" {
			foundLand = true
		}
	}
	if !foundLand {
		t.Error("Swamp should remain in graveyard (lands are not legal return targets)")
	}
}

func TestTasigur_Activate_OnlyLandsInGraveyardSkipsReturn(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 4
	// Mill targets.
	for i := 0; i < 2; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
			Name:  "Filler",
			Owner: 0,
			Types: []string{"land"},
		})
	}
	// Graveyard has lands only.
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Mountain", Owner: 0, Types: []string{"land"}},
		{Name: "Forest", Owner: 0, Types: []string{"land"}},
	}

	tasigur := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Tasigur, the Golden Fang", Owner: 0, Types: []string{"creature", "legendary"}, CMC: 5},
		Controller: 0,
		Owner:      0,
	}

	preHand := len(gs.Seats[0].Hand)
	tasigurTheGoldenFangActivateCustom(gs, tasigur, 0, nil)

	if len(gs.Seats[0].Hand) != preHand {
		t.Errorf("only-lands graveyard: expected hand unchanged, was %d now %d",
			preHand, len(gs.Seats[0].Hand))
	}
	// Mill still happened (2 cards from library to graveyard).
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("expected library empty after mill 2, got %d", len(gs.Seats[0].Library))
	}
}

func TestTasigur_Activate_EmptyGraveyardAfterMillIsNoReturn(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 4
	// Library empty → mill 0 → graveyard stays empty.
	gs.Seats[0].Library = nil
	gs.Seats[0].Graveyard = nil

	tasigur := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Tasigur, the Golden Fang", Owner: 0, Types: []string{"creature", "legendary"}, CMC: 5},
		Controller: 0,
		Owner:      0,
	}

	tasigurTheGoldenFangActivateCustom(gs, tasigur, 0, nil)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("empty graveyard: expected no return, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTasigur_Activate_InsufficientManaFails(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 2 // need 4
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Filler", Owner: 0, Types: []string{"creature"}},
	}
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Brainstorm", Owner: 0, Types: []string{"instant"}, CMC: 1},
	}

	tasigur := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Tasigur, the Golden Fang", Owner: 0, Types: []string{"creature", "legendary"}, CMC: 5},
		Controller: 0,
		Owner:      0,
	}

	tasigurTheGoldenFangActivateCustom(gs, tasigur, 0, nil)

	// No mana paid (PayGenericCost refused), no mill, no return.
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("insufficient mana: mana pool should be unchanged at 2, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("insufficient mana: should not have milled; library=%d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("insufficient mana: should not have returned; hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------
// 4-player adversarial choice — opponent picks across multiple players
// -----------------------------------------------------------------------

func TestTasigur_Activate_4Player_FirstLivingOpponentClockwise(t *testing.T) {
	gs := gameengine.NewGameState(4, nil, nil)
	gs.Seats[0].ManaPool = 4
	// Filler library cards: CMC=99 so when milled into graveyard they
	// don't undercut the intentional CMC=1 return candidate.
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Filler", Owner: 0, Types: []string{"creature"}, CMC: 99},
		{Name: "Filler", Owner: 0, Types: []string{"creature"}, CMC: 99},
	}
	// Seat 1 (clockwise from active) is alive.
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Cheap Spell", Owner: 0, Types: []string{"instant"}, CMC: 1},
		{Name: "Expensive Spell", Owner: 0, Types: []string{"sorcery"}, CMC: 5},
	}

	tasigur := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Tasigur, the Golden Fang", Owner: 0, Types: []string{"creature", "legendary"}, CMC: 5},
		Controller: 0,
		Owner:      0,
	}

	tasigurTheGoldenFangActivateCustom(gs, tasigur, 0, nil)

	// Lowest-CMC nonland (cheap spell, CMC 1) returns.
	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0].Name != "Cheap Spell" {
		t.Fatalf("expected Cheap Spell returned, got hand=%v", handNames(gs.Seats[0].Hand))
	}
}

func TestTasigur_Activate_4Player_SkipsLostOpponents(t *testing.T) {
	gs := gameengine.NewGameState(4, nil, nil)
	gs.Seats[0].ManaPool = 4
	gs.Seats[1].Lost = true // first clockwise opponent dead — skip
	gs.Seats[2].Lost = true // second too
	// Seat 3 is the only living opponent.

	// Filler library card: CMC=99 so when milled it doesn't undercut
	// the intentional CMC=1 return candidate.
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Filler", Owner: 0, Types: []string{"creature"}, CMC: 99},
	}
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1},
	}

	tasigur := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Tasigur, the Golden Fang", Owner: 0, Types: []string{"creature", "legendary"}, CMC: 5},
		Controller: 0,
		Owner:      0,
	}

	tasigurTheGoldenFangActivateCustom(gs, tasigur, 0, nil)

	// Return should still happen — seat 3 makes the choice (only legal
	// candidate is Lightning Bolt).
	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0].Name != "Lightning Bolt" {
		t.Fatalf("expected Lightning Bolt returned (seat 3 is the only living opponent and only legal target); got hand=%v",
			handNames(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------
// CastWithDelve — engine-level cast cost reduction
// -----------------------------------------------------------------------

func TestCastWithDelve_PaysGenericFromGraveyard(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 2 // Tasigur is CMC 5; delve 3 from graveyard + pay 2.
	// Build a Tasigur card with the delve AST keyword.
	tasigurCard := makeDelveCard(t, "Tasigur, the Golden Fang", 5)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, tasigurCard)
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Junk 1", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk 2", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk 3", Owner: 0, Types: []string{"instant"}},
	}

	res, err := gameengine.CastWithDelve(gs, 0, tasigurCard, 3)
	if err != nil {
		t.Fatalf("CastWithDelve(3): %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil CostPaymentResult on success")
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected ManaPool drained to 0 (paid 2 net), got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected graveyard empty after delving 3, got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Exile) != 3 {
		t.Errorf("expected 3 cards in exile (delve targets), got %d", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected card removed from hand, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	if !gameengine.IsDelveCast(gs.Stack[0]) {
		t.Errorf("IsDelveCast should report true on stack top")
	}
}

func TestCastWithDelve_NoDelveKeywordRejects(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 5
	// Plain card without delve.
	plain := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, plain)

	_, err := gameengine.CastWithDelve(gs, 0, plain, 1)
	if err == nil {
		t.Fatal("expected error on non-delve card, got nil")
	}
	cerr, ok := err.(*gameengine.CastError)
	if !ok {
		t.Fatalf("expected *CastError, got %T", err)
	}
	if cerr.Reason != "no_delve_keyword" {
		t.Errorf("expected reason=no_delve_keyword, got %q", cerr.Reason)
	}
}

func TestCastWithDelve_ExceedsGraveyardRejects(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 5
	tasigurCard := makeDelveCard(t, "Tasigur, the Golden Fang", 5)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, tasigurCard)
	// Only 2 cards in graveyard.
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Junk 1", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk 2", Owner: 0, Types: []string{"instant"}},
	}

	// Try to delve 3 → fail before mutating anything.
	_, err := gameengine.CastWithDelve(gs, 0, tasigurCard, 3)
	if err == nil {
		t.Fatal("expected error when delveCount > len(graveyard)")
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("card should remain in hand on failure, hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("mana pool should be untouched on failure, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("no cards should be exiled on failure, got %d", len(gs.Seats[0].Exile))
	}
}

func TestCastWithDelve_DelveExceedsGenericCostRejects(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0
	tasigurCard := makeDelveCard(t, "Tasigur, the Golden Fang", 5)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, tasigurCard)
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
	}

	// Tasigur is CMC 5; trying to delve 6 exceeds the generic portion.
	_, err := gameengine.CastWithDelve(gs, 0, tasigurCard, 6)
	if err == nil {
		t.Fatal("expected error when delveCount > normalCost")
	}
	cerr, ok := err.(*gameengine.CastError)
	if !ok || cerr.Reason != "delve_exceeds_generic_cost" {
		t.Errorf("expected reason=delve_exceeds_generic_cost, got %v", err)
	}
}

func TestCastWithDelve_FullDelveZeroNetCost(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0 // 0 mana, paying all 5 via delve.
	tasigurCard := makeDelveCard(t, "Tasigur, the Golden Fang", 5)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, tasigurCard)
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
		{Name: "Junk", Owner: 0, Types: []string{"instant"}},
	}

	if _, err := gameengine.CastWithDelve(gs, 0, tasigurCard, 5); err != nil {
		t.Fatalf("CastWithDelve(full cost): %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("mana pool should be 0 (no net cost), got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("graveyard should be empty after full-delve, got %d", len(gs.Seats[0].Graveyard))
	}
}

// -----------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------

// makeDelveCard returns a Card with an AST Keyword{Name:"delve"} ability
// so HasDelve / CastWithDelve recognise it. Mirrors the shape used by
// TestDelve_ExilesFromGraveyard in keywords_misc_test.go.
func makeDelveCard(t *testing.T, name string, cmc int) *gameengine.Card {
	t.Helper()
	c := &gameengine.Card{Name: name, Owner: 0, Types: []string{"creature", "legendary"}, CMC: cmc}
	c.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "delve"},
		},
	}
	return c
}
