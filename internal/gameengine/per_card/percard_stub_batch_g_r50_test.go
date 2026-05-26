package per_card

import (
	"testing"
)

// R50 stub-batch port G — 10 gen_*.go ports across two patterns,
// avoiding all picks from R49 batches A/B/C/D/E/F and the R46 stub
// hunt:
//
// (A) LTB additions on partially-ported handlers — set per-seat or
//     game-level flags on ETB but never cleared them when the source
//     left play. Mirrors the R49 batch C Torbran fix.
//
//   - Kruphix, God of Horizons         clear gs.Flags["no_max_hand_
//                                        size_seat_N"] + seat.Flags
//                                        ["kruphix_unspent_mana_
//                                        to_colorless"] on LTB.
//   - Archelos, Lagoon Mystic          clear gs.Flags["archelos_
//                                        tapped_seat"] + gs.Flags
//                                        ["archelos_etb_mode"] on LTB.
//   - Tannuk, Steadfast Second         clear seat.Flags["tannuk_warp_
//                                        grant_2r_active"] on LTB.
//
// (B) Neuter of misleading auto-gen stubs whose custom_*.go handler
//     already implements the full oracle text — gen body emitted a
//     duplicate parser_gap signal:
//
//   - Silverquill, the Disputant        custom owns casualty grant.
//   - Quandrix, the Proof               custom owns +1/+1 doubler.
//   - Obeka, Brute Chronologist         custom owns end-the-turn.
//   - Ellie, Vengeful Hunter            custom owns sac-and-damage.
//   - Shadowheart, Dark Justiciar       custom owns sac-for-X-cards.
//   - Splinter, Radical Rat             custom owns unblockable.
//   - Phenax, God of Deception          custom owns granted mill.

// ---------------------------------------------------------------------------
// Kruphix LTB
// ---------------------------------------------------------------------------

func TestKruphix_ETBSetsFlagsAndLTBClears(t *testing.T) {
	gs := newGame(t, 2)
	kru := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "legendary")

	kruphixETBSetSeatFlags(gs, kru)

	if gs.Flags["no_max_hand_size_seat_0"] != 1 {
		t.Errorf("ETB should set no_max_hand_size_seat_0 = 1")
	}
	if gs.Seats[0].Flags["kruphix_unspent_mana_to_colorless"] != 1 {
		t.Errorf("ETB should set kruphix_unspent_mana_to_colorless flag")
	}

	kruphixLTBClearFlags(gs, kru, map[string]interface{}{"perm": kru})

	if _, has := gs.Flags["no_max_hand_size_seat_0"]; has {
		t.Errorf("LTB should clear no_max_hand_size_seat_0")
	}
	if _, has := gs.Seats[0].Flags["kruphix_unspent_mana_to_colorless"]; has {
		t.Errorf("LTB should clear kruphix_unspent_mana_to_colorless")
	}
}

func TestKruphix_LTBIgnoresOtherPermLeaving(t *testing.T) {
	gs := newGame(t, 2)
	kru := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "legendary")
	other := addPerm(gs, 0, "Bear", "creature")
	kruphixETBSetSeatFlags(gs, kru)
	kruphixLTBClearFlags(gs, kru, map[string]interface{}{"perm": other})
	if gs.Flags["no_max_hand_size_seat_0"] != 1 {
		t.Errorf("non-Kruphix LTB should NOT clear flag")
	}
}

// ---------------------------------------------------------------------------
// Archelos LTB
// ---------------------------------------------------------------------------

func TestArchelos_ETBSetsFlagAndLTBClears(t *testing.T) {
	gs := newGame(t, 2)
	arc := addPerm(gs, 0, "Archelos, Lagoon Mystic", "creature", "legendary")
	arc.Tapped = true

	archelosETBSetFlag(gs, arc)

	if gs.Flags["archelos_tapped_seat"] != 1 {
		t.Errorf("ETB should set archelos_tapped_seat = 1, got %d", gs.Flags["archelos_tapped_seat"])
	}
	if gs.Flags["archelos_etb_mode"] != 1 {
		t.Errorf("ETB-tap mode should be 1; got %d", gs.Flags["archelos_etb_mode"])
	}

	archelosLTBClearFlag(gs, arc, map[string]interface{}{"perm": arc})

	if _, has := gs.Flags["archelos_tapped_seat"]; has {
		t.Errorf("LTB should clear archelos_tapped_seat")
	}
	if _, has := gs.Flags["archelos_etb_mode"]; has {
		t.Errorf("LTB should clear archelos_etb_mode")
	}
}

func TestArchelos_LTBIgnoresOtherPermLeaving(t *testing.T) {
	gs := newGame(t, 2)
	arc := addPerm(gs, 0, "Archelos, Lagoon Mystic", "creature", "legendary")
	arc.Tapped = true
	other := addPerm(gs, 0, "Bear", "creature")
	archelosETBSetFlag(gs, arc)
	archelosLTBClearFlag(gs, arc, map[string]interface{}{"perm": other})
	if gs.Flags["archelos_tapped_seat"] == 0 {
		t.Errorf("non-Archelos LTB should NOT clear flag")
	}
}

// ---------------------------------------------------------------------------
// Tannuk LTB
// ---------------------------------------------------------------------------

func TestTannuk_ETBSetsFlagAndLTBClears(t *testing.T) {
	gs := newGame(t, 2)
	tan := addPerm(gs, 0, "Tannuk, Steadfast Second", "creature", "legendary")

	tannukETBHasteAnthem(gs, tan)

	if gs.Seats[0].Flags["tannuk_warp_grant_2r_active"] != 1 {
		t.Errorf("ETB should set tannuk_warp_grant_2r_active = 1")
	}

	tannukLTBClearFlag(gs, tan, map[string]interface{}{"perm": tan})

	if _, has := gs.Seats[0].Flags["tannuk_warp_grant_2r_active"]; has {
		t.Errorf("LTB should clear tannuk_warp_grant_2r_active")
	}
}

func TestTannuk_LTBIgnoresOtherPermLeaving(t *testing.T) {
	gs := newGame(t, 2)
	tan := addPerm(gs, 0, "Tannuk, Steadfast Second", "creature", "legendary")
	other := addPerm(gs, 0, "Bear", "creature")
	tannukETBHasteAnthem(gs, tan)
	tannukLTBClearFlag(gs, tan, map[string]interface{}{"perm": other})
	if gs.Seats[0].Flags["tannuk_warp_grant_2r_active"] != 1 {
		t.Errorf("non-Tannuk LTB should NOT clear flag")
	}
}

// ---------------------------------------------------------------------------
// After Reset(): the customs still register the canonical handlers for
// each card that USED to have a neutered gen stub. The gen stubs + their
// register calls were deleted in Versailles Phase 2I; this test
// continues to pin "custom handler is wired" coverage.
// ---------------------------------------------------------------------------

func TestBatchG_CustomHandlersStillRegisteredAfterReset(t *testing.T) {
	Reset()
	cards := []struct {
		name       string
		shapeIsETB bool
		shapeIsAct bool
	}{
		{"Silverquill, the Disputant", true, false},
		{"Quandrix, the Proof", true, false},
		{"Obeka, Brute Chronologist", false, true},
		{"Ellie, Vengeful Hunter", false, true},
		{"Shadowheart, Dark Justiciar", false, true},
		{"Splinter, Radical Rat", false, true},
		{"Phenax, God of Deception", false, true},
	}
	for _, c := range cards {
		etb := HasETB(c.name)
		act := HasActivated(c.name)
		if c.shapeIsETB && !etb {
			t.Errorf("%q: expected custom ETB handler post-Reset, none registered", c.name)
		}
		if c.shapeIsAct && !act {
			t.Errorf("%q: expected custom Activated handler post-Reset, none registered", c.name)
		}
	}
}
