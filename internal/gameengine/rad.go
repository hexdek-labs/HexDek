package gameengine

import "strings"

// Rad counter trigger — Fallout set mechanic (24 cards).
//
// CR (Radiation token): "At the beginning of your precombat main phase,
// if you have any rad counters, mill that many cards. For each nonland
// card milled this way, you lose 1 life and a rad counter."
// Rad counters are removed per NONLAND card milled, not per total milled.

// FireRadCounterTriggers processes rad counters for all players. Called
// at the beginning of the precombat main phase (wired from
// tournament/turn.go). Each player with rad counters > 0:
//   1. Mills rad_count cards (or remaining library, whichever is smaller)
//   2. Loses 1 life per nonland card milled
//   3. Removes rad counters equal to the number of cards milled
func FireRadCounterTriggers(gs *GameState) {
	if gs == nil {
		return
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		radCount := 0
		if s.Flags != nil {
			radCount = s.Flags["rad_counters"]
		}
		if radCount <= 0 {
			continue
		}

		// Mill radCount cards (or whatever remains in library).
		nonlandMilled := 0
		cardsMilled := 0
		for j := 0; j < radCount && len(s.Library) > 0; j++ {
			card := s.Library[0]
			MoveCard(gs, card, i, "library", "graveyard", "mill")
			cardsMilled++
			isLand := false
			if card != nil {
				for _, t := range card.Types {
					if strings.ToLower(t) == "land" {
						isLand = true
						break
					}
				}
			}
			if !isLand {
				nonlandMilled++
			}
		}

		// Lose life for each nonland card milled.
		if nonlandMilled > 0 {
			s.Life -= nonlandMilled
			gs.LogEvent(Event{
				Kind:   "rad_damage",
				Seat:   i,
				Amount: nonlandMilled,
				Details: map[string]interface{}{
					"cards_milled": cardsMilled,
					"nonland":      nonlandMilled,
				},
			})
		}

		// Remove rad counters equal to the number of NONLAND cards milled.
		// CR: "For each nonland card milled this way, you lose 1 life
		// and a rad counter." — one rad counter removed per nonland, NOT
		// per total cards milled.
		if s.Flags != nil {
			s.Flags["rad_counters"] -= nonlandMilled
			if s.Flags["rad_counters"] < 0 {
				s.Flags["rad_counters"] = 0
			}
		}

		gs.LogEvent(Event{
			Kind:   "rad_trigger",
			Seat:   i,
			Amount: radCount,
			Details: map[string]interface{}{
				"cards_milled":   cardsMilled,
				"nonland_milled": nonlandMilled,
				"rad_remaining":  s.Flags["rad_counters"],
			},
		})
	}
}
