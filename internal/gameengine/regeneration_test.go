package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// regeneration_test.go — regression pins for the regeneration
// replacement (CR §701.15). Before this, regeneration_shield was set but
// never consumed, so regeneration did nothing.

func regenCreature(gs *GameState, seat int) *Permanent {
	p := addTestPerm(gs, seat, "Regenerator", "creature")
	p.Card.BasePower = 2
	p.Card.BaseToughness = 2
	return p
}

func TestRegen_ShieldReplacesDestroy(t *testing.T) {
	gs := newTestGame(t, 2)
	p := regenCreature(gs, 0)
	GrantRegenerationShield(gs, p)

	if got := DestroyPermanent(gs, p, nil); got {
		t.Fatalf("DestroyPermanent returned true; regeneration should have replaced it")
	}
	// Still on the battlefield, tapped, shield consumed.
	if !onBattlefield(gs, p) {
		t.Errorf("creature should survive on the battlefield")
	}
	if !p.Tapped {
		t.Errorf("regenerated creature should be tapped")
	}
	if p.Flags["regeneration_shield"] != 0 {
		t.Errorf("shield should be consumed, got %d", p.Flags["regeneration_shield"])
	}
}

func TestRegen_NoShieldStillDestroys(t *testing.T) {
	gs := newTestGame(t, 2)
	p := regenCreature(gs, 0)
	if got := DestroyPermanent(gs, p, nil); !got {
		t.Fatalf("DestroyPermanent should return true with no shield (no regression)")
	}
	if onBattlefield(gs, p) {
		t.Errorf("creature without a shield must be destroyed")
	}
}

func TestRegen_ReplacesLethalCombatDamageSBA(t *testing.T) {
	gs := newTestGame(t, 2)
	p := regenCreature(gs, 0)
	GrantRegenerationShield(gs, p)
	p.MarkedDamage = 5 // lethal vs toughness 2

	StateBasedActions(gs)

	if !onBattlefield(gs, p) {
		t.Errorf("creature should survive lethal damage via regeneration")
	}
	if p.MarkedDamage != 0 {
		t.Errorf("regeneration should clear marked damage, got %d", p.MarkedDamage)
	}
	if !p.Tapped {
		t.Errorf("regenerated creature should be tapped")
	}
}

func TestRegen_DoesNotSaveToughnessZero(t *testing.T) {
	gs := newTestGame(t, 2)
	p := regenCreature(gs, 0)
	GrantRegenerationShield(gs, p)
	// -1/-1 counters drop toughness to 0 — §704.5f, NOT regen-replaceable.
	p.Counters["-1/-1"] = 2

	StateBasedActions(gs)

	if onBattlefield(gs, p) {
		t.Errorf("toughness≤0 (§704.5f) is not a destruction; regen must not save it")
	}
}

func TestRegen_TypedSelfGrantsShield(t *testing.T) {
	gs := newTestGame(t, 2)
	p := regenCreature(gs, 0)
	// regenerate_typed with a base="self" filter (the "{cost}: Regenerate
	// this creature" shape, 172/200 of the corpus).
	e := &gameast.ModificationEffect{
		ModKind: "regenerate_typed",
		Args:    []interface{}{map[string]interface{}{"base": "self"}},
	}
	resolveModificationEffect(gs, p, e)
	if p.Flags["regeneration_shield"] != 1 {
		t.Fatalf("regenerate_typed(self) should grant a shield, got %d", p.Flags["regeneration_shield"])
	}
	// And the shield actually works.
	if DestroyPermanent(gs, p, nil) {
		t.Errorf("granted shield should replace the destroy")
	}
}

func TestRegen_ShieldClearsAtEndOfTurn(t *testing.T) {
	gs := newTestGame(t, 2)
	p := regenCreature(gs, 0)
	GrantRegenerationShield(gs, p)
	ScanExpiredDurations(gs, "ending", "cleanup")
	if p.Flags["regeneration_shield"] != 0 {
		t.Errorf("unused regeneration shield should wear off at end of turn, got %d", p.Flags["regeneration_shield"])
	}
}
