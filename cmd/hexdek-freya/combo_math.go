package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase 1 — COMBO = intrinsic compounding math.
//
// A COMBO is detected from card MECHANICS, deck-independently:
//   - ≥2 compounding math-pieces assembled (doubler + doubler, doubler +
//     for-each, two for-each scalers, …), OR
//   - a self-contained loop (the existing true_infinite/determined/Confirmed
//     detection already finds these), OR
//   - (Q4) an enabler (sac-fodder / tutor / recursion) feeding a
//     compounding-math payoff.
//
// Math primitives are read from TWO sources behind the MathOp.Source seam and
// UNIONED (Q1 — Thor AST cross-referenced with Freya's oracle analysis):
//   - oracle: detectOracleMathOps (always available, populated in ClassifyCard
//     onto CardProfile.MathOps).
//   - ast: mathOpsFromAST over the Thor AST corpus (precise; present only when
//     the ast_dataset.jsonl corpus is loaded — best-effort, nil-safe).
//
// Output lands in FreyaReport.Combos (additive; legacy buckets untouched).

// ---------------------------------------------------------------------------
// AST source seam
// ---------------------------------------------------------------------------

// mathASTSource is the minimal read interface combo-math needs from a Thor AST
// corpus. *astload.Corpus satisfies it. nil => oracle-only (graceful degrade).
type mathASTSource interface {
	Get(name string) (*gameast.CardAST, bool)
}

var (
	astMu        sync.Mutex
	astCorpus    mathASTSource
	astMathCache = map[string][]MathOp{}
)

// SetMathASTSource installs the Thor AST corpus used to enrich MathSignatures.
// Called once at startup (main.go) with a best-effort astload.Load result;
// tests inject a fixture. Passing nil disables AST enrichment (oracle-only).
func SetMathASTSource(s mathASTSource) {
	astMu.Lock()
	defer astMu.Unlock()
	astCorpus = s
	astMathCache = map[string][]MathOp{}
}

// ---------------------------------------------------------------------------
// Oracle-sourced math detection (called from ClassifyCard)
// ---------------------------------------------------------------------------

// detectOracleMathOps reads compounding-math operations from reminder-stripped
// oracle text + the already-computed profile flags. Conservative on purpose —
// the Thor AST is the precise source; oracle is the always-available baseline
// that the AST cross-references. ot is otClean (reminder-stripped, lowercased).
func detectOracleMathOps(ot string, p *CardProfile) []MathOp {
	var ops []MathOp
	add := func(kind, res, detail string) {
		ops = append(ops, MathOp{Kind: kind, Resource: res, Detail: detail, Source: MathSourceOracle})
	}

	// Doublers (×2). Reuse Freya's existing damage-doubler flag and add the
	// canonical token / counter / mana doubling phrasings.
	if p.IsDamageDoubler {
		add(MathOpDoubler, "damage", "doubles damage")
	}
	if containsAny(ot, "double the number of tokens", "twice that many tokens",
		"twice that many of those tokens", "create twice that many",
		"creates twice that many", "double the number of those tokens") {
		add(MathOpDoubler, "tokens", "doubles tokens")
	}
	if containsAny(ot, "twice that many +1/+1", "double the number of +1/+1",
		"put twice that many", "twice that many counters", "double the number of counters") {
		add(MathOpDoubler, "counters", "doubles counters")
	}
	if containsAny(ot, "double the amount of mana", "doubles the amount of mana",
		"twice that much of that mana") {
		add(MathOpManaMultiplier, "mana", "doubles mana")
	}

	// For-each scaling — output scales by a board / permanent / counter count.
	// Matches both "for each X you control" and "equal to the number of X you
	// control" phrasings (the same compounding scaler; the latter is what the
	// Thor AST encodes as ScalingKind=creatures_you_control etc.). Restricted
	// to "you control" / counter / devotion anchors so generic "for each
	// color" / "for each opponent" (small fixed denominators) don't qualify.
	scaleVerb := strings.Contains(ot, "for each") ||
		strings.Contains(ot, "number of") || strings.Contains(ot, "equal to the number")
	if scaleVerb && containsAny(ot,
		"creature you control", "creatures you control",
		"permanent you control", "permanents you control",
		"land you control", "lands you control",
		"artifact you control", "artifacts you control",
		"enchantment you control", "enchantments you control",
		"token you control", "tokens you control",
		"+1/+1 counter", "your devotion", "devotion to") {
		add(MathOpForEach, "scaled", "scales by a permanent/counter count you control")
	}

	// Cost reduction toward free / looping casts.
	if containsAny(ot, "without paying its mana cost", "without paying their mana cost",
		"without paying that spell's mana cost") {
		add(MathOpCostReduction, "cast", "free cast")
	}
	if strings.Contains(ot, "less to cast") ||
		(strings.Contains(ot, "spells") && strings.Contains(ot, "cost") && strings.Contains(ot, "less")) {
		add(MathOpCostReduction, "cast", "spell cost reduction")
	}

	return ops
}

// ---------------------------------------------------------------------------
// AST-sourced math detection
// ---------------------------------------------------------------------------

// astMathOpsFor returns the AST-sourced math ops for a card name, cached.
// Empty when no corpus is loaded or the card isn't found / carries no math.
func astMathOpsFor(name string) []MathOp {
	astMu.Lock()
	defer astMu.Unlock()
	if astCorpus == nil {
		return nil
	}
	if ops, ok := astMathCache[name]; ok {
		return ops
	}
	var ops []MathOp
	if card, ok := astCorpus.Get(name); ok && card != nil {
		ops = mathOpsFromAST(card)
	}
	astMathCache[name] = ops
	return ops
}

// mathOpsFromAST walks a parsed CardAST and collects compounding-math ops:
//   - any ScalingAmount whose kind is a board/permanent/counter scaler
//     (creatures_you_control, permanents_you_control, devotion,
//     counters_on_self, …) => for_each
//   - any CounterMod with Op=="double" => doubler (counters)
//
// The walk is reflection-based so it is robust to the full breadth of gameast
// effect/ability node types without enumerating every one. Unexported fields
// (the embedded baseEffect/baseAbility markers) are skipped.
func mathOpsFromAST(card *gameast.CardAST) []MathOp {
	var ops []MathOp
	walkASTValue(reflect.ValueOf(card), &ops, 0)
	return dedupMathOps(ops)
}

var (
	scalingAmountType = reflect.TypeOf(gameast.ScalingAmount{})
	counterModType    = reflect.TypeOf(gameast.CounterMod{})
)

func walkASTValue(v reflect.Value, ops *[]MathOp, depth int) {
	if depth > 80 || !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return
		}
		walkASTValue(v.Elem(), ops, depth+1)
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			walkASTValue(v.Index(i), ops, depth+1)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			walkASTValue(v.MapIndex(k), ops, depth+1)
		}
	case reflect.Struct:
		t := v.Type()
		switch t {
		case scalingAmountType:
			kind := v.FieldByName("ScalingKind").String()
			if isCompoundingScalingKind(kind) {
				*ops = append(*ops, MathOp{
					Kind: MathOpForEach, Resource: "scaled",
					Detail: "scaling:" + kind, Source: MathSourceAST,
				})
			}
		case counterModType:
			if v.FieldByName("Op").String() == "double" {
				*ops = append(*ops, MathOp{
					Kind: MathOpDoubler, Resource: "counters",
					Detail: "double counters", Source: MathSourceAST,
				})
			}
		}
		for i := 0; i < v.NumField(); i++ {
			if !t.Field(i).IsExported() {
				continue // skip embedded baseEffect/baseAbility markers
			}
			walkASTValue(v.Field(i), ops, depth+1)
		}
	}
}

// isCompoundingScalingKind reports whether an AST ScalingAmount kind scales
// with a board / permanent / counter count (i.e. it COMPOUNDS as the board
// grows). Fixed/opaque kinds (x, literal, raw) are excluded — they don't grow.
func isCompoundingScalingKind(kind string) bool {
	switch kind {
	case "", "x", "literal", "raw":
		return false
	}
	return strings.Contains(kind, "you_control") ||
		strings.Contains(kind, "devotion") ||
		strings.Contains(kind, "counters") ||
		strings.Contains(kind, "cards_in") ||
		strings.Contains(kind, "number_of") ||
		strings.Contains(kind, "creatures") ||
		strings.Contains(kind, "permanents") ||
		strings.Contains(kind, "lands") ||
		strings.Contains(kind, "artifacts")
}

// ---------------------------------------------------------------------------
// MathSignature assembly (oracle ∪ ast)
// ---------------------------------------------------------------------------

// buildMathSignature unions a card's oracle-sourced math ops (CardProfile.
// MathOps) with its AST-sourced ops, deduped. This is the Q1 cross-reference:
// either source marking a card as a math piece is sufficient; when both fire,
// the union carries both Source tags.
func buildMathSignature(p CardProfile) MathSignature {
	ops := make([]MathOp, 0, len(p.MathOps))
	ops = append(ops, p.MathOps...)
	ops = append(ops, astMathOpsFor(p.Name)...)
	return MathSignature{Ops: dedupMathOps(ops)}
}

func dedupMathOps(in []MathOp) []MathOp {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]MathOp, 0, len(in))
	for _, op := range in {
		key := op.Kind + "|" + op.Resource + "|" + op.Source
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, op)
	}
	return out
}

// ---------------------------------------------------------------------------
// interactionIsCombo + report population
// ---------------------------------------------------------------------------

// interactionIsCombo decides whether an interaction over `cards` is a COMBO
// under the intrinsic-math model, returning a human reason. It does NOT cover
// self-contained loops — those are recognized separately from the existing
// LoopType (Confirmed/true_infinite/determined) in CategorizeReportCombos.
func interactionIsCombo(cards []CardProfile, sig func(CardProfile) MathSignature) (bool, string) {
	mathPieces := 0
	hasEnabler := false
	var mathNames []string
	for _, c := range cards {
		s := sig(c)
		if s.HasCompoundingMath() {
			mathPieces++
			mathNames = append(mathNames, c.Name)
		}
		if c.IsOutlet || c.IsTutor || c.IsRecursion {
			hasEnabler = true
		}
	}
	// ≥2 compounding math-pieces assembled.
	if mathPieces >= 2 {
		return true, "compounding math assembled: " + strings.Join(mathNames, " + ")
	}
	// Enabler → compounding-math payoff (Q4). The enabler and payoff may be the
	// same card (a self-contained math engine with its own outlet) or two
	// different cards in the interaction.
	if mathPieces >= 1 && hasEnabler {
		return true, "enabler feeds compounding-math payoff: " + strings.Join(mathNames, " + ")
	}
	return false, ""
}

// CategorizeReportCombos populates FreyaReport.Combos from the deck's
// interactions. Idempotent — rebuilds from scratch each call (so it is safe to
// invoke at the end of AnalyzeDeck and again after main.go appends mechanic
// synergies). VALUE / SYNERGY buckets are left for Phase 2/3.
func CategorizeReportCombos(report *FreyaReport) {
	report.Combos = nil

	profByName := make(map[string]CardProfile, len(report.Profiles))
	sigCache := map[string]MathSignature{}
	sig := func(c CardProfile) MathSignature {
		if s, ok := sigCache[c.Name]; ok {
			return s
		}
		s := buildMathSignature(c)
		sigCache[c.Name] = s
		return s
	}
	var mathCards, enablers []CardProfile
	for _, p := range report.Profiles {
		profByName[p.Name] = p
		if sig(p).HasCompoundingMath() {
			mathCards = append(mathCards, p)
		}
		if p.IsOutlet || p.IsTutor || p.IsRecursion {
			enablers = append(enablers, p)
		}
	}

	comboKeys := map[string]bool{}
	add := func(c ComboResult) {
		key := comboCardKey(c.Cards)
		if comboKeys[key] {
			return
		}
		comboKeys[key] = true
		c.Categories = []string{CategoryCombo}
		report.Combos = append(report.Combos, c)
	}

	profilesFor := func(names []string) []CardProfile {
		out := make([]CardProfile, 0, len(names))
		for _, n := range names {
			if p, ok := profByName[n]; ok {
				out = append(out, p)
			}
		}
		return out
	}

	// 1. Re-tag already-detected interactions that qualify as combos: any
	//    self-contained loop (Confirmed / true_infinite / determined) or any
	//    interaction passing the intrinsic-math predicate.
	buckets := [][]ComboResult{
		report.TrueInfinites, report.Determined, report.Finishers,
		report.Synergies, report.GraveyardLoops,
	}
	for _, bucket := range buckets {
		for _, c := range bucket {
			isLoop := c.Confirmed || c.LoopType == "true_infinite" || c.LoopType == "determined"
			ok, _ := interactionIsCombo(profilesFor(c.Cards), sig)
			if isLoop || ok {
				add(c)
			}
		}
	}

	// 2. Synthesize math-assembly combos not surfaced by any existing
	//    detector (two math pieces rarely form a resource loop, so the loop
	//    detectors miss them). Bounded: one synthesized entry per math card.
	for _, m := range mathCards {
		// Prefer a second compounding-math piece (the ≥2-math case).
		var partner *CardProfile
		for i := range mathCards {
			if mathCards[i].Name != m.Name {
				partner = &mathCards[i]
				break
			}
		}
		if partner != nil {
			add(ComboResult{
				Cards:    []string{m.Name, partner.Name},
				LoopType: "combo",
				Description: fmt.Sprintf("%s + %s — compounding math assembled",
					m.Name, partner.Name),
			})
			continue
		}
		// Otherwise pair the lone math payoff with an enabler (Q4).
		for i := range enablers {
			if enablers[i].Name == m.Name {
				continue
			}
			add(ComboResult{
				Cards:    []string{enablers[i].Name, m.Name},
				LoopType: "combo",
				Description: fmt.Sprintf("%s enables %s — enabler feeds compounding-math payoff",
					enablers[i].Name, m.Name),
			})
			break
		}
	}

	// Phase 2: VALUE axis (deck-independent advantage baseline). Runs after
	// the COMBO axis so combo entries can be excluded from value.
	populateValueEngines(report, comboKeys, profByName)

	// Phase 3: SYNERGY axis (commander-relative) + the Layer-B context re-tag
	// (co-tag VALUE entries that advance the plan with +SYNERGY). Runs last so
	// it sees the final COMBO + VALUE buckets.
	valueKeys := make(map[string]bool, len(report.ValueEngines))
	for _, v := range report.ValueEngines {
		valueKeys[comboCardKey(v.Cards)] = true
	}
	populateSynergyPackages(report, comboKeys, valueKeys, profByName)
}

// comboCardKey is the order-independent dedup key for a combo's card set.
func comboCardKey(names []string) string {
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)
	return strings.Join(sorted, "|")
}
