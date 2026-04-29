// mtgsquad-thor — Deterministic per-card interaction stress tester.
//
// For each card in the 32K oracle corpus, Thor places it on the
// battlefield and systematically applies every interaction type:
// destroy, exile, bounce, sacrifice, fight, target, counter, copy,
// phase out, transform. After each interaction it runs all 9 Odin
// invariants. Any failure is logged with exact card + interaction.
//
// Unlike Loki (random chaos), Thor is exhaustive and deterministic —
// every card gets tested against every interaction. The output is a
// surgical hit list: which cards break under which interactions.
//
// Usage:
//
//	go run ./cmd/mtgsquad-thor/ --report data/rules/THOR_REPORT.md
//	go run ./cmd/mtgsquad-thor/ --workers 10 --phases
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	_ "github.com/hexdek/hexdek/internal/gameengine/per_card"
)

type interaction struct {
	Name string
	Fn   func(gs *gameengine.GameState, perm *gameengine.Permanent)
}

type failure struct {
	CardName    string
	Interaction string
	Invariant   string
	Message     string
	Panicked    bool
	PanicMsg    string
}

func main() {
	workers := flag.Int("workers", runtime.NumCPU(), "parallel workers")
	reportPath := flag.String("report", "", "write markdown report to this path")
	withPhases := flag.Bool("phases", false, "also run full turn cycle per card")
	singleCard := flag.String("card", "", "test a single card by name (runs full battery)")
	cardListFile := flag.String("card-list", "", "test cards listed in a file (one name per line)")

	// New module flags.
	keywordMatrix := flag.Bool("keyword-matrix", false, "run keyword combat matrix (~900 pairs)")
	comboPairs := flag.Bool("combo-pairs", false, "run card combo pair tests (staple pairs)")
	replacementTest := flag.Bool("replacement", false, "run replacement effect conflict tests")
	layerStress := flag.Bool("layer-stress", false, "run layer stress boards")
	stackTorture := flag.Bool("stack-torture", false, "run stack torture tests")
	commanderRules := flag.Bool("commander", false, "run commander rule tests")
	apnapTest := flag.Bool("apnap", false, "run APNAP trigger ordering tests")
	zoneChains := flag.Bool("zone-chains", false, "run zone chain reaction tests")
	manaVerify := flag.Bool("mana-verify", false, "run mana payment verification")
	turnStructure := flag.Bool("turn-structure", false, "run turn structure compliance tests")
	spellResolve := flag.Bool("spell-resolve", false, "run spell resolution pipeline for instants/sorceries")
	goldilocksTest := flag.Bool("goldilocks", false, "run Goldilocks 'just right' effect verification")
	advancedMechanics := flag.Bool("advanced-mechanics", false, "run advanced mechanics edge-case tests (~145 scenarios)")
	deepRulesTest := flag.Bool("deep-rules", false, "run deep rules tests (100 scenarios, 20 packs, 13 invariants)")
	claimVerify := flag.Bool("claim-verify", false, "run coverage claim verifier (~60 tests)")
	negativeLegality := flag.Bool("negative-legality", false, "run negative legality pack (~40 tests)")
	chaosGames := flag.Bool("chaos", false, "run chaos game simulation (10K random games)")
	densityStress := flag.Bool("density", false, "run board density stress tests")
	cascadeTorture := flag.Bool("cascade", false, "run trigger cascade torture tests")
	graveyardStorm := flag.Bool("graveyard", false, "run graveyard interaction stress tests")
	oracleDiff := flag.Bool("oracle-diff", false, "run oracle text differential analysis")
	multiplayerChaos := flag.Bool("multiplayer", false, "run 4-player multiplayer chaos games")
	infiniteLoop := flag.Bool("infinite-loop", false, "run infinite loop detection tests")
	rollbackTest := flag.Bool("rollback", false, "run state rollback integrity tests")
	clockTest := flag.Bool("clock", false, "run clock pressure (turn time budget) tests")
	adversarialTest := flag.Bool("adversarial", false, "run adversarial targeting stress tests")
	symmetryTest := flag.Bool("symmetry", false, "run symmetry (player-swap) verification")
	corpusAuditTest := flag.Bool("corpus-audit", false, "run 34K corpus outcome correctness audit")
	corpusEraStr := flag.String("corpus-era", "all", "era filter for corpus audit: all, era1, era2, era3, era4")
	coverageDepth := flag.Bool("coverage-depth", false, "run AST coverage depth audit (Phase 2)")
	oracleCompliance := flag.Bool("oracle-compliance", false, "run per-card handler oracle compliance audit (Phase 3)")
	astFidelity := flag.Bool("ast-fidelity", false, "run AST-oracle fidelity audit (Phase 4)")
	allModules := flag.Bool("all", false, "run all modules (original + new)")

	comboDemo := flag.Bool("combo-demo", false, "run combo resolution demos")

	flag.Parse()

	if *comboDemo {
		runComboDemo()
		return
	}

	log.Println("mtgsquad-thor starting")
	log.Printf("  workers:    %d", *workers)
	log.Printf("  phases:     %v", *withPhases)

	// Load AST corpus.
	log.Println("loading AST corpus from data/rules/ast_dataset.jsonl ...")
	t0 := time.Now()
	corpus, err := astload.Load("data/rules/ast_dataset.jsonl")
	if err != nil {
		log.Fatalf("load corpus: %v", err)
	}
	log.Printf("  %d cards in %s", corpus.CardCount, time.Since(t0))

	// Load oracle corpus for card metadata.
	log.Println("loading oracle corpus from data/rules/oracle-cards.json ...")
	oracleCards, err := loadOracleCards("data/rules/oracle-cards.json")
	if err != nil {
		log.Fatalf("load oracle: %v", err)
	}
	log.Printf("  %d cards loaded", len(oracleCards))

	// Populate AST on oracle cards.
	for _, oc := range oracleCards {
		if a, ok := corpus.Get(oc.Name); ok {
			oc.ast = a
		}
	}

	// Single-card mode: filter to just that card and run full battery.
	if *singleCard != "" {
		var filtered []*oracleCard
		target := strings.ToLower(*singleCard)
		for _, oc := range oracleCards {
			if strings.ToLower(oc.Name) == target {
				filtered = append(filtered, oc)
				break
			}
		}
		if len(filtered) == 0 {
			log.Fatalf("card not found: %q", *singleCard)
		}
		oracleCards = filtered
		*withPhases = true
		*spellResolve = true
		*goldilocksTest = true
		*corpusAuditTest = true
		log.Printf("  SINGLE CARD MODE: %s", filtered[0].Name)
	}

	// Card-list mode: filter to cards in a file and run full battery.
	if *cardListFile != "" {
		listData, err := os.ReadFile(*cardListFile)
		if err != nil {
			log.Fatalf("read card list: %v", err)
		}
		wanted := map[string]bool{}
		for _, line := range strings.Split(string(listData), "\n") {
			name := strings.TrimSpace(line)
			if name != "" {
				wanted[strings.ToLower(name)] = true
			}
		}
		var filtered []*oracleCard
		for _, oc := range oracleCards {
			if wanted[strings.ToLower(oc.Name)] {
				filtered = append(filtered, oc)
			}
		}
		oracleCards = filtered
		*withPhases = true
		*spellResolve = true
		*goldilocksTest = true
		*corpusAuditTest = true
		log.Printf("  CARD LIST MODE: %d cards from %s", len(filtered), *cardListFile)
	}

	// Detect if any specific module was requested.
	anyModuleRequested := *keywordMatrix || *comboPairs || *replacementTest ||
		*layerStress || *stackTorture || *commanderRules || *apnapTest ||
		*zoneChains || *manaVerify || *turnStructure || *spellResolve ||
		*goldilocksTest || *advancedMechanics || *deepRulesTest ||
		*claimVerify || *negativeLegality || *chaosGames ||
			*densityStress || *cascadeTorture || *graveyardStorm || *oracleDiff ||
			*multiplayerChaos || *infiniteLoop || *rollbackTest || *clockTest ||
			*adversarialTest || *symmetryTest || *corpusAuditTest || *coverageDepth || *oracleCompliance || *astFidelity || *allModules

	// If --all is set, enable everything.
	if *allModules {
		*keywordMatrix = true
		*comboPairs = true
		*replacementTest = true
		*layerStress = true
		*stackTorture = true
		*commanderRules = true
		*apnapTest = true
		*zoneChains = true
		*manaVerify = true
		*turnStructure = true
		*spellResolve = true
		*goldilocksTest = true
		*advancedMechanics = true
		*deepRulesTest = true
		*claimVerify = true
		*negativeLegality = true
		*chaosGames = true
		*densityStress = true
		*cascadeTorture = true
		*graveyardStorm = true
		*oracleDiff = true
		*multiplayerChaos = true
		*infiniteLoop = true
		*rollbackTest = true
		*clockTest = true
		*adversarialTest = true
		*symmetryTest = true
		*corpusAuditTest = true
		*coverageDepth = true
		*oracleCompliance = true
		*astFidelity = true
	}

	// Parse corpus era flag.
	if *corpusAuditTest {
		corpusEraFlag = parseEra(*corpusEraStr)
	}

	// Build interaction set.
	interactions := buildInteractions()
	log.Printf("  %d interaction types", len(interactions))

	// Run tests.
	var (
		totalTests int64
		totalFails int64
		mu         sync.Mutex
		failures   []failure
	)

	start := time.Now()

	// Original per-card tests: run when no specific module is requested, --all, or --card.
	if !anyModuleRequested || *allModules || *singleCard != "" || *cardListFile != "" {
		// Build work channel.
		type workItem struct {
			card *oracleCard
			ast  *gameast.CardAST
		}
		work := make(chan workItem, 256)

		go func() {
			for _, oc := range oracleCards {
				ast, _ := corpus.Get(oc.Name)
				work <- workItem{card: oc, ast: ast}
			}
			close(work)
		}()

		var wg sync.WaitGroup
		for i := 0; i < *workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for wi := range work {
					cardFailures := testCard(wi.card, wi.ast, interactions, *withPhases)
					n := int64(len(interactions))
					if *withPhases {
						n += 7 // untap, upkeep, draw, main1, combat, main2, end
					}
					atomic.AddInt64(&totalTests, n)
					if len(cardFailures) > 0 {
						atomic.AddInt64(&totalFails, int64(len(cardFailures)))
						mu.Lock()
						failures = append(failures, cardFailures...)
						mu.Unlock()
					}

					done := atomic.LoadInt64(&totalTests)
					if done%5000 == 0 {
						elapsed := time.Since(start)
						rate := float64(done) / elapsed.Seconds()
						fmt.Printf("  thor: %d tests (%.0f/s) %d fails\n", done, rate, atomic.LoadInt64(&totalFails))
					}
				}
			}()
		}

		wg.Wait()
	}

	// === NEW MODULES ===
	type module struct {
		name    string
		enabled bool
		run     func(corpus *astload.Corpus, oracleCards []*oracleCard) []failure
	}

	modules := []module{
		{"keyword-matrix", *keywordMatrix, runKeywordMatrix},
		{"combo-pairs", *comboPairs, runComboPairs},
		{"replacement", *replacementTest, runReplacement},
		{"layer-stress", *layerStress, runLayerStress},
		{"stack-torture", *stackTorture, runStackTorture},
		{"commander", *commanderRules, runCommander},
		{"apnap", *apnapTest, runAPNAP},
		{"zone-chains", *zoneChains, runZoneChains},
		{"mana-verify", *manaVerify, runManaVerify},
		{"turn-structure", *turnStructure, runTurnStructure},
		// spell-resolve is integrated into per-card tests.
		// Goldilocks runs as standalone module for granular stats.
		{"goldilocks", *goldilocksTest, runGoldilocks},
		{"advanced-mechanics", *advancedMechanics, runAdvancedMechanics},
		{"deep-rules", *deepRulesTest, runDeepRules},
		{"claim-verify", *claimVerify, runClaimVerifier},
		{"negative-legality", *negativeLegality, runNegativeLegality},
		{"chaos", *chaosGames, runChaosGames},
		{"density", *densityStress, runDensityStress},
		{"cascade", *cascadeTorture, runCascadeTorture},
		{"graveyard", *graveyardStorm, runGraveyardStorm},
		{"oracle-diff", *oracleDiff, runOracleDiff},
		{"multiplayer", *multiplayerChaos, runMultiplayerChaos},
		{"infinite-loop", *infiniteLoop, runInfiniteLoop},
		{"rollback", *rollbackTest, runRollbackTorture},
		{"clock", *clockTest, runClockPressure},
		{"adversarial", *adversarialTest, runAdversarial},
		{"symmetry", *symmetryTest, runSymmetry},
		{"corpus-audit", *corpusAuditTest, runCorpusAudit},
		{"coverage-depth", *coverageDepth, runCoverageDepth},
		{"oracle-compliance", *oracleCompliance, runOracleCompliance},
		{"ast-fidelity", *astFidelity, runASTFidelity},
	}

	for _, mod := range modules {
		if !mod.enabled {
			continue
		}
		log.Printf("\n--- module: %s ---", mod.name)
		modStart := time.Now()
		modFails := mod.run(corpus, oracleCards)
		modElapsed := time.Since(modStart)

		mu.Lock()
		failures = append(failures, modFails...)
		mu.Unlock()

		modTests := len(modFails) // at minimum 1 test per failure
		// Count tests more accurately based on module type.
		switch mod.name {
		case "keyword-matrix":
			modTests = len(combatKeywords) * len(combatKeywords)
		case "combo-pairs":
			n := 0
			for _, name := range stapleNames {
				for _, oc := range oracleCards {
					if oc.Name == name {
						n++
						break
					}
				}
			}
			modTests = n * (n - 1) / 2 * 7 // pairs * phases
		case "replacement":
			modTests = 6 // number of replacement scenarios
		case "layer-stress":
			modTests = 6 // number of layer scenarios
		case "stack-torture":
			modTests = 6 // number of stack scenarios
		case "commander":
			modTests = 5 // number of commander scenarios
		case "apnap":
			modTests = 4 // number of APNAP scenarios
		case "zone-chains":
			modTests = 6 // number of zone chain scenarios
		case "mana-verify":
			modTests = 8 // number of mana scenarios
		case "turn-structure":
			modTests = 8 // number of turn structure scenarios
		case "spell-resolve":
			count := 0
			for _, oc := range oracleCards {
				if isInstantOrSorcery(oc) && oc.ast != nil {
					count++
				}
			}
			modTests = count
		case "goldilocks":
			count := 0
			for _, oc := range oracleCards {
				if oc.ast != nil {
					count++
				}
			}
			modTests = count
		case "advanced-mechanics":
			modTests = 145 // 15+10+10+10+15+10+10+15+15+10+10+15
		case "deep-rules":
			modTests = 100 // 8+8+6+6+6+8+6+4+4+4+4+4+4+4+4+4+4+4+4+4
		case "claim-verify":
			modTests = 60 // 15+5+5+10+5+5+5+5+5
		case "negative-legality":
			modTests = 40 // 8+8+8+8+8
		case "chaos":
			modTests = chaosGameCount * chaosMaxTurns * 7
		case "density":
			modTests = 8
		case "cascade":
			modTests = 8
		case "graveyard":
			modTests = 8
		case "oracle-diff":
			count := 0
			for _, oc := range oracleCards {
				if oc.ast != nil {
					count++
				}
			}
			modTests = count
		case "multiplayer":
			modTests = mpGameCount * mpMaxTurns * 2
		case "infinite-loop":
			modTests = 8
		case "rollback":
			modTests = 8
		case "clock":
			modTests = clockGameCount * clockMaxTurns
		case "adversarial":
			modTests = advGameCount * advMaxTurns
		case "symmetry":
			modTests = symGameCount
		case "corpus-audit":
			// Count auditable cards — each card may produce multiple tests.
			count := 0
			for _, oc := range oracleCards {
				if oc.ast != nil {
					count++
				}
			}
			modTests = count
		case "coverage-depth":
			count := 0
			for _, oc := range oracleCards {
				if oc.ast != nil {
					count++
				}
			}
			modTests = count
		case "oracle-compliance":
			modTests = 270
		case "ast-fidelity":
			count := 0
			for _, oc := range oracleCards {
				if oc.ast != nil {
					count++
				}
			}
			modTests = count
		}

		atomic.AddInt64(&totalTests, int64(modTests))
		atomic.AddInt64(&totalFails, int64(len(modFails)))

		log.Printf("  %s: %d tests, %d fails, %s", mod.name, modTests, len(modFails), modElapsed)
	}

	elapsed := time.Since(start)

	log.Printf("\n=== THOR COMPLETE ===")
	log.Printf("  cards tested:  %d", len(oracleCards))
	log.Printf("  total tests:   %d", totalTests)
	log.Printf("  failures:      %d", totalFails)
	log.Printf("  time:          %s", elapsed)
	if elapsed.Seconds() > 0 {
		log.Printf("  rate:          %.0f tests/s", float64(totalTests)/elapsed.Seconds())
	}

	if *reportPath != "" {
		writeReport(*reportPath, oracleCards, failures, elapsed, totalTests)
		log.Printf("  report:        %s", *reportPath)
	}

	// Print top failing cards.
	printTopFailures(failures)
}

func testCard(oc *oracleCard, ast *gameast.CardAST, interactions []interaction, withPhases bool) []failure {
	var fails []failure

	// 1. Interaction tests (destroy, exile, bounce, etc.)
	for _, inter := range interactions {
		f := testInteraction(oc, ast, inter)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	// 2. Phase transition tests
	if withPhases {
		pf := testPhases(oc, ast)
		fails = append(fails, pf...)
	}

	// 3. Spell resolution (instants/sorceries through the stack)
	if oc.ast != nil && isInstantOrSorcery(oc) {
		f := testSpellResolve(oc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	// 4. Effect verification + unverified detection (merged Goldilocks)
	if oc.ast != nil {
		info := extractFirstEffect(oc.ast)
		if info != nil && verifiableEffects[info.kind] {
			// Card has a verifiable effect — test it.
			f := testGoldilocksCard(oc)
			if f != nil {
				fails = append(fails, *f)
			}
		} else if info != nil && !verifiableEffects[info.kind] {
			// Card has a parsed effect but it's not in the verifiable set.
			// Only flag if the effect kind is meaningful (not just a simple
			// static modifier like "enters tapped").
			switch info.kind {
			case "with_modifier", "unknown":
				fails = append(fails, failure{
					CardName:    oc.Name,
					Interaction: "unverified",
					Message:     fmt.Sprintf("effect kind '%s' not in verifiable set", info.kind),
				})
			}
		} else if info == nil {
			// No extractable effect. Only flag if the card has triggered or
			// activated abilities (not just static keyword-grants).
			hasTriggeredOrActivated := false
			for _, ab := range oc.ast.Abilities {
				switch ab.(type) {
				case *gameast.Triggered, *gameast.Activated:
					hasTriggeredOrActivated = true
				}
				if hasTriggeredOrActivated {
					break
				}
			}
			if hasTriggeredOrActivated {
				fails = append(fails, failure{
					CardName:    oc.Name,
					Interaction: "unverified",
					Message:     "has triggered/activated abilities but no extractable effect",
				})
			}
		}
	}

	return fails
}

func testInteraction(oc *oracleCard, ast *gameast.CardAST, inter interaction) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			result = &failure{
				CardName:    oc.Name,
				Interaction: inter.Name,
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			}
		}
	}()

	gs := makeGameState(oc, ast)
	if gs == nil {
		return nil
	}

	perm := findCard(gs, oc.Name)
	if perm == nil {
		return nil
	}

	inter.Fn(gs, perm)

	gameengine.StateBasedActions(gs)

	violations := gameengine.RunAllInvariants(gs)
	if len(violations) > 0 {
		return &failure{
			CardName:    oc.Name,
			Interaction: inter.Name,
			Invariant:   violations[0].Name,
			Message:     violations[0].Message,
		}
	}

	return nil
}

func testPhases(oc *oracleCard, ast *gameast.CardAST) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			fails = append(fails, failure{
				CardName:    oc.Name,
				Interaction: "full_turn_cycle",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			})
		}
	}()

	gs := makeGameState(oc, ast)
	if gs == nil {
		return nil
	}

	phases := []struct {
		name string
		fn   func(gs *gameengine.GameState)
	}{
		{"untap", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "beginning", "untap"
			gameengine.UntapAll(gs, gs.Active)
		}},
		{"upkeep", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "beginning", "upkeep"
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
		{"draw", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "beginning", "draw"
			if len(gs.Seats[gs.Active].Library) > 0 {
				gs.Seats[gs.Active].Hand = append(gs.Seats[gs.Active].Hand,
					gs.Seats[gs.Active].Library[0])
				gs.Seats[gs.Active].Library = gs.Seats[gs.Active].Library[1:]
			}
			gameengine.StateBasedActions(gs)
		}},
		{"main1", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "precombat_main", ""
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
		{"combat", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "combat", "beginning_of_combat"
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
		{"main2", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "postcombat_main", ""
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
		{"end_step", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "ending", "end"
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
	}

	for _, p := range phases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fails = append(fails, failure{
						CardName:    oc.Name,
						Interaction: "phase_" + p.name,
						Panicked:    true,
						PanicMsg:    fmt.Sprintf("%v", r),
					})
				}
			}()

			p.fn(gs)

			violations := gameengine.RunAllInvariants(gs)
			for _, v := range violations {
				fails = append(fails, failure{
					CardName:    oc.Name,
					Interaction: "phase_" + p.name,
					Invariant:   v.Name,
					Message:     v.Message,
				})
			}
		}()
	}

	return fails
}

func makeGameState(oc *oracleCard, ast *gameast.CardAST) *gameengine.GameState {
	gs := &gameengine.GameState{
		Turn:   1,
		Active: 0,
		Phase:  "precombat_main",
		Step:   "",
		Flags:  map[string]int{},
	}

	// 4 seats with basic setup.
	for i := 0; i < 4; i++ {
		seat := &gameengine.Seat{
			Life:  40,
			Flags: map[string]int{},
		}
		// Library: 10 filler cards.
		for j := 0; j < 10; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name:      fmt.Sprintf("Filler %d-%d", i, j),
				Owner:     i,
				Types:     []string{"creature"},
				BasePower: 1, BaseToughness: 1,
			})
		}
		// Hand: 3 filler cards.
		for j := 0; j < 3; j++ {
			seat.Hand = append(seat.Hand, &gameengine.Card{
				Name:  fmt.Sprintf("HandCard %d-%d", i, j),
				Owner: i,
				Types: []string{"creature"},
			})
		}
		gs.Seats = append(gs.Seats, seat)
	}

	// Place the test card on seat 0's battlefield.
	types := oc.Types
	if len(types) == 0 {
		types = []string{"creature"}
	}
	// Skip non-creatures with 0/0 — instants, sorceries, split cards
	// that have no meaningful battlefield presence.
	isCreature := false
	for _, t := range types {
		if t == "creature" {
			isCreature = true
			break
		}
	}
	pow, tough := oc.Power, oc.Toughness
	if isCreature && tough <= 0 {
		tough = 1 // */* creatures default to at least 1/1 to avoid SBA false positives
		if pow <= 0 {
			pow = 1
		}
	}

	card := &gameengine.Card{
		Name:          oc.Name,
		Owner:         0,
		Types:         types,
		Colors:        oc.Colors,
		CMC:           oc.CMC,
		BasePower:     pow,
		BaseToughness: tough,
		AST:           ast,
	}
	perm := &gameengine.Permanent{
		Card:       card,
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	// Also place a vanilla creature on seat 1 for fight targets etc.
	opCard := &gameengine.Card{
		Name: "Opponent Bear", Owner: 1,
		Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
	}
	opPerm := &gameengine.Permanent{
		Card: opCard, Controller: 1, Owner: 1, Flags: map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, opPerm)

	gs.Snapshot()
	return gs
}

func findCard(gs *gameengine.GameState, name string) *gameengine.Permanent {
	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p.Card != nil && p.Card.Name == name {
				return p
			}
		}
	}
	return nil
}

func buildInteractions() []interaction {
	return []interaction{
		{"destroy", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			gameengine.DestroyPermanent(gs, perm, nil)
		}},
		{"exile", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			gameengine.ExilePermanent(gs, perm, nil)
		}},
		{"bounce", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			gameengine.BouncePermanent(gs, perm, nil, "hand")
		}},
		{"sacrifice", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			gameengine.SacrificePermanent(gs, perm, "thor_test")
		}},
		{"fight_mutual", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			opponent := findCard(gs, "Opponent Bear")
			if opponent == nil || !perm.IsCreature() {
				return
			}
			e := &gameast.Fight{
				A: gameast.Filter{Base: "creature", YouControl: true, Targeted: true},
				B: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
			}
			gameengine.ResolveEffect(gs, perm, e)
		}},
		{"target_by_opponent", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			// Simulate being targeted by an opponent spell.
			f := gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true}
			src := &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "Test Bolt", Colors: []string{"R"}, Types: []string{"instant"}},
				Controller: 1, Owner: 1,
			}
			gameengine.PickTarget(gs, src, f)
		}},
		{"damage_3", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			if perm.IsCreature() {
				perm.MarkedDamage += 3
			}
		}},
		{"damage_lethal", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			if perm.IsCreature() {
				perm.MarkedDamage += perm.Toughness() + 1
			}
		}},
		{"counter_mod_plus", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			if perm.Flags == nil {
				perm.Flags = map[string]int{}
			}
			perm.Flags["counter:+1/+1"] += 2
		}},
		{"counter_mod_minus", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			if perm.Flags == nil {
				perm.Flags = map[string]int{}
			}
			perm.Flags["counter:-1/-1"] += 2
		}},
		{"phase_out", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			perm.PhasedOut = true
		}},
		{"tap", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			perm.Tapped = true
		}},
		{"steal", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			// Move control from seat 0 to seat 1.
			if perm.Controller == 0 && len(gs.Seats) > 1 {
				oldBF := gs.Seats[0].Battlefield
				for i, p := range oldBF {
					if p == perm {
						gs.Seats[0].Battlefield = append(oldBF[:i], oldBF[i+1:]...)
						break
					}
				}
				perm.Controller = 1
				gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, perm)
			}
		}},
		{"flicker", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			// Exile then return (like Restoration Angel).
			gameengine.ExilePermanent(gs, perm, nil)
			gameengine.StateBasedActions(gs)
			// Return a copy to battlefield.
			card := perm.Card
			if card == nil {
				return
			}
			newPerm := &gameengine.Permanent{
				Card: card, Controller: card.Owner, Owner: card.Owner,
				Flags: map[string]int{},
			}
			gs.Seats[card.Owner].Battlefield = append(gs.Seats[card.Owner].Battlefield, newPerm)
			// Move from exile back (zone conservation).
			exile := gs.Seats[card.Owner].Exile
			for i, c := range exile {
				if c == card {
					gs.Seats[card.Owner].Exile = append(exile[:i], exile[i+1:]...)
					break
				}
			}
		}},
		{"clone", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			if perm.Card == nil {
				return
			}
			// Create a token copy.
			copyCard := &gameengine.Card{
				Name:          perm.Card.Name,
				Owner:         1,
				Types:         append([]string{"token"}, perm.Card.Types...),
				Colors:        perm.Card.Colors,
				CMC:           perm.Card.CMC,
				BasePower:     perm.Card.BasePower,
				BaseToughness: perm.Card.BaseToughness,
			}
			copyPerm := &gameengine.Permanent{
				Card: copyCard, Controller: 1, Owner: 1,
				Flags: map[string]int{},
			}
			gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, copyPerm)
		}},
	}
}

// === ORACLE LOADER ===

type oracleCard struct {
	Name       string
	TypeLine   string
	Types      []string
	Colors     []string
	CMC        int
	Power      int
	Toughness  int
	OracleText string
	ast        *gameast.CardAST
}

func loadOracleCards(path string) ([]*oracleCard, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []struct {
		Name       string   `json:"name"`
		Layout     string   `json:"layout"`
		TypeLine   string   `json:"type_line"`
		Colors     []string `json:"colors"`
		CMC        float64  `json:"cmc"`
		Power      string   `json:"power"`
		Toughness  string   `json:"toughness"`
		SetName    string   `json:"set_name"`
		OracleText string   `json:"oracle_text"`
		CardFaces  []struct {
			TypeLine   string `json:"type_line"`
			Power      string `json:"power"`
			Toughness  string `json:"toughness"`
			OracleText string `json:"oracle_text"`
		} `json:"card_faces"`
	}
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		return nil, err
	}

	unSets := map[string]bool{
		"Unstable": true, "Unhinged": true, "Unglued": true,
		"Unsanctioned": true, "Unfinity": true,
	}

	var cards []*oracleCard
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Name == "" || seen[e.Name] || unSets[e.SetName] || e.Layout == "token" {
			continue
		}
		seen[e.Name] = true

		typeLine := e.TypeLine
		if typeLine == "" && len(e.CardFaces) > 0 {
			typeLine = e.CardFaces[0].TypeLine
		}

		pow := parseIntSafe(e.Power)
		tough := parseIntSafe(e.Toughness)
		if (pow == 0 && tough == 0) && len(e.CardFaces) > 0 {
			pow = parseIntSafe(e.CardFaces[0].Power)
			tough = parseIntSafe(e.CardFaces[0].Toughness)
		}

		oracleText := e.OracleText
		if oracleText == "" && len(e.CardFaces) > 0 {
			oracleText = e.CardFaces[0].OracleText
		}

		cards = append(cards, &oracleCard{
			Name:       e.Name,
			TypeLine:   typeLine,
			Types:      parseTypes(typeLine),
			Colors:     e.Colors,
			CMC:        int(e.CMC),
			Power:      pow,
			Toughness:  tough,
			OracleText: oracleText,
		})
	}
	return cards, nil
}

func parseTypes(typeLine string) []string {
	tl := strings.ToLower(typeLine)
	var types []string
	for _, t := range []string{"creature", "artifact", "enchantment", "planeswalker", "instant", "sorcery", "land", "battle"} {
		if strings.Contains(tl, t) {
			types = append(types, t)
		}
	}
	// Add subtypes for creature type checks.
	if strings.Contains(tl, "—") {
		parts := strings.SplitN(tl, "—", 2)
		if len(parts) == 2 {
			for _, sub := range strings.Fields(strings.TrimSpace(parts[1])) {
				types = append(types, sub)
			}
		}
	}
	return types
}

func parseIntSafe(s string) int {
	if s == "" || s == "*" {
		return 0
	}
	n := 0
	neg := false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if neg {
		return -n
	}
	return n
}

// === REPORTING ===

func printTopFailures(failures []failure) {
	if len(failures) == 0 {
		fmt.Println("\n  ZERO FAILURES — fully sterile.")
		return
	}

	// Count by card.
	cardCount := map[string]int{}
	for _, f := range failures {
		cardCount[f.CardName]++
	}

	type cardFail struct {
		name  string
		count int
	}
	var sorted []cardFail
	for n, c := range cardCount {
		sorted = append(sorted, cardFail{n, c})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	fmt.Printf("\n  Top failing cards (%d unique):\n", len(sorted))
	for i, cf := range sorted {
		if i >= 20 {
			break
		}
		fmt.Printf("    %3d fails: %s\n", cf.count, cf.name)
	}

	// Count by interaction.
	interCount := map[string]int{}
	panicCount := 0
	for _, f := range failures {
		interCount[f.Interaction]++
		if f.Panicked {
			panicCount++
		}
	}
	fmt.Printf("\n  Failures by interaction:\n")
	var interSorted []cardFail
	for n, c := range interCount {
		interSorted = append(interSorted, cardFail{n, c})
	}
	sort.Slice(interSorted, func(i, j int) bool { return interSorted[i].count > interSorted[j].count })
	for _, ic := range interSorted {
		fmt.Printf("    %6d: %s\n", ic.count, ic.name)
	}
	fmt.Printf("\n  Panics: %d / %d total failures\n", panicCount, len(failures))

	// Count by invariant.
	invCount := map[string]int{}
	for _, f := range failures {
		if f.Invariant != "" {
			invCount[f.Invariant]++
		}
	}
	if len(invCount) > 0 {
		fmt.Printf("\n  Failures by invariant:\n")
		for inv, c := range invCount {
			fmt.Printf("    %6d: %s\n", c, inv)
		}
	}
}

func writeReport(path string, cards []*oracleCard, failures []failure, elapsed time.Duration, totalTests int64) {
	f, err := os.Create(path)
	if err != nil {
		log.Printf("WARNING: could not write report: %v", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# Thor — Per-Card Interaction Stress Test Report\n\n")
	fmt.Fprintf(f, "**Date:** %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "**Cards tested:** %d\n", len(cards))
	fmt.Fprintf(f, "**Total tests:** %d\n", totalTests)
	fmt.Fprintf(f, "**Failures:** %d\n", len(failures))
	fmt.Fprintf(f, "**Time:** %s\n", elapsed.Round(time.Second))
	fmt.Fprintf(f, "**Rate:** %.0f tests/s\n\n", float64(totalTests)/elapsed.Seconds())

	if len(failures) == 0 {
		fmt.Fprintf(f, "## Result: FULLY STERILE\n\nZero failures across all cards and interactions.\n")
		return
	}

	// Panics section.
	var panics []failure
	var invariantFails []failure
	for _, fl := range failures {
		if fl.Panicked {
			panics = append(panics, fl)
		} else {
			invariantFails = append(invariantFails, fl)
		}
	}

	if len(panics) > 0 {
		fmt.Fprintf(f, "## Panics (%d)\n\n", len(panics))
		fmt.Fprintf(f, "| Card | Interaction | Error |\n")
		fmt.Fprintf(f, "|------|-------------|-------|\n")
		for _, p := range panics {
			msg := p.PanicMsg
			msg = strings.ReplaceAll(msg, "\n", " ")
			msg = strings.ReplaceAll(msg, "|", "\\|")
			fmt.Fprintf(f, "| %s | %s | %s |\n", p.CardName, p.Interaction, msg)
		}
		fmt.Fprintln(f)
	}

	if len(invariantFails) > 0 {
		fmt.Fprintf(f, "## Invariant Violations (%d)\n\n", len(invariantFails))
		fmt.Fprintf(f, "| Card | Interaction | Invariant | Message |\n")
		fmt.Fprintf(f, "|------|-------------|-----------|--------|\n")
		limit := 200
		if len(invariantFails) < limit {
			limit = len(invariantFails)
		}
		for i := 0; i < limit; i++ {
			fl := invariantFails[i]
			msg := fl.Message
			if len(msg) > 120 {
				msg = msg[:120] + "..."
			}
			msg = strings.ReplaceAll(msg, "|", "\\|")
			fmt.Fprintf(f, "| %s | %s | %s | %s |\n", fl.CardName, fl.Interaction, fl.Invariant, msg)
		}
		if len(invariantFails) > 200 {
			fmt.Fprintf(f, "\n*... and %d more*\n", len(invariantFails)-200)
		}
		fmt.Fprintln(f)
	}

	// Summary by interaction.
	interCount := map[string]int{}
	for _, fl := range failures {
		interCount[fl.Interaction]++
	}
	fmt.Fprintf(f, "## Failures by Interaction\n\n")
	fmt.Fprintf(f, "| Interaction | Failures |\n")
	fmt.Fprintf(f, "|-------------|----------|\n")
	type kv struct {
		k string
		v int
	}
	var interSorted []kv
	for k, v := range interCount {
		interSorted = append(interSorted, kv{k, v})
	}
	sort.Slice(interSorted, func(i, j int) bool { return interSorted[i].v > interSorted[j].v })
	for _, ic := range interSorted {
		fmt.Fprintf(f, "| %s | %d |\n", ic.k, ic.v)
	}
}
