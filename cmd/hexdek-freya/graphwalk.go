package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// FindLongLoops -- graph-walk detector for 5..7 card combo cycles.
//
// Brute-force enumeration scales as O(n^k); the pair/triple/quad path uses
// it directly because C(n,4) is bounded by the 70-candidate cap (see
// docs/freya-4card-runtime.md). Extending the same approach to 5+ cards
// hits a wall: C(70,5) * 120 perms is two orders of magnitude worse than
// the quad worst case. The graph-walk approach below replaces the
// exhaustive C(n,k) enumeration with depth-bounded DFS that:
//
//   1. Buckets candidates by INTERACTION THEME (mana_positive, untap,
//      token_chain, damage_chain, etb_blink, graveyard_loop, card_draw).
//      Only nodes that share a theme can form a meaningful cycle — a
//      mana-positive ramp piece and an ETB-blink engine almost never
//      participate in the same combo loop unless one of them also
//      satisfies a bridging theme.
//
//   2. Within each theme bucket, builds a directed adjacency where edge
//      a -> b exists iff resourceOverlap(a.Produces, b.Consumes) is
//      non-empty AND passes isInterestingLoop (the same gate the pair /
//      triple / quad detectors use).
//
//   3. Runs depth-bounded DFS from each start node, with the Johnson
//      cycle-enumeration prune (only visit indices >= start, so each
//      cycle's lex-minimum representative is visited exactly once at
//      the bucket level).
//
//   4. On cycle closure (next == start AND path length >= 5), emits a
//      ComboResult through the existing classifyLoop / cyclingCardsInCombo
//      pipeline so the loop-type heuristic and downstream consumers see
//      uniform results regardless of cycle length.
//
//   5. Caps per-bucket node count at graphWalkBucketCap and total
//      emitted combos at graphWalkMaxCombos to bound worst-case wall
//      time. Buckets that exceed the cap are skipped silently — the
//      pair/triple/quad detectors already cover the dominant
//      interactions within such dense theme clusters.
//
// Output is deduplicated against the canonical rotation+reversal of
// each cycle's name sequence so the same combo isn't reported per
// starting node or per traversal direction.
// ---------------------------------------------------------------------------

const (
	graphWalkMinLength = 5
	graphWalkMaxLength = 7
	// Global candidate cap. The whole graph is walked, not per-bucket —
	// real combos cross theme boundaries (a Heliod/Ballista damage chain
	// extended by tokens + blink touches 3+ themes). Theme membership
	// filters NODES (must have >= 1 theme), not cycles.
	graphWalkCandidateCap = 30
	graphWalkMaxCombos    = 50
)

// ThemeBucket categorizes cards by combo interaction theme. A 5-7 card
// cycle is only walked when every node in the cycle shares at least one
// theme — without this filter the depth-7 DFS at N=70 is computationally
// infeasible (worst case ~8B node visits).
type ThemeBucket string

const (
	ThemeManaPositive  ThemeBucket = "mana_positive"
	ThemeUntap         ThemeBucket = "untap"
	ThemeTokenChain    ThemeBucket = "token_chain"
	ThemeDamageChain   ThemeBucket = "damage_chain"
	ThemeETBBlink      ThemeBucket = "etb_blink"
	ThemeGraveyardLoop ThemeBucket = "graveyard_loop"
	ThemeCardDraw      ThemeBucket = "card_draw"
)

// themeBucketsFor returns the interaction themes a card participates in.
// A card may appear in multiple buckets (e.g. a token-producer that also
// drains life). The bucket assignment is the fingerprint pre-filter that
// keeps the graph walk tractable.
func themeBucketsFor(p CardProfile) map[ThemeBucket]bool {
	out := map[ThemeBucket]bool{}
	if containsRes(p.Produces, ResMana) || containsRes(p.Consumes, ResMana) || p.IsManaPayoff {
		out[ThemeManaPositive] = true
	}
	if containsRes(p.Produces, ResUntap) || containsRes(p.Consumes, ResUntap) {
		out[ThemeUntap] = true
	}
	if containsRes(p.Produces, ResToken) || containsRes(p.Consumes, ResToken) ||
		p.IsMassTokens || p.MakesInfiniteTokens {
		out[ThemeTokenChain] = true
	}
	if containsRes(p.Produces, ResDamage) || containsRes(p.Consumes, ResDamage) ||
		p.HasETBDamage || p.HasDeathDrain || p.IsDamageDoubler ||
		p.CounterToDamage || p.DamageToCounter || p.IsXBurn {
		out[ThemeDamageChain] = true
	}
	if p.IsBlinker || p.HasValueETB || p.HasManaETB {
		out[ThemeETBBlink] = true
	}
	if containsRes(p.Produces, ResGraveyard) || containsRes(p.Consumes, ResGraveyard) ||
		containsRes(p.Produces, ResReanimate) || containsRes(p.Consumes, ResReanimate) ||
		containsRes(p.Produces, ResGraveyardFill) || containsRes(p.Consumes, ResGraveyardFill) ||
		p.IsRecursion {
		out[ThemeGraveyardLoop] = true
	}
	if containsRes(p.Produces, ResCard) || containsRes(p.Consumes, ResCard) {
		out[ThemeCardDraw] = true
	}
	return out
}

// FindLongLoops walks the deck's combo graph for cycles of length 5..7.
// See the file header comment for the algorithm. Returns at most
// graphWalkMaxCombos unique cycles.
func FindLongLoops(profiles []CardProfile) []ComboResult {
	// Prefilter to flow-active candidates that participate in at least
	// one combo theme. Themeless flow-active cards (e.g. life-loop
	// fragments) are dropped: they can't anchor a 5+ card combo without
	// the bridging theme buckets, and including them only inflates the
	// candidate count toward the cap. Mirrors the quad path's
	// flow-active prefilter and adds the theme gate as the aggressive
	// fingerprint pre-filter for the longer walks.
	//
	// Cycling explosion is constrained at the candidate level: a single
	// representative cycling card is enough; additional cycling cards
	// only inflate node counts without surfacing new loop shapes.
	var candidates []CardProfile
	cyclingKept := 0
	const maxCyclingCandidates = 1
	for _, p := range profiles {
		if len(p.Produces) == 0 || len(p.Consumes) == 0 {
			continue
		}
		if len(themeBucketsFor(p)) == 0 {
			continue
		}
		if p.HasCycling {
			if cyclingKept >= maxCyclingCandidates {
				continue
			}
			cyclingKept++
		}
		candidates = append(candidates, p)
	}
	if len(candidates) < graphWalkMinLength {
		return nil
	}
	if len(candidates) > graphWalkCandidateCap {
		// Pool too dense for the depth-7 walk. The pair/triple/quad
		// detectors already cover the dominant interactions; skip the
		// long walk to keep wall time bounded. The skip is silent —
		// a deck dense enough to exceed the cap is already heavily
		// over-covered by the smaller-cycle results.
		return nil
	}

	n := len(candidates)
	// Build directed adjacency. Edge x -> y iff x.Produces overlaps
	// y.Consumes (ANY overlap). The "interesting resource" gate is
	// applied cycle-level at closure (mirroring the pair detector's
	// OR-over-edges semantics) — a 5-cycle with counter / damage / life
	// edges is real as long as at least one edge runs through an
	// interesting resource that makes the loop repeatable.
	adj := make([][]int, n)
	edgeRes := make(map[[2]int][]ResourceType, n*n)
	for x := 0; x < n; x++ {
		for y := 0; y < n; y++ {
			if x == y {
				continue
			}
			overlap := resourceOverlap(candidates[x].Produces, candidates[y].Consumes)
			if len(overlap) == 0 {
				continue
			}
			adj[x] = append(adj[x], y)
			edgeRes[[2]int{x, y}] = overlap
		}
	}

	seen := map[string]bool{}
	var results []ComboResult
	visited := make([]bool, n)
	path := make([]int, 0, graphWalkMaxLength)
	for start := 0; start < n; start++ {
		if len(results) >= graphWalkMaxCombos {
			break
		}
		visited[start] = true
		path = append(path, start)
		// ids is identity here — the walker uses path indices directly
		// against `candidates`.
		walkCycles(candidates, adj, edgeRes, start, start,
			visited, path, seen, &results)
		path = path[:0]
		visited[start] = false
	}
	return results
}

// walkCycles is the depth-bounded DFS. The Johnson prune (next >= start)
// ensures each undirected cycle's lex-minimum starting node is the only
// one that emits it — combined with the canonicalCycleKey dedup, this
// prevents the same cycle being reported once per rotation or direction.
func walkCycles(candidates []CardProfile,
	adj [][]int, edgeRes map[[2]int][]ResourceType,
	start, current int,
	visited []bool, path []int,
	seen map[string]bool, results *[]ComboResult) {

	if len(*results) >= graphWalkMaxCombos {
		return
	}

	for _, next := range adj[current] {
		// Cycle closure.
		if next == start {
			if len(path) < graphWalkMinLength {
				continue
			}
			cards := make([]CardProfile, len(path))
			for i, idx := range path {
				cards[i] = candidates[idx]
			}
			// Cycling coalesce: same defense as pair/triple/quad
			// (issue #523). Without this, dense cycling decks would
			// surface ~thousands of 5-7 card pseudo-loops.
			if cyclingCardsInCombo(cards...) >= 2 {
				continue
			}
			// Cycle-level interesting-resource gate: at least one edge
			// in the cycle must run through an interesting resource
			// (mana / token / card / graveyard / untap / land /
			// graveyard_fill / reanimate). Mirrors the pair detector's
			// OR-over-edges semantics — a loop made entirely of life /
			// counter / damage / landfall edges isn't a real combo.
			hasInteresting := false
			for i := 0; i < len(path); i++ {
				from := path[i]
				to := path[(i+1)%len(path)]
				if isInterestingLoop(edgeRes[[2]int{from, to}]) {
					hasInteresting = true
					break
				}
			}
			if !hasInteresting {
				continue
			}
			key := canonicalCycleKey(cards)
			if seen[key] {
				continue
			}
			seen[key] = true

			combo := buildCycleCombo(cards, path, edgeRes)
			*results = append(*results, combo)
			if len(*results) >= graphWalkMaxCombos {
				return
			}
			continue
		}
		// Johnson prune: each undirected cycle is enumerated by its
		// lex-min start; nodes with index < start would re-enumerate
		// the same cycle from a different rotation.
		if next < start {
			continue
		}
		if visited[next] {
			continue
		}
		if len(path) >= graphWalkMaxLength {
			continue
		}
		visited[next] = true
		path = append(path, next)
		walkCycles(candidates, adj, edgeRes, start, next,
			visited, path, seen, results)
		path = path[:len(path)-1]
		visited[next] = false
	}
}

// buildCycleCombo turns a closed cycle into a ComboResult, threading the
// per-edge resource overlap into the description so downstream consumers
// see the same shape as the pair/triple/quad output. Populates a
// LoopAnnotation with the primary output category, reliability class,
// and a one-sentence human-readable summary (see annotateLongLoop).
//
// LoopType: long cycles (5+ cards) are intentionally floored at "synergy"
// regardless of what classifyLoop would emit. The 2-card / 3-card / 4-card
// detectors run classifyLoop against tight, hand-curated card sets whose
// mandatory-trigger / kill-output flags justify a "true_infinite" or
// "determined" label. A 5+ card cycle, by contrast, requires assembling
// 5-7 specific cards from a 100-card deck — far more fragile, and far
// more prone to false-positive resource-flow overlaps. Routing all
// long-loop hits into the Synergies bucket prevents them from inflating
// the bracket-score combo count (which weights TrueInfinites + Determined
// only; see archetype.go:1142, 2595). Real long-cycle combos still
// surface in the deck profile under Synergies — they're just not treated
// as B4-grade categorical wins.
//
// The annotation's Classification field surfaces the TRUE reliability
// (infinite / determined / probabilistic / synergy) even when LoopType
// is floored — this lets UI consumers display "infinite mana via 5-card
// loop" without the floor affecting bracket math.
func buildCycleCombo(cards []CardProfile, path []int,
	edgeRes map[[2]int][]ResourceType) ComboResult {

	names := make([]string, len(cards))
	for i, c := range cards {
		names[i] = c.Name
	}
	flowParts := make([]string, len(path))
	for i := 0; i < len(path); i++ {
		from := path[i]
		to := path[(i+1)%len(path)]
		flowParts[i] = resourceNames(edgeRes[[2]int{from, to}])
	}

	ann := annotateLongLoop(cards, path, edgeRes)

	combo := ComboResult{
		Cards:       names,
		LoopType:    "synergy",
		Class:       comboClassForOutput(ann.PrimaryOutput),
		Resources:   strings.Join(flowParts, " -> ") + " -> loop",
		Description: ann.Summary,
		Annotation:  ann,
	}
	for _, c := range cards {
		if c.HasRandomSelection {
			combo.NonDeterministic = true
			break
		}
	}
	return combo
}

// ---------------------------------------------------------------------------
// Loop annotation -- surfaceable metadata for graph-walked cycles.
//
// Long cycles (5..7 cards) emitted by FindLongLoops carry raw resource
// flow but no human-readable summary. The annotation produced here is
// what the UI / JSON consumer renders directly: a primary output
// category, a reliability classification, and a one-sentence summary.
//
// Survey of existing surfaces (informing the design):
//   - KnownCombo.Description (known_combos.go) — hand-written 1-sentence
//     "Polyraptor ETB → Marauding Raptor deals 2 → Enrage creates copy
//     → copy ETBs → infinite loop." This is the format the annotation
//     emulates structurally: arrow chain + production tail.
//   - ComboClass* taxonomy (known_combos.go) — what the combo PRODUCES.
//     The annotation picks the closest match to populate ComboResult.Class
//     so downstream consumers (hat affinity weighting, JSON readers,
//     filter UIs) don't have to re-classify graph-walked loops.
//   - LoopType (analysis.go) — reliability axis (true_infinite /
//     determined / synergy). Long cycles floor to "synergy" for bracket
//     safety, but the annotation's Classification preserves the real
//     verdict for display.
// ---------------------------------------------------------------------------

// LoopAnnotation is the surfaceable metadata for a detected combo cycle:
// what the loop produces, how reliable it is, and a one-sentence
// human-readable summary suitable for direct UI display.
type LoopAnnotation struct {
	// PrimaryOutput is the dominant resource the loop produces to the
	// outside world per iteration. One of: "mana", "tokens", "damage",
	// "drain", "mill", "cards", "untap", "graveyard_value",
	// "etb_value", "value_loop" (fallback).
	PrimaryOutput string

	// NetProduces lists every distinct resource type that flows along
	// at least one edge in the cycle. Order-stable; used by UI filters.
	NetProduces []ResourceType

	// ExternalEffects lists distinct node-level Effects strings across
	// the cycle (damage / drain / mill / draw / etc.) — the side
	// outputs beyond the self-sustaining edge resources.
	ExternalEffects []string

	// Classification names the reliability class: "infinite" (mandatory
	// triggers, no exit), "determined" (controllable, has kill output),
	// "probabilistic" (involves random selection), or "synergy" (value
	// engine, no categorical win).
	Classification string

	// Summary is a single sentence safe for direct UI display.
	Summary string
}

// annotateLongLoop produces the surfaceable annotation for a closed
// cycle. The function is deterministic — same input cards/path/edges
// always yield the same annotation, regardless of starting node.
func annotateLongLoop(cards []CardProfile, path []int,
	edgeRes map[[2]int][]ResourceType) *LoopAnnotation {

	ann := &LoopAnnotation{}

	// Collect distinct edge resources (preserving first-seen order so
	// the output is deterministic across rotations).
	seenRes := map[ResourceType]bool{}
	for i := 0; i < len(path); i++ {
		from := path[i]
		to := path[(i+1)%len(path)]
		for _, r := range edgeRes[[2]int{from, to}] {
			if !seenRes[r] {
				seenRes[r] = true
				ann.NetProduces = append(ann.NetProduces, r)
			}
		}
	}

	// Collect distinct node-level external effects.
	seenEff := map[string]bool{}
	for _, c := range cards {
		for _, e := range c.Effects {
			if !seenEff[e] {
				seenEff[e] = true
				ann.ExternalEffects = append(ann.ExternalEffects, e)
			}
		}
	}

	ann.PrimaryOutput = pickPrimaryOutput(cards, seenRes, seenEff)
	ann.Classification = classifyLoopReliability(cards, ann.PrimaryOutput)
	ann.Summary = renderLoopSummary(cards, ann)
	return ann
}

// pickPrimaryOutput selects the dominant output category for a cycle.
// Order is priority — explicit kill primitives (damage / drain / mill)
// win over self-sustaining resources (mana / tokens / cards) because
// the kill primitive is what the combo OUTPUTS to opponents per
// iteration, while the edge resources merely sustain the loop.
func pickPrimaryOutput(cards []CardProfile,
	edges map[ResourceType]bool, effects map[string]bool) string {
	// Kill primitives — what the loop actually does to opponents.
	// Order mirrors ClassifyComboHeuristic (combo_class.go): drain is
	// the most-specific kill class (Exquisite Blood family), then damage
	// (Walking Ballista family), then mill (Altar of Dementia family).
	// A combo that fires both damage AND drain (e.g. Heliod pinging +
	// Vito draining) classes as drain — the drain is the headline
	// kill primitive.
	if effects["drain"] {
		return "drain"
	}
	if effects["damage"] {
		return "damage"
	}
	if effects["mill"] {
		return "mill"
	}
	// Self-sustaining production — what the loop generates that the
	// player accumulates. Mana and tokens beat cards/untap because
	// they're the most-common combo win-vehicles (mana sinks, swarm
	// finishers); cards/untap are typically engines, not finishers.
	if edges[ResMana] {
		return "mana"
	}
	if edges[ResToken] {
		return "tokens"
	}
	if edges[ResUntap] {
		return "untap"
	}
	if edges[ResCard] {
		return "cards"
	}
	// Recursion / blink — value engines, not direct wins.
	if edges[ResReanimate] || edges[ResGraveyard] || edges[ResGraveyardFill] {
		return "graveyard_value"
	}
	for _, c := range cards {
		if c.IsBlinker || c.HasValueETB {
			return "etb_value"
		}
	}
	return "value_loop"
}

// classifyLoopReliability returns the reliability bucket for the loop:
// "infinite" (mandatory triggers + would-be infinite resource),
// "probabilistic" (any random selection), "determined" (kill output OR
// optional triggers — player controls iteration count), or "synergy"
// (value engine, no categorical win).
func classifyLoopReliability(cards []CardProfile, primary string) string {
	// Probabilistic dominates — any random selection makes the loop
	// non-deterministic regardless of other properties.
	for _, c := range cards {
		if c.HasRandomSelection {
			return "probabilistic"
		}
	}

	allMandatory := true
	for _, c := range cards {
		if !c.MandatoryTriggers {
			allMandatory = false
			break
		}
	}

	hasKillOutput := false
	for _, c := range cards {
		for _, e := range c.Effects {
			if e == "damage" || e == "drain" || e == "mill" {
				hasKillOutput = true
				break
			}
		}
		if hasKillOutput {
			break
		}
	}

	// Combat-dependent: any piece requiring combat caps iteration at
	// 1/turn — synergy at best, never categorical.
	for _, c := range cards {
		if c.RequiresCombat {
			return "synergy"
		}
	}

	// Self-exile / hand-only recursion breakers — same gates classifyLoop
	// uses in analysis.go. Mirroring them here keeps the long-loop
	// reliability verdict consistent with the pair/triple/quad detector.
	for _, c := range cards {
		if c.SelfExilesOnDeath {
			return "synergy"
		}
	}

	// "Determined" requires either a kill output OR an open-ended
	// resource the player can iterate at will. "Infinite" requires
	// every trigger be mandatory AND no kill output (so the loop
	// genuinely doesn't terminate without an outside intervention).
	switch primary {
	case "damage", "drain", "mill":
		// Has a kill output by definition.
		if allMandatory {
			return "infinite"
		}
		return "determined"
	case "mana", "tokens", "cards", "untap":
		if allMandatory && !hasKillOutput {
			return "infinite"
		}
		if hasKillOutput || allMandatory {
			return "determined"
		}
		return "synergy"
	}
	return "synergy"
}

// renderLoopSummary builds the one-sentence human description. Format
// mirrors KnownCombo.Description shape: "<N>-card <category> loop:
// <name1> → <name2> → ... — <classification> <output-tail>."
func renderLoopSummary(cards []CardProfile, ann *LoopAnnotation) string {
	names := make([]string, len(cards))
	for i, c := range cards {
		names[i] = c.Name
	}
	chain := strings.Join(names, " → ")

	categoryLabel := outputCategoryLabel(ann.PrimaryOutput)
	classLabel := classificationLabel(ann.Classification)

	tail := outputTail(ann.PrimaryOutput, ann.Classification)
	return fmt.Sprintf("%d-card %s loop: %s — %s %s",
		len(cards), categoryLabel, chain, classLabel, tail)
}

func outputCategoryLabel(primary string) string {
	switch primary {
	case "mana":
		return "mana"
	case "tokens":
		return "token"
	case "damage":
		return "damage"
	case "drain":
		return "drain"
	case "mill":
		return "mill"
	case "cards":
		return "card-draw"
	case "untap":
		return "untap"
	case "graveyard_value":
		return "graveyard-value"
	case "etb_value":
		return "ETB-value"
	}
	return "value"
}

func classificationLabel(class string) string {
	switch class {
	case "infinite":
		return "infinite"
	case "determined":
		return "determined"
	case "probabilistic":
		return "probabilistic"
	}
	return "value-engine"
}

func outputTail(primary, class string) string {
	switch primary {
	case "mana":
		return "(loop generates mana every iteration; needs a mana sink to win)."
	case "tokens":
		return "(loop spawns tokens every iteration; pair with ETB damage / sac outlet to win)."
	case "damage":
		if class == "infinite" {
			return "(loop pings damage every iteration with no exit — kills the table)."
		}
		return "(loop pings damage every iteration; player chooses when to stop)."
	case "drain":
		if class == "infinite" {
			return "(loop drains every iteration with no exit — kills the table)."
		}
		return "(loop drains every iteration; player controls when to stop)."
	case "mill":
		if class == "infinite" {
			return "(loop mills every iteration with no exit — decks opponents)."
		}
		return "(loop mills every iteration; player controls iteration count)."
	case "cards":
		return "(loop draws cards every iteration; engine, not direct win)."
	case "untap":
		return "(loop untaps permanents every iteration; pair with a mana-positive activation to go infinite mana)."
	case "graveyard_value":
		return "(loop recurs cards via the graveyard; value engine, not a direct win)."
	case "etb_value":
		return "(loop generates ETB triggers every iteration; pair with an ETB payoff to win)."
	}
	return "(value-engine resource loop)."
}

// comboClassForOutput maps PrimaryOutput to the closest ComboClass*
// constant. Used to backfill ComboResult.Class on graph-walked cycles
// so downstream consumers (hat affinity weighting, JSON readers, filter
// UIs) don't have to re-classify long loops.
func comboClassForOutput(primary string) string {
	switch primary {
	case "mana":
		return ComboClassInfiniteMana
	case "tokens":
		return ComboClassInfiniteTokens
	case "damage":
		return ComboClassInfiniteDamage
	case "drain":
		return ComboClassInfiniteDrain
	case "mill":
		return ComboClassInfiniteMill
	case "etb_value":
		return ComboClassInfiniteETB
	}
	return ComboClassUnknown
}

// canonicalCycleKey returns a stable identifier for a directed cycle that
// is invariant under rotation AND reversal. Two cycles that visit the
// same multiset of cards in the same cyclic order (in either direction)
// produce the same key.
func canonicalCycleKey(cards []CardProfile) string {
	n := len(cards)
	if n == 0 {
		return ""
	}
	names := make([]string, n)
	for i, c := range cards {
		names[i] = c.Name
	}
	best := rotationKey(names, 0)
	for r := 1; r < n; r++ {
		if k := rotationKey(names, r); k < best {
			best = k
		}
	}
	rev := make([]string, n)
	for i := range names {
		rev[i] = names[n-1-i]
	}
	for r := 0; r < n; r++ {
		if k := rotationKey(rev, r); k < best {
			best = k
		}
	}
	return best
}

func rotationKey(names []string, r int) string {
	n := len(names)
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = names[(r+i)%n]
	}
	return strings.Join(parts, "|")
}
