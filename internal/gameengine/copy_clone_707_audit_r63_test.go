package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// copy_clone_707_audit_r63_test.go — CR §707 copy/clone mechanic audit.
// Exercises the copiable-value contract (§707.2 / §706.2), copy-spell
// (§707.10), legend-rule-on-clones (§704.5j), and enters-as-a-copy
// counter timing. Direct-API style (CopyPermanentLayered / resolveCopySpell
// / StateBasedActions) mirroring copy_effects_test.go.

// ---------------------------------------------------------------------------
// §707.2 — a clone copies only COPIABLE values: NOT counters, NOT continuous
// P/T effects on the source. Cloning a buffed creature yields PRINTED P/T.
// ---------------------------------------------------------------------------

func TestCopy707_CloneOfCounterBuffedCreature_CopiesPrintedNotCounters(t *testing.T) {
	gs := newFixtureGame(t)

	// Source: printed 2/2 with three +1/+1 counters → effective 5/5.
	src := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	src.Card.TypeLine = "Creature — Bear"
	src.AddCounter("+1/+1", 3)
	gs.InvalidateCharacteristicsCache()
	if c := GetEffectiveCharacteristics(gs, src); c.Power != 5 || c.Toughness != 5 {
		t.Fatalf("fixture: buffed source should be 5/5; got %d/%d", c.Power, c.Toughness)
	}

	// Clone it.
	clone := addBattlefieldWithAST(gs, 0, "Clone", 0, 0, &gameast.CardAST{Name: "Clone"}, "creature")
	CopyPermanentLayered(gs, clone, src, DurationPermanent)

	// §707.2: counters are NOT copiable. The clone copies the PRINTED 2/2,
	// and has zero counters of its own → effective 2/2.
	cc := GetEffectiveCharacteristics(gs, clone)
	if cc.Power != 2 || cc.Toughness != 2 {
		t.Errorf("clone of a counter-buffed creature must be PRINTED 2/2 (counters not copiable); got %d/%d", cc.Power, cc.Toughness)
	}
	if clone.Counters["+1/+1"] != 0 {
		t.Errorf("clone must not inherit the source's +1/+1 counters; got %d", clone.Counters["+1/+1"])
	}
}

// A clone copies only copiable values, NOT a continuous P/T effect (anthem)
// applied to the source. (The anthem still applies to the clone afterward if
// the clone qualifies — but the COPIED base must be printed.)
func TestCopy707_CloneOfAnthemBuffedCreature_CopiesPrintedBase(t *testing.T) {
	gs := newFixtureGame(t)

	// An anthem source: "creatures you control get +2/+2" via a layer-7c
	// continuous effect targeting the controller's creatures.
	anthem := addBattlefield(gs, 0, "Glorious Anthem", 0, 0, "enchantment")
	registerAnthemPT(gs, anthem, 2, 2, "test-anthem", func(_ *GameState, t *Permanent) bool {
		return t != nil && t.IsCreature() && t.Controller == 0
	})

	src := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	src.Card.TypeLine = "Creature — Bear"
	gs.InvalidateCharacteristicsCache()
	if c := GetEffectiveCharacteristics(gs, src); c.Power != 4 {
		t.Fatalf("fixture: source under anthem should be 4/4; got %d/%d", c.Power, c.Toughness)
	}

	clone := addBattlefieldWithAST(gs, 0, "Clone", 0, 0, &gameast.CardAST{Name: "Clone"}, "creature")
	CopyPermanentLayered(gs, clone, src, DurationPermanent)
	gs.InvalidateCharacteristicsCache()

	// The clone's COPIED base is 2/2 (printed). It is itself under the
	// anthem (it's a creature you control), so effective = 4/4 — but that
	// 4/4 comes from the clone's OWN anthem membership, not from copying the
	// source's buffed value. Verify base via BaseCharacteristics of the
	// swapped card.
	if bc := BaseCharacteristics(clone); bc.Power != 2 || bc.Toughness != 2 {
		t.Errorf("clone's copied BASE must be printed 2/2 (anthem not copiable); got %d/%d", bc.Power, bc.Toughness)
	}
}

// ---------------------------------------------------------------------------
// §706.2 — enters-with-counters is a copiable ABILITY: a clone of a creature
// whose printed text says "enters with N counters" gains that ability and,
// AS IT ENTERS AS A COPY, gets its OWN N counters — but does NOT inherit the
// source's EXISTING counters.
// ---------------------------------------------------------------------------

func TestCopy707_CloneOfEntersWithCounters_GetsOwnNotSource(t *testing.T) {
	gs := newFixtureGame(t)

	// Source: District Mascot shape — printed 0/0, "enters with one +1/+1
	// counter", grown to THREE counters over the game (effective 3/3).
	srcAST := &gameast.CardAST{
		Name: "District Mascot",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{ModKind: "etb_with_counters", Args: []interface{}{1, "+1/+1"}}},
		},
	}
	src := addBattlefieldWithAST(gs, 0, "District Mascot", 0, 0, srcAST, "creature")
	src.AddCounter("+1/+1", 3)
	gs.InvalidateCharacteristicsCache()

	// Clone enters as a copy. Model the §706.9 "enters as a copy"
	// replacement: apply the copy, THEN run the entering permanent's ETB
	// self-replacement (ApplyStaticETBCounters) on the now-copied identity.
	clone := addBattlefieldWithAST(gs, 0, "Clone", 0, 0, &gameast.CardAST{Name: "Clone"}, "creature")
	CopyPermanentLayered(gs, clone, src, DurationPermanent)
	ApplyStaticETBCounters(gs, clone) // the copied "enters with 1 counter" static fires as it enters
	gs.InvalidateCharacteristicsCache()

	// Clone: its OWN one counter from the copied static → 1/1. NOT the
	// source's three (counters aren't copiable).
	if clone.Counters["+1/+1"] != 1 {
		t.Errorf("clone of enters-with-counters creature must get its OWN 1 counter (not source's 3); got %d", clone.Counters["+1/+1"])
	}
	if cc := GetEffectiveCharacteristics(gs, clone); cc.Power != 1 || cc.Toughness != 1 {
		t.Errorf("clone should be 1/1 (0/0 printed + its own etb counter); got %d/%d", cc.Power, cc.Toughness)
	}
	// Source unchanged at 3/3.
	if sc := GetEffectiveCharacteristics(gs, src); sc.Power != 3 {
		t.Errorf("source should remain 3/3; got %d/%d", sc.Power, sc.Toughness)
	}
}

// ---------------------------------------------------------------------------
// §704.5j — Legend rule applies to clones: a clone of a legendary permanent
// you control creates two same-name legends; SBA sends one to the graveyard.
// ---------------------------------------------------------------------------

func TestCopy707_LegendRuleOnClones(t *testing.T) {
	gs := newFixtureGame(t)

	legend := addBattlefieldWithAST(gs, 0, "Krenko, Mob Boss", 3, 3,
		&gameast.CardAST{Name: "Krenko, Mob Boss"}, "creature", "legendary")
	legend.Card.TypeLine = "Legendary Creature — Goblin Warrior"

	clone := addBattlefieldWithAST(gs, 0, "Clone", 0, 0, &gameast.CardAST{Name: "Clone"}, "creature")
	CopyPermanentLayered(gs, clone, legend, DurationPermanent)
	gs.InvalidateCharacteristicsCache()

	// Two "Krenko, Mob Boss" legends controlled by seat 0.
	if GetEffectiveCharacteristics(gs, clone).Name != "Krenko, Mob Boss" {
		t.Fatalf("clone should be named Krenko after copy")
	}
	StateBasedActions(gs)

	// Exactly one Krenko remains on seat 0's battlefield (§704.5j).
	krenkos := 0
	for _, p := range gs.Seats[0].Battlefield {
		if GetEffectiveCharacteristics(gs, p).Name == "Krenko, Mob Boss" {
			krenkos++
		}
	}
	if krenkos != 1 {
		t.Errorf("legend rule must leave exactly ONE Krenko; got %d on battlefield", krenkos)
	}
}

// ---------------------------------------------------------------------------
// §707.10 — copy a spell: the copy is created on the stack, may keep or
// choose new targets, and (for a non-permanent spell) CEASES to exist on
// resolution rather than going to a graveyard.
// ---------------------------------------------------------------------------

func TestCopy707_CopySpell_NewTargetsAndOnStack(t *testing.T) {
	gs := newFixtureGame(t)
	// Original instant on the stack targeting seat 1.
	orig := &StackItem{
		ID: 1, Controller: 0, Kind: "instant",
		Card:    &Card{Name: "Lightning Bolt", Types: []string{"instant"}},
		Targets: []Target{{Kind: TargetKindSeat, Seat: 1}},
	}
	gs.Stack = []*StackItem{orig}

	src := addBattlefield(gs, 0, "Twincast Source", 0, 0, "creature")
	resolveCopySpell(gs, src, &gameast.CopySpell{MayChooseNewTargets: true})

	// A copy is on the stack, flagged IsCopy, above the original.
	if len(gs.Stack) != 2 {
		t.Fatalf("copy-spell must push one copy onto the stack; stack len=%d", len(gs.Stack))
	}
	top := gs.Stack[len(gs.Stack)-1]
	if !top.IsCopy {
		t.Errorf("the pushed copy must be flagged IsCopy")
	}
	if top.Card.DisplayName() != "Lightning Bolt" {
		t.Errorf("copy must be of Lightning Bolt; got %q", top.Card.DisplayName())
	}
	// §707.10c — the copy can take new targets independent of the original.
	top.Targets = []Target{{Kind: TargetKindSeat, Seat: 2}}
	if len(orig.Targets) != 1 || orig.Targets[0].Seat != 1 {
		t.Errorf("retargeting the copy must not mutate the original's targets; got %+v", orig.Targets)
	}
}

// PRODUCTION-PATH regression for the enters-as-a-copy counter-timing bug:
// a clone's copy is applied in the per-card OnETB hook (InvokeETBHook),
// which fires AFTER ApplyStaticETBCounters in resolvePermanentSpellETB. So
// a clone of a "enters with N counters" creature must STILL receive its own
// N counters — the engine re-applies the copied §614.1d self-replacements
// after the ETB hook turns the permanent into a copy.
func TestCopy707_EntersAsCopy_ProductionPath_GetsEtbCounters(t *testing.T) {
	gs := newFixtureGame(t)

	// Source on the battlefield: "enters with one +1/+1 counter", grown to 3.
	srcAST := &gameast.CardAST{
		Name: "District Mascot",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{ModKind: "etb_with_counters", Args: []interface{}{1, "+1/+1"}}},
		},
	}
	src := addBattlefieldWithAST(gs, 0, "District Mascot", 0, 0, srcAST, "creature")
	src.AddCounter("+1/+1", 3)
	gs.InvalidateCharacteristicsCache()

	// Install a clone ETB hook (models Phantasmal Image / Clone): when the
	// entering "Clone" permanent resolves, become a copy of the source.
	saved := ETBHook
	defer func() { ETBHook = saved }()
	ETBHook = func(g *GameState, perm *Permanent) {
		if perm == nil || perm.Card == nil || perm.Card.Name != "Clone" {
			return
		}
		if cp := BecomeCopyOfCard(g, perm, src.Card); cp != nil {
			perm.OriginalCard = perm.Card
			perm.Card = cp
		}
	}

	// Resolve a "Clone" creature permanent spell through the real ETB path.
	cloneCard := &Card{Name: "Clone", Owner: 0, BasePower: 0, BaseToughness: 0, Types: []string{"creature"}}
	item := &StackItem{ID: 99, Controller: 0, Kind: "creature", Card: cloneCard}
	clone := resolvePermanentSpellETB(gs, item)
	if clone == nil {
		t.Fatal("clone permanent failed to enter")
	}
	gs.InvalidateCharacteristicsCache()

	// The clone became District Mascot and must have its OWN 1 counter from
	// the copied "enters with a counter" static — NOT 0, NOT the source's 3.
	if clone.Counters["+1/+1"] != 1 {
		t.Errorf("clone-of-enters-with-counters must get 1 own counter via the copied static; got %d (0 = the copy-applied-after-ETB-counters timing bug)", clone.Counters["+1/+1"])
	}
	if c := GetEffectiveCharacteristics(gs, clone); c.Name != "District Mascot" || c.Power != 1 {
		t.Errorf("clone should be a 1/1 District Mascot; got %s %d/%d", c.Name, c.Power, c.Toughness)
	}
}

func TestCopy707_CopySpellCeasesAsSBA(t *testing.T) {
	gs := newFixtureGame(t)
	// A resolving copy of a non-permanent spell must cease, NOT go to a
	// graveyard. Drive ResolveStackTop on a copy item with no effect.
	owner := 0
	copyItem := &StackItem{
		ID: 7, Controller: owner, Kind: "instant", IsCopy: true,
		Card: &Card{Name: "Bolt Copy", Owner: owner, Types: []string{"instant"}},
	}
	gs.Stack = []*StackItem{copyItem}
	gyBefore := len(gs.Seats[owner].Graveyard)

	ResolveStackTop(gs)

	if len(gs.Stack) != 0 {
		t.Fatalf("stack should be empty after the copy resolves; len=%d", len(gs.Stack))
	}
	if got := len(gs.Seats[owner].Graveyard) - gyBefore; got != 0 {
		t.Errorf("a copy of a non-permanent spell must CEASE, not enter a graveyard; graveyard grew by %d", got)
	}
}
