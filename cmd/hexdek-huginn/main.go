// hexdek-huginn — Emergent interaction discovery CLI.
//
// Reads Heimdall's co-trigger observations, compresses them into
// resource flow patterns, and graduates interactions through confidence
// tiers. Persistent learned interactions feed into Freya for strategy
// augmentation.
//
// Usage:
//
//	hexdek-huginn --ingest                   # process new observations
//	hexdek-huginn --list                     # show all interactions by tier
//	hexdek-huginn --candidates               # show tier 3 promotion candidates
//	hexdek-huginn --stats                    # summary counts per tier
//	hexdek-huginn --prune                    # garbage collect tier 1-2
//	hexdek-huginn --partners "<card name>"   # partners for a card, ranked
//	hexdek-huginn --extend <deck.json>       # cards to add for interaction density
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hexdek/hexdek/internal/huginn"
)

func main() {
	var (
		dir        = flag.String("dir", "data/huginn", "huginn data directory")
		doIngest   = flag.Bool("ingest", false, "process new raw observations")
		doList     = flag.Bool("list", false, "show all interactions by tier")
		doCandidates = flag.Bool("candidates", false, "show tier 3 promotion candidates")
		doStats    = flag.Bool("stats", false, "summary counts per tier")
		doPrune    = flag.Bool("prune", false, "garbage collect stale tier 1-2 entries")
		gamesSince = flag.Int("games-since", 0, "games played since last ingest (for aging)")
		top        = flag.Int("top", 30, "max entries to display per section")
		partners   = flag.String("partners", "", "show Tier 2+ interaction partners for the named card")
		extend     = flag.String("extend", "", "path to a deck JSON; recommends cards to add for interaction density")
		minTier    = flag.Int("min-tier", huginn.TierRecurring, "min tier for --partners / --extend (1=observed, 2=recurring, 3=confirmed)")
		jsonOut    = flag.Bool("json", false, "emit JSON instead of human-readable text for --partners / --extend")
	)
	flag.Parse()

	// --partners and --extend short-circuit the other subcommands: they're
	// query-only and the default-flag fallback below would otherwise run
	// --stats + --list under their feet on every invocation.
	if *partners != "" {
		runPartners(*dir, *partners, *minTier, *top, *jsonOut)
		return
	}
	if *extend != "" {
		runExtend(*dir, *extend, *minTier, *top, *jsonOut)
		return
	}

	noFlag := !*doIngest && !*doList && !*doCandidates && !*doStats && !*doPrune
	if noFlag {
		*doStats = true
		*doList = true
	}

	if *doIngest {
		runIngest(*dir, *gamesSince)
	}
	if *doPrune {
		runPrune(*dir)
	}
	if *doStats {
		runStats(*dir)
	}
	if *doCandidates {
		runCandidates(*dir)
	}
	if *doList {
		runList(*dir, *top)
	}
}

func runPartners(dir, query string, minTier, top int, jsonOut bool) {
	hits, err := huginn.Partners(dir, query, minTier)
	if err != nil {
		log.Fatalf("partners: %v", err)
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]interface{}{
			"query":    query,
			"min_tier": minTier,
			"hits":     hits,
		}); err != nil {
			log.Fatalf("encode: %v", err)
		}
		return
	}
	fmt.Printf("=== PARTNERS for %q (min-tier %d) ===\n", query, minTier)
	if len(hits) == 0 {
		fmt.Println("  (no partners found at this tier)")
		return
	}
	limit := top
	if limit > len(hits) {
		limit = len(hits)
	}
	for i := 0; i < limit; i++ {
		h := hits[i]
		fmt.Printf("  %2d. [T%d] %-40s  score=%6.2f  patterns=%d\n",
			i+1, h.Tier, h.Partner, h.Score, h.PairCount)
		for j, p := range h.Patterns {
			if j >= 3 {
				fmt.Printf("       … and %d more pattern(s)\n", len(h.Patterns)-3)
				break
			}
			fmt.Printf("       → %s\n", p)
		}
	}
	if len(hits) > limit {
		fmt.Printf("  ... and %d more\n", len(hits)-limit)
	}
	fmt.Println()
}

func runExtend(dir, deckPath string, minTier, top int, jsonOut bool) {
	deck, names, err := huginn.LoadDeckJSON(deckPath)
	if err != nil {
		log.Fatalf("extend: %v", err)
	}
	hits, err := huginn.Extend(dir, names, minTier)
	if err != nil {
		log.Fatalf("extend: %v", err)
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]interface{}{
			"deck":     deckPath,
			"name":     deck.Name,
			"size":     len(names),
			"min_tier": minTier,
			"hits":     hits,
		}); err != nil {
			log.Fatalf("encode: %v", err)
		}
		return
	}
	label := deck.Name
	if label == "" {
		label = deckPath
	}
	fmt.Printf("=== EXTEND %q (%d cards, min-tier %d) ===\n", label, len(names), minTier)
	if len(hits) == 0 {
		fmt.Println("  (no candidates found at this tier — the corpus has no observed partners for any deck card)")
		return
	}
	limit := top
	if limit > len(hits) {
		limit = len(hits)
	}
	for i := 0; i < limit; i++ {
		h := hits[i]
		fmt.Printf("  %2d. [T%d] %-40s  score=%6.2f  pairs=%d\n",
			i+1, h.Tier, h.Card, h.Score, h.Pairs)
		if len(h.WithCards) > 0 {
			fmt.Printf("       with: %s\n", strings.Join(h.WithCards, ", "))
		}
		for j, p := range h.Patterns {
			if j >= 2 {
				break
			}
			fmt.Printf("       → %s\n", p)
		}
	}
	if len(hits) > limit {
		fmt.Printf("  ... and %d more\n", len(hits)-limit)
	}
	fmt.Println()
}

func runIngest(dir string, gamesSince int) {
	fmt.Println("=== HUGINN INGEST ===")
	fmt.Println()

	raw, err := huginn.ReadRawObservations(dir)
	if err != nil {
		log.Fatalf("read raw: %v", err)
	}
	fmt.Printf("Raw observations (pairwise): %d\n", len(raw))

	promotions, err := huginn.Ingest(dir, gamesSince)
	if err != nil {
		log.Fatalf("ingest: %v", err)
	}

	interactions, err := huginn.ReadLearnedInteractions(dir)
	if err != nil {
		log.Fatalf("read learned: %v", err)
	}
	fmt.Printf("Learned interactions (pairwise): %d\n", len(interactions))

	if len(promotions) > 0 {
		fmt.Printf("\n*** %d NEW TIER 3 PROMOTIONS (pairwise) ***\n", len(promotions))
		for i, p := range promotions {
			fmt.Printf("  %d. %s  obs=%d  decks=%d  avg-impact=%.1f\n",
				i+1, p.Pattern, p.ObservationCount, p.UniqueDeckCount, p.AvgImpactScore)
			if len(p.ExampleCards) > 0 {
				fmt.Printf("     examples: %s\n", strings.Join(p.ExampleCards, "; "))
			}
		}
	}

	// N-tuple ingestion (3-5 card combos).
	rawNT, err := huginn.ReadRawNTuples(dir)
	if err != nil {
		log.Fatalf("read raw ntuples: %v", err)
	}
	fmt.Printf("Raw observations (n-tuples): %d\n", len(rawNT))

	ntPromotions, err := huginn.IngestNTuples(dir, gamesSince)
	if err != nil {
		log.Fatalf("ingest ntuples: %v", err)
	}

	learnedNT, err := huginn.ReadLearnedNTuples(dir)
	if err != nil {
		log.Fatalf("read learned ntuples: %v", err)
	}
	fmt.Printf("Learned interactions (n-tuples): %d\n", len(learnedNT))

	if len(ntPromotions) > 0 {
		fmt.Printf("\n*** %d NEW TIER 3 PROMOTIONS (n-tuples) ***\n", len(ntPromotions))
		for i, p := range ntPromotions {
			fmt.Printf("  %d. [%s]  obs=%d  decks=%d  avg-impact=%.1f\n",
				i+1, strings.Join(p.Cards, " + "), p.ObservationCount, p.UniqueDeckCount, p.AvgImpactScore)
		}
	}
	fmt.Println()
}

func runPrune(dir string) {
	fmt.Println("=== HUGINN PRUNE ===")
	removed, err := huginn.Prune(dir)
	if err != nil {
		log.Fatalf("prune: %v", err)
	}
	fmt.Printf("Removed %d stale pairwise entries\n", removed)

	ntRemoved, err := huginn.PruneNTuples(dir)
	if err != nil {
		log.Fatalf("prune ntuples: %v", err)
	}
	fmt.Printf("Removed %d stale n-tuple entries\n\n", ntRemoved)
}

func runStats(dir string) {
	fmt.Println("=== HUGINN STATS (pairwise) ===")
	t1, t2, t3, total, err := huginn.Stats(dir)
	if err != nil {
		log.Fatalf("stats: %v", err)
	}
	fmt.Printf("  Tier 1 (OBSERVED):  %d\n", t1)
	fmt.Printf("  Tier 2 (RECURRING): %d\n", t2)
	fmt.Printf("  Tier 3 (CONFIRMED): %d\n", t3)
	fmt.Printf("  Total:              %d\n", total)
	fmt.Println()

	fmt.Println("=== HUGINN STATS (n-tuples) ===")
	learnedNT, err := huginn.ReadLearnedNTuples(dir)
	if err != nil {
		log.Fatalf("stats ntuples: %v", err)
	}
	var nt1, nt2, nt3 int
	for _, ln := range learnedNT {
		switch ln.Tier {
		case huginn.TierObserved:
			nt1++
		case huginn.TierRecurring:
			nt2++
		case huginn.TierConfirmed:
			nt3++
		}
	}
	fmt.Printf("  Tier 1 (OBSERVED):  %d\n", nt1)
	fmt.Printf("  Tier 2 (RECURRING): %d\n", nt2)
	fmt.Printf("  Tier 3 (CONFIRMED): %d\n", nt3)
	fmt.Printf("  Total:              %d\n", len(learnedNT))
	fmt.Println()
}

func runCandidates(dir string) {
	fmt.Println("=== HUGINN PROMOTION CANDIDATES ===")
	interactions, err := huginn.ReadLearnedInteractions(dir)
	if err != nil {
		log.Fatalf("read: %v", err)
	}

	found := false
	for _, li := range interactions {
		if li.Tier == huginn.TierConfirmed {
			if !found {
				fmt.Println()
				found = true
			}
			fmt.Printf("  [TIER 3] %s\n", li.Pattern)
			fmt.Printf("    obs=%d  decks=%d  avg-impact=%.1f  first=%s  last=%s\n",
				li.ObservationCount, li.UniqueDeckCount, li.AvgImpactScore,
				shortDate(li.FirstSeen), shortDate(li.LastSeen))
			if len(li.ExampleCards) > 0 {
				fmt.Printf("    examples: %s\n", strings.Join(li.ExampleCards, "; "))
			}
		}
	}
	if !found {
		fmt.Println("  (no tier 3 interactions yet)")
	}
	fmt.Println()
}

func runList(dir string, top int) {
	interactions, err := huginn.ReadLearnedInteractions(dir)
	if err != nil {
		log.Fatalf("read: %v", err)
	}

	tierNames := map[int]string{
		huginn.TierObserved:  "OBSERVED",
		huginn.TierRecurring: "RECURRING",
		huginn.TierConfirmed: "CONFIRMED",
	}

	for _, tier := range []int{huginn.TierConfirmed, huginn.TierRecurring, huginn.TierObserved} {
		var entries []huginn.LearnedInteraction
		for _, li := range interactions {
			if li.Tier == tier {
				entries = append(entries, li)
			}
		}
		if len(entries) == 0 {
			continue
		}

		fmt.Printf("=== TIER %d: %s (%d entries) ===\n", tier, tierNames[tier], len(entries))
		limit := top
		if limit > len(entries) {
			limit = len(entries)
		}
		for i := 0; i < limit; i++ {
			li := &entries[i]
			fmt.Printf("  %2d. %-50s obs=%-4d decks=%-3d impact=%.1f  age=%d games\n",
				i+1, li.Pattern, li.ObservationCount, li.UniqueDeckCount,
				li.AvgImpactScore, li.GamesSinceLastSeen)
			if len(li.ExampleCards) > 0 {
				exStr := strings.Join(li.ExampleCards, "; ")
				if len(exStr) > 80 {
					exStr = exStr[:77] + "..."
				}
				fmt.Printf("      cards: %s\n", exStr)
			}
		}
		if len(entries) > limit {
			fmt.Printf("  ... and %d more\n", len(entries)-limit)
		}
		fmt.Println()
	}

	if len(interactions) == 0 {
		fmt.Println("(no learned pairwise interactions yet — run --ingest first)")
		fmt.Println()
	}

	// N-tuple listing.
	learnedNT, err := huginn.ReadLearnedNTuples(dir)
	if err != nil {
		log.Fatalf("read ntuples: %v", err)
	}

	for _, tier := range []int{huginn.TierConfirmed, huginn.TierRecurring, huginn.TierObserved} {
		var entries []huginn.LearnedNTuple
		for _, ln := range learnedNT {
			if ln.Tier == tier {
				entries = append(entries, ln)
			}
		}
		if len(entries) == 0 {
			continue
		}

		fmt.Printf("=== N-TUPLES TIER %d: %s (%d entries) ===\n", tier, tierNames[tier], len(entries))
		limit := top
		if limit > len(entries) {
			limit = len(entries)
		}
		for i := 0; i < limit; i++ {
			ln := &entries[i]
			fmt.Printf("  %2d. [%s]  obs=%-4d decks=%-3d impact=%.1f  age=%d games\n",
				i+1, strings.Join(ln.Cards, " + "), ln.ObservationCount, ln.UniqueDeckCount,
				ln.AvgImpactScore, ln.GamesSinceLastSeen)
		}
		if len(entries) > limit {
			fmt.Printf("  ... and %d more\n", len(entries)-limit)
		}
		fmt.Println()
	}

	if len(learnedNT) == 0 && len(interactions) == 0 {
		fmt.Println("(no learned data yet — run --ingest first)")
		fmt.Println()
	}
}

func shortDate(rfc3339 string) string {
	if len(rfc3339) >= 10 {
		return rfc3339[:10]
	}
	return rfc3339
}

// We don't need os.Exit since log.Fatalf handles that.
var _ = os.Exit
