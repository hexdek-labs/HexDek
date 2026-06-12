package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/validation"
)

// Consolidation step 2 — every engine loss writer dual-writes the
// structured Seat.LossDetail beside the freeform LossReason string.
// These pins cover each writer; the freeform string must be unchanged
// (display + legacy-classifier compatibility).

func TestMarkSeatLost_DualWritesDetail(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	d := &validation.LossReason{Category: validation.LossCategoryPoison, Rule: "704.5c"}
	markSeatLost(gs.Seats[0], "ten or more poison counters (CR 704.5c)", d)

	s := gs.Seats[0]
	if !s.Lost || s.LossReason != "ten or more poison counters (CR 704.5c)" {
		t.Fatalf("freeform path changed: Lost=%v reason=%q", s.Lost, s.LossReason)
	}
	if s.LossDetail != d {
		t.Fatalf("LossDetail not stamped")
	}
}

func TestSBA_LifeZero_StampsLossDetail(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Life = -1
	StateBasedActions(gs)

	s := gs.Seats[0]
	if !s.Lost {
		t.Fatal("seat with negative life must lose (704.5a)")
	}
	if s.LossDetail == nil || s.LossDetail.Category != validation.LossCategoryLife || s.LossDetail.Rule != "704.5a" {
		t.Fatalf("704.5a writer must stamp LossDetail{life,704.5a}; got %+v", s.LossDetail)
	}
}

func TestSBA_Poison_StampsLossDetail(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].PoisonCounters = 10
	StateBasedActions(gs)

	s := gs.Seats[0]
	if !s.Lost {
		t.Fatal("seat with 10 poison must lose (704.5c)")
	}
	if s.LossDetail == nil || s.LossDetail.Category != validation.LossCategoryPoison || s.LossDetail.Rule != "704.5c" {
		t.Fatalf("704.5c writer must stamp LossDetail{poison,704.5c}; got %+v", s.LossDetail)
	}
}

func TestSBA_EmptyLibraryDraw_StampsLossDetail(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].AttemptedEmptyDraw = true
	StateBasedActions(gs)

	s := gs.Seats[0]
	if !s.Lost {
		t.Fatal("seat that drew from empty library must lose (704.5b)")
	}
	if s.LossDetail == nil || s.LossDetail.Category != validation.LossCategoryEmptyLibrary || s.LossDetail.Rule != "704.5b" {
		t.Fatalf("704.5b writer must stamp LossDetail{empty_library,704.5b}; got %+v", s.LossDetail)
	}
}

func TestConcedeGame_StampsLossDetail(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	ConcedeGame(gs, 1)

	s := gs.Seats[1]
	if !s.Lost || s.LossReason != "concession" {
		t.Fatalf("concession freeform path changed: Lost=%v reason=%q", s.Lost, s.LossReason)
	}
	if s.LossDetail == nil || s.LossDetail.Category != validation.LossCategoryConcession || s.LossDetail.Rule != "104.3a" {
		t.Fatalf("ConcedeGame must stamp LossDetail{concession,104.3a}; got %+v", s.LossDetail)
	}
}

// TestSeatEliminated_EventCarriesLossCategory pins the elimination
// event's structured detail keys — analytics' inferKiller reads them.
func TestSeatEliminated_EventCarriesLossCategory(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.EventPolicy = EventLogFull
	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "21+ commander damage from Edgar Markov (CR 704.6c)"
	gs.Seats[0].LossDetail = &validation.LossReason{
		Category:   validation.LossCategoryCommanderDamage,
		Rule:       "704.6c",
		SourceCard: "Edgar Markov",
	}
	HandleSeatElimination(gs, 0)

	for _, ev := range gs.EventLog {
		if ev.Kind != "seat_eliminated" {
			continue
		}
		if got, _ := ev.Details["loss_category"].(string); got != validation.LossCategoryCommanderDamage {
			t.Errorf("loss_category = %q, want %q", got, validation.LossCategoryCommanderDamage)
		}
		if got, _ := ev.Details["loss_source_card"].(string); got != "Edgar Markov" {
			t.Errorf("loss_source_card = %q, want Edgar Markov", got)
		}
		return
	}
	t.Fatal("no seat_eliminated event emitted")
}
