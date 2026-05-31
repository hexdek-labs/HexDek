package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Anim Pakal, Thousandth Moon — {2}{R} 3/3 Legendary Creature — Human
// Artificer.
//
//   Whenever you attack with one or more non-Gnome creatures, put a
//   +1/+1 counter on Anim Pakal, then create X 1/1 colorless Gnome
//   artifact creature tokens that are tapped and attacking, where X is
//   the number of +1/+1 counters on Anim Pakal.
//
// Counter is placed FIRST, then X is read — so the first attack spawns
// 1 Gnome, second spawns 2, etc.

func init() {
	registerAnimPakalThousandthMoon(Global())
	AddResetHook(registerAnimPakalThousandthMoon)
}

func registerAnimPakalThousandthMoon(r *Registry) {
	r.OnTrigger("Anim Pakal, Thousandth Moon", "declare_attackers", animPakalAttackers)
}

func animPakalAttackers(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "anim_pakal_attack_gnomes"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	nonGnomeAttackers := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !p.IsAttacking() {
			continue
		}
		if cardHasSubtype(p.Card, "gnome") {
			continue
		}
		nonGnomeAttackers++
	}
	if nonGnomeAttackers == 0 {
		return
	}
	perm.AddCounter("+1/+1", 1)
	x := perm.Counters["+1/+1"]
	spawned := 0
	for i := 0; i < x; i++ {
		token := gameengine.CreateCreatureToken(gs, perm.Controller, "Gnome Token",
			[]string{"creature", "artifact", "gnome"}, 1, 1)
		if token == nil {
			break
		}
		token.Tapped = true
		token.Flags["attacking"] = 1
		token.SummoningSick = false
		if defenderSeat, ok := ctx["defender_seat"].(int); ok {
			token.Flags["attacking_defender_seat"] = defenderSeat + 1
		}
		spawned++
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":                perm.Controller,
		"non_gnome_attackers": nonGnomeAttackers,
		"new_counter_total":   x,
		"gnomes_spawned":      spawned,
	})
}
