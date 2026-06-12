package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/validation"
)

// Consolidation step 1 pins — the additive canonical-type scaffolding.

// TestLossCategoryValuesMatchLossCause pins the cross-package value
// contract: validation.LossCategory* constants are intentionally
// value-identical to gameengine.LossCause's stable strings, so
// `string(cause)` round-trips into LossReason.Category without a
// mapping table (validation cannot import gameengine — cycle).
func TestLossCategoryValuesMatchLossCause(t *testing.T) {
	pairs := []struct {
		cause    LossCause
		category string
	}{
		{LossLife, validation.LossCategoryLife},
		{LossEmptyLibrary, validation.LossCategoryEmptyLibrary},
		{LossPoison, validation.LossCategoryPoison},
		{LossCommanderDamage, validation.LossCategoryCommanderDamage},
		{LossEffect, validation.LossCategoryEffect},
	}
	for _, p := range pairs {
		if string(p.cause) != p.category {
			t.Errorf("LossCause %q != validation category %q — the value contract broke", p.cause, p.category)
		}
	}
}

// TestClone_LossDetailDeepCopied pins the clone semantics of the new
// Seat.LossDetail pointer: a populated struct must be value-copied (no
// aliasing between original and clone), and nil must stay nil.
func TestClone_LossDetailDeepCopied(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].LossDetail = &validation.LossReason{
		Category:   validation.LossCategoryCommanderDamage,
		Rule:       "704.6c",
		SourceCard: "Edgar Markov",
	}

	cl := gs.CloneForRollout(nil)

	if cl.Seats[0].LossDetail == nil {
		t.Fatal("clone dropped LossDetail")
	}
	if cl.Seats[0].LossDetail == gs.Seats[0].LossDetail {
		t.Fatal("clone aliases the original LossDetail pointer")
	}
	if *cl.Seats[0].LossDetail != *gs.Seats[0].LossDetail {
		t.Fatalf("clone value mismatch: %+v vs %+v", *cl.Seats[0].LossDetail, *gs.Seats[0].LossDetail)
	}
	cl.Seats[0].LossDetail.SourceCard = "mutated"
	if gs.Seats[0].LossDetail.SourceCard != "Edgar Markov" {
		t.Fatal("mutating the clone leaked into the original")
	}
	if cl.Seats[1].LossDetail != nil {
		t.Fatal("nil LossDetail should clone to nil")
	}
}
