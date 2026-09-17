package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// regeneration_scope_r63_test.go — scope-boundary pins for the
// regeneration replacement (CR §701.15). Regeneration replaces
// DESTRUCTION only. These tests pin that it does NOT save against exile
// or sacrifice, that a single shield is consumed exactly once (survive
// the first destroy, die to the second), and that the
// `regenerate_target_typed` AST kind routes to the shield-setter.
//
// Reuses the rg_* helpers from regenerate_cant_regen_r63_test.go
// (rg_game / rg_creature / rg_onBattlefield / rg_inGraveyard).

// Scope: EXILE is not a destruction — a regen shield must NOT save the
// permanent and the shield must NOT be consumed (TryRegenerate never
// fires on the exile path).
func TestRegen_ExileNotReplacedByShield(t *testing.T) {
	gs := rg_game()
	c := rg_creature(gs, 0, "Drudge Skeletons", 1, 1)
	GrantRegenerationShield(gs, c)

	if !ExilePermanent(gs, c, nil) {
		t.Fatal("ExilePermanent must remove the creature despite a regen shield (return true)")
	}
	if rg_onBattlefield(gs, c) {
		t.Error("exiled creature must not stay on the battlefield; regen does not replace exile")
	}
	if c.Flags["regeneration_shield"] != 1 {
		t.Errorf("the shield must be untouched by an exile (regen never fired), got %d",
			c.Flags["regeneration_shield"])
	}
}

// Scope: SACRIFICE is not a destruction (CR §701.17 ignores even
// indestructible) — a regen shield must NOT save it and must NOT be
// consumed.
func TestRegen_SacrificeNotReplacedByShield(t *testing.T) {
	gs := rg_game()
	c := rg_creature(gs, 0, "Drudge Skeletons", 1, 1)
	GrantRegenerationShield(gs, c)

	SacrificePermanent(gs, c, "test_sacrifice")

	if rg_onBattlefield(gs, c) {
		t.Error("sacrificed creature must not stay on the battlefield; regen does not replace sacrifice")
	}
	if !rg_inGraveyard(gs, 0, "Drudge Skeletons") {
		t.Error("sacrificed creature must be in the graveyard")
	}
	if c.Flags["regeneration_shield"] != 1 {
		t.Errorf("the shield must be untouched by a sacrifice (regen never fired), got %d",
			c.Flags["regeneration_shield"])
	}
}

// Consumed exactly once: a single shield replaces the FIRST destroy this
// turn; a SECOND destroy the same turn finds no shield and kills.
func TestRegen_SingleShieldConsumedOnce(t *testing.T) {
	gs := rg_game()
	c := rg_creature(gs, 0, "Drudge Skeletons", 1, 1)
	GrantRegenerationShield(gs, c)

	// First destroy: replaced.
	if DestroyPermanent(gs, c, nil) {
		t.Fatal("first destroy must be replaced by the single shield (return false)")
	}
	if !rg_onBattlefield(gs, c) {
		t.Error("creature must survive the first destroy")
	}
	if c.Flags["regeneration_shield"] != 0 {
		t.Errorf("the one shield must be fully consumed, got %d", c.Flags["regeneration_shield"])
	}

	// Second destroy the same turn: no shield left → dies.
	if !DestroyPermanent(gs, c, nil) {
		t.Fatal("second destroy with no shield left must kill the creature (return true)")
	}
	if rg_onBattlefield(gs, c) {
		t.Error("creature must be destroyed by the second destroy")
	}
	if !rg_inGraveyard(gs, 0, "Drudge Skeletons") {
		t.Error("creature must be in the graveyard after the second destroy")
	}
}

// Routing: the `regenerate_target_typed` AST kind (the lone "regenerate
// target creature" corpus card) must reach GrantRegenerationShield, same
// as `regenerate_typed`.
func TestRegen_TargetTypedGrantsShield(t *testing.T) {
	gs := rg_game()
	c := rg_creature(gs, 0, "Regen Target", 3, 3)
	e := &gameast.ModificationEffect{
		ModKind: "regenerate_target_typed",
		Args:    []interface{}{map[string]interface{}{"base": "creature"}},
	}
	resolveModificationEffect(gs, c, e)
	if c.Flags["regeneration_shield"] != 1 {
		t.Fatalf("regenerate_target_typed should grant a shield, got %d", c.Flags["regeneration_shield"])
	}
	// And the granted shield actually replaces a destroy.
	if DestroyPermanent(gs, c, nil) {
		t.Error("granted shield from regenerate_target_typed should replace the destroy")
	}
}
