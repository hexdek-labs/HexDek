package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 deferred mana-pool gap (from /tmp/fable-review/mana-pool-audit-r63.md):
//
// Kruphix, God of Horizons + Horizon Stone — "If you would lose unspent
// mana, that mana becomes COLORLESS instead." At a step/phase boundary
// (CR §500.4 / §106.4) the would-be-lost mana CONVERTS to colorless and
// persists, rather than emptying. Distinct from Omnath/Leyline Tyrant,
// which RETAIN mana in its original color.
//
// Headline regression: Kruphix out, float {R}{G}{U}, cross a step
// boundary → pool reads 3 colorless.

func TestKruphix_UnspentManaBecomesColorlessOnDrain(t *testing.T) {
	gs := newGame(t, 2)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "god")
	kruphixETBSetSeatFlags(gs, kruphix)

	// Float {R}{G}{U}.
	gameengine.AddMana(gs, gs.Seats[0], "R", 1, "test")
	gameengine.AddMana(gs, gs.Seats[0], "G", 1, "test")
	gameengine.AddMana(gs, gs.Seats[0], "U", 1, "test")

	gameengine.DrainAllPools(gs, "ending", "cleanup")

	pool := gs.Seats[0].Mana
	if pool.R != 0 || pool.G != 0 || pool.U != 0 {
		t.Errorf("colored buckets should empty into colorless: R=%d G=%d U=%d (want all 0)",
			pool.R, pool.G, pool.U)
	}
	if pool.C != 3 {
		t.Errorf("3 floated mana should become 3 colorless: want C=3, got C=%d", pool.C)
	}
	if got := pool.Total(); got != 3 {
		t.Errorf("total mana must be conserved across the boundary: want 3, got %d", got)
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("legacy ManaPool int should mirror typed total: want 3, got %d", gs.Seats[0].ManaPool)
	}
}

func TestHorizonStone_UnspentManaBecomesColorlessOnDrain(t *testing.T) {
	gs := newGame(t, 2)
	stone := addPerm(gs, 0, "Horizon Stone", "artifact")
	horizonStoneETB(gs, stone)

	gameengine.AddMana(gs, gs.Seats[0], "W", 2, "test")
	gameengine.AddMana(gs, gs.Seats[0], "B", 1, "test")

	gameengine.DrainAllPools(gs, "ending", "cleanup")

	pool := gs.Seats[0].Mana
	if pool.W != 0 || pool.B != 0 {
		t.Errorf("colored buckets should convert: W=%d B=%d (want 0,0)", pool.W, pool.B)
	}
	if pool.C != 3 {
		t.Errorf("Horizon Stone: 3 floated → 3 colorless, got C=%d", pool.C)
	}
}

// Idempotency: already-colorless mana stays put across additional
// boundaries (it "would be lost" → "becomes colorless" → no change).
func TestKruphix_ColorlessPersistsAcrossMultipleBoundaries(t *testing.T) {
	gs := newGame(t, 2)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "god")
	kruphixETBSetSeatFlags(gs, kruphix)

	gameengine.AddMana(gs, gs.Seats[0], "R", 3, "test")
	gameengine.DrainAllPools(gs, "ending", "cleanup")
	if gs.Seats[0].Mana.C != 3 {
		t.Fatalf("after first drain: want C=3, got %d", gs.Seats[0].Mana.C)
	}
	// Second boundary — colorless must survive unchanged.
	gameengine.DrainAllPools(gs, "precombat_main", "")
	if gs.Seats[0].Mana.C != 3 {
		t.Errorf("colorless must persist across a second boundary: want C=3, got %d", gs.Seats[0].Mana.C)
	}
	if got := gs.Seats[0].Mana.Total(); got != 3 {
		t.Errorf("total still 3 after two boundaries, got %d", got)
	}
}

// A pre-existing {C} bucket plus converted color sums correctly.
func TestKruphix_ExistingColorlessAddsToConverted(t *testing.T) {
	gs := newGame(t, 2)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "god")
	kruphixETBSetSeatFlags(gs, kruphix)

	gameengine.AddMana(gs, gs.Seats[0], "C", 2, "test") // already colorless
	gameengine.AddMana(gs, gs.Seats[0], "G", 4, "test") // converts
	gameengine.DrainAllPools(gs, "ending", "cleanup")

	if gs.Seats[0].Mana.C != 6 {
		t.Errorf("2 existing C + 4 converted G = 6 colorless, got %d", gs.Seats[0].Mana.C)
	}
	if gs.Seats[0].Mana.G != 0 {
		t.Errorf("green should be gone, got %d", gs.Seats[0].Mana.G)
	}
}

// Once Kruphix leaves, the converter is torn down and mana drains to
// zero normally (no lingering colorless conversion).
func TestKruphix_LTBStopsConversion(t *testing.T) {
	gs := newGame(t, 2)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "god")
	kruphixETBSetSeatFlags(gs, kruphix)

	kruphixLTBClearFlags(gs, kruphix, map[string]interface{}{"perm": kruphix})

	gameengine.AddMana(gs, gs.Seats[0], "R", 3, "test")
	gameengine.DrainAllPools(gs, "ending", "cleanup")
	if got := gs.Seats[0].Mana.Total(); got != 0 {
		t.Errorf("after Kruphix LTB, mana should empty normally: want 0, got %d", got)
	}
}

// Horizon Stone LTB likewise tears the converter down.
func TestHorizonStone_LTBStopsConversion(t *testing.T) {
	gs := newGame(t, 2)
	stone := addPerm(gs, 0, "Horizon Stone", "artifact")
	horizonStoneETB(gs, stone)
	horizonStoneLTB(gs, stone, map[string]interface{}{"perm": stone})

	gameengine.AddMana(gs, gs.Seats[0], "U", 2, "test")
	gameengine.DrainAllPools(gs, "ending", "cleanup")
	if got := gs.Seats[0].Mana.Total(); got != 0 {
		t.Errorf("after Horizon Stone LTB, mana should empty: want 0, got %d", got)
	}
}

// Conversion is controller-scoped: an opponent's pool still empties
// normally while only the Kruphix controller's mana converts.
func TestKruphix_OpponentPoolStillDrains(t *testing.T) {
	gs := newGame(t, 2)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "god")
	kruphixETBSetSeatFlags(gs, kruphix)

	gameengine.AddMana(gs, gs.Seats[0], "G", 2, "test") // controller → colorless
	gameengine.AddMana(gs, gs.Seats[1], "G", 2, "test") // opponent → drains

	gameengine.DrainAllPools(gs, "ending", "cleanup")

	if gs.Seats[0].Mana.C != 2 {
		t.Errorf("controller mana should convert: want C=2, got %d", gs.Seats[0].Mana.C)
	}
	if got := gs.Seats[1].Mana.Total(); got != 0 {
		t.Errorf("opponent mana should empty normally: want 0, got %d", got)
	}
}

// A pool_converted_colorless event is logged (and NOT a pool_drain,
// since nothing was actually lost).
func TestKruphix_LogsConversionNotDrain(t *testing.T) {
	gs := newGame(t, 2)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "god")
	kruphixETBSetSeatFlags(gs, kruphix)
	gameengine.AddMana(gs, gs.Seats[0], "R", 2, "test")
	gameengine.DrainAllPools(gs, "ending", "cleanup")

	if hasEvent(gs, "pool_converted_colorless") == 0 {
		t.Errorf("expected a pool_converted_colorless event")
	}
	if hasEvent(gs, "pool_drain") != 0 {
		t.Errorf("no pool_drain expected — nothing was lost, only converted")
	}
}

// -----------------------------------------------------------------------------
// Kruphix "You have no maximum hand size" — verify it is actually wired
// into the §514.1 cleanup discard (the flag was previously set but never
// consulted by CleanupHandSize).
// -----------------------------------------------------------------------------

func TestKruphix_NoMaximumHandSizeWired(t *testing.T) {
	gs := newGame(t, 2)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "god")
	kruphixETBSetSeatFlags(gs, kruphix)

	// 10 cards in hand — over the §402.2 default of 7.
	for i := 0; i < 10; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
			Name:  "Filler",
			Owner: 0,
			Types: []string{"instant"},
		})
	}
	gameengine.CleanupHandSize(gs, 0, 7)
	if len(gs.Seats[0].Hand) != 10 {
		t.Errorf("Kruphix grants no maximum hand size — no discard expected; hand=%d, want 10",
			len(gs.Seats[0].Hand))
	}
}

// Control: without Kruphix, the same 10-card hand discards down to 7.
func TestCleanupHandSize_DefaultStillDiscards(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 10; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
			Name:  "Filler",
			Owner: 0,
			Types: []string{"instant"},
		})
	}
	gameengine.CleanupHandSize(gs, 0, 7)
	if len(gs.Seats[0].Hand) != 7 {
		t.Errorf("default cleanup should discard to 7; hand=%d", len(gs.Seats[0].Hand))
	}
}

// After Kruphix leaves, the no-max-hand-size flag is cleared and the
// cleanup discard resumes.
func TestKruphix_LTBRestoresHandLimit(t *testing.T) {
	gs := newGame(t, 2)
	kruphix := addPerm(gs, 0, "Kruphix, God of Horizons", "creature", "god")
	kruphixETBSetSeatFlags(gs, kruphix)
	kruphixLTBClearFlags(gs, kruphix, map[string]interface{}{"perm": kruphix})

	for i := 0; i < 10; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
			Name:  "Filler",
			Owner: 0,
			Types: []string{"instant"},
		})
	}
	gameengine.CleanupHandSize(gs, 0, 7)
	if len(gs.Seats[0].Hand) != 7 {
		t.Errorf("after Kruphix LTB the 7-card limit should resume; hand=%d", len(gs.Seats[0].Hand))
	}
}
