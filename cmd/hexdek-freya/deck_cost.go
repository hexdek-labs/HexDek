package main

import "fmt"

// ---------------------------------------------------------------------------
// Deck cost tier classification.
//
// Three tiers backed by Scryfall bulk USD prices (already loaded as
// part of oracle-cards.json into oracleEntry.Prices):
//
//	Budget    — total < $100
//	Mid       — total $100 – $500
//	High-end  — total > $500
//
// The tier is computed across all cards in report.Profiles (including
// the commander; including lands — lands often dominate cost for high-
// end mana bases). Per-card pricing uses oracleEntry.USDPrice (USD,
// non-foil, cheapest playable printing per Scryfall's "default"
// printing selection).
//
// Coverage gate: a tier is only assigned when at least 80% of the
// deck's cards have a resolvable USD price. Below that threshold the
// signal is too noisy — a high-end deck whose 30 reserved-list cards
// all have null usd would mis-classify as Budget. When coverage is
// insufficient the tier is left empty and DeckCostNote carries a
// short coverage-deficiency message so downstream consumers (UI,
// upgrade coach) can render "price data unavailable" instead of
// silently treating the deck as Budget.
//
// Notes on data sources, in priority order should the Scryfall bulk
// prove insufficient for some future use case:
//
//   1. Scryfall bulk (current): present in data/rules/oracle-cards.json,
//      refreshed via scripts/fetch-oracle.sh. Covers ~99% of standard-
//      legal-and-older cards with non-null USD on at least one
//      printing. Drawbacks: reserved-list cards (Mox Diamond, dual
//      lands in some bulks) sometimes carry null usd because the
//      retail listing was pulled mid-snapshot.
//
//   2. TCGPlayer API (future): would give us mid/market/low spreads
//      and current sale velocity. Requires API key + nightly cache.
//      Best fit when we add buy-guide rebates ("you can cash out the
//      cut card for $X").
//
//   3. MTGGoldfish / EDHRec scraping (future): only useful as a
//      sanity check against Scryfall — both ultimately consume
//      TCGPlayer downstream, so they're a redundant tier-2 source
//      rather than independent. Skip unless TCGPlayer is unavailable.
//
//   4. Per-printing variant tracking (future): the current logic uses
//      the cheapest printing's USD. A deck specifying "Sol Ring
//      (CMR)" should resolve to the CMR printing's USD, not the
//      bulk-default cheapest. Requires the deckparser to carry set
//      codes through to report.Profiles (today it strips them).
// ---------------------------------------------------------------------------

const (
	deckCostBudgetMaxUSD   = 100.0
	deckCostMidMaxUSD      = 500.0
	deckCostCoverageFloor  = 0.80 // fraction of cards that must have a resolvable price
)

func computeDeckCostTier(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if dp == nil || report == nil || oracle == nil {
		return
	}

	var total float64
	var priced, unpriced int
	for _, p := range report.Profiles {
		entry := oracle.lookup(p.Name)
		if entry == nil {
			unpriced++
			continue
		}
		v, ok := entry.USDPrice()
		if !ok {
			unpriced++
			continue
		}
		total += v
		priced++
	}

	dp.PricedCardCount = priced
	dp.UnpricedCardCount = unpriced

	considered := priced + unpriced
	if considered == 0 {
		return
	}
	coverage := float64(priced) / float64(considered)
	if coverage < deckCostCoverageFloor {
		dp.DeckCostNote = fmt.Sprintf(
			"price data unavailable for %d of %d cards (%.0f%% coverage; tier needs ≥80%%)",
			unpriced, considered, coverage*100)
		return
	}

	dp.EstimatedTotalUSD = total
	switch {
	case total < deckCostBudgetMaxUSD:
		dp.DeckCostTier = "Budget"
	case total <= deckCostMidMaxUSD:
		dp.DeckCostTier = "Mid"
	default:
		dp.DeckCostTier = "High-end"
	}
	if unpriced > 0 {
		dp.DeckCostNote = fmt.Sprintf(
			"$%.2f across %d cards (%d unpriced — real total may be higher)",
			total, priced, unpriced)
	} else {
		dp.DeckCostNote = fmt.Sprintf("$%.2f across %d cards", total, priced)
	}
}
