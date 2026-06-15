package gameengine

import "testing"

// exert_behavior_r63_test.go — behavioral regressions for Exert (CR §702.112).
// Exert was entirely unimplemented before r63 (only an event alias existed).
// These pin the new HasExert / ExertCreature primitive + the one-shot
// skip-next-untap consumption in UntapAll.

func attackingTappedCreature(gs *GameState, seat int, name string) *Permanent {
	p := addKWCombatBattlefield(gs, seat, name, 2, 2, "creature")
	p.Tapped = true
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags["attacking"] = 1
	return p
}

func exertEventCount(gs *GameState) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "exert" {
			n++
		}
	}
	return n
}

// (a) Only an ATTACKING creature can be exerted; exert is opt-in.
func TestExert_OnlyAttackingCreatures(t *testing.T) {
	gs := newKWCombatGame(t)
	c := addKWCombatBattlefield(gs, 0, "Glory-Bound Initiate", 2, 2, "creature")
	c.Tapped = true // tapped but NOT attacking

	if ExertCreature(gs, c) {
		t.Error("(a) a non-attacking creature must not be exertable")
	}

	c.Flags["attacking"] = 1
	if !ExertCreature(gs, c) {
		t.Error("(a) an attacking creature should be exertable")
	}
}

// (b)+(d) An exerted creature does NOT untap during the next untap step (one
// skip), and untaps normally on the following untap step; the flag is consumed
// by exactly one untap.
func TestExert_SkipsExactlyOneUntap(t *testing.T) {
	gs := newKWCombatGame(t)
	c := attackingTappedCreature(gs, 0, "Combat Celebrant")

	if !ExertCreature(gs, c) {
		t.Fatal("setup: exert should succeed on an attacking creature")
	}

	// Next untap step — exerted creature stays tapped.
	UntapAll(gs, 0)
	if !c.Tapped {
		t.Error("(b) an exerted creature must NOT untap during its next untap step")
	}
	if c.Flags["exert_skip_next_untap"] != 0 {
		t.Error("(d) the one-shot exert skip flag must be consumed by that untap step")
	}

	// Following untap step — untaps normally.
	UntapAll(gs, 0)
	if c.Tapped {
		t.Error("(b) the creature must untap normally on the following untap step")
	}
}

// (c) The "when you exert" trigger fires only when actually exerted.
func TestExert_TriggerFiresOnlyWhenExerted(t *testing.T) {
	gs := newKWCombatGame(t)
	c := attackingTappedCreature(gs, 0, "Glory-Bound Initiate")

	ExertCreature(gs, c)
	if exertEventCount(gs) != 1 {
		t.Errorf("(c) exerting should fire exactly one exert trigger, got %d", exertEventCount(gs))
	}

	// Exerting again this combat must not re-fire (already exerted).
	if ExertCreature(gs, c) {
		t.Error("(c) a creature already exerted this combat should not be exerted again")
	}
	if exertEventCount(gs) != 1 {
		t.Errorf("(c) re-exert must not fire another trigger, got %d", exertEventCount(gs))
	}
}

// (f) Declined exert: a creature that is NOT exerted untaps normally next untap.
func TestExert_DeclinedUntapsNormally(t *testing.T) {
	gs := newKWCombatGame(t)
	c := attackingTappedCreature(gs, 0, "Vanilla Attacker")
	// Do NOT call ExertCreature — exert is optional.

	UntapAll(gs, 0)
	if c.Tapped {
		t.Error("(f) a non-exerted creature must untap normally")
	}
	if exertEventCount(gs) != 0 {
		t.Error("(f) declining exert must fire no exert trigger")
	}
}

// The permanent skip_untap (Basalt/Grim Monolith) is distinct from the
// one-shot exert skip and is NOT consumed by exert handling.
func TestExert_DoesNotDisturbPermanentSkipUntap(t *testing.T) {
	gs := newKWCombatGame(t)
	rock := addKWCombatBattlefield(gs, 0, "Basalt Monolith", 0, 0, "artifact")
	rock.Tapped = true
	rock.Flags["skip_untap"] = 1

	UntapAll(gs, 0)
	if !rock.Tapped {
		t.Error("permanent skip_untap must keep the rock tapped")
	}
	if rock.Flags["skip_untap"] != 1 {
		t.Error("permanent skip_untap must NOT be consumed (it never untaps on its own)")
	}
}
