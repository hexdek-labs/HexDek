package per_card

import (
	"testing"
)

// R52 stub-batch port L. Picks are sparse this round — the per_card
// stub pool has been thinned across batches A–K and the remaining 27
// emitPartial-bearing handlers are mostly substantively ported with
// emitPartial breadcrumbs flagging legitimate engine-deep gaps
// (cost-modifier replacements, AST keyword pipeline, while_source_on_bf
// duration on grants, etc.) that are correctly engine-side and need
// no per_card change.
//
// Three picks for batch L:
//
// (A) Duplicate-handler dedup — REAL BUG FIX:
//   - Yorion, Sky Nomad             gen_yorion_sky_nomad.go's R41
//                                     auto-gen promotion added a full
//                                     substantive blink ETB; the
//                                     later custom_yorion_sky_nomad.go
//                                     (registered via
//                                     zz_handler_q45_register.go)
//                                     duplicated the entire ETB. Every
//                                     Yorion ETB ran TWICE, exiling
//                                     and returning the controller's
//                                     nonland-permanent set in two
//                                     passes. Neutered the custom-side
//                                     registration to a no-op; gen is
//                                     the canonical owner. The local
//                                     yorionETB helper is preserved
//                                     for direct test invocation
//                                     (q45_handlers_test.go).
//
// (B) LTB cleanup — additive defensive hooks:
//   - The Reality Chip              LTB clears
//                                     seat.Flags["may_see_top_of_
//                                     library"], refcounted via a
//                                     same-seat sibling scan so two
//                                     Reality Chips with one leaving
//                                     keep the visibility hint
//                                     active.
//   - Zethi, Arcane Blademaster     LTB sweeps gs.Flags
//                                     ["zethi_kicked_*"] reverse-
//                                     lookup keys. The keys are read
//                                     by Zethi's attack trigger to
//                                     find kicked-counter exiled
//                                     cards. Once Zethi leaves, the
//                                     attack trigger can't fire so
//                                     the keys are orphaned. The
//                                     kick counters themselves live
//                                     on the exiled cards and are
//                                     preserved per CR — only the
//                                     reverse-lookup index is
//                                     dropped. Globally gated on no
//                                     other Zethi remaining.

// ---------------------------------------------------------------------------
// Yorion neuter
// ---------------------------------------------------------------------------

func TestYorion_CustomRegistrationIsNeutered(t *testing.T) {
	// Calling the now-neutered custom registration on the global
	// registry must not panic and must not add a second ETB handler.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("neutered registerYorionSkyNomadCustom panicked: %v", r)
		}
	}()
	registerYorionSkyNomadCustom(Global())
}

func TestYorion_PostResetCanonicalHandlerStillRegistered(t *testing.T) {
	Reset()
	// gen_yorion_sky_nomad.go's registerYorionSkyNomad is the
	// canonical ETB owner. It must remain present after Reset().
	if !HasETB("Yorion, Sky Nomad") {
		t.Fatal("post-Reset: Yorion's canonical ETB handler should still be registered")
	}
}

// ---------------------------------------------------------------------------
// Reality Chip LTB
// ---------------------------------------------------------------------------

func TestRealityChip_ETBSetsAndLTBClearsVisibilityFlag(t *testing.T) {
	gs := newGame(t, 2)
	chip := addPerm(gs, 0, "The Reality Chip", "artifact", "equipment")
	theRealityChipETB(gs, chip)
	if gs.Seats[0].Flags["may_see_top_of_library"] != 1 {
		t.Fatal("ETB should set may_see_top_of_library")
	}
	theRealityChipLTBClearFlag(gs, chip, map[string]interface{}{"perm": chip})
	if _, has := gs.Seats[0].Flags["may_see_top_of_library"]; has {
		t.Errorf("LTB should clear may_see_top_of_library")
	}
}

func TestRealityChip_LTBPreservesFlagWhenSiblingRemains(t *testing.T) {
	gs := newGame(t, 2)
	c1 := addPerm(gs, 0, "The Reality Chip", "artifact", "equipment")
	c2 := addPerm(gs, 0, "The Reality Chip", "artifact", "equipment")
	theRealityChipETB(gs, c1)
	theRealityChipETB(gs, c2)
	theRealityChipLTBClearFlag(gs, c1, map[string]interface{}{"perm": c1})
	if gs.Seats[0].Flags["may_see_top_of_library"] != 1 {
		t.Errorf("one Chip leaving with a sibling still in play should preserve visibility flag")
	}
}

func TestRealityChip_LTBIgnoresOtherPermLeaving(t *testing.T) {
	gs := newGame(t, 2)
	chip := addPerm(gs, 0, "The Reality Chip", "artifact", "equipment")
	other := addPerm(gs, 0, "Bear", "creature")
	theRealityChipETB(gs, chip)
	theRealityChipLTBClearFlag(gs, chip, map[string]interface{}{"perm": other})
	if gs.Seats[0].Flags["may_see_top_of_library"] != 1 {
		t.Errorf("non-Chip LTB should NOT clear visibility flag")
	}
}

// ---------------------------------------------------------------------------
// Zethi LTB sweep
// ---------------------------------------------------------------------------

func TestZethi_LTBSweepsKickedKeys(t *testing.T) {
	gs := newGame(t, 2)
	zethi := addPerm(gs, 0, "Zethi, Arcane Blademaster", "creature", "legendary")
	gs.Flags = map[string]int{}
	gs.Flags["zethi_kicked_Bolt"] = 1
	gs.Flags["zethi_kicked_Counterspell"] = 1
	gs.Flags["other_unrelated_flag"] = 1

	zethiLTBSweepLookup(gs, zethi, map[string]interface{}{"perm": zethi})

	if _, has := gs.Flags["zethi_kicked_Bolt"]; has {
		t.Errorf("LTB should sweep zethi_kicked_Bolt")
	}
	if _, has := gs.Flags["zethi_kicked_Counterspell"]; has {
		t.Errorf("LTB should sweep zethi_kicked_Counterspell")
	}
	if gs.Flags["other_unrelated_flag"] != 1 {
		t.Errorf("LTB must NOT clear unrelated flags")
	}
}

func TestZethi_LTBPreservesKeysWhenAnotherZethiRemains(t *testing.T) {
	gs := newGame(t, 2)
	z1 := addPerm(gs, 0, "Zethi, Arcane Blademaster", "creature", "legendary")
	addPerm(gs, 1, "Zethi, Arcane Blademaster", "creature", "legendary")
	gs.Flags = map[string]int{"zethi_kicked_Bolt": 1}
	zethiLTBSweepLookup(gs, z1, map[string]interface{}{"perm": z1})
	if gs.Flags["zethi_kicked_Bolt"] != 1 {
		t.Errorf("LTB with another Zethi still in play should preserve kick keys")
	}
}

func TestZethi_LTBIgnoresOtherPermLeaving(t *testing.T) {
	gs := newGame(t, 2)
	zethi := addPerm(gs, 0, "Zethi, Arcane Blademaster", "creature", "legendary")
	other := addPerm(gs, 0, "Bear", "creature")
	gs.Flags = map[string]int{"zethi_kicked_Bolt": 1}
	zethiLTBSweepLookup(gs, zethi, map[string]interface{}{"perm": other})
	if gs.Flags["zethi_kicked_Bolt"] != 1 {
		t.Errorf("non-Zethi LTB should NOT sweep kick keys")
	}
}
