package main

import (
	"math"
	"strings"
	"testing"
)

// interactionFixture builds a minimal report so computeInteractionQuality
// has Roles.Assignments and Profiles populated. Each input is the card
// name + CMC + a single role tag (defaults to RoleRemoval).
type intCard struct {
	Name string
	CMC  int
	Role RoleTag
}

func interactionFixture(cards []intCard) (*DeckProfile, *FreyaReport) {
	profiles := make([]CardProfile, 0, len(cards))
	assignments := make([]CardRoleAssignment, 0, len(cards))
	for _, c := range cards {
		role := c.Role
		if role == "" {
			role = RoleRemoval
		}
		profiles = append(profiles, CardProfile{Name: c.Name, CMC: c.CMC, TypeLine: "Instant"})
		assignments = append(assignments, CardRoleAssignment{Name: c.Name, Roles: []RoleTag{role}})
	}
	report := &FreyaReport{
		Profiles: profiles,
		Roles: &RoleAnalysis{
			Assignments: assignments,
			TotalCards:  len(profiles),
			RoleCounts:  map[RoleTag]int{},
		},
	}
	return &DeckProfile{}, report
}

// TestInteractionDownsides_PathFlaggedAsRamp pins the canonical case:
// Path to Exile surfaces in InteractionDownsides with Kind="ramp" and
// the AdjustedInteractionQuality reads higher than the raw CMC because
// of the +0.5 penalty. This is the "Path ramp vs StP life" distinction
// the kanban refinement targets.
func TestInteractionDownsides_PathFlaggedAsRamp(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		{Name: "Path to Exile", CMC: 1},
		{Name: "Swords to Plowshares", CMC: 1},
		{Name: "Counterspell", CMC: 2, Role: RoleCounterspell},
	})
	computeInteractionQuality(dp, report, nil)

	if len(dp.InteractionDownsides) != 1 {
		t.Fatalf("want 1 downside (Path), got %d: %+v", len(dp.InteractionDownsides), dp.InteractionDownsides)
	}
	d := dp.InteractionDownsides[0]
	if d.Name != "Path to Exile" {
		t.Errorf("wrong downside card: %q", d.Name)
	}
	if d.Kind != "ramp" {
		t.Errorf("Path Kind = %q, want %q", d.Kind, "ramp")
	}
	if !strings.Contains(d.Note, "basic land") {
		t.Errorf("Note should mention the basic-land search, got: %q", d.Note)
	}
	// Raw = (1 + 1 + 2) / 3 = 1.333; adjusted = (4 + 0.5*1) / 3 = 1.5.
	if math.Abs(dp.InteractionQuality-1.3333) > 0.01 {
		t.Errorf("InteractionQuality = %.4f, want ~1.3333", dp.InteractionQuality)
	}
	if math.Abs(dp.AdjustedInteractionQuality-1.5) > 0.01 {
		t.Errorf("AdjustedInteractionQuality = %.4f, want 1.5", dp.AdjustedInteractionQuality)
	}
	if dp.AdjustedInteractionQuality <= dp.InteractionQuality {
		t.Errorf("adjusted (%.2f) must be > raw (%.2f) when downsides present",
			dp.AdjustedInteractionQuality, dp.InteractionQuality)
	}
}

// TestInteractionDownsides_StPNotFlagged verifies Swords to Plowshares
// (life to opponent — minor, not opponent leverage) is NOT in the
// downside table. This is the explicit "unconditional clean" baseline
// the Path comparison rests on.
func TestInteractionDownsides_StPNotFlagged(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		{Name: "Swords to Plowshares", CMC: 1},
	})
	computeInteractionQuality(dp, report, nil)

	if len(dp.InteractionDownsides) != 0 {
		t.Errorf("StP must NOT be flagged as a downside, got: %+v", dp.InteractionDownsides)
	}
	if dp.AdjustedInteractionQuality != dp.InteractionQuality {
		t.Errorf("with zero downsides, adjusted (%.2f) must equal raw (%.2f)",
			dp.AdjustedInteractionQuality, dp.InteractionQuality)
	}
}

// TestInteractionDownsides_CreatureTokenFamilyFlagged verifies the
// 3/3-token family (Generous Gift, Beast Within, Pongify, Rapid
// Hybridization) all surface with Kind="creature_token". Pins the
// curated table against accidental key drift.
func TestInteractionDownsides_CreatureTokenFamilyFlagged(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		{Name: "Generous Gift", CMC: 3},
		{Name: "Beast Within", CMC: 3},
		{Name: "Pongify", CMC: 1},
		{Name: "Rapid Hybridization", CMC: 1},
	})
	computeInteractionQuality(dp, report, nil)

	if len(dp.InteractionDownsides) != 4 {
		t.Fatalf("want 4 token downsides, got %d: %+v",
			len(dp.InteractionDownsides), dp.InteractionDownsides)
	}
	for _, d := range dp.InteractionDownsides {
		if d.Kind != "creature_token" {
			t.Errorf("%s Kind = %q, want %q", d.Name, d.Kind, "creature_token")
		}
	}
}

// TestInteractionDownsides_NonInteractionIgnored verifies cards tagged
// with non-interaction roles (Ramp, Draw, Threat) are ignored even if
// they appear in the downside table. The downside system is scoped to
// removal/counter/wipe only — that's what the metric is about.
func TestInteractionDownsides_NonInteractionIgnored(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		// Hypothetically: a card named "Path to Exile" tagged as Ramp
		// (would not happen in practice, but defends the role gate).
		{Name: "Path to Exile", CMC: 1, Role: RoleRamp},
	})
	computeInteractionQuality(dp, report, nil)

	if len(dp.InteractionDownsides) != 0 {
		t.Errorf("non-interaction roles should be skipped, got: %+v", dp.InteractionDownsides)
	}
}

// TestInteractionDownsides_AlphabeticalOrder pins the sort order
// (alphabetical by Name) so report output is stable across runs.
func TestInteractionDownsides_AlphabeticalOrder(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		{Name: "Pongify", CMC: 1},
		{Name: "Beast Within", CMC: 3},
		{Name: "Generous Gift", CMC: 3},
	})
	computeInteractionQuality(dp, report, nil)

	if len(dp.InteractionDownsides) != 3 {
		t.Fatalf("want 3, got %d", len(dp.InteractionDownsides))
	}
	want := []string{"Beast Within", "Generous Gift", "Pongify"}
	for i := range want {
		if dp.InteractionDownsides[i].Name != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, dp.InteractionDownsides[i].Name, want[i])
		}
	}
}

// TestInteractionDownsides_AdjustedScalesWithCount verifies the +0.5
// per downside math compounds: 2 downsides in a 4-piece suite shifts
// adjusted CMC by exactly 0.25.
func TestInteractionDownsides_AdjustedScalesWithCount(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		{Name: "Path to Exile", CMC: 1},
		{Name: "Pongify", CMC: 1},
		{Name: "Swords to Plowshares", CMC: 1},
		{Name: "Counterspell", CMC: 2, Role: RoleCounterspell},
	})
	computeInteractionQuality(dp, report, nil)

	if len(dp.InteractionDownsides) != 2 {
		t.Fatalf("want 2 downsides, got %d", len(dp.InteractionDownsides))
	}
	// Raw = 5 / 4 = 1.25; adjusted = (5 + 1.0) / 4 = 1.5.
	if math.Abs(dp.InteractionQuality-1.25) > 0.01 {
		t.Errorf("InteractionQuality = %.4f, want 1.25", dp.InteractionQuality)
	}
	if math.Abs(dp.AdjustedInteractionQuality-1.5) > 0.01 {
		t.Errorf("AdjustedInteractionQuality = %.4f, want 1.5", dp.AdjustedInteractionQuality)
	}
	if diff := dp.AdjustedInteractionQuality - dp.InteractionQuality; math.Abs(diff-0.25) > 0.01 {
		t.Errorf("shift = %.4f, want 0.25 (2 downsides × 0.5 / 4 pieces)", diff)
	}
}

// TestInteractionDownsides_JSONRoundTrips verifies the JSON build path
// surfaces the new fields so downstream consumers (UI, deck-builder
// integrations) actually see the signal.
func TestInteractionDownsides_JSONRoundTrips(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		{Name: "Path to Exile", CMC: 1},
		{Name: "Generous Gift", CMC: 3},
	})
	computeInteractionQuality(dp, report, nil)

	js := buildJSONDeckProfile(dp, report)
	if js == nil {
		t.Fatalf("buildJSONDeckProfile returned nil")
	}
	if len(js.InteractionDownsides) != 2 {
		t.Errorf("InteractionDownsides not mirrored to JSON, got %d entries",
			len(js.InteractionDownsides))
	}
	if js.AdjustedInteractionQuality == 0 {
		t.Errorf("AdjustedInteractionQuality not mirrored to JSON, got 0")
	}
	if js.AdjustedInteractionQuality <= js.InteractionQuality {
		t.Errorf("adjusted (%.2f) must be > raw (%.2f) in JSON",
			js.AdjustedInteractionQuality, js.InteractionQuality)
	}
}

// TestInteractionDownsides_EmptyDeckSafe verifies the function is a
// no-op when there's no interaction at all. Adjusted = 0, downsides
// nil, no panic.
func TestInteractionDownsides_EmptyDeckSafe(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		{Name: "Sol Ring", CMC: 1, Role: RoleRamp},
	})
	computeInteractionQuality(dp, report, nil)

	if dp.InteractionQuality != 0 || dp.AdjustedInteractionQuality != 0 {
		t.Errorf("zero-interaction deck should keep both quality metrics at 0, got raw=%.2f adj=%.2f",
			dp.InteractionQuality, dp.AdjustedInteractionQuality)
	}
	if len(dp.InteractionDownsides) != 0 {
		t.Errorf("zero-interaction deck should have nil downsides, got: %+v", dp.InteractionDownsides)
	}
}

// TestInteractionDownsides_NameLookupIsCaseInsensitive verifies the
// curated table matches regardless of input casing — defends against
// upstream name-normalization drift (Scryfall sometimes returns
// title-case, sometimes preserves the printed casing on novelty
// promos).
func TestInteractionDownsides_NameLookupIsCaseInsensitive(t *testing.T) {
	dp, report := interactionFixture([]intCard{
		{Name: "PATH TO EXILE", CMC: 1},
		{Name: "generous gift", CMC: 3},
	})
	computeInteractionQuality(dp, report, nil)

	if len(dp.InteractionDownsides) != 2 {
		t.Errorf("case-insensitive lookup failed, got: %+v", dp.InteractionDownsides)
	}
}
