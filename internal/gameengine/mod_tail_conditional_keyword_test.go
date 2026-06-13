package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mod_tail_conditional_keyword_test.go — regressions for the conditional
// self keyword-grant parsed_tail handler (worker hex-dev-5).

func condKwCreature(gs *GameState, seat int, name, tail string) *Permanent {
	ast := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{ModKind: "parsed_tail", Args: []interface{}{tail}}},
		},
	}
	p := addBattlefieldWithAST(gs, seat, name, 2, 2, ast, "creature")
	RegisterContinuousEffectsForPermanent(gs, p)
	return p
}

func TestCondKw_BlueCreatureGatesFlying(t *testing.T) {
	gs := newFixtureGame(t)
	scare := condKwCreature(gs, 0, "Wingrattle Scarecrow",
		"this creature has flying as long as you control a blue creature")

	// No blue creature yet → condition false → no flying.
	if gs.HasKeywordOf(scare, "flying") {
		t.Fatalf("flying should NOT be granted with no blue creature")
	}
	// Add a blue creature → condition true → flying.
	blue := addBattlefield(gs, 0, "Blue Bear", 2, 2, "creature")
	blue.Card.Colors = []string{"U"}
	gs.InvalidateCharacteristicsCache()
	if !gs.HasKeywordOf(scare, "flying") {
		t.Errorf("flying SHOULD be granted while controlling a blue creature")
	}
}

func TestCondKw_AnotherVampireExcludesSelf(t *testing.T) {
	gs := newFixtureGame(t)
	crusader := condKwCreature(gs, 0, "Markov Crusader",
		"this creature has haste as long as you control another vampire")
	crusader.Card.TypeLine = "Creature — Vampire Knight"
	gs.InvalidateCharacteristicsCache()

	// Only itself (a vampire) → "another vampire" is false → no haste.
	if gs.HasKeywordOf(crusader, "haste") {
		t.Fatalf("'another vampire' must exclude self — no haste with only itself")
	}
	// A second vampire → haste.
	v2 := addBattlefield(gs, 0, "Vampire Friend", 1, 1, "creature")
	v2.Card.TypeLine = "Creature — Vampire"
	gs.InvalidateCharacteristicsCache()
	if !gs.HasKeywordOf(crusader, "haste") {
		t.Errorf("haste SHOULD be granted while controlling another vampire")
	}
}

func TestCondKw_GateSubtype(t *testing.T) {
	gs := newFixtureGame(t)
	guard := condKwCreature(gs, 0, "Armory Guard",
		"this creature has vigilance as long as you control a gate")
	if gs.HasKeywordOf(guard, "vigilance") {
		t.Fatalf("no vigilance without a gate")
	}
	gate := addBattlefield(gs, 0, "Sea Gate", 0, 0, "land")
	gate.Card.TypeLine = "Land — Gate"
	gs.InvalidateCharacteristicsCache()
	if !gs.HasKeywordOf(guard, "vigilance") {
		t.Errorf("vigilance SHOULD be granted while controlling a gate")
	}
}

func TestCondKw_ArtifactCondition(t *testing.T) {
	gs := newFixtureGame(t)
	skulker := condKwCreature(gs, 0, "Nezumi Bladeblesser",
		"this creature has deathtouch as long as you control an artifact")
	addBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	gs.InvalidateCharacteristicsCache()
	if !gs.HasKeywordOf(skulker, "deathtouch") {
		t.Errorf("deathtouch SHOULD be granted while controlling an artifact")
	}
}

func TestCondKw_UnparseableConditionSkipped(t *testing.T) {
	gs := newFixtureGame(t)
	// Not a "you control a/an/another" shape → regex doesn't match → skip.
	weird := condKwCreature(gs, 0, "Sulam Djinn",
		"this creature has flying as long as green is the most common color among all permanents")
	// Even with green permanents, we never granted (clause not recognized).
	g := addBattlefield(gs, 0, "Green Bear", 2, 2, "creature")
	g.Card.Colors = []string{"G"}
	gs.InvalidateCharacteristicsCache()
	if gs.HasKeywordOf(weird, "flying") {
		t.Errorf("unrecognized condition shape must be skipped, not granted")
	}
}
