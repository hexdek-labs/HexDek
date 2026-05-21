package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// dev/percard-stubs-batchF-r50 — 10 stub ports to real handlers.
// One regression test per port.
// ---------------------------------------------------------------------------

// 1. Rakdos, Lord of Riots — cast restriction CostModMinimum locks
//    Rakdos as uncastable until any opponent has lost life this turn.
func TestStubsBatchF_RakdosLordOfRiots_CastLockedUntilOppLifeLost(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	rakdos := &gameengine.Card{Name: "Rakdos, Lord of Riots", Types: []string{"creature", "legendary", "cost:5"}}
	// No opponent has lost life → cost should be floored at 999.
	cost := gameengine.CalculateTotalCost(gs, rakdos, 0)
	if cost < 999 {
		t.Errorf("expected cost floor >= 999 when no opp lost life, got %d", cost)
	}
	// Opponent loses 1 life → restriction lifts; cost reduces by 1 (the
	// existing cost reduction wires the {1}-less-per-life-lost half).
	gs.Seats[1].Turn.LifeLost = 1
	cost = gameengine.CalculateTotalCost(gs, rakdos, 0)
	if cost >= 999 {
		t.Errorf("expected cost < 999 once opp lost life, got %d", cost)
	}
}

// 2. Hamza, Guardian of Arashin — self-cast reduction in command zone
//    (and on first cast from hand) per command-zone-style scan.
func TestStubsBatchF_HamzaGuardianOfArashin_SelfCastReducesByPlusOnePlusOneCreatureCount(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	// Two creatures with +1/+1 counters under seat 0.
	for i := 0; i < 2; i++ {
		p := &gameengine.Permanent{
			Card:       &gameengine.Card{Name: "Bear", Types: []string{"creature"}},
			Controller: 0,
			Owner:      0,
			Counters:   map[string]int{"+1/+1": 1},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	}
	// Hamza is in command zone (or hand) — not yet on battlefield.
	hamza := &gameengine.Card{Name: "Hamza, Guardian of Arashin", Types: []string{"creature", "legendary", "cost:5"}}
	cost := gameengine.CalculateTotalCost(gs, hamza, 0)
	// Base 5 - 2 (creatures with counters) = 3.
	if cost != 3 {
		t.Errorf("expected cost 3 (5 - 2 +1/+1 creatures), got %d", cost)
	}
}

// 3. Wilson, Refined Grizzly — OnCast stamps cannot_be_countered on
//    the StackItem's CostMeta.
func TestStubsBatchF_WilsonRefinedGrizzly_OnCastStampsCannotBeCountered(t *testing.T) {
	gs := newGame(t, 2)
	card := &gameengine.Card{Name: "Wilson, Refined Grizzly", Types: []string{"creature", "legendary"}}
	item := &gameengine.StackItem{Controller: 0, Card: card}
	wilsonRefinedGrizzlyCast(gs, item)
	if item.CostMeta == nil {
		t.Fatalf("expected CostMeta populated")
	}
	val, ok := item.CostMeta["cannot_be_countered"].(bool)
	if !ok || !val {
		t.Errorf("expected cannot_be_countered=true, got %v", item.CostMeta["cannot_be_countered"])
	}
}

// 4. Norman Osborn — ETB stamps unblockable flag.
func TestStubsBatchF_NormanOsborn_ETBStampsUnblockable(t *testing.T) {
	gs := newGame(t, 2)
	norman := addPerm(gs, 0, "Norman Osborn // Green Goblin", "creature", "legendary")
	gameengine.InvokeETBHook(gs, norman)
	if norman.Flags["unblockable"] != 1 {
		t.Errorf("expected unblockable flag set, got %v", norman.Flags)
	}
}

// 5. Caradora, Heart of Alacria — +1/+1 counter replacement adds one
//    extra counter when 1+ would be placed on a creature/Vehicle the
//    controller controls.
func TestStubsBatchF_Caradora_PlusOneCounterReplacement(t *testing.T) {
	gs := newGame(t, 2)
	caradora := addPerm(gs, 0, "Caradora, Heart of Alacria", "creature", "legendary")
	gameengine.InvokeETBHook(gs, caradora)
	// Now fire a would_put_counter event for a friendly creature with
	// 2 +1/+1 counters incoming.
	bear := addPerm(gs, 0, "Bear", "creature")
	got, _ := gameengine.FirePutCounterEvent(gs, bear, "+1/+1", 2, caradora)
	if got != 3 {
		t.Errorf("expected count 3 after +1 replacement (2+1), got %d", got)
	}
	// Opponent's creature should NOT be affected.
	opp := addPerm(gs, 1, "Foe", "creature")
	got, _ = gameengine.FirePutCounterEvent(gs, opp, "+1/+1", 2, caradora)
	if got != 2 {
		t.Errorf("expected opponent's count unchanged (2), got %d", got)
	}
}

// 6. Lara Croft, Tomb Raider — attack trigger exiles a legendary
//    artifact card from a graveyard and tags it with discovery_counter.
func TestStubsBatchF_LaraCroft_AttackExilesLegendaryArtifactWithDiscoveryTag(t *testing.T) {
	gs := newGame(t, 2)
	lara := addPerm(gs, 0, "Lara Croft, Tomb Raider", "creature", "legendary")
	lara.Flags["attacking"] = 1
	// Seat 1's graveyard contains a legendary artifact (highest cmc).
	relic := &gameengine.Card{Name: "Ancient Relic", Types: []string{"artifact", "legendary", "cmc:6"}}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, relic,
		&gameengine.Card{Name: "Plain Bear", Types: []string{"creature", "cmc:8"}}, // not legendary artifact
	)
	// Also a legendary land in our own yard (lower cmc).
	myLand := &gameengine.Card{Name: "Sacred Site", Types: []string{"land", "legendary", "cmc:0"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, myLand)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": lara,
	})

	// Relic (cmc 6) should be in exile, tagged.
	inExile := false
	for _, c := range gs.Seats[1].Exile {
		if c == relic {
			inExile = true
		}
	}
	if !inExile {
		t.Errorf("expected Ancient Relic moved to seat 1 exile; exile=%v", cardNames(gs.Seats[1].Exile))
	}
	hasTag := false
	for _, t := range relic.Types {
		if t == "discovery_counter" {
			hasTag = true
		}
	}
	if !hasTag {
		t.Errorf("expected discovery_counter tag on Ancient Relic, got types=%v", relic.Types)
	}
}

// 7. The Swarmweaver — Delirium anthem applied when ≥4 distinct card
//    types in graveyard.
func TestStubsBatchF_TheSwarmweaver_DeliriumAnthemAppliesAtFourTypes(t *testing.T) {
	gs := newGame(t, 2)
	weaver := addPerm(gs, 0, "The Swarmweaver", "creature", "legendary", "artifact")
	insect := addPerm(gs, 0, "Friendly Insect", "creature", "insect")
	// Yard with 4 distinct types: creature, artifact, instant, sorcery.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Dead Bear", Types: []string{"creature"}},
		&gameengine.Card{Name: "Dead Bomb", Types: []string{"artifact"}},
		&gameengine.Card{Name: "Dead Bolt", Types: []string{"instant"}},
		&gameengine.Card{Name: "Dead Wrath", Types: []string{"sorcery"}},
	)
	gameengine.InvokeETBHook(gs, weaver)
	if insect.Flags["swarmweaver_anthem"] != 1 {
		t.Errorf("expected delirium anthem on friendly insect, got flags=%v", insect.Flags)
	}
	if insect.Flags["kw:deathtouch"] != 1 {
		t.Errorf("expected kw:deathtouch granted, got %v", insect.Flags)
	}
}

func TestStubsBatchF_TheSwarmweaver_DeliriumOffBelowFour(t *testing.T) {
	gs := newGame(t, 2)
	weaver := addPerm(gs, 0, "The Swarmweaver", "creature", "legendary", "artifact")
	insect := addPerm(gs, 0, "Friendly Insect", "creature", "insect")
	// Only 2 distinct types in yard.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Dead Bear", Types: []string{"creature"}},
		&gameengine.Card{Name: "Dead Bomb", Types: []string{"artifact"}},
	)
	gameengine.InvokeETBHook(gs, weaver)
	if insect.Flags["swarmweaver_anthem"] != 0 {
		t.Errorf("expected no anthem when types<4, got %v", insect.Flags)
	}
}

// 8. Ozai, the Phoenix King — combat_begin re-evaluates the
//    conditional flying/indestructible flags based on current mana pool
//    (without batch F's trigger, only upkeep_controller did the check,
//    leaving mid-turn mana spends unaccounted for).
func TestStubsBatchF_OzaiPhoenixKing_CombatBeginRechecksConditionalKW(t *testing.T) {
	gs := newGame(t, 2)
	ozai := addPerm(gs, 0, "Ozai, the Phoenix King", "creature", "legendary")
	ozai.Card.BasePower = 5
	ozai.Card.BaseToughness = 5
	gs.Seats[0].ManaPool = 0
	gameengine.InvokeETBHook(gs, ozai)
	if ozai.Flags["kw:flying"] != 0 || ozai.Flags["kw:indestructible"] != 0 {
		t.Errorf("expected no flying/indestructible at 0 mana, got %v", ozai.Flags)
	}
	// Player floats 7 mana mid-turn; combat_begin should now see them.
	gs.Seats[0].ManaPool = 7
	gameengine.FireCardTrigger(gs, "combat_begin", map[string]interface{}{
		"active_seat": 0,
	})
	if ozai.Flags["kw:flying"] != 1 || ozai.Flags["kw:indestructible"] != 1 {
		t.Errorf("expected flying/indestructible at >=6 mana, got %v", ozai.Flags)
	}
	// End step drains pool; flags should drop.
	gs.Seats[0].ManaPool = 0
	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})
	if ozai.Flags["kw:flying"] != 0 || ozai.Flags["kw:indestructible"] != 0 {
		t.Errorf("expected flying/indestructible cleared at <6 mana after end_step, got %v", ozai.Flags)
	}
}

// 9. Tannuk, Steadfast Second — LTB cleanup strips the anthem-granted
//    kw:haste from other creatures (when no Tannuk remains). The
//    permanent_ltb trigger is fired with Tannuk still on the
//    battlefield (registry lookup needs to find him there); the
//    cleanup handler reads ctx["perm"] for the leaving permanent.
func TestStubsBatchF_Tannuk_LTBCleanupStripsAnthemHaste(t *testing.T) {
	gs := newGame(t, 2)
	tannuk := addPerm(gs, 0, "Tannuk, Steadfast Second", "creature", "legendary")
	other := addPerm(gs, 0, "Friend", "creature")
	other.Flags["kw:haste"] = 1

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm": tannuk,
	})

	if other.Flags["kw:haste"] != 0 {
		t.Errorf("expected anthem haste cleared on LTB, got %v", other.Flags)
	}
}

// 10. Kolodin, Triumph Caster — end_step sweep clears "until end of
//     turn" flags from Mounts/Vehicles and strips the transient
//     "creature_until_eot" type tag.
func TestStubsBatchF_Kolodin_EndStepSweepClearsUntilEOTFlagsAndTags(t *testing.T) {
	gs := newGame(t, 2)
	kolodin := addPerm(gs, 0, "Kolodin, Triumph Caster", "creature", "legendary")
	vehicle := addPerm(gs, 0, "Truck", "artifact", "vehicle")
	vehicle.Flags["kw:artifact_creature_until_eot"] = 1
	vehicle.Card.Types = append(vehicle.Card.Types, "creature_until_eot")

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})
	_ = kolodin

	if vehicle.Flags["kw:artifact_creature_until_eot"] != 0 {
		t.Errorf("expected vehicle's until-EOT flag cleared, got %v", vehicle.Flags)
	}
	for _, tag := range vehicle.Card.Types {
		if tag == "creature_until_eot" {
			t.Errorf("expected creature_until_eot tag removed, got types=%v", vehicle.Card.Types)
		}
	}
}

