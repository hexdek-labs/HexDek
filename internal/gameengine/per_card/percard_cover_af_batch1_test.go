package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// percard_cover_af_batch1_test.go — regression pins for the A–F DOA
// coverage batch: Champion of Lambholt, Elvish Warmaster, Endrek Sahr,
// Dread Return. Each ability was inert scaffold before its handler.

func afSeatCreatureCount(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsCreature() {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Champion of Lambholt
// ---------------------------------------------------------------------------

func TestChampionOfLambholt_OtherCreatureETB_AddsCounter(t *testing.T) {
	gs := newGame(t, 2)
	champ := addPerm(gs, 0, "Champion of Lambholt", "creature", "human", "warrior")
	champ.Card.BasePower, champ.Card.BaseToughness = 1, 1
	other := addPerm(gs, 0, "Llanowar Elves", "creature", "elf")

	championOfLambholtETB(gs, champ, map[string]interface{}{"perm": other, "controller_seat": 0})

	if champ.Counters["+1/+1"] != 1 {
		t.Fatalf("expected 1 +1/+1 counter, got %d", champ.Counters["+1/+1"])
	}
}

func TestChampionOfLambholt_SelfAndOpponentETB_NoCounter(t *testing.T) {
	gs := newGame(t, 2)
	champ := addPerm(gs, 0, "Champion of Lambholt", "creature", "human", "warrior")
	oppCreature := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")

	// Champion's own ETB — "another creature", so no counter.
	championOfLambholtETB(gs, champ, map[string]interface{}{"perm": champ, "controller_seat": 0})
	// Opponent's creature — "you control", so no counter.
	championOfLambholtETB(gs, champ, map[string]interface{}{"perm": oppCreature, "controller_seat": 1})

	if champ.Counters["+1/+1"] != 0 {
		t.Fatalf("expected 0 counters (self + opponent excluded), got %d", champ.Counters["+1/+1"])
	}
}

// ---------------------------------------------------------------------------
// Elvish Warmaster
// ---------------------------------------------------------------------------

func TestElvishWarmaster_OtherElfETB_MakesTokenOncePerTurn(t *testing.T) {
	gs := newGame(t, 2)
	wm := addPerm(gs, 0, "Elvish Warmaster", "creature", "elf", "warrior")
	wm.Card.BasePower, wm.Card.BaseToughness = 1, 1
	elf1 := addPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	elf1.Card.BasePower, elf1.Card.BaseToughness = 1, 1
	before := afSeatCreatureCount(gs, 0)

	elvishWarmasterETB(gs, wm, map[string]interface{}{"perm": elf1, "controller_seat": 0})
	afterFirst := afSeatCreatureCount(gs, 0)
	if afterFirst != before+1 {
		t.Fatalf("expected one Elf Warrior token, creatures %d→%d", before, afterFirst)
	}

	// Second Elf same turn — "triggers only once each turn": no token.
	elf2 := addPerm(gs, 0, "Fyndhorn Elves", "creature", "elf")
	elf2.Card.BasePower, elf2.Card.BaseToughness = 1, 1
	elvishWarmasterETB(gs, wm, map[string]interface{}{"perm": elf2, "controller_seat": 0})
	if afSeatCreatureCount(gs, 0) != afterFirst+1 {
		// +1 only from elf2 itself being added by addPerm, no new token.
		t.Fatalf("once-per-turn gate failed: second Elf should not mint a token")
	}
}

func TestElvishWarmaster_NonElfAndOpponent_NoToken(t *testing.T) {
	gs := newGame(t, 2)
	wm := addPerm(gs, 0, "Elvish Warmaster", "creature", "elf", "warrior")
	before := afSeatCreatureCount(gs, 0)

	nonElf := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	elvishWarmasterETB(gs, wm, map[string]interface{}{"perm": nonElf, "controller_seat": 0})

	oppElf := addPerm(gs, 1, "Llanowar Elves", "creature", "elf")
	elvishWarmasterETB(gs, wm, map[string]interface{}{"perm": oppElf, "controller_seat": 1})

	// Only the two manually-added creatures (nonElf on seat 0); no token.
	if afSeatCreatureCount(gs, 0) != before+1 {
		t.Fatalf("no token expected for non-Elf / opponent Elf; seat0 creatures unexpected")
	}
}

// ---------------------------------------------------------------------------
// Endrek Sahr, Master Breeder
// ---------------------------------------------------------------------------

func TestEndrekSahr_CreatureCast_MakesMVThrulls(t *testing.T) {
	gs := newGame(t, 2)
	endrek := addPerm(gs, 0, "Endrek Sahr, Master Breeder", "creature", "human", "wizard")
	endrek.Card.BasePower, endrek.Card.BaseToughness = 3, 3

	spell := addCard(gs, 0, "Colossal Dreadmaw", "creature", "dinosaur")
	spell.CMC = 3

	endrekSahrCast(gs, endrek, map[string]interface{}{"caster_seat": 0, "card": spell})

	if got := countThrulls(gs, 0); got != 3 {
		t.Fatalf("expected 3 Thrull tokens for a MV-3 creature spell, got %d", got)
	}
}

func TestEndrekSahr_SevenThrulls_SelfSacrifices(t *testing.T) {
	gs := newGame(t, 2)
	endrek := addPerm(gs, 0, "Endrek Sahr, Master Breeder", "creature", "human", "wizard")
	endrek.Card.BasePower, endrek.Card.BaseToughness = 3, 3

	spell := addCard(gs, 0, "Craterhoof Behemoth", "creature", "beast")
	spell.CMC = 8 // 8 Thrulls ≥ 7 → Endrek sacrifices itself

	endrekSahrCast(gs, endrek, map[string]interface{}{"caster_seat": 0, "card": spell})

	if countThrulls(gs, 0) != 8 {
		t.Errorf("expected 8 Thrulls, got %d", countThrulls(gs, 0))
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == endrek {
			t.Fatal("Endrek Sahr should have sacrificed itself at 7+ Thrulls")
		}
	}
}

func TestEndrekSahr_NoncreatureCast_NoThrulls(t *testing.T) {
	gs := newGame(t, 2)
	endrek := addPerm(gs, 0, "Endrek Sahr, Master Breeder", "creature", "human", "wizard")
	endrek.Card.BasePower, endrek.Card.BaseToughness = 3, 3
	spell := addCard(gs, 0, "Sign in Blood", "sorcery")
	spell.CMC = 2
	endrekSahrCast(gs, endrek, map[string]interface{}{"caster_seat": 0, "card": spell})
	if countThrulls(gs, 0) != 0 {
		t.Fatalf("noncreature spell must not make Thrulls, got %d", countThrulls(gs, 0))
	}
}

// ---------------------------------------------------------------------------
// Dread Return
// ---------------------------------------------------------------------------

func TestDreadReturn_ReanimatesStrongestFromOwnGraveyard(t *testing.T) {
	gs := newGame(t, 2)

	weak := addCard(gs, 0, "Grizzly Bears", "creature", "bear")
	weak.BasePower, weak.BaseToughness = 2, 2
	strong := addCard(gs, 0, "Inkwell Leviathan", "creature", "leviathan")
	strong.BasePower, strong.BaseToughness = 7, 11
	noncreature := addCard(gs, 0, "Sign in Blood", "sorcery")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, weak, strong, noncreature)

	// An opponent creature in THEIR graveyard must be ineligible.
	oppCreature := addCard(gs, 1, "Emrakul", "creature", "eldrazi")
	oppCreature.BasePower, oppCreature.BaseToughness = 15, 15
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, oppCreature)

	item := &gameengine.StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       &gameengine.Card{Name: "Dread Return", Owner: 0, Types: []string{"sorcery"}},
	}
	dreadReturnResolve(gs, item)

	// Strongest OWN creature returns under caster control.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == strong {
			found = true
		}
		if p != nil && p.Card == oppCreature {
			t.Fatal("opponent's graveyard creature must not be reanimated by Dread Return")
		}
	}
	if !found {
		t.Fatal("expected Inkwell Leviathan reanimated under caster control")
	}
	// It should have left the graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == strong {
			t.Error("reanimated card should have left the graveyard")
		}
	}
}
