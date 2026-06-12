package per_card

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/validation"
)

// loss_detail_emitwin_s25_test.go — consolidation step 2.5: the emitWin
// opponents-lose path (CR §104.2a — a player wins, each opponent loses)
// dual-writes Seat.LossDetail beside the freeform string, with the
// winning card as SourceCard.

func TestLossDetail_EmitWinStampsOpponents(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(42)), nil)

	emitWin(gs, 1, "thassas_oracle", "Thassa's Oracle", "Thassa's Oracle devotion win")

	if !gs.Seats[1].Won {
		t.Fatalf("winner not marked Won")
	}
	for i, s := range gs.Seats {
		if i == 1 {
			continue
		}
		if !s.Lost {
			t.Fatalf("seat %d not marked Lost after emitWin", i)
		}
		if s.LossDetail == nil {
			t.Fatalf("seat %d LossDetail not stamped (LossReason=%q)", i, s.LossReason)
		}
		if s.LossDetail.Category != validation.LossCategoryEffect ||
			s.LossDetail.Rule != "104.2a" ||
			s.LossDetail.SourceCard != "Thassa's Oracle" {
			t.Fatalf("seat %d LossDetail = %+v, want effect/104.2a/Thassa's Oracle", i, *s.LossDetail)
		}
		if s.LossReason == "" {
			t.Fatalf("seat %d freeform LossReason no longer stamped — dual-write means BOTH", i)
		}
	}
}
