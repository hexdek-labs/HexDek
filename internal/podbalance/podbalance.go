// Package podbalance computes the 4-deck pod power-tier symmetry
// assessment used by the matchmaker, tournament queue, and the hat.
// Extracted from cmd/hexdek-freya/pod_balance.go (PR #718) into a
// shared internal package so internal/party can consume it directly
// — cmd/hexdek-freya is package main and can't be imported.
//
// The Freya CLI still owns the SOURCE classification (PowerTier
// per deck via ClassifyCEDHPowerTier in cmd/hexdek-freya). This
// package owns only the POD-LEVEL rollup that turns 4 individual
// classifications into a single balance verdict.
package podbalance

import (
	"fmt"
	"sort"
)

// PodBalanceAssessment is the output of AssessPodBalance — a 4-deck
// pod's power-tier symmetry rollup.
//
// Balance bands map TierSpread to a human-readable label:
//
//	"balanced"      — spread ≤ 1 (e.g. B3/B3/B4/B4) — good pod
//	"mild_imbalance" — spread == 2 (e.g. B3/B3/B4/B5)
//	"imbalanced"    — spread == 3 (e.g. B2/B3/B4/B5)
//	"lopsided"      — spread == 4 (e.g. B1/B1/B1/B5) — bad pod
type PodBalanceAssessment struct {
	Decks         []PodDeckTier
	TierSpread    int
	Balance       string
	DominantTier  int
	Outliers      []string
	AvgTier       float64
	AvgConfidence float64
	Verdict       string
}

// PodDeckTier is one pod entry — a deck and its classified tier.
type PodDeckTier struct {
	DeckName   string
	Tier       int
	Confidence float64
	Label      string
}

// AssessPodBalance produces a PodBalanceAssessment for a pod of
// decks. See package doc.
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

	bestCount := 0
	bestTier := 0
	tiersSorted := make([]int, 0, len(tierCounts))
	for t := range tierCounts {
		tiersSorted = append(tiersSorted, t)
	}
	sort.Ints(tiersSorted)
	for _, t := range tiersSorted {
		c := tierCounts[t]
		if c > bestCount {
			bestCount = c
			bestTier = t
		}
	}
	if bestCount >= 2 {
		out.DominantTier = bestTier
	}

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

	out.Balance = BalanceBandForSpread(out.TierSpread)
	out.Verdict = buildVerdict(out)
	return out
}

// BalanceBandForSpread maps a tier spread to the named balance band.
// Exported so external callers (matchmaker, tournament queue) can
// classify a pod's spread without constructing a full assessment.
func BalanceBandForSpread(spread int) string {
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

// BalanceLabel renders the internal band slug as a human-readable
// label suitable for sentence-leading position.
func BalanceLabel(band string) string {
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

func buildVerdict(a *PodBalanceAssessment) string {
	if a == nil {
		return ""
	}
	tiers := make([]int, len(a.Decks))
	for i, d := range a.Decks {
		tiers[i] = d.Tier
	}
	sort.Ints(tiers)
	sortedStrs := make([]string, len(tiers))
	for i, t := range tiers {
		sortedStrs[i] = fmt.Sprintf("T%d", t)
	}
	label := BalanceLabel(a.Balance)
	verdict := fmt.Sprintf("%s pod: %s (avg %.2f).",
		label, joinSlash(sortedStrs), a.AvgTier)
	if len(a.Outliers) > 0 {
		verdict += fmt.Sprintf(" Outliers (≥2 tiers off): %s.",
			joinComma(a.Outliers))
	}
	return verdict
}

func joinSlash(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += " / "
		}
		out += s
	}
	return out
}

func joinComma(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
