package heimdall

import (
	"math"
	"testing"
)

func archiveApprox(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func archiveLineage(versions ...DeckArchiveLineageEntry) []DeckArchiveLineageEntry {
	return versions
}

func archiveVersion(version int, createdAt int64, cardCount int, arch string) DeckArchiveLineageEntry {
	return DeckArchiveLineageEntry{
		DeckVersionEntry: DeckVersionEntry{
			Hash:      "h" + intStr(version),
			Version:   version,
			CardCount: cardCount,
			CreatedAt: createdAt,
		},
		Archetype: arch,
	}
}

func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var out []byte
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func findVersionRow(rows []DeckArchiveVersion, v int) *DeckArchiveVersion {
	for i := range rows {
		if rows[i].Version == v {
			return &rows[i]
		}
	}
	return nil
}

// TestBuildDeckArchive_IdentityAndEmptyDefaults pins the no-data case:
// every slice is non-nil empty, identity fields are echoed through.
func TestBuildDeckArchive_IdentityAndEmptyDefaults(t *testing.T) {
	out := BuildDeckArchive(DeckArchiveInput{
		Owner:            "alice",
		DeckID:           "krenko",
		CurrentCommander: "Krenko, Mob Boss",
		CurrentArchetype: "tribal",
	})
	if out.Owner != "alice" || out.DeckID != "krenko" || out.DeckKey != "alice/krenko" {
		t.Errorf("identity: %+v", out)
	}
	if out.CurrentCommander != "Krenko, Mob Boss" || out.CurrentArchetype != "tribal" {
		t.Errorf("current fields: %+v", out)
	}
	if out.Versions == nil || out.Games.Recent == nil || out.ArchetypeEvolution == nil {
		t.Errorf("slices should be non-nil: %+v", out)
	}
	if out.Games.Total != 0 || out.Games.WinRate != 0 {
		t.Errorf("zero-game stats wrong: %+v", out.Games)
	}
}

// TestBuildDeckArchive_AggregateGameStats pins total / wins / losses /
// draws / winrate counts.
func TestBuildDeckArchive_AggregateGameStats(t *testing.T) {
	out := BuildDeckArchive(DeckArchiveInput{
		Owner:  "a", DeckID: "d",
		Games: []DeckVersionOutcome{
			{FinishedAt: 100, Won: true},
			{FinishedAt: 200, Won: true},
			{FinishedAt: 300, Won: false},
			{FinishedAt: 400, Draw: true},
		},
	})
	if out.Games.Total != 4 || out.Games.Wins != 2 || out.Games.Losses != 1 || out.Games.Draws != 1 {
		t.Errorf("counts: %+v", out.Games)
	}
	if !archiveApprox(out.Games.WinRate, 0.5) {
		t.Errorf("WinRate = %.3f, want 0.5", out.Games.WinRate)
	}
}

// TestBuildDeckArchive_RecentGamesNewestFirstLimitedByN pins the
// recent-games ordering (newest first by FinishedAt desc) and the
// RecentLimit cap.
func TestBuildDeckArchive_RecentGamesNewestFirstLimitedByN(t *testing.T) {
	games := []DeckVersionOutcome{}
	for i := int64(1); i <= 30; i++ {
		games = append(games, DeckVersionOutcome{FinishedAt: i * 100, Won: i%2 == 0})
	}
	out := BuildDeckArchive(DeckArchiveInput{
		Owner: "a", DeckID: "d", Games: games, RecentLimit: 5,
	})
	if len(out.Games.Recent) != 5 {
		t.Fatalf("len Recent = %d, want 5", len(out.Games.Recent))
	}
	// newest first: 30, 29, 28, 27, 26
	wantTs := []int64{3000, 2900, 2800, 2700, 2600}
	for i, want := range wantTs {
		if out.Games.Recent[i].FinishedAt != want {
			t.Errorf("Recent[%d].FinishedAt = %d, want %d", i, out.Games.Recent[i].FinishedAt, want)
		}
	}
}

// TestBuildDeckArchive_DefaultRecentLimitIsTwenty pins that
// RecentLimit <= 0 falls back to 20.
func TestBuildDeckArchive_DefaultRecentLimitIsTwenty(t *testing.T) {
	games := make([]DeckVersionOutcome, 35)
	for i := range games {
		games[i] = DeckVersionOutcome{FinishedAt: int64(i + 1), Won: true}
	}
	out := BuildDeckArchive(DeckArchiveInput{Owner: "a", DeckID: "d", Games: games})
	if len(out.Games.Recent) != deckArchiveDefaultRecent {
		t.Errorf("default recent cap = %d, want %d", len(out.Games.Recent), deckArchiveDefaultRecent)
	}
}

// TestBuildDeckArchive_VersionRollupMirrorsVersionTrends pins that
// the per-version rollup numbers are bit-identical to what
// ComputeDeckVersionTrends would produce (we re-use that engine
// internally).
func TestBuildDeckArchive_VersionRollupMirrorsVersionTrends(t *testing.T) {
	lineage := archiveLineage(
		archiveVersion(1, 1000, 100, ""),
		archiveVersion(2, 2000, 99, ""),
	)
	games := []DeckVersionOutcome{
		// v1 era: 1000 ≤ t < 2000 → 2w/1l
		{FinishedAt: 1100, Won: true},
		{FinishedAt: 1500, Won: true},
		{FinishedAt: 1900, Won: false},
		// v2 era: 2000 ≤ t → 1w/2l
		{FinishedAt: 2100, Won: true},
		{FinishedAt: 2500, Won: false},
		{FinishedAt: 2700, Won: false},
	}
	out := BuildDeckArchive(DeckArchiveInput{
		Owner: "a", DeckID: "d", Lineage: lineage, Games: games,
	})
	v1 := findVersionRow(out.Versions, 1)
	v2 := findVersionRow(out.Versions, 2)
	if v1 == nil || v2 == nil {
		t.Fatalf("missing version rows: %+v", out.Versions)
	}
	if v1.Games != 3 || v1.Wins != 2 || !archiveApprox(v1.WinRate, 2.0/3.0) {
		t.Errorf("v1 rollup: %+v", v1)
	}
	if v2.Games != 3 || v2.Wins != 1 || !archiveApprox(v2.WinRate, 1.0/3.0) {
		t.Errorf("v2 rollup: %+v", v2)
	}
}

// TestBuildDeckArchive_LatestVersionArchetypeBackfilledFromCurrent
// pins the fallback: when the lineage entry for the latest version
// has no archetype but CurrentArchetype is set, the version row gets
// the current archetype.
func TestBuildDeckArchive_LatestVersionArchetypeBackfilledFromCurrent(t *testing.T) {
	lineage := archiveLineage(
		archiveVersion(1, 1000, 100, ""),
		archiveVersion(2, 2000, 99, ""), // no historical snapshot
	)
	out := BuildDeckArchive(DeckArchiveInput{
		Owner: "a", DeckID: "d",
		CurrentArchetype: "voltron",
		Lineage:          lineage,
	})
	v1 := findVersionRow(out.Versions, 1)
	v2 := findVersionRow(out.Versions, 2)
	if v1 == nil || v2 == nil {
		t.Fatalf("missing rows")
	}
	if v1.Archetype != "" {
		t.Errorf("v1 archetype should be empty (no fallback for non-latest): %q", v1.Archetype)
	}
	if v2.Archetype != "voltron" {
		t.Errorf("v2 (latest) archetype should backfill to 'voltron': %q", v2.Archetype)
	}
}

// TestBuildDeckArchive_HistoricalArchetypeWinsOverFallback pins that
// when the latest version's lineage entry HAS an archetype, the
// historical value wins (CurrentArchetype is not clobbered).
func TestBuildDeckArchive_HistoricalArchetypeWinsOverFallback(t *testing.T) {
	lineage := archiveLineage(
		archiveVersion(1, 1000, 100, "tribal"),
		archiveVersion(2, 2000, 99, "midrange"),
	)
	out := BuildDeckArchive(DeckArchiveInput{
		Owner: "a", DeckID: "d",
		CurrentArchetype: "voltron",
		Lineage:          lineage,
	})
	v2 := findVersionRow(out.Versions, 2)
	if v2 == nil || v2.Archetype != "midrange" {
		t.Errorf("v2 should keep historical 'midrange', got %q", v2.Archetype)
	}
}

// TestBuildDeckArchive_ArchetypeEvolutionEmitsOnChange pins that the
// timeline collapses consecutive same-archetype versions into one
// step and emits a new step whenever the archetype changes.
func TestBuildDeckArchive_ArchetypeEvolutionEmitsOnChange(t *testing.T) {
	lineage := archiveLineage(
		archiveVersion(1, 1000, 100, "tribal"),
		archiveVersion(2, 2000, 100, "tribal"),    // same → no step
		archiveVersion(3, 3000, 99, "midrange"),   // change → step
		archiveVersion(4, 4000, 99, "midrange"),   // same → no step
		archiveVersion(5, 5000, 100, "combo"),     // change → step
	)
	out := BuildDeckArchive(DeckArchiveInput{
		Owner: "a", DeckID: "d", Lineage: lineage,
	})
	if len(out.ArchetypeEvolution) != 3 {
		t.Fatalf("expected 3 steps, got %d: %+v", len(out.ArchetypeEvolution), out.ArchetypeEvolution)
	}
	want := []DeckArchiveArchetypeStep{
		{SinceVersion: 1, SinceUnix: 1000, Archetype: "tribal"},
		{SinceVersion: 3, SinceUnix: 3000, Archetype: "midrange"},
		{SinceVersion: 5, SinceUnix: 5000, Archetype: "combo"},
	}
	for i, w := range want {
		if out.ArchetypeEvolution[i] != w {
			t.Errorf("step[%d] = %+v, want %+v", i, out.ArchetypeEvolution[i], w)
		}
	}
}

// TestBuildDeckArchive_ArchetypeEvolutionSkipsEmptyArchetypes pins
// that versions without a known archetype don't anchor a transition
// (and don't reset the prev-archetype tracking).
func TestBuildDeckArchive_ArchetypeEvolutionSkipsEmptyArchetypes(t *testing.T) {
	lineage := archiveLineage(
		archiveVersion(1, 1000, 100, "tribal"),
		archiveVersion(2, 2000, 100, ""),       // unknown — skip
		archiveVersion(3, 3000, 100, ""),       // unknown — skip
		archiveVersion(4, 4000, 100, "tribal"), // SAME as last known → no step
	)
	out := BuildDeckArchive(DeckArchiveInput{
		Owner: "a", DeckID: "d", Lineage: lineage,
	})
	if len(out.ArchetypeEvolution) != 1 {
		t.Errorf("expected 1 step (tribal anchored at v1), got %d: %+v",
			len(out.ArchetypeEvolution), out.ArchetypeEvolution)
	}
}

// TestBuildDeckArchive_UntrackedGamesStillContributeToTotals pins
// that games predating any version still increment Total/Wins/etc.
// even though they don't bucket into a version row.
func TestBuildDeckArchive_UntrackedGamesStillContributeToTotals(t *testing.T) {
	lineage := archiveLineage(archiveVersion(1, 2000, 100, "tribal"))
	games := []DeckVersionOutcome{
		{FinishedAt: 100, Won: true}, // before v1 → untracked
		{FinishedAt: 200, Won: false},
		{FinishedAt: 2100, Won: true}, // v1 era
	}
	out := BuildDeckArchive(DeckArchiveInput{Owner: "a", DeckID: "d", Lineage: lineage, Games: games})
	if out.Games.Total != 3 || out.Games.Wins != 2 {
		t.Errorf("totals: %+v, want 3 games / 2 wins (untracked still counted)", out.Games)
	}
	v1 := findVersionRow(out.Versions, 1)
	if v1 == nil || v1.Games != 1 {
		t.Errorf("v1 should only see 1 game (the post-v1 one): %+v", v1)
	}
}
