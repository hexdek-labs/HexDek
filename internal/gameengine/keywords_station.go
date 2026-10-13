package gameengine

// keywords_station.go — Station (CR §702.184) Aetherdrift mechanic stub.
//
// CR §702.184a: Station is an activated ability of Spacecraft permanents
//               that functions only while the Spacecraft is on the
//               battlefield. "Station [N]" means "Tap an untapped artifact
//               or creature you control: Put X charge counters on this
//               Spacecraft, where X is the tapped permanent's power.
//               Activate only as a sorcery."
// CR §702.184b: When a Spacecraft has [N] or more charge counters on it
//               (its station threshold), it triggers its STATIONED
//               payoff (the "When NAME becomes stationed" trigger or
//               "STATIONED — ..." reminder text) once per stationing,
//               and gains its STATIONED characteristics (typically
//               becoming an artifact CREATURE with printed P/T) until
//               it would leave the battlefield or its counters drop
//               below the threshold.
//
// This file is a STUB: it provides the helpers callers need
// (HasStation, StationThreshold, StationProgress, IsStationed,
// ActivateStation) and the event hooks for the rules engine, but the
// stationed-payoff dispatch — granting P/T, firing the
// "becomes_stationed" trigger to per_card handlers — is left to a
// follow-up wired through resolve_helpers.go once Aetherdrift card
// data lands. Behavior up to and including "permanent reaches N
// counters" is implemented and tested.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasStation / StationThreshold
// ---------------------------------------------------------------------------

// HasStation returns true if the card has the station keyword in its AST.
func HasStation(card *Card) bool {
	_, ok := StationThreshold(card)
	return ok
}

// StationThreshold returns the N value of the card's station keyword —
// the number of charge counters required to trigger the stationed
// payoff. Returns (N, true) if the keyword is present, (0, false)
// otherwise. The keyword arg is accepted as either float64 (JSON) or
// int.
func StationThreshold(card *Card) (int, bool) {
	if card == nil || card.AST == nil {
		return 0, false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "station") {
			continue
		}
		n := 0
		if len(kw.Args) > 0 {
			switch v := kw.Args[0].(type) {
			case float64:
				n = int(v)
			case int:
				n = v
			}
		}
		return n, true
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// StationProgress / IsStationed
// ---------------------------------------------------------------------------

// StationProgress returns the number of charge counters currently on
// the permanent. Reads from p.Counters["charge"] — the same counter
// kind already used by Aetherflux / Coalition Relic since charge
// counters are not a new game object.
func StationProgress(p *Permanent) int {
	if p == nil || p.Counters == nil {
		return 0
	}
	return p.Counters["charge"]
}

// IsStationed returns true when the Spacecraft has accumulated at least
// its station threshold in charge counters. Returns false for cards
// without the station keyword.
func IsStationed(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	n, ok := StationThreshold(p.Card)
	if !ok {
		return false
	}
	return StationProgress(p) >= n
}

// IsSpacecraft returns true if the permanent's type line includes the
// Aetherdrift "Spacecraft" artifact subtype. Mirrors IsVehicle — checks
// both the Types cache and the printed TypeLine since Spacecraft is
// strictly a subtype on Scryfall (e.g. "Artifact — Spacecraft").
func IsSpacecraft(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	for _, t := range p.Card.Types {
		if equalFoldTrimmed(t, "spacecraft") {
			return true
		}
	}
	if p.Card.TypeLine != "" {
		// case-insensitive substring match on TypeLine
		tl := p.Card.TypeLine
		want := "spacecraft"
		for i := 0; i+len(want) <= len(tl); i++ {
			match := true
			for j := 0; j < len(want); j++ {
				c := tl[i+j]
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c != want[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ActivateStation
// ---------------------------------------------------------------------------

// ActivateStation activates the station ability on a Spacecraft. CR
// §702.184a: tap one untapped artifact or creature you control; the
// Spacecraft gains charge counters equal to that permanent's power
// (creatures) or 1 (artifacts with no power, per the printed reminder).
//
// `contributor` is the single permanent being tapped to pay the station
// cost. Multiple permanents are not supported by the printed ability —
// each station activation taps exactly one permanent. Callers wanting
// to station N times in succession should call ActivateStation N times.
//
// On success the contributor is tapped, the spaceship's charge counter
// pool is increased by the contributor's power, and a "station" event
// is logged. If the new counter total crosses the station threshold for
// the FIRST time this stationing cycle, a "becomes_stationed" event is
// also logged so per_card payoff handlers can fire. The actual payoff
// dispatch is owned by the per_card layer (TODO when Aetherdrift cards
// land).
func ActivateStation(gs *GameState, seatIdx int, ship *Permanent, contributor *Permanent) error {
	if gs == nil {
		return &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if ship == nil || contributor == nil {
		return &CastError{Reason: "nil permanent"}
	}
	if ship.Controller != seatIdx {
		return &CastError{Reason: "not_ship_controller"}
	}
	if contributor.Controller != seatIdx {
		return &CastError{Reason: "not_contributor_controller"}
	}
	if _, ok := StationThreshold(ship.Card); !ok {
		return &CastError{Reason: "no_station_ability"}
	}
	if !IsSpacecraft(ship) {
		return &CastError{Reason: "not_spacecraft"}
	}
	if contributor.Tapped {
		return &CastError{Reason: "contributor_tapped"}
	}
	// CR §702.184a — must be an artifact or creature.
	if !contributor.IsCreature() && !cardHasType(contributor.Card, "artifact") {
		return &CastError{Reason: "contributor_wrong_type"}
	}
	// Summoning sickness: creatures used to station follow the same rule
	// as crew (CR §702.122c) — they may station the turn they enter if
	// they're artifacts (no sickness on non-creatures); creature
	// contributors are subject to summoning sickness because activating
	// the station cost taps them.
	// r85 — NOT migrated, deliberately. ActivateStation has zero
	// production callers (grepped: only this file and its tests), so
	// every read in it is unreachable and a layer-aware rewrite here
	// would be unverifiable by anything but the test that calls it
	// directly. Added to the dead-code ledger as entry 6 — delete-or-wire
	// is its own change. The same applies to contributor.Power() below.
	if contributor.IsCreature() && contributor.SummoningSick && !contributor.HasKeyword("haste") {
		return &CastError{Reason: "summoning_sick"}
	}

	power := 0
	if contributor.IsCreature() {
		power = contributor.Power()
	} else {
		// Non-creature artifact contributors add 1 charge counter (per the
		// printed station reminder text on artifact-only contributors).
		power = 1
	}
	if power < 1 {
		return &CastError{Reason: "no_power_contribution"}
	}

	contributor.Tapped = true

	before := StationProgress(ship)
	if ship.Counters == nil {
		ship.Counters = map[string]int{}
	}
	ship.Counters["charge"] += power
	after := ship.Counters["charge"]

	threshold, _ := StationThreshold(ship.Card)
	crossed := before < threshold && after >= threshold

	gs.LogEvent(Event{
		Kind:   "station",
		Seat:   seatIdx,
		Source: ship.Card.DisplayName(),
		Amount: power,
		Details: map[string]interface{}{
			"contributor": contributor.Card.DisplayName(),
			"before":      before,
			"after":       after,
			"threshold":   threshold,
			"rule":        "702.184a",
		},
	})

	if crossed {
		gs.LogEvent(Event{
			Kind:   "becomes_stationed",
			Seat:   seatIdx,
			Source: ship.Card.DisplayName(),
			Details: map[string]interface{}{
				"threshold": threshold,
				"counters":  after,
				"rule":      "702.184b",
			},
		})
	}

	return nil
}

