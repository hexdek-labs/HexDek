package astload

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/gameast"
)

// corpusPath locates data/rules/ast_dataset.jsonl relative to the repo
// root. Test files run from the package dir, so walk up until we find it.
func corpusPath(t testing.TB) string {
	t.Helper()
	// package dir: .../sandbox/mtgsquad/internal/astload
	// corpus:       .../sandbox/mtgsquad/data/rules/ast_dataset.jsonl
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "data", "rules", "ast_dataset.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skipf("ast_dataset.jsonl not found within 6 parents of %s; skipping", dir)
	return ""
}

// TestLoadInlineSample verifies the loader handles a synthetic JSONL blob.
// No filesystem needed — this catches regressions in the discriminator
// dispatch itself without depending on the 40 MB corpus.
func TestLoadInlineSample(t *testing.T) {
	// A Lightning-Bolt-shaped row with an Activated/Damage structure
	// (not the actual corpus shape, which wraps Bolt in a Static) — we
	// want to verify the Activated + Damage branches fire.
	const sample = `{"name":"Lightning Bolt","oracle_text":"Lightning Bolt deals 3 damage to any target.","type_line":"Instant","mana_cost":"{R}","cmc":1.0,"colors":["R"],"ast":{"__ast_type__":"CardAST","name":"Lightning Bolt","abilities":[{"__ast_type__":"Activated","cost":{"__ast_type__":"Cost","mana":{"__ast_type__":"ManaCost","symbols":[{"__ast_type__":"ManaSymbol","raw":"{R}","generic":0,"color":["R"],"is_x":false,"is_phyrexian":false,"is_snow":false}]},"tap":false,"untap":false,"sacrifice":null,"discard":null,"pay_life":null,"exile_self":false,"return_self_to_hand":false,"remove_counters":null,"extra":[]},"effect":{"__ast_type__":"Damage","kind":"damage","amount":3,"target":{"__ast_type__":"Filter","base":"any_target","quantifier":"one","count":null,"targeted":true,"you_control":false,"opponent_controls":false,"nontoken":false,"creature_types":[],"color_filter":[],"color_exclude":[],"mana_value_op":null,"mana_value":null,"extra":[]},"divided":false},"timing_restriction":null,"raw":"{r}: ~ deals 3 damage to any target"}],"parse_errors":[],"fully_parsed":true}}`
	// Adjacent record: ScalingAmount exercise (GainLife amount=scaling).
	const sampleScaling = `{"name":"Synth Merchant","oracle_text":"","type_line":"Creature","mana_cost":"{3}{B}{B}","cmc":5.0,"colors":["B"],"ast":{"__ast_type__":"CardAST","name":"Synth Merchant","abilities":[{"__ast_type__":"Triggered","trigger":{"__ast_type__":"Trigger","event":"etb","actor":null,"target_filter":null,"phase":null,"controller":null,"condition":null},"effect":{"__ast_type__":"GainLife","kind":"gain_life","amount":{"__ast_type__":"ScalingAmount","kind":"devotion","args":["B"]},"target":{"__ast_type__":"Filter","base":"self","quantifier":"one","count":null,"targeted":false,"you_control":false,"opponent_controls":false,"nontoken":false,"creature_types":[],"color_filter":[],"color_exclude":[],"mana_value_op":null,"mana_value":null,"extra":[]}},"intervening_if":null,"raw":"when this creature enters, you gain life equal to your devotion to black"}],"parse_errors":[],"fully_parsed":true}}`
	blob := sample + "\n" + sampleScaling + "\n"

	c, err := LoadReader(bytes.NewBufferString(blob))
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if c.Count() != 2 {
		t.Fatalf("expected 2 cards, got %d (warnings: %v)", c.Count(), c.ParseWarnings)
	}

	// Lightning Bolt: Activated + Damage + R mana cost.
	bolt, ok := c.Get("Lightning Bolt")
	if !ok {
		t.Fatalf("Lightning Bolt not found")
	}
	if len(bolt.Abilities) != 1 {
		t.Fatalf("expected 1 ability, got %d", len(bolt.Abilities))
	}
	act, ok := bolt.Abilities[0].(*gameast.Activated)
	if !ok {
		t.Fatalf("expected *Activated, got %T", bolt.Abilities[0])
	}
	if act.Cost.Mana == nil {
		t.Fatalf("expected mana cost, got nil")
	}
	if act.Cost.Mana.CMC() != 1 {
		t.Errorf("expected CMC 1, got %d", act.Cost.Mana.CMC())
	}
	dmg, ok := act.Effect.(*gameast.Damage)
	if !ok {
		t.Fatalf("expected *Damage, got %T", act.Effect)
	}
	if v, ok := dmg.Amount.IntVal(); !ok || v != 3 {
		t.Errorf("expected Damage amount 3, got (%v, %v)", v, ok)
	}
	if dmg.Target.Base != "any_target" {
		t.Errorf("expected target base any_target, got %q", dmg.Target.Base)
	}

	// Synth Merchant: Triggered + GainLife with ScalingAmount.
	merchant, ok := c.Get("Synth Merchant")
	if !ok {
		t.Fatalf("Synth Merchant not found")
	}
	trig, ok := merchant.Abilities[0].(*gameast.Triggered)
	if !ok {
		t.Fatalf("expected *Triggered, got %T", merchant.Abilities[0])
	}
	gl, ok := trig.Effect.(*gameast.GainLife)
	if !ok {
		t.Fatalf("expected *GainLife, got %T", trig.Effect)
	}
	sa, ok := gl.Amount.ScalingVal()
	if !ok {
		t.Fatalf("expected ScalingAmount on GainLife.Amount, got %+v", gl.Amount)
	}
	if sa.ScalingKind != "devotion" {
		t.Errorf("expected devotion, got %q", sa.ScalingKind)
	}
	if len(sa.Args) != 1 {
		t.Fatalf("expected 1 scaling arg, got %d", len(sa.Args))
	}
	if s, ok := sa.Args[0].(string); !ok || s != "B" {
		t.Errorf("expected arg 'B', got %v", sa.Args[0])
	}

	// Signature should be non-empty for both.
	if bolt.Signature() == "" || merchant.Signature() == "" {
		t.Errorf("empty signature on decoded card")
	}
}

// TestLoadFullCorpus loads the entire ast_dataset.jsonl and verifies:
//   - all rows loaded (count matches the file line count)
//   - load time is under the budget
//   - memory allocation is reasonable
//   - every canonical card parses with the expected structural shape.
func TestLoadFullCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: skipping full corpus load")
	}
	path := corpusPath(t)
	if path == "" {
		return
	}

	// Line count to validate Count() matches the file.
	lineCount, err := countLines(path)
	if err != nil {
		t.Fatalf("count lines: %v", err)
	}

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	t0 := time.Now()
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	elapsed := time.Since(t0)

	runtime.ReadMemStats(&memAfter)
	usedMB := float64(memAfter.HeapAlloc-memBefore.HeapAlloc) / (1024.0 * 1024.0)
	if usedMB < 0 {
		usedMB = float64(memAfter.HeapAlloc) / (1024.0 * 1024.0)
	}

	t.Logf("corpus: %d cards loaded from %s (%.1f MiB bytes)",
		c.Count(), path, float64(c.SourceBytes)/(1024.0*1024.0))
	t.Logf("load time: %s (Load() reported %s)", elapsed, c.LoadDuration)
	t.Logf("heap after load: +%.1f MiB (total %.1f MiB)",
		usedMB, float64(memAfter.HeapAlloc)/(1024.0*1024.0))
	if n := len(c.ParseWarnings); n > 0 {
		t.Logf("parse warnings: %d (first few below)", n)
		for i, w := range c.ParseWarnings {
			if i >= 5 {
				break
			}
			t.Logf("  warning: %s", w)
		}
	}

	// Count matches the file's line count (every line with content should
	// have produced a card). Allow a small tolerance for blank lines.
	if c.Count() < lineCount-5 || c.Count() > lineCount {
		t.Errorf("Count() = %d; expected ~%d (file lines)", c.Count(), lineCount)
	}

	// Load-time budget: 5 seconds on a modern machine (task spec). CI-safe
	// upper bound to avoid false negatives on a busy machine.
	if elapsed > 10*time.Second {
		t.Errorf("load time %s exceeds 10s budget", elapsed)
	}
	// Memory budget: < 500 MB on a 40 MB file (task spec).
	if usedMB > 500 {
		t.Errorf("heap after load %.1f MiB exceeds 500 MiB budget", usedMB)
	}

	verifyCanonicalCards(t, c)

	// Names() returns a sorted, non-empty slice.
	names := c.Names()
	if len(names) != c.Count() {
		t.Errorf("Names() len %d != Count() %d", len(names), c.Count())
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("Names() output is not sorted")
	}
}

// verifyCanonicalCards asserts specific structural properties on well-known
// cards. These are the "10 canonical" checks from the task spec — each one
// stresses a different corner of the discriminator dispatch.
func verifyCanonicalCards(t *testing.T, c *Corpus) {
	t.Helper()

	// Lightning Bolt — Static (Modification kind=spell_effect) in the current
	// corpus. Parser extensions promote direct-damage spells to spell_effect.
	bolt, ok := c.Get("Lightning Bolt")
	if !ok {
		t.Fatalf("Lightning Bolt not found")
	}
	if len(bolt.Abilities) != 1 {
		t.Errorf("Lightning Bolt: expected 1 ability, got %d", len(bolt.Abilities))
	} else if s, ok := bolt.Abilities[0].(*gameast.Static); !ok {
		t.Errorf("Lightning Bolt: expected *Static, got %T", bolt.Abilities[0])
	} else if s.Modification == nil || s.Modification.ModKind != "spell_effect" {
		t.Errorf("Lightning Bolt: expected spell_effect modification, got %+v", s.Modification)
	}

	// Counterspell — Static wrapping a CounterSpell effect in
	// Modification.args (kind=spell_effect).
	counter, ok := c.Get("Counterspell")
	if !ok {
		t.Fatalf("Counterspell not found")
	}
	if len(counter.Abilities) != 1 {
		t.Errorf("Counterspell: expected 1 ability, got %d", len(counter.Abilities))
	} else if s, ok := counter.Abilities[0].(*gameast.Static); !ok {
		t.Errorf("Counterspell: expected *Static, got %T", counter.Abilities[0])
	} else {
		if s.Modification == nil || s.Modification.ModKind != "spell_effect" {
			t.Errorf("Counterspell: expected spell_effect modification, got %+v", s.Modification)
		} else {
			// args[0] should be a *CounterSpell effect.
			if len(s.Modification.Args) == 0 {
				t.Errorf("Counterspell: expected at least 1 arg on modification")
			} else if _, ok := s.Modification.Args[0].(*gameast.CounterSpell); !ok {
				t.Errorf("Counterspell: expected *CounterSpell at args[0], got %T", s.Modification.Args[0])
			}
		}
	}

	// Thassa's Oracle — Triggered + Static(spell_effect=Tutor) + Static(if_intervening_tail).
	thassa, ok := c.Get("Thassa's Oracle")
	if !ok {
		t.Fatalf("Thassa's Oracle not found")
	}
	if len(thassa.Abilities) != 3 {
		t.Errorf("Thassa's Oracle: expected 3 abilities, got %d", len(thassa.Abilities))
	} else {
		if _, ok := thassa.Abilities[0].(*gameast.Triggered); !ok {
			t.Errorf("Thassa's Oracle: ability 0 expected *Triggered, got %T", thassa.Abilities[0])
		}
		if s, ok := thassa.Abilities[1].(*gameast.Static); !ok {
			t.Errorf("Thassa's Oracle: ability 1 expected *Static, got %T", thassa.Abilities[1])
		} else {
			if s.Modification == nil || s.Modification.ModKind != "spell_effect" {
				t.Errorf("Thassa's Oracle: ability 1 modification not spell_effect: %+v", s.Modification)
			} else if len(s.Modification.Args) == 0 {
				t.Errorf("Thassa's Oracle: ability 1 modification has empty args")
			} else if _, ok := s.Modification.Args[0].(*gameast.Tutor); !ok {
				t.Errorf("Thassa's Oracle: ability 1 args[0] expected *Tutor, got %T", s.Modification.Args[0])
			}
		}
	}

	// Humility — single Static/parsed_tail.
	humility, ok := c.Get("Humility")
	if !ok {
		t.Fatalf("Humility not found")
	}
	if len(humility.Abilities) != 1 {
		t.Errorf("Humility: expected 1 ability, got %d", len(humility.Abilities))
	}

	// Blood Moon — Static with Modification.kind=nonbasic_become, args=["mountain"].
	blood, ok := c.Get("Blood Moon")
	if !ok {
		t.Fatalf("Blood Moon not found")
	}
	if s, ok := blood.Abilities[0].(*gameast.Static); ok {
		if s.Modification == nil || s.Modification.ModKind != "nonbasic_become" {
			t.Errorf("Blood Moon: expected nonbasic_become, got %+v", s.Modification)
		} else if len(s.Modification.Args) == 0 || s.Modification.Args[0] != "mountain" {
			t.Errorf("Blood Moon: expected args=[mountain], got %v", s.Modification.Args)
		}
	} else {
		t.Errorf("Blood Moon: expected *Static, got %T", blood.Abilities[0])
	}

	// Rest in Peace — 2 abilities: Triggered + Static(replacement_static -> Replacement -> Exile).
	rip, ok := c.Get("Rest in Peace")
	if !ok {
		t.Fatalf("Rest in Peace not found")
	}
	if len(rip.Abilities) != 2 {
		t.Errorf("Rest in Peace: expected 2 abilities, got %d", len(rip.Abilities))
	} else if s, ok := rip.Abilities[1].(*gameast.Static); ok {
		if s.Modification == nil || s.Modification.ModKind != "replacement_static" {
			t.Errorf("Rest in Peace: ability 1 expected replacement_static mod, got %+v", s.Modification)
		} else if len(s.Modification.Args) == 0 {
			t.Errorf("Rest in Peace: ability 1 modification args empty")
		} else if rep, ok := s.Modification.Args[0].(*gameast.Replacement); !ok {
			t.Errorf("Rest in Peace: ability 1 args[0] expected *Replacement, got %T", s.Modification.Args[0])
		} else if _, ok := rep.ReplaceWith.(*gameast.Exile); !ok {
			t.Errorf("Rest in Peace: Replacement.ReplaceWith expected *Exile, got %T", rep.ReplaceWith)
		}
	}

	// Doubling Season — 2 Static abilities with kind=if_intervening_tail.
	ds, ok := c.Get("Doubling Season")
	if !ok {
		t.Fatalf("Doubling Season not found")
	}
	if len(ds.Abilities) != 2 {
		t.Errorf("Doubling Season: expected 2 abilities, got %d", len(ds.Abilities))
	}
	for i, ab := range ds.Abilities {
		s, ok := ab.(*gameast.Static)
		if !ok {
			t.Errorf("Doubling Season: ability %d expected *Static, got %T", i, ab)
			continue
		}
		if s.Modification == nil || s.Modification.ModKind != "if_intervening_tail" {
			t.Errorf("Doubling Season: ability %d expected if_intervening_tail, got %+v", i, s.Modification)
		}
	}

	// Sol Ring — Activated/tap → AddMana pool={C}{C}.
	sol, ok := c.Get("Sol Ring")
	if !ok {
		t.Fatalf("Sol Ring not found")
	}
	if len(sol.Abilities) != 1 {
		t.Errorf("Sol Ring: expected 1 ability, got %d", len(sol.Abilities))
	} else if a, ok := sol.Abilities[0].(*gameast.Activated); !ok {
		t.Errorf("Sol Ring: expected *Activated, got %T", sol.Abilities[0])
	} else {
		if !a.Cost.Tap {
			t.Errorf("Sol Ring: expected tap cost")
		}
		am, ok := a.Effect.(*gameast.AddMana)
		if !ok {
			t.Errorf("Sol Ring: expected *AddMana effect, got %T", a.Effect)
		} else if len(am.Pool) != 2 {
			t.Errorf("Sol Ring: expected pool length 2, got %d", len(am.Pool))
		}
	}

	// Command Tower — single Activated.
	ct, ok := c.Get("Command Tower")
	if !ok {
		t.Fatalf("Command Tower not found")
	}
	if len(ct.Abilities) != 1 {
		t.Errorf("Command Tower: expected 1 ability, got %d", len(ct.Abilities))
	} else if _, ok := ct.Abilities[0].(*gameast.Activated); !ok {
		t.Errorf("Command Tower: expected *Activated, got %T", ct.Abilities[0])
	}

	// Gray Merchant of Asphodel — Triggered + Static. The Triggered effect is
	// an UnknownEffect (parser didn't structure the devotion-scaling yet).
	// The Static carries a spell_effect → GainLife with amount="var".
	gm, ok := c.Get("Gray Merchant of Asphodel")
	if !ok {
		t.Fatalf("Gray Merchant of Asphodel not found")
	}
	if len(gm.Abilities) != 2 {
		t.Errorf("Gray Merchant: expected 2 abilities, got %d", len(gm.Abilities))
	} else if s, ok := gm.Abilities[1].(*gameast.Static); ok {
		if s.Modification != nil && s.Modification.ModKind == "spell_effect" && len(s.Modification.Args) > 0 {
			if gl, ok := s.Modification.Args[0].(*gameast.GainLife); ok {
				if v, ok := gl.Amount.StrVal(); !ok || v != "var" {
					t.Errorf("Gray Merchant GainLife amount: expected StrVal 'var', got (%q, %v)", v, ok)
				}
			} else {
				t.Errorf("Gray Merchant: ability 1 args[0] expected *GainLife, got %T", s.Modification.Args[0])
			}
		}
	}

	// Signature-sanity on all 10 must be non-empty.
	canonical := []string{
		"Lightning Bolt", "Counterspell", "Thassa's Oracle", "Humility",
		"Blood Moon", "Rest in Peace", "Doubling Season", "Sol Ring",
		"Command Tower", "Gray Merchant of Asphodel",
	}
	for _, name := range canonical {
		if card, ok := c.Get(name); !ok {
			t.Errorf("canonical card %q missing", name)
		} else if card.Signature() == "" {
			t.Errorf("canonical card %q has empty signature", name)
		}
	}
}

// TestNameNormalization exercises the accent-folding lookup path.
func TestNameNormalization(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Lightning Bolt", "lightning bolt"},
		{"LIGHTNING BOLT", "lightning bolt"},
		{"  Lightning  Bolt  ", "lightning bolt"},
		{"Jötun Grunt", "jotun grunt"},
		{"Ætherling", "etherling"}, // Æ → e (single-rune fold; Scryfall already normalizes to "Aetherling")
		{"Lim-Dûl the Necromancer", "lim-dul the necromancer"},
		{"Déjà Vu", "deja vu"},
		{"Sen Triplets", "sen triplets"},
		{"", ""},
	}
	for _, tc := range cases {
		got := normalizeName(tc.in)
		if got != tc.want {
			t.Errorf("normalizeName(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// TestUnknownDiscriminatorFallsBack verifies an effect with an unknown
// __ast_type__ becomes UnknownEffect (instead of panicking).
func TestUnknownDiscriminatorFallsBack(t *testing.T) {
	const row = `{"name":"Bogus Card","ast":{"__ast_type__":"CardAST","name":"Bogus Card","abilities":[{"__ast_type__":"Static","condition":null,"modification":{"__ast_type__":"Modification","kind":"spell_effect","args":[{"__ast_type__":"FuturisticNewEffect","kind":"future","raw_text":"does a new thing"}],"layer":null},"raw":"does a new thing"}],"parse_errors":[],"fully_parsed":true}}`
	c, err := LoadReader(strings.NewReader(row))
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if c.Count() != 1 {
		t.Fatalf("expected 1 card, got %d", c.Count())
	}
	bogus, ok := c.Get("Bogus Card")
	if !ok {
		t.Fatalf("Bogus Card not found")
	}
	s, ok := bogus.Abilities[0].(*gameast.Static)
	if !ok {
		t.Fatalf("expected *Static, got %T", bogus.Abilities[0])
	}
	if s.Modification == nil || len(s.Modification.Args) == 0 {
		t.Fatalf("expected modification with args")
	}
	ue, ok := s.Modification.Args[0].(*gameast.UnknownEffect)
	if !ok {
		t.Fatalf("expected *UnknownEffect fallback, got %T", s.Modification.Args[0])
	}
	// The raw JSON should be preserved in RawText so downstream tooling
	// can still inspect it.
	if !strings.Contains(ue.RawText, "FuturisticNewEffect") {
		t.Errorf("expected raw JSON preserved on UnknownEffect.RawText, got %q", ue.RawText)
	}
	if len(c.ParseWarnings) == 0 {
		t.Errorf("expected a parse warning for unknown discriminator")
	}
}

// TestCorpusGetMissing verifies negative lookups.
func TestCorpusGetMissing(t *testing.T) {
	c, err := LoadReader(strings.NewReader(""))
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if card, ok := c.Get("Does Not Exist"); ok || card != nil {
		t.Errorf("expected (nil, false), got (%v, %v)", card, ok)
	}
	if c.Count() != 0 {
		t.Errorf("empty corpus Count() = %d; want 0", c.Count())
	}
	if len(c.Names()) != 0 {
		t.Errorf("empty corpus Names() non-empty")
	}
	var nilCorpus *Corpus
	if card, ok := nilCorpus.Get("anything"); ok || card != nil {
		t.Errorf("nil corpus should return (nil, false)")
	}
	if nilCorpus.Count() != 0 {
		t.Errorf("nil corpus Count() != 0")
	}
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------

// BenchmarkCorpusLoad measures full-corpus load time + allocations.
//
// Run with:
//
//	go test -bench=BenchmarkCorpusLoad -benchmem -benchtime=3x ./internal/astload/
func BenchmarkCorpusLoad(b *testing.B) {
	path := corpusPath(b)
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("read corpus: %v", err)
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := LoadReader(bytes.NewReader(data))
		if err != nil {
			b.Fatalf("LoadReader: %v", err)
		}
		if c.Count() == 0 {
			b.Fatalf("empty corpus")
		}
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// countLines counts non-empty lines in a file.
func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	buf := make([]byte, 256*1024)
	prev := byte(0)
	for {
		n, err := f.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] == '\n' && prev != 0 {
				count++
			}
			prev = buf[i]
		}
		if err != nil {
			break
		}
	}
	// If the last line didn't end with a newline but had content, count it.
	if prev != 0 && prev != '\n' {
		count++
	}
	return count, nil
}

