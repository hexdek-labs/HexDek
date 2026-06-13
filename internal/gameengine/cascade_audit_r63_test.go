package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// cascade_audit_r63_test.go — regressions for the r63 cascade audit
// (CR §702.85). Pins three fixes to ApplyCascade:
//   1. the cast card's Name is NOT mutated (it leaked "(cascade)" onto the
//      battlefield permanent / graveyard card, breaking name-based matching);
//   2. a cascade-cast spell counts toward storm / cast count (it is cast);
//   3. cascade CHAINS — a cascaded spell that itself has cascade triggers.

func cascAuditGame() *GameState {
	return NewGameState(2, rand.New(rand.NewSource(99)), nil)
}

// Cascade into a real creature: it enters the battlefield with its name
// intact (no "(cascade)" suffix corruption).
func TestCascadeAudit_CreatureEntersWithCleanName(t *testing.T) {
	gs := cascAuditGame()
	bear := &Card{
		Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"},
		CMC: 2, BasePower: 2, BaseToughness: 2,
		AST: &gameast.CardAST{Name: "Grizzly Bears"},
	}
	gs.Seats[0].Library = []*Card{
		{Name: "Forest", Owner: 0, Types: []string{"land"}, CMC: 0},
		bear,
	}
	if !ApplyCascade(gs, 0, 4, "Cascade Spell") {
		t.Fatal("cascade should hit the creature")
	}
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("creature should enter battlefield, got %d permanents", len(gs.Seats[0].Battlefield))
	}
	if got := gs.Seats[0].Battlefield[0].Card.Name; got != "Grizzly Bears" {
		t.Errorf("cascade corrupted the card name: %q (want %q)", got, "Grizzly Bears")
	}
	// Source object identity preserved too.
	if bear.Name != "Grizzly Bears" {
		t.Errorf("cascade mutated the source Card.Name: %q", bear.Name)
	}
}

// A cascade-cast spell is cast → it counts toward the storm / cast tally.
func TestCascadeAudit_CastCountIncremented(t *testing.T) {
	gs := cascAuditGame()
	before := gs.SpellsCastThisTurn
	gs.Seats[0].Library = []*Card{
		{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1,
			AST: &gameast.CardAST{Name: "Lightning Bolt"}},
	}
	if !ApplyCascade(gs, 0, 3, "Cascade Spell") {
		t.Fatal("cascade should hit Lightning Bolt")
	}
	if gs.SpellsCastThisTurn != before+1 {
		t.Errorf("cascade-cast spell should increment cast/storm count: before=%d after=%d", before, gs.SpellsCastThisTurn)
	}
	// And it should be recorded in the per-turn cast list.
	if n := len(gs.Seats[0].Turn.Casts); n < 1 {
		t.Errorf("cascade-cast should append a CastRecord, got %d", n)
	}
}

// Cascade chains: a cascaded spell that itself has cascade fires its own
// cascade trigger (two cascade_trigger events total).
func TestCascadeAudit_Chains(t *testing.T) {
	gs := cascAuditGame()
	inner := &Card{
		Name: "Bituminous Blast", Owner: 0, Types: []string{"instant"}, CMC: 2,
		AST: &gameast.CardAST{
			Name:      "Bituminous Blast",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "cascade"}},
		},
	}
	gs.Seats[0].Library = []*Card{
		inner, // cmc 2 < 5 → the outer cascade hits this cascade spell
		{Name: "Ornithopter", Owner: 0, Types: []string{"artifact"}, CMC: 0,
			BasePower: 0, BaseToughness: 2, AST: &gameast.CardAST{Name: "Ornithopter"}},
	}
	if !ApplyCascade(gs, 0, 5, "Maelstrom Wanderer") {
		t.Fatal("outer cascade should hit Bituminous Blast")
	}
	if n := countEvents(gs, "cascade_trigger"); n < 2 {
		t.Errorf("cascade should chain (inner cascade spell re-triggers): cascade_trigger=%d, want >=2", n)
	}
}

// CR §702.85b — a card with multiple cascade instances ("Cascade, cascade")
// triggers cascade once per instance. CascadeCount drives the cast-path loop.
func TestCascadeAudit_MultipleInstancesCount(t *testing.T) {
	mw := &Card{
		Name: "Maelstrom Wanderer", Owner: 0, Types: []string{"creature"}, CMC: 8,
		AST: &gameast.CardAST{
			Name: "Maelstrom Wanderer",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "cascade"},
				&gameast.Keyword{Name: "cascade"},
			},
		},
	}
	if got := CascadeCount(mw); got != 2 {
		t.Errorf("Maelstrom Wanderer CascadeCount = %d, want 2", got)
	}
	apex := &Card{
		Name: "Apex Devastator", Owner: 0, Types: []string{"creature"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "cascade"}, &gameast.Keyword{Name: "cascade"},
			&gameast.Keyword{Name: "cascade"}, &gameast.Keyword{Name: "cascade"},
		}},
	}
	if got := CascadeCount(apex); got != 4 {
		t.Errorf("Apex Devastator CascadeCount = %d, want 4", got)
	}
	// Driving two cascades fires two cascade_trigger events (each exiles).
	gs := cascAuditGame()
	gs.Seats[0].Library = []*Card{
		{Name: "Spell A", Owner: 0, Types: []string{"instant"}, CMC: 1, AST: &gameast.CardAST{Name: "Spell A"}},
		{Name: "Spell B", Owner: 0, Types: []string{"instant"}, CMC: 1, AST: &gameast.CardAST{Name: "Spell B"}},
	}
	for i := 0; i < CascadeCount(mw); i++ {
		ApplyCascade(gs, 0, 8, "Maelstrom Wanderer")
	}
	if n := countEvents(gs, "cascade_trigger"); n != 2 {
		t.Errorf("two cascade instances should fire 2 cascade_trigger events, got %d", n)
	}
}

// Lands are exiled but never stop the cascade; the comparison is strictly
// LESS-THAN (a nonland with equal MV does not stop it).
func TestCascadeAudit_EqualMVDoesNotStop(t *testing.T) {
	gs := cascAuditGame()
	gs.Seats[0].Library = []*Card{
		{Name: "Equal Cost", Owner: 0, Types: []string{"sorcery"}, CMC: 3}, // == spellCMC, must NOT stop
		{Name: "Cheaper", Owner: 0, Types: []string{"instant"}, CMC: 1,
			AST: &gameast.CardAST{Name: "Cheaper"}},
	}
	if !ApplyCascade(gs, 0, 3, "Cascade Spell") {
		t.Fatal("cascade should skip the equal-MV card and hit the cheaper one")
	}
	// "Equal Cost" (CMC 3, == 3) must have been exiled-and-bottomed, not cast.
	// The cheaper instant was cast; the equal-cost sorcery went to library bottom.
	foundEqualInLibrary := false
	for _, c := range gs.Seats[0].Library {
		if c != nil && c.Name == "Equal Cost" {
			foundEqualInLibrary = true
		}
	}
	if !foundEqualInLibrary {
		t.Error("equal-MV nonland should NOT be cast (must be put on bottom); cascade requires strictly-lesser MV")
	}
}
