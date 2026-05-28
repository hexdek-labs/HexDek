package main

import (
	"fmt"
	"sort"
)

// CEDHPowerTier is the 5-tier power-level classification for a deck,
// derived from six independent signals (Game Changers, Mana Base Grade,
// Tutor Density, Weighted Win-Line Confidence, Interaction Package Score,
// Avg CMC) and named for the WotC bracket vocabulary plus the
// community-standard "cEDH" designation. Each signal independently votes
// for a tier in [1, 5]; the deck's tier is the median vote with a B5
// confirmation gate mirroring estimateMeasuredBracket's cEDH gate.
//
// Diverges from DeckProfile.MeasuredBracket in three ways:
//  1. Explicit "cEDH" label at tier 5 — the existing bracket call uses
//     the WotC "Bracket 5: cEDH" wording but the named layer surfaces
//     this for UIs that want to render "cEDH" prominently.
//  2. Integrates Win-Line Confidence and Interaction Package Score —
//     two newer signals that came in after the original bracket
//     calibration (PRs #664 / #665). MeasuredBracket sees them only
//     through their incidental contribution to gameChangerCount etc;
//     this classifier consumes them as first-class inputs.
//  3. Surfaces every signal's tier-vote individually so UIs can render
//     "scored T4 on tutors, T3 on interaction, T5 on mana base, T4
//     median" — answering "why is this a high-power deck?" cleanly.
type CEDHPowerTier struct {
	Tier    int    // 1-5
	Label   string // "Casual" / "Upgraded Precon" / "High Power" / "cEDH"
	Score   int    // Composite raw score (sum of signal votes, 6..30)
	Signals []CEDHTierSignal
	Verdict string // 1-2 sentence rationale assembled from contributing signals
}

// CEDHTierSignal documents one signal's contribution to the tier call.
// Surfaced individually so consumers can render a "tier breakdown" UI
// without re-deriving from the raw signals.
type CEDHTierSignal struct {
	Name         string // "Game Changers" / "Mana Base Grade" / etc
	Measurement  string // "5 GCs" / "Grade A" / "0.62 avg confidence"
	TierVote     int    // 1-5
	Note         string // optional human-readable rationale
}

// cedhTierLabel maps a tier 1-5 to the named layer. B1-B2 collapse to
// "Casual" per the spec — the WotC bracket framework distinguishes them
// (B1 "Exhibition" / B2 "Core") but the named layer surfaces them as a
// single "Casual" bucket since both indicate "this deck isn't trying to
// race" at the felt-power level.
func cedhTierLabel(tier int) string {
	switch tier {
	case 1, 2:
		return "Casual"
	case 3:
		return "Upgraded Precon"
	case 4:
		return "High Power"
	case 5:
		return "cEDH"
	default:
		return "Unknown"
	}
}

// ClassifyCEDHPowerTier produces the 5-tier named power-level call for a
// deck. Each of six signals votes independently in [1, 5]; the deck's
// tier is the median vote. The tier-5 ("cEDH") call is gated through a
// cEDH-marker check — a deck scoring median 5 still demotes to 4 if it
// lacks at least 2 of {AvgCMC < 2.8, GC ≥ 5, NonLandTutors ≥ 6,
// InteractionPackage ≥ 0.7, ManaBase == "A"}. Mirrors the gate in
// estimateMeasuredBracket but evaluates it on the named-layer's own
// signal set rather than the bracket scoring formula.
//
// Inputs are read from both DeckProfile (most signals) and FreyaReport
// (NonLandTutorCount lives there). Both must be non-nil; returns a
// "Casual" placeholder tier 1 with empty Signals if either is nil.
func ClassifyCEDHPowerTier(dp *DeckProfile, report *FreyaReport) *CEDHPowerTier {
	if dp == nil || report == nil {
		return &CEDHPowerTier{Tier: 1, Label: cedhTierLabel(1)}
	}

	signals := []CEDHTierSignal{
		cedhSignalGameChangers(dp),
		cedhSignalManaBase(dp),
		cedhSignalTutors(report),
		cedhSignalWinLineConfidence(report),
		cedhSignalInteractionPackage(dp),
		cedhSignalAvgCMC(dp),
	}

	tier, score := cedhMedianTier(signals)

	// B5 confirmation gate — median 5 only sticks when the deck shows
	// at least 2 of the cEDH-shape markers. Pre-gate, a midrange B4
	// pile that happens to score T5 across enough signals would mis-
	// classify; the gate enforces that tier 5 requires the actual
	// "tournament cEDH" shape: low CMC + tutor density + GCs + free
	// interaction + immaculate mana.
	gateVerdict := ""
	if tier == 5 {
		markers := 0
		var markerNotes []string
		if dp.AvgCMC < 2.8 {
			markers++
			markerNotes = append(markerNotes, fmt.Sprintf("CMC %.2f<2.8", dp.AvgCMC))
		}
		if dp.GameChangerCount >= 5 {
			markers++
			markerNotes = append(markerNotes, fmt.Sprintf("GCs %d≥5", dp.GameChangerCount))
		}
		if report.NonLandTutorCount >= 6 {
			markers++
			markerNotes = append(markerNotes, fmt.Sprintf("tutors %d≥6", report.NonLandTutorCount))
		}
		if dp.InteractionPackage.Score >= 0.70 {
			markers++
			markerNotes = append(markerNotes, fmt.Sprintf("interaction %.2f≥0.70", dp.InteractionPackage.Score))
		}
		if dp.ManaBaseGrade == "A" {
			markers++
			markerNotes = append(markerNotes, "mana A")
		}
		if markers < 2 {
			tier = 4
			gateVerdict = fmt.Sprintf("median voted 5 but only %d/5 cEDH markers tripped (need ≥2) — demoted to 4", markers)
		} else {
			gateVerdict = fmt.Sprintf("cEDH gate confirmed: %d/5 markers (%s)", markers, joinCommaList(markerNotes))
		}
	}

	out := &CEDHPowerTier{
		Tier:    tier,
		Label:   cedhTierLabel(tier),
		Score:   score,
		Signals: signals,
	}
	out.Verdict = buildCEDHTierVerdict(out, gateVerdict)
	return out
}

// cedhSignalGameChangers — WotC Game Changers list (highest-impact cards
// — Mana Crypt, Demonic Tutor, Underworld Breach family). Tier vote
// thresholds match the bracket framework's GC bands.
func cedhSignalGameChangers(dp *DeckProfile) CEDHTierSignal {
	n := dp.GameChangerCount
	vote := 1
	switch {
	case n >= 8:
		vote = 5
	case n >= 4:
		vote = 4
	case n >= 2:
		vote = 3
	case n >= 1:
		vote = 2
	default:
		vote = 1
	}
	return CEDHTierSignal{
		Name:        "Game Changers",
		Measurement: fmt.Sprintf("%d GC", n),
		TierVote:    vote,
	}
}

// cedhSignalManaBase — Freya's A-F mana base grade. Direct A=5 / B=4 /
// C=3 / D=2 / F=1 mapping. Empty grade (insufficient data) defaults to
// tier 2 (assume mid-grade casual until proven otherwise).
func cedhSignalManaBase(dp *DeckProfile) CEDHTierSignal {
	vote := 2
	switch dp.ManaBaseGrade {
	case "A":
		vote = 5
	case "B":
		vote = 4
	case "C":
		vote = 3
	case "D":
		vote = 2
	case "F":
		vote = 1
	}
	grade := dp.ManaBaseGrade
	if grade == "" {
		grade = "n/a"
	}
	return CEDHTierSignal{
		Name:        "Mana Base Grade",
		Measurement: fmt.Sprintf("Grade %s", grade),
		TierVote:    vote,
	}
}

// cedhSignalTutors — NonLandTutorCount (land ramp doesn't count toward
// consistency; only nonland tutors fetch your wincons). Thresholds
// chosen so the cEDH-typical 8-12 tutor count maps cleanly to tier 5,
// the "casual pile with Demonic Tutor singleton" doesn't trip above
// tier 2, and the 5-7 "upgraded precon with some tutor support" maps
// to tier 4.
func cedhSignalTutors(report *FreyaReport) CEDHTierSignal {
	n := report.NonLandTutorCount
	vote := 1
	switch {
	case n >= 8:
		vote = 5
	case n >= 5:
		vote = 4
	case n >= 3:
		vote = 3
	case n >= 1:
		vote = 2
	default:
		vote = 1
	}
	return CEDHTierSignal{
		Name:        "Tutor Density",
		Measurement: fmt.Sprintf("%d nonland tutors", n),
		TierVote:    vote,
	}
}

// cedhSignalWinLineConfidence — weighted average of WinLine.Confidence
// across the deck's win lines (from PR #664). High average confidence
// means the deck's win lines have tutor support, redundancy, and
// protection — the consistency hallmark of high-power play. Decks with
// no win lines vote tier 1 (no path to win the game on demand).
func cedhSignalWinLineConfidence(report *FreyaReport) CEDHTierSignal {
	if report.WinLines == nil || len(report.WinLines.WinLines) == 0 {
		return CEDHTierSignal{
			Name:        "Win-Line Confidence",
			Measurement: "no win lines",
			TierVote:    1,
		}
	}
	totalWeight := 0
	weighted := 0.0
	for _, wl := range report.WinLines.WinLines {
		w := wl.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
		weighted += float64(w) * wl.Confidence
	}
	avg := 0.0
	if totalWeight > 0 {
		avg = weighted / float64(totalWeight)
	}
	vote := 1
	switch {
	case avg >= 0.70:
		vote = 5
	case avg >= 0.50:
		vote = 4
	case avg >= 0.35:
		vote = 3
	case avg >= 0.20:
		vote = 2
	default:
		vote = 1
	}
	return CEDHTierSignal{
		Name:        "Win-Line Confidence",
		Measurement: fmt.Sprintf("%.2f weighted avg", avg),
		TierVote:    vote,
	}
}

// cedhSignalInteractionPackage — InteractionPackage.Score from PR #665.
// Sums counterspells + opponent-removal + protection + protection-
// tutors into a 0..1 defensive-package score. cEDH decks routinely
// score ≥0.75; mid-power runs 0.4-0.6; casual runs <0.3.
func cedhSignalInteractionPackage(dp *DeckProfile) CEDHTierSignal {
	s := dp.InteractionPackage.Score
	vote := 1
	switch {
	case s >= 0.75:
		vote = 5
	case s >= 0.50:
		vote = 4
	case s >= 0.30:
		vote = 3
	case s >= 0.15:
		vote = 2
	default:
		vote = 1
	}
	return CEDHTierSignal{
		Name:        "Interaction Package",
		Measurement: fmt.Sprintf("%.2f score", s),
		TierVote:    vote,
	}
}

// cedhSignalAvgCMC — average mana value across nonland cards. Low CMC
// is the cEDH curve signature (decks need to win on turn 3-4 to keep
// up with table threat density). Thresholds: <2.5 → T5, <3.0 → T4,
// <3.5 → T3, <4.0 → T2, else T1.
func cedhSignalAvgCMC(dp *DeckProfile) CEDHTierSignal {
	c := dp.AvgCMC
	vote := 1
	switch {
	case c > 0 && c < 2.5:
		vote = 5
	case c < 3.0:
		vote = 4
	case c < 3.5:
		vote = 3
	case c < 4.0:
		vote = 2
	default:
		vote = 1
	}
	return CEDHTierSignal{
		Name:        "Avg CMC",
		Measurement: fmt.Sprintf("%.2f CMC", c),
		TierVote:    vote,
	}
}

// cedhMedianTier returns the median tier-vote across the signal slice
// using the UPPER middle on even-count ties. Also returns the sum of
// votes as a composite score for tie-breaking and UI rendering.
//
// Upper-mid (vs lower-mid) chosen during r60 calibration on the 16-deck
// test corpus: with 6 signals, three of the four cEDH-B5 close misses
// (big_stick, lumra, turbo) had vote shapes 4/4/4/5/5/5 where lower-mid
// returns T4 and upper-mid returns T5. The yarus B4 miss had shape
// 2/3/3/4/4/4 where lower-mid returns T3 and upper-mid returns T4.
// Lower-mid produced 10/16 exact; upper-mid produces 13/16 exact with
// the same threshold tuning.
//
// The remaining ±2 miss (voja_wolf_elf_tribal_b4) is a real
// divergence: the underlying cEDH-shape signals (1 GC, 1 tutor, 0.33
// win-line confidence, 0.17 interaction, CMC 3.21) genuinely read as
// T2-Casual even though WotC's bracket framework labels the deck B4
// based on different signals (commander-driven, tribal-density). The
// classifier surfacing T2 on these signals is the honest answer —
// see TestCEDHPowerTier_Calibration for the documented edge case.
func cedhMedianTier(signals []CEDHTierSignal) (int, int) {
	if len(signals) == 0 {
		return 1, 0
	}
	votes := make([]int, len(signals))
	sum := 0
	for i, s := range signals {
		votes[i] = s.TierVote
		sum += s.TierVote
	}
	sort.Ints(votes)
	mid := len(votes) / 2
	return votes[mid], sum
}

// buildCEDHTierVerdict assembles the human-readable 1-2 sentence
// rationale: the tier label, the top-2 strongest contributing signals,
// and the gate verdict if tier 5 was evaluated.
func buildCEDHTierVerdict(t *CEDHPowerTier, gateVerdict string) string {
	if t == nil {
		return ""
	}
	// Find the top 2 highest-voting signals (the ones pulling the tier UP).
	type kv struct {
		name string
		vote int
		meas string
	}
	kvs := make([]kv, 0, len(t.Signals))
	for _, s := range t.Signals {
		kvs = append(kvs, kv{name: s.Name, vote: s.TierVote, meas: s.Measurement})
	}
	sort.SliceStable(kvs, func(i, j int) bool { return kvs[i].vote > kvs[j].vote })
	top := kvs
	if len(top) > 2 {
		top = top[:2]
	}
	parts := make([]string, 0, len(top))
	for _, k := range top {
		parts = append(parts, fmt.Sprintf("%s %s (T%d)", k.name, k.meas, k.vote))
	}
	verdict := fmt.Sprintf("Tier %d — %s. Strongest signals: %s.", t.Tier, t.Label, joinCommaList(parts))
	if gateVerdict != "" {
		verdict += " " + gateVerdict + "."
	}
	return verdict
}

// joinCommaList renders a string slice as "a, b, c" — basic comma-join.
// Local helper to avoid pulling strings.Join into a one-line caller
// and to make the call sites read more naturally as English.
func joinCommaList(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
