package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Escape tests — CR §702.138
// ---------------------------------------------------------------------------

func newEscapeCard(name string, owner, cmc int, escapeArgs ...any) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "escape", Args: escapeArgs},
			},
		},
	}
}

func newFodderCard(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func newEscapeGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(23))
	gs := NewGameState(2, rng, nil)
	gs.Active = 0
	gs.Step = "precombat_main"
	return gs
}

// seedGraveyard appends the cards to seat's graveyard and returns
// the appended slice for ergonomic destructuring in callers.
func seedGraveyard(gs *GameState, seat int, cards ...*Card) []*Card {
	gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, cards...)
	return cards
}

// ---------------------------------------------------------------------------
// HasEscape / EscapeCost / EscapeExileCount
// ---------------------------------------------------------------------------

func TestHasEscape_Detects(t *testing.T) {
	if !HasEscape(newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)) {
		t.Fatal("HasEscape should be true for an escape card")
	}
}

func TestHasEscape_Negative(t *testing.T) {
	if HasEscape(nil) {
		t.Fatal("HasEscape(nil) should be false")
	}
	plain := &Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant"},
		AST:   &gameast.CardAST{Name: "Lightning Bolt"},
	}
	if HasEscape(plain) {
		t.Fatal("HasEscape should be false for a card without the keyword")
	}
}

func TestEscapeCost_ParsesManaString(t *testing.T) {
	card := newEscapeCard("Kroxa, Titan of Death's Hunger", 1, 2, "{1}{B}{R}", 5)
	if got := EscapeCost(card); got != 3 {
		t.Fatalf("EscapeCost = %d, want 3 (for {1}{B}{R})", got)
	}
}

func TestEscapeExileCount_ReadsNumericArg(t *testing.T) {
	card := newEscapeCard("Kroxa, Titan of Death's Hunger", 1, 2, "{1}{B}{R}", 5)
	if got := EscapeExileCount(card); got != 5 {
		t.Fatalf("EscapeExileCount = %d, want 5", got)
	}
}

// ---------------------------------------------------------------------------
// (a) Escape cast succeeds when grave has enough cards
// ---------------------------------------------------------------------------

func TestCastWithEscape_SucceedsWithEnoughFodder(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	fodder := []*Card{
		newFodderCard("Filler A", 0),
		newFodderCard("Filler B", 0),
		newFodderCard("Filler C", 0),
	}
	seedGraveyard(gs, 0, spell)
	seedGraveyard(gs, 0, fodder...)

	result, err := CastWithEscape(gs, 0, spell, 1, fodder)
	if err != nil {
		t.Fatalf("CastWithEscape returned error: %v", err)
	}
	if result == nil {
		t.Fatal("CastWithEscape returned nil result on success")
	}
	if gs.Seats[0].ManaPool != 4 {
		t.Fatalf("mana left = %d, want 4 (paid 1 of 5)", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	// Spell removed from graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == spell {
			t.Fatal("escape-cast spell should be on the stack, not in graveyard")
		}
	}
	if !SpellEscapedThisTurn(gs, 0) {
		t.Fatal("SpellEscapedThisTurn should be true after a successful escape cast")
	}
}

// ---------------------------------------------------------------------------
// (b) Insufficient grave = rejected
// ---------------------------------------------------------------------------

func TestCastWithEscape_RejectedWithInsufficientGraveyard(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	// Only 2 fodder available; need 3.
	fodder := []*Card{
		newFodderCard("Filler A", 0),
		newFodderCard("Filler B", 0),
	}
	seedGraveyard(gs, 0, spell)
	seedGraveyard(gs, 0, fodder...)

	_, err := CastWithEscape(gs, 0, spell, 1, fodder)
	if err == nil {
		t.Fatal("CastWithEscape should fail when caller supplies fewer exile targets than required")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "insufficient_exile_targets" {
		t.Fatalf("expected CastError insufficient_exile_targets, got %v", err)
	}
	// State preserved: spell still in graveyard, fodder still in graveyard,
	// mana untouched, stack empty, per-turn flag false.
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("mana should be untouched, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 0 {
		t.Fatalf("stack should be empty after rejected escape, got %d items", len(gs.Stack))
	}
	if SpellEscapedThisTurn(gs, 0) {
		t.Fatal("SpellEscapedThisTurn should remain false after a rejected escape")
	}
	if len(gs.Seats[0].Graveyard) != 1+len(fodder) {
		t.Fatalf("graveyard size = %d, want %d (no exile-tax paid)",
			len(gs.Seats[0].Graveyard), 1+len(fodder))
	}
}

func TestCastWithEscape_RejectsSpellAsOwnFodder(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	seedGraveyard(gs, 0, spell)
	seedGraveyard(gs, 0,
		newFodderCard("Filler A", 0),
		newFodderCard("Filler B", 0),
	)
	// Try to use the spell itself as one of the exile targets.
	bad := []*Card{spell, gs.Seats[0].Graveyard[1], gs.Seats[0].Graveyard[2]}

	_, err := CastWithEscape(gs, 0, spell, 1, bad)
	if err == nil {
		t.Fatal("CastWithEscape must reject using the spell itself as exile fodder (§702.148a 'other')")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "exile_target_is_spell" {
		t.Fatalf("expected CastError exile_target_is_spell, got %v", err)
	}
}

func TestCastWithEscape_RejectsDuplicateFodder(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	a := newFodderCard("Filler A", 0)
	b := newFodderCard("Filler B", 0)
	seedGraveyard(gs, 0, spell, a, b)

	dup := []*Card{a, a, b}
	_, err := CastWithEscape(gs, 0, spell, 1, dup)
	if err == nil {
		t.Fatal("CastWithEscape must reject duplicate exile targets")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "duplicate_exile_target" {
		t.Fatalf("expected CastError duplicate_exile_target, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// (c) The N exile-fodder cards are actually exiled
// ---------------------------------------------------------------------------

func TestCastWithEscape_FodderActuallyExiled(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	fodder := []*Card{
		newFodderCard("Filler A", 0),
		newFodderCard("Filler B", 0),
		newFodderCard("Filler C", 0),
	}
	// Add an extra fodder card to confirm only N=3 are taken.
	extra := newFodderCard("Extra Filler", 0)
	seedGraveyard(gs, 0, spell)
	seedGraveyard(gs, 0, fodder...)
	seedGraveyard(gs, 0, extra)

	result, err := CastWithEscape(gs, 0, spell, 1, append(fodder, extra))
	if err != nil {
		t.Fatalf("CastWithEscape failed: %v", err)
	}
	// Exactly 3 fodder went to exile (the required count).
	if len(result.ExiledCards) != 3 {
		t.Fatalf("ExiledCards count = %d, want 3", len(result.ExiledCards))
	}
	for _, want := range fodder {
		foundInExile := false
		for _, c := range gs.Seats[0].Exile {
			if c == want {
				foundInExile = true
				break
			}
		}
		if !foundInExile {
			t.Fatalf("fodder %q should be in exile", want.DisplayName())
		}
		for _, c := range gs.Seats[0].Graveyard {
			if c == want {
				t.Fatalf("fodder %q should NOT still be in graveyard", want.DisplayName())
			}
		}
	}
	// Extra fodder beyond the required count stays in graveyard.
	foundExtraInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == extra {
			foundExtraInExile = true
			break
		}
	}
	if foundExtraInExile {
		t.Fatal("extra fodder beyond required count should NOT be exiled")
	}
	foundExtraInGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == extra {
			foundExtraInGrave = true
			break
		}
	}
	if !foundExtraInGrave {
		t.Fatal("extra fodder should remain in graveyard")
	}
}

// ---------------------------------------------------------------------------
// (d) CostMeta stamped correctly
// ---------------------------------------------------------------------------

func TestCastWithEscape_StampsCostMeta(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	fodder := []*Card{
		newFodderCard("Filler A", 0),
		newFodderCard("Filler B", 0),
		newFodderCard("Filler C", 0),
	}
	seedGraveyard(gs, 0, spell)
	seedGraveyard(gs, 0, fodder...)

	if _, err := CastWithEscape(gs, 0, spell, 1, fodder); err != nil {
		t.Fatalf("CastWithEscape failed: %v", err)
	}
	item := gs.Stack[0]
	if item.Card != spell {
		t.Fatal("stack item should reference the escape-cast spell")
	}
	if item.CastZone != ZoneGraveyard {
		t.Fatalf("CastZone = %q, want %q", item.CastZone, ZoneGraveyard)
	}
	if v, ok := item.CostMeta["escape_cast"]; !ok || v != true {
		t.Fatalf("CostMeta[\"escape_cast\"] = %v, want true", item.CostMeta["escape_cast"])
	}
	if v, ok := item.CostMeta["escape_exile_count"]; !ok || v != 3 {
		t.Fatalf("CostMeta[\"escape_exile_count\"] = %v, want 3", item.CostMeta["escape_exile_count"])
	}
	if v, ok := item.CostMeta["zone_cast_keyword"]; !ok || v != "escape" {
		t.Fatalf("CostMeta[\"zone_cast_keyword\"] = %v, want \"escape\"", item.CostMeta["zone_cast_keyword"])
	}
	if !ShouldExileOnResolve(item) {
		t.Fatal("escape cast must stamp exile_on_resolve=true (§702.148b)")
	}
	if !IsEscapeCast(item) {
		t.Fatal("IsEscapeCast should return true for an escape-cast stack item")
	}
	if got := EscapeExileCountOfItem(item); got != 3 {
		t.Fatalf("EscapeExileCountOfItem = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// (e) On resolve the spell itself goes to exile, not graveyard
// ---------------------------------------------------------------------------

func TestEscape_ResolveExilesSpellNotToGraveyard(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	fodder := []*Card{
		newFodderCard("Filler A", 0),
		newFodderCard("Filler B", 0),
		newFodderCard("Filler C", 0),
	}
	seedGraveyard(gs, 0, spell)
	seedGraveyard(gs, 0, fodder...)

	if _, err := CastWithEscape(gs, 0, spell, 1, fodder); err != nil {
		t.Fatalf("CastWithEscape failed: %v", err)
	}

	ResolveStackTop(gs)

	// Spell must NOT be in graveyard (§702.148b replaces the default
	// §608.2g graveyard destination with exile).
	for _, c := range gs.Seats[0].Graveyard {
		if c == spell {
			t.Fatal("escape-cast spell must go to exile on resolution, not graveyard (§702.148b)")
		}
	}
	// Spell IS in exile.
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == spell {
			foundInExile = true
			break
		}
	}
	if !foundInExile {
		t.Fatal("escape-cast spell should be in owner's exile after resolution")
	}
}

// ---------------------------------------------------------------------------
// Failure-path safety
// ---------------------------------------------------------------------------

func TestCastWithEscape_NoKeyword(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	plain := &Card{
		Name:  "Plain Sorcery",
		Owner: 0,
		Types: []string{"sorcery"},
		AST:   &gameast.CardAST{Name: "Plain Sorcery"},
	}
	seedGraveyard(gs, 0, plain)
	_, err := CastWithEscape(gs, 0, plain, 1, nil)
	if err == nil {
		t.Fatal("CastWithEscape should fail for a card without the escape keyword")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "no_escape_keyword" {
		t.Fatalf("expected CastError no_escape_keyword, got %v", err)
	}
}

func TestCastWithEscape_SpellNotInGraveyard(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	// Spell NOT seeded into graveyard.
	fodder := []*Card{
		newFodderCard("Filler A", 0),
		newFodderCard("Filler B", 0),
		newFodderCard("Filler C", 0),
	}
	seedGraveyard(gs, 0, fodder...)
	_, err := CastWithEscape(gs, 0, spell, 1, fodder)
	if err == nil {
		t.Fatal("CastWithEscape should fail when the spell isn't in seat's graveyard")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "not_in_graveyard" {
		t.Fatalf("expected CastError not_in_graveyard, got %v", err)
	}
}

func TestCastWithEscape_FodderInWrongZone(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	// Two fodder in graveyard, one (badFodder) sitting in hand.
	a := newFodderCard("Filler A", 0)
	b := newFodderCard("Filler B", 0)
	badFodder := newFodderCard("Hand Card", 0)
	seedGraveyard(gs, 0, spell, a, b)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, badFodder)

	_, err := CastWithEscape(gs, 0, spell, 1, []*Card{a, b, badFodder})
	if err == nil {
		t.Fatal("CastWithEscape should reject a fodder card that isn't in seat's graveyard")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "exile_target_not_in_graveyard" {
		t.Fatalf("expected CastError exile_target_not_in_graveyard, got %v", err)
	}
}

func TestCastWithEscape_InsufficientMana(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 0
	spell := newEscapeCard("Underworld Breach", 0, 2, "{R}", 3)
	fodder := []*Card{
		newFodderCard("Filler A", 0),
		newFodderCard("Filler B", 0),
		newFodderCard("Filler C", 0),
	}
	seedGraveyard(gs, 0, spell)
	seedGraveyard(gs, 0, fodder...)
	_, err := CastWithEscape(gs, 0, spell, 1, fodder)
	if err == nil {
		t.Fatal("CastWithEscape should fail with insufficient mana")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "insufficient_mana" {
		t.Fatalf("expected CastError insufficient_mana, got %v", err)
	}
	// State preserved: nothing exiled, mana 0.
	if len(gs.Seats[0].Exile) != 0 {
		t.Fatalf("nothing should be exiled on failure, got %d", len(gs.Seats[0].Exile))
	}
}

func TestCastWithEscape_NegativeManaCostUsesPrinted(t *testing.T) {
	gs := newEscapeGame(t)
	gs.Seats[0].ManaPool = 5
	spell := newEscapeCard("Kroxa, Titan of Death's Hunger", 0, 2, "{1}{B}{R}", 5)
	fodder := []*Card{
		newFodderCard("F1", 0), newFodderCard("F2", 0), newFodderCard("F3", 0),
		newFodderCard("F4", 0), newFodderCard("F5", 0),
	}
	seedGraveyard(gs, 0, spell)
	seedGraveyard(gs, 0, fodder...)

	if _, err := CastWithEscape(gs, 0, spell, -1, fodder); err != nil {
		t.Fatalf("CastWithEscape with -1 sentinel failed: %v", err)
	}
	// Printed EscapeCost is CMC of {1}{B}{R} = 3; mana should drop from 5 to 2.
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("mana left = %d, want 2 (paid EscapeCost=3 of 5)", gs.Seats[0].ManaPool)
	}
}
