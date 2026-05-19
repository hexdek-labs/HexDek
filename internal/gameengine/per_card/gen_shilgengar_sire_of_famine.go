package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShilgengarSireOfFamine wires Shilgengar, Sire of Famine.
//
// Oracle text (Scryfall, verified):
//
//	Flying
//	Sacrifice another creature: Create a Blood token. If you sacrificed
//	an Angel this way, create a number of Blood tokens equal to its
//	toughness instead.
//	{W/B}{W/B}{W/B}, Sacrifice six Blood tokens: Return each creature
//	card from your graveyard to the battlefield with a finality counter
//	on it. Those creatures are Vampires in addition to their other
//	types.
//
// Implementation (R42b stub port):
//   - Flying: AST keyword pipeline.
//   - OnActivated ability index 0 (free sac → Blood): pick a non-self
//     creature via chooseSacVictimNotSelf, sacrifice it, then mint
//     Blood tokens — 1 by default, or `toughness` if the sac'd
//     creature was an Angel (oracle: "instead" replaces the base 1).
//   - OnActivated ability index 1 (3-hybrid mana, sac 6 Bloods →
//     reanimate creatures): mana cost is checked by the caller. We
//     gather six Blood tokens, sacrifice them, then for each creature
//     card in the controller's graveyard drop it onto the battlefield
//     with a finality counter and a "vampire" subtype overlay
//     (Types-slice append until Phase 8 layers supersede). MoveCard
//     mutates Graveyard during iteration, so we snapshot first.
func registerShilgengarSireOfFamine(r *Registry) {
	r.OnActivated("Shilgengar, Sire of Famine", shilgengarActivate)
}

func shilgengarActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		shilgengarSacForBlood(gs, src, ctx)
	case 1:
		shilgengarMassReanimate(gs, src, ctx)
	}
}

func shilgengarSacForBlood(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "shilgengar_sac_for_blood"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	if gs.Seats[seat] == nil || gs.Seats[seat].Lost {
		return
	}

	victim := chooseSacVictimNotSelf(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_other_creature", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	victimName := victim.Card.DisplayName()
	wasAngel := cardHasSubtype(victim.Card, "angel")
	angelTough := 0
	if wasAngel {
		angelTough = victim.Toughness()
		if angelTough < 0 {
			angelTough = 0
		}
	}
	gameengine.SacrificePermanent(gs, victim, "shilgengar_blood_payment")

	bloods := 1
	if wasAngel {
		// "instead" replaces the base 1 with toughness-many Bloods.
		// A 0-toughness Angel still mints 0 (oracle reading).
		bloods = angelTough
	}
	for i := 0; i < bloods; i++ {
		gameengine.CreateBloodToken(gs, seat)
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
		"was_angel":  wasAngel,
		"bloods":     bloods,
	})
}

func shilgengarMassReanimate(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "shilgengar_mass_reanimate"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Gather six Blood tokens to sacrifice.
	var bloods []*gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		if isBloodToken(p) {
			bloods = append(bloods, p)
			if len(bloods) >= 6 {
				break
			}
		}
	}
	if len(bloods) < 6 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_blood_tokens", map[string]interface{}{
			"seat": seat,
			"have": len(bloods),
			"need": 6,
		})
		return
	}
	for i := 0; i < 6; i++ {
		gameengine.SacrificePermanent(gs, bloods[i], "shilgengar_reanim_cost")
	}

	// Snapshot creature cards in graveyard — enterBattlefieldWithETB
	// will mutate Graveyard as it pulls each card.
	pool := make([]*gameengine.Card, 0, len(s.Graveyard))
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") {
			pool = append(pool, c)
		}
	}
	returned := make([]string, 0, len(pool))
	for _, c := range pool {
		newPerm := enterBattlefieldWithETB(gs, seat, c, false)
		if newPerm == nil {
			continue
		}
		newPerm.AddCounter("finality", 1)
		// Vampire subtype overlay — Phase 8 layer 4 will supersede
		// this; until then, we tag the card in place so type-tag
		// queries (cardHasSubtype) reflect the change for tribal
		// triggers like Bloodthirsty Aerialist.
		if newPerm.Card != nil {
			hasVamp := false
			for _, t := range newPerm.Card.Types {
				if t == "vampire" {
					hasVamp = true
					break
				}
			}
			if !hasVamp {
				newPerm.Card.Types = append(newPerm.Card.Types, "vampire")
			}
		}
		returned = append(returned, c.DisplayName())
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              seat,
		"bloods_sacrificed": 6,
		"returned":          returned,
		"returned_count":    len(returned),
	})
}
