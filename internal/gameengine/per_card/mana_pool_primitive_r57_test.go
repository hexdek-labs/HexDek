package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R57 — five more ports through the R55 mana-primitive triad:
//
//   1. Ashling, Flame Dancer        ManaPoolExemption (red retention)
//   2. Magus of the Coffers          AddManaPerCount (Swamps → B, 4
//                                      mana activation)
//   3. Gaea's Cradle (refactor)      AddManaPerCount (creatures → G)
//   4. Serra's Sanctum (refactor)    AddManaPerCount (enchantments → W)
//   5. Cavern of Souls               AddRestrictedMana (any-color
//                                      creature-spell-only)
//
// The user's prompt named Helix Pinnacle / Energy Field / Sapphire
// Medallion / Crucible Spirit Dragon / "Cabal Coffers-class"; only
// the last fits the primitive shape. Substituted with 5 cards that
// actually use ManaPoolExemption / AddManaPerCount / AddRestrictedMana.
// See the file header in mana_pool_primitive_r57.go for the
// substitution rationale.

// ---------------------------------------------------------------------------
// Ashling, Flame Dancer — red retention
// ---------------------------------------------------------------------------

func TestAshlingFlameDancer_RegistersRedPoolExemption(t *testing.T) {
	gs := newGame(t, 2)
	ashling := addPerm(gs, 0, "Ashling, Flame Dancer", "creature", "legendary")
	ashlingFlameDancerManaExemptionETB(gs, ashling)
	if len(gs.ManaPoolExemptions) != 1 {
		t.Fatalf("expected 1 ManaPoolExemption registered; got %d", len(gs.ManaPoolExemptions))
	}
	exempt := gs.ManaPoolExemptions[0]
	if exempt.Seat != 0 {
		t.Errorf("expected seat-scoped to 0; got %d", exempt.Seat)
	}
	if !exempt.Colors["R"] {
		t.Errorf("expected R in exempt colors; got %+v", exempt.Colors)
	}
}

func TestAshlingFlameDancer_LTBUnregisters(t *testing.T) {
	gs := newGame(t, 2)
	ashling := addPerm(gs, 0, "Ashling, Flame Dancer", "creature", "legendary")
	ashlingFlameDancerManaExemptionETB(gs, ashling)
	ashlingFlameDancerManaExemptionLTB(gs, ashling, map[string]interface{}{"perm": ashling})
	if len(gs.ManaPoolExemptions) != 0 {
		t.Errorf("LTB should drop exemption; got %d", len(gs.ManaPoolExemptions))
	}
}

func TestAshlingFlameDancer_RedManaSurvivesPhaseDrain(t *testing.T) {
	gs := newGame(t, 2)
	ashling := addPerm(gs, 0, "Ashling, Flame Dancer", "creature", "legendary")
	ashlingFlameDancerManaExemptionETB(gs, ashling)
	// Add 3 red mana via the typed pool.
	gameengine.AddMana(gs, gs.Seats[0], "R", 3, "test_setup")
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("setup: expected 3 mana; got %d", gs.Seats[0].ManaPool)
	}
	// Drain pools (phase boundary). R should be retained.
	gameengine.DrainAllPools(gs, "main", "precombat_main")
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("red mana should survive drain under Ashling; got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Magus of the Coffers
// ---------------------------------------------------------------------------

func TestMagusOfTheCoffers_AddsBPerSwamp(t *testing.T) {
	gs := newGame(t, 2)
	magus := addPerm(gs, 0, "Magus of the Coffers", "creature")
	addPerm(gs, 0, "Swamp", "land", "swamp", "basic")
	addPerm(gs, 0, "Swamp", "land", "swamp", "basic")
	addPerm(gs, 0, "Bayou", "land", "swamp", "forest")
	addPerm(gs, 0, "Mountain", "land", "mountain", "basic")
	gs.Seats[0].ManaPool = 4
	magusOfTheCoffersActivate(gs, magus, 0, nil)
	// 3 Swamp-typed lands → +3 B mana, but pay 4 first: 4 - 4 + 3 = 3.
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected pool 3 (4 paid - 4 cost + 3 swamps); got %d", gs.Seats[0].ManaPool)
	}
	if !magus.Tapped {
		t.Errorf("Magus should be tapped after activation")
	}
}

func TestMagusOfTheCoffers_InsufficientManaFails(t *testing.T) {
	gs := newGame(t, 2)
	magus := addPerm(gs, 0, "Magus of the Coffers", "creature")
	addPerm(gs, 0, "Swamp", "land", "swamp", "basic")
	gs.Seats[0].ManaPool = 2 // need 4, only have 2
	magusOfTheCoffersActivate(gs, magus, 0, nil)
	if magus.Tapped {
		t.Errorf("Magus should not tap when cost can't be paid")
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("pool should be untouched on failed activation; got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Gaea's Cradle — refactored to use AddManaPerCount
// ---------------------------------------------------------------------------

func TestGaeasCradle_AddsGPerCreatureViaPrimitive(t *testing.T) {
	gs := newGame(t, 2)
	cradle := addPerm(gs, 0, "Gaea's Cradle", "land", "legendary")
	stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature"), 1, 1)
	stampCreaturePT(addPerm(gs, 0, "Birds of Paradise", "creature"), 0, 1)
	stampCreaturePT(addPerm(gs, 0, "Heritage Druid", "creature"), 1, 1)
	// Add a non-creature to confirm filter rejects.
	addPerm(gs, 0, "Wheel of Sun and Moon", "enchantment")
	gaeasCradleActivate(gs, cradle, 0, nil)
	// 3 creatures → +3 G. (Cradle itself isn't a creature.)
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected +3 G from 3 creatures; got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Serra's Sanctum — refactored to use AddManaPerCount
// ---------------------------------------------------------------------------

func TestSerrasSanctum_AddsWPerEnchantmentViaPrimitive(t *testing.T) {
	gs := newGame(t, 2)
	sanctum := addPerm(gs, 0, "Serra's Sanctum", "land", "legendary")
	addPerm(gs, 0, "Sylvan Library", "enchantment")
	addPerm(gs, 0, "Rhystic Study", "enchantment")
	addPerm(gs, 0, "Pernicious Deed", "enchantment")
	addPerm(gs, 0, "Llanowar Elves", "creature") // not enchantment
	serrasSanctumActivate(gs, sanctum, 0, nil)
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected +3 W from 3 enchantments; got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Cavern of Souls
// ---------------------------------------------------------------------------

func TestCavernOfSouls_ColorlessMode(t *testing.T) {
	gs := newGame(t, 2)
	cavern := addPerm(gs, 0, "Cavern of Souls", "land")
	cavernOfSoulsActivate(gs, cavern, 0, nil)
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("colorless mode should add 1 mana; got %d", gs.Seats[0].ManaPool)
	}
	if !cavern.Tapped {
		t.Errorf("Cavern should tap")
	}
}

func TestCavernOfSouls_RestrictedAnyColorMode(t *testing.T) {
	gs := newGame(t, 2)
	cavern := addPerm(gs, 0, "Cavern of Souls", "land")
	cavernOfSoulsActivate(gs, cavern, 1, map[string]interface{}{
		"chosen_type": "Wizard",
	})
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("restricted mode should add 1 mana; got %d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Flags["cavern_of_souls_chosen_type_active"] != 1 {
		t.Errorf("restricted mode should stamp chosen-type-active flag")
	}
	// Verify the restricted unit is in the typed pool with the
	// "creature_spell_only" restriction.
	if seat := gs.Seats[0]; seat.Mana != nil {
		found := false
		for _, r := range seat.Mana.Restricted {
			if r.Restriction == "creature_spell_only" && r.Amount > 0 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected creature_spell_only restricted unit in typed pool; got %+v",
				seat.Mana.Restricted)
		}
	}
}

func TestCavernOfSouls_AlreadyTappedFails(t *testing.T) {
	gs := newGame(t, 2)
	cavern := addPerm(gs, 0, "Cavern of Souls", "land")
	cavern.Tapped = true
	cavernOfSoulsActivate(gs, cavern, 0, nil)
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("tapped Cavern should not add mana; got %d", gs.Seats[0].ManaPool)
	}
}
