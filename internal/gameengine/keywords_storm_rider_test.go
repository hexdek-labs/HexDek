package gameengine

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Test helpers — Storm rider (CR §702.40)
// ---------------------------------------------------------------------------

func newStormGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(401))
	return NewGameState(2, rng, nil)
}

// stormSpellCard builds an instant carrying the storm keyword on its
// AST plus a placeholder Static so HasStorm's oracle-text fallback
// path can be exercised when needed (passing a custom raw line).
func stormSpellCard(name string) *Card {
	return &Card{
		Name:           name,
		Owner:          0,
		CMC:            1,
		ManaCostString: "{R}",
		Types:          []string{"instant"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "storm"},
			},
		},
	}
}

// stormStackItem builds a StackItem suitable for ApplyStormCopy /
// ApplyStormCopies tests, controlled by `seat` and using `card` as the
// underlying spell card.
func stormStackItem(card *Card, seat int) *StackItem {
	return &StackItem{
		Kind:       "spell",
		Card:       card,
		Controller: seat,
		CastZone:   ZoneHand,
	}
}

// ===========================================================================
// HasStorm — detector
// ===========================================================================

func TestHasStorm_DetectsKeyword(t *testing.T) {
	if !HasStorm(stormSpellCard("Bolt Storm")) {
		t.Error("HasStorm should detect AST keyword 'storm'")
	}
}

func TestHasStorm_DetectsByName(t *testing.T) {
	// Grapeshot is in the storm name catalog; even without AST keyword
	// the HasStormKeyword fallback should match.
	c := &Card{Name: "Grapeshot"}
	if !HasStorm(c) {
		t.Error("HasStorm should detect via the storm name catalog (Grapeshot)")
	}
}

func TestHasStorm_DetectsOracleTextReminder(t *testing.T) {
	c := &Card{
		Name: "Grapevine",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Storm — Copy it for each spell cast before it this turn."},
			},
		},
	}
	if !HasStorm(c) {
		t.Error("HasStorm should detect 'storm —' oracle prefix")
	}
	// ASCII hyphen variant.
	c2 := &Card{
		Name: "OldDump",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Storm - copy this spell..."},
			},
		},
	}
	if !HasStorm(c2) {
		t.Error("HasStorm should detect 'storm -' (ASCII hyphen) form")
	}
}

func TestHasStorm_DetectsStormCountPhrasing(t *testing.T) {
	c := &Card{
		Name: "Tendrils Variant",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Target opponent loses 2 life times the storm count."},
			},
		},
	}
	if !HasStorm(c) {
		t.Error("HasStorm should detect 'storm count' payload phrasing")
	}
}

func TestHasStorm_DetectsCopyForEachOtherSpell(t *testing.T) {
	c := &Card{
		Name: "Generic Storm",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "When you cast this, copy it for each other spell cast before it this turn."},
			},
		},
	}
	if !HasStorm(c) {
		t.Error("HasStorm should detect §702.40a 'copy it for each other spell cast' phrasing")
	}
}

func TestHasStorm_NegativeCases(t *testing.T) {
	if HasStorm(nil) {
		t.Error("HasStorm(nil) should be false")
	}
	if HasStorm(&Card{AST: &gameast.CardAST{}}) {
		t.Error("HasStorm on empty AST should be false")
	}
	// Card whose flavor text mentions "storm" but is NOT a storm
	// card — neither keyword, nor reminder prefix, nor §702.40a phrase.
	c := &Card{
		Name: "Flavor",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "A great storm darkens the sky."},
			},
		},
	}
	if HasStorm(c) {
		t.Error("HasStorm should NOT match incidental flavor uses of the word 'storm'")
	}
}

// ===========================================================================
// StormCount — per-seat prior-spell count
// ===========================================================================

func TestStormCount_ZeroAtTurnStart(t *testing.T) {
	gs := newStormGame(t)
	if got := StormCount(gs, 0); got != 0 {
		t.Errorf("StormCount at turn start: want 0, got %d", got)
	}
}

func TestStormCount_DerivedFromSpellsCastMinusSelf(t *testing.T) {
	gs := newStormGame(t)
	// 4 spells cast this turn (the GLOBAL count, including the storm spell
	// once it's pushed). StormCount returns "other spells" = 4 - 1 = 3.
	gs.SpellsCastThisTurn = 4
	if got := StormCount(gs, 0); got != 3 {
		t.Errorf("StormCount at SpellsCastThisTurn=4: want 3, got %d", got)
	}
}

func TestStormCount_NeverNegative(t *testing.T) {
	gs := newStormGame(t)
	gs.SpellsCastThisTurn = 0
	if got := StormCount(gs, 0); got != 0 {
		t.Errorf("StormCount(SpellsCastThisTurn=0): want 0, got %d", got)
	}
}

// CR §702.40a — storm counts spells cast by ALL players. A storm spell cast by
// EITHER seat sees the same prior-spell count (it is NOT per-seat).
func TestStormCount_CountsAllPlayers(t *testing.T) {
	gs := newStormGame(t)
	// 5 spells by seat 0 + 1 by seat 1 = 6 global; the storm spell is one of
	// them, so "other spells" = 5 for whichever seat casts it.
	gs.SpellsCastThisTurn = 6
	if got := StormCount(gs, 0); got != 5 {
		t.Errorf("seat 0 storm count must include opponents' spells: want 5, got %d", got)
	}
	if got := StormCount(gs, 1); got != 5 {
		t.Errorf("storm count is all-players, identical for any caster: want 5, got %d", got)
	}
}

func TestStormCount_ResetsAtTurnEnd(t *testing.T) {
	gs := newStormGame(t)
	gs.SpellsCastThisTurn = 7
	if got := StormCount(gs, 0); got != 6 {
		t.Fatalf("setup: want 6 prior spells, got %d", got)
	}
	// Mirror UntapAll's per-turn cleanup (turn.go zeroes the global counter).
	gs.SpellsCastThisTurn = 0
	if got := StormCount(gs, 0); got != 0 {
		t.Errorf("StormCount after turn reset: want 0, got %d", got)
	}
}

func TestStormCount_NilOrInvalid(t *testing.T) {
	if StormCount(nil, 0) != 0 {
		t.Error("StormCount(nil, ...) should be 0")
	}
	gs := newStormGame(t)
	if StormCount(gs, -1) != 0 || StormCount(gs, 99) != 0 {
		t.Error("StormCount with invalid seat should be 0")
	}
}

// ===========================================================================
// ApplyStormCopy — primitive count-based fan-out
// ===========================================================================

func TestApplyStormCopy_ZeroCountNoCopies(t *testing.T) {
	gs := newStormGame(t)
	original := stormStackItem(stormSpellCard("Grapeshot"), 0)
	gs.Stack = append(gs.Stack, original)
	stackBefore := len(gs.Stack)

	copies := ApplyStormCopy(gs, original, 0)

	if copies != 0 {
		t.Errorf("count=0: want 0 copies, got %d", copies)
	}
	if len(gs.Stack) != stackBefore {
		t.Errorf("stack should be unchanged when count=0; before=%d after=%d",
			stackBefore, len(gs.Stack))
	}
	// No storm_trigger event for count=0.
	for _, ev := range gs.EventLog {
		if ev.Kind == "storm_trigger" {
			t.Fatalf("storm_trigger must not fire for count=0; got %+v", ev)
		}
	}
}

func TestApplyStormCopy_PushesExactCount(t *testing.T) {
	gs := newStormGame(t)
	original := stormStackItem(stormSpellCard("Grapeshot"), 0)
	gs.Stack = append(gs.Stack, original)

	copies := ApplyStormCopy(gs, original, 3)

	if copies != 3 {
		t.Errorf("count=3: want 3, got %d", copies)
	}
	// Stack should contain: original + 3 copies = 4 items.
	if len(gs.Stack) != 4 {
		t.Fatalf("expected 4 stack items (original + 3 copies), got %d", len(gs.Stack))
	}
	// The 3 newest items should be copies (IsCopy=true), each with a
	// distinct fresh Card.
	for i := 1; i <= 3; i++ {
		item := gs.Stack[len(gs.Stack)-i]
		if !item.IsCopy {
			t.Errorf("stack item %d should have IsCopy=true", len(gs.Stack)-i)
		}
		if item.Card == nil || !item.Card.IsCopy {
			t.Errorf("stack item %d's Card should be IsCopy=true", len(gs.Stack)-i)
		}
		if item.Card == original.Card {
			t.Errorf("storm copy %d aliased original card pointer", i)
		}
		// CMC=0 (free to resolve).
		if item.Card.CMC != 0 {
			t.Errorf("storm copy %d CMC: want 0, got %d", i, item.Card.CMC)
		}
		// Name suffix.
		if !strings.Contains(item.Card.Name, "(storm copy") {
			t.Errorf("storm copy %d name should have suffix; got %q", i, item.Card.Name)
		}
	}
	// One storm_trigger event with amount=3.
	saw := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "storm_trigger" && ev.Amount == 3 {
			if rule, _ := ev.Details["rule"].(string); rule == "702.40a" {
				saw = true
				break
			}
		}
	}
	if !saw {
		t.Error("expected a storm_trigger event with amount=3, rule=702.40a")
	}
}

func TestApplyStormCopy_NilOrEmptyOriginal(t *testing.T) {
	gs := newStormGame(t)
	if ApplyStormCopy(nil, nil, 3) != 0 {
		t.Error("ApplyStormCopy(nil, nil, 3) should be 0")
	}
	if ApplyStormCopy(gs, nil, 3) != 0 {
		t.Error("ApplyStormCopy(gs, nil, ...) should be 0")
	}
	if ApplyStormCopy(gs, &StackItem{}, 3) != 0 {
		t.Error("ApplyStormCopy with nil-Card item should be 0")
	}
}

func TestApplyStormCopy_NegativeCountNoOp(t *testing.T) {
	gs := newStormGame(t)
	original := stormStackItem(stormSpellCard("Grapeshot"), 0)
	gs.Stack = append(gs.Stack, original)
	if ApplyStormCopy(gs, original, -2) != 0 {
		t.Error("negative count should be a no-op")
	}
	if len(gs.Stack) != 1 {
		t.Errorf("stack should be unchanged; got %d items", len(gs.Stack))
	}
}

// ===========================================================================
// End-to-end cast-time flow via the existing ApplyStormCopies wrapper
// (which now delegates to ApplyStormCopy)
// ===========================================================================

func TestApplyStormCopies_ZeroPriorSpells_OneTotal(t *testing.T) {
	// (a) Cast storm spell with 0 prior spells. Storm spell itself
	// is the first cast — SpellsCastThisTurn=1 — so storm copies = 0,
	// total stack instances of the spell = 1 (the original).
	gs := newStormGame(t)
	gs.SpellsCastThisTurn = 1 // just this storm spell, no priors
	original := stormStackItem(stormSpellCard("Grapeshot"), 0)
	gs.Stack = append(gs.Stack, original)

	copies := ApplyStormCopies(gs, original, 0)

	if copies != 0 {
		t.Errorf("0 prior spells: want 0 copies, got %d", copies)
	}
	if len(gs.Stack) != 1 {
		t.Errorf("total spell instances on stack: want 1 (original only), got %d", len(gs.Stack))
	}
}

func TestApplyStormCopies_ThreePriorSpells_FourTotal(t *testing.T) {
	// (b) 3 prior spells → 4 total (1 original + 3 copies).
	gs := newStormGame(t)
	gs.SpellsCastThisTurn = 4 // 3 prior + the storm spell itself
	original := stormStackItem(stormSpellCard("Grapeshot"), 0)
	gs.Stack = append(gs.Stack, original)

	copies := ApplyStormCopies(gs, original, 0)

	if copies != 3 {
		t.Errorf("3 prior spells: want 3 copies, got %d", copies)
	}
	if len(gs.Stack) != 4 {
		t.Errorf("total spell instances on stack: want 4 (1 original + 3 copies), got %d", len(gs.Stack))
	}
	// All 3 newest items are real StackItem clones, distinct from the
	// original and from each other.
	seen := map[*StackItem]bool{}
	for i := 1; i <= 3; i++ {
		item := gs.Stack[len(gs.Stack)-i]
		if item == original {
			t.Errorf("storm copy %d aliased the original stack item", i)
		}
		if seen[item] {
			t.Errorf("storm copies %d duplicated each other (same pointer)", i)
		}
		seen[item] = true
	}
}

// ===========================================================================
// (c) Storm count resets at turn end via the per-seat flow
// ===========================================================================

func TestApplyStormCopy_AfterTurnResetProducesZeroCopies(t *testing.T) {
	gs := newStormGame(t)
	gs.SpellsCastThisTurn = 5 // 4 priors + storm self
	if StormCount(gs, 0) != 4 {
		t.Fatalf("setup: want 4 priors, got %d", StormCount(gs, 0))
	}
	// Reset (mirrors UntapAll's per-turn cleanup).
	gs.SpellsCastThisTurn = 0
	if StormCount(gs, 0) != 0 {
		t.Fatalf("StormCount should be 0 after turn reset")
	}
	// Cast a storm spell post-reset — it's the first this turn.
	gs.SpellsCastThisTurn = 1
	original := stormStackItem(stormSpellCard("Grapeshot"), 0)
	gs.Stack = append(gs.Stack, original)
	count := StormCount(gs, 0)
	copies := ApplyStormCopy(gs, original, count)
	if copies != 0 {
		t.Errorf("after turn reset, first storm spell should produce 0 copies; got %d", copies)
	}
}

// ===========================================================================
// (e) Copies are real StackItem clones with IsCopy semantics
// ===========================================================================

func TestApplyStormCopy_CopiesAreRealCloneStackItems(t *testing.T) {
	gs := newStormGame(t)
	original := stormStackItem(stormSpellCard("Tendrils"), 0)
	original.Targets = []Target{{Kind: TargetKindSeat, Seat: 1}}
	gs.Stack = append(gs.Stack, original)

	ApplyStormCopy(gs, original, 2)

	for i := 1; i <= 2; i++ {
		copyItem := gs.Stack[len(gs.Stack)-i]
		// (e1) distinct from original
		if copyItem == original {
			t.Fatalf("copy %d aliased original pointer", i)
		}
		// (e2) IsCopy=true at both StackItem and Card layers
		if !copyItem.IsCopy {
			t.Errorf("copy %d StackItem.IsCopy should be true", i)
		}
		if !copyItem.Card.IsCopy {
			t.Errorf("copy %d Card.IsCopy should be true", i)
		}
		// (e3) Controller mirrored
		if copyItem.Controller != original.Controller {
			t.Errorf("copy %d Controller: want %d, got %d", i, original.Controller, copyItem.Controller)
		}
		// (e4) Targets carried over as a fresh slice (not aliased)
		if len(copyItem.Targets) != 1 || copyItem.Targets[0].Kind != TargetKindSeat || copyItem.Targets[0].Seat != 1 {
			t.Errorf("copy %d should inherit Targets", i)
		}
		// Mutating the copy's targets must not bleed back into original.
		copyItem.Targets[0].Seat = 99
		if original.Targets[0].Seat != 1 {
			t.Fatalf("copy %d Targets aliased original (mutation leaked)", i)
		}
		copyItem.Targets[0].Seat = 1 // restore
		// (e5) Effect pointer shared (effects are immutable)
		if copyItem.Effect != original.Effect {
			t.Errorf("copy %d should share Effect pointer with original", i)
		}
		// (e6) Distinct stack IDs
		if copyItem.ID == original.ID {
			t.Errorf("copy %d shares stack ID with original", i)
		}
	}
}
