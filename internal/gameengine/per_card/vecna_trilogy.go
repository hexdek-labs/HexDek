package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Eye of Vecna
// ---------------------------------------------------------------------------
//
// "When Eye of Vecna enters the battlefield, you draw a card and you
// lose 2 life."
// "At the beginning of your upkeep, you may pay {2}. If you do, draw
// a card and you lose 2 life."

func registerEyeOfVecna(r *Registry) {
	r.OnETB("Eye of Vecna", eyeOfVecnaETB)
	r.OnTrigger("Eye of Vecna", "upkeep_controller", eyeOfVecnaUpkeep)
}

func eyeOfVecnaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if len(s.Library) > 0 {
		drawn := s.Library[0]
		gameengine.MoveCard(gs, drawn, seat, "library", "hand", "draw")
	}
	s.Life -= 2
	gs.LogEvent(gameengine.Event{
		Kind: "eye_of_vecna_etb", Seat: seat,
		Source: "Eye of Vecna",
		Details: map[string]interface{}{"draw": 1, "life_lost": 2},
	})
}

func eyeOfVecnaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Only on your own upkeep.
	if gs.Active != seat {
		return
	}
	s := gs.Seats[seat]
	// AI always pays if life > 4.
	if s.Life <= 4 || s.ManaPool < 2 {
		return
	}
	s.ManaPool -= 2
	gameengine.SyncManaAfterSpend(s)
	if len(s.Library) > 0 {
		drawn := s.Library[0]
		gameengine.MoveCard(gs, drawn, seat, "library", "hand", "draw")
	}
	s.Life -= 2
	gs.LogEvent(gameengine.Event{
		Kind: "eye_of_vecna_upkeep", Seat: seat,
		Source: "Eye of Vecna",
		Details: map[string]interface{}{"draw": 1, "life_lost": 2, "mana_paid": 2},
	})
}

// ---------------------------------------------------------------------------
// Hand of Vecna
// ---------------------------------------------------------------------------
//
// "At the beginning of combat on your turn, equipped creature or a creature
// you control named Vecna gets +X/+X until end of turn, where X is the
// number of cards in your hand."

func registerHandOfVecna(r *Registry) {
	r.OnTrigger("Hand of Vecna", "combat_begin", handOfVecnaCombat)
}

func handOfVecnaCombat(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Active != seat {
		return
	}
	s := gs.Seats[seat]
	x := len(s.Hand)
	if x == 0 {
		return
	}
	// Find equipped creature or a creature named Vecna.
	var target *gameengine.Permanent
	if perm.AttachedTo != nil && perm.AttachedTo.IsCreature() {
		target = perm.AttachedTo
	}
	if target == nil {
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() && p.Card != nil &&
				strings.ToLower(p.Card.DisplayName()) == "vecna" {
				target = p
				break
			}
		}
	}
	if target == nil {
		return
	}
	target.Modifications = append(target.Modifications, gameengine.Modification{
		Power: x, Toughness: x, Duration: "until_end_of_turn",
	})
	gs.LogEvent(gameengine.Event{
		Kind: "hand_of_vecna_boost", Seat: seat,
		Source: "Hand of Vecna",
		Amount: x,
		Details: map[string]interface{}{
			"target":    target.Card.DisplayName(),
			"hand_size": x,
		},
	})
}

// ---------------------------------------------------------------------------
// Book of Vile Darkness
// ---------------------------------------------------------------------------
//
// "At the beginning of your end step, if you lost 2 or more life this
// turn, create a 2/2 black Zombie creature token."
//
// "{T}, Exile Book of Vile Darkness and artifacts you control named Eye
// of Vecna and Hand of Vecna: Create Vecna, a legendary 8/8 black Zombie
// God creature token with indestructible and all triggered abilities of
// the exiled cards."

func registerBookOfVileDarkness(r *Registry) {
	r.OnTrigger("Book of Vile Darkness", "end_step", bookOfVileEndStep)
	r.OnActivated("Book of Vile Darkness", bookOfVileActivated)
}

func bookOfVileEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Active != seat {
		return
	}
	s := gs.Seats[seat]
	lifeLost := 0
	if s.Flags != nil {
		if startLife, ok := s.Flags["life_at_turn_start"]; ok && startLife > s.Life {
			lifeLost = startLife - s.Life
		}
	}
	if lifeLost < 2 {
		return
	}
	gameengine.CreateCreatureToken(gs, seat, "Zombie", []string{"creature", "zombie"}, 2, 2)
	gs.LogEvent(gameengine.Event{
		Kind: "book_of_vile_zombie", Seat: seat,
		Source: "Book of Vile Darkness",
		Details: map[string]interface{}{"life_lost": lifeLost},
	})
}

func bookOfVileActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Find Eye of Vecna and Hand of Vecna on our battlefield.
	var eye, hand *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		switch p.Card.DisplayName() {
		case "Eye of Vecna":
			eye = p
		case "Hand of Vecna":
			hand = p
		}
	}
	if eye == nil || hand == nil {
		gs.LogEvent(gameengine.Event{
			Kind: "vecna_assembly_fail", Seat: seat,
			Source: "Book of Vile Darkness",
			Details: map[string]interface{}{
				"reason":    "missing_components",
				"have_eye":  eye != nil,
				"have_hand": hand != nil,
			},
		})
		return
	}

	// Exile all three.
	gameengine.SacrificePermanent(gs, eye, "vecna_assembly")
	gameengine.SacrificePermanent(gs, hand, "vecna_assembly")
	gameengine.SacrificePermanent(gs, src, "vecna_assembly")

	// Create Vecna — 8/8 black Zombie God, indestructible.
	vecna := gameengine.CreateCreatureToken(gs, seat, "Vecna", []string{
		"legendary", "creature", "zombie", "god",
	}, 8, 8)
	if vecna != nil {
		vecna.GrantedAbilities = append(vecna.GrantedAbilities, "indestructible")
		if vecna.Flags == nil {
			vecna.Flags = map[string]int{}
		}
		vecna.Flags["indestructible"] = 1
	}

	gs.LogEvent(gameengine.Event{
		Kind: "vecna_assembled", Seat: seat,
		Source: "Book of Vile Darkness",
		Details: map[string]interface{}{
			"token": "Vecna",
			"stats": "8/8",
			"rule":  "vecna_composite",
		},
	})
}
