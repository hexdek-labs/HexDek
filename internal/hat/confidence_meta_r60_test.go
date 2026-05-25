package hat

// R60 round 5 — meta-confidence layer on top of the perceived-archetype
// system (#307 archetype-bias curve + #324 contradiction decay).
//
// The Confidence field answers "how sure am I this opponent is X?".
// MetaConfidence answers "how trustworthy IS the answer to that?" —
// a thin-sample classification, a volatile newly-flipped read, or a
// classification that has accumulated contradictions all get lower
// meta even if their raw Confidence sits in the moderate-high range.
// The downstream targeting / removal-selection sites consume
// `effectiveArchetypeBias` (= archetypeBiasMultiplier(Confidence) *
// MetaConfidence) so borderline reads dampen automatically without
// the hat needing to know each call site's bias arithmetic.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// computeMetaConfidence — composition
// ---------------------------------------------------------------------

func TestComputeMetaConfidence_NilProfileIsZero(t *testing.T) {
	if got := computeMetaConfidence(nil, 10); got != 0 {
		t.Errorf("nil profile must yield 0; got %.3f", got)
	}
}

func TestComputeMetaConfidence_UnknownArchetypeIsZero(t *testing.T) {
	prof := &OpponentProfile{Archetype: "unknown", stableTurns: 10}
	if got := computeMetaConfidence(prof, 100); got != 0 {
		t.Errorf("unknown archetype must yield 0; got %.3f", got)
	}
	prof.Archetype = ""
	if got := computeMetaConfidence(prof, 100); got != 0 {
		t.Errorf("empty archetype must yield 0; got %.3f", got)
	}
}

func TestComputeMetaConfidence_ThinSampleDampens(t *testing.T) {
	// Same prof, observation count varies.
	prof := &OpponentProfile{Archetype: "combo", stableTurns: 3}
	thin := computeMetaConfidence(prof, 1)
	full := computeMetaConfidence(prof, metaSampleFullAt)

	if thin >= full {
		t.Errorf("thin sample (1 obs) must dampen meta below full (%d obs); thin=%.3f full=%.3f",
			metaSampleFullAt, thin, full)
	}
}

func TestComputeMetaConfidence_VolatileReadDampens(t *testing.T) {
	// Same prof, stableTurns varies.
	fresh := &OpponentProfile{Archetype: "combo", stableTurns: 0}
	stable := &OpponentProfile{Archetype: "combo", stableTurns: metaStabilityFullAt}

	a := computeMetaConfidence(fresh, 10)
	b := computeMetaConfidence(stable, 10)

	if a >= b {
		t.Errorf("fresh read (0 stableTurns) must dampen meta below stable (%d stableTurns); fresh=%.3f stable=%.3f",
			metaStabilityFullAt, a, b)
	}
}

func TestComputeMetaConfidence_ContradictionsDampen(t *testing.T) {
	clean := &OpponentProfile{Archetype: "combo", stableTurns: 5}
	dirty := &OpponentProfile{Archetype: "combo", stableTurns: 5, Contradictions: 2}

	a := computeMetaConfidence(clean, 10)
	b := computeMetaConfidence(dirty, 10)

	if b >= a {
		t.Errorf("contradicting reads must dampen meta below clean; clean=%.3f dirty=%.3f",
			a, b)
	}
}

func TestComputeMetaConfidence_CappedAtOne(t *testing.T) {
	prof := &OpponentProfile{Archetype: "combo", stableTurns: 100}
	got := computeMetaConfidence(prof, 1000)
	if got > 1.0 {
		t.Errorf("meta-confidence must cap at 1.0; got %.3f", got)
	}
}

func TestComputeMetaConfidence_AtSaturationIsOne(t *testing.T) {
	// All three factors at their natural ceilings → 1.0 product.
	prof := &OpponentProfile{
		Archetype:      "combo",
		stableTurns:    metaStabilityFullAt,
		Contradictions: 0,
	}
	got := computeMetaConfidence(prof, metaSampleFullAt)
	if got < 0.999 {
		t.Errorf("at saturation (full sample + full stability + 0 contradictions) meta should be 1.0; got %.3f", got)
	}
}

func TestComputeMetaConfidence_ContradictionFloor(t *testing.T) {
	// Many contradictions cap dampening at metaContradictionFloor —
	// the meta-confidence drops but doesn't zero.
	prof := &OpponentProfile{
		Archetype:      "combo",
		stableTurns:    metaStabilityFullAt,
		Contradictions: 100,
	}
	got := computeMetaConfidence(prof, metaSampleFullAt)
	if got < metaContradictionFloor-0.01 {
		t.Errorf("contradiction dampening must floor at %.2f; got %.3f", metaContradictionFloor, got)
	}
}

// ---------------------------------------------------------------------
// effectiveArchetypeBias composition
// ---------------------------------------------------------------------

func TestEffectiveArchetypeBias_NilProfileIsZero(t *testing.T) {
	if got := effectiveArchetypeBias(nil); got != 0 {
		t.Errorf("nil profile must yield 0; got %.3f", got)
	}
}

func TestEffectiveArchetypeBias_AtSaturationEqualsRawMultiplier(t *testing.T) {
	prof := &OpponentProfile{
		Archetype:      "combo",
		Confidence:     0.85,
		MetaConfidence: 1.0,
	}
	raw := archetypeBiasMultiplier(prof.Confidence)
	eff := effectiveArchetypeBias(prof)
	if raw != eff {
		t.Errorf("at MetaConfidence=1.0 effective must equal raw multiplier; raw=%.3f eff=%.3f", raw, eff)
	}
}

func TestEffectiveArchetypeBias_DampensWhenMetaLow(t *testing.T) {
	prof := &OpponentProfile{
		Archetype:      "combo",
		Confidence:     0.85,
		MetaConfidence: 0.5,
	}
	raw := archetypeBiasMultiplier(prof.Confidence)
	eff := effectiveArchetypeBias(prof)
	if eff >= raw {
		t.Errorf("low meta must dampen effective below raw; raw=%.3f eff=%.3f", raw, eff)
	}
	want := raw * 0.5
	if eff < want-0.001 || eff > want+0.001 {
		t.Errorf("effective should be raw * meta = %.3f; got %.3f", want, eff)
	}
}

func TestEffectiveArchetypeBias_BelowConfidenceFloorStillZero(t *testing.T) {
	prof := &OpponentProfile{
		Archetype:      "combo",
		Confidence:     0.30, // below floor
		MetaConfidence: 1.0,
	}
	if got := effectiveArchetypeBias(prof); got != 0 {
		t.Errorf("below-floor confidence must zero out regardless of meta; got %.3f", got)
	}
}

// ---------------------------------------------------------------------
// Integration — classifyOpponent populates MetaConfidence
// ---------------------------------------------------------------------

func TestClassifyOpponent_PopulatesMetaConfidence(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	// Seed a profile that will hit the combo branch.
	prof := &OpponentProfile{
		Archetype:    "combo",
		TutorsUsed:   1,
		SpellsPlayed: 3,
		LandsPlayed:  2,
		stableTurns:  2,
	}
	h.opponentProfiles[1] = prof
	// Held mana ≥ 2 needed for the combo classifier branch.
	for i := 0; i < 3; i++ {
		island := newTestCardMinimal("Island", []string{"land"}, 0, nil)
		newTestPermanent(gs.Seats[1], island, 0, 0)
	}
	h.opponentHeldMana[1] = 3

	gs.Turn = 3
	got := h.classifyOpponent(gs, 1)

	if got.MetaConfidence <= 0 {
		t.Errorf("classifyOpponent must populate MetaConfidence; got %.3f", got.MetaConfidence)
	}
	if got.MetaConfidence > 1.0 {
		t.Errorf("MetaConfidence must be capped at 1.0; got %.3f", got.MetaConfidence)
	}
}

// End-to-end: a borderline classification (fresh, thin sample) should
// produce a noticeably lower effectiveArchetypeBias than a saturated
// one with the same raw Confidence.
func TestEffectiveBias_BorderlineDampensVsSaturated(t *testing.T) {
	borderline := &OpponentProfile{
		Archetype:   "combo",
		Confidence:  0.65,
		stableTurns: 0,
	}
	borderline.MetaConfidence = computeMetaConfidence(borderline, 2)

	saturated := &OpponentProfile{
		Archetype:   "combo",
		Confidence:  0.65,
		stableTurns: metaStabilityFullAt,
	}
	saturated.MetaConfidence = computeMetaConfidence(saturated, metaSampleFullAt)

	bBias := effectiveArchetypeBias(borderline)
	sBias := effectiveArchetypeBias(saturated)

	if bBias >= sBias {
		t.Errorf("borderline classification must dampen effective bias below saturated; borderline=%.3f saturated=%.3f",
			bBias, sBias)
	}
	if bBias/sBias > 0.7 {
		t.Errorf("borderline should dampen meaningfully (< 70%% of saturated); ratio=%.2f", bBias/sBias)
	}
}

// The headline case the directive flags: hat has a moderate-high
// Confidence (0.70) on a borderline call. Meta-confidence dampens the
// effective bias so the hat doesn't lean hard on a shaky read.
func TestEffectiveBias_ModerateConfBorderlineSampleDampensVisibly(t *testing.T) {
	thin := &OpponentProfile{
		Archetype:   "combo",
		Confidence:  0.70,
		stableTurns: 1,
	}
	thin.MetaConfidence = computeMetaConfidence(thin, 1)

	raw := archetypeBiasMultiplier(thin.Confidence) // = 0.70 (linear band)
	eff := effectiveArchetypeBias(thin)

	if eff >= raw*0.8 {
		t.Errorf("thin-sample borderline meta should pull effective bias to < 80%% of raw; raw=%.3f eff=%.3f ratio=%.2f",
			raw, eff, eff/raw)
	}
}

// Contradictions interact with meta as well as Confidence — a
// classification with 3 contradictions has both lower Confidence
// (from #324's per-contradiction decay) AND lower MetaConfidence,
// compounding the damping on the effective bias.
func TestEffectiveBias_ContradictionsCompoundDampening(t *testing.T) {
	clean := &OpponentProfile{
		Archetype:   "combo",
		Confidence:  0.85,
		stableTurns: metaStabilityFullAt,
	}
	clean.MetaConfidence = computeMetaConfidence(clean, metaSampleFullAt)

	dirty := &OpponentProfile{
		Archetype:      "combo",
		Confidence:     0.85, // raw confidence pinned equal — only meta differs
		stableTurns:    metaStabilityFullAt,
		Contradictions: 2,
	}
	dirty.MetaConfidence = computeMetaConfidence(dirty, metaSampleFullAt)

	cBias := effectiveArchetypeBias(clean)
	dBias := effectiveArchetypeBias(dirty)

	if dBias >= cBias {
		t.Errorf("with same raw Confidence, contradictions must dampen effective via meta; clean=%.3f dirty=%.3f",
			cBias, dBias)
	}
}
