package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r93 — keyword-flag refcount batch 3: the seven handlers that paired a
// "skip anything that already has the keyword" guard with a delete of
// the SHARED Flags["kw:<x>"].
//
// The guard is the self-defeating shape r87's own doc names and r91
// found still live in Old One Eye. It is not caution: a printed keyword
// lives in the AST and no strip in the tree can reach it, so the only
// thing the guard ever skipped was ANOTHER grant's shared flag — which
// made the handler decline to record its own grant, and the other
// source then took the keyword away while this one was still live.
//
// The discriminating direction is therefore the GRANT one: let another
// source hold the flag first, run the handler, then let that source
// leave. Pre-migration the keyword vanishes.

const r93OtherSource = "r93_other_source"

// r93FireDelayed runs every registered delayed trigger's EffectFn,
// mirroring the production runners at phases.go:639 and state.go:692 —
// which, note, do NOT invalidate the characteristics cache around the
// call. That is why each EffectFn has to do it itself.
func r93FireDelayed(gs *gameengine.GameState) {
	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.EffectFn != nil {
			dt.EffectFn(gs)
		}
	}
	gs.DelayedTriggers = nil
}

// r93SurvivesOtherSourceStrip is the shared body of the grant-direction
// case: other source grants kw, the handler runs, other source leaves,
// kw must still be there because the handler recorded its own grant.
func r93SurvivesOtherSourceStrip(t *testing.T, gs *gameengine.GameState, p *gameengine.Permanent, kw string) {
	t.Helper()
	if n := gameengine.KeywordFlagGrantCount(p, kw); n != 2 {
		t.Fatalf("%q grant count = %d, want 2 (the other source plus the handler's own) — flags: %+v", kw, n, p.Flags)
	}
	gameengine.RevokeKeywordFlag(p, kw, r93OtherSource)
	if !p.HasKeyword(kw) {
		t.Errorf("%q vanished when the OTHER source left, although the handler is still granting it (flags: %+v)", kw, p.Flags)
	}
	if !gs.HasKeywordOf(p, kw) {
		t.Errorf("%q is gone on the layer-aware authority after the other source left (flags: %+v)", kw, p.Flags)
	}
}

// ---------------------------------------------------------------------
// Vito — "{3}{B}{B}: Creatures you control gain lifelink until end of
// turn". Guard was p.HasKeyword("lifelink").
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR93_VitoGrantSurvivesOtherSourceStrip(t *testing.T) {
	gs := newGame(t, 2)
	vito := r89Creature(gs, 0, "Vito, Thorn of the Dusk Rose", "creature", "vampire")
	ally := r89Creature(gs, 0, "R93 Ally", "creature")
	gameengine.GrantKeywordFlag(ally, "lifelink", r93OtherSource)
	gs.Seats[0].ManaPool = 10

	vitoThornGrantLifelink(gs, vito)

	if !ally.HasKeyword("lifelink") {
		t.Fatalf("fixture inert: Vito's grant did not reach the ally (flags: %+v)", ally.Flags)
	}
	r93SurvivesOtherSourceStrip(t, gs, ally, "lifelink")
}

// Over-suppression guard, measured quiet against the revert. It passes
// pre-change too, but for the wrong reason: the guard made the handler
// skip the permanent entirely, so there was no grant to strip and the
// other source survived by accident. Post-change it survives because
// the refcount held it. What this pins is that the migration did not
// introduce a strip that eats a co-held grant.
func TestKeywordFlagRefcountR93_VitoStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	vito := r89Creature(gs, 0, "Vito, Thorn of the Dusk Rose", "creature", "vampire")
	ally := r89Creature(gs, 0, "R93 Ally", "creature")
	gameengine.GrantKeywordFlag(ally, "lifelink", r93OtherSource)
	gs.Seats[0].ManaPool = 10

	vitoThornGrantLifelink(gs, vito)
	r93FireDelayed(gs)

	r89Assert(t, ally, "lifelink", 1)
}

// ---------------------------------------------------------------------
// Balmor — "creatures you control get +1/+0 and gain trample until end
// of turn". Guard was p.Flags["kw:trample"] == 0.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR93_BalmorGrantSurvivesOtherSourceStrip(t *testing.T) {
	gs := newGame(t, 2)
	balmor := r89Creature(gs, 0, "Balmor, Battlemage Captain", "creature", "dragon")
	ally := r89Creature(gs, 0, "R93 Balmor Ally", "creature")
	gameengine.GrantKeywordFlag(ally, "trample", r93OtherSource)

	balmorTeamPump(gs, balmor, map[string]interface{}{})

	if !ally.HasKeyword("trample") {
		t.Fatalf("fixture inert: Balmor's pump did not reach the ally (flags: %+v)", ally.Flags)
	}
	r93SurvivesOtherSourceStrip(t, gs, ally, "trample")
}

// TestKeywordFlagRefcountR93_BalmorStripInvalidates pins the cache half.
// The strip lives in a delayed-trigger EffectFn and neither runner
// invalidates around the call, so before r93 the layer-aware authority
// kept answering true from an entry taken while the grant was live.
func TestKeywordFlagRefcountR93_BalmorStripInvalidates(t *testing.T) {
	gs := newGame(t, 2)
	balmor := r89Creature(gs, 0, "Balmor, Battlemage Captain", "creature", "dragon")
	ally := r89Creature(gs, 0, "R93 Balmor Cache Ally", "creature")

	balmorTeamPump(gs, balmor, map[string]interface{}{})

	// Prime the characteristics cache through the layer-aware authority
	// while the grant is live. Without this read the test passes against
	// the bug, because there is no stale entry to serve.
	if !gs.HasKeywordOf(ally, "trample") {
		t.Fatalf("fixture inert: Balmor's trample did not reach the layer authority (flags: %+v)", ally.Flags)
	}

	r93FireDelayed(gs)

	if gs.HasKeywordOf(ally, "trample") {
		t.Errorf("layer authority still reports trample after the strip — the EffectFn did not invalidate (flags: %+v)", ally.Flags)
	}
}

// ---------------------------------------------------------------------
// Cecil, Redeemed Paladin — attacking creatures gain indestructible.
// Guard was p.Flags["kw:indestructible"] == 0.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR93_CecilGrantSurvivesOtherSourceStrip(t *testing.T) {
	gs := newGame(t, 2)
	cecil := r89Creature(gs, 0, "Cecil, Redeemed Paladin", "creature", "knight")
	cecil.Transformed = true
	cecil.Flags["attacking"] = 1
	ally := r89Creature(gs, 0, "R93 Cecil Ally", "creature")
	ally.Flags["attacking"] = 1
	gameengine.GrantKeywordFlag(ally, "indestructible", r93OtherSource)

	cecilAttackProtect(gs, cecil, map[string]interface{}{"attacker_perm": cecil})

	if !ally.HasKeyword("indestructible") {
		t.Fatalf("fixture inert: Cecil's grant did not reach the ally (flags: %+v)", ally.Flags)
	}
	r93SurvivesOtherSourceStrip(t, gs, ally, "indestructible")
}

// ---------------------------------------------------------------------
// Abzan Falconer — creatures with a +1/+1 counter have flying. Guards
// were p.HasKeyword("flying") AND p.Flags["kw:flying"] == 1.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR93_AbzanGrantSurvivesOtherSourceStrip(t *testing.T) {
	gs := newGame(t, 2)
	abzan := r89Creature(gs, 0, "Abzan Falconer", "creature", "soldier")
	ally := r89Creature(gs, 0, "R93 Abzan Ally", "creature")
	ally.AddCounter("+1/+1", 1)
	gameengine.GrantKeywordFlag(ally, "flying", r93OtherSource)

	abzanFalconerGrantSweep(gs, abzan)

	if !ally.HasKeyword("flying") {
		t.Fatalf("fixture inert: Abzan's sweep did not reach the ally (flags: %+v)", ally.Flags)
	}
	r93SurvivesOtherSourceStrip(t, gs, ally, "flying")
}

// Over-suppression guard, measured quiet against the revert. It passes
// pre-change too, but for the wrong reason: the guard made the handler
// skip the permanent entirely, so there was no grant to strip and the
// other source survived by accident. Post-change it survives because
// the refcount held it. What this pins is that the migration did not
// introduce a strip that eats a co-held grant.
func TestKeywordFlagRefcountR93_AbzanStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	abzan := r89Creature(gs, 0, "Abzan Falconer", "creature", "soldier")
	ally := r89Creature(gs, 0, "R93 Abzan Ally", "creature")
	ally.AddCounter("+1/+1", 1)
	gameengine.GrantKeywordFlag(ally, "flying", r93OtherSource)

	abzanFalconerGrantSweep(gs, abzan)
	abzanFalconerOwnLTB(gs, abzan, map[string]interface{}{"perm": abzan})

	r89Assert(t, ally, "flying", 1)
}

// ---------------------------------------------------------------------
// Fearless Swashbuckler — Vehicles you control have haste. Guard was
// p.HasKeyword("haste").
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR93_SwashbucklerGrantSurvivesOtherSourceStrip(t *testing.T) {
	gs := newGame(t, 2)
	swash := r89Creature(gs, 0, "Fearless Swashbuckler", "creature", "pirate")
	veh := r89Creature(gs, 0, "R93 Vehicle", "artifact", "vehicle")
	gameengine.GrantKeywordFlag(veh, "haste", r93OtherSource)

	fearlessSwashbucklerHasteSweep(gs, swash)

	if !veh.HasKeyword("haste") {
		t.Fatalf("fixture inert: the Swashbuckler's sweep did not reach the Vehicle (flags: %+v)", veh.Flags)
	}
	r93SurvivesOtherSourceStrip(t, gs, veh, "haste")
}

// Over-suppression guard, measured quiet against the revert. It passes
// pre-change too, but for the wrong reason: the guard made the handler
// skip the permanent entirely, so there was no grant to strip and the
// other source survived by accident. Post-change it survives because
// the refcount held it. What this pins is that the migration did not
// introduce a strip that eats a co-held grant.
func TestKeywordFlagRefcountR93_SwashbucklerStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	swash := r89Creature(gs, 0, "Fearless Swashbuckler", "creature", "pirate")
	veh := r89Creature(gs, 0, "R93 Vehicle", "artifact", "vehicle")
	gameengine.GrantKeywordFlag(veh, "haste", r93OtherSource)

	fearlessSwashbucklerHasteSweep(gs, swash)
	fearlessSwashbucklerOwnLTB(gs, swash, map[string]interface{}{"perm": swash})

	r89Assert(t, veh, "haste", 1)
}

// ---------------------------------------------------------------------
// Lyse Hext — the two "clear any stale grant" paths used a bare delete
// of the shared flag, which took away a source that had nothing to do
// with Lyse and was not stale at all.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR93_LyseStaleClearKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	lyse := r89Creature(gs, 0, "Lyse Hext", "creature", "monk")
	gameengine.GrantKeywordFlag(lyse, "double_strike", r93OtherSource)
	// Make the reset path think a turn has rolled over.
	lyse.Flags["lyse_count_turn"] = gs.Turn - 1

	lyseHextResetIfStale(gs, lyse, map[string]interface{}{})

	if !lyse.HasKeyword("double_strike") {
		t.Errorf("Lyse's stale-clear deleted another source's double strike (flags: %+v)", lyse.Flags)
	}
	if n := gameengine.KeywordFlagGrantCount(lyse, "double_strike"); n != 1 {
		t.Errorf("double_strike grant count = %d, want 1 (the other source only) — flags: %+v", n, lyse.Flags)
	}
}

// ---------------------------------------------------------------------
// Archpriest of Shadows — backup target gains deathtouch. Guard was
// !target.HasKeyword("deathtouch") && target.Flags["kw:deathtouch"] == 0.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR93_ArchpriestGrantSurvivesOtherSourceStrip(t *testing.T) {
	gs := newGame(t, 2)
	arch := r89Creature(gs, 0, "Archpriest of Shadows", "creature", "cleric")
	ally := r89Creature(gs, 0, "R93 Archpriest Ally", "creature")
	ally.Card.BasePower = 9
	gameengine.GrantKeywordFlag(ally, "deathtouch", r93OtherSource)
	// The deathtouch grant is in the ETB (backup) handler — the
	// combat-damage trigger next to it only reanimates.
	archpriestOfShadowsETB(gs, arch)

	if !ally.HasKeyword("deathtouch") {
		t.Fatalf("fixture inert: Archpriest's backup did not reach the ally (flags: %+v)", ally.Flags)
	}
	r93SurvivesOtherSourceStrip(t, gs, ally, "deathtouch")
}
