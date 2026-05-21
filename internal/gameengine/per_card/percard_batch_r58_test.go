package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R58 — port tests for the eight partial-residual cards rescued via
// the R54+R55 primitives (and one bonus permanent_etb-hook port).
//
//   1. Ashling, Flame Dancer           — ManaPoolExemption{R}
//   2. Fire Lord Zuko                  — ManaPoolExemption{R} per-combat
//   3. Raphael, Ninja Destroyer        — ManaPoolExemption{R} until EOT
//   4. The Master of Keys              — ZoneCastPolicy(grave, ench, exile-on-resolve)
//   5. Lara Croft, Tomb Raider         — ZoneCastPolicy(exile, discovery, EOT)
//   6. The Reality Chip                — ZoneCastPolicy(library_top, self, while-on-bf)
//   7. Sokrates, Athenian Teacher      — stale R54 partial dropped (replacement IS wired)
//   8. The Wandering Minstrel          — permanent_etb hook untaps controller's lands

// ---------------------------------------------------------------------------
// 1. Ashling, Flame Dancer — stale R36 partial dropped
// ---------------------------------------------------------------------------
//
// The R57 side-handler registerAshlingFlameDancerManaExemption (in
// mana_pool_primitive_r57.go) already calls RegisterManaPoolExemption
// at ETB. The R58 sweep drops the now-stale "unspent red mana retention
// not modelled" breadcrumb from the gen file's ETB hook. The behavior
// is already covered by mana_pool_primitive_r57_test.go; this test
// pins only the partial-dropped contract.

func TestAshlingFlameDancer_R58NoStalePartialOnETB(t *testing.T) {
	gs := newGame(t, 2)
	ash := addPerm(gs, 0, "Ashling, Flame Dancer", "creature", "legendary")

	ashlingFlameDancerETB(gs, ash)

	for _, ev := range gs.EventLog {
		if ev.Kind != "per_card_partial" {
			continue
		}
		reason, _ := ev.Details["reason"].(string)
		if reason == "unspent_red_mana_retention_static_not_modelled_at_per_card_layer" {
			t.Errorf("stale partial breadcrumb should be gone; still seeing %q", reason)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Fire Lord Zuko — ManaPoolExemption{R} per-combat
// ---------------------------------------------------------------------------

func TestFireLordZuko_FirebendingRegistersExemptionAndDelayedTrigger(t *testing.T) {
	gs := newGame(t, 2)
	zuko := addPerm(gs, 0, "Fire Lord Zuko", "creature", "legendary")
	zuko.Card.BasePower = 4
	zuko.Card.BaseToughness = 4

	fireLordZukoFirebending(gs, zuko, map[string]interface{}{
		"attacker_perm": zuko,
	})

	if len(gs.ManaPoolExemptions) != 1 {
		t.Fatalf("firebending should register a {R} exemption; got %d", len(gs.ManaPoolExemptions))
	}
	if !gs.ManaPoolExemptions[0].Colors["R"] {
		t.Errorf("exemption should retain R")
	}
	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("firebending X=4 should have added 4 mana; got pool=%d", gs.Seats[0].ManaPool)
	}
	if len(gs.DelayedTriggers) == 0 {
		t.Errorf("firebending should have queued an end_of_combat delayed trigger")
	}
}

func TestFireLordZuko_LTBUnregistersExemption(t *testing.T) {
	gs := newGame(t, 2)
	zuko := addPerm(gs, 0, "Fire Lord Zuko", "creature", "legendary")
	zuko.Card.BasePower = 2
	zuko.Card.BaseToughness = 2
	fireLordZukoFirebending(gs, zuko, map[string]interface{}{"attacker_perm": zuko})
	if len(gs.ManaPoolExemptions) != 1 {
		t.Fatalf("firebending precondition failed")
	}

	fireLordZukoLTB(gs, zuko, map[string]interface{}{"perm": zuko})

	if len(gs.ManaPoolExemptions) != 0 {
		t.Errorf("LTB should drop exemption; got %d remaining", len(gs.ManaPoolExemptions))
	}
}

// ---------------------------------------------------------------------------
// 3. Raphael, Ninja Destroyer — ManaPoolExemption{R} until EOT
// ---------------------------------------------------------------------------

func TestRaphaelNinjaDestroyer_EnrageRegistersExemption(t *testing.T) {
	gs := newGame(t, 2)
	raph := addPerm(gs, 0, "Raphael, Ninja Destroyer", "creature", "legendary")

	raphaelNinjaDestroyerEnrage(gs, raph, map[string]interface{}{
		"target_perm": raph,
		"amount":      3,
	})

	if len(gs.ManaPoolExemptions) != 1 {
		t.Fatalf("enrage should register a {R} exemption; got %d", len(gs.ManaPoolExemptions))
	}
	if !gs.ManaPoolExemptions[0].Colors["R"] {
		t.Errorf("exemption should retain R")
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("enrage should have added 3 mana; got pool=%d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Flags["raphael_keep_red_until_eot"] != 3 {
		t.Errorf("seat flag tally should equal damage amount; got %d", gs.Seats[0].Flags["raphael_keep_red_until_eot"])
	}
	if len(gs.DelayedTriggers) == 0 {
		t.Errorf("enrage should queue an end_of_turn delayed trigger")
	}
}

func TestRaphaelNinjaDestroyer_LTBUnregistersExemption(t *testing.T) {
	gs := newGame(t, 2)
	raph := addPerm(gs, 0, "Raphael, Ninja Destroyer", "creature", "legendary")
	raphaelNinjaDestroyerEnrage(gs, raph, map[string]interface{}{
		"target_perm": raph,
		"amount":      2,
	})
	if len(gs.ManaPoolExemptions) != 1 {
		t.Fatalf("enrage precondition failed")
	}

	raphaelNinjaDestroyerLTB(gs, raph, map[string]interface{}{"perm": raph})

	if len(gs.ManaPoolExemptions) != 0 {
		t.Errorf("LTB should drop exemption; got %d remaining", len(gs.ManaPoolExemptions))
	}
}

// ---------------------------------------------------------------------------
// 4. The Master of Keys — ZoneCastPolicy(graveyard, enchantment, exile-on-resolve)
// ---------------------------------------------------------------------------

func TestTheMasterOfKeys_ETBRegistersEnchantmentEscapePolicy(t *testing.T) {
	gs := newGame(t, 2)
	mk := addPerm(gs, 0, "The Master of Keys", "creature", "legendary")

	theMasterOfKeysETB(gs, mk)

	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("ETB should register 1 policy; got %d", len(gs.ZoneCastPolicies))
	}
	p := gs.ZoneCastPolicies[0]
	if p.Zone != gameengine.ZoneGraveyard {
		t.Errorf("policy zone should be graveyard; got %s", p.Zone)
	}
	if !p.ExileOnResolve {
		t.Errorf("escape grants ExileOnResolve")
	}
	if p.OwnerScope != "self" || p.CasterScope != "controller" {
		t.Errorf("expected self/controller scopes; got owner=%q caster=%q", p.OwnerScope, p.CasterScope)
	}
}

func TestTheMasterOfKeys_PolicyMatchesEnchantmentOnly(t *testing.T) {
	gs := newGame(t, 2)
	mk := addPerm(gs, 0, "The Master of Keys", "creature", "legendary")
	theMasterOfKeysETB(gs, mk)

	ench := &gameengine.Card{Name: "Sigil of the Empty Throne", Owner: 0, Types: []string{"enchantment"}}
	creature := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}

	if gameengine.FindZoneCastPolicy(gs, 0, ench, 0, gameengine.ZoneGraveyard) == nil {
		t.Errorf("enchantment in controller's graveyard should match escape policy")
	}
	if gameengine.FindZoneCastPolicy(gs, 0, creature, 0, gameengine.ZoneGraveyard) != nil {
		t.Errorf("non-enchantment should NOT match escape policy")
	}
}

func TestTheMasterOfKeys_LTBUnregistersPolicy(t *testing.T) {
	gs := newGame(t, 2)
	mk := addPerm(gs, 0, "The Master of Keys", "creature", "legendary")
	theMasterOfKeysETB(gs, mk)
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("ETB precondition failed")
	}

	theMasterOfKeysLTB(gs, mk, map[string]interface{}{"perm": mk})

	if len(gs.ZoneCastPolicies) != 0 {
		t.Errorf("LTB should drop policy; got %d remaining", len(gs.ZoneCastPolicies))
	}
}

// ---------------------------------------------------------------------------
// 5. Lara Croft, Tomb Raider — ZoneCastPolicy(exile, discovery, EOT)
// ---------------------------------------------------------------------------

func TestLaraCroft_AttackExilesAndRegistersDiscoveryPolicy(t *testing.T) {
	gs := newGame(t, 2)
	lara := addPerm(gs, 0, "Lara Croft, Tomb Raider", "creature", "legendary")
	target := &gameengine.Card{Name: "Mox Diamond", Owner: 1, Types: []string{"artifact", "legendary"}}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, target)

	laraCroftAttackTrigger(gs, lara, map[string]interface{}{"attacker_perm": lara})

	// Card should be in opp's exile, discovery_counter stamped.
	foundInExile := false
	for _, c := range gs.Seats[1].Exile {
		if c == target {
			foundInExile = true
			break
		}
	}
	if !foundInExile {
		t.Errorf("target should be moved to opp's exile")
	}
	hasDiscovery := false
	for _, t := range target.Types {
		if t == "discovery_counter" {
			hasDiscovery = true
		}
	}
	if !hasDiscovery {
		t.Errorf("target should have discovery_counter type stamped")
	}
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("attack should register 1 policy; got %d", len(gs.ZoneCastPolicies))
	}
	p := gs.ZoneCastPolicies[0]
	if p.Zone != gameengine.ZoneExile {
		t.Errorf("policy zone should be exile; got %s", p.Zone)
	}
	if p.Duration != "until_end_of_turn" {
		t.Errorf("policy duration should be until_end_of_turn; got %s", p.Duration)
	}
}

func TestLaraCroft_PolicyMatchesOnlyDiscoveryStampedCards(t *testing.T) {
	gs := newGame(t, 2)
	lara := addPerm(gs, 0, "Lara Croft, Tomb Raider", "creature", "legendary")
	target := &gameengine.Card{Name: "Mox Diamond", Owner: 1, Types: []string{"artifact", "legendary"}}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, target)
	laraCroftAttackTrigger(gs, lara, map[string]interface{}{"attacker_perm": lara})

	if gameengine.FindZoneCastPolicy(gs, 0, target, 1, gameengine.ZoneExile) == nil {
		t.Errorf("discovery-stamped card should match policy")
	}

	other := &gameengine.Card{Name: "Plains", Owner: 0, Types: []string{"land"}}
	if gameengine.FindZoneCastPolicy(gs, 0, other, 0, gameengine.ZoneExile) != nil {
		t.Errorf("non-discovery card should NOT match policy")
	}
}

func TestLaraCroft_LTBUnregistersPolicy(t *testing.T) {
	gs := newGame(t, 2)
	lara := addPerm(gs, 0, "Lara Croft, Tomb Raider", "creature", "legendary")
	target := &gameengine.Card{Name: "Mox Diamond", Owner: 1, Types: []string{"artifact", "legendary"}}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, target)
	laraCroftAttackTrigger(gs, lara, map[string]interface{}{"attacker_perm": lara})
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("attack precondition failed")
	}

	laraCroftLTB(gs, lara, map[string]interface{}{"perm": lara})

	if len(gs.ZoneCastPolicies) != 0 {
		t.Errorf("LTB should drop policy; got %d remaining", len(gs.ZoneCastPolicies))
	}
}

// ---------------------------------------------------------------------------
// 6. The Reality Chip — ZoneCastPolicy(library_top, self, while-on-bf)
// ---------------------------------------------------------------------------

func TestTheRealityChip_ETBRegistersLibraryTopPolicy(t *testing.T) {
	gs := newGame(t, 2)
	chip := addPerm(gs, 0, "The Reality Chip", "artifact", "equipment")

	theRealityChipETB(gs, chip)

	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("ETB should register 1 policy; got %d", len(gs.ZoneCastPolicies))
	}
	p := gs.ZoneCastPolicies[0]
	if p.Zone != "library_top" {
		t.Errorf("policy zone should be library_top; got %s", p.Zone)
	}
	if p.OwnerScope != "self" || p.CasterScope != "controller" {
		t.Errorf("expected self/controller scopes; got owner=%q caster=%q", p.OwnerScope, p.CasterScope)
	}
	if p.Duration != "while_source_on_bf" {
		t.Errorf("policy duration should be while_source_on_bf; got %s", p.Duration)
	}
	if gs.Seats[0].Flags["may_see_top_of_library"] != 1 {
		t.Errorf("ETB should stamp may_see_top_of_library seat flag")
	}
}

func TestTheRealityChip_LTBUnregistersPolicyAndClearsFlag(t *testing.T) {
	gs := newGame(t, 2)
	chip := addPerm(gs, 0, "The Reality Chip", "artifact", "equipment")
	theRealityChipETB(gs, chip)
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("ETB precondition failed")
	}

	theRealityChipLTBClearFlag(gs, chip, map[string]interface{}{"perm": chip})

	if len(gs.ZoneCastPolicies) != 0 {
		t.Errorf("LTB should drop policy; got %d remaining", len(gs.ZoneCastPolicies))
	}
	if gs.Seats[0].Flags["may_see_top_of_library"] != 0 {
		t.Errorf("LTB should clear may_see_top_of_library; got %d", gs.Seats[0].Flags["may_see_top_of_library"])
	}
}

// ---------------------------------------------------------------------------
// 7. Sokrates, Athenian Teacher — stale R54 partial dropped
// ---------------------------------------------------------------------------
//
// The damage-replacement primitive was wired in R54 but the breadcrumb
// emitPartial from the pre-primitive port wasn't dropped. R58 removes
// the stale call. This test pins the still-wired primitive and verifies
// no per_card_partial event fires for the dialogue activation.

func TestSokrates_DialogueRegistersReplacementWithoutStalePartial(t *testing.T) {
	gs := newGame(t, 2)
	sok := addPerm(gs, 0, "Sokrates, Athenian Teacher", "creature", "legendary")
	target := addPerm(gs, 0, "Grizzly Bears", "creature")

	beforeReps := len(gs.DamageReplacements)
	sokratesDialogue(gs, sok, 0, map[string]interface{}{"target_perm": target})
	afterReps := len(gs.DamageReplacements)

	if afterReps != beforeReps+1 {
		t.Errorf("dialogue should register exactly 1 damage replacement; before=%d after=%d", beforeReps, afterReps)
	}
	if target.Flags["sokrates_dialogue_until_eot"] != 1 {
		t.Errorf("dialogue should stamp target's sokrates_dialogue_until_eot flag")
	}
	// No stale per_card_partial event from the activation slug.
	for _, ev := range gs.EventLog {
		if ev.Kind != "per_card_partial" {
			continue
		}
		reason, _ := ev.Details["reason"].(string)
		if reason == "combat-damage→draws conversion needs engine-side replacement effect on the dialogue flag" {
			t.Errorf("stale partial breadcrumb should be gone; still seeing %q", reason)
		}
	}
}

// ---------------------------------------------------------------------------
// 8. The Wandering Minstrel — permanent_etb hook untaps controller's lands
// ---------------------------------------------------------------------------

func TestWanderingMinstrel_UntapsControllerLandsOnETB(t *testing.T) {
	gs := newGame(t, 2)
	minstrel := addPerm(gs, 0, "The Wandering Minstrel", "creature", "legendary")
	land := addPerm(gs, 0, "Mountain", "land")
	land.Tapped = true

	theWanderingMinstrelOnLandETB(gs, minstrel, map[string]interface{}{
		"perm": land,
	})

	if land.Tapped {
		t.Errorf("Minstrel's permanent_etb should untap controller's land; got Tapped=true")
	}
}

func TestWanderingMinstrel_IgnoresOpponentLands(t *testing.T) {
	gs := newGame(t, 2)
	minstrel := addPerm(gs, 0, "The Wandering Minstrel", "creature", "legendary")
	oppLand := addPerm(gs, 1, "Mountain", "land")
	oppLand.Tapped = true

	theWanderingMinstrelOnLandETB(gs, minstrel, map[string]interface{}{
		"perm": oppLand,
	})

	if !oppLand.Tapped {
		t.Errorf("Minstrel must NOT untap opponent's land; got Tapped=false")
	}
}

func TestWanderingMinstrel_IgnoresNonLandPermanents(t *testing.T) {
	gs := newGame(t, 2)
	minstrel := addPerm(gs, 0, "The Wandering Minstrel", "creature", "legendary")
	creature := addPerm(gs, 0, "Grizzly Bears", "creature")
	creature.Tapped = true

	theWanderingMinstrelOnLandETB(gs, minstrel, map[string]interface{}{
		"perm": creature,
	})

	if !creature.Tapped {
		t.Errorf("Minstrel must NOT untap non-land permanents; got Tapped=false")
	}
}
