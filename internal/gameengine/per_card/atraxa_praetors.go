package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAtraxaPraetorsVoice wires Atraxa, Praetors' Voice.
//
// Oracle text:
//
//	Flying, vigilance, deathtouch, lifelink.
//	At the beginning of your end step, proliferate. (Choose any number
//	of permanents and/or players with counters, then give each another
//	counter of each kind already there.)
//
// Greedy AI policy mirrors the engine's stock proliferate (see
// resolve_helpers.go case "proliferate"):
//   - Add a counter of every existing kind to each permanent we control.
//   - For opponent permanents, skip beneficial "+1/+1" counters and only
//     proliferate harmful counters (-1/-1, stun, etc.).
//   - On the controller, proliferate energy/experience.
//   - On opponents, proliferate poison and rad.
func registerAtraxaPraetorsVoice(r *Registry) {
	r.OnTrigger("Atraxa, Praetors' Voice", "end_step", atraxaPraetorsEndStep)
}

func atraxaPraetorsEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "atraxa_praetors_voice_proliferate"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	if gs.Seats[seat] == nil || gs.Seats[seat].Lost {
		return
	}

	proliferated := 0

	// Permanents.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || len(p.Counters) == 0 {
				continue
			}
			isOurs := p.Controller == seat
			for kind, count := range p.Counters {
				if count <= 0 {
					continue
				}
				if !isOurs && kind == "+1/+1" {
					continue
				}
				p.AddCounter(kind, 1)
				proliferated++
			}
		}
	}

	// Players.
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if i == seat {
			if s.Flags != nil {
				if s.Flags["energy_counters"] > 0 {
					s.Flags["energy_counters"]++
					proliferated++
				}
				if s.Flags["experience_counters"] > 0 {
					s.Flags["experience_counters"]++
					proliferated++
				}
			}
		} else {
			if s.PoisonCounters > 0 {
				s.PoisonCounters++
				proliferated++
			}
			if s.Flags != nil && s.Flags["rad_counters"] > 0 {
				s.Flags["rad_counters"]++
				proliferated++
			}
		}
	}

	if proliferated > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "proliferate",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Amount: proliferated,
		Details: map[string]interface{}{
			"rule": "701.27",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"proliferated": proliferated,
	})
}
