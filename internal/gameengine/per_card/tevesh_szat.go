package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTeveshSzat wires Tevesh Szat, Doom of Fools.
//
// Oracle text:
//
//	Legendary Planeswalker — Szat. Loyalty 4. Partner.
//	+2: Create two 0/1 black Thrull creature tokens.
//	+1: You may sacrifice another creature or planeswalker. If you do,
//	    draw two cards, then draw another card if the sacrificed permanent
//	    was a commander.
//	-10: Gain control of all commanders. Put all commanders from the
//	     command zone onto the battlefield under your control.
//	Tevesh Szat, Doom of Fools can be your commander.
//
// The engine has no native loyalty-cost framework for planeswalker
// activations (the +N / -N delta isn't part of the AST cost model — see
// jace_wielder_of_mysteries.go's note). The handler manages loyalty
// directly via perm.Counters["loyalty"] += delta. Activation indexing
// follows oracle order: 0 = +2, 1 = +1, 2 = -10.
//
// On ETB, the engine's stack.go falls back to BaseToughness (or 3) for
// starting loyalty (CR §306.5b). We pin it to 4 explicitly so Tevesh
// always lines up with the printed value regardless of how the card was
// loaded.
func registerTeveshSzat(r *Registry) {
	r.OnETB("Tevesh Szat, Doom of Fools", teveshSzatETB)
	r.OnActivated("Tevesh Szat, Doom of Fools", teveshSzatActivate)
}

func teveshSzatETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["loyalty"] = 4
}

func teveshSzatActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		teveshSzatPlusTwo(gs, src)
	case 1:
		teveshSzatPlusOne(gs, src)
	case 2:
		teveshSzatMinusTen(gs, src)
	}
}

func teveshSzatPlusTwo(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "tevesh_szat_plus_two_thrulls"
	src.AddCounter("loyalty", 2)
	for i := 0; i < 2; i++ {
		token := &gameengine.Card{
			Name:          "Thrull Token",
			Owner:         src.Controller,
			BasePower:     0,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "thrull"},
			Colors:        []string{"B"},
			TypeLine:      "Token Creature — Thrull",
		}
		enterBattlefieldWithETB(gs, src.Controller, token, false)
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             src.Controller,
		"loyalty":          src.Counters["loyalty"],
		"tokens_created":   2,
	})
}

func teveshSzatPlusOne(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "tevesh_szat_plus_one_sac_draw"
	src.AddCounter("loyalty", 1)

	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}

	victim := teveshSzatPickSacrifice(gs, src)
	if victim == nil {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":    src.Controller,
			"loyalty": src.Counters["loyalty"],
			"draw":    0,
			"reason":  "no_eligible_sacrifice",
		})
		return
	}

	wasCommander := teveshSzatIsCommander(gs, victim)
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "tevesh_szat_plus_one")

	drawN := 2
	if wasCommander {
		drawN = 3
	}
	drawn := 0
	for i := 0; i < drawN && len(seat.Library) > 0; i++ {
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, src.Controller, "library", "hand", "draw")
		drawn++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          src.Controller,
		"loyalty":       src.Counters["loyalty"],
		"sacrificed":    victimName,
		"was_commander": wasCommander,
		"draw":          drawn,
	})
}

func teveshSzatMinusTen(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "tevesh_szat_minus_ten_steal_commanders"
	src.AddCounter("loyalty", -10)
	dest := src.Controller

	type stolen struct {
		from int
		name string
	}
	var battlefieldSteals []stolen
	var zoneSteals []stolen

	// Gain control of all commanders on the battlefield (skip our own).
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		var keep []*gameengine.Permanent
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				keep = append(keep, p)
				continue
			}
			if p.Controller == dest {
				keep = append(keep, p)
				continue
			}
			if !teveshSzatIsCommander(gs, p) {
				keep = append(keep, p)
				continue
			}
			old := p.Controller
			p.Controller = dest
			p.Timestamp = gs.NextTimestamp()
			gs.Seats[dest].Battlefield = append(gs.Seats[dest].Battlefield, p)
			gs.LogEvent(gameengine.Event{
				Kind:   "gain_control",
				Seat:   dest,
				Target: old,
				Source: src.Card.DisplayName(),
				Details: map[string]interface{}{
					"target_card": p.Card.DisplayName(),
					"reason":      "tevesh_szat_minus_ten",
				},
			})
			battlefieldSteals = append(battlefieldSteals, stolen{from: old, name: p.Card.DisplayName()})
		}
		s.Battlefield = keep
	}

	// Pull every commander out of the command zone and onto our battlefield.
	// Snapshot before iterating — MoveCard mutates s.CommandZone via
	// removeCardFromZone.
	for seatIdx, s := range gs.Seats {
		if s == nil || len(s.CommandZone) == 0 {
			continue
		}
		zoneSnap := append([]*gameengine.Card(nil), s.CommandZone...)
		for _, c := range zoneSnap {
			if c == nil {
				continue
			}
			actualDest := gameengine.MoveCard(gs, c, seatIdx, "command_zone", "battlefield", "tevesh_szat_minus_ten")
			if actualDest != "battlefield" {
				continue
			}
			ent := enterBattlefieldWithETB(gs, dest, c, false)
			if ent != nil {
				zoneSteals = append(zoneSteals, stolen{from: seatIdx, name: c.DisplayName()})
			}
		}
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":                dest,
		"loyalty":             src.Counters["loyalty"],
		"battlefield_stolen":  len(battlefieldSteals),
		"command_zone_pulled": len(zoneSteals),
	})
}

// teveshSzatPickSacrifice picks the cheapest "another creature or
// planeswalker" we control. Avoids Tevesh herself (she's the source —
// sacrificing her loses the activation's value source for partner pairs).
func teveshSzatPickSacrifice(gs *gameengine.GameState, src *gameengine.Permanent) *gameengine.Permanent {
	if gs == nil || src == nil {
		return nil
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestCMC := 1<<31 - 1
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsCreature() && !p.IsPlaneswalker() {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc < bestCMC {
			best = p
			bestCMC = cmc
		}
	}
	return best
}

// teveshSzatIsCommander returns true if the permanent's card name appears
// in its owner's CommanderNames list.
func teveshSzatIsCommander(gs *gameengine.GameState, p *gameengine.Permanent) bool {
	if gs == nil || p == nil || p.Card == nil {
		return false
	}
	owner := p.Owner
	if owner < 0 || owner >= len(gs.Seats) || gs.Seats[owner] == nil {
		return false
	}
	want := strings.ToLower(p.Card.DisplayName())
	for _, name := range gs.Seats[owner].CommanderNames {
		if strings.ToLower(name) == want {
			return true
		}
	}
	return false
}
