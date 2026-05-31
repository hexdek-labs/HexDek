package main

import (
	"fmt"
	"sort"
	"strings"
)

// Pattern-mode clusters fabrication TraceResults by the engine code
// path closest to the mint-bypass site. The key insight from PR #839's
// single-shot tracing: every fabricated InstanceID's first reference
// is the engine event that first OBSERVED the rogue *Card — i.e. the
// closest event boundary to the bypass site. Many fabrications across
// many games will share the same first-event shape because they all
// trace to the same handful of bypass paths.
//
// Clustering shrinks "spelunk 80 hits / 6 distinct IDs / 4 games" to
// "this 3-line ranked table — fix the top entry, expect the cluster
// to drop by N." Each row is a hypothesis: "this engine entry point
// is the most likely bypass site."
//
// The pattern key intentionally excludes the seat number and CMC bin
// — they vary game-to-game and would shatter clusters that are
// fundamentally the same code path. Provenance stays in the key
// because OG / TK / CP fabricate from genuinely different code paths
// (deck-load mint vs token-mint vs copy-mint).

// PatternKey is the cluster discriminator. Same key ↔ same bypass
// hypothesis; different keys ↔ separate bisects.
type PatternKey struct {
	// Provenance from the §3 InstanceID layout. "<bad>" if the ID
	// itself was undecodable (rare — usually means the regex caught
	// something weird).
	Provenance string

	// FirstEventKind is the Kind of the first event the trace
	// matched against the fabricated ID. "<not_found>" when the
	// trace returned NotFound (no event referenced the ID at all —
	// the strongest mint-bypass signal, usually meaning the *Card
	// only appeared after a hand-rolled struct-literal).
	FirstEventKind string

	// MatchKind is how the first reference was identified
	// ("instance_id" / "card_name" / "<none>"). Splitting on this
	// matters: instance_id matches mean SOMETHING is logging the
	// ID, so the event is the bypass site itself or one step
	// downstream. card_name fallbacks mean the engine never knew
	// the ID at all — the bypass is more upstream than the first
	// observed event.
	MatchKind string
}

// String renders the key as a single-line cluster label for the
// rank table. Stable / pure / used by tests.
func (k PatternKey) String() string {
	return fmt.Sprintf("prov=%s first=%s match=%s",
		k.Provenance, k.FirstEventKind, k.MatchKind)
}

// PatternEntry aggregates the hits sharing a single PatternKey, with
// bounded samples so the report stays scannable even when one cluster
// has hundreds of hits (the r60 fresh-main h1OGVR200096 in game 411
// alone is 46 hits — they should collapse to ONE row).
type PatternEntry struct {
	Hits         int      // total fabrication hits in this cluster
	UniqueIDs    int      // distinct InstanceIDs in this cluster (≤ Hits)
	SampleIDs    []string // up to 3 representative InstanceIDs
	SampleCards  []string // up to 3 representative card names (aligned w/ IDs)
	SampleEvent  string   // representative first-event description
	uniqueIDSet  map[string]struct{}
}

const sampleCap = 3

// PatternAggregate is the rolled-up cluster map. Returned by
// BuildPatternAggregate; passed to RenderPatternSummary + accumulated
// across replays in watch mode.
type PatternAggregate struct {
	ByKey         map[PatternKey]*PatternEntry
	TotalTraces   int
	ReplaysSeen   int
}

// NewPatternAggregate constructs an empty aggregate ready for
// incremental Merge calls (watch mode's per-replay accumulation).
func NewPatternAggregate() *PatternAggregate {
	return &PatternAggregate{
		ByKey: map[PatternKey]*PatternEntry{},
	}
}

// MergeTraces folds a batch of TraceResults (typically one replay's
// worth) into the aggregate. Idempotent w.r.t. multiple Merge calls;
// not idempotent w.r.t. duplicate TraceResults (same hit counted
// twice if Merge'd twice — caller's responsibility to dedupe).
func (a *PatternAggregate) MergeTraces(traces []TraceResult) {
	if a == nil {
		return
	}
	for _, t := range traces {
		k := PatternKey{
			Provenance:     decodedProvOrBad(t.Decoded),
			FirstEventKind: firstEventKindOrNotFound(t.First),
			MatchKind:      t.First.MatchKind,
		}
		entry, ok := a.ByKey[k]
		if !ok {
			entry = &PatternEntry{uniqueIDSet: map[string]struct{}{}}
			a.ByKey[k] = entry
		}
		entry.Hits++
		if _, seen := entry.uniqueIDSet[t.InstanceID]; !seen {
			entry.uniqueIDSet[t.InstanceID] = struct{}{}
			entry.UniqueIDs++
			if len(entry.SampleIDs) < sampleCap {
				entry.SampleIDs = append(entry.SampleIDs, t.InstanceID)
				entry.SampleCards = append(entry.SampleCards, sampleCardName(t))
			}
		}
		if entry.SampleEvent == "" && !t.First.NotFound {
			entry.SampleEvent = fmt.Sprintf("kind=%q source=%q seat=%d",
				t.First.Event.Kind, t.First.Event.Source, t.First.Event.Seat)
		}
		a.TotalTraces++
	}
}

// MergeReplay is the watch-mode convenience: AnalyzeReplay + MergeTraces
// + ReplaysSeen bump in one call.
func (a *PatternAggregate) MergeReplay(r *Replay) {
	if a == nil || r == nil {
		return
	}
	a.MergeTraces(AnalyzeReplay(r))
	a.ReplaysSeen++
}

// BuildPatternAggregate is the one-shot variant — convenience wrapper
// over NewPatternAggregate + MergeReplay for the non-watch CLI path.
func BuildPatternAggregate(r *Replay) *PatternAggregate {
	agg := NewPatternAggregate()
	agg.MergeReplay(r)
	return agg
}

// RankedKeys returns the cluster keys ordered by Hits desc, breaking
// ties with UniqueIDs desc, then with the key's String form for total
// determinism.
func (a *PatternAggregate) RankedKeys() []PatternKey {
	if a == nil {
		return nil
	}
	keys := make([]PatternKey, 0, len(a.ByKey))
	for k := range a.ByKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ei := a.ByKey[keys[i]]
		ej := a.ByKey[keys[j]]
		if ei.Hits != ej.Hits {
			return ei.Hits > ej.Hits
		}
		if ei.UniqueIDs != ej.UniqueIDs {
			return ei.UniqueIDs > ej.UniqueIDs
		}
		return keys[i].String() < keys[j].String()
	})
	return keys
}

// RenderPatternSummary formats the aggregate as a human-readable
// post-mortem table. Idempotent / pure. Returns "" when no clusters
// exist (caller decides whether to print "no fabrications" message).
func RenderPatternSummary(a *PatternAggregate) string {
	if a == nil || len(a.ByKey) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Pattern aggregate — %d fabrication traces across %d replay(s), %d distinct cluster(s)\n",
		a.TotalTraces, a.ReplaysSeen, len(a.ByKey))
	fmt.Fprintln(&b)
	for rank, k := range a.RankedKeys() {
		e := a.ByKey[k]
		fmt.Fprintf(&b, "#%d  hits=%d  unique_ids=%d  %s\n", rank+1, e.Hits, e.UniqueIDs, k.String())
		for i, id := range e.SampleIDs {
			card := ""
			if i < len(e.SampleCards) {
				card = e.SampleCards[i]
			}
			fmt.Fprintf(&b, "       sample: %s (%s)\n", id, sampleOrUnknown(card))
		}
		if e.SampleEvent != "" {
			fmt.Fprintf(&b, "       first event: %s\n", e.SampleEvent)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

// decodedProvOrBad returns the decoded provenance string or "<bad>"
// when the InstanceID layout didn't parse. Keeps clusters separate
// when one bypass path emits malformed IDs that another doesn't.
func decodedProvOrBad(d DecodedInstanceID) string {
	if !d.Valid {
		return "<bad>"
	}
	return d.Provenance
}

// firstEventKindOrNotFound returns the first-event Kind or the
// sentinel "<not_found>" so the not-found cluster has a stable label.
func firstEventKindOrNotFound(f FirstAppearance) string {
	if f.NotFound {
		return "<not_found>"
	}
	if f.Event.Kind == "" {
		return "<empty_kind>"
	}
	return f.Event.Kind
}

// sampleCardName returns the trace's card name with a friendly
// fallback when CardIndex didn't resolve it.
func sampleCardName(t TraceResult) string {
	if t.CardName == "" {
		return "<no card_index entry>"
	}
	return t.CardName
}

// sampleOrUnknown is the inverse formatter for the rank table.
func sampleOrUnknown(s string) string {
	if s == "" {
		return "<unknown>"
	}
	return s
}
