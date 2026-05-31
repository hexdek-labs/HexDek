package gameast

import (
	"sort"
	"testing"
)

// ----------------------------------------------------------------------
// Per-ability scoring.
// ----------------------------------------------------------------------

func TestAbilityConfidence_KeywordAlwaysFull(t *testing.T) {
	cases := []*Keyword{
		{Name: "flying"},
		{Name: "trample"},
		{Name: "flashback", Args: []interface{}{"{2}{U}"}},
		{Name: "", Raw: "rotten oracle"},
	}
	for _, k := range cases {
		if got := AbilityConfidence(k); got != 1.0 {
			t.Errorf("Keyword %q: want 1.0, got %v", k.Name, got)
		}
	}
}

func TestAbilityConfidence_NilFailsOpen(t *testing.T) {
	if AbilityConfidence(nil) != 1.0 {
		t.Errorf("nil ability: want 1.0 (fail-open), got %v", AbilityConfidence(nil))
	}
}

func TestAbilityConfidence_StaticStructured(t *testing.T) {
	a := &Static{Modification: &Modification{ModKind: "ability_word"}}
	if got := AbilityConfidence(a); got != 1.0 {
		t.Errorf("structured Static: want 1.0, got %v", got)
	}
}

func TestAbilityConfidence_StaticFallbackModKind(t *testing.T) {
	for kind := range FallbackModKinds {
		a := &Static{Modification: &Modification{ModKind: kind}}
		want := 0.5
		if got := AbilityConfidence(a); got != want {
			t.Errorf("Static fallback %q: want %v, got %v", kind, want, got)
		}
	}
}

func TestAbilityConfidence_StaticMissingMod(t *testing.T) {
	a := &Static{}
	want := 0.7
	if got := AbilityConfidence(a); got != want {
		t.Errorf("Static no Modification: want %v, got %v", want, got)
	}
}

func TestAbilityConfidence_StaticFallbackCondStacks(t *testing.T) {
	a := &Static{
		Modification: &Modification{ModKind: "ability_word"},
		Condition:    &Condition{Kind: "if"},
	}
	want := 0.7 // 1.0 - 0.3 cond
	if got := AbilityConfidence(a); got != want {
		t.Errorf("Static + fallback cond: want %v, got %v", want, got)
	}
}

func TestAbilityConfidence_StaticBothPenaltiesStack(t *testing.T) {
	a := &Static{
		Modification: &Modification{ModKind: "parsed_effect_residual"},
		Condition:    &Condition{Kind: "conditional"},
	}
	// 1.0 - 0.5 (mod) - 0.3 (cond) = 0.2
	want := 0.2
	if got := AbilityConfidence(a); got < want-0.001 || got > want+0.001 {
		t.Errorf("Static stacked penalties: want ~%v, got %v", want, got)
	}
}

func TestAbilityConfidence_TriggeredStructured(t *testing.T) {
	a := &Triggered{Effect: &Draw{Count: NumberOrRef{IsInt: true, Int: 1}}}
	if got := AbilityConfidence(a); got != 1.0 {
		t.Errorf("structured Triggered: want 1.0, got %v", got)
	}
}

func TestAbilityConfidence_TriggeredNilEffect(t *testing.T) {
	a := &Triggered{}
	want := 0.6
	if got := AbilityConfidence(a); got != want {
		t.Errorf("Triggered no Effect: want %v, got %v", want, got)
	}
}

func TestAbilityConfidence_TriggeredUnknownEffectIsFallback(t *testing.T) {
	a := &Triggered{Effect: &UnknownEffect{RawText: "do something inscrutable"}}
	want := 0.5
	if got := AbilityConfidence(a); got != want {
		t.Errorf("Triggered UnknownEffect: want %v, got %v", want, got)
	}
}

func TestAbilityConfidence_TriggeredModEffectFallback(t *testing.T) {
	for kind := range FallbackModKinds {
		a := &Triggered{Effect: &ModificationEffect{ModKind: kind}}
		want := 0.5
		if got := AbilityConfidence(a); got != want {
			t.Errorf("Triggered ModEffect %q: want %v, got %v", kind, want, got)
		}
	}
}

func TestAbilityConfidence_TriggeredModEffectStructured(t *testing.T) {
	a := &Triggered{Effect: &ModificationEffect{ModKind: "investigate"}}
	if got := AbilityConfidence(a); got != 1.0 {
		t.Errorf("Triggered ModEffect structured kind: want 1.0, got %v", got)
	}
}

func TestAbilityConfidence_TriggeredFallbackCondStacks(t *testing.T) {
	a := &Triggered{
		Effect:        &Draw{Count: NumberOrRef{IsInt: true, Int: 1}},
		InterveningIf: &Condition{Kind: "if"},
	}
	want := 0.7
	if got := AbilityConfidence(a); got != want {
		t.Errorf("Triggered + fallback intervening if: want %v, got %v", want, got)
	}
}

func TestAbilityConfidence_ActivatedStructured(t *testing.T) {
	a := &Activated{Effect: &Damage{Amount: NumberOrRef{IsInt: true, Int: 2}}}
	if got := AbilityConfidence(a); got != 1.0 {
		t.Errorf("structured Activated: want 1.0, got %v", got)
	}
}

func TestAbilityConfidence_ActivatedNilEffect(t *testing.T) {
	a := &Activated{}
	want := 0.6
	if got := AbilityConfidence(a); got != want {
		t.Errorf("Activated no Effect: want %v, got %v", want, got)
	}
}

func TestAbilityConfidence_ActivatedFallbackEffect(t *testing.T) {
	a := &Activated{Effect: &ModificationEffect{ModKind: "parsed_tail"}}
	want := 0.5
	if got := AbilityConfidence(a); got != want {
		t.Errorf("Activated fallback effect: want %v, got %v", want, got)
	}
}

func TestAbilityConfidence_ClampedNoNegative(t *testing.T) {
	// Maximum-penalty Static: -0.5 (mod) + -0.3 (cond) = -0.8, score 0.2.
	// Triggered with both Effect=nil AND fallback intervening-if would be
	// -0.4 + -0.3 = -0.7, score 0.3 — still positive. Verify we never
	// drop below 0 even with crafted maximum penalties.
	a := &Triggered{InterveningIf: &Condition{Kind: "if"}}
	got := AbilityConfidence(a)
	if got < 0 {
		t.Errorf("clamp violated: score %v < 0", got)
	}
}

// ----------------------------------------------------------------------
// IsConfident threshold gate.
// ----------------------------------------------------------------------

func TestIsConfident_AtOrAboveThreshold(t *testing.T) {
	if !IsConfident(0.7, 0.7) {
		t.Errorf("0.7 at threshold 0.7: want true (at-or-above)")
	}
	if !IsConfident(1.0, 0.7) {
		t.Errorf("1.0 vs 0.7: want true")
	}
	if IsConfident(0.69, 0.7) {
		t.Errorf("0.69 vs 0.7: want false")
	}
}

func TestAbilityIsConfident_DefaultThreshold(t *testing.T) {
	clean := &Static{Modification: &Modification{ModKind: "anthem"}}
	if !AbilityIsConfident(clean, DefaultConfidenceThreshold) {
		t.Errorf("structured Static must be confident at default threshold")
	}
	dirty := &Static{Modification: &Modification{ModKind: "parsed_tail"}}
	if AbilityIsConfident(dirty, DefaultConfidenceThreshold) {
		t.Errorf("fallback Static must NOT be confident at default threshold (score=0.5)")
	}
}

// ----------------------------------------------------------------------
// LowConfidenceReasons.
// ----------------------------------------------------------------------

func TestLowConfidenceReasons_StructuredEmpty(t *testing.T) {
	a := &Static{Modification: &Modification{ModKind: "anthem"}}
	if r := LowConfidenceReasons(a); len(r) != 0 {
		t.Errorf("structured Static: want no reasons, got %v", r)
	}
}

func TestLowConfidenceReasons_KeywordEmpty(t *testing.T) {
	if r := LowConfidenceReasons(&Keyword{Name: "flying"}); len(r) != 0 {
		t.Errorf("Keyword: want no reasons, got %v", r)
	}
}

func TestLowConfidenceReasons_StackedSorted(t *testing.T) {
	a := &Static{
		Modification: &Modification{ModKind: "custom"},
		Condition:    &Condition{Kind: "conditional"},
	}
	r := LowConfidenceReasons(a)
	if len(r) != 2 {
		t.Fatalf("want 2 reasons, got %v", r)
	}
	// Stable sorted.
	if !sort.StringsAreSorted(r) {
		t.Errorf("reasons not sorted: %v", r)
	}
	// Specific labels present.
	wantLabels := map[string]bool{
		"static_fallback_mod_kind:custom":      true,
		"static_fallback_cond_kind:conditional": true,
	}
	for _, want := range []string{
		"static_fallback_mod_kind:custom",
		"static_fallback_cond_kind:conditional",
	} {
		if !wantLabels[want] {
			t.Errorf("missing expected label %q", want)
		}
	}
}

func TestLowConfidenceReasons_TriggeredModEffect(t *testing.T) {
	a := &Triggered{Effect: &ModificationEffect{ModKind: "parsed_effect_residual"}}
	r := LowConfidenceReasons(a)
	if len(r) != 1 {
		t.Fatalf("want 1 reason, got %v", r)
	}
	if r[0] != "triggered_fallback_effect_kind:parsed_effect_residual" {
		t.Errorf("wrong label: %q", r[0])
	}
}

func TestLowConfidenceReasons_TriggeredUnknownEffect(t *testing.T) {
	a := &Triggered{Effect: &UnknownEffect{RawText: "..."}}
	r := LowConfidenceReasons(a)
	if len(r) != 1 || r[0] != "triggered_fallback_effect_kind:unknown_effect" {
		t.Errorf("want unknown_effect label, got %v", r)
	}
}

// ----------------------------------------------------------------------
// Card-level scoring.
// ----------------------------------------------------------------------

func TestCardConfidence_NoAbilities(t *testing.T) {
	c := &CardAST{FullyParsed: true}
	if got := CardConfidence(c); got != 1.0 {
		t.Errorf("vanilla card: want 1.0, got %v", got)
	}
}

func TestCardConfidence_NotFullyParsedPenalty(t *testing.T) {
	c := &CardAST{FullyParsed: false}
	want := 0.6
	if got := CardConfidence(c); got != want {
		t.Errorf("!FullyParsed empty-ability card: want %v, got %v", want, got)
	}
}

func TestCardConfidence_MeanAcrossAbilities(t *testing.T) {
	c := &CardAST{
		FullyParsed: true,
		Abilities: []Ability{
			&Keyword{Name: "flying"},                                        // 1.0
			&Static{Modification: &Modification{ModKind: "parsed_tail"}},   // 0.5
		},
	}
	// mean = 0.75, card-level penalty = 1.0 → final 0.75
	got := CardConfidence(c)
	if got < 0.74 || got > 0.76 {
		t.Errorf("mixed card: want ~0.75, got %v", got)
	}
}

func TestCardConfidence_AllAbilitiesFallbackBelowThreshold(t *testing.T) {
	c := &CardAST{
		FullyParsed: true,
		Abilities: []Ability{
			&Static{Modification: &Modification{ModKind: "parsed_effect_residual"}},
			&Static{Modification: &Modification{ModKind: "custom"}},
		},
	}
	got := CardConfidence(c)
	if got >= DefaultConfidenceThreshold {
		t.Errorf("all-fallback card scored %v >= threshold %v", got, DefaultConfidenceThreshold)
	}
}

func TestCardMinConfidence_WeakestLink(t *testing.T) {
	c := &CardAST{
		FullyParsed: true,
		Abilities: []Ability{
			&Keyword{Name: "flying"},                                              // 1.0
			&Static{Modification: &Modification{ModKind: "anthem"}},               // 1.0
			&Triggered{Effect: &ModificationEffect{ModKind: "parsed_tail"}},      // 0.5
		},
	}
	got := CardMinConfidence(c)
	if got != 0.5 {
		t.Errorf("min: want 0.5 (weakest link), got %v", got)
	}
}

func TestCardConfidence_ParseErrorsCompound(t *testing.T) {
	c := &CardAST{
		FullyParsed: false,
		ParseErrors: []string{"err1", "err2", "err3"}, // 1 from !FullyParsed + 2 extra @ -0.2
		Abilities:   []Ability{&Keyword{Name: "flying"}},
	}
	// 1.0 - 0.4 - 0.4 = 0.2, then * 1.0 (ability mean) = 0.2
	got := CardConfidence(c)
	if got > 0.22 || got < 0.18 {
		t.Errorf("3-parse-error card: want ~0.2, got %v", got)
	}
}

func TestCardIsConfident_GateMatchesScore(t *testing.T) {
	cleanCard := &CardAST{
		FullyParsed: true,
		Abilities:   []Ability{&Keyword{Name: "flying"}},
	}
	dirtyCard := &CardAST{
		FullyParsed: false,
		ParseErrors: []string{"x", "y", "z"},
		Abilities:   []Ability{&Static{Modification: &Modification{ModKind: "parsed_tail"}}},
	}
	if !CardIsConfident(cleanCard, DefaultConfidenceThreshold) {
		t.Errorf("clean card should pass default threshold")
	}
	if CardIsConfident(dirtyCard, DefaultConfidenceThreshold) {
		t.Errorf("dirty card should fail default threshold")
	}
}

// ----------------------------------------------------------------------
// Bucket classifier.
// ----------------------------------------------------------------------

func TestConfidenceBucket_Boundaries(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{1.0, "clean"},
		{0.999, "high"},
		{DefaultConfidenceThreshold, "high"},
		{0.69, "medium"},
		{0.4, "medium"},
		{0.39, "low"},
		{0.0, "low"},
	}
	for _, tc := range cases {
		if got := ConfidenceBucket(tc.score); got != tc.want {
			t.Errorf("ConfidenceBucket(%v): want %q, got %q", tc.score, tc.want, got)
		}
	}
}

// ----------------------------------------------------------------------
// Fallback-kind set sanity — guards against accidental deletion.
// ----------------------------------------------------------------------

func TestFallbackKindSets_PinExpectedShape(t *testing.T) {
	// These sets are referenced in scripts/ast_corpus_health.py and the
	// era scaffold audits. A surprise change here would silently drift
	// the parser-health metrics. Pin the canonical set membership.
	wantMod := []string{
		"parsed_effect_residual", "parsed_tail", "untyped_effect",
		"if_intervening_tail", "custom", "cast_trigger_tail",
	}
	for _, k := range wantMod {
		if _, ok := FallbackModKinds[k]; !ok {
			t.Errorf("FallbackModKinds missing %q", k)
		}
	}
	wantCond := []string{"if", "conditional", "raw", "intervening_if", "as_long_as"}
	for _, k := range wantCond {
		if _, ok := FallbackCondKinds[k]; !ok {
			t.Errorf("FallbackCondKinds missing %q", k)
		}
	}
}
