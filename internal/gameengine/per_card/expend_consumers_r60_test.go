package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// expend_consumers_r60 — 6 handlers wired in expend_consumers_r60.go
// covering the BLB Expend N mechanic (CR §702.190).
// -----------------------------------------------------------------------------

// expendCtx builds the ctx FireExpendTriggers (keywords_expend.go:226)
// forwards to FireCardTrigger("expend", ...).
func expendCtx(src *gameengine.Permanent, threshold, total int) map[string]interface{} {
	return map[string]interface{}{
		"source":     src,
		"controller": src.Controller,
		"threshold":  threshold,
		"total":      total,
	}
}

// -----------------------------------------------------------------------------
// Bark-Knuckle Boxer — Expend 4 → indestructible UEOT
// -----------------------------------------------------------------------------

func TestBarkKnuckleBoxer_ExpendGrantsIndestructibleUEOT(t *testing.T) {
	gs := newGame(t, 2)
	boxer := addPerm(gs, 0, "Bark-Knuckle Boxer", "creature")

	gameengine.FireCardTrigger(gs, "expend", expendCtx(boxer, 4, 4))

	if boxer.Flags["kw:indestructible"] != 1 {
		t.Errorf("expected kw:indestructible=1 on Boxer, got %d", boxer.Flags["kw:indestructible"])
	}
	if len(gs.DelayedTriggers) < 1 {
		t.Errorf("expected next_end_step cleanup registered, got %d delayed triggers", len(gs.DelayedTriggers))
	}
}

func TestBarkKnuckleBoxer_WrongThresholdDoesNothing(t *testing.T) {
	gs := newGame(t, 2)
	boxer := addPerm(gs, 0, "Bark-Knuckle Boxer", "creature")

	// Trailtracker's threshold 8 must not fire Boxer's payoff.
	gameengine.FireCardTrigger(gs, "expend", expendCtx(boxer, 8, 8))

	if boxer.Flags["kw:indestructible"] == 1 {
		t.Errorf("Boxer should not gain indestructible on threshold 8")
	}
}

func TestBarkKnuckleBoxer_SiblingCopyDoesNotFire(t *testing.T) {
	gs := newGame(t, 2)
	b1 := addPerm(gs, 0, "Bark-Knuckle Boxer", "creature")
	b2 := addPerm(gs, 0, "Bark-Knuckle Boxer", "creature")

	// Only b1 is the source; b2 must not gain indestructible just because
	// fireTrigger walks the battlefield by name and dispatches to every
	// bearer of that name.
	gameengine.FireCardTrigger(gs, "expend", expendCtx(b1, 4, 4))

	if b1.Flags["kw:indestructible"] != 1 {
		t.Errorf("source bearer b1 should have indestructible")
	}
	if b2.Flags["kw:indestructible"] == 1 {
		t.Errorf("sibling copy b2 must not fire payoff (src-eq guard)")
	}
}

// -----------------------------------------------------------------------------
// Junkblade Bruiser — Expend 4 → +2/+1 UEOT
// -----------------------------------------------------------------------------

func TestJunkbladeBruiser_ExpendBuffsSelf(t *testing.T) {
	gs := newGame(t, 2)
	bruiser := addPerm(gs, 0, "Junkblade Bruiser", "creature")

	gameengine.FireCardTrigger(gs, "expend", expendCtx(bruiser, 4, 4))

	if len(bruiser.Modifications) != 1 {
		t.Fatalf("expected 1 Modification on Bruiser, got %d", len(bruiser.Modifications))
	}
	m := bruiser.Modifications[0]
	if m.Power != 2 || m.Toughness != 1 {
		t.Errorf("expected +2/+1 mod, got +%d/+%d", m.Power, m.Toughness)
	}
	if m.Duration != "until_end_of_turn" {
		t.Errorf("expected UEOT duration, got %q", m.Duration)
	}
}

// -----------------------------------------------------------------------------
// Bakersbane Duo — Expend 4 → +1/+1 UEOT
// -----------------------------------------------------------------------------

func TestBakersbaneDuo_ExpendBuffsSelf(t *testing.T) {
	gs := newGame(t, 2)
	duo := addPerm(gs, 0, "Bakersbane Duo", "creature")

	gameengine.FireCardTrigger(gs, "expend", expendCtx(duo, 4, 4))

	if len(duo.Modifications) != 1 {
		t.Fatalf("expected 1 Modification on Duo, got %d", len(duo.Modifications))
	}
	m := duo.Modifications[0]
	if m.Power != 1 || m.Toughness != 1 {
		t.Errorf("expected +1/+1 mod, got +%d/+%d", m.Power, m.Toughness)
	}
}

// -----------------------------------------------------------------------------
// Teapot Slinger — Expend 4 → 2 damage to each opponent
// -----------------------------------------------------------------------------

func TestTeapotSlinger_PingsEachOpponent(t *testing.T) {
	gs := newGame(t, 4)
	slinger := addPerm(gs, 0, "Teapot Slinger", "creature")

	startLives := []int{gs.Seats[1].Life, gs.Seats[2].Life, gs.Seats[3].Life}
	gameengine.FireCardTrigger(gs, "expend", expendCtx(slinger, 4, 4))

	for i, opp := range []int{1, 2, 3} {
		if gs.Seats[opp].Life != startLives[i]-2 {
			t.Errorf("seat %d: expected life %d, got %d", opp, startLives[i]-2, gs.Seats[opp].Life)
		}
	}
	if gs.Seats[0].Life != 20 {
		t.Errorf("controller should be untouched, got life %d", gs.Seats[0].Life)
	}
}

func TestTeapotSlinger_SkipsLostOpponents(t *testing.T) {
	gs := newGame(t, 4)
	slinger := addPerm(gs, 0, "Teapot Slinger", "creature")
	gs.Seats[2].Lost = true
	gs.Seats[2].Life = 0

	startSeat1 := gs.Seats[1].Life
	startSeat3 := gs.Seats[3].Life
	gameengine.FireCardTrigger(gs, "expend", expendCtx(slinger, 4, 4))

	if gs.Seats[1].Life != startSeat1-2 {
		t.Errorf("seat 1 (alive) should take 2 dmg")
	}
	if gs.Seats[3].Life != startSeat3-2 {
		t.Errorf("seat 3 (alive) should take 2 dmg")
	}
	if gs.Seats[2].Life != 0 {
		t.Errorf("Lost seat 2 must not take damage; life=%d", gs.Seats[2].Life)
	}
}

// -----------------------------------------------------------------------------
// Wandertale Mentor — Expend 4 → +1/+1 counter on self
// -----------------------------------------------------------------------------

func TestWandertaleMentor_ExpendAddsCounter(t *testing.T) {
	gs := newGame(t, 2)
	mentor := addPerm(gs, 0, "Wandertale Mentor", "creature")

	gameengine.FireCardTrigger(gs, "expend", expendCtx(mentor, 4, 4))

	if mentor.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 +1/+1 counter, got %d", mentor.Counters["+1/+1"])
	}
}

func TestWandertaleMentor_CountersAccumulate(t *testing.T) {
	gs := newGame(t, 2)
	mentor := addPerm(gs, 0, "Wandertale Mentor", "creature")

	// Two separate threshold crossings (e.g. turn 3 crossed 4, turn 4
	// crossed 4 again after the per-turn reset). Counters are permanent
	// and accumulate across triggers.
	gameengine.FireCardTrigger(gs, "expend", expendCtx(mentor, 4, 4))
	gameengine.FireCardTrigger(gs, "expend", expendCtx(mentor, 4, 4))

	if mentor.Counters["+1/+1"] != 2 {
		t.Errorf("expected 2 +1/+1 counters after two triggers, got %d", mentor.Counters["+1/+1"])
	}
}

// -----------------------------------------------------------------------------
// Trailtracker Scout — Expend 8 → return permanent card from graveyard
// -----------------------------------------------------------------------------

func TestTrailtrackerScout_ReturnsBestPermanentOnExpend8(t *testing.T) {
	gs := newGame(t, 2)
	cheap := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact", "cmc:1"}}
	big := &gameengine.Card{Name: "Mind's Eye", Owner: 0, Types: []string{"artifact", "cmc:5"}}
	noise := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant", "cmc:1"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, cheap, big, noise)

	scout := addPerm(gs, 0, "Trailtracker Scout", "creature")
	gameengine.FireCardTrigger(gs, "expend", expendCtx(scout, 8, 8))

	foundBig := false
	for _, c := range gs.Seats[0].Hand {
		if c == big {
			foundBig = true
		}
	}
	if !foundBig {
		t.Errorf("expected Mind's Eye (highest-CMC permanent) in hand; hand=%v", handNames(gs.Seats[0].Hand))
	}
	// Instant must stay in graveyard.
	stillInstant := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == noise {
			stillInstant = true
		}
	}
	if !stillInstant {
		t.Errorf("instant (non-permanent card) must stay in graveyard")
	}
}

func TestTrailtrackerScout_NoEligibleTargetIsCleanNoOp(t *testing.T) {
	gs := newGame(t, 2)
	// Graveyard has only an instant — no permanent card.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}},
	)
	scout := addPerm(gs, 0, "Trailtracker Scout", "creature")

	gameengine.FireCardTrigger(gs, "expend", expendCtx(scout, 8, 8))

	// "Up to one" — should NOT emit per_card_failed (clean no-op).
	if hasEvent(gs, "per_card_failed") > 0 {
		t.Errorf("up-to-one target must not emit per_card_failed on no eligible target")
	}
	// Hand still empty.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("hand should remain empty when no eligible target")
	}
}

func TestTrailtrackerScout_WrongThresholdDoesNothing(t *testing.T) {
	gs := newGame(t, 2)
	big := &gameengine.Card{Name: "Mind's Eye", Owner: 0, Types: []string{"artifact", "cmc:5"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, big)
	scout := addPerm(gs, 0, "Trailtracker Scout", "creature")

	// Threshold 4 (Boxer's threshold) must not fire Scout's payoff.
	gameengine.FireCardTrigger(gs, "expend", expendCtx(scout, 4, 4))

	for _, c := range gs.Seats[0].Hand {
		if c == big {
			t.Errorf("Scout must not fire on threshold 4")
		}
	}
}

// -----------------------------------------------------------------------------
// Engine integration — TrackManaSpentThisTurn drives the trigger end-to-end
// -----------------------------------------------------------------------------

func TestExpendConsumer_TrackManaSpentDrivesTriggerEndToEnd(t *testing.T) {
	gs := newGame(t, 2)
	mentor := addPerm(gs, 0, "Wandertale Mentor", "creature")
	// FireExpendTriggers gates on HasExpendTrigger, which reads the
	// card's AST. addPerm doesn't populate AST, so we stamp the Expend 4
	// keyword by hand to drive the end-to-end path.
	mentor.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "expend", Args: []interface{}{4}},
		},
	}

	// Build to threshold 4 in two installments to confirm the
	// threshold-CROSSING semantic (not "any spend after threshold").
	gameengine.TrackManaSpentThisTurn(gs, 0, 2)
	if mentor.Counters["+1/+1"] != 0 {
		t.Errorf("counter should not fire below threshold; got %d", mentor.Counters["+1/+1"])
	}
	gameengine.TrackManaSpentThisTurn(gs, 0, 2) // 2 → 4 crosses threshold

	if mentor.Counters["+1/+1"] != 1 {
		t.Errorf("expected counter after crossing threshold 4; got %d", mentor.Counters["+1/+1"])
	}

	// Further spending in the SAME turn must not re-fire (CR §702.190a:
	// trigger fires at the crossing, not on subsequent spends).
	gameengine.TrackManaSpentThisTurn(gs, 0, 3)
	if mentor.Counters["+1/+1"] != 1 {
		t.Errorf("expected counter to stay at 1 (no re-fire same turn); got %d", mentor.Counters["+1/+1"])
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — every handler reachable post-registerDefaults.
// -----------------------------------------------------------------------------

func TestExpendConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Bark-Knuckle Boxer",
		"Junkblade Bruiser",
		"Bakersbane Duo",
		"Teapot Slinger",
		"Wandertale Mentor",
		"Trailtracker Scout",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler for %s", name)
		}
	}
}
