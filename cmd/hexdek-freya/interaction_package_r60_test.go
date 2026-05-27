package main

import (
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// interactionPackageFixture wires the minimal Roles+Profiles+WinLines+oracle
// shape so computeInteractionPackage runs end-to-end without booting the
// full Freya pipeline. WinLines is intentionally populated with a couple
// of synthetic lines so the per-winline DefendedBy back-fill can be
// asserted on real list elements.
func interactionPackageFixture(
	assignments []CardRoleAssignment,
	profiles []CardProfile,
	tutorMap []TutorInfo,
	winLines []WinLine,
	oracleText map[string]string,
) (*DeckProfile, *FreyaReport, *oracleDB) {
	report := &FreyaReport{
		Profiles: profiles,
		Roles: &RoleAnalysis{
			Assignments: assignments,
			RoleCounts:  map[RoleTag]int{},
			TotalCards:  len(profiles),
		},
		WinLines: &WinLineAnalysis{
			WinLines: winLines,
			TutorMap: tutorMap,
		},
	}
	for _, a := range assignments {
		for _, r := range a.Roles {
			report.Roles.RoleCounts[r]++
		}
	}
	return &DeckProfile{}, report, stubOracleWithText(oracleText)
}

// TestInteractionPackage_EmptyDeck pins the defensive-zero path: a deck
// with no counterspells / protection / opp-int / tutors emits Score=0,
// empty slices, and writes nil DefendedBy onto every win line so the
// JSON renderer omits the field cleanly.
func TestInteractionPackage_EmptyDeck(t *testing.T) {
	dp, report, oracle := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Beater", Roles: []RoleTag{RoleThreat}},
		},
		[]CardProfile{{Name: "Beater", CMC: 4}},
		nil,
		[]WinLine{
			{Pieces: []string{"Beater"}, Type: "combat"},
		},
		map[string]string{"beater": "Trample."},
	)
	computeInteractionPackage(dp, report, oracle)

	pkg := dp.InteractionPackage
	if pkg.Score != 0 {
		t.Errorf("empty deck score = %v, want 0", pkg.Score)
	}
	if len(pkg.Counterspells)+len(pkg.OpponentInteraction)+len(pkg.Protection)+len(pkg.ProtectionTutors) != 0 {
		t.Errorf("empty deck should produce empty buckets, got %+v", pkg)
	}
	if report.WinLines.WinLines[0].DefendedBy != nil {
		t.Errorf("DefendedBy should be nil when no defenders, got %v", report.WinLines.WinLines[0].DefendedBy)
	}
}

// TestInteractionPackage_CounterspellsBucket verifies counterspells flow
// from RoleCounterspell assignments into pkg.Counterspells, contribute to
// the score at the 1/4 saturation rate (4 counters = full credit), and
// surface on every win line's DefendedBy.
func TestInteractionPackage_CounterspellsBucket(t *testing.T) {
	dp, report, oracle := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Counterspell", Roles: []RoleTag{RoleCounterspell}},
			{Name: "Force of Will", Roles: []RoleTag{RoleCounterspell}},
			{Name: "Mana Drain", Roles: []RoleTag{RoleCounterspell}},
			{Name: "Swan Song", Roles: []RoleTag{RoleCounterspell}},
		},
		[]CardProfile{
			{Name: "Counterspell", CMC: 2},
			{Name: "Force of Will", CMC: 5},
			{Name: "Mana Drain", CMC: 2},
			{Name: "Swan Song", CMC: 1},
		},
		nil,
		[]WinLine{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
		map[string]string{},
	)
	computeInteractionPackage(dp, report, oracle)

	wantCounters := []string{"Counterspell", "Force of Will", "Mana Drain", "Swan Song"}
	if !reflect.DeepEqual(dp.InteractionPackage.Counterspells, wantCounters) {
		t.Errorf("Counterspells = %v, want %v", dp.InteractionPackage.Counterspells, wantCounters)
	}
	// 4 counters → 1.0 contribution; 0 from other 3 buckets → score = 0.25.
	if got := dp.InteractionPackage.Score; math.Abs(got-0.25) > 0.001 {
		t.Errorf("Score = %v, want 0.25 (4 counters / 0 else)", got)
	}
	defenders := report.WinLines.WinLines[0].DefendedBy
	if !reflect.DeepEqual(defenders, wantCounters) {
		t.Errorf("DefendedBy = %v, want %v", defenders, wantCounters)
	}
}

// TestInteractionPackage_ProtectionBucket verifies the dedicated-protection
// list resolves via name matching against the curated protectionCards table,
// scores at 1/3 saturation, and joins the per-winline DefendedBy union.
func TestInteractionPackage_ProtectionBucket(t *testing.T) {
	dp, report, oracle := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Veil of Summer", Roles: []RoleTag{RoleProtection}},
			{Name: "Silence", Roles: []RoleTag{RoleProtection}},
			{Name: "Defense Grid", Roles: []RoleTag{RoleProtection}},
		},
		[]CardProfile{
			{Name: "Veil of Summer", CMC: 1},
			{Name: "Silence", CMC: 1},
			{Name: "Defense Grid", CMC: 2},
		},
		nil,
		[]WinLine{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
		map[string]string{},
	)
	computeInteractionPackage(dp, report, oracle)

	wantProt := []string{"Defense Grid", "Silence", "Veil of Summer"}
	if !reflect.DeepEqual(dp.InteractionPackage.Protection, wantProt) {
		t.Errorf("Protection = %v, want %v", dp.InteractionPackage.Protection, wantProt)
	}
	// 3 protection → 1.0 / 4 = 0.25 score.
	if got := dp.InteractionPackage.Score; math.Abs(got-0.25) > 0.001 {
		t.Errorf("Score = %v, want 0.25 (3 protection / 0 else)", got)
	}
	if !reflect.DeepEqual(report.WinLines.WinLines[0].DefendedBy, wantProt) {
		t.Errorf("DefendedBy = %v, want %v", report.WinLines.WinLines[0].DefendedBy, wantProt)
	}
}

// TestInteractionPackage_OpponentInteractionBucket verifies opp-interaction
// removal (Krosan Grip / Boseiju / Pyroblast) scores via the
// OpponentInteraction bucket but is INTENTIONALLY excluded from per-winline
// DefendedBy — these cards open the window for a win line by answering
// opposing hate, they don't defend the resolving line itself.
func TestInteractionPackage_OpponentInteractionBucket(t *testing.T) {
	dp, report, oracle := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Krosan Grip", Roles: []RoleTag{RoleRemoval}},
			{Name: "Boseiju, Who Endures", Roles: []RoleTag{RoleRemoval}},
			{Name: "Pyroblast", Roles: []RoleTag{RoleRemoval}},
		},
		[]CardProfile{
			{Name: "Krosan Grip", CMC: 3},
			{Name: "Boseiju, Who Endures", CMC: 0},
			{Name: "Pyroblast", CMC: 1},
		},
		nil,
		[]WinLine{
			{Pieces: []string{"Thassa's Oracle"}, Type: "infinite"},
		},
		map[string]string{},
	)
	computeInteractionPackage(dp, report, oracle)

	wantOpp := []string{"Boseiju, Who Endures", "Krosan Grip", "Pyroblast"}
	if !reflect.DeepEqual(dp.InteractionPackage.OpponentInteraction, wantOpp) {
		t.Errorf("OpponentInteraction = %v, want %v", dp.InteractionPackage.OpponentInteraction, wantOpp)
	}
	// DefendedBy is counterspells + protection only — opp-int doesn't list.
	if got := report.WinLines.WinLines[0].DefendedBy; got != nil {
		t.Errorf("DefendedBy should be nil when only opp-int present, got %v", got)
	}
}

// TestInteractionPackage_ProtectionTutorsGatedOnProtection verifies the
// ProtectionTutors bucket only counts tutors when the deck ALSO has
// protection cards — a deck with Mystical Tutor but no Veil/Defense Grid
// shouldn't get credit for "could tutor protection" because there's nothing
// to find.
func TestInteractionPackage_ProtectionTutorsGatedOnProtection(t *testing.T) {
	// Case A: tutor without protection — tutor bucket stays empty.
	dpA, reportA, oracleA := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Mystical Tutor", Roles: []RoleTag{RoleTutor}},
		},
		[]CardProfile{
			{Name: "Mystical Tutor", CMC: 1, TypeLine: "Instant"},
		},
		[]TutorInfo{
			{Name: "Mystical Tutor", Restricted: "instant_sorcery", Delivery: "top"},
		},
		[]WinLine{{Pieces: []string{"X"}, Type: "infinite"}},
		map[string]string{"veil of summer": "Instant"},
	)
	computeInteractionPackage(dpA, reportA, oracleA)
	if len(dpA.InteractionPackage.ProtectionTutors) != 0 {
		t.Errorf("case A (no protection): ProtectionTutors should be empty, got %v",
			dpA.InteractionPackage.ProtectionTutors)
	}

	// Case B: tutor + protection — tutor bucket populates.
	dpB, reportB, oracleB := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Mystical Tutor", Roles: []RoleTag{RoleTutor}},
			{Name: "Veil of Summer", Roles: []RoleTag{RoleProtection}},
		},
		[]CardProfile{
			{Name: "Mystical Tutor", CMC: 1, TypeLine: "Instant"},
			{Name: "Veil of Summer", CMC: 1, TypeLine: "Instant"},
		},
		[]TutorInfo{
			{Name: "Mystical Tutor", Restricted: "instant_sorcery", Delivery: "top"},
		},
		[]WinLine{{Pieces: []string{"X"}, Type: "infinite"}},
		map[string]string{"veil of summer": "Instant"},
	)
	computeInteractionPackage(dpB, reportB, oracleB)
	wantTutors := []string{"Mystical Tutor"}
	if !reflect.DeepEqual(dpB.InteractionPackage.ProtectionTutors, wantTutors) {
		t.Errorf("case B: ProtectionTutors = %v, want %v",
			dpB.InteractionPackage.ProtectionTutors, wantTutors)
	}
}

// TestInteractionPackage_LandTutorSkipped verifies land-restricted tutors
// (Crop Rotation, Expedition Map) are NOT counted as protection tutors,
// even when the deck has protection — land tutors structurally can't fetch
// instant/enchantment protection.
func TestInteractionPackage_LandTutorSkipped(t *testing.T) {
	dp, report, oracle := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Crop Rotation", Roles: []RoleTag{RoleTutor}},
			{Name: "Veil of Summer", Roles: []RoleTag{RoleProtection}},
		},
		[]CardProfile{
			{Name: "Crop Rotation", CMC: 1, TypeLine: "Instant"},
			{Name: "Veil of Summer", CMC: 1, TypeLine: "Instant"},
		},
		[]TutorInfo{
			{Name: "Crop Rotation", Restricted: "land", Delivery: "battlefield"},
		},
		[]WinLine{{Pieces: []string{"X"}, Type: "infinite"}},
		map[string]string{"veil of summer": "Instant"},
	)
	computeInteractionPackage(dp, report, oracle)
	if len(dp.InteractionPackage.ProtectionTutors) != 0 {
		t.Errorf("land tutor should NOT count as protection tutor, got %v",
			dp.InteractionPackage.ProtectionTutors)
	}
}

// TestInteractionPackage_FullKitScoresHigh verifies the canonical "Thoracle
// + protection package" deck saturates all four buckets and scores 1.0.
func TestInteractionPackage_FullKitScoresHigh(t *testing.T) {
	dp, report, oracle := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Counterspell", Roles: []RoleTag{RoleCounterspell}},
			{Name: "Force of Will", Roles: []RoleTag{RoleCounterspell}},
			{Name: "Mana Drain", Roles: []RoleTag{RoleCounterspell}},
			{Name: "Mental Misstep", Roles: []RoleTag{RoleCounterspell}},
			{Name: "Veil of Summer", Roles: []RoleTag{RoleProtection}},
			{Name: "Silence", Roles: []RoleTag{RoleProtection}},
			{Name: "Defense Grid", Roles: []RoleTag{RoleProtection}},
			{Name: "Krosan Grip", Roles: []RoleTag{RoleRemoval}},
			{Name: "Nature's Claim", Roles: []RoleTag{RoleRemoval}},
			{Name: "Pyroblast", Roles: []RoleTag{RoleRemoval}},
			{Name: "Mystical Tutor", Roles: []RoleTag{RoleTutor}},
			{Name: "Demonic Tutor", Roles: []RoleTag{RoleTutor}},
		},
		[]CardProfile{
			{Name: "Counterspell", CMC: 2}, {Name: "Force of Will", CMC: 5},
			{Name: "Mana Drain", CMC: 2}, {Name: "Mental Misstep", CMC: 1},
			{Name: "Veil of Summer", CMC: 1, TypeLine: "Instant"},
			{Name: "Silence", CMC: 1, TypeLine: "Instant"},
			{Name: "Defense Grid", CMC: 2, TypeLine: "Artifact"},
			{Name: "Krosan Grip", CMC: 3}, {Name: "Nature's Claim", CMC: 1},
			{Name: "Pyroblast", CMC: 1},
			{Name: "Mystical Tutor", CMC: 1, TypeLine: "Instant"},
			{Name: "Demonic Tutor", CMC: 2, TypeLine: "Sorcery"},
		},
		[]TutorInfo{
			{Name: "Mystical Tutor", Restricted: "instant_sorcery", Delivery: "top"},
			{Name: "Demonic Tutor", Restricted: "any", Delivery: "hand"},
		},
		[]WinLine{{Pieces: []string{"Thassa's Oracle"}, Type: "infinite"}},
		map[string]string{
			"veil of summer": "Instant", "silence": "Instant", "defense grid": "Artifact",
		},
	)
	computeInteractionPackage(dp, report, oracle)
	if got := dp.InteractionPackage.Score; math.Abs(got-1.0) > 0.001 {
		t.Errorf("full-kit Score = %v, want 1.0", got)
	}
	if len(dp.InteractionPackage.ProtectionTutors) != 2 {
		t.Errorf("ProtectionTutors len = %d, want 2", len(dp.InteractionPackage.ProtectionTutors))
	}
}

// TestInteractionPackage_DefendedByCap verifies the per-winline DefendedBy
// list is capped at defendedByCap entries. A deck with 8 counterspells + 12
// protection cards would otherwise list 20 defenders per line — too noisy
// for the JSON payload.
func TestInteractionPackage_DefendedByCap(t *testing.T) {
	// Build 10 counterspells + protectionCards.size of every protection
	// card so the combined defender list far exceeds the cap.
	assignments := []CardRoleAssignment{}
	profiles := []CardProfile{}
	for i := 0; i < 10; i++ {
		name := "Counter " + string(rune('A'+i))
		assignments = append(assignments, CardRoleAssignment{Name: name, Roles: []RoleTag{RoleCounterspell}})
		profiles = append(profiles, CardProfile{Name: name, CMC: 2})
	}
	for protKey := range protectionCards {
		display := strings.Title(protKey)
		// Patch the protectionCards lookup by using the actual capitalized
		// display name — match works because computeInteractionPackage
		// lowercases the profile name before lookup.
		assignments = append(assignments, CardRoleAssignment{Name: display, Roles: []RoleTag{RoleProtection}})
		profiles = append(profiles, CardProfile{Name: display, CMC: 1})
	}

	dp, report, oracle := interactionPackageFixture(
		assignments, profiles, nil,
		[]WinLine{{Pieces: []string{"X"}, Type: "infinite"}},
		map[string]string{},
	)
	computeInteractionPackage(dp, report, oracle)
	got := report.WinLines.WinLines[0].DefendedBy
	if len(got) != defendedByCap {
		t.Errorf("DefendedBy len = %d, want cap %d", len(got), defendedByCap)
	}
	// Cap should preserve alphabetical order (a stable truncation).
	if !sort.StringsAreSorted(got) {
		t.Errorf("DefendedBy should remain alphabetically sorted after cap, got %v", got)
	}
}

// TestInteractionPackage_ScoreSaturates verifies each bucket caps at its
// saturation point — going from 4 to 8 counterspells doesn't keep growing
// the bucket beyond 1.0.
func TestInteractionPackage_ScoreSaturates(t *testing.T) {
	assignments := []CardRoleAssignment{}
	profiles := []CardProfile{}
	for i := 0; i < 8; i++ {
		name := "Counter " + string(rune('A'+i))
		assignments = append(assignments, CardRoleAssignment{Name: name, Roles: []RoleTag{RoleCounterspell}})
		profiles = append(profiles, CardProfile{Name: name, CMC: 2})
	}
	dp, report, oracle := interactionPackageFixture(
		assignments, profiles, nil,
		[]WinLine{{Pieces: []string{"X"}, Type: "infinite"}},
		map[string]string{},
	)
	computeInteractionPackage(dp, report, oracle)
	// 8 counters but only the counter bucket — saturates to 1.0, score = 0.25.
	if got := dp.InteractionPackage.Score; math.Abs(got-0.25) > 0.001 {
		t.Errorf("Score with 8 counters = %v, want saturated 0.25", got)
	}
}

// TestInteractionPackage_PerWinLineUniformDefenders verifies every win line
// in the report gets the SAME DefendedBy union — different line types
// (infinite vs combat vs commander_damage) lean on counterspells vs
// protection to different degrees, but at the deckbuilder level the same
// package answers all of them.
func TestInteractionPackage_PerWinLineUniformDefenders(t *testing.T) {
	dp, report, oracle := interactionPackageFixture(
		[]CardRoleAssignment{
			{Name: "Counterspell", Roles: []RoleTag{RoleCounterspell}},
			{Name: "Heroic Intervention", Roles: []RoleTag{RoleProtection}},
		},
		[]CardProfile{
			{Name: "Counterspell", CMC: 2},
			{Name: "Heroic Intervention", CMC: 2, TypeLine: "Instant"},
		},
		nil,
		[]WinLine{
			{Pieces: []string{"Thassa's Oracle"}, Type: "infinite"},
			{Pieces: []string{"Hero"}, Type: "combat"},
			{Pieces: []string{"Cmdr"}, Type: "commander_damage"},
		},
		map[string]string{"heroic intervention": "Instant"},
	)
	computeInteractionPackage(dp, report, oracle)
	want := []string{"Counterspell", "Heroic Intervention"}
	for i, wl := range report.WinLines.WinLines {
		if !reflect.DeepEqual(wl.DefendedBy, want) {
			t.Errorf("line[%d] (%s) DefendedBy = %v, want %v",
				i, wl.Type, wl.DefendedBy, want)
		}
	}
}
