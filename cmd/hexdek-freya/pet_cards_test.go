package main

import (
	"fmt"
	"testing"
)

// petCardSetup builds a minimal report+profile fixture for pet-card
// tests with the deck's archetype set (so the off-archetype fingerprint
// check has something to compare against) and a CardPowerLevels list
// already populated (since computePetCards reads from that as its
// candidate set).
func petCardSetup(archetype string, cards []CardProfile, assignments []CardRoleAssignment) (*DeckProfile, *FreyaReport) {
	report := &FreyaReport{
		Profiles: cards,
		Roles: &RoleAnalysis{
			Assignments: assignments,
			TotalCards:  len(cards),
		},
	}
	dp := &DeckProfile{PrimaryArchetype: archetype}
	computeCardPower(dp, report)
	return dp, report
}

// TestComputePetCards_OffArchetypeCreatureFlagged verifies the canonical
// case: a low-tier creature whose role doesn't match the deck's
// fingerprint surfaces as a pet card.
func TestComputePetCards_OffArchetypeCreatureFlagged(t *testing.T) {
	// Combo deck fingerprint includes Combo/Tutor/Draw/Ramp. A creature
	// whose only role is Threat is off-archetype.
	cards := []CardProfile{
		{Name: "Off Threat", CMC: 4, TypeLine: "Creature — Beast"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Off Threat", Roles: []RoleTag{RoleThreat}},
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	computePetCards(dp, report)

	if len(dp.PetCards) != 1 {
		t.Fatalf("want 1 pet card, got %d: %+v", len(dp.PetCards), dp.PetCards)
	}
	if dp.PetCards[0].Name != "Off Threat" {
		t.Errorf("wrong pet card: %s", dp.PetCards[0].Name)
	}
}

// TestComputePetCards_OnArchetypeCreatureNotFlagged verifies a creature
// whose role IS in the deck's fingerprint is NOT a pet card — it's
// playing the role the deck wants.
func TestComputePetCards_OnArchetypeCreatureNotFlagged(t *testing.T) {
	// Combo fingerprint includes RoleTutor at 0.10 — a CMC-3 Creature
	// tutor scores low but is on-archetype.
	cards := []CardProfile{
		{Name: "Aligned Creature", CMC: 3, TypeLine: "Creature — Wizard"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Aligned Creature", Roles: []RoleTag{RoleTutor}},
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	computePetCards(dp, report)

	if len(dp.PetCards) != 0 {
		t.Errorf("on-archetype creature should NOT be a pet card, got: %+v", dp.PetCards)
	}
}

// TestComputePetCards_NonCreatureNotFlagged verifies a low-tier off-
// archetype noncreature (sorcery, artifact, etc.) is NOT a pet card —
// the heuristic is "creatures only" because that's where players form
// flavor attachments. A bad spell is just a bad spell.
func TestComputePetCards_NonCreatureNotFlagged(t *testing.T) {
	cards := []CardProfile{
		{Name: "Off Sorcery", CMC: 4, TypeLine: "Sorcery"},
		{Name: "Off Artifact", CMC: 4, TypeLine: "Artifact"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Off Sorcery", Roles: []RoleTag{RoleThreat}},
		{Name: "Off Artifact", Roles: []RoleTag{RoleThreat}},
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	computePetCards(dp, report)

	if len(dp.PetCards) != 0 {
		t.Errorf("noncreature should NOT be a pet card, got: %+v", dp.PetCards)
	}
}

// TestComputePetCards_HighTierCreatureNotFlagged verifies an S/A/B
// tier creature is NOT a pet card — pet cards are only for low-tier
// cards the builder kept despite the score.
func TestComputePetCards_HighTierCreatureNotFlagged(t *testing.T) {
	// Many roles + low CMC will score high power.
	cards := []CardProfile{
		{Name: "Strong Creature", CMC: 1, TypeLine: "Creature — Wizard"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Strong Creature", Roles: []RoleTag{RoleThreat, RoleRemoval, RoleDraw, RoleRamp}},
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	if dp.CardPowerLevels[0].PowerTier == "C" || dp.CardPowerLevels[0].PowerTier == "D" {
		t.Fatalf("test fixture expected high tier, got %s (power %d)",
			dp.CardPowerLevels[0].PowerTier, dp.CardPowerLevels[0].Power)
	}
	computePetCards(dp, report)
	if len(dp.PetCards) != 0 {
		t.Errorf("high-tier creature should NOT be pet card, got: %+v", dp.PetCards)
	}
}

// TestComputePetCards_UntaggedCreatureNotFlagged verifies a creature
// with NO role tags is NOT a pet card — pure-filler / untagged cards
// signal "builder didn't realize this was bad", not "builder loves it".
// The pet-card category needs the role tag as evidence of intent.
func TestComputePetCards_UntaggedCreatureNotFlagged(t *testing.T) {
	cards := []CardProfile{
		{Name: "Untagged Creature", CMC: 4, TypeLine: "Creature — Goblin"},
	}
	dp, report := petCardSetup("Combo", cards, nil)
	computePetCards(dp, report)
	if len(dp.PetCards) != 0 {
		t.Errorf("untagged creature should NOT be pet card, got: %+v", dp.PetCards)
	}
}

// TestComputePetCards_DeadSlotNotFlagged verifies the existing dead-
// slot pattern (CMC 5+ Utility-only) is NOT promoted to pet card —
// those are obvious cuts the cuttable path already owns, and double-
// flagging would muddy both signals.
func TestComputePetCards_DeadSlotNotFlagged(t *testing.T) {
	cards := []CardProfile{
		{Name: "Dead Slot Creature", CMC: 6, TypeLine: "Creature — Beast"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Dead Slot Creature", Roles: []RoleTag{RoleUtility}},
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	computePetCards(dp, report)
	if len(dp.PetCards) != 0 {
		t.Errorf("CMC 6 Utility-only dead slot should NOT be pet card, got: %+v", dp.PetCards)
	}
}

// TestComputePetCards_LegendaryGetsSignatureFraming verifies legendary
// creatures get the "signature flavor pick" reason string (vs the
// plain "personal-taste pick" for nonlegendary creatures) — legendaries
// are usually the strongest pet-card signal.
func TestComputePetCards_LegendaryGetsSignatureFraming(t *testing.T) {
	cards := []CardProfile{
		{Name: "Pet Legend", CMC: 4, TypeLine: "Legendary Creature — Human Wizard"},
		{Name: "Pet Regular", CMC: 4, TypeLine: "Creature — Goblin"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Pet Legend", Roles: []RoleTag{RoleThreat}},
		{Name: "Pet Regular", Roles: []RoleTag{RoleThreat}},
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	computePetCards(dp, report)

	if len(dp.PetCards) != 2 {
		t.Fatalf("want 2 pet cards, got %d", len(dp.PetCards))
	}
	byName := map[string]PetCard{}
	for _, pc := range dp.PetCards {
		byName[pc.Name] = pc
	}
	legend := byName["Pet Legend"]
	regular := byName["Pet Regular"]
	if !contains(legend.Reason, "signature") {
		t.Errorf("legendary reason should contain 'signature', got %q", legend.Reason)
	}
	if contains(regular.Reason, "signature") {
		t.Errorf("regular reason should NOT contain 'signature', got %q", regular.Reason)
	}
}

// TestComputePetCards_CappedAt8 verifies the 8-entry display cap.
func TestComputePetCards_CappedAt8(t *testing.T) {
	// 12 off-archetype CMC-4 creatures all qualify.
	cards := make([]CardProfile, 12)
	assignments := make([]CardRoleAssignment, 12)
	for i := range cards {
		name := fmt.Sprintf("Pet %d", i)
		cards[i] = CardProfile{Name: name, CMC: 4, TypeLine: "Creature — Goblin"}
		assignments[i] = CardRoleAssignment{Name: name, Roles: []RoleTag{RoleThreat}}
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	computePetCards(dp, report)
	if len(dp.PetCards) > 8 {
		t.Errorf("pet-card list cap exceeded: %d (max 8)", len(dp.PetCards))
	}
}

// TestComputePetCards_PowerDescendingOrder verifies pet cards are
// listed Power-descending (the "highest-power flavor picks" lead so
// the most defensible keeps appear first).
func TestComputePetCards_PowerDescendingOrder(t *testing.T) {
	// Three off-archetype creatures at different CMCs → different
	// CMCEfficiency components → different total power.
	cards := []CardProfile{
		{Name: "High Pet", CMC: 2, TypeLine: "Creature — Beast"},
		{Name: "Low Pet", CMC: 6, TypeLine: "Creature — Beast"},
		{Name: "Mid Pet", CMC: 4, TypeLine: "Creature — Beast"},
	}
	assignments := []CardRoleAssignment{
		{Name: "High Pet", Roles: []RoleTag{RoleThreat}},
		{Name: "Low Pet", Roles: []RoleTag{RoleThreat}},
		{Name: "Mid Pet", Roles: []RoleTag{RoleThreat}},
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	computePetCards(dp, report)
	for i := 1; i < len(dp.PetCards); i++ {
		if dp.PetCards[i].Power > dp.PetCards[i-1].Power {
			t.Errorf("pet cards not Power-descending: %s(%d) > %s(%d) at index %d",
				dp.PetCards[i].Name, dp.PetCards[i].Power,
				dp.PetCards[i-1].Name, dp.PetCards[i-1].Power, i)
		}
	}
}

// TestComputePetCards_PartiallyMatchedRolesNotFlagged verifies that a
// creature whose role list partially overlaps with the archetype
// fingerprint is NOT a pet card — even one matching role means the
// builder included it for a deck-aligned reason.
func TestComputePetCards_PartiallyMatchedRolesNotFlagged(t *testing.T) {
	// Combo fingerprint includes RoleTutor. Creature has BOTH RoleThreat
	// (off-archetype) AND RoleTutor (on-archetype) — should NOT be a pet.
	cards := []CardProfile{
		{Name: "Hybrid Creature", CMC: 4, TypeLine: "Creature — Wizard"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Hybrid Creature", Roles: []RoleTag{RoleThreat, RoleTutor}},
	}
	dp, report := petCardSetup("Combo", cards, assignments)
	computePetCards(dp, report)
	if len(dp.PetCards) != 0 {
		t.Errorf("creature with at least one matching role should NOT be pet card, got: %+v", dp.PetCards)
	}
}

// contains is a tiny test helper to keep the assertion lines readable.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
