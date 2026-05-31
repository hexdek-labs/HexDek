package main

import (
	"fmt"
	"io"
	"sort"
)

// ---------------------------------------------------------------------------
// Combo timing — earliest reasonable turn each detected combo can
// realistically assemble. Critical input for bracket calibration:
//
//   - cEDH (B5): typical kill-turn 3-4. A deck with multiple 2-cost
//     combos and heavy tutor density that estimates to turn-3
//     assembles MUST live in B5 regardless of card-list signals.
//
//   - high-power (B4): typical kill-turn 5-6. Combos at turn 4-5
//     with moderate tutor density.
//
//   - mid / casual (B2-B3): kill-turn 7+ or no reliable assembly.
//
// The estimate is a pacing heuristic, not a simulation. For each
// combo we compute:
//
//   1. NaturalTurn — the turn at which available cumulative mana
//      (assumed 1 mana per turn baseline, accumulating as N(N+1)/2)
//      is sufficient to cast every piece, clamped so we never claim
//      a piece resolves before its CMC is achievable on a single
//      turn. Includes a +1 "set up + activate" buffer for the
//      activation turn.
//
//   2. RampCompression — turns saved by the deck's ramp package.
//      floor(RampCount/8), capped at 2. Every ~8 ramp pieces accelerates
//      the natural curve by ~1 turn (Solving N(N+1)/2 ≥ totalCMC with
//      an extra mana-per-turn of 0.125 ≈ 1 turn off at total CMC 8).
//
//   3. TutorCompression — turns saved by the deck's tutor package.
//      floor(NonLandTutorCount/4), capped at 2. Every ~4 tutors
//      compresses the "find missing piece" wait by ~1 turn.
//
//   4. HandPenalty — +1 turn when KeepableHandPct < 0.50 (bad hands
//      mulligan to fewer cards, slowing assembly).
//
//   EarliestTurn = max(2, NaturalTurn - RampCompression - TutorCompression + HandPenalty)
//
// Floor at 2 because turn-1 combos don't exist in standard EDH (1 mana
// + 3-CMC commander). Hermit Druid / Ad Nauseam-style turn-2 wins are
// the documented fastest.
//
// Per-combo Pacing label maps EarliestTurn to "fast" (≤3) / "mid"
// (4-5) / "slow" (6+). Deck-level BracketHint takes the MIN
// EarliestTurn across all combos and maps to bracket signal: B5 cEDH
// (≤3), B4 high-power (4-5), B2-B3 mid/casual (6+). Hint is
// advisory — final bracket classification still flows through
// archetype.go's full signal set.
//
// Wiring: built in BuildDeckProfile AFTER computeOpeningHandSim so
// dp.KeepableHandPct is available for the hand-penalty factor.
// ---------------------------------------------------------------------------

// ComboTimingEstimate is the per-combo earliest-turn estimate.
type ComboTimingEstimate struct {
	ComboIndex int
	Label      string
	Source     string

	TotalCMC         int // sum of CMC across all pieces
	MaxPieceCMC      int // largest single-piece CMC
	NaturalTurn      int // raw mana-availability turn (before adjustments)
	RampCompression  int // turns saved by ramp pkg
	TutorCompression int // turns saved by tutor pkg
	HandPenalty      int // +1 when KeepableHandPct < 0.50
	EarliestTurn     int // final estimate (clamped >= 1)

	// Pacing — display label keyed off EarliestTurn:
	//   "fast" ≤ 3, "mid" 4-5, "slow" ≥ 6.
	Pacing string
}

// ComboTimingReport is the deck-level rollup.
type ComboTimingReport struct {
	PerCombo []ComboTimingEstimate

	MinTurn    int
	MaxTurn    int
	MedianTurn int

	FastestComboIndex int
	FastestComboLabel string
	FastestTurn       int

	SlowestComboIndex int
	SlowestComboLabel string
	SlowestTurn       int

	// BracketHint advisory string from MinTurn:
	//   "B5 cEDH" ≤ 3
	//   "B4 high-power" 4-5
	//   "B2-B3 mid/casual" ≥ 6
	BracketHint string
}

// BuildComboTimingReport produces the per-combo + deck-level timing
// analysis. Returns nil when the report has no combos in the
// categorical-win bucket. Reads:
//
//   - report.TrueInfinites + Determined + GraveyardLoops for combos
//   - report.Profiles for per-piece CMC
//   - report.NonLandTutorCount for tutor compression
//   - dp.RampCount for ramp compression
//   - dp.KeepableHandPct for hand penalty
func BuildComboTimingReport(report *FreyaReport, dp *DeckProfile) *ComboTimingReport {
	if report == nil || dp == nil {
		return nil
	}

	type entry struct {
		Index  int
		Label  string
		Cards  []string
		Source string
	}
	var entries []entry
	addAll := func(combos []ComboResult, source string) {
		for _, c := range combos {
			if len(c.Cards) == 0 {
				continue
			}
			cards := append([]string(nil), c.Cards...)
			sort.Strings(cards)
			entries = append(entries, entry{
				Label:  joinPlus(cards),
				Cards:  cards,
				Source: source,
			})
		}
	}
	addAll(report.TrueInfinites, "true_infinite")
	addAll(report.Determined, "determined")
	addAll(report.GraveyardLoops, "graveyard_loop")
	if len(entries) == 0 {
		return nil
	}

	// Dedup + index alignment: same convention as
	// BuildComboMetaInteraction (source asc, label asc) so per-combo
	// indices across the three combo reports line up.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Source != entries[j].Source {
			return entries[i].Source < entries[j].Source
		}
		return entries[i].Label < entries[j].Label
	})
	seen := map[string]bool{}
	deduped := entries[:0]
	for _, e := range entries {
		key := e.Source + "|" + e.Label
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, e)
	}
	for i := range deduped {
		deduped[i].Index = i
	}

	// Per-deck compression factors (computed once; same for every combo).
	rampCompression := dp.RampCount / 8
	if rampCompression > 2 {
		rampCompression = 2
	}
	tutorCompression := report.NonLandTutorCount / 4
	if tutorCompression > 2 {
		tutorCompression = 2
	}
	handPenalty := 0
	if dp.KeepableHandPct > 0 && dp.KeepableHandPct < 0.50 {
		handPenalty = 1
	}

	profileByName := profileMapByName(report.Profiles)

	out := &ComboTimingReport{}
	for _, e := range deduped {
		t := ComboTimingEstimate{
			ComboIndex: e.Index,
			Label:      e.Label,
			Source:     e.Source,
		}
		for _, name := range e.Cards {
			p, ok := profileByName[name]
			if !ok {
				continue
			}
			t.TotalCMC += p.CMC
			if p.CMC > t.MaxPieceCMC {
				t.MaxPieceCMC = p.CMC
			}
		}
		t.NaturalTurn = naturalManaTurn(t.TotalCMC, t.MaxPieceCMC)
		t.RampCompression = rampCompression
		t.TutorCompression = tutorCompression
		t.HandPenalty = handPenalty
		t.EarliestTurn = t.NaturalTurn - t.RampCompression - t.TutorCompression + t.HandPenalty
		// Floor at 2 — no turn-1 combos in standard EDH (1 mana,
		// 3-CMC commander, etc.). Hermit Druid / Ad Nauseam-style
		// turn-2 wins are the documented fastest. Stops aggressive
		// ramp+tutor compression from claiming impossible turn-1 kills.
		if t.EarliestTurn < 2 {
			t.EarliestTurn = 2
		}
		t.Pacing = pacingLabel(t.EarliestTurn)
		out.PerCombo = append(out.PerCombo, t)
	}

	turns := make([]int, len(out.PerCombo))
	for i, p := range out.PerCombo {
		turns[i] = p.EarliestTurn
	}
	sort.Ints(turns)
	out.MinTurn = turns[0]
	out.MaxTurn = turns[len(turns)-1]
	out.MedianTurn = turns[len(turns)/2]

	for i, p := range out.PerCombo {
		if i == 0 || p.EarliestTurn < out.FastestTurn {
			out.FastestComboIndex = i
			out.FastestComboLabel = p.Label
			out.FastestTurn = p.EarliestTurn
		}
		if i == 0 || p.EarliestTurn > out.SlowestTurn {
			out.SlowestComboIndex = i
			out.SlowestComboLabel = p.Label
			out.SlowestTurn = p.EarliestTurn
		}
	}
	out.BracketHint = bracketHintFromTurn(out.MinTurn)
	return out
}

// naturalManaTurn solves for the smallest T such that N(N+1)/2 >=
// totalCMC (sum of cumulative 1-per-turn mana is sufficient to cast
// every piece), clamped so T >= maxPieceCMC (can't cast a 6-CMC
// spell before turn 6), then +1 for the "set up + activate" turn.
// Returns 1 when totalCMC == 0 (placeholder / unmapped pieces).
func naturalManaTurn(totalCMC, maxPieceCMC int) int {
	if totalCMC <= 0 {
		return 1
	}
	t := 1
	for t*(t+1)/2 < totalCMC {
		t++
		if t > 20 {
			break // defensive — no combo legitimately needs > 20 turns
		}
	}
	if maxPieceCMC > t {
		t = maxPieceCMC
	}
	return t + 1 // +1 for the activation/resolution turn
}

func pacingLabel(turn int) string {
	switch {
	case turn <= 3:
		return "fast"
	case turn <= 5:
		return "mid"
	default:
		return "slow"
	}
}

func bracketHintFromTurn(minTurn int) string {
	switch {
	case minTurn <= 3:
		return "B5 cEDH"
	case minTurn <= 5:
		return "B4 high-power"
	default:
		return "B2-B3 mid/casual"
	}
}

// joinPlus is a tiny helper that joins strings with " + " — avoids
// pulling in strings just for one Join call.
func joinPlus(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += " + " + parts[i]
	}
	return out
}

// printComboTiming renders the human-readable section. No-op when r
// is nil or has no combos.
func printComboTiming(w io.Writer, r *ComboTimingReport) {
	if r == nil || len(r.PerCombo) == 0 {
		return
	}
	fmt.Fprintf(w, "[CYA] COMBO TIMING -- min turn %d, max %d, median %d (bracket hint: %s)\n",
		r.MinTurn, r.MaxTurn, r.MedianTurn, r.BracketHint)
	fmt.Fprintf(w, "  Fastest combo: [%d] %s (turn %d)\n",
		r.FastestComboIndex, r.FastestComboLabel, r.FastestTurn)
	if r.SlowestComboIndex != r.FastestComboIndex {
		fmt.Fprintf(w, "  Slowest combo: [%d] %s (turn %d)\n",
			r.SlowestComboIndex, r.SlowestComboLabel, r.SlowestTurn)
	}
	for _, p := range r.PerCombo {
		adjustments := ""
		if p.RampCompression > 0 || p.TutorCompression > 0 || p.HandPenalty > 0 {
			adjustments = fmt.Sprintf(" [natural %d - ramp %d - tutor %d + hand %d]",
				p.NaturalTurn, p.RampCompression, p.TutorCompression, p.HandPenalty)
		}
		fmt.Fprintf(w, "  [%d] %s — turn %d (%s)%s\n",
			p.ComboIndex, p.Label, p.EarliestTurn, p.Pacing, adjustments)
	}
	fmt.Fprintf(w, "\n")
}

// comboTimingToJSON projects the report into the JSON shape (defined
// in report.go). Returns nil when the input is nil so the JSON field
// is omitted via omitempty.
func comboTimingToJSON(r *ComboTimingReport) *jsonComboTimingReport {
	if r == nil {
		return nil
	}
	perCombo := make([]jsonComboTimingEntry, len(r.PerCombo))
	for i, p := range r.PerCombo {
		perCombo[i] = jsonComboTimingEntry{
			ComboIndex:       p.ComboIndex,
			Label:            p.Label,
			Source:           p.Source,
			TotalCMC:         p.TotalCMC,
			MaxPieceCMC:      p.MaxPieceCMC,
			NaturalTurn:      p.NaturalTurn,
			RampCompression:  p.RampCompression,
			TutorCompression: p.TutorCompression,
			HandPenalty:      p.HandPenalty,
			EarliestTurn:     p.EarliestTurn,
			Pacing:           p.Pacing,
		}
	}
	return &jsonComboTimingReport{
		PerCombo:          perCombo,
		MinTurn:           r.MinTurn,
		MaxTurn:           r.MaxTurn,
		MedianTurn:        r.MedianTurn,
		FastestComboIndex: r.FastestComboIndex,
		FastestComboLabel: r.FastestComboLabel,
		FastestTurn:       r.FastestTurn,
		SlowestComboIndex: r.SlowestComboIndex,
		SlowestComboLabel: r.SlowestComboLabel,
		SlowestTurn:       r.SlowestTurn,
		BracketHint:       r.BracketHint,
	}
}
