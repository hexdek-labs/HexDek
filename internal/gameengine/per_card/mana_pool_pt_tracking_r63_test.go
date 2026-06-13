package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 mana-pool audit regressions:
//
//   - Omnath, Locus of Mana "+1/+1 for each unspent green mana" now
//     tracks the live pool via a layer-7c pool-driven self-boost (was an
//     emitPartial stub). Drain/persist of the green itself was already
//     correct; this pins the P/T-reads-the-pool half.
//   - Leyline Tyrant red retention exemption + self-dies "pay any amount
//     of {R}, deal that much" payoff (previously no handler at all).

// withGreenBase1 stamps the realistic 1/1 base on a hand-built Omnath.
func omnathPerm(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	p := addPerm(gs, seat, "Omnath, Locus of Mana", "creature", "legendary", "elemental")
	p.Card.BasePower = 1
	p.Card.BaseToughness = 1
	return p
}

func TestOmnath_PowerTracksUnspentGreen(t *testing.T) {
	gs := newGame(t, 2)
	omnath := omnathPerm(gs, 0)
	omnathLocusOfManaETB(gs, omnath)

	// Base 1/1 with no green floated.
	if got := gs.PowerOf(omnath); got != 1 {
		t.Fatalf("base power: want 1, got %d", got)
	}
	if got := gs.ToughnessOf(omnath); got != 1 {
		t.Fatalf("base toughness: want 1, got %d", got)
	}

	// Float 5 green → 6/6.
	gameengine.AddMana(gs, gs.Seats[0], "G", 5, "test")
	if got := gs.PowerOf(omnath); got != 6 {
		t.Errorf("with 5 green: want power 6, got %d", got)
	}
	if got := gs.ToughnessOf(omnath); got != 6 {
		t.Errorf("with 5 green: want toughness 6, got %d", got)
	}

	// Non-green mana does NOT pump Omnath.
	gameengine.AddMana(gs, gs.Seats[0], "R", 3, "test")
	if got := gs.PowerOf(omnath); got != 6 {
		t.Errorf("red mana must not pump Omnath: want 6, got %d", got)
	}
}

func TestOmnath_PowerDropsWhenGreenSpent(t *testing.T) {
	gs := newGame(t, 2)
	omnath := omnathPerm(gs, 0)
	omnathLocusOfManaETB(gs, omnath)
	gameengine.AddMana(gs, gs.Seats[0], "G", 5, "test")
	if got := gs.ToughnessOf(omnath); got != 6 {
		t.Fatalf("precondition: want 6, got %d", got)
	}
	// Spend 2 green via the canonical consume path → 3 green left → 4/4.
	gameengine.ConsumeColoredMana(gs, 0, "G", 2)
	if got := gs.PowerOf(omnath); got != 4 {
		t.Errorf("after spending 2 green: want power 4, got %d", got)
	}
}

// Omnath's exemption keeps green across a §106.4 drain AND the P/T
// follows the retained green back down (non-green is gone).
func TestOmnath_PTTracksAcrossPhaseDrain(t *testing.T) {
	gs := newGame(t, 2)
	omnath := omnathPerm(gs, 0)
	omnathLocusOfManaETB(gs, omnath)
	gameengine.AddMana(gs, gs.Seats[0], "G", 4, "test")
	gameengine.AddMana(gs, gs.Seats[0], "R", 3, "test")
	// 4 green + 3 red, Omnath = 1+4 = 5/5.
	if got := gs.PowerOf(omnath); got != 5 {
		t.Fatalf("precondition power: want 5, got %d", got)
	}
	gameengine.DrainAllPools(gs, "ending", "cleanup")
	// Green retained (exemption), red drained. Omnath still 5/5.
	if gs.Seats[0].Mana.G != 4 {
		t.Errorf("green should persist: want 4, got %d", gs.Seats[0].Mana.G)
	}
	if gs.Seats[0].Mana.R != 0 {
		t.Errorf("red should drain: want 0, got %d", gs.Seats[0].Mana.R)
	}
	if got := gs.PowerOf(omnath); got != 5 {
		t.Errorf("post-drain power should still read retained green: want 5, got %d", got)
	}
}

// LTB tears down both the exemption and the P/T boost cleanly, and the
// gate counter returns to zero.
func TestOmnath_LTBClearsPTBoostAndGate(t *testing.T) {
	gs := newGame(t, 2)
	omnath := omnathPerm(gs, 0)
	omnathLocusOfManaETB(gs, omnath)
	gameengine.AddMana(gs, gs.Seats[0], "G", 3, "test")
	if got := gs.PowerOf(omnath); got != 4 {
		t.Fatalf("precondition: want 4, got %d", got)
	}
	if gs.PoolDrivenPTEffects != 1 {
		t.Fatalf("gate counter should be 1 while Omnath is out, got %d", gs.PoolDrivenPTEffects)
	}
	omnathLocusOfManaLTB(gs, omnath, map[string]interface{}{"perm": omnath})
	if gs.PoolDrivenPTEffects != 0 {
		t.Errorf("gate counter should return to 0 after LTB, got %d", gs.PoolDrivenPTEffects)
	}
	// Boost gone → reads base 1/1 again.
	gs.InvalidateCharacteristicsCache()
	if got := gs.PowerOf(omnath); got != 1 {
		t.Errorf("after LTB power should drop to base 1, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// Leyline Tyrant
// -----------------------------------------------------------------------------

func TestLeylineTyrant_RedManaPersistsAcrossDrain(t *testing.T) {
	gs := newGame(t, 2)
	tyrant := addPerm(gs, 0, "Leyline Tyrant", "creature", "dragon")
	tyrant.Card.BasePower = 4
	tyrant.Card.BaseToughness = 4
	leylineTyrantETB(gs, tyrant)

	gameengine.AddMana(gs, gs.Seats[0], "R", 6, "test")
	gameengine.AddMana(gs, gs.Seats[0], "G", 2, "test")
	gameengine.DrainAllPools(gs, "ending", "cleanup")
	if gs.Seats[0].Mana.R != 6 {
		t.Errorf("red should persist via Leyline Tyrant: want 6, got %d", gs.Seats[0].Mana.R)
	}
	if gs.Seats[0].Mana.G != 0 {
		t.Errorf("non-red (green) should drain: want 0, got %d", gs.Seats[0].Mana.G)
	}
}

func TestLeylineTyrant_DiesDealsDamageEqualToRed(t *testing.T) {
	gs := newGame(t, 2)
	tyrant := addPerm(gs, 0, "Leyline Tyrant", "creature", "dragon")
	tyrant.Card.BasePower = 4
	tyrant.Card.BaseToughness = 4
	leylineTyrantETB(gs, tyrant)
	gameengine.AddMana(gs, gs.Seats[0], "R", 5, "test")

	startLife := gs.Seats[1].Life
	// Drive the real death path so creature_dies fires through the
	// engine dispatcher (not a hand-rolled FireCardTrigger).
	gameengine.DestroyPermanent(gs, tyrant, nil)
	gameengine.StateBasedActions(gs)
	gameengine.DrainStack(gs)

	if got := startLife - gs.Seats[1].Life; got != 5 {
		t.Errorf("Leyline Tyrant death should deal 5 (red pool) to opponent; dealt %d", got)
	}
	if gs.Seats[0].Mana.R != 0 {
		t.Errorf("red should be consumed paying the death trigger: want 0, got %d", gs.Seats[0].Mana.R)
	}
}

func TestLeylineTyrant_DiesNoRedNoDamage(t *testing.T) {
	gs := newGame(t, 2)
	tyrant := addPerm(gs, 0, "Leyline Tyrant", "creature", "dragon")
	tyrant.Card.BasePower = 4
	tyrant.Card.BaseToughness = 4
	leylineTyrantETB(gs, tyrant)
	// No red floated.
	startLife := gs.Seats[1].Life
	gameengine.DestroyPermanent(gs, tyrant, nil)
	gameengine.StateBasedActions(gs)
	gameengine.DrainStack(gs)
	if gs.Seats[1].Life != startLife {
		t.Errorf("no red → no damage; opponent life changed from %d to %d", startLife, gs.Seats[1].Life)
	}
}
