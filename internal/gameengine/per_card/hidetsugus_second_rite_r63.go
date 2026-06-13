package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// hidetsugus_second_rite_r63.go — per_card handler for Hidetsugu's Second Rite.
//
// Oracle text (Scryfall / ast_dataset):
//
//	If target player has exactly 10 life, Hidetsugu's Second Rite deals 10
//	damage to that player.
//
// {2}{R} Instant. A conditional "10 → 0" snipe. Parses to an inert
// fallback node; the runtime text fallback has no "exactly N life → deal
// N" shape — fully DOA.
//
// Implementation: OnResolve (authoritative). Prefer an opponent sitting
// at EXACTLY 10 life and deal 10. If none qualifies, the spell does
// nothing (it still resolves — the condition simply isn't met).
func init() {
	registerHidetsugusSecondRiteR63(Global())
	AddResetHook(registerHidetsugusSecondRiteR63)
}

func registerHidetsugusSecondRiteR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Hidetsugu's Second Rite", hidetsugusSecondRiteResolve)
}

func hidetsugusSecondRiteResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "hidetsugus_second_rite"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	target := -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		if os.Life == 10 {
			target = opp
			break
		}
	}
	if target >= 0 {
		gameengine.DealDamage(gs, target, 10, "Hidetsugu's Second Rite")
	}
	emit(gs, slug, "Hidetsugu's Second Rite", map[string]interface{}{
		"seat": seat, "target": target,
	})
}
