package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// oneshot_flashback_grants.go — one-shot sorcery / instant flashback grants
// that hand graveyard cards flashback until end of turn at resolution time.
//
// Unlike the continuous Iroh / Lier grants (handled via the same
// GraveyardFlashbackGrant struct with a battlefield-source timestamp), the
// cards in this file have no battlefield permanent: they're sorceries and
// instants that resolve and emit a transient grant that expires at the
// cleanup step. The primitive used is
// gameengine.RegisterEOTGraveyardFlashbackGrant for mass grants and
// gameengine.GrantFlashbackUntilEOT for single-target grants.
//
// Cards covered:
//   - Past in Flames (sorcery, mass grant to controller's graveyard)
//   - Will of the Jeskai (sorcery, modal — flashback mode = mass grant)
//   - Flashback (instant, single-target i/s grant; reuses Snapcaster primitive)
//   - Recoup (sorcery, single-target sorcery-only grant)

func init() {
	registerOneShotFlashbackGrants(Global())
	AddResetHook(registerOneShotFlashbackGrants)
}

func registerOneShotFlashbackGrants(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Past in Flames", pastInFlamesResolve)
	r.OnResolve("Will of the Jeskai", willOfTheJeskaiResolve)
	r.OnResolve("Flashback", flashbackInstantResolve)
	r.OnResolve("Recoup", recoupResolve)
}

// pastInFlamesResolve registers an EOT mass-flashback grant for the
// controller's graveyard. Past in Flames itself is exiled-on-resolve
// when cast via its own flashback (CR §702.34c is handled by the
// CastFlashback stack item); when hard-cast it goes to graveyard
// normally and is then eligible for its own grant in any subsequent
// turn (but not this turn — at the moment the grant is registered,
// Past in Flames is still on the stack, not in the graveyard).
//
// Oracle:
//
//	Each instant and sorcery card in your graveyard gains flashback
//	until end of turn. The flashback cost is equal to its mana cost.
//	Flashback {4}{R}
func pastInFlamesResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "past_in_flames"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.RegisterEOTGraveyardFlashbackGrant(gs, seat, "Past in Flames", gameengine.PrintedMassFlashbackCost)
	emit(gs, slug, "Past in Flames", map[string]interface{}{
		"seat":          seat,
		"graveyard_len": len(gs.Seats[seat].Graveyard),
	})
}

// willOfTheJeskaiResolve resolves the two-mode "Will" cycle entry. Mode 1
// is a Wheel-of-Fortune-style discard-and-redraw-5; mode 2 is the
// Past-in-Flames mass-flashback grant. If the controller controls a
// commander at cast time, both modes resolve.
//
// Oracle:
//
//	Choose one. If you control a commander as you cast this spell, you
//	may choose both instead.
//	• Each player may discard their hand and draw five cards.
//	• Each instant and sorcery card in your graveyard gains flashback
//	  until end of turn. The flashback cost is equal to its mana cost.
//
// Mode selection policy (until full modal-choice plumbing lands): the
// flashback mode is the value engine the card is named for, so we
// default to it. Mode 1 also runs when the controller has a commander
// (the "may choose both" clause). For mode 1 we treat the "may discard"
// per player as opt-in — opponents skip if their hand is small (<= 1),
// the controller takes it whenever it's a net gain.
func willOfTheJeskaiResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "will_of_the_jeskai"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	hasCommander := willOfTheJeskaiControllerHasCommander(gs, seat)

	// Mode 2: mass flashback grant (always chosen).
	gameengine.RegisterEOTGraveyardFlashbackGrant(gs, seat, "Will of the Jeskai", gameengine.PrintedMassFlashbackCost)

	// Mode 1: discard hand, draw 5 — only if we chose both.
	if hasCommander {
		willOfTheJeskaiDiscardDrawFive(gs, seat)
	}

	emit(gs, slug, "Will of the Jeskai", map[string]interface{}{
		"seat":           seat,
		"both_modes":     hasCommander,
		"graveyard_len":  len(gs.Seats[seat].Graveyard),
	})
}

// willOfTheJeskaiControllerHasCommander checks whether `seat` controls
// at least one of its commander permanents on the battlefield. This is
// the same shape used by Jeska's Will (game_changers.go) — kept local
// to avoid a cross-file refactor.
func willOfTheJeskaiControllerHasCommander(gs *gameengine.GameState, seat int) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
	if s == nil {
		return false
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, cn := range s.CommanderNames {
			if strings.EqualFold(p.Card.DisplayName(), cn) {
				return true
			}
		}
	}
	return false
}

// willOfTheJeskaiDiscardDrawFive runs the "Each player may discard their
// hand and draw five cards" mode. Controller always takes it; opponents
// take it only when they'd net positive (current hand size < 5 or empty).
// "May" is per-player, so a player with a curated 5-card hand may decline.
func willOfTheJeskaiDiscardDrawFive(gs *gameengine.GameState, controller int) {
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		// Opponents only opt in if it's strictly an upgrade; the
		// controller always takes it (the entire reason to cast Will).
		if i != controller && len(s.Hand) >= 5 {
			continue
		}
		for len(s.Hand) > 0 {
			gameengine.DiscardCard(gs, s.Hand[0], i)
		}
		for j := 0; j < 5 && len(s.Library) > 0; j++ {
			card := s.Library[0]
			gameengine.MoveCard(gs, card, i, "library", "hand", "draw")
		}
	}
}

// flashbackInstantResolve grants flashback to a single targeted instant
// or sorcery in the controller's graveyard until end of turn at its
// printed mana cost. Reuses GrantFlashbackUntilEOT (the Snapcaster
// primitive) since the per-card grant shape is identical.
//
// Oracle:
//
//	Target instant or sorcery card in your graveyard gains flashback
//	until end of turn. The flashback cost is equal to its mana cost.
//
// Target selection policy (until full target-prompt plumbing lands):
// pick the highest-CMC eligible card. Falls back gracefully if the
// graveyard has no instants/sorceries.
func flashbackInstantResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "flashback_instant"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	target := pickHighestCMCInstantOrSorcery(gs.Seats[seat].Graveyard, item.Card)
	if target == nil {
		emitFail(gs, slug, "Flashback", "no_eligible_target_in_graveyard", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	gameengine.GrantFlashbackUntilEOT(gs, target, seat, "Flashback")
	emit(gs, slug, "Flashback", map[string]interface{}{
		"seat":   seat,
		"target": target.DisplayName(),
	})
}

// pickHighestCMCInstantOrSorcery returns the eligible card with the
// largest CMC in `cards`, or nil if none qualify. Skips `self` so the
// resolving Flashback spell itself (if a hypothetical pointer pun put
// it in the graveyard already) is never picked.
func pickHighestCMCInstantOrSorcery(cards []*gameengine.Card, self *gameengine.Card) *gameengine.Card {
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range cards {
		if c == nil || c == self {
			continue
		}
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	return best
}

// recoupResolve grants flashback to a single targeted sorcery (no
// instants) in the controller's graveyard until end of turn at its
// printed mana cost. Same shape as Flashback (the instant) but with a
// sorcery-only filter; GrantFlashbackUntilEOT enforces the i/s gate so
// we additionally guard sorcery-only here at target-pick time.
//
// Oracle:
//
//	Target sorcery card in your graveyard gains flashback until end of
//	turn. The flashback cost is equal to its mana cost.
//	Flashback {3}{R}
func recoupResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "recoup"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	target := pickHighestCMCSorcery(gs.Seats[seat].Graveyard, item.Card)
	if target == nil {
		emitFail(gs, slug, "Recoup", "no_sorcery_target_in_graveyard", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	gameengine.GrantFlashbackUntilEOT(gs, target, seat, "Recoup")
	emit(gs, slug, "Recoup", map[string]interface{}{
		"seat":   seat,
		"target": target.DisplayName(),
	})
}

func pickHighestCMCSorcery(cards []*gameengine.Card, self *gameengine.Card) *gameengine.Card {
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range cards {
		if c == nil || c == self {
			continue
		}
		if !cardHasType(c, "sorcery") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	return best
}
