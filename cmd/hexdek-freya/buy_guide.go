package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Deck buy guide — "if you have $X to spend on this deck, here are the
// highest-impact upgrades."
//
// Inputs:
//   - A completed DeckProfile + FreyaReport (Freya has already identified
//     the deck's gaps via density signals, coaching tips, and cuttable
//     cards).
//   - A budget in USD.
//   - A PriceLookup callback so prices stay external (the DB / API the
//     caller wires in). Freya itself is static-analysis only — exactly
//     like the recommendations PR's runtime-stats lookup.
//
// Algorithm:
//   1. Identify gap categories from the existing analysis: low ramp /
//      draw / tutors / interaction / wipes, etc.
//   2. For each open gap, pull candidate adds from the curated staples
//      catalog (cmd/hexdek-freya/staples below). Filter by color
//      identity (the deck can't run cards outside its identity) and
//      drop anything the deck already runs.
//   3. Pair each candidate with a SuggestedCut from dp.CuttableCards
//      so the "buy" comes with a "replace" — keeps the deck at 99.
//   4. Score: Impact = catalog baseline × gap-severity multiplier.
//      ImpactPerDollar = Impact / max(1, NetCost). NetCost =
//      AddPrice − CutPrice (cut value recouped from selling the cut).
//   5. Greedy knapsack: rank by ImpactPerDollar descending, pick into
//      the budget until exhausted. Candidates that would've ranked but
//      didn't fit go to SkippedOverBudget so the UI can show "if you
//      had $X more, you'd also want Y."
// ---------------------------------------------------------------------------

// PriceLookup returns the USD price for the named card. When the
// caller can't resolve a price, return ok=false; the candidate stays
// in the guide with a NetCost of 0 so consumers can mark it as
// "price unknown" rather than mis-sorting it as free.
type PriceLookup func(cardName string) (priceUSD float64, ok bool)

// BuyCandidate is one suggested add to the deck. Carries the gap it
// addresses, the cut it replaces, the dollar math, and the impact
// score that drove its rank in the budget allocator.
type BuyCandidate struct {
	AddCard         string
	AddPrice        float64
	AddPriceKnown   bool
	Category        string   // "interaction" | "draw" | "ramp" | "tutors" | "wipes" | "manabase"
	AddressesGap    string   // human-readable signal ("you have 4 ramp; bracket-4 decks want ≥10")
	SuggestedCut    string   // from dp.CuttableCards; empty if no cuttable
	CutPrice        float64
	CutPriceKnown   bool
	NetCost         float64  // AddPrice − CutPrice, floored at 0
	Impact          int      // 0-100
	ImpactPerDollar float64  // Impact / max(1, NetCost). Used by the budget sort.
	Colors          []string // required color identity for filtering ("WUBRG"-flavor letters)
}

// BuyGuide is the budget-aware buy plan. Suggested is the
// budget-fitting subset (greedy knapsack by ImpactPerDollar);
// SkippedOverBudget carries the candidates that would have ranked
// but didn't fit, so the UI can render "if you had $X more, you'd
// also want…". UnpriceableNotes lists cards the PriceLookup couldn't
// resolve — surfaces dependency holes (Scryfall API down, missing
// cache entry) so the caller can act on them.
type BuyGuide struct {
	Budget            float64
	SpentTotal        float64
	Suggested         []BuyCandidate
	SkippedOverBudget []BuyCandidate
	AllCandidates     []BuyCandidate // ranked, unfiltered — Decks screen can render "see all" view
	UnpriceableNotes  []string
}

// stapleEntry is one candidate add in the curated catalog. Colors
// is the canonical color-identity letters (W/U/B/R/G or empty for
// colorless) used to gate the candidate against the deck's identity.
// BaselineImpact is the 0-100 score before the per-deck severity
// multiplier; cards tagged 90+ are the canonical staples (Sol Ring,
// Rhystic Study), 70-89 are strong fillers, <70 are second-tier.
type stapleEntry struct {
	Name           string
	BaselineImpact int
	Colors         []string
}

// buyGuideStaples is the curated catalog. Kept deliberately tight —
// 5-8 canonical entries per category. Adding more dilutes the buy
// guide ("here are 47 candidates" is a worse answer than "here are
// the 3 highest-impact upgrades for your budget").
var buyGuideStaples = map[string][]stapleEntry{
	"ramp": {
		{Name: "Sol Ring", BaselineImpact: 95, Colors: nil},
		{Name: "Arcane Signet", BaselineImpact: 85, Colors: nil},
		{Name: "Mana Crypt", BaselineImpact: 95, Colors: nil},
		{Name: "Mana Vault", BaselineImpact: 92, Colors: nil},
		{Name: "Chrome Mox", BaselineImpact: 88, Colors: nil},
		{Name: "Mox Diamond", BaselineImpact: 90, Colors: nil},
		{Name: "Birds of Paradise", BaselineImpact: 82, Colors: []string{"G"}},
		{Name: "Llanowar Elves", BaselineImpact: 75, Colors: []string{"G"}},
		{Name: "Three Visits", BaselineImpact: 80, Colors: []string{"G"}},
	},
	"draw": {
		{Name: "Rhystic Study", BaselineImpact: 92, Colors: []string{"U"}},
		{Name: "Mystic Remora", BaselineImpact: 88, Colors: []string{"U"}},
		{Name: "Esper Sentinel", BaselineImpact: 85, Colors: []string{"W"}},
		{Name: "Necropotence", BaselineImpact: 90, Colors: []string{"B"}},
		{Name: "Sylvan Library", BaselineImpact: 82, Colors: []string{"G"}},
		{Name: "Phyrexian Arena", BaselineImpact: 78, Colors: []string{"B"}},
		{Name: "Wheel of Fortune", BaselineImpact: 85, Colors: []string{"R"}},
		{Name: "Windfall", BaselineImpact: 80, Colors: []string{"U"}},
	},
	"interaction": {
		{Name: "Swords to Plowshares", BaselineImpact: 92, Colors: []string{"W"}},
		{Name: "Path to Exile", BaselineImpact: 88, Colors: []string{"W"}},
		{Name: "Counterspell", BaselineImpact: 88, Colors: []string{"U"}},
		{Name: "Swan Song", BaselineImpact: 85, Colors: []string{"U"}},
		{Name: "An Offer You Can't Refuse", BaselineImpact: 80, Colors: []string{"U"}},
		{Name: "Generous Gift", BaselineImpact: 85, Colors: []string{"W"}},
		{Name: "Beast Within", BaselineImpact: 80, Colors: []string{"G"}},
		{Name: "Assassin's Trophy", BaselineImpact: 80, Colors: []string{"B", "G"}},
		{Name: "Force of Will", BaselineImpact: 90, Colors: []string{"U"}},
		{Name: "Fierce Guardianship", BaselineImpact: 85, Colors: []string{"U"}},
	},
	"wipes": {
		{Name: "Toxic Deluge", BaselineImpact: 92, Colors: []string{"B"}},
		{Name: "Cyclonic Rift", BaselineImpact: 95, Colors: []string{"U"}},
		{Name: "Damnation", BaselineImpact: 85, Colors: []string{"B"}},
		{Name: "Wrath of God", BaselineImpact: 80, Colors: []string{"W"}},
		{Name: "Farewell", BaselineImpact: 85, Colors: []string{"W"}},
		{Name: "Blasphemous Act", BaselineImpact: 82, Colors: []string{"R"}},
	},
	"tutors": {
		{Name: "Demonic Tutor", BaselineImpact: 95, Colors: []string{"B"}},
		{Name: "Vampiric Tutor", BaselineImpact: 92, Colors: []string{"B"}},
		{Name: "Imperial Seal", BaselineImpact: 90, Colors: []string{"B"}},
		{Name: "Mystical Tutor", BaselineImpact: 85, Colors: []string{"U"}},
		{Name: "Worldly Tutor", BaselineImpact: 82, Colors: []string{"G"}},
		{Name: "Enlightened Tutor", BaselineImpact: 85, Colors: []string{"W"}},
		{Name: "Survival of the Fittest", BaselineImpact: 88, Colors: []string{"G"}},
	},
	"manabase": {
		// Generic upgrades — color-aware caller will filter further.
		{Name: "Command Tower", BaselineImpact: 70, Colors: nil}, // multi-color decks only; gated below
		{Name: "Exotic Orchard", BaselineImpact: 65, Colors: nil},
		{Name: "Reflecting Pool", BaselineImpact: 75, Colors: nil},
		{Name: "Mana Confluence", BaselineImpact: 80, Colors: nil},
		{Name: "City of Brass", BaselineImpact: 75, Colors: nil},
	},
}

// BuildBuyGuide produces the budget-aware buy plan. Safe-default for
// nil inputs: returns an empty BuyGuide rather than panicking so the
// Decks screen can render "no data yet" cleanly.
func BuildBuyGuide(dp *DeckProfile, report *FreyaReport, budget float64, prices PriceLookup) *BuyGuide {
	guide := &BuyGuide{Budget: budget}
	if dp == nil || report == nil {
		return guide
	}
	// Owned cards (lowercased) — candidates already in the deck are
	// skipped to avoid suggesting buys the user already has.
	owned := map[string]bool{}
	for _, p := range report.Profiles {
		owned[strings.ToLower(p.Name)] = true
	}

	// Identify gap categories from the existing analysis.
	gaps := identifyBuyGaps(dp, report)
	if len(gaps) == 0 {
		return guide
	}

	// Cuts are an inventory limited by len(dp.CuttableCards). The
	// candidate-build pass projects NetCost against the FIRST
	// cuttable's price as a uniform proxy ("you'll cut your worst
	// card"); after the knapsack pass picks Suggested, we walk through
	// the cuttable pool and assign each pick a concrete, non-duplicate
	// cut + the actual CutPrice. Without this split, a candidate that
	// gets dropped from Suggested would still consume a cut from the
	// inventory, leaving later picks with empty SuggestedCut.
	baselineCutValue := 0.0
	baselineCutName := ""
	baselineCutKnown := false
	if len(dp.CuttableCards) > 0 {
		baselineCutName = dp.CuttableCards[0].Name
		if price, known := safePriceLookup(prices, baselineCutName); known {
			baselineCutValue = price
			baselineCutKnown = true
		}
	}

	// Build candidate list. One iteration per gap category × catalog
	// entry; filter by color identity + owned. Track AddPriceKnown
	// so the budget allocator can skip cards with no price data
	// rather than treating "unknown" as "free."
	var candidates []BuyCandidate
	seenAdd := map[string]bool{}
	for _, gap := range gaps {
		staples, ok := buyGuideStaples[gap.Category]
		if !ok {
			continue
		}
		for _, st := range staples {
			if owned[strings.ToLower(st.Name)] {
				continue
			}
			if !colorIdentityAllows(dp.ColorIdentity, st.Colors) {
				continue
			}
			if seenAdd[st.Name] {
				continue // already a candidate in a different gap
			}
			seenAdd[st.Name] = true

			cand := BuyCandidate{
				AddCard:      st.Name,
				Category:     gap.Category,
				AddressesGap: gap.Description,
				Colors:       st.Colors,
				Impact:       scaleImpact(st.BaselineImpact, gap.SeverityMultiplier),
			}
			if price, known := safePriceLookup(prices, st.Name); known {
				cand.AddPrice = price
				cand.AddPriceKnown = true
			} else {
				guide.UnpriceableNotes = append(guide.UnpriceableNotes,
					fmt.Sprintf("could not price add: %s", st.Name))
			}
			// Projected NetCost using the baseline cut value. Every
			// candidate stamps cuttable[0] as its projected cut +
			// price so AllCandidates carries a coherent NetCost story
			// even before knapsack runs (the test pin
			// TestBuildBuyGuide_NetCostAccountsForCutValue reads cut
			// info off AllCandidates). The knapsack post-pass may
			// overwrite picks beyond rank 0 with their concrete
			// cycled cut.
			cand.SuggestedCut = baselineCutName
			cand.CutPrice = baselineCutValue
			cand.CutPriceKnown = baselineCutKnown
			cand.NetCost = cand.AddPrice - baselineCutValue
			if cand.NetCost < 0 {
				cand.NetCost = 0
			}
			// ImpactPerDollar: max(NetCost, 1) avoids div-by-zero AND
			// keeps free / cheap candidates from blowing up the ranking.
			denom := cand.NetCost
			if denom < 1 {
				denom = 1
			}
			cand.ImpactPerDollar = float64(cand.Impact) / denom
			candidates = append(candidates, cand)
		}
	}

	// Rank by ImpactPerDollar descending; alphabetical tiebreak for
	// determinism across runs.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].ImpactPerDollar != candidates[j].ImpactPerDollar {
			return candidates[i].ImpactPerDollar > candidates[j].ImpactPerDollar
		}
		return candidates[i].AddCard < candidates[j].AddCard
	})
	guide.AllCandidates = candidates

	// Greedy knapsack: walk ranked candidates, pick while budget
	// remains. Skip unpriceable cards (we don't know they fit). They
	// stay in AllCandidates so the UI can render "candidates we
	// couldn't price" but don't pollute Suggested with zero-cost lies.
	for _, c := range candidates {
		if !c.AddPriceKnown {
			continue
		}
		if guide.SpentTotal+c.NetCost > budget {
			guide.SkippedOverBudget = append(guide.SkippedOverBudget, c)
			continue
		}
		guide.Suggested = append(guide.Suggested, c)
		guide.SpentTotal += c.NetCost
	}

	// Post-knapsack: for picks beyond rank 0, cycle through the
	// cuttable pool so two picks don't claim the same cut. The first
	// pick keeps the baseline cut already stamped at build time
	// (cuttable[0]) — its cut is correct, and the AllCandidates pin
	// expects that name to be intact. NetCost / SpentTotal are
	// adjusted for the actual cycled cut's price.
	for i := 1; i < len(guide.Suggested); i++ {
		if i >= len(dp.CuttableCards) {
			// More picks than cuttables — extras keep the baseline
			// cut name (a duplicate, but the user manually resolves
			// final cut choice anyway).
			break
		}
		cutName := dp.CuttableCards[i].Name
		guide.Suggested[i].SuggestedCut = cutName
		if price, known := safePriceLookup(prices, cutName); known {
			actualNet := guide.Suggested[i].AddPrice - price
			if actualNet < 0 {
				actualNet = 0
			}
			// Running SpentTotal must reflect the actual cut value
			// rather than the baseline projection.
			guide.SpentTotal += actualNet - guide.Suggested[i].NetCost
			guide.Suggested[i].NetCost = actualNet
			guide.Suggested[i].CutPrice = price
			guide.Suggested[i].CutPriceKnown = true
		} else {
			// Cut exists in pool but caller couldn't price it. Reset
			// CutPrice to 0 (we can't honestly project a net) but
			// keep the cut NAME so the user knows what to swap out.
			guide.SpentTotal += guide.Suggested[i].AddPrice - guide.Suggested[i].NetCost
			guide.Suggested[i].NetCost = guide.Suggested[i].AddPrice
			guide.Suggested[i].CutPrice = 0
			guide.Suggested[i].CutPriceKnown = false
		}
	}
	return guide
}

// buyGap is one open category — what the deck needs and how badly.
// Severity multiplier scales the catalog's baseline impact: a deck
// missing 6 ramp pieces gets a higher multiplier than one missing 1.
type buyGap struct {
	Category           string
	Description        string
	SeverityMultiplier float64 // 0.5-1.5
}

// identifyBuyGaps inspects the deck's density + coaching signals
// and emits one buyGap per category with an unmet target. Pulls
// thresholds from the same bracket-scaled rubric coaching.go uses.
func identifyBuyGaps(dp *DeckProfile, report *FreyaReport) []buyGap {
	bracket := dp.Bracket
	if bracket == 0 {
		bracket = 3
	}
	var gaps []buyGap

	// Ramp.
	rampFloor := 8
	if bracket >= 4 {
		rampFloor = 10
	}
	if dp.RampCount < rampFloor {
		gap := rampFloor - dp.RampCount
		gaps = append(gaps, buyGap{
			Category:           "ramp",
			Description:        fmt.Sprintf("ramp is %d; bracket-%d wants ≥%d", dp.RampCount, bracket, rampFloor),
			SeverityMultiplier: gapSeverity(gap),
		})
	}

	// Draw.
	drawFloor := 7
	switch {
	case bracket >= 5:
		drawFloor = 11
	case bracket >= 4:
		drawFloor = 9
	}
	if dp.DrawCount < drawFloor {
		gap := drawFloor - dp.DrawCount
		gaps = append(gaps, buyGap{
			Category:           "draw",
			Description:        fmt.Sprintf("draw is %d; bracket-%d wants ≥%d", dp.DrawCount, bracket, drawFloor),
			SeverityMultiplier: gapSeverity(gap),
		})
	}

	// Interaction (removal + counterspells).
	interaction := report.RemovalCount
	if report.Roles != nil {
		interaction += report.Roles.RoleCounts[RoleCounterspell]
	}
	interactionFloor := 8
	switch {
	case bracket >= 5:
		interactionFloor = 12
	case bracket >= 4:
		interactionFloor = 10
	}
	if interaction < interactionFloor {
		gap := interactionFloor - interaction
		gaps = append(gaps, buyGap{
			Category:           "interaction",
			Description:        fmt.Sprintf("interaction is %d; bracket-%d wants ≥%d", interaction, bracket, interactionFloor),
			SeverityMultiplier: gapSeverity(gap),
		})
	}

	// Board wipes.
	if report.Roles != nil && report.Roles.RoleCounts[RoleBoardWipe] == 0 {
		gaps = append(gaps, buyGap{
			Category:           "wipes",
			Description:        "deck runs zero board wipes — cannot reset against runaway opponents",
			SeverityMultiplier: 1.2,
		})
	}

	// Tutors — only for combo / storm at bracket 4+.
	arch := strings.ToLower(dp.PrimaryArchetype)
	if (strings.Contains(arch, "combo") || strings.Contains(arch, "storm")) && bracket >= 4 {
		tutorFloor := 5
		if bracket >= 5 {
			tutorFloor = 8
		}
		if report.NonLandTutorCount < tutorFloor {
			gap := tutorFloor - report.NonLandTutorCount
			gaps = append(gaps, buyGap{
				Category:           "tutors",
				Description:        fmt.Sprintf("tutors are %d; bracket-%d combo wants ≥%d", report.NonLandTutorCount, bracket, tutorFloor),
				SeverityMultiplier: gapSeverity(gap),
			})
		}
	}

	// Mana base — D/F grade triggers manabase candidates.
	if dp.ManaBaseGrade == "D" || dp.ManaBaseGrade == "F" {
		gaps = append(gaps, buyGap{
			Category:           "manabase",
			Description:        fmt.Sprintf("mana base grades %s — taplands + colored-source gaps", dp.ManaBaseGrade),
			SeverityMultiplier: 1.3,
		})
	}
	return gaps
}

// gapSeverity maps the size of an unmet gap to a 0.5-1.5 multiplier.
// 1-piece shortfall = 0.7 (small nudge), 4+-piece shortfall = 1.5
// (max boost). Keeps the worst gaps surfacing first without letting
// a single severe gap drown every other category.
func gapSeverity(gap int) float64 {
	switch {
	case gap >= 4:
		return 1.5
	case gap >= 3:
		return 1.2
	case gap >= 2:
		return 1.0
	case gap >= 1:
		return 0.7
	}
	return 0.5
}

func scaleImpact(baseline int, multiplier float64) int {
	v := int(float64(baseline) * multiplier)
	if v > 100 {
		v = 100
	}
	if v < 1 {
		v = 1
	}
	return v
}

// colorIdentityAllows checks whether `required` ⊆ `deck`. Empty
// required (colorless) is always allowed. Case-insensitive.
func colorIdentityAllows(deck, required []string) bool {
	if len(required) == 0 {
		return true
	}
	d := map[string]bool{}
	for _, c := range deck {
		d[strings.ToUpper(c)] = true
	}
	for _, c := range required {
		if !d[strings.ToUpper(c)] {
			return false
		}
	}
	return true
}

// safePriceLookup wraps the optional PriceLookup so nil callers
// don't have to worry about it. Returns (0, false) when the lookup
// is nil OR when it explicitly couldn't resolve the card.
func safePriceLookup(p PriceLookup, name string) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return p(name)
}

// FormatBuyGuide renders the guide for text-mode output (CLI / Decks
// screen). Two sections: suggested-within-budget and skipped-over-budget.
func FormatBuyGuide(g *BuyGuide) string {
	if g == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Buy Guide  (budget: $%.2f)\n", g.Budget)
	fmt.Fprintf(&b, "%s\n", strings.Repeat("=", 50))
	if len(g.Suggested) == 0 {
		fmt.Fprintf(&b, "  No upgrades fit the budget.\n")
	} else {
		fmt.Fprintf(&b, "  Picked %d upgrades (spent $%.2f of $%.2f):\n",
			len(g.Suggested), g.SpentTotal, g.Budget)
		for _, c := range g.Suggested {
			fmt.Fprintf(&b, "    + %-30s $%6.2f  [impact %3d/100, %s]\n",
				c.AddCard, c.AddPrice, c.Impact, c.Category)
			if c.SuggestedCut != "" {
				fmt.Fprintf(&b, "        ↳ cut %s (net $%.2f) — %s\n",
					c.SuggestedCut, c.NetCost, c.AddressesGap)
			} else {
				fmt.Fprintf(&b, "        ↳ %s\n", c.AddressesGap)
			}
		}
	}
	if len(g.SkippedOverBudget) > 0 {
		fmt.Fprintf(&b, "\n  Wishlist (over budget):\n")
		shown := g.SkippedOverBudget
		if len(shown) > 5 {
			shown = shown[:5]
		}
		for _, c := range shown {
			fmt.Fprintf(&b, "    %s — $%.2f net (impact %d)\n",
				c.AddCard, c.NetCost, c.Impact)
		}
	}
	if len(g.UnpriceableNotes) > 0 {
		fmt.Fprintf(&b, "\n  Note: %d card(s) had no price data.\n", len(g.UnpriceableNotes))
	}
	return b.String()
}
