package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDemonicPact wires Demonic Pact.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Demonic%20Pact):
//
//	At the beginning of your upkeep, choose one that hasn't been chosen —
//	  • This enchantment deals 4 damage to any target and you gain 4 life.
//	  • Target opponent discards two cards.
//	  • Draw two cards.
//	  • You lose the game.
//
// {2}{B}{B} Enchantment. The classic "ramp into your own loss" — turns 1-3
// of the Pact are pure value (12 life swing, 2 discards forced, 2 cards
// drawn), but turn 4's only remaining choice is "lose the game". Standard
// EDH play is to sac, bounce, or steal control of Pact before turn 4.
//
// Implementation:
//   - OnTrigger("upkeep"): only fires on controller's own upkeep. Tracks
//     which modes have been chosen via perm.Flags["pact_mode_X_chosen"]
//     for X in {damage, discard, draw, lose}. Picks the first un-chosen
//     mode in priority order: damage → discard → draw → lose.
//   - Mode resolution:
//     * damage:  deal 4 damage to first alive opponent, gain 4 life
//     * discard: force first alive opponent with cards in hand to discard 2
//     * draw:    controller draws 2
//     * lose:    Pact's controller is marked Lost (CR §104.2 game loss)
//   - State persists across turns via the per-permanent Flags map.
//     Flicker / re-cast cleanly resets the slate since a new Permanent
//     has fresh Flags.
func registerDemonicPact(r *Registry) {
	r.OnTrigger("Demonic Pact", "upkeep", demonicPactUpkeep)
}

func demonicPactUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "demonic_pact"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}

	mode := pickDemonicPactMode(perm)
	if mode == "" {
		// All modes chosen — but "lose" should be picked last and is
		// always available since picker order is damage→discard→draw→lose.
		// Defensive: if we somehow land here, no-op.
		return
	}
	perm.Flags["pact_mode_"+mode+"_chosen"] = 1

	switch mode {
	case "damage":
		// Deal 4 damage to first alive opponent, gain 4 life.
		targetSeat := -1
		for i, s := range gs.Seats {
			if s == nil || s.Lost || i == seat {
				continue
			}
			targetSeat = i
			break
		}
		if targetSeat >= 0 {
			gameengine.DealDamage(gs, targetSeat, 4, "Demonic Pact")
		}
		gameengine.GainLife(gs, seat, 4, "Demonic Pact")
	case "discard":
		for i, s := range gs.Seats {
			if s == nil || s.Lost || i == seat {
				continue
			}
			if len(s.Hand) == 0 {
				continue
			}
			gameengine.DiscardN(gs, i, 2, "Demonic Pact")
			break
		}
	case "draw":
		drawOne(gs, seat, "Demonic Pact")
		drawOne(gs, seat, "Demonic Pact")
	case "lose":
		// CR §104.3e: route through canonical helper so §614
		// would_lose_game (Platinum Angel / Angel's Grace) can cancel.
		gameengine.MarkSeatLostByEffect(gs, seat, "Demonic Pact")
		gs.LogEvent(gameengine.Event{
			Kind:   "seat_lost",
			Seat:   seat,
			Source: "Demonic Pact",
			Details: map[string]interface{}{
				"reason": "demonic_pact_final_mode",
				"rule":   "104.2",
			},
		})
		_ = gs.CheckEnd()
	}

	emit(gs, slug, "Demonic Pact", map[string]interface{}{
		"seat": seat,
		"mode": mode,
	})
}

// pickDemonicPactMode returns the first un-chosen mode in priority
// order: damage → discard → draw → lose.
func pickDemonicPactMode(perm *gameengine.Permanent) string {
	if perm.Flags["pact_mode_damage_chosen"] == 0 {
		return "damage"
	}
	if perm.Flags["pact_mode_discard_chosen"] == 0 {
		return "discard"
	}
	if perm.Flags["pact_mode_draw_chosen"] == 0 {
		return "draw"
	}
	if perm.Flags["pact_mode_lose_chosen"] == 0 {
		return "lose"
	}
	return ""
}
