package gameengine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestEOTExpiryTimingGate_r101 — an "until end of turn" undo scheduled
// as a delayed trigger must be scheduled for the CLEANUP step, not the
// end step.
//
// # WHY THIS GATE EXISTS
//
// CR §514.2: "When the cleanup step begins ... all 'until end of turn'
// and 'this turn' effects end." The end step is §513 and runs BEFORE
// the cleanup step, with the §513.1 priority window in between —
// end-of-turn removal, flash creatures, Teferi's Protection all happen
// there. delayedTriggerMatches maps both "next_end_step" and
// "end_of_turn" to `step == "end"`, so an undo scheduled with either
// ran one full priority window early.
//
// Thirty sites across thirty files did exactly that, every one of them
// a card whose printed duration is "until end of turn": keyword grants
// (Skithiryx's haste, Winota's indestructible, Vito's team lifelink)
// and control reversions (Insurrection, Zidane, Dihada's -11,
// Kenrith). The correctly-timed keyword primitive —
// ExpireKeywordGrantsEOT, opted into with Flags["kw_eot:<name>"] — had
// existed since r64 with five adopters and was never reached for.
//
// Same canonical-helper-bypass shape as amass / populate / crew, and
// the same lesson as r97: the class is only closed once something
// mechanical refuses the next instance.
//
// WHAT THIS FLAGS
//
// A RegisterDelayedTrigger whose TriggerAt is an END-STEP value and
// whose EffectFn body strips the Flags["kw:<name>"] surface (through
// ClearKeywordFlags, RevokeKeywordFlag, or a raw delete) or reverts a
// Controller. Those are §514.2 effects and belong at
// CleanupStepTrigger.
//
// WHAT THIS DELIBERATELY DOES NOT FLAG
//
//   - "your_next_turn" / "your_next_end_step" / "next_upkeep" — a
//     different printed duration. Dihada's +2 is "until your NEXT
//     TURN" and correctly uses your_next_turn; §514.2 does not govern
//     it.
//   - An end-step delayed trigger that does anything else. "At the
//     beginning of the next end step, sacrifice it" is a real §513
//     triggered ability and belongs exactly where it is.
//
// There is no baseline file. The count is zero.
func TestEOTExpiryTimingGate_r101(t *testing.T) {
	root := moduleRootR97(t)
	fset := token.NewFileSet()

	endStepValues := map[string]bool{
		"next_end_step": true,
		"end_of_turn":   true,
		"end_step":      true,
	}

	type finding struct {
		file, trig, why string
		line            int
	}
	var bad []finding

	err := filepath.Walk(filepath.Join(root, "internal"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "RegisterDelayedTrigger" || len(call.Args) == 0 {
				return true
			}
			var lit *ast.CompositeLit
			switch a := call.Args[0].(type) {
			case *ast.UnaryExpr:
				lit, _ = a.X.(*ast.CompositeLit)
			case *ast.CompositeLit:
				lit = a
			}
			if lit == nil {
				return true
			}
			var trig string
			var effect ast.Node
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, _ := kv.Key.(*ast.Ident)
				if key == nil {
					continue
				}
				switch key.Name {
				case "TriggerAt":
					if bl, ok := kv.Value.(*ast.BasicLit); ok && bl.Kind == token.STRING {
						trig = strings.Trim(bl.Value, `"`)
					}
				case "EffectFn":
					effect = kv.Value
				}
			}
			if effect == nil || !endStepValues[trig] {
				return true
			}
			why := ""
			ast.Inspect(effect, func(m ast.Node) bool {
				if c, ok := m.(*ast.CallExpr); ok {
					name := ""
					switch fn := c.Fun.(type) {
					case *ast.SelectorExpr:
						name = fn.Sel.Name
					case *ast.Ident:
						name = fn.Name
					}
					switch name {
					case "ClearKeywordFlags", "RevokeKeywordFlag":
						why = "strips a kw: keyword grant"
					case "delete":
						if len(c.Args) == 2 {
							if bl, ok := c.Args[1].(*ast.BasicLit); ok &&
								strings.HasPrefix(strings.Trim(bl.Value, `"`), "kw:") {
								why = "raw-deletes a kw: keyword flag"
							}
						}
					}
				}
				if as, ok := m.(*ast.AssignStmt); ok {
					for _, l := range as.Lhs {
						if s, ok := l.(*ast.SelectorExpr); ok && s.Sel.Name == "Controller" {
							if why == "" {
								why = "reverts a Controller (an until-EOT control change)"
							}
						}
					}
				}
				return true
			})
			if why != "" {
				bad = append(bad, finding{
					file: rel, trig: trig, why: why,
					line: fset.Position(lit.Pos()).Line,
				})
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(bad) == 0 {
		return
	}
	sort.Slice(bad, func(i, j int) bool {
		if bad[i].file != bad[j].file {
			return bad[i].file < bad[j].file
		}
		return bad[i].line < bad[j].line
	})
	var b strings.Builder
	b.WriteString("until-EOT undo scheduled for the END step, not the cleanup step (CR 514.2):\n")
	for _, f := range bad {
		b.WriteString("  " + f.file + ":" + itoaR101(f.line) +
			"  TriggerAt " + f.trig + " — " + f.why + "\n")
	}
	b.WriteString("\nUse gameengine.CleanupStepTrigger. If the printed duration really is\n")
	b.WriteString("a beginning-of-end-step triggered ability, the effect does not belong\n")
	b.WriteString("in this shape — see keyword_grant_expiry_cleanup_r101.go.\n")
	t.Error(b.String())
	_ = os.Stdout
}

func itoaR101(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
