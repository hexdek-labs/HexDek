package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAshnodFleshMechanist wires Ashnod, Flesh Mechanist.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Deathtouch
//	Whenever Ashnod attacks, you may sacrifice another creature. If
//	you do, create a tapped Powerstone token.
//	{5}, Exile a creature card from your graveyard: Create a tapped
//	3/3 colorless Zombie artifact creature token.
//
// Implementation:
//   - Deathtouch: AST keyword pipeline.
//   - "creature_attacks" trigger: when Ashnod herself is the declared
//     attacker, may-sacrifice another creature → mint a tapped Powerstone
//     token. The "may" defaults to YES whenever a positive-score victim
//     exists (Powerstone ramp + sac trigger compounding is almost always
//     correct in this deck shell). The Powerstone enters tapped per
//     oracle (we tap the most-recently-appended powerstone permanent).
//   - OnActivated(0): the {5} cost is settled by the engine; the
//     graveyard-exile additional cost is paid here by removing the best
//     creature card from the controller's graveyard. Mints a tapped 3/3
//     colorless Zombie artifact creature token.
func registerAshnodFleshMechanist(r *Registry) {
	r.OnTrigger("Ashnod, Flesh Mechanist", "creature_attacks", ashnodFleshMechanistAttack)
	r.OnActivated("Ashnod, Flesh Mechanist", ashnodFleshMechanistActivate)
}

func ashnodFleshMechanistAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ashnod_flesh_mechanist_attack_powerstone"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := perm.Controller
	victim := chooseSacVictimNotSelf(gs, seat, perm, ctx)
	if victim == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_other_creature", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "ashnod_flesh_mechanist_attack")

	gameengine.CreatePowerstoneToken(gs, seat)
	tapMostRecentTokenWithSubtype(gs, seat, "powerstone")

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
		"token":      "Powerstone (tapped)",
	})
}

func ashnodFleshMechanistActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ashnod_flesh_mechanist_zombie_token"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if abilityIdx != 0 {
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

	// Pay the additional cost: exile a creature card from your graveyard.
	exiled := ashnodExileCreatureFromGraveyard(gs, seat)
	if exiled == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_in_graveyard", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	// Create a tapped 3/3 colorless Zombie artifact creature token.
	tokenCard := &gameengine.Card{
		Name:          "Zombie Token",
		Owner:         seat,
		Types:         []string{"creature", "token", "artifact", "zombie"},
		BasePower:     3,
		BaseToughness: 3,
	}
	tok := enterBattlefieldWithETB(gs, seat, tokenCard, true)
	if tok != nil {
		// Tokens enter without summoning sickness only when oracle says so;
		// here it enters tapped, so block-decision parity is unaffected
		// either way. Keep summoning sickness consistent with default.
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "Zombie Token",
			"reason": "ashnod_flesh_mechanist_activate",
			"power":  3,
			"tough":  3,
			"tapped": true,
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"exiled_card":  exiled.DisplayName(),
		"token":        "3/3 Zombie artifact (tapped)",
	})
}

// ashnodExileCreatureFromGraveyard removes the highest-CMC creature card
// from seat's graveyard and exiles it. Returns the card moved, or nil if
// no creature card was available.
func ashnodExileCreatureFromGraveyard(gs *gameengine.GameState, seat int) *gameengine.Card {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range s.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	if best == nil {
		return nil
	}
	gameengine.MoveCard(gs, best, seat, "graveyard", "exile", "ashnod_activate_cost")
	return best
}

// tapMostRecentTokenWithSubtype taps the most recently-appended permanent
// in seat's battlefield whose card has the given subtype tag. Used when a
// helper that mints a token doesn't expose the resulting permanent — we
// need to enter-tapped after the fact.
func tapMostRecentTokenWithSubtype(gs *gameengine.GameState, seat int, subtype string) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	for i := len(s.Battlefield) - 1; i >= 0; i-- {
		p := s.Battlefield[i]
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, subtype) {
			continue
		}
		p.Tapped = true
		return
	}
}
