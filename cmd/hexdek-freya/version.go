package main

import (
	"os"
	"runtime/debug"
	"strings"
	"sync"
)

// version — build-derived cache invalidation.
//
// Why this file exists
//
// The Freya report cache is keyed on (deck contents + FreyaVersion).
// Nothing in that key describes the CODE. So a classifier fix that
// changes what Freya concludes about a deck does not, by itself,
// invalidate a single cache entry: the new binary reads the old
// binary's answer off disk and prints it as its own.
//
// That is not a theoretical hazard. It happened on 2026-09-15: a
// cuttable_cards fix was made, the binary rebuilt, and the output was
// byte-identical except for a timestamp — because every deck hit the
// r60.1 cache. The build was green, the tests were green, and the fix
// reached nobody. The only signal was that the binary provably did not
// contain the string its own output was printing.
//
// The pre-existing mitigation was a hand-maintained constant with a
// comment asking the author to remember to bump it. That is a process
// control on a failure mode that produces no error, no warning, and no
// visible difference. It will be forgotten, and when it is forgotten
// nothing anywhere fails.
//
// So the token is now derived from the build itself:
//
//   - Clean build from a git checkout → freyaVersionBase + "-" + the
//     short VCS revision. Any commit changes the revision, so any
//     deployed change invalidates every cache entry automatically.
//     Cache hit rate goes to zero once per deploy, by design.
//
//   - Build from a MODIFIED tree (uncommitted edits — i.e. every
//     local dev iteration) → the cache is disabled outright, because
//     the revision stamp cannot distinguish one edit from the next.
//     This is precisely the situation that produced the 2026-09-15
//     incident, and a stale hit there is far more expensive than the
//     ~1-3s of analysis it saves.
//
//   - No VCS information at all → fall back to the base constant and
//     leave the cache enabled, matching the old semantics exactly.
//     This is the case for `go test`, tarball builds, and — verified
//     on 2026-09-15, not assumed — any build run from a LINKED GIT
//     WORKTREE, where `.git` is a file rather than a directory and the
//     Go toolchain stamps no vcs.revision at all (confirmed with
//     `go build -buildvcs=true` + `go version -m`; the same build from
//     the primary clone stamps normally).
//
//     That last one matters: in a worktree this whole mechanism is
//     inert and the old stale-cache trap is live again. Falling back
//     silently would be the exact failure shape this file exists to
//     remove, so VersionMode() reports which regime is in force and
//     --version prints it.
//
// Escape hatch: setting FREYA_CACHE_PIN=1 restores the old behaviour
// (bare base constant, cache always enabled) for anyone doing
// profiling or bulk corpus work where repeated cold analysis is the
// thing being measured. It is opt-in and never the default, so
// forgetting it is safe in the direction that matters.

// freyaVersionBase is the hand-maintained schema/semantics token. Bump
// it when a Freya change alters the FreyaReport schema or classifier
// output in a way worth naming. It is no longer load-bearing for cache
// correctness — the build revision covers that — but it remains the
// human-readable part of the version and the fallback when no VCS
// information is available.
//
// r60.2: the Utility-only cut penalties were removed. RoleUtility is
// the role tagger's FALLBACK for "no role matched", so penalising it
// turned "we could not classify this card" into "cut this card" —
// measured wrong on a real user deck three times out of three.
const freyaVersionBase = "r60.2"

// cachePinEnv, when set to a non-empty value other than "0"/"false",
// pins FreyaVersion to freyaVersionBase and force-enables the cache.
const cachePinEnv = "FREYA_CACHE_PIN"

var (
	versionOnce  sync.Once
	freyaVersion string
	cacheUsable  bool
	versionMode  string
)

// Version modes, reported by VersionMode() and printed by --version so
// the active cache-invalidation regime is never a thing you have to
// infer from behaviour.
const (
	// VersionModeBuild — clean build with VCS info. Auto-invalidation
	// active: any commit changes the token.
	VersionModeBuild = "build-derived"
	// VersionModeDirty — built from uncommitted edits. Cache disabled.
	VersionModeDirty = "dirty-tree (cache disabled)"
	// VersionModePinned — FREYA_CACHE_PIN set. Legacy behaviour.
	VersionModePinned = "pinned (FREYA_CACHE_PIN)"
	// VersionModeNoVCS — no VCS info available (go test, tarball build,
	// or a linked git worktree). Auto-invalidation INACTIVE: the token
	// is the hand-maintained base constant and the old
	// remember-to-bump-it discipline applies.
	VersionModeNoVCS = "no-vcs (auto-invalidation inactive)"
)

// FreyaVersion is the cache-invalidation token, baked into both the
// cache filename and the cache entry body, and reported by --version.
//
// It is a function rather than a constant because it is derived from
// the build. Call it freely — the derivation runs once.
func FreyaVersion() string {
	versionOnce.Do(computeVersion)
	return freyaVersion
}

// BuildCacheUsable reports whether the report cache may be used for
// THIS build. False for a binary built from a modified working tree,
// where the revision stamp cannot distinguish one edit from the next
// and a cache hit would serve the previous edit's conclusions.
//
// This is an additional gate, not a replacement for the --no-cache
// flag: --no-cache disables the cache for one run, BuildCacheUsable
// disables it for one binary.
func BuildCacheUsable() bool {
	versionOnce.Do(computeVersion)
	return cacheUsable
}

// VersionMode reports which cache-invalidation regime this binary is
// running under. Printed by --version.
func VersionMode() string {
	versionOnce.Do(computeVersion)
	return versionMode
}

func computeVersion() {
	rev, modified, haveVCS := vcsStamp()
	pinned := cachePinned()
	freyaVersion, cacheUsable = deriveVersion(pinned, rev, modified, haveVCS)
	versionMode = deriveVersionMode(pinned, rev, modified, haveVCS)
}

// deriveVersionMode names the regime deriveVersion selected. Kept
// alongside it (and tested against the same table) so the two can't
// drift into disagreeing about what the binary is doing.
func deriveVersionMode(pinned bool, rev string, modified, haveVCS bool) string {
	switch {
	case pinned:
		return VersionModePinned
	case !haveVCS || rev == "":
		return VersionModeNoVCS
	case modified:
		return VersionModeDirty
	default:
		return VersionModeBuild
	}
}

// deriveVersion is the whole decision, factored out as a pure function
// so it can be tested against every combination of inputs. The
// alternative — testing through the sync.Once — would require mutating
// process-global state and would make the tests order-dependent on
// each other.
//
// Returns the version token and whether the report cache may be used.
func deriveVersion(pinned bool, rev string, modified, haveVCS bool) (version string, cacheOK bool) {
	// The escape hatch wins over everything: someone who asked for the
	// legacy behaviour gets exactly the legacy behaviour.
	if pinned {
		return freyaVersionBase, true
	}
	// No VCS information (go test, tarball build, some vendored CI) —
	// behave exactly as the old hand-maintained constant did.
	if !haveVCS || rev == "" {
		return freyaVersionBase, true
	}
	if modified {
		// Built from uncommitted edits. The revision is identical
		// across every edit in the session, so it cannot serve as a
		// cache key — and this is the precise situation that produced
		// the 2026-09-15 stale-cache incident.
		return freyaVersionBase + "-" + rev + "-dirty", false
	}
	return freyaVersionBase + "-" + rev, true
}

// cachePinned reports whether FREYA_CACHE_PIN asks for the legacy
// hand-maintained-constant behaviour. Any value counts as "on" except
// the empty string, "0" and "false", so `FREYA_CACHE_PIN=1` and
// `FREYA_CACHE_PIN=yes` both work and `FREYA_CACHE_PIN=0` does not
// silently enable it.
func cachePinned() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(cachePinEnv)))
	switch v {
	case "", "0", "false":
		return false
	}
	return true
}

// vcsStamp reads the VCS revision and dirty flag that the Go toolchain
// embeds at build time (Go 1.18+, on by default when building inside a
// git checkout). Returns ok=false when the binary carries no build
// info or no vcs.revision setting.
//
// The revision is truncated to 12 hex characters: long enough that a
// collision across the project's history is not a practical concern,
// short enough to keep cache filenames readable.
func vcsStamp() (rev string, modified bool, ok bool) {
	info, available := debug.ReadBuildInfo()
	if !available || info == nil {
		return "", false, false
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if rev == "" {
		return "", false, false
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return rev, modified, true
}
