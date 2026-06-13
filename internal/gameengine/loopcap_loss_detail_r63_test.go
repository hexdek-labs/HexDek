package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/judge"
)

// loopcap_loss_detail_r63_test.go — r63 win_reason consistency.
//
// The SBA-cap draw path already stamped a LossCategoryLoopDraw detail on
// every seat it marked Lost, so a mandatory-loop draw classifies as
// "draw". The trigger-fire-cap and per-card inline-depth-cap paths
// (stack.go / trigger_stack_bridge.go) set Seat.Lost raw — no LossDetail
// and no Flags["game_draw"] — so the same kind of forced draw would fall
// through heimdall.classifyLossOfSeat's life/poison heuristic and be
// recorded as "mill"/"combat". This pins the fix: those paths now route
// through markSeatLostLoopDraw and set the game_draw flag, matching the
// SBA-cap path.

func TestLoopCapDraw_PerCardDepth_StampsLoopDrawDetail(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"

	card := &Card{Name: "Synthetic Role Loop", Types: []string{"enchantment"}, CMC: 1, Owner: 0}
	perm := &Permanent{Card: card, Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	// A handler that re-entrantly pushes itself trips the inline-depth cap,
	// which collapses the game to a §104.4b draw.
	var loop func(gs *GameState, p *Permanent, ctx map[string]interface{})
	loop = func(gs *GameState, p *Permanent, ctx map[string]interface{}) {
		PushPerCardTrigger(gs, p, loop, nil)
	}
	PushPerCardTrigger(gs, perm, loop, nil)

	if gs.Flags["ended"] != 1 {
		t.Fatal("depth cap did not end the game")
	}
	// The cap path must now flag the game as a draw, like the SBA-cap path.
	if gs.Flags["game_draw"] != 1 {
		t.Error("loop-cap draw did not set Flags[\"game_draw\"]=1 — inconsistent with the SBA-cap path")
	}
	// Every seat it marked Lost must carry the structured loop-draw cause
	// so it classifies as a draw, not the mill/combat heuristic fallback.
	for i, s := range gs.Seats {
		if s == nil || !s.Lost {
			continue
		}
		if s.LossDetail == nil {
			t.Fatalf("seat %d Lost via loop cap with no LossDetail — would misclassify", i)
		}
		if s.LossDetail.Category != judge.LossCategoryLoopDraw {
			t.Errorf("seat %d LossDetail.Category = %q, want %q",
				i, s.LossDetail.Category, judge.LossCategoryLoopDraw)
		}
		if s.LossDetail.Rule != "104.4b" {
			t.Errorf("seat %d LossDetail.Rule = %q, want 104.4b", i, s.LossDetail.Rule)
		}
	}
}
