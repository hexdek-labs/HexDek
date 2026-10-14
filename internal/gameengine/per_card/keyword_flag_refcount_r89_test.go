package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// keyword_flag_refcount_r89_test.go — batch 1 of the migration behind
// the r87 refcount primitive (filed `6f332cbc`).
//
// Each case drives the handler's REAL entry point to make the grant, has
// a second legitimate grantor hold the same shared Flags["kw:<x>"], then
// drives the handler's REAL strip path. The keyword must survive,
// because the other source still grants it. Pre-migration every one of
// these fails: the old strip was an unconditional `delete`.
//
// The second grantor is a bare GrantKeywordFlag rather than another
// card's handler on purpose — it stands for any of the ~224 sites still
// raw, and keeps the case readable about which half is under test.

const r89OtherSource = "r89_other_source"

// r89Creature makes a creature with a real body. addPerm leaves it 0/0
// and every trigger dispatch opens a priority round, so a 0/0 is
// destroyed by the §704.5f SBA before the strip path runs — and the
// test then "passes" because the keyword is gone with the permanent.
func r89Creature(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Permanent {
	p := addPerm(gs, seat, name, types...)
	p.Card.BasePower = 2
	p.Card.BaseToughness = 2
	return p
}

func r89Assert(t *testing.T, p *gameengine.Permanent, kw string, wantCount int) {
	t.Helper()
	if !p.HasKeyword(kw) {
		t.Errorf("%q was stripped although another source still grants it (flags: %+v)", kw, p.Flags)
	}
	if n := gameengine.KeywordFlagGrantCount(p, kw); n != wantCount {
		t.Errorf("%q grant count = %d, want %d (flags: %+v)", kw, n, wantCount, p.Flags)
	}
}

// ---------------------------------------------------------------------
// Najeela — the activated ability buffs every attacking creature, so
// sharing a keyword flag with another grantor is her normal case.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR89_NajeelaStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	najeela := r89Creature(gs, 0, "Najeela, the Blade-Blossom", "creature", "warrior")
	attacker := r89Creature(gs, 0, "R89 Attacker", "creature", "warrior")
	attacker.Flags["attacking"] = 1
	// Najeela's ability is combat-phase gated.
	gs.Phase = "combat"

	// Another source already grants haste and trample to the attacker.
	gameengine.GrantKeywordFlag(attacker, "haste", r89OtherSource)
	gameengine.GrantKeywordFlag(attacker, "trample", r89OtherSource)

	najeelaActivate(gs, najeela, 0, nil)

	if !attacker.HasKeyword("lifelink") {
		t.Fatalf("fixture inert: Najeela's buff did not reach the attacker (flags: %+v)", attacker.Flags)
	}
	r89Assert(t, attacker, "haste", 2)

	// End step: Najeela's grants expire.
	gameengine.FireDelayedTriggers(gs, "end", "end_step")

	r89Assert(t, attacker, "haste", 1)
	r89Assert(t, attacker, "trample", 1)
	if attacker.HasKeyword("lifelink") {
		t.Errorf("lifelink had only Najeela behind it and must be gone (flags: %+v)", attacker.Flags)
	}
}

// ---------------------------------------------------------------------
// Leonardo — the five-colour grant, same shape on the controller's whole
// board.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR89_LeonardoStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	leo := r89Creature(gs, 0, "Leonardo, the Balance", "creature")
	ally := r89Creature(gs, 0, "R89 Ally", "creature")
	gameengine.GrantKeywordFlag(ally, "trample", r89OtherSource)

	leonardoFiveColorGrant(gs, leo, 0, nil)

	if !ally.HasKeyword("menace") {
		t.Fatalf("fixture inert: Leonardo's grant did not reach the ally (flags: %+v)", ally.Flags)
	}
	r89Assert(t, ally, "trample", 2)

	gameengine.FireDelayedTriggers(gs, "end", "end_step")

	r89Assert(t, ally, "trample", 1)
	if ally.HasKeyword("menace") {
		t.Errorf("menace had only Leonardo behind it and must be gone (flags: %+v)", ally.Flags)
	}
}

// ---------------------------------------------------------------------
// Commander Mustard — the static soldier anthem. This one ALSO removes a
// phantom keyword: its provenance flag used to be written as
// "kw:from_commander_mustard", inside the namespace runtimeKeywordSources
// folds verbatim, so it reached chars.Keywords as a keyword of that name
// (measured in TestKeywordFoldR89_… in the gameengine package).
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR89_MustardStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	mustard := r89Creature(gs, 0, "Commander Mustard", "creature", "soldier")
	soldier := r89Creature(gs, 0, "R89 Soldier", "creature", "soldier")
	gameengine.GrantKeywordFlag(soldier, "haste", r89OtherSource)

	mustardRefreshSoldierAnthem(gs, mustard)

	if !soldier.HasKeyword("vigilance") {
		t.Fatalf("fixture inert: Mustard's anthem did not reach the soldier (flags: %+v)", soldier.Flags)
	}
	r89Assert(t, soldier, "haste", 2)

	mustardStripSoldierAnthem(gs, mustard)

	r89Assert(t, soldier, "haste", 1)
	if soldier.HasKeyword("vigilance") {
		t.Errorf("vigilance had only Mustard behind it and must be gone (flags: %+v)", soldier.Flags)
	}
}

// TestKeywordFlagRefcountR89_MustardProvenanceIsNoLongerAKeyword pins
// the phantom this migration removed. Before r89 the soldier's keyword
// set contained "from_commander_mustard".
func TestKeywordFlagRefcountR89_MustardProvenanceIsNoLongerAKeyword(t *testing.T) {
	gs := newGame(t, 2)
	mustard := r89Creature(gs, 0, "Commander Mustard", "creature", "soldier")
	soldier := r89Creature(gs, 0, "R89 Soldier", "creature", "soldier")

	mustardRefreshSoldierAnthem(gs, mustard)

	for k := range soldier.Flags {
		if k == "kw:from_commander_mustard" {
			t.Errorf("the provenance tag is back inside the kw: namespace, where the keyword fold takes it verbatim")
		}
	}
	if soldier.HasKeyword("from_commander_mustard") {
		t.Errorf("provenance is answering as a keyword (flags: %+v)", soldier.Flags)
	}
}

// ---------------------------------------------------------------------
// Dihada — her -11 mass threaten carried a hand-rolled "did it already
// have haste?" snapshot. That snapshot could only see grantors present
// at the moment she stamped; a grantor arriving AFTER her was still
// clobbered on the revert. This case is that second half, which the old
// bookkeeping could not pass however carefully it was written.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR89_DihadaRevertKeepsLaterGrantor(t *testing.T) {
	gs := newGame(t, 2)
	dihada := r89Creature(gs, 0, "Dihada, Binder of Wills", "planeswalker")
	victim := r89Creature(gs, 1, "R89 Victim", "creature")

	dihadaMinusEleven(gs, dihada)

	if !victim.HasKeyword("haste") {
		t.Fatalf("fixture inert: Dihada's threaten did not reach the victim (flags: %+v)", victim.Flags)
	}
	// A SECOND grantor arrives after Dihada's stamp — the case the old
	// pre-state snapshot was structurally unable to see.
	gameengine.GrantKeywordFlag(victim, "haste", r89OtherSource)
	r89Assert(t, victim, "haste", 2)

	gameengine.FireDelayedTriggers(gs, "end", "end_step")

	r89Assert(t, victim, "haste", 1)
}
