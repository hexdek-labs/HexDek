package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// enlist_wire_r63_test.go — Enlist (CR §702.154a). The keywords_enlist.go
// primitive (HasEnlist/CanEnlist/ApplyEnlist) was complete and correct but had
// NO live caller — enlist was inert in real combat. These pin the
// DeclareAttackers wiring (ApplyEnlistForAttackers) and the primitive's key
// properties.

func enlistAttacker(gs *GameState, seat int, name string, pow, tough int) *Permanent {
	ast := &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "enlist"}},
	}
	return addBattlefieldWithAST(gs, seat, name, pow, tough, ast, "creature")
}

// (1)+(2)+(3) An enlist attacker (stamped attacking) taps the highest-power
// legal enlistee and gains its power until end of turn; the enlistee is tapped
// but NOT marked attacking.
func TestEnlist_TapsHelperAndPumps(t *testing.T) {
	gs := newCombatGame(t)
	atk := enlistAttacker(gs, 0, "Recruiter Sergeant", 3, 3)
	setPermFlag(atk, flagAttacking, true) // simulate post-declaration
	weak := addBattlefield(gs, 0, "Weak Helper", 1, 1, "creature")
	strong := addBattlefield(gs, 0, "Strong Helper", 4, 4, "creature")

	ApplyEnlistForAttackers(gs, 0, []*Permanent{atk})

	if !strong.Tapped {
		t.Fatal("the highest-power legal enlistee should be tapped")
	}
	if weak.Tapped {
		t.Fatal("only one enlistee should be tapped (the highest power)")
	}
	if strong.IsAttacking() {
		t.Fatal("the enlisted creature must NOT become an attacker (CR §702.154a)")
	}
	if got := atk.Power(); got != 7 {
		t.Fatalf("attacker should gain the enlistee's +4 power (3→7), got %d", got)
	}
	if atk.Flags["enlisted_this_combat"] != 1 {
		t.Fatal("enlisted_this_combat should be set for payoffs (Aradesh)")
	}
}

// (3-revert) The power bonus is until-end-of-turn and reverts at cleanup.
func TestEnlist_PowerRevertsAtCleanup(t *testing.T) {
	gs := newCombatGame(t)
	atk := enlistAttacker(gs, 0, "Recruiter", 2, 2)
	setPermFlag(atk, flagAttacking, true)
	addBattlefield(gs, 0, "Helper", 3, 3, "creature")

	ApplyEnlistForAttackers(gs, 0, []*Permanent{atk})
	if atk.Power() != 5 {
		t.Fatalf("attacker should be 5 after enlist, got %d", atk.Power())
	}
	ScanExpiredDurations(gs, "ending", "cleanup")
	if atk.Power() != 2 {
		t.Fatalf("attacker power should revert to 2 at cleanup, got %d", atk.Power())
	}
}

// (CanEnlist) Summoning-sick / already-tapped / attacking creatures are not
// legal enlistees.
func TestEnlist_IllegalEnlisteesSkipped(t *testing.T) {
	gs := newCombatGame(t)
	atk := enlistAttacker(gs, 0, "Recruiter", 3, 3)
	setPermFlag(atk, flagAttacking, true)

	sick := addBattlefield(gs, 0, "Sick", 5, 5, "creature")
	sick.SummoningSick = true
	tapped := addBattlefield(gs, 0, "Tapped", 5, 5, "creature")
	tapped.Tapped = true
	otherAtk := addBattlefield(gs, 0, "Other Attacker", 5, 5, "creature")
	setPermFlag(otherAtk, flagAttacking, true)

	_ = tapped
	_ = otherAtk
	ApplyEnlistForAttackers(gs, 0, []*Permanent{atk})

	if atk.Power() != 3 {
		t.Fatalf("no legal enlistee exists — attacker power should stay 3, got %d", atk.Power())
	}
	if sick.Tapped {
		t.Fatal("a summoning-sick creature must not be enlisted")
	}
	if atk.Flags["enlisted_this_combat"] == 1 {
		t.Fatal("enlisted_this_combat must not be set when no legal enlistee exists")
	}
}

// (5) Integration: DeclareAttackers auto-invokes enlist. A defender helper
// won't be declared as an attacker (greedy excludes defenders) but is a legal
// enlistee, so the declared enlist attacker taps it and gains its power.
func TestEnlist_WiredIntoDeclareAttackers(t *testing.T) {
	gs := newCombatGame(t)
	atk := enlistAttacker(gs, 0, "Recruiter", 3, 3)
	helper := addBattlefield(gs, 0, "Wall Helper", 2, 2, "creature")
	helper.Flags["kw:defender"] = 1 // can't attack, but is a legal enlistee

	DeclareAttackers(gs, 0)

	if !helper.Tapped {
		t.Fatal("DeclareAttackers should auto-enlist the defender helper")
	}
	if helper.IsAttacking() {
		t.Fatal("the enlisted defender must not be attacking")
	}
	if got := atk.Power(); got != 5 {
		t.Fatalf("declared enlist attacker should be pumped to 5 (3+2), got %d", got)
	}
}
