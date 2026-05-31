package analytics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// CycleObservation records a detected resolution cycle — either an
// engine-detected mandatory loop (the §727 no_op_loop / cr_727
// detector breaking out of an infinite resolution chain) or a
// post-game graph-walker hit (a small cycle of cards in the
// causal-link graph that resolved finitely but represents an
// infinite-combo SHAPE — e.g. A produces mana, B consumes mana to
// activate, C untaps A, A produces mana again).
//
// Both sources are equally interesting to Huginn: engine cycles ARE
// infinite combos by construction (the engine literally couldn't
// progress); graph cycles are infinite combo CANDIDATES that the
// engine resolved finitely because the loop didn't have enough
// fingerprint stability to trip the §727 detector, but the shape is
// still a combo if the players choose to repeat it.
//
// The participating_iids list is the InstanceID of each distinct
// card driving one iteration of the cycle, in the order they
// appeared. Length should equal the CycleLength field; differences
// indicate some participants share an InstanceID (re-cast spell,
// flicker-and-return, etc.).
type CycleObservation struct {
	CycleLength        int      `json:"cycle_length"`        // number of distinct steps in one full cycle
	ParticipatingIIDs  []string `json:"participating_iids"`  // ordered, deduped; may be shorter than CycleLength if multi-step same-card cycles
	ParticipatingCards []string `json:"participating_cards"` // display names, aligned with ParticipatingIIDs
	TurnWindow         int      `json:"turn_window"`         // turn the cycle was detected
	DetectedBy         string   `json:"detected_by"`         // "engine_no_op_loop" / "engine_cr_727" / "graph_walker"
	GameID             string   `json:"game_id"`
}

// DetectEngineCycles scans the event log for "infinite_cycle" events
// emitted by loop_shortcut.go's projectAndApply. These are 1:1 with
// real engine loop-breaks — each event becomes one CycleObservation.
//
// The Details map carries:
//
//	cycle_length         int     — loop period (number of stack-resolution steps per cycle)
//	participating_iids   []string — ordered InstanceIDs of distinct cards driving the cycle
//	participating_names  []string — display names aligned with the IIDs
//	detected_by          string  — "engine_no_op_loop" or "engine_cr_727"
//
// Missing fields default to zero/empty — emit the observation
// regardless so Huginn sees the cycle even on legacy event-log shapes.
func DetectEngineCycles(events []gameengine.Event, gameIdx int) []CycleObservation {
	if len(events) == 0 {
		return nil
	}
	gameID := fmt.Sprintf("game-%d", gameIdx)
	var out []CycleObservation
	turn := 1
	for _, ev := range events {
		// Engine emits "turn_started" / "turn_begin" / etc. — we read
		// any Kind whose Details carry a "turn" int for window
		// attribution. Falling back to incremental "turn" event names
		// also works.
		if t, ok := ev.Details["turn"].(int); ok {
			turn = t
		}
		if ev.Kind != "infinite_cycle" {
			continue
		}
		obs := CycleObservation{
			TurnWindow: turn,
			GameID:     gameID,
		}
		if v, ok := ev.Details["cycle_length"].(int); ok {
			obs.CycleLength = v
		} else if ev.Amount > 0 {
			obs.CycleLength = ev.Amount
		}
		if v, ok := ev.Details["participating_iids"].([]string); ok {
			obs.ParticipatingIIDs = append([]string(nil), v...)
		}
		if v, ok := ev.Details["participating_names"].([]string); ok {
			obs.ParticipatingCards = append([]string(nil), v...)
		}
		if v, ok := ev.Details["detected_by"].(string); ok {
			obs.DetectedBy = v
		} else {
			obs.DetectedBy = "engine_unknown"
		}
		out = append(out, obs)
	}
	return out
}

// DetectGraphCycles walks the post-game cooccurrence graph (derived
// from DetectCoTriggers output) looking for small cycles — 2-card and
// 3-card cycles only, to keep DFS bounded and cycle interpretability
// tight. Longer cycles get absorbed into shorter ones (an A→B→C→D→A
// 4-cycle contains an A→B→C→A 3-cycle on a subset of the same edges
// if the graph is dense enough that we'd find them by enumeration; if
// not, the larger cycle is meaningful and tracked separately in a
// future PR).
//
// A 2-cycle is mutual: A→B AND B→A both fire as causal links in the
// same turn window (an A enables B which enables A, the textbook
// 2-card combo shape — Heliod + Walking Ballista, Kiki + Felidar).
// A 3-cycle is A→B→C→A in some ordering. CoTriggerObservation
// records the directionality via EffectPattern ("A produces mana, B
// consumes mana") which we treat as directed A→B for graph purposes.
//
// The result is in deterministic order: cycle length asc, then
// lexicographic by sorted-participants.
//
// Engine-detected cycles take precedence — if a 2-cycle's IIDs are
// already in any engine CycleObservation, the graph cycle is omitted
// (would be a duplicate signal). Caller passes engineCycles for this
// dedup.
func DetectGraphCycles(events []gameengine.Event, nSeats, gameIdx int, engineCycles []CycleObservation) []CycleObservation {
	if len(events) == 0 || nSeats <= 0 {
		return nil
	}
	cots := DetectCoTriggers(events, nSeats, gameIdx)
	if len(cots) == 0 {
		return nil
	}
	gameID := fmt.Sprintf("game-%d", gameIdx)

	// Build a directed graph keyed by card name (CoTriggerObservation
	// doesn't carry InstanceID — we look those up from the event log
	// later for the ParticipatingIIDs field).
	type edge struct{ to, pattern string; turn int }
	graph := map[string][]edge{}
	for _, c := range cots {
		from, to := directionFromPattern(c)
		graph[from] = append(graph[from], edge{to: to, pattern: c.EffectPattern, turn: c.TurnWindow})
	}

	// Dedup set of engine-detected IID groups so we don't re-report.
	engineIIDSets := map[string]bool{}
	for _, ec := range engineCycles {
		engineIIDSets[sortedKey(ec.ParticipatingIIDs)] = true
	}

	// Build (name → first-seen InstanceID) from the event log. We
	// scan once and remember the first IID associated with each card
	// name. Misses (legacy/unminted cards) get empty strings; the
	// observation still records the cycle by NAME so Huginn can match
	// it post-hoc.
	nameToIID := buildNameToIIDIndex(events)

	var cycles []CycleObservation

	// 2-cycles (mutual edges).
	seen2 := map[string]bool{}
	for from, edges := range graph {
		for _, e := range edges {
			if from == e.to {
				continue // self-loop — not a 2-cycle for our purposes
			}
			// Check reverse edge exists.
			for _, back := range graph[e.to] {
				if back.to != from {
					continue
				}
				key := pairKey(from, e.to)
				if seen2[key] {
					continue
				}
				seen2[key] = true
				iids := lookupIIDs(nameToIID, []string{from, e.to})
				if engineIIDSets[sortedKey(iids)] {
					continue // already reported by engine
				}
				cycles = append(cycles, CycleObservation{
					CycleLength:        2,
					ParticipatingIIDs:  iids,
					ParticipatingCards: []string{from, e.to},
					TurnWindow:         e.turn,
					DetectedBy:         "graph_walker",
					GameID:             gameID,
				})
			}
		}
	}

	// 3-cycles. Walk A→B→C, check C→A back-edge.
	seen3 := map[string]bool{}
	for a, edgesA := range graph {
		for _, eA := range edgesA {
			b := eA.to
			if b == a {
				continue
			}
			for _, eB := range graph[b] {
				c := eB.to
				if c == a || c == b {
					continue
				}
				for _, eC := range graph[c] {
					if eC.to != a {
						continue
					}
					key := tripleKey(a, b, c)
					if seen3[key] {
						continue
					}
					seen3[key] = true
					names := []string{a, b, c}
					iids := lookupIIDs(nameToIID, names)
					if engineIIDSets[sortedKey(iids)] {
						continue
					}
					cycles = append(cycles, CycleObservation{
						CycleLength:        3,
						ParticipatingIIDs:  iids,
						ParticipatingCards: names,
						TurnWindow:         eC.turn,
						DetectedBy:         "graph_walker",
						GameID:             gameID,
					})
				}
			}
		}
	}

	// Deterministic order: length asc, then sorted-participants
	// lexicographic.
	sort.Slice(cycles, func(i, j int) bool {
		if cycles[i].CycleLength != cycles[j].CycleLength {
			return cycles[i].CycleLength < cycles[j].CycleLength
		}
		return sortedKey(cycles[i].ParticipatingCards) < sortedKey(cycles[j].ParticipatingCards)
	})
	return cycles
}

// directionFromPattern picks A→B vs B→A based on whether the
// EffectPattern string puts CardA or CardB on the "produces" side.
// Defaults to A→B when the pattern doesn't make the direction
// explicit — the graph walker tolerates this because a 2-cycle still
// works as long as both edges exist.
func directionFromPattern(c CoTriggerObservation) (from, to string) {
	pat := strings.ToLower(c.EffectPattern)
	aIdx := strings.Index(pat, strings.ToLower(c.CardA))
	bIdx := strings.Index(pat, strings.ToLower(c.CardB))
	// Whichever name appears FIRST in the pattern is the producer
	// ("A produces mana, B consumes mana" → A is from).
	if aIdx >= 0 && (bIdx < 0 || aIdx < bIdx) {
		return c.CardA, c.CardB
	}
	if bIdx >= 0 && (aIdx < 0 || bIdx < aIdx) {
		return c.CardB, c.CardA
	}
	return c.CardA, c.CardB
}

// buildNameToIIDIndex scans events for cards that log an instance_id
// in Details. First occurrence wins (matches the CardIndex semantics
// used by the forensics CLI). Cards without an IID in the event log
// get an empty string in the lookup; the cycle observation still
// records by name so Huginn matches downstream.
func buildNameToIIDIndex(events []gameengine.Event) map[string]string {
	out := map[string]string{}
	for _, ev := range events {
		if ev.Source == "" {
			continue
		}
		if _, seen := out[ev.Source]; seen {
			continue
		}
		if ev.Details == nil {
			continue
		}
		iid, ok := ev.Details["instance_id"].(string)
		if !ok || iid == "" {
			continue
		}
		out[ev.Source] = iid
	}
	return out
}

func lookupIIDs(index map[string]string, names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = index[n] // empty string if absent — preserved intentionally
	}
	return out
}

func pairKey(a, b string) string {
	if a < b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

func tripleKey(a, b, c string) string {
	xs := []string{a, b, c}
	sort.Strings(xs)
	return strings.Join(xs, "\x00")
}

func sortedKey(xs []string) string {
	cp := append([]string(nil), xs...)
	sort.Strings(cp)
	return strings.Join(cp, "\x00")
}
