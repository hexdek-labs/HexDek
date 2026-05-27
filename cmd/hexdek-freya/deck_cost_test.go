package main

import (
	"strings"
	"testing"
)

// r60 add: deck-cost tier classification via Scryfall bulk USD
// prices. Three tiers (Budget <$100, Mid $100-$500, High-end >$500)
// with an 80% coverage gate — below that the signal is too noisy
// to classify, so dp.DeckCostTier stays empty and dp.DeckCostNote
// carries the coverage warning.

// stubPricedOracle builds a minimal oracleDB where each card has a
// USD price set via the Scryfall-shaped prices block. Mirrors the
// stubOracleWithText pattern in archetype_new_r60_test.go.
func stubPricedOracle(prices map[string]string) *oracleDB {
	db := &oracleDB{byName: map[string]*oracleEntry{}}
	for name, usd := range prices {
		e := &oracleEntry{
			Name:   name,
			Prices: map[string]string{"usd": usd},
		}
		db.byName[normalizeName(name)] = e
		db.byName[strings.ToLower(strings.TrimSpace(name))] = e
	}
	return db
}

// stubUnpricedOracle builds an oracleDB where each card has an entry
// but no usd price (Prices map present with empty/null usd value).
// Mirrors what Scryfall returns for reserved-list cards whose
// marketplace listing was pulled mid-snapshot.
func stubUnpricedOracle(names ...string) *oracleDB {
	db := &oracleDB{byName: map[string]*oracleEntry{}}
	for _, n := range names {
		e := &oracleEntry{
			Name:   n,
			Prices: map[string]string{"usd": ""},
		}
		db.byName[normalizeName(n)] = e
		db.byName[strings.ToLower(strings.TrimSpace(n))] = e
	}
	return db
}

// reportFromNames builds a minimal FreyaReport with a CardProfile
// entry per supplied name, sufficient for computeDeckCostTier to
// iterate over them.
func reportFromNames(names ...string) *FreyaReport {
	profiles := make([]CardProfile, 0, len(names))
	for _, n := range names {
		profiles = append(profiles, CardProfile{Name: n})
	}
	return &FreyaReport{Profiles: profiles}
}

// TestUSDPrice_ParsesAndRoundtrips pins oracleEntry.USDPrice parsing.
func TestUSDPrice_ParsesAndRoundtrips(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]string
		want float64
		ok   bool
	}{
		{"normal price", map[string]string{"usd": "12.34"}, 12.34, true},
		{"penny price", map[string]string{"usd": "0.05"}, 0.05, true},
		{"whitespace trimmed", map[string]string{"usd": "  4.99  "}, 4.99, true},
		{"empty value", map[string]string{"usd": ""}, 0, false},
		{"missing usd key", map[string]string{"eur": "10"}, 0, false},
		{"nil prices map", nil, 0, false},
		{"unparseable", map[string]string{"usd": "free"}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &oracleEntry{Prices: tc.raw}
			got, ok := e.USDPrice()
			if ok != tc.ok {
				t.Fatalf("ok mismatch: got %v want %v", ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("price mismatch: got %v want %v", got, tc.want)
			}
		})
	}
}

// TestUSDPrice_NilEntrySafe confirms calling USDPrice on a nil
// receiver doesn't panic — callers often dispatch via
// oracle.lookup() which returns nil for missing cards.
func TestUSDPrice_NilEntrySafe(t *testing.T) {
	var e *oracleEntry
	v, ok := e.USDPrice()
	if ok || v != 0 {
		t.Errorf("nil entry should return (0, false); got (%v, %v)", v, ok)
	}
}

// TestDeckCostTier_BudgetUnder100 pins the Budget tier: total <$100
// classifies as Budget, with a populated note.
func TestDeckCostTier_BudgetUnder100(t *testing.T) {
	dp := &DeckProfile{}
	report := reportFromNames("Sol Ring", "Counterspell", "Cultivate", "Swords to Plowshares", "Path to Exile")
	oracle := stubPricedOracle(map[string]string{
		"Sol Ring":             "1.50",
		"Counterspell":         "0.75",
		"Cultivate":            "0.50",
		"Swords to Plowshares": "2.00",
		"Path to Exile":        "1.25",
	})
	computeDeckCostTier(dp, report, oracle)
	if dp.DeckCostTier != "Budget" {
		t.Errorf("total $6 should classify as Budget; got %q (total=%v)", dp.DeckCostTier, dp.EstimatedTotalUSD)
	}
	if dp.PricedCardCount != 5 || dp.UnpricedCardCount != 0 {
		t.Errorf("expected 5 priced / 0 unpriced; got %d/%d", dp.PricedCardCount, dp.UnpricedCardCount)
	}
	if dp.DeckCostNote == "" {
		t.Errorf("note should be populated for a classifiable deck")
	}
}

// TestDeckCostTier_MidBoundary100Inclusive pins the Mid lower
// boundary: exactly $100 classifies as Mid (not Budget). The
// threshold is `< 100` for Budget, so $100.00 falls into Mid.
func TestDeckCostTier_MidBoundary100Inclusive(t *testing.T) {
	dp := &DeckProfile{}
	report := reportFromNames("CardA", "CardB")
	oracle := stubPricedOracle(map[string]string{
		"CardA": "50.00",
		"CardB": "50.00",
	})
	computeDeckCostTier(dp, report, oracle)
	if dp.DeckCostTier != "Mid" {
		t.Errorf("total $100 should be Mid (not Budget); got %q", dp.DeckCostTier)
	}
}

// TestDeckCostTier_MidUpperBoundary500Inclusive pins the Mid upper
// boundary: exactly $500 classifies as Mid (not High-end). The
// threshold is `<= 500` for Mid, so $500.00 stays in Mid.
func TestDeckCostTier_MidUpperBoundary500Inclusive(t *testing.T) {
	dp := &DeckProfile{}
	report := reportFromNames("CardA", "CardB")
	oracle := stubPricedOracle(map[string]string{
		"CardA": "250.00",
		"CardB": "250.00",
	})
	computeDeckCostTier(dp, report, oracle)
	if dp.DeckCostTier != "Mid" {
		t.Errorf("total $500 should be Mid (not High-end); got %q", dp.DeckCostTier)
	}
}

// TestDeckCostTier_HighEndOver500 pins the High-end tier: total
// >$500 classifies as High-end.
func TestDeckCostTier_HighEndOver500(t *testing.T) {
	dp := &DeckProfile{}
	report := reportFromNames("Mox Pearl", "Black Lotus")
	oracle := stubPricedOracle(map[string]string{
		"Mox Pearl":   "1000.00",
		"Black Lotus": "5000.00",
	})
	computeDeckCostTier(dp, report, oracle)
	if dp.DeckCostTier != "High-end" {
		t.Errorf("total $6000 should be High-end; got %q", dp.DeckCostTier)
	}
	if dp.EstimatedTotalUSD != 6000 {
		t.Errorf("estimated total = %v, want 6000", dp.EstimatedTotalUSD)
	}
}

// TestDeckCostTier_CoverageBelow80LeavesTierEmpty pins the 80%
// coverage gate: when fewer than 80% of cards have a resolvable USD
// price, the tier is left empty and DeckCostNote explains why.
//
// Construction: 8 cards, 6 priced (75% coverage — just under the
// floor). Even though the priced subset sums to $300 (Mid), no
// tier is assigned.
func TestDeckCostTier_CoverageBelow80LeavesTierEmpty(t *testing.T) {
	dp := &DeckProfile{}
	names := []string{
		"P1", "P2", "P3", "P4", "P5", "P6", // priced
		"U1", "U2", // unpriced
	}
	report := reportFromNames(names...)
	db := stubPricedOracle(map[string]string{
		"P1": "50.00", "P2": "50.00", "P3": "50.00",
		"P4": "50.00", "P5": "50.00", "P6": "50.00",
	})
	// Add unpriced entries (present in oracle but null USD).
	for _, n := range []string{"U1", "U2"} {
		e := &oracleEntry{Name: n, Prices: map[string]string{"usd": ""}}
		db.byName[normalizeName(n)] = e
	}
	computeDeckCostTier(dp, report, db)
	if dp.DeckCostTier != "" {
		t.Errorf("coverage 75%% should leave tier empty; got %q", dp.DeckCostTier)
	}
	if dp.EstimatedTotalUSD != 0 {
		t.Errorf("EstimatedTotalUSD should remain 0 when coverage gate trips; got %v", dp.EstimatedTotalUSD)
	}
	if !strings.Contains(dp.DeckCostNote, "coverage") {
		t.Errorf("note should mention coverage; got %q", dp.DeckCostNote)
	}
	// Priced/unpriced counts still populated so callers can render
	// the diagnostic.
	if dp.PricedCardCount != 6 || dp.UnpricedCardCount != 2 {
		t.Errorf("counts wrong: priced=%d unpriced=%d", dp.PricedCardCount, dp.UnpricedCardCount)
	}
}

// TestDeckCostTier_CoverageAt80Classifies confirms the boundary on
// the other side: exactly 80% coverage IS sufficient (the gate is
// `>= 0.80`, not `> 0.80`).
func TestDeckCostTier_CoverageAt80Classifies(t *testing.T) {
	dp := &DeckProfile{}
	names := []string{
		"P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", // 8 priced
		"U1", "U2", // 2 unpriced
	}
	report := reportFromNames(names...)
	db := stubPricedOracle(map[string]string{
		"P1": "5.00", "P2": "5.00", "P3": "5.00", "P4": "5.00",
		"P5": "5.00", "P6": "5.00", "P7": "5.00", "P8": "5.00",
	})
	for _, n := range []string{"U1", "U2"} {
		e := &oracleEntry{Name: n, Prices: map[string]string{"usd": ""}}
		db.byName[normalizeName(n)] = e
	}
	computeDeckCostTier(dp, report, db)
	if dp.DeckCostTier != "Budget" {
		t.Errorf("coverage 80%% with $40 total should classify Budget; got tier=%q total=%v",
			dp.DeckCostTier, dp.EstimatedTotalUSD)
	}
	if !strings.Contains(dp.DeckCostNote, "unpriced") {
		t.Errorf("note should mention unpriced cards when present; got %q", dp.DeckCostNote)
	}
}

// TestDeckCostTier_NilOracleNoOp confirms safety when oracle is nil
// — Freya runs without oracle data in unit-test contexts elsewhere
// in the codebase, so this path must be a no-op.
func TestDeckCostTier_NilOracleNoOp(t *testing.T) {
	dp := &DeckProfile{}
	report := reportFromNames("Sol Ring")
	computeDeckCostTier(dp, report, nil)
	if dp.DeckCostTier != "" {
		t.Errorf("nil oracle should leave tier empty; got %q", dp.DeckCostTier)
	}
	if dp.EstimatedTotalUSD != 0 || dp.PricedCardCount != 0 {
		t.Errorf("nil oracle should leave all fields zero")
	}
}

// TestDeckCostTier_EmptyDeckSafe confirms an empty report doesn't
// crash or set a tier.
func TestDeckCostTier_EmptyDeckSafe(t *testing.T) {
	dp := &DeckProfile{}
	report := &FreyaReport{}
	oracle := stubPricedOracle(map[string]string{})
	computeDeckCostTier(dp, report, oracle)
	if dp.DeckCostTier != "" {
		t.Errorf("empty deck should leave tier empty; got %q", dp.DeckCostTier)
	}
}

// TestDeckCostTier_TieringOrderingMonotonic pins the threshold
// ordering: a $50 deck < a $300 deck < a $1000 deck, and they land
// in Budget / Mid / High-end respectively.
func TestDeckCostTier_TieringOrderingMonotonic(t *testing.T) {
	cases := []struct {
		name      string
		perCard   string
		nCards    int
		wantTier  string
		wantTotal float64
	}{
		{"budget", "5.00", 10, "Budget", 50},
		{"mid", "30.00", 10, "Mid", 300},
		{"high", "100.00", 10, "High-end", 1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names := make([]string, tc.nCards)
			prices := map[string]string{}
			for i := 0; i < tc.nCards; i++ {
				n := tc.name + "_card_" + string(rune('a'+i))
				names[i] = n
				prices[n] = tc.perCard
			}
			dp := &DeckProfile{}
			computeDeckCostTier(dp, reportFromNames(names...), stubPricedOracle(prices))
			if dp.DeckCostTier != tc.wantTier {
				t.Errorf("tier mismatch: got %q want %q (total=%v)", dp.DeckCostTier, tc.wantTier, dp.EstimatedTotalUSD)
			}
			if dp.EstimatedTotalUSD != tc.wantTotal {
				t.Errorf("total mismatch: got %v want %v", dp.EstimatedTotalUSD, tc.wantTotal)
			}
		})
	}
}
