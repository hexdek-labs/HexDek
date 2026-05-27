package main

import (
	"testing"
)

// stax_detection_r60_test.go — r60 audit + refinement of isStax.
//
// Audit verdict (run against ~25 canonical Stax cards using their actual
// Scryfall oracle text):
//
//   CAUGHT pre-refinement (via existing patterns):
//     - Winter Orb / Static Orb / Stasis    ("can't untap", "players can't", "skip")
//     - Rule of Law / Eidolon of Rhetoric   ("each player can't")
//     - Spirit of the Labyrinth             ("can't draw")
//     - Drannith Magistrate / Grand Abolisher / Ethersworn Canonist
//       (via p.Effects "lock" populated by analysis.go from "can't cast")
//
//   MISSED pre-refinement (gaps the refinement closes):
//     - Trinisphere, Sphere of Resistance, Thalia Guardian of Thraben,
//       Lodestone Golem — tax cost wording. analysis.go's parser checks
//       `Contains(ot, "costs")` (plural-s), missing the "Spells cost {1}
//       more" canonical wording. Trinisphere uses an entirely different
//       "would cost less than" minimum-cost shape.
//     - Null Rod, Cursed Totem, Stony Silence, Linvala Keeper of Silence,
//       Pithing Needle, Phyrexian Revoker, Collector Ouphe — activated-
//       ability lock, "can't be activated".
//     - Blood Moon, Magus of the Moon — land-type lock.
//     - Containment Priest — ETB-exile lock.
//     - Hushwing Gryff, Tocatli Honor Guard, Torpor Orb — ETB-trigger lock.
//     - Smokestack, Tangle Wire — periodic each-player tax.
//     - Opposition Agent, Aven Mindcensor, Leonin Arbiter — search intercept.
//     - Archon of Emeria — "each opponent can't" (the dedicated arm
//       provides a tighter match than the lock-effect path).
//
// Negative coverage pins that the refined patterns don't false-fire on
// ramp / removal / tutor / counterspell cards that share substrings.

func assertStax(t *testing.T, name, ot string, want bool) {
	t.Helper()
	p := CardProfile{Name: name}
	got := isStax(p, lowerForTest(ot))
	if got != want {
		t.Errorf("isStax(%q) = %v, want %v\n  oracle: %s", name, got, want, ot)
	}
}

// -----------------------------------------------------------------------------
// Pre-existing detection — pin so the refinement doesn't regress
// -----------------------------------------------------------------------------

func TestStax_WinterOrb_PreExisting(t *testing.T) {
	assertStax(t, "Winter Orb",
		"Players can't untap more than one land during their untap steps.",
		true)
}

func TestStax_StaticOrb_PreExisting(t *testing.T) {
	assertStax(t, "Static Orb",
		"As long as Static Orb is untapped, players can't untap more than two permanents during their untap steps.",
		true)
}

func TestStax_Stasis_PreExisting(t *testing.T) {
	assertStax(t, "Stasis",
		"Players skip their untap steps.\nCumulative upkeep {U}",
		true)
}

func TestStax_RuleOfLaw_PreExisting(t *testing.T) {
	assertStax(t, "Rule of Law",
		"Each player can't cast more than one spell each turn.",
		true)
}

func TestStax_SpiritOfTheLabyrinth_PreExisting(t *testing.T) {
	assertStax(t, "Spirit of the Labyrinth",
		"Each player can't draw more than one card each turn.",
		true)
}

// -----------------------------------------------------------------------------
// Tax-cost gaps — Sphere of Resistance / Thalia / Lodestone / Trinisphere
// -----------------------------------------------------------------------------

func TestStax_SphereOfResistance_TaxCostFix(t *testing.T) {
	// Singular "cost" missed by analysis.go's "costs" parser check.
	assertStax(t, "Sphere of Resistance",
		"Spells cost {1} more to cast.",
		true)
}

func TestStax_ThaliaGuardian_TaxCostFix(t *testing.T) {
	assertStax(t, "Thalia, Guardian of Thraben",
		"First strike\nNoncreature spells cost {1} more to cast.",
		true)
}

func TestStax_LodestoneGolem_TaxCostFix(t *testing.T) {
	assertStax(t, "Lodestone Golem",
		"Noncreature spells cost {1} more to cast.",
		true)
}

func TestStax_Trinisphere_MinCostFix(t *testing.T) {
	// Different shape entirely — minimum cost. Caught via "would cost less than".
	assertStax(t, "Trinisphere",
		"As long as Trinisphere is untapped, each spell that would cost less than three mana to cast costs three mana to cast.",
		true)
}

// -----------------------------------------------------------------------------
// Activated-ability lock — Null Rod family
// -----------------------------------------------------------------------------

func TestStax_NullRod_CantBeActivated(t *testing.T) {
	assertStax(t, "Null Rod",
		"Activated abilities of artifacts can't be activated unless they're mana abilities.",
		true)
}

func TestStax_StonySilence_CantBeActivated(t *testing.T) {
	assertStax(t, "Stony Silence",
		"Activated abilities of artifacts can't be activated unless they're mana abilities.",
		true)
}

func TestStax_CursedTotem_CantBeActivated(t *testing.T) {
	assertStax(t, "Cursed Totem",
		"Activated abilities of creatures can't be activated.",
		true)
}

func TestStax_LinvalaKeeperOfSilence_CantBeActivated(t *testing.T) {
	assertStax(t, "Linvala, Keeper of Silence",
		"Flying\nActivated abilities of creatures your opponents control can't be activated.",
		true)
}

func TestStax_PithingNeedle_CantBeActivated(t *testing.T) {
	assertStax(t, "Pithing Needle",
		"As Pithing Needle enters, choose a card name.\nActivated abilities of sources with the chosen name can't be activated unless they're mana abilities.",
		true)
}

func TestStax_CollectorOuphe_CantBeActivated(t *testing.T) {
	assertStax(t, "Collector Ouphe",
		"Activated abilities of artifacts can't be activated unless they're mana abilities.",
		true)
}

// -----------------------------------------------------------------------------
// Land-type lock — Blood Moon family
// -----------------------------------------------------------------------------

func TestStax_BloodMoon_LandLock(t *testing.T) {
	assertStax(t, "Blood Moon",
		"Nonbasic lands are Mountains.",
		true)
}

func TestStax_MagusOfTheMoon_LandLock(t *testing.T) {
	assertStax(t, "Magus of the Moon",
		"Nonbasic lands are Mountains.",
		true)
}

// -----------------------------------------------------------------------------
// ETB-exile lock — Containment Priest
// -----------------------------------------------------------------------------

func TestStax_ContainmentPriest_ETBExile(t *testing.T) {
	assertStax(t, "Containment Priest",
		"Flash\nIf a nontoken creature would enter the battlefield and it wasn't cast, exile it instead.",
		true)
}

// -----------------------------------------------------------------------------
// ETB-trigger lock — Hushwing Gryff family
// -----------------------------------------------------------------------------

func TestStax_HushwingGryff_ETBTriggerLock(t *testing.T) {
	assertStax(t, "Hushwing Gryff",
		"Flash\nFlying\nCreature abilities that trigger when a creature enters don't trigger.",
		true)
}

func TestStax_TorporOrb_ETBTriggerLock(t *testing.T) {
	assertStax(t, "Torpor Orb",
		"Creature abilities that trigger when a creature enters don't trigger.",
		true)
}

// -----------------------------------------------------------------------------
// Periodic each-player tax — Smokestack / Tangle Wire
// -----------------------------------------------------------------------------

func TestStax_Smokestack_EachPlayerSac(t *testing.T) {
	assertStax(t, "Smokestack",
		"At the beginning of your upkeep, you may put a soot counter on Smokestack.\nAt the beginning of each player's upkeep, that player sacrifices a permanent for each soot counter on Smokestack.",
		true)
}

func TestStax_TangleWire_EachPlayerTap(t *testing.T) {
	assertStax(t, "Tangle Wire",
		"Fading 4\nAt the beginning of each player's upkeep, that player taps an untapped artifact, creature, or land they control for each fade counter on Tangle Wire.",
		true)
}

// -----------------------------------------------------------------------------
// Search intercept — Opposition Agent / Aven Mindcensor / Leonin Arbiter
// -----------------------------------------------------------------------------

func TestStax_OppositionAgent_SearchIntercept(t *testing.T) {
	assertStax(t, "Opposition Agent",
		"Flash\nYou control your opponents while they're searching their libraries.\nWhile an opponent is searching their library, they exile each card they find. You may play those cards for as long as they remain exiled, and you may spend mana as though it were mana of any color to cast those spells.\nIf an opponent would search a library, you control that player while they do.",
		true)
}

func TestStax_AvenMindcensor_SearchIntercept(t *testing.T) {
	assertStax(t, "Aven Mindcensor",
		"Flash\nFlying\nIf an opponent would search a library, that player searches the top four cards of that library instead.",
		true)
}

func TestStax_LeoninArbiter_SearchIntercept(t *testing.T) {
	assertStax(t, "Leonin Arbiter",
		"Players can't search libraries. Any player may pay {2} for that player to ignore this effect until end of turn.",
		true)
}

// -----------------------------------------------------------------------------
// Opponent-targeting locks — Drannith / Grand Abolisher / Archon
// -----------------------------------------------------------------------------

func TestStax_DranniMagistrate_OpponentsLock(t *testing.T) {
	assertStax(t, "Drannith Magistrate",
		"Your opponents can't cast spells from anywhere other than their hands.",
		true)
}

func TestStax_GrandAbolisher_OpponentsLock(t *testing.T) {
	assertStax(t, "Grand Abolisher",
		"Your opponents can't cast spells or activate abilities of artifacts, creatures, or enchantments during your turn.",
		true)
}

func TestStax_ArchonOfEmeria_EachOpponentLock(t *testing.T) {
	assertStax(t, "Archon of Emeria",
		"Flying\nEach opponent can't cast more than one spell each turn.\nNonbasic lands your opponents control enter the battlefield tapped.",
		true)
}

// -----------------------------------------------------------------------------
// Negative coverage — common false-positive candidates must NOT fire
// -----------------------------------------------------------------------------

func TestStax_DemonicTutor_NotStax(t *testing.T) {
	// "Search your library" must not trip the search-intercept arm
	// (which gates on "would search" framing).
	assertStax(t, "Demonic Tutor",
		"Search your library for a card, put that card into your hand, then shuffle.",
		false)
}

func TestStax_CounterspellInstant_NotStax(t *testing.T) {
	// "Counter target spell" doesn't share any stax phrasing.
	assertStax(t, "Counterspell",
		"Counter target spell.",
		false)
}

func TestStax_GrizzlyBears_NotStax(t *testing.T) {
	assertStax(t, "Grizzly Bears", "", false)
}

func TestStax_RampantGrowth_NotStax(t *testing.T) {
	assertStax(t, "Rampant Growth",
		"Search your library for a basic land card, put it onto the battlefield tapped, then shuffle.",
		false)
}

func TestStax_LightningBolt_NotStax(t *testing.T) {
	assertStax(t, "Lightning Bolt",
		"Lightning Bolt deals 3 damage to any target.",
		false)
}

func TestStax_DoublingSeason_NotStax(t *testing.T) {
	// "Would enter the battlefield" without "exile it instead" must NOT trip
	// the ETB-exile-lock arm.
	assertStax(t, "Doubling Season",
		"If an effect would create one or more tokens under your control, it creates twice that many of those tokens instead.\nIf an effect would put one or more counters on a permanent you control, it puts twice that many of those counters on that permanent instead.",
		false)
}

func TestStax_ManaVault_NotStax(t *testing.T) {
	// Mana Vault is a mana rock, not stax. The existing "don't untap"
	// pattern correctly does NOT fire here — Mana Vault uses "doesn't
	// untap" (third-person singular contraction), which is wording the
	// pre-r60 detector deliberately avoided to keep self-don't-untap
	// rocks (Mana Vault, Grim Monolith, Basalt Monolith) out of the
	// stax bucket. Pin defensively.
	assertStax(t, "Mana Vault",
		"Mana Vault doesn't untap during your untap step.\nAt the beginning of your upkeep, if Mana Vault is tapped, Mana Vault deals 1 damage to you.\n{T}: Add {C}{C}{C}.",
		false)
}

// -----------------------------------------------------------------------------
// Role-tag integration — confirm RoleStax flows through TagCardRole
// -----------------------------------------------------------------------------

func TestStax_RoleTag_NullRod(t *testing.T) {
	tags := tagsForOracle("Null Rod",
		"Activated abilities of artifacts can't be activated unless they're mana abilities.",
		"Artifact", 2)
	if !hasRole(tags, RoleStax) {
		t.Errorf("Null Rod must carry RoleStax, got %v", tags)
	}
}

func TestStax_RoleTag_Thalia(t *testing.T) {
	tags := tagsForOracle("Thalia, Guardian of Thraben",
		"First strike\nNoncreature spells cost {1} more to cast.",
		"Legendary Creature — Human Soldier", 2)
	if !hasRole(tags, RoleStax) {
		t.Errorf("Thalia must carry RoleStax, got %v", tags)
	}
}

func TestStax_RoleTag_BloodMoon(t *testing.T) {
	tags := tagsForOracle("Blood Moon",
		"Nonbasic lands are Mountains.",
		"Enchantment", 3)
	if !hasRole(tags, RoleStax) {
		t.Errorf("Blood Moon must carry RoleStax, got %v", tags)
	}
}
