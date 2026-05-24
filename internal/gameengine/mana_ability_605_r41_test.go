package gameengine

// CR §605 mana-ability verification (r41).
//
// CR §605.1a — A mana ability is an activated ability that (1) doesn't
//   target, (2) could put mana into a player's mana pool when it resolves,
//   and (3) is not a loyalty ability.
//
// CR §605.3a — Activating a mana ability doesn't use the stack. It
//   resolves immediately and can't be responded to.
//
// CR §605.1b — A triggered ability is a mana ability only if it doesn't
//   target AND it triggers from an activated or triggered mana ability.
//   ETB triggers that produce mana (e.g. Priest of Urabrask, "When ~
//   enters, add {R}{R}") do NOT qualify — they use the stack like any
//   other triggered ability.
//
// This file verifies that contract end-to-end:
//   - Llanowar Elves (1G creature, {T}: Add {G}) — mana ability, no stack.
//   - Bloom Tender (1G creature, {T}: Add one mana of any color for each
//     color among permanents you control) — mana ability, no stack.
//   - Sol Ring (artifact, {T}: Add {C}{C}) — mana ability, no stack.
//   - Priest of Urabrask (creature, ETB add {R}{R}) — triggered ability,
//     DOES use the stack and is responseable. Confirms the §605.1b
//     boundary.
//
// And the cost-payment contract (§605.3a + §602.1b): a mana ability
// resolves inline, so its mana is available immediately within the same
// priority window — concretely, the controller can activate to fill
// their pool and then pay a cost.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// makeManaCreature builds a creature with a single {T}: AddMana ability.
// `pool` is a parsed mana symbol list (e.g. "{G}" -> {Color: ["G"]}); if
// `anyCount` > 0 the ability instead produces N any-color pips.
func makeManaCreature(gs *GameState, seat int, name string, pool []gameast.ManaSymbol, anyCount int) *Permanent {
	eff := &gameast.AddMana{Pool: pool, AnyColorCount: anyCount}
	perm := makeCreaturePerm(gs, seat, name, eff, true)
	// Llanowar Elves cast turn 1 has summoning sickness; the §605 contract
	// is tested independently of that timing — clear it so the activation
	// reaches the dispatcher.
	perm.SummoningSick = false
	return perm
}

// stackPushesSince counts "stack_push" events emitted after eventIdx (the
// log length captured before the call under test).
func stackPushesSince(gs *GameState, eventIdx int) int {
	n := 0
	for i := eventIdx; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "stack_push" {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// §605.3a — mana abilities do not use the stack.
// ---------------------------------------------------------------------------

func TestMana605_LlanowarElves_NoStack(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0

	elves := makeManaCreature(gs, 0, "Llanowar Elves",
		[]gameast.ManaSymbol{{Raw: "{G}", Color: []string{"G"}}}, 0)

	if !IsManaAbility(elves, 0) {
		t.Fatal("Llanowar Elves {T}: Add {G} must classify as a mana ability")
	}

	preLog := len(gs.EventLog)
	preStack := len(gs.Stack)

	if err := ActivateAbility(gs, 0, elves, 0, nil); err != nil {
		t.Fatalf("activation should succeed: %v", err)
	}

	// §605.3a: did not use the stack at any point.
	if got := stackPushesSince(gs, preLog); got != 0 {
		t.Fatalf("mana ability must not push to stack; got %d stack_push events", got)
	}
	if len(gs.Stack) != preStack {
		t.Fatalf("stack size changed: %d -> %d", preStack, len(gs.Stack))
	}

	// The mana ability-specific event must fire (not the generic one).
	if !hasEventOfKind(gs, "activate_mana_ability") {
		t.Fatal("expected activate_mana_ability event")
	}
	if hasEventOfKind(gs, "activate_ability") {
		t.Fatal("mana ability must NOT log activate_ability (stack path)")
	}

	// Pool got a pip.
	if gs.Seats[0].ManaPool != 1 {
		t.Fatalf("Llanowar Elves should produce 1 mana; pool=%d", gs.Seats[0].ManaPool)
	}
	if !elves.Tapped {
		t.Fatal("Llanowar Elves should be tapped after activation")
	}
}

func TestMana605_BloomTender_NoStack(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0

	// Bloom Tender: {T}: Add one mana of any color for each color among
	// permanents you control. With G+W permanents the parser would emit
	// AnyColorCount=2; the §605 contract is independent of the count.
	tender := makeManaCreature(gs, 0, "Bloom Tender", nil, 2)

	if !IsManaAbility(tender, 0) {
		t.Fatal("Bloom Tender {T}: Add any-color mana must classify as a mana ability")
	}

	preLog := len(gs.EventLog)
	if err := ActivateAbility(gs, 0, tender, 0, nil); err != nil {
		t.Fatalf("activation should succeed: %v", err)
	}

	if got := stackPushesSince(gs, preLog); got != 0 {
		t.Fatalf("Bloom Tender mana ability must not push to stack; got %d", got)
	}
	if !hasEventOfKind(gs, "activate_mana_ability") {
		t.Fatal("expected activate_mana_ability event")
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("Bloom Tender should produce 2 mana; pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestMana605_SolRing_NoStack(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0

	// Artifact Sol Ring: {T}: Add {C}{C}. Verifies the artifact branch
	// shares the §605.3a inline-resolution contract with the creature
	// branch above.
	manaEff := &gameast.AddMana{Pool: []gameast.ManaSymbol{
		{Raw: "{C}", Generic: 0},
		{Raw: "{C}", Generic: 0},
	}}
	sol := makeArtifactPerm(gs, 0, "Sol Ring", manaEff, true)

	if !IsManaAbility(sol, 0) {
		t.Fatal("Sol Ring {T}: Add {C}{C} must classify as a mana ability")
	}

	preLog := len(gs.EventLog)
	if err := ActivateAbility(gs, 0, sol, 0, nil); err != nil {
		t.Fatalf("activation should succeed: %v", err)
	}

	if got := stackPushesSince(gs, preLog); got != 0 {
		t.Fatalf("Sol Ring mana ability must not push to stack; got %d", got)
	}
	if !hasEventOfKind(gs, "activate_mana_ability") {
		t.Fatal("expected activate_mana_ability event")
	}
	// AddMana.Pool with two {C} symbols yields 2 generic pips under the
	// MVP resolver (one per symbol). Same magnitude the dedicated Sol Ring
	// path produces via ApplyArtifactMana.
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("Sol Ring should produce 2 mana; pool=%d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// §605.3a — "can't be responded to."
//
// Operationalized as: opponents never receive priority during the
// activation. The engine opens priority only by way of PriorityRound,
// which the non-mana path calls after PushStackItem. We assert no
// priority-pass events and no stack_push events were emitted between
// pre-activation and post-activation log indices.
// ---------------------------------------------------------------------------

func TestMana605_ManaAbility_CannotBeRespondedTo(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0

	elves := makeManaCreature(gs, 0, "Llanowar Elves",
		[]gameast.ManaSymbol{{Raw: "{G}", Color: []string{"G"}}}, 0)

	preLog := len(gs.EventLog)
	if err := ActivateAbility(gs, 0, elves, 0, nil); err != nil {
		t.Fatalf("activation should succeed: %v", err)
	}

	for i := preLog; i < len(gs.EventLog); i++ {
		switch gs.EventLog[i].Kind {
		case "stack_push":
			t.Fatalf("mana ability must not push to stack (§605.3a); event %d", i)
		case "priority_pass", "priority_passed", "priority_opened":
			t.Fatalf("mana ability must not open a priority round (§605.3a); event %d kind=%s",
				i, gs.EventLog[i].Kind)
		}
	}
}

// ---------------------------------------------------------------------------
// §602.1b + §605.3a — mana produced is available during cost payment.
//
// The mana ability resolves inline as part of the same activation, so its
// pips are spendable on the immediately following cost — concretely, a
// player at 0 mana with two Llanowar Elves out can activate both and then
// pay a generic cost of 2.
// ---------------------------------------------------------------------------

func TestMana605_ManaProducedDuringCostPayment(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0

	e1 := makeManaCreature(gs, 0, "Llanowar Elves #1",
		[]gameast.ManaSymbol{{Raw: "{G}", Color: []string{"G"}}}, 0)
	e2 := makeManaCreature(gs, 0, "Llanowar Elves #2",
		[]gameast.ManaSymbol{{Raw: "{G}", Color: []string{"G"}}}, 0)

	if err := ActivateAbility(gs, 0, e1, 0, nil); err != nil {
		t.Fatalf("activate elves #1: %v", err)
	}
	if err := ActivateAbility(gs, 0, e2, 0, nil); err != nil {
		t.Fatalf("activate elves #2: %v", err)
	}

	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("after two Elves activations pool should be 2; got %d",
			gs.Seats[0].ManaPool)
	}

	// The pool is now spendable on a generic-2 cost — this is the
	// concrete "mana produced during cost payment" guarantee.
	if !PayGenericCost(nil, gs.Seats[0], 2, "generic", "test", "Cultivate") {
		t.Fatalf("two-Elves pool must pay generic-2; pool=%d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("pool should drain to 0 after paying 2; got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// §605.1b boundary — Priest of Urabrask's ETB add-mana trigger is NOT a
// mana ability. It triggers from an ETB event (not from an activated /
// triggered mana ability), so it goes on the stack like any other
// triggered ability and IS responseable.
// ---------------------------------------------------------------------------

func TestMana605_PriestOfUrabrask_TriggeredNotManaAbility(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0

	// Build a Priest of Urabrask permanent. The card itself doesn't need
	// a fully wired AST trigger — we exercise the boundary by directly
	// pushing the ETB-style trigger through the engine's triggered-ability
	// entry point. The §605.1b classifier is what we're testing: a
	// triggered "add mana" effect uses the stack.
	priest := &Permanent{
		Card: &Card{
			Name:  "Priest of Urabrask",
			Owner: 0,
			Types: []string{"creature"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, priest)

	// IsManaAbility only inspects activated abilities (§605.1a). The
	// Priest has no activated ability on its AST, so the classifier must
	// return false even for ability index 0.
	if IsManaAbility(priest, 0) {
		t.Fatal("Priest of Urabrask has no activated abilities; IsManaAbility must be false")
	}

	preLog := len(gs.EventLog)

	// "When Priest of Urabrask enters, add {R}{R}." Push as a triggered
	// ability via the §603 entry point.
	etbAddRR := &gameast.AddMana{Pool: []gameast.ManaSymbol{
		{Raw: "{R}", Color: []string{"R"}},
		{Raw: "{R}", Color: []string{"R"}},
	}}
	item := PushTriggeredAbility(gs, priest, etbAddRR)
	if item == nil {
		t.Fatal("PushTriggeredAbility returned nil")
	}

	// §605.1b: triggered ability — MUST have used the stack.
	if got := stackPushesSince(gs, preLog); got == 0 {
		t.Fatal("Priest ETB trigger must push to stack (not a §605.1b mana ability)")
	}

	// Contrast: the §605.3a-only event must NOT have fired for a
	// triggered ability.
	for i := preLog; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "activate_mana_ability" {
			t.Fatal("triggered add-mana must not log activate_mana_ability")
		}
	}

	// The trigger resolved inline after the priority round (no responders
	// in this fixture) and the {R}{R} reached the pool.
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("Priest ETB should add 2 mana on resolution; pool=%d",
			gs.Seats[0].ManaPool)
	}
}
