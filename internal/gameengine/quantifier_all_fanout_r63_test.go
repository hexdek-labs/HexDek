package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — PickTarget fan-out for quantifier "all".
//
// The parser emits Quantifier="all" for untargeted mass effects ("untap
// all creatures you control", "destroy all artifacts", "creatures you
// control gain haste until end of turn"). Pre-r63, PickTarget's fan-out
// switch only recognized "each"/"each_player", so every quantifier-all
// filter fell through to the single-target policy pick and the effect
// acted on exactly ONE permanent. 834 corpus filters carry the shape;
// the goldilocks N-Z dead-effect cluster surfaced it via Village
// Bell-Ringer / Sky Hussar / Out of Time (ETB "untap all creatures you
// control"), Relentless Assault / Waves of Aggression ("untap all
// creatures that attacked"), and Riptide Chronologist ("untap all
// creatures of the chosen type").

// TestUntapAll_YouControl_FansOut pins the Village Bell-Ringer / Sky
// Hussar shape: untap all creatures you control — every own creature
// untaps, opponents stay tapped.
func TestUntapAll_YouControl_FansOut(t *testing.T) {
	gs := newFixtureGame(t)
	src := untapTestPerm(gs, 0, "Village Bell-Ringer")
	fc := untapTestPerm(gs, 0, "Bystander")
	opp := untapTestPerm(gs, 1, "Opponent Creature")

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "creature", Quantifier: "all", YouControl: true},
	})

	if src.Tapped || fc.Tapped {
		t.Errorf("all own creatures should untap: src.Tapped=%v fc.Tapped=%v", src.Tapped, fc.Tapped)
	}
	if !opp.Tapped {
		t.Errorf("opponent creature must stay tapped under a you_control filter")
	}
	if c := countEvents(gs, "untap_done"); c != 2 {
		t.Errorf("expected 2 untap_done events, got %d", c)
	}
}

// TestUntapAll_Global_FansOut pins the Riptide Chronologist / Relentless
// Assault shape: untap all creatures (no control constraint) — every
// creature on every battlefield untaps.
func TestUntapAll_Global_FansOut(t *testing.T) {
	gs := newFixtureGame(t)
	src := untapTestPerm(gs, 0, "Riptide Chronologist")
	fc := untapTestPerm(gs, 0, "Bystander")
	opp1 := untapTestPerm(gs, 1, "Opponent Creature A")
	opp2 := untapTestPerm(gs, 1, "Opponent Creature B")

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "creature", Quantifier: "all"},
	})

	for name, p := range map[string]*Permanent{"src": src, "fc": fc, "opp1": opp1, "opp2": opp2} {
		if p.Tapped {
			t.Errorf("%s should be untapped after 'untap all creatures'", name)
		}
	}
}

// TestDestroyAll_FansOut pins the breadth of the fix beyond untap: a
// Destroy with a quantifier-all filter ("destroy all artifacts") must
// hit every matching permanent, not the single policy pick.
func TestDestroyAll_FansOut(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Shatterstorm", 0, 0, "sorcery")
	a1 := addBattlefield(gs, 0, "Own Artifact", 0, 0, "artifact")
	a2 := addBattlefield(gs, 1, "Opp Artifact A", 0, 0, "artifact")
	a3 := addBattlefield(gs, 1, "Opp Artifact B", 0, 0, "artifact")
	cr := addBattlefield(gs, 1, "Opp Creature", 2, 2, "creature")

	ResolveEffect(gs, src, &gameast.Destroy{
		Target: gameast.Filter{Base: "artifact", Quantifier: "all"},
	})

	for name, p := range map[string]*Permanent{"a1": a1, "a2": a2, "a3": a3} {
		if onBattlefield(gs, p) {
			t.Errorf("artifact %s should be destroyed by 'destroy all artifacts'", name)
		}
	}
	if !onBattlefield(gs, cr) {
		t.Errorf("non-artifact creature must survive 'destroy all artifacts'")
	}
}

// TestUntapAll_NoMatch_FallsBackToSource pins the empty fan-out path:
// when no permanent matches the all-filter, resolveUntap's untap-self
// fallback still applies (unchanged from pre-r63 behavior on empty
// picks).
func TestUntapAll_NoMatch_FallsBackToSource(t *testing.T) {
	gs := newFixtureGame(t)
	src := untapTestPerm(gs, 0, "Lonely Untapper")

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "land", Quantifier: "all"},
	})

	if src.Tapped {
		t.Errorf("empty all-filter pick should fall back to untapping the source")
	}
}

// onBattlefield reports whether p is still on any seat's battlefield.
func onBattlefield(gs *GameState, p *Permanent) bool {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, q := range s.Battlefield {
			if q == p {
				return true
			}
		}
	}
	return false
}
