package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// fakeCorpus is the test double for ASTLookup. astload.Corpus is real-disk
// only and unwieldy in tests; this map-backed shim is all Analyze touches.
type fakeCorpus map[string]*gameast.CardAST

func (f fakeCorpus) Get(name string) (*gameast.CardAST, bool) {
	c, ok := f[name]
	return c, ok
}

// mkAST builds a CardAST whose Abilities slice carries one Keyword node
// per name passed in. Used to populate the "AST emitted this keyword"
// side of each test case.
func mkAST(name string, keywords ...string) *gameast.CardAST {
	abs := make([]gameast.Ability, 0, len(keywords))
	for _, k := range keywords {
		abs = append(abs, &gameast.Keyword{Name: k})
	}
	return &gameast.CardAST{Name: name, Abilities: abs}
}

// TestAnalyze_CleanCorpus_NoDrift confirms the happy path: when every
// oracle keyword IS present in the AST, the report has zero entries and
// all tallies are empty. Anti-regression for false-positive drift.
func TestAnalyze_CleanCorpus_NoDrift(t *testing.T) {
	entries := []OracleEntry{
		{Name: "Storm Crow", TypeLine: "Creature — Bird", OracleText: "Flying"},
		{Name: "Lightning Bolt", TypeLine: "Instant", OracleText: "Lightning Bolt deals 3 damage to any target."},
	}
	corpus := fakeCorpus{
		"Storm Crow":      mkAST("Storm Crow", "Flying"),
		"Lightning Bolt":  mkAST("Lightning Bolt"),
	}

	r := Analyze(entries, corpus, nil)
	if len(r.Entries) != 0 {
		t.Errorf("expected zero drift entries, got %d: %+v", len(r.Entries), r.Entries)
	}
	if r.CardsScanned != 2 {
		t.Errorf("CardsScanned = %d, want 2", r.CardsScanned)
	}
	if len(r.PerKeyword)+len(r.PerEra)+len(r.PerCardType) != 0 {
		t.Errorf("tallies should be empty: kw=%v era=%v ct=%v",
			r.PerKeyword, r.PerEra, r.PerCardType)
	}
}

// TestAnalyze_PerKeyword_Tally pins the legacy per-keyword aggregation:
// one card with two missed keywords contributes 1 to each keyword's count,
// not 2 to a single bucket. Two cards missing the same keyword should
// stack.
func TestAnalyze_PerKeyword_Tally(t *testing.T) {
	entries := []OracleEntry{
		{Name: "A", TypeLine: "Creature", OracleText: "Flying\nTrample"},
		{Name: "B", TypeLine: "Creature", OracleText: "Flying"},
	}
	corpus := fakeCorpus{
		"A": mkAST("A"), // AST emits nothing — both keywords missed
		"B": mkAST("B"),
	}

	r := Analyze(entries, corpus, nil)
	if r.PerKeyword["flying"] != 2 {
		t.Errorf("flying tally = %d, want 2", r.PerKeyword["flying"])
	}
	if r.PerKeyword["trample"] != 1 {
		t.Errorf("trample tally = %d, want 1", r.PerKeyword["trample"])
	}
}

// TestAnalyze_PerEra_Tally is the load-bearing r60 addition: missing-
// keyword events must land in the correct era bucket. The classifier is
// keyword-driven (era 4 = "discover", era 3 = "mutate"/"foretell", era 2 =
// "crew"/"adapt", era 1 = default). A creature with "mutate" in oracle
// text but no AST Keyword should bump era3.
func TestAnalyze_PerEra_Tally(t *testing.T) {
	entries := []OracleEntry{
		// Era 1: vanilla flying creature.
		{Name: "Era1 Card", TypeLine: "Creature — Bird", OracleText: "Flying"},
		// Era 2: crew vehicle.
		{Name: "Era2 Vehicle", TypeLine: "Artifact — Vehicle", OracleText: "Crew 3"},
		// Era 3: mutate creature.
		{Name: "Era3 Mutator", TypeLine: "Creature — Beast",
			OracleText: "Mutate 2{G}\nFlying"},
		// Era 4: discover spell.
		{Name: "Era4 Spell", TypeLine: "Sorcery", OracleText: "Discover 3.\nFlashback {3}{R}"},
	}
	corpus := fakeCorpus{
		"Era1 Card":     mkAST("Era1 Card"),
		"Era2 Vehicle":  mkAST("Era2 Vehicle"),
		"Era3 Mutator":  mkAST("Era3 Mutator"),
		"Era4 Spell":    mkAST("Era4 Spell"),
	}

	r := Analyze(entries, corpus, nil)

	if r.PerEra["era1"] < 1 {
		t.Errorf("era1 should have ≥1 missing-keyword event (flying), got %d: %v",
			r.PerEra["era1"], r.PerEra)
	}
	if r.PerEra["era2"] < 1 {
		t.Errorf("era2 should have ≥1 missing-keyword event (crew), got %d", r.PerEra["era2"])
	}
	if r.PerEra["era3"] < 1 {
		t.Errorf("era3 should have ≥1 missing-keyword event (mutate), got %d", r.PerEra["era3"])
	}
	if r.PerEra["era4"] < 1 {
		t.Errorf("era4 should have ≥1 missing-keyword event (discover), got %d", r.PerEra["era4"])
	}
}

// TestAnalyze_PerCardType_Tally confirms the second r60 dimension: missing
// keywords on a creature land in "creature", on an instant in "instant",
// etc. The dual-type case (Artifact Creature) must land in "creature" per
// primaryCardType's priority — keyword gaps cluster on combat-keyword
// coverage, which only matters when the card actually fights.
func TestAnalyze_PerCardType_Tally(t *testing.T) {
	entries := []OracleEntry{
		{Name: "Flyer", TypeLine: "Creature — Bird", OracleText: "Flying"},
		{Name: "Bolt", TypeLine: "Instant", OracleText: "Storm"},
		{Name: "Walker", TypeLine: "Legendary Planeswalker — Jace",
			OracleText: "Cascade"},
		{Name: "Artiprowl", TypeLine: "Artifact Creature — Construct",
			OracleText: "Prowl"},
	}
	corpus := fakeCorpus{
		"Flyer":     mkAST("Flyer"),
		"Bolt":      mkAST("Bolt"),
		"Walker":    mkAST("Walker"),
		"Artiprowl": mkAST("Artiprowl"),
	}

	r := Analyze(entries, corpus, nil)

	if r.PerCardType["creature"] < 2 {
		t.Errorf("creature should absorb both vanilla and artifact-creature: got %d",
			r.PerCardType["creature"])
	}
	if r.PerCardType["instant"] != 1 {
		t.Errorf("instant should have exactly 1: got %d", r.PerCardType["instant"])
	}
	if r.PerCardType["planeswalker"] != 1 {
		t.Errorf("planeswalker should have exactly 1: got %d", r.PerCardType["planeswalker"])
	}
	if r.PerCardType["artifact"] != 0 {
		t.Errorf("artifact bucket should be empty (creature wins over artifact): got %d",
			r.PerCardType["artifact"])
	}
}

// TestAnalyze_TallyConservation is an invariant pin: summed events across
// PerKeyword, PerEra, PerCardType must all equal each other and equal the
// total missing-keyword event count. If any future refactor introduces a
// bucket-routing bug (e.g. classifyEra returns "" instead of "era1"),
// this catches it.
func TestAnalyze_TallyConservation(t *testing.T) {
	entries := []OracleEntry{
		{Name: "A", TypeLine: "Creature — Bird", OracleText: "Flying\nTrample\nVigilance"},
		{Name: "B", TypeLine: "Instant", OracleText: "Storm\nCascade"},
		{Name: "C", TypeLine: "Creature — Mutant",
			OracleText: "Mutate 2{G}\nFlying"},
	}
	corpus := fakeCorpus{
		"A": mkAST("A"),
		"B": mkAST("B"),
		"C": mkAST("C", "Flying"), // C has Flying — only Mutate misses
	}

	r := Analyze(entries, corpus, nil)

	kw := sumValues(r.PerKeyword)
	era := sumValues(r.PerEra)
	ct := sumValues(r.PerCardType)
	if kw != era || era != ct {
		t.Errorf("tallies disagree: kw=%d era=%d ct=%d", kw, era, ct)
	}
	// Counts: A=3, B=2, C=1 → total 6.
	if kw != 6 {
		t.Errorf("expected 6 total missing-keyword events, got %d (entries=%+v)", kw, r.Entries)
	}
}

// TestAnalyze_UnSetsExcluded confirms the joke-set filter still works
// after the refactor: cards from "Unfinity" et al. must not contribute
// to ANY tally, even if their oracle text mentions a keyword the AST
// missed.
func TestAnalyze_UnSetsExcluded(t *testing.T) {
	entries := []OracleEntry{
		{Name: "Real Card", SetName: "Tarkir Dragonstorm",
			TypeLine: "Creature — Bird", OracleText: "Flying"},
		{Name: "Joke Card", SetName: "Unfinity",
			TypeLine: "Creature — Squirrel", OracleText: "Flying"},
	}
	corpus := fakeCorpus{
		"Real Card": mkAST("Real Card"),
		"Joke Card": mkAST("Joke Card"),
	}
	unSets := map[string]bool{"Unfinity": true}

	r := Analyze(entries, corpus, unSets)
	if r.PerKeyword["flying"] != 1 {
		t.Errorf("flying tally should be 1 (joke card filtered): got %d", r.PerKeyword["flying"])
	}
	if r.CardsScanned != 1 {
		t.Errorf("CardsScanned = %d, want 1 (joke card skipped)", r.CardsScanned)
	}
}

// TestAnalyze_MissingFromCorpus_Skipped pins: oracle entries that have no
// AST entry are SKIPPED (the parser hasn't even seen the card, so it
// shouldn't be counted as a parser miss). This is what the `if !ok
// { continue }` line guarantees.
func TestAnalyze_MissingFromCorpus_Skipped(t *testing.T) {
	entries := []OracleEntry{
		{Name: "Has AST", TypeLine: "Creature", OracleText: "Flying"},
		{Name: "No AST", TypeLine: "Creature", OracleText: "Trample"},
	}
	corpus := fakeCorpus{
		"Has AST": mkAST("Has AST"),
	}

	r := Analyze(entries, corpus, nil)
	if r.CardsScanned != 1 {
		t.Errorf("CardsScanned = %d, want 1 (only Has AST has a corpus entry)", r.CardsScanned)
	}
	if r.PerKeyword["trample"] != 0 {
		t.Errorf("trample should NOT be tallied (No AST is not in corpus): got %d",
			r.PerKeyword["trample"])
	}
}

// TestAnalyze_DuplicateName_Deduped confirms the seen-name guard: if
// Scryfall ships duplicate name entries (basic lands, promo printings),
// only the first contributes. The second is silently dropped.
func TestAnalyze_DuplicateName_Deduped(t *testing.T) {
	entries := []OracleEntry{
		{Name: "Dragon", TypeLine: "Creature — Dragon", OracleText: "Flying"},
		{Name: "Dragon", TypeLine: "Creature — Dragon", OracleText: "Flying"},
	}
	corpus := fakeCorpus{"Dragon": mkAST("Dragon")}

	r := Analyze(entries, corpus, nil)
	if r.CardsScanned != 1 {
		t.Errorf("CardsScanned = %d, want 1 (dup name deduped)", r.CardsScanned)
	}
	if r.PerKeyword["flying"] != 1 {
		t.Errorf("flying should tally exactly once: got %d", r.PerKeyword["flying"])
	}
}

// TestClassifyEra_FallthroughBuckets pins the era classifier on its own,
// independent of Analyze. Tests the priority order (era 4 markers beat
// era 3 beat era 2 beat era 1) by stacking markers from multiple eras.
func TestClassifyEra_FallthroughBuckets(t *testing.T) {
	cases := []struct {
		text, typeLine, want string
	}{
		{"Flying", "Creature", "era1"},
		{"Crew 3", "Artifact — Vehicle", "era2"},
		{"Mutate 1G", "Creature", "era3"},
		{"Discover 3", "Sorcery", "era4"},
		// Era 4 markers must dominate era 3 markers when both appear.
		{"Mutate 1G. Discover 3.", "Creature", "era4"},
		// Era 3 marker beats era 2.
		{"Crew 3. Mutate 1G.", "Artifact Creature — Vehicle", "era3"},
	}
	for _, c := range cases {
		got := classifyEra(c.text, c.typeLine)
		if got != c.want {
			t.Errorf("classifyEra(%q, %q) = %q, want %q", c.text, c.typeLine, got, c.want)
		}
	}
}

// TestPrimaryCardType_DualTypePriority pins the dual-type tie-breaker:
// "Artifact Creature" must return "creature" so the combat-keyword bias
// holds. Land sits last so "Creature Land" still routes to creature.
func TestPrimaryCardType_DualTypePriority(t *testing.T) {
	cases := []struct {
		typeLine, want string
	}{
		{"Creature — Bird", "creature"},
		{"Instant", "instant"},
		{"Legendary Planeswalker — Jace", "planeswalker"},
		{"Artifact Creature — Construct", "creature"},
		{"Enchantment Creature — God", "creature"},
		{"Land — Forest", "land"},
		{"Tribal Instant — Goblin", "instant"},
		{"", "other"},
	}
	for _, c := range cases {
		got := primaryCardType(c.typeLine)
		if got != c.want {
			t.Errorf("primaryCardType(%q) = %q, want %q", c.typeLine, got, c.want)
		}
	}
}
