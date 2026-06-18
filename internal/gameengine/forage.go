package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// forage.go — CR §701.61 (Forage, Bloomburrow).
//
// "To forage means to exile three cards from your graveyard or sacrifice
//  a Food." Foraging is an action/cost with a player CHOICE between two
//  payment modes; a player can only forage if at least one mode is
//  payable. Whenever a player forages, a "forage" event fires so
//  "whenever you forage" payoffs (Camellia, the Seedmiser; Corpseberry
//  Cultivator; ...) observe it.
//
// Before this primitive existed, every forage surface was inert: the
// generic AST `Modification{kind:"forage"}` and the `paid_optional_cost`
// conditional ("you may forage. If you do, X") logged a no-op, so the
// gated effect (Treetop Sentries' draw, Bushy Bodyguard's +1/+1 counters,
// Camellia's anthem) never happened and no cards left the graveyard.

// CanForage reports whether `seat` is able to pay the forage cost — it has
// at least three cards in its graveyard OR controls a Food permanent to
// sacrifice. This is the affordability gate used both by cost payers (an
// activation whose cost includes forage rejects when CanForage is false)
// and by optional "you may forage" gates.
func CanForage(gs *GameState, seat int) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return false
	}
	if len(gs.Seats[seat].Graveyard) >= 3 {
		return true
	}
	return findForageFood(gs, seat) != nil
}

// findForageFood returns a Food permanent controlled by `seat`, or nil.
func findForageFood(gs *GameState, seat int) *Permanent {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return nil
	}
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && cardHasSubtype(p.Card, "food") {
			return p
		}
	}
	return nil
}

// Forage pays the forage cost for `seat` and fires the forage event,
// returning true on success. It pays nothing and returns false when the
// seat cannot forage (CanForage == false) — callers gate their effect on
// the return value (the "if you do" branch).
//
// Payment-mode choice (the player's per CR §701.61): prefer sacrificing a
// Food when one is available — that keeps graveyard resources intact and
// feeds the Food/sacrifice payoffs that forage decks are built around
// (Camellia counts sacrificed Foods); fall back to exiling three cards
// from the graveyard. The exile path routes each card through MoveCard so
// §614 replacements + zone-change triggers fire and zone conservation is
// preserved (the three cards move graveyard → exile, they are not lost).
// Refining this into a Hat-driven choice is a follow-up; the mode does not
// affect which gated effect resolves.
func Forage(gs *GameState, seat int, sourceName string) bool {
	if !CanForage(gs, seat) {
		return false
	}
	s := gs.Seats[seat]
	mode := ""
	if food := findForageFood(gs, seat); food != nil {
		SacrificePermanent(gs, food, "forage")
		mode = "sacrifice_food"
	} else {
		// Exile three cards from the graveyard, oldest first. Drain the
		// source slice up front so MoveCard's internal removal is a no-op
		// (mirrors exileCardsForCost's escape/delve idiom in costs.go).
		gy := s.Graveyard
		exiled := make([]*Card, 0, 3)
		remaining := make([]*Card, 0, len(gy))
		for _, c := range gy {
			if c == nil {
				continue
			}
			if len(exiled) < 3 {
				exiled = append(exiled, c)
			} else {
				remaining = append(remaining, c)
			}
		}
		s.Graveyard = remaining
		for _, c := range exiled {
			MoveCard(gs, c, seat, "graveyard", "exile", "forage")
		}
		mode = "exile_three"
	}

	gs.LogEvent(Event{
		Kind:   "forage",
		Seat:   seat,
		Source: sourceName,
		Details: map[string]interface{}{
			"mode": mode,
			"rule": "701.61",
		},
	})
	// "Whenever you forage" payoffs (Camellia, the Seedmiser; Corpseberry
	// Cultivator) observe the event.
	FireCardTrigger(gs, "forage", map[string]interface{}{
		"seat": seat,
	})
	return true
}

// costHasForage reports whether an activated ability's cost includes the
// forage action. Thor encodes "{cost}, Forage: ..." with the literal
// "forage" appended to Cost.Extra (alongside structured mana/tap costs).
func costHasForage(c gameast.Cost) bool {
	for _, ex := range c.Extra {
		if strings.EqualFold(strings.TrimSpace(ex), "forage") {
			return true
		}
	}
	return false
}
