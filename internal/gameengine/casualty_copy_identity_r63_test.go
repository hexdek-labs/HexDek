package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// casualty_copy_identity_r63_test.go — r63 mechanic-probe (CR §702.153c).
// Pins property (3): the casualty copy is a REAL copy created through the
// canonical MintSpellCopy chokepoint — a DISTINCT *Card with its own
// InstanceID, never a pointer alias of the source card. This is the same
// CardIdentity discipline as the cross-seat aliasing fix (3bf7edaa): a
// hand-rolled `copy.Card = item.Card` would put one *Card on the stack twice
// and trip CardIdentity / leak at §707.10 ceasing. The existing casualty
// suite covers the cost gate, power check, token resolution, and copy
// triggers; this fills the not-yet-pinned non-aliasing property.

func TestCasualty_CopyIsDistinctCardNotAlias(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].ManaPool = 4
	fodder := addKWCombatBattlefield(gs, 0, "Fodder", 2, 2, "creature")
	_ = fodder

	// Original casualty spell on the stack with a real (OG) InstanceID.
	orig := instantSpellCard("Make Disappear", 2, nil,
		&gameast.Keyword{Name: "casualty", Args: []interface{}{float64(1)}})
	MintOGInstanceID(gs, orig)
	if orig.InstanceID == "" {
		t.Fatal("fixture: original spell must have an InstanceID")
	}
	item := &StackItem{Card: orig, Controller: 0, Kind: "spell"}
	PushStackItem(gs, item)

	stackBefore := len(gs.Stack)

	// Pay casualty 1 (sacrifice the power-2 fodder) and copy.
	if !ApplyCasualty(gs, 0, item, 1) {
		t.Fatal("ApplyCasualty should pay and copy with a legal power-1+ sacrifice")
	}

	// A copy was pushed above the original.
	if len(gs.Stack) != stackBefore+1 {
		t.Fatalf("casualty must push exactly one copy: stack %d -> %d", stackBefore, len(gs.Stack))
	}

	// Find the copy stack item.
	var copyItem *StackItem
	for _, it := range gs.Stack {
		if it != nil && it.IsCopy {
			copyItem = it
			break
		}
	}
	if copyItem == nil {
		t.Fatal("casualty copy StackItem (IsCopy) not found on the stack")
	}

	// (3) Non-aliasing discipline.
	if copyItem.Card == orig {
		t.Fatal("CardIdentity: casualty copy aliases the source *Card pointer — must be a distinct DeepCopy")
	}
	if copyItem.Card.InstanceID == "" {
		t.Error("casualty copy must carry its own minted InstanceID")
	}
	if copyItem.Card.InstanceID == orig.InstanceID {
		t.Error("casualty copy must NOT share the source's InstanceID (CardIdentity / §707.10)")
	}
	if !copyItem.Card.IsCopy {
		t.Error("casualty copy *Card must be flagged IsCopy so §707.10 ceasing retires it (no graveyard dup)")
	}
	// Copiable values carried (it's a faithful copy, not a vanilla stub).
	if copyItem.Card.Name != orig.Name {
		t.Errorf("casualty copy must copy the name: got %q want %q", copyItem.Card.Name, orig.Name)
	}
	// The copy is controlled by the caster and carries the casualty_copy mark.
	if copyItem.Controller != 0 {
		t.Errorf("casualty copy controller: got %d want 0", copyItem.Controller)
	}
}
