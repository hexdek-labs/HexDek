package main

import (
	"testing"
)

// suspend_pseudo_tutor_r60_test.go — pins the r60 suspend extension to the
// RolePseudoTutor detector and the new RoleSuspendFinisher tag.
//
// Suspend (keyword: "Suspend N—{cost}") lets a player exile a card from
// hand and cast it for free N turns later. Two role-tag consequences:
//
//   1. RolePseudoTutor on every suspend card. The suspend timer is the
//      consistency engine — Lotus Bloom on turn 1 is "tutored" 3 mana on
//      turn 4, Search for Tomorrow is a "tutored" land drop on turn 3,
//      etc. Same mechanic-family slot as cascade / transmute / unearth /
//      companion.
//
//   2. RoleSuspendFinisher on the subset whose body is a game-swinging
//      mass effect — Living End (mass reanimate from graveyards),
//      Restore Balance (equalizing sac), Hypergenesis-adjacent
//      sweepers. The deck IS the finisher; the suspend timer is the
//      delivery vector. This is a SEPARATE tag from PseudoTutor because
//      archetype classifiers want a distinct signal for "this card
//      anchors a Suspend deck" vs "this card is filler".
//
// Vanilla suspend value cards (Lotus Bloom, Search for Tomorrow,
// Ancestral Vision, Wheel of Fate) get PseudoTutor but NOT
// SuspendFinisher. The negative coverage pins that boundary.

// -----------------------------------------------------------------------------
// Suspend — pseudo-tutor (the suspend card itself "tutors" its future cast)
// -----------------------------------------------------------------------------

func TestPseudoTutor_LotusBloom_Suspend(t *testing.T) {
	ot := "Suspend 3—{0} (Rather than cast this card from your hand, you may pay {0} and exile it with three time counters on it. At the beginning of your upkeep, remove a time counter. When the last is removed, cast it without paying its mana cost.)\n{T}, Sacrifice Lotus Bloom: Add three mana of any one color."
	tl := "Artifact"
	tags := tagsForOracle("Lotus Bloom", ot, tl, 0)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Lotus Bloom must flag RolePseudoTutor (suspend), got %v", tags)
	}
	if hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Lotus Bloom must NOT flag RoleSuspendFinisher — body is targeted ramp, not a mass swing. tags=%v", tags)
	}
}

func TestPseudoTutor_SearchForTomorrow_Suspend(t *testing.T) {
	ot := "Search your library for a basic land card, put it onto the battlefield, then shuffle.\nSuspend 2—{G} (Rather than cast this card from your hand, you may pay {G} and exile it with two time counters on it. At the beginning of your upkeep, remove a time counter. When the last is removed, cast it without paying its mana cost.)"
	tl := "Sorcery"
	tags := tagsForOracle("Search for Tomorrow", ot, tl, 3)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Search for Tomorrow must flag RolePseudoTutor (suspend), got %v", tags)
	}
	if hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Search for Tomorrow must NOT flag RoleSuspendFinisher — single land fetch, not a swing. tags=%v", tags)
	}
}

func TestPseudoTutor_AncestralVision_Suspend(t *testing.T) {
	ot := "Suspend 4—{U}\nTarget player draws three cards."
	tl := "Sorcery"
	tags := tagsForOracle("Ancestral Vision", ot, tl, 0)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Ancestral Vision must flag RolePseudoTutor (suspend), got %v", tags)
	}
	if hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Ancestral Vision must NOT flag RoleSuspendFinisher — pure card draw, not a swing. tags=%v", tags)
	}
}

func TestPseudoTutor_WheelOfFate_Suspend(t *testing.T) {
	ot := "Suspend 4—{R}\nEach player discards their hand, then draws seven cards."
	tl := "Sorcery"
	tags := tagsForOracle("Wheel of Fate", ot, tl, 0)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Wheel of Fate must flag RolePseudoTutor (suspend), got %v", tags)
	}
	if hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Wheel of Fate must NOT flag RoleSuspendFinisher — wheel is value, not a finisher. tags=%v", tags)
	}
}

// -----------------------------------------------------------------------------
// Suspend-finisher — pseudo-tutor AND finisher (the deck IS this card)
// -----------------------------------------------------------------------------

func TestSuspendFinisher_LivingEnd(t *testing.T) {
	ot := "Suspend 3—{2}{B}{B}\nExile all creature cards from all graveyards. Each player who had a creature card exiled this way puts all creature cards from their graveyard onto the battlefield, then sacrifices all creatures they control that weren't put onto the battlefield this way."
	tl := "Sorcery"
	tags := tagsForOracle("Living End", ot, tl, 0)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Living End must flag RolePseudoTutor (suspend), got %v", tags)
	}
	if !hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Living End must flag RoleSuspendFinisher — mass reanimate from all graveyards. tags=%v", tags)
	}
}

func TestSuspendFinisher_RestoreBalance(t *testing.T) {
	ot := "Suspend 6—{W}\nEach player chooses a number of lands they control equal to the number of lands controlled by the player who controls the fewest, then sacrifices the rest. Players discard cards and sacrifice creatures the same way."
	tl := "Sorcery"
	tags := tagsForOracle("Restore Balance", ot, tl, 0)
	if !hasRole(tags, RolePseudoTutor) {
		t.Errorf("Restore Balance must flag RolePseudoTutor (suspend), got %v", tags)
	}
	if !hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Restore Balance must flag RoleSuspendFinisher — equalizing sac (sacrifices the rest). tags=%v", tags)
	}
}

// -----------------------------------------------------------------------------
// Negative coverage
// -----------------------------------------------------------------------------

func TestSuspend_GrantedSuspend_NotFlagged(t *testing.T) {
	// Hypothetical "each instant and sorcery card in your hand has suspend 3—{0}"
	// — anti-grant guard must prevent both PseudoTutor and SuspendFinisher.
	ot := "Each instant and sorcery card in your hand has suspend 3—{0}."
	tl := "Enchantment"
	tags := tagsForOracle("Hypothetical Suspend Granter", ot, tl, 4)
	if hasRole(tags, RolePseudoTutor) {
		t.Errorf("Granted-suspend enchantment must NOT flag RolePseudoTutor, got %v", tags)
	}
	if hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Granted-suspend enchantment must NOT flag RoleSuspendFinisher, got %v", tags)
	}
}

func TestSuspend_ProseMention_NotFlagged(t *testing.T) {
	// Card text that talks about suspend without itself having the keyword.
	ot := "Whenever you cast a spell with suspend, scry 1."
	tl := "Enchantment"
	tags := tagsForOracle("Suspend Payoff", ot, tl, 2)
	if hasRole(tags, RolePseudoTutor) {
		t.Errorf("Suspend-payoff card must NOT flag RolePseudoTutor (prose mention only), got %v", tags)
	}
}

func TestSuspendFinisher_VanillaCreature_NotFlagged(t *testing.T) {
	tags := tagsForOracle("Grizzly Bears", "", "Creature — Bear", 2)
	if hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Grizzly Bears must NOT flag RoleSuspendFinisher, got %v", tags)
	}
}

func TestSuspendFinisher_BoardwipeWithoutSuspend_NotFlagged(t *testing.T) {
	// Wrath of God — mass effect but no suspend. Must NOT trip SuspendFinisher.
	ot := "Destroy all creatures. They can't be regenerated."
	tl := "Sorcery"
	tags := tagsForOracle("Wrath of God", ot, tl, 4)
	if hasRole(tags, RoleSuspendFinisher) {
		t.Errorf("Wrath of God must NOT flag RoleSuspendFinisher — no suspend keyword. tags=%v", tags)
	}
}

// -----------------------------------------------------------------------------
// Direct isPseudoTutor + isSuspendFinisher unit tests
// -----------------------------------------------------------------------------

func TestIsPseudoTutor_SuspendDirectCases(t *testing.T) {
	cases := []struct {
		name string
		ot   string
		want bool
	}{
		{"suspend with zero cost", "Suspend 3—{0}", true},
		{"suspend with colored cost", "Suspend 4—{U}", true},
		{"suspend with multi-color", "Suspend 6—{2}{W}{W}", true},
		{"granted suspend", "Each card has suspend 3—{0}.", false},
		{"prose mention", "Whenever you cast a spell with suspend, scry 1.", false},
		{"empty text", "", false},
		{"vanilla", "Flying, vigilance", false},
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

func TestIsSuspendFinisher_DirectCases(t *testing.T) {
	cases := []struct {
		name string
		ot   string
		want bool
	}{
		{
			"Living End — mass graveyard reanimate",
			"Suspend 3—{2}{B}{B}\nExile all creature cards from all graveyards. Each player who had a creature card exiled this way puts all creature cards from their graveyard onto the battlefield, then sacrifices all creatures.",
			true,
		},
		{
			"Restore Balance — sacrifices the rest",
			"Suspend 6—{W}\nEach player chooses a number of lands they control equal to the number of lands controlled by the player who controls the fewest, then sacrifices the rest.",
			true,
		},
		{
			"hypothetical suspend + mass burn",
			"Suspend 4—{R}\nThis spell deals damage to each creature and each player equal to the number of cards in your hand.",
			true,
		},
		{
			"hypothetical suspend + exile-all sweeper",
			"Suspend 5—{2}{W}\nExile all permanents.",
			true,
		},
		{
			"Lotus Bloom — suspend + ramp, NOT a finisher",
			"Suspend 3—{0}\n{T}, Sacrifice Lotus Bloom: Add three mana of any one color.",
			false,
		},
		{
			"Ancestral Vision — suspend + draw, NOT a finisher",
			"Suspend 4—{U}\nTarget player draws three cards.",
			false,
		},
		{
			"Search for Tomorrow — suspend + single land fetch, NOT a finisher",
			"Search your library for a basic land card, put it onto the battlefield, then shuffle.\nSuspend 2—{G}",
			false,
		},
		{
			"Wheel of Fate — suspend + wheel, NOT a finisher",
			"Suspend 4—{R}\nEach player discards their hand, then draws seven cards.",
			false,
		},
		{
			"granted suspend on mass effect — anti-grant guard",
			"Each instant card has suspend 3—{0}.\nDestroy all creatures.",
			false,
		},
		{
			"mass effect but no suspend — Wrath of God",
			"Destroy all creatures. They can't be regenerated.",
			false,
		},
		{
			"empty text",
			"",
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isSuspendFinisher(c.ot)
			if got != c.want {
				t.Errorf("isSuspendFinisher(%q) = %v, want %v", c.ot, got, c.want)
			}
		})
	}
}
