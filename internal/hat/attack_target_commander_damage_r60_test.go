package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// attack_target_commander_damage_r60_test.go — regressions for the
// commander-damage path in ChooseAttackTarget. Pre-r60 the picker
// only consulted target.Life for kill-shot detection, ignoring the
// commander-damage clock (CR §704.6c). A Voltron board would aim its
// 5-power commander at the lowest-life opponent even when a higher-
// life target with 17+ accumulated commander damage from us would
// die instead — throwing away one of the deck's primary win lines.
// See yggdrasil.go::bestTarget section #1b.
//
// Helpers (newTestGame, newTestPermanent, newTestCardMinimal,
// primedYggdrasilHat, addKeyword) come from hat_test.go /
// opponent_archetype_test.go.

// setCmdrDmg seeds gs.Seats[victim].CommanderDamage[dealer][cmdrName] = n,
// initializing the maps as needed.
func setCmdrDmg(seat *gameengine.Seat, dealer int, cmdrName string, n int) {
	if seat.CommanderDamage == nil {
		seat.CommanderDamage = map[int]map[string]int{}
	}
	if seat.CommanderDamage[dealer] == nil {
		seat.CommanderDamage[dealer] = map[string]int{}
	}
	seat.CommanderDamage[dealer][cmdrName] = n
}

// Headline ask: a high-life opponent with a near-lethal commander-damage
// clock should be preferred over a low-life opponent with no clock, when
// the swing pushes them over §704.6c's 21-damage line. 5-power commander
// + 17 existing cmdr damage = 22 → lethal.
func TestChooseAttackTarget_CmdrDamageKillShotPrefersClockedSeat(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Turn = 5
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"Voltron Cmdr"}

	// Seat 1: high life (30) but already 17 commander damage from us — one
	// more 5-power swing crosses the lethal-clock line.
	gs.Seats[1].Life = 30
	setCmdrDmg(gs.Seats[1], 0, "Voltron Cmdr", 17)

	// Seat 2: lower life (20) but zero commander damage clock. Pre-r60
	// life-only kill-shot would not fire either (5 < 20), but the
	// life-ramp / step bonuses would prefer this seat slightly. The
	// commander-damage kill-shot should override.
	gs.Seats[2].Life = 20

	h := primedYggdrasilHat(3)

	atk := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Voltron Cmdr", []string{"creature", "legendary"}, 4, nil), 5, 5)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 1 {
		t.Fatalf("expected seat 1 (cmdr-damage kill-shot 17+5=22) over seat 2 (no clock); got %d", got)
	}
}

// Double-strike swings double the projected damage, so a 3-power DS
// commander + 16 existing clock = 16 + 6 = 22 → lethal. A 3-power
// non-DS commander against the same target would project 19, not
// lethal.
func TestChooseAttackTarget_CmdrDamageHonorsDoubleStrike(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Turn = 5
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"DS Cmdr"}
	gs.Seats[1].Life = 30
	setCmdrDmg(gs.Seats[1], 0, "DS Cmdr", 16) // 3-power non-DS hits 19; 3-power DS hits 22
	gs.Seats[2].Life = 18                     // would-be lower-life pick without ds kill-shot

	h := primedYggdrasilHat(3)

	atk := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("DS Cmdr", []string{"creature", "legendary"}, 4, nil), 3, 3)
	addKeyword(atk, "double strike")

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 1 {
		t.Fatalf("DS commander projecting 6 damage onto 16-clock seat is lethal — expected seat 1; got %d", got)
	}
}

// Below-lethal but already-invested clock: the graded "closing-clock"
// bonus should still pull the picker toward the clocked seat over an
// otherwise-equal seat. Use a 2-power commander + 14 clock = 16
// (not lethal), bonus = 14/7 = +2.0.
func TestChooseAttackTarget_CmdrDamageClockedSeatGetsGradedBias(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Turn = 5
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"Grindy Cmdr"}
	gs.Seats[1].Life = 30
	setCmdrDmg(gs.Seats[1], 0, "Grindy Cmdr", 14)
	gs.Seats[2].Life = 30 // identical life, no clock

	h := primedYggdrasilHat(3)

	atk := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Grindy Cmdr", []string{"creature", "legendary"}, 4, nil), 2, 2)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 1 {
		t.Fatalf("equal-life seats with 14-clock vs 0-clock — graded bias should prefer seat 1 (clock); got %d", got)
	}
}

// Inverse: a non-commander attacker against the SAME clocked board should
// NOT receive the commander-damage bonus — the clock is per-commander,
// not per-attacker. The picker should fall through to other scoring.
// Use an opponent with strictly lower life so the life-ramp pulls the
// picker the other way, demonstrating the clocked-seat bias didn't fire.
func TestChooseAttackTarget_NonCommanderAttackerIgnoresClock(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Turn = 5
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"Voltron Cmdr"}

	// Seat 1: 30 life, 17 cmdr-damage clock from us
	gs.Seats[1].Life = 30
	setCmdrDmg(gs.Seats[1], 0, "Voltron Cmdr", 17)

	// Seat 2: 10 life, no clock — the life-ramp + step bumps prefer this
	// seat strongly (below 20 + below 10 thresholds).
	gs.Seats[2].Life = 10

	h := primedYggdrasilHat(3)

	// Attacker is a vanilla creature, NOT our commander.
	atk := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Random Beater", []string{"creature"}, 3, nil), 5, 5)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("non-commander attacker should ignore cmdr-damage clock and prefer low-life seat 2; got %d", got)
	}
}

// Edge: commander-format flag off means no commander damage applies.
// The bonus must not fire — guards against accidentally biasing free-
// form / non-commander games where CommanderDamage is meaningless.
func TestChooseAttackTarget_CmdrDamageSkippedWhenFormatOff(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Turn = 5
	gs.CommanderFormat = false // explicitly disabled
	gs.Seats[0].CommanderNames = []string{"Voltron Cmdr"}

	// Even if a stale CommanderDamage map is set, the format gate in
	// IsCommanderCard returns false → bonus skipped.
	gs.Seats[1].Life = 30
	setCmdrDmg(gs.Seats[1], 0, "Voltron Cmdr", 17)
	gs.Seats[2].Life = 10

	h := primedYggdrasilHat(3)

	atk := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Voltron Cmdr", []string{"creature", "legendary"}, 4, nil), 5, 5)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("non-commander format: cmdr-damage bonus must not fire; expected low-life seat 2; got %d", got)
	}
}

// Zero-clock seat: bonus must not fire when the clock map exists but
// reports 0 damage from our commander. Otherwise the graded bonus
// would tilt early-game targeting toward any opponent that's ever been
// indexed in the map.
func TestChooseAttackTarget_CmdrDamageZeroClockNoBonus(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Turn = 5
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"Voltron Cmdr"}

	gs.Seats[1].Life = 30
	setCmdrDmg(gs.Seats[1], 0, "Voltron Cmdr", 0) // map exists, value 0
	gs.Seats[2].Life = 10                          // strictly lower life

	h := primedYggdrasilHat(3)

	atk := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Voltron Cmdr", []string{"creature", "legendary"}, 4, nil), 5, 5)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("zero-clock seat must not receive any cmdr-damage bias; expected low-life seat 2; got %d", got)
	}
}
