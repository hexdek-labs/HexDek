package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r143 — "if this spell was kicked, [body]" is now ASKED rather than
// assumed. The fixture is Goblin Barrage's real shape, read off the live
// dataset: args[0] is a Modification{cast_mode_check, ["kicked"]} and
// args[1] is plain body text.

func r143Args(mode, body string) []interface{} {
	return []interface{}{
		&gameast.Modification{ModKind: "cast_mode_check", Args: []any{mode}},
		body,
	}
}

// r143Item builds the resolving SPELL item for card, optionally kicked.
func r143Item(card *Card, kicked interface{}) *StackItem {
	it := &StackItem{Kind: "spell", Card: card, CostMeta: map[string]interface{}{}}
	if kicked != nil {
		it.CostMeta["kicked"] = kicked
	}
	return it
}

// r143Resolving installs item as the innermost resolving item, the way
// ResolveStackTop does, and returns the synthetic *Permanent a handler
// would actually receive for an instant or sorcery: CostMeta-less, which
// is the whole reason this plumbing had to exist.
func r143Resolving(gs *GameState, item *StackItem) *Permanent {
	gs.resolvingItems = append(gs.resolvingItems, item)
	return &Permanent{Card: item.Card, Controller: item.Controller}
}

func TestR143_AnUnkickedSpellDoesNotGetItsKickedTail(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "R143 Barrage"}
	item := r143Item(card, false)
	src := r143Resolving(gs, item)

	allow, decided := interveningTailAllows(gs, src, r143Args("kicked", "it also deals 4 damage"))
	if !decided {
		t.Fatal("the engine could not decide a mode it demonstrably tracks (CostMeta kicked)")
	}
	if allow {
		t.Fatal("an UNKICKED spell was granted its kicked-only tail — " +
			"this is the assume-true behaviour r143 exists to stop")
	}
}

func TestR143_AKickedSpellStillGetsItsTail(t *testing.T) {
	// The other direction, and the one a too-eager gate breaks: paying
	// the kicker must still buy the effect.
	gs := newFixtureGame(t)
	card := &Card{Name: "R143 Barrage"}
	src := r143Resolving(gs, r143Item(card, true))

	allow, decided := interveningTailAllows(gs, src, r143Args("kicked", "it also deals 4 damage"))
	if !decided || !allow {
		t.Fatalf("a KICKED spell lost its tail: allow=%v decided=%v", allow, decided)
	}
}

func TestR143_AnUnevaluableModeFallsBackToAssumeTrue(t *testing.T) {
	// castModeSatisfied cannot answer "five_or_more_mana_spent" — nothing
	// tracks it. decided MUST be false so the handler keeps its old
	// behaviour. An unknown is not a false; answering "no" would strip
	// effects off cards that earned them.
	gs := newFixtureGame(t)
	card := &Card{Name: "R143 Unknown Mode"}
	src := r143Resolving(gs, r143Item(card, true))

	_, decided := interveningTailAllows(gs, src, r143Args("five_or_more_mana_spent", "body"))
	if decided {
		t.Fatal("the engine claimed to decide a mode it does not track")
	}
}

func TestR143_WithNoResolvingItemNothingIsDecided(t *testing.T) {
	// A tail reached outside a spell resolution (no item on the stack)
	// must not be answered from thin air.
	gs := newFixtureGame(t)
	src := &Permanent{Card: &Card{Name: "R143 Orphan"}}

	if _, decided := interveningTailAllows(gs, src, r143Args("kicked", "body")); decided {
		t.Fatal("decided a cast-mode question with no resolving spell")
	}
}

func TestR143_ATriggeredAbilityIsNotMistakenForTheSpell(t *testing.T) {
	// The innermost resolving item may be a TRIGGERED ability. Its
	// CostMeta describes the ability, not the spell that made the
	// permanent, so asking it "was this kicked" reads the wrong object.
	// It must not be answered.
	gs := newFixtureGame(t)
	card := &Card{Name: "R143 Trigger Source"}
	perm := &Permanent{Card: card}
	gs.resolvingItems = append(gs.resolvingItems, &StackItem{
		Kind: "triggered", Source: perm, Card: card,
		CostMeta: map[string]interface{}{"kicked": true},
	})

	if _, decided := interveningTailAllows(gs, perm, r143Args("kicked", "body")); decided {
		t.Fatal("a TRIGGERED ability's CostMeta was read as the spell's cast mode")
	}
}

func TestR143_ALegacyKindTriggeredItemIsStillNotTheSpell(t *testing.T) {
	// StackItem.Kind is documented as backward-compatible: the EMPTY
	// string means "spell (Source == nil) OR triggered ability (Source
	// != nil)". So the Kind check alone cannot separate them, and a
	// legacy triggered item would otherwise be read as the resolving
	// spell. This is the case the Source guard exists for, and it is
	// deliberately distinct from the Kind=="triggered" test above —
	// disabling the Source guard leaves that one passing.
	gs := newFixtureGame(t)
	card := &Card{Name: "R143 Legacy Trigger"}
	perm := &Permanent{Card: card}
	gs.resolvingItems = append(gs.resolvingItems, &StackItem{
		Kind: "", Source: perm, Card: card,
		CostMeta: map[string]interface{}{"kicked": true},
	})

	if _, decided := interveningTailAllows(gs, perm, r143Args("kicked", "body")); decided {
		t.Fatal("a legacy-Kind TRIGGERED item was read as the resolving spell")
	}
}

func TestR143_AnotherCardsTailIsNotAnsweredFromThisSpell(t *testing.T) {
	// The resolving spell is one card; the tail belongs to a different
	// permanent's text. Answering from the resolving item would attribute
	// one card's kicker to another's condition.
	gs := newFixtureGame(t)
	resolving := &Card{Name: "R143 The Spell"}
	gs.resolvingItems = append(gs.resolvingItems, r143Item(resolving, true))
	other := &Permanent{Card: &Card{Name: "R143 Someone Else"}}

	if _, decided := interveningTailAllows(gs, other, r143Args("kicked", "body")); decided {
		t.Fatal("answered one card's tail from a different card's resolving spell")
	}
}

func TestR143_ANonCastModeConditionIsLeftAlone(t *testing.T) {
	// if_intervening_tail covers far more than cast modes. Everything
	// that is not a cast_mode_check must keep the old path untouched.
	gs := newFixtureGame(t)
	card := &Card{Name: "R143 Other Condition"}
	src := r143Resolving(gs, r143Item(card, true))

	args := []interface{}{"if you control a Zombie", "body"}
	if _, decided := interveningTailAllows(gs, src, args); decided {
		t.Fatal("a non-cast-mode condition was claimed as decided")
	}
}
