package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// Phase 6 — State-based action tests (CR §704).
//
// Reuses the fixture helpers defined in resolve_test.go (same package):
//   - newFixtureGame(t)         — 2-seat game, RNG seeded with 42
//   - addBattlefield(gs, seat, name, pow, tough, types...)
//   - addLibrary / addHand / countEvents / lastEventOfKind
// -----------------------------------------------------------------------------

// addToken places a token permanent on the battlefield (Types include "token"
// so §704.5d's zone-sweep key recognizes it).
func addToken(gs *GameState, seat int, name string, pow, tough int) *Permanent {
	return addBattlefield(gs, seat, name, pow, tough, "creature", "token")
}

// addTokenCardInZone pushes a token-tagged Card into the given non-battlefield
// zone so we can verify §704.5d's sweep.
func addTokenCardInZone(gs *GameState, seat int, name, zone string) *Card {
	c := &Card{Name: name, Owner: seat, Types: []string{"creature", "token"}}
	gs.moveToZone(seat, c, zone)
	return c
}

// -----------------------------------------------------------------------------
// §704.5a — life ≤ 0 → loss.
// -----------------------------------------------------------------------------

func TestSBA_704_5a_LifeZeroLoses(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 0
	if !StateBasedActions(gs) {
		t.Fatal("expected SBA to fire for life=0")
	}
	if !gs.Seats[0].Lost {
		t.Fatal("seat 0 should have lost")
	}
	if countEvents(gs, "sba_704_5a") == 0 {
		t.Fatal("missing sba_704_5a event")
	}
}

func TestSBA_704_5a_NegativeLifeLoses(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[1].Life = -7
	StateBasedActions(gs)
	if !gs.Seats[1].Lost {
		t.Fatal("seat 1 should have lost with life=-7")
	}
	if gs.Seats[1].LossReason == "" {
		t.Fatal("LossReason should be set")
	}
}

// -----------------------------------------------------------------------------
// §704.5b — drew from empty library → loss.
// -----------------------------------------------------------------------------

func TestSBA_704_5b_EmptyLibraryDrawLoses(t *testing.T) {
	gs := newFixtureGame(t)
	// Simulate a draw attempt against an empty library.
	if _, drew := gs.drawOne(0); drew {
		t.Fatal("drawOne should have returned drew=false against empty library")
	}
	if !StateBasedActions(gs) {
		t.Fatal("expected SBA to fire")
	}
	if !gs.Seats[0].Lost {
		t.Fatal("seat 0 should have lost from empty-library draw")
	}
	if gs.Seats[0].AttemptedEmptyDraw {
		t.Fatal("AttemptedEmptyDraw should be cleared after SBA consumes it")
	}
	if countEvents(gs, "sba_704_5b") == 0 {
		t.Fatal("missing sba_704_5b event")
	}
}

// -----------------------------------------------------------------------------
// §704.5c — 10+ poison counters → loss.
// -----------------------------------------------------------------------------

func TestSBA_704_5c_TenPoisonLoses(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].PoisonCounters = 10
	StateBasedActions(gs)
	if !gs.Seats[0].Lost {
		t.Fatal("seat 0 should have lost from 10 poison counters")
	}
	if countEvents(gs, "sba_704_5c") == 0 {
		t.Fatal("missing sba_704_5c event")
	}
}

func TestSBA_704_5c_NinePoisonSurvives(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].PoisonCounters = 9
	StateBasedActions(gs)
	if gs.Seats[0].Lost {
		t.Fatal("seat 0 should NOT have lost from 9 poison counters")
	}
}

// -----------------------------------------------------------------------------
// §704.5d — tokens in non-battlefield zones cease to exist.
// -----------------------------------------------------------------------------

func TestSBA_704_5d_TokenInGraveyardCeases(t *testing.T) {
	gs := newFixtureGame(t)
	addTokenCardInZone(gs, 0, "Soldier Token", "graveyard")
	addTokenCardInZone(gs, 0, "Soldier Token", "exile")
	// Add a non-token card to verify it stays.
	gs.moveToZone(0, &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}, "graveyard")

	StateBasedActions(gs)

	if len(gs.Seats[0].Graveyard) != 1 {
		t.Fatalf("expected 1 card in graveyard after token sweep, got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Exile) != 0 {
		t.Fatalf("expected 0 cards in exile, got %d", len(gs.Seats[0].Exile))
	}
	if countEvents(gs, "sba_704_5d") < 2 {
		t.Fatalf("expected at least 2 sba_704_5d events (graveyard + exile), got %d", countEvents(gs, "sba_704_5d"))
	}
}

// -----------------------------------------------------------------------------
// §704.5f — creature toughness ≤ 0 → graveyard.
// -----------------------------------------------------------------------------

func TestSBA_704_5f_ZeroToughnessDies(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Weakling", 1, 0, "creature")
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatalf("expected empty battlefield, got %d", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Graveyard) != 1 || gs.Seats[0].Graveyard[0] != p.Card {
		t.Fatal("Weakling should be in graveyard")
	}
	if countEvents(gs, "sba_704_5f") == 0 {
		t.Fatal("missing sba_704_5f event")
	}
}

func TestSBA_704_5f_MinusCountersZeroToughnessDies(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	// Three -1/-1 counters → effective 2-3 = -1 toughness.
	p.AddCounter("-1/-1", 3)
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("Grizzly Bears at -1 toughness should be in graveyard")
	}
}

// -----------------------------------------------------------------------------
// §704.5g — lethal damage → destroy (indestructible survives).
// -----------------------------------------------------------------------------

func TestSBA_704_5g_LethalDamageDestroys(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Runeclaw Bear", 2, 2, "creature")
	p.MarkedDamage = 2
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("Runeclaw Bear with lethal damage should be destroyed")
	}
	if countEvents(gs, "sba_704_5g") == 0 {
		t.Fatal("missing sba_704_5g event")
	}
}

func TestSBA_704_5g_IndestructibleSurvivesLethal(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Darksteel Myr", 0, 1, "creature")
	p.MarkedDamage = 99
	p.Flags = map[string]int{"indestructible": 1}
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatal("Indestructible creature should survive lethal damage (CR §702.12b)")
	}
}

// -----------------------------------------------------------------------------
// §704.5i — planeswalker 0 loyalty → graveyard.
// -----------------------------------------------------------------------------

func TestSBA_704_5i_ZeroLoyaltyDies(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Chandra, Torch of Defiance", 0, 0, "planeswalker", "legendary")
	p.AddCounter("loyalty", 3)
	p.AddCounter("loyalty", -3) // ult: 0 loyalty
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("Planeswalker at 0 loyalty should be in graveyard")
	}
	if countEvents(gs, "sba_704_5i") == 0 {
		t.Fatal("missing sba_704_5i event")
	}
}

func TestSBA_704_5i_MissingLoyaltySkipsKill(t *testing.T) {
	// Python's safety net: a planeswalker without an initialized loyalty
	// counter should NOT die on SBA — it's treated as "ETB init pending".
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Jace, the Mind Sculptor", 0, 0, "planeswalker", "legendary")
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatal("Planeswalker without loyalty counter should NOT die")
	}
}

// -----------------------------------------------------------------------------
// §704.5j — Legend Rule: earliest timestamp survives.
// -----------------------------------------------------------------------------

func TestSBA_704_5j_LegendRuleKeepsEarliest(t *testing.T) {
	gs := newFixtureGame(t)
	a := addBattlefield(gs, 0, "Emrakul, the Aeons Torn", 15, 15, "creature", "legendary")
	b := addBattlefield(gs, 0, "Emrakul, the Aeons Torn", 15, 15, "creature", "legendary")
	if a.Timestamp >= b.Timestamp {
		t.Fatalf("expected a.Timestamp < b.Timestamp; got %d vs %d", a.Timestamp, b.Timestamp)
	}
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("expected 1 Emrakul surviving, got %d", len(gs.Seats[0].Battlefield))
	}
	if gs.Seats[0].Battlefield[0] != a {
		t.Fatal("earliest-timestamp Emrakul should have survived")
	}
	if countEvents(gs, "sba_704_5j_keep") == 0 {
		t.Fatal("missing sba_704_5j_keep event")
	}
}

func TestSBA_704_5j_LegendaryAcrossControllersOk(t *testing.T) {
	// Two copies of the same legend under DIFFERENT controllers are legal.
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Karn Liberated", 6, 6, "planeswalker", "legendary")
	addBattlefield(gs, 1, "Karn Liberated", 6, 6, "planeswalker", "legendary")
	// Give both the loyalty ETB.
	gs.Seats[0].Battlefield[0].AddCounter("loyalty", 6)
	gs.Seats[1].Battlefield[0].AddCounter("loyalty", 6)
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 1 || len(gs.Seats[1].Battlefield) != 1 {
		t.Fatal("legend rule should NOT fire across controllers")
	}
}

// -----------------------------------------------------------------------------
// §704.5k — World Rule: newest survives.
// -----------------------------------------------------------------------------

func TestSBA_704_5k_WorldRuleKeepsNewest(t *testing.T) {
	gs := newFixtureGame(t)
	old := addBattlefield(gs, 0, "Nether Void", 0, 0, "enchantment", "world")
	newer := addBattlefield(gs, 1, "Crusade", 0, 0, "enchantment", "world")
	if newer.Timestamp <= old.Timestamp {
		t.Fatal("timestamps should be strictly increasing")
	}
	StateBasedActions(gs)
	// Old dies; new survives.
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("older world permanent should have died")
	}
	if len(gs.Seats[1].Battlefield) != 1 {
		t.Fatal("newer world permanent should have survived")
	}
}

// -----------------------------------------------------------------------------
// §704.5n — Equipment attached to non-creature → unattach.
// -----------------------------------------------------------------------------

func TestSBA_704_5n_EquipmentUnattachesFromNonCreature(t *testing.T) {
	gs := newFixtureGame(t)
	land := addBattlefield(gs, 0, "Plains", 0, 0, "land", "basic")
	sword := addBattlefield(gs, 0, "Sword of Fire and Ice", 0, 0, "artifact", "equipment")
	sword.AttachedTo = land
	StateBasedActions(gs)
	if sword.AttachedTo != nil {
		t.Fatal("Equipment should have unattached from land (CR §704.5n / §301.5)")
	}
	// Stays on the battlefield.
	if len(gs.Seats[0].Battlefield) != 2 {
		t.Fatal("Equipment should remain on battlefield")
	}
	if countEvents(gs, "sba_704_5n") == 0 {
		t.Fatal("missing sba_704_5n event")
	}
}

// -----------------------------------------------------------------------------
// §704.5p — creature attached to something → unattach.
// -----------------------------------------------------------------------------

func TestSBA_704_5p_CreatureUnattaches(t *testing.T) {
	gs := newFixtureGame(t)
	host := addBattlefield(gs, 0, "Mountain", 0, 0, "land")
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.AttachedTo = host
	StateBasedActions(gs)
	if bear.AttachedTo != nil {
		t.Fatal("creature should have been unattached per §704.5p")
	}
	if countEvents(gs, "sba_704_5p") == 0 {
		t.Fatal("missing sba_704_5p event")
	}
}

// -----------------------------------------------------------------------------
// §704.5q — +1/+1 and -1/-1 counters annihilate.
// -----------------------------------------------------------------------------

func TestSBA_704_5q_CounterAnnihilation(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Servant of the Scale", 1, 1, "creature")
	p.AddCounter("+1/+1", 3)
	p.AddCounter("-1/-1", 2)
	StateBasedActions(gs)
	if got := p.Counters["+1/+1"]; got != 1 {
		t.Fatalf("expected 1 +1/+1 after annihilation, got %d", got)
	}
	if _, has := p.Counters["-1/-1"]; has {
		t.Fatalf("expected -1/-1 key removed after hitting zero, got %+v", p.Counters)
	}
	if countEvents(gs, "sba_704_5q") == 0 {
		t.Fatal("missing sba_704_5q event")
	}
}

// -----------------------------------------------------------------------------
// §704.5s — Saga sacrifice when lore counters >= final chapter.
// -----------------------------------------------------------------------------

func TestSBA_704_5s_SagaSacrificeAtFinalChapter(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "History of Benalia", 0, 0, "enchantment", "saga")
	p.Counters = map[string]int{
		"saga_final_chapter": 3,
		"lore":               3,
	}
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("Saga at final chapter should be sacrificed")
	}
	if countEvents(gs, "sba_704_5s_saga") == 0 {
		t.Fatal("missing sba_704_5s_saga event")
	}
}

// -----------------------------------------------------------------------------
// §704.5v — battle at 0 defense → graveyard.
// -----------------------------------------------------------------------------

func TestSBA_704_5v_BattleZeroDefenseDies(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Invasion of Ravnica", 0, 0, "battle", "siege")
	p.Counters = map[string]int{"defense": 0}
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("Battle at 0 defense should have died")
	}
	if countEvents(gs, "sba_704_5v") == 0 {
		t.Fatal("missing sba_704_5v event")
	}
}

// -----------------------------------------------------------------------------
// §704.5y — Role uniqueness (newest timestamp stays).
// -----------------------------------------------------------------------------

func TestSBA_704_5y_RoleUniquenessKeepsNewest(t *testing.T) {
	gs := newFixtureGame(t)
	holder := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	role1 := addBattlefield(gs, 0, "Cursed Role", 0, 0, "enchantment", "aura", "role")
	role2 := addBattlefield(gs, 0, "Monster Role", 0, 0, "enchantment", "aura", "role")
	role1.AttachedTo = holder
	role2.AttachedTo = holder
	if role2.Timestamp <= role1.Timestamp {
		t.Fatal("role2 should have the newer timestamp")
	}
	StateBasedActions(gs)
	// role1 should have died; role2 should survive.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == role2 {
			found = true
		}
		if p == role1 {
			t.Fatal("older role should have died (CR §704.5y)")
		}
	}
	if !found {
		t.Fatal("newer role should still be on battlefield")
	}
}

// -----------------------------------------------------------------------------
// §704.6c — Commander damage 21 → loss.
// -----------------------------------------------------------------------------

func TestSBA_704_6c_CommanderDamageLoss(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = true
	gs.Seats[1].CommanderNames = []string{"Edgar Markov"}
	// Seat 0 takes 21 from seat 1's Edgar Markov.
	AccumulateCommanderDamage(gs, 0, 1, "Edgar Markov", 21)
	StateBasedActions(gs)
	if !gs.Seats[0].Lost {
		t.Fatal("seat 0 should have lost from 21 commander damage (CR §704.6c)")
	}
	if countEvents(gs, "sba_704_6c") == 0 {
		t.Fatal("missing sba_704_6c event")
	}
}

func TestSBA_704_6c_NonCommanderGameSkips(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = false
	gs.Seats[0].CommanderDamage = map[int]map[string]int{
		1: {"Edgar Markov": 50},
	}
	StateBasedActions(gs)
	if gs.Seats[0].Lost {
		t.Fatal("seat 0 should NOT have lost in non-Commander format")
	}
}

// -----------------------------------------------------------------------------
// §704.6d — commander in graveyard → command zone.
// -----------------------------------------------------------------------------

func TestSBA_704_6d_CommanderFromGraveyardReturns(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = true
	gs.Seats[0].CommanderNames = []string{"Atraxa, Praetors' Voice"}
	cmdr := &Card{Name: "Atraxa, Praetors' Voice", Owner: 0, Types: []string{"creature", "legendary"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, cmdr)
	StateBasedActions(gs)
	if len(gs.Seats[0].CommandZone) != 1 || gs.Seats[0].CommandZone[0] != cmdr {
		t.Fatal("commander should have returned to command zone (CR §704.6d / §903.9a)")
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Fatal("graveyard should be empty after commander return")
	}
	if countEvents(gs, "sba_704_6d") == 0 {
		t.Fatal("missing sba_704_6d event")
	}
}

func TestSBA_704_6d_CommanderFromExileReturns(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = true
	gs.Seats[1].CommanderNames = []string{"The Ur-Dragon"}
	cmdr := &Card{Name: "The Ur-Dragon", Owner: 1, Types: []string{"creature", "legendary"}}
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, cmdr)
	StateBasedActions(gs)
	if len(gs.Seats[1].CommandZone) != 1 {
		t.Fatalf("expected commander back in command zone, got %d entries", len(gs.Seats[1].CommandZone))
	}
	if len(gs.Seats[1].Exile) != 0 {
		t.Fatal("exile should be empty after commander return")
	}
}

// -----------------------------------------------------------------------------
// §704.3 — iteration: multi-pass convergence.
// -----------------------------------------------------------------------------

func TestSBA_704_3_MultiPassConvergence(t *testing.T) {
	// Two 0-toughness creatures + two legend-rule conflicts should converge
	// in at most a couple passes.
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Weakling A", 1, 0, "creature")
	addBattlefield(gs, 0, "Weakling B", 1, 0, "creature")
	addBattlefield(gs, 0, "Godzilla", 10, 10, "creature", "legendary")
	addBattlefield(gs, 0, "Godzilla", 10, 10, "creature", "legendary")
	changed := StateBasedActions(gs)
	if !changed {
		t.Fatal("expected SBA to report changes")
	}
	// Remaining: 1 Godzilla (legend rule keeps earliest).
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("expected 1 permanent surviving, got %d", len(gs.Seats[0].Battlefield))
	}
	// Graveyard should have 3 entries: 2 Weaklings + 1 Godzilla.
	if len(gs.Seats[0].Graveyard) != 3 {
		t.Fatalf("expected 3 cards in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
	if countEvents(gs, "sba_cycle_complete") == 0 {
		t.Fatal("missing sba_cycle_complete event")
	}
}

// -----------------------------------------------------------------------------
// 40-pass safety cap.
// -----------------------------------------------------------------------------

// TestSBA_704_3_MaxPassesCap verifies the 40-pass cap doesn't panic and
// emits the sba_cap_hit event. We contrive an infinite loop by cheating
// at the state layer: a permanent whose toughness drops to 0 after we put
// it back on the battlefield on each pass. Easier to simulate: seed a
// seat with 41 independent 0-toughness creatures; each pass only destroys
// them in one sweep, so the whole thing converges quickly — that's NOT a
// cap trigger. Instead we add a permanent that re-spawns ITSELF via a
// callback-like nudge: we mutate state between passes via a hook we
// inject as a special "regrowth" permanent. Since the cap protects
// against malformed cards, the simplest test is to verify the cap path
// with a carefully constructed scenario: tons of pairs of legendary
// permanents with incrementally larger timestamps, so each pass removes
// one and then new ones become candidates. Actually the simplest thing:
// seed 50 +1/+1 / -1/-1 permanents — each pass wipes them in one sweep.
//
// The pragmatic test: confirm no crash and that simpler scenarios never
// hit the cap. A truly pathological case would require a card to re-
// spawn state every pass, which requires resolver hooks not yet wired.
// We assert: a 40+ permanent SBA scenario converges in few passes and
// does NOT emit sba_cap_hit.
func TestSBA_704_3_NoSpuriousCap(t *testing.T) {
	gs := newFixtureGame(t)
	// 100 creatures at 0 toughness — all destroyed in one pass.
	for i := 0; i < 100; i++ {
		addBattlefield(gs, 0, "ZeroToughnessGuy", 0, 0, "creature")
	}
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatalf("expected all 100 to die, got %d remaining", len(gs.Seats[0].Battlefield))
	}
	if countEvents(gs, "sba_cap_hit") != 0 {
		t.Fatal("40-pass cap should NOT have engaged for a 1-pass scenario")
	}
}

// -----------------------------------------------------------------------------
// Idempotency — no event spam after player is already lost.
// -----------------------------------------------------------------------------

func TestSBA_704_5a_DoesNotSpamAfterLoss(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = -1
	StateBasedActions(gs)
	StateBasedActions(gs)
	StateBasedActions(gs)
	if countEvents(gs, "sba_704_5a") != 1 {
		t.Fatalf("expected exactly 1 sba_704_5a event across 3 runs, got %d",
			countEvents(gs, "sba_704_5a"))
	}
}

// -----------------------------------------------------------------------------
// Nil safety.
// -----------------------------------------------------------------------------

func TestStateBasedActions_NilSafe(t *testing.T) {
	if StateBasedActions(nil) {
		t.Fatal("nil GameState should report no changes")
	}
}

// -----------------------------------------------------------------------------
// Wave 4 — §704.5h deathtouch damage kill.
// -----------------------------------------------------------------------------

func TestSBA_704_5h_DeathtouchDamageKills(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Colossal Dreadmaw", 6, 6, "creature")
	// Simulate 1 damage from a deathtouch source.
	p.MarkedDamage = 1
	p.Flags["deathtouch_damaged"] = 1
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("creature with deathtouch damage should be destroyed (CR §704.5h)")
	}
	if countEvents(gs, "sba_704_5h") == 0 {
		t.Fatal("missing sba_704_5h event")
	}
}

func TestSBA_704_5h_DeathtouchIndestructibleSurvives(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Darksteel Colossus", 11, 11, "creature")
	p.MarkedDamage = 1
	p.Flags["deathtouch_damaged"] = 1
	p.Flags["indestructible"] = 1
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatal("indestructible creature should survive deathtouch damage")
	}
}

func TestSBA_704_5h_NoDamageFlagNoKill(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	p.MarkedDamage = 1
	// No deathtouch_damaged flag — should not die.
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatal("creature without deathtouch flag should survive 1 damage")
	}
}

// -----------------------------------------------------------------------------
// Wave 4 — Phased-out permanents skip SBAs.
// -----------------------------------------------------------------------------

func TestSBA_PhasedOutSkipsLegendRule(t *testing.T) {
	gs := newFixtureGame(t)
	a := addBattlefield(gs, 0, "Avacyn", 5, 5, "creature", "legendary")
	b := addBattlefield(gs, 0, "Avacyn", 5, 5, "creature", "legendary")
	// Phase out the second one.
	b.PhasedOut = true
	StateBasedActions(gs)
	// Both should survive — phased-out doesn't count for legend rule.
	if len(gs.Seats[0].Battlefield) != 2 {
		t.Fatalf("expected 2 legendaries (one phased out), got %d", len(gs.Seats[0].Battlefield))
	}
	_ = a
}

func TestSBA_PhasedOutSkipsToughnessDeath(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Weakling", 1, 0, "creature")
	p.PhasedOut = true
	StateBasedActions(gs)
	// Should survive because phased-out permanents don't exist.
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatal("phased-out 0-toughness creature should not die")
	}
}

func TestSBA_PhasedOutSkipsPlaneswalkerLoyalty(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Jace", 0, 0, "planeswalker")
	p.Counters = map[string]int{"loyalty": 0}
	p.PhasedOut = true
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatal("phased-out planeswalker at 0 loyalty should not die")
	}
}

// -----------------------------------------------------------------------------
// Wave 4 — §704.5m Aura orphan detection.
// -----------------------------------------------------------------------------

func TestSBA_704_5m_AuraOrphanAfterTargetRemoval(t *testing.T) {
	gs := newFixtureGame(t)
	host := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	aura := addBattlefield(gs, 0, "Pacifism", 0, 0, "enchantment", "aura")
	aura.AttachedTo = host
	// Now remove the host manually.
	gs.removePermanent(host)
	StateBasedActions(gs)
	// Aura should die because its target is gone.
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("Aura with removed host should die (CR §704.5m)")
	}
}

// -----------------------------------------------------------------------------
// Benchmark — 4 seats, ~30 permanents, 3 with marked damage.
// -----------------------------------------------------------------------------

func BenchmarkStateBasedActions(b *testing.B) {
	// Construct a realistic mid-game state each iteration via a fresh game
	// so SBA-induced mutations (destroy events) don't accumulate.
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		gs := NewGameState(4, nil, nil)
		for seat := 0; seat < 4; seat++ {
			for j := 0; j < 6; j++ {
				addBattlefield(gs, seat, "Forest", 0, 0, "land")
			}
			addBattlefield(gs, seat, "Grizzly Bears", 2, 2, "creature")
			addBattlefield(gs, seat, "Llanowar Elves", 1, 1, "creature")
		}
		// Three creatures with marked damage (non-lethal).
		gs.Seats[0].Battlefield[6].MarkedDamage = 1 // Grizzly Bears: 1/2 damage
		gs.Seats[1].Battlefield[6].MarkedDamage = 1
		gs.Seats[2].Battlefield[6].MarkedDamage = 1
		b.StartTimer()
		_ = StateBasedActions(gs)
	}
}

// -----------------------------------------------------------------------------
// §704.5r — counter limit enforcement.
// -----------------------------------------------------------------------------

func TestSBA_704_5r_CounterLimitTrims(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Rasputin Dreamweaver", 4, 1, "creature")
	p.Card.AST = &gameast.CardAST{
		Name: "Rasputin Dreamweaver",
		Abilities: []gameast.Ability{
			&gameast.Static{
				Raw: "Rasputin can't have more than seven dream counters on it",
			},
		},
	}
	p.Counters["dream"] = 10

	if !StateBasedActions(gs) {
		t.Fatal("expected SBA to fire for excess dream counters")
	}
	if p.Counters["dream"] != 7 {
		t.Fatalf("expected 7 dream counters, got %d", p.Counters["dream"])
	}
	if countEvents(gs, "sba_704_5r") != 1 {
		t.Fatal("missing sba_704_5r event")
	}
}

func TestSBA_704_5r_NoFireWhenUnderLimit(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Rasputin Dreamweaver", 4, 1, "creature")
	p.Card.AST = &gameast.CardAST{
		Name: "Rasputin Dreamweaver",
		Abilities: []gameast.Ability{
			&gameast.Static{
				Raw: "Rasputin can't have more than seven dream counters on it",
			},
		},
	}
	p.Counters["dream"] = 5

	if StateBasedActions(gs) {
		if countEvents(gs, "sba_704_5r") > 0 {
			t.Fatal("SBA should not fire when under limit")
		}
	}
	if p.Counters["dream"] != 5 {
		t.Fatalf("expected 5 dream counters unchanged, got %d", p.Counters["dream"])
	}
}

func TestParseCounterLimit(t *testing.T) {
	tests := []struct {
		raw      string
		wantN    int
		wantKind string
	}{
		{"Rasputin can't have more than seven dream counters on it", 7, "dream"},
		{"This can't have more than 3 charge counters on it", 3, "charge"},
		{"This can't have more than ten ice counters on it", 10, "ice"},
		{"Flying", -1, ""},
		{"", -1, ""},
	}
	for _, tt := range tests {
		n, kind := parseCounterLimit(tt.raw)
		if n != tt.wantN || kind != tt.wantKind {
			t.Errorf("parseCounterLimit(%q) = (%d, %q), want (%d, %q)",
				tt.raw, n, kind, tt.wantN, tt.wantKind)
		}
	}
}
