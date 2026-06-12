package outcome

// Corpus audit — runs the OUTCOME check across every card in the AST
// dataset whose effects fall in phase-1 scope. Skips when the dataset
// is absent (gitignored, see CLAUDE.md). Findings are written to
// /tmp/fable-review/outcome-findings-r63.csv; the test itself fails
// only on harness errors, not on findings (it is an audit, and known
// divergences are triaged in the r63 report).

import (
	"encoding/csv"
	"fmt"
	"os"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
)

func TestOutcome_CorpusAudit(t *testing.T) {
	const datasetPath = "../../../data/rules/ast_dataset.jsonl"
	if _, err := os.Stat(datasetPath); err != nil {
		t.Skipf("AST dataset not present (%v) — corpus audit skipped", err)
	}
	corpus, err := astload.Load(datasetPath)
	if err != nil {
		t.Fatalf("astload: %v", err)
	}

	var findings []*Finding
	inScope, passed := 0, 0
	kindsInScope := map[string]int{}
	for _, name := range corpus.Names() {
		ast, _ := corpus.Get(name)
		if ast == nil {
			continue
		}
		for _, ex := range ExtractEffects(ast) {
			finding, ran := RunEffect(name, ex.Raw, ex.Effect)
			if !ran {
				continue
			}
			inScope++
			kindsInScope[ex.Effect.Kind()]++
			if finding == nil {
				passed++
			} else {
				findings = append(findings, finding)
			}
		}
	}

	t.Logf("outcome corpus audit: %d effects in phase-1 scope, %d passed, %d diverged",
		inScope, passed, len(findings))
	t.Logf("in-scope by kind: %v", kindsInScope)

	f, err := os.Create("/tmp/fable-review/outcome-findings-r63.csv")
	if err != nil {
		t.Fatalf("findings csv: %v", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"card", "kind", "expected", "actual", "raw"})
	for _, fd := range findings {
		_ = w.Write([]string{fd.CardName, fd.Kind, fd.Expected, fd.Actual, fd.Raw})
	}
	fmt.Printf("OUTCOME-AUDIT inScope=%d passed=%d diverged=%d\n", inScope, passed, len(findings))
}
