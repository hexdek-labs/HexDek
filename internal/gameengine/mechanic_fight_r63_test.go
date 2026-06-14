package gameengine

// r63 — FIGHT (CR §701.12) + one-sided bite audit. Fight/bite damage is dealt
// BY the creature and is NOT combat damage, so it must route through the
// noncombat-damage path (deals-damage triggers, replacement/prevention,
// MarkedDamage for SBA) and apply the dealer's deathtouch and lifelink. The old
// resolveFight raw-incremented MarkedDamage, skipping lifelink + every
// deals-damage trigger.

import (
	"math/rand"
	"testing"
)

func fightGame() *GameState {
	gs := NewGameState(2, rand.New(rand.NewSource(9)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func fightCreature(gs *GameState, seat int, name string, pow, tough int, kws ...string) *Permanent {
	p := &Permanent{
		Card: &Card{Name: name, Owner: seat, Types: []string{"creature"},
			TypeLine: "Creature", BasePower: pow, BaseToughness: tough},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	for _, kw := range kws {
		p.Flags["kw:"+kw] = 1
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func fightOnBattlefield(gs *GameState, p *Permanent) bool {
	for _, s := range gs.Seats {
		for _, q := range s.Battlefield {
			if q == p {
				return true
			}
		}
	}
	return false
}

// (a) both fighters deal damage simultaneously (powers captured before damage):
// a 1/1 deathtouch vs a 5/5 — the 5/5 kills the 1/1 AND the 1/1's deathtouch
// kills the 5/5; both die.
func TestFightR63_Simultaneous(t *testing.T) {
	gs := fightGame()
	small := fightCreature(gs, 0, "Deathtoucher", 1, 1, "deathtouch")
	big := fightCreature(gs, 1, "Behemoth", 5, 5)
	// Mirror resolveFight: capture both powers, then deal both directions.
	aPow, bPow := small.Power(), big.Power()
	dealFightDamage(gs, small, big, aPow)
	dealFightDamage(gs, big, small, bPow)
	StateBasedActions(gs)
	if fightOnBattlefield(gs, small) {
		t.Fatalf("the 1/1 should die to the 5/5's fight damage")
	}
	if fightOnBattlefield(gs, big) {
		t.Fatalf("the 5/5 should die to the 1/1's deathtouch fight damage (simultaneous)")
	}
}

// (b) deathtouch makes sublethal fight damage lethal.
func TestFightR63_Deathtouch(t *testing.T) {
	gs := fightGame()
	dt := fightCreature(gs, 0, "Tracker", 1, 1, "deathtouch")
	victim := fightCreature(gs, 1, "Wall", 0, 6)
	dealFightDamage(gs, dt, victim, dt.Power())
	StateBasedActions(gs)
	if fightOnBattlefield(gs, victim) {
		t.Fatalf("deathtouch fight damage (1 to a 0/6) should be lethal via SBA")
	}
}

// (d) 0-power deals 0 damage (legal no-op): no damage marked, no lifelink.
func TestFightR63_ZeroPower(t *testing.T) {
	gs := fightGame()
	zero := fightCreature(gs, 0, "Wimp", 0, 3, "lifelink")
	target := fightCreature(gs, 1, "Bear", 2, 2)
	before := gs.Seats[0].Life
	dealFightDamage(gs, zero, target, zero.Power())
	if target.MarkedDamage != 0 {
		t.Fatalf("0-power should deal 0 damage; got marked %d", target.MarkedDamage)
	}
	if gs.Seats[0].Life != before {
		t.Fatalf("0-power lifelink should gain 0 life; life %d → %d", before, gs.Seats[0].Life)
	}
}

// (f) lifelink: the dealer's controller gains life equal to the damage dealt.
func TestFightR63_Lifelink(t *testing.T) {
	gs := fightGame()
	ll := fightCreature(gs, 0, "Lifelinker", 3, 3, "lifelink")
	target := fightCreature(gs, 1, "Ogre", 4, 4)
	before := gs.Seats[0].Life
	dealFightDamage(gs, ll, target, ll.Power())
	if gs.Seats[0].Life != before+3 {
		t.Fatalf("lifelink should gain 3 life from a 3-power fighter; life %d → %d", before, gs.Seats[0].Life)
	}
	if target.MarkedDamage != 3 {
		t.Fatalf("target should be marked 3; got %d", target.MarkedDamage)
	}
}

// (g) damage is dealt BY the creature through the noncombat-damage path — it
// logs a "damage" event attributed to the fighter (the raw-MarkedDamage path
// logged nothing), so deals-damage / damaged-by watchers can observe it.
func TestFightR63_DealsDamageThroughProperPath(t *testing.T) {
	gs := fightGame()
	a := fightCreature(gs, 0, "Striker", 2, 2)
	b := fightCreature(gs, 1, "Target", 3, 3)
	dealFightDamage(gs, a, b, a.Power())
	sawDamage := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "damage" && ev.Source == "Striker" {
			sawDamage = true
		}
	}
	if !sawDamage {
		t.Fatalf("fight damage must route through the damage path (a 'damage' event from the fighter); none logged")
	}
}
