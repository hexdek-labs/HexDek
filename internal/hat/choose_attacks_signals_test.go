package hat

// Regressions for dev/hat-choose-attacks-r60 — two new ChooseAttackers
// signals: defender deathtouch density as a per-attacker value brake,
// and commander-tax-aware pruning that no longer auto-protects every
// commander from the profitability gate.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// Deathtouch density brake
// ---------------------------------------------------------------------

// TestYggdrasil_ChooseAttackers_DeathtouchDensityBrake: a marginal
// vanilla 2/2 has a safe lane open (seat 2 fields nothing) so
// canSwingProfitably says yes, but seat 1 fields a wall of untapped
// deathtouch blockers. The brake should subtract enough from the
// attacker's value to push it below the swing threshold and keep it
// home — opponents' DT density is the worst-case lane and a 2/2 dying
// to a 1/1 deathtouch swap is a bad trade.
func TestYggdrasil_ChooseAttackers_DeathtouchDensityBrake(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	// Opp life high so the lethal-swing path doesn't trigger.
	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 40

	// Seat 1 fields three untapped 1/1 deathtouch creatures (Spider
	// tribal flavor). Seat 2 fields a vanilla blocker so the R60r5
	// open-lane bonus doesn't fire — without that, swinging at the
	// undefended seat would correctly outweigh the DT brake. We want
	// to isolate the brake's behavior on density alone here.
	for i := 0; i < 3; i++ {
		dt := newTestPermanent(gs.Seats[1], newTestCardMinimal("Spider", []string{"creature"}, 1, nil), 1, 1)
		addKeyword(dt, "deathtouch")
		dt.Tapped = false
	}
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Wall", []string{"creature"}, 1, nil), 0, 4)

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)

	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{atk})
	for _, p := range got {
		if p == atk {
			t.Fatalf("vanilla 2/2 should be held by deathtouch-density brake (3 DT blockers on one opp); got %d attackers", len(got))
		}
	}
}

// TestYggdrasil_ChooseAttackers_DeathtouchBrakeSkippedByTrample: same
// board state, but the attacker has trample. Trample leaks excess past
// the deathtouch chump (deathtouch only marks 1 lethal damage, the rest
// trickles to the player), so the brake must NOT fire. Calibrates that
// the carve-out actually carves.
func TestYggdrasil_ChooseAttackers_DeathtouchBrakeSkippedByTrample(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 40

	for i := 0; i < 3; i++ {
		dt := newTestPermanent(gs.Seats[1], newTestCardMinimal("Spider", []string{"creature"}, 1, nil), 1, 1)
		addKeyword(dt, "deathtouch")
	}

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Beast", []string{"creature"}, 4, nil), 4, 4)
	addKeyword(atk, "trample")

	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{atk})
	found := false
	for _, p := range got {
		if p == atk {
			found = true
		}
	}
	if !found {
		t.Fatalf("trample attacker should bypass DT-density brake; got 0 of 1 attackers")
	}
}

// TestYggdrasil_ChooseAttackers_DeathtouchBrakeSkippedByFirstStrike:
// first-strike attackers kill the deathtouch body in 510.5 before
// taking deathtouch damage, so DT density doesn't apply.
func TestYggdrasil_ChooseAttackers_DeathtouchBrakeSkippedByFirstStrike(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 40

	for i := 0; i < 3; i++ {
		dt := newTestPermanent(gs.Seats[1], newTestCardMinimal("Spider", []string{"creature"}, 1, nil), 1, 1)
		addKeyword(dt, "deathtouch")
	}

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Knight", []string{"creature"}, 2, nil), 2, 2)
	addKeyword(atk, "first strike")

	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{atk})
	found := false
	for _, p := range got {
		if p == atk {
			found = true
		}
	}
	if !found {
		t.Fatalf("first-strike attacker should bypass DT-density brake; got 0 of 1 attackers")
	}
}

// ---------------------------------------------------------------------
// Commander-tax-aware pruning
// ---------------------------------------------------------------------

// TestYggdrasil_ChooseAttackers_VanillaCommanderPrunedToProtectTax:
// a vanilla 2/2 commander swinging into a board where every opponent
// has an untapped 5/5 blocker would die for zero damage AND owe +{2}
// on its next recast (CR §903.8). The old strategic shield kept it in
// the swing pool unconditionally; with commander-tax awareness, a
// non-value-engine non-combo non-attack-trigger commander gets pruned
// just like any other clean-trade attacker.
func TestYggdrasil_ChooseAttackers_VanillaCommanderPrunedToProtectTax(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.CommanderFormat = true
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	gs.Seats[0].CommanderNames = []string{"Vanilla Cmdr"}
	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 40

	// Every opponent has an untapped 5/5 blocker — no safe lane exists.
	for _, seat := range []*gameengine.Seat{gs.Seats[1], gs.Seats[2]} {
		blk := newTestPermanent(seat, newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 5, 5)
		blk.Tapped = false
	}

	cmdr := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Vanilla Cmdr", []string{"creature", "legendary"}, 3, nil), 2, 2)

	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{cmdr})
	for _, p := range got {
		if p == cmdr {
			t.Fatalf("vanilla 2/2 commander into clean 5/5 blockers should be pruned (commander tax); got %d attackers", len(got))
		}
	}
}

// TestYggdrasil_ChooseAttackers_CommanderNearLethalClockKept: a 2/2
// commander that has already dealt 13 commander damage to an opponent
// should still swing into a clean trade — the next hit (or even a
// chump-aided buff later) crosses the §704.6c clock, and that win is
// worth the commander-tax cost.
func TestYggdrasil_ChooseAttackers_CommanderNearLethalClockKept(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.CommanderFormat = true
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	gs.Seats[0].CommanderNames = []string{"Vanilla Cmdr"}
	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 40

	// Seat 1 has accumulated 13 commander damage from our commander —
	// one more clean hit pushes past §704.6c's 21-damage threshold
	// when buffed or doubled in a future combat.
	gs.Seats[1].CommanderDamage = map[int]map[string]int{
		0: {"Vanilla Cmdr": 13},
	}

	for _, seat := range []*gameengine.Seat{gs.Seats[1], gs.Seats[2]} {
		blk := newTestPermanent(seat, newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 5, 5)
		blk.Tapped = false
	}

	cmdr := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Vanilla Cmdr", []string{"creature", "legendary"}, 3, nil), 2, 2)

	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{cmdr})
	found := false
	for _, p := range got {
		if p == cmdr {
			found = true
		}
	}
	if !found {
		t.Fatalf("commander near lethal clock (13 dmg) should keep attacking despite clean trade; got 0 of 1 attackers")
	}
}

// TestCommanderClockNearLethal_BelowThreshold: 12 commander damage is
// below the threshold (13). The helper should return false so the
// commander-tax pruning fires on a clean-trade swing.
func TestCommanderClockNearLethal_BelowThreshold(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"Cmdr"}
	gs.Seats[1].CommanderDamage = map[int]map[string]int{
		0: {"Cmdr": 12},
	}
	cmdrCard := newTestCardMinimal("Cmdr", []string{"creature", "legendary"}, 3, nil)
	if commanderClockNearLethal(gs, 0, cmdrCard, 13) {
		t.Errorf("12 commander damage should not register as near lethal at threshold 13")
	}
	gs.Seats[1].CommanderDamage[0]["Cmdr"] = 13
	if !commanderClockNearLethal(gs, 0, cmdrCard, 13) {
		t.Errorf("13 commander damage should register as near lethal at threshold 13")
	}
}
