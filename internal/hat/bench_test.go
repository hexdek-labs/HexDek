package hat

// Hat benchmarks — target <100ns/call for hot-path methods so the
// 50k games/sec Phase 15 budget isn't eaten by decision overhead.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// BenchmarkGreedy_ChooseAttackers exercises the hot-path combat decision.
func BenchmarkGreedy_ChooseAttackers(b *testing.B) {
	gs := benchGame(b, 4)
	h := &GreedyHat{}
	// 6 legal attackers.
	legal := make([]*gameengine.Permanent, 0, 6)
	for i := 0; i < 6; i++ {
		c := newTestCardMinimal("A", []string{"creature"}, 2, nil)
		legal = append(legal, newTestPermanent(gs.Seats[0], c, 2, 2))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.ChooseAttackers(gs, 0, legal)
	}
}

// BenchmarkPoker_ChooseAttackers — PokerHat on CALL.
func BenchmarkPoker_ChooseAttackers(b *testing.B) {
	gs := benchGame(b, 4)
	h := NewPokerHat()
	legal := make([]*gameengine.Permanent, 0, 6)
	for i := 0; i < 6; i++ {
		c := newTestCardMinimal("A", []string{"creature"}, 2, nil)
		legal = append(legal, newTestPermanent(gs.Seats[0], c, 2, 2))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.ChooseAttackers(gs, 0, legal)
	}
}

// BenchmarkPoker_ThreatBreakdown — the 7-dim cost.
func BenchmarkPoker_ThreatBreakdown(b *testing.B) {
	gs := benchGame(b, 4)
	h := NewPokerHat()
	for i := 0; i < 4; i++ {
		c := newTestCardMinimal("X", []string{"creature"}, 3, nil)
		newTestPermanent(gs.Seats[1], c, 3, 3)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.threatBreakdown(gs, 0, gs.Seats[1])
	}
}

// BenchmarkPoker_ObserveEvent — the broadcast cost we pay on every
// LogEvent. Must stay small.
func BenchmarkPoker_ObserveEvent(b *testing.B) {
	gs := benchGame(b, 4)
	h := NewPokerHat()
	gs.Seats[0].Hat = h
	ev := &gameengine.Event{Kind: "turn_start", Seat: 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ObserveEvent(gs, 0, ev)
	}
}

// BenchmarkGreedy_ChooseCastFromHand — sort-by-CMC baseline.
func BenchmarkGreedy_ChooseCastFromHand(b *testing.B) {
	gs := benchGame(b, 2)
	h := &GreedyHat{}
	hand := make([]*gameengine.Card, 0, 7)
	for i := 0; i < 7; i++ {
		hand = append(hand, newTestCardMinimal("C", []string{"creature"}, i+1, nil))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.ChooseCastFromHand(gs, 0, hand)
	}
}

// BenchmarkPoker_ChooseCastFromHand_HOLD — worst-case HOLD bucketing.
func BenchmarkPoker_ChooseCastFromHand_HOLD(b *testing.B) {
	gs := benchGame(b, 2)
	h := NewPokerHatWithMode(ModeHold)
	hand := make([]*gameengine.Card, 0, 7)
	for i := 0; i < 7; i++ {
		hand = append(hand, newTestCardMinimal("C", []string{"creature"}, i+1, nil))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.ChooseCastFromHand(gs, 0, hand)
	}
}

func benchGame(b *testing.B, n int) *gameengine.GameState {
	b.Helper()
	return gameengine.NewGameState(n, nil, nil)
}
