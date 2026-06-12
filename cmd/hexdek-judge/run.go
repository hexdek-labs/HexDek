package main

// run.go — the Hex Judge's standing-gate RUN MODE (r63 Tier-1
// completion: the Judge is whole; this turns it ON).
//
//	hexdek-judge --run [--games N] [--seed S] [--ast ...] [--oracle ...]
//	              [--baseline data/judge/judge-baseline.json]
//	              [--update-baseline]
//	hexdek-judge --write-ci-sample          # regenerate the committed CI sample
//
// Exercises ALL FIVE dimensions and exits non-zero on any violation NOT
// in the committed baseline:
//
//   - OUTCOME + PROGRESSION (scenario_driver mode): the corpus audits,
//     driven through the packages' exported check APIs over every card
//     in the given AST dataset. Deterministic — fingerprints are
//     per-card, so a single new divergence fails the gate.
//   - CONSERVATION + STATE-INTEGRITY + LEGALITY (inline mode): N seeded
//     chaos games with the InstanceID strict census, the full invariant
//     table, and the ride-along legality validator attached; every
//     violation flows through judge.LogViolation and is collected by a
//     registered sink. Game trajectories shift when the engine changes,
//     so fingerprints are CLASS-level (dimension|surface|name): a new
//     violation CLASS fails the gate; count drift within a known class
//     is reported but does not fail.
//
// The baseline is the known-residual set siblings are still burning
// down — committed in-repo (diffable) and refreshed via
// --update-baseline once a residual is fixed.

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/judge/outcome"
	"github.com/hexdek/hexdek/internal/judge/progression"
	"github.com/hexdek/hexdek/internal/tournament"
)

// judgeBaseline is the committed known-residual set. Fingerprint →
// count at capture time. Counts matter only for reporting; the gate
// fails on fingerprints absent from the map.
type judgeBaseline struct {
	// Comment is a human note about when/how the baseline was captured.
	Comment string `json:"comment"`
	// Fingerprints maps violation fingerprint → count at capture.
	Fingerprints map[string]int `json:"fingerprints"`
}

// runJudgeGate executes the full five-dimension run and returns the
// process exit code.
func runJudgeGate(astPath, oraclePath, baselinePath string, games int, seed int64, update bool) int {
	got := map[string]int{}

	// ---- scenario dimensions: OUTCOME + PROGRESSION corpus audits ----
	corpus, err := astload.Load(astPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge run: astload: %v\n", err)
		return 2
	}
	scenarioInScope := runScenarioAudits(corpus, got)

	// ---- inline dimensions: seeded chaos games ----
	meta, err := deckparser.LoadMetaFromJSONL(astPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge run: meta: %v\n", err)
		return 2
	}
	if err := meta.SupplementWithOracleJSON(oraclePath); err != nil {
		fmt.Fprintf(os.Stderr, "judge run: oracle P/T supplement: %v (continuing)\n", err)
	}
	chaosCorpus, err := gameengine.LoadChaosCorpusFromOracleJSON(oraclePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge run: oracle corpus: %v\n", err)
		return 2
	}
	runSeededGames(chaosCorpus, corpus, meta, games, seed, got)

	fmt.Printf("judge run: %d scenario checks in scope; %d seeded games; %d distinct violation fingerprints\n",
		scenarioInScope, games, len(got))

	// ---- baseline handling ----
	if update {
		bl := judgeBaseline{
			Comment:      "Hex Judge known-residual baseline. Regenerate with: go run ./cmd/hexdek-judge --run --update-baseline (same --ast/--oracle/--games/--seed as CI). The gate fails on fingerprints NOT in this set.",
			Fingerprints: got,
		}
		data, _ := json.MarshalIndent(bl, "", "  ")
		if err := os.WriteFile(baselinePath, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "judge run: write baseline: %v\n", err)
			return 2
		}
		fmt.Printf("judge run: baseline updated (%s, %d fingerprints)\n", baselinePath, len(got))
		return 0
	}

	baseline := judgeBaseline{Fingerprints: map[string]int{}}
	if data, err := os.ReadFile(baselinePath); err == nil {
		if err := json.Unmarshal(data, &baseline); err != nil {
			fmt.Fprintf(os.Stderr, "judge run: baseline parse: %v\n", err)
			return 2
		}
	} else {
		fmt.Fprintf(os.Stderr, "judge run: no baseline at %s — treating every violation as new\n", baselinePath)
	}

	var newFps, knownFps, fixedFps []string
	for fp := range got {
		if _, ok := baseline.Fingerprints[fp]; ok {
			knownFps = append(knownFps, fp)
		} else {
			newFps = append(newFps, fp)
		}
	}
	for fp := range baseline.Fingerprints {
		if _, ok := got[fp]; !ok {
			fixedFps = append(fixedFps, fp)
		}
	}
	sort.Strings(newFps)
	sort.Strings(knownFps)
	sort.Strings(fixedFps)

	for _, fp := range knownFps {
		fmt.Printf("  KNOWN  %s (count %d, baseline %d)\n", fp, got[fp], baseline.Fingerprints[fp])
	}
	for _, fp := range fixedFps {
		fmt.Printf("  FIXED? %s (in baseline, not seen — refresh the baseline to lock in)\n", fp)
	}
	for _, fp := range newFps {
		fmt.Printf("  NEW    %s (count %d)\n", fp, got[fp])
	}

	if len(newFps) > 0 {
		fmt.Printf("judge run: FAIL — %d NEW violation fingerprint(s) not in baseline %s\n", len(newFps), baselinePath)
		return 1
	}
	fmt.Printf("judge run: PASS — no new violations (%d known residuals, %d possibly fixed)\n", len(knownFps), len(fixedFps))
	return 0
}

// runScenarioAudits walks the corpus through the OUTCOME and PROGRESSION
// exported check APIs (the same walks as their committed audit tests),
// recording per-card fingerprints. Returns the number of in-scope checks.
func runScenarioAudits(corpus *astload.Corpus, got map[string]int) int {
	inScope := 0
	for _, name := range corpus.Names() {
		ast, _ := corpus.Get(name)
		if ast == nil {
			continue
		}
		for _, ab := range ast.Abilities {
			// OUTCOME phase-3 replacement arm: enters-with-counters.
			if st, ok := ab.(*gameast.Static); ok && st.Modification != nil {
				if fd, ran := outcome.CheckETBCounters(name, st.Modification); ran {
					inScope++
					if fd != nil {
						got["outcome_etbc|"+name]++
					}
				}
			}
			// PROGRESSION trigger checks.
			if tr, ok := ab.(*gameast.Triggered); ok {
				var findings []*progression.Finding
				var ran bool
				if findings, ran = progression.CheckTrigger(name, tr); !ran {
					if findings, ran = progression.CheckPhaseTrigger(name, tr); !ran {
						findings, ran = progression.CheckLTBTrigger(name, tr)
					}
				}
				if ran {
					inScope++
					for _, fd := range findings {
						got["progression|"+name+"|"+fd.Event+"|"+fd.Check]++
					}
				}
			}
		}
		// OUTCOME effect checks.
		for _, ex := range outcome.ExtractEffects(ast) {
			finding, ran := outcome.RunEffect(name, ex.Raw, ex.Effect)
			if !ran {
				continue
			}
			inScope++
			if finding != nil {
				got["outcome|"+name+"|"+finding.Kind]++
			}
		}
	}
	return inScope
}

// runSeededGames plays N deterministic chaos games with every inline
// Judge surface armed, collecting violations class-level through the
// router sink. Mirrors loki's runChaosGame loop (deck seed formula
// included) without the report machinery.
func runSeededGames(chaosCorpus *gameengine.ChaosCorpus, corpus *astload.Corpus,
	meta *deckparser.MetaDB, games int, masterSeed int64, got map[string]int) {

	gameengine.SetStrictCensusDefault(true)
	defer gameengine.SetStrictCensusDefault(false)

	unregister := judge.RegisterSink(func(v judge.ValidationViolation) {
		dim := v.Dimension
		if dim == "" {
			dim = "untagged"
		}
		got["game|"+dim+"|"+v.Surface+"|"+v.Name]++
	})
	defer unregister()

	const nSeats = 4
	const maxTurns = 60
	for g := 0; g < games; g++ {
		runOneSeededGame(chaosCorpus, corpus, meta, g, masterSeed, nSeats, maxTurns, got)
	}
}

func runOneSeededGame(chaosCorpus *gameengine.ChaosCorpus, corpus *astload.Corpus,
	meta *deckparser.MetaDB, gameIdx int, masterSeed int64, nSeats, maxTurns int,
	got map[string]int) {

	defer func() {
		if r := recover(); r != nil {
			got[fmt.Sprintf("game|panic|%v", r)]++
		}
	}()

	// Same seed math as loki's runChaosGame so repros transfer.
	deckSeed := masterSeed + int64(gameIdx)*10000 + 1
	shuffleSeed := deckSeed + 7
	deckRng := rand.New(rand.NewSource(deckSeed))
	gameRng := rand.New(rand.NewSource(shuffleSeed))

	decks := make([]*gameengine.ChaosDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		decks[i] = gameengine.GenerateChaosDeck(chaosCorpus, deckRng)
		if decks[i] == nil {
			got["game|harness|deck_generation_failed"]++
			return
		}
	}

	gs := gameengine.NewGameState(nSeats, gameRng, corpus)
	gs.Seed = deckSeed
	gs.Legality = gameengine.NewLegalityValidator(deckSeed)

	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i, cd := range decks {
		cmdr := deckparser.BuildCardFromName(cd.Commander.Name, corpus, meta)
		if cmdr == nil {
			cmdr = &gameengine.Card{
				Name: cd.Commander.Name, Owner: i,
				Types:     []string{"legendary", "creature"},
				BasePower: cd.Commander.Power, BaseToughness: cd.Commander.Toughness,
				CMC: cd.Commander.CMC, Colors: cd.Commander.Colors,
			}
			if cmdr.BaseToughness == 0 {
				cmdr.BaseToughness = 1
			}
		} else {
			cmdr.Owner = i
		}
		lib := make([]*gameengine.Card, 0, len(cd.Cards))
		for _, name := range cd.Cards {
			c := deckparser.BuildCardFromName(name, corpus, meta)
			if c == nil {
				c = &gameengine.Card{Name: name, Owner: i}
				for _, cc := range chaosCorpus.All {
					if cc.Name == name {
						c.Types = cc.Types
						c.BasePower = cc.Power
						c.BaseToughness = cc.Toughness
						c.CMC = cc.CMC
						c.Colors = cc.Colors
						break
					}
				}
			} else {
				c.Owner = i
			}
			lib = append(lib, c)
		}
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		commanderDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: []*gameengine.Card{cmdr},
			Library:        lib,
		}
	}

	gameengine.SetupCommanderGame(gs, commanderDecks)
	for i := 0; i < nSeats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}
	for i := 0; i < nSeats; i++ {
		for j := 0; j < 7 && len(gs.Seats[i].Library) > 0; j++ {
			c := gs.Seats[i].Library[0]
			gs.Seats[i].Library = gs.Seats[i].Library[1:]
			gs.Seats[i].Hand = append(gs.Seats[i].Hand, c)
		}
	}
	gs.Active = gameRng.Intn(nSeats)
	gs.Turn = 1

	gameengine.RunAllInvariants(gs)
	for turn := 1; turn <= maxTurns; turn++ {
		gs.Turn = turn
		func() {
			defer func() {
				if r := recover(); r != nil {
					got[fmt.Sprintf("game|panic|turn:%v", r)]++
				}
			}()
			tournament.TakeTurn(gs)
		}()
		gameengine.RunAllInvariants(gs)
		gameengine.StateBasedActions(gs)
		gameengine.RunAllInvariants(gs)
		if gs.CheckEnd() {
			break
		}
		gs.Active = gameengine.NextLivingSeat(gs)
	}
}

// writeCISample builds the committed representative sample used by the
// CI gate: an AST slice (every card in OUTCOME/PROGRESSION scope, capped
// deterministically) + the matching oracle slice plus a chaos deck pool
// (legendary creatures, nonland spread, lands). Re-run after corpus
// refreshes; the baseline must be regenerated alongside.
func writeCISample(astPath, oraclePath, outAST, outOracle string, scopeCap int) error {
	corpus, err := astload.Load(astPath)
	if err != nil {
		return fmt.Errorf("astload: %v", err)
	}

	names := corpus.Names()
	sort.Strings(names)
	want := map[string]bool{}
	scoped := 0
	for _, name := range names {
		if scoped >= scopeCap {
			break
		}
		ast, _ := corpus.Get(name)
		if ast == nil {
			continue
		}
		inScope := false
		for _, ab := range ast.Abilities {
			if st, ok := ab.(*gameast.Static); ok && st.Modification != nil {
				if _, ran := outcome.CheckETBCounters(name, st.Modification); ran {
					inScope = true
				}
			}
			if tr, ok := ab.(*gameast.Triggered); ok {
				if _, ok := progression.InScopeTrigger(tr); ok {
					inScope = true
				}
			}
		}
		if !inScope {
			for _, ex := range outcome.ExtractEffects(ast) {
				if _, ran := outcome.RunEffect(name, ex.Raw, ex.Effect); ran {
					inScope = true
					break
				}
			}
		}
		if inScope {
			want[name] = true
			scoped++
		}
	}

	// Chaos deck pool from the oracle corpus: deterministic prefixes of
	// each category (sorted by name).
	chaosCorpus, err := gameengine.LoadChaosCorpusFromOracleJSON(oraclePath)
	if err != nil {
		return fmt.Errorf("oracle corpus: %v", err)
	}
	addCat := func(cards []*gameengine.ChaosCard, n int) {
		names := make([]string, 0, len(cards))
		for _, c := range cards {
			names = append(names, c.Name)
		}
		sort.Strings(names)
		for i := 0; i < n && i < len(names); i++ {
			want[names[i]] = true
		}
	}
	addCat(chaosCorpus.LegendaryCreatures, 24)
	addCat(chaosCorpus.NonLand, 300)
	addCat(chaosCorpus.NonBasicLands, 40)
	for _, basic := range []string{"Plains", "Island", "Swamp", "Mountain", "Forest", "Wastes"} {
		want[basic] = true
	}

	// AST sample: copy matching JSONL lines verbatim (manifest first).
	if err := filterJSONLByName(astPath, outAST, want); err != nil {
		return err
	}
	// Oracle sample: copy matching entries verbatim.
	if err := filterOracleByName(oraclePath, outOracle, want); err != nil {
		return err
	}
	fmt.Printf("judge ci-sample: %d names selected (%d scenario-scoped) → %s + %s\n",
		len(want), scoped, outAST, outOracle)
	return nil
}

func filterJSONLByName(in, out string, want map[string]bool) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	var outLines []string
	for i, line := range splitLines(string(data)) {
		if len(line) == 0 {
			continue
		}
		var probe struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(line), &probe); err != nil {
			continue
		}
		if i == 0 && probe.Name == "" {
			outLines = append(outLines, line) // dataset manifest header
			continue
		}
		if want[probe.Name] {
			outLines = append(outLines, line)
		}
	}
	return os.WriteFile(out, []byte(joinLines(outLines)), 0o644)
}

func filterOracleByName(in, out string, want map[string]bool) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	var kept []json.RawMessage
	for _, e := range entries {
		var probe struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(e, &probe); err != nil {
			continue
		}
		if want[probe.Name] {
			kept = append(kept, e)
		}
	}
	outData, err := json.Marshal(kept)
	if err != nil {
		return err
	}
	return os.WriteFile(out, outData, 0o644)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func joinLines(lines []string) string {
	res := ""
	for _, l := range lines {
		res += l + "\n"
	}
	return res
}
