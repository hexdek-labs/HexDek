package main

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestPatternKey_String_Stable(t *testing.T) {
	k := PatternKey{Provenance: "OG", FirstEventKind: "enter_battlefield", MatchKind: "instance_id"}
	got := k.String()
	want := "prov=OG first=enter_battlefield match=instance_id"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildPatternAggregate_MultiClusterFixture(t *testing.T) {
	r, err := LoadReplay("testdata/replay-multi-cluster.json")
	if err != nil {
		t.Fatalf("LoadReplay: %v", err)
	}
	agg := BuildPatternAggregate(r)
	if agg.TotalTraces != 5 {
		t.Fatalf("TotalTraces: got %d, want 5", agg.TotalTraces)
	}
	if agg.ReplaysSeen != 1 {
		t.Fatalf("ReplaysSeen: got %d, want 1", agg.ReplaysSeen)
	}
	if len(agg.ByKey) != 3 {
		t.Fatalf("clusters: got %d, want 3", len(agg.ByKey))
	}

	// Cluster A — OG + enter_battlefield + instance_id should be
	// the top-ranked (3 hits, 2 unique IDs).
	ranked := agg.RankedKeys()
	topKey := ranked[0]
	if topKey.Provenance != "OG" || topKey.FirstEventKind != "enter_battlefield" || topKey.MatchKind != "instance_id" {
		t.Fatalf("top cluster wrong: %+v", topKey)
	}
	topEntry := agg.ByKey[topKey]
	if topEntry.Hits != 3 || topEntry.UniqueIDs != 2 {
		t.Fatalf("top cluster counts: hits=%d unique=%d, want 3+2", topEntry.Hits, topEntry.UniqueIDs)
	}

	// Cluster B — TK + creature_attacks + card_name (1 hit, 1 unique).
	found := false
	for _, k := range ranked {
		if k.Provenance == "TK" && k.FirstEventKind == "creature_attacks" && k.MatchKind == "card_name" {
			found = true
			if agg.ByKey[k].Hits != 1 {
				t.Fatalf("TK cluster hits: got %d, want 1", agg.ByKey[k].Hits)
			}
		}
	}
	if !found {
		t.Fatalf("TK card_name cluster missing")
	}

	// Cluster C — OG + <not_found> + <none> (1 hit, 1 unique) —
	// Bayou Dragonfly never referenced in events.
	found = false
	for _, k := range ranked {
		if k.FirstEventKind == "<not_found>" && k.MatchKind == "<none>" {
			found = true
		}
	}
	if !found {
		t.Fatalf("not_found cluster missing")
	}
}

func TestPatternAggregate_RankOrder_TiesByUniqueIDs(t *testing.T) {
	agg := NewPatternAggregate()
	// Two clusters with same Hits=2 but different UniqueIDs.
	agg.MergeTraces([]TraceResult{
		// Cluster X — 2 hits, 2 unique IDs.
		mkTrace("h0OGVR100001", "Card X1", gameengine.Event{Kind: "enter_battlefield"}, "instance_id"),
		mkTrace("h0OGVR100002", "Card X2", gameengine.Event{Kind: "enter_battlefield"}, "instance_id"),
		// Cluster Y — 2 hits, 1 unique ID.
		mkTrace("h1OGVU200001", "Card Y", gameengine.Event{Kind: "creature_attacks"}, "card_name"),
		mkTrace("h1OGVU200001", "Card Y", gameengine.Event{Kind: "creature_attacks"}, "card_name"),
	})
	ranked := agg.RankedKeys()
	if len(ranked) != 2 {
		t.Fatalf("clusters: got %d, want 2", len(ranked))
	}
	// Tie on Hits=2, Cluster X wins on UniqueIDs=2 vs Cluster Y's 1.
	if ranked[0].FirstEventKind != "enter_battlefield" {
		t.Fatalf("tie-break wrong; want enter_battlefield first, got %s", ranked[0].FirstEventKind)
	}
}

func TestPatternAggregate_MergeAcrossReplays(t *testing.T) {
	agg := NewPatternAggregate()

	r1 := &Replay{
		GameIdx: 411,
		Violations: []ViolationRecord{
			{Turn: 23, Invariant: "ZoneConservation", Message: `ZoneConservation: InstanceID "h1OGVR200096" present in a zone but not in (Minted - Ceased) — fabrication or stale ceased entry`},
		},
		Events: []gameengine.Event{
			{Kind: "enter_battlefield", Seat: 1, Source: "Goblin Bushwhacker", Details: map[string]interface{}{"instance_id": "h1OGVR200096"}},
		},
		CardIndex: map[string]string{"h1OGVR200096": "Goblin Bushwhacker"},
	}
	r2 := &Replay{
		GameIdx: 2762,
		Violations: []ViolationRecord{
			{Turn: 17, Invariant: "ZoneConservation", Message: `ZoneConservation: InstanceID "h1OGVR200056" present in a zone but not in (Minted - Ceased) — fabrication or stale ceased entry`},
		},
		Events: []gameengine.Event{
			{Kind: "enter_battlefield", Seat: 1, Source: "Lava Spike", Details: map[string]interface{}{"instance_id": "h1OGVR200056"}},
		},
		CardIndex: map[string]string{"h1OGVR200056": "Lava Spike"},
	}
	agg.MergeReplay(r1)
	agg.MergeReplay(r2)

	if agg.ReplaysSeen != 2 {
		t.Fatalf("ReplaysSeen: got %d, want 2", agg.ReplaysSeen)
	}
	if agg.TotalTraces != 2 {
		t.Fatalf("TotalTraces: got %d, want 2", agg.TotalTraces)
	}
	if len(agg.ByKey) != 1 {
		t.Fatalf("expected 2 fabrications to collapse to 1 cluster (same code-path key); got %d", len(agg.ByKey))
	}
	// The collapsed cluster should report 2 unique IDs.
	for _, e := range agg.ByKey {
		if e.UniqueIDs != 2 {
			t.Fatalf("unique IDs across replays: got %d, want 2", e.UniqueIDs)
		}
	}
}

func TestRenderPatternSummary_Empty(t *testing.T) {
	if got := RenderPatternSummary(nil); got != "" {
		t.Fatalf("nil aggregate must return empty; got %q", got)
	}
	if got := RenderPatternSummary(NewPatternAggregate()); got != "" {
		t.Fatalf("empty aggregate must return empty; got %q", got)
	}
}

func TestRenderPatternSummary_Output(t *testing.T) {
	r, err := LoadReplay("testdata/replay-multi-cluster.json")
	if err != nil {
		t.Fatalf("LoadReplay: %v", err)
	}
	agg := BuildPatternAggregate(r)
	out := RenderPatternSummary(agg)
	for _, want := range []string{
		"5 fabrication traces", "1 replay(s)", "3 distinct cluster(s)",
		"#1", "hits=3", "unique_ids=2", "prov=OG", "first=enter_battlefield", "match=instance_id",
		"Lava Spike", "Fireblast",
		"first=<not_found>", "match=<none>", "Bayou Dragonfly",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// mkTrace is a tiny test-only constructor for raw TraceResult fixtures
// where building a full Replay would be overkill.
func mkTrace(id, name string, first gameengine.Event, matchKind string) TraceResult {
	d := DecodeInstanceID(id)
	return TraceResult{
		InstanceID: id,
		Decoded:    d,
		CardName:   name,
		First: FirstAppearance{
			EventIdx:  0,
			Event:     first,
			MatchKind: matchKind,
			NotFound:  matchKind == "<none>",
		},
	}
}
