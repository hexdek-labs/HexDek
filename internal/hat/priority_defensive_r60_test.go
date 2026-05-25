package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// priority_defensive_r60_test.go — regressions for the R60 defensive
// signals on YggdrasilHat.ChooseResponse. Companion to the offensive
// signals in priority_round_r60_test.go.
//
// Pre-R60 the response path was counter-only: if the defending seat was
// being swung at for lethal, holding a Fog in hand did nothing — the
// hat had no way to play it during the opponent-priority window between
// declare-attackers and combat damage. Now the hat scans for fog /
// mass-protection answers BEFORE the counter fast-path so the saved
// life is the strictly-better trade.

// triggerOnStack returns a benign trigger-shaped StackItem the
// ChooseResponse priority window opens around. The contents don't
// matter — the priority window does.
func triggerOnStack(controller int) *gameengine.StackItem {
	return &gameengine.StackItem{
		Controller: controller,
		Card:       newTestCardMinimal("Some Attack Trigger", []string{"creature"}, 2, nil),
		Effect:     &gameast.Damage{},
	}
}

// attackingPerm puts an attacker on attackerSeat's battlefield targeting
// defenderSeat with the given power. Marks the attacking + defender
// flags the helpers read.
func attackingPerm(t *testing.T, gs *gameengine.GameState, attackerSeat, defenderSeat, power int) *gameengine.Permanent {
	t.Helper()
	card := newTestCardMinimal("Bear", []string{"creature"}, 2, nil)
	p := newTestPermanent(gs.Seats[attackerSeat], card, power, power)
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags["attacking"] = 1
	gameengine.SetAttackerDefender(p, defenderSeat)
	return p
}

// fogCard returns an instant-speed Fog (prevent all combat damage).
func fogCard() *gameengine.Card {
	return newTestCardMinimal("Fog", []string{"instant"}, 1,
		&gameast.CardAST{
			Name: "Fog",
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Prevent all combat damage that would be dealt this turn."},
			},
		})
}

// heroicIntervention returns a mass-protection instant (creatures you
// control gain hexproof and indestructible until end of turn).
func heroicIntervention() *gameengine.Card {
	return newTestCardMinimal("Heroic Intervention", []string{"instant"}, 2,
		&gameast.CardAST{
			Name: "Heroic Intervention",
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Permanents you control gain hexproof and indestructible until end of turn."},
			},
		})
}

// setupDefender wires a YggdrasilHat onto seat 0 with the given hand,
// mana to cast anything in it, and a benign trigger on the stack from
// seat 1 so the priority window opens. Returns hat + state + stack top.
func setupDefender(t *testing.T, hand []*gameengine.Card, life int) (*YggdrasilHat, *gameengine.GameState, *gameengine.StackItem) {
	t.Helper()
	gs := newTestGame(t, 2)
	gs.Phase = "combat"
	gs.Step = "declare_attackers"
	gs.Seats[0].Hand = hand
	gs.Seats[0].ManaPool = 10
	gs.Seats[0].Life = life
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	return h, gs, triggerOnStack(1)
}

// -----------------------------------------------------------------------------
// Lethal-combat fog response
// -----------------------------------------------------------------------------

func TestChooseResponse_R60_CastsFogOnLethalIncoming(t *testing.T) {
	h, gs, top := setupDefender(t, []*gameengine.Card{fogCard()}, 5)
	attackingPerm(t, gs, 1, 0, 7) // seat 1 swings 7 at seat 0 (5 life → lethal)

	got := h.ChooseResponse(gs, 0, top)
	if got == nil {
		t.Fatal("hat should cast Fog when incoming combat damage is lethal")
	}
	if got.Card.DisplayName() != "Fog" {
		t.Fatalf("expected Fog response, got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_CastsMassProtectionOnLethalIncoming(t *testing.T) {
	h, gs, top := setupDefender(t, []*gameengine.Card{heroicIntervention()}, 4)
	attackingPerm(t, gs, 1, 0, 5)

	got := h.ChooseResponse(gs, 0, top)
	if got == nil {
		t.Fatal("hat should cast Heroic Intervention when incoming is lethal")
	}
	if got.Card.DisplayName() != "Heroic Intervention" {
		t.Fatalf("expected Heroic Intervention, got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_PrefersFogOverCounter(t *testing.T) {
	// Holding both a Counterspell and a Fog; lethal incoming should
	// pick the Fog because a counter doesn't undo declared attackers.
	h, gs, top := setupDefender(t, []*gameengine.Card{counterInHand(), fogCard()}, 6)
	attackingPerm(t, gs, 1, 0, 8)

	got := h.ChooseResponse(gs, 0, top)
	if got == nil {
		t.Fatal("hat should respond when lethal")
	}
	if got.Card.DisplayName() != "Fog" {
		t.Fatalf("expected Fog over Counterspell, got %v", got.Card.DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Negative cases — fog should NOT fire
// -----------------------------------------------------------------------------

func TestChooseResponse_R60_NoFogWhenSurvivable(t *testing.T) {
	// 5 power swinging into 20 life — not lethal, hold the Fog.
	h, gs, top := setupDefender(t, []*gameengine.Card{fogCard()}, 20)
	attackingPerm(t, gs, 1, 0, 5)

	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("hat should hold Fog when not lethal; got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_NoFogOutsideCombat(t *testing.T) {
	h, gs, top := setupDefender(t, []*gameengine.Card{fogCard()}, 5)
	gs.Phase = "main2"
	gs.Step = ""
	attackingPerm(t, gs, 1, 0, 7) // doesn't matter — phase guard fires first

	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("Fog must only fire in combat phase; got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_NoFogPostBlockers(t *testing.T) {
	// Blockers were already declared — defer to the blocker math, don't
	// double-count by also casting a fog.
	h, gs, top := setupDefender(t, []*gameengine.Card{fogCard()}, 5)
	gs.Step = "declare_blockers"
	attackingPerm(t, gs, 1, 0, 7)

	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("Fog must not fire after declare_blockers; got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_NoFogWhenUnaffordable(t *testing.T) {
	h, gs, top := setupDefender(t, []*gameengine.Card{fogCard()}, 5)
	gs.Seats[0].ManaPool = 0
	attackingPerm(t, gs, 1, 0, 7)

	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("Fog must not fire when unaffordable; got %v", got.Card.DisplayName())
	}
}

// -----------------------------------------------------------------------------
// isDefensiveInstantSpell unit cases
// -----------------------------------------------------------------------------

func TestIsDefensiveInstantSpell_Positive(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
	}{
		{"Fog", "Prevent all combat damage that would be dealt this turn."},
		{"Holy Day", "Prevent all combat damage that would be dealt this turn."},
		{"Moment's Peace", "Prevent all combat damage that would be dealt this turn. Flashback {1}{G}."},
		{"Heroic Intervention", "Permanents you control gain hexproof and indestructible until end of turn."},
		{"Boros Charm-like", "Creatures you control gain indestructible until end of turn."},
		{"Teferi's Protection",
			"Until your next turn, your life total can't change and you have protection from everything. All permanents you control phase out."},
	}
	for _, c := range cases {
		card := spellWithOracle(c.name, 2, c.oracle)
		if !isDefensiveInstantSpell(card) {
			t.Errorf("%q should be defensive (oracle=%q)", c.name, c.oracle)
		}
	}
}

func TestIsDefensiveInstantSpell_Negative(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
	}{
		{"Lightning Bolt", "Lightning Bolt deals 3 damage to any target."},
		{"Giant Growth single target", "Target creature gets +3/+3 until end of turn."},
		{"Counterspell", "Counter target spell."},
		{"Indestructible aura", "Enchanted creature has indestructible."}, // permanent grant, not turn grant
		{"Empty", ""},
	}
	for _, c := range cases {
		card := spellWithOracle(c.name, 2, c.oracle)
		if isDefensiveInstantSpell(card) {
			t.Errorf("%q should NOT be defensive (oracle=%q)", c.name, c.oracle)
		}
	}
}

func TestIsDefensiveInstantSpell_NilSafe(t *testing.T) {
	if isDefensiveInstantSpell(nil) {
		t.Fatal("nil card must not classify as defensive")
	}
}

// -----------------------------------------------------------------------------
// unblockedCombatDamageTo unit cases
// -----------------------------------------------------------------------------

func TestUnblockedCombatDamageTo_SumsAttackerPower(t *testing.T) {
	gs := newTestGame(t, 3)
	attackingPerm(t, gs, 1, 0, 3) // seat1 → seat0: 3
	attackingPerm(t, gs, 1, 0, 2) // seat1 → seat0: 2
	attackingPerm(t, gs, 2, 0, 4) // seat2 → seat0: 4
	attackingPerm(t, gs, 2, 1, 5) // seat2 → seat1: irrelevant for seat0

	if got := unblockedCombatDamageTo(gs, 0); got != 9 {
		t.Errorf("expected 9 incoming for seat 0, got %d", got)
	}
	if got := unblockedCombatDamageTo(gs, 1); got != 5 {
		t.Errorf("expected 5 incoming for seat 1, got %d", got)
	}
	if got := unblockedCombatDamageTo(gs, 2); got != 0 {
		t.Errorf("expected 0 incoming for seat 2, got %d", got)
	}
}

func TestUnblockedCombatDamageTo_DoubleStrikeCountsTwice(t *testing.T) {
	gs := newTestGame(t, 2)
	p := attackingPerm(t, gs, 1, 0, 3)
	p.Flags["kw:double_strike"] = 1
	if got := unblockedCombatDamageTo(gs, 0); got != 6 {
		t.Errorf("double-strike 3-power should count as 6, got %d", got)
	}
}

func TestUnblockedCombatDamageTo_NilSafe(t *testing.T) {
	if got := unblockedCombatDamageTo(nil, 0); got != 0 {
		t.Errorf("nil gs should return 0, got %d", got)
	}
}
