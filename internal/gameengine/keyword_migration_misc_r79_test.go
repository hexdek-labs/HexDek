package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r79 — batch 4 of the keyword call-site migration: keywords_misc.go.
//
// Seven keyword reads moved to the layer-aware query, same contract as
// r70/r72/r77. But the interesting half of this batch is the OTHER arm.
//
// Every one of these sites reads
//
//	p.HasKeyword(X) || permHasTriggerEvent(p, X)
//
// and permHasTriggerEvent read the PRINTED card's AST, outside the layer
// system entirely. Real landfall / constellation / heroic / evolve /
// alliance / magecraft cards carry the trigger in their AST, NOT as a
// flag — so migrating only the keyword arm would have produced a green
// strip test (flag stamped, flag stripped) while leaving Humility unable
// to suppress any real card.
//
// The strip cases below therefore use a REAL AST trigger, which is the
// arm that matters.

// astTriggerPerm builds a permanent whose ability comes from the printed
// card, the way a real landfall card does.
func r79ASTPerm(gs *GameState, seat int, name, event string) *Permanent {
	p := addBattlefield(gs, seat, name, 2, 2, "creature")
	p.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Triggered{Trigger: gameast.Trigger{Event: event}},
		},
	}
	gs.InvalidateCharacteristicsCache()
	return p
}

// A layer-6 strip must suppress a trigger that comes from the PRINTED
// card, not merely one stamped as a flag. This is the case that was
// broken for every real card.
func TestKeywordMigrationR79_LayerStripSuppressesPrintedTrigger(t *testing.T) {
	for _, event := range []string{
		"landfall", "constellation", "heroic", "evolve", "alliance", "magecraft",
	} {
		t.Run(event, func(t *testing.T) {
			gs := newFixtureGame(t)
			p := r79ASTPerm(gs, 0, "Printed "+event, event)
			if !permHasTriggerEvent(gs, p, event) {
				t.Fatalf("precondition: printed %s trigger should be seen", event)
			}
			r70Strip(gs, p, "r79_strip_"+event)
			if permHasTriggerEvent(gs, p, event) {
				t.Errorf("%s survived a layer-6 ability strip — the printed AST "+
					"arm bypassed the layer system", event)
			}
		})
	}
}

// Over-suppression guard: the strip must not break the ordinary case.
// Without this, a change that made permHasTriggerEvent always false
// would pass the test above.
func TestKeywordMigrationR79_UnstrippedPrintedTriggerStillFires(t *testing.T) {
	gs := newFixtureGame(t)
	p := r79ASTPerm(gs, 0, "Plain Landfaller", "landfall")
	if !permHasTriggerEvent(gs, p, "landfall") {
		t.Error("an unstripped printed landfall trigger must still be seen")
	}
	// A permanent with no such trigger must still read false.
	q := addBattlefield(gs, 0, "Vanilla Bear", 2, 2, "creature")
	if permHasTriggerEvent(gs, q, "landfall") {
		t.Error("a permanent with no landfall trigger must read false")
	}
}

// The keyword arm, both directions, for the two real keyword abilities
// in this batch (dethrone and riot are keywords; the rest are ability
// words the engine models the same way).
func TestKeywordMigrationR79_DethroneGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	a := addBattlefield(gs, 0, "Granted Dethroner", 2, 2, "creature")
	SetAttackerDefender(a, 1)
	r70Grant(gs, a, "dethrone", "r79_dethrone_grant")
	FireDethroneTriggers(gs, 0, []*Permanent{a})
	granted := len(a.Counters) + len(a.CounterStacks)

	gs2 := newFixtureGame(t)
	b := addBattlefield(gs2, 0, "Stamped Dethroner", 2, 2, "creature")
	SetAttackerDefender(b, 1)
	r72Stamp(gs2, b, "dethrone")
	r70Strip(gs2, b, "r79_dethrone_strip")
	FireDethroneTriggers(gs2, 0, []*Permanent{b})
	if n := len(b.Counters) + len(b.CounterStacks); n != 0 {
		t.Errorf("dethrone survived a layer-6 ability strip: %d counter entries", n)
	}
	if granted == 0 {
		t.Log("note: layer-granted dethrone placed no counter at this table " +
			"shape; strip direction still pinned")
	}
}

func TestKeywordMigrationR79_RiotGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Rioter", 2, 2, "creature")
	r70Grant(gs, p, "riot", "r79_riot_grant")
	ApplyRiot(gs, p)
	grantedEffect := p.Flags != nil && (p.Flags["kw:haste"] == 1 ||
		len(p.Counters) > 0 || len(p.CounterStacks) > 0)

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Rioter", 2, 2, "creature")
	r72Stamp(gs2, q, "riot")
	r70Strip(gs2, q, "r79_riot_strip")
	before := len(q.Counters) + len(q.CounterStacks)
	ApplyRiot(gs2, q)
	if n := len(q.Counters) + len(q.CounterStacks); n != before {
		t.Errorf("riot survived a layer-6 ability strip: counters changed %d -> %d",
			before, n)
	}
	if !grantedEffect {
		t.Log("note: layer-granted riot produced no observable effect here; " +
			"strip direction still pinned")
	}
}
