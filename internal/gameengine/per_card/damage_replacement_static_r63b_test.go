package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Regressions for the §614 static damage-replacement wave 2
// (damage_replacement_static_r63b.go). Each card was inert before this
// change. Tests register via InvokeETBHook then drive a fully-populated
// DamageContext through the engine's ApplyDamageReplacement chain.
//
// dsApplyToPlayer / dsRegister live in damage_scale_family_r63_test.go
// (same package).

// dsApplyCtx runs the chain for an arbitrary context and returns the
// resulting amount + prevented flag.
func dsApplyCtx(gs *gameengine.GameState, src *gameengine.Permanent, targetSeat int, targetPerm *gameengine.Permanent, kind gameengine.DamageKind, amount int) (int, bool) {
	ctx := &gameengine.DamageContext{
		Source: src, TargetSeat: targetSeat, TargetPerm: targetPerm,
		Kind: kind, Amount: amount,
	}
	return gameengine.ApplyDamageReplacement(gs, ctx)
}

func TestGhostsOfInnocent_HalvesRoundedDown(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Ghosts of the Innocent", "enchantment")
	src := addPerm(gs, 0, "Goblin", "creature")
	if got := dsApplyToPlayer(gs, src, 1, 7); got != 3 {
		t.Errorf("7 halved rounded down = 3, got %d", got)
	}
	if got := dsApplyToPlayer(gs, src, 1, 8); got != 4 {
		t.Errorf("8 halved = 4, got %d", got)
	}
}

func TestBenevolentUnicorn_SpellMinus1(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Benevolent Unicorn", "enchantment")
	bolt := addPerm(gs, 0, "Lightning Bolt", "instant")
	if got := dsApplyToPlayer(gs, bolt, 1, 3); got != 2 {
		t.Errorf("spell 3-1 = 2, got %d", got)
	}
	// Non-spell (creature combat) source → unaffected.
	creat := addPerm(gs, 0, "Bear", "creature")
	if got := dsApplyToPlayer(gs, creat, 1, 3); got != 3 {
		t.Errorf("non-spell source must not be reduced, got %d", got)
	}
}

func TestLashknifeBarrier_MinusOneToYourCreatures(t *testing.T) {
	gs := newGame(t, 2)
	lash := dsRegister(t, gs, "Lashknife Barrier", "enchantment")
	mine := addPerm(gs, lash.Controller, "Soldier", "creature")
	src := addPerm(gs, 1, "Goblin", "creature")
	if got, _ := dsApplyCtx(gs, src, mine.Controller, mine, gameengine.DamageCombatCreature, 3); got != 2 {
		t.Errorf("damage to your creature reduced by 1 (3→2), got %d", got)
	}
	// A creature an opponent controls is unaffected.
	theirs := addPerm(gs, 1, "Ogre", "creature")
	if got, _ := dsApplyCtx(gs, src, theirs.Controller, theirs, gameengine.DamageCombatCreature, 3); got != 3 {
		t.Errorf("opponent's creature must not be reduced, got %d", got)
	}
}

func TestDivinePresence_CapsAtThree(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Divine Presence", "enchantment")
	src := addPerm(gs, 0, "Goblin", "creature")
	if got := dsApplyToPlayer(gs, src, 1, 9); got != 3 {
		t.Errorf("9 (≥4) → 3, got %d", got)
	}
	if got := dsApplyToPlayer(gs, src, 1, 2); got != 2 {
		t.Errorf("2 (<4) unchanged, got %d", got)
	}
}

func TestForethoughtAmulet_InstantSorceryToYouSetTwo(t *testing.T) {
	gs := newGame(t, 2)
	amulet := dsRegister(t, gs, "Forethought Amulet", "artifact")
	bolt := addPerm(gs, 1, "Fireball", "sorcery")
	// Spell dealing 5 to the amulet's controller → 2.
	if got := dsApplyToPlayer(gs, bolt, amulet.Controller, 5); got != 2 {
		t.Errorf("instant/sorcery 5 to you → 2, got %d", got)
	}
	// Only 2 damage (<3) — unchanged.
	if got := dsApplyToPlayer(gs, bolt, amulet.Controller, 2); got != 2 {
		t.Errorf("<3 unchanged, got %d", got)
	}
	// Spell to an opponent — unaffected.
	if got := dsApplyToPlayer(gs, bolt, 1, 5); got != 5 {
		t.Errorf("damage to non-controller must not be reduced, got %d", got)
	}
}

func TestChargingTuskodon_DoublesOwnCombatToPlayer(t *testing.T) {
	gs := newGame(t, 2)
	tusk := dsRegister(t, gs, "Charging Tuskodon", "creature")
	if got, _ := dsApplyCtx(gs, tusk, 1, nil, gameengine.DamageCombatPlayer, 4); got != 8 {
		t.Errorf("own combat to player doubles (4→8), got %d", got)
	}
	// Noncombat (e.g. an ability) → not doubled.
	if got, _ := dsApplyCtx(gs, tusk, 1, nil, gameengine.DamageNonCombatPlayer, 4); got != 4 {
		t.Errorf("noncombat must not double, got %d", got)
	}
	// A different source → not doubled.
	other := addPerm(gs, 0, "Goblin", "creature")
	if got, _ := dsApplyCtx(gs, other, 1, nil, gameengine.DamageCombatPlayer, 4); got != 4 {
		t.Errorf("other source must not double, got %d", got)
	}
}

func TestGoldnightCastigator_DoublesToYouAndSelf(t *testing.T) {
	gs := newGame(t, 2)
	gc := dsRegister(t, gs, "Goldnight Castigator", "creature")
	src := addPerm(gs, 1, "Goblin", "creature")
	// To you (gc's controller, seat 0).
	if got := dsApplyToPlayer(gs, src, gc.Controller, 3); got != 6 {
		t.Errorf("damage to you doubles (3→6), got %d", got)
	}
	// To this creature (Goldnight Castigator itself).
	if got, _ := dsApplyCtx(gs, src, gc.Controller, gc, gameengine.DamageCombatCreature, 3); got != 6 {
		t.Errorf("damage to Goldnight Castigator doubles (3→6), got %d", got)
	}
	// To an opponent → unchanged.
	if got := dsApplyToPlayer(gs, src, 1, 3); got != 3 {
		t.Errorf("damage to opponent unchanged, got %d", got)
	}
}

func TestTorWauki_NoncombatPlusOneExcludesSelf(t *testing.T) {
	gs := newGame(t, 2)
	tor := dsRegister(t, gs, "Tor Wauki the Younger", "creature")
	src := addPerm(gs, 0, "Goblin", "creature")
	// Another source you control, noncombat → +1.
	if got, _ := dsApplyCtx(gs, src, 1, nil, gameengine.DamageNonCombatPlayer, 2); got != 3 {
		t.Errorf("noncombat from another source you control +1 (2→3), got %d", got)
	}
	// Combat → unaffected.
	if got, _ := dsApplyCtx(gs, src, 1, nil, gameengine.DamageCombatPlayer, 2); got != 2 {
		t.Errorf("combat must not get +1, got %d", got)
	}
	// Tor Wauki itself ("another") → unaffected.
	if got, _ := dsApplyCtx(gs, tor, 1, nil, gameengine.DamageNonCombatPlayer, 2); got != 2 {
		t.Errorf("Tor Wauki's own damage must not get +1, got %d", got)
	}
}

func TestSoundOfDrums_DoublesEnchantedCreatureCombat(t *testing.T) {
	gs := newGame(t, 2)
	creature := addPerm(gs, 0, "Goblin", "creature")
	aura := addPerm(gs, 0, "The Sound of Drums", "enchantment", "aura")
	aura.AttachedTo = creature
	gameengine.InvokeETBHook(gs, aura)
	if got, _ := dsApplyCtx(gs, creature, 1, nil, gameengine.DamageCombatPlayer, 3); got != 6 {
		t.Errorf("enchanted creature combat doubles (3→6), got %d", got)
	}
	// A different creature → unaffected.
	other := addPerm(gs, 0, "Bear", "creature")
	if got, _ := dsApplyCtx(gs, other, 1, nil, gameengine.DamageCombatPlayer, 3); got != 3 {
		t.Errorf("unenchanted creature must not double, got %d", got)
	}
}

func TestInquisitorsFlail_DoublesBothWays(t *testing.T) {
	gs := newGame(t, 2)
	equipped := addPerm(gs, 0, "Knight", "creature")
	flail := addPerm(gs, 0, "Inquisitor's Flail", "artifact", "equipment")
	flail.AttachedTo = equipped
	gameengine.InvokeETBHook(gs, flail)
	// Equipped creature deals combat damage → ×2.
	if got, _ := dsApplyCtx(gs, equipped, 1, nil, gameengine.DamageCombatPlayer, 3); got != 6 {
		t.Errorf("equipped combat doubles (3→6), got %d", got)
	}
	// Another creature deals combat damage TO equipped creature → ×2.
	attacker := addPerm(gs, 1, "Ogre", "creature")
	if got, _ := dsApplyCtx(gs, attacker, 0, equipped, gameengine.DamageCombatCreature, 2); got != 4 {
		t.Errorf("combat to equipped creature doubles (2→4), got %d", got)
	}
}

func TestAnthemOfRakdos_HellbentGated(t *testing.T) {
	gs := newGame(t, 2)
	anthem := dsRegister(t, gs, "Anthem of Rakdos", "enchantment")
	src := addPerm(gs, 0, "Goblin", "creature")
	// Hand not empty → no doubling.
	gs.Seats[anthem.Controller].Hand = append(gs.Seats[anthem.Controller].Hand, &gameengine.Card{Name: "Card"})
	if got := dsApplyToPlayer(gs, src, 1, 3); got != 3 {
		t.Errorf("with cards in hand, no hellbent double, got %d", got)
	}
	// Empty hand → ×2.
	gs.Seats[anthem.Controller].Hand = nil
	if got := dsApplyToPlayer(gs, src, 1, 3); got != 6 {
		t.Errorf("hellbent (empty hand) doubles (3→6), got %d", got)
	}
}

func TestFatedFirepower_PlusFireCounters(t *testing.T) {
	gs := newGame(t, 2)
	ff := dsRegister(t, gs, "Fated Firepower", "enchantment")
	ff.Counters["fire"] = 3
	src := addPerm(gs, 0, "Goblin", "creature")
	// To opponent → +3.
	if got := dsApplyToPlayer(gs, src, 1, 2); got != 5 {
		t.Errorf("+3 fire counters to opponent (2→5), got %d", got)
	}
	// To you → no bonus.
	if got := dsApplyToPlayer(gs, src, ff.Controller, 2); got != 2 {
		t.Errorf("no bonus to your own seat, got %d", got)
	}
}

func TestRemKarolus_PreventToYouPlusOneToOpp(t *testing.T) {
	gs := newGame(t, 2)
	rem := dsRegister(t, gs, "Rem Karolus, Stalwart Slayer", "creature")
	bolt := addPerm(gs, 1, "Lightning Bolt", "instant")
	// Spell to you → prevented. (Engine treats Prevented as no-damage
	// regardless of the residual amount; consumers gate on `Prevented ||
	// Amount <= 0`, so we assert the flag, not a zeroed amount.)
	if _, prevented := dsApplyCtx(gs, bolt, rem.Controller, nil, gameengine.DamageNonCombatPlayer, 5); !prevented {
		t.Errorf("spell to you should be prevented")
	}
	// Spell to opponent → +1.
	if got, _ := dsApplyCtx(gs, bolt, 1, nil, gameengine.DamageNonCombatPlayer, 3); got != 4 {
		t.Errorf("spell to opponent +1 (3→4), got %d", got)
	}
	// Non-spell (combat) to you → NOT prevented (only spells).
	creat := addPerm(gs, 1, "Goblin", "creature")
	if got, prevented := dsApplyCtx(gs, creat, rem.Controller, nil, gameengine.DamageCombatPlayer, 3); prevented || got != 3 {
		t.Errorf("combat damage to you must not be prevented by Rem Karolus, got amount=%d prevented=%v", got, prevented)
	}
}

// Condition-gated: Rollercrusher (delirium) and Far Fortune (max speed),
// Aether Revolt (revolt) — verify the gate blocks when the condition is
// off. (Active-condition end-to-end is covered by the engine's own
// delirium/speed/revolt tests; here we pin the OFF gate + you-control.)
func TestConditionGated_OffByDefault(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "The Rollercrusher Ride", "enchantment")
	dsRegister(t, gs, "Aether Revolt", "enchantment")
	src := addPerm(gs, 0, "Goblin", "creature")
	// Fresh game: no delirium, no revolt → noncombat to opponent unchanged.
	if got, _ := dsApplyCtx(gs, src, 1, nil, gameengine.DamageNonCombatPlayer, 4); got != 4 {
		t.Errorf("conditions off → no modification, got %d", got)
	}
}
