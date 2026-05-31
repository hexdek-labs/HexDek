package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRashmiAndRagavan wires Rashmi and Ragavan.
//
// Oracle text (Scryfall, verified 2026-05-30, {U}{R}, 3/2 Legendary
// Creature — Elf Monkey):
//
//	Whenever you cast your first spell during each of your turns,
//	exile the top card of target opponent's library and create a
//	Treasure token. Then you may cast the exiled card without paying
//	its mana cost if it's a spell with mana value less than the
//	number of artifacts you control. If you don't cast it this way,
//	you may cast it this turn.
//
// Implementation (R60 stub sweep batch 2):
//   - "spell_cast" on Rashmi's controller's own turn, first cast: exile
//     top of first non-empty opponent's library (existing) and mint a
//     Treasure (existing).
//   - Free-cast grant (this batch): count controller's artifacts AFTER
//     the Treasure mints, so the new Treasure counts toward the
//     threshold (oracle order: "create a Treasure token. Then you may
//     cast..."). If exiled card's mana value < artifact count, register
//     NewFreeCastFromExilePermission with ManaCost=0. Otherwise register
//     the same permission with ManaCost=-1 (normal cost, the "you may
//     cast it this turn" fallback). Both stamp Duration="until_end_of_turn"
//     + GrantTurn + SourceTimestamp for r60 ZoneCastGrantExpiry hygiene.
//   - Lands are excluded from the free-cast eligibility check ("if it's
//     a spell" — lands aren't spells per CR §111). Lands get no grant.
func registerRashmiAndRagavan(r *Registry) {
	r.OnTrigger("Rashmi and Ragavan", "spell_cast", rashmiRagavanCast)
}

func rashmiRagavanCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rashmi_ragavan_first_spell"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	if gs.Active != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["rashmi_first_spell_turn"] == gs.Turn+1 {
		return
	}
	perm.Flags["rashmi_first_spell_turn"] = gs.Turn + 1

	var targetOpp int = -1
	var exiledCard *gameengine.Card
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost || len(s.Library) == 0 {
			continue
		}
		targetOpp = opp
		exiledCard = s.Library[0]
		break
	}
	if targetOpp >= 0 && exiledCard != nil {
		s := gs.Seats[targetOpp]
		gameengine.MoveCard(gs, exiledCard, targetOpp, "library", "exile", "rashmi_exile")
		_ = s
	}
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Treasure",
		[]string{"artifact", "treasure"}, 0, 0)
	if tok != nil && tok.Card != nil {
		tok.Card.Types = []string{"token", "artifact", "treasure"}
		tok.Card.BasePower = 0
		tok.Card.BaseToughness = 0
	}

	// Grant: cast-from-exile permission for the exiled card.
	grantKind := "none"
	if exiledCard != nil && !cardHasType(exiledCard, "land") {
		seat := gs.Seats[perm.Controller]
		artifactCount := 0
		if seat != nil {
			for _, p := range seat.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if cardHasType(p.Card, "artifact") {
					artifactCount++
				}
			}
		}
		exiledCMC := cardCMC(exiledCard)
		grant := gameengine.NewFreeCastFromExilePermission(perm.Controller, perm.Card.DisplayName())
		if exiledCMC < artifactCount {
			grant.ManaCost = 0
			grantKind = "free"
		} else {
			grant.ManaCost = -1
			grantKind = "normal_cost"
		}
		grant.Duration = "until_end_of_turn"
		grant.GrantTurn = gs.Turn
		grant.SourceTimestamp = perm.Timestamp
		gameengine.RegisterZoneCastGrant(gs, exiledCard, grant)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"target":     targetOpp,
		"grant_kind": grantKind,
	})
}
