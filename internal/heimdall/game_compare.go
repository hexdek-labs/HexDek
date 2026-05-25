package heimdall

import (
	"sort"
)

// GameComparison is the side-by-side diff of two completed games. The
// shape is JSON-stable and rendered by /api/games/compare:
//
//   - GameA / GameB embed the full GameSummary for each game so the
//     frontend can render the two sides without a follow-up summary
//     fetch.
//   - Outcome / Turns are scalar-level diffs (same-or-different
//     booleans + numeric deltas), pre-computed so consumers don't have
//     to re-derive them by comparing fields.
//   - MVPDiff partitions the union of MVP card names into A-only,
//     B-only, and shared.
//   - TurningPointDivergence walks each turn that appears in either
//     game's TurningPoints and emits one row per turn with the
//     subset of points present only in A, only in B, or in both.
//   - SharedDecks is the intersection of DeckKeys (helpful when the
//     two games involve overlapping decks vs entirely separate ones).
//
// Comparison is intentionally pure end-state — the function does not
// reach into the DB, does not aggregate across other games, and is
// deterministic for a given input pair. Multi-game "winrate over time"
// stats belong in analytics.AggregateDecks, not here.
type GameComparison struct {
	GameA GameSummary `json:"game_a"`
	GameB GameSummary `json:"game_b"`

	Outcome                OutcomeDiff              `json:"outcome"`
	Turns                  TurnsDiff                `json:"turns"`
	MVPDiff                MVPDiff                  `json:"mvp_diff"`
	TurningPointDivergence []TurningPointDivergence `json:"turning_point_divergence"`
	SharedDecks            []string                 `json:"shared_decks"`
}

// OutcomeDiff captures the "who won and how" comparison. SameWinnerSeat
// compares the integer seat index; SameWinnerName compares the
// commander/owner string (these can disagree if the same deck-key
// won from a different seat in each game). SameEndReason flags
// whether both games ended the same way (combat / decking / etc.).
type OutcomeDiff struct {
	SameWinnerSeat bool `json:"same_winner_seat"`
	SameWinnerName bool `json:"same_winner_name"`
	SameEndReason  bool `json:"same_end_reason"`
}

// TurnsDiff is the length comparison: Delta = A.Turns - B.Turns.
// Positive Delta = A took longer; negative = B took longer.
type TurnsDiff struct {
	GameATurns int `json:"game_a_turns"`
	GameBTurns int `json:"game_b_turns"`
	Delta      int `json:"delta"`
}

// MVPDiff partitions MVP card names by which game's MVP set they
// appear in. Lists are sorted alphabetically for determinism.
// Comparison is by card name only (MVP score isn't compared — a card
// that's MVP-tier in both games is "shared" regardless of relative
// rank).
type MVPDiff struct {
	OnlyInA []string `json:"only_in_a"`
	OnlyInB []string `json:"only_in_b"`
	Shared  []string `json:"shared"`
}

// TurningPointDivergence groups the turning points from both games at
// a single turn. BothFired is the intersection by (Kind, Seat) at
// that turn — same kind of event happened to the same seat at the
// same turn in both games (e.g., commander_first_cast for seat 1 at
// turn 4 in both). OnlyInA / OnlyInB are the turning points present
// at this turn in just one game.
type TurningPointDivergence struct {
	Turn      int            `json:"turn"`
	OnlyInA   []TurningPoint `json:"only_in_a,omitempty"`
	OnlyInB   []TurningPoint `json:"only_in_b,omitempty"`
	BothFired []TurningPoint `json:"both_fired,omitempty"`
}

// CompareGames returns the full side-by-side comparison of two
// summaries. Either side may be a "db_only" summary — the diff falls
// out the same way since MVPDiff just sees empty MVP lists and
// TurningPointDivergence just sees the single game_end turning point
// on each side. The function never returns an error; it's a pure
// transformation.
func CompareGames(a, b GameSummary) GameComparison {
	return GameComparison{
		GameA:                  a,
		GameB:                  b,
		Outcome:                compareOutcome(a, b),
		Turns:                  TurnsDiff{GameATurns: a.Turns, GameBTurns: b.Turns, Delta: a.Turns - b.Turns},
		MVPDiff:                diffMVP(a.MVPCards, b.MVPCards),
		TurningPointDivergence: diffTurningPoints(a.TurningPoints, b.TurningPoints),
		SharedDecks:            intersectDeckKeys(a.DeckKeys, b.DeckKeys),
	}
}

func compareOutcome(a, b GameSummary) OutcomeDiff {
	return OutcomeDiff{
		SameWinnerSeat: a.Winner == b.Winner,
		SameWinnerName: a.WinnerName == b.WinnerName,
		SameEndReason:  a.EndReason == b.EndReason,
	}
}

func diffMVP(a, b []MVPCard) MVPDiff {
	aNames := mvpNameSet(a)
	bNames := mvpNameSet(b)
	d := MVPDiff{OnlyInA: []string{}, OnlyInB: []string{}, Shared: []string{}}
	for n := range aNames {
		if bNames[n] {
			d.Shared = append(d.Shared, n)
		} else {
			d.OnlyInA = append(d.OnlyInA, n)
		}
	}
	for n := range bNames {
		if !aNames[n] {
			d.OnlyInB = append(d.OnlyInB, n)
		}
	}
	sort.Strings(d.OnlyInA)
	sort.Strings(d.OnlyInB)
	sort.Strings(d.Shared)
	return d
}

func mvpNameSet(mvps []MVPCard) map[string]bool {
	out := make(map[string]bool, len(mvps))
	for i := range mvps {
		if n := mvps[i].CardName; n != "" {
			out[n] = true
		}
	}
	return out
}

// diffTurningPoints groups by turn, then by (Kind, Seat) within a
// turn. The result is sorted by Turn ascending. A turn with identical
// turning points in both games still emits a row (with empty
// OnlyInA/OnlyInB) so consumers can render the full timeline; if
// callers want only divergent turns they filter on
// `len(OnlyInA)+len(OnlyInB) > 0`.
func diffTurningPoints(a, b []TurningPoint) []TurningPointDivergence {
	turns := make(map[int]bool)
	for i := range a {
		turns[a[i].Turn] = true
	}
	for i := range b {
		turns[b[i].Turn] = true
	}
	if len(turns) == 0 {
		return []TurningPointDivergence{}
	}

	sortedTurns := make([]int, 0, len(turns))
	for t := range turns {
		sortedTurns = append(sortedTurns, t)
	}
	sort.Ints(sortedTurns)

	type key struct {
		kind string
		seat int
	}
	out := make([]TurningPointDivergence, 0, len(sortedTurns))
	for _, t := range sortedTurns {
		aPoints := turningPointsAtTurn(a, t)
		bPoints := turningPointsAtTurn(b, t)
		aKeys := make(map[key]TurningPoint, len(aPoints))
		bKeys := make(map[key]TurningPoint, len(bPoints))
		for i := range aPoints {
			aKeys[key{aPoints[i].Kind, aPoints[i].Seat}] = aPoints[i]
		}
		for i := range bPoints {
			bKeys[key{bPoints[i].Kind, bPoints[i].Seat}] = bPoints[i]
		}
		var onlyA, onlyB, both []TurningPoint
		for k, tp := range aKeys {
			if _, ok := bKeys[k]; ok {
				both = append(both, tp)
			} else {
				onlyA = append(onlyA, tp)
			}
		}
		for k, tp := range bKeys {
			if _, ok := aKeys[k]; !ok {
				onlyB = append(onlyB, tp)
			}
		}
		sortTurningPoints(onlyA)
		sortTurningPoints(onlyB)
		sortTurningPoints(both)
		out = append(out, TurningPointDivergence{
			Turn:      t,
			OnlyInA:   onlyA,
			OnlyInB:   onlyB,
			BothFired: both,
		})
	}
	return out
}

func turningPointsAtTurn(tps []TurningPoint, turn int) []TurningPoint {
	out := make([]TurningPoint, 0, 4)
	for i := range tps {
		if tps[i].Turn == turn {
			out = append(out, tps[i])
		}
	}
	return out
}

func sortTurningPoints(tps []TurningPoint) {
	sort.SliceStable(tps, func(i, j int) bool {
		if tps[i].Seat != tps[j].Seat {
			return tps[i].Seat < tps[j].Seat
		}
		return tps[i].Kind < tps[j].Kind
	})
}

func intersectDeckKeys(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return []string{}
	}
	aSet := make(map[string]bool, len(a))
	for _, k := range a {
		aSet[k] = true
	}
	seen := make(map[string]bool, len(a))
	out := make([]string, 0)
	for _, k := range b {
		if aSet[k] && !seen[k] {
			out = append(out, k)
			seen[k] = true
		}
	}
	sort.Strings(out)
	return out
}
