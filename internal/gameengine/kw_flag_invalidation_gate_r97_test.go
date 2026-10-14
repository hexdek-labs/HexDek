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

// TestKwFlagInvalidationGate_r97 — every production mutation of the
// Flags["kw:<name>"] keyword surface must make itself visible to the
// layer-aware keyword query.
//
// # WHY THIS GATE EXISTS
//
// Flags["kw:<name>"] is folded into the layer BASE by
// runtimeKeywordSources (keyword_source_unification_r65.go), and the
// computed characteristics are cached per GameState behind
// charCacheEpoch. Writing to the Flags map does not bump that epoch, so
// a hand-written stamp or delete leaves every layer-aware reader holding
// the pre-mutation answer. Since r66 collapsed keywordActive down to the
// layer query alone, that is the answer combat, evasion and the §508.1a
// legality validator use.
//
// This class had been fixed by hand four times before a detector
// existed — r87 (refcount), r89, r91, r93 (seven instances) — and each
// fix left the next instance to be found by hand. When the gate was
// first written for r97 it found 141 uncovered sites across 83 files,
// including nine files that looked covered on a per-FILE grep for
// InvalidateCharacteristicsCache because the invalidation sat in the
// OUTER function while the mutation sat in a delayed-trigger closure
// that runs a whole combat later.
//
// WHAT COUNTS AS COVERED
//
//   - the mutation goes through SetKeywordFlags / ClearKeywordFlags
//     (keyword_flag_mutation_r97.go), which own the invalidation; or
//   - the innermost enclosing function-like scope calls
//     InvalidateCharacteristicsCache.
//
// The scope is the innermost FuncDecl *or FuncLit*, deliberately. A
// delayed-trigger EffectFn is its own scope: it runs long after the
// function that registered it, the runners (FireDelayedTriggers,
// FireEventDelayedTriggers) do not invalidate for it, and the
// registering function's own invalidation happened at registration
// time. That is the invalidation contract, and it is the distinction
// that makes this gate find things a per-file grep cannot.
//
// The r87 refcounted pair GrantKeywordFlag / RevokeKeywordFlag is
// checked under the same rule: their doc puts the invalidation
// obligation on the caller, so the calling scope must discharge it. All
// 58 of their call sites did when this gate was written — that arm is
// here to keep it true, not because it was broken.
//
// There is no baseline file. The count is zero and the gate is the only
// thing keeping it there; a suppression list would be the mechanism by
// which it stops being zero.
func TestKwFlagInvalidationGate_r97(t *testing.T) {
	root := moduleRootR97(t)
	fset := token.NewFileSet()

	type finding struct {
		file, scope, kind, key string
		line                   int
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

		var scan func(body *ast.BlockStmt, name string)
		scan = func(body *ast.BlockStmt, name string) {
			covered := scopeInvalidatesR97(body)
			ast.Inspect(body, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.FuncLit:
					if v.Body == body {
						return true
					}
					scan(v.Body, name+".func")
					return false
				case *ast.AssignStmt:
					if covered {
						return true
					}
					for _, lhs := range v.Lhs {
						if key, ok := kwFlagIndexR97(lhs); ok {
							bad = append(bad, finding{rel, name, "set", key, fset.Position(v.Pos()).Line})
						}
					}
				case *ast.CallExpr:
					if covered {
						return true
					}
					if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
						switch sel.Sel.Name {
						case "GrantKeywordFlag", "RevokeKeywordFlag":
							bad = append(bad, finding{rel, name, "r87", sel.Sel.Name,
								fset.Position(v.Pos()).Line})
						}
					}
					if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "delete" && len(v.Args) == 2 {
						if sel, ok := v.Args[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "Flags" {
							if lit, ok := v.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								if s := strings.Trim(lit.Value, `"`); strings.HasPrefix(s, "kw:") {
									bad = append(bad, finding{rel, name, "del", s,
										fset.Position(v.Pos()).Line})
								}
							}
						}
					}
				}
				return true
			})
		}

		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				scan(fd.Body, fd.Name.Name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// The gate's own definitions mutate the surface by construction.
	kept := bad[:0]
	for _, b := range bad {
		if strings.HasSuffix(b.file, "keyword_flag_mutation_r97.go") ||
			strings.HasSuffix(b.file, "keyword_flag_grants_r87.go") ||
			strings.HasSuffix(b.file, "keyword_grant_expiry.go") {
			continue
		}
		kept = append(kept, b)
	}
	bad = kept

	if len(bad) == 0 {
		return
	}
	sort.Slice(bad, func(i, j int) bool {
		if bad[i].file != bad[j].file {
			return bad[i].file < bad[j].file
		}
		return bad[i].line < bad[j].line
	})
	for _, b := range bad {
		t.Errorf("%s:%d %s() mutates the kw: keyword surface (%s %s) without invalidating\n"+
			"\tUse SetKeywordFlags / ClearKeywordFlags (keyword_flag_mutation_r97.go), which\n"+
			"\town the invalidation, or call gs.InvalidateCharacteristicsCache() in THIS scope.\n"+
			"\tA closure is its own scope: the function that registered it invalidated at\n"+
			"\tregistration time, which is not when the closure runs.",
			b.file, b.line, b.scope, b.kind, b.key)
	}
	t.Errorf("%d uncovered kw: mutation(s)", len(bad))
}

// scopeInvalidatesR97 reports whether this scope calls
// InvalidateCharacteristicsCache, not descending into nested FuncLits
// (which are their own scopes).
func scopeInvalidatesR97(body *ast.BlockStmt) bool {
	found := false
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		ast.Inspect(n, func(x ast.Node) bool {
			if found {
				return false
			}
			if lit, ok := x.(*ast.FuncLit); ok && lit.Body != body {
				return false
			}
			if call, ok := x.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok &&
					sel.Sel.Name == "InvalidateCharacteristicsCache" {
					found = true
					return false
				}
			}
			return true
		})
	}
	walk(body)
	return found
}

// kwFlagIndexR97 matches `<expr>.Flags["kw:<name>"]`.
func kwFlagIndexR97(e ast.Expr) (string, bool) {
	ix, ok := e.(*ast.IndexExpr)
	if !ok {
		return "", false
	}
	sel, ok := ix.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Flags" {
		return "", false
	}
	lit, ok := ix.Index.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s := strings.Trim(lit.Value, `"`)
	if !strings.HasPrefix(s, "kw:") {
		return "", false
	}
	return s, true
}

func moduleRootR97(t *testing.T) string {
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
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}
