package per_card

// r43 — second half of the Krark / Decorum-Dissertation zone-conservation
// fix. The first half (stack.go IsCopy flag) ensures paradigm-cast copies
// cease to exist on resolution. This file pins the second half:
// paradigmExileItem must skip the gs.ParadigmExile registration when the
// stack item is a copy. Without that, every paradigm tick re-registered
// the copy's *Card pointer — accumulating dangling refs that
// checkZoneConservation counts as real cards (11 by turn 47 in Loki r41
// game 181, seed 41).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestParadigmExileItem_SkipsCopies(t *testing.T) {
	gs := newGame(t, 2)
	original := &gameengine.Card{Name: "Decorum Dissertation", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Exile = []*gameengine.Card{original}
	gs.ParadigmExile = map[int][]*gameengine.Card{0: {original}}

	// First resolution: original is treated normally. After this, the
	// engine has one entry in ParadigmExile and the card sits in exile.
	originalItem := &gameengine.StackItem{Controller: 0, Card: original}
	paradigmExileItem(gs, originalItem, 0, "decorum_dissertation", "Decorum Dissertation")
	if got := len(gs.ParadigmExile[0]); got != 2 {
		// The fixture already had one entry, so the original cast pushes a second.
		t.Fatalf("after one original cast expected 2 paradigm-exile entries (fixture+register), got %d", got)
	}

	// Now resolve 10 copy items. Each tick should be a no-op for the
	// ParadigmExile registry — copies cease on resolution per CR §707.10.
	startLen := len(gs.ParadigmExile[0])
	for i := 0; i < 10; i++ {
		copyCard := original.DeepCopy()
		copyCard.IsCopy = true
		copyItem := &gameengine.StackItem{
			Controller: 0,
			Card:       copyCard,
			IsCopy:     true,
		}
		paradigmExileItem(gs, copyItem, 0, "decorum_dissertation", "Decorum Dissertation")
	}
	if got := len(gs.ParadigmExile[0]); got != startLen {
		t.Fatalf("paradigm-exile leaked on copies: %d → %d after 10 copy ticks (Δ=%d, expected 0)",
			startLen, got, got-startLen)
	}
}

// TestParadigmExileItem_CopySkipsExileOnResolve — copies must also NOT
// flip the exile_on_resolve CostMeta. With the IsCopy short-circuit in
// place this is incidental, but pin it so a future refactor doesn't
// reintroduce the side-effect.
func TestParadigmExileItem_CopySkipsExileOnResolve(t *testing.T) {
	gs := newGame(t, 2)
	copyItem := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Decorum Dissertation", Owner: 0, IsCopy: true},
		IsCopy:     true,
	}
	paradigmExileItem(gs, copyItem, 0, "decorum_dissertation", "Decorum Dissertation")
	if copyItem.CostMeta != nil {
		if v, ok := copyItem.CostMeta["exile_on_resolve"]; ok {
			t.Fatalf("copy should not have exile_on_resolve set, got %v", v)
		}
	}
}
