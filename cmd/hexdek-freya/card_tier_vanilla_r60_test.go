package main

import (
	"strings"
	"testing"
)

// card_tier_vanilla_r60_test.go — regressions for the r60 vanilla-cuttable
// signal added to computeCardQualityTiers. Pre-r60 the cuttable detector
// only fired on (CMC>=4 && len(roles)==1 && roles[0]==Utility), missing
// the classic vanilla case where the card has ZERO role tags because no
// role-detector fires on it (Hill Giant, Craw Wurm, niche spells with
// no synergy hooks). These cards still landed in the bottom-5 cuttable
// list via score=0 catch-all, but with a generic "low synergy" reason
// rather than the explicit "vanilla cut" framing deckbuilders need.
//
// New behavior: CMC >= 4 with len(roles)==0 AND not in any win-line /
// value-chain → -2.0 score with creature-aware "vanilla creature" or
// "no synergy" reasoning. The win-line / chain-piece override means a
// combo target that happens to lack role tags (e.g. a vanilla creature
// fetched by Birthing Pod) is NOT incorrectly flagged.

// -----------------------------------------------------------------------------
// Vanilla creature flagged with explicit reason
// -----------------------------------------------------------------------------

func TestVanillaCuttable_HillGiantFlaggedWithExplicitReason(t *testing.T) {
	profiles := []CardProfile{
		// Hill Giant — CMC 4, 3/3, no abilities. Classic vanilla cut.
		{Name: "Hill Giant", CMC: 4, TypeLine: "Creature — Giant"},
		// A real star to keep the score range realistic.
		{Name: "Star Card", CMC: 2, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Hill Giant", Roles: []RoleTag{}}, // zero roles — the key case
		{Name: "Star Card", Roles: []RoleTag{RoleRamp, RoleDraw, RoleRemoval}},
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	var hg *CardQuality
	for i, c := range dp.CuttableCards {
		if c.Name == "Hill Giant" {
			hg = &dp.CuttableCards[i]
			break
		}
	}
	if hg == nil {
		t.Fatalf("Hill Giant not in CuttableCards: %v", dp.CuttableCards)
	}
	if !strings.Contains(hg.Reason, "vanilla creature") {
		t.Errorf("Hill Giant reason should say 'vanilla creature'; got %q", hg.Reason)
	}
	if !strings.Contains(hg.Reason, "CMC 4") {
		t.Errorf("Hill Giant reason should name CMC; got %q", hg.Reason)
	}
}

// -----------------------------------------------------------------------------
// Vanilla noncreature gets the noncreature-flavored reason
// -----------------------------------------------------------------------------

func TestVanillaCuttable_NoncreatureFlagsAsNoSynergy(t *testing.T) {
	profiles := []CardProfile{
		// Hypothetical CMC-5 sorcery with no role tags (no removal, no
		// draw, no ramp, no combo — pure flavor/filler).
		{Name: "Niche Sorcery", CMC: 5, TypeLine: "Sorcery"},
		{Name: "Star Card", CMC: 2, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Niche Sorcery", Roles: []RoleTag{}},
		{Name: "Star Card", Roles: []RoleTag{RoleRamp, RoleDraw, RoleRemoval}},
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	var ns *CardQuality
	for i, c := range dp.CuttableCards {
		if c.Name == "Niche Sorcery" {
			ns = &dp.CuttableCards[i]
			break
		}
	}
	if ns == nil {
		t.Fatalf("Niche Sorcery not in CuttableCards: %v", dp.CuttableCards)
	}
	if strings.Contains(ns.Reason, "vanilla creature") {
		t.Errorf("noncreature should NOT say 'vanilla creature'; got %q", ns.Reason)
	}
	if !strings.Contains(ns.Reason, "no role tags") {
		t.Errorf("noncreature reason should say 'no role tags'; got %q", ns.Reason)
	}
}

// -----------------------------------------------------------------------------
// Win-line override — combo piece with no roles is NOT flagged
// -----------------------------------------------------------------------------

// A vanilla creature that's a target of a Birthing Pod chain is a
// combo piece — should NOT be flagged vanilla-cut even though it has
// no role tags. The win-line / chain-piece membership IS the role.
func TestVanillaCuttable_WinLinePieceOverridesPenalty(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Pod Target", CMC: 4, TypeLine: "Creature — Beast"},
		{Name: "Star Card", CMC: 2, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Pod Target", Roles: []RoleTag{}}, // zero roles
		{Name: "Star Card", Roles: []RoleTag{RoleRamp, RoleDraw, RoleRemoval}},
	}
	report := makeTierTestReport(profiles, assignments)
	// Wire Pod Target into a win line.
	report.WinLines = &WinLineAnalysis{
		WinLines: []WinLine{
			{Pieces: []string{"Pod Target", "Birthing Pod"}},
		},
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	// Pod Target should NOT be in the cuttable list with a vanilla reason —
	// win-line membership wins (it'd be a star with the +3.0 win-line
	// bonus, dwarfing the vanilla penalty).
	for _, c := range dp.CuttableCards {
		if c.Name == "Pod Target" {
			t.Errorf("Pod Target was in CuttableCards (%q) — win-line override should prevent this",
				c.Reason)
		}
	}
}

// Same override for value-chain bridge cards.
func TestVanillaCuttable_BridgeOverridesPenalty(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Bridge Card", CMC: 5, TypeLine: "Creature — Wizard"},
		{Name: "Star Card", CMC: 2, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Bridge Card", Roles: []RoleTag{}},
		{Name: "Star Card", Roles: []RoleTag{RoleRamp, RoleDraw, RoleRemoval}},
	}
	report := makeTierTestReport(profiles, assignments)
	report.ValueChains = []ValueChain{
		{
			BridgeCards: []string{"Bridge Card"},
		},
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	for _, c := range dp.CuttableCards {
		if c.Name == "Bridge Card" {
			t.Errorf("Bridge Card was in CuttableCards — value-chain bridge override should prevent this")
		}
	}
}

// -----------------------------------------------------------------------------
// CMC threshold — CMC 3 vanilla NOT flagged (below threshold)
// -----------------------------------------------------------------------------

func TestVanillaCuttable_BelowThresholdNotFlagged(t *testing.T) {
	profiles := []CardProfile{
		// CMC 3 vanilla — should NOT trigger the vanilla penalty.
		// There are real 3-mana cards with no role tags that earn their
		// slot via raw stats (Watchwolf, Tarmogoyf).
		{Name: "Watchwolf", CMC: 3, TypeLine: "Creature — Wolf"},
		// Star anchor + a real cuttable so the bottom-5 list has room.
		{Name: "Star Card", CMC: 2, TypeLine: "Instant"},
		{Name: "Filler 1", CMC: 5, TypeLine: "Creature"},
		{Name: "Filler 2", CMC: 5, TypeLine: "Creature"},
		{Name: "Filler 3", CMC: 5, TypeLine: "Creature"},
		{Name: "Filler 4", CMC: 5, TypeLine: "Creature"},
		{Name: "Filler 5", CMC: 5, TypeLine: "Creature"},
		{Name: "Filler 6", CMC: 5, TypeLine: "Creature"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Watchwolf", Roles: []RoleTag{}},
		{Name: "Star Card", Roles: []RoleTag{RoleRamp, RoleDraw, RoleRemoval}},
		{Name: "Filler 1", Roles: []RoleTag{}},
		{Name: "Filler 2", Roles: []RoleTag{}},
		{Name: "Filler 3", Roles: []RoleTag{}},
		{Name: "Filler 4", Roles: []RoleTag{}},
		{Name: "Filler 5", Roles: []RoleTag{}},
		{Name: "Filler 6", Roles: []RoleTag{}},
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	// Watchwolf has score = 0 (no roles, no CMC penalty since CMC=3 is
	// below threshold). The cuttable list takes bottom-5 with score<=0.
	// With six CMC-5 fillers all at -2.0, Watchwolf at 0.0 should sort
	// ABOVE all the fillers and stay OFF the cuttable list.
	for _, c := range dp.CuttableCards {
		if c.Name == "Watchwolf" {
			t.Errorf("CMC 3 vanilla 'Watchwolf' was in CuttableCards (%q) — "+
				"the CMC>=4 threshold should exempt it", c.Reason)
		}
	}
}

// -----------------------------------------------------------------------------
// Single-role defense — a CMC>=4 card with 1 role is NOT flagged vanilla
// -----------------------------------------------------------------------------

// The vanilla detector requires len(roles)==0; a card with even one role
// tag (other than Utility, which has its own -1.0 penalty path) shouldn't
// trip the vanilla penalty.
func TestVanillaCuttable_SingleNonUtilityRoleNotFlaggedVanilla(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Mid Threat", CMC: 5, TypeLine: "Creature — Beast"},
		{Name: "Star Card", CMC: 2, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Mid Threat", Roles: []RoleTag{RoleThreat}}, // single non-Utility role
		{Name: "Star Card", Roles: []RoleTag{RoleRamp, RoleDraw, RoleRemoval}},
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	// Mid Threat: score = 1.0 (one role) - 0 (Utility-only penalty doesn't
	// fire since role is Threat, not Utility) - 0 (vanilla penalty doesn't
	// fire since len(roles) > 0) = 1.0. Solid range.
	for _, c := range dp.CuttableCards {
		if c.Name == "Mid Threat" {
			t.Errorf("Single-role threat at CMC 5 should NOT be cuttable; got reason %q", c.Reason)
		}
	}
	// And the reason on its solid entry (if surfaced) must NOT contain
	// the vanilla wording.
	for _, c := range dp.SolidCards {
		if c.Name == "Mid Threat" && strings.Contains(c.Reason, "vanilla") {
			t.Errorf("Mid Threat reason includes 'vanilla' incorrectly; got %q", c.Reason)
		}
	}
}

// -----------------------------------------------------------------------------
// Cuttable detail fields are populated
// -----------------------------------------------------------------------------

// The new path populates s.detected / s.whyCut / s.effect — verify these
// flow through to CardQuality output fields. CardQuality has Reason but
// not these extra fields exposed; this test pins the Reason at minimum.
func TestVanillaCuttable_HasNamedCMCInReason(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Big Vanilla", CMC: 7, TypeLine: "Creature — Wurm"},
		{Name: "Star Card", CMC: 2, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Big Vanilla", Roles: []RoleTag{}},
		{Name: "Star Card", Roles: []RoleTag{RoleRamp, RoleDraw, RoleRemoval}},
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	for _, c := range dp.CuttableCards {
		if c.Name == "Big Vanilla" {
			if !strings.Contains(c.Reason, "CMC 7") {
				t.Errorf("Big Vanilla reason should name CMC 7; got %q", c.Reason)
			}
			return
		}
	}
	t.Errorf("Big Vanilla missing from CuttableCards: %v", dp.CuttableCards)
}
