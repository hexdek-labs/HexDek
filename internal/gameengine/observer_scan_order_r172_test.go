package gameengine

import "testing"

// r172 — the reorderings in IsCreature, FireEvolveTriggers and
// FireSoulbondTriggers are behaviour-preserving by construction, so most
// of their safety is carried by the EXISTING evolve/soulbond suites
// (keywords_flagged_test.go, keyword_migration_batch9_r85_test.go),
// which pass at HEAD and must keep passing.
//
// What follows are the surfaces the reordering specifically puts at
// risk: cases where the cheap test and the expensive test disagree, and
// where the OLD order would have short-circuited before ever asking the
// question the NEW order asks first.

// r172Suppressed builds a permanent whose type line says "creature" but
// whose current state says otherwise — the only shape where hasType and
// permSuppressesCreatureType disagree, and therefore the only shape the
// IsCreature reorder could possibly break.
func r172Suppressed(gs *GameState, seat int, name, flag string) *Permanent {
	p := addBattlefield(gs, seat, name, 2, 2, "creature", "enchantment")
	p.Flags[flag] = 1
	return p
}

func TestR172_IsCreature_SuppressionStillWinsOverTypeLine(t *testing.T) {
	gs := newFixtureGame(t)
	for _, flag := range []string{"bestowed", "reconfigured", "not_creature_while_impending"} {
		t.Run(flag, func(t *testing.T) {
			p := r172Suppressed(gs, 0, "Suppressed "+flag, flag)
			if !p.hasType("creature") {
				t.Fatalf("precondition: the type line must say creature, or this "+
					"test proves nothing about the reorder (flag %s)", flag)
			}
			if p.IsCreature() {
				t.Errorf("CR 702.103a/702.151b/702.176 — %s removes the creature "+
					"type, but IsCreature said true; the cheap type test must not "+
					"be allowed to answer on its own", flag)
			}
		})
	}
}

func TestR172_IsCreature_PlainCasesUnchanged(t *testing.T) {
	gs := newFixtureGame(t)
	cre := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	land := addBattlefield(gs, 0, "Forest", 0, 0, "land")
	if !cre.IsCreature() {
		t.Error("a plain creature must still read as a creature")
	}
	if land.IsCreature() {
		t.Error("a land must not read as a creature")
	}
}

// The evolve filter now asks "does this have evolve?" BEFORE "is this a
// creature?". A non-creature carrying the keyword is the case that
// ordering change exposes: the old code never reached the keyword test.
func TestR172_Evolve_KeywordOnNonCreatureStillSkipped(t *testing.T) {
	gs := newFixtureGame(t)
	bystander := addBattlefield(gs, 0, "Evolving Relic", 0, 0, "artifact")
	bystander.Flags["kw:evolve"] = 1
	gs.InvalidateCharacteristicsCache()

	entering := addBattlefield(gs, 0, "Big Arrival", 7, 7, "creature")
	FireEvolveTriggers(gs, 0, entering)

	if bystander.Counters["+1/+1"] != 0 {
		t.Errorf("CR 702.100 — evolve is a creature ability; a noncreature "+
			"permanent carrying the keyword must not evolve, got %d counters",
			bystander.Counters["+1/+1"])
	}
}

// A creature whose creature-ness is SUPPRESSED must also be skipped, and
// this is the sharper version: hasType says creature, the flag says not.
func TestR172_Evolve_SuppressedCreatureStillSkipped(t *testing.T) {
	gs := newFixtureGame(t)
	bystander := r172Suppressed(gs, 0, "Bestowed Evolver", "bestowed")
	bystander.Flags["kw:evolve"] = 1
	gs.InvalidateCharacteristicsCache()

	entering := addBattlefield(gs, 0, "Big Arrival", 7, 7, "creature")
	FireEvolveTriggers(gs, 0, entering)

	if bystander.Counters["+1/+1"] != 0 {
		t.Errorf("a bestowed Aura is not a creature (CR 702.103a) and must not "+
			"evolve, got %d counters", bystander.Counters["+1/+1"])
	}
}

// The positive direction, so the two tests above cannot pass vacuously by
// the scan simply never firing.
func TestR172_Evolve_StillFiresForARealEvolveCreature(t *testing.T) {
	gs := newFixtureGame(t)
	evolver := addBattlefield(gs, 0, "Experiment One", 1, 1, "creature")
	evolver.Flags["kw:evolve"] = 1
	gs.InvalidateCharacteristicsCache()

	entering := addBattlefield(gs, 0, "Big Arrival", 7, 7, "creature")
	FireEvolveTriggers(gs, 0, entering)

	if evolver.Counters["+1/+1"] != 1 {
		t.Fatalf("CR 702.100b — a 7/7 entering beside a 1/1 evolve creature must "+
			"place exactly one +1/+1 counter, got %d", evolver.Counters["+1/+1"])
	}
}

func TestR172_Soulbond_KeywordOnNonCreatureStillSkipped(t *testing.T) {
	gs := newFixtureGame(t)
	bystander := addBattlefield(gs, 0, "Bonding Relic", 0, 0, "artifact")
	bystander.Flags["kw:soulbond"] = 1
	gs.InvalidateCharacteristicsCache()

	mate := addBattlefield(gs, 0, "Vanilla Mate", 2, 2, "creature")
	FireSoulbondTriggers(gs, 0, mate)

	if IsPaired(bystander) || IsPaired(mate) {
		t.Error("CR 702.95a — soulbond pairs CREATURES; a noncreature carrying " +
			"the keyword must not pair")
	}
}

func TestR172_Soulbond_SuppressedCreatureStillSkipped(t *testing.T) {
	gs := newFixtureGame(t)
	bystander := r172Suppressed(gs, 0, "Bestowed Bonder", "bestowed")
	bystander.Flags["kw:soulbond"] = 1
	gs.InvalidateCharacteristicsCache()

	mate := addBattlefield(gs, 0, "Vanilla Mate", 2, 2, "creature")
	FireSoulbondTriggers(gs, 0, mate)

	if IsPaired(bystander) || IsPaired(mate) {
		t.Error("a bestowed Aura is not a creature (CR 702.103a) and must not pair")
	}
}

func TestR172_Soulbond_StillPairsARealSoulbondCreature(t *testing.T) {
	gs := newFixtureGame(t)
	bonder := addBattlefield(gs, 0, "Trusted Forcemage", 3, 3, "creature")
	bonder.Flags["kw:soulbond"] = 1
	gs.InvalidateCharacteristicsCache()

	mate := addBattlefield(gs, 0, "Vanilla Mate", 2, 2, "creature")
	FireSoulbondTriggers(gs, 0, mate)

	if !IsPaired(bonder) || !IsPaired(mate) {
		t.Fatalf("CR 702.95a — an unpaired soulbond creature must pair with the "+
			"entering creature (bonder paired=%v, mate paired=%v)",
			IsPaired(bonder), IsPaired(mate))
	}
}

// The IsPaired(newCreature) break moved to the top of the loop body. With
// TWO eligible soulbond creatures the entering creature must still pair
// with exactly one of them — this is what the relocated break guards.
func TestR172_Soulbond_EnteringCreaturePairsWithExactlyOne(t *testing.T) {
	gs := newFixtureGame(t)
	var bonders []*Permanent
	for _, n := range []string{"Bonder A", "Bonder B", "Bonder C"} {
		b := addBattlefield(gs, 0, n, 3, 3, "creature")
		b.Flags["kw:soulbond"] = 1
		bonders = append(bonders, b)
	}
	gs.InvalidateCharacteristicsCache()

	mate := addBattlefield(gs, 0, "Vanilla Mate", 2, 2, "creature")
	FireSoulbondTriggers(gs, 0, mate)

	paired := 0
	for _, b := range bonders {
		if IsPaired(b) {
			paired++
		}
	}
	if !IsPaired(mate) {
		t.Fatal("the entering creature should have paired with one of three bonders")
	}
	if paired != 1 {
		t.Errorf("CR 702.95a — a creature pairs with exactly ONE other; %d of the "+
			"three bonders paired with a single entering creature", paired)
	}
}

// The soulbond loop now evaluates IsPaired(newCreature) before the nil
// guard on p. A nil battlefield entry must still be survivable.
func TestR172_Soulbond_NilBattlefieldEntryIsSurvivable(t *testing.T) {
	gs := newFixtureGame(t)
	bonder := addBattlefield(gs, 0, "Trusted Forcemage", 3, 3, "creature")
	bonder.Flags["kw:soulbond"] = 1
	gs.InvalidateCharacteristicsCache()
	mate := addBattlefield(gs, 0, "Vanilla Mate", 2, 2, "creature")

	// splice a nil in ahead of the bonder
	bf := gs.Seats[0].Battlefield
	gs.Seats[0].Battlefield = append([]*Permanent{nil}, bf...)

	FireSoulbondTriggers(gs, 0, mate)

	if !IsPaired(bonder) || !IsPaired(mate) {
		t.Error("a nil battlefield entry must be skipped, not abort the scan")
	}
}
