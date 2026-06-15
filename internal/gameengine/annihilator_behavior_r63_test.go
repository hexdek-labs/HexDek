package gameengine

import "testing"

// annihilator_behavior_r63_test.go — behavioral probe for Annihilator N
// (CR §702.86). Annihilator is live (fired from the attack-trigger pipeline).
// These pin the properties the count-only existing tests didn't: the DEFENDING
// player chooses (via their own hat), multiplayer isolation, fewer-than-N, and
// non-creature eligibility.

func annihilatorCreatureNamed(gs *GameState, seat int, name string) bool {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			return true
		}
	}
	return false
}

// (c) The DEFENDING player chooses which of their own permanents to sacrifice —
// the engine must honor that seat's SacrificeChooser hat, not impose an
// attacker-side "smallest" pick.
func TestAnnihilator_DefenderChoosesVictim(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Eldrazi", 10, 10, "annihilator", "creature")
	atk.Flags["annihilator_n"] = 1

	// Defender's hat will sacrifice "SacMe" (a big creature), keeping the small ones.
	gs.Seats[1].Hat = &sacStub{pick: map[int]string{1: "SacMe"}}
	addKWCombatBattlefield(gs, 1, "Keep1", 1, 1, "creature")
	addKWCombatBattlefield(gs, 1, "Keep2", 2, 2, "creature")
	addKWCombatBattlefield(gs, 1, "SacMe", 5, 5, "creature")

	setAttackerDefender(atk, 1)
	ApplyAnnihilator(gs, atk, 1)

	if annihilatorCreatureNamed(gs, 1, "SacMe") {
		t.Error("(c) the defending player's hat chose 'SacMe' — it must be the one sacrificed")
	}
	if !annihilatorCreatureNamed(gs, 1, "Keep1") || !annihilatorCreatureNamed(gs, 1, "Keep2") {
		t.Error("(c) only the defender's chosen permanent should be sacrificed")
	}
}

// (b) Multiplayer — only the DEFENDING player sacrifices; other opponents are
// untouched.
func TestAnnihilator_OnlyDefendingPlayerInMultiplayer(t *testing.T) {
	gs := newKWCombatGame4P(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Eldrazi", 10, 10, "annihilator", "creature")
	atk.Flags["annihilator_n"] = 2
	addKWCombatBattlefield(gs, 1, "D1", 1, 1, "creature")
	addKWCombatBattlefield(gs, 1, "D2", 2, 2, "creature")
	addKWCombatBattlefield(gs, 2, "Bystander2", 3, 3, "creature")
	addKWCombatBattlefield(gs, 3, "Bystander3", 3, 3, "creature")

	setAttackerDefender(atk, 1)
	ApplyAnnihilator(gs, atk, 1)

	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("(b) defending seat 1 should sacrifice both permanents, got %d left", len(gs.Seats[1].Battlefield))
	}
	if len(gs.Seats[2].Battlefield) != 1 || len(gs.Seats[3].Battlefield) != 1 {
		t.Error("(b) non-defending opponents must be unaffected by annihilator")
	}
}

// (d) Fewer than N permanents → sacrifice all they have (no crash, no over-sac).
func TestAnnihilator_FewerThanNSacrificesAll(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Emrakul", 15, 15, "annihilator", "creature")
	atk.Flags["annihilator_n"] = 6
	addKWCombatBattlefield(gs, 1, "Only1", 1, 1, "creature")

	setAttackerDefender(atk, 1)
	ApplyAnnihilator(gs, atk, 1)

	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("(d) defender with 1 permanent vs annihilator 6 should lose it, got %d left", len(gs.Seats[1].Battlefield))
	}
}

// (c)+(e) Any permanent type is eligible (lands too), and the sacrifice routes
// through the canonical path (emits a sacrifice event so dies/LTB fire).
func TestAnnihilator_NonCreaturePermanentsEligible(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Eldrazi", 10, 10, "annihilator", "creature")
	atk.Flags["annihilator_n"] = 2
	// Defender's hat picks the land first, then the artifact (both non-creatures).
	gs.Seats[1].Hat = &sacStub{pick: map[int]string{1: "Wastes"}}
	addKWCombatBattlefield(gs, 1, "Wastes", 0, 0, "land")
	addKWCombatBattlefield(gs, 1, "Relic", 0, 0, "artifact")
	addKWCombatBattlefield(gs, 1, "Keeper", 4, 4, "creature")

	setAttackerDefender(atk, 1)
	ApplyAnnihilator(gs, atk, 1)

	// Land was the hat's first pick; the creature should survive (hat keeps it).
	if annihilatorCreatureNamed(gs, 1, "Wastes") {
		t.Error("(c) the land the defender chose must be sacrificeable")
	}
	if !annihilatorCreatureNamed(gs, 1, "Keeper") {
		t.Error("(c) the defender should be able to keep its creature by choosing non-creatures")
	}
	sacCount := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "sacrifice" {
			sacCount++
		}
	}
	if sacCount != 2 {
		t.Errorf("(e) annihilator 2 should emit 2 canonical sacrifice events, got %d", sacCount)
	}
}
