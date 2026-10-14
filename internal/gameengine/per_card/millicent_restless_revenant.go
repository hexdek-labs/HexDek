package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMillicentRestlessRevenant wires Millicent, Restless Revenant.
//
// Oracle text:
//
//	Affinity for Spirits (This spell costs {1} less to cast for each
//	Spirit you control.)
//	Flying
//	Whenever Millicent or another nontoken Spirit you control dies or
//	deals combat damage to a player, create a 1/1 white Spirit creature
//	token with flying.
//
// We wire two trigger paths: combat_damage_player (catches Millicent or
// other Spirits hitting a player) and creature_dies (catches Millicent
// or other nontoken Spirits going to the graveyard).
func registerMillicentRestlessRevenant(r *Registry) {
	r.OnTrigger("Millicent, Restless Revenant", "combat_damage_player", millicentSpiritCombat)
	r.OnTrigger("Millicent, Restless Revenant", "creature_dies", millicentSpiritDies)
}

func millicentMakeToken(gs *gameengine.GameState, seat int, slug, source string, ctx map[string]interface{}) {
	tok := gameengine.CreateCreatureToken(gs, seat, "Spirit Token",
		[]string{"creature", "spirit"}, 1, 1)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, tok, "flying")
		if tok.Card != nil {
			tok.Card.Colors = []string{"W"}
		}
	}
	d := map[string]interface{}{"seat": seat, "trigger": source}
	for k, v := range ctx {
		d[k] = v
	}
	emit(gs, slug, "Millicent, Restless Revenant", d)
}

func millicentIsNontokenSpirit(card *gameengine.Card) bool {
	if card == nil {
		return false
	}
	for _, t := range card.Types {
		if strings.EqualFold(t, "token") {
			return false
		}
	}
	if strings.Contains(strings.ToLower(card.TypeLine), "token") {
		return false
	}
	for _, t := range card.Types {
		if strings.EqualFold(t, "spirit") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(card.TypeLine), "spirit")
}

func millicentSpiritCombat(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "millicent_combat_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceName, _ := ctx["source_card"].(string)
	if sourceName == perm.Card.DisplayName() {
		millicentMakeToken(gs, perm.Controller, slug, "millicent_self", ctx)
		return
	}
	s := gs.Seats[sourceSeat]
	if s == nil {
		return
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() != sourceName {
			continue
		}
		if !millicentIsNontokenSpirit(p.Card) {
			return
		}
		millicentMakeToken(gs, perm.Controller, slug, "other_spirit", ctx)
		return
	}
}

func millicentSpiritDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "millicent_dies_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	deadController, _ := ctx["controller_seat"].(int)
	if deadController != perm.Controller {
		return
	}
	deadCard, _ := ctx["card"].(*gameengine.Card)
	if deadCard == nil {
		return
	}
	if deadCard.DisplayName() == perm.Card.DisplayName() {
		millicentMakeToken(gs, perm.Controller, slug, "millicent_self_dies", ctx)
		return
	}
	if !millicentIsNontokenSpirit(deadCard) {
		return
	}
	millicentMakeToken(gs, perm.Controller, slug, "other_spirit_dies", ctx)
}
