package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// combat_damage_batch_r63_test.go — firing-cardinality regressions for the
// §510.4 BATCHED "whenever one or more creatures you control deal combat
// damage to a player" templating. The engine fires combat_damage_player
// once per (attacker, defending player) pair, so a correct handler must
// dedupe per damaged player: three of your creatures hitting one player
// fires the ability ONCE, not three times.

func countTreasuresOnBattlefield(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && cardHasType(p.Card, "treasure") {
			n++
		}
	}
	return n
}

// addCombatSource adds a trigger-bearing permanent with real P/T so it
// survives state-based actions (a 0/0 would be put into the graveyard the
// instant the first trigger resolves, masking the dedupe behaviour under
// test).
func addCombatSource(gs *gameengine.GameState, seat int, name string) *gameengine.Permanent {
	p := addPerm(gs, seat, name, "creature")
	p.Card.BasePower, p.Card.BaseToughness = 2, 4
	return p
}

// fireCombatDamageToPlayer simulates one attacker (source_seat) connecting
// with one defender, exactly as combat.go's fireCombatDamageTriggers does
// per damaging creature.
func fireCombatDamageToPlayer(gs *gameengine.GameState, sourceSeat, defenderSeat int) {
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   sourceSeat,
		"source_card":   "Attacker",
		"defender_seat": defenderSeat,
		"amount":        2,
	})
}

// -----------------------------------------------------------------------------
// Rev, Tithe Extractor — create a Treasure + impulse-exile the defender's
// top card, ONCE per damaged player per combat.
// -----------------------------------------------------------------------------

func TestRevTitheExtractor_ThreeCreaturesOnePlayer_FiresOnce(t *testing.T) {
	gs := newGame(t, 2)
	addCombatSource(gs, 0, "Rev, Tithe Extractor")
	gs.Seats[1].Library = []*gameengine.Card{
		{Name: "Opp Top", Owner: 1},
		{Name: "Opp Second", Owner: 1},
		{Name: "Opp Third", Owner: 1},
	}

	// Three of your creatures all deal combat damage to seat 1 this combat.
	for i := 0; i < 3; i++ {
		fireCombatDamageToPlayer(gs, 0, 1)
	}

	if got := countTreasuresOnBattlefield(gs, 0); got != 1 {
		t.Errorf("expected exactly 1 Treasure (batched once per damaged player); got %d", got)
	}
	if got := len(gs.Seats[1].Exile); got != 1 {
		t.Errorf("expected exactly 1 exiled card; got %d", got)
	}
}

func TestRevTitheExtractor_DamageToTwoPlayers_FiresPerPlayer(t *testing.T) {
	gs := newGame(t, 3)
	addCombatSource(gs, 0, "Rev, Tithe Extractor")
	gs.Seats[1].Library = []*gameengine.Card{{Name: "S1 Top", Owner: 1}}
	gs.Seats[2].Library = []*gameengine.Card{{Name: "S2 Top", Owner: 2}}

	// Two creatures hit seat 1, one hits seat 2 → fires once per damaged
	// player = two total.
	fireCombatDamageToPlayer(gs, 0, 1)
	fireCombatDamageToPlayer(gs, 0, 1)
	fireCombatDamageToPlayer(gs, 0, 2)

	if got := countTreasuresOnBattlefield(gs, 0); got != 2 {
		t.Errorf("expected 2 Treasures (one per damaged player); got %d", got)
	}
	if got := len(gs.Seats[1].Exile) + len(gs.Seats[2].Exile); got != 2 {
		t.Errorf("expected 2 exiled cards (one per damaged player); got %d", got)
	}
}

func TestRevTitheExtractor_IgnoresOpponentDamage(t *testing.T) {
	gs := newGame(t, 2)
	addCombatSource(gs, 0, "Rev, Tithe Extractor")
	gs.Seats[0].Library = []*gameengine.Card{{Name: "My Top", Owner: 0}}

	// Opponent's creature deals combat damage to you — Rev triggers only
	// on YOUR creatures' combat damage.
	fireCombatDamageToPlayer(gs, 1, 0)

	if got := countTreasuresOnBattlefield(gs, 0); got != 0 {
		t.Errorf("opp damage must not trigger Rev; got %d Treasures", got)
	}
	if got := len(gs.Seats[0].Exile); got != 0 {
		t.Errorf("opp damage must not exile; got %d", got)
	}
}

func TestRevTitheExtractor_EmptyLibrary_StillMakesTreasure(t *testing.T) {
	gs := newGame(t, 2)
	addCombatSource(gs, 0, "Rev, Tithe Extractor")
	gs.Seats[1].Library = nil // empty defender library

	fireCombatDamageToPlayer(gs, 0, 1)

	if got := countTreasuresOnBattlefield(gs, 0); got != 1 {
		t.Errorf("Treasure is unconditional on a hit; got %d", got)
	}
	if got := len(gs.Seats[1].Exile); got != 0 {
		t.Errorf("empty library → nothing to exile; got %d", got)
	}
}

// -----------------------------------------------------------------------------
// Gonti, Canny Acquisitor — pin the existing correct dedupe so a future
// refactor can't regress it back to per-attacker firing.
// -----------------------------------------------------------------------------

func TestGontiCannyAcquisitor_ThreeCreaturesOnePlayer_ExilesOnce(t *testing.T) {
	gs := newGame(t, 2)
	addCombatSource(gs, 0, "Gonti, Canny Acquisitor")
	gs.Seats[1].Library = []*gameengine.Card{
		{Name: "Opp Top", Owner: 1},
		{Name: "Opp Second", Owner: 1},
		{Name: "Opp Third", Owner: 1},
	}

	for i := 0; i < 3; i++ {
		fireCombatDamageToPlayer(gs, 0, 1)
	}

	if got := len(gs.Seats[1].Exile); got != 1 {
		t.Errorf("Gonti exiles once per damaged player; got %d exiles for 3 attackers", got)
	}
}
