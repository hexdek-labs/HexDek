package main

import (
	"strings"
	"testing"
)

// tutor_inference_r60_test.go — regressions for the r60 tutor inference
// extension covering modal cards + complex wordings the pre-r60 matcher
// dropped:
//
//   - Power-restricted creature tutors (Imperial Recruiter, Ranger of Eos):
//     "creature card with power N or less" → power_le_N
//   - Toughness-restricted creature tutors (Recruiter of the Guard):
//     "creature card with toughness N or less" → toughness_le_N
//   - Variable-X mana-value tutors (Chord of Calling, Birthing Pod,
//     Eldritch Evolution): "mana value X or less" / "equal to … plus N"
//     → cmc_variable
//   - Modal cards with a tutor as ONE mode (Mystical Tutor-style modal
//     variants): "Choose one — • Search your library for an instant
//     or sorcery card …" — must still flag as IsTutor.

// -----------------------------------------------------------------------------
// Power-restricted creature tutors
// -----------------------------------------------------------------------------

func TestTutor_ImperialRecruiter_PowerRestricted(t *testing.T) {
	ot := strings.ToLower(
		"when this creature enters, search your library for a creature card with " +
			"power 2 or less, reveal it, put it into your hand, then shuffle.")
	tl := strings.ToLower("creature — human")

	p := CardProfile{Name: "Imperial Recruiter"}
	classifyTutorInto(&p, ot, tl, "Imperial Recruiter")
	if !p.IsTutor {
		t.Fatal("Imperial Recruiter not flagged as tutor")
	}
	if p.IsLandTutor {
		t.Error("Imperial Recruiter should not be a land tutor")
	}

	restricted, delivery := inferTutorRestriction(ot, tl)
	if restricted != "power_le_2" {
		t.Errorf("Imperial Recruiter restriction = %q, want %q", restricted, "power_le_2")
	}
	if delivery != "hand" {
		t.Errorf("Imperial Recruiter delivery = %q, want %q", delivery, "hand")
	}
}

func TestTutor_RangerOfEos_PowerRestricted(t *testing.T) {
	ot := strings.ToLower(
		"when this creature enters, you may search your library for up to two " +
			"creature cards with mana value 1 or less, reveal them, put them into " +
			"your hand, then shuffle.")
	tl := strings.ToLower("creature — human archer")

	// Ranger of Eos uses mana value (not power), so should still land in
	// cmcle1 — guards against power_le_ pattern catching the wrong axis.
	restricted, _ := inferTutorRestriction(ot, tl)
	if restricted != "cmcle1" {
		t.Errorf("Ranger of Eos restriction = %q, want cmcle1 (mana value bound, NOT power)",
			restricted)
	}
}

// -----------------------------------------------------------------------------
// Toughness-restricted creature tutors
// -----------------------------------------------------------------------------

func TestTutor_RecruiterOfTheGuard_ToughnessRestricted(t *testing.T) {
	ot := strings.ToLower(
		"first strike. when this creature enters, search your library for a " +
			"creature card with toughness 2 or less, reveal it, put it into your " +
			"hand, then shuffle.")
	tl := strings.ToLower("creature — human soldier")

	p := CardProfile{Name: "Recruiter of the Guard"}
	classifyTutorInto(&p, ot, tl, "Recruiter of the Guard")
	if !p.IsTutor {
		t.Fatal("Recruiter of the Guard not flagged as tutor")
	}

	restricted, _ := inferTutorRestriction(ot, tl)
	if restricted != "toughness_le_2" {
		t.Errorf("Recruiter of the Guard restriction = %q, want %q",
			restricted, "toughness_le_2")
	}
}

// -----------------------------------------------------------------------------
// Variable-X mana-value tutors
// -----------------------------------------------------------------------------

func TestTutor_ChordOfCalling_VariableMV(t *testing.T) {
	ot := strings.ToLower(
		"convoke. search your library for a creature card with mana value x or " +
			"less, put it onto the battlefield, then shuffle.")
	tl := strings.ToLower("instant")

	p := CardProfile{Name: "Chord of Calling"}
	classifyTutorInto(&p, ot, tl, "Chord of Calling")
	if !p.IsTutor {
		t.Fatal("Chord of Calling not flagged as tutor")
	}

	restricted, delivery := inferTutorRestriction(ot, tl)
	if restricted != "cmc_variable" {
		t.Errorf("Chord of Calling restriction = %q, want cmc_variable", restricted)
	}
	if delivery != "battlefield" {
		t.Errorf("Chord of Calling delivery = %q, want battlefield", delivery)
	}
}

func TestTutor_BirthingPod_VariableMV(t *testing.T) {
	ot := strings.ToLower(
		"{1}{g/p}, {t}, sacrifice a creature: search your library for a creature " +
			"card with mana value equal to the sacrificed creature's mana value " +
			"plus 1, put it onto the battlefield, then shuffle. activate only as a " +
			"sorcery.")
	tl := strings.ToLower("artifact")

	p := CardProfile{Name: "Birthing Pod"}
	classifyTutorInto(&p, ot, tl, "Birthing Pod")
	if !p.IsTutor {
		t.Fatal("Birthing Pod not flagged as tutor")
	}

	restricted, _ := inferTutorRestriction(ot, tl)
	if restricted != "cmc_variable" {
		t.Errorf("Birthing Pod restriction = %q, want cmc_variable", restricted)
	}
}

// -----------------------------------------------------------------------------
// Modal cards with a tutor mode
// -----------------------------------------------------------------------------

// Modal "search your library" — Mystical Tutor variant. Pre-r60 the
// substring match already caught these; this test pins that behavior
// against accidental regression as the modal-aware splitModes helper
// gets adopted by other classifiers.
func TestTutor_ModalCard_TutorMode(t *testing.T) {
	ot := strings.ToLower(
		"choose one — • counter target spell. • search your library for an " +
			"instant or sorcery card, reveal it, then shuffle and put that card " +
			"on top.")
	tl := strings.ToLower("instant")

	p := CardProfile{Name: "Modal Mystical Tutor"}
	classifyTutorInto(&p, ot, tl, "Modal Mystical Tutor")
	if !p.IsTutor {
		t.Fatal("modal card with search mode should flag as tutor")
	}

	restricted, delivery := inferTutorRestriction(ot, tl)
	if restricted != "instant_sorcery" {
		t.Errorf("modal Mystical Tutor restriction = %q, want instant_sorcery", restricted)
	}
	if delivery != "top" {
		t.Errorf("modal Mystical Tutor delivery = %q, want top", delivery)
	}
}

// Modal card whose ONLY tutor mode is land-fetch — should still be
// flagged as a land tutor.
func TestTutor_ModalCard_LandFetchMode(t *testing.T) {
	ot := strings.ToLower(
		"choose one — • destroy target creature. • search your library for a " +
			"basic land card, put it onto the battlefield tapped, then shuffle.")
	tl := strings.ToLower("instant")

	p := CardProfile{Name: "Modal Cultivate"}
	classifyTutorInto(&p, ot, tl, "Modal Cultivate")
	if !p.IsTutor {
		t.Fatal("modal card with land-search mode should flag as tutor")
	}
	if !p.IsLandTutor {
		t.Error("modal Cultivate should also flag as land tutor (basic land mode)")
	}
}

// -----------------------------------------------------------------------------
// extractPowerLeBound / extractToughnessLeBound boundary
// -----------------------------------------------------------------------------

// "power 0 or less" is valid (Phantasmal Image style — 0/X creatures
// exist). Test the lower boundary explicitly so a future tightening of
// the loop range doesn't drop a real card on the floor.
func TestExtractPowerLeBound_ZeroBoundary(t *testing.T) {
	ot := "search your library for a creature card with power 0 or less, reveal it, put it into your hand."
	got := extractPowerLeBound(ot)
	if got != 0 {
		t.Errorf("power 0 or less should extract 0, got %d", got)
	}
}

// No "search your library" context → must return -1 (avoids false-firing
// on a flavor-text "power 2 or less" mention).
func TestExtractPowerLeBound_RequiresSearchContext(t *testing.T) {
	ot := "creatures with power 2 or less can't attack you."
	got := extractPowerLeBound(ot)
	if got != -1 {
		t.Errorf("power 2 or less outside search-your-library should return -1, got %d", got)
	}
}

// detectVariableCmcSearch must not false-fire on a literal-digit clause
// — "mana value 3 or less" is cmcle3, not cmc_variable.
func TestDetectVariableCmcSearch_LiteralDigitNotVariable(t *testing.T) {
	ot := "search your library for a creature card with mana value 3 or less, put it onto the battlefield."
	if detectVariableCmcSearch(ot) {
		t.Error("literal-digit 'mana value 3 or less' should NOT match cmc_variable")
	}
}

// -----------------------------------------------------------------------------
// tutorCanFind integration
// -----------------------------------------------------------------------------

func TestTutorCanFind_PowerLeRestriction(t *testing.T) {
	typeByName := map[string]string{
		"dockside extortionist": "creature — goblin pirate",
		"craterhoof behemoth":   "creature — beast",
		"sol ring":              "artifact",
	}

	// Without oracle DB (passed as nil), the power-le path falls back
	// to "creature type match → assume coverable". Verifies both that
	// the restriction key parses AND that the creature-type gate fires.
	tutor := TutorInfo{Restricted: "power_le_2"}

	if !tutorCanFind(tutor, "dockside extortionist", typeByName, nil) {
		t.Error("power_le_2 should fall back to creature-type match when no oracle data")
	}
	if tutorCanFind(tutor, "sol ring", typeByName, nil) {
		t.Error("power_le_2 should reject artifacts even without oracle data")
	}
}

func TestTutorCanFind_CmcVariable(t *testing.T) {
	typeByName := map[string]string{
		"craterhoof behemoth":   "creature — beast",
		"thassa's oracle":       "creature — merfolk wizard",
		"sol ring":              "artifact",
		"demonic tutor":         "sorcery",
	}
	tutor := TutorInfo{Restricted: "cmc_variable"}

	// Any creature passes the soft restriction.
	if !tutorCanFind(tutor, "craterhoof behemoth", typeByName, nil) {
		t.Error("cmc_variable should match any creature")
	}
	if !tutorCanFind(tutor, "thassa's oracle", typeByName, nil) {
		t.Error("cmc_variable should match Thassa's Oracle (creature)")
	}
	// Non-creatures don't pass.
	if tutorCanFind(tutor, "sol ring", typeByName, nil) {
		t.Error("cmc_variable should reject artifacts")
	}
	if tutorCanFind(tutor, "demonic tutor", typeByName, nil) {
		t.Error("cmc_variable should reject sorceries")
	}
}
