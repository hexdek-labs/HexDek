package gameengine

import (
	"math/rand"
	"testing"
)

// keywords_bargain_mirror_r63_test.go — regressions for the r63 bargain
// stale-flag fix. The "when this enters, if it was bargained" ETB riders
// (per_card/bargain_consumers_r60.go) previously read a per-seat cast-time
// counter that leaked across a bargained spell countered before it entered,
// and could cross-attribute between simultaneous casts. CR §702.176c — the
// bargained decision now travels with the permanent via
// MirrorBargainToPermanent (mirroring kicker/squad), so each entering
// permanent carries exactly its own decision.

func TestMirrorBargainToPermanent_StampsWhenBargained(t *testing.T) {
	item := &StackItem{CostMeta: map[string]interface{}{"bargained": true}}
	perm := &Permanent{Flags: map[string]int{}}
	MirrorBargainToPermanent(item, perm)
	if perm.Flags["bargained"] != 1 {
		t.Fatalf("bargained cast should stamp perm.Flags[bargained]=1, got %d", perm.Flags["bargained"])
	}
}

func TestMirrorBargainToPermanent_NoStampWhenNotBargained(t *testing.T) {
	// Explicit false.
	item := &StackItem{CostMeta: map[string]interface{}{"bargained": false}}
	perm := &Permanent{Flags: map[string]int{}}
	MirrorBargainToPermanent(item, perm)
	if _, ok := perm.Flags["bargained"]; ok {
		t.Fatalf("non-bargained cast must leave perm.Flags[bargained] unset")
	}
	// Absent key (the common case: card was never bargained).
	item2 := &StackItem{CostMeta: map[string]interface{}{}}
	perm2 := &Permanent{Flags: map[string]int{}}
	MirrorBargainToPermanent(item2, perm2)
	if _, ok := perm2.Flags["bargained"]; ok {
		t.Fatalf("absent CostMeta[bargained] must leave perm.Flags unset")
	}
}

func TestMirrorBargainToPermanent_NoCostMetaIsNoOp(t *testing.T) {
	// Copies (§707.10f) carry no CostMeta — must be a safe no-op.
	item := &StackItem{}
	perm := &Permanent{Flags: map[string]int{}}
	MirrorBargainToPermanent(item, perm)
	if len(perm.Flags) != 0 {
		t.Fatalf("nil CostMeta must not touch perm.Flags, got %v", perm.Flags)
	}
}

func TestMirrorBargainToPermanent_NilSafe(t *testing.T) {
	// Must not panic on nil inputs.
	MirrorBargainToPermanent(nil, &Permanent{})
	MirrorBargainToPermanent(&StackItem{CostMeta: map[string]interface{}{"bargained": true}}, nil)
	// nil Flags is allocated on demand.
	perm := &Permanent{}
	MirrorBargainToPermanent(&StackItem{CostMeta: map[string]interface{}{"bargained": true}}, perm)
	if perm.Flags["bargained"] != 1 {
		t.Fatalf("nil perm.Flags should be allocated and stamped, got %v", perm.Flags)
	}
}

// End-to-end: ApplyBargain pays the cost + stamps the StackItem's CostMeta,
// and MirrorBargainToPermanent then carries that decision onto a permanent
// minted from the same item — the chain the engine runs at resolution.
func TestApplyBargain_ThenMirror_CarriesFlagToPermanent(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(57)), nil)
	gs.Active = 0
	// A token to satisfy the bargain sacrifice (cheapest eligible candidate).
	addTypePerm(gs, 0, "Treasure Token", "artifact", "token")

	spell := bargainSpellCard("High Fae Negotiator", 0)
	spell.Types = []string{"creature"}
	item := &StackItem{Card: spell, Controller: 0}

	if !ApplyBargain(gs, 0, item) {
		t.Fatal("ApplyBargain should succeed when an eligible sacrifice exists")
	}
	if v, _ := item.CostMeta["bargained"].(bool); !v {
		t.Fatal("ApplyBargain must stamp CostMeta[bargained]=true")
	}

	perm := &Permanent{Card: spell, Controller: 0, Flags: map[string]int{}}
	MirrorBargainToPermanent(item, perm)
	if perm.Flags["bargained"] != 1 {
		t.Fatalf("bargained decision should carry to the entering permanent, got %d", perm.Flags["bargained"])
	}
}
