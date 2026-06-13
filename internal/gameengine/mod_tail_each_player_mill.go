package gameengine

import (
	"regexp"
	"strings"
)

// mod_tail_each_player_mill.go — generic handler for the parsed_tail effect
// shape "each player mills N cards" / "each opponent mills N cards".
//
// This is one of the few cleanly runtime-handleable structured `parsed_tail`
// clusters left after the modal + deal_damage_count sweep: a symmetric mill
// effect that resolves once (no static/staleness/condition modelling). Cards:
// Chill of Foreboding, Path of the Schemer, Flotsam // Jetsam, Unseal the
// Necropolis, Mind Sculpt-style tails, etc. — all routed through
// resolveResidualByText and previously inert.
//
// Conservative scoping:
//   - Matches ONLY the plain leading "each {player|opponent} mills N cards"
//     clause. Conditional variants ("each player WHO discarded a card mills …")
//     and variable counts ("mills X cards") are NOT matched (return false →
//     stay inert) rather than mis-resolve.
//   - "opponent" scope = the source controller's opponents; "player" scope =
//     all living seats.
//   - Trailing riders ("… , then you may cast a spell") are not part of this
//     clause and remain inert — the mill itself is now applied (strict
//     improvement over the prior do-nothing behavior).
var reEachPlayerMill = regexp.MustCompile(
	`(?i)^each (player|opponent) mills (one|two|three|four|five|six|seven|eight|nine|ten|\d+) cards?`)

// EachPlayerMillFromTail returns true when it recognizes and resolves an
// "each player/opponent mills N cards" parsed_tail clause.
func EachPlayerMillFromTail(gs *GameState, src *Permanent, raw string) bool {
	if gs == nil {
		return false
	}
	m := reEachPlayerMill.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return false
	}
	scope := strings.ToLower(m[1])
	n, ok := firstNumber(strings.ToLower(m[2]))
	if !ok || n <= 0 {
		return false
	}

	seat := controllerSeat(src)
	var targets []int
	if scope == "opponent" {
		targets = gs.Opponents(seat)
	} else {
		for i, s := range gs.Seats {
			if s != nil && !s.Lost {
				targets = append(targets, i)
			}
		}
	}

	milled := 0
	for _, t := range targets {
		for i := 0; i < n; i++ {
			if _, did := gs.millOne(t); did {
				milled++
			}
		}
	}
	gs.LogEvent(Event{
		Kind:   "mill",
		Seat:   seat,
		Source: sourceName(src),
		Amount: milled,
		Details: map[string]interface{}{
			"each":       scope,
			"per_player": n,
			"rule":       "parsed_tail:each_player_mill",
		},
	})
	return true
}
