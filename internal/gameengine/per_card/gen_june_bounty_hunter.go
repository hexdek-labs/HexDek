package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJuneBountyHunter wires June, Bounty Hunter.
//
// Oracle text (Scryfall, verified):
//
//	June can't be blocked as long as you've drawn two or more cards
//	this turn.
//	{1}, Sacrifice another creature: Create a Clue token. Activate
//	only during your turn. (It's an artifact with "{2}, Sacrifice this
//	token: Draw a card.")
//
// Implementation (R49 stub port):
//   - "Can't be blocked while you've drawn 2+ this turn": Flags["unblockable"]
//     is checked by combat blocker-legality. Refreshed at combat_begin
//     and at ETB based on seat.Turn.CardsDrawn.
//   - Activated cost: {1} + sac another creature. Engine activation
//     dispatch pays mana before reaching here; defensive seat.ManaPool
//     >= 1 check kept for fixture callers.
//   - "Activate only during your turn": guard on active_seat ==
//     controller. The auto-gen handler had no such gate.
//   - Sac victim: smallest non-commander other creature.
//   - Effect: spawn a Clue token via engine-provided CreateClueToken,
//     which carries the {2}, Sac: Draw a card surface.
func registerJuneBountyHunter(r *Registry) {
	r.OnETB("June, Bounty Hunter", juneBountyHunterETB)
	r.OnActivated("June, Bounty Hunter", juneBountyHunterActivate)
	r.OnTrigger("June, Bounty Hunter", "combat_begin", juneBountyHunterCombatBegin)
}

func juneBountyHunterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "june_bounty_hunter_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	juneRefreshUnblockable(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func juneBountyHunterCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	juneRefreshUnblockable(gs, perm)
}

func juneRefreshUnblockable(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if seat.Turn.CardsDrawn >= 2 {
		perm.Flags["unblockable"] = 1
	} else {
		delete(perm.Flags, "unblockable")
	}
}

func juneBountyHunterActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "june_bounty_hunter_sac_clue"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	if ctx != nil {
		if active, ok := ctx["active_seat"].(int); ok && active != seatIdx {
			emitFail(gs, slug, src.Card.DisplayName(), "not_controllers_turn", map[string]interface{}{
				"active_seat": active,
			})
			return
		}
	}

	if seat.ManaPool < 1 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  1,
			"available": seat.ManaPool,
		})
		return
	}

	var victim *gameengine.Permanent
	bestPT := 1 << 30
	bestTS := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == src || !p.IsCreature() {
			continue
		}
		if yawgmothIsCommander(gs, p) {
			continue
		}
		pt := gs.PowerOf(p) + gs.ToughnessOf(p)
		if pt < bestPT || (pt == bestPT && p.Timestamp > bestTS) {
			bestPT = pt
			bestTS = p.Timestamp
			victim = p
		}
	}
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_to_sacrifice", nil)
		return
	}

	seat.ManaPool -= 1
	gameengine.SyncManaAfterSpend(seat)
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "june_sac_cost")
	gameengine.CreateClueToken(gs, seatIdx)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       seatIdx,
		"sacrificed": victimName,
		"token":      "Clue Token",
	})
}
