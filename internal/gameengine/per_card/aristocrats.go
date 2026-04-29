package per_card

import "github.com/hexdek/hexdek/internal/gameengine"

// Aristocrat death-trigger cards: whenever a creature dies, drain opponents.

func registerBloodArtist(r *Registry) {
	r.OnTrigger("Blood Artist", "creature_dies", bloodArtistTrigger)
}

func bloodArtistTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Target opponent loses 1, you gain 1.
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		s.Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Target: -1,
			Source: "Blood Artist",
			Details: map[string]interface{}{"amount": -1, "cause": "blood_artist"},
		})
		break // "target opponent" — pick first alive opponent
	}
	gameengine.GainLife(gs, seat, 1, "Blood Artist")
}

func registerZulaportCutthroat(r *Registry) {
	r.OnTrigger("Zulaport Cutthroat", "creature_dies", zulaportTrigger)
}

func zulaportTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}
	// Each opponent loses 1 life, you gain 1 life.
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		s.Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Target: -1,
			Source: "Zulaport Cutthroat",
			Details: map[string]interface{}{"amount": -1, "cause": "zulaport_cutthroat"},
		})
	}
	gameengine.GainLife(gs, seat, 1, "Zulaport Cutthroat")
}

func registerBastionOfRemembrance(r *Registry) {
	r.OnTrigger("Bastion of Remembrance", "creature_dies", bastionTrigger)
}

func bastionTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Only triggers on creatures YOU control dying.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		s.Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Target: -1,
			Source: "Bastion of Remembrance",
			Details: map[string]interface{}{"amount": -1, "cause": "bastion_of_remembrance"},
		})
	}
	gameengine.GainLife(gs, seat, 1, "Bastion of Remembrance")
}

func registerCruelCelebrant(r *Registry) {
	r.OnTrigger("Cruel Celebrant", "creature_dies", cruelCelebrantTrigger)
}

func cruelCelebrantTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Only on creatures/planeswalkers YOU control dying.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		s.Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Target: -1,
			Source: "Cruel Celebrant",
			Details: map[string]interface{}{"amount": -1, "cause": "cruel_celebrant"},
		})
	}
	gameengine.GainLife(gs, seat, 1, "Cruel Celebrant")
}

func registerVindictiveVampire(r *Registry) {
	r.OnTrigger("Vindictive Vampire", "creature_dies", vindictiveTrigger)
}

func vindictiveTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Only another creature YOU control dying.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		s.Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Target: -1,
			Source: "Vindictive Vampire",
			Details: map[string]interface{}{"amount": -1, "cause": "vindictive_vampire"},
		})
	}
	gameengine.GainLife(gs, seat, 1, "Vindictive Vampire")
}

func registerSyrKonrad(r *Registry) {
	r.OnTrigger("Syr Konrad, the Grim", "creature_dies", syrKonradTrigger)
}

func syrKonradTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Whenever ANY creature dies → each opponent loses 1.
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		s.Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Target: -1,
			Source: "Syr Konrad, the Grim",
			Details: map[string]interface{}{"amount": -1, "cause": "syr_konrad"},
		})
	}
}
