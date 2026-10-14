package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Mayhem tests — CR §702.187
// ---------------------------------------------------------------------------

func newMayhemCard(name string, owner, cmc int, mayhemArg string) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "mayhem", Args: []any{mayhemArg}},
			},
		},
	}
}

func newMayhemGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(43))
	return NewGameState(2, rng, nil)
}

// ---------------------------------------------------------------------------
// HasMayhem / MayhemCost
// ---------------------------------------------------------------------------

func TestHasMayhem_Detects(t *testing.T) {
	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	if !HasMayhem(card) {
		t.Fatal("HasMayhem returned false for a card with mayhem keyword")
	}
}

func TestHasMayhem_Negative(t *testing.T) {
	card := newInstantCard("Lightning Bolt", 0, 1)
	if HasMayhem(card) {
		t.Fatal("HasMayhem returned true for a card without mayhem")
	}
}

func TestMayhemCost_ParsesManaString(t *testing.T) {
	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	if got := MayhemCost(card); got != 1 {
		t.Fatalf("MayhemCost = %d, want 1 (for {R})", got)
	}
}

func TestMayhemCost_NumericArg(t *testing.T) {
	card := &Card{
		Name: "Numeric Mayhem", Owner: 0, Types: []string{"sorcery"}, CMC: 3,
		AST: &gameast.CardAST{
			Name: "Numeric Mayhem",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "mayhem", Args: []any{2}},
			},
		},
	}
	if got := MayhemCost(card); got != 2 {
		t.Fatalf("MayhemCost numeric = %d, want 2", got)
	}
}

func TestMayhemCost_NoArgsFallsBackToCMC(t *testing.T) {
	card := &Card{
		Name: "Argless Mayhem", Owner: 0, Types: []string{"sorcery"}, CMC: 4,
		AST: &gameast.CardAST{
			Name: "Argless Mayhem",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "mayhem"},
			},
		},
	}
	if got := MayhemCost(card); got != 4 {
		t.Fatalf("MayhemCost no-args = %d, want 4 (CMC fallback)", got)
	}
}

// ---------------------------------------------------------------------------
// DiscardCard wires MayhemDiscards (the eligibility gate)
// ---------------------------------------------------------------------------

func TestDiscardCard_RecordsMayhemEligibility(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Turn = 4
	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	DiscardCard(gs, card, 0)

	if !MayhemEligible(gs, card) {
		t.Fatal("MayhemEligible should be true immediately after discard on the same turn")
	}
	if gs.MayhemDiscards[card] != 4 {
		t.Fatalf("MayhemDiscards[card] = %d, want 4 (current turn)", gs.MayhemDiscards[card])
	}
}

func TestDiscardCard_NonMayhemNotRecorded(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Turn = 4
	card := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	DiscardCard(gs, card, 0)

	if MayhemEligible(gs, card) {
		t.Fatal("MayhemEligible should be false for a card without mayhem")
	}
}

// ---------------------------------------------------------------------------
// CastMayhem — happy path
// ---------------------------------------------------------------------------

func TestCastMayhem_PaysCostAndPushesStack(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Active = 0
	gs.Turn = 3
	gs.Seats[0].ManaPool = 2

	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0) // moves to graveyard + marks mayhem-eligible

	result, err := CastMayhem(gs, 0, card, 1)
	if err != nil {
		t.Fatalf("CastMayhem returned error: %v", err)
	}
	if result == nil {
		t.Fatal("CastMayhem returned nil result on success")
	}
	// Mana paid: pool was 2, cost 1 → 1 left.
	if gs.Seats[0].ManaPool != 1 {
		t.Fatalf("expected 1 mana left, got %d", gs.Seats[0].ManaPool)
	}
	// Card removed from graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("mayhem card should no longer be in graveyard")
		}
	}
	// Stack item present, tagged exile_on_resolve.
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
	// r99 — INVERTED. §702.187 has three subrules and none is an exile
	// clause; the "§702.187c" this used to cite is the costless variant
	// of the alternative cost. Asserted as the absence of the key AND
	// through the predicate, so neither a stale stamp nor a changed
	// predicate can pass this silently.
	if _, present := item.CostMeta["exile_on_resolve"]; present {
		t.Fatalf("mayhem stack item carries exile_on_resolve; §702.187 has no exile clause "+
			"(CostMeta=%+v)", item.CostMeta)
	}
	if ShouldExileOnResolve(item) {
		t.Fatal("ShouldExileOnResolve is true for a mayhem item")
	}
	if v := item.CostMeta["mayhem"]; v != true {
		t.Fatalf("CostMeta[\"mayhem\"] = %v, want true", v)
	}
	if v := item.CostMeta["mayhem_cost"]; v != 1 {
		t.Fatalf("CostMeta[\"mayhem_cost\"] = %v, want 1", v)
	}
	if !SpellMayhemCastThisTurn(gs, 0) {
		t.Fatal("SpellMayhemCastThisTurn(0) returned false after a mayhem cast")
	}
}

func TestCastMayhem_NegOneUsesPrintedCost(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Active = 0
	gs.Turn = 3
	gs.Seats[0].ManaPool = 1

	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)

	if _, err := CastMayhem(gs, 0, card, -1); err != nil {
		t.Fatalf("CastMayhem(-1) failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected 0 mana left after paying printed {R}, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// CastMayhem — failure paths
// ---------------------------------------------------------------------------

func TestCastMayhem_NoKeyword(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Turn = 3
	gs.Seats[0].ManaPool = 5
	card := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	if _, err := CastMayhem(gs, 0, card, 1); err == nil {
		t.Fatal("CastMayhem should fail for a card without mayhem")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("mana should be unchanged, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastMayhem_NotDiscardedThisTurn(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Turn = 3
	gs.Seats[0].ManaPool = 5
	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	// Card sits in graveyard without ever passing through DiscardCard.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	if _, err := CastMayhem(gs, 0, card, 1); err == nil {
		t.Fatal("CastMayhem should fail when card was not discarded this turn")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("mana should be unchanged after failed CastMayhem, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastMayhem_DiscardedPreviousTurnIneligible(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Active = 0
	gs.Turn = 5
	gs.Seats[0].ManaPool = 5

	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	// Discard on turn 5...
	DiscardCard(gs, card, 0)
	// ...then time passes.
	gs.Turn = 6

	if MayhemEligible(gs, card) {
		t.Fatal("MayhemEligible should be false on a later turn")
	}
	if _, err := CastMayhem(gs, 0, card, 1); err == nil {
		t.Fatal("CastMayhem should fail when discard was on a prior turn")
	}
}

func TestCastMayhem_InsufficientMana(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Turn = 3
	gs.Seats[0].ManaPool = 0
	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)
	if _, err := CastMayhem(gs, 0, card, 1); err == nil {
		t.Fatal("CastMayhem should fail with insufficient mana")
	}
	// Card stays in graveyard since the cost-check rejected the cast.
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("card should remain in graveyard after failed CastMayhem")
	}
}

// ---------------------------------------------------------------------------
// End-to-end: discard → mayhem-cast → resolve → exile
// ---------------------------------------------------------------------------

// r99 — was TestMayhem_DiscardCastResolveExile_EndToEnd, which pinned
// the bug. Re-pointed at the POSITIVE destination (the graveyard)
// rather than having its exile assertion deleted: "not in exile" also
// passes for a card that vanished entirely.
func TestMayhem_DiscardCastResolvesToGraveyard_EndToEnd(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Active = 0
	gs.Turn = 3
	gs.Seats[0].ManaPool = 1
	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	// 1. Discard — lands in graveyard, marked mayhem-eligible.
	DiscardCard(gs, card, 0)
	if !MayhemEligible(gs, card) {
		t.Fatal("expected MayhemEligible after discard")
	}

	// 2. Cast from graveyard via mayhem.
	if _, err := CastMayhem(gs, 0, card, 1); err != nil {
		t.Fatalf("CastMayhem failed: %v", err)
	}

	// 3. Resolve — mayhem has no exile clause, so the spell takes the
	// ordinary post-stack routing to its owner's graveyard, where it
	// stays (it is no longer mayhem-eligible, since it was not
	// discarded again).
	ResolveStackTop(gs)

	foundInGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			foundInGrave = true
			break
		}
	}
	if !foundInGrave {
		t.Fatal("mayhem spell should be in its owner's graveyard after resolution")
	}
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			t.Fatal("mayhem spell was exiled; §702.187 has no exile clause")
		}
	}
}

// ---------------------------------------------------------------------------
// EndOfTurnCleanup expires mayhem eligibility
// ---------------------------------------------------------------------------

func TestClearMayhemDiscards_ExpiresWindow(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Turn = 3
	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)
	if !MayhemEligible(gs, card) {
		t.Fatal("expected MayhemEligible after discard")
	}

	ClearMayhemDiscards(gs)
	if MayhemEligible(gs, card) {
		t.Fatal("MayhemEligible should be false after ClearMayhemDiscards")
	}
}

// CastMayhem on a card that was already mayhem-cast this turn must be
// rejected — the eligibility record is consumed on success.
func TestCastMayhem_OneShotPerDiscard(t *testing.T) {
	gs := newMayhemGame(t)
	gs.Active = 0
	gs.Turn = 3
	gs.Seats[0].ManaPool = 2
	card := newMayhemCard("Voltage Surge", 0, 2, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	DiscardCard(gs, card, 0)

	if _, err := CastMayhem(gs, 0, card, 1); err != nil {
		t.Fatalf("first CastMayhem failed: %v", err)
	}
	// Pretend the card came back to graveyard from the stack.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastMayhem(gs, 0, card, 1); err == nil {
		t.Fatal("CastMayhem should fail on a card whose mayhem eligibility was already consumed")
	}
}
