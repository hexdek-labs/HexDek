package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// dead_dispatch_ability_word_r63_test.go — consolidated dead-dispatch sweep.
// Ability-word triggers (Raid, Revolt, Morbid, Imprint, Undergrowth,
// Addendum, Delirium, Metalcraft, Threshold, Domain, Coven, Lieutenant,
// Survival, …) parse to a Static{ability_word → Triggered} wrapper that the
// engine's top-level *Triggered scans skip, so the abilities were INERT — the
// exact dead shape that left landfall / dredge / constellation doing nothing.
// Fixed by unwrapAbilityWordTriggered at the self-ETB (etb_dispatch.go) and
// phase (phases.go) dispatch scans. These pin the fix per dispatch class.

func dwGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(13)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func dwDrain(gs *GameState) {
	for i := 0; i < 80 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

// abilityWordPerm builds a permanent whose AST is a Static{ability_word →
// Triggered{event, GainLife controller+1}} wrapper — the exact parse shape.
func abilityWordPerm(gs *GameState, seat int, name, word, event, phase string) *Permanent {
	inner := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: event, Phase: phase},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "controller"}},
		Raw:     word + " — at the beginning of each end step, you gain 1 life",
	}
	c := &Card{Name: name, Owner: seat, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	c.AST = &gameast.CardAST{Name: name, Abilities: []gameast.Ability{
		&gameast.Static{Modification: &gameast.Modification{ModKind: "ability_word", Args: []interface{}{word, "triggered", inner}}},
	}}
	p := &Permanent{Card: c, Controller: seat, Owner: seat, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Self-ETB ability-word triggers now fire on the bearer's own ETB.
func TestDeadDispatch_SelfETBAbilityWords(t *testing.T) {
	for _, word := range []string{"raid", "revolt", "morbid", "imprint", "undergrowth", "addendum", "delirium", "metalcraft"} {
		gs := dwGame(t)
		p := abilityWordPerm(gs, 0, "Probe "+word, word, "etb", "")
		life := gs.Seats[0].Life
		FirePermanentETBTriggers(gs, p)
		dwDrain(gs)
		if gs.Seats[0].Life-life != 1 {
			t.Fatalf("self-ETB ability word %q did not fire (lifeΔ=%d, want 1) — still inert", word, gs.Seats[0].Life-life)
		}
	}
}

// Phase ability-word triggers now fire on the matching step.
func TestDeadDispatch_PhaseAbilityWords(t *testing.T) {
	gs := dwGame(t)
	abilityWordPerm(gs, 0, "Probe morbid-phase", "morbid", "phase", "end_step")
	life := gs.Seats[0].Life
	gs.Phase, gs.Step = "ending", "end"
	FirePhaseTriggers(gs, gs.Phase, gs.Step)
	dwDrain(gs)
	if gs.Seats[0].Life-life != 1 {
		t.Fatalf("phase ability word did not fire on end step (lifeΔ=%d, want 1)", gs.Seats[0].Life-life)
	}
}

// A non-matching step must NOT fire the phase ability-word trigger.
func TestDeadDispatch_PhaseAbilityWord_WrongStepNoFire(t *testing.T) {
	gs := dwGame(t)
	abilityWordPerm(gs, 0, "Probe morbid-phase", "morbid", "phase", "end_step")
	life := gs.Seats[0].Life
	gs.Phase, gs.Step = "beginning", "upkeep"
	FirePhaseTriggers(gs, gs.Phase, gs.Step)
	dwDrain(gs)
	if gs.Seats[0].Life != life {
		t.Fatalf("end_step ability word fired on upkeep (lifeΔ=%d)", gs.Seats[0].Life-life)
	}
}

// The unwrap excludes ability words that own a dedicated dispatcher (no
// double-fire), and passes nil for non-wrappers.
func TestDeadDispatch_UnwrapExcludesDedicatedAndNonWrappers(t *testing.T) {
	mk := func(word string) gameast.Ability {
		return &gameast.Static{Modification: &gameast.Modification{
			ModKind: "ability_word",
			Args:    []interface{}{word, "triggered", &gameast.Triggered{Trigger: gameast.Trigger{Event: "etb"}}},
		}}
	}
	for _, word := range []string{"landfall", "constellation", "eerie", "magecraft", "heroic"} {
		if unwrapAbilityWordTriggered(mk(word)) != nil {
			t.Fatalf("ability word %q has a dedicated dispatcher; must NOT be unwrapped here (double-fire risk)", word)
		}
	}
	for _, word := range []string{"raid", "morbid", "imprint", "coven"} {
		if unwrapAbilityWordTriggered(mk(word)) == nil {
			t.Fatalf("ability word %q must unwrap", word)
		}
	}
	// A top-level Triggered is not a Static wrapper → nil.
	if unwrapAbilityWordTriggered(&gameast.Triggered{Trigger: gameast.Trigger{Event: "etb"}}) != nil {
		t.Fatal("top-level Triggered must not be treated as an ability-word wrapper")
	}
	// A non-ability-word Static → nil.
	if unwrapAbilityWordTriggered(&gameast.Static{Modification: &gameast.Modification{ModKind: "anthem"}}) != nil {
		t.Fatal("non-ability-word Static must return nil")
	}
}

// Death-class ability-word triggers now fire on the bearer's OWN death,
// exactly once (dead-dispatch sweep r63 #5 — Slaughterhouse Bouncer / Hellbent
// "when this creature dies, if you have no cards in hand, …"). The inner
// *Triggered{event:"die"} lives in a Static{ability_word} wrapper that the
// self-dies scan (fireSelfZoneChangeTriggers, zone_change.go) now unwraps. The
// single GainLife +1 proves it fires exactly once — no double-dispatch.
func TestDeadDispatch_DeathAbilityWord(t *testing.T) {
	for _, word := range []string{"hellbent", "morbid", "delirium"} {
		gs := dwGame(t)
		p := abilityWordPerm(gs, 0, "Death Probe "+word, word, "die", "")
		life := gs.Seats[0].Life
		DestroyPermanent(gs, p, nil) // the bearer dies → fires its self-death trigger
		dwDrain(gs)
		if got := gs.Seats[0].Life - life; got != 1 {
			t.Fatalf("death ability word %q did not fire EXACTLY once on the bearer's death (lifeΔ=%d, want 1) — still inert or double-fired", word, got)
		}
	}
}
