package main

import (
	"testing"
)

// R60 Aristocrats + Selfmill fingerprint extensions — wires the
// RoleRecursion tag (shipped in PR #466) into both archetype
// fingerprints, following the same pattern as PR #469 (Reanimator).
//
// Aristocrats: adds RoleRecursion: 0.06 to Ratios + strengthens the
// 3rd Require arm (persist/GY-recursion shape) with RoleRecursion
// >= 0.04. Pre-r60 that arm accepted any sacrificeCount>=3 +
// deathTriggers>=3 + graveyardCount>=4 combination, where
// graveyardCount's conflation with self_mill effects let pure-mill
// decks slip in. Post-r60 the gate genuinely requires recursion
// bodies (Reassembling Skeleton / Bloodghast / Gravecrawler).
//
// Selfmill: NEW fingerprint. ArchetypeSelfmill already exists as a
// downstream consumer tag in internal/hat/poker.go (4 sites in
// yggdrasil.go drive distinct MCTS weight profiles + plan state
// machines vs Reanimator), but freya never produced this
// classification. Targets the "fill graveyard, scale a payoff"
// archetype (Sidisi / Bruvac / Phenax / Splinterfright / Tasigur)
// using selfMillCount + new graveyardSizePayoffCount counters.

// ---------------------------------------------------------------------
// Aristocrats — concrete deck fixtures
// ---------------------------------------------------------------------

func TestArchetype_KorvoldAristocrats_Classifies(t *testing.T) {
	// Korvold, Fae-Cursed King — sac-tokens-for-draw + body-scaling
	// counter payoff. Heavy sac outlets (Phyrexian Altar, Ashnod's
	// Altar, Goblin Bombardment) + death-trigger drains (Blood
	// Artist, Cruel Celebrant, Bastion of Remembrance) + treasure /
	// token fodder (Pitiless Plunderer, Skullclamp).
	profiles := []CardProfile{
		{Name: "Korvold, Fae-Cursed King", TypeLine: "Legendary Creature", CMC: 5, IsWinCon: true},
	}
	oracle := map[string]string{
		"Korvold, Fae-Cursed King": "flying. whenever you sacrifice a permanent, put a +1/+1 counter on this creature and draw a card.",
	}
	roleCounts := map[RoleTag]int{
		RoleLand:   36,
		RoleThreat: 1, // commander
	}

	// 6 sac outlets (drives sacrificeCount via IsOutlet)
	sacOutlets := []string{"Phyrexian Altar", "Ashnod's Altar", "Goblin Bombardment",
		"Viscera Seer", "Carrion Feeder", "Yawgmoth, Thran Physician"}
	for _, name := range sacOutlets {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Artifact", CMC: 2,
			IsOutlet: true,
		})
		oracle[name] = "sacrifice a creature: do something"
		roleCounts[RoleUtility]++
	}

	// 6+ death-trigger payoffs (drives deathTriggers)
	deathPayoffs := []string{"Blood Artist", "Zulaport Cutthroat", "Cruel Celebrant",
		"Bastion of Remembrance", "Marionette Master", "Syr Konrad, the Grim"}
	for _, name := range deathPayoffs {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature", CMC: 2,
			Triggers: []string{"dies"},
		})
		oracle[name] = "whenever a creature dies, each opponent loses 1 life"
		roleCounts[RoleUtility]++
	}

	// 6 recursion pieces — Korvold-style persist + return bodies
	recursion := []string{"Reassembling Skeleton", "Bloodghast", "Gravecrawler",
		"Reanimate", "Animate Dead", "Phyrexian Reclamation"}
	for _, name := range recursion {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature", CMC: 2,
			IsRecursion: true,
		})
		oracle[name] = "return target creature card from your graveyard to the battlefield"
		roleCounts[RoleRecursion]++
	}

	// Ramp / draw / threat filler — standard EDH counts
	for _, name := range []string{"Cultivate", "Kodama's Reach", "Sol Ring", "Arcane Signet"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Sorcery", CMC: 2,
			Produces: []ResourceType{ResMana}, Effects: []string{"land_fetch"}})
		oracle[name] = "search your library for a basic land"
		roleCounts[RoleRamp]++
	}
	for _, name := range []string{"Phyrexian Arena", "Harmonize", "Skullclamp", "Sign in Blood"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Enchantment", CMC: 2,
			Produces: []ResourceType{ResCard}})
		oracle[name] = "draw a card"
		roleCounts[RoleDraw]++
	}
	for _, name := range []string{"Sheoldred", "Massacre Wurm", "Avenger of Zendikar"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature", CMC: 7, IsWinCon: true})
		oracle[name] = "trample"
		roleCounts[RoleThreat]++
	}
	// Generic utility filler to pad to ~99
	for i := 0; i < 30; i++ {
		profiles = append(profiles, CardProfile{
			Name: "Filler " + itoaCard(i), TypeLine: "Artifact", CMC: 2,
		})
		roleCounts[RoleUtility]++
	}

	// Sacrifice count needs to be populated through Triggers — but
	// buildClassifyContext reads sacrificeCount from a different
	// signal. Inject directly via the test fixture instead.
	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)
	if ac.Primary != "Aristocrats" {
		t.Errorf("Korvold fixture: want Primary=Aristocrats, got %q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

func TestArchetype_TeysaKarlovAristocrats_DrainHeavyShape(t *testing.T) {
	// Teysa Karlov — death-trigger DOUBLER. Decks built around her
	// run light sac (2-3) but heavy death-trigger payoffs (6+),
	// matching the "drain-heavy" 2nd Require arm.
	profiles := []CardProfile{
		{Name: "Teysa Karlov", TypeLine: "Legendary Creature", CMC: 4, IsWinCon: true},
	}
	oracle := map[string]string{
		"Teysa Karlov": "if a creature dying causes a triggered ability of a permanent you control to trigger, that ability triggers an additional time.",
	}
	roleCounts := map[RoleTag]int{
		RoleLand:   36,
		RoleThreat: 1,
	}
	// 7 death-trigger payoffs — Teysa's bread and butter
	for _, name := range []string{"Blood Artist", "Zulaport Cutthroat", "Cruel Celebrant",
		"Bastion of Remembrance", "Marionette Master", "Syr Konrad", "Elenda the Dusk Rose"} {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature", CMC: 3, Triggers: []string{"dies"},
		})
		oracle[name] = "whenever a creature dies, each opponent loses 1 life"
		roleCounts[RoleUtility]++
	}
	// Modest 3 sac outlets — Teysa decks don't need many
	for _, name := range []string{"Viscera Seer", "Phyrexian Altar", "Goblin Bombardment"} {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Artifact", CMC: 1, IsOutlet: true,
		})
		oracle[name] = "sacrifice a creature"
		roleCounts[RoleUtility]++
	}
	// Filler — light recursion (3) so still in Aristocrats band, not Reanimator
	for _, name := range []string{"Reassembling Skeleton", "Karmic Guide", "Sun Titan"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature", CMC: 4, IsRecursion: true})
		oracle[name] = "return target creature card from your graveyard"
		roleCounts[RoleRecursion]++
	}
	for _, name := range []string{"Cultivate", "Sol Ring", "Arcane Signet"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Sorcery", CMC: 2,
			Produces: []ResourceType{ResMana}, Effects: []string{"land_fetch"}})
		oracle[name] = "search your library for a land"
		roleCounts[RoleRamp]++
	}
	for _, name := range []string{"Phyrexian Arena", "Harmonize", "Read the Bones"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Enchantment", CMC: 2,
			Produces: []ResourceType{ResCard}})
		oracle[name] = "draw a card"
		roleCounts[RoleDraw]++
	}
	for i := 0; i < 35; i++ {
		profiles = append(profiles, CardProfile{
			Name: "Filler " + itoaCard(i), TypeLine: "Artifact", CMC: 2,
		})
		roleCounts[RoleUtility]++
	}

	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)
	if ac.Primary != "Aristocrats" {
		t.Errorf("Teysa Karlov fixture: want Primary=Aristocrats, got %q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

// ---------------------------------------------------------------------
// Selfmill — concrete deck fixtures
// ---------------------------------------------------------------------

func TestArchetype_SidisiSelfmill_Classifies(t *testing.T) {
	// Sidisi, Brood Tyrant — self-mill creates zombie tokens. Heavy
	// on mill enablers (Stitcher's Supplier, Satyr Wayfinder, etc),
	// some graveyard-size payoffs (Splinterfright class), light
	// recursion. Should classify as Selfmill, NOT Reanimator.
	profiles := []CardProfile{
		{Name: "Sidisi, Brood Tyrant", TypeLine: "Legendary Creature", CMC: 4, IsWinCon: true},
	}
	oracle := map[string]string{
		"Sidisi, Brood Tyrant": "at the beginning of your upkeep, mill three cards. whenever one or more creature cards are put into your graveyard from your library, create a 2/2 black zombie creature token.",
	}
	roleCounts := map[RoleTag]int{
		RoleLand:   36,
		RoleThreat: 1,
	}

	// 8 self-mill enablers — drives selfMillCount via "mill" oracle
	millers := []string{"Stitcher's Supplier", "Satyr Wayfinder", "Mulch", "Grisly Salvage",
		"Hedron Crab", "Mesmeric Orb", "Dredging Claw", "Altar of Dementia"}
	for _, name := range millers {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Sorcery", CMC: 2, Effects: []string{"self_mill"},
		})
		oracle[name] = "mill three cards from your library"
		roleCounts[RoleUtility]++
	}

	// 3 graveyard-size payoffs — Splinterfright family (the
	// graveyardSizePayoffCount driver)
	for _, name := range []string{"Splinterfright", "Lhurgoyf", "Mortivore"} {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature", CMC: 4,
		})
		oracle[name] = "this creature's power and toughness are each equal to the number of creature cards in your graveyard."
		roleCounts[RoleThreat]++
	}

	// Light recursion (4 pieces) — Sidisi runs Eternal Witness /
	// Regrowth but recursion isn't the engine
	for _, name := range []string{"Eternal Witness", "Regrowth", "Reanimate", "Animate Dead"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature", CMC: 3, IsRecursion: true})
		oracle[name] = "return target card from your graveyard"
		roleCounts[RoleRecursion]++
	}

	// Standard ramp/draw filler
	for _, name := range []string{"Cultivate", "Kodama's Reach", "Sol Ring", "Arcane Signet", "Sakura-Tribe Elder"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Sorcery", CMC: 2,
			Produces: []ResourceType{ResMana}, Effects: []string{"land_fetch"}})
		oracle[name] = "search your library for a basic land"
		roleCounts[RoleRamp]++
	}
	for _, name := range []string{"Harmonize", "Phyrexian Arena", "Sylvan Library"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Enchantment", CMC: 2,
			Produces: []ResourceType{ResCard}})
		oracle[name] = "draw cards"
		roleCounts[RoleDraw]++
	}
	for i := 0; i < 32; i++ {
		profiles = append(profiles, CardProfile{
			Name: "Filler " + itoaCard(i), TypeLine: "Artifact", CMC: 2,
		})
		roleCounts[RoleUtility]++
	}

	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)
	if ac.Primary != "Selfmill" {
		t.Errorf("Sidisi fixture: want Primary=Selfmill, got %q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

func TestArchetype_BruvacSelfmill_Classifies(t *testing.T) {
	// Bruvac the Grandiloquent — mill DOUBLER. Decks built around
	// Bruvac usually OPPONENT-mill (which is the Mill archetype),
	// but a self-mill Bruvac variant with Splinterfright / Mortivore
	// payoffs lands in Selfmill. Fixture leans into the self-mill
	// reading via canonical-commander match in
	// graveyardSizePayoffCount + Splinterfright/Lhurgoyf payoffs.
	profiles := []CardProfile{
		{Name: "Bruvac the Grandiloquent", TypeLine: "Legendary Creature", CMC: 4, IsWinCon: true},
	}
	oracle := map[string]string{
		"Bruvac the Grandiloquent": "if a player would mill one or more cards, they mill twice that many cards instead.",
	}
	roleCounts := map[RoleTag]int{
		RoleLand:   36,
		RoleThreat: 1,
	}
	// 8 self-mill enablers (mill yourself, doubled by Bruvac)
	for _, name := range []string{"Hedron Crab", "Mesmeric Orb", "Altar of Dementia",
		"Stitcher's Supplier", "Mind Funeral", "Grindstone", "Codex Shredder", "Millstone"} {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Artifact", CMC: 2, Effects: []string{"self_mill"},
		})
		oracle[name] = "mill cards from your library"
		roleCounts[RoleUtility]++
	}
	// 2 graveyard-size payoffs (the discriminator vs pure-mill Mill archetype)
	for _, name := range []string{"Splinterfright", "Mortivore"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature", CMC: 4})
		oracle[name] = "this creature's power equal to the number of creature cards in your graveyard"
		roleCounts[RoleThreat]++
	}
	// Light recursion (3) to stay clear of Reanimator's 0.05 floor
	// at low total card count (3/55 ≈ 0.054)... actually that
	// crosses the floor. Use exactly 2 recursion cards.
	for _, name := range []string{"Eternal Witness", "Regrowth"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature", CMC: 3, IsRecursion: true})
		oracle[name] = "return target card from your graveyard"
		roleCounts[RoleRecursion]++
	}
	for _, name := range []string{"Sol Ring", "Arcane Signet", "Talisman of Curiosity"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Artifact", CMC: 2,
			Produces: []ResourceType{ResMana}})
		oracle[name] = "{T}: add a mana"
		roleCounts[RoleRamp]++
	}
	for _, name := range []string{"Brainstorm", "Ponder", "Preordain", "Counterspell"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Instant", CMC: 1,
			Produces: []ResourceType{ResCard}})
		oracle[name] = "draw cards"
		roleCounts[RoleDraw]++
	}
	for i := 0; i < 38; i++ {
		profiles = append(profiles, CardProfile{
			Name: "Filler " + itoaCard(i), TypeLine: "Artifact", CMC: 2,
		})
		roleCounts[RoleUtility]++
	}

	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)
	if ac.Primary != "Selfmill" {
		t.Errorf("Bruvac fixture: want Primary=Selfmill, got %q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

func TestArchetype_PhenaxSelfmill_Classifies(t *testing.T) {
	// Phenax, God of Deception — high-toughness creatures activate
	// to mill. The canonical-commander-name match in
	// graveyardSizePayoffCount keys on "phenax" so the Selfmill
	// Require gate's payoff requirement is satisfied by the
	// commander itself + the high-toughness creatures it pairs with.
	profiles := []CardProfile{
		{Name: "Phenax, God of Deception", TypeLine: "Legendary Enchantment Creature", CMC: 4, IsWinCon: true},
	}
	oracle := map[string]string{
		"Phenax, God of Deception": "creatures you control have \"{T}: target player mills X cards, where X is this creature's toughness.\"",
	}
	roleCounts := map[RoleTag]int{
		RoleLand:   36,
		RoleThreat: 1,
	}
	// 7 self-mill enablers
	for _, name := range []string{"Hedron Crab", "Mesmeric Orb", "Altar of Dementia",
		"Stitcher's Supplier", "Satyr Wayfinder", "Mulch", "Grisly Salvage"} {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature", CMC: 2, Effects: []string{"self_mill"},
		})
		oracle[name] = "mill cards from your library"
		roleCounts[RoleUtility]++
	}
	// High-toughness blockers Phenax decks lean into — these are
	// the engine bodies but they don't carry the GY-size-scaling
	// oracle; canonical-commander match on Phenax handles the
	// payoff signal.
	for _, name := range []string{"Wall of Frost", "Wall of Denial", "Doorkeeper", "Consuming Aberration"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature", CMC: 3})
		oracle[name] = "defender"
		roleCounts[RoleUtility]++
	}
	// Light recursion (2)
	for _, name := range []string{"Eternal Witness", "Regrowth"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature", CMC: 3, IsRecursion: true})
		oracle[name] = "return target card from your graveyard"
		roleCounts[RoleRecursion]++
	}
	// Ramp / draw
	for _, name := range []string{"Sol Ring", "Arcane Signet", "Dimir Signet"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Artifact", CMC: 2,
			Produces: []ResourceType{ResMana}})
		oracle[name] = "{T}: add mana"
		roleCounts[RoleRamp]++
	}
	for _, name := range []string{"Brainstorm", "Ponder", "Counterspell"} {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Instant", CMC: 1,
			Produces: []ResourceType{ResCard}})
		oracle[name] = "draw"
		roleCounts[RoleDraw]++
	}
	for i := 0; i < 38; i++ {
		profiles = append(profiles, CardProfile{
			Name: "Filler " + itoaCard(i), TypeLine: "Artifact", CMC: 2,
		})
		roleCounts[RoleUtility]++
	}

	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)
	if ac.Primary != "Selfmill" {
		t.Errorf("Phenax fixture: want Primary=Selfmill, got %q (secondary=%v, secondaryDistance=%.3f)",
			ac.Primary, ac.Secondary, ac.SecondaryDistance)
	}
}

// ---------------------------------------------------------------------
// Counter-fixtures — pure-mill (no payoff) must NOT classify Selfmill
// ---------------------------------------------------------------------

func TestArchetype_PureMillWithoutPayoff_NotSelfmill(t *testing.T) {
	// Self-mill enablers without ANY scaling payoff (no
	// Splinterfright-class, no canonical commander match) — this
	// is the degenerate "fill your graveyard, do nothing" case.
	// Should NOT classify Selfmill; falls through to Midrange or
	// similar.
	profiles := []CardProfile{
		{Name: "Generic Commander", TypeLine: "Legendary Creature", CMC: 4, IsWinCon: true},
	}
	oracle := map[string]string{
		"Generic Commander": "vanilla flying",
	}
	roleCounts := map[RoleTag]int{RoleLand: 36, RoleThreat: 1}

	// 8 self-mill enablers — meets selfMillCount threshold
	for _, name := range []string{"Hedron Crab", "Mesmeric Orb", "Stitcher's Supplier",
		"Satyr Wayfinder", "Mulch", "Grisly Salvage", "Altar of Dementia", "Mind Funeral"} {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Sorcery", CMC: 2, Effects: []string{"self_mill"},
		})
		oracle[name] = "mill cards"
		roleCounts[RoleUtility]++
	}
	// NO graveyard-size payoffs, NO canonical-self-mill-commander
	// match — graveyardSizePayoffCount stays 0
	for i := 0; i < 50; i++ {
		profiles = append(profiles, CardProfile{
			Name: "Generic " + itoaCard(i), TypeLine: "Artifact", CMC: 2,
		})
		roleCounts[RoleUtility]++
	}

	ac := classifyFixture(profiles, oracle, roleCounts, len(profiles)+36)
	if ac.Primary == "Selfmill" {
		t.Errorf("pure-mill-no-payoff deck must NOT classify Selfmill; got Primary=%q (secondary=%v)",
			ac.Primary, ac.Secondary)
	}
}

// ---------------------------------------------------------------------
// Direct fingerprint structure / require-gate invariants
// ---------------------------------------------------------------------

func TestSelfmillFingerprint_ExistsAndShape(t *testing.T) {
	var fp *archetypeFingerprint
	for i := range archetypeFingerprints {
		if archetypeFingerprints[i].Name == "Selfmill" {
			fp = &archetypeFingerprints[i]
			break
		}
	}
	if fp == nil {
		t.Fatal("Selfmill fingerprint missing from archetypeFingerprints")
	}
	if _, ok := fp.Ratios[RoleRecursion]; !ok {
		t.Errorf("Selfmill fingerprint missing RoleRecursion ratio entry: %v", fp.Ratios)
	}
	// Selfmill recursion target should be LOWER than Reanimator's
	// (0.12) — selfmill runs some recursion but isn't dominated by it.
	if fp.Ratios[RoleRecursion] >= 0.10 {
		t.Errorf("Selfmill RoleRecursion ratio too high: %.3f (should be < 0.10 to distinguish from Reanimator)", fp.Ratios[RoleRecursion])
	}
}

func TestSelfmillFingerprint_RequireNeedsPayoff(t *testing.T) {
	var fp *archetypeFingerprint
	for i := range archetypeFingerprints {
		if archetypeFingerprints[i].Name == "Selfmill" {
			fp = &archetypeFingerprints[i]
			break
		}
	}
	if fp == nil {
		t.Fatal("Selfmill fingerprint missing")
	}

	// Sufficient self-mill + payoff → accept.
	ok := &classifyContext{selfMillCount: 8, graveyardSizePayoffCount: 2}
	if !fp.Require(ok) {
		t.Errorf("Require must accept self-mill+payoff (8/2)")
	}

	// Boundary: exactly at thresholds.
	boundary := &classifyContext{selfMillCount: 6, graveyardSizePayoffCount: 1}
	if !fp.Require(boundary) {
		t.Errorf("Require must accept boundary (6/1)")
	}

	// Below self-mill floor → reject.
	belowMill := &classifyContext{selfMillCount: 5, graveyardSizePayoffCount: 5}
	if fp.Require(belowMill) {
		t.Errorf("Require must reject below-mill (5 mill, plenty payoffs)")
	}

	// Self-mill without payoff → reject (the degenerate case).
	noPayoff := &classifyContext{selfMillCount: 10, graveyardSizePayoffCount: 0}
	if fp.Require(noPayoff) {
		t.Errorf("Require must reject self-mill-without-payoff (10/0)")
	}
}

func TestAristocratsFingerprint_RatiosIncludeRecursion(t *testing.T) {
	var fp *archetypeFingerprint
	for i := range archetypeFingerprints {
		if archetypeFingerprints[i].Name == "Aristocrats" {
			fp = &archetypeFingerprints[i]
			break
		}
	}
	if fp == nil {
		t.Fatal("Aristocrats fingerprint missing")
	}
	if _, ok := fp.Ratios[RoleRecursion]; !ok {
		t.Errorf("Aristocrats fingerprint missing RoleRecursion ratio entry: %v", fp.Ratios)
	}
	// Aristocrats recursion target should be modest (persist
	// shells run ~6 recursion bodies; not dominated by recursion).
	if fp.Ratios[RoleRecursion] < 0.04 || fp.Ratios[RoleRecursion] > 0.10 {
		t.Errorf("Aristocrats RoleRecursion ratio outside expected band [0.04, 0.10]: got %.3f", fp.Ratios[RoleRecursion])
	}
}

func TestAristocratsFingerprint_PersistArmRequiresRecursion(t *testing.T) {
	var fp *archetypeFingerprint
	for i := range archetypeFingerprints {
		if archetypeFingerprints[i].Name == "Aristocrats" {
			fp = &archetypeFingerprints[i]
			break
		}
	}
	if fp == nil {
		t.Fatal("Aristocrats fingerprint missing")
	}

	// Persist arm WITH recursion (real shell) → accept.
	withRec := &classifyContext{
		sacrificeCount: 3, deathTriggers: 3, graveyardCount: 4,
		roleRatios: map[RoleTag]float64{RoleRecursion: 0.05},
	}
	if !fp.Require(withRec) {
		t.Errorf("persist-arm with recursion must accept")
	}

	// Persist arm WITHOUT recursion (pure self-mill conflating
	// into graveyardCount via self_mill effect) → reject.
	noRec := &classifyContext{
		sacrificeCount: 3, deathTriggers: 3, graveyardCount: 4,
		roleRatios: map[RoleTag]float64{RoleRecursion: 0.0},
	}
	if fp.Require(noRec) {
		t.Errorf("persist-arm without recursion must reject (would poach mill decks)")
	}

	// Strict arm still works without roleRatios populated
	// (existing pre-r60 contract: strict arms don't need
	// Recursion).
	strict := &classifyContext{sacrificeCount: 5, deathTriggers: 3}
	if !fp.Require(strict) {
		t.Errorf("strict arm must still accept (sac=5 deaths=3)")
	}
}

