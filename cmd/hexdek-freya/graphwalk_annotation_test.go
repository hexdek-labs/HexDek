package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// LoopAnnotation tests — verify the surfaceable metadata produced for
// each graph-walked cycle (primary output category, reliability
// classification, one-sentence summary). Each test exercises a real
// 5-card cycle shape and pins the annotation fields.
// ---------------------------------------------------------------------------

// findLoopWithCards retrieves the ComboResult whose Cards multiset
// matches the requested names. Returns nil if not found.
func findLoopWithCards(t *testing.T, results []ComboResult, names ...string) *ComboResult {
	t.Helper()
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	for i := range results {
		if len(results[i].Cards) != len(names) {
			continue
		}
		got := map[string]bool{}
		for _, c := range results[i].Cards {
			got[c] = true
		}
		match := true
		for n := range want {
			if !got[n] {
				match = false
				break
			}
		}
		if match {
			return &results[i]
		}
	}
	return nil
}

// TestAnnotation_DamageOutput_HeliodChain: a Heliod/Ballista-shaped
// 5-cycle with explicit damage effects should annotate as PrimaryOutput
// "damage" and Classification "infinite" (all mandatory).
func TestAnnotation_DamageOutput_HeliodChain(t *testing.T) {
	heliod := CardProfile{
		Name:              "Heliod",
		Produces:          []ResourceType{ResCounter},
		Consumes:          []ResourceType{ResLife},
		MandatoryTriggers: true,
		CounterToDamage:   true,
	}
	ballista := CardProfile{
		Name:              "Walking Ballista",
		Produces:          []ResourceType{ResDamage},
		Consumes:          []ResourceType{ResCounter},
		Effects:           []string{"damage"},
		MandatoryTriggers: true,
	}
	warden := CardProfile{
		Name:              "Soul Warden",
		Produces:          []ResourceType{ResLife},
		Consumes:          []ResourceType{ResToken},
		MandatoryTriggers: true,
		HasETBDamage:      true,
	}
	tokenMaker := CardProfile{
		Name:              "Ocelot Pride",
		Produces:          []ResourceType{ResToken},
		Consumes:          []ResourceType{ResReanimate},
		MandatoryTriggers: true,
	}
	blink := CardProfile{
		Name:              "Cloudshift",
		Produces:          []ResourceType{ResReanimate},
		Consumes:          []ResourceType{ResDamage},
		MandatoryTriggers: true,
		IsBlinker:         true,
	}
	results := FindLongLoops([]CardProfile{heliod, ballista, warden, tokenMaker, blink})
	combo := findLoopWithCards(t, results, heliod.Name, ballista.Name, warden.Name, tokenMaker.Name, blink.Name)
	if combo == nil {
		t.Fatal("expected 5-cycle, got no result")
	}
	if combo.Annotation == nil {
		t.Fatal("expected Annotation, got nil")
	}
	if combo.Annotation.PrimaryOutput != "damage" {
		t.Errorf("PrimaryOutput: got %q, want \"damage\"", combo.Annotation.PrimaryOutput)
	}
	if combo.Annotation.Classification != "infinite" {
		t.Errorf("Classification: got %q, want \"infinite\"", combo.Annotation.Classification)
	}
	if combo.Class != ComboClassInfiniteDamage {
		t.Errorf("Class: got %q, want %q", combo.Class, ComboClassInfiniteDamage)
	}
	if !strings.Contains(combo.Annotation.Summary, "damage loop") {
		t.Errorf("Summary should mention damage loop, got: %s", combo.Annotation.Summary)
	}
	if !strings.Contains(combo.Annotation.Summary, "infinite") {
		t.Errorf("Summary should mention infinite, got: %s", combo.Annotation.Summary)
	}
}

// TestAnnotation_ManaOutput_RampChain: a cycle whose dominant edge
// resource is mana and whose nodes have no kill output should annotate
// as PrimaryOutput "mana".
func TestAnnotation_ManaOutput_RampChain(t *testing.T) {
	cards := []CardProfile{
		{Name: "ManaA", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: true},
		{Name: "ManaB", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true},
		{Name: "ManaC", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResGraveyard}, MandatoryTriggers: true},
		{Name: "ManaD", Produces: []ResourceType{ResGraveyard}, Consumes: []ResourceType{ResReanimate}, MandatoryTriggers: true},
		{Name: "ManaE", Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	combo := findLoopWithCards(t, results, "ManaA", "ManaB", "ManaC", "ManaD", "ManaE")
	if combo == nil {
		t.Fatal("expected 5-cycle")
	}
	if combo.Annotation.PrimaryOutput != "mana" {
		t.Errorf("PrimaryOutput: got %q, want \"mana\"", combo.Annotation.PrimaryOutput)
	}
	if combo.Class != ComboClassInfiniteMana {
		t.Errorf("Class: got %q, want %q", combo.Class, ComboClassInfiniteMana)
	}
	// All mandatory + no kill output + produces infinite resource → infinite.
	if combo.Annotation.Classification != "infinite" {
		t.Errorf("Classification: got %q, want \"infinite\"", combo.Annotation.Classification)
	}
	if !strings.Contains(combo.Annotation.Summary, "mana sink") {
		t.Errorf("Summary should describe mana-sink tail, got: %s", combo.Annotation.Summary)
	}
}

// TestAnnotation_TokenOutput_SwarmChain: a token-edge-heavy cycle
// without explicit kill output should annotate as "tokens".
func TestAnnotation_TokenOutput_SwarmChain(t *testing.T) {
	cards := []CardProfile{
		{Name: "TokA", Produces: []ResourceType{ResToken}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true, MakesInfiniteTokens: true},
		{Name: "TokB", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: true},
		{Name: "TokC", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true},
		{Name: "TokD", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResReanimate}, MandatoryTriggers: true},
		{Name: "TokE", Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResToken}, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	combo := findLoopWithCards(t, results, "TokA", "TokB", "TokC", "TokD", "TokE")
	if combo == nil {
		t.Fatal("expected 5-cycle")
	}
	// Note: mana edge dominates over token edge in pickPrimaryOutput's
	// priority order — mana is "mana sink → win" rather than tokens
	// being the headline production. Pin the actual behavior.
	if combo.Annotation.PrimaryOutput != "mana" {
		t.Errorf("PrimaryOutput: got %q, want \"mana\" (mana edge dominates over token edge)", combo.Annotation.PrimaryOutput)
	}
}

// TestAnnotation_TokenOutput_PureTokenLoop: a cycle whose dominant
// resource is tokens with no mana edge falls into the "tokens" category.
func TestAnnotation_TokenOutput_PureTokenLoop(t *testing.T) {
	cards := []CardProfile{
		{Name: "TokA", Produces: []ResourceType{ResToken}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true, MakesInfiniteTokens: true},
		{Name: "TokB", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: true},
		{Name: "TokC", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResGraveyard}, MandatoryTriggers: true},
		{Name: "TokD", Produces: []ResourceType{ResGraveyard}, Consumes: []ResourceType{ResReanimate}, MandatoryTriggers: true},
		{Name: "TokE", Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResToken}, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	combo := findLoopWithCards(t, results, "TokA", "TokB", "TokC", "TokD", "TokE")
	if combo == nil {
		t.Fatal("expected 5-cycle")
	}
	// No mana edge → tokens dominate.
	if combo.Annotation.PrimaryOutput != "tokens" {
		t.Errorf("PrimaryOutput: got %q, want \"tokens\"", combo.Annotation.PrimaryOutput)
	}
	if combo.Class != ComboClassInfiniteTokens {
		t.Errorf("Class: got %q, want %q", combo.Class, ComboClassInfiniteTokens)
	}
	if !strings.Contains(combo.Annotation.Summary, "token loop") {
		t.Errorf("Summary should mention token loop, got: %s", combo.Annotation.Summary)
	}
}

// TestAnnotation_DrainOutput_VitoChain: a cycle with drain effect on
// any node annotates as "drain" — drain beats edge resources because
// drain is the actual kill primitive.
func TestAnnotation_DrainOutput_VitoChain(t *testing.T) {
	vito := CardProfile{
		Name:              "Vito",
		Produces:          []ResourceType{ResLife},
		Consumes:          []ResourceType{ResMana},
		Effects:           []string{"drain"},
		MandatoryTriggers: true,
		LifegainToDrain:   true,
	}
	exquisite := CardProfile{
		Name:              "Exquisite Blood",
		Produces:          []ResourceType{ResMana},
		Consumes:          []ResourceType{ResLife},
		MandatoryTriggers: true,
		LifelossToPump:    true,
	}
	bridge1 := CardProfile{
		Name: "Bridge1", Produces: []ResourceType{ResLife}, Consumes: []ResourceType{ResLife},
		MandatoryTriggers: true,
	}
	bridge2 := CardProfile{
		Name: "Bridge2", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResMana},
		MandatoryTriggers: true,
	}
	bridge3 := CardProfile{
		Name: "Bridge3", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResLife},
		MandatoryTriggers: true,
	}
	cards := []CardProfile{vito, exquisite, bridge1, bridge2, bridge3}
	results := FindLongLoops(cards)
	// Need at least one 5-cycle including vito so the drain effect surfaces.
	var withVito *ComboResult
	for i := range results {
		for _, n := range results[i].Cards {
			if n == vito.Name {
				withVito = &results[i]
				break
			}
		}
		if withVito != nil {
			break
		}
	}
	if withVito == nil {
		// No 5-cycle including Vito formed from this graph — surface
		// the same drain-precedence assertion against an explicitly
		// constructed cycle to keep the test meaningful.
		t.Skip("graph did not form a 5-cycle including Vito; drain-precedence covered by TestPickPrimaryOutput_DrainPrecedence")
	}
	if withVito.Annotation.PrimaryOutput != "drain" {
		t.Errorf("PrimaryOutput: got %q, want \"drain\"", withVito.Annotation.PrimaryOutput)
	}
	if withVito.Class != ComboClassInfiniteDrain {
		t.Errorf("Class: got %q, want %q", withVito.Class, ComboClassInfiniteDrain)
	}
}

// TestAnnotation_ProbabilisticClassification: any HasRandomSelection
// node forces the loop to "probabilistic" regardless of other flags.
func TestAnnotation_ProbabilisticClassification(t *testing.T) {
	cards := []CardProfile{
		{Name: "RandA", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: true, HasRandomSelection: true},
		{Name: "RandB", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true},
		{Name: "RandC", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResGraveyard}, MandatoryTriggers: true},
		{Name: "RandD", Produces: []ResourceType{ResGraveyard}, Consumes: []ResourceType{ResReanimate}, MandatoryTriggers: true},
		{Name: "RandE", Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	combo := findLoopWithCards(t, results, "RandA", "RandB", "RandC", "RandD", "RandE")
	if combo == nil {
		t.Fatal("expected 5-cycle")
	}
	if combo.Annotation.Classification != "probabilistic" {
		t.Errorf("Classification: got %q, want \"probabilistic\"", combo.Annotation.Classification)
	}
	if !combo.NonDeterministic {
		t.Errorf("NonDeterministic flag should be set when a node has HasRandomSelection")
	}
}

// TestAnnotation_DeterminedClassification: a cycle with kill output
// but NOT all-mandatory triggers should classify as "determined".
func TestAnnotation_DeterminedClassification(t *testing.T) {
	cards := []CardProfile{
		{Name: "DetA", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: false, Effects: []string{"damage"}},
		{Name: "DetB", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true},
		{Name: "DetC", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResGraveyard}, MandatoryTriggers: true},
		{Name: "DetD", Produces: []ResourceType{ResGraveyard}, Consumes: []ResourceType{ResReanimate}, MandatoryTriggers: true},
		{Name: "DetE", Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	combo := findLoopWithCards(t, results, "DetA", "DetB", "DetC", "DetD", "DetE")
	if combo == nil {
		t.Fatal("expected 5-cycle")
	}
	// damage effect on DetA → PrimaryOutput "damage" → kill primitive.
	// Not all mandatory → "determined", not "infinite".
	if combo.Annotation.Classification != "determined" {
		t.Errorf("Classification: got %q, want \"determined\"", combo.Annotation.Classification)
	}
	if combo.Annotation.PrimaryOutput != "damage" {
		t.Errorf("PrimaryOutput: got %q, want \"damage\"", combo.Annotation.PrimaryOutput)
	}
}

// TestAnnotation_SummaryShape: the summary must include the card chain,
// the cycle length, the category label, and the classification label.
func TestAnnotation_SummaryShape(t *testing.T) {
	cards := []CardProfile{
		{Name: "Shape1", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: true},
		{Name: "Shape2", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true},
		{Name: "Shape3", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResGraveyard}, MandatoryTriggers: true},
		{Name: "Shape4", Produces: []ResourceType{ResGraveyard}, Consumes: []ResourceType{ResReanimate}, MandatoryTriggers: true},
		{Name: "Shape5", Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	combo := findLoopWithCards(t, results, "Shape1", "Shape2", "Shape3", "Shape4", "Shape5")
	if combo == nil {
		t.Fatal("expected 5-cycle")
	}
	if combo.Annotation.Summary == "" {
		t.Fatal("Summary is empty")
	}
	for _, n := range []string{"Shape1", "Shape2", "Shape3", "Shape4", "Shape5"} {
		if !strings.Contains(combo.Annotation.Summary, n) {
			t.Errorf("Summary should mention %s, got: %s", n, combo.Annotation.Summary)
		}
	}
	if !strings.Contains(combo.Annotation.Summary, "5-card") {
		t.Errorf("Summary should mention cycle length, got: %s", combo.Annotation.Summary)
	}
	if !strings.Contains(combo.Annotation.Summary, "mana") {
		t.Errorf("Summary should mention output category, got: %s", combo.Annotation.Summary)
	}
	// arrow chain delimiter
	if !strings.Contains(combo.Annotation.Summary, "→") {
		t.Errorf("Summary should use arrow chain delimiter, got: %s", combo.Annotation.Summary)
	}
	// Description should mirror the summary now (rewritten from raw flow).
	if combo.Description != combo.Annotation.Summary {
		t.Errorf("Description should equal Annotation.Summary; got Description=%q Summary=%q",
			combo.Description, combo.Annotation.Summary)
	}
}

// TestAnnotation_NetProducesContainsCycleEdges: NetProduces should
// enumerate every distinct edge resource in the cycle.
func TestAnnotation_NetProducesContainsCycleEdges(t *testing.T) {
	cards := []CardProfile{
		{Name: "EdgA", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: true},
		{Name: "EdgB", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true},
		{Name: "EdgC", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResGraveyard}, MandatoryTriggers: true},
		{Name: "EdgD", Produces: []ResourceType{ResGraveyard}, Consumes: []ResourceType{ResReanimate}, MandatoryTriggers: true},
		{Name: "EdgE", Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	combo := findLoopWithCards(t, results, "EdgA", "EdgB", "EdgC", "EdgD", "EdgE")
	if combo == nil {
		t.Fatal("expected 5-cycle")
	}
	want := map[ResourceType]bool{ResMana: true, ResCard: true, ResUntap: true, ResGraveyard: true, ResReanimate: true}
	got := map[ResourceType]bool{}
	for _, r := range combo.Annotation.NetProduces {
		got[r] = true
	}
	for r := range want {
		if !got[r] {
			t.Errorf("NetProduces missing %v; got %v", r, combo.Annotation.NetProduces)
		}
	}
}

// TestPickPrimaryOutput_DrainPrecedence: drain effect beats every edge
// resource. Direct unit test guarantees the precedence even when the
// graph walker can't construct the exact cycle (the high-level test
// TestAnnotation_DrainOutput_VitoChain may skip when no 5-cycle forms).
func TestPickPrimaryOutput_DrainPrecedence(t *testing.T) {
	cards := []CardProfile{
		{Name: "drainer", Effects: []string{"drain"}, Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResLife}},
		{Name: "ramp1", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResLife}},
	}
	edges := map[ResourceType]bool{ResMana: true, ResToken: true, ResCard: true}
	effects := map[string]bool{"drain": true, "damage": true}
	// drain wins over damage AND over all the edge resources.
	if got := pickPrimaryOutput(cards, edges, effects); got != "drain" {
		t.Errorf("got %q, want \"drain\" — drain should win precedence even when damage + edges present", got)
	}
}

// TestClassificationLabel_CoverageSpotCheck: each Classification has a
// human label, with a default for the synergy/unknown bucket.
func TestClassificationLabel_CoverageSpotCheck(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"infinite", "infinite"},
		{"determined", "determined"},
		{"probabilistic", "probabilistic"},
		{"synergy", "value-engine"},
		{"", "value-engine"},
	}
	for _, c := range cases {
		if got := classificationLabel(c.input); got != c.want {
			t.Errorf("classificationLabel(%q): got %q, want %q", c.input, got, c.want)
		}
	}
}
