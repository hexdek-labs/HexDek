package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 — ChooseResponse must only offer a counterspell that can LEGALLY target
// the incoming item. Before r63 the counter loop checked only "have a counter
// + can pay", so the hat returned a counter against triggered/activated
// abilities (which counterspells can't target). The engine then rejected it
// (counter_filter_mismatch) AND — because bestCounter was non-nil — the hat
// skipped its reactive-removal fallback, so it interacted with NOTHING. The
// r63 A/B measured 106 of 125 chosen responses dying exactly here.

// A counter must NOT be offered against a triggered ability on the stack.
func TestChooseResponse_R63_DoesNotCounterTriggeredAbility(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeControl}, 0, 0)

	// Triggered ability on the stack: Card == nil, Source set, opponent-
	// controlled — a counterspell ("counter target spell") cannot target it.
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Value Engine", Types: []string{"creature"}},
		Controller: 1, Owner: 1, Flags: map[string]int{},
	}
	top := &gameengine.StackItem{Controller: 1, Source: src, Kind: "triggered"}

	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("hat must NOT offer a counterspell against a triggered ability; got %v",
			got.Card.DisplayName())
	}
}

// Positive control: against a counterable, high-threat spell the gate lets the
// counter through (so the fix didn't just disable countering).
func TestChooseResponse_R63_StillCountersThreateningSpell(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeControl}, 0, 0)

	// "Take an extra turn" → mustCounter, and it's a real spell the counter
	// can target.
	threat := spellWithOracle("Time Warp", 5, "Take an extra turn after this one.")
	top := &gameengine.StackItem{
		Controller: 1,
		Card:       threat,
		Effect:     &gameast.ExtraTurn{},
	}
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("hat must still counter a counterable mustCounter spell (Time Warp)")
	}
}
