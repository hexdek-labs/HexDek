package hexapi

import (
	"context"
	"encoding/json"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// freya_freshness — is this stored analysis still current?
//
// A deck's analysis is a file on disk (`<owner>/freya/<id>.strategy.json`)
// written by the Freya binary at some point in the past. Freya's
// conclusions change when Freya's code changes, and the stored file is
// not rewritten when that happens. So an analysis can be arbitrarily
// old relative to the logic the project now believes is correct, and
// until 2026-09-15 nothing recorded which version produced it —
// meaning "this may be out of date" was not a claim anything could
// make, however true it was.
//
// Freya now stamps `freya_version` and `generated_at` into every file
// it writes. This file supplies the other half: what version is
// running right now, so the two can be compared.

// currentFreyaVersion caches the running Freya binary's version
// string. Resolved once per process by executing `hexdek-freya
// --version` and reading the first line.
//
// Cached deliberately: this is consulted on every analysis read, and
// the answer cannot change without the binary being replaced, which
// means a restart. A subprocess per page view to learn a constant
// would be a self-inflicted load problem.
var (
	freyaVersionOnce  sync.Once
	freyaVersionValue string
)

// freyaVersionProbeTimeout bounds the one-shot version subprocess. The
// binary prints a version and exits, so this is generous; it exists so
// a wedged or wrong binary degrades to "unknown version" instead of
// hanging the first request that touches an analysis.
const freyaVersionProbeTimeout = 5 * time.Second

// CurrentFreyaVersion returns the version string of the Freya binary
// this server would execute, or "" when it can't be determined (binary
// missing, probe failed, unexpected output).
//
// An empty return is a real possibility and callers must handle it:
// test binaries don't ship hexdek-freya, and a deployment can be
// mid-swap. "" means "we don't know", and the correct behaviour when
// we don't know is to make no staleness claim at all — never to guess
// that a stored analysis is stale.
func CurrentFreyaVersion() string {
	freyaVersionOnce.Do(func() {
		freyaVersionValue = probeFreyaVersion()
	})
	return freyaVersionValue
}

func probeFreyaVersion() string {
	bin, ok := findFreyaBinary()
	if !ok {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), freyaVersionProbeTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		log.Printf("freya: version probe failed: %v", err)
		return ""
	}
	// First line is "hexdek-freya <version>"; the rest is cache/schema
	// detail we don't need here.
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	fields := strings.Fields(line)
	if len(fields) < 2 {
		log.Printf("freya: version probe returned unparseable first line %q", line)
		return ""
	}
	return fields[len(fields)-1]
}

// annotateAnalysisFreshness takes the raw bytes of a stored
// strategy.json and returns them with freshness fields added:
//
//	current_freya_version  the version running now ("" → field omitted)
//	stale                  true when the stored version is known to
//	                       differ from the current one
//
// The stored document is otherwise passed through untouched — every
// existing key keeps its value and its meaning. On any decode failure
// the original bytes are returned unchanged, because a freshness
// annotation is never worth failing a read the user would otherwise
// have gotten.
//
// Staleness requires knowing the CURRENT version. When the probe
// failed we know nothing, so no fields are added and no claim is made.
//
// When the current version IS known, a stored file with no
// `freya_version` is reported stale. That is not a guess from absent
// data: the field has been written by every Freya run since
// 2026-09-15, so a file lacking it was necessarily produced by an
// earlier build than the one running now. The frontend distinguishes
// the two stale cases by the presence of `generated_at` — "generated
// on <date> by <version>" versus "predates version tracking" — so the
// user gets the honest wording either way.
func annotateAnalysisFreshness(raw []byte) []byte {
	return annotateFreshnessWith(raw, CurrentFreyaVersion())
}

// annotateFreshnessWith is annotateAnalysisFreshness with the current
// version injected, so the decision can be tested at both settings.
// Test binaries don't ship the freya binary, which would otherwise
// make the "current version known" branch — the one that actually
// runs in production — unreachable from the test suite.
func annotateFreshnessWith(raw []byte, current string) []byte {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw
	}
	if doc == nil {
		return raw
	}

	if current == "" {
		// Nothing to compare against. Add no fields and make no claim.
		return raw
	}
	doc["current_freya_version"] = current

	stored, _ := doc["freya_version"].(string)
	doc["stale"] = stored != current

	annotated, err := json.Marshal(doc)
	if err != nil {
		return raw
	}
	return annotated
}
