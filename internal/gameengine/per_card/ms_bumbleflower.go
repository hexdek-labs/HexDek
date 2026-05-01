package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMsBumbleflower wires Ms. Bumbleflower.
//
// Oracle text:
//
//	Whenever you draw your second card each turn, create a 1/1 white
//	Rabbit creature token.
//	{2}{G}{W}: Draw a card for each token you control.
//
// Implementation:
//   - "Draw your second card" tracking: hooks "player_would_draw" — the
//     pre-draw event fired by FireDrawTriggerObservers (cast_counts.go).
//     We tally `count` against `bumble_drawn_t<turn>` on the controller's
//     seat. If the tally crosses from <2 to >=2 during this event, the
//     trigger fires once per turn.
//     Note: `player_would_draw` is fired for "draw a card" effects
//     (resolve.go #712, cast_counts.go observer ticks) and for the
//     turn-based draw step. It is NOT fired for every internal
//     library→hand move. The dominant CR §120 "draw a card" path uses
//     this hook, so the behavior matches the oracle for normal play.
//   - OnActivated: pays {2}{G}{W} (≈ 4 mana) and draws one card per
//     token the controller controls (creature OR non-creature tokens
//     count, per oracle "for each token you control"). Lands that
//     happen to be tokens (Wandering Fumarole etc.) also count.
func registerMsBumbleflower(r *Registry) {
	r.OnTrigger("Ms. Bumbleflower", "player_would_draw", msBumbleflowerWouldDraw)
	r.OnActivated("Ms. Bumbleflower", msBumbleflowerActivate)
}

func msBumbleflowerWouldDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ms_bumbleflower_second_draw_rabbit"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	drawSeat, ok := ctx["draw_seat"].(int)
	if !ok || drawSeat != perm.Controller {
		return
	}
	count, _ := ctx["count"].(int)
	if count <= 0 {
		count = 1
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	turnKey := bumbleflowerDrawKey(gs.Turn)
	before := seat.Flags[turnKey]
	after := before + count
	seat.Flags[turnKey] = after
	bumbleflowerPruneDrawKeys(seat, gs.Turn)

	// Trigger fires when crossing the second-draw threshold.
	if before >= 2 || after < 2 {
		return
	}

	token := &gameengine.Card{
		Name:          "Rabbit Token",
		Owner:         perm.Controller,
		Types:         []string{"creature", "token", "rabbit", "pip:W"},
		Colors:        []string{"W"},
		BasePower:     1,
		BaseToughness: 1,
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"draws_before":   before,
		"draws_after":    after,
		"token":          "Rabbit Token",
	})
}

func msBumbleflowerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ms_bumbleflower_token_draw"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}
	const cost = 4 // {2}{G}{W}
	if seat.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      seatIdx,
			"required":  cost,
			"available": seat.ManaPool,
		})
		return
	}

	tokenCount := 0
	for _, p := range seat.Battlefield {
		if isToken(p) {
			tokenCount++
		}
	}
	if tokenCount <= 0 {
		// Cost still pays; oracle wording isn't conditional. Spend the
		// mana and emit a zero-draw event so logs show the activation.
		seat.ManaPool -= cost
		gameengine.SyncManaAfterSpend(seat)
		gs.LogEvent(gameengine.Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: cost,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "bumbleflower_activation",
			},
		})
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":   seatIdx,
			"tokens": 0,
			"drew":   0,
		})
		return
	}

	seat.ManaPool -= cost
	gameengine.SyncManaAfterSpend(seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "pay_mana",
		Seat:   seatIdx,
		Amount: cost,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"reason": "bumbleflower_activation",
		},
	})

	drew := 0
	for i := 0; i < tokenCount; i++ {
		if drawOne(gs, seatIdx, src.Card.DisplayName()) != nil {
			drew++
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seatIdx,
		"tokens": tokenCount,
		"drew":   drew,
	})
}

func bumbleflowerDrawKey(turn int) string {
	return fmt.Sprintf("bumble_drawn_t%d", turn+1)
}

func bumbleflowerPruneDrawKeys(seat *gameengine.Seat, currentTurn int) {
	if seat == nil || seat.Flags == nil {
		return
	}
	prefix := "bumble_drawn_t"
	cutoff := currentTurn + 1
	for k := range seat.Flags {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		n := 0
		_, err := fmt.Sscanf(k[len(prefix):], "%d", &n)
		if err != nil {
			continue
		}
		if n < cutoff {
			delete(seat.Flags, k)
		}
	}
}
