package gameengine

// extort_behavior_r63_test.go — r63 probe lock-ins for Extort (CR §702.99).
// Extort is LIVE and correct (fired from FireCastTriggerObservers, reached by
// CastSpell/CastFromZone). The existing suite (keywords_flagged_test.go) covers
// drain/gain, no-mana, multiple-permanents, 4-player, and the copy-guard. These
// pin the two properties it left uncovered: living-opponent scaling under
// elimination (f) and the "your casts only" gate (a).

import "testing"

func extortProbeCast(name string) *Card {
	return &Card{Name: name, Owner: 0, Types: []string{"instant"}}
}

// (f) Extort scales with LIVING opponents only — an eliminated opponent is
// neither drained nor counted toward the gain.
func TestExtort_ScalesWithLivingOpponentsOnly(t *testing.T) {
	gs := newKWCombatGame4P(t)
	addKWCombatBattlefieldWithKeyword(gs, 0, "Crypt Ghast", 2, 2, "extort", "creature")
	gs.Seats[0].ManaPool = 5
	for i := 0; i < 4; i++ {
		gs.Seats[i].Life = 20
	}
	gs.Seats[3].Lost = true // only seats 1 and 2 remain live opponents

	FireCastTriggerObservers(gs, extortProbeCast("Some Spell"), 0, false)

	if gs.Seats[1].Life != 19 || gs.Seats[2].Life != 19 {
		t.Errorf("(f) living opponents should each lose 1; got s1=%d s2=%d", gs.Seats[1].Life, gs.Seats[2].Life)
	}
	if gs.Seats[3].Life != 20 {
		t.Errorf("(f) the eliminated opponent must not be drained; got %d", gs.Seats[3].Life)
	}
	if gs.Seats[0].Life != 22 {
		t.Errorf("(f) controller should gain only 2 (two living opponents), got %d", gs.Seats[0].Life)
	}
}

// (a) Extort only triggers on the controller's OWN casts — an opponent's spell
// must not fire it.
func TestExtort_OnlyYourCasts(t *testing.T) {
	gs := newKWCombatGame4P(t)
	addKWCombatBattlefieldWithKeyword(gs, 0, "Crypt Ghast", 2, 2, "extort", "creature")
	gs.Seats[0].ManaPool = 5
	for i := 0; i < 4; i++ {
		gs.Seats[i].Life = 20
	}

	// Seat 1 (an opponent) casts a spell.
	FireCastTriggerObservers(gs, extortProbeCast("Opp Spell"), 1, false)

	if gs.Seats[0].Life != 20 {
		t.Errorf("(a) your extort must not fire on an opponent's cast; controller life=%d", gs.Seats[0].Life)
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("(a) no extort mana should be spent on an opponent's cast; pool=%d", gs.Seats[0].ManaPool)
	}
}
