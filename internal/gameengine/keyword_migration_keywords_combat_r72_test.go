package gameengine

import (
	"math/rand"
	"testing"
)

// myriad needs a table with more than one opponent to copy toward.
func newMyriadR72Game(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(4, rand.New(rand.NewSource(42)), nil)
}

// r72 — batch 2 of the keyword call-site migration: keywords_combat.go.
//
// Same contract as r70 (see keyword_migration_combat_r70_test.go, which
// owns the r70Grant / r70Strip helpers reused here): every rules-enforcing
// keyword read in this file moves off the layer-blind p.HasKeyword, and
// each keyword gets a regression proving BOTH directions —
//
//	GRANT  a §613 layer effect confers the keyword -> the rule obeys it
//	STRIP  a layer-6 "loses all abilities" wipe removes it -> obeyed too
//
// Nine helpers gained a *GameState parameter to make this possible
// (the five CanBlock* evasion checks, HasBanding, and the three
// Get<Keyword>N value readers). Every caller already had gs in hand.
//
// NOT migrated, deliberately: HasTrampleOverPlaneswalkers — zero
// production callers; see the comment at its definition.

// r72Stamp puts the layer-blind flag a per_card handler would write, so
// the STRIP direction has something to actually strip. Without it the
// strip case would pass vacuously.
func r72Stamp(gs *GameState, p *Permanent, kw string) {
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags["kw:"+kw] = 1
	gs.InvalidateCharacteristicsCache()
}

// ---------------------------------------------------------------------------
// Evasion — CR §702.13 intimidate, §702.36 fear, §702.28 shadow,
//           §702.120 skulk, daunt
// ---------------------------------------------------------------------------

func TestKeywordMigrationR72_IntimidateGrantAndStrip(t *testing.T) {
	// GRANT: a red attacker that GAINS intimidate cannot be blocked by green.
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Intimidator", 3, 3, "creature")
	atk.Card.Colors = []string{"R"}
	blk := addBattlefield(gs, 1, "Green Bear", 2, 2, "creature")
	blk.Card.Colors = []string{"G"}
	if !CanBlockCombatKeywords(gs, atk, blk) {
		t.Fatal("precondition: green should block a plain red attacker")
	}
	r70Grant(gs, atk, "intimidate", "r72_intim_grant")
	if CanBlockCombatKeywords(gs, atk, blk) {
		t.Error("layer-granted intimidate did not reach the block check")
	}

	// STRIP: a stamped intimidate wiped by layer 6 must stop restricting.
	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Intimidator", 3, 3, "creature")
	atk2.Card.Colors = []string{"R"}
	blk2 := addBattlefield(gs2, 1, "Green Bear", 2, 2, "creature")
	blk2.Card.Colors = []string{"G"}
	r72Stamp(gs2, atk2, "intimidate")
	if CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Fatal("precondition: stamped intimidate should restrict the block")
	}
	r70Strip(gs2, atk2, "r72_intim_strip")
	if !CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Error("intimidate survived a layer-6 ability strip")
	}
}

func TestKeywordMigrationR72_FearGrantAndStrip(t *testing.T) {
	// GRANT: a white blocker cannot block an attacker that gains fear.
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Fearsome", 3, 3, "creature")
	atk.Card.Colors = []string{"B"}
	blk := addBattlefield(gs, 1, "White Bear", 2, 2, "creature")
	blk.Card.Colors = []string{"W"}
	r70Grant(gs, atk, "fear", "r72_fear_grant")
	if CanBlockCombatKeywords(gs, atk, blk) {
		t.Error("layer-granted fear did not reach the block check")
	}

	// STRIP.
	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Fearsome", 3, 3, "creature")
	atk2.Card.Colors = []string{"B"}
	blk2 := addBattlefield(gs2, 1, "White Bear", 2, 2, "creature")
	blk2.Card.Colors = []string{"W"}
	r72Stamp(gs2, atk2, "fear")
	if CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Fatal("precondition: stamped fear should restrict the block")
	}
	r70Strip(gs2, atk2, "r72_fear_strip")
	if !CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Error("fear survived a layer-6 ability strip")
	}
}

func TestKeywordMigrationR72_ShadowGrantAndStrip(t *testing.T) {
	// GRANT: shadow on the attacker locks out a non-shadow blocker.
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Shade", 2, 2, "creature")
	blk := addBattlefield(gs, 1, "Plain Bear", 2, 2, "creature")
	r70Grant(gs, atk, "shadow", "r72_shadow_grant")
	if CanBlockCombatKeywords(gs, atk, blk) {
		t.Error("layer-granted shadow did not reach the block check")
	}

	// STRIP.
	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Shade", 2, 2, "creature")
	blk2 := addBattlefield(gs2, 1, "Plain Bear", 2, 2, "creature")
	r72Stamp(gs2, atk2, "shadow")
	if CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Fatal("precondition: stamped shadow should restrict the block")
	}
	r70Strip(gs2, atk2, "r72_shadow_strip")
	if !CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Error("shadow survived a layer-6 ability strip")
	}
}

func TestKeywordMigrationR72_SkulkGrantAndStrip(t *testing.T) {
	// GRANT: skulk on a 2/2 locks out a 4/4 blocker.
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Skulker", 2, 2, "creature")
	blk := addBattlefield(gs, 1, "Big Blocker", 4, 4, "creature")
	r70Grant(gs, atk, "skulk", "r72_skulk_grant")
	if CanBlockCombatKeywords(gs, atk, blk) {
		t.Error("layer-granted skulk did not reach the block check")
	}

	// STRIP.
	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Skulker", 2, 2, "creature")
	blk2 := addBattlefield(gs2, 1, "Big Blocker", 4, 4, "creature")
	r72Stamp(gs2, atk2, "skulk")
	if CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Fatal("precondition: stamped skulk should restrict the block")
	}
	r70Strip(gs2, atk2, "r72_skulk_strip")
	if !CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Error("skulk survived a layer-6 ability strip")
	}
}

func TestKeywordMigrationR72_DauntGrantAndStrip(t *testing.T) {
	// GRANT: daunt locks out a power-2 blocker.
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Daunter", 4, 4, "creature")
	blk := addBattlefield(gs, 1, "Small Blocker", 2, 2, "creature")
	r70Grant(gs, atk, "daunt", "r72_daunt_grant")
	if CanBlockCombatKeywords(gs, atk, blk) {
		t.Error("layer-granted daunt did not reach the block check")
	}

	// STRIP.
	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Daunter", 4, 4, "creature")
	blk2 := addBattlefield(gs2, 1, "Small Blocker", 2, 2, "creature")
	r72Stamp(gs2, atk2, "daunt")
	if CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Fatal("precondition: stamped daunt should restrict the block")
	}
	r70Strip(gs2, atk2, "r72_daunt_strip")
	if !CanBlockCombatKeywords(gs2, atk2, blk2) {
		t.Error("daunt survived a layer-6 ability strip")
	}
}

// ---------------------------------------------------------------------------
// r72 second defect, found while threading gs: skulk and daunt compared
// p.Power(), the LAYER-BLIND power. Both are rules enforcement — an
// illegal block is a corrupt game state, not a bad AI score — so unlike
// combat.go's incomingIsLethal/scoreBlock they move to gs.PowerOf.
// ---------------------------------------------------------------------------

func TestKeywordMigrationR72_SkulkUsesEffectivePower(t *testing.T) {
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Skulker", 3, 3, "creature")
	r72Stamp(gs, atk, "skulk")
	blk := addBattlefield(gs, 1, "Pumped Blocker", 2, 2, "creature")
	if !CanBlockCombatKeywords(gs, atk, blk) {
		t.Fatal("precondition: a 2/2 should block a 3/3 skulker")
	}
	// A §613 layer-7c anthem lifts the blocker to 4/4.
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerPT, Sublayer: "c", HandlerID: "r72_skulk_anthem",
		Duration:  DurationPermanent,
		Predicate: func(_ *GameState, p *Permanent) bool { return p == blk },
		ApplyFn: func(_ *GameState, _ *Permanent, c *Characteristics) {
			c.Power += 2
			c.Toughness += 2
		},
	})
	gs.InvalidateCharacteristicsCache()
	if CanBlockCombatKeywords(gs, atk, blk) {
		t.Error("skulk compared printed power — a layer-pumped 4/4 blocked a 3/3 skulker")
	}
}

func TestKeywordMigrationR72_DauntUsesEffectivePower(t *testing.T) {
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Daunter", 4, 4, "creature")
	r72Stamp(gs, atk, "daunt")
	blk := addBattlefield(gs, 1, "Pumped Blocker", 1, 1, "creature")
	if CanBlockCombatKeywords(gs, atk, blk) {
		t.Fatal("precondition: a 1/1 cannot block a daunter")
	}
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerPT, Sublayer: "c", HandlerID: "r72_daunt_anthem",
		Duration:  DurationPermanent,
		Predicate: func(_ *GameState, p *Permanent) bool { return p == blk },
		ApplyFn: func(_ *GameState, _ *Permanent, c *Characteristics) {
			c.Power += 3
			c.Toughness += 3
		},
	})
	gs.InvalidateCharacteristicsCache()
	if !CanBlockCombatKeywords(gs, atk, blk) {
		t.Error("daunt compared printed power — a layer-pumped 4/4 could not block")
	}
}

// ---------------------------------------------------------------------------
// Banding — CR §702.22
// ---------------------------------------------------------------------------

func TestKeywordMigrationR72_BandingGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Bander", 2, 2, "creature")
	r70Grant(gs, p, "banding", "r72_band_grant")
	if !HasBanding(gs, p) {
		t.Error("layer-granted banding not seen by HasBanding")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Bander", 2, 2, "creature")
	r72Stamp(gs2, q, "banding")
	if !HasBanding(gs2, q) {
		t.Fatal("precondition: stamped banding should read true")
	}
	r70Strip(gs2, q, "r72_band_strip")
	if HasBanding(gs2, q) {
		t.Error("banding survived a layer-6 ability strip")
	}
}

// ---------------------------------------------------------------------------
// The N-valued keywords — rampage §702.23, annihilator §702.86,
// afflict §702.130. The Flags["<kw>_n"] short-circuit is deliberately
// NOT set here: it would mask the keyword read being tested.
// ---------------------------------------------------------------------------

func TestKeywordMigrationR72_RampageGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Rampager", 2, 2, "creature")
	r70Grant(gs, p, "rampage", "r72_ramp_grant")
	if GetRampageN(gs, p) <= 0 {
		t.Error("layer-granted rampage not seen by GetRampageN")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Rampager", 2, 2, "creature")
	r72Stamp(gs2, q, "rampage")
	if GetRampageN(gs2, q) <= 0 {
		t.Fatal("precondition: stamped rampage should read > 0")
	}
	r70Strip(gs2, q, "r72_ramp_strip")
	if n := GetRampageN(gs2, q); n != 0 {
		t.Errorf("rampage survived a layer-6 ability strip: N=%d", n)
	}
}

func TestKeywordMigrationR72_AnnihilatorGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Annihilator", 2, 2, "creature")
	r70Grant(gs, p, "annihilator", "r72_ann_grant")
	if GetAnnihilatorN(gs, p) <= 0 {
		t.Error("layer-granted annihilator not seen by GetAnnihilatorN")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Annihilator", 2, 2, "creature")
	r72Stamp(gs2, q, "annihilator")
	if GetAnnihilatorN(gs2, q) <= 0 {
		t.Fatal("precondition: stamped annihilator should read > 0")
	}
	r70Strip(gs2, q, "r72_ann_strip")
	if n := GetAnnihilatorN(gs2, q); n != 0 {
		t.Errorf("annihilator survived a layer-6 ability strip: N=%d", n)
	}
}

func TestKeywordMigrationR72_AfflictGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Afflicter", 2, 2, "creature")
	r70Grant(gs, p, "afflict", "r72_aff_grant")
	if GetAfflictN(gs, p) <= 0 {
		t.Error("layer-granted afflict not seen by GetAfflictN")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Afflicter", 2, 2, "creature")
	r72Stamp(gs2, q, "afflict")
	if GetAfflictN(gs2, q) <= 0 {
		t.Fatal("precondition: stamped afflict should read > 0")
	}
	r70Strip(gs2, q, "r72_aff_strip")
	if n := GetAfflictN(gs2, q); n != 0 {
		t.Errorf("afflict survived a layer-6 ability strip: N=%d", n)
	}
}

// ---------------------------------------------------------------------------
// Attack triggers — battle cry §702.91, melee §702.121, myriad §702.116
// ---------------------------------------------------------------------------

func TestKeywordMigrationR72_BattleCryGrantAndStrip(t *testing.T) {
	// GRANT: the OTHER attacker must pick up +1/+0.
	gs := newFixtureGame(t)
	crier := addBattlefield(gs, 0, "Granted Crier", 2, 2, "creature")
	other := addBattlefield(gs, 0, "Fellow Attacker", 2, 2, "creature")
	r70Grant(gs, crier, "battle cry", "r72_bc_grant")
	ApplyBattleCry(gs, 0, []*Permanent{crier, other})
	if len(other.Modifications) == 0 {
		t.Error("layer-granted battle cry did not pump the other attacker")
	}

	// STRIP.
	gs2 := newFixtureGame(t)
	crier2 := addBattlefield(gs2, 0, "Stamped Crier", 2, 2, "creature")
	other2 := addBattlefield(gs2, 0, "Fellow Attacker", 2, 2, "creature")
	r72Stamp(gs2, crier2, "battle cry")
	r70Strip(gs2, crier2, "r72_bc_strip")
	ApplyBattleCry(gs2, 0, []*Permanent{crier2, other2})
	if len(other2.Modifications) != 0 {
		t.Error("battle cry survived a layer-6 ability strip — other attacker was pumped")
	}
}

func TestKeywordMigrationR72_MeleeGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Meleer", 2, 2, "creature")
	SetAttackerDefender(atk, 1)
	r70Grant(gs, atk, "melee", "r72_melee_grant")
	ApplyMelee(gs, 0, []*Permanent{atk})
	if len(atk.Modifications) == 0 {
		t.Error("layer-granted melee did not pump the attacker")
	}

	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Meleer", 2, 2, "creature")
	SetAttackerDefender(atk2, 1)
	r72Stamp(gs2, atk2, "melee")
	r70Strip(gs2, atk2, "r72_melee_strip")
	ApplyMelee(gs2, 0, []*Permanent{atk2})
	if len(atk2.Modifications) != 0 {
		t.Error("melee survived a layer-6 ability strip — attacker was pumped")
	}
}

func TestKeywordMigrationR72_MyriadGrantAndStrip(t *testing.T) {
	// GRANT: myriad must mint a token copy per other opponent.
	gs := newMyriadR72Game(t)
	atk := addBattlefield(gs, 0, "Granted Myriader", 2, 2, "creature")
	SetAttackerDefender(atk, 1)
	before := len(gs.Seats[0].Battlefield)
	r70Grant(gs, atk, "myriad", "r72_myr_grant")
	FireMyriadTriggers(gs, 0, []*Permanent{atk})
	grantedDelta := len(gs.Seats[0].Battlefield) - before

	// STRIP: the same board with a stamped myriad wiped must mint nothing.
	gs2 := newMyriadR72Game(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Myriader", 2, 2, "creature")
	SetAttackerDefender(atk2, 1)
	r72Stamp(gs2, atk2, "myriad")
	before2 := len(gs2.Seats[0].Battlefield)
	r70Strip(gs2, atk2, "r72_myr_strip")
	FireMyriadTriggers(gs2, 0, []*Permanent{atk2})
	if d := len(gs2.Seats[0].Battlefield) - before2; d != 0 {
		t.Errorf("myriad survived a layer-6 ability strip: %d tokens created", d)
	}
	if grantedDelta <= 0 {
		t.Errorf("layer-granted myriad minted no token copies (delta=%d)", grantedDelta)
	}
}

// ---------------------------------------------------------------------------
// Provoke — CR §702.39
// ---------------------------------------------------------------------------

func TestKeywordMigrationR72_ProvokeGrantAndStrip(t *testing.T) {
	// GRANT: provoke untaps the defender's creature and forces the block.
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Provoker", 3, 3, "creature")
	SetAttackerDefender(atk, 1)
	victim := addBattlefield(gs, 1, "Tapped Defender", 2, 2, "creature")
	victim.Tapped = true
	r70Grant(gs, atk, "provoke", "r72_prov_grant")
	FireProvokeTriggers(gs, []*Permanent{atk})
	if victim.Tapped {
		t.Error("layer-granted provoke did not untap the provoked creature")
	}

	// STRIP.
	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Provoker", 3, 3, "creature")
	SetAttackerDefender(atk2, 1)
	victim2 := addBattlefield(gs2, 1, "Tapped Defender", 2, 2, "creature")
	victim2.Tapped = true
	r72Stamp(gs2, atk2, "provoke")
	r70Strip(gs2, atk2, "r72_prov_strip")
	FireProvokeTriggers(gs2, []*Permanent{atk2})
	if !victim2.Tapped {
		t.Error("provoke survived a layer-6 ability strip — defender was untapped")
	}
}
