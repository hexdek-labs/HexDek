package main

import "testing"

// eval_weights_pillowfort_r60_test.go — pins the new Pillowfort eval-
// weight profile. Background: an audit of cmd/hexdek-freya/deckprofile.go's
// `defaultWeights` map against the 30 archetypes the classifier in
// archetypes.go actually emits found 8 archetypes (Selfmill, Pillowfort,
// Group Slug, Group Hug, Cycling, Toxic, Vehicles, Damage Redirect) had
// no eval-weight entry — they fell through to midrange via the
// `if defaults == nil { defaults = defaultWeights["midrange"] }`
// fallback in ComputeEvalWeights. Pillowfort was picked as the
// most-underweighted of the 8 because its eval profile is the most
// semantically opposite to midrange (defensive vs proactive, high
// LifeResource vs low, etc.).

// freyaWeightsBaseline returns a DeckProfile / FreyaReport pair that
// produces zero adjustment on top of the archetype defaults (no tutor,
// no recursion, no value-chain depth, no ramp, no win lines, plenty of
// interaction). Lets each test isolate the archetype-baseline lookup
// without other ComputeEvalWeights paths firing.
func freyaWeightsBaseline(arch string) (*DeckProfile, *FreyaReport) {
	dp := &DeckProfile{
		PrimaryArchetype: arch,
		RampCount:        0,
		WinLineCount:     0,
		HasTutorAccess:   false,
	}
	r := &FreyaReport{
		NonLandTutorCount: 0,
		RemovalCount:      10, // > 5 so ThreatExposure adjustment doesn't fire
		Roles:             &RoleAnalysis{RoleCounts: map[RoleTag]int{}},
	}
	return dp, r
}

// -----------------------------------------------------------------------------
// 1. Profile registration: Pillowfort exists in defaultWeights
// -----------------------------------------------------------------------------

func TestPillowfortWeights_RegisteredInDefaultsTable(t *testing.T) {
	w, ok := defaultWeights["pillowfort"]
	if !ok || w == nil {
		t.Fatal("defaultWeights[\"pillowfort\"] missing — eval-weight fallback to midrange would apply")
	}
	// Sanity that the entry isn't a midrange clone — the whole point of
	// adding it is meaningful divergence.
	mid := defaultWeights["midrange"]
	if *w == *mid {
		t.Error("pillowfort weights identical to midrange — divergence is the entire purpose of this profile")
	}
}

// -----------------------------------------------------------------------------
// 2. ComputeEvalWeights surfaces Pillowfort weights for Pillowfort decks
// -----------------------------------------------------------------------------

func TestComputeEvalWeights_PillowfortDeckGetsTunedProfile(t *testing.T) {
	dp, r := freyaWeightsBaseline("Pillowfort")
	got := ComputeEvalWeights(dp, r)

	want := defaultWeights["pillowfort"]
	if got == nil || want == nil {
		t.Fatal("nil weights returned from ComputeEvalWeights")
	}
	if *got != *want {
		t.Errorf("Pillowfort deck got non-Pillowfort weights:\n  got  = %+v\n  want = %+v", *got, *want)
	}
}

func TestComputeEvalWeights_LowercaseArchetypeAlsoMatches(t *testing.T) {
	// ComputeEvalWeights calls strings.ToLower(dp.PrimaryArchetype), so
	// the all-lowercase form must produce identical weights to the
	// Capitalized form the classifier emits.
	dpUpper, r := freyaWeightsBaseline("Pillowfort")
	dpLower, _ := freyaWeightsBaseline("pillowfort")
	upper := ComputeEvalWeights(dpUpper, r)
	lower := ComputeEvalWeights(dpLower, r)
	if *upper != *lower {
		t.Errorf("Pillowfort vs pillowfort case mismatch:\n  upper = %+v\n  lower = %+v", *upper, *lower)
	}
}

// -----------------------------------------------------------------------------
// 3. Pillowfort weights diverge from midrange in the EXPECTED directions
// -----------------------------------------------------------------------------

func TestPillowfortWeights_DivergeFromMidrangeCorrectly(t *testing.T) {
	pf := defaultWeights["pillowfort"]
	mid := defaultWeights["midrange"]
	if pf == nil || mid == nil {
		t.Fatal("missing weight entry")
	}

	// Defensive deck: lower BoardPresence, higher LifeResource +
	// ThreatExposure than midrange. CommanderProgress lower (passive
	// commander). These are the four signature axes that distinguish
	// pillowfort from midrange — if any flip on a future re-tune, the
	// profile no longer behaves as a pillowfort and this test fires.
	cases := []struct {
		name string
		got  float64
		mid  float64
		want string // "lower" or "higher"
	}{
		{"BoardPresence", pf.BoardPresence, mid.BoardPresence, "lower"},
		{"LifeResource", pf.LifeResource, mid.LifeResource, "higher"},
		{"ThreatExposure", pf.ThreatExposure, mid.ThreatExposure, "higher"},
		{"CommanderProgress", pf.CommanderProgress, mid.CommanderProgress, "lower"},
		{"CardAdvantage", pf.CardAdvantage, mid.CardAdvantage, "higher"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.want == "lower" && !(c.got < c.mid) {
				t.Errorf("%s: got %v, want LOWER than midrange %v (defensive deck shouldn't over-weight this axis)",
					c.name, c.got, c.mid)
			}
			if c.want == "higher" && !(c.got > c.mid) {
				t.Errorf("%s: got %v, want HIGHER than midrange %v (defensive deck should over-weight this axis)",
					c.name, c.got, c.mid)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// 4. Counterfactual: an unregistered classified archetype still falls
//    back to midrange — confirms the fallback path is real (and that
//    the Pillowfort entry actually closes a gap, not just shadows an
//    existing entry).
// -----------------------------------------------------------------------------

func TestComputeEvalWeights_UnregisteredArchetypeStillFallsBackToMidrange(t *testing.T) {
	// "Group Slug" is one of the 7 remaining classified-but-unregistered
	// archetypes in archetypes.go (Selfmill, Group Slug, Group Hug,
	// Cycling, Toxic, Vehicles, Damage Redirect). Until those get
	// dedicated entries, they continue to fall back to midrange — this
	// test pins that fallback so a future regression that breaks the
	// fallback path surfaces immediately, AND documents the remaining
	// gap for the follow-up sweep.
	dpGroupSlug, r := freyaWeightsBaseline("Group Slug")
	gsWeights := ComputeEvalWeights(dpGroupSlug, r)

	dpMidrange, _ := freyaWeightsBaseline("Midrange")
	midWeights := ComputeEvalWeights(dpMidrange, r)

	if *gsWeights != *midWeights {
		t.Errorf("Group Slug should still fall back to midrange (no entry yet):\n  got = %+v\n  mid = %+v",
			*gsWeights, *midWeights)
	}

	// Pillowfort, by contrast, must NOT match midrange anymore — that's
	// the entire deliverable of this branch.
	dpPF, _ := freyaWeightsBaseline("Pillowfort")
	pfWeights := ComputeEvalWeights(dpPF, r)
	if *pfWeights == *midWeights {
		t.Errorf("Pillowfort must NOT match midrange (the new entry is supposed to override the fallback)")
	}
}

// -----------------------------------------------------------------------------
// 5. Existing ComputeEvalWeights adjustment paths still stack on top of
//    the Pillowfort baseline (defensive — guards against the new entry
//    accidentally short-circuiting the post-archetype boost logic).
// -----------------------------------------------------------------------------

func TestComputeEvalWeights_PillowfortBaselineStackesPostArchetypeAdjustments(t *testing.T) {
	// Start from Pillowfort + a heavy ramp package (RampCount >= 14)
	// which adds +0.3 to ManaAdvantage. Pillowfort baseline ManaAdvantage
	// is 0.7 → expect 1.0.
	dp, r := freyaWeightsBaseline("Pillowfort")
	dp.RampCount = 14

	got := ComputeEvalWeights(dp, r)
	wantMana := defaultWeights["pillowfort"].ManaAdvantage + 0.3
	if got.ManaAdvantage != wantMana {
		t.Errorf("Pillowfort + heavy ramp ManaAdvantage: got %v, want %v (baseline 0.7 + 0.3 ramp boost)",
			got.ManaAdvantage, wantMana)
	}

	// And the low-interaction ThreatExposure boost (+0.3) should also
	// stack: r.RemovalCount = 0 in this case.
	r2 := &FreyaReport{
		NonLandTutorCount: 0,
		RemovalCount:      0, // < 5 triggers ThreatExposure +0.3
		Roles:             &RoleAnalysis{RoleCounts: map[RoleTag]int{}},
	}
	got2 := ComputeEvalWeights(dp, r2)
	wantThreat := defaultWeights["pillowfort"].ThreatExposure + 0.3
	if got2.ThreatExposure != wantThreat {
		t.Errorf("Pillowfort + low-interaction ThreatExposure: got %v, want %v (baseline 1.5 + 0.3 boost)",
			got2.ThreatExposure, wantThreat)
	}
}
