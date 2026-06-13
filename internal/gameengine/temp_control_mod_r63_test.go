package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// temp_control_mod_r63_test.go — regression pins for the temp_control
// scaffold-KIND handler (hex-dev-3). The kind was previously inert (fell
// to resolveModificationEffect's default), so Act of Treason / Goatnap /
// Portent of Betrayal / … did nothing.

// A Threaten effect steals an opponent's creature: control moves to the
// caster, the creature is untapped + de-summoning-sick (haste), and the
// steal is registered for the end-of-turn revert.
func TestScaffoldKind_TempControl_StealsAndReverts(t *testing.T) {
	gs := newFixtureGame(t)
	// Caster = seat 0, casting "Act of Treason".
	caster := addBattlefield(gs, 0, "Act of Treason", 0, 0)

	// Opponent (seat 1) controls a tapped, summoning-sick beater.
	victim := addBattlefield(gs, 1, "Big Threat", 6, 6, "creature")
	victim.Owner = 1 // helper leaves Permanent.Owner zero-valued; set it to verify non-mutation
	victim.Tapped = true
	victim.SummoningSick = true

	eff := &gameast.ModificationEffect{ModKind: "temp_control"}
	ResolveEffect(gs, caster, eff)

	// Control moved to seat 0.
	if victim.Controller != 0 {
		t.Fatalf("temp_control: victim.Controller = %d, want 0 (stolen)", victim.Controller)
	}
	onZero := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			onZero = true
		}
	}
	if !onZero {
		t.Error("temp_control: victim should be on seat 0's battlefield")
	}
	for _, p := range gs.Seats[1].Battlefield {
		if p == victim {
			t.Error("temp_control: victim should no longer be on seat 1's battlefield")
		}
	}
	// Untapped + can attack this turn (haste).
	if victim.Tapped {
		t.Error("temp_control: victim should be untapped")
	}
	if victim.SummoningSick {
		t.Error("temp_control: victim should have haste (not summoning sick)")
	}
	// Ownership unchanged (CR §108.3).
	if victim.Owner != 1 {
		t.Errorf("temp_control: victim.Owner = %d, want 1 (ownership never changes)", victim.Owner)
	}
	// Registered for the end-of-turn revert.
	if len(gs.TempControlGrants) != 1 {
		t.Fatalf("temp_control: TempControlGrants = %d, want 1", len(gs.TempControlGrants))
	}

	// The cleanup-step revert returns it to its owner.
	ExpireTempControlGrants(gs)
	if victim.Controller != 1 {
		t.Errorf("temp_control: after EOT revert, Controller = %d, want 1", victim.Controller)
	}
}

// No opponent creature → no-op (no panic, no spurious steal).
func TestScaffoldKind_TempControl_NoTargetNoOp(t *testing.T) {
	gs := newFixtureGame(t)
	caster := addBattlefield(gs, 0, "Act of Treason", 0, 0)

	eff := &gameast.ModificationEffect{ModKind: "temp_control"}
	ResolveEffect(gs, caster, eff)

	if len(gs.TempControlGrants) != 0 {
		t.Errorf("temp_control: no opponent creature should be a no-op, got %d grants", len(gs.TempControlGrants))
	}
}

// Picks the highest-power opponent creature.
func TestScaffoldKind_TempControl_PicksHighestPower(t *testing.T) {
	gs := newFixtureGame(t)
	caster := addBattlefield(gs, 0, "Act of Treason", 0, 0)
	small := addBattlefield(gs, 1, "Small", 1, 1, "creature")
	big := addBattlefield(gs, 1, "Big", 7, 7, "creature")
	_ = small

	eff := &gameast.ModificationEffect{ModKind: "temp_control"}
	ResolveEffect(gs, caster, eff)

	if big.Controller != 0 {
		t.Errorf("temp_control: should steal the highest-power creature (Big), Big.Controller=%d", big.Controller)
	}
	if small.Controller != 1 {
		t.Errorf("temp_control: should NOT steal the small creature, Small.Controller=%d", small.Controller)
	}
}
