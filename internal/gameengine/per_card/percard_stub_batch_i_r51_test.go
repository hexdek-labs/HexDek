package per_card

import (
	"testing"
)

// R51 stub-batch port I — 10 trigger-only stubs (no OnActivated /
// OnCast / OnResolve registrations) receiving permanent_ltb cleanup
// hooks. Avoids all picks from R49 batches A/B/C/D/E/F, R50 batchG/H,
// and the R46 stub hunt.
//
// Pattern: each handler set a seat-level or game-level Flag on ETB
// (or trigger fire) but never cleared it on source-leaves-play. After
// a Cyclonic Rift, Bojuka Bog, or exile, the contract flag remained
// stuck on the GameState for downstream consumers — same regression
// shape as the R49 batchC Torbran fix.
//
// Picks:
//   - Zaffai and the Tempests       once-per-turn free I/S grant
//   - Nadu, Winged Wisdom           nadu_grant_active target dispatcher
//   - Neriv, Heart of the Storm     entered-this-turn damage doubler
//   - Cecily, Haunted Mage          max_hand_size = 11
//   - The Second Doctor             no_max_hand_size + opponent-attack
//                                    restriction
//   - Kuja, Genome Sorcerer //
//     Trance Kuja, Fate Defied      Flare Star wizard damage doubler
//   - Samut, the Driving Force      per-turn speed-bump gate
//   - Sen Triplets                  per-opponent lock / reveal / play-
//                                    from-other-hand
//   - Storm, Force of Nature        storm_grant_pending (defensive)
//   - Lightning, Army of One        stagger_seat*_until_turn (defensive)

// ---------------------------------------------------------------------------
// Zaffai LTB
// ---------------------------------------------------------------------------

func TestZaffai_ETBSetsAndLTBClearsFlags(t *testing.T) {
	gs := newGame(t, 2)
	z := addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary")
	zaffaiAndTheTempestsETB(gs, z)
	if gs.Seats[0].Flags["zaffai_free_cast_available"] == 0 {
		t.Fatal("ETB should set zaffai_free_cast_available")
	}
	// Add a stale per-turn used marker too.
	gs.Seats[0].Flags["zaffai_free_cast_used_t3"] = 1

	zaffaiLTBClearFlags(gs, z, map[string]interface{}{"perm": z})

	if _, has := gs.Seats[0].Flags["zaffai_free_cast_available"]; has {
		t.Errorf("LTB should clear zaffai_free_cast_available")
	}
	if _, has := gs.Seats[0].Flags["zaffai_free_cast_used_t3"]; has {
		t.Errorf("LTB should sweep zaffai_free_cast_used_* keys")
	}
}

// ---------------------------------------------------------------------------
// Nadu LTB
// ---------------------------------------------------------------------------

func TestNadu_ETBSetsAndLTBClears(t *testing.T) {
	gs := newGame(t, 2)
	nadu := addPerm(gs, 0, "Nadu, Winged Wisdom", "creature", "legendary")
	naduWingedWisdomETB(gs, nadu)
	if gs.Seats[0].Flags["nadu_grant_active"] != 1 {
		t.Fatal("ETB should set nadu_grant_active")
	}
	naduLTBClearFlag(gs, nadu, map[string]interface{}{"perm": nadu})
	if _, has := gs.Seats[0].Flags["nadu_grant_active"]; has {
		t.Errorf("LTB should clear nadu_grant_active")
	}
}

// ---------------------------------------------------------------------------
// Neriv LTB
// ---------------------------------------------------------------------------

func TestNeriv_LTBClearsSeatFlagAndPerPermMarkers(t *testing.T) {
	gs := newGame(t, 2)
	neriv := addPerm(gs, 0, "Neriv, Heart of the Storm", "creature", "legendary")
	other := addPerm(gs, 0, "Bear", "creature")
	nerivETBSetSeatFlag(gs, neriv)
	if other.Flags == nil {
		other.Flags = map[string]int{}
	}
	other.Flags["neriv_doubles_damage"] = 1

	nerivLTBClearFlags(gs, neriv, map[string]interface{}{"perm": neriv})

	if _, has := gs.Seats[0].Flags["neriv_double_etb_damage_active"]; has {
		t.Errorf("LTB should clear neriv_double_etb_damage_active")
	}
	if _, has := other.Flags["neriv_doubles_damage"]; has {
		t.Errorf("LTB should sweep neriv_doubles_damage marker off other perms")
	}
}

// ---------------------------------------------------------------------------
// Cecily LTB (refcounted across two Cecilys)
// ---------------------------------------------------------------------------

func TestCecily_LTBClearsMaxHandSizeFlag(t *testing.T) {
	gs := newGame(t, 2)
	c := addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary")
	cecilyHauntedMageETB(gs, c)
	if gs.Seats[0].Flags["max_hand_size"] != 11 {
		t.Fatal("ETB should set max_hand_size = 11")
	}
	cecilyLTBClearFlag(gs, c, map[string]interface{}{"perm": c})
	if _, has := gs.Seats[0].Flags["max_hand_size"]; has {
		t.Errorf("LTB should clear max_hand_size when no other Cecily remains")
	}
}

func TestCecily_LTBPreservesFlagWhenSecondCecilyRemains(t *testing.T) {
	gs := newGame(t, 2)
	c1 := addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary")
	c2 := addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary")
	cecilyHauntedMageETB(gs, c1)
	cecilyHauntedMageETB(gs, c2)
	cecilyLTBClearFlag(gs, c1, map[string]interface{}{"perm": c1})
	if gs.Seats[0].Flags["max_hand_size"] != 11 {
		t.Errorf("one Cecily leaving with a second still in play should preserve max_hand_size")
	}
}

// ---------------------------------------------------------------------------
// The Second Doctor LTB
// ---------------------------------------------------------------------------

func TestSecondDoctor_LTBClearsAllSeatsNoMaxHandSize(t *testing.T) {
	gs := newGame(t, 3)
	doc := addPerm(gs, 0, "The Second Doctor", "creature", "legendary")
	theSecondDoctorETB(gs, doc)
	for i := range gs.Seats {
		if gs.Seats[i].Flags["no_max_hand_size"] != 1 {
			t.Fatalf("ETB should stamp seat %d", i)
		}
	}
	// Stamp the cant_attack restriction tied to controller +1 = 1.
	gs.Seats[1].Flags["cant_attack_doctor_controller"] = 0 + 1
	gs.Seats[1].Flags["cant_attack_doctor_controller_until_turn"] = gs.Turn + 2

	theSecondDoctorLTB(gs, doc, map[string]interface{}{"perm": doc})

	for i := range gs.Seats {
		if _, has := gs.Seats[i].Flags["no_max_hand_size"]; has {
			t.Errorf("LTB should clear seat %d no_max_hand_size", i)
		}
	}
	if _, has := gs.Seats[1].Flags["cant_attack_doctor_controller"]; has {
		t.Errorf("LTB should clear the attack-restriction tied to this Doctor's controller")
	}
}

// ---------------------------------------------------------------------------
// Kuja LTB
// ---------------------------------------------------------------------------

func TestKuja_LTBClearsDamageDoublerFlag(t *testing.T) {
	gs := newGame(t, 2)
	kuja := addPerm(gs, 0, "Kuja, Genome Sorcerer", "creature", "legendary")
	kujaETBSetDamageDoubler(gs, kuja)
	if gs.Seats[0].Flags["kuja_wizard_damage_doubler_active"] != 1 {
		t.Fatal("ETB should set kuja_wizard_damage_doubler_active")
	}
	kujaLTBClearFlag(gs, kuja, map[string]interface{}{"perm": kuja})
	if _, has := gs.Seats[0].Flags["kuja_wizard_damage_doubler_active"]; has {
		t.Errorf("LTB should clear kuja_wizard_damage_doubler_active")
	}
}

// ---------------------------------------------------------------------------
// Samut LTB — speed counter persists, only turn-gate clears
// ---------------------------------------------------------------------------

func TestSamut_LTBClearsTurnGateButKeepsSpeed(t *testing.T) {
	gs := newGame(t, 2)
	samut := addPerm(gs, 0, "Samut, the Driving Force", "creature", "legendary")
	samutETBInitSpeed(gs, samut)
	gs.Seats[0].Flags["speed_bumped_this_turn"] = 1

	samutLTBClearTurnGate(gs, samut, map[string]interface{}{"perm": samut})

	if _, has := gs.Seats[0].Flags["speed_bumped_this_turn"]; has {
		t.Errorf("LTB should clear the per-turn speed-bump gate")
	}
	if gs.Seats[0].Flags["speed"] == 0 {
		t.Errorf("LTB must NOT clear the speed counter — it's a player property")
	}
}

// ---------------------------------------------------------------------------
// Sen Triplets LTB
// ---------------------------------------------------------------------------

func TestSenTriplets_LTBSweepsLockAndRevealFlags(t *testing.T) {
	gs := newGame(t, 3)
	sen := addPerm(gs, 0, "Sen Triplets", "creature", "legendary")
	gs.Seats[1].Flags["sen_triplets_locked"] = 1
	gs.Seats[1].Flags["sen_triplets_revealed"] = 1
	gs.Seats[0].Flags["sen_triplets_play_from"] = 2

	senTripletsLTBSweep(gs, sen, map[string]interface{}{"perm": sen})

	if _, has := gs.Seats[1].Flags["sen_triplets_locked"]; has {
		t.Errorf("LTB should clear sen_triplets_locked on opponent")
	}
	if _, has := gs.Seats[1].Flags["sen_triplets_revealed"]; has {
		t.Errorf("LTB should clear sen_triplets_revealed on opponent")
	}
	if _, has := gs.Seats[0].Flags["sen_triplets_play_from"]; has {
		t.Errorf("LTB should clear controller's sen_triplets_play_from")
	}
}

// ---------------------------------------------------------------------------
// Storm + Lightning LTB
// ---------------------------------------------------------------------------

func TestStorm_LTBClearsStormGrantPending(t *testing.T) {
	gs := newGame(t, 2)
	storm := addPerm(gs, 0, "Storm, Force of Nature", "creature", "legendary")
	gs.Seats[0].Flags["storm_grant_pending"] = 1
	stormLTBClearFlag(gs, storm, map[string]interface{}{"perm": storm})
	if _, has := gs.Seats[0].Flags["storm_grant_pending"]; has {
		t.Errorf("LTB should clear storm_grant_pending")
	}
}

func TestLightning_LTBSweepsStaggerKeys(t *testing.T) {
	gs := newGame(t, 3)
	lit := addPerm(gs, 0, "Lightning, Army of One", "creature", "legendary")
	gs.Flags = map[string]int{}
	gs.Flags["lightning_stagger_seat1_until_turn"] = 6
	gs.Flags["lightning_stagger_seat2_until_turn"] = 6
	gs.Flags["other_unrelated_flag"] = 1

	lightningLTBClearStagger(gs, lit, map[string]interface{}{"perm": lit})

	if _, has := gs.Flags["lightning_stagger_seat1_until_turn"]; has {
		t.Errorf("LTB should sweep seat1 stagger key")
	}
	if _, has := gs.Flags["lightning_stagger_seat2_until_turn"]; has {
		t.Errorf("LTB should sweep seat2 stagger key")
	}
	if gs.Flags["other_unrelated_flag"] != 1 {
		t.Errorf("LTB must NOT clear unrelated flags")
	}
}

// ---------------------------------------------------------------------------
// Cross-cutting: non-source-perm LTB ignored by every handler.
// ---------------------------------------------------------------------------

func TestBatchI_NonSourceLTBIgnoredByAllHandlers(t *testing.T) {
	gs := newGame(t, 2)
	other := addPerm(gs, 0, "Bear", "creature")
	z := addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary")
	zaffaiAndTheTempestsETB(gs, z)

	// Fire each LTB with the wrong perm; none should clear.
	zaffaiLTBClearFlags(gs, z, map[string]interface{}{"perm": other})
	naduLTBClearFlag(gs, z, map[string]interface{}{"perm": other})
	nerivLTBClearFlags(gs, z, map[string]interface{}{"perm": other})
	cecilyLTBClearFlag(gs, z, map[string]interface{}{"perm": other})
	theSecondDoctorLTB(gs, z, map[string]interface{}{"perm": other})
	kujaLTBClearFlag(gs, z, map[string]interface{}{"perm": other})
	samutLTBClearTurnGate(gs, z, map[string]interface{}{"perm": other})
	senTripletsLTBSweep(gs, z, map[string]interface{}{"perm": other})
	stormLTBClearFlag(gs, z, map[string]interface{}{"perm": other})
	lightningLTBClearStagger(gs, z, map[string]interface{}{"perm": other})

	if gs.Seats[0].Flags["zaffai_free_cast_available"] == 0 {
		t.Errorf("non-source-perm LTB should NOT clear Zaffai's grant flag")
	}
}
