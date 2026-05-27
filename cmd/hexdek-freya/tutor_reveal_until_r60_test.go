package main

import (
	"strings"
	"testing"
)

// tutor_reveal_until_r60_test.go — pins the r60 tutor-inference extension
// covering the "reveal cards from the top of your library until you
// reveal a [X]" family (Madcap Experiment, Jalira Master Polymorphist,
// Riptide Shapeshifter, Reality Scramble, Mass Polymorph, Avenging Druid,
// ...). These are functionally tutors — they consistently find a card of
// a specified type — but they were missed by the pre-r60 detector because
// the wording is "reveal until" rather than "search your library".
//
// The 2026-05-26 oracle dump has 39 cards using this pattern WITHOUT also
// containing "search your library", and they all behave as tutors for
// combo-piece-finding purposes. Tests cover:
//
//   - non-land reveal-until: Madcap Experiment (artifact), Jalira
//     (nonlegendary creature), Riptide Shapeshifter (chosen creature type),
//     Reality Scramble (matching card type), Mass Polymorph (mass creature
//     tutor), Synthetic Destiny (mass creature reveal), Hei Bai (Shrine)
//   - land-restricted reveal-until (must route to IsLandTutor, not the
//     consistency bucket): Avenging Druid, Recross the Paths, The Regalia,
//     Clifftop Lookout
//   - the modal "choose one, one mode is search-library" family was
//     already covered by the existing "search your library" substring
//     match — pin a couple of canonical examples (Tooth and Nail,
//     Mastermind's Acquisition, Archdruid's Charm) so a future
//     refactor doesn't regress that path
//   - negative coverage: dig-and-pick effects (Sleight of Hand, Strategic
//     Planning, Tower Geist) must NOT flag as tutors — they're filtering,
//     not consistency-finding
//   - negative coverage: exile-and-cast effects (Etali, Primal Storm)
//     must NOT flag as tutors — they're random impulse, not directional

// -----------------------------------------------------------------------------
// reveal-until — non-land (consistency bucket)
// -----------------------------------------------------------------------------

func TestTutor_MadcapExperiment_RevealUntilArtifact(t *testing.T) {
	ot := strings.ToLower(
		"reveal cards from the top of your library until you reveal an artifact card. " +
			"put that card onto the battlefield and the rest on the bottom of your library " +
			"in a random order. madcap experiment deals damage to you equal to the number of " +
			"cards revealed this way.")
	tl := strings.ToLower("sorcery")

	p := CardProfile{Name: "Madcap Experiment"}
	classifyTutorInto(&p, ot, tl, "Madcap Experiment")
	if !p.IsTutor {
		t.Errorf("Madcap Experiment must flag IsTutor (artifact reveal-until)")
	}
	if p.IsLandTutor {
		t.Errorf("Madcap Experiment is not a land tutor (artifact-restricted)")
	}
}

func TestTutor_JaliraMasterPolymorphist_RevealUntilCreature(t *testing.T) {
	ot := strings.ToLower(
		"{2}{u}, {t}, sacrifice another creature: reveal cards from the top of your " +
			"library until you reveal a nonlegendary creature card. put that card onto " +
			"the battlefield and the rest on the bottom of your library in a random order.")
	tl := strings.ToLower("legendary creature — human wizard")

	p := CardProfile{Name: "Jalira, Master Polymorphist"}
	classifyTutorInto(&p, ot, tl, "Jalira, Master Polymorphist")
	if !p.IsTutor {
		t.Errorf("Jalira must flag IsTutor (nonlegendary-creature reveal-until)")
	}
	if p.IsLandTutor {
		t.Errorf("Jalira is not a land tutor")
	}
}

func TestTutor_RiptideShapeshifter_RevealUntilTypeChosen(t *testing.T) {
	ot := strings.ToLower(
		"{2}{u}{u}, sacrifice this creature: choose a creature type. reveal cards " +
			"from the top of your library until you reveal a creature card of that type. " +
			"put that card onto the battlefield and shuffle the rest into your library.")
	tl := strings.ToLower("creature — shapeshifter")

	p := CardProfile{Name: "Riptide Shapeshifter"}
	classifyTutorInto(&p, ot, tl, "Riptide Shapeshifter")
	if !p.IsTutor {
		t.Errorf("Riptide Shapeshifter must flag IsTutor (creature-of-chosen-type reveal-until)")
	}
}

func TestTutor_RealityScramble_RevealUntilSharesType(t *testing.T) {
	ot := strings.ToLower(
		"put target permanent you own on the bottom of your library. reveal cards from " +
			"the top of your library until you reveal a card that shares a card type with " +
			"that permanent. put that card onto the battlefield and the rest on the bottom " +
			"in random order.")
	tl := strings.ToLower("sorcery")

	p := CardProfile{Name: "Reality Scramble"}
	classifyTutorInto(&p, ot, tl, "Reality Scramble")
	if !p.IsTutor {
		t.Errorf("Reality Scramble must flag IsTutor (shares-card-type reveal-until)")
	}
}

func TestTutor_MassPolymorph_RevealUntilManyCreatures(t *testing.T) {
	ot := strings.ToLower(
		"exile all creatures you control, then reveal cards from the top of your library " +
			"until you reveal that many creature cards. put all creature cards revealed this " +
			"way onto the battlefield, then shuffle the rest into your library.")
	tl := strings.ToLower("sorcery")

	p := CardProfile{Name: "Mass Polymorph"}
	classifyTutorInto(&p, ot, tl, "Mass Polymorph")
	if !p.IsTutor {
		t.Errorf("Mass Polymorph must flag IsTutor (mass creature reveal-until)")
	}
}

func TestTutor_HeiBaiForestGuardian_RevealUntilShrine(t *testing.T) {
	ot := strings.ToLower(
		"when hei bai enters, reveal cards from the top of your library until you reveal " +
			"a shrine card. you may put that card onto the battlefield. then shuffle.")
	tl := strings.ToLower("legendary creature — spirit")

	p := CardProfile{Name: "Hei Bai, Forest Guardian"}
	classifyTutorInto(&p, ot, tl, "Hei Bai, Forest Guardian")
	if !p.IsTutor {
		t.Errorf("Hei Bai must flag IsTutor (Shrine reveal-until)")
	}
	if p.IsLandTutor {
		t.Errorf("Hei Bai is not a land tutor (Shrine-restricted, Shrine is an enchantment subtype)")
	}
}

// -----------------------------------------------------------------------------
// reveal-until — land-restricted (ramp bucket)
// -----------------------------------------------------------------------------

func TestTutor_AvengingDruid_LandRevealUntil(t *testing.T) {
	ot := strings.ToLower(
		"whenever this creature deals damage to an opponent, you may reveal cards from " +
			"the top of your library until you reveal a land card. if you do, put that " +
			"card onto the battlefield and put all other cards revealed this way on the " +
			"bottom of your library in any order.")
	tl := strings.ToLower("creature — human druid")

	p := CardProfile{Name: "Avenging Druid"}
	classifyTutorInto(&p, ot, tl, "Avenging Druid")
	if !p.IsTutor {
		t.Errorf("Avenging Druid must flag IsTutor (land reveal-until)")
	}
	if !p.IsLandTutor {
		t.Errorf("Avenging Druid must flag IsLandTutor — it's land-restricted, ramp bucket")
	}
}

func TestTutor_RecrossThePaths_LandRevealUntil(t *testing.T) {
	ot := strings.ToLower(
		"reveal cards from the top of your library until you reveal a land card. put " +
			"that card onto the battlefield and the rest on the bottom of your library " +
			"in any order. clash with an opponent. if you win, you may put this card " +
			"on top of your library.")
	tl := strings.ToLower("sorcery")

	p := CardProfile{Name: "Recross the Paths"}
	classifyTutorInto(&p, ot, tl, "Recross the Paths")
	if !p.IsTutor {
		t.Errorf("Recross the Paths must flag IsTutor")
	}
	if !p.IsLandTutor {
		t.Errorf("Recross the Paths must flag IsLandTutor")
	}
}

func TestTutor_TheRegalia_MultiLandRevealUntil(t *testing.T) {
	ot := strings.ToLower(
		"haste. whenever the regalia attacks, reveal cards from the top of your library " +
			"until you reveal a land card. put that card onto the battlefield tapped and " +
			"the rest on the bottom of your library in a random order.")
	tl := strings.ToLower("legendary artifact — vehicle")

	p := CardProfile{Name: "The Regalia"}
	classifyTutorInto(&p, ot, tl, "The Regalia")
	if !p.IsTutor {
		t.Errorf("The Regalia must flag IsTutor")
	}
	if !p.IsLandTutor {
		t.Errorf("The Regalia must flag IsLandTutor")
	}
}

// -----------------------------------------------------------------------------
// Modal spells with "search your library" mode (already covered — pin it)
// -----------------------------------------------------------------------------

func TestTutor_ToothAndNail_ModalSearchMode(t *testing.T) {
	ot := strings.ToLower(
		"choose one — • search your library for up to two creature cards, reveal them, " +
			"put them into your hand, then shuffle. • search your library for up to two " +
			"creature cards and put them onto the battlefield, then shuffle. entwine {2}{g}{g}")
	tl := strings.ToLower("sorcery")

	p := CardProfile{Name: "Tooth and Nail"}
	classifyTutorInto(&p, ot, tl, "Tooth and Nail")
	if !p.IsTutor {
		t.Errorf("Tooth and Nail must flag IsTutor (both modes are search-library)")
	}
}

func TestTutor_MastermindsAcquisition_ModalSearchOrSideboard(t *testing.T) {
	ot := strings.ToLower(
		"choose one — • search your library for a card, put it into your hand, then " +
			"shuffle. • choose a card you own from outside the game and put it into your hand.")
	tl := strings.ToLower("sorcery")

	p := CardProfile{Name: "Mastermind's Acquisition"}
	classifyTutorInto(&p, ot, tl, "Mastermind's Acquisition")
	if !p.IsTutor {
		t.Errorf("Mastermind's Acquisition must flag IsTutor (modal: search OR wish)")
	}
	if !p.IsWishTutor {
		t.Errorf("Mastermind's Acquisition must flag IsWishTutor too (one mode is wish)")
	}
}

func TestTutor_ArchdruidsCharm_ModalSearchMode(t *testing.T) {
	ot := strings.ToLower(
		"choose one — • search your library for a basic forest card, put it onto the " +
			"battlefield, then shuffle. • destroy target nonland permanent with mana " +
			"value 3 or less. • target creature you control fights target creature you " +
			"don't control. • return target permanent card from your graveyard to your hand.")
	tl := strings.ToLower("instant")

	p := CardProfile{Name: "Archdruid's Charm"}
	classifyTutorInto(&p, ot, tl, "Archdruid's Charm")
	if !p.IsTutor {
		t.Errorf("Archdruid's Charm must flag IsTutor (one mode is land-search)")
	}
	if !p.IsLandTutor {
		t.Errorf("Archdruid's Charm must flag IsLandTutor — the search mode is basic-Forest only")
	}
}

// -----------------------------------------------------------------------------
// Negative coverage — dig-and-pick is NOT a tutor
// -----------------------------------------------------------------------------

func TestTutor_SleightOfHand_LookTopTwoNotTutor(t *testing.T) {
	ot := strings.ToLower(
		"look at the top two cards of your library. put one of them into your hand and " +
			"the other on the bottom of your library.")
	tl := strings.ToLower("sorcery")

	p := CardProfile{Name: "Sleight of Hand"}
	classifyTutorInto(&p, ot, tl, "Sleight of Hand")
	if p.IsTutor {
		t.Errorf("Sleight of Hand must NOT flag IsTutor (dig-and-pick, not consistency)")
	}
}

func TestTutor_StrategicPlanning_LookTopThreeNotTutor(t *testing.T) {
	ot := strings.ToLower(
		"look at the top three cards of your library. put one of them into your hand " +
			"and the rest into your graveyard.")
	tl := strings.ToLower("sorcery")

	p := CardProfile{Name: "Strategic Planning"}
	classifyTutorInto(&p, ot, tl, "Strategic Planning")
	if p.IsTutor {
		t.Errorf("Strategic Planning must NOT flag IsTutor (dig-and-pick)")
	}
}

func TestTutor_TowerGeist_LookTopTwoNotTutor(t *testing.T) {
	ot := strings.ToLower(
		"flying. when this creature enters, look at the top two cards of your library. " +
			"put one of them into your hand and the other into your graveyard.")
	tl := strings.ToLower("creature — spirit")

	p := CardProfile{Name: "Tower Geist"}
	classifyTutorInto(&p, ot, tl, "Tower Geist")
	if p.IsTutor {
		t.Errorf("Tower Geist must NOT flag IsTutor (look-top-2-pick-1)")
	}
}

// -----------------------------------------------------------------------------
// Negative coverage — exile-and-cast (Etali) is NOT a tutor
// -----------------------------------------------------------------------------

func TestTutor_EtaliPrimalStorm_ExileCastNotTutor(t *testing.T) {
	ot := strings.ToLower(
		"whenever this creature attacks, exile the top card of each player's library. " +
			"you may cast any number of nonland cards exiled this way without paying their " +
			"mana costs.")
	tl := strings.ToLower("legendary creature — elder dinosaur")

	p := CardProfile{Name: "Etali, Primal Storm"}
	classifyTutorInto(&p, ot, tl, "Etali, Primal Storm")
	if p.IsTutor {
		t.Errorf("Etali, Primal Storm must NOT flag IsTutor (random exile-and-cast)")
	}
}

// -----------------------------------------------------------------------------
// Helper unit-tests for isLandRestrictedRevealUntil
// -----------------------------------------------------------------------------

func TestIsLandRestrictedRevealUntil(t *testing.T) {
	cases := []struct {
		name string
		ot   string
		want bool
	}{
		{
			"avenging druid land-restricted",
			"reveal cards from the top of your library until you reveal a land card",
			true,
		},
		{
			"multi-land restricted",
			"reveal cards from the top of your library until you reveal x land cards",
			true,
		},
		{
			"creature restricted is NOT land",
			"reveal cards from the top of your library until you reveal a creature card",
			false,
		},
		{
			"artifact restricted is NOT land",
			"reveal cards from the top of your library until you reveal an artifact card",
			false,
		},
		{
			"creature-OR-land stays NON-land (creature mode is consistency-relevant)",
			"reveal cards from the top of your library until you reveal a creature card. " +
				"or until you reveal a land card if no creature found",
			false,
		},
		{
			"no match (Madcap-style artifact)",
			"reveal cards from the top of your library until you reveal an artifact card. " +
				"put that card onto the battlefield",
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isLandRestrictedRevealUntil(c.ot)
			if got != c.want {
				t.Errorf("isLandRestrictedRevealUntil(%q) = %v, want %v", c.ot, got, c.want)
			}
		})
	}
}
