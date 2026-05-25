package heimdall

import (
	"math"
	"testing"
)

const weekSec = int64(7 * 24 * 3600)

func metaApprox(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func findMetaArch(rows []ArchetypeMetaTrend, slug string) *ArchetypeMetaTrend {
	for i := range rows {
		if rows[i].Archetype == slug {
			return &rows[i]
		}
	}
	return nil
}

// TestComputeMetaArchetypeTrends_Empty pins the no-data shape: well-
// formed envelope with non-nil empty slices.
func TestComputeMetaArchetypeTrends_Empty(t *testing.T) {
	out := ComputeMetaArchetypeTrends(nil, 1_000_000, 4)
	if out.Archetypes == nil || out.BiggestGainers == nil || out.BiggestLosers == nil {
		t.Errorf("slices should be non-nil empty: %+v", out)
	}
	if out.Weeks != 4 || out.TotalGames != 0 {
		t.Errorf("envelope: %+v", out)
	}
}

// TestComputeMetaArchetypeTrends_WeekBucketing pins that games are
// placed into the correct week index based on their FinishedAt
// relative to refUnix.
func TestComputeMetaArchetypeTrends_WeekBucketing(t *testing.T) {
	ref := int64(10 * weekSec) // arbitrary anchor at 10 weeks
	// 4-week window → buckets [ref-4w, ref-3w), [ref-3w, ref-2w),
	// [ref-2w, ref-1w), [ref-1w, ref).
	games := []MetaGameInput{
		// bucket 0 (oldest)
		{FinishedAt: ref - 4*weekSec, Archetype: "control", Won: true},
		// bucket 1
		{FinishedAt: ref - 3*weekSec, Archetype: "control", Won: false},
		// bucket 2
		{FinishedAt: ref - 2*weekSec, Archetype: "control", Won: true},
		// bucket 3 (most recent)
		{FinishedAt: ref - 1*weekSec, Archetype: "control", Won: false},
		// outside window — dropped
		{FinishedAt: ref - 5*weekSec, Archetype: "control", Won: true},
		{FinishedAt: ref + 1, Archetype: "control", Won: false},
	}
	out := ComputeMetaArchetypeTrends(games, ref, 4)
	if out.TotalGames != 4 {
		t.Errorf("TotalGames = %d, want 4 (out-of-window dropped)", out.TotalGames)
	}
	c := findMetaArch(out.Archetypes, "control")
	if c == nil {
		t.Fatalf("missing control row")
	}
	if len(c.Weekly) != 4 {
		t.Fatalf("Weekly len = %d, want 4", len(c.Weekly))
	}
	wants := []struct {
		games, wins int
	}{
		{1, 1}, {1, 0}, {1, 1}, {1, 0},
	}
	for i, w := range wants {
		if c.Weekly[i].Games != w.games || c.Weekly[i].Wins != w.wins {
			t.Errorf("bucket[%d] = %+v, want games=%d wins=%d", i, c.Weekly[i], w.games, w.wins)
		}
	}
}

// TestComputeMetaArchetypeTrends_DeltaSplitsHalfWeeks pins that the
// recent-vs-prior delta uses the second-half weeks vs first-half
// weeks (NOT a game-count split).
func TestComputeMetaArchetypeTrends_DeltaSplitsHalfWeeks(t *testing.T) {
	ref := int64(20 * weekSec)
	// 4 weeks: prior = weeks 0+1, recent = weeks 2+3.
	// Prior: 10w/10l (50%); Recent: 8w/2l (80%). Δ = +0.30 → "up".
	mk := func(weekIdx int, win bool) MetaGameInput {
		ts := ref - int64(4-weekIdx)*weekSec
		return MetaGameInput{FinishedAt: ts, Archetype: "combo", Won: win}
	}
	var games []MetaGameInput
	for i := 0; i < 10; i++ { // prior wins
		games = append(games, mk(0, true))
	}
	for i := 0; i < 10; i++ { // prior losses
		games = append(games, mk(1, false))
	}
	for i := 0; i < 8; i++ { // recent wins
		games = append(games, mk(2, true))
	}
	for i := 0; i < 2; i++ { // recent losses
		games = append(games, mk(3, false))
	}
	out := ComputeMetaArchetypeTrends(games, ref, 4)
	c := findMetaArch(out.Archetypes, "combo")
	if c == nil {
		t.Fatal("missing combo")
	}
	if !metaApprox(c.Delta, 0.3) {
		t.Errorf("Delta = %.3f, want 0.3 (recent 0.8 - prior 0.5)", c.Delta)
	}
	if c.Direction != "up" {
		t.Errorf("Direction = %q, want 'up'", c.Direction)
	}
}

// TestComputeMetaArchetypeTrends_InsufficientWhenBelowMinSamples pins
// that the per-side sample-size floor produces "insufficient_data"
// even if the delta is large.
func TestComputeMetaArchetypeTrends_InsufficientWhenBelowMinSamples(t *testing.T) {
	ref := int64(20 * weekSec)
	// Only 4 games total (under the 10/side floor). Skew them to
	// 100% recent win vs 0% prior — direction should still mark
	// insufficient_data.
	games := []MetaGameInput{
		{FinishedAt: ref - 3*weekSec, Archetype: "stax", Won: false},
		{FinishedAt: ref - 3*weekSec, Archetype: "stax", Won: false},
		{FinishedAt: ref - 1*weekSec, Archetype: "stax", Won: true},
		{FinishedAt: ref - 1*weekSec, Archetype: "stax", Won: true},
	}
	out := ComputeMetaArchetypeTrends(games, ref, 4)
	s := findMetaArch(out.Archetypes, "stax")
	if s == nil {
		t.Fatal("missing stax")
	}
	if s.Direction != "insufficient_data" {
		t.Errorf("Direction = %q, want 'insufficient_data' (only %d+%d games)",
			s.Direction, 2, 2)
	}
}

// TestComputeMetaArchetypeTrends_FlatWhenSwingBelowThreshold pins
// the threshold-band classification: under metaTrendsDeltaThreshold
// becomes "flat" when sample sizes are adequate.
func TestComputeMetaArchetypeTrends_FlatWhenSwingBelowThreshold(t *testing.T) {
	ref := int64(20 * weekSec)
	// Prior 10w/10l (50%), Recent 11w/9l (55%). Δ = +0.05 — above
	// threshold. Tweak to 10w/10l prior, 10w/10l recent (50%/50%) →
	// Δ=0 → flat.
	mk := func(weekIdx int, win bool) MetaGameInput {
		ts := ref - int64(4-weekIdx)*weekSec
		return MetaGameInput{FinishedAt: ts, Archetype: "midrange", Won: win}
	}
	var games []MetaGameInput
	for i := 0; i < 10; i++ {
		games = append(games, mk(0, true))
	}
	for i := 0; i < 10; i++ {
		games = append(games, mk(1, false))
	}
	for i := 0; i < 10; i++ {
		games = append(games, mk(2, true))
	}
	for i := 0; i < 10; i++ {
		games = append(games, mk(3, false))
	}
	out := ComputeMetaArchetypeTrends(games, ref, 4)
	m := findMetaArch(out.Archetypes, "midrange")
	if m == nil {
		t.Fatal("missing midrange")
	}
	if !metaApprox(m.Delta, 0) {
		t.Errorf("Delta = %.3f, want 0", m.Delta)
	}
	if m.Direction != "flat" {
		t.Errorf("Direction = %q, want 'flat'", m.Direction)
	}
}

// TestComputeMetaArchetypeTrends_BiggestGainersAndLosersPartition
// pins that gainers and losers come from the meaningful-direction
// rows only, sorted by Δ desc/asc, and capped at metaTrendsBiggestN.
func TestComputeMetaArchetypeTrends_BiggestGainersAndLosersPartition(t *testing.T) {
	ref := int64(20 * weekSec)
	// Build 7 archetypes with varying deltas. All have ≥10 games per
	// side so none are insufficient.
	build := func(slug string, priorWR, recentWR float64) []MetaGameInput {
		var out []MetaGameInput
		// Prior bucket (week 0): 10 games at priorWR
		priorW := int(priorWR * 10)
		for i := 0; i < 10; i++ {
			out = append(out, MetaGameInput{
				FinishedAt: ref - 3*weekSec,
				Archetype:  slug,
				Won:        i < priorW,
			})
		}
		// Recent bucket (week 2): 10 games at recentWR
		recentW := int(recentWR * 10)
		for i := 0; i < 10; i++ {
			out = append(out, MetaGameInput{
				FinishedAt: ref - 1*weekSec,
				Archetype:  slug,
				Won:        i < recentW,
			})
		}
		return out
	}

	var games []MetaGameInput
	games = append(games, build("aaa_huge_gain", 0.2, 0.9)...)     // +0.7
	games = append(games, build("bbb_med_gain", 0.4, 0.6)...)      // +0.2
	games = append(games, build("ccc_tiny_gain", 0.5, 0.6)...)     // +0.1
	games = append(games, build("ddd_flat", 0.5, 0.5)...)          // 0
	games = append(games, build("eee_tiny_loss", 0.5, 0.4)...)     // -0.1
	games = append(games, build("fff_med_loss", 0.6, 0.3)...)      // -0.3
	games = append(games, build("ggg_huge_loss", 0.9, 0.2)...)     // -0.7

	out := ComputeMetaArchetypeTrends(games, ref, 4)

	if len(out.BiggestGainers) == 0 || out.BiggestGainers[0].Archetype != "aaa_huge_gain" {
		t.Errorf("BiggestGainers[0] = %+v, want aaa_huge_gain first", out.BiggestGainers)
	}
	if len(out.BiggestLosers) == 0 || out.BiggestLosers[0].Archetype != "ggg_huge_loss" {
		t.Errorf("BiggestLosers[0] = %+v, want ggg_huge_loss first", out.BiggestLosers)
	}
	// "flat" (delta 0) should NOT appear in either list (the filter
	// drops delta==0 from gainers, delta==0 from losers).
	for _, g := range out.BiggestGainers {
		if g.Archetype == "ddd_flat" {
			t.Errorf("flat archetype shouldn't be in gainers: %+v", g)
		}
	}
	for _, l := range out.BiggestLosers {
		if l.Archetype == "ddd_flat" {
			t.Errorf("flat archetype shouldn't be in losers: %+v", l)
		}
	}
	// Cap at metaTrendsBiggestN (5 each).
	if len(out.BiggestGainers) > metaTrendsBiggestN {
		t.Errorf("gainers len %d > cap %d", len(out.BiggestGainers), metaTrendsBiggestN)
	}
	if len(out.BiggestLosers) > metaTrendsBiggestN {
		t.Errorf("losers len %d > cap %d", len(out.BiggestLosers), metaTrendsBiggestN)
	}
}

// TestComputeMetaArchetypeTrends_InsufficientExcludedFromMovers pins
// that an archetype marked insufficient_data doesn't pollute the
// gainers/losers lists even if its Δ is large.
func TestComputeMetaArchetypeTrends_InsufficientExcludedFromMovers(t *testing.T) {
	ref := int64(20 * weekSec)
	// Build one strong-but-small archetype (insufficient) alongside
	// one weak gainer (sufficient).
	var games []MetaGameInput
	// strong-but-small "spike": 2 prior losses, 2 recent wins → Δ=+1.0
	for i := 0; i < 2; i++ {
		games = append(games, MetaGameInput{FinishedAt: ref - 3*weekSec, Archetype: "spike", Won: false})
	}
	for i := 0; i < 2; i++ {
		games = append(games, MetaGameInput{FinishedAt: ref - 1*weekSec, Archetype: "spike", Won: true})
	}
	// sufficient mild gainer: 10 prior (3w), 10 recent (5w) → Δ=+0.20
	for i := 0; i < 10; i++ {
		games = append(games, MetaGameInput{FinishedAt: ref - 3*weekSec, Archetype: "real", Won: i < 3})
	}
	for i := 0; i < 10; i++ {
		games = append(games, MetaGameInput{FinishedAt: ref - 1*weekSec, Archetype: "real", Won: i < 5})
	}
	out := ComputeMetaArchetypeTrends(games, ref, 4)
	for _, g := range out.BiggestGainers {
		if g.Archetype == "spike" {
			t.Errorf("insufficient_data archetype leaked into gainers: %+v", g)
		}
	}
	// "real" should be the only gainer.
	if len(out.BiggestGainers) != 1 || out.BiggestGainers[0].Archetype != "real" {
		t.Errorf("BiggestGainers = %+v, want only 'real'", out.BiggestGainers)
	}
}

// TestComputeMetaArchetypeTrends_ArchetypeSortByGamesDesc pins the
// Archetypes list ordering (most-played first, alpha tiebreak).
func TestComputeMetaArchetypeTrends_ArchetypeSortByGamesDesc(t *testing.T) {
	ref := int64(20 * weekSec)
	mk := func(slug string, n int) []MetaGameInput {
		var out []MetaGameInput
		for i := 0; i < n; i++ {
			out = append(out, MetaGameInput{
				FinishedAt: ref - 2*weekSec,
				Archetype:  slug,
				Won:        false,
			})
		}
		return out
	}
	var games []MetaGameInput
	games = append(games, mk("alpha", 5)...)
	games = append(games, mk("beta", 20)...)
	games = append(games, mk("gamma", 20)...)  // ties beta — alpha sort by name → beta first
	games = append(games, mk("delta", 1)...)
	out := ComputeMetaArchetypeTrends(games, ref, 4)
	if len(out.Archetypes) != 4 {
		t.Fatalf("got %d, want 4", len(out.Archetypes))
	}
	wantOrder := []string{"beta", "gamma", "alpha", "delta"}
	for i, want := range wantOrder {
		if out.Archetypes[i].Archetype != want {
			t.Errorf("Archetypes[%d] = %s, want %s", i, out.Archetypes[i].Archetype, want)
		}
	}
}

// TestComputeMetaArchetypeTrends_EmptyArchetypeBucketedAsUnknown pins
// that games with Archetype="" land in the "unknown" bucket.
func TestComputeMetaArchetypeTrends_EmptyArchetypeBucketedAsUnknown(t *testing.T) {
	ref := int64(20 * weekSec)
	games := []MetaGameInput{
		{FinishedAt: ref - 2*weekSec, Archetype: "", Won: true},
		{FinishedAt: ref - 2*weekSec, Archetype: "", Won: false},
	}
	out := ComputeMetaArchetypeTrends(games, ref, 4)
	u := findMetaArch(out.Archetypes, "unknown")
	if u == nil || u.TotalGames != 2 {
		t.Errorf("unknown bucket: %+v", u)
	}
}

// TestComputeMetaArchetypeTrends_WeeksClampedToMax pins that
// weeks > metaTrendsMaxWeeks gets clamped (and weeks <= 0 falls back
// to default).
func TestComputeMetaArchetypeTrends_WeeksClampedToMax(t *testing.T) {
	out := ComputeMetaArchetypeTrends(nil, 1_000_000, 99)
	if out.Weeks != metaTrendsMaxWeeks {
		t.Errorf("Weeks = %d, want clamped to %d", out.Weeks, metaTrendsMaxWeeks)
	}
	out = ComputeMetaArchetypeTrends(nil, 1_000_000, 0)
	if out.Weeks != metaTrendsDefaultWeeks {
		t.Errorf("Weeks = %d, want default %d", out.Weeks, metaTrendsDefaultWeeks)
	}
}
