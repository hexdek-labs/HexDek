package analytics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// CoTriggerObservation records a single co-trigger event between two cards
// that occurred in the same turn window with a verified causal link: one
// card produced a resource that the other consumed.
type CoTriggerObservation struct {
	CardA         string  `json:"card_a"`
	CardB         string  `json:"card_b"`
	ImpactScore   float64 `json:"impact_score"`   // life_delta + board_delta + mana_delta + cards_drawn
	TurnWindow    int     `json:"turn_window"`     // turn when co-trigger occurred
	EffectPattern string  `json:"effect_pattern"`  // e.g. "A produces mana, B consumes mana"
	GameID        string  `json:"game_id"`         // game index for tracing
}

// turnSnapshot tracks per-seat resource deltas and card events within a
// single turn, used by DetectCoTriggers to calculate impact and find
// causal links.
type turnSnapshot struct {
	// cardEvents maps card name -> list of (produces, consumes) pairs.
	cardEvents map[string][]cardResourceEvent

	// Per-seat deltas for impact calculation.
	lifeDelta  int
	boardDelta int
	manaSpent  int
	cardsDrawn int
}

// cardResourceEvent records what resources a single event from a card
// produced or consumed.
type cardResourceEvent struct {
	produces []string
	consumes []string
}

// DetectCoTriggers walks the event log and finds pairs of cards on the
// same seat that both fired events in the same turn AND have a causal
// resource link (A produces X, B consumes X or vice versa).
//
// The causal link filter is critical -- without it the output would be
// flooded with thousands of coincidental pairs.
func DetectCoTriggers(events []gameengine.Event, nSeats int, gameIdx int) []CoTriggerObservation {
	if len(events) == 0 || nSeats <= 0 {
		return nil
	}

	gameID := fmt.Sprintf("game-%d", gameIdx)

	// We process turn by turn. Each turn collects per-seat card events.
	snapshots := make([]turnSnapshot, nSeats)
	currentTurn := 1

	resetSnapshots := func() {
		for i := range snapshots {
			snapshots[i] = turnSnapshot{
				cardEvents: make(map[string][]cardResourceEvent),
			}
		}
	}
	resetSnapshots()

	var observations []CoTriggerObservation

	// flushTurn examines the current turn's snapshots and emits
	// co-trigger observations for any causally linked card pairs.
	flushTurn := func(turn int) {
		for seat := 0; seat < nSeats; seat++ {
			snap := &snapshots[seat]
			if len(snap.cardEvents) < 2 {
				continue // need at least 2 distinct cards
			}

			// Build a list of card names that participated this turn.
			cardNames := make([]string, 0, len(snap.cardEvents))
			for name := range snap.cardEvents {
				cardNames = append(cardNames, name)
			}
			sort.Strings(cardNames) // deterministic ordering

			// Check all pairs for causal links.
			for i := 0; i < len(cardNames); i++ {
				for j := i + 1; j < len(cardNames); j++ {
					nameA := cardNames[i]
					nameB := cardNames[j]

					pattern := findCausalLink(snap.cardEvents[nameA], snap.cardEvents[nameB], nameA, nameB)
					if pattern == "" {
						continue // no causal link
					}

					// Calculate impact score for this seat this turn.
					impact := float64(abs(snap.lifeDelta)) +
						float64(abs(snap.boardDelta)) +
						float64(snap.manaSpent) +
						float64(snap.cardsDrawn)

					observations = append(observations, CoTriggerObservation{
						CardA:         nameA,
						CardB:         nameB,
						ImpactScore:   impact,
						TurnWindow:    turn,
						EffectPattern: pattern,
						GameID:        gameID,
					})
				}
			}
		}
	}

	// Walk the event log.
	for idx := range events {
		ev := &events[idx]

		// Detect turn boundaries.
		if ev.Kind == "turn_start" {
			if t, ok := detailInt(ev, "turn"); ok && t > currentTurn {
				flushTurn(currentTurn)
				currentTurn = t
				resetSnapshots()
			}
			continue
		}

		seat := ev.Seat
		if seat < 0 || seat >= nSeats {
			continue
		}
		snap := &snapshots[seat]

		// Track resource deltas for impact calculation.
		switch ev.Kind {
		case "life_change":
			snap.lifeDelta += ev.Amount
		case "enter_battlefield":
			snap.boardDelta++
		case "leave_battlefield":
			snap.boardDelta--
		case "pay_mana":
			snap.manaSpent += ev.Amount
		case "draw_card":
			snap.cardsDrawn++
		}

		// Record which card produced this event and its resource profile.
		cardName := ev.Source
		if cardName == "" {
			continue
		}

		// Only track event kinds that represent meaningful card actions.
		switch ev.Kind {
		case "triggered_ability", "cast", "create_token", "damage",
			"draw_card", "life_change", "pay_mana", "pool_drain",
			"enter_battlefield", "leave_battlefield", "play_land",
			"sacrifice", "destroy":
			// These are tracked.
		default:
			continue
		}

		produces, consumes := EventResources(ev)
		snap.cardEvents[cardName] = append(snap.cardEvents[cardName], cardResourceEvent{
			produces: produces,
			consumes: consumes,
		})
	}

	// Flush the final turn.
	flushTurn(currentTurn)

	return observations
}

// findCausalLink checks whether any event from card A produces a resource
// that any event from card B consumes (or vice versa). Returns a
// human-readable pattern string, or "" if no link is found.
func findCausalLink(eventsA, eventsB []cardResourceEvent, nameA, nameB string) string {
	// Collect all resources produced/consumed by each card.
	producedByA := collectResources(eventsA, true)
	consumedByA := collectResources(eventsA, false)
	producedByB := collectResources(eventsB, true)
	consumedByB := collectResources(eventsB, false)

	// Check A -> B: A produces something B consumes.
	for res := range producedByA {
		if consumedByB[res] {
			return fmt.Sprintf("%s produces %s, %s consumes %s", nameA, res, nameB, res)
		}
	}

	// Check B -> A: B produces something A consumes.
	for res := range producedByB {
		if consumedByA[res] {
			return fmt.Sprintf("%s produces %s, %s consumes %s", nameB, res, nameA, res)
		}
	}

	return ""
}

// collectResources builds a set of resource types from a list of card
// resource events. If produces is true, collects produced resources;
// otherwise collects consumed resources.
func collectResources(events []cardResourceEvent, produces bool) map[string]bool {
	out := make(map[string]bool)
	for _, e := range events {
		if produces {
			for _, r := range e.produces {
				out[r] = true
			}
		} else {
			for _, r := range e.consumes {
				out[r] = true
			}
		}
	}
	return out
}

// abs returns the absolute value of an int.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// --- Aggregation for multi-game analysis ---

// CoTriggerSummary aggregates co-trigger observations across multiple
// games for a single card pair.
type CoTriggerSummary struct {
	CardA        string
	CardB        string
	Occurrences  int
	TotalImpact  float64
	AvgImpact    float64
	TopPattern   string // most frequent effect pattern
	patternCount map[string]int
}

// AggregateCoTriggers groups observations by (CardA, CardB) pair,
// sums impact, counts occurrences, and returns sorted by total impact
// descending.
func AggregateCoTriggers(observations []CoTriggerObservation) []CoTriggerSummary {
	if len(observations) == 0 {
		return nil
	}

	byPair := make(map[string]*CoTriggerSummary)

	for _, obs := range observations {
		// Normalize pair order (alphabetical) so A+B and B+A merge.
		a, b := obs.CardA, obs.CardB
		if a > b {
			a, b = b, a
		}
		key := a + "\x00" + b

		s, ok := byPair[key]
		if !ok {
			s = &CoTriggerSummary{
				CardA:        a,
				CardB:        b,
				patternCount: make(map[string]int),
			}
			byPair[key] = s
		}
		s.Occurrences++
		s.TotalImpact += obs.ImpactScore

		// Normalize the pattern to use the canonical (alphabetical) card
		// names so that "X produces mana, Y consumes mana" and the reverse
		// ordering merge into the same pattern.
		pattern := obs.EffectPattern
		if obs.CardA > obs.CardB {
			// The observation used reversed names; swap back in pattern.
			pattern = strings.ReplaceAll(pattern, obs.CardA, "\x01")
			pattern = strings.ReplaceAll(pattern, obs.CardB, obs.CardA)
			pattern = strings.ReplaceAll(pattern, "\x01", obs.CardB)
		}
		s.patternCount[pattern]++
	}

	// Finalize averages and find top pattern.
	result := make([]CoTriggerSummary, 0, len(byPair))
	for _, s := range byPair {
		if s.Occurrences > 0 {
			s.AvgImpact = s.TotalImpact / float64(s.Occurrences)
		}
		// Find most frequent pattern.
		maxCount := 0
		for pat, cnt := range s.patternCount {
			if cnt > maxCount {
				maxCount = cnt
				s.TopPattern = pat
			}
		}
		s.patternCount = nil // don't leak internal state
		result = append(result, *s)
	}

	// Sort by total impact descending.
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].TotalImpact != result[j].TotalImpact {
			return result[i].TotalImpact > result[j].TotalImpact
		}
		return result[i].Occurrences > result[j].Occurrences
	})

	return result
}
