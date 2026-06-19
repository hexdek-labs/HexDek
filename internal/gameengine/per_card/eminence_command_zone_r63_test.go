package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// eminence_command_zone_r63_test.go — Eminence (CR §702.107). The per_card
// trigger dispatcher walked the battlefield only, so eminence triggered
// abilities (Edgar Markov's "whenever you cast another Vampire spell") never
// fired while the commander sat in the COMMAND ZONE. OnEminenceTrigger +
// fireTrigger's command-zone scan fix that, without firing non-eminence
// abilities from the command zone.

func countVampireTokens(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Vampire Token" {
			n++
		}
	}
	return n
}

// (1) THE KEY CASE: Edgar in the command zone — casting a Vampire spell fires
// the eminence trigger and makes a Vampire token.
func TestEminence_FiresFromCommandZone(t *testing.T) {
	gs := newGame(t, 2)
	edgar := addCard(gs, 0, "Edgar Markov", "creature", "legendary", "vampire")
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, edgar)

	vamp := addCard(gs, 0, "Vampire Nighthawk", "creature", "vampire")
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        vamp,
	})

	if got := countVampireTokens(gs, 0); got != 1 {
		t.Fatalf("eminence should make 1 Vampire token from the command zone, got %d", got)
	}
}

// (2) Eminence still fires from the BATTLEFIELD (no regression) — and exactly
// once (no double-fire).
func TestEminence_FiresFromBattlefieldOnce(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Edgar Markov", "creature", "legendary", "vampire")
	vamp := addCard(gs, 0, "Vampire Nighthawk", "creature", "vampire")
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        vamp,
	})
	if got := countVampireTokens(gs, 0); got != 1 {
		t.Fatalf("eminence from the battlefield should make exactly 1 token, got %d", got)
	}
}

// (3) The NON-eminence "whenever Edgar attacks" trigger must NOT fire from the
// command zone (CR §400.10b — only the eminence ability functions there).
func TestEminence_NonEminenceTriggerStaysOffCommandZone(t *testing.T) {
	gs := newGame(t, 2)
	edgar := addCard(gs, 0, "Edgar Markov", "creature", "legendary", "vampire")
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, edgar)
	vamp := addPerm(gs, 0, "Vampire Soldier", "creature", "vampire")

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": vamp,
	})

	if vamp.Counters["+1/+1"] != 0 {
		t.Fatalf("Edgar's non-eminence attack trigger must not fire from the command zone, got %d counters", vamp.Counters["+1/+1"])
	}
}

// (4) Only the controller's own casts trigger their command-zone Edgar.
func TestEminence_OnlyControllersCast(t *testing.T) {
	gs := newGame(t, 2)
	edgar := addCard(gs, 0, "Edgar Markov", "creature", "legendary", "vampire")
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, edgar)

	oppVamp := addCard(gs, 1, "Opp Vampire", "creature", "vampire")
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"card":        oppVamp,
	})

	if countVampireTokens(gs, 0) != 0 {
		t.Fatal("an opponent's Vampire cast must not trigger your command-zone Edgar")
	}
}

// (5) A non-Vampire cast does not trigger eminence (handler's type gate still
// applies from the command zone).
func TestEminence_NonVampireCastNoToken(t *testing.T) {
	gs := newGame(t, 2)
	edgar := addCard(gs, 0, "Edgar Markov", "creature", "legendary", "vampire")
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, edgar)

	bear := addCard(gs, 0, "Grizzly Bears", "creature", "bear")
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        bear,
	})

	if countVampireTokens(gs, 0) != 0 {
		t.Fatal("a non-Vampire cast must not make a Vampire token via eminence")
	}
}
