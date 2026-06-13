package gameengine

import (
	"math/rand"
	"testing"
)

// mod_tail_grant_keyword_eot_r63_test.go — regressions for the
// "<subject> gains <keyword> until end of turn" parsed_tail handler.

func grantGame() (*GameState, *Permanent) {
	gs := NewGameState(2, rand.New(rand.NewSource(5)), nil)
	src := &Permanent{Card: &Card{Name: "Grant Spell", Owner: 0}, Controller: 0, Owner: 0, Flags: map[string]int{}}
	return gs, src
}

func ownCreature(gs *GameState, seat, pow, tough int, name string, colors ...string) *Permanent {
	c := &Card{Name: name, Owner: seat, Types: []string{"creature"}, BasePower: pow, BaseToughness: tough, Colors: colors}
	p := &Permanent{Card: c, Controller: seat, Owner: seat, Timestamp: gs.NextTimestamp(), Flags: map[string]int{}, Counters: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Single-target grant: "target creature you control gains trample until end
// of turn" → the controller's (best) creature has trample.
func TestGrantKw_SingleTarget(t *testing.T) {
	gs, src := grantGame()
	bear := ownCreature(gs, 0, 2, 2, "Bear")
	if bear.HasKeyword("trample") {
		t.Fatal("precondition: should not have trample yet")
	}
	ok := resolveGrantKeywordEOTText(gs, src, "target creature you control gains trample until end of turn")
	if !ok {
		t.Fatal("handler should recognize the grant")
	}
	if !bear.HasKeyword("trample") {
		t.Error("creature should have trample after the grant")
	}
}

// Mass grant: "creatures you control gain vigilance until end of turn" → all
// own creatures get vigilance; opponent's creature does not.
func TestGrantKw_MassOwnCreatures(t *testing.T) {
	gs, src := grantGame()
	a := ownCreature(gs, 0, 1, 1, "A")
	b := ownCreature(gs, 0, 3, 3, "B")
	opp := ownCreature(gs, 1, 4, 4, "Opp")
	resolveGrantKeywordEOTText(gs, src, "creatures you control gain vigilance until end of turn")
	if !a.HasKeyword("vigilance") || !b.HasKeyword("vigilance") {
		t.Error("all own creatures should gain vigilance")
	}
	if opp.HasKeyword("vigilance") {
		t.Error("opponent's creature must NOT gain vigilance")
	}
}

// Multi-keyword grant: "gains menace, lifelink, and haste".
func TestGrantKw_MultipleKeywords(t *testing.T) {
	gs, src := grantGame()
	c := ownCreature(gs, 0, 2, 2, "Multi")
	resolveGrantKeywordEOTText(gs, src, "target creature gains menace, lifelink, and haste until end of turn")
	for _, kw := range []string{"menace", "lifelink", "haste"} {
		if !c.HasKeyword(kw) {
			t.Errorf("creature should have %q", kw)
		}
	}
}

// Indestructible grant is honored by IsIndestructible (reads GrantedAbilities).
func TestGrantKw_IndestructibleHonored(t *testing.T) {
	gs, src := grantGame()
	c := ownCreature(gs, 0, 2, 2, "Hero")
	resolveGrantKeywordEOTText(gs, src, "creatures you control gain indestructible until end of turn")
	if !c.IsIndestructible() {
		t.Error("creature should be indestructible after the grant")
	}
}

// Color-filtered mass grant: only white creatures.
func TestGrantKw_ColorFilteredMass(t *testing.T) {
	gs, src := grantGame()
	w := ownCreature(gs, 0, 1, 1, "Whitey", "W")
	r := ownCreature(gs, 0, 1, 1, "Reddy", "R")
	resolveGrantKeywordEOTText(gs, src, "white creatures you control gain first strike until end of turn")
	if !w.HasKeyword("first strike") {
		t.Error("white creature should gain first strike")
	}
	if r.HasKeyword("first strike") {
		t.Error("non-white creature must NOT gain first strike")
	}
}

// The grant expires at end of turn (GrantedAbilities cleared at cleanup).
func TestGrantKw_ExpiresEndOfTurn(t *testing.T) {
	gs, src := grantGame()
	c := ownCreature(gs, 0, 2, 2, "Temp")
	resolveGrantKeywordEOTText(gs, src, "target creature gains flying until end of turn")
	if !c.HasKeyword("flying") {
		t.Fatal("should have flying during the turn")
	}
	c.GrantedAbilities = c.GrantedAbilities[:0] // simulate the EOT cleanup (phases.go:401)
	if c.HasKeyword("flying") {
		t.Error("flying should be gone after end-of-turn cleanup")
	}
}

// Conservative: arg-bearing / non-whitelisted grants stay inert (return
// false), never mis-resolve.
func TestGrantKw_SkipsNonWhitelisted(t *testing.T) {
	gs, src := grantGame()
	ownCreature(gs, 0, 2, 2, "X")
	if resolveGrantKeywordEOTText(gs, src, "target creature gains protection from red until end of turn") {
		t.Error("protection-from-X is not whitelisted and must stay inert")
	}
	if resolveGrantKeywordEOTText(gs, src, "you gain 3 life until end of turn") {
		t.Error("non-keyword 'gain life' must not be treated as a keyword grant")
	}
}

// No own creature → no-op (returns false), no panic.
func TestGrantKw_NoCreatureNoop(t *testing.T) {
	gs, src := grantGame()
	if resolveGrantKeywordEOTText(gs, src, "target creature you control gains trample until end of turn") {
		t.Error("with no creature, should return false (inert)")
	}
}

// Integration via the residual dispatcher (the parsed_tail route).
func TestGrantKw_IntegrationViaResidualDispatch(t *testing.T) {
	gs, src := grantGame()
	c := ownCreature(gs, 0, 2, 2, "Combatant")
	if !resolveResidualByText(gs, src, "creatures you control gain deathtouch until end of turn") {
		t.Fatal("resolveResidualByText should report the grant as handled")
	}
	if !c.HasKeyword("deathtouch") {
		t.Error("integration: creature should have deathtouch")
	}
}

// Trigger/conditional-framed grants stay inert (parser gap, not an
// unconditional one-shot).
func TestGrantKw_SkipsTriggerFramed(t *testing.T) {
	gs, src := grantGame()
	ownCreature(gs, 0, 2, 2, "Y")
	if resolveGrantKeywordEOTText(gs, src, "whenever this creature attacks, it gains trample until end of turn") {
		t.Error("trigger-framed grant must stay inert")
	}
	if resolveGrantKeywordEOTText(gs, src, "if you control a forest, target creature gains reach until end of turn") {
		t.Error("conditional-framed grant must stay inert")
	}
}
