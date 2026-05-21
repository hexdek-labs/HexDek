package per_card

import (
	"testing"
)

// R49 stub-batch port C — 10 gen_*.go ports across two patterns:
//
// (A) Substantive additions on partially-ported handlers (no custom
//     handler exists; the gen file is the canonical wire-up):
//   - Torbran, Thane of Red Fell      LTB refcount decrement so the
//                                       +2 red-damage bonus doesn't
//                                       stick on the GameState forever
//                                       after Torbran leaves play.
//   - Sokrates, Athenian Teacher      upkeep_controller refresh of
//                                       the kw:hexproof-while-untapped
//                                       runtime flag, and LTB cleanup
//                                       that clears the flag if the
//                                       perm leaves play.
//
// (B) Neuter of misleading auto-gen stubs where a custom_*.go handler
//     already implements the full oracle text — the gen body emitted
//     a duplicate or wrong-shape partial breadcrumb that polluted
//     the parser_gap channel (and in two cases minted broken token
//     side effects):
//   - Ureni, the Song Unending        custom owns divided-X damage.
//   - Jadzi, Oracle of Arcavios //    custom owns magecraft + discard-
//     Journey to the Oracle             bounce activation.
//   - Mister Negative                 custom owns life-exchange ETB.
//   - Ghen, Arcanum Weaver            custom owns the activated
//                                       enchantment reanimate.
//   - Jhoira, Ageless Innovator       custom owns ingenuity counter
//                                       + cheat-from-hand activate.
//   - Gyruda, Doom of Depths          custom owns mill-and-reanimate.
//   - Asmoranomardicadaistinaculdacar custom owns Cookbook tutor +
//                                       sac-2-Foods damage activate.
//   - Galazeth Prismari               custom owns Treasure ETB +
//                                       artifact-tap I/S mana grant.
//                                       (Auto-gen body was minting a
//                                       phantom 1/1 token on top of
//                                       the custom's Treasure — fixed.)

// ---------------------------------------------------------------------------
// Torbran, Thane of Red Fell — LTB cleanup
// ---------------------------------------------------------------------------

// NOTE: Torbran was migrated in R54 from the seat-flag breadcrumb
// scheme to the real engine DamageReplacement primitive
// (gs.DamageReplacements). The R49 batch C tests below have been
// updated to assert against the new registry — both the +2 bonus
// stacking across multiple Torbrans and the LTB unregistration are
// now observable via len(gs.DamageReplacements) instead of seat
// flags. The original test names are preserved for git-blame
// continuity. See percard_damage_replacement_r54_test.go for the
// new tests asserting the actual damage delta on combat / noncombat
// damage paths.

func TestTorbran_ETBSetsBonusAndLTBClears(t *testing.T) {
	gs := newGame(t, 2)
	torbran := addPerm(gs, 0, "Torbran, Thane of Red Fell", "creature", "legendary")

	torbranETBRegisterReplacement(gs, torbran)

	if len(gs.DamageReplacements) != 1 {
		t.Errorf("ETB should register 1 damage replacement, got %d", len(gs.DamageReplacements))
	}

	torbranLTBUnregister(gs, torbran, map[string]interface{}{"perm": torbran})

	if len(gs.DamageReplacements) != 0 {
		t.Errorf("LTB should unregister Torbran's replacement, got %d", len(gs.DamageReplacements))
	}
}

func TestTorbran_TwoTorbransLTBLeavesPartialBonus(t *testing.T) {
	gs := newGame(t, 2)
	t1 := addPerm(gs, 0, "Torbran, Thane of Red Fell", "creature", "legendary")
	t2 := addPerm(gs, 0, "Torbran, Thane of Red Fell", "creature", "legendary")
	torbranETBRegisterReplacement(gs, t1)
	torbranETBRegisterReplacement(gs, t2)
	if len(gs.DamageReplacements) != 2 {
		t.Fatalf("two Torbrans should register 2 damage replacements, got %d", len(gs.DamageReplacements))
	}
	torbranLTBUnregister(gs, t1, map[string]interface{}{"perm": t1})
	if len(gs.DamageReplacements) != 1 {
		t.Errorf("after one Torbran leaves, 1 replacement should remain; got %d", len(gs.DamageReplacements))
	}
}

func TestTorbran_LTBIgnoresOtherPermLeavingPlay(t *testing.T) {
	gs := newGame(t, 2)
	torbran := addPerm(gs, 0, "Torbran, Thane of Red Fell", "creature", "legendary")
	other := addPerm(gs, 0, "Bear", "creature")
	torbranETBRegisterReplacement(gs, torbran)
	torbranLTBUnregister(gs, torbran, map[string]interface{}{"perm": other})
	if len(gs.DamageReplacements) != 1 {
		t.Errorf("non-Torbran LTB should NOT unregister; got %d replacements", len(gs.DamageReplacements))
	}
}

// ---------------------------------------------------------------------------
// Sokrates, Athenian Teacher — upkeep refresh + LTB clear
// ---------------------------------------------------------------------------

func TestSokrates_UpkeepRefreshTogglesByTapState(t *testing.T) {
	gs := newGame(t, 2)
	sok := addPerm(gs, 0, "Sokrates, Athenian Teacher", "creature", "legendary")

	sok.Tapped = false
	sokratesUpkeepRefreshHexproof(gs, sok, map[string]interface{}{})
	if sok.Flags["kw:hexproof"] != 1 {
		t.Errorf("untapped Sokrates should have hexproof refreshed; flags=%+v", sok.Flags)
	}

	sok.Tapped = true
	sokratesUpkeepRefreshHexproof(gs, sok, map[string]interface{}{})
	if sok.Flags["kw:hexproof"] != 0 {
		t.Errorf("tapped Sokrates should lose hexproof on refresh; flags=%+v", sok.Flags)
	}
}

func TestSokrates_LTBClearsHexproof(t *testing.T) {
	gs := newGame(t, 2)
	sok := addPerm(gs, 0, "Sokrates, Athenian Teacher", "creature", "legendary")
	sok.Flags["kw:hexproof"] = 1
	sokratesLTBClearHexproof(gs, sok, map[string]interface{}{"perm": sok})
	if sok.Flags["kw:hexproof"] != 0 {
		t.Errorf("LTB should clear kw:hexproof; flags=%+v", sok.Flags)
	}
}

func TestSokrates_LTBIgnoresOtherPermLeavingPlay(t *testing.T) {
	gs := newGame(t, 2)
	sok := addPerm(gs, 0, "Sokrates, Athenian Teacher", "creature", "legendary")
	other := addPerm(gs, 0, "Bear", "creature")
	sok.Flags["kw:hexproof"] = 1
	sokratesLTBClearHexproof(gs, sok, map[string]interface{}{"perm": other})
	if sok.Flags["kw:hexproof"] != 1 {
		t.Errorf("non-Sokrates LTB should NOT clear hexproof")
	}
}

// ---------------------------------------------------------------------------
// Neutered auto-gen stubs — register functions exist but add no
// behavior. The custom handlers (registered from elsewhere) own the
// real implementation; the test confirms each neuter call is safe.
// ---------------------------------------------------------------------------

func TestBatchC_NeutersAreCallableNoOps(t *testing.T) {
	// Calling each neutered register on the global registry must not
	// panic, and must not introduce a duplicate ETB/activated handler
	// that would compete with the custom path.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("neutered registration panicked: %v", r)
		}
	}()
	r := Global()
	registerUreniTheSongUnending(r)
	registerJadziOracleOfArcaviosJourneyToTheOracle(r)
	registerMisterNegative(r)
	registerGhenArcanumWeaver(r)
	registerJhoiraAgelessInnovator(r)
	registerGyrudaDoomOfDepths(r)
	registerAsmoranomardicadaistinaculdacar(r)
	registerGalazethPrismari(r)
}

// ---------------------------------------------------------------------------
// End-to-end: the customs still fire after Reset() — i.e., the neuters
// haven't disturbed the registry's view of who handles each card.
// ---------------------------------------------------------------------------

func TestBatchC_CustomHandlersStillRegisteredAfterReset(t *testing.T) {
	Reset()
	// All 8 neutered cards should still report a registered ETB or
	// Activated handler (sourced from the custom_*.go file).
	cards := []struct {
		name        string
		shapeIsETB  bool
		shapeIsAct  bool
	}{
		{"Ureni, the Song Unending", true, false},
		{"Jadzi, Oracle of Arcavios // Journey to the Oracle", false, true},
		{"Mister Negative", true, false},
		{"Ghen, Arcanum Weaver", false, true},
		{"Jhoira, Ageless Innovator", false, true},
		{"Gyruda, Doom of Depths", true, false},
		{"Asmoranomardicadaistinaculdacar", true, false},
		{"Galazeth Prismari", true, false},
	}
	for _, c := range cards {
		etb := HasETB(c.name)
		act := HasActivated(c.name)
		if c.shapeIsETB && !etb {
			t.Errorf("%q: expected custom ETB handler, none registered (HasETB=false)", c.name)
		}
		if c.shapeIsAct && !act {
			t.Errorf("%q: expected custom Activated handler, none registered (HasActivated=false)", c.name)
		}
	}
}
