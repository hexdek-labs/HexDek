package hat

// Regressions for dev/hat-assign-blockers-r60 — two new AssignBlockers
// signals: lifelink-killshot at parity trades, and a trample-leak waste
// guard that drops single-chump blocks when the leak still kills us.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// Lifelink-killshot — accept a parity (mutual-kill) trade against a
// lifelink attacker; preventing the 2x life-swing makes parity a win.
// ---------------------------------------------------------------------

// TestYggdrasil_AssignBlockers_LifelinkParityBlocked: 4/4 lifelink
// attacker swings into a defender at high life with a 4/4 vanilla
// blocker. No survivor exists (mutual kill), and the favorable-trade
// fallback rejects equal-stat blockers (`bSum < atkSum`). Pre-fix the
// hat let it through — 4 damage taken AND opp gained 4 (8-life swing)
// to save a 4/4. Post-fix the lifelink-killshot branch picks the 4/4.
func TestYggdrasil_AssignBlockers_LifelinkParityBlocked(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h

	gs.Seats[1].Life = 40 // not at lethal — willDie path is OFF
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Ajani's Pridemate", []string{"creature"}, 3, nil), 4, 4)
	addKeyword(atk, "lifelink")
	blk := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 4, 4)
	blk.Tapped = false

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(out[atk]) != 1 || out[atk][0] != blk {
		t.Fatalf("lifelink-killshot should block 4/4 lifelink with the 4/4 vanilla (mutual kill); got %v", out[atk])
	}
}

// TestYggdrasil_AssignBlockers_NonLifelinkParityNotBlocked: control
// case — without lifelink, the same 4/4-vs-4/4 parity should NOT be
// blocked (no survivor, no favorable trade, not lethal). Pins that
// the lifelink branch is gated on the lifelink keyword and didn't
// regress the general "skip parity trades when ahead" behavior.
func TestYggdrasil_AssignBlockers_NonLifelinkParityNotBlocked(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h

	gs.Seats[1].Life = 40
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 3, nil), 4, 4)
	blk := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 3, nil), 4, 4)
	blk.Tapped = false

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(out[atk]) != 0 {
		t.Fatalf("vanilla 4/4 vs 4/4 should not be blocked at parity (not lethal, no survivor); got %v", out[atk])
	}
}

// TestYggdrasil_AssignBlockers_LifelinkSurvivorPreferred: with a
// strict survivor available (5/5 vs 4/4 lifelink), the survivor wins
// over the parity-killshot branch. Confirms the lifelink branch is
// only consulted when no survivor exists.
func TestYggdrasil_AssignBlockers_LifelinkSurvivorPreferred(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h

	gs.Seats[1].Life = 40
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Pridemate", []string{"creature"}, 3, nil), 4, 4)
	addKeyword(atk, "lifelink")
	// 5/5 vanilla — survives the trade.
	survivor := newTestPermanent(gs.Seats[1], newTestCardMinimal("Ogre", []string{"creature"}, 4, nil), 5, 5)
	// 4/4 parity-killer — would also work, but should NOT be chosen
	// over the survivor.
	parity := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 3, nil), 4, 4)
	_ = parity

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(out[atk]) != 1 || out[atk][0] != survivor {
		t.Fatalf("survivor 5/5 should be picked over parity 4/4 vs lifelink 4/4; got %v", out[atk])
	}
}

// ---------------------------------------------------------------------
// Trample-leak waste guard — drop single-chump blocks against trample
// attackers when the leak still kills us.
// ---------------------------------------------------------------------

// TestYggdrasil_AssignBlockers_SimpleTrampleChumpDropped: 8/8 simple
// trample attacker, defender at 4 life with a 1/1 token. Pre-fix the
// favorable-trade fallback picked the 1/1 (lighter blocker) — chump
// absorbs 1, 7 trample over, we die anyway and burned the chump.
// Post-fix the trample-leak post-decision guard drops the chump.
func TestYggdrasil_AssignBlockers_SimpleTrampleChumpDropped(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h

	gs.Seats[1].Life = 4
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Trampler", []string{"creature"}, 6, nil), 8, 8)
	addKeyword(atk, "trample")
	token := newTestPermanent(gs.Seats[1], newTestCardMinimal("Spirit Token", []string{"creature", "token"}, 0, nil), 1, 1)
	token.Tapped = false

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(out[atk]) != 0 {
		t.Fatalf("trample 8/8 vs 1/1 chump at 4 life should drop the chump (leak still kills); got %v", out[atk])
	}
}

// TestYggdrasil_AssignBlockers_TrampleChumpKeptWhenItSavesUs: same
// trample shape but with a chump big enough to actually save us
// (4/4 chump vs 8/8 simple trample at 5 life: absorbs 4, leak 4, we
// drop to 1 life — survives). Guard must NOT fire here.
func TestYggdrasil_AssignBlockers_TrampleChumpKeptWhenItSavesUs(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h

	gs.Seats[1].Life = 5
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Trampler", []string{"creature"}, 6, nil), 8, 8)
	addKeyword(atk, "trample")
	chump := newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 0, 4)
	chump.Tapped = false

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(out[atk]) != 1 || out[atk][0] != chump {
		t.Fatalf("trample 8/8 with 4-toughness chump at 5 life should block (leak 4, we live at 1); got %v", out[atk])
	}
}

// TestYggdrasil_AssignBlockers_TrampleWasteSkippedForDTChump: a
// deathtouch chump kills the trampler regardless of the leak math —
// the trade-up carve-out keeps the block.
func TestYggdrasil_AssignBlockers_TrampleWasteSkippedForDTChump(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h

	gs.Seats[1].Life = 4
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Trampler", []string{"creature"}, 6, nil), 8, 8)
	addKeyword(atk, "trample")
	dt := newTestPermanent(gs.Seats[1], newTestCardMinimal("Adder", []string{"creature"}, 1, nil), 1, 1)
	addKeyword(dt, "deathtouch")
	dt.Tapped = false

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(out[atk]) != 1 || out[atk][0] != dt {
		t.Fatalf("DT chump should kill trampler in trade-up; guard must not fire; got %v", out[atk])
	}
}

// TestYggdrasil_AssignBlockers_TrampleWasteSkippedForInfectAttacker:
// must-block attackers (infect here) override the trample-waste
// guard — we MUST chump because any unblocked hit is catastrophic
// (poison clock). The guard checks `!mustBlock` so this should pass
// the chump through even though leak still kills us by life.
func TestYggdrasil_AssignBlockers_TrampleWasteSkippedForInfectAttacker(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h

	gs.Seats[1].Life = 4
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Trampler", []string{"creature"}, 6, nil), 8, 8)
	addKeyword(atk, "trample")
	addKeyword(atk, "infect")
	token := newTestPermanent(gs.Seats[1], newTestCardMinimal("Token", []string{"creature", "token"}, 0, nil), 1, 1)
	token.Tapped = false

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(out[atk]) == 0 {
		t.Fatalf("infect attacker is must-block; trample-waste guard must NOT drop the chump; got %v", out[atk])
	}
}
