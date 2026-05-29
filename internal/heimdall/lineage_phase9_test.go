package heimdall

import (
	"strings"
	"testing"
)

// Property: BuildLineageTree walks Source + Enabler chains acyclically.
// Set up a 3-deep lineage: Thopter token (TK) → minted by ability
// instance (AB), which itself was triggered by Sai (OG).
func TestPhase9_BuildLineageTree_WalksAcyclic(t *testing.T) {
	records := map[string]LineageRecord{
		"h0OGVW20042": {
			InstanceID: "h0OGVW20042",
			Name:       "Sai of the Shining Sword",
			Provenance: "OG",
		},
		"h0ABVW20456": {
			InstanceID:        "h0ABVW20456",
			Name:              "Sai trigger",
			Provenance:        "AB",
			SourceInstanceID:  "h0OGVW20042",
			EnablerInstanceID: "h0OGVW20042",
		},
		"h0TKVC01234": {
			InstanceID:        "h0TKVC01234",
			Name:              "Thopter Token",
			Provenance:        "TK",
			EnablerInstanceID: "h0ABVW20456",
		},
	}
	tree := BuildLineageTree(records, "h0TKVC01234")
	if tree == nil {
		t.Fatal("BuildLineageTree returned nil for known ID")
	}
	if tree.InstanceID != "h0TKVC01234" {
		t.Fatalf("root InstanceID = %q, want %q", tree.InstanceID, "h0TKVC01234")
	}
	if tree.Enabler == nil {
		t.Fatal("expected enabler subtree for TK provenance")
	}
	if tree.Enabler.InstanceID != "h0ABVW20456" {
		t.Fatalf("enabler InstanceID = %q, want %q", tree.Enabler.InstanceID, "h0ABVW20456")
	}
	if tree.Enabler.Source == nil || tree.Enabler.Source.InstanceID != "h0OGVW20042" {
		t.Fatalf("expected enabler.Source to walk back to Sai OG")
	}
}

// Property: cycle in the lineage graph terminates rather than recursing
// forever. Build a self-source loop (A's SourceInstanceID points at A);
// the walker must NOT stack-overflow.
func TestPhase9_BuildLineageTree_AcyclicGuard(t *testing.T) {
	records := map[string]LineageRecord{
		"h0CPVR10001": {
			InstanceID:       "h0CPVR10001",
			Name:             "self-cycle",
			Provenance:       "CP",
			SourceInstanceID: "h0CPVR10001", // self-reference
		},
	}
	tree := BuildLineageTree(records, "h0CPVR10001")
	if tree == nil {
		t.Fatal("walker dropped a real ID on cycle")
	}
	// Source should be nil (visited-guard rejected re-entry).
	if tree.Source != nil {
		t.Errorf("acyclicity violated: tree.Source is non-nil for self-cycle, want nil")
	}
}

// Property: a 2-cycle (A→B, B→A) terminates without infinite recursion.
func TestPhase9_BuildLineageTree_TwoCycleGuard(t *testing.T) {
	records := map[string]LineageRecord{
		"h0CPVR10001": {InstanceID: "h0CPVR10001", Provenance: "CP", SourceInstanceID: "h0CPVR10002"},
		"h0CPVR10002": {InstanceID: "h0CPVR10002", Provenance: "CP", SourceInstanceID: "h0CPVR10001"},
	}
	tree := BuildLineageTree(records, "h0CPVR10001")
	if tree == nil || tree.Source == nil {
		t.Fatal("expected one walk-step into the partner")
	}
	if tree.Source.Source != nil {
		t.Error("2-cycle re-entered root: visited-guard failed")
	}
}

// Property: unknown ID returns nil at root, but mid-walk emits a stub
// "(unknown)" node so callers see the dangling reference.
func TestPhase9_BuildLineageTree_UnknownRootReturnsNil(t *testing.T) {
	records := map[string]LineageRecord{}
	if got := BuildLineageTree(records, "h0OGVW20999"); got != nil {
		t.Errorf("unknown root must return nil, got %#v", got)
	}
}

// Property: empty / nil records map gracefully returns nil.
func TestPhase9_BuildLineageTree_EmptyInputs(t *testing.T) {
	if BuildLineageTree(nil, "h0OGVW20042") != nil {
		t.Error("nil records map must return nil")
	}
	if BuildLineageTree(map[string]LineageRecord{}, "") != nil {
		t.Error("empty ID must return nil")
	}
}

// Property: RenderLineageText produces a non-empty multi-line string
// matching the design v2 §13 example shape — root description first,
// then a "via enabler:" line for TK provenance.
func TestPhase9_RenderLineageText_MatchesExampleShape(t *testing.T) {
	records := map[string]LineageRecord{
		"h0OGVW20042": {InstanceID: "h0OGVW20042", Name: "Sai of the Shining Sword", Provenance: "OG"},
		"h0ABVW20456": {InstanceID: "h0ABVW20456", Name: "Sai trigger", Provenance: "AB", SourceInstanceID: "h0OGVW20042"},
		"h0TKVC01234": {InstanceID: "h0TKVC01234", Name: "Thopter Token", Provenance: "TK", EnablerInstanceID: "h0ABVW20456"},
	}
	tree := BuildLineageTree(records, "h0TKVC01234")
	out := RenderLineageText(tree)
	if out == "" {
		t.Fatal("RenderLineageText returned empty string for known tree")
	}
	if !strings.Contains(out, "Thopter Token") {
		t.Errorf("rendered text missing root name. Got:\n%s", out)
	}
	if !strings.Contains(out, "via enabler:") {
		t.Errorf("rendered text missing enabler line for TK provenance. Got:\n%s", out)
	}
	if !strings.Contains(out, "h0TKVC01234") {
		t.Error("rendered text missing root InstanceID")
	}
}

// Property: Merged children are walked once each (Mutate / Meld
// lineage). 3 merged cards under one top card all appear in the tree.
func TestPhase9_BuildLineageTree_WalksMergedChildren(t *testing.T) {
	records := map[string]LineageRecord{
		"h0OGVG40001": {InstanceID: "h0OGVG40001", Name: "Top Card", Provenance: "OG", MergedCardIDs: []string{"h0OGVG40002", "h0OGVG40003"}},
		"h0OGVG40002": {InstanceID: "h0OGVG40002", Name: "Merged 1", Provenance: "OG"},
		"h0OGVG40003": {InstanceID: "h0OGVG40003", Name: "Merged 2", Provenance: "OG"},
	}
	tree := BuildLineageTree(records, "h0OGVG40001")
	if tree == nil {
		t.Fatal("tree nil for merged top card")
	}
	if len(tree.Merged) != 2 {
		t.Fatalf("expected 2 merged children, got %d", len(tree.Merged))
	}
	got := map[string]bool{tree.Merged[0].InstanceID: true, tree.Merged[1].InstanceID: true}
	for _, want := range []string{"h0OGVG40002", "h0OGVG40003"} {
		if !got[want] {
			t.Errorf("missing merged child %q", want)
		}
	}
}
