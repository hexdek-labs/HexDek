package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// torgaar_famine_incarnate_r60.go — per_card handler for Torgaar,
// Famine Incarnate.
//
// Oracle text (Scryfall / ast_dataset):
//
//	As an additional cost to cast this spell, you may sacrifice any
//	number of creatures. This spell costs {2} less to cast for each
//	creature sacrificed this way.
//	When Torgaar, Famine Incarnate enters, up to one target player's
//	life total becomes half their starting life total, rounded down.
//
// {6}{B}{B} Legendary Creature. The ETB halves a player's life to half
// their STARTING total (20 in a 40-life Commander game) — a massive
// single-target life-swing / aggro reset. The "becomes half starting
// life" clause parses to a `parsed_effect_residual` raw-text node (no
// structured set-life node), so the generic dispatch logged it inert
// and Torgaar's whole payoff did nothing. (The additional-cost sacrifice
// is a separate casting-cost node handled by the cost pipeline; this
// handler is only the ETB body.)
//
// Implementation:
//   - OnETB. "up to one target player" — hat policy targets the
//     opponent with the HIGHEST current life (maximum reduction; never
//     self-target, since lowering your own life is strictly bad with no
//     payoff on this card). If every opponent's current life is already
//     <= half their starting total, the effect is a no-op gain and we
//     skip (the "up to one" wording lets us choose no target).
//   - Sets that seat's Life to floor(StartingLife/2) and fires the
//     life_change trigger + state-based actions, mirroring
//     resolveSetLife so lifegain/loss payoffs (Exquisite Blood, etc.)
//     and the 0-life SBA observe the swing.
func init() {
	registerTorgaarFamineIncarnateR60(Global())
	AddResetHook(registerTorgaarFamineIncarnateR60)
}

func registerTorgaarFamineIncarnateR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Torgaar, Famine Incarnate", torgaarFamineIncarnateETB)
}

func torgaarFamineIncarnateETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "torgaar_famine_incarnate"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Target the highest-life opponent for whom the effect is an actual
	// reduction.
	target := -1
	bestLife := -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		half := os.StartingLife / 2
		if os.Life <= half {
			continue // "up to one" — no benefit, decline to target this one
		}
		if os.Life > bestLife {
			bestLife = os.Life
			target = opp
		}
	}
	if target < 0 {
		emitFail(gs, slug, "Torgaar, Famine Incarnate", "no_beneficial_target", nil)
		return
	}

	ts := gs.Seats[target]
	newLife := ts.StartingLife / 2
	prev := ts.Life
	ts.Life = newLife

	gs.LogEvent(gameengine.Event{
		Kind:   "set_life",
		Seat:   seat,
		Target: target,
		Source: "Torgaar, Famine Incarnate",
		Amount: newLife,
	})
	gs.LogEvent(gameengine.Event{
		Kind:   "life_change",
		Seat:   target,
		Source: "Torgaar, Famine Incarnate",
		Amount: newLife - prev,
		Details: map[string]interface{}{
			"from": prev,
			"to":   newLife,
		},
	})
	gameengine.FireCardTrigger(gs, "life_change", map[string]interface{}{
		"seat":   target,
		"amount": newLife - prev,
		"source": "Torgaar, Famine Incarnate",
	})
	gameengine.StateBasedActions(gs)

	emit(gs, slug, "Torgaar, Famine Incarnate", map[string]interface{}{
		"seat":        seat,
		"target_seat": target,
		"from":        prev,
		"to":          newLife,
	})
}
