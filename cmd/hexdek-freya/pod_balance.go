package main

import "github.com/hexdek/hexdek/internal/podbalance"

// PodBalanceAssessment and AssessPodBalance moved to
// internal/podbalance in PR #719 so internal/party's matchmaker can
// consume them directly (cmd/hexdek-freya is package main and can't
// be imported). Type aliases below preserve the original freya-side
// import surface — every prior consumer of cmd/hexdek-freya's
// PodBalanceAssessment / PodDeckTier / AssessPodBalance continues to
// work without source changes.
//
// See package podbalance docs (internal/podbalance/podbalance.go)
// for the full type / function contracts.

// PodBalanceAssessment aliases podbalance.PodBalanceAssessment.
type PodBalanceAssessment = podbalance.PodBalanceAssessment

// PodDeckTier aliases podbalance.PodDeckTier.
type PodDeckTier = podbalance.PodDeckTier

// AssessPodBalance forwards to podbalance.AssessPodBalance.
func AssessPodBalance(decks []PodDeckTier) *PodBalanceAssessment {
	return podbalance.AssessPodBalance(decks)
}

// balanceBandForSpread + balanceBandLabel kept as local-private
// wrappers so the freya-side pod_balance_test.go tests that call
// them continue to work without import-path churn.
func balanceBandForSpread(spread int) string {
	return podbalance.BalanceBandForSpread(spread)
}
