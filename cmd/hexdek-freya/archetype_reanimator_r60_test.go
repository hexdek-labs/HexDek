package main

import (
	"testing"
)

// R60 Reanimator fingerprint — wires the new RoleRecursion tag
// (shipped in PR #466) into the archetype fingerprint so Reanimator-
// class decks score against the role density rather than relying
// solely on bag-of-words graveyardCount + selfMillCount.
//
// Fixture decks model the three canonical Reanimator commanders:
// Meren of Clan Nel Toth (sacrifice + experience-counter
// reanimation), Muldrotha the Gravetide (play-from-graveyard
// goodstuff), Karador Ghost Chieftain (creature-recursion tribal).
// Each fixture provides:
//   - 6+ recursion pieces (RoleRecursion ≥ 0.05 to pass the
//     strengthened Require gate)
//   - 2+ self-mill enablers (graveyardCount + selfMillCount gates)
//   - representative role counts for Draw / Tutor / Threat / Ramp
//     so the deck lands close to the 0.12/0.10/0.08/0.08/0.08
//     fingerprint target
//
// Plus a counter-fixture (pure-mill Bruvac-style deck) that should
// NOT misclassify as Reanimator after the gate strengthens.

// reanimatorFixture builds a minimal-but-representative Reanimator
// deck around the named commander. The recursion cards are real
// canonical pieces; threat / ramp / draw filler is generic so the
// fingerprint match is driven by the recursion+threat density, not
// by deck-specific synergies.
type reanimatorFixture struct {
	commander       string
	commanderOracle string
	recursionCards  []string // each pre-stamped IsRecursion=true
	threatCards     []string // big reanimation targets
	rampCards       []string
	drawCards       []string
	tutorCards      []string
	millCards       []string // self-mill enablers
	filler          int      // generic Utility-tagged cards to pad to 99
}

func (f reanimatorFixture) build() ([]CardProfile, map[string]string, map[RoleTag]int) {
	profiles := []CardProfile{}
	oracle := map[string]string{}
	roleCounts := map[RoleTag]int{
		RoleLand: 36, // standard 36-land manabase
	}

	// Commander has both Recursion (graveyard text) and Threat
	// (legendary creature finisher) roles.
	profiles = append(profiles, CardProfile{
		Name: f.commander, TypeLine: "Legendary Creature", CMC: 5,
		IsRecursion: true,
	})
	oracle[f.commander] = f.commanderOracle
	roleCounts[RoleRecursion]++
	roleCounts[RoleThreat]++

	for _, name := range f.recursionCards {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature", CMC: 3,
			IsRecursion: true,
		})
		oracle[name] = "return target creature card from your graveyard to the battlefield"
		roleCounts[RoleRecursion]++
	}
	for _, name := range f.threatCards {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature", CMC: 7,
			IsWinCon: true,
		})
		oracle[name] = "trample"
		roleCounts[RoleThreat]++
	}
	for _, name := range f.rampCards {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Sorcery", CMC: 2,
			Produces: []ResourceType{ResMana},
			Effects:  []string{"land_fetch"},
		})
		oracle[name] = "search your library for a basic land"
		roleCounts[RoleRamp]++
	}
	for _, name := range f.drawCards {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Sorcery", CMC: 3,
			Produces: []ResourceType{ResCard},
		})
		oracle[name] = "draw three cards"
		roleCounts[RoleDraw]++
	}
	for _, name := range f.tutorCards {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Sorcery", CMC: 2,
			IsTutor: true,
		})
		oracle[name] = "search your library for a card"
		roleCounts[RoleTutor]++
	}
	for _, name := range f.millCards {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Sorcery", CMC: 2,
			Effects: []string{"self_mill"},
		})
		oracle[name] = "mill three cards"
		roleCounts[RoleUtility]++
	}
	// Filler — generic utility cards padding to ~99 total.
	for i := 0; i < f.filler; i++ {
		profiles = append(profiles, CardProfile{
			Name: "Filler " + itoaCard(i), TypeLine: "Artifact", CMC: 2,
		})
		roleCounts[RoleUtility]++
	}
	return profiles, oracle, roleCounts
}

func itoaCard(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// ---------------------------------------------------------------------
// Concrete reanimator decks classify as Reanimator
// ---------------------------------------------------------------------

func TestArchetype_MerenReanimator_Classifies(t *testing.T) {
	// Meren of Clan Nel Toth — sacrifice + experience-counter
	// reanimation. Heavy on creature-recursion pieces (Eternal
	// Witness, Reveillark, Karmic Guide), self-mill enablers
	// (Stitcher's Supplier, Satyr Wayfinder), and big targets
	// (Razaketh, Lord of the Void, Sheoldred).
	fx := reanimatorFixture{
		commander:       "Meren of Clan Nel Toth",
		commanderOracle: "at the beginning of your end step, choose target creature card in your graveyard. if you have experience counters equal to or greater than that card's mana value, return that card to the battlefield. otherwise, put it into your hand.",
		recursionCards: []string{
			"Eternal Witness", "Reveillark", "Karmic Guide",
			"Reanimate", "Animate Dead", "Necromancy",
			"Victimize", "Sevinne's Reclamation",
			"Phyrexian Reclamation", "Bone Shards",
			"Sun Titan", "Regrowth",
		},
		threatCards: []string{
			"Razaketh, the Foulblooded", "Lord of the Void", "Sheoldred", "Massacre Wurm",
		},
		rampCards:  []string{"Cultivate", "Kodama's Reach", "Sol Ring", "Arcane Signet"},
		drawCards:  []string{"Harmonize", "Phyrexian Arena", "Read the Bones", "Sign in Blood"},
		tutorCards: []string{"Demonic Tutor", "Vampiric Tutor", "Buried Alive", "Entomb"},
		millCards:  []string{"Stitcher's Supplier", "Satyr Wayfinder", "Mulch", "Grisly Salvage"},
		filler:     35,
	}
	profiles, oracle, roleCounts := fx.build()
	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)

	if ac.Primary != "Reanimator" {
		t.Errorf("Meren fixture: want Primary=Reanimator, got %q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

func TestArchetype_MuldrothaReanimator_Classifies(t *testing.T) {
	// Muldrotha the Gravetide — play-one-of-each-type-from-grave
	// goodstuff. Heavy on small recursive value (Eternal Witness,
	// Sakura-Tribe Elder, Crucible of Worlds class) and synergy
	// with self-mill / fetch.
	fx := reanimatorFixture{
		commander:       "Muldrotha, the Gravetide",
		commanderOracle: "during each of your turns, you may play a land and cast a permanent spell of each permanent type from your graveyard.",
		recursionCards: []string{
			"Eternal Witness", "Regrowth", "Reanimate", "Animate Dead",
			"Sun Titan", "Crucible of Worlds", "Ramunap Excavator",
			"World Shaper", "Splendid Reclamation", "Life from the Loam",
		},
		threatCards: []string{"Massacre Wurm", "Avenger of Zendikar", "Hornet Queen"},
		rampCards:   []string{"Cultivate", "Kodama's Reach", "Sol Ring", "Arcane Signet", "Sakura-Tribe Elder"},
		drawCards:   []string{"Harmonize", "Sylvan Library", "Beast Whisperer", "Guardian Project"},
		tutorCards:  []string{"Demonic Tutor", "Buried Alive", "Entomb", "Worldly Tutor"},
		millCards:   []string{"Mulch", "Grisly Salvage", "Satyr Wayfinder", "Stitcher's Supplier"},
		filler:      34,
	}
	profiles, oracle, roleCounts := fx.build()
	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)

	if ac.Primary != "Reanimator" {
		t.Errorf("Muldrotha fixture: want Primary=Reanimator, got %q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

func TestArchetype_KaradorReanimator_Classifies(t *testing.T) {
	// Karador Ghost Chieftain — once-per-turn creature-from-grave
	// caster. Heavy on small ETB creatures + sac outlets.
	fx := reanimatorFixture{
		commander:       "Karador, Ghost Chieftain",
		commanderOracle: "during each of your turns, you may cast one creature spell from your graveyard.",
		recursionCards: []string{
			"Eternal Witness", "Karmic Guide", "Reveillark",
			"Sun Titan", "Fiend Hunter", "Mwonvuli Beast Tracker",
			"Reanimate", "Animate Dead", "Unburial Rites",
			"Renegade Rallier", "Regrowth",
		},
		threatCards: []string{"Razaketh, the Foulblooded", "Massacre Wurm", "Avenger of Zendikar"},
		rampCards:   []string{"Cultivate", "Kodama's Reach", "Sol Ring", "Arcane Signet"},
		drawCards:   []string{"Harmonize", "Phyrexian Arena", "Skullclamp", "Sign in Blood"},
		tutorCards:  []string{"Demonic Tutor", "Birthing Pod", "Buried Alive", "Entomb"},
		millCards:   []string{"Stitcher's Supplier", "Satyr Wayfinder", "Mulch", "Grisly Salvage"},
		filler:      35,
	}
	profiles, oracle, roleCounts := fx.build()
	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)

	if ac.Primary != "Reanimator" {
		t.Errorf("Karador fixture: want Primary=Reanimator, got %q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

// ---------------------------------------------------------------------
// Counter-fixture: pure-mill deck must NOT misclassify
// ---------------------------------------------------------------------

func TestArchetype_PureMillDeck_NotMisclassifiedAsReanimator(t *testing.T) {
	// Pre-r60 fingerprint: graveyardCount >= 6 && selfMillCount >= 2
	// — a Bruvac mill deck with 8+ mill payoffs but 0 recursion
	// pieces would pass the gate via the self_mill effect tag
	// inflating graveyardCount. After r60, the RoleRecursion floor
	// (0.05) prevents this misclassification.
	profiles := []CardProfile{
		{Name: "Bruvac the Grandiloquent", TypeLine: "Legendary Creature", CMC: 4},
	}
	oracle := map[string]string{
		"Bruvac the Grandiloquent": "if an opponent would mill one or more cards, they mill twice that many cards instead.",
	}
	roleCounts := map[RoleTag]int{
		RoleLand: 36,
	}
	// 10 mill payoffs — drives graveyardCount via self_mill effect
	// AND selfMillCount via oracle "mill"
	millerNames := []string{"Mind Funeral", "Glimpse the Unthinkable", "Maddening Cacophony",
		"Mind Grind", "Traumatize", "Tasha's Hideous Laughter",
		"Bruvac's Spell 1", "Bruvac's Spell 2", "Bruvac's Spell 3", "Bruvac's Spell 4"}
	for _, name := range millerNames {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Sorcery", CMC: 3,
			Effects: []string{"self_mill"},
		})
		oracle[name] = "target opponent mills " + name + " cards"
		roleCounts[RoleUtility]++
	}
	// Generic filler — NO recursion pieces at all.
	for i := 0; i < 50; i++ {
		profiles = append(profiles, CardProfile{
			Name: "Generic " + itoaCard(i), TypeLine: "Artifact", CMC: 2,
		})
		roleCounts[RoleUtility]++
	}
	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)

	if ac.Primary == "Reanimator" {
		t.Errorf("pure-mill deck must NOT classify as Reanimator (no recursion pieces); got Primary=%q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

// ---------------------------------------------------------------------
// Direct fingerprint structure / require-gate invariants
// ---------------------------------------------------------------------

func TestReanimatorFingerprint_RatiosIncludeRecursion(t *testing.T) {
	var fp *archetypeFingerprint
	for i := range archetypeFingerprints {
		if archetypeFingerprints[i].Name == "Reanimator" {
			fp = &archetypeFingerprints[i]
			break
		}
	}
	if fp == nil {
		t.Fatal("Reanimator fingerprint missing from archetypeFingerprints")
	}
	recRatio, ok := fp.Ratios[RoleRecursion]
	if !ok {
		t.Errorf("Reanimator fingerprint missing RoleRecursion ratio entry: %v", fp.Ratios)
	}
	if recRatio < 0.08 {
		t.Errorf("Reanimator RoleRecursion ratio too low: got %.3f, want ≥0.08", recRatio)
	}
}

func TestReanimatorFingerprint_RequireGateNeedsRoleRecursion(t *testing.T) {
	// Pin the strengthened gate: a deck that passes graveyardCount
	// + selfMillCount thresholds but has 0% RoleRecursion must NOT
	// pass Require.
	var fp *archetypeFingerprint
	for i := range archetypeFingerprints {
		if archetypeFingerprints[i].Name == "Reanimator" {
			fp = &archetypeFingerprints[i]
			break
		}
	}
	if fp == nil {
		t.Fatal("Reanimator fingerprint missing")
	}

	// Mill-only ctx: passes pre-r60 gate, fails r60 gate.
	millOnly := &classifyContext{
		graveyardCount: 10,
		selfMillCount:  6,
		roleRatios:     map[RoleTag]float64{RoleRecursion: 0.0},
	}
	if fp.Require(millOnly) {
		t.Errorf("Require gate must reject mill-only deck (RoleRecursion=0)")
	}

	// Real reanimator ctx: passes both pre-r60 and r60 gates.
	real := &classifyContext{
		graveyardCount: 10,
		selfMillCount:  6,
		roleRatios:     map[RoleTag]float64{RoleRecursion: 0.10},
	}
	if !fp.Require(real) {
		t.Errorf("Require gate must accept real reanimator deck (RoleRecursion=0.10)")
	}

	// Boundary: exactly at the 0.05 floor passes.
	boundary := &classifyContext{
		graveyardCount: 10,
		selfMillCount:  6,
		roleRatios:     map[RoleTag]float64{RoleRecursion: 0.05},
	}
	if !fp.Require(boundary) {
		t.Errorf("Require gate must accept boundary deck (RoleRecursion=0.05)")
	}

	// Below floor fails.
	below := &classifyContext{
		graveyardCount: 10,
		selfMillCount:  6,
		roleRatios:     map[RoleTag]float64{RoleRecursion: 0.04},
	}
	if fp.Require(below) {
		t.Errorf("Require gate must reject deck below floor (RoleRecursion=0.04)")
	}
}
