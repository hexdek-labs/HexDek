package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// batch_ah_r60_test.go — smoke tests for the 5 batch-AH handlers.
// Pattern mirrors batch_y_r60_test.go: fire the relevant trigger or
// ETB hook directly with seeded ctx, then assert against game state
// or the event log via the per-card emit() breadcrumb.

// -----------------------------------------------------------------------------
// Polukranos, World Eater
// -----------------------------------------------------------------------------

func TestPolukranos_FightsHighestPowerPerOpponent(t *testing.T) {
	gs := newGame(t, 4) // 3 opponents
	polu := addPerm(gs, 0, "Polukranos, World Eater", "creature", "legendary", "hydra")
	polu.Card.BasePower = 5
	polu.Card.BaseToughness = 5

	// Seat 1: 3/3 bear + 6/6 fatty (Polukranos should pick the 6/6).
	bear := addPerm(gs, 1, "Bear", "creature")
	bear.Card.BasePower = 3
	bear.Card.BaseToughness = 3
	fatty := addPerm(gs, 1, "Fatty", "creature")
	fatty.Card.BasePower = 6
	fatty.Card.BaseToughness = 6

	// Seat 2: 2/2.
	smol := addPerm(gs, 2, "Smol", "creature")
	smol.Card.BasePower = 2
	smol.Card.BaseToughness = 2

	// Seat 3: empty battlefield.

	// Activate with X = 3 (would fight all opponents' picks if available).
	gameengine.InvokeActivatedHook(gs, polu, 0, map[string]interface{}{"x": 3})

	// Polukranos took (6 + 2) = 8 damage marked.
	if got := polu.Counters["damage_marked"]; got != 8 {
		t.Errorf("Polukranos should have 8 damage_marked (6 from fatty + 2 from smol), got %d", got)
	}
	// Fatty took 5, Smol took 5, bear took 0 (not picked).
	if got := fatty.Counters["damage_marked"]; got != 5 {
		t.Errorf("Fatty should have 5 damage_marked, got %d", got)
	}
	if got := smol.Counters["damage_marked"]; got != 5 {
		t.Errorf("Smol should have 5 damage_marked, got %d", got)
	}
	if got := bear.Counters["damage_marked"]; got != 0 {
		t.Errorf("Bear (lower-power than Fatty on same seat) should NOT have been fought, got %d damage", got)
	}
}

func TestPolukranos_XLimitsTargetCount(t *testing.T) {
	gs := newGame(t, 4)
	polu := addPerm(gs, 0, "Polukranos, World Eater", "creature", "legendary", "hydra")
	polu.Card.BasePower = 5
	polu.Card.BaseToughness = 5

	// Three opponents each with a creature.
	for seat := 1; seat <= 3; seat++ {
		c := addPerm(gs, seat, "Bear", "creature")
		c.Card.BasePower = 2
		c.Card.BaseToughness = 2
	}

	// X = 1: only the highest-power opponent creature (here all 2s, tie
	// → first one wins via stable sort) gets fought.
	gameengine.InvokeActivatedHook(gs, polu, 0, map[string]interface{}{"x": 1})

	fought := 0
	for seat := 1; seat <= 3; seat++ {
		for _, p := range gs.Seats[seat].Battlefield {
			if p != nil && p.Counters["damage_marked"] > 0 {
				fought++
			}
		}
	}
	if fought != 1 {
		t.Errorf("X=1 should fight exactly 1 target, got %d", fought)
	}
}

func TestPolukranos_NoTargetsAvailable(t *testing.T) {
	gs := newGame(t, 4)
	polu := addPerm(gs, 0, "Polukranos, World Eater", "creature", "legendary", "hydra")
	polu.Card.BasePower = 5

	// All opponents empty.
	gameengine.InvokeActivatedHook(gs, polu, 0, map[string]interface{}{"x": 3})

	if polu.Counters["damage_marked"] != 0 {
		t.Errorf("no targets → no damage on Polukranos, got %d", polu.Counters["damage_marked"])
	}
}

// -----------------------------------------------------------------------------
// Hungering Hydra
// -----------------------------------------------------------------------------

func TestHungeringHydra_ETBWithXCounters(t *testing.T) {
	gs := newGame(t, 2)
	hydra := addPerm(gs, 0, "Hungering Hydra", "creature", "hydra")
	hydra.Card.BasePower = 0
	hydra.Card.BaseToughness = 0
	hydra.Flags["x_paid"] = 4

	gameengine.InvokeETBHook(gs, hydra)

	if got := hydra.Counters["+1/+1"]; got != 4 {
		t.Errorf("Hungering Hydra should ETB with X=4 +1/+1 counters, got %d", got)
	}
}

func TestHungeringHydra_GrowsOnDamageTaken(t *testing.T) {
	gs := newGame(t, 2)
	hydra := addPerm(gs, 0, "Hungering Hydra", "creature", "hydra")
	hydra.Card.BasePower = 0
	hydra.Card.BaseToughness = 0
	hydra.Counters["+1/+1"] = 3

	// 5 damage dealt to hydra → +5 +1/+1 counters.
	gameengine.FireCardTrigger(gs, "damage_taken", map[string]interface{}{
		"target_perm": hydra,
		"amount":      5,
	})

	if got := hydra.Counters["+1/+1"]; got != 8 {
		t.Errorf("Hungering Hydra should have 3+5=8 counters after taking 5 damage, got %d", got)
	}
}

func TestHungeringHydra_OnlyFiresOnSelfDamage(t *testing.T) {
	gs := newGame(t, 2)
	hydra := addPerm(gs, 0, "Hungering Hydra", "creature", "hydra")
	hydra.Counters["+1/+1"] = 2
	other := addPerm(gs, 0, "Bear", "creature")

	// Damage to a different creature should NOT grow the hydra.
	gameengine.FireCardTrigger(gs, "damage_taken", map[string]interface{}{
		"target_perm": other,
		"amount":      5,
	})

	if got := hydra.Counters["+1/+1"]; got != 2 {
		t.Errorf("Hungering Hydra must only react to damage on ITSELF, got %d counters", got)
	}
}

// -----------------------------------------------------------------------------
// Grim Hireling
// -----------------------------------------------------------------------------

func TestGrimHireling_CombatDamageCreatesTreasure(t *testing.T) {
	gs := newGame(t, 2)
	hire := addPerm(gs, 0, "Grim Hireling", "creature", "legendary", "human", "mercenary")
	hire.Card.BasePower = 4

	beforeTokens := countTreasuresAH(gs, 0)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"defender_seat": 1,
		"amount":        4,
	})

	afterTokens := countTreasuresAH(gs, 0)
	if afterTokens-beforeTokens != 1 {
		t.Errorf("Grim Hireling should create 1 Treasure per damage event, got delta %d", afterTokens-beforeTokens)
	}
}

func TestGrimHireling_OpponentDamageDoesNotCreateTreasure(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Grim Hireling", "creature", "legendary")

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   1, // opponent dealing damage — Grim is on seat 0
		"defender_seat": 0,
		"amount":        4,
	})

	if countTreasuresAH(gs, 0) != 0 {
		t.Errorf("Grim Hireling must only trigger on OUR creatures' combat damage")
	}
}

// -----------------------------------------------------------------------------
// Glint-Horn Buccaneer
// -----------------------------------------------------------------------------

func TestGlintHornBuccaneer_DiscardPingsEachOpponent(t *testing.T) {
	gs := newGame(t, 4) // 3 opponents
	addPerm(gs, 0, "Glint-Horn Buccaneer", "creature", "minotaur", "pirate")
	addLibrary(gs, 0, "A") // for the "may draw"

	livesBefore := []int{gs.Seats[1].Life, gs.Seats[2].Life, gs.Seats[3].Life}

	gameengine.FireCardTrigger(gs, "card_discarded", map[string]interface{}{
		"discarder_seat": 0,
	})

	for i, opp := range []int{1, 2, 3} {
		if gs.Seats[opp].Life != livesBefore[i]-1 {
			t.Errorf("opponent %d should have lost 1 life (Glint-Horn ping); was %d → got %d",
				opp, livesBefore[i], gs.Seats[opp].Life)
		}
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("should have drawn the may-draw card; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestGlintHornBuccaneer_OpponentDiscardDoesNotFire(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Glint-Horn Buccaneer", "creature")
	lifeBefore := gs.Seats[1].Life

	gameengine.FireCardTrigger(gs, "card_discarded", map[string]interface{}{
		"discarder_seat": 1, // opponent discarding — Glint is on seat 0
	})

	if gs.Seats[1].Life != lifeBefore {
		t.Errorf("Glint-Horn must only fire on controller's discards")
	}
}

// -----------------------------------------------------------------------------
// Old Stickfingers
// -----------------------------------------------------------------------------

func TestOldStickfingers_MillsUntilXCreaturesReanimates(t *testing.T) {
	gs := newGame(t, 2)
	// Stack library: top is index 0 in this engine's MoveCard semantics.
	// Build: [Bear (creature), Plains (land), Wolf (creature), Mountain (land)]
	// X = 2 → mill until 2 creatures hit graveyard.
	addLibrary(gs, 0, "Bear", "Plains", "Wolf", "Mountain")
	// Mark types on library cards directly.
	gs.Seats[0].Library[0].Types = []string{"creature"}
	gs.Seats[0].Library[1].Types = []string{"land"}
	gs.Seats[0].Library[2].Types = []string{"creature"}
	gs.Seats[0].Library[3].Types = []string{"land"}

	stickfingers := addPerm(gs, 0, "Old Stickfingers", "creature", "legendary", "horror")
	stickfingers.Flags["x_paid"] = 2

	beforeBF := len(gs.Seats[0].Battlefield)
	gameengine.InvokeETBHook(gs, stickfingers)

	// Library should have 1 card left (Mountain) — Bear + Plains + Wolf milled.
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected 1 card left in library after milling 3, got %d", len(gs.Seats[0].Library))
	}
	// 2 creatures reanimated → 2 new perms on battlefield (besides Stickfingers).
	if len(gs.Seats[0].Battlefield)-beforeBF != 2 {
		t.Errorf("expected 2 reanimated creatures, got delta %d", len(gs.Seats[0].Battlefield)-beforeBF)
	}
	// Plains should be in graveyard (milled, not creature).
	gyHasPlains := false
	for _, c := range gs.Seats[0].Graveyard {
		if c != nil && c.Name == "Plains" {
			gyHasPlains = true
			break
		}
	}
	if !gyHasPlains {
		t.Errorf("Plains (non-creature mill) should be in graveyard")
	}
}

func TestOldStickfingers_XZeroNoMill(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Bear", "Wolf")
	gs.Seats[0].Library[0].Types = []string{"creature"}
	gs.Seats[0].Library[1].Types = []string{"creature"}

	stickfingers := addPerm(gs, 0, "Old Stickfingers", "creature", "legendary", "horror")
	// X not set → 0.

	gameengine.InvokeETBHook(gs, stickfingers)

	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("X=0 should mill nothing, library=%d", len(gs.Seats[0].Library))
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func countTreasuresAH(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && cardHasType(p.Card, "treasure") {
			n++
		}
	}
	return n
}
