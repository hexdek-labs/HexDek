// validate-composition-prior runs the PR #408 prior-wired TrueSkill
// against vanilla TrueSkill on a synthetic 5-pod gauntlet to measure
// whether the composition prior improves prediction of next-game
// outcomes.
//
// Methodology:
//  1. Bootstrap: 200 games per pod (1000 total) feed the prior ONLY.
//     Neither TrueSkill system sees these games — they only seed the
//     prior's pairwise + archetype baseline tables.
//  2. Test: 100 games per pod (500 total). For each game:
//     a. Predict winner using each system's current ratings
//        (argmax μ) and compute log-loss vs. softmax(μ/σ).
//     b. Run the game (sample outcome from the synthetic true-rate
//        distribution for that pod).
//     c. Update both TrueSkill systems with the actual rank order.
//  3. Report: top-1 accuracy + cumulative log-loss for both systems.
//
// Per-pod outcomes are drawn from hand-crafted "true winrate"
// distributions that mirror the meta-study (PR #322) findings —
// each pod has a dominant archetype with ~40-65% winrate and 2-3
// supporting archetypes that share the remainder.
package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"

	"github.com/hexdek/hexdek/internal/trueskill"
)

// pod describes one 4-archetype composition with synthetic
// per-archetype true winrates that sum to 1.0.
type pod struct {
	name      string
	decks     [4]string // deck identifiers for TS rating keys
	archetypes [4]string
	trueRates [4]float64 // P(deck i wins) — sums to 1
}

// pods mirror the meta-study compositions with hand-crafted true
// rates that reflect the meta-study's observed dominance patterns.
var pods = []pod{
	{
		name:       "C1_mill_voltron_spell_lands",
		decks:      [4]string{"Phenax", "Wyleth", "Kalamax", "Windgrace"},
		archetypes: [4]string{"Mill", "Voltron", "Spellslinger", "LandsMatter"},
		trueRates:  [4]float64{0.65, 0.05, 0.15, 0.15},
	},
	{
		name:       "C2_aristo_reani_tribal_life",
		decks:      [4]string{"Korvold", "Meren", "EdgarMarkov", "Heliod"},
		archetypes: [4]string{"Aristocrats", "Reanimator", "Tribal", "Lifegain"},
		trueRates:  [4]float64{0.45, 0.20, 0.20, 0.15},
	},
	{
		name:       "C3_self_ench_artif_super",
		decks:      [4]string{"Sidisi", "Sythis", "Breya", "Atraxa"},
		archetypes: [4]string{"Selfmill", "Enchantress", "Artifacts", "Superfriends"},
		trueRates:  [4]float64{0.20, 0.15, 0.20, 0.45},
	},
	{
		name:       "C4_hug_counters_combo_extra",
		decks:      [4]string{"KynaiosTiro", "Ezuri", "Kenrith", "Najeela"},
		archetypes: [4]string{"GroupHug", "CountersMatter", "Combo", "ExtraCombats"},
		trueRates:  [4]float64{0.15, 0.15, 0.50, 0.20},
	},
	{
		name:       "C5_lands_blink_reani_stax",
		decks:      [4]string{"Aesi", "Aminatou", "Karador", "Teferi"},
		archetypes: [4]string{"LandsMatter", "Blink", "Reanimator", "Stax"},
		trueRates:  [4]float64{0.40, 0.20, 0.20, 0.20},
	},
}

// sampleWinner draws an outcome from the pod's trueRates distribution.
func sampleWinner(p pod, rng *rand.Rand) int {
	r := rng.Float64()
	cum := 0.0
	for i, rate := range p.trueRates {
		cum += rate
		if r < cum {
			return i
		}
	}
	return 3 // last seat fallback
}

// ranksFromWinner builds a TrueSkill rank array where the winner is
// rank 0 and other seats split rank 1 (no fine-grained ordering
// observable from synthetic data, so all losers are tied).
func ranksFromWinner(winner int, n int) []int {
	ranks := make([]int, n)
	for i := range ranks {
		if i == winner {
			ranks[i] = 0
		} else {
			ranks[i] = 1
		}
	}
	return ranks
}

// effectiveMu returns the pod-conditioned strength for each deck.
// For the prior-aware system, this is raw μ + composition offset
// (the same offset UpdateWithComposition applies during training).
// For the standard system, it's just raw μ.
//
// The prior's μ is stored as "skill modulo composition" — the
// offsets are baked out during training so the rating represents
// what the deck would achieve in an average pod. To compare two
// decks in a SPECIFIC pod, the offset must be added back.
func effectiveMu(ts *trueskill.TrueSkillRatings, decks [4]string, archs [4]string, prior *trueskill.CompositionPrior, weight, scale float64) [4]float64 {
	out := [4]float64{}
	archSlice := []string{archs[0], archs[1], archs[2], archs[3]}
	for i, d := range decks {
		out[i] = ts.Ratings[d].Mu
		if prior != nil && weight > 0 {
			conf := prior.Confidence(archs[i], archSlice)
			exp := prior.ExpectedWinrate(archs[i], archSlice)
			out[i] += weight * conf * scale * (exp - 0.25)
		}
	}
	// Use softmax temperature β ≈ σ (8.33) for Bradley-Terry style
	// calibration so softmax over μ produces reasonable probabilities.
	const beta = 8.33
	for i := range out {
		out[i] /= beta
	}
	return out
}

// predictedWinner returns argmax(effective μ) over the participant
// set. Ties broken by seat order.
func predictedWinner(effMu [4]float64) int {
	bestIdx, bestMu := 0, math.Inf(-1)
	for i, mu := range effMu {
		if mu > bestMu {
			bestMu = mu
			bestIdx = i
		}
	}
	return bestIdx
}

// logLoss returns -log(softmax_p(actualWinner)) over the effective
// μ values, after the β temperature applied in effectiveMu.
func logLoss(effMu [4]float64, actual int) float64 {
	maxMu := math.Inf(-1)
	for _, mu := range effMu {
		if mu > maxMu {
			maxMu = mu
		}
	}
	sumExp := 0.0
	for _, mu := range effMu {
		sumExp += math.Exp(mu - maxMu)
	}
	actualProb := math.Exp(effMu[actual]-maxMu) / sumExp
	if actualProb < 1e-15 {
		actualProb = 1e-15
	}
	return -math.Log(actualProb)
}

// runValidation runs the bootstrap + test phases and returns the
// per-system top-1 accuracy and cumulative log-loss across the test
// phase.
type result struct {
	totalTests       int
	stdTop1          int
	priorTop1        int
	stdLogLossTotal  float64
	priorLogLossTotal float64
	perPod           map[string]podResult
}
type podResult struct {
	games           int
	stdTop1         int
	priorTop1       int
	stdLogLoss      float64
	priorLogLoss    float64
}

func runValidation(bootstrapGames, testGames int, seed int64) result {
	rng := rand.New(rand.NewSource(seed))

	prior := trueskill.NewCompositionPrior(4)

	// Bootstrap phase: prior learns archetype patterns from N games
	// per pod. The two TS systems are NOT updated in this phase —
	// they enter the test phase cold.
	for _, p := range pods {
		for i := 0; i < bootstrapGames; i++ {
			winnerIdx := sampleWinner(p, rng)
			podArch := []string{p.archetypes[0], p.archetypes[1], p.archetypes[2], p.archetypes[3]}
			_ = prior.ObserveGame(podArch, p.archetypes[winnerIdx])
		}
	}

	// Collect unique deck names across all pods for the TS init.
	deckSet := map[string]bool{}
	for _, p := range pods {
		for _, d := range p.decks {
			deckSet[d] = true
		}
	}
	deckNames := make([]string, 0, len(deckSet))
	for d := range deckSet {
		deckNames = append(deckNames, d)
	}
	sort.Strings(deckNames)

	stdTS := trueskill.NewTrueSkillRatings(deckNames)
	priorTS := trueskill.NewTrueSkillRatings(deckNames)
	priorCfg := trueskill.CompositionUpdateConfig{
		Prior:         prior,
		Weight:        0.5,
		MuOffsetScale: 10.0,
	}

	res := result{perPod: map[string]podResult{}}

	// Test phase: each game we predict-then-observe-then-update.
	for _, p := range pods {
		pr := podResult{}
		decks := p.decks
		archs := []string{p.archetypes[0], p.archetypes[1], p.archetypes[2], p.archetypes[3]}
		participants := []string{decks[0], decks[1], decks[2], decks[3]}
		archsArr := [4]string{archs[0], archs[1], archs[2], archs[3]}
		for i := 0; i < testGames; i++ {
			// Compute effective μ (with composition offset for prior-aware)
			stdEff := effectiveMu(stdTS, decks, archsArr, nil, 0, 0)
			priorEff := effectiveMu(priorTS, decks, archsArr, prior,
				priorCfg.Weight, priorCfg.MuOffsetScale)
			// Predict first
			stdPred := predictedWinner(stdEff)
			priorPred := predictedWinner(priorEff)
			// Sample actual outcome
			actual := sampleWinner(p, rng)
			// Score
			pr.games++
			if stdPred == actual {
				pr.stdTop1++
				res.stdTop1++
			}
			if priorPred == actual {
				pr.priorTop1++
				res.priorTop1++
			}
			sll := logLoss(stdEff, actual)
			pll := logLoss(priorEff, actual)
			pr.stdLogLoss += sll
			pr.priorLogLoss += pll
			res.stdLogLossTotal += sll
			res.priorLogLossTotal += pll
			res.totalTests++
			// Update both systems
			ranks := ranksFromWinner(actual, 4)
			stdTS.Update(participants, ranks)
			priorTS.UpdateWithComposition(participants, ranks, archs, priorCfg)
		}
		res.perPod[p.name] = pr
	}
	return res
}

func main() {
	bootstrap := flag.Int("bootstrap", 200, "games per pod to seed the prior")
	test := flag.Int("test", 100, "test games per pod (prediction + update)")
	seed := flag.Int64("seed", 42, "RNG seed")
	flag.Parse()
	res := runValidation(*bootstrap, *test, *seed)

	fmt.Printf("# Composition Prior Validation — %d bootstrap + %d test games per pod\n\n", *bootstrap, *test)
	fmt.Printf("Seed: %d\n\n", *seed)
	fmt.Printf("## Aggregate (across 5 pods, %d test games total)\n\n", res.totalTests)
	fmt.Printf("| Metric | Standard TrueSkill | Prior-aware TrueSkill | Improvement |\n")
	fmt.Printf("|---|---:|---:|---:|\n")
	stdAcc := float64(res.stdTop1) / float64(res.totalTests) * 100
	priorAcc := float64(res.priorTop1) / float64(res.totalTests) * 100
	fmt.Printf("| Top-1 accuracy | %.1f%% | %.1f%% | %+.1f pp |\n",
		stdAcc, priorAcc, priorAcc-stdAcc)
	stdLL := res.stdLogLossTotal / float64(res.totalTests)
	priorLL := res.priorLogLossTotal / float64(res.totalTests)
	fmt.Printf("| Mean log-loss | %.4f | %.4f | %+.4f |\n",
		stdLL, priorLL, stdLL-priorLL)

	fmt.Printf("\n## Per-pod breakdown\n\n")
	fmt.Printf("| Pod | Games | Std acc | Prior acc | Std LL | Prior LL |\n")
	fmt.Printf("|---|---:|---:|---:|---:|---:|\n")
	for _, p := range pods {
		pr := res.perPod[p.name]
		fmt.Printf("| %s | %d | %.1f%% | %.1f%% | %.3f | %.3f |\n",
			p.name, pr.games,
			float64(pr.stdTop1)/float64(pr.games)*100,
			float64(pr.priorTop1)/float64(pr.games)*100,
			pr.stdLogLoss/float64(pr.games),
			pr.priorLogLoss/float64(pr.games))
	}

	if priorLL >= stdLL && priorAcc <= stdAcc {
		fmt.Printf("\nVERDICT: Prior did NOT improve prediction.\n")
		os.Exit(1)
	}
	fmt.Printf("\nVERDICT: Prior improved prediction.\n")
}
