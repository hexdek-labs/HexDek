package main

import (
	"sort"
)

// roleSynergyThemes maps each RoleTag to the commander-theme strings
// (from commanderThemePatterns in deckprofile.go) whose presence
// promotes that role in the multi-tag priority sort. The lookup is
// used by RefineRolesByCommanderThemes to break ties (equal
// rolePriority values) by preferring the role whose strategic
// purpose aligns with what the commander's oracle text rewards.
//
// Keep the mapping conservative — each entry is a deliberate "this
// theme makes this role's strategic value visibly higher" claim, not
// a generic "any creature theme touches RoleThreat" stretch. Roles
// missing from the map (Combo, Stax, SuspendFinisher, Counterspell,
// Tutor, PseudoTutor, Removal, BoardWipe, Utility, Land) have no
// theme-driven alignment — their priority is structural, not
// commander-conditional.
var roleSynergyThemes = map[RoleTag][]string{
	// Draw and Ramp are the canonical tied pair (both at rolePriority
	// 11). A "drawing" commander makes the deck's draw engine the
	// load-bearing piece; a "lands" commander makes ramp the
	// strategic identity.
	RoleDraw: {"drawing", "spellcasting"},
	RoleRamp: {"lands"},

	// Recursion sits at priority 9 alone today, but adding the theme
	// alignment future-proofs the case where a priority-9 sibling
	// gets introduced — a "graveyard" commander wants Recursion to
	// lead any tie it lands in.
	RoleRecursion: {"graveyard"},

	// Threat sits at priority 3. A "combat" / "tokens" / "tribal"
	// commander promotes Threat in any future tie.
	RoleThreat: {"combat", "tokens", "tribal"},

	// Protection at priority 10 aligns with "lifegain" commanders —
	// Soul Warden / Karlov / Oloro shells treat their lifegain
	// engines as the protective shell that lets the slow plan close.
	RoleProtection: {"lifegain"},
}

// roleAlignsWithCommanderTheme reports whether the given role has at
// least one theme in roleSynergyThemes that also appears in the
// deck's commander themes. Used as the tie-break key in
// RefineRolesByCommanderThemes.
func roleAlignsWithCommanderTheme(role RoleTag, themes []string) bool {
	roleThemes, ok := roleSynergyThemes[role]
	if !ok {
		return false
	}
	themeSet := make(map[string]bool, len(themes))
	for _, t := range themes {
		themeSet[t] = true
	}
	for _, rt := range roleThemes {
		if themeSet[rt] {
			return true
		}
	}
	return false
}

// RefineRolesByCommanderThemes re-sorts each Assignment's Roles list
// using commander-theme alignment as a tie-breaker for roles that
// share a rolePriority value. The primary sort key is still the
// static rolePriority (so Combo / Stax / Threat never drop below
// Draw / Ramp regardless of theme); ties are broken by:
//
//   1. commander-theme alignment (the role mapped in
//      roleSynergyThemes whose themes intersect `themes` wins),
//   2. roleTieBreakOrder fallback (deterministic when neither side
//      aligns),
//   3. alphabetical on the RoleTag string (last resort).
//
// No-op when themes is empty (no signal to apply) or when ra is
// nil (defensive for the pre-build paths). Mutates each
// Assignment.Roles slice in place — callers that hold the report's
// Roles map see the new ordering on next read.
//
// Wired from BuildDeckProfile AFTER computeCommanderSynergy
// populates dp.CommanderThemes, so the strategy.json card_roles map
// (which reads Assignment.Roles[0] as the primary tag) reflects the
// commander-aware ordering.
func RefineRolesByCommanderThemes(ra *RoleAnalysis, themes []string) {
	if ra == nil || len(themes) == 0 {
		return
	}
	for i := range ra.Assignments {
		roles := ra.Assignments[i].Roles
		if len(roles) < 2 {
			continue
		}
		sort.SliceStable(roles, func(a, b int) bool {
			pa, pb := rolePriority[roles[a]], rolePriority[roles[b]]
			if pa != pb {
				return pa < pb
			}
			alignsA := roleAlignsWithCommanderTheme(roles[a], themes)
			alignsB := roleAlignsWithCommanderTheme(roles[b], themes)
			if alignsA != alignsB {
				return alignsA
			}
			ta, taOk := roleTieBreakOrder[roles[a]]
			tb, tbOk := roleTieBreakOrder[roles[b]]
			if taOk && tbOk {
				return ta < tb
			}
			if taOk {
				return true
			}
			if tbOk {
				return false
			}
			return string(roles[a]) < string(roles[b])
		})
	}
}
