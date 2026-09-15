package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Madness tests — CR §702.35
// ---------------------------------------------------------------------------

func newMadnessCard(name string, owner, cmc int, madnessArg string) *Card {
	args := []any{}
	if madnessArg != "" {
		args = append(args, madnessArg)
	}
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "madness", Args: args},
			},
		},
	}
}

func newPlainCardForMadness(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		AST:   &gameast.CardAST{Name: name},
	}
}

func newMadnessGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(31))
	gs := NewGameState(2, rng, nil)
	gs.Active = 0
	gs.Step = "precombat_main"
	gs.Turn = 1
	return gs
}

func handHas(seat *Seat, card *Card) bool {
	for _, c := range seat.Hand {
		if c == card {
			return true
		}
	}
	return false
}

func exileHas(seat *Seat, card *Card) bool {
	for _, c := range seat.Exile {
		if c == card {
			return true
		}
	}
	return false
}

func graveyardHas(seat *Seat, card *Card) bool {
	for _, c := range seat.Graveyard {
		if c == card {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// HasMadness / MadnessCostParsed
// ---------------------------------------------------------------------------

func TestMadnessCostParsed_ManaString(t *testing.T) {
	c := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	if got := MadnessCostParsed(c); got != 1 {
		t.Fatalf("MadnessCostParsed = %d, want 1 (for {R})", got)
	}
}

func TestMadnessCostParsed_NoKeyword(t *testing.T) {
	c := newPlainCardForMadness("Plain Instant", 0)
	if got := MadnessCostParsed(c); got != 0 {
		t.Fatalf("MadnessCostParsed = %d, want 0 (no keyword)", got)
	}
}

// ---------------------------------------------------------------------------
// (a) Discard with madness exiles instead of going to graveyard
// ---------------------------------------------------------------------------

func TestDiscardMadness_ExilesInsteadOfGraveyard(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Turn = 3
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	// Drive the live discard path — DiscardCard should route the
	// madness card to exile, not graveyard.
	DiscardCard(gs, card, 0)

	if handHas(gs.Seats[0], card) {
		t.Fatal("card should be removed from hand after discard")
	}
	if graveyardHas(gs.Seats[0], card) {
		t.Fatal("madness card must NOT go to graveyard on discard (§702.34a)")
	}
	if !exileHas(gs.Seats[0], card) {
		t.Fatal("madness card should be in exile after discard")
	}
	// Per-card meta stamped.
	if card.Meta == nil {
		t.Fatal("card.Meta should be initialised by OnDiscardMadness")
	}
	if got := card.Meta["madness_exile_turn"]; got != 3 {
		t.Fatalf("Meta[\"madness_exile_turn\"] = %v, want 3", got)
	}
	if got := card.Meta["madness_exiled_by_seat"]; got != 0 {
		t.Fatalf("Meta[\"madness_exiled_by_seat\"] = %v, want 0", got)
	}
	// Game-level side map populated.
	if !HasOpenMadnessWindow(gs, 0, card) {
		t.Fatal("HasOpenMadnessWindow should be true after a madness discard")
	}
	// ZoneCastPermission registered so the AI / Hat can see the
	// cast option from exile.
	grant := GetZoneCastGrant(gs, card)
	if grant == nil {
		t.Fatal("expected a ZoneCastPermission to be registered after madness exile")
	}
	if grant.Zone != ZoneExile || grant.Keyword != "madness" {
		t.Fatalf("grant zone/keyword = %q/%q, want %q/\"madness\"", grant.Zone, grant.Keyword, ZoneExile)
	}
	if grant.RequireController != 0 {
		t.Fatalf("grant.RequireController = %d, want 0", grant.RequireController)
	}
	if grant.ManaCost != 1 {
		t.Fatalf("grant.ManaCost = %d, want 1 (parsed {R})", grant.ManaCost)
	}
}

// ---------------------------------------------------------------------------
// (b) Cast-during-window succeeds + CostMeta stamped
// ---------------------------------------------------------------------------

func TestCastWithMadness_DuringWindow_SucceedsAndStampsCostMeta(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Seats[0].ManaPool = 3
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	// Stage 1: discard moves the card to exile and opens the window.
	DiscardCard(gs, card, 0)

	// Stage 2: cast for madness cost.
	if _, err := CastWithMadness(gs, 0, card, 1); err != nil {
		t.Fatalf("CastWithMadness failed: %v", err)
	}
	// Mana spent.
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("mana left = %d, want 2 (paid madness 1 of 3)", gs.Seats[0].ManaPool)
	}
	// Card off exile and on the stack.
	if exileHas(gs.Seats[0], card) {
		t.Fatal("card should be off exile after CastWithMadness")
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]
	if item.Card != card {
		t.Fatal("stack item should reference the madness-cast card")
	}
	if item.CastZone != ZoneExile {
		t.Fatalf("CastZone = %q, want %q", item.CastZone, ZoneExile)
	}
	if v, ok := item.CostMeta["madness_cast"]; !ok || v != true {
		t.Fatalf("CostMeta[\"madness_cast\"] = %v, want true", item.CostMeta["madness_cast"])
	}
	if v, ok := item.CostMeta["madness_cost"]; !ok || v != 1 {
		t.Fatalf("CostMeta[\"madness_cost\"] = %v, want 1", item.CostMeta["madness_cost"])
	}
	if v, ok := item.CostMeta["zone_cast_keyword"]; !ok || v != "madness" {
		t.Fatalf("CostMeta[\"zone_cast_keyword\"] = %v, want \"madness\"", item.CostMeta["zone_cast_keyword"])
	}
	if !IsMadnessCast(item) {
		t.Fatal("IsMadnessCast should return true for the madness-cast stack item")
	}
	// Madness doesn't change resolution destination.
	if ShouldExileOnResolve(item) {
		t.Fatal("madness cast must not stamp exile_on_resolve")
	}
	// Per-turn flag flipped.
	if !SpellMadnessCastThisTurn(gs, 0) {
		t.Fatal("SpellMadnessCastThisTurn should be true after a madness cast")
	}
	// Window + grant consumed.
	if HasOpenMadnessWindow(gs, 0, card) {
		t.Fatal("madness window should be closed after CastWithMadness consumes it")
	}
	if GetZoneCastGrant(gs, card) != nil {
		t.Fatal("ZoneCastPermission should be removed after CastWithMadness")
	}
}

func TestCastWithMadness_NegativeCostUsesPrinted(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Seats[0].ManaPool = 3
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)

	if _, err := CastWithMadness(gs, 0, card, -1); err != nil {
		t.Fatalf("CastWithMadness with -1 sentinel failed: %v", err)
	}
	// Printed cost is {R} = 1 CMC.
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("mana left = %d, want 2 (printed MadnessCostParsed 1 of 3)", gs.Seats[0].ManaPool)
	}
}

func TestCastWithMadness_RejectsWithoutWindow(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Seats[0].ManaPool = 3
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	// Card sitting in exile without an open window (e.g. exiled by
	// some other effect).
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, card)

	_, err := CastWithMadness(gs, 0, card, 1)
	if err == nil {
		t.Fatal("CastWithMadness should fail when no madness window is open")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "no_madness_window" {
		t.Fatalf("expected CastError no_madness_window, got %v", err)
	}
}

func TestCastWithMadness_RejectsWrongSeat(t *testing.T) {
	gs := newMadnessGame(t)
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)
	// An opponent attempting to cast it doesn't get the window.
	gs.Seats[1].ManaPool = 3
	_, err := CastWithMadness(gs, 1, card, 1)
	if err == nil {
		t.Fatal("CastWithMadness should fail when invoked by the wrong seat")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "wrong_madness_seat" {
		t.Fatalf("expected CastError wrong_madness_seat, got %v", err)
	}
}

func TestCastWithMadness_InsufficientMana(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Seats[0].ManaPool = 0
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)

	_, err := CastWithMadness(gs, 0, card, 1)
	if err == nil {
		t.Fatal("CastWithMadness should fail with insufficient mana")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "insufficient_mana" {
		t.Fatalf("expected CastError insufficient_mana, got %v", err)
	}
	// Card still in exile, window still open.
	if !exileHas(gs.Seats[0], card) {
		t.Fatal("card should remain in exile after failed cast")
	}
	if !HasOpenMadnessWindow(gs, 0, card) {
		t.Fatal("madness window should remain open after failed cast")
	}
}

// ---------------------------------------------------------------------------
// (c) Decline cast routes to graveyard at end of window
// ---------------------------------------------------------------------------

func TestResolveMadnessWindow_DeclineRoutesToGraveyard(t *testing.T) {
	gs := newMadnessGame(t)
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)

	if !exileHas(gs.Seats[0], card) {
		t.Fatal("setup: card should be in exile after discard")
	}

	routed := ResolveMadnessWindow(gs, 0)
	if routed != 1 {
		t.Fatalf("ResolveMadnessWindow routed %d cards, want 1", routed)
	}
	if exileHas(gs.Seats[0], card) {
		t.Fatal("card should no longer be in exile after the window closes")
	}
	if !graveyardHas(gs.Seats[0], card) {
		t.Fatal("declined madness card must be routed to graveyard (§702.34a)")
	}
	if HasOpenMadnessWindow(gs, 0, card) {
		t.Fatal("madness window should be cleared after ResolveMadnessWindow")
	}
	if GetZoneCastGrant(gs, card) != nil {
		t.Fatal("ZoneCastPermission should be removed when window closes without a cast")
	}
}

func TestResolveMadnessWindow_AllSeats_WhenSeatIdxNegative(t *testing.T) {
	gs := newMadnessGame(t)
	cardA := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	cardB := newMadnessCard("Big Tantrum", 1, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, cardA)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, cardB)
	DiscardCard(gs, cardA, 0)
	DiscardCard(gs, cardB, 1)

	routed := ResolveMadnessWindow(gs, -1)
	if routed != 2 {
		t.Fatalf("ResolveMadnessWindow(-1) routed %d, want 2 (all seats)", routed)
	}
	if !graveyardHas(gs.Seats[0], cardA) || !graveyardHas(gs.Seats[1], cardB) {
		t.Fatal("both seats' madness cards should be routed to their own graveyards")
	}
}

func TestResolveMadnessWindow_AfterCastIsNoOp(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Seats[0].ManaPool = 3
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)
	if _, err := CastWithMadness(gs, 0, card, 1); err != nil {
		t.Fatalf("CastWithMadness failed: %v", err)
	}

	routed := ResolveMadnessWindow(gs, 0)
	if routed != 0 {
		t.Fatalf("ResolveMadnessWindow after a cast should route 0 (was %d)", routed)
	}
	// Card is on the stack (resolved later); not in graveyard yet.
	if graveyardHas(gs.Seats[0], card) {
		t.Fatal("ResolveMadnessWindow must not route a card that's mid-cast on the stack")
	}
}

// ---------------------------------------------------------------------------
// (d) Discard without madness goes straight to graveyard
// ---------------------------------------------------------------------------

func TestDiscard_NonMadnessGoesStraightToGraveyard(t *testing.T) {
	gs := newMadnessGame(t)
	card := newPlainCardForMadness("Plain Instant", 0)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	DiscardCard(gs, card, 0)

	if handHas(gs.Seats[0], card) {
		t.Fatal("card should be removed from hand after discard")
	}
	if exileHas(gs.Seats[0], card) {
		t.Fatal("non-madness card must NOT be exiled on discard")
	}
	if !graveyardHas(gs.Seats[0], card) {
		t.Fatal("non-madness card should go straight to graveyard on discard")
	}
	if HasOpenMadnessWindow(gs, 0, card) {
		t.Fatal("non-madness discard should not open a madness window")
	}
}

// ---------------------------------------------------------------------------
// Negative-path safety
// ---------------------------------------------------------------------------

func TestOnDiscardMadness_NonMadnessCardNoOp(t *testing.T) {
	gs := newMadnessGame(t)
	card := newPlainCardForMadness("Plain", 0)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	if OnDiscardMadness(gs, 0, card) {
		t.Fatal("OnDiscardMadness must return false for a non-madness card")
	}
	if !handHas(gs.Seats[0], card) {
		t.Fatal("OnDiscardMadness must not move a non-madness card")
	}
}

func TestOnDiscardMadness_CardNotInHandNoOp(t *testing.T) {
	gs := newMadnessGame(t)
	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	// Card NOT in hand.
	if OnDiscardMadness(gs, 0, card) {
		t.Fatal("OnDiscardMadness must return false when card isn't in hand")
	}
}
