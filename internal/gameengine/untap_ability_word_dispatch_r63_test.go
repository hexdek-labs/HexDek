package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// untap_ability_word_dispatch_r63_test.go — dead-dispatch follow-up #2
// (UNTAP class). Inspired ("Whenever this creature becomes untapped, …")
// parses to a Static{ability_word "inspired" → Triggered} wrapper. HasInspired
// only detected a bare Keyword node, so FireInspiredTriggers no-op'd — the
// whole inspired family (incl. its 6 per_card handlers) was inert. Fixed:
// detection sees the wrapper, and FireInspiredTriggers resolves the inner
// effect for cards without a per_card handler (guarded against double-fire).

func untapGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(19)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func untapDrain(gs *GameState) {
	for i := 0; i < 80 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

// inspiredPerm builds a tapped creature whose AST is a Static{ability_word
// "inspired" → Triggered{becomes_untapped, GainLife controller+1}} wrapper.
func inspiredPerm(gs *GameState) *Permanent {
	inner := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "becomes_untapped"},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "controller"}},
		Raw:     "inspired — whenever this creature becomes untapped, you gain 1 life",
	}
	c := &Card{Name: "Probe Inspired", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	c.AST = &gameast.CardAST{Name: c.Name, Abilities: []gameast.Ability{
		&gameast.Static{Modification: &gameast.Modification{ModKind: "ability_word", Args: []interface{}{"inspired", "triggered", inner}}},
	}}
	p := &Permanent{Card: c, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	p.Tapped = true
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return p
}

// Detection now sees the ability_word wrapper.
func TestUntapAbilityWord_HasInspiredDetectsWrapper(t *testing.T) {
	gs := untapGame(t)
	p := inspiredPerm(gs)
	if !HasInspiredPerm(p) {
		t.Fatal("HasInspiredPerm must detect the ability_word inspired wrapper")
	}
}

// Inspired fires exactly once when the creature untaps.
func TestUntapAbilityWord_InspiredFiresOnceOnUntap(t *testing.T) {
	gs := untapGame(t)
	p := inspiredPerm(gs)
	life := gs.Seats[0].Life

	if !UntapPermanent(gs, p, "test") {
		t.Fatal("UntapPermanent should report a tapped→untapped transition")
	}
	untapDrain(gs)
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("inspired fired %d times on untap, want exactly 1 (was INERT pre-fix)", got)
	}

	// No fresh transition (already untapped) → no second fire.
	if UntapPermanent(gs, p, "test") {
		t.Fatal("UntapPermanent on an already-untapped permanent must return false")
	}
	untapDrain(gs)
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("inspired re-fired without a tapped→untapped transition (total %d, want 1)", got)
	}
}

// Inspired fires via the untap STEP (UntapAll in phases.go) — the canonical
// "your untap step" surface.
func TestUntapAbilityWord_InspiredFiresOnUntapStep(t *testing.T) {
	gs := untapGame(t)
	gs.Active = 0
	inspiredPerm(gs) // tapped inspired creature on seat 0's battlefield
	life := gs.Seats[0].Life
	UntapAll(gs, 0)
	untapDrain(gs)
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("inspired did not fire on the untap step (lifeΔ=%d, want 1)", got)
	}
}

// Double-fire guard: when a per_card handler owns the "inspired" event (the
// 6 Theros consumers — Pain Seer, Oreskos Sun Guide, …), the generic effect
// resolution is SKIPPED — the per_card hook is the single fire, no double.
func TestUntapAbilityWord_InspiredNoDoubleFire_PerCardGuard(t *testing.T) {
	gs := untapGame(t)
	p := inspiredPerm(gs)

	old := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Probe Inspired" && event == "inspired"
	}
	defer func() { HasTriggerHook = old }()

	life := gs.Seats[0].Life
	UntapPermanent(gs, p, "test")
	untapDrain(gs)
	// The generic resolution (the +1) must be skipped because per_card owns
	// it; the per_card hook fires instead (no real handler registered in this
	// unit test, so no life change) — proving the two paths don't BOTH fire.
	if gs.Seats[0].Life != life {
		t.Fatalf("generic inspired effect resolved despite a per_card handler owning it (double-fire path): lifeΔ=%d", gs.Seats[0].Life-life)
	}
}
