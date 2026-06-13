package gameengine

// Regression for the deep loki r63c sweep finding: canAttack used the
// non-layer-aware p.HasKeyword("defender"), so a creature with a
// LAYER-GRANTED defender (scaffold_keyword_grant "creatures … have
// defender") was admitted to the legal attacker pool, the §508.1
// sanitizer kept it (it WAS in-pool), and it attacked illegally — 52
// §508.1a "declared attacker has defender" hits across the 4000-game
// max-turns-100 sweep, every one a granted-defender attacker. The fix
// gates the legal pool on gs.HasKeywordOf, matching the §508.1a validator.

import (
	"testing"
)

func grantLayerKeyword(gs *GameState, target *Permanent, kw string) {
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:      LayerAbility,
		Timestamp:  1,
		SourcePerm: target,
		HandlerID:  "test_grant_" + kw,
		Predicate:  func(_ *GameState, p *Permanent) bool { return p == target },
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Keywords = append(chars.Keywords, kw)
		},
	})
	gs.InvalidateCharacteristicsCache()
}

func attackTestCreature(gs *GameState, seat int, name string, power int) *Permanent {
	p := &Permanent{
		Card:          &Card{Name: name, Owner: seat, Types: []string{"creature"}, BasePower: power, BaseToughness: power},
		Controller:    seat,
		Owner:         seat,
		SummoningSick: false,
		Tapped:        false,
		Flags:         map[string]int{},
		Counters:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func declaredContains(declared []*Permanent, p *Permanent) bool {
	for _, d := range declared {
		if d == p {
			return true
		}
	}
	return false
}

func TestDeclareAttackers_GrantedDefender_Excluded(t *testing.T) {
	gs := newTestGameState(2)
	gs.Active = 0

	normal := attackTestCreature(gs, 0, "Vanilla Bear", 3)
	stuck := attackTestCreature(gs, 0, "Granted-Defender Creature", 4)
	grantLayerKeyword(gs, stuck, "defender")

	// Preconditions: the granted defender is layer-real but not printed.
	if stuck.HasKeyword("defender") {
		t.Fatal("precondition: attacker must NOT have a printed/Flags defender")
	}
	if !gs.HasKeywordOf(stuck, "defender") {
		t.Fatal("precondition: attacker must have a LAYER-granted defender")
	}

	declared := DeclareAttackers(gs, 0)

	if declaredContains(declared, stuck) {
		t.Error("a creature with layer-granted defender must NOT be declared as an attacker (§508.1a)")
	}
	if !declaredContains(declared, normal) {
		t.Error("the vanilla creature should still attack (fix must not over-suppress)")
	}
}

// Sanity: a printed-defender creature was already excluded (canAttack) —
// pin that the new gate doesn't change that path.
func TestDeclareAttackers_PrintedDefender_StillExcluded(t *testing.T) {
	gs := newTestGameState(2)
	gs.Active = 0
	// Power 2 so the canAttack power gate doesn't pre-exclude it — the
	// printed-defender check is what must keep it out.
	wall := attackTestCreature(gs, 0, "Walking Wall", 2)
	wall.Flags["kw:defender"] = 1 // printed/Flags defender

	declared := DeclareAttackers(gs, 0)
	if declaredContains(declared, wall) {
		t.Error("a printed-defender 0/8 wall must never attack")
	}
}
