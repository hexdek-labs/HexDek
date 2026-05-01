package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGishathSunsAvatar wires Gishath, Sun's Avatar (Ixalan).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	{5}{R}{G}{W}, 7/6 Legendary Creature — Dinosaur Avatar
//	Vigilance, trample, haste
//	Whenever Gishath, Sun's Avatar deals combat damage to a player,
//	reveal that many cards from the top of your library. Put any
//	number of Dinosaur creature cards from among them onto the
//	battlefield and the rest on the bottom of your library in a
//	random order.
//
// Engine wiring:
//   - vigilance / trample / haste: AST keyword pipeline.
//   - OnTrigger("combat_damage_player"): fires when Gishath herself
//     deals combat damage to a player. Reveals the top N cards
//     (N = damage dealt), puts every Dinosaur creature card onto the
//     battlefield, and shuffles the remainder onto the bottom of
//     Gishath's controller's library.
func registerGishathSunsAvatar(r *Registry) {
	r.OnTrigger("Gishath, Sun's Avatar", "combat_damage_player", gishathCombatDamage)
}

func gishathCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gishath_suns_avatar_reveal_dinos"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceCard, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller || sourceCard != perm.Card.DisplayName() {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	n := amount
	if n > len(s.Library) {
		n = len(s.Library)
	}
	if n == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "library_empty", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	revealed := make([]*gameengine.Card, 0, n)
	for i := 0; i < n; i++ {
		revealed = append(revealed, s.Library[i])
	}

	dinos := make([]*gameengine.Card, 0, n)
	rest := make([]*gameengine.Card, 0, n)
	for _, c := range revealed {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") && cardHasType(c, "dinosaur") {
			dinos = append(dinos, c)
		} else {
			rest = append(rest, c)
		}
	}

	// Drop the revealed prefix off the library; we'll re-insert the
	// non-dinosaur remainder on the bottom in random order.
	s.Library = s.Library[n:]

	dinoNames := make([]string, 0, len(dinos))
	for _, c := range dinos {
		newPerm := enterBattlefieldWithETB(gs, seat, c, false)
		if newPerm == nil {
			continue
		}
		dinoNames = append(dinoNames, c.DisplayName())
		gs.LogEvent(gameengine.Event{
			Kind:   "put_onto_battlefield",
			Seat:   seat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"card":      c.DisplayName(),
				"from_zone": "library",
				"reason":    "gishath_combat_damage",
			},
		})
	}

	if len(rest) > 0 && gs.Rng != nil {
		gs.Rng.Shuffle(len(rest), func(i, j int) {
			rest[i], rest[j] = rest[j], rest[i]
		})
	}
	bottomedNames := make([]string, 0, len(rest))
	for _, c := range rest {
		s.Library = append(s.Library, c)
		bottomedNames = append(bottomedNames, c.DisplayName())
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             seat,
		"damage":           amount,
		"revealed":         len(revealed),
		"dinos_played":     dinoNames,
		"bottomed_cards":   bottomedNames,
		"bottomed_count":   len(rest),
	})
}
