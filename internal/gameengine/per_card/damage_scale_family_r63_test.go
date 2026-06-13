package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// damage_scale_family_r63_test.go — regressions for the §614 multiplier/
// bonus damage-replacement family (damage_scale_family_r63.go).
//
// Each card was inert before this change (its replacement parsed to a
// log-only if_intervening_tail / replacement_static scaffold). Tests
// register the handler via InvokeETBHook (the same path the engine uses
// at ETB), then drive a fully-populated DamageContext through the
// engine's ApplyDamageReplacement chain and assert the post-replacement
// amount — plus filter no-ops, an end-to-end combat delta, and LTB
// unregistration.

// dsRegister puts a permanent on seat 0 and fires its ETB hook so the
// family closure registers into gs.DamageReplacements.
func dsRegister(t *testing.T, gs *gameengine.GameState, name string, types ...string) *gameengine.Permanent {
	t.Helper()
	p := addPerm(gs, 0, name, types...)
	gameengine.InvokeETBHook(gs, p)
	return p
}

// dsApply runs the replacement chain for a source dealing `amount` to a
// target seat (player damage) and returns the resulting amount.
func dsApplyToPlayer(gs *gameengine.GameState, src *gameengine.Permanent, targetSeat, amount int) int {
	ctx := &gameengine.DamageContext{
		Source:     src,
		SourceName: "",
		TargetSeat: targetSeat,
		Kind:       gameengine.DamageNonCombatPlayer,
		Amount:     amount,
	}
	out, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	return out
}

func TestGratuitousViolence_DoublesCreatureYouControl(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Gratuitous Violence", "enchantment")
	src := addPerm(gs, 0, "Goblin", "creature")
	if got := dsApplyToPlayer(gs, src, 1, 3); got != 6 {
		t.Errorf("creature you control should deal double (3→6), got %d", got)
	}
	// Non-creature source you control (e.g. an artifact pinger) — not doubled.
	rock := addPerm(gs, 0, "Cursed Scroll", "artifact")
	if got := dsApplyToPlayer(gs, rock, 1, 3); got != 3 {
		t.Errorf("non-creature source must not double, got %d", got)
	}
	// Opponent-controlled creature — not doubled.
	opp := addPerm(gs, 1, "Bear", "creature")
	if got := dsApplyToPlayer(gs, opp, 0, 2); got != 2 {
		t.Errorf("opponent's creature must not be doubled by your Gratuitous Violence, got %d", got)
	}
}

func TestFieryEmancipation_TriplesSourceYouControl(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Fiery Emancipation", "enchantment")
	src := addPerm(gs, 0, "Goblin", "creature")
	if got := dsApplyToPlayer(gs, src, 1, 4); got != 12 {
		t.Errorf("source you control should deal triple (4→12), got %d", got)
	}
}

func TestEmbermawHellion_RedSourcePlus1_ExcludesSelf(t *testing.T) {
	gs := newGame(t, 2)
	ember := dsRegister(t, gs, "Embermaw Hellion", "creature", "pip:R")
	// Another red source you control → +1.
	redSrc := addPerm(gs, 0, "Goblin Guide", "creature", "pip:R")
	if got := dsApplyToPlayer(gs, redSrc, 1, 2); got != 3 {
		t.Errorf("another red source you control should get +1 (2→3), got %d", got)
	}
	// "another" — Embermaw itself does NOT get the +1.
	if got := dsApplyToPlayer(gs, ember, 1, 2); got != 2 {
		t.Errorf("Embermaw must not buff its OWN damage (another red source), got %d", got)
	}
	// Non-red source → unaffected.
	white := addPerm(gs, 0, "Soldier", "creature", "pip:W")
	if got := dsApplyToPlayer(gs, white, 1, 2); got != 2 {
		t.Errorf("non-red source must not get +1, got %d", got)
	}
}

func TestTwinflameTyrant_DoublesToOpponentsOnly(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Twinflame Tyrant", "creature")
	src := addPerm(gs, 0, "Goblin", "creature")
	// To an opponent → ×2.
	if got := dsApplyToPlayer(gs, src, 1, 3); got != 6 {
		t.Errorf("damage to opponent should double (3→6), got %d", got)
	}
	// To yourself (seat 0) → unchanged.
	if got := dsApplyToPlayer(gs, src, 0, 3); got != 3 {
		t.Errorf("damage to your own seat must not double, got %d", got)
	}
}

func TestCalamityBearer_DoublesGiantsOnly(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Calamity Bearer", "creature", "giant")
	giant := addPerm(gs, 0, "Frost Giant", "creature", "giant")
	if got := dsApplyToPlayer(gs, giant, 1, 5); got != 10 {
		t.Errorf("giant source should double (5→10), got %d", got)
	}
	nonGiant := addPerm(gs, 0, "Goblin", "creature")
	if got := dsApplyToPlayer(gs, nonGiant, 1, 5); got != 5 {
		t.Errorf("non-giant source must not double, got %d", got)
	}
}

func TestObosh_DoublesOddManaValueOnly(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Obosh, the Preypiercer", "creature")
	odd := addPerm(gs, 0, "Goblin", "creature")
	odd.Card.CMC = 3
	if got := dsApplyToPlayer(gs, odd, 1, 2); got != 4 {
		t.Errorf("odd-MV source should double (2→4), got %d", got)
	}
	even := addPerm(gs, 0, "Ogre", "creature")
	even.Card.CMC = 4
	if got := dsApplyToPlayer(gs, even, 1, 2); got != 2 {
		t.Errorf("even-MV source must not double, got %d", got)
	}
}

func TestUncivilUnrest_RequiresPlusCounter(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Uncivil Unrest", "enchantment")
	withCounter := addPerm(gs, 0, "Goblin", "creature")
	withCounter.Counters["+1/+1"] = 1
	if got := dsApplyToPlayer(gs, withCounter, 1, 3); got != 6 {
		t.Errorf("creature with +1/+1 counter should double (3→6), got %d", got)
	}
	noCounter := addPerm(gs, 0, "Bear", "creature")
	if got := dsApplyToPlayer(gs, noCounter, 1, 3); got != 3 {
		t.Errorf("creature without a +1/+1 counter must not double, got %d", got)
	}
}

func TestFiendishDuo_DoublesToOpponentPlayerOnly(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Fiendish Duo", "creature")
	// "a source" — even an opponent's source dealing to ANOTHER opponent
	// of Fiendish Duo's controller doubles; the filter is target-only
	// (to an opponent, player). Use a seat-0 source to an opponent player.
	src := addPerm(gs, 0, "Goblin", "creature")
	if got := dsApplyToPlayer(gs, src, 1, 4); got != 8 {
		t.Errorf("damage to an opponent player should double (4→8), got %d", got)
	}
	// To a permanent (TargetPerm set) → NOT doubled ("to an opponent" = player).
	perm := addPerm(gs, 1, "Wall", "creature")
	ctx := &gameengine.DamageContext{
		Source: src, TargetSeat: 1, TargetPerm: perm,
		Kind: gameengine.DamageNonCombatCreature, Amount: 4,
	}
	if out, _ := gameengine.ApplyDamageReplacement(gs, ctx); out != 4 {
		t.Errorf("Fiendish Duo is player-only; permanent damage must not double, got %d", out)
	}
}

func TestFireServant_DoublesRedInstantSorceryYouControl(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Fire Servant", "creature")
	// Synthetic spell source: a red instant you control.
	bolt := addPerm(gs, 0, "Lightning Bolt", "instant", "pip:R")
	if got := dsApplyToPlayer(gs, bolt, 1, 3); got != 6 {
		t.Errorf("red instant you control should double (3→6), got %d", got)
	}
	// A red creature (not instant/sorcery) → unchanged.
	creat := addPerm(gs, 0, "Goblin", "creature", "pip:R")
	if got := dsApplyToPlayer(gs, creat, 1, 3); got != 3 {
		t.Errorf("non-spell red source must not double under Fire Servant, got %d", got)
	}
}

func TestMechanizedWarfare_ArtifactSourcePlus1ToOpponent(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Mechanized Warfare", "enchantment")
	rock := addPerm(gs, 0, "Cursed Scroll", "artifact")
	if got := dsApplyToPlayer(gs, rock, 1, 2); got != 3 {
		t.Errorf("artifact source you control should get +1 to opponent (2→3), got %d", got)
	}
	// To your own seat → no bonus.
	if got := dsApplyToPlayer(gs, rock, 0, 2); got != 2 {
		t.Errorf("Mechanized Warfare must not buff damage to your own seat, got %d", got)
	}
}

// End-to-end: Gratuitous Violence doubles real combat damage through
// DealCombatDamageStep (proving the registry is consulted by combat).
func TestGratuitousViolence_EndToEndCombat(t *testing.T) {
	gs := newGame(t, 2)
	dsRegister(t, gs, "Gratuitous Violence", "enchantment")
	atk := addPerm(gs, 0, "Goblin Guide", "creature")
	atk.Card.BasePower = 2
	atk.Card.BaseToughness = 2
	start := gs.Seats[1].Life
	gameengine.DealCombatDamageStep(gs, []*gameengine.Permanent{atk},
		map[*gameengine.Permanent][]*gameengine.Permanent{atk: nil}, false)
	if got := start - gs.Seats[1].Life; got != 4 {
		t.Errorf("Gratuitous Violence should double combat damage (2→4), got %d", got)
	}
}

// Multiple doublers stack (×2 then ×3 = ×6), and LTB unregisters cleanly.
func TestDamageScale_StackAndLTBUnregister(t *testing.T) {
	gs := newGame(t, 2)
	gv := dsRegister(t, gs, "Gratuitous Violence", "enchantment")
	dsRegister(t, gs, "Fiery Emancipation", "enchantment")
	if n := len(gs.DamageReplacements); n != 2 {
		t.Fatalf("expected 2 registered replacements, got %d", n)
	}
	src := addPerm(gs, 0, "Goblin", "creature")
	if got := dsApplyToPlayer(gs, src, 1, 1); got != 6 {
		t.Errorf("×2 then ×3 should be ×6 (1→6), got %d", got)
	}
	// Fire Gratuitous Violence's LTB → its replacement drops, leaving ×3.
	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{"perm": gv})
	if got := dsApplyToPlayer(gs, src, 1, 1); got != 3 {
		t.Errorf("after Gratuitous Violence LTB only ×3 remains (1→3), got %d", got)
	}
}
