package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// outlaw_etb_consumers_r60 — 6 handlers (across 7 OnTrigger sites,
// since Rakish Crew has two) wired in outlaw_etb_consumers_r60.go
// covering the OTJ Outlaw type-group payoff family.
// -----------------------------------------------------------------------------

// outlawETBCtx mirrors FireOutlawETBTriggers's ctx shape.
func outlawETBCtx(entering, watcher *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"perm":            entering,
		"card":            entering.Card,
		"card_name":       entering.Card.DisplayName(),
		"controller_seat": entering.Controller,
		"watcher":         watcher,
		"watcher_seat":    watcher.Controller,
	}
}

// permEtbCtx mirrors the engine's permanent_etb ctx.
func permEtbCtx(p *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"perm":            p,
		"controller_seat": p.Controller,
		"card":            p.Card,
	}
}

// permSacCtx mirrors zone_change.go's permanent_sacrificed ctx.
func permSacCtx(p *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"perm":            p,
		"card":            p.Card,
		"controller_seat": p.Controller,
		"card_name":       p.Card.DisplayName(),
		"to_zone":         "graveyard",
	}
}

// -----------------------------------------------------------------------------
// Vial Smasher
// -----------------------------------------------------------------------------

func TestVialSmasher_PingsOppOnAnotherOutlawETB(t *testing.T) {
	gs := newGame(t, 2)
	vial := addPerm(gs, 0, "Vial Smasher, Gleeful Grenadier", "creature", "legendary", "goblin", "mercenary")
	vial.Card.BasePower, vial.Card.BaseToughness = 1, 3
	gs.Seats[1].Life = 20

	rogue := addPerm(gs, 0, "Random Rogue", "creature", "rogue")
	rogue.Card.BasePower, rogue.Card.BaseToughness = 2, 2
	gameengine.FireCardTrigger(gs, "outlaw_etb", outlawETBCtx(rogue, vial))

	if gs.Seats[1].Life != 19 {
		t.Errorf("expected 1 damage to opp; got %d life", gs.Seats[1].Life)
	}
}

func TestVialSmasher_DoesNotFireOnSelfETB(t *testing.T) {
	gs := newGame(t, 2)
	vial := addPerm(gs, 0, "Vial Smasher, Gleeful Grenadier", "creature", "legendary", "goblin", "mercenary")
	vial.Card.BasePower, vial.Card.BaseToughness = 1, 3
	gs.Seats[1].Life = 20

	// Vial Smasher's own ETB — oracle says "another outlaw" so the
	// handler must skip self-events.
	gameengine.FireCardTrigger(gs, "outlaw_etb", outlawETBCtx(vial, vial))

	if gs.Seats[1].Life != 20 {
		t.Errorf("Vial must not ping on its OWN ETB; opp life=%d", gs.Seats[1].Life)
	}
}

func TestVialSmasher_DoesNotFireOnOppOutlawETB(t *testing.T) {
	gs := newGame(t, 2)
	vial := addPerm(gs, 0, "Vial Smasher, Gleeful Grenadier", "creature", "legendary", "goblin", "mercenary")
	vial.Card.BasePower, vial.Card.BaseToughness = 1, 3
	gs.Seats[1].Life = 20

	oppOutlaw := addPerm(gs, 1, "Opp Rogue", "creature", "rogue")
	oppOutlaw.Card.BasePower, oppOutlaw.Card.BaseToughness = 2, 2
	gameengine.FireCardTrigger(gs, "outlaw_etb", outlawETBCtx(oppOutlaw, vial))

	if gs.Seats[1].Life != 20 {
		t.Errorf("opp's outlaw should not trigger Vial's payoff; opp life=%d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Mine Raider
// -----------------------------------------------------------------------------

func TestMineRaider_CreatesTreasureWhenAnotherOutlaw(t *testing.T) {
	gs := newGame(t, 2)
	// Pre-existing outlaw on the battlefield (give it real P/T so SBA
	// doesn't destroy it mid-dispatch and drop the outlaw count to 1).
	other := addPerm(gs, 0, "Other Rogue", "creature", "rogue")
	other.Card.BasePower, other.Card.BaseToughness = 1, 1
	raider := addPerm(gs, 0, "Mine Raider", "creature", "rogue")
	raider.Card.BasePower, raider.Card.BaseToughness = 3, 2
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", permEtbCtx(raider))

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected +1 (Treasure token); pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	foundTreasure := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if t == "treasure" {
				foundTreasure = true
				break
			}
		}
	}
	if !foundTreasure {
		t.Errorf("expected a Treasure on battlefield")
	}
}

func TestMineRaider_NoTreasureWhenSoleOutlaw(t *testing.T) {
	gs := newGame(t, 2)
	raider := addPerm(gs, 0, "Mine Raider", "creature", "rogue")
	raider.Card.BasePower, raider.Card.BaseToughness = 3, 2
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", permEtbCtx(raider))

	if len(gs.Seats[0].Battlefield) != pre {
		t.Errorf("with only itself as outlaw, Mine Raider must NOT mint a Treasure; pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
}

// -----------------------------------------------------------------------------
// Hellspur Posse Boss
// -----------------------------------------------------------------------------

func TestHellspurPosseBoss_CreatesTwoMercenariesOnETB(t *testing.T) {
	gs := newGame(t, 2)
	boss := addPerm(gs, 0, "Hellspur Posse Boss", "creature", "lizard", "rogue")
	boss.Card.BasePower, boss.Card.BaseToughness = 4, 4
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", permEtbCtx(boss))

	if len(gs.Seats[0].Battlefield) != pre+2 {
		t.Errorf("expected +2 (two Mercenary tokens); pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	mercs := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Mercenary" {
			mercs++
		}
	}
	if mercs != 2 {
		t.Errorf("expected 2 Mercenary tokens, got %d", mercs)
	}
}

// -----------------------------------------------------------------------------
// Charred Graverobber
// -----------------------------------------------------------------------------

func TestCharredGraverobber_ReturnsOutlawFromGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	bigOutlaw := &gameengine.Card{Name: "Olivia, Opulent Outlaw", Owner: 0,
		Types: []string{"creature", "vampire", "assassin", "cmc:4"}}
	smallOutlaw := &gameengine.Card{Name: "Random Rogue", Owner: 0,
		Types: []string{"creature", "rogue", "cmc:1"}}
	noise := &gameengine.Card{Name: "Llanowar Elves", Owner: 0,
		Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bigOutlaw, smallOutlaw, noise)

	grob := addPerm(gs, 0, "Charred Graverobber", "creature", "skeleton", "mercenary")
	grob.Card.BasePower, grob.Card.BaseToughness = 2, 2
	gameengine.FireCardTrigger(gs, "permanent_etb", permEtbCtx(grob))

	// Olivia (highest-CMC outlaw) returns to hand.
	foundBig := false
	for _, c := range gs.Seats[0].Hand {
		if c == bigOutlaw {
			foundBig = true
		}
	}
	if !foundBig {
		t.Errorf("expected highest-CMC outlaw (Olivia) returned to hand; hand=%v", handNames(gs.Seats[0].Hand))
	}
	// Non-outlaw stays.
	stillInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == noise {
			stillInGY = true
		}
	}
	if !stillInGY {
		t.Errorf("non-outlaw should stay in graveyard")
	}
}

func TestCharredGraverobber_NoOutlawEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature"}},
	)
	grob := addPerm(gs, 0, "Charred Graverobber", "creature", "skeleton", "mercenary")
	grob.Card.BasePower, grob.Card.BaseToughness = 2, 2
	gameengine.FireCardTrigger(gs, "permanent_etb", permEtbCtx(grob))

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no outlaw in graveyard")
	}
}

// -----------------------------------------------------------------------------
// Rakish Crew — ETB token half
// -----------------------------------------------------------------------------

func TestRakishCrew_CreatesMercenaryOnETB(t *testing.T) {
	gs := newGame(t, 2)
	crew := addPerm(gs, 0, "Rakish Crew", "enchantment")
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", permEtbCtx(crew))

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected +1 (Mercenary token); pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
}

// -----------------------------------------------------------------------------
// Rakish Crew — outlaw-dies half
// -----------------------------------------------------------------------------

func TestRakishCrew_DrainsOnYourOutlawDies(t *testing.T) {
	gs := newGame(t, 4)
	addPerm(gs, 0, "Rakish Crew", "enchantment")
	for i := 0; i < 4; i++ {
		gs.Seats[i].Life = 20
	}
	dyingOutlaw := addPerm(gs, 0, "Random Rogue", "creature", "rogue")
	dyingOutlaw.Card.BasePower, dyingOutlaw.Card.BaseToughness = 2, 2

	gameengine.FireCardTrigger(gs, "creature_dies", permSacCtx(dyingOutlaw))

	if gs.Seats[0].Life != 21 {
		t.Errorf("expected +1 life to controller, got %d", gs.Seats[0].Life)
	}
	for i := 1; i < 4; i++ {
		if gs.Seats[i].Life != 19 {
			t.Errorf("opp %d expected 19 life, got %d", i, gs.Seats[i].Life)
		}
	}
}

func TestRakishCrew_NonOutlawDoesNotTrigger(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Rakish Crew", "enchantment")
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	// Plain creature (no outlaw subtype) dies — Rakish Crew must not fire.
	plain := addPerm(gs, 0, "Llanowar Elves", "creature")
	plain.Card.BasePower, plain.Card.BaseToughness = 1, 1
	gameengine.FireCardTrigger(gs, "creature_dies", permSacCtx(plain))

	if gs.Seats[0].Life != 20 || gs.Seats[1].Life != 20 {
		t.Errorf("non-outlaw death must not trigger Rakish Crew; seat0=%d seat1=%d",
			gs.Seats[0].Life, gs.Seats[1].Life)
	}
}

func TestRakishCrew_OppOutlawDoesNotTrigger(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Rakish Crew", "enchantment")
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	oppOutlaw := addPerm(gs, 1, "Opp Rogue", "creature", "rogue")
	oppOutlaw.Card.BasePower, oppOutlaw.Card.BaseToughness = 2, 2
	gameengine.FireCardTrigger(gs, "creature_dies", permSacCtx(oppOutlaw))

	if gs.Seats[0].Life != 20 || gs.Seats[1].Life != 20 {
		t.Errorf("opp outlaw death must not trigger; seat0=%d seat1=%d",
			gs.Seats[0].Life, gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Discreet Retreat — first outlaw cast each turn → draw 1 + lose 1
// -----------------------------------------------------------------------------

func TestDiscreetRetreat_FirstOutlawCastDrawsAndLosesOne(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top")
	addPerm(gs, 0, "Discreet Retreat", "enchantment", "aura")
	gs.Seats[0].Life = 20

	outlaw := addCard(gs, 0, "Random Rogue", "creature", "rogue", "cmc:2")
	gameengine.FireCardTrigger(gs, "spell_cast", castCtx(0, outlaw))

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn, got hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != 19 {
		t.Errorf("expected -1 life, got %d", gs.Seats[0].Life)
	}
}

func TestDiscreetRetreat_SecondOutlawSameTurnNoOps(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top", "Next")
	addPerm(gs, 0, "Discreet Retreat", "enchantment", "aura")
	gs.Seats[0].Life = 20

	outlaw1 := addCard(gs, 0, "First Rogue", "creature", "rogue", "cmc:1")
	gameengine.FireCardTrigger(gs, "spell_cast", castCtx(0, outlaw1))
	// First fire drew + cost 1.

	outlaw2 := addCard(gs, 0, "Second Rogue", "creature", "rogue", "cmc:1")
	gameengine.FireCardTrigger(gs, "spell_cast", castCtx(0, outlaw2))

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("second outlaw cast same turn must NOT trigger another draw; hand=%d",
			len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != 19 {
		t.Errorf("second outlaw cast same turn must NOT cost another life; life=%d",
			gs.Seats[0].Life)
	}
}

func TestDiscreetRetreat_NonOutlawCastDoesNotTrigger(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top")
	addPerm(gs, 0, "Discreet Retreat", "enchantment", "aura")
	gs.Seats[0].Life = 20

	// Plain creature (not an outlaw) — handler must abstain.
	plain := addCard(gs, 0, "Llanowar Elves", "creature", "cmc:1")
	gameengine.FireCardTrigger(gs, "spell_cast", castCtx(0, plain))

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("non-outlaw spell must not trigger Discreet Retreat; hand=%d",
			len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != 20 {
		t.Errorf("life should be unchanged; got %d", gs.Seats[0].Life)
	}
}

func TestDiscreetRetreat_IgnoresOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top")
	addPerm(gs, 0, "Discreet Retreat", "enchantment", "aura")
	gs.Seats[0].Life = 20

	oppOutlaw := addCard(gs, 1, "Opp Rogue", "creature", "rogue", "cmc:1")
	gameengine.FireCardTrigger(gs, "spell_cast", castCtx(1, oppOutlaw))

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("opp casts must not trigger Discreet Retreat; hand=%d",
			len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Registry smoke
// -----------------------------------------------------------------------------

func TestOutlawETBConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Vial Smasher, Gleeful Grenadier",
		"Mine Raider",
		"Hellspur Posse Boss",
		"Charred Graverobber",
		"Rakish Crew",
		"Discreet Retreat",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler for %s", name)
		}
	}
}
