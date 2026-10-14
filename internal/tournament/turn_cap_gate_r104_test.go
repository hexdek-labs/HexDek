package tournament

// r104 — the turn-cap adjudication gate.
//
// THE SCOPE RULE. The hazard is a turn-cap OUTCOME decided by hand, so
// the gate keys on the thing only an adjudicator does: naming a
// turn-cap end reason. Any function that mentions "turn_cap_leader" or
// "turn_cap_all_dead" is deciding or reporting that outcome and must
// get its eligibility from TurnCapEligible / TurnCapWinner, which apply
// the CR §104.3b life test and the §104.4a draw.
//
// Deliberately NOT keyed on the `bestLife := -999` loop SHAPE. That
// shape also appears in per_card/saskia_the_unyielding.go, which picks
// a "choose a player" target and is AI policy, not an outcome rule —
// matching on shape would flag it and invite someone to "fix" a
// correct site. Matching on the end-reason vocabulary cannot.
//
// No baseline file. Six sites at HEAD; zero after.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTurnCapAdjudicationGate_r104(t *testing.T) {
	root := moduleRootR104(t)
	var findings []string

	for _, pkg := range []string{
		filepath.Join(root, "internal", "tournament"),
		filepath.Join(root, "internal", "hexapi"),
		filepath.Join(root, "internal", "paritycheck"),
	} {
		entries, err := os.ReadDir(pkg)
		if err != nil {
			t.Fatalf("read %s: %v", pkg, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(pkg, name)
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			rel, _ := filepath.Rel(root, path)
			// The helper is the implementation of the rule.
			if strings.HasSuffix(rel, "turn_cap_104_4a_r104.go") {
				continue
			}
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				var namesReason bool
				var usesHelper bool
				var at token.Pos
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.BasicLit:
						if x.Kind != token.STRING {
							return true
						}
						if strings.Contains(x.Value, "turn_cap_leader") ||
							strings.Contains(x.Value, "turn_cap_all_dead") {
							namesReason = true
							if !at.IsValid() {
								at = x.Pos()
							}
						}
					case *ast.CallExpr:
						switch callee(x.Fun) {
						case "TurnCapWinner", "TurnCapEligible":
							usesHelper = true
						}
					}
					return true
				})
				if namesReason && !usesHelper {
					findings = append(findings,
						fset.Position(at).String()+" in "+fn.Name.Name+"  ("+rel+")")
				}
			}
		}
	}

	if len(findings) > 0 {
		t.Errorf("CR 104.3b/104.4a: %d hand-rolled turn-cap adjudication(s) not using "+
			"TurnCapEligible/TurnCapWinner:", len(findings))
		for _, f := range findings {
			t.Errorf("  %s", f)
		}
	}
}

func callee(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

func moduleRootR104(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test's working directory")
		}
		dir = parent
	}
}
