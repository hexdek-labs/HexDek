package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// keyword_flag_refcount_r91_test.go — batch 2 of the migration behind
// the r87 refcount primitive (filed `6f332cbc`): the four handlers that
// still carried a provenance tag INSIDE the "kw:" namespace.
//
// That namespace is the one runtimeKeywordSources folds VERBATIM into
// chars.Keywords, so each of these four was also fabricating a keyword
// — "menace_from_gisa", "haste_from_kolodin", "trample_from_old_one_eye",
// "lifelink_from_heliod_ueot". Those were the last four in the tree;
// migrating them retires the convention from production code entirely.
//
// Helpers r89Creature / r89Assert / r89OtherSource come from
// keyword_flag_refcount_r89_test.go.

// r91NoPhantomKeyword asserts the handler's provenance no longer
// answers as a keyword through the layer fold. Pre-migration each of
// these returned true.
func r91NoPhantomKeyword(t *testing.T, gs *gameengine.GameState, p *gameengine.Permanent, phantom string) {
	t.Helper()
	if gs.HasKeywordOf(p, phantom) {
		t.Errorf("provenance tag %q is answering as a keyword through the layer fold (flags: %+v)", phantom, p.Flags)
	}
	if _, ok := p.Flags["kw:"+phantom]; ok {
		t.Errorf("provenance tag is back inside the kw: namespace, which the fold takes verbatim (flags: %+v)", p.Flags)
	}
}

// ---------------------------------------------------------------------
// Gisa — "Skeletons and Zombies you control get +1/+1 and have menace".
// Her refresh strips and re-stamps on every permanent_etb, so she holds
// and releases the shared flag constantly.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR91_GisaStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	gisa := r89Creature(gs, 0, "Gisa, the Hellraiser", "creature", "zombie")
	zombie := r89Creature(gs, 0, "R91 Zombie", "creature", "zombie")
	gameengine.GrantKeywordFlag(zombie, "menace", r89OtherSource)

	gisaRefreshAnthemOnETB(gs, gisa)

	if !zombie.HasKeyword("menace") {
		t.Fatalf("fixture inert: Gisa's anthem did not reach the zombie (flags: %+v)", zombie.Flags)
	}
	r89Assert(t, zombie, "menace", 2)

	// Gisa's real own-LTB entry point.
	gisaOwnLTB(gs, gisa, map[string]interface{}{"perm": gisa})

	r89Assert(t, zombie, "menace", 1)
	r91NoPhantomKeyword(t, gs, zombie, "menace_from_gisa")
}

func TestKeywordFlagRefcountR91_GisaGrantIsNotAPhantomKeyword(t *testing.T) {
	gs := newGame(t, 2)
	gisa := r89Creature(gs, 0, "Gisa, the Hellraiser", "creature", "zombie")
	zombie := r89Creature(gs, 0, "R91 Zombie", "creature", "zombie")

	gisaRefreshAnthemOnETB(gs, gisa)

	if !gs.HasKeywordOf(zombie, "menace") {
		t.Fatalf("fixture inert: menace did not reach the layer-aware authority (flags: %+v)", zombie.Flags)
	}
	r91NoPhantomKeyword(t, gs, zombie, "menace_from_gisa")
}

// ---------------------------------------------------------------------
// Kolodin — "Mounts and Vehicles you control have haste". Haste is the
// most-shared granted keyword in the tree, so a second grantor is this
// handler's ordinary case.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR91_KolodinStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	kolodin := r89Creature(gs, 0, "Kolodin, Triumph Caster", "creature")
	mount := r89Creature(gs, 0, "R91 Mount", "creature", "mount")
	gameengine.GrantKeywordFlag(mount, "haste", r89OtherSource)

	kolodinRefreshAnthemOnETB(gs, kolodin)

	if !mount.HasKeyword("haste") {
		t.Fatalf("fixture inert: Kolodin's anthem did not reach the mount (flags: %+v)", mount.Flags)
	}
	r89Assert(t, mount, "haste", 2)

	kolodinOwnLTB(gs, kolodin, map[string]interface{}{"perm": kolodin})

	r89Assert(t, mount, "haste", 1)
	r91NoPhantomKeyword(t, gs, mount, "haste_from_kolodin")
}

// ---------------------------------------------------------------------
// Old One Eye — "Other creatures you control have trample".
//
// This is the batch's real find. Its stamp was gated on
//
//	if p.Flags["kw:trample"] != 1 { ...stamp... }
//
// which is precisely the guard r87's doc names as self-defeating: a
// creature that ALREADY shows trample is skipped, so Old One Eye never
// records its own grant, and when the source it deferred to leaves, that
// source deletes the shared flag and takes trample away with Old One Eye
// still on the battlefield.
//
// The ORDER here is what makes the case: the other grantor arrives
// FIRST. Post-migration the grant is unconditional and idempotent.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR91_OldOneEyeRecordsOverAnExistingGrant(t *testing.T) {
	gs := newGame(t, 2)
	one := r89Creature(gs, 0, "Old One Eye", "creature", "legendary")
	bear := r89Creature(gs, 0, "R91 Bear", "creature")

	// The other source gets there first — the case the old guard
	// structurally could not handle.
	gameengine.GrantKeywordFlag(bear, "trample", r89OtherSource)

	oldOneEyeApplyAnthemOnETB(gs, one)

	if !gameengine.HasKeywordFlagGrant(bear, "trample", oldOneEyeTrampleMarker) {
		t.Fatalf("Old One Eye declined to record its own grant because another source had already set the shared flag (flags: %+v)", bear.Flags)
	}
	r89Assert(t, bear, "trample", 2)

	// The other source leaves. Old One Eye is still out, so trample stays.
	gameengine.RevokeKeywordFlag(bear, "trample", r89OtherSource)

	if !bear.HasKeyword("trample") {
		t.Errorf("trample was removed with Old One Eye still on the battlefield (flags: %+v)", bear.Flags)
	}
	if !gs.HasKeywordOf(bear, "trample") {
		t.Errorf("layer-aware authority disagrees: trample gone with Old One Eye still out (flags: %+v)", bear.Flags)
	}
	r89Assert(t, bear, "trample", 1)
}

func TestKeywordFlagRefcountR91_OldOneEyeStripKeepsOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	one := r89Creature(gs, 0, "Old One Eye", "creature", "legendary")
	bear := r89Creature(gs, 0, "R91 Bear", "creature")

	oldOneEyeApplyAnthemOnETB(gs, one)
	// A grantor arriving AFTER the anthem — invisible to any pre-state
	// snapshot, which is the shape Dihada's r89 case pinned.
	gameengine.GrantKeywordFlag(bear, "trample", r89OtherSource)
	r89Assert(t, bear, "trample", 2)

	oldOneEyeOwnLTB(gs, one, map[string]interface{}{"perm": one})

	r89Assert(t, bear, "trample", 1)
	r91NoPhantomKeyword(t, gs, bear, "trample_from_old_one_eye")
}

// ---------------------------------------------------------------------
// Heliod — "{1}{W}: Another target creature gains lifelink until end of
// turn." Two defects here, and the second is not a collision at all.
// ---------------------------------------------------------------------

func TestKeywordFlagRefcountR91_HeliodExpiryKeepsLaterGrantor(t *testing.T) {
	gs := newGame(t, 2)
	heliod := r89Creature(gs, 0, "Heliod, Sun-Crowned", "creature", "legendary")
	ally := r89Creature(gs, 0, "R91 Ally", "creature")
	gs.Seats[0].ManaPool = 5

	heliodSunCrownedActivate(gs, heliod, 0, nil)

	if !ally.HasKeyword("lifelink") {
		t.Fatalf("fixture inert: Heliod's grant did not reach the ally (flags: %+v)", ally.Flags)
	}
	// Heliod's own target preference skips anything already showing
	// lifelink, so the second grantor necessarily arrives afterwards.
	gameengine.GrantKeywordFlag(ally, "lifelink", r89OtherSource)
	r89Assert(t, ally, "lifelink", 2)

	gameengine.FireCleanupStepExpiries(gs)

	r89Assert(t, ally, "lifelink", 1)
	r91NoPhantomKeyword(t, gs, ally, "lifelink_from_heliod_ueot")
}

// TestKeywordFlagRefcountR91_HeliodGrantInvalidatesTheCache pins the
// other defect, which has nothing to do with refcounting.
//
// The old grant path set Flags["kw:lifelink"] and never invalidated the
// characteristics cache, although its own cleanup trigger did. Any
// cached entry taken before the activation therefore kept answering "no
// lifelink" for a creature that had just been granted it. The test
// primes the cache with a layer-aware read first, which is what makes
// the gap observable — without that read the next query computes fresh
// and the bug hides.
func TestKeywordFlagRefcountR91_HeliodGrantInvalidatesTheCache(t *testing.T) {
	gs := newGame(t, 2)
	heliod := r89Creature(gs, 0, "Heliod, Sun-Crowned", "creature", "legendary")
	ally := r89Creature(gs, 0, "R91 Ally", "creature")
	gs.Seats[0].ManaPool = 5

	// Prime the characteristics cache for this permanent.
	if gs.HasKeywordOf(ally, "lifelink") {
		t.Fatalf("fixture wrong: the ally already has lifelink before the grant")
	}

	heliodSunCrownedActivate(gs, heliod, 0, nil)

	if !ally.HasKeyword("lifelink") {
		t.Fatalf("fixture inert: Heliod's grant did not reach the ally (flags: %+v)", ally.Flags)
	}
	if !gs.HasKeywordOf(ally, "lifelink") {
		t.Errorf("the layer-aware authority is serving a stale cache entry: the grant landed on the flag surface but nothing invalidated (flags: %+v)", ally.Flags)
	}
}
