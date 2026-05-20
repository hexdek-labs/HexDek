package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Regression for fix #1 in dev/half-finished-r48:
// stack_trace.go documents "trigger_resolve" as a distinct event kind
// (CR §608.2 for triggers), but ResolveStackTop used to emit "resolve"
// for everything. Audit consumers that count triggers separately from
// spells had no signal to work with. After the fix, an item with
// Kind=="triggered" (or with Source!=nil and no Card) emits
// trigger_resolve; pure spells still emit resolve.
func TestHalfFinishedR48_TriggerResolveEvent(t *testing.T) {
	GlobalStackTrace.Enable()
	defer GlobalStackTrace.Disable()
	GlobalStackTrace.Reset()

	gs := &GameState{
		Seats: []*Seat{
			{Idx: 0, Life: 40, Battlefield: []*Permanent{}},
			{Idx: 1, Life: 40, Battlefield: []*Permanent{}},
		},
		Flags: map[string]int{},
	}
	gs.Active = 0
	gs.Phase = "precombat_main"

	// Build a permanent as the trigger source so we have a non-nil
	// Source on the StackItem (the "real trigger" shape).
	srcCard := &Card{Name: "Trigger Source", Types: []string{"creature"}, Owner: 0}
	srcPerm := &Permanent{
		Card: srcCard, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, srcPerm)

	// Triggered ability — Source != nil, Kind == "triggered".
	gs.Stack = append(gs.Stack, &StackItem{
		Card:       srcCard,
		Source:     srcPerm,
		Controller: 0,
		Kind:       "triggered",
		Effect:     &gameast.Draw{Count: *gameast.NumInt(1)},
	})
	ResolveStackTop(gs)

	// Plain spell — Source == nil.
	spellCard := &Card{Name: "Plain Spell", Types: []string{"sorcery"}, Owner: 0}
	gs.Stack = append(gs.Stack, &StackItem{
		Card:       spellCard,
		Controller: 0,
		Effect:     &gameast.Draw{Count: *gameast.NumInt(1)},
	})
	ResolveStackTop(gs)

	sawTriggerResolve := 0
	sawResolve := 0
	for _, e := range GlobalStackTrace.Entries {
		switch e.Action {
		case "trigger_resolve":
			sawTriggerResolve++
		case "resolve":
			sawResolve++
		}
	}
	if sawTriggerResolve != 1 {
		t.Errorf("expected exactly 1 trigger_resolve, got %d (entries: %+v)", sawTriggerResolve, GlobalStackTrace.Entries)
	}
	if sawResolve != 1 {
		t.Errorf("expected exactly 1 resolve, got %d (entries: %+v)", sawResolve, GlobalStackTrace.Entries)
	}
}
