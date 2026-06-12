package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/validation"
)

// loss_detail_dualwrite_s25_test.go — consolidation step 2.5.
//
// Every loss writer that stamps the freeform Seat.LossReason string now
// dual-writes the structured Seat.LossDetail beside it. These tests pin
// each writer's Category/Rule/SourceCard and that the string contract is
// unchanged (readers still parse the string until step 2 flips them).

func requireDetail(t *testing.T, s *Seat, wantCat, wantRule, wantSource string) {
	t.Helper()
	if !s.Lost {
		t.Fatalf("seat not marked Lost")
	}
	if s.LossDetail == nil {
		t.Fatalf("LossDetail not stamped (LossReason=%q)", s.LossReason)
	}
	if s.LossDetail.Category != wantCat {
		t.Fatalf("Category = %q, want %q", s.LossDetail.Category, wantCat)
	}
	if s.LossDetail.Rule != wantRule {
		t.Fatalf("Rule = %q, want %q", s.LossDetail.Rule, wantRule)
	}
	if s.LossDetail.SourceCard != wantSource {
		t.Fatalf("SourceCard = %q, want %q", s.LossDetail.SourceCard, wantSource)
	}
	if s.LossReason == "" {
		t.Fatalf("freeform LossReason no longer stamped — dual-write means BOTH")
	}
}

func TestLossDetail_SBALife704_5a(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	gs.Seats[1].Life = 0
	sba704_5a(gs)
	requireDetail(t, gs.Seats[1], validation.LossCategoryLife, "704.5a", "")
}

func TestLossDetail_SBAEmptyDraw704_5b(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(2)), nil)
	gs.Seats[2].AttemptedEmptyDraw = true
	sba704_5b(gs)
	requireDetail(t, gs.Seats[2], validation.LossCategoryEmptyLibrary, "704.5b", "")
}

func TestLossDetail_SBAPoison704_5c(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(3)), nil)
	gs.Seats[3].PoisonCounters = 10
	sba704_5c(gs)
	requireDetail(t, gs.Seats[3], validation.LossCategoryPoison, "704.5c", "")
}

func TestLossDetail_SBACommanderDamage704_6c(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(4)), nil)
	gs.CommanderFormat = true
	gs.Seats[1].CommanderDamage = map[int]map[string]int{
		2: {"Edgar Markov": 21},
	}
	sba704_6c(gs)
	requireDetail(t, gs.Seats[1], validation.LossCategoryCommanderDamage, "704.6c", "Edgar Markov")
}

func TestLossDetail_MarkSeatLostByEffect104_3e(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(5)), nil)
	if !MarkSeatLostByEffect(gs, 2, "Demonic Pact") {
		t.Fatalf("MarkSeatLostByEffect did not apply")
	}
	requireDetail(t, gs.Seats[2], validation.LossCategoryEffect, "104.3e", "Demonic Pact")
}

func TestLossDetail_Concession104_3a(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(6)), nil)
	ConcedeGame(gs, 0)
	requireDetail(t, gs.Seats[0], validation.LossCategoryConcession, "104.3a", "")
}

// TestLossDetail_LoopDrawCategoryStored pins the §104.4b loop-cap shape
// through the shared helper (the cap path itself needs a pathological
// SBA loop to drive end-to-end; the call site passes exactly this
// literal).
func TestLossDetail_LoopDrawCategoryStored(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(7)), nil)
	markSeatLost(gs.Seats[0], "mandatory loop draw (CR 104.4b via SBA cap)",
		&validation.LossReason{Category: validation.LossCategoryLoopDraw, Rule: "104.4b"})
	requireDetail(t, gs.Seats[0], validation.LossCategoryLoopDraw, "104.4b", "")
}

// TestLossDetail_RoundTripsThroughClone — the structured detail must
// survive CloneForRollout with value (not pointer) semantics, so a
// rollout writer can't corrupt the real game's record.
func TestLossDetail_RoundTripsThroughClone(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(8)), nil)
	gs.Seats[1].Life = 0
	sba704_5a(gs)

	clone := gs.CloneForRollout(rand.New(rand.NewSource(9)))
	cd := clone.Seats[1].LossDetail
	if cd == nil || cd.Category != validation.LossCategoryLife {
		t.Fatalf("LossDetail did not round-trip through clone: %+v", cd)
	}
	if cd == gs.Seats[1].LossDetail {
		t.Fatalf("clone aliases the original LossDetail pointer")
	}
	cd.Category = "corrupted"
	if gs.Seats[1].LossDetail.Category != validation.LossCategoryLife {
		t.Fatalf("mutating the clone's LossDetail corrupted the original")
	}
}
