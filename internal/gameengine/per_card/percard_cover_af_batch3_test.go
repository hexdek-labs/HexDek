package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// percard_cover_af_batch3_test.go — regression pins for A–F batch 3:
// Angel of Despair, Dictate of Erebos, Dark Prophecy, Crackling Doom.

func afOnBF(gs *gameengine.GameState, seat int, c *gameengine.Card) bool {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card == c {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Angel of Despair
// ---------------------------------------------------------------------------

func TestAngelOfDespair_ETB_DestroysBestOpponentPermanent(t *testing.T) {
	gs := newGame(t, 2)
	angel := addPerm(gs, 0, "Angel of Despair", "creature", "angel")
	angel.Card.BasePower, angel.Card.BaseToughness = 5, 5
	lotus := addPerm(gs, 1, "Gilded Lotus", "artifact", "cmc:5")
	addPerm(gs, 1, "Island", "land")

	angelOfDespairETB(gs, angel)

	if afOnBF(gs, 1, lotus.Card) {
		t.Fatal("Angel of Despair should have destroyed the opponent's Gilded Lotus")
	}
}

// ---------------------------------------------------------------------------
// Dictate of Erebos
// ---------------------------------------------------------------------------

func TestDictateOfErebos_YourCreatureDies_EachOpponentSacrifices(t *testing.T) {
	gs := newGame(t, 2)
	dictate := addPerm(gs, 0, "Dictate of Erebos", "enchantment")
	weak := addPerm(gs, 1, "Goblin Token", "creature", "goblin")
	weak.Card.BasePower, weak.Card.BaseToughness = 1, 1
	strong := addPerm(gs, 1, "Serra Angel", "creature", "angel")
	strong.Card.BasePower, strong.Card.BaseToughness = 4, 4

	dictateOfErebosDies(gs, dictate, map[string]interface{}{"controller_seat": 0, "card": &gameengine.Card{Name: "Dead"}})

	if afOnBF(gs, 1, weak.Card) {
		t.Error("opponent should have sacrificed their weakest creature")
	}
	if !afOnBF(gs, 1, strong.Card) {
		t.Error("opponent should keep their stronger creature (sac the weakest)")
	}
}

func TestDictateOfErebos_OpponentCreatureDies_NoEdict(t *testing.T) {
	gs := newGame(t, 2)
	dictate := addPerm(gs, 0, "Dictate of Erebos", "enchantment")
	oppCreature := addPerm(gs, 1, "Serra Angel", "creature", "angel")
	oppCreature.Card.BasePower, oppCreature.Card.BaseToughness = 4, 4

	// A creature an OPPONENT controls dying must not trigger the edict.
	dictateOfErebosDies(gs, dictate, map[string]interface{}{"controller_seat": 1, "card": &gameengine.Card{Name: "Dead"}})

	if !afOnBF(gs, 1, oppCreature.Card) {
		t.Error("edict must not fire when a non-controlled creature dies")
	}
}

// ---------------------------------------------------------------------------
// Dark Prophecy
// ---------------------------------------------------------------------------

func TestDarkProphecy_YourCreatureDies_DrawAndLoseLife(t *testing.T) {
	gs := newGame(t, 2)
	dp := addPerm(gs, 0, "Dark Prophecy", "enchantment")
	gs.Seats[0].Life = 40
	gs.Seats[0].Library = append(gs.Seats[0].Library, addCard(gs, 0, "Swamp", "land"))
	handBefore := len(gs.Seats[0].Hand)

	darkProphecyDies(gs, dp, map[string]interface{}{"controller_seat": 0, "card": &gameengine.Card{Name: "Dead"}})

	if len(gs.Seats[0].Hand) != handBefore+1 {
		t.Errorf("expected to draw a card, hand %d→%d", handBefore, len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != 39 {
		t.Errorf("expected to lose 1 life (40→39), got %d", gs.Seats[0].Life)
	}
}

// ---------------------------------------------------------------------------
// Crackling Doom
// ---------------------------------------------------------------------------

func TestCracklingDoom_DamageEachOpponentAndSacGreatestPower(t *testing.T) {
	gs := newGame(t, 3)
	gs.Seats[1].Life, gs.Seats[2].Life = 40, 40

	// Seat 1: a small and a big creature — big one must be sacrificed.
	small1 := addPerm(gs, 1, "Goblin", "creature", "goblin")
	small1.Card.BasePower, small1.Card.BaseToughness = 1, 1
	big1 := addPerm(gs, 1, "Dragon", "creature", "dragon")
	big1.Card.BasePower, big1.Card.BaseToughness = 6, 6

	// Seat 2: one creature.
	big2 := addPerm(gs, 2, "Wurm", "creature", "wurm")
	big2.Card.BasePower, big2.Card.BaseToughness = 7, 7

	item := &gameengine.StackItem{Kind: "spell", Controller: 0,
		Card: &gameengine.Card{Name: "Crackling Doom", Owner: 0, Types: []string{"instant"}}}
	cracklingDoomResolve(gs, item)

	if gs.Seats[1].Life != 38 || gs.Seats[2].Life != 38 {
		t.Errorf("each opponent should take 2 damage; got s1=%d s2=%d", gs.Seats[1].Life, gs.Seats[2].Life)
	}
	if afOnBF(gs, 1, big1.Card) {
		t.Error("seat 1 should have sacrificed its greatest-power creature (Dragon)")
	}
	if !afOnBF(gs, 1, small1.Card) {
		t.Error("seat 1 should keep its small creature")
	}
	if afOnBF(gs, 2, big2.Card) {
		t.Error("seat 2 should have sacrificed its only/greatest creature")
	}
}
