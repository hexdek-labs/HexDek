package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// class_level_production_path_r63_test.go — r63 Class probe.
//
// The 6 curated class_level_up consumers (class_level_up_consumers_r60.go) were
// only ever exercised by tests calling FireCardTrigger directly; in real games
// nothing fired class_level_up (the only emitters were the dead LevelUpClass /
// AdvanceClassLevel helpers). The r63 fix fires the trigger from the production
// level-up path (the AST level_marker resolve handler → FireClassLevelTriggers).
// This drives that real path and asserts the curated consumer fires exactly
// once — proving both the wiring AND the no-double-fire guard (the generic AST
// dispatch skips per_card-owned classes).

func TestWizardClass_FiresOnProductionLevelMarkerPath(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top", "Next", "Third")
	wiz := addClassPerm(gs, 0, "Wizard Class", 1)

	// Drive the production path: resolve the level-2 activated ability's
	// level_marker effect, exactly as ActivateAbility would at runtime.
	gameengine.ResolveEffect(gs, wiz, &gameast.ModificationEffect{
		ModKind: "level_marker", Args: []interface{}{2},
	})

	// Wizard Class becomes level 2 → draw two. Exactly two (a double-fire from
	// the generic AST path firing alongside the per_card consumer would draw 4).
	if got := len(gs.Seats[0].Hand); got != 2 {
		t.Fatalf("Wizard Class via production level-up should draw exactly 2; hand=%d", got)
	}
	if wiz.Flags["class_level"] != 2 {
		t.Fatalf("class_level flag should sync to 2 on the production path, got %d", wiz.Flags["class_level"])
	}
}
