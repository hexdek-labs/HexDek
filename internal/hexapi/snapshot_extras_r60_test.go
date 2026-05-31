package hexapi

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 spectator UX regressions for the two new snapshot helpers:
//   buildManaPoolByColor — flattens ColoredManaPool → map<color,int>
//   buildStackSnapshot   — engine *StackItem slice → display snapshots
//
// Both feed the frontend Spectator UI components shipped in the
// same batch (ManaPoolPips + StackPanel).

// -----------------------------------------------------------------------------
// buildManaPoolByColor
// -----------------------------------------------------------------------------

func TestBuildManaPoolByColor_NilSafe(t *testing.T) {
	if got := buildManaPoolByColor(nil); got != nil {
		t.Errorf("nil pool should yield nil map, got %v", got)
	}
}

func TestBuildManaPoolByColor_EmptyPoolYieldsNil(t *testing.T) {
	p := &gameengine.ColoredManaPool{}
	if got := buildManaPoolByColor(p); got != nil {
		t.Errorf("empty pool should yield nil (omitempty elides field), got %v", got)
	}
}

func TestBuildManaPoolByColor_OnlyNonZeroBucketsAppear(t *testing.T) {
	p := &gameengine.ColoredManaPool{W: 2, U: 0, B: 0, R: 1, G: 0, C: 3, Any: 0}
	got := buildManaPoolByColor(p)
	if got["W"] != 2 || got["R"] != 1 || got["C"] != 3 {
		t.Errorf("expected W=2 R=1 C=3 in map; got %v", got)
	}
	for _, k := range []string{"U", "B", "G", "Any"} {
		if _, ok := got[k]; ok {
			t.Errorf("zero bucket %q should be absent from map; got %v", k, got)
		}
	}
}

func TestBuildManaPoolByColor_RestrictedRollsIntoAny(t *testing.T) {
	p := &gameengine.ColoredManaPool{
		Any: 1,
		Restricted: []gameengine.RestrictedMana{
			{Amount: 2, Color: "U", Restriction: "creature_spell_only", Source: "Cavern of Souls"},
			{Amount: 3, Color: "", Restriction: "noncreature", Source: "Bloodsworn Steward"},
		},
	}
	got := buildManaPoolByColor(p)
	if got["Any"] != 6 {
		t.Errorf("Any bucket should roll up 1+2+3=6 (legacy Any + restricted); got %d (full: %v)", got["Any"], got)
	}
}

func TestBuildManaPoolByColor_AllFiveColors(t *testing.T) {
	p := &gameengine.ColoredManaPool{W: 1, U: 1, B: 1, R: 1, G: 1}
	got := buildManaPoolByColor(p)
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		if got[c] != 1 {
			t.Errorf("expected %q=1 in map; got %v", c, got)
		}
	}
	if len(got) != 5 {
		t.Errorf("expected exactly 5 keys, got %d (%v)", len(got), got)
	}
}

// -----------------------------------------------------------------------------
// buildStackSnapshot
// -----------------------------------------------------------------------------

func TestBuildStackSnapshot_EmptyYieldsNil(t *testing.T) {
	if got := buildStackSnapshot(nil); got != nil {
		t.Errorf("nil stack should yield nil, got %v", got)
	}
	if got := buildStackSnapshot([]*gameengine.StackItem{}); got != nil {
		t.Errorf("empty stack should yield nil, got %v", got)
	}
}

func TestBuildStackSnapshot_SpellResolvesCardName(t *testing.T) {
	card := &gameengine.Card{Name: "Counterspell"}
	stack := []*gameengine.StackItem{
		{ID: 1, Controller: 2, Card: card, Kind: "spell"},
	}
	got := buildStackSnapshot(stack)
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].Source != "Counterspell" {
		t.Errorf("source = %q, want Counterspell", got[0].Source)
	}
	if got[0].Controller != 2 || got[0].Kind != "spell" {
		t.Errorf("controller/kind wrong: %+v", got[0])
	}
}

func TestBuildStackSnapshot_ActivatedAbilityUsesSourcePermanentName(t *testing.T) {
	srcCard := &gameengine.Card{Name: "Birds of Paradise"}
	source := &gameengine.Permanent{Card: srcCard, Controller: 0}
	stack := []*gameengine.StackItem{
		{ID: 7, Controller: 0, Source: source, Kind: "activated"},
	}
	got := buildStackSnapshot(stack)
	if len(got) != 1 || got[0].Source != "Birds of Paradise" || got[0].Kind != "activated" {
		t.Errorf("expected single activated Birds of Paradise; got %+v", got)
	}
}

func TestBuildStackSnapshot_PreservesCopyAndCounteredFlags(t *testing.T) {
	card := &gameengine.Card{Name: "Mystical Tutor"}
	stack := []*gameengine.StackItem{
		{Controller: 1, Card: card, Kind: "spell", IsCopy: true},
		{Controller: 3, Card: card, Kind: "spell", Countered: true},
	}
	got := buildStackSnapshot(stack)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if !got[0].IsCopy || got[0].Countered {
		t.Errorf("[0] should be IsCopy=true, Countered=false; got %+v", got[0])
	}
	if got[1].IsCopy || !got[1].Countered {
		t.Errorf("[1] should be Countered=true, IsCopy=false; got %+v", got[1])
	}
}

func TestBuildStackSnapshot_NilItemSkipped(t *testing.T) {
	card := &gameengine.Card{Name: "Lightning Bolt"}
	stack := []*gameengine.StackItem{
		nil,
		{Controller: 0, Card: card, Kind: "spell"},
		nil,
	}
	got := buildStackSnapshot(stack)
	if len(got) != 1 {
		t.Errorf("nil items should be skipped; got %d entries: %+v", len(got), got)
	}
}

func TestBuildStackSnapshot_OrderPreserved(t *testing.T) {
	a := &gameengine.Card{Name: "Spell A"}
	b := &gameengine.Card{Name: "Spell B"}
	c := &gameengine.Card{Name: "Spell C"}
	// Engine convention: Stack[0] = bottom, Stack[len-1] = top. Frontend
	// renders top-down; the helper must preserve engine order so the
	// caller can iterate end-to-start for display.
	stack := []*gameengine.StackItem{
		{Card: a, Kind: "spell"},
		{Card: b, Kind: "spell"},
		{Card: c, Kind: "spell"},
	}
	got := buildStackSnapshot(stack)
	if len(got) != 3 || got[0].Source != "Spell A" || got[1].Source != "Spell B" || got[2].Source != "Spell C" {
		t.Errorf("order not preserved; got %+v", got)
	}
}
