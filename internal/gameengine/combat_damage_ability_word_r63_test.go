package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// combat_damage_ability_word_r63_test.go — dead-dispatch follow-up #3
// (COMBAT-DAMAGE class). Ability-word combat-damage triggers (Domain,
// Hellbent, Metalcraft, Threshold, Council's-Dilemma — "Whenever this
// creature deals combat damage to a player, …") parse to a
// Static{ability_word → Triggered} wrapper the combat-damage AST scan in
// fireCombatDamageTriggers skipped, so they were inert. Fixed by unwrapping
// in that scan; composes with the existing per_card double-fire gate.

func cdGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(23)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func cdDrain(gs *GameState) {
	for i := 0; i < 80 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

// combatDamagePerm builds a creature (controlled by `seat`) whose AST is a
// Static{ability_word word → Triggered{combat_damage_player, GainLife
// controller+1}} wrapper.
func combatDamagePerm(gs *GameState, seat int, word string) *Permanent {
	inner := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "combat_damage_player"},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "controller"}},
		Raw:     word + " — whenever this creature deals combat damage to a player, you gain 1 life",
	}
	c := &Card{Name: "Probe " + word, Owner: seat, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	c.AST = &gameast.CardAST{Name: c.Name, Abilities: []gameast.Ability{
		&gameast.Static{Modification: &gameast.Modification{ModKind: "ability_word", Args: []interface{}{word, "triggered", inner}}},
	}}
	p := &Permanent{Card: c, Controller: seat, Owner: seat, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Combat-damage ability words fire when the source connects with a player.
func TestCombatDamageAbilityWord_FiresOnPlayerDamage(t *testing.T) {
	for _, word := range []string{"domain", "hellbent", "metalcraft", "threshold"} {
		gs := cdGame(t)
		src := combatDamagePerm(gs, 0, word)
		life := gs.Seats[0].Life
		fireCombatDamageTriggers(gs, src, 2, "player", 1, nil)
		cdDrain(gs)
		if got := gs.Seats[0].Life - life; got != 1 {
			t.Fatalf("combat-damage ability word %q did not fire on player damage (lifeΔ=%d, want 1)", word, got)
		}
	}
}

// Source attribution: the SOURCE's controller gets the effect, fired once
// per the single combat-damage event.
func TestCombatDamageAbilityWord_SourceAttribution(t *testing.T) {
	gs := cdGame(t)
	src := combatDamagePerm(gs, 1, "domain") // seat 1's creature
	l0, l1 := gs.Seats[0].Life, gs.Seats[1].Life
	fireCombatDamageTriggers(gs, src, 3, "player", 0, nil) // hits seat 0
	cdDrain(gs)
	if gs.Seats[1].Life-l1 != 1 {
		t.Fatalf("controller (seat 1) should gain 1 (its own trigger); got %d", gs.Seats[1].Life-l1)
	}
	if gs.Seats[0].Life != l0 {
		t.Fatalf("the damaged player (seat 0) must not gain life; got %d", gs.Seats[0].Life-l0)
	}
}

// Double-fire guard: when a per_card handler owns combat_damage_player, the
// generic AST resolution is skipped (the existing targetKind=="player" gate),
// so per_card-owned cards fire once via FireCardTrigger, not twice.
func TestCombatDamageAbilityWord_PerCardGuardNoDoubleFire(t *testing.T) {
	gs := cdGame(t)
	src := combatDamagePerm(gs, 0, "domain")

	old := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == src.Card.DisplayName() && event == "combat_damage_player"
	}
	defer func() { HasTriggerHook = old }()

	life := gs.Seats[0].Life
	fireCombatDamageTriggers(gs, src, 2, "player", 1, nil)
	cdDrain(gs)
	if gs.Seats[0].Life != life {
		t.Fatalf("generic combat-damage effect resolved despite a per_card handler (double-fire path): lifeΔ=%d", gs.Seats[0].Life-life)
	}
}
