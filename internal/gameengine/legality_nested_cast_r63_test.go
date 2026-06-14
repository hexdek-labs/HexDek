package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestLegality_NestedSameSeatCast_NotFlaggedAsDoubleDeduction replays loki
// r63b sweep Cluster 2 (seed 3361338, game 336, turn 22, seat 3).
//
// Mechanism: a seat casts Invisibility ({1}{U}, CMC 2). The cast fires a
// controller's cast-trigger (Urtet, Remnant of Memnarch) which lands on the
// stack and grants priority INSIDE the announcing window — the engine's
// accepted "trigger resolves inside the cast window" pattern (see CastSpell's
// gs.ResolvingCards note). The same seat responds by casting a SECOND spell,
// Consign to Memory ({U}, CMC 1), and pays its cost before Invisibility's
// FinishCast snapshots PoolAfter. The seat's pool moves 4 -> 2 (Invisibility's
// {1}{U}) -> 1 (Consign's {U}).
//
// Pre-fix the OUTER (Invisibility) §601.2f-h cost-paid window read a pool
// delta of 3 (PoolBefore 4 - PoolAfter 1) against an announced total of 2 and
// false-flagged "over-paid (double-deduction)" — even though the engine never
// over-charged anything: it is two legitimate casts (2 + 1). The nested cast's
// own base cost was the missing third leg of the in-window accounting
// (NoteManaAdd / NoteManaSpend already mirror a child's added / aux mana onto
// the parent; creditNestedSpendToParents now folds in the child's base cost).
//
// This pins the EXACT pool delta from game 336 and the exact aux credit.
func TestLegality_NestedSameSeatCast_NotFlaggedAsDoubleDeduction(t *testing.T) {
	gs := legalityFixture(t)
	seat := gs.Seats[0]
	seat.ManaPool = 4
	EnsureTypedPool(seat) // bridge the 4 into the typed pool

	outer := &Card{
		Name:   "Invisibility",
		Owner:  0,
		Colors: []string{"U"},
		Types:  []string{"enchantment", "aura", "cost:2"},
		AST:    &gameast.CardAST{Name: "Invisibility"},
	}
	inner := instantSpellCard("Consign to Memory", 1, []string{"U"})

	// --- Invisibility announced; its {1}{U} leaves the pool (4 -> 2). ---
	outerObs := gs.Legality.BeginCast(gs, 0, outer)
	if outerObs.BaseCostAtAnnounce != 2 {
		t.Fatalf("fixture broken: Invisibility announced cost = %d, want 2", outerObs.BaseCostAtAnnounce)
	}
	seat.ManaPool -= 2
	SyncManaAfterSpend(seat)

	// --- Nested Consign to Memory cast + paid + finished INSIDE the window
	//     (2 -> 1). ---
	innerObs := gs.Legality.BeginCast(gs, 0, inner)
	seat.ManaPool -= 1
	SyncManaAfterSpend(seat)
	gs.Legality.FinishCast(gs, innerObs, &StackItem{Controller: 0, Card: inner})

	// --- Invisibility's window closes; PoolAfter is snapshotted at 1. ---
	gs.Legality.FinishCast(gs, outerObs, &StackItem{Controller: 0, Card: outer})

	if got := violationsByRule(gs.Legality, "601.2f-h"); len(got) != 0 {
		t.Fatalf("nested same-seat cast false-flagged the outer spell as a double-deduction: %v", got)
	}
	// The nested cast's net cost (1) must have been credited to the outer
	// window's aux total — exactly, not approximately.
	if outerObs.AuxManaSpentDuringWindow != 1 {
		t.Fatalf("outer aux credit = %d, want exactly 1 (the nested cast's {U} cost)",
			outerObs.AuxManaSpentDuringWindow)
	}
	// The inner window must independently read clean (spent 1 == its {U}).
	if outerObs.PoolBefore != 4 || outerObs.PoolAfter != 1 || innerObs.PoolBefore != 2 || innerObs.PoolAfter != 1 {
		t.Fatalf("pool trajectory drifted from game 336: outer[%d->%d] inner[%d->%d], want outer[4->1] inner[2->1]",
			outerObs.PoolBefore, outerObs.PoolAfter, innerObs.PoolBefore, innerObs.PoolAfter)
	}
}

// TestLegality_NoNestedCast_RealOverpayStillFlags guards the fix against
// over-suppression: with NO nested cast, the same outer numbers (pool delta 3
// vs announced 2) must still flag, so creditNestedSpendToParents only neutralizes
// genuine nested-cast costs, never a true single-cast over-deduction.
func TestLegality_NoNestedCast_RealOverpayStillFlags(t *testing.T) {
	gs := legalityFixture(t)
	seat := gs.Seats[0]
	seat.ManaPool = 4
	EnsureTypedPool(seat)

	outer := &Card{
		Name:   "Invisibility",
		Owner:  0,
		Colors: []string{"U"},
		Types:  []string{"enchantment", "aura", "cost:2"},
		AST:    &gameast.CardAST{Name: "Invisibility"},
	}

	obs := gs.Legality.BeginCast(gs, 0, outer)
	// Simulate a genuine 3-for-2 over-charge with no nested cast in flight.
	seat.ManaPool -= 3
	SyncManaAfterSpend(seat)
	gs.Legality.FinishCast(gs, obs, &StackItem{Controller: 0, Card: outer})

	if got := violationsByRule(gs.Legality, "601.2f-h"); len(got) != 1 {
		t.Fatalf("real single-cast over-deduction (3 vs 2) no longer flagged — fix over-suppresses: %v", got)
	}
}
