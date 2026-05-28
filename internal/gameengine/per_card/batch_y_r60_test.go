package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch Y (R60) — tests for 5 new commander handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Krenko, Tin Street Kingpin
// -----------------------------------------------------------------------------

func TestKrenkoTinStreet_AttackCountersAndTokens(t *testing.T) {
	gs := newGame(t, 4) // 3 opponents
	krenko := addPerm(gs, 0, "Krenko, Tin Street Kingpin", "creature", "legendary", "goblin")

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": krenko,
		"attacker_seat": 0,
	})

	if krenko.Counters["+1/+1"] != 3 {
		t.Errorf("expected 3 +1/+1 counters (3 opponents), got %d", krenko.Counters["+1/+1"])
	}
	// Count Goblin tokens minted (in addition to Krenko himself).
	tokens := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p == krenko || p.Card == nil {
			continue
		}
		if p.IsToken() && cardHasType(p.Card, "goblin") {
			tokens++
			if !p.Tapped {
				t.Errorf("Goblin token should be tapped, got Tapped=%v", p.Tapped)
			}
		}
	}
	if tokens != 3 {
		t.Errorf("expected 3 Goblin tokens, got %d", tokens)
	}
}

func TestKrenkoTinStreet_OnlyFiresOnOwnAttack(t *testing.T) {
	gs := newGame(t, 4)
	addPerm(gs, 0, "Krenko, Tin Street Kingpin", "creature", "legendary", "goblin")
	other := addPerm(gs, 0, "Llanowar Elves", "creature")

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": 0,
	})

	tokens := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsToken() && cardHasType(p.Card, "goblin") {
			tokens++
		}
	}
	if tokens != 0 {
		t.Errorf("Krenko must only trigger when HE attacks, got %d tokens", tokens)
	}
}

// -----------------------------------------------------------------------------
// Talrand, Sky Summoner
// -----------------------------------------------------------------------------

func TestTalrand_DrakeOnInstantSorceryCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Talrand, Sky Summoner", "creature", "legendary")
	instant := &gameengine.Card{Name: "Counterspell", Types: []string{"instant"}}

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        instant,
	})

	drakes := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsToken() && cardHasType(p.Card, "drake") {
			drakes++
		}
	}
	if drakes != 1 {
		t.Errorf("expected 1 Drake token after instant cast, got %d", drakes)
	}
}

func TestTalrand_NoDrakeOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Talrand, Sky Summoner", "creature", "legendary")
	instant := &gameengine.Card{Name: "Counterspell", Types: []string{"instant"}}

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 1,
		"card":        instant,
	})

	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsToken() && cardHasType(p.Card, "drake") {
			t.Errorf("Talrand must not fire on opp cast, got Drake token")
		}
	}
}

// -----------------------------------------------------------------------------
// Sram, Senior Edificer
// -----------------------------------------------------------------------------

func TestSram_DrawsOnAuraCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sram, Senior Edificer", "creature", "legendary")
	addLibrary(gs, 0, "A", "B")
	aura := &gameengine.Card{Name: "Bear Umbra", Types: []string{"enchantment", "aura"}}

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        aura,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Sram should cantrip on Aura cast, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestSram_DrawsOnEquipmentCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sram, Senior Edificer", "creature", "legendary")
	addLibrary(gs, 0, "A", "B")
	gear := &gameengine.Card{Name: "Skullclamp", Types: []string{"artifact", "equipment"}}

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        gear,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Sram should cantrip on Equipment cast, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestSram_DrawsOnVehicleCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sram, Senior Edificer", "creature", "legendary")
	addLibrary(gs, 0, "A", "B")
	vehicle := &gameengine.Card{Name: "Heart of Kiran", Types: []string{"artifact", "vehicle"}}

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        vehicle,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Sram should cantrip on Vehicle cast, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestSram_IgnoresUnrelatedSpells(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sram, Senior Edificer", "creature", "legendary")
	addLibrary(gs, 0, "A", "B")
	burn := &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}}

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        burn,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Sram must skip non-Aura/Equipment/Vehicle, hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Pia Nalaar, Consul of Revival
// -----------------------------------------------------------------------------

func TestPiaNalaar_ThopterOnArtifactCreatureDies(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Pia Nalaar, Consul of Revival", "creature", "legendary")
	dying := addPerm(gs, 0, "Steel Overseer", "creature", "artifact")

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"controller_seat": 0,
	})

	thopters := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsToken() && cardHasType(p.Card, "thopter") {
			thopters++
			if !p.Tapped {
				t.Errorf("Thopter must be tapped on ETB, got Tapped=%v", p.Tapped)
			}
		}
	}
	if thopters != 1 {
		t.Errorf("expected 1 Thopter from artifact-creature death, got %d", thopters)
	}
}

func TestPiaNalaar_IgnoresNonArtifactCreature(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Pia Nalaar, Consul of Revival", "creature", "legendary")
	dying := addPerm(gs, 0, "Grizzly Bears", "creature")

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"controller_seat": 0,
	})

	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsToken() && cardHasType(p.Card, "thopter") {
			t.Errorf("Pia Nalaar must require ARTIFACT creature dying, got Thopter")
		}
	}
}

func TestPiaNalaar_IgnoresTokenDeath(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Pia Nalaar, Consul of Revival", "creature", "legendary")
	// Token artifact creature dies — "nontoken" gate should block.
	dying := addPerm(gs, 0, "Thopter Token", "creature", "artifact", "token", "thopter")

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"controller_seat": 0,
	})

	thopters := 0
	for _, p := range gs.Seats[0].Battlefield {
		// Exclude the dying perm itself (it's still on the BF since
		// we didn't actually move it — fixture-level shortcut).
		if p == nil || p == dying || p.Card == nil {
			continue
		}
		if p.IsToken() && cardHasType(p.Card, "thopter") {
			thopters++
		}
	}
	if thopters != 0 {
		t.Errorf("Pia Nalaar must skip token deaths, got %d new Thopters", thopters)
	}
}

// -----------------------------------------------------------------------------
// Etali, Primal Storm
// -----------------------------------------------------------------------------

func TestEtali_AttackExilesTopOfEachLibrary(t *testing.T) {
	gs := newGame(t, 3)
	etali := addPerm(gs, 0, "Etali, Primal Storm", "creature", "legendary")
	// Library "top" = last index in this engine. Seed each seat.
	addLibrary(gs, 0, "Bottom0", "Top0")
	addLibrary(gs, 1, "Bottom1", "Top1")
	addLibrary(gs, 2, "Bottom2", "Top2")

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": etali,
		"attacker_seat": 0,
	})

	// CR §400.7c — each exiled card goes to its OWNER's exile zone
	// (the seat the library belonged to), not Etali's controller's
	// exile. Etali's free-cast grant references the cards by pointer,
	// so the cast pipeline still finds them regardless of which seat
	// holds the exile pile.
	for seat := 0; seat < 3; seat++ {
		if len(gs.Seats[seat].Exile) != 1 {
			t.Errorf("seat %d should hold its own exiled library top (CR §400.7c), got %d (events: %d)",
				seat, len(gs.Seats[seat].Exile), hasEvent(gs, "etali_exile"))
		}
	}
	// Each seat's library should have lost its top card.
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("seat 0 library should be 1, got %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[1].Library) != 1 {
		t.Errorf("seat 1 library should be 1, got %d", len(gs.Seats[1].Library))
	}
	if len(gs.Seats[2].Library) != 1 {
		t.Errorf("seat 2 library should be 1, got %d", len(gs.Seats[2].Library))
	}
	if hasEvent(gs, "etali_exile") != 3 {
		t.Errorf("expected 3 etali_exile events, got %d", hasEvent(gs, "etali_exile"))
	}
}

func TestEtali_RegistersGrantsOnNonlandsOnly(t *testing.T) {
	gs := newGame(t, 2)
	etali := addPerm(gs, 0, "Etali, Primal Storm", "creature", "legendary")

	// Seat 0's top is a nonland (instant); seat 1's top is a land.
	nonland := &gameengine.Card{Name: "Counterspell", Owner: 0, Types: []string{"instant"}}
	land := &gameengine.Card{Name: "Mountain", Owner: 1, Types: []string{"land", "basic"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, nonland)
	gs.Seats[1].Library = append(gs.Seats[1].Library, land)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": etali,
		"attacker_seat": 0,
	})

	// CR §400.7c — each card lands in its OWNER's exile: seat 0's
	// nonland → seat 0 exile; seat 1's land → seat 1 exile.
	if len(gs.Seats[0].Exile) != 1 {
		t.Errorf("seat 0 owner-exile should hold the nonland, got %d cards", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[1].Exile) != 1 {
		t.Errorf("seat 1 owner-exile should hold the land, got %d cards", len(gs.Seats[1].Exile))
	}
	// Only the nonland should have a ZoneCastGrant.
	if grant, ok := gs.ZoneCastGrants[nonland]; !ok || grant == nil {
		t.Errorf("expected free-cast grant on nonland Counterspell, got nil")
	}
	if grant, ok := gs.ZoneCastGrants[land]; ok && grant != nil {
		t.Errorf("must not register cast grant on land card, got %+v", grant)
	}
}

func TestEtali_OnlyFiresOnSelfAttack(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Etali, Primal Storm", "creature", "legendary")
	other := addPerm(gs, 0, "Goblin Token", "creature", "token")
	addLibrary(gs, 0, "Top0")
	addLibrary(gs, 1, "Top1")

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": 0,
	})

	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("Etali must only fire when HE attacks, got %d exiles", len(gs.Seats[0].Exile))
	}
}

// -----------------------------------------------------------------------------
// Registry smoke test — confirm all 5 handlers are wired
// -----------------------------------------------------------------------------

func TestBatchYR60_AllRegistered(t *testing.T) {
	cases := []struct {
		card  string
		event string
		ctx   map[string]interface{}
		setup func(*testing.T, *gameengine.GameState, *gameengine.Permanent)
	}{
		{
			card:  "Krenko, Tin Street Kingpin",
			event: "creature_attacks",
			setup: func(_ *testing.T, _ *gameengine.GameState, _ *gameengine.Permanent) {},
		},
		{
			card:  "Talrand, Sky Summoner",
			event: "instant_or_sorcery_cast",
			ctx:   map[string]interface{}{"card": &gameengine.Card{Name: "Bolt", Types: []string{"instant"}}},
			setup: func(_ *testing.T, _ *gameengine.GameState, _ *gameengine.Permanent) {},
		},
		{
			card:  "Sram, Senior Edificer",
			event: "spell_cast",
			ctx:   map[string]interface{}{"card": &gameengine.Card{Name: "Bear Umbra", Types: []string{"aura"}}},
			setup: func(_ *testing.T, gs *gameengine.GameState, _ *gameengine.Permanent) {
				addLibrary(gs, 0, "X")
			},
		},
		{
			card:  "Etali, Primal Storm",
			event: "creature_attacks",
			setup: func(_ *testing.T, gs *gameengine.GameState, _ *gameengine.Permanent) {
				addLibrary(gs, 0, "X")
				addLibrary(gs, 1, "Y")
			},
		},
	}
	for _, c := range cases {
		gs := newGame(t, 2)
		commander := addPerm(gs, 0, c.card, "creature", "legendary")
		c.setup(t, gs, commander)
		before := len(gs.EventLog)
		ctx := map[string]interface{}{"caster_seat": 0, "attacker_perm": commander, "attacker_seat": 0}
		for k, v := range c.ctx {
			ctx[k] = v
		}
		gameengine.FireCardTrigger(gs, c.event, ctx)
		if len(gs.EventLog) <= before {
			t.Errorf("%s: no events fired on %q (before=%d after=%d)",
				c.card, c.event, before, len(gs.EventLog))
		}
	}
	// Pia Nalaar — needs a dying perm in ctx.
	gs := newGame(t, 2)
	addPerm(gs, 0, "Pia Nalaar, Consul of Revival", "creature", "legendary")
	dying := addPerm(gs, 0, "Steel Overseer", "creature", "artifact")
	before := len(gs.EventLog)
	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            dying,
		"controller_seat": 0,
	})
	if len(gs.EventLog) <= before {
		t.Errorf("Pia Nalaar: no events fired (before=%d after=%d)", before, len(gs.EventLog))
	}
}
