package gameengine

// r102 — the CR §302.6 control-change gate.
//
// §302.6 verbatim: "A creature's activated ability with the tap symbol
// or the untap symbol in its activation cost can't be activated unless
// the creature has been under its controller's control continuously
// since their most recent turn began. A creature can't attack unless it
// has been under its controller's control continuously since their most
// recent turn began."
//
// Permanent.SummoningSick IS that predicate: the untap step clears it
// for exactly the permanents the active seat controls (phases.go), so a
// true value means "this controller has not had it since their turn
// began". Nothing SET it on a control change, so a stolen creature
// attacked and tapped on the turn it changed hands.
//
// THE SCOPE RULE. The hazard is a battlefield REPARENT: a scope that
// both assigns `X.Controller = …` and touches a `.Battlefield` slice is
// moving a permanent between controllers by hand. Such a scope must go
// through ApplyControlChange (which applies §302.6), or make an
// explicit, visible SummoningSick decision of its own.
//
// Scoped per INNERMOST function (FuncDecl or FuncLit), not per file and
// not per outer function — r97's lesson. Three of the reverts in this
// class live in delayed-trigger closures, and a per-FuncDecl scope would
// let the enclosing handler's own stamp satisfy them vacuously.
//
// No baseline file. The count is 0 and stays 0.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestControlChangeSicknessGate_r102(t *testing.T) {
	root := moduleRootR97(t)
	var findings []string

	for _, pkg := range []string{
		filepath.Join(root, "internal", "gameengine"),
		filepath.Join(root, "internal", "gameengine", "per_card"),
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
			scanControlScopes(f, fset, rel, &findings)
		}
	}

	if len(findings) > 0 {
		t.Errorf("CR 302.6: %d hand-rolled control change(s) not routed through ApplyControlChange:", len(findings))
		for _, f := range findings {
			t.Errorf("  %s", f)
		}
	}
}

// scanControlScopes walks every innermost function scope in f.
func scanControlScopes(f *ast.File, fset *token.FileSet, rel string, findings *[]string) {
	var visit func(body *ast.BlockStmt, owner string, pos token.Pos)
	visit = func(body *ast.BlockStmt, owner string, pos token.Pos) {
		if body == nil {
			return
		}
		var assignsController bool
		var touchesBattlefield bool
		var usesPrimitive bool
		var ctrlPos token.Pos

		ast.Inspect(body, func(n ast.Node) bool {
			// Do not descend into a nested function literal: it is its
			// own innermost scope and gets its own visit() below.
			if lit, ok := n.(*ast.FuncLit); ok && lit.Body != body {
				visit(lit.Body, owner+".func", lit.Pos())
				return false
			}
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					switch sel.Sel.Name {
					case "Controller":
						assignsController = true
						if !ctrlPos.IsValid() {
							ctrlPos = sel.Pos()
						}
					}
				}
			case *ast.SelectorExpr:
				if x.Sel.Name == "Battlefield" {
					touchesBattlefield = true
				}
			case *ast.CallExpr:
				if name := calleeName(x.Fun); name == "ApplyControlChange" {
					usesPrimitive = true
				}
			}
			return true
		})

		if assignsController && touchesBattlefield && !usesPrimitive {
			*findings = append(*findings,
				fset.Position(ctrlPos).String()+" in "+owner+"  ("+rel+")")
		}
	}

	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		// The primitive IS the implementation of the rule; it is the one
		// place the raw assignment belongs.
		if fn.Name.Name == "ApplyControlChange" {
			continue
		}
		visit(fn.Body, fn.Name.Name, fn.Pos())
	}
}

func calleeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}
