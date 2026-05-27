package main

import (
	"testing"
)

// rolePriority must put Draw and Ramp at the same value (the
// intentional tie that motivates the tie-breaker). Pins the design
// — a future edit that gives Ramp a different priority breaks this
// test and forces the author to either justify the change or remove
// the tie-breaker mechanism.
func TestRolePriority_DrawAndRampTied(t *testing.T) {
	if rolePriority[RoleDraw] != rolePriority[RoleRamp] {
		t.Errorf("RoleDraw and RoleRamp must share priority (r60 design); got Draw=%d Ramp=%d",
			rolePriority[RoleDraw], rolePriority[RoleRamp])
	}
}

// roleAlignsWithCommanderTheme: positive cases (role mapped + theme
// present) and negative cases (role not mapped / theme absent /
// role mapped but unrelated themes).
func TestRoleAlignsWithCommanderTheme_PositiveAndNegative(t *testing.T) {
	cases := []struct {
		role   RoleTag
		themes []string
		want   bool
		desc   string
	}{
		{RoleDraw, []string{"drawing"}, true, "drawing commander → Draw aligned"},
		{RoleDraw, []string{"spellcasting"}, true, "spellcasting commander → Draw aligned (cantrip storm)"},
		{RoleRamp, []string{"lands"}, true, "lands commander → Ramp aligned"},
		{RoleRamp, []string{"drawing"}, false, "drawing commander → Ramp NOT aligned"},
		{RoleDraw, []string{"lands"}, false, "lands commander → Draw NOT aligned"},
		{RoleRecursion, []string{"graveyard"}, true, "graveyard commander → Recursion aligned"},
		{RoleThreat, []string{"combat", "tokens"}, true, "combat+tokens commander → Threat aligned"},
		{RoleProtection, []string{"lifegain"}, true, "lifegain commander → Protection aligned"},
		// Roles not in roleSynergyThemes — should always return false.
		{RoleCombo, []string{"drawing"}, false, "RoleCombo has no theme mapping"},
		{RoleStax, []string{"sacrifice"}, false, "RoleStax has no theme mapping"},
		// Empty themes never align.
		{RoleDraw, []string{}, false, "empty themes → no alignment"},
		{RoleDraw, nil, false, "nil themes → no alignment"},
	}
	for _, tc := range cases {
		if got := roleAlignsWithCommanderTheme(tc.role, tc.themes); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.desc, got, tc.want)
		}
	}
}

// TagCardRole with a dual Draw+Ramp card (Produces both ResMana and
// ResCard — Selvala Heart of the Wilds shape) must default to
// "Draw first" via roleTieBreakOrder when no commander themes are
// known. Preserves pre-r60 behavior.
func TestTagCardRole_DualDrawRamp_DefaultDrawFirst(t *testing.T) {
	p := CardProfile{
		Name:     "Selvala, Heart of the Wilds",
		TypeLine: "Legendary Creature — Elf Scout",
		CMC:      3,
		Produces: []ResourceType{ResCard, ResMana},
	}
	roles := TagCardRole(p.Name, "whenever another creature with power 4 or greater enters, you may draw a card. add g.",
		p.TypeLine, "", p.CMC, p)
	if len(roles) < 2 {
		t.Fatalf("expected dual-role tagging, got %v", roles)
	}
	if roles[0] != RoleDraw || roles[1] != RoleRamp {
		t.Errorf("default tie-break must put Draw before Ramp; got %v", roles)
	}
}

// RefineRolesByCommanderThemes flips a Draw+Ramp card's primary role
// to Ramp when the commander themes include "lands". Mirrors the
// Karametra / Lord Windgrace / Aesi shell intuition: a lands-matter
// commander wants its Selvala-shaped value piece tagged primarily
// as Ramp.
func TestRefineRoles_LandsCommanderPromotesRamp(t *testing.T) {
	ra := &RoleAnalysis{
		Assignments: []CardRoleAssignment{
			{Name: "Selvala, Heart of the Wilds", Roles: []RoleTag{RoleDraw, RoleRamp}},
		},
	}
	RefineRolesByCommanderThemes(ra, []string{"lands"})
	if ra.Assignments[0].Roles[0] != RoleRamp {
		t.Errorf("lands commander should promote Ramp to primary; got %v", ra.Assignments[0].Roles)
	}
}

// Symmetry: a "drawing" commander keeps Draw as primary (and pins
// that the lands path is not over-firing — a draw-themed commander
// must NOT accidentally promote Ramp).
func TestRefineRoles_DrawingCommanderKeepsDrawPrimary(t *testing.T) {
	ra := &RoleAnalysis{
		Assignments: []CardRoleAssignment{
			{Name: "Selvala, Heart of the Wilds", Roles: []RoleTag{RoleRamp, RoleDraw}},
		},
	}
	RefineRolesByCommanderThemes(ra, []string{"drawing"})
	if ra.Assignments[0].Roles[0] != RoleDraw {
		t.Errorf("drawing commander should promote Draw to primary; got %v", ra.Assignments[0].Roles)
	}
}

// Tie-breaker must NOT cross priority bands. A Threat+Ramp card
// (e.g. a creature that ramps on attack with commander "lands"
// theme) — Ramp ALIGNS but Threat has a much stronger static
// priority (3 vs 11), so Threat MUST stay primary. Defends against
// over-eager promotion that would let the synergy boost swap across
// the priority structure.
func TestRefineRoles_NeverCrossesPriorityBands(t *testing.T) {
	ra := &RoleAnalysis{
		Assignments: []CardRoleAssignment{
			{Name: "Test", Roles: []RoleTag{RoleThreat, RoleRamp}},
		},
	}
	RefineRolesByCommanderThemes(ra, []string{"lands"})
	if ra.Assignments[0].Roles[0] != RoleThreat {
		t.Errorf("Threat (priority 3) must stay primary regardless of theme alignment on Ramp (priority 11); got %v",
			ra.Assignments[0].Roles)
	}
}

// Empty themes is a no-op — the function must not reorder
// Assignments when there's no commander-theme signal. Defends
// against the case where dp.CommanderThemes ends up empty (no
// commander or no theme matches in commander oracle text).
func TestRefineRoles_EmptyThemesNoOp(t *testing.T) {
	ra := &RoleAnalysis{
		Assignments: []CardRoleAssignment{
			{Name: "Selvala, Heart of the Wilds", Roles: []RoleTag{RoleDraw, RoleRamp}},
		},
	}
	RefineRolesByCommanderThemes(ra, nil)
	if ra.Assignments[0].Roles[0] != RoleDraw {
		t.Errorf("empty themes must leave order untouched; got %v", ra.Assignments[0].Roles)
	}
	RefineRolesByCommanderThemes(ra, []string{})
	if ra.Assignments[0].Roles[0] != RoleDraw {
		t.Errorf("empty themes (slice form) must leave order untouched; got %v", ra.Assignments[0].Roles)
	}
}

// nil receiver safety — BuildDeckProfile calls RefineRoles before
// some pipelines guarantee report.Roles is non-nil. Defensive nil
// check is a tested contract.
func TestRefineRoles_NilReceiverSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RefineRolesByCommanderThemes panicked on nil ra: %v", r)
		}
	}()
	RefineRolesByCommanderThemes(nil, []string{"drawing"})
}

// Single-role assignments are unchanged — the function only touches
// multi-role lists, so a pure-ramp Cultivate or pure-draw Harmonize
// must come through untouched (defends against accidental
// stable-sort weirdness on len-1 slices).
func TestRefineRoles_SingleRoleAssignmentsUntouched(t *testing.T) {
	ra := &RoleAnalysis{
		Assignments: []CardRoleAssignment{
			{Name: "Cultivate", Roles: []RoleTag{RoleRamp}},
			{Name: "Harmonize", Roles: []RoleTag{RoleDraw}},
		},
	}
	RefineRolesByCommanderThemes(ra, []string{"lands"})
	if ra.Assignments[0].Roles[0] != RoleRamp {
		t.Errorf("Cultivate single-role should stay RoleRamp; got %v", ra.Assignments[0].Roles)
	}
	if ra.Assignments[1].Roles[0] != RoleDraw {
		t.Errorf("Harmonize single-role should stay RoleDraw; got %v", ra.Assignments[1].Roles)
	}
}

// End-to-end: a card with both Draw and Ramp, when neither role
// aligns with the commander's themes, falls back to roleTieBreakOrder
// (Draw first). Pins the deterministic fallback.
func TestRefineRoles_UnrelatedThemesFallBackToDefault(t *testing.T) {
	ra := &RoleAnalysis{
		Assignments: []CardRoleAssignment{
			{Name: "Selvala, Heart of the Wilds", Roles: []RoleTag{RoleRamp, RoleDraw}},
		},
	}
	// Sacrifice theme aligns with neither Draw nor Ramp.
	RefineRolesByCommanderThemes(ra, []string{"sacrifice"})
	if ra.Assignments[0].Roles[0] != RoleDraw {
		t.Errorf("unrelated themes must fall back to roleTieBreakOrder (Draw first); got %v",
			ra.Assignments[0].Roles)
	}
}
