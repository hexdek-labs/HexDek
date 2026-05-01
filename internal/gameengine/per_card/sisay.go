package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSisayWeatherlightCaptain wires Sisay, Weatherlight Captain.
//
// Oracle text:
//
//	Sisay gets +1/+1 for each color among other legendary permanents you
//	control.
//	{W}{U}{B}{R}{G}: Search your library for a legendary permanent card
//	with mana value less than Sisay's power, put that card onto the
//	battlefield, then shuffle.
//
// The 5-color activation is the engine of every Sisay deck: every cast
// turns into the highest-impact legendary you can fit under her power
// (often Yisan/Najeela/Jegantha early, then Atraxa/Kenrith/Jodah later
// once the +1/+1-per-color anthem stacks). The lack of restriction to
// nonland legendary permanents means Gaea's Cradle, Karakas, and the
// Urza tron lands are all on the table — which is part of why Sisay is
// banned at high power and is a key cEDH commander.
//
// Implementation:
//   - OnActivated: compute Sisay's effective power = BasePower +
//     counters/mods + colors-among-other-legendary-permanents-you-control
//     (the static buff isn't wired through the §613 layer system, so we
//     compute it inline here).
//   - Search for the highest-CMC legendary permanent card in library with
//     mana value strictly less than Sisay's power.
//   - Put it onto the battlefield via the full ETB cascade. Shuffle.
func registerSisayWeatherlightCaptain(r *Registry) {
	r.OnActivated("Sisay, Weatherlight Captain", sisayActivate)
}

func sisayActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sisay_weatherlight_legendary_tutor"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Sisay's effective power: base + counters/mods + distinct colors among
	// OTHER legendary permanents you control.
	colorSet := map[string]bool{}
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsLegendary() {
			continue
		}
		for _, c := range p.Card.Colors {
			colorSet[strings.ToUpper(strings.TrimSpace(c))] = true
		}
	}
	power := src.Power() + len(colorSet)

	// Find the highest-CMC legendary permanent card in library with CMC < power.
	foundIdx := -1
	bestCMC := -1
	for i, c := range s.Library {
		if c == nil {
			continue
		}
		if !cardHasType(c, "legendary") {
			continue
		}
		if !isPermanentCard(c) {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc >= power {
			continue
		}
		if cmc > bestCMC {
			bestCMC = cmc
			foundIdx = i
		}
	}

	if foundIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, src.Card.DisplayName(), "no_legal_legendary_under_power", map[string]interface{}{
			"seat":  seat,
			"power": power,
		})
		return
	}

	tutored := s.Library[foundIdx]
	s.Library = append(s.Library[:foundIdx], s.Library[foundIdx+1:]...)
	shuffleLibraryPerCard(gs, seat)
	enterBattlefieldWithETB(gs, seat, tutored, false)

	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"found_card": tutored.DisplayName(),
			"reason":     "sisay_weatherlight_captain",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"sisay_power":  power,
		"colors":       len(colorSet),
		"tutored":      tutored.DisplayName(),
		"tutored_cmc":  bestCMC,
	})
	_ = gs.CheckEnd()
}

