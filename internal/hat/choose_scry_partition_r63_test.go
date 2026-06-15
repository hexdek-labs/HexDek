package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// choose_scry_partition_r63_test.go — regression for the r63 within-zone
// CardIdentity dup producer. The YggdrasilHat top-empty fallback in ChooseScry
// / ChooseSurveil forced cards[0] onto top but removed a DIFFERENT (last)
// element from the other pile, leaving cards[0] in BOTH halves. Scry/Surveil
// then double-inserted it into the library. These assert the returned split is
// always a proper partition (no card in both halves, none dropped).

func splitIsPartition(t *testing.T, looked, a, b []*gameengine.Card) {
	t.Helper()
	if len(a)+len(b) != len(looked) {
		t.Fatalf("split count %d+%d != looked %d", len(a), len(b), len(looked))
	}
	count := map[*gameengine.Card]int{}
	for _, c := range a {
		count[c]++
	}
	for _, c := range b {
		count[c]++
	}
	for _, c := range looked {
		if count[c] != 1 {
			name := "<nil>"
			if c != nil {
				name = c.DisplayName()
			}
			t.Errorf("looked card %q appears %d times across the split (want exactly 1)", name, count[c])
		}
	}
}

// ChooseScry must return a proper partition even when every card scores below
// threshold (the top-empty fallback path that previously duplicated cards[0]).
func TestYggdrasilChooseScry_FallbackIsPartition(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeControl}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	// Two low-value creatures Control would bottom → top-empty fallback fires.
	a := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	b := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)
	looked := []*gameengine.Card{a, b}

	top, bottom := h.ChooseScry(gs, 0, looked)
	splitIsPartition(t, looked, top, bottom)
	// The forced-on-top card (a) must NOT also be in bottom.
	if scryContains(top, a) && scryContains(bottom, a) {
		t.Error("cards[0] must not appear in BOTH top and bottom")
	}
}

// ChooseSurveil's analogous fallback must also be a proper partition.
func TestYggdrasilChooseSurveil_FallbackIsPartition(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeControl}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	a := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	b := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)
	looked := []*gameengine.Card{a, b}

	graveyard, top := h.ChooseSurveil(gs, 0, looked)
	splitIsPartition(t, looked, graveyard, top)
	if scryContains(top, a) && scryContains(graveyard, a) {
		t.Error("cards[0] must not appear in BOTH top and graveyard")
	}
}
