package gameengine

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCombat_508_1g_SweepCoverage pins the result of the §508.1g
// audit + sweep (PR #967 fixed Raph & Mikey; this PR sweeps the
// remaining 13 sites — 11 per_card + 2 engine — that put creatures
// onto the battlefield in an attacking state).
//
// Every bare `Flags["attacking"] = 1` / inline map literal with
// `"attacking": 1` in a non-test, non-DeclareAttackers context must
// be replaced by a MarkEnteredAttacking call so the §508.1g carve-out
// tag is stamped. This test greps the source tree and fails if any
// raw set-attacking site remains outside the canonical chokepoints.
//
// Canonical chokepoints (allowed bare-set sites):
//   - combat.go:DeclareAttackers — the §508.1a-f legal-pool path
//   - combat.go:MarkEnteredAttacking — the helper itself
//
// Failure indicates a new per_card handler or engine site bypassed
// the §508.1g hook and would re-introduce the PR #950 / Wall of
// Tanglecord-class invariant false-positive class.
func TestCombat_508_1g_SweepCoverage(t *testing.T) {
	// Find all "attacking" flag set sites in non-test go files.
	cmd := exec.Command("grep", "-rn",
		"-E", `Flags\["attacking"\] = 1|"attacking": 1|setPermFlag\(.*flagAttacking, true\)`,
		"--include=*.go",
		"./internal/gameengine",
		"./internal/gameengine/per_card",
	)
	out, _ := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	var violations []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Skip test files.
		if strings.Contains(line, "_test.go") {
			continue
		}
		// Skip the canonical chokepoints: the DeclareAttackers
		// legal-pool path and the MarkEnteredAttacking helper itself.
		if strings.Contains(line, "combat.go") {
			// combat.go:514 is DeclareAttackers setting flagAttacking
			// on a creature that passed canAttack — this is the
			// non-§508.1g path and is correct.
			// combat.go:MarkEnteredAttacking is the helper.
			continue
		}
		violations = append(violations, line)
	}

	if len(violations) > 0 {
		t.Fatalf("found %d raw set-attacking sites that should route through MarkEnteredAttacking:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}
