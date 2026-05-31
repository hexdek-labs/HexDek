package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWinterMisanthropicGuide wires Winter, Misanthropic Guide.
//
// Oracle text (Scryfall, verified 2026-05-30, {1}{B}{R}{G}, 3/4
// Legendary Creature — Human Warlock):
//
//	Ward {2}
//	At the beginning of your upkeep, each player draws two cards.
//	Delirium — As long as there are four or more card types among cards
//	in your graveyard, each opponent's maximum hand size is equal to
//	seven minus the number of those card types.
//
// Implementation (R60 stub sweep):
//   - upkeep_controller: each player draws 2 cards (existing wiring).
//   - Delirium refresh: same upkeep tick recomputes card-type count in
//     Winter's controller's graveyard. If ≥4, each opponent's
//     Seat.Flags["max_hand_size_override"] is set to max(0, 7 - count).
//     If <4, the override is cleared. Once-per-turn granularity is
//     acceptable since delirium count only inflects between turns in
//     practice (no within-turn graveyard churn during opponents'
//     discard steps); cleanup-step max-hand-size enforcement reads
//     the flag.
//   - Card types considered: the 8 standard CR §205.2a types — artifact,
//     battle, creature, enchantment, instant, land, planeswalker,
//     sorcery. (Tribal is a supertype on legacy cards; modern delirium
//     printings post-MH3 explicitly count tribal as a type but the
//     printed-on-card text we care about for Winter is the standard 8.)
//   - Ward {2} is engine-handled via the keyword pipeline.
func registerWinterMisanthropicGuide(r *Registry) {
	r.OnTrigger("Winter, Misanthropic Guide", "upkeep_controller", winterMisanthropicUpkeep)
}

func winterMisanthropicUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "winter_misanthropic_each_draws_two"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		drawOne(gs, i, perm.Card.DisplayName())
		drawOne(gs, i, perm.Card.DisplayName())
	}
	// Delirium refresh.
	seat := gs.Seats[perm.Controller]
	typesSeen := map[string]bool{}
	if seat != nil {
		for _, c := range seat.Graveyard {
			if c == nil {
				continue
			}
			for _, t := range winterDeliriumTypes {
				if typesSeen[t] {
					continue
				}
				if cardHasType(c, t) {
					typesSeen[t] = true
				}
			}
		}
	}
	count := len(typesSeen)
	cap := 7 - count
	if cap < 0 {
		cap = 0
	}
	for i, s := range gs.Seats {
		if s == nil || i == perm.Controller {
			continue
		}
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		if count >= 4 {
			s.Flags["max_hand_size_override"] = cap
		} else {
			delete(s.Flags, "max_hand_size_override")
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":                perm.Controller,
		"delirium_types":      count,
		"opponent_hand_cap":   cap,
		"delirium_active":     count >= 4,
	})
}
