package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func mortiOnBattlefield(gs *gameengine.GameState) bool {
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Mortivore" {
			return true
		}
	}
	return false
}

// PROBE B: dynamic growth — a creature dying into a graveyard grows Mortivore.
func TestCDAProbe_DynamicGrowthOnDeath(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gyCreature("A"), gyCreature("B"))
	gameengine.InvokeETBHook(gs, morti)
	if gs.ToughnessOf(morti) != 2 {
		t.Fatalf("baseline want 2, got %d", gs.ToughnessOf(morti))
	}
	// Another creature dies into the yard via the SBA destroy path.
	victim := addCreature(gs, 0, "Doomed", 0, 0) // 0/0 → dies to SBA
	gameengine.StateBasedActions(gs)
	if gs.ToughnessOf(morti) != 3 {
		t.Errorf("after a creature died into the yard, Mortivore want 3 toughness, got %d", gs.ToughnessOf(morti))
	}
	if !mortiOnBattlefield(gs) {
		t.Error("Mortivore self-destroyed after a creature died into the yard")
	}
	_ = victim
}

// PROBE C (property d): a +1/+1 counter must save a 0-count Mortivore (0/0
// base from the CDA + 1/1 counter = 1/1).
func TestCDAProbe_CounterSavesZeroCount(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature") // empty graveyards → 0/0 base
	gameengine.InvokeETBHook(gs, morti)
	morti.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	if got := gs.ToughnessOf(morti); got != 1 {
		t.Errorf("0-count Mortivore + one +1/+1 counter must be toughness 1 (7a/7b CDA then 7c counter), got %d", got)
	}
	gameengine.StateBasedActions(gs)
	if !mortiOnBattlefield(gs) {
		t.Error("Mortivore with a +1/+1 counter self-destroyed at 0 graveyard count")
	}
}

// PROBE D: counters stack on top of the CDA count.
func TestCDAProbe_CounterStacksOnCount(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gyCreature("A"), gyCreature("B"), gyCreature("C"))
	gameengine.InvokeETBHook(gs, morti)
	morti.AddCounter("+1/+1", 2)
	gs.InvalidateCharacteristicsCache()
	if p, tg := gs.PowerOf(morti), gs.ToughnessOf(morti); p != 5 || tg != 5 {
		t.Errorf("3 creatures + 2 counters want 5/5, got %d/%d", p, tg)
	}
}

// PROBE F: reanimation/blink entry (FirePermanentETBTriggers) registers the CDA.
func TestCDAProbe_ReanimationRegistersCDA(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gyCreature("A"), gyCreature("B"), gyCreature("C"))
	morti := addPerm(gs, 0, "Mortivore", "creature")
	gameengine.FirePermanentETBTriggers(gs, morti) // the reanimate/token/blink path
	if got := gs.ToughnessOf(morti); got != 3 {
		t.Errorf("reanimated Mortivore want toughness 3, got %d (CDA not registered on non-cast entry?)", got)
	}
	gameengine.StateBasedActions(gs)
	if !mortiOnBattlefield(gs) {
		t.Error("reanimated Mortivore self-destroyed (no CDA registered)")
	}
}

// THE self-destroy (7174n1c spectator report): a creature dies into the yard
// (non-simultaneously), which should GROW Mortivore past its marked damage so
// it survives the next §704.5g check — but the stale CDA cache kept Mortivore's
// toughness at its pre-death value, so the lethal-damage SBA read it too low
// and destroyed it. Pre-fix: Mortivore dies. Post-fix: survives.
func TestCDAProbe_SelfDestroy_StaleToughnessAfterDeath(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gyCreature("A"), gyCreature("B"))
	gameengine.InvokeETBHook(gs, morti) // 2/2

	// A creature dies into the yard FIRST (separate event) → Mortivore is now 3/3.
	addCreature(gs, 0, "Doomed", 0, 0)
	gameengine.StateBasedActions(gs)
	if gs.ToughnessOf(morti) != 3 {
		t.Fatalf("after the death Mortivore must be 3 toughness, got %d", gs.ToughnessOf(morti))
	}

	// THEN Mortivore takes 2 damage — below its (current) toughness 3.
	morti.MarkedDamage = 2
	gameengine.StateBasedActions(gs)
	if !mortiOnBattlefield(gs) {
		t.Error("Mortivore self-destroyed: 2 damage vs a stale toughness of 2 (should be 3/3 and survive)")
	}
}

// Property (a)/(d): the CDA is layer 7a, so a 7b "set base P/T" effect
// overrides it, and 7c +1/+1 counters then add on top (7a → 7b → 7c).
func TestCDAProbe_LayerOrder_7a_then_7b_then_7c(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		gyCreature("A"), gyCreature("B"), gyCreature("C")) // CDA → 3/3 at 7a
	gameengine.InvokeETBHook(gs, morti)
	if gs.ToughnessOf(morti) != 3 {
		t.Fatalf("CDA baseline want 3, got %d", gs.ToughnessOf(morti))
	}
	// A 7b "becomes 5/5" set effect must OVERRIDE the 7a CDA.
	gameengine.RegisterSetPT(gs, morti, 5, 5, gameengine.DurationPermanent, "Become 5/5", "test_set")
	gs.InvalidateCharacteristicsCache()
	if p, tg := gs.PowerOf(morti), gs.ToughnessOf(morti); p != 5 || tg != 5 {
		t.Errorf("7b set-PT must override the 7a CDA → 5/5, got %d/%d", p, tg)
	}
	// A 7c +1/+1 counter then adds on top → 6/6.
	morti.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	if p, tg := gs.PowerOf(morti), gs.ToughnessOf(morti); p != 6 || tg != 6 {
		t.Errorf("7c counter on top of the 7b set must be 6/6, got %d/%d", p, tg)
	}
}

// PROBE E: combat lethal-damage SBA uses the layered toughness.
func TestCDAProbe_CombatLethalUsesLayeredToughness(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		gyCreature("A"), gyCreature("B"), gyCreature("C"))
	gameengine.InvokeETBHook(gs, morti) // 3/3
	morti.MarkedDamage = 2              // below layered toughness 3
	gameengine.StateBasedActions(gs)
	if !mortiOnBattlefield(gs) {
		t.Error("Mortivore (3/3 via CDA) wrongly died to 2 marked damage — SBA read base toughness")
	}
	// Now lethal.
	morti.MarkedDamage = 3
	gameengine.StateBasedActions(gs)
	if mortiOnBattlefield(gs) {
		t.Error("Mortivore (3/3) should have died to 3 marked damage")
	}
}
