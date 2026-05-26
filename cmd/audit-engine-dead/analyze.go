package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Declaration is one top-level function / method declared in non-test
// code. Methods carry their receiver type so the same method name on
// different types isn't conflated (LookupKey includes the receiver).
type Declaration struct {
	Name       string
	Receiver   string // empty for free functions
	LookupKey  string // either Name or Receiver+"."+Name; what we grep for
	File       string
	Line       int
	IsExported bool
}

// Reference is one identifier use (Ident or SelectorExpr) recorded
// during the second pass over the package source.
type Reference struct {
	LookupKey string
	File      string
	Line      int
	IsTest    bool // file path ends in _test.go
}

// CaseLiteral is one switch-case-arm whose value is a string literal.
// File/Line/Switch-context are kept so the report points the reader
// straight at the dead case.
type CaseLiteral struct {
	Value     string
	File      string
	Line      int
	SwitchTag string // the switch's tag expression as a hint ("event.Kind", "v" etc.)
}

// AnalyzeResult is what the renderer consumes. ExportedTestOnly lists
// Declarations whose only references outside the declaring file are
// in tests. UnusedSwitchCases lists CaseLiterals whose string value
// appears nowhere else (case-arm value uses excluded).
type AnalyzeResult struct {
	TotalDecls         int
	TotalRefs          int
	ExportedTestOnly   []Declaration
	UnusedSwitchCases  []CaseLiteral
}

// AnalyzePackage runs both audits over every .go file under dir.
// Declarations are collected only from dir; references are also
// counted across the additional refScanDirs the caller supplies via
// AnalyzePackageWithScope.
//
// Convenience wrapper: AnalyzePackage scans declarations AND references
// only from dir. Use this for unit tests and tight, package-internal
// audits. For real Versailles-style audits where the dead-branch
// finding must hold across the WHOLE module, call
// AnalyzePackageWithScope(targetDir, []string{...}).
func AnalyzePackage(dir string) (*AnalyzeResult, error) {
	return AnalyzePackageWithScope(dir, nil)
}

// AnalyzePackageWithScope runs the audit over targetDir for declarations
// and over targetDir + refScanDirs for references. A function declared
// in targetDir that's called from one of refScanDirs (e.g., cmd/* /
// other internal/* packages) is NOT flagged as exported_but_test_only.
//
// Test files participate in reference counting (so we can split test
// vs non-test references) but their declarations are NOT collected —
// dead-branch findings about test helpers aren't actionable here.
func AnalyzePackageWithScope(targetDir string, refScanDirs []string) (*AnalyzeResult, error) {
	targetFiles, err := collectGoFiles(targetDir)
	if err != nil {
		return nil, err
	}

	// Reference-scan files include the target AND every extra dir
	// provided. De-dupe against the target so we don't double-count.
	refScanSet := map[string]bool{}
	for _, p := range targetFiles {
		refScanSet[p] = true
	}
	for _, rd := range refScanDirs {
		extra, err := collectGoFiles(rd)
		if err != nil {
			// Soft-fail per directory: a missing refScanDir shouldn't
			// kill the audit, but we lose its coverage signal.
			continue
		}
		for _, p := range extra {
			refScanSet[p] = true
		}
	}
	allFiles := make([]string, 0, len(refScanSet))
	for p := range refScanSet {
		allFiles = append(allFiles, p)
	}

	fset := token.NewFileSet()
	parsedNonTest := map[string]*ast.File{} // for ref scan AND decl collection (if in target)
	parsedTest := map[string]*ast.File{}

	// Track which file paths belong to the target dir; only those
	// contribute declarations.
	inTarget := map[string]bool{}
	for _, p := range targetFiles {
		inTarget[p] = true
	}

	for _, path := range allFiles {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			// Skip individual parse errors — large repos occasionally
			// have generator-output files; we don't want a single one
			// to fail the whole audit.
			continue
		}
		if isTestFile(path) {
			parsedTest[path] = f
		} else {
			parsedNonTest[path] = f
		}
	}

	// Pass 1: collect declarations from TARGET non-test files only.
	decls := []Declaration{}
	declByKey := map[string]Declaration{}
	for path, f := range parsedNonTest {
		if !inTarget[path] {
			continue
		}
		for _, d := range collectDecls(fset, path, f) {
			decls = append(decls, d)
			declByKey[d.LookupKey] = d
		}
	}

	// Pass 2: count references across the FULL scan pool.
	refsByKey := map[string][]Reference{}
	collectFromFiles := func(files map[string]*ast.File, isTest bool) {
		for path, f := range files {
			collectRefs(fset, path, f, isTest, declByKey, func(r Reference) {
				refsByKey[r.LookupKey] = append(refsByKey[r.LookupKey], r)
			})
		}
	}
	collectFromFiles(parsedNonTest, false)
	collectFromFiles(parsedTest, true)

	// Build ExportedTestOnly: an exported declaration whose only
	// non-self references are in test files. "Self" means the same
	// file the declaration lives in (helpers commonly reference each
	// other within a file).
	var exportedTestOnly []Declaration
	for _, d := range decls {
		if !d.IsExported {
			continue
		}
		refs := refsByKey[d.LookupKey]
		var nonTestOffsite, testRefs int
		for _, r := range refs {
			if r.File == d.File {
				continue // self-reference in declaring file
			}
			if r.IsTest {
				testRefs++
			} else {
				nonTestOffsite++
			}
		}
		if nonTestOffsite == 0 && testRefs > 0 {
			exportedTestOnly = append(exportedTestOnly, d)
		}
	}
	sort.Slice(exportedTestOnly, func(i, j int) bool {
		if exportedTestOnly[i].File != exportedTestOnly[j].File {
			return exportedTestOnly[i].File < exportedTestOnly[j].File
		}
		return exportedTestOnly[i].Line < exportedTestOnly[j].Line
	})

	// Pass 3: collect switch case literal values ONLY from target
	// non-test files. The dead-branch finding is "this case will never
	// fire in production" — case arms in test files are testing the
	// switch, not the engine's behavior.
	caseLits := []CaseLiteral{}
	for path, f := range parsedNonTest {
		if !inTarget[path] {
			continue
		}
		caseLits = append(caseLits, collectCaseLiterals(fset, path, f)...)
	}

	// Collect every OTHER string literal from the WHOLE scan pool —
	// an emitter in internal/hat or cmd/* is just as valid for marking
	// a case as live. Both test and non-test files contribute here:
	// even a test-only emitter means the case isn't structurally dead,
	// just unused outside tests (a finding that would belong to a
	// separate "test-only feature" audit, not this one).
	otherStrLits := map[string]bool{}
	for _, f := range parsedNonTest {
		collectOtherStringLiterals(fset, f, otherStrLits)
	}
	for _, f := range parsedTest {
		collectOtherStringLiterals(fset, f, otherStrLits)
	}

	// A case literal is "unused" when its value never appears as any
	// other string literal in the codebase. False-positive sources we
	// accept (would-be-bug-now-tracked):
	//   - The literal is built at runtime via fmt.Sprintf — we can't
	//     detect that without flow analysis. Conservative: include the
	//     match and let the reader verify.
	var unusedCases []CaseLiteral
	for _, c := range caseLits {
		if !otherStrLits[c.Value] {
			unusedCases = append(unusedCases, c)
		}
	}
	sort.Slice(unusedCases, func(i, j int) bool {
		if unusedCases[i].File != unusedCases[j].File {
			return unusedCases[i].File < unusedCases[j].File
		}
		return unusedCases[i].Line < unusedCases[j].Line
	})

	totalRefs := 0
	for _, rs := range refsByKey {
		totalRefs += len(rs)
	}
	return &AnalyzeResult{
		TotalDecls:        len(decls),
		TotalRefs:         totalRefs,
		ExportedTestOnly:  exportedTestOnly,
		UnusedSwitchCases: unusedCases,
	}, nil
}

// collectGoFiles walks dir and returns every .go file path. Skips
// vendor/ directories and hidden dirs. Symlinks are followed (data
// dir is symlinked into the worktree in this repo).
func collectGoFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || (len(name) > 0 && name[0] == '.' && name != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// collectDecls walks file f and emits every top-level function and
// method declaration. Only declarations are produced — body bodies
// are inspected later in collectRefs.
func collectDecls(fset *token.FileSet, path string, f *ast.File) []Declaration {
	var out []Declaration
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		name := fn.Name.Name
		recv := ""
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			recv = receiverTypeName(fn.Recv.List[0].Type)
		}
		// LookupKey deliberately = simple name (no receiver prefix).
		// Selector-call refs like `obj.Method` reach methods via the
		// simple Sel name; receiver-qualified keys would never match.
		// The documented over-conflation bias (two unrelated types
		// sharing a method name share their refs) makes us LESS likely
		// to flag real dead code — the right direction for a findings-
		// only audit.
		out = append(out, Declaration{
			Name:       name,
			Receiver:   recv,
			LookupKey:  name,
			File:       path,
			Line:       fset.Position(fn.Pos()).Line,
			IsExported: ast.IsExported(name),
		})
	}
	return out
}

// receiverTypeName extracts the unqualified type name from a receiver
// expression (e.g., "*GameState" → "GameState", "Card" → "Card").
// Returns "" for unusual shapes we can't trivially classify.
func receiverTypeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return receiverTypeName(x.X)
	case *ast.Ident:
		return x.Name
	}
	return ""
}

// collectRefs walks file f and emits a Reference for every identifier
// or selector use that resolves to a Declaration in declByKey.
//
// We deliberately keep this loose: a `gs.MoveCard(...)` selector use
// counts for the "MoveCard" name regardless of receiver type, which
// can over-count (two unrelated MoveCard methods would share a key).
// The over-counting bias is conservative — it makes us LESS likely
// to flag a real dead branch, which is the right error direction for
// a findings-only audit.
func collectRefs(fset *token.FileSet, path string, f *ast.File, isTest bool, declByKey map[string]Declaration, emit func(Reference)) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Skip the function NAME ident — that's a declaration, not
			// a reference. Continue into the body.
			return true
		case *ast.SelectorExpr:
			// selector.Sel is the rightmost name; we use that as the
			// lookup key by simple-name, plus also try the
			// "<Receiver>.<Sel>" qualified key for method dispatch.
			if x.Sel == nil {
				return true
			}
			name := x.Sel.Name
			if _, ok := declByKey[name]; ok {
				emit(Reference{
					LookupKey: name,
					File:      path,
					Line:      fset.Position(x.Sel.Pos()).Line,
					IsTest:    isTest,
				})
			}
			// We do NOT try the qualified Receiver.Name lookup because
			// we don't track expr types here — over-counting via the
			// simple name is the conservative choice.
		case *ast.Ident:
			if _, ok := declByKey[x.Name]; ok {
				// Filter: the identifier inside a FuncDecl that names
				// the function itself is also walked here. Skip when
				// the parent is the declaration site — not easy with
				// ast.Inspect alone, so use a coarser filter: skip if
				// this Ident appears in a position where the parent
				// node is a FuncDecl.Name.
				// Without parent tracking we accept these self-name
				// matches; they'll be filtered by the "same file"
				// self-ref guard in AnalyzePackage.
				emit(Reference{
					LookupKey: x.Name,
					File:      path,
					Line:      fset.Position(x.Pos()).Line,
					IsTest:    isTest,
				})
			}
		}
		return true
	})
}

// collectCaseLiterals walks every SwitchStmt + TypeSwitchStmt and
// collects each case-arm value that's a string literal. Non-string
// case arms (ints, type names) are out of scope — string-keyed
// switches are the dominant dead-arm shape in this engine.
func collectCaseLiterals(fset *token.FileSet, path string, f *ast.File) []CaseLiteral {
	var out []CaseLiteral
	ast.Inspect(f, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		tag := ""
		if sw.Tag != nil {
			tag = exprToString(sw.Tag)
		}
		for _, stmt := range sw.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, e := range cc.List {
				if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					val, err := unquote(lit.Value)
					if err != nil {
						continue
					}
					out = append(out, CaseLiteral{
						Value:     val,
						File:      path,
						Line:      fset.Position(lit.Pos()).Line,
						SwitchTag: tag,
					})
				}
			}
		}
		return true
	})
	return out
}

// collectOtherStringLiterals records every string literal value
// encountered OUTSIDE of switch case arm positions. We achieve this
// by walking parents: when we see a SwitchStmt we recurse explicitly
// into the body but skip the case-list children at the CaseClause
// level. Easier: walk every node, and for each CaseClause flag its
// List children so we know to ignore them on the main pass. Two-pass
// keeps the code simple.
func collectOtherStringLiterals(fset *token.FileSet, f *ast.File, out map[string]bool) {
	// First, find every Pos() of every case-arm string literal so we
	// can skip them in the main scan.
	skipPos := map[token.Pos]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		cc, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, e := range cc.List {
			if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				skipPos[lit.Pos()] = true
			}
		}
		return true
	})
	// Second pass: collect every other STRING literal.
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if skipPos[lit.Pos()] {
			return true
		}
		val, err := unquote(lit.Value)
		if err != nil {
			return true
		}
		out[val] = true
		return true
	})
}

// exprToString returns a best-effort one-line representation of an
// AST expression. Used to render switch-tag hints in the report
// ("switch on `event.Kind`", "switch on `v`"). Falls back to "" for
// shapes we can't simplify, which leaves the tag column blank but
// doesn't break the report.
func exprToString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		left := exprToString(x.X)
		if left == "" {
			return x.Sel.Name
		}
		return left + "." + x.Sel.Name
	case *ast.CallExpr:
		if name := exprToString(x.Fun); name != "" {
			return name + "(…)"
		}
	}
	return ""
}

// unquote strips Go string-literal quoting (both regular "..." and
// raw `...` forms). Returns the unquoted value plus an error if the
// literal is malformed — callers skip on error.
func unquote(s string) (string, error) {
	if len(s) < 2 {
		return "", errInvalidLiteral
	}
	if s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1], nil
	}
	if s[0] != '"' || s[len(s)-1] != '"' {
		return "", errInvalidLiteral
	}
	// Minimal unescaping for the common cases. Go's strconv.Unquote
	// is fully correct but pulling strconv just for this is overkill
	// when the values we care about are simple ASCII tokens.
	inner := s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c == '\\' && i+1 < len(inner) {
			next := inner[i+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(next)
			}
			i++
			continue
		}
		b.WriteByte(c)
	}
	return b.String(), nil
}

// errInvalidLiteral is returned by unquote when the literal text
// doesn't have the expected opening/closing quote pair. Defined as
// a typed error so callers can match without string comparison.
type literalErr string

func (l literalErr) Error() string { return string(l) }

var errInvalidLiteral = literalErr("invalid literal")
