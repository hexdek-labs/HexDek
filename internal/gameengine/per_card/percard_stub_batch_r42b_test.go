package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R42b stub-batch ports — five gen_*.go pure-stub handlers from the
// alphabetical second half (n-z), distinct from earlier R36/R37/R41
// sets and from dev-5's parallel R42 batch (Rosnakht and Second
// Doctor were claimed by both and resolved in favor of dev-5's
// versions when r42 landed in main first — see r42b commit message).
//
// Picks (final):
//   - Uril, the Miststalker             Aura-count P/T buff via Modifications
//   - Noctis, Prince of Lucis           graveyard zone-cast grant +3 life
//   - Shilgengar, Sire of Famine        activated sac-for-Blood + mass reanim
//   - Wraith, Vicious Vigilante         unblockable static flag
//   - Zaffai and the Tempests           once-per-turn free I/S cast tracker

// ---------------------------------------------------------------------------
// Uril, the Miststalker — +2/+2 per Aura attached
// ---------------------------------------------------------------------------

func TestUril_NoBuffWithoutAuras(t *testing.T) {
	gs := newGame(t, 2)
	uril := stampCreaturePT(addPerm(gs, 0, "Uril, the Miststalker", "creature", "legendary"), 5, 5)

	urilTheMiststalkerETB(gs, uril)

	if got := uril.Power(); got != 5 {
		t.Errorf("Uril power without auras = %d, want 5 (base)", got)
	}
	if got := uril.Toughness(); got != 5 {
		t.Errorf("Uril toughness without auras = %d, want 5 (base)", got)
	}
}

func TestUril_OneAuraGrants2_2(t *testing.T) {
	gs := newGame(t, 2)
	uril := stampCreaturePT(addPerm(gs, 0, "Uril, the Miststalker", "creature", "legendary"), 5, 5)
	aura := addPerm(gs, 0, "Holy Strength", "enchantment", "aura")
	aura.AttachedTo = uril

	urilTheMiststalkerETB(gs, uril)

	if got := uril.Power(); got != 7 {
		t.Errorf("Uril power with 1 aura = %d, want 7 (5+2)", got)
	}
	if got := uril.Toughness(); got != 7 {
		t.Errorf("Uril toughness with 1 aura = %d, want 7 (5+2)", got)
	}
}

func TestUril_ThreeAurasGrants6_6(t *testing.T) {
	gs := newGame(t, 2)
	uril := stampCreaturePT(addPerm(gs, 0, "Uril, the Miststalker", "creature", "legendary"), 5, 5)
	for i := 0; i < 3; i++ {
		a := addPerm(gs, 0, "Aura X", "enchantment", "aura")
		a.AttachedTo = uril
	}

	urilTheMiststalkerETB(gs, uril)

	if got := uril.Power(); got != 11 {
		t.Errorf("Uril power with 3 auras = %d, want 11 (5+6)", got)
	}
}

func TestUril_RecomputeReplacesPriorBuff(t *testing.T) {
	gs := newGame(t, 2)
	uril := stampCreaturePT(addPerm(gs, 0, "Uril, the Miststalker", "creature", "legendary"), 5, 5)
	a := addPerm(gs, 0, "Aura One", "enchantment", "aura")
	a.AttachedTo = uril

	urilTheMiststalkerETB(gs, uril)
	if uril.Power() != 7 {
		t.Fatalf("initial buff missing; power=%d", uril.Power())
	}

	// Detach + re-trigger via LTB recompute. Simulate the Aura leaving.
	a.AttachedTo = nil
	for i, p := range gs.Seats[0].Battlefield {
		if p == a {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield[:i], gs.Seats[0].Battlefield[i+1:]...)
			break
		}
	}
	urilRecomputeOnAnyLTB(gs, uril, map[string]interface{}{})

	if got := uril.Power(); got != 5 {
		t.Errorf("after Aura LTB, Uril power = %d, want 5 (recompute should drop buff)", got)
	}
	// Modifications slice should be free of any uril_aura_buff entries.
	for _, m := range uril.Modifications {
		if m.Duration == urilAuraBuffTag {
			t.Errorf("residual uril_aura_buff Modification after recompute: %+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Noctis, Prince of Lucis — graveyard cast permissions for artifacts
// ---------------------------------------------------------------------------

func TestNoctis_RegistersGrantsForArtifactsInGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	noctis := stampCreaturePT(addPerm(gs, 0, "Noctis, Prince of Lucis", "creature", "legendary"), 3, 3)

	// Plant graveyard cards: 2 artifacts, 1 creature, 1 instant.
	art1 := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}}
	art2 := &gameengine.Card{Name: "Mana Vault", Owner: 0, Types: []string{"artifact"}}
	creat := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	inst := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, art1, art2, creat, inst)

	noctisPrinceOfLucisETB(gs, noctis)

	for _, c := range []*gameengine.Card{art1, art2} {
		if gs.ZoneCastGrants == nil || gs.ZoneCastGrants[c] == nil {
			t.Errorf("no grant registered for artifact %q", c.DisplayName())
			continue
		}
		g := gs.ZoneCastGrants[c]
		if g.Zone != gameengine.ZoneGraveyard {
			t.Errorf("grant Zone = %q, want graveyard", g.Zone)
		}
		if g.RequireController != 0 {
			t.Errorf("grant RequireController = %d, want 0", g.RequireController)
		}
		if len(g.AdditionalCosts) == 0 || g.AdditionalCosts[0].Kind != gameengine.AddCostKindPayLife {
			t.Errorf("grant should carry pay-life additional cost; got %+v", g.AdditionalCosts)
		}
		if g.AdditionalCosts[0].LifeAmount != 3 {
			t.Errorf("grant life cost = %d, want 3", g.AdditionalCosts[0].LifeAmount)
		}
	}
	if gs.ZoneCastGrants != nil && gs.ZoneCastGrants[creat] != nil {
		t.Errorf("creature card should NOT receive Noctis grant")
	}
	if gs.ZoneCastGrants != nil && gs.ZoneCastGrants[inst] != nil {
		t.Errorf("instant card should NOT receive Noctis grant")
	}
}

func TestNoctis_RefreshOnLTBPicksUpFreshArtifact(t *testing.T) {
	gs := newGame(t, 2)
	noctis := stampCreaturePT(addPerm(gs, 0, "Noctis, Prince of Lucis", "creature", "legendary"), 3, 3)

	// Empty graveyard at ETB time → 0 grants.
	noctisPrinceOfLucisETB(gs, noctis)
	if gs.ZoneCastGrants != nil && len(gs.ZoneCastGrants) != 0 {
		t.Fatalf("expected 0 grants at ETB with empty gy, got %d", len(gs.ZoneCastGrants))
	}

	// An artifact ends up in graveyard later. Fire the refresh trigger.
	newArt := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, newArt)
	noctisRefreshGrants(gs, noctis, map[string]interface{}{})

	if gs.ZoneCastGrants == nil || gs.ZoneCastGrants[newArt] == nil {
		t.Errorf("refresh should have registered grant for newly-arrived artifact")
	}
}

// ---------------------------------------------------------------------------
// Shilgengar, Sire of Famine — activated Blood + mass reanim
// ---------------------------------------------------------------------------

func TestShilgengar_SacNonAngelCreatesOneBlood(t *testing.T) {
	gs := newGame(t, 2)
	shil := stampCreaturePT(addPerm(gs, 0, "Shilgengar, Sire of Famine", "creature", "legendary"), 5, 5)
	victim := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature"), 1, 1)

	preGY := len(gs.Seats[0].Graveyard)
	shilgengarSacForBlood(gs, shil, map[string]interface{}{
		"creature_perm": victim,
	})

	// Victim should be in graveyard.
	if got := len(gs.Seats[0].Graveyard); got != preGY+1 {
		t.Errorf("graveyard delta = %d, want +1 after sac", got-preGY)
	}
	// Exactly one Blood token should be on the battlefield.
	bloods := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && isBloodToken(p) {
			bloods++
		}
	}
	if bloods != 1 {
		t.Errorf("expected 1 Blood token, got %d", bloods)
	}
}

func TestShilgengar_SacAngelCreatesToughnessBloods(t *testing.T) {
	gs := newGame(t, 2)
	shil := stampCreaturePT(addPerm(gs, 0, "Shilgengar, Sire of Famine", "creature", "legendary"), 5, 5)
	angel := stampCreaturePT(addPerm(gs, 0, "Serra Angel", "creature", "angel"), 4, 4)

	shilgengarSacForBlood(gs, shil, map[string]interface{}{
		"creature_perm": angel,
	})

	bloods := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && isBloodToken(p) {
			bloods++
		}
	}
	if bloods != 4 {
		t.Errorf("Angel with toughness 4 should mint 4 Bloods; got %d", bloods)
	}
}

func TestShilgengar_SacRefusesWhenNoOtherCreature(t *testing.T) {
	gs := newGame(t, 2)
	shil := stampCreaturePT(addPerm(gs, 0, "Shilgengar, Sire of Famine", "creature", "legendary"), 5, 5)

	preBF := len(gs.Seats[0].Battlefield)
	shilgengarSacForBlood(gs, shil, nil)
	if got := len(gs.Seats[0].Battlefield); got != preBF {
		t.Errorf("with no sac fodder, should not mint Bloods or sac self; bf delta=%d", got-preBF)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event when no other creature available")
	}
}

func TestShilgengar_MassReanimReturnsAllCreaturesWithFinality(t *testing.T) {
	gs := newGame(t, 2)
	shil := stampCreaturePT(addPerm(gs, 0, "Shilgengar, Sire of Famine", "creature", "legendary"), 5, 5)

	// Seat 0: six Blood tokens.
	for i := 0; i < 6; i++ {
		gameengine.CreateBloodToken(gs, 0)
	}

	// Seat 0 graveyard: 2 creatures + 1 noncreature.
	c1 := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	c2 := &gameengine.Card{Name: "Wolf", Owner: 0, Types: []string{"creature"}}
	noncreat := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c1, c2, noncreat)

	shilgengarMassReanimate(gs, shil, nil)

	// Both creature cards should now be permanents with finality counters.
	returned := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card == c1 || p.Card == c2 {
			returned++
			if p.Counters["finality"] != 1 {
				t.Errorf("returned %q should have finality counter; got %d",
					p.Card.DisplayName(), p.Counters["finality"])
			}
			hasVamp := false
			for _, ty := range p.Card.Types {
				if ty == "vampire" {
					hasVamp = true
				}
			}
			if !hasVamp {
				t.Errorf("returned %q should have vampire subtype; types=%v",
					p.Card.DisplayName(), p.Card.Types)
			}
		}
	}
	if returned != 2 {
		t.Errorf("expected 2 creatures returned, got %d", returned)
	}
	// Noncreature should still be in graveyard.
	stillThere := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == noncreat {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Errorf("noncreature card should remain in graveyard")
	}
	// 0 Bloods left.
	bloods := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && isBloodToken(p) {
			bloods++
		}
	}
	if bloods != 0 {
		t.Errorf("all 6 Bloods should be sacrificed; %d remain", bloods)
	}
}

func TestShilgengar_MassReanimNeedsSixBloods(t *testing.T) {
	gs := newGame(t, 2)
	shil := stampCreaturePT(addPerm(gs, 0, "Shilgengar, Sire of Famine", "creature", "legendary"), 5, 5)

	// Only 5 Bloods.
	for i := 0; i < 5; i++ {
		gameengine.CreateBloodToken(gs, 0)
	}
	c1 := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c1)

	shilgengarMassReanimate(gs, shil, nil)
	// Bear should NOT have been returned (insufficient bloods).
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == c1 {
			t.Errorf("with only 5 Bloods, mass reanim should not fire")
		}
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed for insufficient bloods")
	}
}

// ---------------------------------------------------------------------------
// Wraith, Vicious Vigilante — Fear Gas unblockable static
// ---------------------------------------------------------------------------

func TestWraith_ETBStampsUnblockableFlag(t *testing.T) {
	gs := newGame(t, 2)
	wraith := stampCreaturePT(addPerm(gs, 0, "Wraith, Vicious Vigilante", "creature"), 3, 3)

	wraithViciousVigilanteETB(gs, wraith)

	if wraith.Flags["unblockable"] != 1 {
		t.Errorf("Wraith should be stamped unblockable; flags=%v", wraith.Flags)
	}
}

func TestWraith_ETBNilSafe(t *testing.T) {
	wraithViciousVigilanteETB(nil, nil)
	gs := newGame(t, 2)
	wraithViciousVigilanteETB(gs, nil)
	// No panic = pass.
}

// ---------------------------------------------------------------------------
// Zaffai and the Tempests — once-per-turn free I/S cast tracker
// ---------------------------------------------------------------------------

func TestZaffai_ETBStampsAvailableFlag(t *testing.T) {
	gs := newGame(t, 2)
	zaffai := stampCreaturePT(addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary"), 3, 3)

	zaffaiAndTheTempestsETB(gs, zaffai)

	if got := gs.Seats[0].Flags["zaffai_free_cast_available"]; got == 0 {
		t.Errorf("Zaffai ETB should stamp zaffai_free_cast_available; got %d", got)
	}
}

func TestZaffai_SpellCastConsumesOncePerTurn(t *testing.T) {
	gs := newGame(t, 2)
	zaffai := stampCreaturePT(addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary"), 3, 3)
	zaffaiAndTheTempestsETB(gs, zaffai)

	spell := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	zaffaiSpellCastConsume(gs, zaffai, map[string]interface{}{
		"caster_seat": 0,
		"card":        spell,
	})

	key := zaffaiUsedKey(gs.Turn)
	if got := gs.Seats[0].Flags[key]; got == 0 {
		t.Errorf("first I/S cast should consume the ration; flag %s = %d", key, got)
	}
}

func TestZaffai_IgnoresOpponentSpells(t *testing.T) {
	gs := newGame(t, 2)
	zaffai := stampCreaturePT(addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary"), 3, 3)
	zaffaiAndTheTempestsETB(gs, zaffai)

	spell := &gameengine.Card{Name: "Bolt", Owner: 1, Types: []string{"instant"}}
	zaffaiSpellCastConsume(gs, zaffai, map[string]interface{}{
		"caster_seat": 1, // opponent
		"card":        spell,
	})

	key := zaffaiUsedKey(gs.Turn)
	if got := gs.Seats[0].Flags[key]; got != 0 {
		t.Errorf("opponent's I/S cast should NOT consume Zaffai's ration; flag %s = %d", key, got)
	}
}

func TestZaffai_IgnoresNonInstantSorcerySpells(t *testing.T) {
	gs := newGame(t, 2)
	zaffai := stampCreaturePT(addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary"), 3, 3)
	zaffaiAndTheTempestsETB(gs, zaffai)

	creature := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"creature"}}
	zaffaiSpellCastConsume(gs, zaffai, map[string]interface{}{
		"caster_seat": 0,
		"card":        creature,
	})

	key := zaffaiUsedKey(gs.Turn)
	if got := gs.Seats[0].Flags[key]; got != 0 {
		t.Errorf("creature cast should NOT consume Zaffai's ration; flag %s = %d", key, got)
	}
}

func TestZaffai_UpkeepRefreshClearsPriorTurnsUsedFlag(t *testing.T) {
	gs := newGame(t, 2)
	zaffai := stampCreaturePT(addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary"), 3, 3)
	zaffaiAndTheTempestsETB(gs, zaffai)

	// Manually stamp two stale "used" flags from prior turns.
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags[zaffaiUsedKey(0)] = 1
	gs.Seats[0].Flags[zaffaiUsedKey(3)] = 4

	zaffaiUpkeepRefresh(gs, zaffai, map[string]interface{}{
		"active_seat": 0,
	})

	for k := range gs.Seats[0].Flags {
		if len(k) > len(zaffaiUsedPrefix) && k[:len(zaffaiUsedPrefix)] == zaffaiUsedPrefix {
			t.Errorf("upkeep refresh should clear used-flags; %s still set", k)
		}
	}
}
