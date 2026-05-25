package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// priority_stack_depth_r60_test.go — R60 stack-depth awareness in
// ChooseResponse.
//
// Survey: at stack depth ≥ 3 the pre-fix ChooseResponse misjudged three
// concrete response opportunities because it only inspected the literal
// top StackItem via stackItemScore (CMC + mass-effect bump):
//
//   1. Counter-targeting-our-spell — opp casts Counterspell on our
//      spell below; the 2-CMC counter scored ≈ 2, below minScore=3 for
//      midrange. The hat passed, the counter resolved, our committed
//      spell evaporated.
//   2. Investment-protection — we had a non-copy spell on the stack
//      below, opp put a trigger/spell above. The pile-on could resolve
//      and disrupt our spell via state changes (additional counters,
//      ETB pingers, target-redirecting removal), but the top's CMC
//      proxy didn't see the stake.
//   3. Hostile pile-up — 2+ opponent items pending. Holding a counter
//      for "the next big threat" is the wrong frame at deep stacks —
//      the pile IS the big moment, and the counter held past it
//      usually fails to find a use.
//
// computeStackDepthSignals produces three signals (mustCounter,
// scoreBonus, minScoreDelta) consumed by ChooseResponse to address
// each case. These regressions pin the contract.

// pushStackItem appends a StackItem onto gs.Stack with the given card,
// controller, and optional targets. Returns the pushed item for further
// linking (e.g. as a Target of a later item).
func pushStackItem(gs *gameengine.GameState, card *gameengine.Card, controller int, targets ...gameengine.Target) *gameengine.StackItem {
	item := &gameengine.StackItem{
		ID:         len(gs.Stack) + 1,
		Card:       card,
		Controller: controller,
		Targets:    targets,
	}
	gs.Stack = append(gs.Stack, item)
	return item
}

// counterspellTargetingStack builds a Counterspell-shaped card with
// oracle text the heuristic-fallback scan recognizes ("counter target
// spell"). Used in the resolved-target tests by stamping its Targets
// directly; used in the heuristic test by leaving Targets empty.
func counterspellTargetingStack() *gameengine.Card {
	return spellWithOracle("Counterspell", 2, "Counter target spell.")
}

// -----------------------------------------------------------------------------
// Signal 1 — counter-targeting-our-spell (resolved Targets path)
// -----------------------------------------------------------------------------

// TestChooseResponse_StackDepth_CountersCounterTargetingOurSpell pins
// the mustCounter trigger for the most-impactful misjudgement: an
// opponent's Counterspell pointed at OUR pending spell. Pre-fix, the
// 2-CMC counter scored below minScore and we passed — the fix forces
// the counter.
func TestChooseResponse_StackDepth_CountersCounterTargetingOurSpell(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	// Stack (bottom → top):
	//   [our Cultivate]                   ← we committed mana to this
	//   [opp Counterspell targets ours]   ← top of stack — must counter
	ourSpell := pushStackItem(gs, spellWithOracle("Cultivate", 3, "Search your library for up to two basic land cards."), 0)
	oppCounter := pushStackItem(gs, counterspellTargetingStack(), 1,
		gameengine.Target{Kind: gameengine.TargetKindStackItem, Stack: ourSpell})

	resp := h.ChooseResponse(gs, 0, oppCounter)
	if resp == nil {
		t.Fatalf("hat MUST counter an opp Counterspell pointed at our spell; got pass")
	}
	if resp.Card == nil || resp.Card.DisplayName() != "Counterspell" {
		t.Errorf("hat should respond with the Counterspell in hand; got %v", resp.Card)
	}
}

// -----------------------------------------------------------------------------
// Signal 1 — heuristic fallback when Targets aren't resolved
// -----------------------------------------------------------------------------

// TestChooseResponse_StackDepth_CountersCounter_HeuristicFallback pins
// the oracle-text fallback used when the engine pushed the spell
// without a resolved Targets slice (some paths leave Targets empty,
// including a number of test fixtures). The heuristic: top has
// "counter target spell" oracle AND one of our spells is below.
func TestChooseResponse_StackDepth_CountersCounter_HeuristicFallback(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushStackItem(gs, spellWithOracle("Our Spell", 3, "Draw three cards."), 0)
	// Note: no Targets stamped on the opp counter — heuristic path only.
	oppCounter := pushStackItem(gs, counterspellTargetingStack(), 1)

	resp := h.ChooseResponse(gs, 0, oppCounter)
	if resp == nil {
		t.Fatalf("heuristic must-counter must fire when top is a counter and our spell is below")
	}
}

// -----------------------------------------------------------------------------
// Signal 1 — do NOT fire when the counter targets an OPPONENT's spell
// -----------------------------------------------------------------------------

// TestChooseResponse_StackDepth_DoesNotMustCounterOppOnOpp pins the
// politics-aware contract: an opp counter aimed at ANOTHER opp's spell
// is helping us — we shouldn't burn our counter to save the spell.
// (The political-allocation path in ChooseResponse handles this for
// the score-gate route; the stack-depth path must not override it.)
func TestChooseResponse_StackDepth_DoesNotMustCounterOppOnOpp(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	// Seat 2's spell, seat 1 counters it — we're seat 0.
	oppSpell := pushStackItem(gs, spellWithOracle("Opp Spell", 4, "Draw four cards."), 2)
	oppOnOppCounter := pushStackItem(gs, counterspellTargetingStack(), 1,
		gameengine.Target{Kind: gameengine.TargetKindStackItem, Stack: oppSpell})

	sig := h.computeStackDepthSignals(gs, 0, oppOnOppCounter)
	if sig.mustCounter {
		t.Errorf("must-counter should not fire when opp counter targets a different opp's spell (politics)")
	}
}

// -----------------------------------------------------------------------------
// Signal 2 — investment-protection at deep stack
// -----------------------------------------------------------------------------

// TestComputeStackDepthSignals_InvestmentBelowAtDepth pins the
// scoreBonus when stack depth ≥ 3 and we have a non-copy spell of our
// own on the stack below the top. The pile-on raises the stake of
// letting the top resolve.
func TestComputeStackDepthSignals_InvestmentBelowAtDepth(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushStackItem(gs, spellWithOracle("Our Spell", 3, "Draw two cards."), 0)
	pushStackItem(gs, spellWithOracle("Opp Trigger", 2, "Each player draws a card."), 1)
	top := pushStackItem(gs, spellWithOracle("Opp Spell B", 2, "Target creature gets +2/+2."), 1)

	sig := h.computeStackDepthSignals(gs, 0, top)
	if sig.scoreBonus < 2 {
		t.Errorf("investment-below-at-depth should add scoreBonus ≥ 2; got %d (reason=%q)",
			sig.scoreBonus, sig.reason)
	}
}

// TestComputeStackDepthSignals_ShallowStackNoInvestmentBonus pins the
// negative side: at depth < 3 the investment bonus must not fire
// (cheap path is the common case; we don't want to over-respond to a
// 2-item stack).
func TestComputeStackDepthSignals_ShallowStackNoInvestmentBonus(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushStackItem(gs, spellWithOracle("Our Spell", 3, "Draw two cards."), 0)
	// Top is not a counter, so signal-1 won't fire either.
	top := pushStackItem(gs, spellWithOracle("Opp Spell", 2, "Target creature gets +2/+2."), 1)

	sig := h.computeStackDepthSignals(gs, 0, top)
	if sig.scoreBonus != 0 {
		t.Errorf("depth=2 (no signal-1 match) should produce no scoreBonus; got %d (reason=%q)",
			sig.scoreBonus, sig.reason)
	}
	if sig.minScoreDelta != 0 {
		t.Errorf("depth=2 should produce no minScoreDelta; got %d", sig.minScoreDelta)
	}
}

// -----------------------------------------------------------------------------
// Signal 3 — hostile pile-up at depth
// -----------------------------------------------------------------------------

// TestComputeStackDepthSignals_HostilePileUpLowersThreshold pins the
// minScoreDelta=1 trigger when 2+ hostile items are pending below the
// top. Counter-war / trigger-chain commit-or-lose framing.
func TestComputeStackDepthSignals_HostilePileUpLowersThreshold(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushStackItem(gs, spellWithOracle("Opp Spell A", 2, "Each opponent loses 2 life."), 1)
	pushStackItem(gs, spellWithOracle("Opp Spell B", 2, "Target creature gets +2/+2."), 1)
	top := pushStackItem(gs, spellWithOracle("Opp Spell C", 2, "Look at the top three cards of your library."), 1)

	sig := h.computeStackDepthSignals(gs, 0, top)
	if sig.minScoreDelta != 1 {
		t.Errorf("hostile pile-up at depth should produce minScoreDelta=1; got %d (reason=%q)",
			sig.minScoreDelta, sig.reason)
	}
}

// -----------------------------------------------------------------------------
// Signal 2/3 negative — no false positive on our own pile-up
// -----------------------------------------------------------------------------

// TestComputeStackDepthSignals_OurOwnPileNoHostileDelta pins that
// minScoreDelta only fires on HOSTILE pile-up; our own chain of cast
// spells (we cast 3 things in a row, opp passes) must not lower our
// own gate.
func TestComputeStackDepthSignals_OurOwnPileNoHostileDelta(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushStackItem(gs, spellWithOracle("Our A", 2, "Draw a card."), 0)
	pushStackItem(gs, spellWithOracle("Our B", 2, "Draw a card."), 0)
	top := pushStackItem(gs, spellWithOracle("Opp X", 2, "Target creature gets +2/+2."), 1)

	sig := h.computeStackDepthSignals(gs, 0, top)
	if sig.minScoreDelta != 0 {
		t.Errorf("our own spells below top should NOT trigger hostile pile-up delta; got %d (reason=%q)",
			sig.minScoreDelta, sig.reason)
	}
}

// -----------------------------------------------------------------------------
// Integration — depth-context bonus tips a borderline incoming over
// -----------------------------------------------------------------------------

// TestComputeStackDepthSignals_TargetsOurPermanentBonus pins the
// +2 scoreBonus when top targets a permanent we control. The bonus is
// what ChooseResponse folds into stackItemScore before the minScore
// gate; testing it at the helper level keeps the assertion stable
// against unrelated minScore tuning (relPos, kingmaker, politics).
func TestComputeStackDepthSignals_TargetsOurPermanentBonus(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	ourCreature := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Our Threat", []string{"creature"}, 0, nil), 4, 4)

	removal := spellWithOracle("Spot Removal", 2, "Target creature gets -3/-3 until end of turn.")
	top := pushStackItem(gs, removal, 1,
		gameengine.Target{Kind: gameengine.TargetKindPermanent, Permanent: ourCreature})

	sig := h.computeStackDepthSignals(gs, 0, top)
	if sig.scoreBonus < 2 {
		t.Errorf("targets-our-permanent should add scoreBonus ≥ 2; got %d (reason=%q)",
			sig.scoreBonus, sig.reason)
	}
	if sig.reason == "" {
		t.Errorf("scoreBonus producing path must set a reason for log audit")
	}
}

// -----------------------------------------------------------------------------
// Negative — no signals at depth 1, fast path
// -----------------------------------------------------------------------------

// TestComputeStackDepthSignals_Depth1NoSignals pins the fast-path
// contract: at the depth=1 common case (single spell, no other stack
// activity), no signal fires.
func TestComputeStackDepthSignals_Depth1NoSignals(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	top := pushStackItem(gs, spellWithOracle("Lone Spell", 3, "Draw two cards."), 1)

	sig := h.computeStackDepthSignals(gs, 0, top)
	if sig.mustCounter {
		t.Errorf("depth=1 should not produce mustCounter")
	}
	if sig.scoreBonus != 0 {
		t.Errorf("depth=1 should produce no scoreBonus; got %d", sig.scoreBonus)
	}
	if sig.minScoreDelta != 0 {
		t.Errorf("depth=1 should produce no minScoreDelta; got %d", sig.minScoreDelta)
	}
}

// TestComputeStackDepthSignals_NilTopIsSafe pins the boundary: nil top
// or out-of-range seatIdx returns zero-value signals without panic.
func TestComputeStackDepthSignals_NilTopIsSafe(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	sig := h.computeStackDepthSignals(gs, 0, nil)
	if sig.mustCounter || sig.scoreBonus != 0 || sig.minScoreDelta != 0 {
		t.Errorf("nil top must return zero signals; got %+v", sig)
	}
	sig = h.computeStackDepthSignals(gs, -1, &gameengine.StackItem{Controller: 1, Card: &gameengine.Card{AST: &gameast.CardAST{}}})
	if sig.mustCounter || sig.scoreBonus != 0 || sig.minScoreDelta != 0 {
		t.Errorf("invalid seatIdx must return zero signals; got %+v", sig)
	}
}
