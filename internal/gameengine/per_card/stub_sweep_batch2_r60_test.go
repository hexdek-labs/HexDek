package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestYgraEaterOfAll_AnyOtherCreatureDyingAddsTwoCounters pins the
// "other creatures are Food in addition to their other types" §613
// type-add: while Ygra is on the battlefield, any other creature
// dying counts as a Food dying for trigger purposes.
func TestYgraEaterOfAll_AnyOtherCreatureDyingAddsTwoCounters(t *testing.T) {
	gs := newGame(t, 2)
	ygra := addPerm(gs, 0, "Ygra, Eater of All", "legendary", "creature", "cat", "beast")
	prey := addCard(gs, 0, "Llanowar Elves", "creature", "elf")

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            nil,
		"card":            prey,
		"to_zone":         "graveyard",
		"controller_seat": 0,
	})

	if got := ygra.Counters["+1/+1"]; got != 2 {
		t.Errorf("expected 2 +1/+1 counters from creature-as-Food dies; got %d", got)
	}
}

// TestYgraEaterOfAll_SelfLTBDoesNotStack pins the "other" qualifier:
// Ygra's own LTB does NOT trigger her counter ability (and we'd be
// adding counters to a permanent already on the way out).
func TestYgraEaterOfAll_SelfLTBDoesNotStack(t *testing.T) {
	gs := newGame(t, 2)
	ygra := addPerm(gs, 0, "Ygra, Eater of All", "legendary", "creature", "cat", "beast")

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            ygra,
		"card":            ygra.Card,
		"to_zone":         "graveyard",
		"controller_seat": 0,
	})

	if got := ygra.Counters["+1/+1"]; got != 0 {
		t.Errorf("self-LTB should not add counters; got %d", got)
	}
}

// TestVihaanGoldwaker_OutlawAnthemStampsVigilanceAndHaste pins the
// outlaw anthem: other outlaw creatures we control get vigilance + haste.
func TestVihaanGoldwaker_OutlawAnthemStampsVigilanceAndHaste(t *testing.T) {
	gs := newGame(t, 2)
	pirate := addPerm(gs, 0, "Test Pirate", "creature", "pirate")
	rogue := addPerm(gs, 0, "Test Rogue", "creature", "rogue")
	nonOutlaw := addPerm(gs, 0, "Test Elf", "creature", "elf")
	vihaan := addPerm(gs, 0, "Vihaan, Goldwaker", "legendary", "creature", "human", "warlock")

	gameengine.InvokeETBHook(gs, vihaan)

	if pirate.Flags["kw:vigilance"] != 1 || pirate.Flags["kw:haste"] != 1 {
		t.Errorf("pirate should have both kw flags: %+v", pirate.Flags)
	}
	if rogue.Flags["kw:vigilance"] != 1 || rogue.Flags["kw:haste"] != 1 {
		t.Errorf("rogue should have both kw flags: %+v", rogue.Flags)
	}
	if nonOutlaw.Flags["kw:vigilance"] == 1 || nonOutlaw.Flags["kw:haste"] == 1 {
		t.Errorf("non-outlaw should not have anthem flags: %+v", nonOutlaw.Flags)
	}
	if vihaan.Flags["kw:vigilance"] == 1 || vihaan.Flags["kw:haste"] == 1 {
		t.Errorf("Vihaan herself (other-qualifier) should not get the grant: %+v", vihaan.Flags)
	}
}

// TestVihaanGoldwaker_LTBStripsGrantedFlagsOnly pins the LTB cleanup
// strips only flags WE granted, leaving native AST keywords alone.
func TestVihaanGoldwaker_LTBStripsGrantedFlagsOnly(t *testing.T) {
	gs := newGame(t, 2)
	pirate := addPerm(gs, 0, "Test Pirate", "creature", "pirate")
	nativeVig := addPerm(gs, 0, "Native Vig Pirate", "creature", "pirate")
	nativeVig.Flags["kw:vigilance"] = 1 // pre-existing native
	vihaan := addPerm(gs, 0, "Vihaan, Goldwaker", "legendary", "creature", "human", "warlock")

	gameengine.InvokeETBHook(gs, vihaan)

	// Sanity: pirate got the grant, nativeVig got only the haste add
	// (vigilance native via the flag was respected by the HasKeyword check).
	if pirate.Flags["vihaan_goldwaker_grant"] != 1 {
		t.Fatalf("setup: pirate should have grant marker: %+v", pirate.Flags)
	}

	// Fire LTB.
	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            vihaan,
		"card":            vihaan.Card,
		"to_zone":         "graveyard",
		"controller_seat": 0,
	})

	if pirate.Flags["kw:vigilance"] == 1 || pirate.Flags["kw:haste"] == 1 {
		t.Errorf("granted flags should strip on LTB: %+v", pirate.Flags)
	}
	if pirate.Flags["vihaan_goldwaker_grant"] == 1 {
		t.Errorf("grant marker should strip on LTB: %+v", pirate.Flags)
	}
	// Native-vigilance pirate: vigilance still set (was native, never granted
	// by us — we skipped because HasKeyword("vigilance") returned true).
	if nativeVig.Flags["kw:vigilance"] != 1 {
		t.Errorf("native vigilance must survive LTB strip: %+v", nativeVig.Flags)
	}
}

// TestYunaGrandSummoner_ActivationStampsBonusFlag pins the {T}: Add
// one mana of any color activation, which also stamps the deferred-cast
// counter-bonus flag.
func TestYunaGrandSummoner_ActivationStampsBonusFlag(t *testing.T) {
	gs := newGame(t, 2)
	yuna := addPerm(gs, 0, "Yuna, Grand Summoner", "legendary", "creature", "human")

	gameengine.InvokeActivatedHook(gs, yuna, 0, nil)

	if !yuna.Tapped {
		t.Errorf("expected Yuna to be tapped after activation")
	}
	if got := gs.Seats[0].Flags["yuna_next_creature_bonus_turn"]; got != gs.Turn+1 {
		t.Errorf("expected bonus flag for turn+1=%d, got %d", gs.Turn+1, got)
	}
}

// TestYunaGrandSummoner_NextCreatureETBGetsTwoCounters pins the
// cross-trigger handshake: activate Yuna, then a creature ETBs under
// our control, and that creature gets two +1/+1 counters.
func TestYunaGrandSummoner_NextCreatureETBGetsTwoCounters(t *testing.T) {
	gs := newGame(t, 2)
	yuna := addPerm(gs, 0, "Yuna, Grand Summoner", "legendary", "creature", "human")
	gameengine.InvokeActivatedHook(gs, yuna, 0, nil)

	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            bear,
		"controller_seat": 0,
	})

	if got := bear.Counters["+1/+1"]; got != 2 {
		t.Errorf("expected 2 +1/+1 counters from Yuna bonus; got %d", got)
	}
	// One-shot: flag consumed.
	if _, ok := gs.Seats[0].Flags["yuna_next_creature_bonus_turn"]; ok {
		t.Errorf("bonus flag should be consumed after first creature ETB")
	}
}

// TestYunaGrandSummoner_NoActivationNoBonus pins the negative: without
// the activation, an entering creature gets no bonus.
func TestYunaGrandSummoner_NoActivationNoBonus(t *testing.T) {
	gs := newGame(t, 2)
	_ = addPerm(gs, 0, "Yuna, Grand Summoner", "legendary", "creature", "human")
	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            bear,
		"controller_seat": 0,
	})

	if got := bear.Counters["+1/+1"]; got != 0 {
		t.Errorf("no activation → no bonus; got %d counters", got)
	}
}

// TestRashmiAndRagavan_FreeCastWhenCMCLessThanArtifacts pins the
// free-cast grant: exiled card with MV < artifact count gets a
// ManaCost=0 cast-from-exile permission.
func TestRashmiAndRagavan_FreeCastWhenCMCLessThanArtifacts(t *testing.T) {
	gs := newGame(t, 2)
	rashmi := addPerm(gs, 0, "Rashmi and Ragavan", "legendary", "creature", "elf", "monkey")
	// Real P/T so SBA 704.5f doesn't kill Rashmi mid-trigger and reap her
	// grant via the LTB ExpireSourceGrants pass.
	rashmi.Card.BasePower = 3
	rashmi.Card.BaseToughness = 2
	// Three artifacts already on board (so artifact count after Treasure mint = 4).
	addPerm(gs, 0, "Sol Ring", "artifact")
	addPerm(gs, 0, "Mana Vault", "artifact")
	addPerm(gs, 0, "Mox Diamond", "artifact")

	// Top of opponent's library: a CMC-2 instant (< 4 artifacts → free).
	exiled := &gameengine.Card{
		Name:  "Cheap Instant",
		Owner: 1,
		Types: []string{"instant", "cmc:2"},
	}
	gs.Seats[1].Library = append([]*gameengine.Card{exiled}, gs.Seats[1].Library...)

	gs.Active = 0
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Lightning Bolt",
		"is_creature": false,
	})

	_ = rashmi
	grant, ok := gs.ZoneCastGrants[exiled]
	if !ok {
		t.Fatalf("expected free-cast grant registered for exiled card; got none. grants=%+v", gs.ZoneCastGrants)
	}
	if grant.ManaCost != 0 {
		t.Errorf("expected ManaCost=0 (free); got %d", grant.ManaCost)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("expected Duration=until_end_of_turn; got %q", grant.Duration)
	}
	if grant.RequireController != 0 {
		t.Errorf("expected RequireController=0; got %d", grant.RequireController)
	}
}

// TestRashmiAndRagavan_NormalCostGrantWhenCMCNotLess pins the
// "if you don't cast it this way, you may cast it this turn" fallback:
// when MV is NOT less than artifact count, grant ManaCost=-1 (cast
// at normal cost from exile this turn).
func TestRashmiAndRagavan_NormalCostGrantWhenCMCNotLess(t *testing.T) {
	gs := newGame(t, 2)
	rashmi := addPerm(gs, 0, "Rashmi and Ragavan", "legendary", "creature", "elf", "monkey")
	rashmi.Card.BasePower = 3
	rashmi.Card.BaseToughness = 2
	// No prior artifacts. After Treasure mint, artifact count = 1.
	exiled := &gameengine.Card{
		Name:  "Big Spell",
		Owner: 1,
		Types: []string{"sorcery", "cmc:5"},
	}
	gs.Seats[1].Library = append([]*gameengine.Card{exiled}, gs.Seats[1].Library...)

	gs.Active = 0
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Whatever",
		"is_creature": false,
	})

	grant, ok := gs.ZoneCastGrants[exiled]
	if !ok {
		t.Fatalf("expected normal-cost grant registered for exiled card")
	}
	if grant.ManaCost != -1 {
		t.Errorf("expected ManaCost=-1 (normal cost); got %d", grant.ManaCost)
	}
}
