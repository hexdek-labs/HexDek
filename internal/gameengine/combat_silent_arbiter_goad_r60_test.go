package gameengine

// r60 — declare-attackers ordering audit (CR §508 / §509.1d).
//
// Audit of DeclareAttackers found one ordering bug: Silent Arbiter
// trimmed `chosen` to its first entry AFTER enforceGoadMustAttack
// appended goaded creatures at the tail. A non-goaded creature could
// end up as the sole attacker while a goaded creature stayed home —
// silently violating CR §701.39 "the goaded creature must attack" and
// §509.1d (maximize requirements satisfied subject to the restriction).
//
// Fix: when Silent Arbiter is active and we trim to 1 attacker, prefer
// any goaded creature in `chosen`. The 1-attacker restriction is still
// honored; the requirement is now too.

import (
	"testing"
)

// markSilentArbiter puts a Silent Arbiter permanent on the given seat's
// battlefield and sets the flag the existing scanner reads.
func markSilentArbiter(gs *GameState, seat int) *Permanent {
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["silent_arbiter_active"] = 1
	arbiter := &Permanent{
		Card:       &Card{Name: "Silent Arbiter", Owner: seat, Types: []string{"creature"}},
		Controller: seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, arbiter)
	return arbiter
}

// TestDeclareAttackers_SilentArbiterPrefersGoaded asserts that when
// Silent Arbiter caps attackers at 1, the trimmer prefers a goaded
// creature over a non-goaded one — satisfying both the restriction
// (max 1 attacker) and the requirement (goaded must attack if able)
// per CR §509.1d.
func TestDeclareAttackers_SilentArbiterPrefersGoaded(t *testing.T) {
	gs := newCombatGame(t)
	gs.Turn = 5

	// Seat 0: two attackers. The vanilla one is added first so it lands
	// at chosen[0] under the default (no Hat) "all legal attackers"
	// path — pre-fix, this would be the lone surviving attacker.
	vanilla := addCreature(gs, 0, "Vanilla Bear", 2, 2)
	goaded := addCreature(gs, 0, "Goaded Ogre", 3, 3)
	GoadCreature(gs, 1 /*goader*/, goaded)
	if !IsGoaded(goaded, gs.Turn) {
		t.Fatalf("precondition: Goaded Ogre should register as goaded on turn 5")
	}
	if !MustAttackIfAble(gs, goaded) {
		t.Fatalf("precondition: goaded creature controlled by active seat must attack if able")
	}

	// Silent Arbiter on a defender's battlefield so the cap fires.
	markSilentArbiter(gs, 1)

	// Give seat 1 an opponent permanent presence (already alive — life 40
	// from NewGameState). DeclareAttackers needs at least one living
	// opponent.

	declared := DeclareAttackers(gs, 0)
	if len(declared) != 1 {
		t.Fatalf("Silent Arbiter should cap attackers at 1; got %d", len(declared))
	}
	if declared[0] != goaded {
		t.Errorf("Silent Arbiter trim should prefer the goaded creature (CR §509.1d); "+
			"chose %q instead of %q", declared[0].Card.DisplayName(), goaded.Card.DisplayName())
	}
	_ = vanilla
}

// TestDeclareAttackers_SilentArbiterNoGoad_PreservesFirst confirms the
// fix doesn't regress the no-goad case: when nothing in `chosen` is
// goaded, the first entry is still picked (matches the prior behavior
// for the common path).
func TestDeclareAttackers_SilentArbiterNoGoad_PreservesFirst(t *testing.T) {
	gs := newCombatGame(t)
	gs.Turn = 5

	first := addCreature(gs, 0, "First Bear", 2, 2)
	addCreature(gs, 0, "Second Bear", 2, 2)
	markSilentArbiter(gs, 1)

	declared := DeclareAttackers(gs, 0)
	if len(declared) != 1 {
		t.Fatalf("Silent Arbiter should cap attackers at 1; got %d", len(declared))
	}
	if declared[0] != first {
		t.Errorf("no-goad case: trim should keep the first entry; chose %q",
			declared[0].Card.DisplayName())
	}
}

// TestDeclareAttackers_SilentArbiterAllGoaded picks any of the goaded
// creatures — defends against over-narrowing the fix to "ONLY the
// goaded that was originally appended by enforceGoadMustAttack."
func TestDeclareAttackers_SilentArbiterAllGoaded(t *testing.T) {
	gs := newCombatGame(t)
	gs.Turn = 5

	g1 := addCreature(gs, 0, "Goaded A", 2, 2)
	g2 := addCreature(gs, 0, "Goaded B", 3, 3)
	GoadCreature(gs, 1, g1)
	GoadCreature(gs, 1, g2)
	markSilentArbiter(gs, 1)

	declared := DeclareAttackers(gs, 0)
	if len(declared) != 1 {
		t.Fatalf("Silent Arbiter should cap attackers at 1; got %d", len(declared))
	}
	picked := declared[0]
	if !MustAttackIfAble(gs, picked) {
		t.Errorf("all-goaded case: trim should pick a goaded creature; chose %q (must_attack=%v)",
			picked.Card.DisplayName(), MustAttackIfAble(gs, picked))
	}
}

// TestDeclareAttackers_SilentArbiterEventDetails asserts the
// attack_restricted event records the goad-preference decision so
// Heimdall can audit the call path.
func TestDeclareAttackers_SilentArbiterEventDetails(t *testing.T) {
	gs := newCombatGame(t)
	gs.Turn = 5

	addCreature(gs, 0, "Vanilla", 2, 2)
	goaded := addCreature(gs, 0, "Goaded", 3, 3)
	GoadCreature(gs, 1, goaded)
	markSilentArbiter(gs, 1)

	DeclareAttackers(gs, 0)

	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind != "attack_restricted" {
			continue
		}
		if ev.Details["reason"] != "silent_arbiter" {
			continue
		}
		if v, ok := ev.Details["goad_pref"].(bool); ok && v {
			if r, _ := ev.Details["rule"].(string); r == "509.1d" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("attack_restricted event should record goad_pref=true + rule=509.1d when Silent Arbiter kept a goaded creature")
	}
}
