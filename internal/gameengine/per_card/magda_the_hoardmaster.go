package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMagdaTheHoardmaster wires Magda, the Hoardmaster.
//
// Oracle text:
//
//	Whenever you commit a crime, create a tapped Treasure token. This
//	ability triggers only once each turn.
//	Sacrifice three Treasures: Create a 4/4 red Scorpion Dragon creature
//	token with flying and haste. Activate only as a sorcery.
//
// Implementation: "commit a crime" is not yet a tracked event — we'd
// need to plumb opponent-targeting through cast/ability resolution.
// emitPartial for the trigger. The activated ability is implemented
// (sac 3 treasures → 4/4 red flying haste Scorpion Dragon).
func registerMagdaTheHoardmaster(r *Registry) {
	r.OnTrigger("Magda, the Hoardmaster", "spell_cast", magdaHoardmasterMaybeCrime)
	r.OnActivated("Magda, the Hoardmaster", magdaHoardmasterActivated)
}

func magdaHoardmasterMaybeCrime(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "magda_hoardmaster_crime_treasure"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"crime_detection_unimplemented")
}

func magdaHoardmasterActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "magda_hoardmaster_sac_3_treasures_dragon"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	// Find 3 treasures.
	treasures := []*gameengine.Permanent{}
	for _, p := range gs.Seats[seat].Battlefield {
		if len(treasures) >= 3 {
			break
		}
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "treasure") {
			treasures = append(treasures, p)
		}
	}
	if len(treasures) < 3 {
		emitFail(gs, slug, src.Card.DisplayName(), "not_enough_treasures", map[string]interface{}{
			"have": len(treasures),
		})
		return
	}
	for _, t := range treasures {
		gameengine.SacrificePermanent(gs, t, "magda_hoardmaster_sac")
	}
	token := &gameengine.Card{
		Name:          "Scorpion Dragon Token",
		Owner:         seat,
		BasePower:     4,
		BaseToughness: 4,
		Types:         []string{"token", "creature", "scorpion", "dragon"},
		Colors:        []string{"R"},
		TypeLine:      "Token Creature — Scorpion Dragon",
	}
	tok := enterBattlefieldWithETB(gs, seat, token, false)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, tok, "flying")
		gameengine.SetKeywordFlags(gs, tok, "haste")
		tok.SummoningSick = false
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"sacrificed": 3,
	})
}
