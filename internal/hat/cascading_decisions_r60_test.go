package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// cascading_decisions_r60_test.go — R60 audit of decision-cascade
// instrumentation. A single priority window can fan out into many
// nested decisions (cast a tutor → ChooseTarget on resolution →
// resolve trigger → ChooseTarget again → ChooseMode for modal
// payload). Pre-R60 the hat had three deficits:
//
//   1. tier instrumentation was only at the four parent entry points
//      (ChooseCastFromHand, ChooseActivation, ChooseAttackers,
//      ChooseResponse). MjolnirStats undercounted the real Ragnarok
//      footprint 3-to-1 when a Demonic-Tutor-style cast resolved
//      with multiple downstream Choose* calls.
//
//   2. no parent-tier propagation. A Ragnarok-evaluated tutor cast
//      cascaded into a TierMjolnir first-match ChooseTarget — the
//      expensive parent decision priced the WRONG thing.
//
//   3. OrderTriggers's triggerPriority dereferenced item.Card, but
//      OrderTriggers exclusively receives triggered abilities where
//      item.Card == nil. Every priority returned 0 and the sort was
//      a silent no-op; trigger ordering was effectively insertion
//      order regardless of trigger payload.
//
// Three fixes:
//   - recordParentTier(t, turn) on the four entry points stamps
//     lastParentTier + lastParentTierTurn so downstream Choose*
//     methods can read the cascade context.
//   - recordCascadeDecision(gs) on the downstream methods bumps
//     tierCounts using the parent tier (when still in the same
//     turn) — MjolnirStats reflects the full cascade now.
//   - triggerPriority uses effectiveResponseCard so trigger payload
//     oracle text drives the sort. OrderTriggers actually orders.

// -----------------------------------------------------------------------------
// recordParentTier — sets lastParentTier + turn stamp
// -----------------------------------------------------------------------------

func TestRecordParentTier_StampsLastTierAndTurn(t *testing.T) {
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 7)
	if h.lastParentTier != TierRagnarok {
		t.Errorf("lastParentTier should be TierRagnarok; got %v", h.lastParentTier)
	}
	if h.lastParentTierTurn != 7 {
		t.Errorf("lastParentTierTurn should be 7; got %d", h.lastParentTierTurn)
	}
}

func TestRecordParentTier_AlsoBumpsTierCount(t *testing.T) {
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	pre := h.tierCounts[TierGungnir]
	h.recordParentTier(TierGungnir, 3)
	if h.tierCounts[TierGungnir] != pre+1 {
		t.Errorf("recordParentTier must also bump tierCounts; pre=%d post=%d",
			pre, h.tierCounts[TierGungnir])
	}
}

// -----------------------------------------------------------------------------
// parentTierIs — turn-stamp invalidation
// -----------------------------------------------------------------------------

func TestParentTierIs_TrueWhenSameTurnAndAbove(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5)
	if !h.parentTierIs(gs, TierGungnir) {
		t.Errorf("Ragnarok >= Gungnir on same turn should report true")
	}
	if !h.parentTierIs(gs, TierRagnarok) {
		t.Errorf("Ragnarok >= Ragnarok on same turn should report true")
	}
}

func TestParentTierIs_FalseOnDifferentTurn(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 8
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5) // recorded on turn 5
	if h.parentTierIs(gs, TierGungnir) {
		t.Errorf("stale tier from previous turn must NOT report true; got Ragnarok bleed-through into turn 8")
	}
}

func TestParentTierIs_FalseWhenBelowMin(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierMjolnir, 5)
	if h.parentTierIs(gs, TierRagnarok) {
		t.Errorf("Mjolnir < Ragnarok should report false")
	}
}

// -----------------------------------------------------------------------------
// recordCascadeDecision — inherits parent tier within same turn
// -----------------------------------------------------------------------------

func TestRecordCascadeDecision_UsesParentTierSameTurn(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5)
	pre := h.tierCounts[TierRagnarok]
	got := h.recordCascadeDecision(gs)
	if got != TierRagnarok {
		t.Errorf("cascade decision should inherit parent tier Ragnarok; got %v", got)
	}
	if h.tierCounts[TierRagnarok] != pre+1 {
		t.Errorf("cascade must bump Ragnarok counter; pre=%d post=%d",
			pre, h.tierCounts[TierRagnarok])
	}
}

func TestRecordCascadeDecision_FallsBackToMjolnirOnNewTurn(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 8
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5) // stale
	got := h.recordCascadeDecision(gs)
	if got != TierMjolnir {
		t.Errorf("stale parent tier must NOT cascade; expected Mjolnir fallback, got %v", got)
	}
}

// -----------------------------------------------------------------------------
// Cast-then-target cascade — Ragnarok parent propagates to ChooseTarget
// -----------------------------------------------------------------------------

// TestChooseTarget_BumpsCascadeCounter pins the new instrumentation:
// ChooseTarget with 2+ legal targets now records a cascade decision,
// so MjolnirStats reflects the downstream work. Pre-R60, this method
// recorded nothing and the parent's tier was the only one counted.
func TestChooseTarget_BumpsCascadeCounter(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5)
	preRag := h.tierCounts[TierRagnarok]

	t1 := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Creature A", []string{"creature"}, 2, nil), 2, 2)
	t2 := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Creature B", []string{"creature"}, 3, nil), 3, 3)
	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: t1},
		{Kind: gameengine.TargetKindPermanent, Permanent: t2},
	}
	h.ChooseTarget(gs, 0, gameast.Filter{Base: "creature"}, legal)
	if h.tierCounts[TierRagnarok] != preRag+1 {
		t.Errorf("ChooseTarget under Ragnarok parent should cascade-bump Ragnarok counter; pre=%d post=%d",
			preRag, h.tierCounts[TierRagnarok])
	}
}

// TestChooseTarget_SingleLegalSkipsCascadeBump — when there's only
// one legal target there's no decision to make; we should NOT bump
// the cascade counter. Keeps the telemetry honest.
func TestChooseTarget_SingleLegalSkipsCascadeBump(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5)
	preRag := h.tierCounts[TierRagnarok]

	t1 := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Lone", []string{"creature"}, 2, nil), 2, 2)
	h.ChooseTarget(gs, 0, gameast.Filter{Base: "creature"},
		[]gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: t1}})
	if h.tierCounts[TierRagnarok] != preRag {
		t.Errorf("single legal target must NOT bump cascade; pre=%d post=%d",
			preRag, h.tierCounts[TierRagnarok])
	}
}

// -----------------------------------------------------------------------------
// ChooseMode + ChooseX + OrderTriggers cascade instrumentation
// -----------------------------------------------------------------------------

func TestChooseMode_BumpsCascadeCounter(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5)
	preRag := h.tierCounts[TierRagnarok]

	modes := []gameast.Effect{&gameast.Draw{}, &gameast.Damage{}}
	h.ChooseMode(gs, 0, modes)
	if h.tierCounts[TierRagnarok] != preRag+1 {
		t.Errorf("ChooseMode under Ragnarok parent should cascade-bump; pre=%d post=%d",
			preRag, h.tierCounts[TierRagnarok])
	}
}

func TestChooseX_BumpsCascadeCounter(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5)
	preRag := h.tierCounts[TierRagnarok]

	c := newTestCardMinimal("Hydra", []string{"creature"}, 0, nil)
	h.ChooseX(gs, 0, c, 6)
	if h.tierCounts[TierRagnarok] != preRag+1 {
		t.Errorf("ChooseX under Ragnarok parent should cascade-bump; pre=%d post=%d",
			preRag, h.tierCounts[TierRagnarok])
	}
}

func TestOrderTriggers_BumpsCascadeCounter(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	h.recordParentTier(TierRagnarok, 5)
	preRag := h.tierCounts[TierRagnarok]

	src1 := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("A", []string{"creature"}, 2, nil), 2, 2)
	src2 := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("B", []string{"creature"}, 2, nil), 2, 2)
	trigs := []*gameengine.StackItem{
		{Kind: "triggered", Source: src1, Controller: 0},
		{Kind: "triggered", Source: src2, Controller: 0},
	}
	h.OrderTriggers(gs, 0, trigs)
	if h.tierCounts[TierRagnarok] != preRag+1 {
		t.Errorf("OrderTriggers under Ragnarok parent should cascade-bump; pre=%d post=%d",
			preRag, h.tierCounts[TierRagnarok])
	}
}

// -----------------------------------------------------------------------------
// triggerPriority — source-card-aware (bug fix from #310 family)
// -----------------------------------------------------------------------------

// TestTriggerPriority_ReadsSourceCardForTriggeredAbilities pins the
// fix to the silent no-op bug. Pre-R60 triggerPriority returned 0
// for all triggered abilities (because item.Card == nil) and
// OrderTriggers's sort was effectively insertion-order. Now the
// source's oracle text drives the score.
func TestTriggerPriority_ReadsSourceCardForTriggeredAbilities(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	// High-priority trigger source (draw + token).
	highSrc := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Bountiful Promise", []string{"creature"}, 3,
			&gameast.CardAST{
				Name:      "Bountiful Promise",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Whenever this attacks, draw a card and create a 1/1 token."}},
			}),
		2, 2)
	highTrig := &gameengine.StackItem{Kind: "triggered", Source: highSrc, Controller: 0}

	// Vanilla low-priority source (no payload keywords).
	lowSrc := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Vanilla", []string{"creature"}, 1, nil),
		1, 1)
	lowTrig := &gameengine.StackItem{Kind: "triggered", Source: lowSrc, Controller: 0}

	hi := h.triggerPriority(highTrig)
	lo := h.triggerPriority(lowTrig)
	if hi <= lo {
		t.Errorf("trigger with draw+token oracle should outrank vanilla source; hi=%.2f lo=%.2f",
			hi, lo)
	}
}

// TestOrderTriggers_ActuallyReorders pins the end-to-end fix.
// Pre-R60 a vanilla trigger before a high-impact trigger stayed in
// insertion order. Post-R60 the high-impact trigger floats to the
// END (LIFO resolution → end-of-slice resolves first).
func TestOrderTriggers_ActuallyReorders(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	highSrc := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Bountiful", []string{"creature"}, 3,
			&gameast.CardAST{
				Name:      "Bountiful",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Draw a card and create a 1/1 token."}},
			}),
		2, 2)
	lowSrc := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Vanilla", []string{"creature"}, 1, nil),
		1, 1)

	// Insertion order: high first, vanilla second.
	in := []*gameengine.StackItem{
		{Kind: "triggered", Source: highSrc, Controller: 0},
		{Kind: "triggered", Source: lowSrc, Controller: 0},
	}
	out := h.OrderTriggers(gs, 0, in)
	if len(out) != 2 {
		t.Fatalf("expected 2 items; got %d", len(out))
	}
	// Highest priority should be at the END (resolves first via LIFO).
	if out[len(out)-1].Source != highSrc {
		t.Errorf("highest-priority trigger should be last in slice (resolves first); got source=%v at end",
			out[len(out)-1].Source)
	}
}
