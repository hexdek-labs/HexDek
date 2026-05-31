package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Synthetic CardAST fixtures.
// ---------------------------------------------------------------------------

func cardClean(name string) *gameast.CardAST {
	return &gameast.CardAST{
		Name:        name,
		FullyParsed: true,
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{ModKind: "anthem"}},
		},
	}
}

func cardKeyword(name string) *gameast.CardAST {
	return &gameast.CardAST{
		Name:        name,
		FullyParsed: true,
		Abilities:   []gameast.Ability{&gameast.Keyword{Name: "flying"}},
	}
}

func cardBoundary(name string) *gameast.CardAST {
	// Single -0.5 mod-kind penalty → score 0.50 (at engine threshold).
	return &gameast.CardAST{
		Name:        name,
		FullyParsed: true,
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{ModKind: "parsed_tail"}},
		},
	}
}

func cardStacked(name string) *gameast.CardAST {
	// Stacked -0.5 mod + -0.3 cond → score 0.20.
	return &gameast.CardAST{
		Name:        name,
		FullyParsed: true,
		Abilities: []gameast.Ability{
			&gameast.Static{
				Modification: &gameast.Modification{ModKind: "parsed_effect_residual"},
				Condition:    &gameast.Condition{Kind: "conditional"},
			},
		},
	}
}

func cardMixedTwo(name string, kind1, kind2 string) *gameast.CardAST {
	return &gameast.CardAST{
		Name:        name,
		FullyParsed: true,
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{ModKind: kind1}},
			&gameast.Static{Modification: &gameast.Modification{ModKind: kind2}},
		},
	}
}

func cardEmpty(name string) *gameast.CardAST {
	return &gameast.CardAST{Name: name, FullyParsed: true}
}

// ---------------------------------------------------------------------------
// buildExplorerEntry — per-card row construction.
// ---------------------------------------------------------------------------

func TestBuildExplorerEntry_PerAbilityScores(t *testing.T) {
	card := cardMixedTwo("Mixed", "anthem", "parsed_tail")
	entry := buildExplorerEntry(card)
	if entry.NumAbilities != 2 {
		t.Errorf("NumAbilities: want 2, got %d", entry.NumAbilities)
	}
	if entry.NumFallback != 1 {
		t.Errorf("NumFallback: want 1, got %d", entry.NumFallback)
	}
	if len(entry.Abilities) != 2 {
		t.Fatalf("Abilities len: want 2, got %d", len(entry.Abilities))
	}
	if entry.Abilities[0].Score != 1.0 {
		t.Errorf("ability[0] score: want 1.0, got %v", entry.Abilities[0].Score)
	}
	if entry.Abilities[1].Score != 0.5 {
		t.Errorf("ability[1] score: want 0.5, got %v", entry.Abilities[1].Score)
	}
	if entry.Abilities[0].Kind != "static" {
		t.Errorf("ability[0] kind: want static, got %q", entry.Abilities[0].Kind)
	}
}

func TestBuildExplorerEntry_NilAbilitySafe(t *testing.T) {
	card := &gameast.CardAST{
		Name:        "Nil Ability",
		FullyParsed: true,
		Abilities:   []gameast.Ability{nil},
	}
	entry := buildExplorerEntry(card)
	if len(entry.Abilities) != 1 {
		t.Fatalf("want 1 ability row, got %d", len(entry.Abilities))
	}
	if entry.Abilities[0].Kind != "?" {
		t.Errorf("nil ability Kind: want ?, got %q", entry.Abilities[0].Kind)
	}
	if entry.Abilities[0].Score != 1.0 {
		t.Errorf("nil ability scores 1.0 (fail-open) by gameast contract; got %v", entry.Abilities[0].Score)
	}
}

// ---------------------------------------------------------------------------
// CollectExplorerEntriesFromCards — sort + cap + filter.
// ---------------------------------------------------------------------------

func TestCollectExplorerEntries_SortedAscending(t *testing.T) {
	cards := []*gameast.CardAST{
		cardBoundary("B"),  // 0.50
		cardStacked("S"),   // 0.20
		cardClean("C"),     // 1.00 — filtered
		cardKeyword("K"),   // 1.00 — filtered (keyword scores clean)
	}
	got := CollectExplorerEntriesFromCards(cards, 0)
	if len(got) != 2 {
		t.Fatalf("want 2 entries (clean filtered), got %d", len(got))
	}
	if got[0].Name != "S" || got[0].Score != 0.2 {
		t.Errorf("worst entry: want (S, 0.20), got (%s, %v)", got[0].Name, got[0].Score)
	}
	if got[1].Name != "B" || got[1].Score != 0.5 {
		t.Errorf("second entry: want (B, 0.50), got (%s, %v)", got[1].Name, got[1].Score)
	}
}

func TestCollectExplorerEntries_SkipsEmptyAndCleanCards(t *testing.T) {
	cards := []*gameast.CardAST{
		cardEmpty("E"),
		cardClean("C"),
		cardStacked("S"),
	}
	got := CollectExplorerEntriesFromCards(cards, 0)
	if len(got) != 1 {
		t.Fatalf("want 1 entry (only stacked), got %d", len(got))
	}
	if got[0].Name != "S" {
		t.Errorf("want S, got %s", got[0].Name)
	}
}

func TestCollectExplorerEntries_LimitClamps(t *testing.T) {
	cards := []*gameast.CardAST{
		cardStacked("A"),
		cardStacked("B"),
		cardStacked("C"),
		cardStacked("D"),
	}
	got := CollectExplorerEntriesFromCards(cards, 2)
	if len(got) != 2 {
		t.Fatalf("limit=2: want 2, got %d", len(got))
	}
	// Tie-breaker on Name (alphabetical) when Scores are equal.
	if got[0].Name != "A" || got[1].Name != "B" {
		t.Errorf("tiebreak: want A,B, got %s,%s", got[0].Name, got[1].Name)
	}
}

func TestCollectExplorerEntries_ZeroLimitMeansUnlimited(t *testing.T) {
	cards := []*gameast.CardAST{
		cardStacked("A"), cardStacked("B"), cardStacked("C"),
	}
	got := CollectExplorerEntriesFromCards(cards, 0)
	if len(got) != 3 {
		t.Errorf("limit=0 (unlimited): want 3, got %d", len(got))
	}
}

func TestCollectExplorerEntries_NilCorpusSafe(t *testing.T) {
	got := CollectExplorerEntries(nil, 50)
	if got != nil {
		t.Errorf("nil corpus: want nil, got %v", got)
	}
}

func TestCollectExplorerEntries_NilCardSafe(t *testing.T) {
	got := CollectExplorerEntriesFromCards([]*gameast.CardAST{nil, cardStacked("S")}, 0)
	if len(got) != 1 {
		t.Errorf("nil card in slice: want skipped, got %d entries", len(got))
	}
}

// ---------------------------------------------------------------------------
// RenderExplorerMarkdown — output shape.
// ---------------------------------------------------------------------------

func TestRenderExplorerMarkdown_EmitsTitleAndEntries(t *testing.T) {
	entries := CollectExplorerEntriesFromCards([]*gameast.CardAST{cardStacked("Test Card")}, 0)
	var buf bytes.Buffer
	if err := RenderExplorerMarkdown(&buf, entries, ""); err != nil {
		t.Fatalf("RenderExplorerMarkdown: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "# AST Confidence Explorer — bottom 1 cards") {
		t.Errorf("missing default title; got:\n%s", out)
	}
	if !strings.Contains(out, "## #1 Test Card") {
		t.Errorf("missing entry header; got:\n%s", out)
	}
	if !strings.Contains(out, "static_fallback_mod_kind:parsed_effect_residual") {
		t.Errorf("missing reason label; got:\n%s", out)
	}
	if !strings.Contains(out, "static_fallback_cond_kind:conditional") {
		t.Errorf("missing condition reason; got:\n%s", out)
	}
}

func TestRenderExplorerMarkdown_CustomTitle(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderExplorerMarkdown(&buf, nil, "# Custom Header"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "# Custom Header") {
		t.Errorf("custom title not used; got:\n%s", buf.String())
	}
}

func TestRenderExplorerMarkdown_EmptyEntries(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderExplorerMarkdown(&buf, nil, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "no low-confidence cards") {
		t.Errorf("empty entries: want fallback message, got:\n%s", buf.String())
	}
}

func TestRenderExplorerMarkdown_PerAbilityKindLabel(t *testing.T) {
	cards := []*gameast.CardAST{
		{
			Name:        "Multi",
			FullyParsed: true,
			Abilities: []gameast.Ability{
				&gameast.Static{Modification: &gameast.Modification{ModKind: "parsed_tail"}},
				&gameast.Triggered{
					Trigger: gameast.Trigger{Event: "etb"},
					Effect:  &gameast.UnknownEffect{RawText: "..."},
				},
				&gameast.Activated{Effect: &gameast.UnknownEffect{RawText: "..."}},
			},
		},
	}
	entries := CollectExplorerEntriesFromCards(cards, 0)
	var buf bytes.Buffer
	_ = RenderExplorerMarkdown(&buf, entries, "")
	out := buf.String()
	for _, want := range []string{"(static, score", "(triggered, score", "(activated, score"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing kind label %q; got:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// AggregateReasons + RenderReasonSummary.
// ---------------------------------------------------------------------------

func TestAggregateReasons_CountsAcrossEntries(t *testing.T) {
	cards := []*gameast.CardAST{
		cardStacked("A"), // mod + cond reasons
		cardStacked("B"), // same shape
		cardBoundary("C"), // single mod reason (parsed_tail)
	}
	entries := CollectExplorerEntriesFromCards(cards, 0)
	hist := AggregateReasons(entries)
	if hist["static_fallback_mod_kind:parsed_effect_residual"] != 2 {
		t.Errorf("mod_kind:parsed_effect_residual: want 2, got %d", hist["static_fallback_mod_kind:parsed_effect_residual"])
	}
	if hist["static_fallback_cond_kind:conditional"] != 2 {
		t.Errorf("cond_kind:conditional: want 2, got %d", hist["static_fallback_cond_kind:conditional"])
	}
	if hist["static_fallback_mod_kind:parsed_tail"] != 1 {
		t.Errorf("mod_kind:parsed_tail: want 1, got %d", hist["static_fallback_mod_kind:parsed_tail"])
	}
}

func TestRenderReasonSummary_SortedDescByCount(t *testing.T) {
	cards := []*gameast.CardAST{
		cardStacked("A"), cardStacked("B"), cardStacked("C"),
		cardBoundary("D"),
	}
	entries := CollectExplorerEntriesFromCards(cards, 0)
	var buf bytes.Buffer
	if err := RenderReasonSummary(&buf, entries); err != nil {
		t.Fatalf("err: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "## Aggregate reason histogram") {
		t.Errorf("missing summary header; got:\n%s", out)
	}
	// Higher-count reason must appear before lower-count reason.
	stackedIdx := strings.Index(out, "static_fallback_mod_kind:parsed_effect_residual")
	tailIdx := strings.Index(out, "static_fallback_mod_kind:parsed_tail")
	if stackedIdx < 0 || tailIdx < 0 {
		t.Fatalf("missing rows; got:\n%s", out)
	}
	if stackedIdx > tailIdx {
		t.Errorf("count-desc ordering violated: higher-count row should appear first\n%s", out)
	}
}

func TestRenderReasonSummary_EmptyEntriesNoOp(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderReasonSummary(&buf, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("empty input: want no output, got %q", buf.String())
	}
}
