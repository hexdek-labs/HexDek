package hat

// Tests for evasion-aware attack-target selection and profitability
// pruning added in dev/attack-target-selection. These exercise the
// shared poker.go helpers (canSwingProfitably / evasionScore /
// hasUnconditionalEvasion) and the YggdrasilHat ChooseAttackers and
// ChooseAttackTarget integration paths.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// canSwingProfitably
// ---------------------------------------------------------------------

func TestCanSwingProfitably_VanillaIntoCleanBlock(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)
	blk := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
	blk.Tapped = false

	if canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("2/2 into untapped 3/3 should be unprofitable")
	}
}

func TestCanSwingProfitably_TappedBlockerIgnored(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)
	blk := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
	blk.Tapped = true

	if !canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("2/2 into tapped 3/3 should be profitable")
	}
}

func TestCanSwingProfitably_DeathtouchAlwaysProfitable(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Adder", []string{"creature"}, 2, nil), 1, 1)
	addKeyword(atk, "deathtouch")
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 5, 5)

	if !canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("deathtouch attacker should always swing profitably")
	}
}

// R60 — replaces "TrampleAlwaysProfitable". The pre-R60 behavior
// short-circuited canSwingProfitably to true for any trample
// attacker, but trample only leaks damage when attacker.Power >
// sum(blocker.Toughness). A 3-power trampler into a 5-toughness wall
// leaks 0 and just dies for nothing — leakIsMeaningfulAgainst rejects
// the swing. The corollary positive cases ("trample with real leak"
// and "trample with no blockers") still pass.
func TestCanSwingProfitably_TrampleIntoTallWallIsNotProfitable(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Beast", []string{"creature"}, 4, nil), 3, 3)
	addKeyword(atk, "trample")
	// 4/5 wall absorbs all 3 attacker power → 0 trample leak →
	// attacker just dies. The pre-R60 short-circuit hid this; the
	// R60 leak math correctly prunes it.
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 4, 5)

	if canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("3-power trample into a 5-toughness wall leaks 0 damage and should NOT be profitable")
	}
}

// R60 — trample WITH meaningful leak still swings profitably. A 6/6
// trample into a 4/5 blocker leaks 2 damage past, which exceeds the
// leak threshold (≥ 2 AND ≥ half attacker power). The trade burns the
// attacker but advances the win clock by a useful amount.
func TestCanSwingProfitably_TrampleWithMeaningfulLeakIsProfitable(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Big Beast", []string{"creature"}, 6, nil), 6, 6)
	addKeyword(atk, "trample")
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 4, 4)

	if !canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("6-power trample into a 4-toughness blocker leaks 2 damage and should be profitable")
	}
}

func TestCanSwingProfitably_UnblockableAlwaysProfitable(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Slip", []string{"creature"}, 2, nil), 2, 2)
	addKeyword(atk, "unblockable")
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 5, 5)

	if !canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("unblockable attacker should always swing profitably")
	}
}

func TestCanSwingProfitably_ProfitableIfAnyOpponentOpen(t *testing.T) {
	gs := newTestGame(t, 3)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
	if !canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1], gs.Seats[2]}) {
		t.Errorf("profitable if at least one opponent is open")
	}
}

func TestCanSwingProfitably_FlyerIgnoresGroundBlockers(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Drake", []string{"creature"}, 3, nil), 2, 2)
	addKeyword(atk, "flying")
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Ogre", []string{"creature"}, 4, nil), 4, 4)

	if !canSwingProfitably(gs, atk, []*gameengine.Seat{gs.Seats[1]}) {
		t.Errorf("flying attacker into ground-only board should be profitable")
	}
}

// ---------------------------------------------------------------------
// evasionScore
// ---------------------------------------------------------------------

func TestEvasionScore_UnblockableMaxes(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Slip", []string{"creature"}, 2, nil), 2, 2)
	addKeyword(atk, "unblockable")
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 5, nil), 5, 5)

	if got := evasionScore(gs, atk, gs.Seats[1]); got < 0.99 {
		t.Errorf("unblockable score=%.2f, want ~1.0", got)
	}
}

func TestEvasionScore_FlyerOpenSky(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Drake", []string{"creature"}, 3, nil), 2, 2)
	addKeyword(atk, "flying")
	for i := 0; i < 3; i++ {
		newTestPermanent(gs.Seats[1], newTestCardMinimal("Ogre", []string{"creature"}, 4, nil), 4, 4)
	}
	if got := evasionScore(gs, atk, gs.Seats[1]); got < 0.99 {
		t.Errorf("flyer over ground-only=%.2f, want ~1.0", got)
	}
}

func TestEvasionScore_FlyerVsMixedBlockers(t *testing.T) {
	gs := newTestGame(t, 2)
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Drake", []string{"creature"}, 3, nil), 2, 2)
	addKeyword(atk, "flying")
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Ogre", []string{"creature"}, 4, nil), 4, 4)
	flyer := newTestPermanent(gs.Seats[1], newTestCardMinimal("Pegasus", []string{"creature"}, 2, nil), 1, 2)
	addKeyword(flyer, "flying")

	got := evasionScore(gs, atk, gs.Seats[1])
	if got <= 0.4 || got > 0.85 {
		t.Errorf("partial evasion score=%.2f, want in (0.4, 0.85]", got)
	}
}

// ---------------------------------------------------------------------
// hasUnconditionalEvasion
// ---------------------------------------------------------------------

func TestHasUnconditionalEvasion(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, kw := range []string{"unblockable", "shadow", "horsemanship"} {
		p := newTestPermanent(gs.Seats[0], newTestCardMinimal(kw, []string{"creature"}, 2, nil), 2, 2)
		addKeyword(p, kw)
		if !hasUnconditionalEvasion(p) {
			t.Errorf("%s should be unconditional evasion", kw)
		}
	}
	plain := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)
	if hasUnconditionalEvasion(plain) {
		t.Errorf("vanilla creature flagged as unconditional evasion")
	}
	flyer := newTestPermanent(gs.Seats[0], newTestCardMinimal("Drake", []string{"creature"}, 3, nil), 2, 2)
	addKeyword(flyer, "flying")
	if hasUnconditionalEvasion(flyer) {
		t.Errorf("flying is conditional (reach can block) — should NOT be unconditional")
	}
}

// ---------------------------------------------------------------------
// YggdrasilHat ChooseAttackTarget — evasion + archenemy
// ---------------------------------------------------------------------

func TestYggdrasil_ChooseAttackTarget_ShadowAimsAtThreat(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	gs.Seats[1].Life = 8
	gs.Seats[2].Life = 40
	for i := 0; i < 4; i++ {
		newTestPermanent(gs.Seats[2], newTestCardMinimal("Soldier", []string{"creature"}, 2, nil), 4, 4)
	}

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Phantom", []string{"creature"}, 3, nil), 3, 3)
	addKeyword(atk, "shadow")

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("shadow attacker should aim at threat (seat 2); got seat %d", got)
	}
}

func TestYggdrasil_ChooseAttackTarget_FlyerPrefersOpenSky(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	gs.Seats[1].Life = 30
	gs.Seats[2].Life = 30
	guard := newTestPermanent(gs.Seats[1], newTestCardMinimal("Sphinx", []string{"creature"}, 5, nil), 4, 4)
	addKeyword(guard, "flying")
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Ogre", []string{"creature"}, 4, nil), 4, 4)

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Drake", []string{"creature"}, 3, nil), 2, 2)
	addKeyword(atk, "flying")

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("flyer should prefer open-sky seat 2; got seat %d", got)
	}
}

func TestYggdrasil_ChooseAttackTarget_ArchenemyAvoidance(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	h.seatCount = 3
	h.damageDealtTo = []int{0, 36, 4}

	gs.Seats[1].Life = 25
	gs.Seats[2].Life = 25
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("archenemy avoidance: should pick seat 2 (less-focused); got seat %d", got)
	}
}

// ---------------------------------------------------------------------
// YggdrasilHat ChooseAttackers — profitability prune
// ---------------------------------------------------------------------

func TestYggdrasil_ChooseAttackers_PrunesUnprofitable(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	for _, seat := range []*gameengine.Seat{gs.Seats[1], gs.Seats[2]} {
		seat.Life = 30
		for i := 0; i < 3; i++ {
			newTestPermanent(seat, newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
		}
	}
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)

	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{atk})
	for _, p := range got {
		if p == atk {
			t.Fatalf("vanilla 2/2 into walls of 3/3s should be pruned; got %d attackers", len(got))
		}
	}
}

func TestYggdrasil_ChooseAttackers_KeepsTrampleAndEvasion(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	for _, seat := range []*gameengine.Seat{gs.Seats[1], gs.Seats[2]} {
		seat.Life = 30
		for i := 0; i < 3; i++ {
			newTestPermanent(seat, newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)
		}
	}
	tramp := newTestPermanent(gs.Seats[0], newTestCardMinimal("Beast", []string{"creature"}, 4, nil), 3, 3)
	addKeyword(tramp, "trample")
	flyer := newTestPermanent(gs.Seats[0], newTestCardMinimal("Drake", []string{"creature"}, 3, nil), 2, 2)
	addKeyword(flyer, "flying")
	dt := newTestPermanent(gs.Seats[0], newTestCardMinimal("Adder", []string{"creature"}, 2, nil), 1, 1)
	addKeyword(dt, "deathtouch")

	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{tramp, flyer, dt})
	have := map[*gameengine.Permanent]bool{}
	for _, p := range got {
		have[p] = true
	}
	for name, p := range map[string]*gameengine.Permanent{"trample": tramp, "flyer": flyer, "deathtouch": dt} {
		if !have[p] {
			t.Errorf("%s should NOT be pruned by profitability gate", name)
		}
	}
}

// ---------------------------------------------------------------------
// seatHasReachOrFlyingBlocker — per-defender granularity helper used by
// the +3.0 open-sky bonus inside bestTarget.
// ---------------------------------------------------------------------

func TestSeatHasReachOrFlyingBlocker_FlyingDetected(t *testing.T) {
	gs := newTestGame(t, 2)
	flyer := newTestPermanent(gs.Seats[1], newTestCardMinimal("Sphinx", []string{"creature"}, 5, nil), 4, 4)
	addKeyword(flyer, "flying")
	if !seatHasReachOrFlyingBlocker(gs.Seats[1]) {
		t.Fatalf("opponent flyer should register as a flyer-blocker on this seat")
	}
}

func TestSeatHasReachOrFlyingBlocker_ReachDetected(t *testing.T) {
	gs := newTestGame(t, 2)
	spider := newTestPermanent(gs.Seats[1], newTestCardMinimal("Giant Spider", []string{"creature"}, 4, nil), 2, 4)
	addKeyword(spider, "reach")
	if !seatHasReachOrFlyingBlocker(gs.Seats[1]) {
		t.Fatalf("Giant Spider with reach should register as a flyer-blocker")
	}
}

func TestSeatHasReachOrFlyingBlocker_TappedIgnored(t *testing.T) {
	gs := newTestGame(t, 2)
	flyer := newTestPermanent(gs.Seats[1], newTestCardMinimal("Sphinx", []string{"creature"}, 5, nil), 4, 4)
	addKeyword(flyer, "flying")
	flyer.Tapped = true
	if seatHasReachOrFlyingBlocker(gs.Seats[1]) {
		t.Fatalf("a tapped flyer shouldn't count as a viable flyer-blocker")
	}
}

func TestSeatHasReachOrFlyingBlocker_GroundOnlyReturnsFalse(t *testing.T) {
	gs := newTestGame(t, 2)
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
	if seatHasReachOrFlyingBlocker(gs.Seats[1]) {
		t.Fatalf("ground-only board should report no flyer-blocker")
	}
}

// ---------------------------------------------------------------------
// Evasion matching — flyer prefers the opponent without reach/flying
// blockers; the +3.0 open-sky bonus dominates over equal life totals.
// ---------------------------------------------------------------------

func TestYggdrasil_ChooseAttackTarget_FlyerOpenSkyOverGuardedSky(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	// Both opponents have equal life so the linear ramp doesn't tilt.
	gs.Seats[1].Life = 25
	gs.Seats[2].Life = 25

	// Seat 1 has a Sphinx in the air — flying lane closed.
	guard := newTestPermanent(gs.Seats[1], newTestCardMinimal("Sphinx", []string{"creature"}, 5, nil), 4, 4)
	addKeyword(guard, "flying")
	// Seat 2 has only ground creatures — flying lane fully open.
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Ogre", []string{"creature"}, 4, nil), 4, 4)

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Drake", []string{"creature"}, 3, nil), 2, 2)
	addKeyword(atk, "flying")

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("flyer should target the opponent with no flying/reach blockers (seat 2); got seat %d", got)
	}
}

func TestYggdrasil_ChooseAttackTarget_FlyerVsReachIsNotOpenSky(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	gs.Seats[1].Life = 25
	gs.Seats[2].Life = 25

	// Seat 1 has a Giant Spider — reach closes the lane just like flying.
	spider := newTestPermanent(gs.Seats[1], newTestCardMinimal("Giant Spider", []string{"creature"}, 4, nil), 2, 4)
	addKeyword(spider, "reach")
	// Seat 2 has only ground creatures.
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Ogre", []string{"creature"}, 4, nil), 4, 4)

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Drake", []string{"creature"}, 3, nil), 2, 2)
	addKeyword(atk, "flying")

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("reach should block the open-sky bonus; flyer should target seat 2; got seat %d", got)
	}
}

// ---------------------------------------------------------------------
// Life-total focus fire — step bonuses at <10 (+2.0) and <5 (+4.0)
// drive ChooseAttackTarget toward finishing-range opponents.
// ---------------------------------------------------------------------

func TestYggdrasil_ChooseAttackTarget_FocusBelowTenLife(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	// Seat 1: 9 life — past the <10 step.
	// Seat 2: 30 life — only the linear ramp applies.
	gs.Seats[1].Life = 9
	gs.Seats[2].Life = 30

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 1 {
		t.Fatalf("opponent at 9 life should be focus-fired; got seat %d", got)
	}
}

func TestYggdrasil_ChooseAttackTarget_FocusBelowFiveLifeBeatsBelowTen(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	// Seat 1: 9 life (only the <10 bonus).
	// Seat 2: 4 life (both <10 and <5 bonuses, plus a higher linear ramp).
	gs.Seats[1].Life = 9
	gs.Seats[2].Life = 4

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("opponent at 4 life should outrank opponent at 9 life; got seat %d", got)
	}
}

// ---------------------------------------------------------------------
// Zero-power attacker filter — already enforced by ChooseAttackers'
// `if pw <= 0 { continue }` guard. Locked in with a regression test
// so a future refactor can't quietly send 0-power creatures.
// ---------------------------------------------------------------------

func TestYggdrasil_ChooseAttackers_SkipsZeroPower(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	zero := newTestPermanent(gs.Seats[0], newTestCardMinimal("Tarmogoyf-stub", []string{"creature"}, 2, nil), 0, 1)
	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{zero})
	if len(got) != 0 {
		t.Fatalf("0-power creatures must not be sent to combat; got %d attackers", len(got))
	}
}

func TestYggdrasil_ChooseAttackers_SkipsNegativePower(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0

	neg := newTestPermanent(gs.Seats[0], newTestCardMinimal("Mistshroud", []string{"creature"}, 2, nil), -1, 2)
	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{neg})
	if len(got) != 0 {
		t.Fatalf("negative-power creatures must not be sent to combat; got %d attackers", len(got))
	}
}
