package main

import (
	"fmt"
	"sort"
)

// PodBalanceAssessment is the output of AssessPodBalance — a 4-deck
// pod's power-tier symmetry rollup. Built for tournament queue systems
// (gauntlet, party-mode matchmaker) and the hat to decide whether a
// pod will play out as a balanced 4-way or as a lopsided game where
// the highest-tier deck pressures everyone else off the table.
//
// The spec from PR #718: "are the 4 power-tiers within 1 tier of
// each other (good pod) or spread B1-B5 (bad pod, will be
// lopsided)."
//
// Balance bands map TierSpread to a human-readable label:
//
//	"balanced"      — spread ≤ 1 (e.g. B3/B3/B4/B4) — good pod
//	"mild_imbalance" — spread == 2 (e.g. B3/B3/B4/B5)
//	"imbalanced"    — spread == 3 (e.g. B2/B3/B4/B5)
//	"lopsided"      — spread == 4 (e.g. B1/B1/B1/B5) — bad pod
//
// TournamentRunner / party matchmaker can use this to:
//   - Refuse to queue lopsided pods (spread ≥ 3) without explicit
//     "casual mismatch is OK" consent
//   - Surface the assessment to the spectator UI ("this pod is B3/B3/
//     B4/B5 — expect the B5 to attract early aggression")
//   - Feed the hat: at-table awareness of pod balance lets the hat
//     scale its threat-tracking ("I'm the lowest-tier deck — play
//     conservatively and stay under the politics radar")
type PodBalanceAssessment struct {
	// Decks is the 4 pod entries (or however many were supplied).
	// Sorted in input order — pod position is meaningful.
	Decks []PodDeckTier

	// TierSpread is max(Tier) - min(Tier) across all decks.
	// 0-4 range (5 tiers means max possible spread is 4).
	TierSpread int

	// Balance is the named band: "balanced" / "mild_imbalance" /
	// "imbalanced" / "lopsided". See PodBalanceAssessment doc for
	// the mapping.
	Balance string

	// DominantTier is the most common tier in the pod (mode). On a
	// tie, the LOWER tier wins (conservative — a 2-2 split between
	// T3 and T4 reports T3 as dominant since the table baseline is
	// more casual). When fewer than 2 decks share a tier, returns 0.
	DominantTier int

	// Outliers names decks that sit ≥2 tiers away from DominantTier
	// — the cards driving the imbalance. Empty when no deck is
	// 2+ off (every deck is within 1 tier of the table mode).
	Outliers []string

	// AvgTier is the arithmetic mean of all deck tiers, rounded to
	// 2 decimal places via float64 storage. Useful for sorting pods
	// by average power independently of the balance verdict.
	AvgTier float64

	// AvgConfidence is the mean PowerTier confidence across the
	// pod. Low avg confidence (<0.5) signals that the balance
	// verdict itself should be trusted less — boundary-tier decks
	// might be playing closer to their secondary tier in practice.
	AvgConfidence float64

	// Verdict is a 1-2 sentence summary suitable for tournament
	// UI / party screen. Includes the balance label + tier shape
	// + any outlier callout.
	Verdict string
}

// PodDeckTier is one pod entry — a deck and its classified tier.
// Sortable by tier for the assessment's own ordering operations.
type PodDeckTier struct {
	DeckName   string
	Tier       int
	Confidence float64
	Label      string
}

// AssessPodBalance produces a PodBalanceAssessment for a pod of
// decks. Accepts pre-classified tiers (typically built by calling
// ClassifyCEDHPowerTier on each deck's profile + report).
//
// Returns a zero-value assessment (with Balance="empty") when given
// fewer than 2 decks — a "pod" of 0 or 1 has no balance question to
// answer. Doesn't enforce exactly-4 because: (a) party-mode supports
// 3-player tables, (b) gauntlet test runs sometimes assess pairs.
// Tournament-runner integration should enforce 4 at the queue layer.
func AssessPodBalance(decks []PodDeckTier) *PodBalanceAssessment {
	if len(decks) < 2 {
		return &PodBalanceAssessment{
			Decks:   decks,
			Balance: "empty",
			Verdict: "pod too small to assess balance",
		}
	}

	out := &PodBalanceAssessment{Decks: decks}

	tierMin := decks[0].Tier
	tierMax := decks[0].Tier
	tierSum := 0
	confSum := 0.0
	tierCounts := map[int]int{}
	for _, d := range decks {
		if d.Tier < tierMin {
			tierMin = d.Tier
		}
		if d.Tier > tierMax {
			tierMax = d.Tier
		}
		tierSum += d.Tier
		confSum += d.Confidence
		tierCounts[d.Tier]++
	}
	out.TierSpread = tierMax - tierMin
	out.AvgTier = float64(tierSum) / float64(len(decks))
	out.AvgConfidence = confSum / float64(len(decks))

	// Dominant tier: highest count, ties broken by LOWER tier
	// (conservative — assume table baseline is the more casual seat).
	bestCount := 0
	bestTier := 0
	tiersSorted := make([]int, 0, len(tierCounts))
	for t := range tierCounts {
		tiersSorted = append(tiersSorted, t)
	}
	sort.Ints(tiersSorted)
	for _, t := range tiersSorted {
		c := tierCounts[t]
		// Strictly greater so equal counts at HIGHER tiers don't
		// overwrite the LOWER-tier winner (sort.Ints above gives
		// ascending order, so first hit at a given count wins).
		if c > bestCount {
			bestCount = c
			bestTier = t
		}
	}
	if bestCount >= 2 {
		out.DominantTier = bestTier
	}

	// Outliers: ≥2 tiers from DominantTier (when DominantTier set)
	// or from the rounded-mean tier (when no mode).
	refTier := out.DominantTier
	if refTier == 0 {
		refTier = int(out.AvgTier + 0.5)
	}
	for _, d := range decks {
		diff := d.Tier - refTier
		if diff < 0 {
			diff = -diff
		}
		if diff >= 2 {
			out.Outliers = append(out.Outliers, d.DeckName)
		}
	}

	out.Balance = balanceBandForSpread(out.TierSpread)
	out.Verdict = buildPodBalanceVerdict(out)
	return out
}

// balanceBandForSpread maps a tier spread to the named balance band.
// Spread of 0 or 1 reports "balanced" — the user-defined good-pod
// criterion ("within 1 tier of each other").
func balanceBandForSpread(spread int) string {
	switch {
	case spread <= 1:
		return "balanced"
	case spread == 2:
		return "mild_imbalance"
	case spread == 3:
		return "imbalanced"
	default:
		return "lopsided"
	}
}

// buildPodBalanceVerdict assembles the 1-2 sentence summary. Format:
//
//	"Balanced pod: T3 / T3 / T4 / T4 (avg 3.50)."
//	"Imbalanced pod: T1 / T3 / T4 / T5 (avg 3.25). Outliers
//	 (≥2 tiers off): tribal_pile, cedh_kinnan."
func buildPodBalanceVerdict(a *PodBalanceAssessment) string {
	if a == nil {
		return ""
	}
	tierStrs := make([]string, len(a.Decks))
	tiers := make([]int, len(a.Decks))
	for i, d := range a.Decks {
		tierStrs[i] = fmt.Sprintf("T%d", d.Tier)
		tiers[i] = d.Tier
	}
	sort.Ints(tiers)
	sortedStrs := make([]string, len(tiers))
	for i, t := range tiers {
		sortedStrs[i] = fmt.Sprintf("T%d", t)
	}
	label := podBalanceLabel(a.Balance)
	verdict := fmt.Sprintf("%s pod: %s (avg %.2f).",
		label, joinPodTierList(sortedStrs), a.AvgTier)
	if len(a.Outliers) > 0 {
		verdict += fmt.Sprintf(" Outliers (≥2 tiers off): %s.",
			joinCommaList(a.Outliers))
	}
	return verdict
}

// podBalanceLabel renders the internal balance band as a human-
// readable label suitable for sentence-leading position. The
// internal underscored slug ("mild_imbalance") becomes "Mildly
// imbalanced", etc.
func podBalanceLabel(band string) string {
	switch band {
	case "balanced":
		return "Balanced"
	case "mild_imbalance":
		return "Mildly imbalanced"
	case "imbalanced":
		return "Imbalanced"
	case "lopsided":
		return "Lopsided"
	case "empty":
		return "Empty"
	default:
		return band
	}
}

// joinPodTierList is the " / " join for tier sequence rendering.
// Kept local to pod_balance.go since joinCommaList (in
// power_tier_cedh.go) uses ", " and we want a different separator
// for tier sequences ("T3 / T3 / T4 / T4" reads more naturally
// than "T3, T3, T4, T4").
func joinPodTierList(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += " / "
		}
		out += s
	}
	return out
}
