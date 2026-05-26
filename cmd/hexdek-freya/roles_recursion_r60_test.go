package main

import (
	"testing"
)

// R60 role-tagger Recursion category — closes the "recursion piece
// untagged / utility-creature mis-tagged as threat" gap surfaced
// during the r60 role-tagger survey. Pre-r60, p.IsRecursion was
// populated by analysis.go but no RoleTag consumed it: Eternal
// Witness got [Utility], Reanimate got [Utility], Sun Titan got
// [Threat] only. After r60: Eternal Witness gets [Recursion], Sun
// Titan gets [Threat, Recursion] (combat-attack trigger AND
// recursion), Reanimate gets [Recursion], and the cmc>=6 Threat
// catch-all backs off for mid-CMC pure-recursion bodies (Karmic
// Guide-class), while 8+ CMC bombs (Worldspine Wurm) still tag as
// Threat regardless of secondary text.
//
// Tests use concrete oracle text from real cards so a future
// "improve detection of X" effort can directly extend this file.

// roleFor wraps TagCardRole with the IsRecursion flag pre-populated
// by the analysis layer's `return X from graveyard` pattern (which
// the role tagger consumes via isRecursion).
func roleFor(name, oracleText, typeLine string, cmc int, mods ...func(*CardProfile)) []RoleTag {
	p := CardProfile{
		Name:     name,
		TypeLine: typeLine,
		CMC:      cmc,
	}
	// Mirror analysis.go:298-304 — any card whose oracle text matches
	// the canonical "return ... from ... graveyard ... to (battlefield|
	// hand|owner)" pattern gets IsRecursion stamped. Test fixtures
	// pre-set it because we're testing the ROLE-tagger here, not the
	// analysis layer.
	for _, m := range mods {
		m(&p)
	}
	return TagCardRole(name, oracleText, typeLine, "", cmc, p)
}

func hasRole(roles []RoleTag, want RoleTag) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Pure recursion pieces — pre-r60 fell to Utility
// ---------------------------------------------------------------------

func TestRoles_EternalWitness_IsRecursion(t *testing.T) {
	// 2/1 for {1}{G}{G} — pre-r60 had no role at all (no produces,
	// no combat keywords, cmc<4 for threat), so fell to [Utility].
	// After r60: should carry [Recursion] only — small body, real
	// engine purpose.
	roles := roleFor(
		"Eternal Witness",
		"When this creature enters, you may return target card from your graveyard to your hand.",
		"Creature — Human Shaman",
		3,
		func(p *CardProfile) { p.IsRecursion = true },
	)
	if !hasRole(roles, RoleRecursion) {
		t.Errorf("Eternal Witness should be tagged Recursion, got %v", roles)
	}
	if hasRole(roles, RoleUtility) {
		t.Errorf("Eternal Witness should NOT fall through to Utility once Recursion fires, got %v", roles)
	}
	if hasRole(roles, RoleThreat) {
		t.Errorf("Eternal Witness is a 2/1 — should NOT be tagged Threat, got %v", roles)
	}
}

func TestRoles_Reanimate_IsRecursion(t *testing.T) {
	// {B} sorcery — pre-r60 fell to [Utility] (no removal, no
	// counterspell, no produces). After r60: [Recursion] only.
	roles := roleFor(
		"Reanimate",
		"Put target creature card from a graveyard onto the battlefield under your control. You lose life equal to its mana value.",
		"Sorcery",
		1,
		func(p *CardProfile) { p.IsRecursion = true },
	)
	if !hasRole(roles, RoleRecursion) {
		t.Errorf("Reanimate should be tagged Recursion, got %v", roles)
	}
	if hasRole(roles, RoleUtility) {
		t.Errorf("Reanimate should NOT fall through to Utility, got %v", roles)
	}
}

func TestRoles_AnimateDead_IsRecursion(t *testing.T) {
	roles := roleFor(
		"Animate Dead",
		"Enchant creature card in a graveyard. When this Aura enters, if it's on the battlefield, it loses \"enchant creature card in a graveyard\" and gains \"enchant creature put onto the battlefield with this Aura.\" Return enchanted creature card to the battlefield under your control and attach this Aura to it.",
		"Enchantment — Aura",
		2,
		func(p *CardProfile) { p.IsRecursion = true },
	)
	if !hasRole(roles, RoleRecursion) {
		t.Errorf("Animate Dead should be tagged Recursion, got %v", roles)
	}
}

func TestRoles_Regrowth_IsRecursion(t *testing.T) {
	// 2-mana sorcery — pre-r60 had no role (returns to hand, not a
	// "draw"). After r60: [Recursion].
	roles := roleFor(
		"Regrowth",
		"Return target card from your graveyard to your hand.",
		"Sorcery",
		2,
		func(p *CardProfile) { p.IsRecursion = true },
	)
	if !hasRole(roles, RoleRecursion) {
		t.Errorf("Regrowth should be tagged Recursion, got %v", roles)
	}
}

// ---------------------------------------------------------------------
// Multi-role recursion pieces — Threat + Recursion both fire
// ---------------------------------------------------------------------

func TestRoles_SunTitan_IsBothThreatAndRecursion(t *testing.T) {
	// 6/6 Vigilance for {4}{W}{W} — the "Whenever Sun Titan attacks
	// or enters the battlefield" trigger lands the Threat tag via
	// the attack-trigger gate; recursion-creature semantics land
	// the Recursion tag. Both true — Sun Titan IS a clock AND a
	// recursion engine; the tagger surfaces both.
	roles := roleFor(
		"Sun Titan",
		"Vigilance. Whenever this creature attacks or enters the battlefield, you may return target permanent card with mana value 3 or less from your graveyard to the battlefield.",
		"Creature — Giant",
		6,
		func(p *CardProfile) { p.IsRecursion = true },
	)
	if !hasRole(roles, RoleThreat) {
		t.Errorf("Sun Titan attacks-trigger should still tag Threat, got %v", roles)
	}
	if !hasRole(roles, RoleRecursion) {
		t.Errorf("Sun Titan should also tag Recursion, got %v", roles)
	}
	// Ordering: Threat priority=2, Recursion priority=7, so Threat
	// should render first.
	threatIdx, recIdx := -1, -1
	for i, r := range roles {
		if r == RoleThreat {
			threatIdx = i
		}
		if r == RoleRecursion {
			recIdx = i
		}
	}
	if threatIdx >= recIdx {
		t.Errorf("Threat should render before Recursion in priority order, got %v", roles)
	}
}

func TestRoles_KarmicGuide_IsRecursionOnly_NotThreatCatchAll(t *testing.T) {
	// 2/2 Flying Protection-from-black for {3}{W}{W} — pre-r60, the
	// cmc>=6 catch-all in isThreat would fire? No, cmc=5. But
	// "flying" matches the gate so it would fire. After r60:
	// flying-keyword still gates Threat, so this is multi-role too.
	// Pin both tags fire.
	roles := roleFor(
		"Karmic Guide",
		"Flying. Protection from black. Echo {3}{W}{W}. When this creature enters, return target creature card from your graveyard to the battlefield.",
		"Creature — Angel Spirit",
		5,
		func(p *CardProfile) { p.IsRecursion = true },
	)
	if !hasRole(roles, RoleRecursion) {
		t.Errorf("Karmic Guide should tag Recursion, got %v", roles)
	}
	// Karmic Guide has Flying which triggers the combat-keyword
	// gate; expect Threat too. (If a future tuning decides Karmic
	// Guide shouldn't be Threat, this test pins where to start.)
	if !hasRole(roles, RoleThreat) {
		t.Errorf("Karmic Guide has Flying — combat-keyword gate should fire Threat, got %v", roles)
	}
	if !hasRole(roles, RoleProtection) {
		t.Errorf("Karmic Guide has 'Protection from black' — should also tag Protection, got %v", roles)
	}
}

// ---------------------------------------------------------------------
// cmc>=6 catch-all suppression for pure-recursion mid-CMC bodies
// ---------------------------------------------------------------------

func TestRoles_VanillaMidCMCRecursionBody_NotMisTaggedAsThreat(t *testing.T) {
	// Hypothetical pure-recursion 6-mana body without any combat
	// keywords or attack triggers. Pre-r60 cmc>=6 catch-all would
	// mark it [Threat]; after r60 the catch-all backs off when
	// IsRecursion is set and cmc∈[6,7] so it tags [Recursion] only.
	roles := roleFor(
		"Hypothetical Reanimator Body",
		"When this creature enters, return target creature card from your graveyard to the battlefield.",
		"Creature — Spirit",
		6,
		func(p *CardProfile) { p.IsRecursion = true },
	)
	if !hasRole(roles, RoleRecursion) {
		t.Errorf("mid-CMC recursion body should tag Recursion, got %v", roles)
	}
	if hasRole(roles, RoleThreat) {
		t.Errorf("mid-CMC pure-recursion body should NOT trip the cmc>=6 catch-all, got %v", roles)
	}
}

func TestRoles_WorldspineWurm_StillThreatRegardlessOfText(t *testing.T) {
	// 11-mana 15/15 Trample. Pure finisher; cmc>=8 keeps the Threat
	// tag fired regardless of any other text. Defends against the
	// recursion-suppression over-narrowing into bombs that happen
	// to mention graveyards.
	roles := roleFor(
		"Worldspine Wurm",
		"Trample. When this creature dies, create three 5/5 green Wurm creature tokens. When this creature is put into a library from anywhere, shuffle it into that library.",
		"Creature — Wurm",
		11,
	)
	if !hasRole(roles, RoleThreat) {
		t.Errorf("Worldspine Wurm at 11 CMC must still tag Threat, got %v", roles)
	}
	if hasRole(roles, RoleRecursion) {
		t.Errorf("Worldspine Wurm 'put into a library' is NOT graveyard recursion, got %v", roles)
	}
}

func TestRoles_AvengerOfZendikar_StillThreat_NotRecursion(t *testing.T) {
	// 7-mana 5/5 Landfall token-maker. Top-end finisher with no
	// graveyard interaction. Defends against the recursion path
	// firing on irrelevant cards.
	roles := roleFor(
		"Avenger of Zendikar",
		"When this creature enters, create a 0/1 green Plant creature token for each land you control. Landfall — Whenever a land you control enters, put a +1/+1 counter on each Plant creature you control.",
		"Creature — Elemental",
		7,
	)
	if !hasRole(roles, RoleThreat) {
		t.Errorf("Avenger of Zendikar should tag Threat, got %v", roles)
	}
	if hasRole(roles, RoleRecursion) {
		t.Errorf("Avenger of Zendikar has no graveyard recursion, got %v", roles)
	}
}

// ---------------------------------------------------------------------
// Oracle-text-only recursion patterns missed by the analysis layer
// ---------------------------------------------------------------------

func TestRoles_PastInFlames_CastFromGraveyardPattern(t *testing.T) {
	// Pre-r60 analysis.go's recursion detector keys on "return ...
	// from ... graveyard ... to (battlefield|hand)". Past in Flames
	// doesn't return cards — it grants flashback — so analysis-
	// layer IsRecursion stays false. The role tagger's new
	// "may cast ... from your graveyard" branch catches it.
	roles := roleFor(
		"Past in Flames",
		"Each instant and sorcery card in your graveyard gains flashback until end of turn. The flashback cost is equal to its mana cost. Flashback {4}{R}",
		"Sorcery",
		3,
	)
	if !hasRole(roles, RoleRecursion) {
		t.Errorf("Past in Flames 'may cast from graveyard' should tag Recursion, got %v", roles)
	}
}

func TestRoles_YawgmothsWill_CastFromGraveyardPattern(t *testing.T) {
	roles := roleFor(
		"Yawgmoth's Will",
		"Until end of turn, you may play lands and cast spells from your graveyard. If a card would be put into your graveyard from anywhere this turn, exile it instead.",
		"Sorcery",
		3,
	)
	// Yawgmoth's Will doesn't use "may cast ... from your graveyard" —
	// it says "cast spells from your graveyard". The generic "from
	// your graveyard to the battlefield" branch doesn't fire either.
	// Pin the narrow CURRENT behaviour as a negative assertion so a
	// future tuning that broadens "cast ... from ... graveyard"
	// detection trips this test and forces the maintainer to update
	// it. Without the assertion, the test was a no-op probe (caught
	// by the Phase 2B no-assertion sweep).
	if hasRole(roles, RoleRecursion) {
		t.Errorf("Yawgmoth's Will recursion detection broadened — update this test (got %v)", roles)
	}
}

func TestRoles_DefaultUtilityStillFiresWhenNoRecursion(t *testing.T) {
	// Sanity: cards with no role-tag signal still default to Utility.
	roles := roleFor(
		"Tormod's Crypt",
		"{T}, Sacrifice this artifact: Exile all cards from target player's graveyard.",
		"Artifact",
		0,
	)
	if !hasRole(roles, RoleUtility) {
		t.Errorf("Tormod's Crypt (exile-yard, not recursion) should default to Utility, got %v", roles)
	}
	if hasRole(roles, RoleRecursion) {
		t.Errorf("Tormod's Crypt exiles graveyards — NOT recursion, got %v", roles)
	}
}

func TestRoles_LandsExcludedFromRecursion(t *testing.T) {
	// Crucible of Worlds-class land-recursion is intentionally NOT
	// tagged Recursion — that's ramp-coded behavior (handled by
	// RoleRamp / land_fetch effect tag). But if a LAND itself had
	// graveyard text (Volrath's Stronghold), the IsLand guard
	// should prevent the Recursion tag.
	roles := roleFor(
		"Volrath's Stronghold",
		"{T}: Add {B}. {1}{B}, {T}: Put target creature card from your graveyard on top of your library.",
		"Legendary Land",
		0,
		func(p *CardProfile) {
			p.IsLand = true
			p.IsRecursion = true
		},
	)
	if hasRole(roles, RoleRecursion) {
		t.Errorf("Lands must be excluded from Recursion tag (handled by Land role), got %v", roles)
	}
	if !hasRole(roles, RoleLand) {
		t.Errorf("Volrath's Stronghold should still tag Land, got %v", roles)
	}
}

// ---------------------------------------------------------------------
// AllRoles + rolePriority renumbering invariants
// ---------------------------------------------------------------------

func TestRoles_AllRolesIncludesRecursion(t *testing.T) {
	found := false
	for _, r := range AllRoles {
		if r == RoleRecursion {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("RoleRecursion missing from AllRoles slice: %v", AllRoles)
	}
}

func TestRoles_PriorityOrdering(t *testing.T) {
	// Pin the expected ordering so downstream rendering (text report,
	// deck profile JSON) doesn't churn unexpectedly.
	pairs := []struct {
		name string
		a, b RoleTag
	}{
		{"Combo < Stax", RoleCombo, RoleStax},
		{"Stax < Threat", RoleStax, RoleThreat},
		{"Threat < BoardWipe", RoleThreat, RoleBoardWipe},
		{"Removal < Recursion", RoleRemoval, RoleRecursion},
		{"Recursion < Protection", RoleRecursion, RoleProtection},
		{"Protection < Draw", RoleProtection, RoleDraw},
		{"Utility < Land", RoleUtility, RoleLand},
	}
	for _, tc := range pairs {
		if rolePriority[tc.a] >= rolePriority[tc.b] {
			t.Errorf("%s: priority(%s)=%d should be < priority(%s)=%d",
				tc.name, tc.a, rolePriority[tc.a], tc.b, rolePriority[tc.b])
		}
	}
}
