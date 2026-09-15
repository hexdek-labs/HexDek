package main

import (
	"strings"
	"testing"
)

// The build-derived version exists to close a failure mode that
// produces no error: a code change that alters Freya's conclusions
// while every deck keeps serving the previous build's cached answer.
// These tests pin the properties that failure mode depends on.

func TestFreyaVersion_NonEmptyAndPrefixed(t *testing.T) {
	v := FreyaVersion()
	if strings.TrimSpace(v) == "" {
		t.Fatal("FreyaVersion() is empty — an empty token makes every cache " +
			"filename collapse to the same unversioned shape, so no cache " +
			"entry could ever be invalidated")
	}
	if !strings.HasPrefix(v, "r") {
		t.Errorf("FreyaVersion() = %q, want the r-prefixed convention (e.g. r60.2)", v)
	}
	if !strings.HasPrefix(v, freyaVersionBase) {
		t.Errorf("FreyaVersion() = %q, want it to start with the base token %q so "+
			"the human-readable version stays legible in filenames and --version",
			v, freyaVersionBase)
	}
}

func TestFreyaVersion_Stable(t *testing.T) {
	// The value is baked into cache filenames AND compared against the
	// version embedded in cache file bodies. If it varied between calls
	// within one process, every write would land under a name the next
	// read couldn't find, silently disabling the cache.
	first := FreyaVersion()
	for i := 0; i < 5; i++ {
		if got := FreyaVersion(); got != first {
			t.Fatalf("FreyaVersion() not stable: call 0 = %q, call %d = %q", first, i+1, got)
		}
	}
}

func TestFreyaVersion_FilenameSafe(t *testing.T) {
	// FreyaVersion() is concatenated into a path by CacheFilePath. A
	// separator or wildcard in it would write cache entries to
	// unintended locations.
	v := FreyaVersion()
	for _, bad := range []string{"/", `\`, "*", "?", ":", "\"", "<", ">", "|", " ", "\n", "\t"} {
		if strings.Contains(v, bad) {
			t.Errorf("FreyaVersion() = %q contains %q, which is not safe in a filename", v, bad)
		}
	}
}

func TestCachePinned_OnlyExplicitTruthyValues(t *testing.T) {
	// The escape hatch restores the old always-cache behaviour. Getting
	// this backwards would silently re-enable stale hits for everyone
	// who happens to have the variable set to "0".
	cases := []struct {
		val  string
		want bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"FALSE", false},
		{"  false  ", false},
		{"1", true},
		{"yes", true},
		{"true", true},
	}
	for _, tc := range cases {
		t.Setenv(cachePinEnv, tc.val)
		if got := cachePinned(); got != tc.want {
			t.Errorf("cachePinned() with %s=%q = %v, want %v", cachePinEnv, tc.val, got, tc.want)
		}
	}
}

func TestVcsStamp_TruncatesRevision(t *testing.T) {
	// Not asserting that VCS info is present — it isn't under `go test`
	// in every environment, and the fallback path is legitimate. What
	// must hold is that IF a revision is reported it is short enough to
	// keep filenames readable.
	rev, _, ok := vcsStamp()
	if !ok {
		t.Skip("no VCS build info in this binary — fallback path, nothing to assert")
	}
	if len(rev) > 12 {
		t.Errorf("vcsStamp() revision = %q (%d chars), want <= 12", rev, len(rev))
	}
	if rev == "" {
		t.Error("vcsStamp() reported ok with an empty revision")
	}
}

// TestDeriveVersion covers the whole decision table. deriveVersion is
// pure precisely so this can assert every branch without touching the
// process-global sync.Once that FreyaVersion()/BuildCacheUsable() sit
// behind.
func TestDeriveVersion(t *testing.T) {
	const rev = "a1b2c3d4e5f6"

	cases := []struct {
		name        string
		pinned      bool
		rev         string
		modified    bool
		haveVCS     bool
		wantVersion string
		wantCacheOK bool
		why         string
	}{
		{
			name: "clean build stamps the revision and keeps the cache",
			rev:  rev, haveVCS: true,
			wantVersion: freyaVersionBase + "-" + rev, wantCacheOK: true,
			why: "this is the production path — the revision changes on every " +
				"commit, so a deploy invalidates the corpus automatically",
		},
		{
			name: "dirty build disables the cache",
			rev:  rev, modified: true, haveVCS: true,
			wantVersion: freyaVersionBase + "-" + rev + "-dirty", wantCacheOK: false,
			why: "THE regression this file exists for: with uncommitted edits the " +
				"revision is constant across edits, so a cache hit serves the " +
				"previous edit's conclusions to the current binary",
		},
		{
			name:    "no VCS info falls back to the base token with the cache on",
			haveVCS: false,
			// go test, tarball builds, some vendored CI.
			wantVersion: freyaVersionBase, wantCacheOK: true,
			why: "must match the old constant's behaviour exactly so the test " +
				"suite and non-git builds are unaffected",
		},
		{
			name:    "ok-but-empty revision is treated as no VCS info",
			rev:     "",
			haveVCS: true,
			// Defensive: vcsStamp shouldn't return this, but an empty
			// token would collapse every cache filename together.
			wantVersion: freyaVersionBase, wantCacheOK: true,
			why: "an empty revision would produce the trailing-dash token " +
				"'r60.2-' rather than anything meaningful",
		},
		{
			name:   "pin overrides a dirty build",
			pinned: true, rev: rev, modified: true, haveVCS: true,
			wantVersion: freyaVersionBase, wantCacheOK: true,
			why: "the escape hatch is for profiling runs that need the cache; " +
				"if dirty still won, the hatch would do nothing in the one " +
				"situation anyone would reach for it",
		},
		{
			name:   "pin overrides a clean build",
			pinned: true, rev: rev, haveVCS: true,
			wantVersion: freyaVersionBase, wantCacheOK: true,
			why: "pinned means the legacy bare constant, with no revision suffix",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotVersion, gotCacheOK := deriveVersion(tc.pinned, tc.rev, tc.modified, tc.haveVCS)
			if gotVersion != tc.wantVersion {
				t.Errorf("version = %q, want %q\n  %s", gotVersion, tc.wantVersion, tc.why)
			}
			if gotCacheOK != tc.wantCacheOK {
				t.Errorf("cacheOK = %v, want %v\n  %s", gotCacheOK, tc.wantCacheOK, tc.why)
			}
		})
	}
}

// TestDeriveVersion_DistinctRevisionsDistinctTokens is the property the
// whole mechanism rests on: two different commits must not share a
// cache namespace. If this ever passes trivially (both tokens equal),
// every deploy would silently reuse the previous build's answers.
func TestDeriveVersion_DistinctRevisionsDistinctTokens(t *testing.T) {
	a, _ := deriveVersion(false, "aaaaaaaaaaaa", false, true)
	b, _ := deriveVersion(false, "bbbbbbbbbbbb", false, true)
	if a == b {
		t.Fatalf("two different revisions produced the same version token %q — "+
			"cache entries from one build would be served to the other", a)
	}
}
