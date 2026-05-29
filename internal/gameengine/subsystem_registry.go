// Package gameengine — InstanceID Phase 6 subsystem activation registry.
//
// Per docs/instanceid-system-v2-r60.md §4.4 + §9 + §13 and the Probe D
// audit at docs/subsystem-activation-audit-r60.md, ten optional MTG
// subsystems (Day/Night, Monarch, Initiative, Ascend+CityBlessing,
// Dungeons, Ring tempts, Energy, Experience, Foretell) sit dormant in
// every game until the first card that interacts with them touches a
// zone. Once awoken, the subsystem stays awake for the rest of the game
// (sticky per CR).
//
// The registry is consulted on every card-zone-change event. The cost
// of staying dormant is one slice walk + ten predicate calls per zone
// change; the cost of waking is a single per-seat flag flip plus an
// audit-log event. Hot-path consumers (per_card handlers, SBAs) read
// Seat.MonarchActive / DayNightActive / etc. — cheap booleans — rather
// than re-scanning oracle text.
//
// The 484 activator cards classified in Probe D are wired by predicate,
// not per-card stub — each predicate matches the printed-oracle shape
// or keyword that Probe D identified as the activation source.

package gameengine

import (
	"strings"
)

// Dungeon represents the dungeon a seat is currently venturing in
// (CR §309). Phase 6 only needs the type's identity for Seat.CurrentDungeon;
// the venture/room state machine is owned by the per_card dungeon
// handlers and lives on subsequent phases. Keeping the struct here
// avoids a forward-declaration import cycle and makes the Phase 6 hook
// self-contained.
type Dungeon struct {
	Name        string
	CurrentRoom string
	Completed   bool
}

// Subsystem enumerates the ten optional MTG mechanics that the engine
// keeps dormant until a card-zone-change event wakes them. The ordering
// is stable (used as slice indices in Game.SubsystemHooks).
type Subsystem int

const (
	SubsystemDayNight Subsystem = iota
	SubsystemMonarch
	SubsystemInitiative
	SubsystemAscend
	SubsystemDungeon
	SubsystemRingTempts
	SubsystemEnergy
	SubsystemExperience
	SubsystemForetell
	SubsystemCityBlessing
)

// String renders the subsystem name for audit-log events and tests.
func (s Subsystem) String() string {
	switch s {
	case SubsystemDayNight:
		return "DayNight"
	case SubsystemMonarch:
		return "Monarch"
	case SubsystemInitiative:
		return "Initiative"
	case SubsystemAscend:
		return "Ascend"
	case SubsystemDungeon:
		return "Dungeon"
	case SubsystemRingTempts:
		return "RingTempts"
	case SubsystemEnergy:
		return "Energy"
	case SubsystemExperience:
		return "Experience"
	case SubsystemForetell:
		return "Foretell"
	case SubsystemCityBlessing:
		return "CityBlessing"
	}
	return "Unknown"
}

// SubsystemEvent carries the zone-change context that DormantHook
// predicates inspect. Mirrors the EventSpec shape from
// docs/instanceid-system-v2-r60.md §9; concrete enough for the predicate
// helpers below to make wake/no-wake decisions without touching the
// full event log.
type SubsystemEvent struct {
	Card     *Card
	Seat     int    // seat index that owns the activation event (controller or owner)
	FromZone string // empty when the event is a static-resolve activation
	ToZone   string
}

// DormantHook is the per-subsystem activation slot. Active flips from
// false to true exactly once per game; OnActivate runs at the flip
// edge. Subsystem-state mutation (flipping the per-seat bool, seeding
// SBAs, etc.) lives in OnActivate.
type DormantHook struct {
	Subsystem  Subsystem
	Active     bool
	Predicate  func(gs *GameState, ev SubsystemEvent) bool
	OnActivate func(gs *GameState, ev SubsystemEvent)
}

// RegisterSubsystemHooks seeds the ten dormant hooks on a fresh
// GameState. Idempotent — repeat calls are no-ops once the slice is
// populated. Called from NewGameState so callers don't have to remember.
func RegisterSubsystemHooks(gs *GameState) {
	if gs == nil || len(gs.SubsystemHooks) > 0 {
		return
	}
	gs.SubsystemHooks = []*DormantHook{
		{Subsystem: SubsystemDayNight, Predicate: predicateDayNight, OnActivate: activateDayNight},
		{Subsystem: SubsystemMonarch, Predicate: predicateMonarch, OnActivate: activateMonarch},
		{Subsystem: SubsystemInitiative, Predicate: predicateInitiative, OnActivate: activateInitiative},
		{Subsystem: SubsystemAscend, Predicate: predicateAscend, OnActivate: activateAscend},
		{Subsystem: SubsystemDungeon, Predicate: predicateDungeon, OnActivate: activateDungeon},
		{Subsystem: SubsystemRingTempts, Predicate: predicateRingTempts, OnActivate: activateRingTempts},
		{Subsystem: SubsystemEnergy, Predicate: predicateEnergy, OnActivate: activateEnergy},
		{Subsystem: SubsystemExperience, Predicate: predicateExperience, OnActivate: activateExperience},
		{Subsystem: SubsystemForetell, Predicate: predicateForetell, OnActivate: activateForetell},
		{Subsystem: SubsystemCityBlessing, Predicate: predicateCityBlessing, OnActivate: activateCityBlessing},
	}
}

// CheckSubsystemActivation walks the dormant-hook registry and wakes
// any hook whose predicate matches the given zone-change event. Safe
// to call on every card movement — dormant hooks short-circuit at the
// predicate; awake hooks short-circuit at the Active flag.
//
// Per docs/instanceid-system-v2-r60.md §9: the activation event runs
// before the rest of the zone-change observer chain so per_card
// handlers that depend on Seat.MonarchActive / DayNightActive observe
// the awakened state on the same tick the wake-up card resolved.
func CheckSubsystemActivation(gs *GameState, ev SubsystemEvent) {
	if gs == nil || ev.Card == nil {
		return
	}
	if len(gs.SubsystemHooks) == 0 {
		RegisterSubsystemHooks(gs)
	}
	for _, h := range gs.SubsystemHooks {
		if h == nil || h.Active {
			continue
		}
		if h.Predicate == nil || !h.Predicate(gs, ev) {
			continue
		}
		h.Active = true
		if h.OnActivate != nil {
			h.OnActivate(gs, ev)
		}
		gs.LogEvent(Event{
			Kind:   "subsystem_activated",
			Seat:   ev.Seat,
			Target: -1,
			Source: ev.Card.DisplayName(),
			Details: map[string]interface{}{
				"subsystem": h.Subsystem.String(),
				"from_zone": ev.FromZone,
				"to_zone":   ev.ToZone,
			},
		})
	}
}

// ----------------------------------------------------------------------
// Predicates — one per subsystem, matched against Probe D's oracle
// patterns. All predicates take the lowercased oracle text once to
// keep the per-event cost bounded.
// ----------------------------------------------------------------------

func predicateDayNight(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	// Daybound/Nightbound keyword OR explicit "becomes day/night" line
	// (Brimstone Vandal ETB-replacement, Tovolar upkeep trigger,
	// Into the Night / Unnatural Moonrise sorceries).
	if strings.Contains(text, "daybound") || strings.Contains(text, "nightbound") {
		return true
	}
	if strings.Contains(text, "becomes day") || strings.Contains(text, "becomes night") {
		return true
	}
	if strings.Contains(text, "it is day") || strings.Contains(text, "it is night") {
		return true
	}
	return false
}

func predicateMonarch(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	// "you become the monarch" / "target player becomes the monarch" /
	// "that player becomes the monarch" — all flagged in Probe D.
	return strings.Contains(text, "become the monarch") ||
		strings.Contains(text, "becomes the monarch")
}

func predicateInitiative(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	return strings.Contains(text, "take the initiative") ||
		strings.Contains(text, "takes the initiative")
}

func predicateAscend(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	// The printed Ascend keyword line. Per Probe D the city's-blessing
	// subsystem has no independent activator surface — it rides on top
	// of Ascend. Consumers that read "if you have the city's blessing"
	// without printing Ascend (Tendershoot Dryad's body, Slippery
	// Scoundrel's hexproof rider) live on cards that ALSO print Ascend,
	// so the keyword check is sufficient.
	if strings.Contains(text, "ascend ") || strings.HasSuffix(text, "ascend") {
		return true
	}
	// Defensive: bare "ascend\n" without trailing space.
	return strings.Contains(text, "\nascend") || strings.Contains(text, "ascend\n")
}

func predicateDungeon(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	return strings.Contains(text, "venture into the dungeon") ||
		strings.Contains(text, "venture into the undercity")
}

func predicateRingTempts(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	return strings.Contains(text, "the ring tempts you")
}

func predicateEnergy(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	// {E} producers are the activators; pure consumers (`pay {E}`)
	// don't wake the subsystem per Probe D's "you get {E}" criterion.
	// "energy counter" / "energy counters" catches the keyword body.
	if strings.Contains(text, "you get {e}") {
		return true
	}
	if strings.Contains(text, "energy counter") {
		return true
	}
	return false
}

func predicateExperience(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	// "you get an experience counter" / "you get N experience counters" —
	// granting sources wake the subsystem; tracking-only readers don't.
	return strings.Contains(text, "experience counter")
}

func predicateForetell(gs *GameState, ev SubsystemEvent) bool {
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	if strings.Contains(text, "foretell") {
		return true
	}
	return false
}

func predicateCityBlessing(gs *GameState, ev SubsystemEvent) bool {
	// CityBlessing fires alongside Ascend at activation time, but it
	// only flips the per-seat HasCityBlessing flag once the controller
	// already commands 10+ permanents (CR §702.131). We still register
	// the hook so it can fire AFTER Ascend wakes — the predicate gate
	// is the 10-permanent threshold, scoped to the activating seat.
	text := OracleTextLower(ev.Card)
	if text == "" {
		return false
	}
	if !strings.Contains(text, "ascend") && !strings.Contains(text, "city's blessing") {
		return false
	}
	if ev.Seat < 0 || ev.Seat >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[ev.Seat]
	if seat == nil {
		return false
	}
	return len(seat.Battlefield) >= 10
}

// ----------------------------------------------------------------------
// OnActivate callbacks — flip the sticky bool / seed per-seat resource
// pools. Most are one-liners; the structured shape is kept so future
// per-subsystem bookkeeping (SBA registration, replacement-effect
// install) has an obvious home.
// ----------------------------------------------------------------------

func activateDayNight(gs *GameState, ev SubsystemEvent) {
	// Day/Night is game-wide per CR §726.3, so the flag is propagated
	// across all seats. Hot-path readers can then check any seat's
	// DayNightActive cheaply.
	for _, s := range gs.Seats {
		if s != nil {
			s.DayNightActive = true
		}
	}
}

func activateMonarch(gs *GameState, ev SubsystemEvent) {
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		gs.Seats[ev.Seat].MonarchActive = true
	}
}

func activateInitiative(gs *GameState, ev SubsystemEvent) {
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		gs.Seats[ev.Seat].InitiativeHolder = true
	}
}

func activateAscend(gs *GameState, ev SubsystemEvent) {
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		gs.Seats[ev.Seat].AscendActive = true
	}
}

func activateDungeon(gs *GameState, ev SubsystemEvent) {
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		seat := gs.Seats[ev.Seat]
		// CurrentDungeon stays nil until the seat actually ventures
		// (the per_card venture handlers populate it); the activation
		// flag is the AscendActive-shaped marker on the seat.
		seat.CurrentDungeon = nil
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["dungeon_subsystem_active"] = 1
	}
}

func activateRingTempts(gs *GameState, ev SubsystemEvent) {
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		seat := gs.Seats[ev.Seat]
		// RingTempts is a counter, not an activation bool. The hook
		// merely marks the subsystem live; per_card temptation handlers
		// continue to increment RingTempts when their effect resolves.
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["ring_subsystem_active"] = 1
	}
}

func activateEnergy(gs *GameState, ev SubsystemEvent) {
	// Energy pool lives on the activating seat. EnergyCounters mirrors
	// the legacy Flags["energy_counters"] integer so per_card handlers
	// reading the typed field see the same value once it's incremented.
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		seat := gs.Seats[ev.Seat]
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["energy_subsystem_active"] = 1
	}
}

func activateExperience(gs *GameState, ev SubsystemEvent) {
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		seat := gs.Seats[ev.Seat]
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["experience_subsystem_active"] = 1
	}
}

func activateForetell(gs *GameState, ev SubsystemEvent) {
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		seat := gs.Seats[ev.Seat]
		if seat.ForetellExile == nil {
			seat.ForetellExile = make([]*Card, 0, 4)
		}
		// If the activating event is the foretell-exile movement itself,
		// route the card into the per-seat bucket so consumers can find
		// it. The cast path will pull from this slice when the foretell
		// cost is paid.
		if ev.ToZone == "exile" && ev.Card != nil && ev.Card.FaceDown {
			seat.ForetellExile = append(seat.ForetellExile, ev.Card)
		}
	}
}

func activateCityBlessing(gs *GameState, ev SubsystemEvent) {
	if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil {
		gs.Seats[ev.Seat].HasCityBlessing = true
	}
}
