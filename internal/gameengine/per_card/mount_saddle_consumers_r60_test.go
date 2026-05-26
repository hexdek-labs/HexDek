package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// mount_saddle_consumers_r60 — 7 handlers wired in
// mount_saddle_consumers_r60.go covering the OTJ Mount/Saddle payoff
// family (CR §702.171). One card fires on the engine's `mount_saddled`
// event (Stubborn Burrowfiend); the other six fire on the `attack`
// event and gate on PermIsSaddled inside the handler.
// -----------------------------------------------------------------------------

// mountSaddledCtx builds the ctx FireMountSaddledTriggers forwards.
func mountSaddledCtx(mount *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"mount":      mount,
		"controller": mount.Controller,
	}
}

// attackCtx builds the ctx combat.go forwards for `creature_attacks`
// (aliased to `attack`).
func attackCtx(atk *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"attacker_perm": atk,
		"attacker_seat": atk.Controller,
		"attacker_card": atk.Card,
	}
}

// saddleMount marks a mount as saddled until end of turn, the same way
// the engine's SaddleMount / ActivateSaddle paths do.
func saddleMount(p *gameengine.Permanent) {
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags["saddled"] = 1
}

// -----------------------------------------------------------------------------
// Stubborn Burrowfiend
// -----------------------------------------------------------------------------

func TestStubbornBurrowfiend_MillsAndPumpsOnBecomesSaddled(t *testing.T) {
	gs := newGame(t, 2)
	fiend := addPerm(gs, 0, "Stubborn Burrowfiend", "creature", "mount")
	fiend.Card.BasePower, fiend.Card.BaseToughness = 3, 3
	addLibrary(gs, 0, "MillA", "MillB", "Stay")
	// Pre-seed graveyard with 2 creature cards so X = 2 after milling 2
	// non-creature cards from library.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Old Bear", Owner: 0, Types: []string{"creature"}},
		&gameengine.Card{Name: "Old Wolf", Owner: 0, Types: []string{"creature"}},
	)

	saddleMount(fiend)
	gameengine.FireCardTrigger(gs, "mount_saddled", mountSaddledCtx(fiend))

	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected 2 cards milled (library 3→1), got %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Graveyard) != 4 {
		t.Errorf("expected graveyard 2→4 after milling 2, got %d", len(gs.Seats[0].Graveyard))
	}
	// X = creature cards in graveyard AFTER milling. The mill places 2
	// non-creature cards (MillA, MillB) into the graveyard, so X still =
	// 2 (the pre-seeded creatures).
	if fiend.Power() != 3+2 || fiend.Toughness() != 3+2 {
		t.Errorf("expected +2/+2 pump (X=2 creatures in graveyard); got %d/%d", fiend.Power(), fiend.Toughness())
	}
}

func TestStubbornBurrowfiend_FirstTimeOnlyPerTurn(t *testing.T) {
	gs := newGame(t, 2)
	fiend := addPerm(gs, 0, "Stubborn Burrowfiend", "creature", "mount")
	fiend.Card.BasePower, fiend.Card.BaseToughness = 3, 3
	addLibrary(gs, 0, "A", "B", "C", "D", "E")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Old Bear", Owner: 0, Types: []string{"creature"}},
	)

	saddleMount(fiend)
	// First fire — should mill 2.
	gameengine.FireCardTrigger(gs, "mount_saddled", mountSaddledCtx(fiend))
	libAfterFirst := len(gs.Seats[0].Library)

	// Second fire same turn — must NO-op (oracle: "for the first time
	// each turn").
	gameengine.FireCardTrigger(gs, "mount_saddled", mountSaddledCtx(fiend))
	if len(gs.Seats[0].Library) != libAfterFirst {
		t.Errorf("second mount_saddled this turn must no-op; library went %d→%d",
			libAfterFirst, len(gs.Seats[0].Library))
	}
}

func TestStubbornBurrowfiend_OtherMountDoesNotTrigger(t *testing.T) {
	gs := newGame(t, 2)
	fiend := addPerm(gs, 0, "Stubborn Burrowfiend", "creature", "mount")
	fiend.Card.BasePower, fiend.Card.BaseToughness = 3, 3
	addLibrary(gs, 0, "A", "B")

	// A SEPARATE mount becomes saddled (e.g. Alacrian Jaguar). Burrowfiend's
	// trigger must not fire — its "this creature" filter excludes it.
	other := addPerm(gs, 0, "Alacrian Jaguar", "creature", "mount")
	other.Card.BasePower, other.Card.BaseToughness = 3, 3
	saddleMount(other)
	gameengine.FireCardTrigger(gs, "mount_saddled", mountSaddledCtx(other))

	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("Burrowfiend should not mill on another mount's saddle; library=%d", len(gs.Seats[0].Library))
	}
}

// -----------------------------------------------------------------------------
// Alacrian Jaguar
// -----------------------------------------------------------------------------

func TestAlacrianJaguar_PumpsOnSaddledAttack(t *testing.T) {
	gs := newGame(t, 2)
	jaguar := addPerm(gs, 0, "Alacrian Jaguar", "creature", "mount")
	jaguar.Card.BasePower, jaguar.Card.BaseToughness = 3, 3
	saddleMount(jaguar)

	gameengine.FireCardTrigger(gs, "attack", attackCtx(jaguar))

	if jaguar.Power() != 5 || jaguar.Toughness() != 5 {
		t.Errorf("expected 5/5 after +2/+2 on saddled attack, got %d/%d", jaguar.Power(), jaguar.Toughness())
	}
}

func TestAlacrianJaguar_NoPumpWhenNotSaddled(t *testing.T) {
	gs := newGame(t, 2)
	jaguar := addPerm(gs, 0, "Alacrian Jaguar", "creature", "mount")
	jaguar.Card.BasePower, jaguar.Card.BaseToughness = 3, 3
	// NOT saddled.

	gameengine.FireCardTrigger(gs, "attack", attackCtx(jaguar))

	if jaguar.Power() != 3 || jaguar.Toughness() != 3 {
		t.Errorf("unsaddled attack must not pump; got %d/%d", jaguar.Power(), jaguar.Toughness())
	}
}

func TestAlacrianJaguar_IgnoresAllyAttack(t *testing.T) {
	gs := newGame(t, 2)
	jaguar := addPerm(gs, 0, "Alacrian Jaguar", "creature", "mount")
	jaguar.Card.BasePower, jaguar.Card.BaseToughness = 3, 3
	saddleMount(jaguar)

	// Some OTHER ally attacks. Jaguar's "this creature attacks" filter
	// must not fire on a sibling.
	ally := addPerm(gs, 0, "Llanowar Elves", "creature")
	ally.Card.BasePower, ally.Card.BaseToughness = 1, 1
	gameengine.FireCardTrigger(gs, "attack", attackCtx(ally))

	if jaguar.Power() != 3 {
		t.Errorf("Jaguar must not self-pump on ally attack; got power %d", jaguar.Power())
	}
}

// -----------------------------------------------------------------------------
// Brightfield Mustang
// -----------------------------------------------------------------------------

func TestBrightfieldMustang_UntapsAndCountersOnSaddledAttack(t *testing.T) {
	gs := newGame(t, 2)
	mustang := addPerm(gs, 0, "Brightfield Mustang", "creature", "mount")
	mustang.Card.BasePower, mustang.Card.BaseToughness = 4, 4
	mustang.Tapped = true // attacked, so the engine tapped it pre-trigger
	saddleMount(mustang)

	gameengine.FireCardTrigger(gs, "attack", attackCtx(mustang))

	if mustang.Tapped {
		t.Errorf("Mustang should be untapped post-trigger")
	}
	if mustang.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 +1/+1 counter, got %d", mustang.Counters["+1/+1"])
	}
}

// -----------------------------------------------------------------------------
// Gilded Ghoda
// -----------------------------------------------------------------------------

func TestGildedGhoda_CreatesTreasureOnSaddledAttack(t *testing.T) {
	gs := newGame(t, 2)
	ghoda := addPerm(gs, 0, "Gilded Ghoda", "creature", "mount")
	ghoda.Card.BasePower, ghoda.Card.BaseToughness = 2, 2
	saddleMount(ghoda)
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "attack", attackCtx(ghoda))

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected battlefield +1 (Treasure token), pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, ty := range p.Card.Types {
			if ty == "treasure" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected a Treasure token on battlefield")
	}
}

// -----------------------------------------------------------------------------
// Venomsac Lagac
// -----------------------------------------------------------------------------

func TestVenomsacLagac_GainsToughnessOnSaddledAttack(t *testing.T) {
	gs := newGame(t, 2)
	lagac := addPerm(gs, 0, "Venomsac Lagac", "creature", "mount")
	lagac.Card.BasePower, lagac.Card.BaseToughness = 2, 2
	saddleMount(lagac)

	gameengine.FireCardTrigger(gs, "attack", attackCtx(lagac))

	if lagac.Power() != 2 {
		t.Errorf("expected unchanged power 2, got %d", lagac.Power())
	}
	if lagac.Toughness() != 5 {
		t.Errorf("expected +0/+3 → toughness 5, got %d", lagac.Toughness())
	}
}

// -----------------------------------------------------------------------------
// Bridled Bighorn
// -----------------------------------------------------------------------------

func TestBridledBighorn_CreatesSheepOnSaddledAttack(t *testing.T) {
	gs := newGame(t, 2)
	bighorn := addPerm(gs, 0, "Bridled Bighorn", "creature", "mount")
	bighorn.Card.BasePower, bighorn.Card.BaseToughness = 3, 4
	saddleMount(bighorn)
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "attack", attackCtx(bighorn))

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected battlefield +1 (Sheep token), pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	foundSheep := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Sheep" && p.Card.BasePower == 1 && p.Card.BaseToughness == 1 {
			foundSheep = true
			break
		}
	}
	if !foundSheep {
		t.Errorf("expected a 1/1 Sheep token; battlefield=%v", battlefieldNames(gs.Seats[0]))
	}
}

// -----------------------------------------------------------------------------
// District Mascot
// -----------------------------------------------------------------------------

func TestDistrictMascot_AddsCounterOnSaddledAttack(t *testing.T) {
	gs := newGame(t, 2)
	mascot := addPerm(gs, 0, "District Mascot", "creature", "mount")
	mascot.Card.BasePower, mascot.Card.BaseToughness = 1, 1
	mascot.AddCounter("+1/+1", 1) // simulating the ETB counter
	saddleMount(mascot)

	gameengine.FireCardTrigger(gs, "attack", attackCtx(mascot))

	if mascot.Counters["+1/+1"] != 2 {
		t.Errorf("expected 2 +1/+1 counters (ETB 1 + saddled-attack 1), got %d", mascot.Counters["+1/+1"])
	}
}

func TestDistrictMascot_DoesNothingUnsaddled(t *testing.T) {
	gs := newGame(t, 2)
	mascot := addPerm(gs, 0, "District Mascot", "creature", "mount")
	mascot.Card.BasePower, mascot.Card.BaseToughness = 1, 1
	mascot.AddCounter("+1/+1", 1) // ETB counter only

	gameengine.FireCardTrigger(gs, "attack", attackCtx(mascot))

	if mascot.Counters["+1/+1"] != 1 {
		t.Errorf("unsaddled attack must not add counter; got %d", mascot.Counters["+1/+1"])
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — all 7 handlers reachable.
// -----------------------------------------------------------------------------

func TestMountSaddleConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Stubborn Burrowfiend",
		"Alacrian Jaguar",
		"Brightfield Mustang",
		"Gilded Ghoda",
		"Venomsac Lagac",
		"Bridled Bighorn",
		"District Mascot",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler registered for %s", name)
		}
	}
}

// -----------------------------------------------------------------------------
// Test helper
// -----------------------------------------------------------------------------

func battlefieldNames(seat *gameengine.Seat) []string {
	out := []string{}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		out = append(out, p.Card.DisplayName())
	}
	return out
}
