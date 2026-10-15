package gameengine

import "testing"

// r112 — semantic cover for the sba704_5y O(n^2) rewrite.
//
// The rewrite indexes Roles once instead of rescanning every seat's whole
// battlefield per permanent. TestSBA_704_5y_RoleUniquenessKeepsNewest
// already pins the same-controller case; these pin the directions it does
// not, which are exactly the ones an indexing mistake would break:
//
//	- Roles controlled by DIFFERENT players must BOTH survive. CR §704.5z
//	  (the rule this function implements — see the citation note in the
//	  r112 write-up) says "more than one Role controlled by the SAME player".
//	  Grouping by holder without re-splitting by controller would kill one.
//	- Roles on DIFFERENT holders must all survive. A holder-indexing bug
//	  that bucketed every Role together would kill all but one.
//	- A board with no Roles must take the early return and change nothing.

func r112Role(gs *GameState, seat int, name string, holder *Permanent) *Permanent {
	r := addBattlefield(gs, seat, name, 0, 0, "enchantment", "aura", "role")
	r.AttachedTo = holder
	return r
}

func r112OnBattlefield(gs *GameState, want *Permanent) bool {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == want {
				return true
			}
		}
	}
	return false
}

func TestRoleUniqueness_DifferentControllersBothSurvive_r112(t *testing.T) {
	gs := newFixtureGame(t)
	holder := addBattlefield(gs, 0, "r112 Bear", 2, 2, "creature")
	mine := r112Role(gs, 0, "r112 Cursed Role", holder)
	theirs := r112Role(gs, 1, "r112 Monster Role", holder)

	StateBasedActions(gs)

	if !r112OnBattlefield(gs, mine) {
		t.Error("seat 0's Role died — §704.5z only removes Roles controlled by the SAME player")
	}
	if !r112OnBattlefield(gs, theirs) {
		t.Error("seat 1's Role died — §704.5z only removes Roles controlled by the SAME player")
	}
}

func TestRoleUniqueness_DifferentHoldersAllSurvive_r112(t *testing.T) {
	gs := newFixtureGame(t)
	h1 := addBattlefield(gs, 0, "r112 Bear One", 2, 2, "creature")
	h2 := addBattlefield(gs, 0, "r112 Bear Two", 2, 2, "creature")
	r1 := r112Role(gs, 0, "r112 Role A", h1)
	r2 := r112Role(gs, 0, "r112 Role B", h2)

	StateBasedActions(gs)

	if !r112OnBattlefield(gs, r1) {
		t.Error("Role on holder 1 died — it is the only Role on that permanent")
	}
	if !r112OnBattlefield(gs, r2) {
		t.Error("Role on holder 2 died — it is the only Role on that permanent")
	}
}

// The early return. STAMP: assert the board is untouched, not merely that
// nothing crashed — "no Roles" is the overwhelmingly common production case
// and is the whole reason the rewrite matters.
func TestRoleUniqueness_NoRolesChangesNothing_r112(t *testing.T) {
	gs := newFixtureGame(t)
	a := addBattlefield(gs, 0, "r112 Plain Bear", 2, 2, "creature")
	b := addBattlefield(gs, 1, "r112 Plain Ox", 3, 3, "creature")

	StateBasedActions(gs)

	if !r112OnBattlefield(gs, a) || !r112OnBattlefield(gs, b) {
		t.Fatal("a board with no Roles lost a permanent to the Role SBA")
	}
	if got := countEvents(gs, "sba_704_5y"); got != 0 {
		t.Errorf("Role SBA fired %d times on a board with no Roles", got)
	}
}

// Same controller, THREE Roles: only the newest survives. Guards the
// destroyed-set bookkeeping the rewrite added.
func TestRoleUniqueness_ThreeSameControllerKeepsNewest_r112(t *testing.T) {
	gs := newFixtureGame(t)
	holder := addBattlefield(gs, 0, "r112 Bear", 2, 2, "creature")
	r1 := r112Role(gs, 0, "r112 Role One", holder)
	r2 := r112Role(gs, 0, "r112 Role Two", holder)
	r3 := r112Role(gs, 0, "r112 Role Three", holder)
	if !(r3.Timestamp > r2.Timestamp && r2.Timestamp > r1.Timestamp) {
		t.Fatal("fixture: timestamps must be increasing")
	}

	StateBasedActions(gs)

	if r112OnBattlefield(gs, r1) {
		t.Error("oldest Role survived")
	}
	if r112OnBattlefield(gs, r2) {
		t.Error("middle Role survived")
	}
	if !r112OnBattlefield(gs, r3) {
		t.Error("newest Role died — §704.5z keeps the most recent timestamp")
	}
}
