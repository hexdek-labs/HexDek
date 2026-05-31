package hat

import (
	"testing"
)

// TestNormalizeStrategyProfile_NilSafe pins the defensive nil-handling.
// Called on a nil sp must not panic. The two LoadStrategyFromFreya
// paths can both reach the normalize call with sp populated, but
// future callers might pass nil.
func TestNormalizeStrategyProfile_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil sp should not panic, got: %v", r)
		}
	}()
	normalizeStrategyProfile(nil)
}

// TestNormalizeStrategyProfile_BracketClamp pins the [0, 5] bracket
// range. Negative → 0 (unset sentinel); >5 → 5. The 0 floor is
// deliberate: consumers handle 0 as "no bracket info" via archetype
// defaults, so clamping a negative value to 0 is the correct legacy-
// safe behavior rather than clamping to 1.
func TestNormalizeStrategyProfile_BracketClamp(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{-3, 0}, {-1, 0}, {0, 0}, {1, 1}, {3, 3}, {5, 5}, {6, 5}, {99, 5},
	}
	for _, tc := range cases {
		sp := &StrategyProfile{Bracket: tc.in}
		normalizeStrategyProfile(sp)
		if sp.Bracket != tc.want {
			t.Errorf("Bracket %d: want %d, got %d", tc.in, tc.want, sp.Bracket)
		}
	}
}

// TestNormalizeStrategyProfile_PowerTierClamp mirrors the bracket
// clamp for PowerTier. ApplyPowerTierRouting already guards out-of-
// band values at consumption time, but clamping at load time keeps the
// invariant explicit and prevents the value flowing to other potential
// consumers in an invalid state.
func TestNormalizeStrategyProfile_PowerTierClamp(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{-5, 0}, {0, 0}, {1, 1}, {5, 5}, {7, 5},
	}
	for _, tc := range cases {
		sp := &StrategyProfile{PowerTier: tc.in}
		normalizeStrategyProfile(sp)
		if sp.PowerTier != tc.want {
			t.Errorf("PowerTier %d: want %d, got %d", tc.in, tc.want, sp.PowerTier)
		}
	}
}

// TestNormalizeStrategyProfile_RatioFieldsClamp pins [0.0, 1.0] for
// PowerTierConfidence and CommanderSynergy. These feed direct
// multipliers in downstream scoring — an out-of-range value would
// distort scores rather than fall through.
func TestNormalizeStrategyProfile_RatioFieldsClamp(t *testing.T) {
	sp := &StrategyProfile{
		PowerTierConfidence: -0.5,
		CommanderSynergy:    1.5,
	}
	normalizeStrategyProfile(sp)
	if sp.PowerTierConfidence != 0 {
		t.Errorf("PowerTierConfidence: want 0 (clamp from -0.5), got %v", sp.PowerTierConfidence)
	}
	if sp.CommanderSynergy != 1.0 {
		t.Errorf("CommanderSynergy: want 1.0 (clamp from 1.5), got %v", sp.CommanderSynergy)
	}

	// Reverse: above-max for confidence, below-min for synergy.
	sp = &StrategyProfile{
		PowerTierConfidence: 2.0,
		CommanderSynergy:    -0.3,
	}
	normalizeStrategyProfile(sp)
	if sp.PowerTierConfidence != 1.0 {
		t.Errorf("PowerTierConfidence: want 1.0 (clamp from 2.0), got %v", sp.PowerTierConfidence)
	}
	if sp.CommanderSynergy != 0 {
		t.Errorf("CommanderSynergy: want 0 (clamp from -0.3), got %v", sp.CommanderSynergy)
	}
}

// TestNormalizeStrategyProfile_NonNegativeCounts pins the >=0 floor
// for count fields. Negative counts would break ProtectionRatio
// (denominator becomes negative or zero) and the cheapInteractionPassAdjust
// switch (the == 0 arm wouldn't fire when the value is -1, leaving the
// deck silently misclassified).
func TestNormalizeStrategyProfile_NonNegativeCounts(t *testing.T) {
	sp := &StrategyProfile{
		InteractionAvgCMC:    -1.5,
		CheapInteraction:     -3,
		ProtectedKeyPieces:   -1,
		UnprotectedKeyPieces: -5,
	}
	normalizeStrategyProfile(sp)
	if sp.InteractionAvgCMC != 0 {
		t.Errorf("InteractionAvgCMC: want 0, got %v", sp.InteractionAvgCMC)
	}
	if sp.CheapInteraction != 0 {
		t.Errorf("CheapInteraction: want 0, got %d", sp.CheapInteraction)
	}
	if sp.ProtectedKeyPieces != 0 {
		t.Errorf("ProtectedKeyPieces: want 0, got %d", sp.ProtectedKeyPieces)
	}
	if sp.UnprotectedKeyPieces != 0 {
		t.Errorf("UnprotectedKeyPieces: want 0, got %d", sp.UnprotectedKeyPieces)
	}
}

// TestNormalizeStrategyProfile_PercentFieldsClamp pins [0, 100]
// for KeepableHandPct and PowerPercentile. Mulligan logic gates on
// >0 && <60, so a 150% KeepableHandPct would silently bypass the
// pickier-with-marginal-hands branch and let bad hands through.
func TestNormalizeStrategyProfile_PercentFieldsClamp(t *testing.T) {
	sp := &StrategyProfile{
		KeepableHandPct: 150,
		PowerPercentile: 200,
	}
	normalizeStrategyProfile(sp)
	if sp.KeepableHandPct != 100 {
		t.Errorf("KeepableHandPct: want 100, got %v", sp.KeepableHandPct)
	}
	if sp.PowerPercentile != 100 {
		t.Errorf("PowerPercentile: want 100, got %d", sp.PowerPercentile)
	}

	sp = &StrategyProfile{
		KeepableHandPct: -25,
		PowerPercentile: -10,
	}
	normalizeStrategyProfile(sp)
	if sp.KeepableHandPct != 0 {
		t.Errorf("KeepableHandPct: want 0, got %v", sp.KeepableHandPct)
	}
	if sp.PowerPercentile != 0 {
		t.Errorf("PowerPercentile: want 0, got %d", sp.PowerPercentile)
	}
}

// TestNormalizeStrategyProfile_PreservesValidValues guards against
// the clamp accidentally rewriting legitimate values. Every
// in-range value must pass through untouched.
func TestNormalizeStrategyProfile_PreservesValidValues(t *testing.T) {
	sp := &StrategyProfile{
		Bracket:              3,
		PowerTier:            4,
		PowerTierConfidence:  0.75,
		CommanderSynergy:     0.42,
		InteractionAvgCMC:    2.3,
		CheapInteraction:     6,
		ProtectedKeyPieces:   5,
		UnprotectedKeyPieces: 3,
		KeepableHandPct:      67.5,
		PowerPercentile:      78,
	}
	normalizeStrategyProfile(sp)
	// Field-by-field check (StrategyProfile contains slices so can't
	// use == on the whole struct).
	if sp.Bracket != 3 {
		t.Errorf("Bracket mutated: want 3, got %d", sp.Bracket)
	}
	if sp.PowerTier != 4 {
		t.Errorf("PowerTier mutated: want 4, got %d", sp.PowerTier)
	}
	if sp.PowerTierConfidence != 0.75 {
		t.Errorf("PowerTierConfidence mutated: want 0.75, got %v", sp.PowerTierConfidence)
	}
	if sp.CommanderSynergy != 0.42 {
		t.Errorf("CommanderSynergy mutated: want 0.42, got %v", sp.CommanderSynergy)
	}
	if sp.InteractionAvgCMC != 2.3 {
		t.Errorf("InteractionAvgCMC mutated: want 2.3, got %v", sp.InteractionAvgCMC)
	}
	if sp.CheapInteraction != 6 {
		t.Errorf("CheapInteraction mutated: want 6, got %d", sp.CheapInteraction)
	}
	if sp.ProtectedKeyPieces != 5 {
		t.Errorf("ProtectedKeyPieces mutated: want 5, got %d", sp.ProtectedKeyPieces)
	}
	if sp.UnprotectedKeyPieces != 3 {
		t.Errorf("UnprotectedKeyPieces mutated: want 3, got %d", sp.UnprotectedKeyPieces)
	}
	if sp.KeepableHandPct != 67.5 {
		t.Errorf("KeepableHandPct mutated: want 67.5, got %v", sp.KeepableHandPct)
	}
	if sp.PowerPercentile != 78 {
		t.Errorf("PowerPercentile mutated: want 78, got %d", sp.PowerPercentile)
	}
}

// TestNormalizeStrategyProfile_Idempotent pins that calling normalize
// twice produces the same result as calling once. Each operation is a
// clamp, so re-clamping a clamped value is a no-op — but this
// invariant should be explicit so future additions don't accidentally
// add a non-idempotent transform.
func TestNormalizeStrategyProfile_Idempotent(t *testing.T) {
	sp := &StrategyProfile{
		Bracket:             99,
		PowerTier:           -5,
		PowerTierConfidence: 1.5,
		CommanderSynergy:    -0.2,
		CheapInteraction:    -3,
		KeepableHandPct:     250,
		PowerPercentile:     -10,
	}
	normalizeStrategyProfile(sp)
	first := struct {
		B, PT, CI, PP        int
		PTC, CS, IAC, KHP    float64
		PKP, UKP             int
	}{
		sp.Bracket, sp.PowerTier, sp.CheapInteraction, sp.PowerPercentile,
		sp.PowerTierConfidence, sp.CommanderSynergy, sp.InteractionAvgCMC, sp.KeepableHandPct,
		sp.ProtectedKeyPieces, sp.UnprotectedKeyPieces,
	}
	normalizeStrategyProfile(sp)
	second := struct {
		B, PT, CI, PP        int
		PTC, CS, IAC, KHP    float64
		PKP, UKP             int
	}{
		sp.Bracket, sp.PowerTier, sp.CheapInteraction, sp.PowerPercentile,
		sp.PowerTierConfidence, sp.CommanderSynergy, sp.InteractionAvgCMC, sp.KeepableHandPct,
		sp.ProtectedKeyPieces, sp.UnprotectedKeyPieces,
	}
	if first != second {
		t.Errorf("normalize not idempotent: first %+v, second %+v", first, second)
	}
}

// TestNormalizeStrategyProfile_AppliedByBuildPath pins the wiring
// at the loader level — buildFromStrategyJSON must call normalize
// before returning so consumers downstream of LoadStrategyFromFreya
// never see unclamped values. Defends against a future refactor
// that defines normalize but disconnects it from the build path.
func TestNormalizeStrategyProfile_AppliedByBuildPath(t *testing.T) {
	sj := &strategyFileJSON{
		Archetype:           "combo",
		Bracket:             99,                  // out of [0,5]
		PowerTier:           -3,                  // out of [0,5]
		PowerTierConfidence: 1.7,                 // out of [0,1]
		CommanderSynergy:    -0.5,                // out of [0,1]
		CheapInteraction:    -2,                  // <0
		KeepableHandPct:     150,                 // out of [0,100] → 100
		PowerPercentile:     -10,                 // <0 → 0
		ProtectedKeyPieces:  -1,                  // <0 → 0
	}
	sp := buildFromStrategyJSON(sj)
	if sp == nil {
		t.Fatal("expected StrategyProfile")
	}
	if sp.Bracket != 5 {
		t.Errorf("Bracket: want 5 (clamp from 99), got %d", sp.Bracket)
	}
	if sp.PowerTier != 0 {
		t.Errorf("PowerTier: want 0 (clamp from -3), got %d", sp.PowerTier)
	}
	if sp.PowerTierConfidence != 1.0 {
		t.Errorf("PowerTierConfidence: want 1.0 (clamp from 1.7), got %v", sp.PowerTierConfidence)
	}
	if sp.CommanderSynergy != 0 {
		t.Errorf("CommanderSynergy: want 0 (clamp from -0.5), got %v", sp.CommanderSynergy)
	}
	if sp.CheapInteraction != 0 {
		t.Errorf("CheapInteraction: want 0 (clamp from -2), got %d", sp.CheapInteraction)
	}
	if sp.KeepableHandPct != 100 {
		t.Errorf("KeepableHandPct: want 100 (clamp from 150), got %v", sp.KeepableHandPct)
	}
	if sp.PowerPercentile != 0 {
		t.Errorf("PowerPercentile: want 0 (clamp from -10), got %d", sp.PowerPercentile)
	}
	if sp.ProtectedKeyPieces != 0 {
		t.Errorf("ProtectedKeyPieces: want 0 (clamp from -1), got %d", sp.ProtectedKeyPieces)
	}
}

// TestClampInt is a unit test on the helper used by normalize. Pins
// the [lo, hi] inclusive semantics so an off-by-one in the helper
// can't silently drift the invariants the schema doc specifies.
func TestClampInt(t *testing.T) {
	cases := []struct {
		v, lo, hi, want int
	}{
		{-5, 0, 5, 0},   // below lo
		{0, 0, 5, 0},    // at lo (inclusive)
		{3, 0, 5, 3},    // middle
		{5, 0, 5, 5},    // at hi (inclusive)
		{10, 0, 5, 5},   // above hi
		{-1, -3, 3, -1}, // negative range middle
	}
	for _, tc := range cases {
		if got := clampInt(tc.v, tc.lo, tc.hi); got != tc.want {
			t.Errorf("clampInt(%d, %d, %d): want %d, got %d", tc.v, tc.lo, tc.hi, tc.want, got)
		}
	}
}

// TestClampFloat is the float64 sibling of TestClampInt.
func TestClampFloat(t *testing.T) {
	cases := []struct {
		v, lo, hi, want float64
	}{
		{-0.5, 0, 1, 0},
		{0, 0, 1, 0},
		{0.5, 0, 1, 0.5},
		{1.0, 0, 1, 1.0},
		{1.5, 0, 1, 1.0},
		{-1.0, -5, 5, -1.0},
	}
	for _, tc := range cases {
		if got := clampFloat(tc.v, tc.lo, tc.hi); got != tc.want {
			t.Errorf("clampFloat(%v, %v, %v): want %v, got %v", tc.v, tc.lo, tc.hi, tc.want, got)
		}
	}
}
