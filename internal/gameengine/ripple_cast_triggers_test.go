package gameengine

import (
	"math/rand"
	"testing"
)

// ripple_cast_triggers_test.go — a ripple cast IS a cast (CR §702.60a,
// "you may cast any cards revealed this way"), so it must count for
// storm/magecraft and fire the "whenever you cast a spell" family.
//
// castOneRippleFree built its StackItem directly and called none of the
// cast-side helpers, so ripple-cast spells were invisible to every cast
// payoff. Same bypass class as the plot path fixed earlier, and the same
// helpers close it.
//
// Filed as "magecraft missing on Replicate / Demonstrate / Ripple". Two
// thirds of that was already stale — replicate and demonstrate both route
// through FireSpellCopyTriggers today. Ripple was the real gap, and it is
// not a copy path at all: it is a cast that fired nothing.

func rippleCastGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(11)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.EventPolicy = EventLogFull
	return gs
}

func rippleSpellCard(name string) *Card {
	return &Card{
		Name:     name,
		Types:    []string{"instant"},
		TypeLine: "Instant",
		CMC:      1,
	}
}

// TestRippleCast_CountsTowardStorm — the cast count drives storm and the
// "second spell this turn" payoffs. Pre-fix a ripple cast moved neither.
func TestRippleCast_CountsTowardStorm(t *testing.T) {
	gs := rippleCastGame(t)
	card := rippleSpellCard("Rippled Bolt")
	before := gs.SpellsCastThisTurn

	castOneRippleFree(gs, 0, rippleSpellCard("Ripple Source"), card, 4)

	if gs.SpellsCastThisTurn <= before {
		t.Fatalf("SpellsCastThisTurn = %d, want > %d — ripple cast did not count",
			gs.SpellsCastThisTurn, before)
	}
}

// TestRippleCast_FiresCastTriggerFamily — "whenever you cast a spell"
// and the instant/sorcery variant must both see a ripple cast. Observed
// through TriggerHook, the seam the discover free-cast regression uses,
// because FireCardTrigger dispatches to handlers rather than writing
// these kinds into EventLog.
func TestRippleCast_FiresCastTriggerFamily(t *testing.T) {
	gs := rippleCastGame(t)
	card := rippleSpellCard("Rippled Bolt")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	seen := map[string]bool{}
	TriggerHook = func(_ *GameState, ev string, ctx map[string]interface{}) {
		if c, _ := ctx["card"].(*Card); c == card {
			seen[ev] = true
		}
	}

	castOneRippleFree(gs, 0, rippleSpellCard("Ripple Source"), card, 4)

	for _, want := range []string{"spell_cast", "instant_or_sorcery_cast"} {
		if !seen[want] {
			t.Errorf("ripple cast did not fire %q — cast pipeline still bypassed", want)
		}
	}
}

// TestRippleCast_FiresMagecraftVisibleCast — magecraft is an ability
// word, so it has no CR entry of its own (§207.2c): magecraft
// triggers on casting an instant or sorcery, and a ripple cast is a
// cast. Magecraft keys off the instant/sorcery cast dispatch, so that
// firing for the ripple-cast card is the observable property.
func TestRippleCast_FiresMagecraftVisibleCast(t *testing.T) {
	gs := rippleCastGame(t)
	card := rippleSpellCard("Rippled Bolt")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var sawInstantCast bool
	TriggerHook = func(_ *GameState, ev string, ctx map[string]interface{}) {
		if ev != "instant_or_sorcery_cast" {
			return
		}
		if c, _ := ctx["card"].(*Card); c == card {
			sawInstantCast = true
		}
	}

	castOneRippleFree(gs, 0, rippleSpellCard("Ripple Source"), card, 4)

	if !sawInstantCast {
		t.Fatal("ripple cast fired no instant_or_sorcery_cast — magecraft cannot see it")
	}
}

// TestRippleCast_NoncreatureVariantForInstant — the family is scoped,
// so an instant must not fire the creature variant.
func TestRippleCast_NoncreatureVariantForInstant(t *testing.T) {
	gs := rippleCastGame(t)
	card := rippleSpellCard("Rippled Bolt")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var sawCreatureCast bool
	TriggerHook = func(_ *GameState, ev string, ctx map[string]interface{}) {
		if ev != "creature_spell_cast" {
			return
		}
		if c, _ := ctx["card"].(*Card); c == card {
			sawCreatureCast = true
		}
	}

	castOneRippleFree(gs, 0, rippleSpellCard("Ripple Source"), card, 4)

	if sawCreatureCast {
		t.Fatal("instant ripple cast fired creature_spell_cast")
	}
}
