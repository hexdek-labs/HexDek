package gameengine

// R60 — §704.5a life ≤ 0 loss verification across cross-system edge
// cases. The existing TestSBA_704_5a_* battery covers the basic
// triggers (life = 0 / negative, simultaneous-on-two-seats,
// Platinum Angel save, flag reset on recovery, no-spam-after-loss).
//
// This file pins the cross-system shapes that the basic battery does
// not exercise:
//
//   1. Gain-life prevention (Sulfuric Vortex shape) — a §614 replacement
//      cancels a would-be life gain, so the seat at ≤0 life never
//      climbs back above 0. SBA must still fire the loss.
//
//   2. Simultaneous loss + gain on the SAME seat with net-positive
//      result — sequential `LoseLife` then `GainLife` brings life back
//      above 0 before SBA runs. The loss must NOT fire, because SBA
//      reads `s.Life` at call time per CR §704.3 (not historically).
//
//   3. Lifelink-style "lethal-but-also-drain" — one seat takes lethal
//      life loss while a different seat gains life from the same
//      effect (combat lifelink semantics). The §704.5a loss applies
//      only to the dying seat; the other seat's gain is independent
//      and must not influence the loss check.
//
// Rule citations:
//   §704.3   — SBAs read state at call time
//   §704.5a  — life ≤ 0 → loss
//   §614     — replacement effects (would_gain_life)
//   §702.15  — lifelink (life-gain on damage source's controller)

import (
	"testing"
)

// -----------------------------------------------------------------------------
// Gain-life prevention keeps the dying seat dying.
//
// Setup: seat 0 at 0 life with a "no life gain" replacement registered
// (Sulfuric Vortex / Erebos, God of the Dead / Leyline of Punishment
// shape — modeled here as a synthetic replacement that cancels every
// would_gain_life event on seat 0).
//
// Lifelink, Healing Salve, Soul Warden, etc. all route through
// FireGainLifeEvent before mutating Seat.Life, so the cancellation
// short-circuits the gain at the source — Life stays at 0, and SBA
// fires the loss on the next pass.
// -----------------------------------------------------------------------------

func TestSBA_704_5a_GainLifePrevented_StillLoses(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 0

	// Register a synthetic Sulfuric-Vortex-style cancellation. Real
	// cards do this via RegisterReplacementsForPermanent; we use a
	// hand-built ReplacementEffect to avoid dragging in a specific card.
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      "test:no_life_gain_seat0",
		ControllerSeat: 1,
		Category:       CategoryOther,
		Applies:        func(_ *GameState, ev *ReplEvent) bool { return ev.TargetSeat == 0 },
		ApplyFn:        func(_ *GameState, ev *ReplEvent) { ev.Cancelled = true },
	})

	// A would-be 7-life gain (e.g., Angel's Mercy, lifelink hit, etc.).
	gained, cancelled := FireGainLifeEvent(gs, 0, 7, nil)
	if !cancelled {
		t.Fatal("would_gain_life should have been cancelled by the replacement")
	}
	if gained != 0 && gained != 7 {
		// Either 0 (count reset by some handler) or unchanged is fine —
		// the key contract is Cancelled, which the caller honors before
		// calling state.GainLife.
		_ = gained
	}
	// Critically: Life must NOT have moved. The cancellation lives at
	// the §614 layer; if the test fixture had bypassed FireGainLifeEvent
	// and called state.GainLife directly, this would creep up — pinning
	// that callers all funnel through the replacement gate.
	if gs.Seats[0].Life != 0 {
		t.Fatalf("Life must remain at 0 after a cancelled gain; got %d", gs.Seats[0].Life)
	}

	StateBasedActions(gs)

	if !gs.Seats[0].Lost {
		t.Fatal("seat 0 must lose: at 0 life and cancellation kept it there (CR §704.5a)")
	}
	if countEvents(gs, "sba_704_5a") == 0 {
		t.Fatal("missing sba_704_5a event")
	}
	// Spam-suppression flag should be set per §704.5a contract.
	if !gs.Seats[0].SBA704_5a_emitted {
		t.Fatal("SBA704_5a_emitted should be set on loss")
	}
}

// -----------------------------------------------------------------------------
// Simultaneous loss + gain on the same seat — net-positive survives.
//
// Two effects resolve "simultaneously" from the player's view (e.g.,
// a single stack resolution dealing 5 damage + 5 lifelink-equivalent
// gain on the same seat — Children of Korlis shape: "you gain life
// equal to life lost this turn"). The engine processes them
// sequentially: LoseLife flips Life to 0, GainLife restores it to 5.
//
// §704.3 says SBAs run when a player would receive priority — which is
// AFTER the resolution completes. So SBA reads Life=5 and no-ops.
//
// Pins that sba704_5a doesn't track historical low-water marks.
// -----------------------------------------------------------------------------

func TestSBA_704_5a_LossThenGain_NetPositiveSurvives(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 5

	// Sequential loss then gain, both within the same stack resolution
	// from the player's POV — but SBA sees only the post-resolution Life.
	LoseLife(gs, 0, 5, "test")
	if gs.Seats[0].Life != 0 {
		t.Fatalf("setup: expected Life=0 mid-resolution, got %d", gs.Seats[0].Life)
	}
	GainLife(gs, 0, 5, "test")
	if gs.Seats[0].Life != 5 {
		t.Fatalf("setup: expected Life=5 post-resolution, got %d", gs.Seats[0].Life)
	}

	StateBasedActions(gs)

	if gs.Seats[0].Lost {
		t.Fatal("seat 0 must NOT lose: Life=5 at SBA time; brief intermediate Life=0 must not trigger §704.5a")
	}
	if countEvents(gs, "sba_704_5a") != 0 {
		t.Fatalf("no sba_704_5a should fire; got %d", countEvents(gs, "sba_704_5a"))
	}
	// The transient zero must not have set the spam-suppression flag —
	// it should only be touched when an actual loss attempt runs through
	// the gate.
	if gs.Seats[0].SBA704_5a_emitted {
		t.Fatal("SBA704_5a_emitted should stay false when no loss attempt fires")
	}
}

// -----------------------------------------------------------------------------
// Lifelink-style "lethal-but-also-drain" — different seats.
//
// Combat-lifelink semantics: source on seat 0 deals lethal damage to
// seat 1; lifelink causes seat 0 to gain life equal to that damage.
// The §704.5a loss applies only to the seat that hit ≤0 life — seat 1.
// Seat 0's gain is independent and must not gate or cancel the loss.
//
// We exercise the post-damage state directly (LoseLife + GainLife)
// rather than wiring up the combat pipeline, so the test is hermetic
// to combat ordering changes.
// -----------------------------------------------------------------------------

func TestSBA_704_5a_LethalLossWithDrainGainOnOtherSeat(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 3

	// Lifelink kill: seat 1 takes 5 (lethal-and-then-some); seat 0
	// gains 5. Both belong to the SAME damage event from the seat 1
	// perspective; SBA must not let seat 0's gain undo seat 1's loss.
	LoseLife(gs, 1, 5, "lifelink_attacker")
	GainLife(gs, 0, 5, "lifelink_attacker")
	if gs.Seats[1].Life != -2 {
		t.Fatalf("setup: expected seat 1 Life=-2, got %d", gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != 25 {
		t.Fatalf("setup: expected seat 0 Life=25, got %d", gs.Seats[0].Life)
	}

	StateBasedActions(gs)

	if !gs.Seats[1].Lost {
		t.Fatal("seat 1 must lose at -2 life — opponent's lifelink gain is independent (§704.5a)")
	}
	if gs.Seats[0].Lost {
		t.Fatal("seat 0 must survive: gained 5 life via lifelink, total 25")
	}
	if got := gs.Seats[0].Life; got != 25 {
		t.Fatalf("seat 0 Life must remain 25 after SBA, got %d", got)
	}
	// Exactly one 704.5a fires — only for the dying seat.
	if got := countEvents(gs, "sba_704_5a"); got != 1 {
		t.Fatalf("expected exactly one sba_704_5a (for seat 1), got %d", got)
	}
	// Audit the seat attribution on the loss event.
	ev := lastEventOfKind(gs, "sba_704_5a")
	if ev.Seat != 1 {
		t.Errorf("sba_704_5a Seat field should be 1, got %d", ev.Seat)
	}
}
