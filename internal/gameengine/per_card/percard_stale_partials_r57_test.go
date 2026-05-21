package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R57 stale-partial swap-only ports — the six cards flagged in
// docs/percard-stub-census-r56.md as having an existing R54/R55
// primitive whose handler still operates via flag-set breadcrumbs.
//
// Each test asserts the new primitive registration count and the
// LTB cleanup. No new engine surface — these are pure consumer-side
// wirings.
//
//   1. Ozai, the Phoenix King      → RegisterManaPoolExemption (R)
//   2. Kruphix, God of Horizons    → RegisterManaPoolExemption (WUBRGC)
//   3. Zaffai and the Tempests     → existing ZoneCastPolicy retained;
//                                    stale partial dropped
//   4. Aminatou, Veil Piercer      → RegisterZoneCastPolicy
//                                    (hand, enchantment predicate)
//   5. Tannuk, Steadfast Second    → RegisterZoneCastPolicy
//                                    (hand, artifact OR red creature,
//                                     ManaCost=3, ExileOnResolve)
//   6. Sen Triplets                → RegisterZoneCastPolicy
//                                    (hand, OwnerScope=opponents,
//                                     CasterScope=controller, UEOT)

// ---------------------------------------------------------------------------
// 1. Ozai — mana pool exemption R
// ---------------------------------------------------------------------------

func TestOzai_ETBRegistersManaPoolExemption(t *testing.T) {
	gs := newGame(t, 2)
	pre := len(gs.ManaPoolExemptions)
	ozai := addPerm(gs, 0, "Ozai, the Phoenix King", "creature", "legendary")
	ozaiETBSetFlagsAndConditionalKW(gs, ozai)
	if got := len(gs.ManaPoolExemptions) - pre; got != 1 {
		t.Errorf("Ozai ETB should register 1 mana-pool exemption; delta=%d", got)
	}
	// LTB drops it.
	ozaiLTBUnregister(gs, ozai, map[string]interface{}{"perm": ozai})
	if len(gs.ManaPoolExemptions) != pre {
		t.Errorf("Ozai LTB should drop exemption; pre=%d after=%d", pre, len(gs.ManaPoolExemptions))
	}
}

// ---------------------------------------------------------------------------
// 2. Kruphix — mana pool exemption WUBRGC
// ---------------------------------------------------------------------------

func TestKruphix_ETBRegistersManaPoolExemption(t *testing.T) {
	gs := newGame(t, 2)
	pre := len(gs.ManaPoolExemptions)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "legendary", "god")
	kruphixETBSetSeatFlags(gs, kruphix)
	if got := len(gs.ManaPoolExemptions) - pre; got != 1 {
		t.Errorf("Kruphix ETB should register 1 mana-pool exemption; delta=%d", got)
	}
	kruphixLTBClearFlags(gs, kruphix, map[string]interface{}{"perm": kruphix})
	if len(gs.ManaPoolExemptions) != pre {
		t.Errorf("Kruphix LTB should drop exemption; pre=%d after=%d", pre, len(gs.ManaPoolExemptions))
	}
}

// ---------------------------------------------------------------------------
// 3. Zaffai — stale partial removed (no count change expected; just
//    confirms the partial is no longer emitted at registration).
// ---------------------------------------------------------------------------

func TestZaffai_NoStaleEmitPartialAtETB(t *testing.T) {
	gs := newGame(t, 2)
	zaffai := addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary")
	zaffaiAndTheTempestsETB(gs, zaffai)
	// Scan event log for the dropped partial marker.
	for _, ev := range gs.EventLog {
		if ev.Details == nil {
			continue
		}
		if reason, ok := ev.Details["reason"].(string); ok {
			if reason == "once_per_turn_cap_enforced_via_consume_trigger_cast_pipeline_must_check_zaffai_free_cast_used_t<turn>" {
				t.Errorf("Zaffai ETB should no longer emit the stale partial; saw event %+v", ev)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Aminatou — ZoneCastPolicy on enchantment-in-hand
// ---------------------------------------------------------------------------

func TestAminatou_ETBRegistersZoneCastPolicy(t *testing.T) {
	gs := newGame(t, 2)
	pre := len(gs.ZoneCastPolicies)
	am := addPerm(gs, 0, "Aminatou, Veil Piercer", "creature", "legendary")
	aminatouVeilPiercerETB(gs, am)
	if got := len(gs.ZoneCastPolicies) - pre; got != 1 {
		t.Errorf("Aminatou ETB should register 1 ZoneCastPolicy; delta=%d", got)
	}

	// Verify the policy predicate matches an enchantment card.
	ench := &gameengine.Card{Name: "Sylvan Library", Owner: 0, Types: []string{"enchantment"}}
	if !aminatouEnchantmentPredicate(ench) {
		t.Errorf("predicate should match enchantment card")
	}
	bear := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	if aminatouEnchantmentPredicate(bear) {
		t.Errorf("predicate should NOT match non-enchantment card")
	}

	aminatouVeilPiercerLTB(gs, am, map[string]interface{}{"perm": am})
	if len(gs.ZoneCastPolicies) != pre {
		t.Errorf("Aminatou LTB should drop policy; pre=%d after=%d", pre, len(gs.ZoneCastPolicies))
	}
}

// ---------------------------------------------------------------------------
// 5. Tannuk — warp ZoneCastPolicy
// ---------------------------------------------------------------------------

func TestTannuk_ETBRegistersWarpPolicy(t *testing.T) {
	gs := newGame(t, 2)
	pre := len(gs.ZoneCastPolicies)
	tannuk := addPerm(gs, 0, "Tannuk, Steadfast Second", "creature", "legendary")
	tannukETBHasteAnthem(gs, tannuk)
	if got := len(gs.ZoneCastPolicies) - pre; got != 1 {
		t.Errorf("Tannuk ETB should register 1 ZoneCastPolicy; delta=%d", got)
	}
}

func TestTannuk_WarpPredicateMatchesArtifactOrRedCreature(t *testing.T) {
	// Artifact: yes.
	artifact := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}}
	if !tannukWarpPredicate(artifact) {
		t.Errorf("warp predicate should match artifact")
	}
	// Red creature via Colors: yes.
	red := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"creature"}, Colors: []string{"R"}}
	if !tannukWarpPredicate(red) {
		t.Errorf("warp predicate should match red creature via Colors")
	}
	// Red creature via pip:R: yes.
	redPip := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"creature", "pip:R"}}
	if !tannukWarpPredicate(redPip) {
		t.Errorf("warp predicate should match red creature via pip:R type tag")
	}
	// Non-red creature: no.
	blue := &gameengine.Card{Name: "Merfolk", Owner: 0, Types: []string{"creature"}, Colors: []string{"U"}}
	if tannukWarpPredicate(blue) {
		t.Errorf("warp predicate should NOT match blue creature")
	}
	// Enchantment: no.
	ench := &gameengine.Card{Name: "Sylvan Library", Owner: 0, Types: []string{"enchantment"}}
	if tannukWarpPredicate(ench) {
		t.Errorf("warp predicate should NOT match enchantment")
	}
}

// ---------------------------------------------------------------------------
// 6. Sen Triplets — cast-from-opp-hand ZoneCastPolicy
// ---------------------------------------------------------------------------

func TestSenTriplets_UpkeepRegistersOppHandPolicy(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 25 // highest-life opponent → targeted
	sen := addPerm(gs, 0, "Sen Triplets", "creature", "legendary")
	pre := len(gs.ZoneCastPolicies)

	senTripletsUpkeep(gs, sen, map[string]interface{}{"active_seat": 0})

	if got := len(gs.ZoneCastPolicies) - pre; got != 1 {
		t.Errorf("Sen Triplets upkeep should register 1 ZoneCastPolicy; delta=%d", got)
	}
	// Confirm the policy has the opponents/controller scope.
	last := gs.ZoneCastPolicies[len(gs.ZoneCastPolicies)-1]
	if last.OwnerScope != "opponents" {
		t.Errorf("expected OwnerScope=opponents; got %q", last.OwnerScope)
	}
	if last.CasterScope != "controller" {
		t.Errorf("expected CasterScope=controller; got %q", last.CasterScope)
	}
	if last.Zone != gameengine.ZoneHand {
		t.Errorf("expected Zone=hand; got %q", last.Zone)
	}
	if !last.SpendAnyColor {
		t.Errorf("expected SpendAnyColor=true")
	}
}

func TestSenTriplets_LTBDropsPolicy(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 25
	sen := addPerm(gs, 0, "Sen Triplets", "creature", "legendary")
	senTripletsUpkeep(gs, sen, map[string]interface{}{"active_seat": 0})

	pre := len(gs.ZoneCastPolicies)
	if pre == 0 {
		t.Fatalf("setup: expected at least 1 policy registered")
	}
	senTripletsLTBSweep(gs, sen, map[string]interface{}{"perm": sen})
	if len(gs.ZoneCastPolicies) != pre-1 {
		t.Errorf("LTB sweep should drop the policy; %d → %d", pre, len(gs.ZoneCastPolicies))
	}
}
