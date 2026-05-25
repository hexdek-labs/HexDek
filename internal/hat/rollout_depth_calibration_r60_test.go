package hat

import (
	"testing"
)

// rollout_depth_calibration_r60_test.go — pins the R60 round 5+ MCTS
// rollout-depth calibration. Pre-calibration the engine ran a single
// uniform `rolloutDepth = 3` constant across all 22 custom-tuned
// archetypes; this file enforces the per-archetype look-up table in
// rolloutDepthFor (rollout.go).
//
// Calibration intent (see rolloutDepthFor's doc comment for rationale):
//   - Depth 3 (fast clock): Aggro, Voltron, Reanimator, Midrange (default),
//     Tribal, Blink, ExtraCombats, GroupHug, CountersMatter
//   - Depth 4 (compound plans): Combo, Spellslinger, Storm, Aristocrats,
//     Selfmill, Enchantress, Artifacts, Ramp
//   - Depth 5 (slow lock / long-value plans): Control, Stax, Mill,
//     Lifegain, Superfriends, LandsMatter
//   - Unknown / empty falls back to depth 3 (preserves pre-calibration
//     behavior for external callers without an archetype string).

func TestRolloutDepthFor_UnknownFallsToBaseline(t *testing.T) {
	if got := rolloutDepthFor(""); got != rolloutDepth {
		t.Errorf("empty archetype should fall back to baseline rolloutDepth (%d); got %d",
			rolloutDepth, got)
	}
	if got := rolloutDepthFor("not_a_real_archetype"); got != rolloutDepth {
		t.Errorf("unknown archetype string should fall back to baseline rolloutDepth (%d); got %d",
			rolloutDepth, got)
	}
	if got := rolloutDepthFor(ArchetypeUnknown); got != rolloutDepth {
		t.Errorf("ArchetypeUnknown should fall back to baseline rolloutDepth (%d); got %d",
			rolloutDepth, got)
	}
}

func TestRolloutDepthFor_MidrangeIsBaseline(t *testing.T) {
	// Midrange anchors the baseline — bumping it would silently increase
	// rollout cost for every nil-strategy code path that uses the
	// fallback. Pin it explicitly to the same value as rolloutDepth.
	if got := rolloutDepthFor(ArchetypeMidrange); got != rolloutDepth {
		t.Errorf("Midrange should match baseline rolloutDepth (%d); got %d",
			rolloutDepth, got)
	}
}

func TestRolloutDepthFor_FastClockArchetypes(t *testing.T) {
	cases := []string{
		ArchetypeAggro,
		ArchetypeVoltron,
		ArchetypeReanimator,
		ArchetypeTribal,
		ArchetypeBlink,
		ArchetypeExtraCombats,
		ArchetypeGroupHug,
		ArchetypeCountersMatter,
	}
	for _, arch := range cases {
		if got := rolloutDepthFor(arch); got != 3 {
			t.Errorf("fast-clock archetype %q should rollout depth=3 (kill window fits); got %d",
				arch, got)
		}
	}
}

func TestRolloutDepthFor_CompoundPlanArchetypes(t *testing.T) {
	cases := []string{
		ArchetypeCombo,
		ArchetypeSpellslinger,
		ArchetypeStorm,
		ArchetypeAristocrats,
		ArchetypeSelfmill,
		ArchetypeEnchantress,
		ArchetypeArtifacts,
		ArchetypeRamp,
	}
	for _, arch := range cases {
		if got := rolloutDepthFor(arch); got != 4 {
			t.Errorf("compound-plan archetype %q should rollout depth=4 (tutor+win cycle / value chain); got %d",
				arch, got)
		}
	}
}

func TestRolloutDepthFor_SlowPlanArchetypes(t *testing.T) {
	cases := []string{
		ArchetypeControl,
		ArchetypeStax,
		ArchetypeMill,
		ArchetypeLifegain,
		ArchetypeSuperfriends,
		ArchetypeLandsMatter,
	}
	for _, arch := range cases {
		if got := rolloutDepthFor(arch); got != 5 {
			t.Errorf("slow-plan archetype %q should rollout depth=5 (lock / long-value materializes over 5+ turns); got %d",
				arch, got)
		}
	}
}

// TestRolloutDepthFor_BoundsHold pins both ends of the calibration: no
// archetype rolls out shallower than the baseline (3) — that would make
// rollouts strictly worse than the inner heuristic — and no archetype
// rolls out deeper than 5, which is the empirical cost ceiling
// (depth 6 ≈ 2× baseline cost, not justified absent gauntlet evidence).
func TestRolloutDepthFor_BoundsHold(t *testing.T) {
	all := []string{
		ArchetypeUnknown, ArchetypeAggro, ArchetypeMidrange, ArchetypeControl,
		ArchetypeRamp, ArchetypeCombo, ArchetypeStax, ArchetypeReanimator,
		ArchetypeSpellslinger, ArchetypeTribal, ArchetypeAristocrats,
		ArchetypeSelfmill, ArchetypeEnchantress, ArchetypeArtifacts,
		ArchetypeLifegain, ArchetypeVoltron, ArchetypeLandsMatter,
		ArchetypeCountersMatter, ArchetypeMill, ArchetypeStorm,
		ArchetypeSuperfriends, ArchetypeBlink, ArchetypeExtraCombats,
		ArchetypeGroupHug,
	}
	for _, arch := range all {
		d := rolloutDepthFor(arch)
		if d < rolloutDepth {
			t.Errorf("archetype %q rollout depth %d < baseline %d — rollouts would be strictly shallower than the inner heuristic",
				arch, d, rolloutDepth)
		}
		if d > 5 {
			t.Errorf("archetype %q rollout depth %d > 5 — exceeds the calibration cost ceiling",
				arch, d)
		}
	}
}

// TestRolloutDepthFor_OrderingByPlanWindow pins the gameplay invariant
// behind the calibration: Control's plan window must be at least as
// long as Aggro's. Aggro wins fast; Control wins long. If a future
// retune ever inverts this it would mean the calibration model itself
// has flipped, and the change should be explicit.
func TestRolloutDepthFor_OrderingByPlanWindow(t *testing.T) {
	if rolloutDepthFor(ArchetypeControl) <= rolloutDepthFor(ArchetypeAggro) {
		t.Errorf("Control rollout depth (%d) must exceed Aggro (%d) — Control wins on longer plan windows",
			rolloutDepthFor(ArchetypeControl), rolloutDepthFor(ArchetypeAggro))
	}
	if rolloutDepthFor(ArchetypeStax) <= rolloutDepthFor(ArchetypeAggro) {
		t.Errorf("Stax rollout depth (%d) must exceed Aggro (%d) — Stax lock pieces compound over 5+ turns",
			rolloutDepthFor(ArchetypeStax), rolloutDepthFor(ArchetypeAggro))
	}
	if rolloutDepthFor(ArchetypeCombo) <= rolloutDepthFor(ArchetypeAggro) {
		t.Errorf("Combo rollout depth (%d) must exceed Aggro (%d) — Combo decisions span tutor+win cycles",
			rolloutDepthFor(ArchetypeCombo), rolloutDepthFor(ArchetypeAggro))
	}
}
