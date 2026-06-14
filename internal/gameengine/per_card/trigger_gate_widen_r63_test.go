package per_card

// r63 — self-gate audit WIDENED beyond the attack-trigger family
// (95a8426d / ca938842) to the other per-permanent dispatch families.
//
// fireTrigger (registry.go) dispatches a per-permanent event by walking
// EVERY battlefield permanent and invoking each matching handler with
// ctx carrying the event SUBJECT (ctx["perm"] for etb/dies/ltb, or
// source_card name for combat damage). A handler implementing a
// self-referential "whenever THIS ..." ability must gate on the subject
// being itself, or it fires on every OTHER permanent's event.
//
// Each test below is NO-FIRE-ON-OTHER (the regression that pins the bug)
// paired with a FIRE-ON-SELF positive proving the harness is sensitive.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// onBattlefield / countByName are shared test helpers defined elsewhere in
// this package (replacement_orphans_r63_test.go, percard_cover_ac_r60_test.go).

// -----------------------------------------------------------------------------
// dies / LTB family — ctx["perm"] gate
// -----------------------------------------------------------------------------

// Banisher Priest (shared ltbReturnLinkedExile handler — also covers
// Oblivion Ring, Detention Sphere, Faceless Butcher) must NOT return its
// linked exile when a FOREIGN permanent leaves the battlefield.
func TestSelfGate_BanisherPriest_ForeignLTBDoesNotReturnExile(t *testing.T) {
	gs := newGame(t, 3)
	priest := stampCreaturePT(addPerm(gs, 0, "Banisher Priest", "creature"), 2, 2)
	victim := stampCreaturePT(addPerm(gs, 1, "Big Threat", "creature"), 5, 5)
	foreign := addPerm(gs, 0, "Foreign Token", "creature", "token")

	banisherPriestETB(gs, priest)
	if len(priest.LinkedExile) != 1 {
		t.Fatalf("setup: expected Banisher Priest to hold 1 linked exile, got %d", len(priest.LinkedExile))
	}
	if onBattlefield(gs, victim) {
		t.Fatalf("setup: victim should have been exiled off the battlefield")
	}

	// A foreign permanent leaves — must NOT pop the prisoner.
	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            foreign,
		"card":            foreign.Card,
		"controller_seat": foreign.Controller,
		"to_zone":         "graveyard",
	})
	if len(priest.LinkedExile) != 1 {
		t.Errorf("foreign LTB returned Banisher Priest's prisoner (self-gate missing): "+
			"LinkedExile %d, want 1", len(priest.LinkedExile))
	}

	// Banisher Priest itself leaves — NOW the prisoner returns.
	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            priest,
		"card":            priest.Card,
		"controller_seat": priest.Controller,
		"to_zone":         "graveyard",
	})
	if len(priest.LinkedExile) != 0 {
		t.Errorf("Banisher Priest's own LTB did not return the prisoner: LinkedExile %d, want 0",
			len(priest.LinkedExile))
	}
}

// Hostage Taker (hostageTakerLTB) — same shape, distinct handler.
func TestSelfGate_HostageTaker_ForeignLTBDoesNotReturnExile(t *testing.T) {
	gs := newGame(t, 3)
	taker := stampCreaturePT(addPerm(gs, 0, "Hostage Taker", "creature"), 2, 3)
	stampCreaturePT(addPerm(gs, 1, "Captured Beast", "creature"), 4, 4)
	foreign := addPerm(gs, 0, "Foreign Token", "creature", "token")

	hostageTakerETB(gs, taker)
	if len(taker.LinkedExile) != 1 {
		t.Fatalf("setup: expected Hostage Taker to hold 1 linked exile, got %d", len(taker.LinkedExile))
	}

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            foreign,
		"card":            foreign.Card,
		"controller_seat": foreign.Controller,
		"to_zone":         "graveyard",
	})
	if len(taker.LinkedExile) != 1 {
		t.Errorf("foreign LTB returned Hostage Taker's prisoner (self-gate missing): "+
			"LinkedExile %d, want 1", len(taker.LinkedExile))
	}

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            taker,
		"card":            taker.Card,
		"controller_seat": taker.Controller,
		"to_zone":         "graveyard",
	})
	if len(taker.LinkedExile) != 0 {
		t.Errorf("Hostage Taker's own LTB did not return the prisoner: LinkedExile %d, want 0",
			len(taker.LinkedExile))
	}
}

// Knowledge Pool — must not clear its exile-timestamp tags on a foreign LTB.
func TestSelfGate_KnowledgePool_ForeignLTBKeepsTags(t *testing.T) {
	gs := newGame(t, 3)
	kp := addPerm(gs, 0, "Knowledge Pool", "artifact")
	tagged := &gameengine.Card{Name: "Stashed Spell", Owner: 0, ExiledByTimestamp: kp.Timestamp}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, tagged)
	foreign := addPerm(gs, 0, "Foreign Token", "creature", "token")

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            foreign,
		"card":            foreign.Card,
		"controller_seat": foreign.Controller,
		"to_zone":         "graveyard",
	})
	if tagged.ExiledByTimestamp != kp.Timestamp {
		t.Errorf("foreign LTB cleared Knowledge Pool's exile tag (self-gate missing): "+
			"ExiledByTimestamp %d, want %d", tagged.ExiledByTimestamp, kp.Timestamp)
	}

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            kp,
		"card":            kp.Card,
		"controller_seat": kp.Controller,
		"to_zone":         "graveyard",
	})
	if tagged.ExiledByTimestamp != 0 {
		t.Errorf("Knowledge Pool's own LTB did not clear its exile tag: ExiledByTimestamp %d, want 0",
			tagged.ExiledByTimestamp)
	}
}

// Tombstone Stairwell — must not destroy its linked Tombspawn tokens on a
// foreign LTB.
func TestSelfGate_TombstoneStairwell_ForeignLTBKeepsTokens(t *testing.T) {
	gs := newGame(t, 3)
	stairwell := addPerm(gs, 0, "Tombstone Stairwell", "enchantment")
	tomb := stampCreaturePT(
		addPerm(gs, 0, "Tombspawn", "creature", "token", tombstoneStairwellLinkType(stairwell)), 2, 2)
	foreign := addPerm(gs, 0, "Foreign Token", "creature", "token")

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            foreign,
		"card":            foreign.Card,
		"controller_seat": foreign.Controller,
		"to_zone":         "graveyard",
	})
	if !onBattlefield(gs, tomb) {
		t.Errorf("foreign LTB destroyed Tombstone Stairwell's linked token (self-gate missing)")
	}

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            stairwell,
		"card":            stairwell.Card,
		"controller_seat": stairwell.Controller,
		"to_zone":         "graveyard",
	})
	if onBattlefield(gs, tomb) {
		t.Errorf("Tombstone Stairwell's own LTB did not destroy its linked token")
	}
}

// Gruff Triplets — "when THIS dies, +1/+1 = its power on each Gruff Triplets
// you control" must not buff siblings on a foreign creature's death.
func TestSelfGate_GruffTriplets_ForeignDeathDoesNotBuff(t *testing.T) {
	gs := newGame(t, 3)
	gruffA := stampCreaturePT(addPerm(gs, 0, "Gruff Triplets", "creature"), 4, 4)
	gruffB := stampCreaturePT(addPerm(gs, 0, "Gruff Triplets", "creature"), 4, 4)
	foreign := stampCreaturePT(addPerm(gs, 0, "Random Bear", "creature"), 2, 2)

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            foreign,
		"card":            foreign.Card,
		"controller_seat": foreign.Controller,
		"to_zone":         "graveyard",
	})
	if got := gruffB.Counters["+1/+1"]; got != 0 {
		t.Errorf("foreign creature death buffed Gruff Triplets sibling (self-gate missing): +1/+1 = %d, want 0", got)
	}

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            gruffA,
		"card":            gruffA.Card,
		"controller_seat": gruffA.Controller,
		"to_zone":         "graveyard",
	})
	if got := gruffB.Counters["+1/+1"]; got != 4 {
		t.Errorf("Gruff Triplets own death did not buff sibling: +1/+1 = %d, want 4", got)
	}
}

// Aerith Gainsborough — "when THIS dies, distribute its +1/+1 counters to each
// legendary you control" must not fire on a foreign creature's death.
func TestSelfGate_Aerith_ForeignDeathDoesNotDistribute(t *testing.T) {
	gs := newGame(t, 3)
	aerith := stampCreaturePT(addPerm(gs, 0, "Aerith Gainsborough", "creature", "legendary"), 1, 1)
	aerith.Counters["+1/+1"] = 3
	legend := stampCreaturePT(addPerm(gs, 0, "Some Legend", "creature", "legendary"), 2, 2)
	foreign := stampCreaturePT(addPerm(gs, 0, "Random Bear", "creature"), 2, 2)

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            foreign,
		"card":            foreign.Card,
		"controller_seat": foreign.Controller,
		"to_zone":         "graveyard",
	})
	if got := legend.Counters["+1/+1"]; got != 0 {
		t.Errorf("foreign death distributed Aerith's counters (self-gate missing): +1/+1 = %d, want 0", got)
	}

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            aerith,
		"card":            aerith.Card,
		"controller_seat": aerith.Controller,
		"to_zone":         "graveyard",
	})
	if got := legend.Counters["+1/+1"]; got != 3 {
		t.Errorf("Aerith's own death did not distribute counters: +1/+1 = %d, want 3", got)
	}
}

// Hidetsugu and Kairi — "when THIS dies, exile top of your library, opponent
// loses life = its MV" must not self-mill + drain on a foreign death.
func TestSelfGate_Hidetsugu_ForeignDeathDoesNotDrain(t *testing.T) {
	gs := newGame(t, 3)
	hk := stampCreaturePT(addPerm(gs, 0, "Hidetsugu and Kairi", "creature", "legendary"), 5, 5)
	addLibrary(gs, 0, "Top Card", "Second Card")
	foreign := stampCreaturePT(addPerm(gs, 0, "Random Bear", "creature"), 2, 2)

	libBefore := len(gs.Seats[0].Library)
	lifeOpp1 := gs.Seats[1].Life
	lifeOpp2 := gs.Seats[2].Life

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            foreign,
		"card":            foreign.Card,
		"controller_seat": foreign.Controller,
		"to_zone":         "graveyard",
	})
	if len(gs.Seats[0].Library) != libBefore {
		t.Errorf("foreign death made Hidetsugu exile from library (self-gate missing): lib %d, want %d",
			len(gs.Seats[0].Library), libBefore)
	}
	if gs.Seats[1].Life != lifeOpp1 || gs.Seats[2].Life != lifeOpp2 {
		t.Errorf("foreign death made Hidetsugu drain an opponent (self-gate missing): opp life %d/%d, want %d/%d",
			gs.Seats[1].Life, gs.Seats[2].Life, lifeOpp1, lifeOpp2)
	}

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            hk,
		"card":            hk.Card,
		"controller_seat": hk.Controller,
		"to_zone":         "graveyard",
	})
	if len(gs.Seats[0].Library) != libBefore-1 {
		t.Errorf("Hidetsugu's own death did not exile top of library: lib %d, want %d",
			len(gs.Seats[0].Library), libBefore-1)
	}
}

// -----------------------------------------------------------------------------
// damage family — source_card name gate
// -----------------------------------------------------------------------------

// Magnus the Red — "whenever THIS deals combat damage to a player, create a
// 3/3 Spawn" must not mint a token when a DIFFERENT creature its controller
// controls deals the damage.
func TestSelfGate_MagnusTheRed_ForeignDamageDoesNotMint(t *testing.T) {
	gs := newGame(t, 3)
	stampCreaturePT(addPerm(gs, 0, "Magnus the Red", "creature", "legendary"), 6, 6)
	stampCreaturePT(addPerm(gs, 0, "Charging Bear", "creature"), 2, 2)

	before := countByName(gs, 0, "Spawn Token")
	// The BEAR (not Magnus) deals combat damage to a player.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Charging Bear",
		"defender_seat": 1,
		"amount":        2,
	})
	if got := countByName(gs, 0, "Spawn Token") - before; got != 0 {
		t.Errorf("a non-Magnus creature's combat damage minted a Spawn (self-gate missing): +%d tokens, want 0", got)
	}

	// Magnus itself connects.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Magnus the Red",
		"defender_seat": 1,
		"amount":        6,
	})
	if got := countByName(gs, 0, "Spawn Token") - before; got != 1 {
		t.Errorf("Magnus's own combat damage did not mint a Spawn: +%d tokens, want 1", got)
	}
}

// -----------------------------------------------------------------------------
// lore-counter / saga family — ctx["perm"] gate (fire site now threads perm)
// -----------------------------------------------------------------------------

// dispatchSagaPhase7 — advancing one saga must not drive a DIFFERENT
// controlled phase-7 saga's chapter effect. History of Benalia (ch2 → Knight
// token) advances; The Birth of Meletis (ch2 → gain 2 life) must stay inert.
func TestSelfGate_SagaPhase7_AdvancingOneSagaDoesNotDriveAnother(t *testing.T) {
	gs := newGame(t, 3)
	history := addPerm(gs, 0, "History of Benalia", "enchantment", "saga")
	addPerm(gs, 0, "The Birth of Meletis", "enchantment", "saga")

	knightsBefore := countByName(gs, 0, "Knight Token")
	lifeBefore := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "lore_counter_added", map[string]interface{}{
		"seat":    0,
		"card":    "History of Benalia",
		"chapter": 2,
		"perm":    history,
	})
	if got := countByName(gs, 0, "Knight Token") - knightsBefore; got != 1 {
		t.Errorf("History of Benalia chapter 2 did not mint its Knight: +%d, want 1", got)
	}
	if gs.Seats[0].Life != lifeBefore {
		t.Errorf("advancing History also ran The Birth of Meletis chapter 2 (self-gate missing): "+
			"life %d, want %d (no lifegain)", gs.Seats[0].Life, lifeBefore)
	}
}
