package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// beneficial_target_r61_test.go — r61 PR-8 Fix B.
//
// Pins the new beneficial-effect branch in ChooseTarget: a single-target
// BENEFICIAL effect (pump / +1/+1 / regen / protective keyword grant)
// targets the AI's OWN creature, while a HARMFUL effect (damage / destroy)
// still targets the best ENEMY creature. The harmful case directly guards
// the PR-7 reactive-removal reliance on best-enemy targeting being
// unchanged.

// pushOwnSpell puts a stack item controlled by seatIdx carrying `eff` onto
// the stack, mirroring the cast-time targeting window (CR §601.2c) the hat
// reads to classify intent.
func pushOwnSpell(gs *gameengine.GameState, seatIdx int, name string, eff gameast.Effect) {
	gs.Stack = append(gs.Stack, &gameengine.StackItem{
		ID:         len(gs.Stack) + 1,
		Controller: seatIdx,
		Card:       newTestCardMinimal(name, []string{"instant"}, 1, nil),
		Effect:     eff,
	})
}

// -----------------------------------------------------------------------
// classifier unit cases
// -----------------------------------------------------------------------

func TestClassifyTargetIntent_PumpIsBeneficial(t *testing.T) {
	eff := &gameast.Buff{Power: 3, Toughness: 3, Target: gameast.Filter{Base: "creature", Targeted: true}}
	if got := classifyTargetIntent(eff); got != intentBeneficial {
		t.Fatalf("+3/+3 pump should be beneficial; got %v", got)
	}
}

func TestClassifyTargetIntent_NegativeBuffIsHarmful(t *testing.T) {
	eff := &gameast.Buff{Power: -3, Toughness: -3, Target: gameast.Filter{Base: "creature", Targeted: true}}
	if got := classifyTargetIntent(eff); got != intentHarmful {
		t.Fatalf("-3/-3 shrink should be harmful; got %v", got)
	}
}

func TestClassifyTargetIntent_PlusCountersBeneficial(t *testing.T) {
	eff := &gameast.CounterMod{Op: "put", CounterKind: "+1/+1", Target: gameast.Filter{Base: "creature", Targeted: true}}
	if got := classifyTargetIntent(eff); got != intentBeneficial {
		t.Fatalf("put +1/+1 counter should be beneficial; got %v", got)
	}
}

func TestClassifyTargetIntent_MinusCountersHarmful(t *testing.T) {
	eff := &gameast.CounterMod{Op: "put", CounterKind: "-1/-1", Target: gameast.Filter{Base: "creature", Targeted: true}}
	if got := classifyTargetIntent(eff); got != intentHarmful {
		t.Fatalf("put -1/-1 counter should be harmful; got %v", got)
	}
}

func TestClassifyTargetIntent_ProtectiveGrantBeneficial(t *testing.T) {
	for _, kw := range []string{"indestructible", "hexproof", "protection from red", "trample"} {
		eff := &gameast.GrantAbility{AbilityName: kw, Target: gameast.Filter{Base: "creature", Targeted: true}}
		if got := classifyTargetIntent(eff); got != intentBeneficial {
			t.Fatalf("grant %q should be beneficial; got %v", kw, got)
		}
	}
}

func TestClassifyTargetIntent_DamageAndDestroyHarmful(t *testing.T) {
	if got := classifyTargetIntent(&gameast.Damage{Target: gameast.Filter{Base: "creature", Targeted: true}}); got != intentHarmful {
		t.Fatalf("damage should be harmful; got %v", got)
	}
	if got := classifyTargetIntent(&gameast.Destroy{Target: gameast.Filter{Base: "creature", Targeted: true}}); got != intentHarmful {
		t.Fatalf("destroy should be harmful; got %v", got)
	}
}

func TestClassifyTargetIntent_MixedBuffAndDamageIsHarmful(t *testing.T) {
	// Conservative composition: a spell that both pumps AND damages must
	// NOT be classified beneficial (any harmful leaf → harmful).
	eff := &gameast.Sequence{Items: []gameast.Effect{
		&gameast.Buff{Power: 2, Toughness: 2},
		&gameast.Damage{Target: gameast.Filter{Base: "creature", Targeted: true}},
	}}
	if got := classifyTargetIntent(eff); got != intentHarmful {
		t.Fatalf("mixed buff+damage must stay harmful; got %v", got)
	}
}

func TestClassifyTargetIntent_NilAndUnknown(t *testing.T) {
	if got := classifyTargetIntent(nil); got != intentUnknown {
		t.Fatalf("nil effect should be unknown; got %v", got)
	}
	if got := classifyTargetIntent(&gameast.Draw{}); got != intentUnknown {
		t.Fatalf("draw (no target creature) should be unknown; got %v", got)
	}
}

// -----------------------------------------------------------------------
// ChooseTarget integration — the headline Fix B contract
// -----------------------------------------------------------------------

// TestChooseTarget_BeneficialPumpTargetsOwnCreature is the core win: a
// +1/+1 pump whose filter admits both own and opponent creatures targets
// the AI's OWN creature, NOT the strongest enemy.
func TestChooseTarget_BeneficialPumpTargetsOwnCreature(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	own := mkTargetCreature(gs, 0, "My Bear", 2, 2, nil, "")
	enemy := mkTargetCreature(gs, 1, "Their Dragon", 6, 6, nil, "")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0

	// A +3/+3 pump is resolving (own seat-0 spell on the stack).
	pushOwnSpell(gs, 0, "Giant Growth", &gameast.Buff{
		Power: 3, Toughness: 3,
		Target: gameast.Filter{Base: "creature", Targeted: true},
	})

	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: own},
		{Kind: gameengine.TargetKindPermanent, Permanent: enemy},
	}
	picked := h.ChooseTarget(gs, 0, gameast.Filter{Base: "creature", Targeted: true}, legal)
	if picked.Permanent != own {
		got := "<nil>"
		if picked.Permanent != nil && picked.Permanent.Card != nil {
			got = picked.Permanent.Card.Name
		}
		t.Fatalf("beneficial pump should target OWN creature (My Bear); got %s", got)
	}
}

// TestChooseTarget_BeneficialProtectionTargetsOwnCreature pins the
// protection/regen case (grant indestructible).
func TestChooseTarget_BeneficialProtectionTargetsOwnCreature(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	own := mkTargetCreature(gs, 0, "My Knight", 4, 4, nil, "")
	enemy := mkTargetCreature(gs, 1, "Their Wurm", 7, 7, nil, "")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0

	pushOwnSpell(gs, 0, "Shelter", &gameast.GrantAbility{
		AbilityName: "indestructible",
		Target:      gameast.Filter{Base: "creature", Targeted: true},
	})

	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: own},
		{Kind: gameengine.TargetKindPermanent, Permanent: enemy},
	}
	picked := h.ChooseTarget(gs, 0, gameast.Filter{Base: "creature", Targeted: true}, legal)
	if picked.Permanent != own {
		got := "<nil>"
		if picked.Permanent != nil && picked.Permanent.Card != nil {
			got = picked.Permanent.Card.Name
		}
		t.Fatalf("protective grant should target OWN creature (My Knight); got %s", got)
	}
}

// TestChooseTarget_BeneficialPicksBiggestOwnAttacker — among multiple own
// creatures, the buff goes on the biggest (the attacker the pump pushes
// through).
func TestChooseTarget_BeneficialPicksBiggestOwnAttacker(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	small := mkTargetCreature(gs, 0, "Small", 1, 1, nil, "")
	big := mkTargetCreature(gs, 0, "Big", 5, 5, nil, "")
	enemy := mkTargetCreature(gs, 1, "Enemy", 6, 6, nil, "")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0

	pushOwnSpell(gs, 0, "Pump", &gameast.Buff{
		Power: 2, Toughness: 2,
		Target: gameast.Filter{Base: "creature", Targeted: true},
	})

	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: small},
		{Kind: gameengine.TargetKindPermanent, Permanent: big},
		{Kind: gameengine.TargetKindPermanent, Permanent: enemy},
	}
	picked := h.ChooseTarget(gs, 0, gameast.Filter{Base: "creature", Targeted: true}, legal)
	if picked.Permanent != big {
		got := "<nil>"
		if picked.Permanent != nil && picked.Permanent.Card != nil {
			got = picked.Permanent.Card.Name
		}
		t.Fatalf("beneficial pump should target the biggest OWN attacker (Big); got %s", got)
	}
}

// TestChooseTarget_HarmfulStillTargetsEnemy is the CRITICAL regression
// guard for PR-7: a HARMFUL effect (destroy) must STILL target the best
// ENEMY creature — the beneficial branch must not intercept removal.
func TestChooseTarget_HarmfulStillTargetsEnemy(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	own := mkTargetCreature(gs, 0, "My Bear", 2, 2, nil, "")
	enemy := mkTargetCreature(gs, 1, "Their Dragon", 6, 6, nil, "")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0

	// A destroy spell is resolving.
	pushOwnSpell(gs, 0, "Murder", &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Targeted: true},
	})

	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: own},
		{Kind: gameengine.TargetKindPermanent, Permanent: enemy},
	}
	picked := h.ChooseTarget(gs, 0, gameast.Filter{Base: "creature", Targeted: true}, legal)
	if picked.Permanent != enemy {
		got := "<nil>"
		if picked.Permanent != nil && picked.Permanent.Card != nil {
			got = picked.Permanent.Card.Name
		}
		t.Fatalf("harmful destroy must STILL target the best ENEMY (Their Dragon); got %s", got)
	}
}

// TestChooseTarget_NoResolvingEffectKeepsRemovalBehavior pins that an
// EMPTY stack (no intent signal) keeps the existing removal behavior — the
// same path the existing target_priority_signals tests exercise. Confirms
// the beneficial branch is strictly additive (unknown → harmful default).
func TestChooseTarget_NoResolvingEffectKeepsRemovalBehavior(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	own := mkTargetCreature(gs, 0, "My Bear", 2, 2, nil, "")
	enemy := mkTargetCreature(gs, 1, "Their Dragon", 6, 6, nil, "")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0

	// No stack item → intentUnknown → existing removal scorer → enemy.
	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: own},
		{Kind: gameengine.TargetKindPermanent, Permanent: enemy},
	}
	picked := h.ChooseTarget(gs, 0, gameast.Filter{Base: "creature", Targeted: true}, legal)
	if picked.Permanent != enemy {
		got := "<nil>"
		if picked.Permanent != nil && picked.Permanent.Card != nil {
			got = picked.Permanent.Card.Name
		}
		t.Fatalf("no resolving effect → existing removal behavior (best enemy); got %s", got)
	}
}
