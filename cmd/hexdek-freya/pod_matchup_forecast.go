package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// PodMatchupForecast is the predicted matchup outcome rollup for a
// 4-deck Commander pod. Given the per-seat archetypes, it consults
// the metaMatchupDB (PR #196 / #139) to compute each seat's net edge
// against the other three seats, then converts net edges to expected
// win probabilities via softmax.
//
// Use cases:
//   - Tournament UI: render "expected win probability per seat"
//     before the game starts so the table sees the predicted
//     asymmetry ("seat 0 has the Combo deck — 40% expected; seat 2's
//     Voltron is the underdog at 12%")
//   - Hat 3rd-eye intelligence: at game start the hat reads its own
//     seat's forecast to scale early aggression — if the hat is the
//     dominant seat, it knows the table will gang up early and can
//     play conservatively until the threat-density evens out
//   - Decks-screen: show the user "vs your typical pod, your win
//     rate is X%" by averaging across simulated pods
//
// Forecast methodology:
//   1. For each seat, look up its archetype's rating against each
//      of the 3 opponent archetypes in metaMatchupDB
//   2. Convert each (rating, strength) into a numeric edge:
//        favored × strong       = +0.30
//        favored × moderate     = +0.15
//        favored × slight       = +0.05
//        neutral / missing      =  0
//        unfavored × slight     = -0.05
//        unfavored × moderate   = -0.15
//        unfavored × strong     = -0.30
//   3. Sum the 3 per-opponent edges into a seat's base score
//   4. Softmax across all 4 seats to produce win probabilities
//      that sum to 1.0
//
// Asymmetry detection: when the dominant seat's predicted win
// probability exceeds 0.40 (vs the uniform 0.25 baseline), the
// Asymmetric flag is set — UIs can render "this pod has a clear
// favorite" prominently.
type PodMatchupForecast struct {
	// Seats is the per-seat forecast in input order. Pod position
	// is meaningful (seat 0 = first player after the random first-
	// player choice, etc.).
	Seats []PodSeatForecast

	// DominantSeat is the index of the seat with the highest
	// WinProbability. On ties (rare; archetypes producing identical
	// edge sums), the LOWER index wins (seat 0 is "first turn order"
	// — a tiny structural advantage in 4-player Commander).
	DominantSeat int

	// AvgEdgeMagnitude is the mean of |NumericEdge| across all 12
	// directional matchups (4 seats × 3 opponents). High mean →
	// the pod is full of strong matchup gradients; near-zero mean
	// → the pod is structurally balanced (4 archetypes that don't
	// have strong opinions about each other).
	AvgEdgeMagnitude float64

	// Asymmetric is true when DominantSeat's WinProbability > 0.40
	// (vs the uniform 0.25 baseline = +60% relative). UIs can use
	// this to gate "this pod has a clear favorite" callouts.
	Asymmetric bool

	// Verdict is a 1-2 sentence summary rendering the dominant
	// seat, the underdog, and the overall pod shape. Suitable for
	// pre-game UI and the spectator screen.
	Verdict string
}

// PodSeatForecast is one seat's slice of the pod forecast — the
// seat's archetype, predicted win probability, and the 3 per-
// opponent edges that drove the prediction.
type PodSeatForecast struct {
	SeatIndex      int
	Archetype      string  // input archetype string (preserves caller's casing)
	ArchetypeKey   string  // canonicalized key used for metaMatchupDB lookup
	WinProbability float64 // 0.0-1.0; sums to 1.0 across all seats
	BaseScore      float64 // sum of NumericEdge across the 3 opponents
	EdgesVsOthers  []SeatMatchupEdge
}

// SeatMatchupEdge is the directional matchup signal from one seat
// to one opponent seat, surfaced for explanation UIs ("seat 0 favors
// seat 1: Combo > Voltron — goldfish lands before commander damage
// assembles").
type SeatMatchupEdge struct {
	OpponentSeat      int
	OpponentArchetype string
	Rating            string  // "favored" / "neutral" / "unfavored" / "" if no entry
	Strength          string  // "strong" / "moderate" / "slight" / "" if neutral
	NumericEdge       float64 // signed edge per the rating × strength table
	Reason            string  // copied from metaMatchupDB entry; empty when neutral or missing
}

// ForecastPodMatchups computes the PodMatchupForecast for a pod of
// archetype strings. Accepts any string slice — the function
// canonicalises each via canonicaliseMetaArchetypeKey (the same
// helper computeMetaPositioning uses) so callers can pass either
// the raw classifier output ("Go Wide Tokens") or the canonical key
// ("aggro").
//
// Returns nil for pods with fewer than 2 seats (no matchups to
// forecast). Returns a forecast for any pod with 2+ seats — though
// Commander is canonically 4-player, the function generalizes so
// 3-player party-mode and 2-player gauntlet runs can use it too.
func ForecastPodMatchups(archetypes []string) *PodMatchupForecast {
	if len(archetypes) < 2 {
		return nil
	}

	n := len(archetypes)
	seats := make([]PodSeatForecast, n)
	totalEdgeMag := 0.0
	totalDirections := 0

	for i, arch := range archetypes {
		key := canonicaliseMetaArchetypeKey(arch)
		seat := PodSeatForecast{
			SeatIndex:    i,
			Archetype:    arch,
			ArchetypeKey: key,
		}
		baseScore := 0.0
		for j, oppArch := range archetypes {
			if i == j {
				continue
			}
			oppKey := canonicaliseMetaArchetypeKey(oppArch)
			edge := lookupSeatEdge(key, oppKey)
			edge.OpponentSeat = j
			edge.OpponentArchetype = oppArch
			baseScore += edge.NumericEdge
			totalEdgeMag += math.Abs(edge.NumericEdge)
			totalDirections++
			seat.EdgesVsOthers = append(seat.EdgesVsOthers, edge)
		}
		seat.BaseScore = baseScore
		seats[i] = seat
	}

	softmaxToWinProbabilities(seats)

	out := &PodMatchupForecast{
		Seats: seats,
	}
	if totalDirections > 0 {
		out.AvgEdgeMagnitude = totalEdgeMag / float64(totalDirections)
	}

	// Dominant seat: highest WinProbability; ties broken by LOWER
	// seat index (first-player tie-break).
	out.DominantSeat = 0
	for i := 1; i < len(seats); i++ {
		if seats[i].WinProbability > seats[out.DominantSeat].WinProbability {
			out.DominantSeat = i
		}
	}
	out.Asymmetric = seats[out.DominantSeat].WinProbability > 0.40

	out.Verdict = buildPodForecastVerdict(out)
	return out
}

// lookupSeatEdge consults metaMatchupDB for the directional matchup
// from `fromKey` against `vsKey`. Both keys must be canonical
// lowercase (the output of canonicaliseMetaArchetypeKey). Returns a
// SeatMatchupEdge with NumericEdge derived from the rating ×
// strength table; OpponentSeat / OpponentArchetype left zero-value
// (the caller fills those in to keep this function pod-shape-
// independent).
func lookupSeatEdge(fromKey, vsKey string) SeatMatchupEdge {
	entries, ok := metaMatchupDB[fromKey]
	if !ok {
		return SeatMatchupEdge{}
	}
	for _, e := range entries {
		// vsArchetype in the DB uses title-case display strings
		// (e.g. "Combo", "Stax", "Go Wide Tokens"). Compare against
		// the canonical key derived from the display string so
		// "Combo" matches "combo" and "Go Wide Tokens" → "aggro"
		// matches the "aggro" canonical input.
		entryKey := canonicaliseMetaArchetypeKey(e.vsArchetype)
		if entryKey != vsKey {
			continue
		}
		out := SeatMatchupEdge{
			Rating:      e.rating,
			Reason:      e.reason,
			NumericEdge: edgeMagnitudeFor(e.rating, metaMatchupStrengthOrDefault(e)),
		}
		// Apply the (fromKey, vsArchetype) strength override if
		// present — mirrors the override layer that
		// computeMetaPositioning applies in its display path.
		overrideKey := [2]string{fromKey, e.vsArchetype}
		if override, ok := metaMatchupStrengthOverrides[overrideKey]; ok {
			out.Strength = override
			out.NumericEdge = edgeMagnitudeFor(e.rating, override)
		} else {
			out.Strength = metaMatchupStrengthOrDefault(e)
		}
		return out
	}
	return SeatMatchupEdge{}
}

// edgeMagnitudeFor converts a (rating, strength) pair into the
// signed numeric edge used by the softmax aggregator.
//
//	favored × strong       = +0.30
//	favored × moderate     = +0.15
//	favored × slight       = +0.05
//	neutral                =  0
//	unfavored × slight     = -0.05
//	unfavored × moderate   = -0.15
//	unfavored × strong     = -0.30
//
// Empty rating returns 0 (no data — treat as neutral). Unknown
// strength values fall through to the moderate magnitude.
func edgeMagnitudeFor(rating, strength string) float64 {
	if rating == "" || rating == "neutral" {
		return 0
	}
	mag := 0.15 // default moderate
	switch strength {
	case "strong":
		mag = 0.30
	case "slight":
		mag = 0.05
	}
	if rating == "unfavored" {
		return -mag
	}
	return mag
}

// softmaxToWinProbabilities converts BaseScore on each seat into
// WinProbability via softmax. Uses a temperature of 2.0 — chosen so
// the per-seat probabilities span a realistic Commander range
// (~10-50% with strong matchup gradients) rather than collapsing to
// 0.001 / 0.998 / 0.001 / 0.001 with raw exp(score).
//
// Temperature calibration: a strong-favored matchup contributes
// +0.30, a strong-unfavored contributes -0.30. A seat with three
// strong-favored matchups scores +0.90; one with three
// strong-unfavored scores -0.90. With temperature 2.0 the softmax
// produces ~0.50 vs ~0.05 across the extreme spread, leaving room
// for mid-tier ratings without saturating.
func softmaxToWinProbabilities(seats []PodSeatForecast) {
	const temperature = 2.0
	maxScore := math.Inf(-1)
	for _, s := range seats {
		if s.BaseScore > maxScore {
			maxScore = s.BaseScore
		}
	}
	if math.IsInf(maxScore, -1) {
		// All-empty case — divide uniform 1/N.
		uniform := 1.0 / float64(len(seats))
		for i := range seats {
			seats[i].WinProbability = uniform
		}
		return
	}
	sumExp := 0.0
	expScores := make([]float64, len(seats))
	for i, s := range seats {
		// Subtract maxScore before exp for numerical stability;
		// the resulting probabilities are unchanged.
		expScores[i] = math.Exp(temperature * (s.BaseScore - maxScore))
		sumExp += expScores[i]
	}
	if sumExp <= 0 {
		uniform := 1.0 / float64(len(seats))
		for i := range seats {
			seats[i].WinProbability = uniform
		}
		return
	}
	for i := range seats {
		seats[i].WinProbability = expScores[i] / sumExp
	}
}

// buildPodForecastVerdict assembles the 1-2 sentence summary. Format:
//
//	"Forecast: Combo (seat 0) leads at 42%, Voltron (seat 2)
//	 underdog at 12%. Asymmetric pod — Combo has the table."
func buildPodForecastVerdict(f *PodMatchupForecast) string {
	if f == nil || len(f.Seats) == 0 {
		return ""
	}
	// Find lowest-probability seat for the underdog callout.
	dom := f.DominantSeat
	under := 0
	for i := 1; i < len(f.Seats); i++ {
		if f.Seats[i].WinProbability < f.Seats[under].WinProbability {
			under = i
		}
	}
	parts := []string{
		fmt.Sprintf("%s (seat %d) leads at %.0f%%",
			f.Seats[dom].Archetype, dom, f.Seats[dom].WinProbability*100),
	}
	if under != dom {
		parts = append(parts, fmt.Sprintf("%s (seat %d) underdog at %.0f%%",
			f.Seats[under].Archetype, under, f.Seats[under].WinProbability*100))
	}
	v := "Forecast: " + strings.Join(parts, ", ") + "."
	if f.Asymmetric {
		v += fmt.Sprintf(" Asymmetric pod — %s has the table.",
			f.Seats[dom].Archetype)
	} else if f.AvgEdgeMagnitude < 0.05 {
		v += " Balanced pod — no archetype has a meaningful matchup edge."
	}
	return v
}

// SortedSeatsByWinProbability returns a copy of the forecast's
// Seats slice sorted by WinProbability descending. Useful for UI
// rendering ("show the predicted leaderboard") without mutating the
// input. Ties broken by lower SeatIndex.
func SortedSeatsByWinProbability(f *PodMatchupForecast) []PodSeatForecast {
	if f == nil {
		return nil
	}
	out := make([]PodSeatForecast, len(f.Seats))
	copy(out, f.Seats)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].WinProbability != out[j].WinProbability {
			return out[i].WinProbability > out[j].WinProbability
		}
		return out[i].SeatIndex < out[j].SeatIndex
	})
	return out
}
