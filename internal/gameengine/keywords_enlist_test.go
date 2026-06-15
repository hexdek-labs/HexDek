package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_enlist_test.go — property regressions for the Enlist primitive
// (CR §702.154). Enlist was entirely unimplemented before r63 (only a header
// comment + a per-card payoff reading a flag nothing set); these pin the new
// HasEnlist / CanEnlist / ApplyEnlist behavior across every property.

func newEnlistGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(57)), nil)
}

func enlistCreature(gs *GameState, seat int, name string, power int) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: power,
		AST:           &gameast.CardAST{Name: name},
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func TestHasEnlist_Detects(t *testing.T) {
	c := &Card{Name: "Aradesh", AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "enlist"}}}}
	if !HasEnlist(c) {
		t.Fatal("HasEnlist should detect the enlist keyword")
	}
	if HasEnlist(&Card{Name: "Bear", AST: &gameast.CardAST{}}) {
		t.Fatal("HasEnlist should be false for a vanilla creature")
	}
}

// (a)+(b)+(c)+(e) Enlisting a valid creature taps it, adds its power to the
// attacker until EOT, doesn't make the enlistee an attacker, and sets the
// payoff flag.
func TestApplyEnlist_TapsAndAddsPower(t *testing.T) {
	gs := newEnlistGame(t)
	attacker := enlistCreature(gs, 0, "Aradesh", 3)
	attacker.Flags["attacking"] = 1
	helper := enlistCreature(gs, 0, "Big Helper", 4)

	if !ApplyEnlist(gs, 0, attacker, helper) {
		t.Fatal("ApplyEnlist should succeed for an untapped, non-sick, non-attacking creature")
	}
	if !helper.Tapped {
		t.Error("(a) the enlisted creature must be tapped")
	}
	if helper.IsAttacking() {
		t.Error("(c) the enlisted creature must NOT become an attacker")
	}
	if got := attacker.Power(); got != 7 {
		t.Errorf("(b) attacker power should be 3+4=7, got %d", got)
	}
	if attacker.Flags["enlisted_this_combat"] != 1 {
		t.Error("(b) enlisted_this_combat payoff flag should be set on the attacker")
	}
}

// (a) Summoning-sick / tapped / attacking / opponent / self / non-creature
// enlistees are all rejected (atomic — no tap).
func TestCanEnlist_RejectsIllegalTargets(t *testing.T) {
	gs := newEnlistGame(t)
	attacker := enlistCreature(gs, 0, "Aradesh", 3)
	attacker.Flags["attacking"] = 1

	sick := enlistCreature(gs, 0, "Sick", 2)
	sick.SummoningSick = true
	if CanEnlist(gs, 0, attacker, sick) {
		t.Error("summoning-sick creature must not be enlistable")
	}

	tapped := enlistCreature(gs, 0, "Tapped", 2)
	tapped.Tapped = true
	if CanEnlist(gs, 0, attacker, tapped) {
		t.Error("already-tapped creature must not be enlistable")
	}

	otherAttacker := enlistCreature(gs, 0, "Co-Attacker", 2)
	otherAttacker.Flags["attacking"] = 1
	if CanEnlist(gs, 0, attacker, otherAttacker) {
		t.Error("an attacking creature must not be enlistable (nonattacking only)")
	}

	opp := enlistCreature(gs, 1, "Enemy", 5)
	if CanEnlist(gs, 0, attacker, opp) {
		t.Error("an opponent's creature must not be enlistable")
	}

	if CanEnlist(gs, 0, attacker, attacker) {
		t.Error("a creature can't enlist itself")
	}

	// Failed enlist taps nothing.
	if ApplyEnlist(gs, 0, attacker, sick) {
		t.Error("ApplyEnlist must fail for an illegal target")
	}
	if sick.Tapped {
		t.Error("a failed enlist must not tap the target (atomic)")
	}
}

// (d) Enlisting a 0-power creature is legal and adds 0.
func TestApplyEnlist_ZeroPowerLegal(t *testing.T) {
	gs := newEnlistGame(t)
	attacker := enlistCreature(gs, 0, "Aradesh", 3)
	attacker.Flags["attacking"] = 1
	wall := enlistCreature(gs, 0, "Wall", 0) // 0/0 base; set toughness so it survives

	if !ApplyEnlist(gs, 0, attacker, wall) {
		t.Fatal("(d) enlisting a 0-power creature must be legal")
	}
	if !wall.Tapped {
		t.Error("(d) the 0-power enlistee must still be tapped")
	}
	if got := attacker.Power(); got != 3 {
		t.Errorf("(d) 0-power enlist adds 0; attacker should stay 3, got %d", got)
	}
	if attacker.Flags["enlisted_this_combat"] != 1 {
		t.Error("(d) enlisting still sets the payoff flag even at 0 power")
	}
}

// (e) The added power is a snapshot at enlist time, not a continuous link:
// pumping the enlistee afterward does NOT change the bonus already granted.
func TestApplyEnlist_PowerIsSnapshot(t *testing.T) {
	gs := newEnlistGame(t)
	attacker := enlistCreature(gs, 0, "Aradesh", 3)
	attacker.Flags["attacking"] = 1
	helper := enlistCreature(gs, 0, "Helper", 2)

	ApplyEnlist(gs, 0, attacker, helper)
	if got := attacker.Power(); got != 5 {
		t.Fatalf("setup: attacker should be 3+2=5, got %d", got)
	}
	// Pump the enlistee after the fact.
	helper.AddCounter("+1/+1", 3)
	gs.InvalidateCharacteristicsCache()
	if got := attacker.Power(); got != 5 {
		t.Errorf("(e) enlist bonus is a snapshot; later pumping the enlistee must not change it. attacker=%d, want 5", got)
	}
}

// (e) The +power bonus is "until end of turn" and clears in the §514.2 cleanup.
func TestApplyEnlist_BonusClearsAtEndOfTurn(t *testing.T) {
	gs := newEnlistGame(t)
	gs.Active = 0
	attacker := enlistCreature(gs, 0, "Aradesh", 3)
	attacker.Flags["attacking"] = 1
	helper := enlistCreature(gs, 0, "Helper", 4)

	ApplyEnlist(gs, 0, attacker, helper)
	if attacker.Power() != 7 {
		t.Fatalf("setup: expected 7, got %d", attacker.Power())
	}
	ScanExpiredDurations(gs, "ending", "cleanup")
	if got := attacker.Power(); got != 3 {
		t.Errorf("(e) enlist +power must clear at end of turn; attacker=%d, want 3", got)
	}
}
