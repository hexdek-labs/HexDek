package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRatadrabikOfUrborg wires Ratadrabik of Urborg.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Vigilance, ward {2}
//	Other Zombies you control have vigilance.
//	Whenever another legendary creature you control dies, create a
//	token that's a copy of that creature, except it's not legendary
//	and it's a 2/2 black Zombie in addition to its other colors and
//	types.
//
// Implementation:
//   - Vigilance + ward {2} via ETB Flag stamping (engine ward machinery
//     consults perm.Flags["kw:ward"]/["ward_cost"]).
//   - "creature_dies" trigger: when a *legendary* creature controlled by
//     Ratadrabik's controller (other than Ratadrabik) dies, mint a copy
//     token by cloning Card.Name and stripping the legendary supertype.
//     The clone is forced to a 2/2 with creature/zombie/token types and
//     black added to its colors.
func registerRatadrabikOfUrborg(r *Registry) {
	r.OnETB("Ratadrabik of Urborg", ratadrabikETB)
	r.OnTrigger("Ratadrabik of Urborg", "creature_dies", ratadrabikLegendaryDies)
}

func ratadrabikETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	gameengine.SetKeywordFlags(gs, perm, "ward")
	perm.Flags["ward_cost"] = 2
}

func ratadrabikLegendaryDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ratadrabik_legendary_zombie_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	deadController, _ := ctx["controller_seat"].(int)
	if deadController != perm.Controller {
		return
	}
	deadCard, _ := ctx["card"].(*gameengine.Card)
	if deadCard == nil || deadCard == perm.Card {
		return
	}
	if !cardHasType(deadCard, "creature") || !cardHasType(deadCard, "legendary") {
		return
	}

	// Build a cloned token: not legendary, 2/2, black + zombie added,
	// keep other colors/types.
	types := []string{"token", "creature", "zombie"}
	for _, t := range deadCard.Types {
		if t == "legendary" {
			continue
		}
		dup := false
		for _, e := range types {
			if e == t {
				dup = true
				break
			}
		}
		if !dup {
			types = append(types, t)
		}
	}
	colors := []string{"B"}
	for _, c := range deadCard.Colors {
		if c == "B" {
			continue
		}
		colors = append(colors, c)
	}
	token := &gameengine.Card{
		Name:          deadCard.DisplayName() + " Token",
		Owner:         perm.Controller,
		Types:         types,
		Colors:        colors,
		BasePower:     2,
		BaseToughness: 2,
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"copied":   deadCard.DisplayName(),
		"token_pt": "2/2",
	})
}
