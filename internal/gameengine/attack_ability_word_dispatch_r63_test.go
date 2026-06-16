package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// attack_ability_word_dispatch_r63_test.go — dead-dispatch follow-up #1
// (ATTACK class). Ability-word attack triggers parse to a
// Static{ability_word → Triggered} wrapper invisible to the attack-trigger
// scans, so they were inert. Battalion (the dominant one) has a dedicated
// cardinality-gated dispatcher (FireBattalionTriggers) that only fired a
// per_card event and never resolved the effect; the other attack ability
// words (Ferocious/Formidable/Threshold/…) had no dispatch at all. Fixed via
// the shared iterAttackTriggers unwrap + FireBattalionTriggers resolution.

func atkGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(17)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func atkDrain(gs *GameState) {
	for i := 0; i < 80 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

// abilityWordAttacker builds an attacking creature on seat 0 whose AST is a
// Static{ability_word word → Triggered{event, GainLife controller+1}} wrapper.
func abilityWordAttacker(gs *GameState, name, word, event string) *Permanent {
	inner := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: event},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "controller"}},
		Raw:     word + " — whenever this creature and at least two other creatures attack, you gain 1 life",
	}
	c := &Card{Name: name, Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	c.AST = &gameast.CardAST{Name: name, Abilities: []gameast.Ability{
		&gameast.Static{Modification: &gameast.Modification{ModKind: "ability_word", Args: []interface{}{word, "triggered", inner}}},
	}}
	p := &Permanent{Card: c, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	setPermFlag(p, flagAttacking, true)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return p
}

func vanillaAttacker(gs *GameState, name string) *Permanent {
	c := &Card{Name: name, Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	p := &Permanent{Card: c, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	setPermFlag(p, flagAttacking, true)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return p
}

// Battalion fires when the bearer + 2 others (3 total declared) attack.
func TestAttackAbilityWord_BattalionFiresWithThree(t *testing.T) {
	gs := atkGame(t)
	b := abilityWordAttacker(gs, "Boros Elite", "battalion", "formation_attack")
	o1 := vanillaAttacker(gs, "Ally One")
	o2 := vanillaAttacker(gs, "Ally Two")
	declared := []*Permanent{b, o1, o2}

	life := gs.Seats[0].Life
	FireBattalionTriggers(gs, 0, declared)
	atkDrain(gs)
	if gs.Seats[0].Life-life != 1 {
		t.Fatalf("battalion did not fire with 3 declared attackers (lifeΔ=%d, want 1) — still inert", gs.Seats[0].Life-life)
	}
}

// Battalion does NOT fire with only the bearer + 1 other (2 total) —
// respects the "and at least two other creatures attack" cardinality on the
// declared-attacker count.
func TestAttackAbilityWord_BattalionSilentWithTwo(t *testing.T) {
	gs := atkGame(t)
	b := abilityWordAttacker(gs, "Boros Elite", "battalion", "formation_attack")
	o1 := vanillaAttacker(gs, "Ally One")
	declared := []*Permanent{b, o1}

	life := gs.Seats[0].Life
	FireBattalionTriggers(gs, 0, declared)
	atkDrain(gs)
	if gs.Seats[0].Life != life {
		t.Fatalf("battalion fired with only 2 attackers (lifeΔ=%d) — cardinality gate broken", gs.Seats[0].Life-life)
	}
}

// Battalion source not itself attacking → silent (CR §702.101a "this
// creature … attack").
func TestAttackAbilityWord_BattalionSilentWhenSourceNotAttacking(t *testing.T) {
	gs := atkGame(t)
	b := abilityWordAttacker(gs, "Boros Elite", "battalion", "formation_attack")
	setPermFlag(b, flagAttacking, false) // bearer not attacking
	o1 := vanillaAttacker(gs, "Ally One")
	o2 := vanillaAttacker(gs, "Ally Two")
	life := gs.Seats[0].Life
	FireBattalionTriggers(gs, 0, []*Permanent{b, o1, o2})
	atkDrain(gs)
	if gs.Seats[0].Life != life {
		t.Fatalf("battalion fired while its source wasn't attacking (lifeΔ=%d)", gs.Seats[0].Life-life)
	}
}

// Other attack ability words (Ferocious/Formidable/…) fire via the shared
// iterAttackTriggers unwrap on the self-attack path.
func TestAttackAbilityWord_OtherAttackWordsFire(t *testing.T) {
	for _, word := range []string{"ferocious", "formidable", "threshold", "metalcraft", "delirium", "coven", "parley"} {
		gs := atkGame(t)
		a := abilityWordAttacker(gs, "Probe "+word, word, "attack")
		life := gs.Seats[0].Life
		fireAttackTriggers(gs, 0, []*Permanent{a})
		atkDrain(gs)
		if gs.Seats[0].Life-life != 1 {
			t.Fatalf("attack ability word %q did not fire on the bearer's attack (lifeΔ=%d, want 1)", word, gs.Seats[0].Life-life)
		}
	}
}

// Shared-helper consistency: iterAttackTriggers surfaces the unwrapped
// attack ability word and excludes battalion (cardinality-gated elsewhere).
func TestAttackAbilityWord_IterAttackTriggersUnwrap(t *testing.T) {
	mk := func(word, event string) *Card {
		c := &Card{Name: word, Types: []string{"creature"}}
		c.AST = &gameast.CardAST{Name: word, Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{ModKind: "ability_word",
				Args: []interface{}{word, "triggered", &gameast.Triggered{Trigger: gameast.Trigger{Event: event}, Effect: &gameast.GainLife{}}}}},
		}}
		return c
	}
	if got := iterAttackTriggers(mk("ferocious", "attack")); len(got) != 1 {
		t.Fatalf("ferocious attack trigger must be surfaced by iterAttackTriggers; got %d", len(got))
	}
	// battalion has a dedicated cardinality-gated dispatcher — must NOT be
	// surfaced here (self_and aliases to "attack", so the exclusion matters).
	if got := iterAttackTriggers(mk("battalion", "self_and")); len(got) != 0 {
		t.Fatalf("battalion must be excluded from iterAttackTriggers (dedicated dispatcher); got %d", len(got))
	}
}
