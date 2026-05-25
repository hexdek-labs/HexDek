package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// stack_resolution_r60_test.go — R60 follow-up to #310 (priority-
// window awareness for triggers). Adds multi-LEVEL stack-resolution
// awareness: the hat now reasons about which stack item is the
// meaningful target of its response across the whole stack, not just
// the literal top.
//
// Two new signals on stackDepthSignals:
//
//   - holdForBiggerBelow: a higher-threat hostile item sits below
//     the top, and we have a SINGLE affordable counter in hand.
//     Burning the counter on the smaller top wastes it — passing
//     lets the small top resolve, then the engine grants us a fresh
//     priority window over the bigger threat (now the new top).
//     Disabled with 2+ counters (we can counter both).
//
//   - forceCounterShield: top is a counter-protection spell
//     (Veil of Summer / Autumn's Veil / Silence-shape) and a hostile
//     item sits below it. We have ONE window — counter the shield
//     now or watch our entire counter chain fail on the bigger
//     threats below. Sets mustCounter.

// shieldCard builds a counter-protection-shape spell. Used by the
// forceCounterShield tests.
func shieldCard(name string, oracle string) *gameengine.Card {
	return newTestCardMinimal(name, []string{"instant"}, 1,
		&gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{&gameast.Static{Raw: oracle}},
		})
}

// pushOppSpell — helper to add a hostile spell onto gs.Stack.
func pushOppSpell(gs *gameengine.GameState, card *gameengine.Card, controller int) *gameengine.StackItem {
	item := &gameengine.StackItem{
		ID:         len(gs.Stack) + 1,
		Card:       card,
		Controller: controller,
	}
	gs.Stack = append(gs.Stack, item)
	return item
}

// -----------------------------------------------------------------------------
// holdForBiggerBelow — defer the counter for the bigger threat below
// -----------------------------------------------------------------------------

// TestStackResolution_HoldsForBiggerBelow_SingleCounter pins the
// canonical case: a 1-CMC cantrip top, a 7-CMC mass-removal bottom,
// and we have ONE Counterspell in hand. Pre-#310 the hat would
// counter the cantrip (well, actually it scored 1 < minScore=3 so
// it passed); the meaningful case for THIS audit is a borderline
// top (score ≥ minScore) where the bigger threat below should
// override into a pass.
//
// Stack: [Cyclonic Rift (CMC 7, destroy-all bonus → score 12),
//         Mind Stone (CMC 2, top)]. Top is below minScore so it
// passes naturally — but we want to assert the helper signal too.
func TestStackResolution_HoldsForBiggerBelow_SingleCounter(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	// Bottom: Cyclonic Rift overload — mass bounce.
	pushOppSpell(gs,
		spellWithOracle("Cyclonic Rift", 7, "Return all nonland permanents your opponents control to their owners' hands."),
		1)
	// Borderline top: 3-CMC removal that targets nothing of ours, so
	// the gate would otherwise let it through.
	top := pushOppSpell(gs,
		spellWithOracle("Lava Coil", 3, "Lava Coil deals 4 damage to target creature."),
		1)

	var sig stackDepthSignals
	h.stackResolutionSignals(gs, 0, top, &sig)
	if !sig.holdForBiggerBelow {
		t.Errorf("expected holdForBiggerBelow when bigger threat (Rift) sits below borderline top; sig=%+v",
			sig)
	}
}

// TestStackResolution_NoHoldWithMultipleCounters pins the
// multi-counter relaxation: with two affordable counters in hand,
// we can counter the top AND the bigger below — defer is
// inappropriate.
func TestStackResolution_NoHoldWithMultipleCounters(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand(), counterInHand()}
	gs.Seats[0].ManaPool = 8
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushOppSpell(gs,
		spellWithOracle("Cyclonic Rift", 7, "Return all nonland permanents your opponents control to their owners' hands."),
		1)
	top := pushOppSpell(gs,
		spellWithOracle("Lava Coil", 3, "Lava Coil deals 4 damage to target creature."),
		1)

	var sig stackDepthSignals
	h.stackResolutionSignals(gs, 0, top, &sig)
	if sig.holdForBiggerBelow {
		t.Errorf("must NOT hold when we have 2+ counters (can spend on both); sig=%+v", sig)
	}
}

// TestStackResolution_NoHoldWhenTopIsBigger pins the inverse: when
// the top IS the bigger threat and only smaller items sit below,
// no deferral.
func TestStackResolution_NoHoldWhenTopIsBigger(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 8
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushOppSpell(gs,
		spellWithOracle("Lightning Bolt", 1, "Lightning Bolt deals 3 damage to any target."),
		1)
	top := pushOppSpell(gs,
		spellWithOracle("Cyclonic Rift", 7, "Return all nonland permanents your opponents control to their owners' hands."),
		1)

	var sig stackDepthSignals
	h.stackResolutionSignals(gs, 0, top, &sig)
	if sig.holdForBiggerBelow {
		t.Errorf("must not hold when top itself is the bigger threat; sig=%+v", sig)
	}
}

// TestStackResolution_NoHoldOnSinglestackItem pins the depth=1
// fast-path: with only the top on the stack, no below comparison
// applies.
func TestStackResolution_NoHoldOnSingleStackItem(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	top := pushOppSpell(gs,
		spellWithOracle("Lava Coil", 3, "Lava Coil deals 4 damage to target creature."),
		1)

	var sig stackDepthSignals
	h.stackResolutionSignals(gs, 0, top, &sig)
	if sig.holdForBiggerBelow {
		t.Errorf("depth=1 must not trigger holdForBiggerBelow; sig=%+v", sig)
	}
}

// TestStackResolution_NoHoldWhenMustCounterAlreadySet — when an
// upstream signal already mandates the counter (e.g., shield, Etali,
// our spell being countered), holdForBiggerBelow is bypassed so the
// must-counter wins. The helper short-circuits early.
func TestStackResolution_NoHoldWhenMustCounterAlreadySet(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushOppSpell(gs,
		spellWithOracle("Cyclonic Rift", 7, "Return all nonland permanents your opponents control to their owners' hands."),
		1)
	top := pushOppSpell(gs,
		spellWithOracle("Lava Coil", 3, "Lava Coil deals 4 damage to target creature."),
		1)

	sig := stackDepthSignals{mustCounter: true, reason: "preset_must_counter"}
	h.stackResolutionSignals(gs, 0, top, &sig)
	if sig.holdForBiggerBelow {
		t.Errorf("must-counter override must suppress holdForBiggerBelow; sig=%+v", sig)
	}
}

// -----------------------------------------------------------------------------
// forceCounterShield — top is a counter-protection spell
// -----------------------------------------------------------------------------

// TestStackResolution_ForceCounterShield_VeilOfSummer pins the
// canonical case: opp casts Veil of Summer above a Cyclonic Rift.
// If Veil resolves, our blue counters can't touch the Rift. The
// shield is our ONE window — must counter it now.
func TestStackResolution_ForceCounterShield_VeilOfSummer(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushOppSpell(gs,
		spellWithOracle("Cyclonic Rift", 7, "Return all nonland permanents your opponents control to their owners' hands."),
		1)
	veil := shieldCard("Veil of Summer",
		"Draw a card if an opponent has cast a blue or black spell this turn. Until end of turn, you and permanents you control gain hexproof from blue and from black.")
	top := pushOppSpell(gs, veil, 1)

	var sig stackDepthSignals
	h.stackResolutionSignals(gs, 0, top, &sig)
	if !sig.forceCounterShield {
		t.Errorf("Veil of Summer with hostile threat below should force counter-shield; sig=%+v",
			sig)
	}
	if !sig.mustCounter {
		t.Errorf("forceCounterShield must also set mustCounter; sig=%+v", sig)
	}
}

// TestStackResolution_ForceCounterShield_NoFireWithoutHostileBelow
// pins the negative: a Veil with NOTHING under it isn't a shield
// over anything — let it resolve and save the counter for later.
func TestStackResolution_ForceCounterShield_NoFireWithoutHostileBelow(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	veil := shieldCard("Veil of Summer",
		"Until end of turn, you and permanents you control gain hexproof from blue and from black.")
	top := pushOppSpell(gs, veil, 1)

	var sig stackDepthSignals
	h.stackResolutionSignals(gs, 0, top, &sig)
	if sig.forceCounterShield {
		t.Errorf("Veil with no hostile threat below shouldn't force counter; sig=%+v", sig)
	}
}

// TestStackResolution_ForceCounterShield_CantBeCountered pins the
// "Cavern of Souls" / "Conqueror's Flail" shape — "spells you cast
// can't be countered." Same family as Veil; mustCounter when
// hostile items pending below.
func TestStackResolution_ForceCounterShield_CantBeCounteredShape(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushOppSpell(gs,
		spellWithOracle("Cyclonic Rift", 7, "Return all nonland permanents your opponents control to their owners' hands."),
		1)
	shield := shieldCard("Autumn's Veil",
		"Spells you control can't be countered by blue or black spells this turn, and creatures you control can't be the targets of blue or black spells this turn.")
	top := pushOppSpell(gs, shield, 1)

	var sig stackDepthSignals
	h.stackResolutionSignals(gs, 0, top, &sig)
	if !sig.forceCounterShield {
		t.Errorf("Autumn's Veil ('can't be countered') with hostile below should fire shield signal; sig=%+v",
			sig)
	}
}

// -----------------------------------------------------------------------------
// Integration with ChooseResponse — holdForBiggerBelow forces pass
// -----------------------------------------------------------------------------

// TestChooseResponse_HoldsForBiggerThreatBelow pins the end-to-end
// pass behavior. Setup: midrange hat with single Counterspell;
// stack has Rift (CMC 7) below a borderline 3-CMC removal. Score
// of the removal (3) ≥ midrange minScore (3) — would normally
// counter — but the bigger threat below should defer.
func TestChooseResponse_HoldsForBiggerThreatBelow(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushOppSpell(gs,
		spellWithOracle("Cyclonic Rift", 7, "Return all nonland permanents your opponents control to their owners' hands."),
		1)
	top := pushOppSpell(gs,
		spellWithOracle("Lava Coil", 3, "Lava Coil deals 4 damage to target creature."),
		1)

	if resp := h.ChooseResponse(gs, 0, top); resp != nil {
		t.Errorf("hat should HOLD the counter for the Rift below (single counter); got %v",
			resp.Card.DisplayName())
	}
}

// TestChooseResponse_VeilOfSummerForcesCounter — Veil top with Rift
// below forces the must-counter path, overriding any pass.
func TestChooseResponse_VeilOfSummerForcesCounter(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	pushOppSpell(gs,
		spellWithOracle("Cyclonic Rift", 7, "Return all nonland permanents your opponents control to their owners' hands."),
		1)
	veil := shieldCard("Veil of Summer",
		"Until end of turn, you and permanents you control gain hexproof from blue and from black.")
	top := pushOppSpell(gs, veil, 1)

	if resp := h.ChooseResponse(gs, 0, top); resp == nil {
		t.Errorf("Veil of Summer above Rift must force counter; got pass")
	}
}

// TestStackResolutionSignals_NilSafe pins the boundary: nil inputs
// return without mutating sig.
func TestStackResolutionSignals_NilSafe(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)

	var sig stackDepthSignals
	h.stackResolutionSignals(nil, 0, nil, &sig)
	if sig.holdForBiggerBelow || sig.forceCounterShield {
		t.Errorf("nil inputs must leave sig untouched; got %+v", sig)
	}
	h.stackResolutionSignals(gs, -1, &gameengine.StackItem{}, &sig)
	if sig.holdForBiggerBelow || sig.forceCounterShield {
		t.Errorf("invalid seatIdx must leave sig untouched; got %+v", sig)
	}
}
