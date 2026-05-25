package hexapi

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// r60 — drift detector for docs/hexapi-openapi.yaml.
//
// The OpenAPI spec is hand-maintained, so without a regression test
// it will silently fall behind every new route. This test scans
// every internal/hexapi/*.go (non-test) file for `mux.HandleFunc`
// calls, extracts the method + path, and asserts each registered
// route appears in docs/hexapi-openapi.yaml. The reverse direction
// (spec endpoints with no implementing route) is also checked so a
// rename can't leave a phantom in the spec.
//
// The check is intentionally string-based: we don't depend on a YAML
// parser (the repo has no YAML dep, and pulling one in just for
// tests would be overkill). The spec uses canonical path entries
// like `  /api/decks:` and `  /api/decks/{owner}/{id}:` — matching
// either form is sufficient to confirm presence.

// muxHandleFuncRE matches a route registration like:
//   mux.HandleFunc("GET /api/decks", h.handleListDecks)
//   mux.HandleFunc("POST /api/decks/{owner}/{id}/clone", ...)
//
// The path may contain `{name}` segments and / characters but no
// quotes or newlines.
var muxHandleFuncRE = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+)\s+([^"]+)"`)

func TestOpenAPICoverage_AllRegisteredRoutesAreDocumented(t *testing.T) {
	repoRoot := findRepoRoot(t)
	specPath := filepath.Join(repoRoot, "docs", "hexapi-openapi.yaml")

	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}
	spec := string(specBytes)

	// Sanity: the spec must declare itself as OpenAPI 3.x.
	if !strings.Contains(spec, "openapi: 3.") {
		t.Fatalf("spec missing `openapi: 3.x` preamble (first 200 chars: %q)", spec[:min(200, len(spec))])
	}

	registered := collectMuxRoutes(t, filepath.Join(repoRoot, "internal", "hexapi"))
	if len(registered) == 0 {
		t.Fatalf("no mux.HandleFunc routes parsed from internal/hexapi — regex broken?")
	}

	// Build the set of unique paths so missing rows surface as one
	// failure per path rather than once per (method, path) pair.
	seenPath := map[string]bool{}
	for _, r := range registered {
		seenPath[r.path] = true
	}

	var missing []string
	for path := range seenPath {
		// OpenAPI path entries live at two-space indent and end with
		// a colon. The trailing colon avoids false positives when
		// `/api/decks` is a prefix of `/api/decks/foo`.
		key := "  " + path + ":"
		if !strings.Contains(spec, key) {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("docs/hexapi-openapi.yaml is missing %d registered route(s):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestOpenAPICoverage_NoPhantomSpecEntries goes the other direction:
// every path documented in the spec must have at least one
// mux.HandleFunc backing it. A rename or deletion that forgets to
// update the spec leaves a phantom row — the check catches it.
func TestOpenAPICoverage_NoPhantomSpecEntries(t *testing.T) {
	repoRoot := findRepoRoot(t)
	specPath := filepath.Join(repoRoot, "docs", "hexapi-openapi.yaml")

	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}
	specPaths := extractSpecPaths(string(specBytes))
	if len(specPaths) == 0 {
		t.Fatalf("no paths extracted from spec — regex broken?")
	}

	registered := collectMuxRoutes(t, filepath.Join(repoRoot, "internal", "hexapi"))
	registeredSet := map[string]bool{}
	for _, r := range registered {
		registeredSet[r.path] = true
	}

	var phantom []string
	for _, p := range specPaths {
		if !registeredSet[p] {
			phantom = append(phantom, p)
		}
	}
	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Errorf("docs/hexapi-openapi.yaml documents %d path(s) with no mux.HandleFunc backing them:\n  %s",
			len(phantom), strings.Join(phantom, "\n  "))
	}
}

type muxRoute struct {
	method string
	path   string
	file   string
}

func collectMuxRoutes(t *testing.T, hexapiDir string) []muxRoute {
	t.Helper()
	entries, err := os.ReadDir(hexapiDir)
	if err != nil {
		t.Fatalf("read hexapi dir: %v", err)
	}
	var out []muxRoute
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(hexapiDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range muxHandleFuncRE.FindAllStringSubmatch(string(body), -1) {
			out = append(out, muxRoute{
				method: m[1],
				path:   m[2],
				file:   name,
			})
		}
	}
	return out
}

// specPathRE matches a path entry header in the OpenAPI spec.
// The grammar we emit is:
//
//   /api/foo:
//   /api/foo/{bar}:
//
// at two-space indent — every other YAML key that starts with `/`
// would also be valid but the spec only uses these for path
// declarations.
var specPathRE = regexp.MustCompile(`(?m)^ {2}(/[^\s:]+):\s*$`)

func extractSpecPaths(spec string) []string {
	matches := specPathRE.FindAllStringSubmatch(spec, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// findRepoRoot walks up from this test file's location until it
// finds a directory containing `go.mod`. Lets the test run from
// any package CWD without hard-coding the worktree path.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found walking up from %s", filepath.Dir(file))
		}
		dir = parent
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
