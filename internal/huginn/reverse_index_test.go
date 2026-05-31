package huginn

import (
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// withTempDir returns a fresh tmp dir and resets the reverse-index
// singleton both before and after the test. Tests must always scope
// to a tmp dir to avoid stomping the real data/huginn/ tree.
func withTempDir(t *testing.T) string {
	t.Helper()
	ResetReverseIndex()
	t.Cleanup(ResetReverseIndex)
	return t.TempDir()
}

func TestBuildReverseIndex_EmptyDir(t *testing.T) {
	dir := withTempDir(t)
	if err := BuildReverseIndex(dir); err != nil {
		t.Fatalf("BuildReverseIndex on empty dir: %v", err)
	}
	if got := LookupReverseIndex("orc-x"); got != nil {
		t.Fatalf("expected nil for unseen oracle, got %v", got)
	}
	if got := ProvenanceOf("hX"); got != ProvUnknown {
		t.Fatalf("expected ProvUnknown for unseen IID, got %v", got)
	}
}

func TestAppendAndRead_RoundTripWithDedup(t *testing.T) {
	dir := withTempDir(t)
	obs := []InstanceObservation{
		{InstanceID: "h0OGVR300001", OracleID: "orc-bolt", CardName: "Lightning Bolt", Provenance: ProvOG, SeatIdx: 0, GameID: "g1", Turn: 1},
		// Same InstanceID re-observed later in the game — dedup target.
		{InstanceID: "h0OGVR300001", OracleID: "orc-bolt", CardName: "Lightning Bolt", Provenance: ProvOG, SeatIdx: 0, GameID: "g1", Turn: 4},
		{InstanceID: "h0CPVR300002", OracleID: "orc-bolt", CardName: "Lightning Bolt", Provenance: ProvCP, SourceInstanceID: "h0OGVR300001", SeatIdx: 0, GameID: "g1", Turn: 4},
		{InstanceID: "h1OGVG100003", OracleID: "orc-bop", CardName: "Birds of Paradise", Provenance: ProvOG, SeatIdx: 1, GameID: "g1", Turn: 1},
		{InstanceID: "h1TKVG100004", OracleID: "orc-bop", CardName: "Birds of Paradise", Provenance: ProvTK, EnablerInstanceID: "h1ABVR200099", SeatIdx: 1, GameID: "g1", Turn: 6},
	}
	if err := AppendInstanceObservations(dir, obs); err != nil {
		t.Fatalf("AppendInstanceObservations: %v", err)
	}
	read, err := ReadInstanceObservations(dir)
	if err != nil {
		t.Fatalf("ReadInstanceObservations: %v", err)
	}
	if len(read) != len(obs) {
		t.Fatalf("read back %d obs, want %d", len(read), len(obs))
	}

	if err := BuildReverseIndex(dir); err != nil {
		t.Fatalf("BuildReverseIndex: %v", err)
	}

	bolt := LookupReverseIndex("orc-bolt")
	sort.Strings(bolt)
	want := []string{"h0CPVR300002", "h0OGVR300001"}
	if !reflect.DeepEqual(bolt, want) {
		t.Fatalf("LookupReverseIndex(bolt) = %v, want %v (dedup of duplicate OG obs)", bolt, want)
	}

	bop := LookupReverseIndex("orc-bop")
	sort.Strings(bop)
	wantBop := []string{"h1OGVG100003", "h1TKVG100004"}
	if !reflect.DeepEqual(bop, wantBop) {
		t.Fatalf("LookupReverseIndex(bop) = %v, want %v", bop, wantBop)
	}
}

func TestProvenanceOf_KnownAndUnknown(t *testing.T) {
	dir := withTempDir(t)
	obs := []InstanceObservation{
		{InstanceID: "iidOG", OracleID: "orc-1", Provenance: ProvOG},
		{InstanceID: "iidTK", OracleID: "orc-2", Provenance: ProvTK},
		{InstanceID: "iidCP", OracleID: "orc-3", Provenance: ProvCP},
		{InstanceID: "iidAB", OracleID: "", Provenance: ProvAB}, // empty oracle still populates prov map
	}
	if err := AppendInstanceObservations(dir, obs); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := BuildReverseIndex(dir); err != nil {
		t.Fatalf("build: %v", err)
	}
	cases := map[string]Provenance{
		"iidOG": ProvOG,
		"iidTK": ProvTK,
		"iidCP": ProvCP,
		"iidAB": ProvAB,
	}
	for iid, want := range cases {
		if got := ProvenanceOf(iid); got != want {
			t.Errorf("ProvenanceOf(%s) = %v, want %v", iid, got, want)
		}
	}
	if got := ProvenanceOf("never-seen"); got != ProvUnknown {
		t.Errorf("ProvenanceOf(unseen) = %v, want ProvUnknown", got)
	}
	// AB observation has empty OracleID so it should NOT appear in any
	// reverse-index bucket.
	if got := LookupReverseIndex(""); got != nil {
		t.Errorf("LookupReverseIndex(\"\") = %v, want nil (empty OracleID is dropped)", got)
	}
}

func TestDeriveLineage_Table(t *testing.T) {
	og := InstanceObservation{InstanceID: "iidA"}
	copyOfA := InstanceObservation{InstanceID: "iidB", SourceInstanceID: "iidA"}
	otherCopyOfA := InstanceObservation{InstanceID: "iidC", SourceInstanceID: "iidA"}
	sharedSrcX := InstanceObservation{InstanceID: "iidD", SourceInstanceID: "iidX"}
	sharedSrcY := InstanceObservation{InstanceID: "iidE", SourceInstanceID: "iidX"}
	enabled := InstanceObservation{InstanceID: "iidF", EnablerInstanceID: "iidA"}
	indep1 := InstanceObservation{InstanceID: "iidG"}
	indep2 := InstanceObservation{InstanceID: "iidH"}

	type tc struct {
		name string
		a, b InstanceObservation
		want string
	}
	cases := []tc{
		{"a_is_copy_of_b", copyOfA, og, "a_is_copy_of_b"},
		{"b_is_copy_of_a", og, copyOfA, "b_is_copy_of_a"},
		{"shared_source", sharedSrcX, sharedSrcY, "shared_source"},
		// shared_source must not false-positive when both SourceInstanceID
		// values are empty (the most common case in OG-vs-OG edges).
		{"both_empty_source_is_independent", indep1, indep2, ""},
		{"a_enables_b", og, enabled, "a_enables_b"},
		{"b_enables_a", enabled, og, "b_enables_a"},
		{"independent", indep1, otherCopyOfA, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DeriveLineage(c.a, c.b); got != c.want {
				t.Errorf("DeriveLineage = %q, want %q", got, c.want)
			}
		})
	}
}

func TestResetAndLazyRebuild(t *testing.T) {
	dir := withTempDir(t)
	obs := []InstanceObservation{
		{InstanceID: "iid1", OracleID: "orcA", Provenance: ProvOG},
	}
	if err := AppendInstanceObservations(dir, obs); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := BuildReverseIndex(dir); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := LookupReverseIndex("orcA"); len(got) != 1 {
		t.Fatalf("pre-reset ReverseIndex = %v, want one entry", got)
	}

	ResetReverseIndex()
	// After reset, the lazy path would try defaultHuginnDir. To verify
	// reset cleared in-memory state without doing real-disk I/O, re-
	// build explicitly from the same temp dir and confirm round-trip.
	if err := BuildReverseIndex(dir); err != nil {
		t.Fatalf("rebuild after reset: %v", err)
	}
	if got := LookupReverseIndex("orcA"); len(got) != 1 || got[0] != "iid1" {
		t.Fatalf("post-rebuild ReverseIndex = %v, want [iid1]", got)
	}
	if got := ProvenanceOf("iid1"); got != ProvOG {
		t.Fatalf("post-rebuild ProvenanceOf = %v, want ProvOG", got)
	}
}

func TestConcurrentReads_NoDataRace(t *testing.T) {
	dir := withTempDir(t)
	obs := []InstanceObservation{
		{InstanceID: "iid1", OracleID: "orcA", Provenance: ProvOG},
		{InstanceID: "iid2", OracleID: "orcA", Provenance: ProvCP, SourceInstanceID: "iid1"},
		{InstanceID: "iid3", OracleID: "orcB", Provenance: ProvTK, EnablerInstanceID: "iid1"},
	}
	if err := AppendInstanceObservations(dir, obs); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := BuildReverseIndex(dir); err != nil {
		t.Fatalf("build: %v", err)
	}

	const goroutines = 100
	const iters = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				_ = LookupReverseIndex("orcA")
				_ = LookupReverseIndex("orcB")
				_ = ProvenanceOf("iid1")
				_ = ProvenanceOf("iid2")
				_ = ProvenanceOf("missing")
			}
		}()
	}
	wg.Wait()
}

func TestJSONLStreaming_OneThousandObservations(t *testing.T) {
	dir := withTempDir(t)
	const n = 1000
	obs := make([]InstanceObservation, n)
	for i := 0; i < n; i++ {
		obs[i] = InstanceObservation{
			InstanceID: instanceid.FormatID(i%4, ProvOG, instanceid.Visible, "C", 0, i),
			OracleID:   "orc-bulk",
			CardName:   "Bulk",
			Provenance: ProvOG,
			SeatIdx:    i % 4,
			GameID:     "g-bulk",
			Turn:       i,
		}
	}
	if err := AppendInstanceObservations(dir, obs); err != nil {
		t.Fatalf("append 1000: %v", err)
	}
	read, err := ReadInstanceObservations(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(read) != n {
		t.Fatalf("read back %d, want %d (catches array-mode unmarshal bug)", len(read), n)
	}
	if err := BuildReverseIndex(dir); err != nil {
		t.Fatalf("build: %v", err)
	}
	got := LookupReverseIndex("orc-bulk")
	if len(got) != n {
		t.Fatalf("ReverseIndex returned %d unique IIDs, want %d (one per row, all distinct)", len(got), n)
	}
}

func TestInstanceInteractionEdge_RoundTrip(t *testing.T) {
	dir := withTempDir(t)
	edges := []InstanceInteractionEdge{
		{InstanceA: "iA1", OracleA: "oA", ProvenanceA: ProvOG, InstanceB: "iB1", OracleB: "oB", ProvenanceB: ProvOG, Pattern: "p→c", ImpactScore: 5.5, TurnWindow: 3, GameID: "g1", Lineage: ""},
		{InstanceA: "iA1", OracleA: "oA", ProvenanceA: ProvOG, InstanceB: "iA2", OracleB: "oA", ProvenanceB: ProvCP, Pattern: "self-copy", ImpactScore: 2.0, TurnWindow: 4, GameID: "g1", Lineage: "b_is_copy_of_a"},
		{InstanceA: "iA3", OracleA: "oA", ProvenanceA: ProvCP, InstanceB: "iA4", OracleB: "oA", ProvenanceB: ProvCP, Pattern: "sibling-copies", ImpactScore: 1.0, TurnWindow: 5, GameID: "g1", Lineage: "shared_source"},
		{InstanceA: "iX", OracleA: "oX", ProvenanceA: ProvOG, InstanceB: "iY", OracleB: "oY", ProvenanceB: ProvTK, Pattern: "enables", ImpactScore: 3.0, TurnWindow: 7, GameID: "g2", Lineage: "a_enables_b"},
		{InstanceA: "iZ1", OracleA: "oZ", ProvenanceA: ProvAB, InstanceB: "iZ2", OracleB: "oZ", ProvenanceB: ProvAB, Pattern: "ability-pair", ImpactScore: 0.5, TurnWindow: 8, GameID: "g2", Lineage: ""},
	}
	if err := AppendInstanceInteractionEdges(dir, edges); err != nil {
		t.Fatalf("append edges: %v", err)
	}
	read, err := ReadInstanceInteractionEdges(dir)
	if err != nil {
		t.Fatalf("read edges: %v", err)
	}
	if !reflect.DeepEqual(read, edges) {
		t.Fatalf("round-trip mismatch:\ngot:  %+v\nwant: %+v", read, edges)
	}
}

func TestReadMissingFile_EmptySlice(t *testing.T) {
	dir := withTempDir(t)
	gotObs, err := ReadInstanceObservations(dir)
	if err != nil {
		t.Fatalf("ReadInstanceObservations on missing: %v", err)
	}
	if len(gotObs) != 0 {
		t.Fatalf("expected empty slice, got %v", gotObs)
	}
	gotEdges, err := ReadInstanceInteractionEdges(dir)
	if err != nil {
		t.Fatalf("ReadInstanceInteractionEdges on missing: %v", err)
	}
	if len(gotEdges) != 0 {
		t.Fatalf("expected empty slice, got %v", gotEdges)
	}
}

func TestObservationsFromGameAnalyses_LegacyTolerance(t *testing.T) {
	// Until analytics.GameAnalysis carries InstanceID-grained data, the
	// bridge returns an empty slice. The contract is "callers may invoke
	// freely; an empty result means nothing to persist this batch."
	got := ObservationsFromGameAnalyses(nil, nil)
	if len(got) != 0 {
		t.Fatalf("nil input should yield empty result, got %v", got)
	}
}
