package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBertaWiseExtrapolator wires Berta, Wise Extrapolator.
//
// Oracle text:
//
//	At the beginning of your end step, if you gained 3 or more life this
//	turn, draw a card.
//	{2}{W}{B}: Create a 1/1 white Cleric creature token with lifelink.
//
// Implementation:
//   - OnTrigger "life_gained": tally life gained on the controller's
//     seat into a per-turn flag (`berta_lg_t<turn>`). Stale prior-turn
//     keys are pruned at the same time so seat.Flags stays bounded.
//   - OnTrigger "end_step": gates on active_seat == controller ("your
//     end step"), reads the per-turn tally, and draws one card if the
//     threshold (3) is met. The flag is cleared after firing so the
//     intervening-if condition (CR §603.4) is evaluated against this
//     turn's gains only.
//   - OnActivated: spends {2}{W}{B} (≈ 4 mana in the simplified pool
//     model) and creates a 1/1 white Cleric token whose AST carries the
//     `lifelink` keyword so the engine's combat lifelink path attributes
//     the gain back to Berta's controller.
func registerBertaWiseExtrapolator(r *Registry) {
	r.OnTrigger("Berta, Wise Extrapolator", "life_gained", bertaLifeGained)
	r.OnTrigger("Berta, Wise Extrapolator", "end_step", bertaEndStep)
	r.OnActivated("Berta, Wise Extrapolator", bertaActivate)
}

func bertaLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainSeat, ok := ctx["seat"].(int)
	if !ok || gainSeat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags[bertaLifeKey(gs.Turn)] += amount
}

func bertaEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "berta_end_step_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	turnKey := bertaLifeKey(gs.Turn)
	gained := seat.Flags[turnKey]
	delete(seat.Flags, turnKey)
	bertaPruneLifeKeys(seat, gs.Turn)

	if gained < 3 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"gained":   gained,
			"drew":     0,
			"required": 3,
		})
		return
	}
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drewName := ""
	if drawn != nil {
		drewName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"gained": gained,
		"drew":   1,
		"card":   drewName,
	})
}

func bertaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "berta_create_cleric_token"
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
	const cost = 4 // {2}{W}{B}
	if seat.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      seatIdx,
			"required":  cost,
			"available": seat.ManaPool,
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
			"reason": "berta_cleric_activation",
		},
	})

	token := &gameengine.Card{
		Name:          "Cleric Token",
		Owner:         seatIdx,
		Types:         []string{"creature", "token", "cleric", "pip:W"},
		Colors:        []string{"W"},
		BasePower:     1,
		BaseToughness: 1,
		AST: &gameast.CardAST{
			Name: "Cleric Token",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "lifelink"},
			},
		},
	}
	enterBattlefieldWithETB(gs, seatIdx, token, false)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"token":     "Cleric Token",
		"cost_paid": cost,
		"keywords":  []string{"lifelink"},
	})
}

func bertaLifeKey(turn int) string {
	return fmt.Sprintf("berta_lg_t%d", turn+1)
}

func bertaPruneLifeKeys(seat *gameengine.Seat, currentTurn int) {
	if seat == nil || seat.Flags == nil {
		return
	}
	prefix := "berta_lg_t"
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
