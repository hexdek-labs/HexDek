package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/gameengine/counters"
)

// Regression tests for the CR 614.6 opponent-creature-would-die → exile
// replacements (Kalitas, Liesa, Ravenous Slime).

// addCreature places a creature permanent with explicit P/T.
func addCreature(gs *gameengine.GameState, seat int, name string, pow, tough int, types ...string) *gameengine.Permanent {
	p := addPerm(gs, seat, name, append([]string{"creature"}, types...)...)
	p.Card.BasePower = pow
	p.Card.BaseToughness = tough
	return p
}

func countZombies(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Zombie" {
			n++
		}
	}
	return n
}

func killViaSBA(gs *gameengine.GameState, victim *gameengine.Permanent) {
	victim.MarkedDamage = 999
	gameengine.StateBasedActions(gs)
}

func TestZoneRepl_Kalitas_OppCreatureExiledAndZombie(t *testing.T) {
	gs := newGame(t, 2)
	kalitas := addCreature(gs, 0, "Kalitas, Traitor of Ghet", 3, 4, "legendary")
	gameengine.InvokeETBHook(gs, kalitas)
	victim := addCreature(gs, 1, "Grizzly Bears", 2, 2)

	killViaSBA(gs, victim)

	if len(gs.Seats[1].Exile) != 1 {
		t.Fatalf("opp creature should be exiled; exile=%d gy=%d",
			len(gs.Seats[1].Exile), len(gs.Seats[1].Graveyard))
	}
	if len(gs.Seats[1].Graveyard) != 0 {
		t.Errorf("opp creature should NOT be in graveyard; gy=%d", len(gs.Seats[1].Graveyard))
	}
	if z := countZombies(gs, 0); z != 1 {
		t.Errorf("expected exactly 1 Zombie token (applies-once), got %d", z)
	}
}

func TestZoneRepl_Kalitas_OwnCreatureStaysGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	kalitas := addCreature(gs, 0, "Kalitas, Traitor of Ghet", 3, 4, "legendary")
	gameengine.InvokeETBHook(gs, kalitas)
	victim := addCreature(gs, 0, "Grizzly Bears", 2, 2)

	killViaSBA(gs, victim)

	if len(gs.Seats[0].Graveyard) != 1 {
		t.Fatalf("own creature should go to graveyard; gy=%d exile=%d",
			len(gs.Seats[0].Graveyard), len(gs.Seats[0].Exile))
	}
	if z := countZombies(gs, 0); z != 0 {
		t.Errorf("Kalitas must not fire on own creature; zombies=%d", z)
	}
}

func TestZoneRepl_Kalitas_OppTokenNotExiled(t *testing.T) {
	gs := newGame(t, 2)
	kalitas := addCreature(gs, 0, "Kalitas, Traitor of Ghet", 3, 4, "legendary")
	gameengine.InvokeETBHook(gs, kalitas)
	// A token opponent creature — Kalitas only hits NONtoken creatures.
	victim := addCreature(gs, 1, "Saproling", 1, 1)
	victim.Card.Types = append(victim.Card.Types, "token")

	killViaSBA(gs, victim)

	if z := countZombies(gs, 0); z != 0 {
		t.Errorf("Kalitas must not fire on a token; zombies=%d", z)
	}
}

func TestZoneRepl_Liesa_OppCreatureExiled(t *testing.T) {
	gs := newGame(t, 2)
	liesa := addCreature(gs, 0, "Liesa, Forgotten Archangel", 3, 3, "legendary")
	gameengine.InvokeETBHook(gs, liesa)
	victim := addCreature(gs, 1, "Grizzly Bears", 2, 2)

	killViaSBA(gs, victim)

	if len(gs.Seats[1].Exile) != 1 || len(gs.Seats[1].Graveyard) != 0 {
		t.Fatalf("opp creature should exile; exile=%d gy=%d",
			len(gs.Seats[1].Exile), len(gs.Seats[1].Graveyard))
	}
}

func TestZoneRepl_Liesa_OwnCreatureStaysGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	liesa := addCreature(gs, 0, "Liesa, Forgotten Archangel", 3, 3, "legendary")
	gameengine.InvokeETBHook(gs, liesa)
	victim := addCreature(gs, 0, "Grizzly Bears", 2, 2)

	killViaSBA(gs, victim)

	if len(gs.Seats[0].Graveyard) != 1 {
		t.Fatalf("own creature should stay in graveyard; gy=%d exile=%d",
			len(gs.Seats[0].Graveyard), len(gs.Seats[0].Exile))
	}
}

func TestZoneRepl_RavenousSlime_ExileAndGrow(t *testing.T) {
	gs := newGame(t, 2)
	slime := addCreature(gs, 0, "Ravenous Slime", 1, 1)
	gameengine.InvokeETBHook(gs, slime)
	victim := addCreature(gs, 1, "Hill Giant", 3, 3)

	killViaSBA(gs, victim)

	if len(gs.Seats[1].Exile) != 1 {
		t.Fatalf("opp creature should exile; exile=%d gy=%d",
			len(gs.Seats[1].Exile), len(gs.Seats[1].Graveyard))
	}
	// Ravenous Slime gains +1/+1 counters equal to the dying creature's
	// power (3). It was 1/1, so it should now be 4/4.
	if got := counters.CounterCount(slime.AsCounterTarget(), "+1/+1"); got != 3 {
		t.Errorf("expected 3 +1/+1 counters on Ravenous Slime, got %d", got)
	}
}

// CR 614.6 — co-present Rest in Peace + Kalitas: the death is redirected
// to exile exactly once; at most one of the two riders resolves (here we
// pin that the creature lands in exile and Kalitas never makes more than
// one zombie regardless of which replacement the player applies first).
func TestZoneRepl_Kalitas_WithRestInPeace_NoDoubleRedirect(t *testing.T) {
	gs := newGame(t, 2)
	kalitas := addCreature(gs, 0, "Kalitas, Traitor of Ghet", 3, 4, "legendary")
	gameengine.InvokeETBHook(gs, kalitas)
	rip := addPerm(gs, 0, "Rest in Peace", "enchantment")
	gameengine.RegisterReplacementsForPermanent(gs, rip)
	victim := addCreature(gs, 1, "Grizzly Bears", 2, 2)

	killViaSBA(gs, victim)

	if len(gs.Seats[1].Exile) != 1 || len(gs.Seats[1].Graveyard) != 0 {
		t.Fatalf("creature should be exiled exactly once; exile=%d gy=%d",
			len(gs.Seats[1].Exile), len(gs.Seats[1].Graveyard))
	}
	if z := countZombies(gs, 0); z > 1 {
		t.Errorf("at most one Zombie may be made; got %d", z)
	}
}
