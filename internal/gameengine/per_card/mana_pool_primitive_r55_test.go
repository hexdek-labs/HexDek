package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r55 — port tests for the five mana-primitive cards (Omnath Locus of
// Mana, Upwelling, Cabal Coffers, Sanctum Weaver, Ancient Ziggurat).

// ---------------------------------------------------------------------
// Omnath, Locus of Mana — registers a controller-scoped G exemption
// ---------------------------------------------------------------------

func TestOmnathLocusOfMana_ETBRegistersGExemption(t *testing.T) {
	gs := newGame(t, 2)
	omnath := addPerm(gs, 0, "Omnath, Locus of Mana", "creature", "legendary")

	omnathLocusOfManaETB(gs, omnath)

	if len(gs.ManaPoolExemptions) != 1 {
		t.Fatalf("expected 1 exemption registered; got %d", len(gs.ManaPoolExemptions))
	}
	e := gs.ManaPoolExemptions[0]
	if e.Source != omnath {
		t.Errorf("exemption source should be omnath; got %v", e.Source)
	}
	if e.Seat != 0 {
		t.Errorf("exemption should be scoped to seat 0; got %d", e.Seat)
	}
	if !e.Colors["G"] {
		t.Errorf("exemption should retain G; got %v", e.Colors)
	}
}

func TestOmnathLocusOfMana_LTBUnregistersExemption(t *testing.T) {
	gs := newGame(t, 2)
	omnath := addPerm(gs, 0, "Omnath, Locus of Mana", "creature", "legendary")
	omnathLocusOfManaETB(gs, omnath)
	if len(gs.ManaPoolExemptions) != 1 {
		t.Fatalf("ETB precondition failed")
	}

	omnathLocusOfManaLTB(gs, omnath, map[string]interface{}{
		"perm": omnath,
	})

	if len(gs.ManaPoolExemptions) != 0 {
		t.Errorf("LTB should drop exemption; got %d remaining", len(gs.ManaPoolExemptions))
	}
}

func TestOmnathLocusOfMana_GreenManaSurvivesPhaseDrain(t *testing.T) {
	gs := newGame(t, 2)
	omnath := addPerm(gs, 0, "Omnath, Locus of Mana", "creature", "legendary")
	omnathLocusOfManaETB(gs, omnath)

	gameengine.AddMana(gs, gs.Seats[0], "G", 4, "test")
	gameengine.AddMana(gs, gs.Seats[0], "R", 3, "test")
	gameengine.DrainAllPools(gs, "main", "")

	if gs.Seats[0].Mana.G != 4 {
		t.Errorf("expected G=4 retained; got %d", gs.Seats[0].Mana.G)
	}
	if gs.Seats[0].Mana.R != 0 {
		t.Errorf("expected R drained; got %d", gs.Seats[0].Mana.R)
	}
}

// Other seats are NOT exempt from drain — Omnath's clause is "you don't
// lose unspent green mana"; only the controller benefits.
func TestOmnathLocusOfMana_OtherSeatsStillDrainGreen(t *testing.T) {
	gs := newGame(t, 2)
	omnath := addPerm(gs, 0, "Omnath, Locus of Mana", "creature", "legendary")
	omnathLocusOfManaETB(gs, omnath)

	gameengine.AddMana(gs, gs.Seats[1], "G", 3, "test")
	gameengine.DrainAllPools(gs, "main", "")

	if gs.Seats[1].Mana.G != 0 {
		t.Errorf("seat 1's green should drain (Omnath is on seat 0); got %d", gs.Seats[1].Mana.G)
	}
}

// ---------------------------------------------------------------------
// Upwelling — all-seat all-color exemption
// ---------------------------------------------------------------------

func TestUpwelling_AllSeatsKeepAllManaThroughDrain(t *testing.T) {
	gs := newGame(t, 3)
	up := addPerm(gs, 0, "Upwelling", "enchantment")

	upwellingETB(gs, up)

	gameengine.AddMana(gs, gs.Seats[0], "W", 2, "test")
	gameengine.AddMana(gs, gs.Seats[1], "R", 3, "test")
	gameengine.AddMana(gs, gs.Seats[2], "U", 1, "test")
	before := []int{
		gs.Seats[0].Mana.Total(),
		gs.Seats[1].Mana.Total(),
		gs.Seats[2].Mana.Total(),
	}

	gameengine.DrainAllPools(gs, "combat", "end_of_combat")

	for i, b := range before {
		if gs.Seats[i].Mana.Total() != b {
			t.Errorf("seat %d should retain mana under Upwelling; before=%d after=%d",
				i, b, gs.Seats[i].Mana.Total())
		}
	}
}

func TestUpwelling_LTBClearsAllSeatExemption(t *testing.T) {
	gs := newGame(t, 2)
	up := addPerm(gs, 0, "Upwelling", "enchantment")
	upwellingETB(gs, up)
	if len(gs.ManaPoolExemptions) != 1 {
		t.Fatalf("precondition failed")
	}

	upwellingLTB(gs, up, map[string]interface{}{
		"perm": up,
	})

	if len(gs.ManaPoolExemptions) != 0 {
		t.Errorf("LTB should drop Upwelling exemption; got %d remaining",
			len(gs.ManaPoolExemptions))
	}
}

// ---------------------------------------------------------------------
// Cabal Coffers — Add {B} per Swamp
// ---------------------------------------------------------------------

func TestCabalCoffers_AddsBPerSwamp(t *testing.T) {
	gs := newGame(t, 2)
	coffers := addPerm(gs, 0, "Cabal Coffers", "land")
	// Three Swamps.
	for i := 0; i < 3; i++ {
		addPerm(gs, 0, "Swamp", "land", "basic", "swamp")
	}
	// One mountain (not a Swamp).
	addPerm(gs, 0, "Mountain", "land", "basic", "mountain")
	gs.Seats[0].ManaPool = 2

	cabalCoffersActivate(gs, coffers, 0, nil)

	if !coffers.Tapped {
		t.Error("Cabal Coffers should be tapped after activation")
	}
	if gs.Seats[0].Mana.B != 3 {
		t.Errorf("expected B=3 (one per Swamp); got %d", gs.Seats[0].Mana.B)
	}
}

func TestCabalCoffers_FailsWithoutManaCost(t *testing.T) {
	gs := newGame(t, 2)
	coffers := addPerm(gs, 0, "Cabal Coffers", "land")
	addPerm(gs, 0, "Swamp", "land", "swamp")
	gs.Seats[0].ManaPool = 1 // insufficient

	cabalCoffersActivate(gs, coffers, 0, nil)

	if coffers.Tapped {
		t.Error("should not have tapped without paying {2}")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed: insufficient_mana")
	}
}

// ---------------------------------------------------------------------
// Sanctum Weaver — Add X mana per enchantment
// ---------------------------------------------------------------------

func TestSanctumWeaver_AddsXPerEnchantmentInChosenColor(t *testing.T) {
	gs := newGame(t, 2)
	weaver := addPerm(gs, 0, "Sanctum Weaver", "creature")
	// Two enchantments.
	addPerm(gs, 0, "Sylvan Library", "enchantment")
	addPerm(gs, 0, "Rhystic Study", "enchantment")
	// One non-enchantment.
	addPerm(gs, 0, "Llanowar Elves", "creature")

	sanctumWeaverActivate(gs, weaver, 0, nil)

	if !weaver.Tapped {
		t.Error("Sanctum Weaver should be tapped")
	}
	// Default color heuristic: G (Sanctum Weaver is itself green;
	// fallback when no other color dominates).
	if gs.Seats[0].Mana.G != 2 {
		t.Errorf("expected G=2 (one per enchantment); got %d", gs.Seats[0].Mana.G)
	}
}

func TestSanctumWeaver_NoEnchantmentsAddsZero(t *testing.T) {
	gs := newGame(t, 2)
	weaver := addPerm(gs, 0, "Sanctum Weaver", "creature")

	sanctumWeaverActivate(gs, weaver, 0, nil)

	if gs.Seats[0].Mana.Total() != 0 {
		t.Errorf("no enchantments → no mana; got total=%d", gs.Seats[0].Mana.Total())
	}
}

// ---------------------------------------------------------------------
// Ancient Ziggurat — Add 1 mana, creature-spell only
// ---------------------------------------------------------------------

func TestAncientZiggurat_AddsCreatureRestrictedMana(t *testing.T) {
	gs := newGame(t, 2)
	zig := addPerm(gs, 0, "Ancient Ziggurat", "land")

	ancientZigguratActivate(gs, zig, 0, nil)

	if !zig.Tapped {
		t.Error("Ancient Ziggurat should be tapped")
	}
	if gs.Seats[0].Mana.Total() != 1 {
		t.Errorf("expected 1 mana total; got %d", gs.Seats[0].Mana.Total())
	}
	if len(gs.Seats[0].Mana.Restricted) != 1 {
		t.Fatalf("expected 1 Restricted entry; got %d", len(gs.Seats[0].Mana.Restricted))
	}
	r := gs.Seats[0].Mana.Restricted[0]
	if r.Restriction != "creature_spell_only" {
		t.Errorf("restriction should be creature_spell_only; got %q", r.Restriction)
	}
}

func TestAncientZiggurat_RestrictedManaPaysCreatureNotSorcery(t *testing.T) {
	gs := newGame(t, 2)
	zig := addPerm(gs, 0, "Ancient Ziggurat", "land")

	ancientZigguratActivate(gs, zig, 0, nil)

	// Creature spell: payable.
	if !gameengine.PayGenericCost(gs, gs.Seats[0], 1, "creature", "cast", "Test Creature") {
		t.Error("Ziggurat mana should pay a creature spell")
	}
	// Re-tap reset + re-add for sorcery test.
	zig.Tapped = false
	gs.Seats[0].Mana = nil
	gs.Seats[0].ManaPool = 0
	ancientZigguratActivate(gs, zig, 0, nil)

	if gameengine.PayGenericCost(gs, gs.Seats[0], 1, "sorcery", "cast", "Test Sorcery") {
		t.Error("Ziggurat mana should NOT pay a sorcery")
	}
}

// ---------------------------------------------------------------------
// Engine primitive directly — Register/Unregister + multi-card stack
// ---------------------------------------------------------------------

func TestRegisterManaPoolExemption_MultipleCardsStack(t *testing.T) {
	gs := newGame(t, 2)
	omnath := addPerm(gs, 0, "Omnath, Locus of Mana", "creature", "legendary")
	up := addPerm(gs, 1, "Upwelling", "enchantment")
	omnathLocusOfManaETB(gs, omnath)
	upwellingETB(gs, up)

	// Upwelling "any" should win for seat 0 (covered by Upwelling's
	// all-seat scope) — PoolExemptColors returns {"any": true}.
	ex0 := gameengine.PoolExemptColors(gs, gs.Seats[0])
	if !ex0["any"] {
		t.Errorf("Upwelling should give seat 0 'any' exemption; got %v", ex0)
	}

	// Remove Upwelling — seat 0 should fall back to G-only via Omnath.
	upwellingLTB(gs, up, map[string]interface{}{"perm": up})
	ex0 = gameengine.PoolExemptColors(gs, gs.Seats[0])
	if ex0["any"] {
		t.Errorf("Upwelling LTB should drop the 'any' exemption; got %v", ex0)
	}
	if !ex0["G"] {
		t.Errorf("Omnath should still give seat 0 G exemption post-Upwelling-LTB; got %v", ex0)
	}
}
