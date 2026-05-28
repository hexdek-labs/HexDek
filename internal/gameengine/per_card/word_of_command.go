package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWordOfCommand wires Word of Command.
//
// Oracle text (verified via data/rules/ast_dataset.jsonl, 2026-05-28):
//
//	{B}{B} Instant
//	Look at target opponent's hand and choose a card from it. You
//	control that player until Word of Command finishes resolving. The
//	player plays that card if able. While doing so, the player can
//	activate mana abilities only if they're from lands that player
//	controls and only if mana they produce is spent to activate other
//	mana abilities of lands the player controls and/or to play that
//	card. If the chosen card is cast as a spell, you control the player
//	while that spell is resolving.
//
// One of the most CR-deep cards ever printed — Mindslaver in 1-card
// form, except the temporary-AI-takeover lives entirely inside one
// spell's resolution window. The "play if able" + "control the
// player" + scoped mana-ability restriction stack is the
// canonical "hard for an MVP" surface.
//
// Implementation:
//   - OnResolve. Pick target opponent (largest hand = heuristic for
//     "most threatening / most cards to choose from"). Reveal their
//     hand by logging the contents (Heimdall / spectator UI consumes
//     the per_card_handler event).
//   - Pick a card to force-play. Heuristic — the WORST card by
//     EV-to-the-opp: highest-CMC card if it can't be paid (forced
//     mana drain), otherwise a tutor with no good target, otherwise
//     the most expensive card in hand (most likely to brick).
//   - emitPartial for the actual force-play mechanic — the engine
//     doesn't have a "force opponent's seat to play card X on stack
//     under my AI's control" routing path, and the §605.3 mana-ability
//     scope is a separate layered restriction that lives in the cast-
//     legality pipeline. Mindslaver's "you control target player for
//     their next turn" is similarly marked as a partial wire-up in
//     mindslaver.go's ControlledBy delayed trigger; Word of Command is
//     the inverse-scope variant (control bounded by a single spell's
//     resolution, not a full turn).
//   - The hand-reveal half DOES resolve fully — that's the half the
//     spell has worked correctly without an AI takeover since printing.
//     Reveal alone has real game-state value (gives the caster
//     perfect information for the rest of the turn).
func registerWordOfCommand(r *Registry) {
	r.OnResolve("Word of Command", wordOfCommandResolve)
}

func wordOfCommandResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "word_of_command_reveal_and_force"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Pick target opponent — largest hand (matches Praetor's Grasp).
	target := -1
	bestHand := -1
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		if len(s.Hand) > bestHand {
			bestHand = len(s.Hand)
			target = i
		}
	}
	if target < 0 {
		emitFail(gs, slug, "Word of Command", "no_living_opponent", nil)
		return
	}
	oppHand := gs.Seats[target].Hand
	if len(oppHand) == 0 {
		emitFail(gs, slug, "Word of Command", "opponent_hand_empty", map[string]interface{}{
			"target_seat": target,
		})
		return
	}

	// Reveal the hand into the event log. Spectator / Heimdall consume
	// this as a hidden-information disclosure event.
	revealedNames := make([]string, 0, len(oppHand))
	for _, c := range oppHand {
		if c != nil {
			revealedNames = append(revealedNames, c.DisplayName())
		}
	}

	// Pick the WORST card from the controller's perspective — the one
	// most likely to brick the opponent's turn. Heuristic:
	// (1) skip lands (forcing a land play just helps the opponent ramp)
	// (2) pick the highest-CMC card (most likely uncastable, most mana
	//     drain if it IS castable, biggest tempo cost regardless)
	// Falls back to first non-land or first hand-slot if all lands.
	pickIdx := -1
	bestCMC := -1
	for i, c := range oppHand {
		if c == nil {
			continue
		}
		if wordOfCommandIsLand(c) {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			pickIdx = i
		}
	}
	if pickIdx < 0 {
		// All lands — fall back to first card (forcing a land play is
		// a minor net-negative for the opponent on tempo because they
		// could have held it for a more impactful turn).
		pickIdx = 0
	}
	picked := oppHand[pickIdx]
	pickedName := ""
	if picked != nil {
		pickedName = picked.DisplayName()
	}

	emit(gs, slug, "Word of Command", map[string]interface{}{
		"seat":          seat,
		"target_seat":   target,
		"revealed_hand": revealedNames,
		"forced_card":   pickedName,
	})

	emitPartial(gs, slug, "Word of Command",
		"force_play_via_opponent_ai_takeover_unmodeled_picked_card_logged_only")
}

func wordOfCommandIsLand(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "land" {
			return true
		}
	}
	return false
}
