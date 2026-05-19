package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R42b stub-batch ports — five gen_*.go pure-stub handlers from the
// alphabetical second half (n-z), distinct from dev-5's parallel
// r42 batch and from earlier R36/R37/R41 sets.
//
// Picks:
//   - Rosnakht, Heir of Rohgahh        Heroic Kobold token on spell-cast
//   - Uril, the Miststalker             Aura-count P/T buff via Modifications
//   - The Second Doctor                 end_step draw + opponent attack flag
//   - Noctis, Prince of Lucis           graveyard zone-cast grant +3 life
//   - Shilgengar, Sire of Famine        activated sac-for-Blood + mass reanim

// ---------------------------------------------------------------------------
// Rosnakht — Heroic spell-cast trigger
// ---------------------------------------------------------------------------

func TestRosnakht_HeroicMintsKoboldOnSelfTarget(t *testing.T) {
	gs := newGame(t, 2)
	ros := stampCreaturePT(addPerm(gs, 0, "Rosnakht, Heir of Rohgahh", "creature", "legendary"), 1, 1)

	spellCard := &gameengine.Card{
		Name:  "Giant Growth",
		Owner: 0,
		Types: []string{"instant"},
	}
	item := &gameengine.StackItem{
		Card:       spellCard,
		Controller: 0,
		Targets: []gameengine.Target{
			{Kind: gameengine.TargetKindPermanent, Permanent: ros},
		},
	}

	preBF := len(gs.Seats[0].Battlefield)
	rosnakhtHeroicKoboldToken(gs, ros, map[string]interface{}{
		"caster_seat": 0,
		"card":        spellCard,
		"stack_item":  item,
	})
	if got := len(gs.Seats[0].Battlefield); got != preBF+1 {
		t.Fatalf("battlefield delta = %d, want +1 Kobold", got-preBF)
	}
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Kobolds of Kher Keep" {
			found = true
			if p.Card.BasePower != 0 || p.Card.BaseToughness != 1 {
				t.Errorf("Kobold P/T = %d/%d, want 0/1", p.Card.BasePower, p.Card.BaseToughness)
			}
			hasR := false
			for _, c := range p.Card.Colors {
				if c == "R" {
					hasR = true
				}
			}
			if !hasR {
				t.Errorf("Kobold colors = %v, want includes R", p.Card.Colors)
			}
		}
	}
	if !found {
		t.Fatal("Kobold token not found on battlefield")
	}
}

func TestRosnakht_HeroicSkipsWhenSpellDoesNotTargetRosnakht(t *testing.T) {
	gs := newGame(t, 2)
	ros := stampCreaturePT(addPerm(gs, 0, "Rosnakht, Heir of Rohgahh", "creature", "legendary"), 1, 1)
	other := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature"), 1, 1)

	spellCard := &gameengine.Card{
		Name:  "Giant Growth",
		Owner: 0,
		Types: []string{"instant"},
	}
	item := &gameengine.StackItem{
		Card:       spellCard,
		Controller: 0,
		Targets: []gameengine.Target{
			{Kind: gameengine.TargetKindPermanent, Permanent: other},
		},
	}

	preBF := len(gs.Seats[0].Battlefield)
	rosnakhtHeroicKoboldToken(gs, ros, map[string]interface{}{
		"caster_seat": 0,
		"card":        spellCard,
		"stack_item":  item,
	})
	if got := len(gs.Seats[0].Battlefield); got != preBF {
		t.Errorf("spell that doesn't target Rosnakht should NOT mint Kobold; bf delta=%d", got-preBF)
	}
}

func TestRosnakht_HeroicIgnoresOpponentCasts(t *testing.T) {
	gs := newGame(t, 2)
	ros := stampCreaturePT(addPerm(gs, 0, "Rosnakht, Heir of Rohgahh", "creature", "legendary"), 1, 1)

	spellCard := &gameengine.Card{
		Name:  "Smite",
		Owner: 1,
		Types: []string{"instant"},
	}
	item := &gameengine.StackItem{
		Card:       spellCard,
		Controller: 1, // opponent's spell
		Targets: []gameengine.Target{
			{Kind: gameengine.TargetKindPermanent, Permanent: ros},
		},
	}

	preBF := len(gs.Seats[0].Battlefield)
	rosnakhtHeroicKoboldToken(gs, ros, map[string]interface{}{
		"caster_seat": 1,
		"card":        spellCard,
		"stack_item":  item,
	})
	if got := len(gs.Seats[0].Battlefield); got != preBF {
		t.Errorf("opponent's spell targeting Rosnakht should NOT trigger Heroic; bf delta=%d", got-preBF)
	}
}

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
// The Second Doctor — How Civil of You end step
// ---------------------------------------------------------------------------

func TestSecondDoctor_EachPlayerDraws(t *testing.T) {
	gs := newGame(t, 3)
	doc := stampCreaturePT(addPerm(gs, 0, "The Second Doctor", "creature", "legendary"), 4, 4)
	addLibrary(gs, 0, "A1")
	addLibrary(gs, 1, "B1")
	addLibrary(gs, 2, "C1")

	theSecondDoctorHowCivil(gs, doc, map[string]interface{}{
		"active_seat": 0,
	})

	for i := 0; i < 3; i++ {
		if len(gs.Seats[i].Hand) != 1 {
			t.Errorf("seat %d should have drawn 1; hand size = %d", i, len(gs.Seats[i].Hand))
		}
	}
}

func TestSecondDoctor_StampsAttackBlockFlagOnOpponentsThatDrew(t *testing.T) {
	gs := newGame(t, 3)
	doc := stampCreaturePT(addPerm(gs, 0, "The Second Doctor", "creature", "legendary"), 4, 4)
	addLibrary(gs, 0, "A1")
	addLibrary(gs, 1, "B1")
	// seat 2 has no library — can't draw, shouldn't get flag.

	theSecondDoctorHowCivil(gs, doc, map[string]interface{}{
		"active_seat": 0,
	})

	flagKey := doctorAttackBlockKey(0)
	if v := gs.Seats[1].Flags[flagKey]; v != gs.Turn+1 {
		t.Errorf("seat 1 should have flag %s = %d; got %d", flagKey, gs.Turn+1, v)
	}
	if v := gs.Seats[2].Flags[flagKey]; v != 0 {
		t.Errorf("seat 2 didn't draw and should NOT have flag; got %d", v)
	}
	if v := gs.Seats[0].Flags[flagKey]; v != 0 {
		t.Errorf("Doctor's own seat should NOT have flag against itself; got %d", v)
	}
}

func TestSecondDoctor_SkipsOpponentEndStep(t *testing.T) {
	gs := newGame(t, 2)
	doc := stampCreaturePT(addPerm(gs, 0, "The Second Doctor", "creature", "legendary"), 4, 4)
	addLibrary(gs, 0, "A1")
	addLibrary(gs, 1, "B1")

	theSecondDoctorHowCivil(gs, doc, map[string]interface{}{
		"active_seat": 1, // opponent's end step
	})

	for i := 0; i < 2; i++ {
		if len(gs.Seats[i].Hand) != 0 {
			t.Errorf("seat %d should NOT have drawn on opponent end step; hand=%d", i, len(gs.Seats[i].Hand))
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
