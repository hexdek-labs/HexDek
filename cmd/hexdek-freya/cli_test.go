package main

import (
	"bytes"
	"flag"
	"runtime"
	"strings"
	"testing"
)

// TestVersionString pins the multi-line version banner. The Freya
// version doubles as the cache invalidation token, so the banner must
// surface it AND the cache-path schema reference so anyone debugging
// a stale cache can find the directory + filename layout from
// `--version` alone.
func TestVersionString(t *testing.T) {
	s := VersionString()
	for _, want := range []string{
		"hexdek-freya",
		FreyaVersion,
		"cache schema:",
		DefaultCacheDir,
		"go runtime:",
		runtime.Version(),
	} {
		if !strings.Contains(s, want) {
			t.Errorf("VersionString missing %q\n---\n%s", want, s)
		}
	}
	// Multi-line: at least 3 lines (program + cache + runtime).
	if lines := strings.Count(s, "\n"); lines < 3 {
		t.Errorf("VersionString = %d newlines, want at least 3", lines)
	}
}

// TestJSONSchemaDoc pins the schema-doc sections. Each top-level
// section header must be present so users searching for a specific
// field family (cache envelope, combo entry, metrics) find what they
// expect.
func TestJSONSchemaDoc(t *testing.T) {
	s := JSONSchemaDoc()
	for _, want := range []string{
		"Freya JSON Output Schema",
		FreyaVersion,
		"TOP LEVEL: FreyaReport",
		"COMBO ENTRY: ComboResult",
		"METRICS MODE: FreyaMetrics",
		"CACHE ENTRY ENVELOPE",
		// Top-level fields the doc must reference (smoke-test, not
		// exhaustive). If a major top-level field gets renamed or
		// removed, the test surfaces it.
		"Archetype",
		"WinLines",
		"TrueInfinites",
		"Roles",
		"Profile",
		// Cache envelope fields.
		"freya_version",
		"deck_hash",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSONSchemaDoc missing %q\n---\n%s", want, s)
		}
	}
}

// TestPrintUsage pins the usage banner structure: synopsis, examples,
// and flags must all appear, and flag definitions from the passed
// FlagSet must be reflected (so a missing-flag dump doesn't silently
// hide options from --help). We register a fresh FlagSet with two
// flags to keep the test independent of main()'s real registration.
func TestPrintUsage(t *testing.T) {
	fs := flag.NewFlagSet("hexdek-freya", flag.ContinueOnError)
	fs.String("deck", "", "test deck flag")
	fs.Bool("version", false, "test version flag")

	var buf bytes.Buffer
	PrintUsage(&buf, fs)
	out := buf.String()
	for _, want := range []string{
		"hexdek-freya — Commander deck analysis",
		"USAGE:",
		"--deck <file>",
		"--all-decks <dir>",
		"--version",
		"--json-schema",
		"EXAMPLES:",
		"--format json",
		"--mode focus",
		"--mode metrics",
		"--all-decks",
		"--no-cache",
		"FLAGS:",
		// Flag definitions from the injected FlagSet must surface.
		"-deck",
		"test deck flag",
		"-version",
		"test version flag",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintUsage missing %q\n---\n%s", want, out)
		}
	}
}

// TestProgressLine_Padding pins the index-width formatting. A
// 12/100-deck run must produce a 3-digit-wide index column so the
// log lines stay aligned vertically.
func TestProgressLine_Padding(t *testing.T) {
	cases := []struct {
		name              string
		idx, total        int
		basename          string
		wantHasSubstrings []string
	}{
		{
			name: "padded mid-corpus",
			idx:  12, total: 100, basename: "ragost_artifact.txt",
			wantHasSubstrings: []string{"[ 12/100]", "ragost_artifact.txt"},
		},
		{
			name: "first of a small corpus",
			idx:  1, total: 7, basename: "azula.txt",
			wantHasSubstrings: []string{"[1/7]", "azula.txt"},
		},
		{
			name: "last of a 1000-deck corpus",
			idx:  1000, total: 1000, basename: "ezuri.txt",
			wantHasSubstrings: []string{"[1000/1000]", "ezuri.txt"},
		},
		{
			name: "small index of large corpus padded to 4 digits",
			idx:  5, total: 1234, basename: "demo.txt",
			wantHasSubstrings: []string{"[   5/1234]", "demo.txt"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ProgressLine(c.idx, c.total, c.basename)
			for _, want := range c.wantHasSubstrings {
				if !strings.Contains(got, want) {
					t.Errorf("ProgressLine = %q, missing %q", got, want)
				}
			}
		})
	}
}

// TestProgressLine_AlignsAcrossRange pins the visual-alignment
// invariant: every line produced for the same total must share the
// same prefix-up-to-`/` width so a console viewer doesn't see
// shifting columns. This is the property that motivated the helper
// in the first place — a naked %d/%d would jitter at decade
// boundaries.
func TestProgressLine_AlignsAcrossRange(t *testing.T) {
	total := 150
	prefixLen := -1
	for i := 1; i <= total; i++ {
		line := ProgressLine(i, total, "x.txt")
		slash := strings.Index(line, "/")
		if slash < 0 {
			t.Fatalf("ProgressLine for i=%d missing slash: %q", i, line)
		}
		if prefixLen == -1 {
			prefixLen = slash
			continue
		}
		if slash != prefixLen {
			t.Errorf("ProgressLine for i=%d has slash at %d, prior lines had it at %d (alignment broken)",
				i, slash, prefixLen)
		}
	}
}

// TestVersionString_StaysInSyncWithCachePath defends the cache /
// version coupling: VersionString claims a cache-schema location and
// version, and CacheFilePath uses the same FreyaVersion to construct
// real paths. If someone bumps FreyaVersion in cache.go but forgets
// to re-render VersionString, this test fails because the embedded
// version no longer matches what the cache helper produces.
func TestVersionString_StaysInSyncWithCachePath(t *testing.T) {
	v := VersionString()
	path := CacheFilePath("/dummy", "abc")
	if !strings.Contains(v, FreyaVersion) {
		t.Fatalf("VersionString doesn't mention FreyaVersion (%q)\n%s", FreyaVersion, v)
	}
	if !strings.Contains(path, "v"+FreyaVersion+".json") {
		t.Errorf("CacheFilePath drift: %q lacks v%s.json", path, FreyaVersion)
	}
}
