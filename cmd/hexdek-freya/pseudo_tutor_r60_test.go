package main

import (
	"testing"
)

// pseudo_tutor_r60_test.go — pins the r60 RolePseudoTutor detector.
//
// "Pseudo-tutors" are mechanics that consistently surface a specific card
// without being a literal search-library / reveal-until tutor:
//
//   - Cascade (Bloodbraid Elf, Maelstrom Wanderer): tutors a cheaper card
//     from the top of the library and casts it for free.
//   - Transmute (Dimir House Guard, Drift of Phantasms, Muddle the Mixture):
//     activated discard-this-card to tutor a same-MV card. ALSO trips the
//     hard-tutor flag via the embedded "search your library" clause, so
//     these cards expectedly carry BOTH Tutor and PseudoTutor tags.
//   - Companion (Yorion, Lurrus, Jegantha): one-shot wish for the named
//     companion from outside the game.
//   - Unearth (Hellspark Elemental, Anathemancer, Corpse Connoisseur):
//     graveyard-as-hand recursion that treats the graveyard as a tutor
//     surface for the combo piece.
//
// Negative coverage: cards that grant cascade / unearth (Yidris) or
// reference these mechanics in unrelated prose must NOT flag.

func tagsForOracle(name, ot, tl string, cmc int) []RoleTag {
	p := CardProfile{Name: name, TypeLine: tl, CMC: cmc}
	classifyTutorInto(&p, lowerForTest(ot), lowerForTest(tl), name)
	return TagCardRole(name, ot, tl, "", cmc, p)
}

func lowerForTest(s string) string {
	// keep identical case-folding behavior as analysis.go's call site —
	// avoid pulling strings import noise into the test file proper.
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}

// hasRole helper is defined in roles_recursion_r60_test.go and reused here.

// -----------------------------------------------------------------------------
// Cascade
// -----------------------------------------------------------------------------

func TestPseudoTutor_BloodbraidElf_Cascade(t *testing.T) {
	ot := "Haste\nCascade (When you cast this spell, exile cards from the top of your library until you exile a nonland card that costs less. You may cast it. Put the exiled cards on the bottom in a random order.)"
	tags := tagsForOracle("Bloodbraid Elf", ot, "Creature — Elf Berserker", 4)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Bloodbraid Elf must flag RolePseudoTutor (cascade), got %v", tags)
	}
}

func TestPseudoTutor_MaelstromWanderer_DoubleCascade(t *testing.T) {
	ot := "Eminence — Creatures you control have haste as long as Maelstrom Wanderer is in the command zone or on the battlefield.\nCascade, cascade (When you cast this spell, exile cards from the top of your library until you exile a nonland card that costs less. You may cast it. Put the exiled cards on the bottom of your library in a random order.)"
	tags := tagsForOracle("Maelstrom Wanderer", ot, "Legendary Creature — Elemental", 8)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Maelstrom Wanderer must flag RolePseudoTutor (double cascade), got %v", tags)
	}
}

// -----------------------------------------------------------------------------
// Transmute — also IsTutor via embedded "search your library"
// -----------------------------------------------------------------------------

func TestPseudoTutor_DimirHouseGuard_Transmute(t *testing.T) {
	ot := "Fear\nSacrifice another creature: Regenerate Dimir House Guard.\nTransmute {1}{B}{B} ({1}{B}{B}, Discard this card: Search your library for a card with the same mana value as this card, reveal it, put it into your hand, then shuffle. Transmute only as a sorcery.)"
	tl := "Creature — Human Mercenary"
	tags := tagsForOracle("Dimir House Guard", ot, tl, 4)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Dimir House Guard must flag RolePseudoTutor (transmute), got %v", tags)
	}
	if !hasRole(tags, RoleTutor) {
		t.Errorf("Dimir House Guard must ALSO flag RoleTutor — transmute embeds 'search your library' (double-tag is intentional), got %v", tags)
	}
}

func TestPseudoTutor_DriftOfPhantasms_Transmute(t *testing.T) {
	ot := "Defender, flying\nTransmute {1}{U}{U} ({1}{U}{U}, Discard this card: Search your library for a card with the same mana value as this card, reveal it, put it into your hand, then shuffle. Transmute only as a sorcery.)"
	tl := "Creature — Spirit"
	tags := tagsForOracle("Drift of Phantasms", ot, tl, 5)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Drift of Phantasms must flag RolePseudoTutor, got %v", tags)
	}
}

// -----------------------------------------------------------------------------
// Companion
// -----------------------------------------------------------------------------

func TestPseudoTutor_Yorion_Companion(t *testing.T) {
	ot := "Companion — Your starting deck contains at least twenty cards more than the minimum deck size.\nFlash\nFlying\nWhen Yorion, Sky Nomad enters, exile any number of other non-token, non-Aura permanents you own and control. Return those cards to the battlefield at the beginning of the next end step."
	tl := "Legendary Creature — Bird Serpent"
	tags := tagsForOracle("Yorion, Sky Nomad", ot, tl, 5)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Yorion must flag RolePseudoTutor (companion), got %v", tags)
	}
}

func TestPseudoTutor_Lurrus_Companion(t *testing.T) {
	ot := "Companion — Each permanent card in your starting deck has mana value 2 or less.\nDuring each of your turns, you may cast one permanent spell with mana value 2 or less from your graveyard."
	tl := "Legendary Creature — Cat Nightmare"
	tags := tagsForOracle("Lurrus of the Dream-Den", ot, tl, 3)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Lurrus must flag RolePseudoTutor (companion), got %v", tags)
	}
}

// -----------------------------------------------------------------------------
// Unearth
// -----------------------------------------------------------------------------

func TestPseudoTutor_HellsparkElemental_Unearth(t *testing.T) {
	ot := "Trample, haste\nAt the beginning of the end step, sacrifice Hellspark Elemental.\nUnearth {R} ({R}: Return this card from your graveyard to the battlefield. It gains haste. Exile it at the beginning of the next end step or if it would leave the battlefield. Unearth only as a sorcery.)"
	tl := "Creature — Elemental"
	tags := tagsForOracle("Hellspark Elemental", ot, tl, 2)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Hellspark Elemental must flag RolePseudoTutor (unearth), got %v", tags)
	}
}

func TestPseudoTutor_CorpseConnoisseur_Unearth(t *testing.T) {
	ot := "When Corpse Connoisseur enters, you may search your library for a creature card, put it into your graveyard, then shuffle.\nUnearth {3}{B}"
	tl := "Creature — Zombie Wizard"
	tags := tagsForOracle("Corpse Connoisseur", ot, tl, 5)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Corpse Connoisseur must flag RolePseudoTutor (unearth), got %v", tags)
	}
}

// -----------------------------------------------------------------------------
// Negative coverage — granted keywords must NOT trip RolePseudoTutor
// -----------------------------------------------------------------------------

func TestPseudoTutor_Yidris_GrantsCascade_NotFlagged(t *testing.T) {
	ot := "Menace\nWhenever you deal combat damage to a player with Yidris, Maelstrom Wielder, until end of turn, spells you cast from your hand have cascade."
	tl := "Legendary Creature — Zombie Minotaur"
	tags := tagsForOracle("Yidris, Maelstrom Wielder", ot, tl, 5)
	if hasRole(tags, RolePseudoTutor) {
		t.Errorf("Yidris must NOT flag RolePseudoTutor — it GRANTS cascade, doesn't have it. tags=%v", tags)
	}
}

func TestPseudoTutor_GrantedUnearth_NotFlagged(t *testing.T) {
	// Hypothetical "equipped creature has unearth {1}{B}" — same anti-grant
	// pattern as Yidris but for unearth. Pin defensively so a future graveyard-
	// theme equipment doesn't false-fire.
	ot := "Equipped creature has unearth {1}{B}.\nEquip {2}"
	tl := "Artifact — Equipment"
	tags := tagsForOracle("Hypothetical Unearth Granter", ot, tl, 2)
	if hasRole(tags, RolePseudoTutor) {
		t.Errorf("Granted-unearth equipment must NOT flag RolePseudoTutor, got %v", tags)
	}
}

// -----------------------------------------------------------------------------
// Negative coverage — vanilla cards must not trip
// -----------------------------------------------------------------------------

func TestPseudoTutor_VanillaCreature_NotFlagged(t *testing.T) {
	ot := ""
	tl := "Creature — Bear"
	tags := tagsForOracle("Grizzly Bears", ot, tl, 2)
	if hasRole(tags, RolePseudoTutor) {
		t.Errorf("Grizzly Bears must NOT flag RolePseudoTutor, got %v", tags)
	}
}

func TestPseudoTutor_PlainsWithoutKeyword_NotFlagged(t *testing.T) {
	ot := "({T}: Add {W}.)"
	tl := "Basic Land — Plains"
	tags := tagsForOracle("Plains", ot, tl, 0)
	if hasRole(tags, RolePseudoTutor) {
		t.Errorf("Plains must NOT flag RolePseudoTutor, got %v", tags)
	}
}

// -----------------------------------------------------------------------------
// Direct isPseudoTutor unit tests
// -----------------------------------------------------------------------------

func TestIsPseudoTutor_DirectCases(t *testing.T) {
	cases := []struct {
		name string
		ot   string
		want bool
	}{
		{"cascade bare keyword", "Haste\nCascade", true},
		{"cascade, cascade", "Cascade, cascade", true},
		{"transmute with cost", "Transmute {1}{B}{B}", true},
		{"unearth with cost", "Unearth {R}", true},
		{"companion em-dash", "Companion — Your starting deck contains only basic lands.", true},
		{"companion of (older wording)", "Companion of Jegantha, the Wellspring.", true},
		{"granted cascade — Yidris", "spells you cast from your hand have cascade.", false},
		{"granted unearth", "Equipped creature has unearth {1}{B}.", false},
		{"vanilla creature", "Flying, vigilance", false},
		{"empty text", "", false},
		{"mention in prose only", "Whenever a card with cascade is exiled, draw a card.", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isPseudoTutor(c.ot)
			if got != c.want {
				t.Errorf("isPseudoTutor(%q) = %v, want %v", c.ot, got, c.want)
			}
		})
	}
}
