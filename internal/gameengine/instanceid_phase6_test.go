package gameengine

// InstanceID Phase 6 — subsystem activation registry tests.
//
// Each subtest builds a fresh GameState (which seeds the ten dormant
// hooks via NewGameState), constructs a card carrying the activation
// keyword/oracle phrase, and asserts that FireZoneChangeTriggers wakes
// the right subsystem on the right seat without disturbing other
// hooks. Per docs/instanceid-system-v2-r60.md §13 Phase 6 spec.

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// freshPhase6Game builds a 4-seat game state with all subsystem hooks
// installed and dormant. Returns the state for the test to populate.
func freshPhase6Game(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(60260))
	gs := NewGameState(4, rng, nil)
	if len(gs.SubsystemHooks) != 10 {
		t.Fatalf("expected 10 dormant hooks after NewGameState, got %d", len(gs.SubsystemHooks))
	}
	for _, h := range gs.SubsystemHooks {
		if h == nil {
			t.Fatal("nil hook entry in SubsystemHooks")
		}
		if h.Active {
			t.Fatalf("hook %s pre-activated", h.Subsystem)
		}
	}
	return gs
}

// phase6Card builds a Card with the given name + oracle text + type
// line. The oracle text is wrapped into a Static Raw so that the
// engine's OracleTextLower accessor surfaces it for predicate matching.
func phase6Card(name string, owner int, oracle string, typeLine string) *Card {
	ast := &gameast.CardAST{Name: name, Abilities: []gameast.Ability{
		&gameast.Static{Raw: oracle},
	}}
	return &Card{
		AST:      ast,
		Name:     name,
		Owner:    owner,
		TypeLine: typeLine,
	}
}

// hookByName fetches a hook by subsystem identity. Convenience for the
// per-subtest assertion calls.
func hookByName(gs *GameState, s Subsystem) *DormantHook {
	for _, h := range gs.SubsystemHooks {
		if h != nil && h.Subsystem == s {
			return h
		}
	}
	return nil
}

// activeSubsystems returns the set of awake subsystem names.
func activeSubsystems(gs *GameState) []string {
	out := make([]string, 0)
	for _, h := range gs.SubsystemHooks {
		if h != nil && h.Active {
			out = append(out, h.Subsystem.String())
		}
	}
	return out
}

// ---------------------------------------------------------------------
// Dormancy
// ---------------------------------------------------------------------

func TestPhase6_DormantOnUnrelatedCard(t *testing.T) {
	gs := freshPhase6Game(t)
	// A vanilla creature with no subsystem hooks in its text.
	card := phase6Card("Grizzly Bears", 0, "vanilla 2/2", "Creature — Bear")
	FireZoneChangeTriggers(gs, nil, card, "hand", "battlefield")
	if got := activeSubsystems(gs); len(got) != 0 {
		t.Fatalf("vanilla card woke subsystems: %v", got)
	}
	if gs.Seats[0].MonarchActive || gs.Seats[0].DayNightActive || gs.Seats[0].AscendActive {
		t.Fatalf("unrelated card flipped a per-seat activation flag")
	}
}

// ---------------------------------------------------------------------
// Monarch — Court of Vantress, the spec's named example.
// ---------------------------------------------------------------------

func TestPhase6_CourtOfVantressWakesMonarch(t *testing.T) {
	gs := freshPhase6Game(t)
	court := phase6Card(
		"Court of Vantress",
		2,
		"When this enchantment enters, you become the monarch.",
		"Enchantment",
	)
	FireZoneChangeTriggers(gs, nil, court, "hand", "battlefield")

	monarch := hookByName(gs, SubsystemMonarch)
	if monarch == nil || !monarch.Active {
		t.Fatal("Monarch hook did not activate")
	}
	if !gs.Seats[2].MonarchActive {
		t.Fatal("Seat 2 MonarchActive flag did not flip")
	}
	for i, s := range gs.Seats {
		if i == 2 {
			continue
		}
		if s.MonarchActive {
			t.Fatalf("Seat %d MonarchActive flipped — should be scoped to controller seat", i)
		}
	}

	// Activation event must be in the log.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "subsystem_activated" {
			if details, ok := ev.Details["subsystem"].(string); ok && details == "Monarch" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("subsystem_activated/Monarch event missing from event log")
	}
}

// ---------------------------------------------------------------------
// Day/Night — daybound keyword (game-wide flag, mirrored to all seats).
// ---------------------------------------------------------------------

func TestPhase6_DayboundWakesDayNight(t *testing.T) {
	gs := freshPhase6Game(t)
	werewolf := phase6Card(
		"Reckless Stormseeker",
		1,
		"Daybound (If a player casts no spells during their own turn, it becomes night next turn.)",
		"Creature — Human Werewolf",
	)
	FireZoneChangeTriggers(gs, nil, werewolf, "hand", "battlefield")

	dn := hookByName(gs, SubsystemDayNight)
	if dn == nil || !dn.Active {
		t.Fatal("DayNight hook did not activate on daybound card")
	}
	for i, s := range gs.Seats {
		if !s.DayNightActive {
			t.Fatalf("Seat %d DayNightActive not flipped — should be game-wide", i)
		}
	}
	// Other subsystems should remain dormant.
	if hookByName(gs, SubsystemMonarch).Active {
		t.Fatal("daybound card spuriously woke Monarch")
	}
}

// ---------------------------------------------------------------------
// Ascend (and CityBlessing under the 10-permanent gate).
// ---------------------------------------------------------------------

func TestPhase6_AscendWakesAscendAndCityBlessing(t *testing.T) {
	gs := freshPhase6Game(t)

	// Seat 3 already commands 10 permanents — meets the city's-blessing
	// threshold so the CityBlessing predicate gates open at activation.
	for i := 0; i < 10; i++ {
		filler := phase6Card("Filler Land", 3, "vanilla", "Land")
		gs.Seats[3].Battlefield = append(gs.Seats[3].Battlefield, &Permanent{
			Card:       filler,
			Controller: 3,
			Owner:      3,
		})
	}

	ascender := phase6Card(
		"Snubhorn Sentry",
		3,
		"Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.)",
		"Creature — Dinosaur",
	)
	FireZoneChangeTriggers(gs, nil, ascender, "hand", "battlefield")

	if !hookByName(gs, SubsystemAscend).Active {
		t.Fatal("Ascend hook did not activate")
	}
	if !gs.Seats[3].AscendActive {
		t.Fatal("Seat 3 AscendActive not flipped")
	}
	if !hookByName(gs, SubsystemCityBlessing).Active {
		t.Fatal("CityBlessing hook did not activate at 10+ permanents with Ascend")
	}
	if !gs.Seats[3].HasCityBlessing {
		t.Fatal("Seat 3 HasCityBlessing not flipped")
	}
}

func TestPhase6_AscendWithoutThresholdLeavesCityBlessingDormant(t *testing.T) {
	gs := freshPhase6Game(t)

	ascender := phase6Card(
		"Skymarcher Aspirant",
		0,
		"Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.)",
		"Creature — Vampire Soldier",
	)
	FireZoneChangeTriggers(gs, nil, ascender, "hand", "battlefield")

	if !hookByName(gs, SubsystemAscend).Active {
		t.Fatal("Ascend hook did not activate")
	}
	if hookByName(gs, SubsystemCityBlessing).Active {
		t.Fatal("CityBlessing hook activated below 10-permanent threshold")
	}
	if gs.Seats[0].HasCityBlessing {
		t.Fatal("HasCityBlessing flipped without meeting threshold")
	}
}

// ---------------------------------------------------------------------
// Foretell — exiling a face-down card from hand populates ForetellExile.
// ---------------------------------------------------------------------

func TestPhase6_ForetellExilePopulatesBucket(t *testing.T) {
	gs := freshPhase6Game(t)

	card := phase6Card(
		"Behold the Multiverse",
		1,
		"Foretell {1}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.)",
		"Instant",
	)
	card.FaceDown = true

	FireZoneChangeTriggers(gs, nil, card, "hand", "exile")

	if !hookByName(gs, SubsystemForetell).Active {
		t.Fatal("Foretell hook did not activate")
	}
	if len(gs.Seats[1].ForetellExile) != 1 {
		t.Fatalf("ForetellExile not populated, got %d entries", len(gs.Seats[1].ForetellExile))
	}
	if gs.Seats[1].ForetellExile[0] != card {
		t.Fatal("ForetellExile entry does not point at the foretold card")
	}
}

// ---------------------------------------------------------------------
// Remaining subsystems — single-shot smoke coverage so the wiring is
// pinned for every hook entry.
// ---------------------------------------------------------------------

func TestPhase6_InitiativeWakeup(t *testing.T) {
	gs := freshPhase6Game(t)
	card := phase6Card(
		"White Plume Adventurer",
		0,
		"When this creature enters, you take the initiative.",
		"Creature — Orc Cleric",
	)
	FireZoneChangeTriggers(gs, nil, card, "hand", "battlefield")
	if !hookByName(gs, SubsystemInitiative).Active {
		t.Fatal("Initiative hook did not activate")
	}
	if !gs.Seats[0].InitiativeHolder {
		t.Fatal("InitiativeHolder not flipped on seat 0")
	}
}

func TestPhase6_DungeonWakeup(t *testing.T) {
	gs := freshPhase6Game(t)
	card := phase6Card(
		"Nadaar, Selfless Paladin",
		2,
		"Whenever Nadaar enters or attacks, venture into the dungeon.",
		"Legendary Creature — Dragon Knight",
	)
	FireZoneChangeTriggers(gs, nil, card, "hand", "battlefield")
	if !hookByName(gs, SubsystemDungeon).Active {
		t.Fatal("Dungeon hook did not activate")
	}
	if gs.Seats[2].Flags["dungeon_subsystem_active"] != 1 {
		t.Fatal("dungeon_subsystem_active flag not set")
	}
}

func TestPhase6_RingTemptsWakeup(t *testing.T) {
	gs := freshPhase6Game(t)
	card := phase6Card(
		"Frodo Baggins",
		3,
		"Whenever Frodo Baggins or another legendary creature you control enters, the Ring tempts you.",
		"Legendary Creature — Halfling Scout",
	)
	FireZoneChangeTriggers(gs, nil, card, "hand", "battlefield")
	if !hookByName(gs, SubsystemRingTempts).Active {
		t.Fatal("RingTempts hook did not activate")
	}
}

func TestPhase6_EnergyWakeup(t *testing.T) {
	gs := freshPhase6Game(t)
	card := phase6Card(
		"Aether Hub",
		1,
		"When this land enters, you get {E} (an energy counter).",
		"Land",
	)
	FireZoneChangeTriggers(gs, nil, card, "hand", "battlefield")
	if !hookByName(gs, SubsystemEnergy).Active {
		t.Fatal("Energy hook did not activate")
	}
}

func TestPhase6_ExperienceWakeup(t *testing.T) {
	gs := freshPhase6Game(t)
	card := phase6Card(
		"Meren of Clan Nel Toth",
		0,
		"Whenever another creature you control dies, you get an experience counter.",
		"Legendary Creature — Human Shaman",
	)
	FireZoneChangeTriggers(gs, nil, card, "hand", "battlefield")
	if !hookByName(gs, SubsystemExperience).Active {
		t.Fatal("Experience hook did not activate")
	}
}

// ---------------------------------------------------------------------
// Idempotency — once active, a hook stays active without re-firing
// OnActivate or emitting duplicate audit events.
// ---------------------------------------------------------------------

func TestPhase6_HookStaysActiveAndDoesNotReFire(t *testing.T) {
	gs := freshPhase6Game(t)
	court := phase6Card("Court of Ambition", 2, "When this enchantment enters, you become the monarch.", "Enchantment")
	FireZoneChangeTriggers(gs, nil, court, "hand", "battlefield")
	FireZoneChangeTriggers(gs, nil, court, "battlefield", "graveyard")
	FireZoneChangeTriggers(gs, nil, court, "graveyard", "hand")

	monarchActivations := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "subsystem_activated" {
			if name, _ := ev.Details["subsystem"].(string); name == "Monarch" {
				monarchActivations++
			}
		}
	}
	if monarchActivations != 1 {
		t.Fatalf("Monarch activation event fired %d times, want 1", monarchActivations)
	}
	if !hookByName(gs, SubsystemMonarch).Active {
		t.Fatal("Monarch hook lost Active flag after subsequent zone changes")
	}
}

// ---------------------------------------------------------------------
// Predicate negative coverage — pure consumers ("pay {E}", "if you have
// the city's blessing") must NOT wake their subsystems.
// ---------------------------------------------------------------------

func TestPhase6_PureEnergyConsumerDoesNotWakeEnergy(t *testing.T) {
	gs := freshPhase6Game(t)
	card := phase6Card(
		"Energy Spender",
		0,
		"{T}: Pay {E}{E}. If you do, draw a card.",
		"Artifact",
	)
	FireZoneChangeTriggers(gs, nil, card, "hand", "battlefield")
	if hookByName(gs, SubsystemEnergy).Active {
		t.Fatal("Pure consumer woke Energy subsystem")
	}
}

// ---------------------------------------------------------------------
// String accessor sanity (Subsystem.String covers all ten entries).
// ---------------------------------------------------------------------

func TestPhase6_SubsystemStringExhaustive(t *testing.T) {
	want := []string{
		"DayNight", "Monarch", "Initiative", "Ascend", "Dungeon",
		"RingTempts", "Energy", "Experience", "Foretell", "CityBlessing",
	}
	for i, w := range want {
		got := Subsystem(i).String()
		if got != w {
			t.Fatalf("Subsystem(%d).String() = %q, want %q", i, got, w)
		}
	}
	if !strings.Contains(Subsystem(99).String(), "Unknown") {
		t.Fatal("out-of-range Subsystem should render as Unknown")
	}
}
