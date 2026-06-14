package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEdELonesomeEyebot wires ED-E, Lonesome Eyebot.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Flying
//	ED-E My Love — Whenever you attack, if the number of attacking
//	  creatures is greater than the number of quest counters on ED-E,
//	  put a quest counter on it.
//	{2}, Sacrifice ED-E: Draw a card, then draw an additional card for
//	  each quest counter on ED-E.
func registerEdELonesomeEyebot(r *Registry) {
	r.OnTrigger("ED-E, Lonesome Eyebot", "attack_declared", edEAttackTrigger)
	r.OnActivated("ED-E, Lonesome Eyebot", edESacrificeDraw)
}

func edEAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ed_e_quest_counter"
	if gs == nil || perm == nil {
		return
	}
	// "Whenever you attack" fires once per declare-attackers step, but the
	// engine fires "attack_declared" once per declared attacker. Without a
	// gate the per-attacker re-fire ratchets a quest counter up to once per
	// attacker (the count<=current guard only blocks after each add). Gate to
	// the first hit per turn so at most one quest counter lands per combat
	// (Caesar/Raffine once-per-turn dedup pattern).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["ed_e_attack_fired"] >= gs.Turn {
		return
	}
	perm.Flags["ed_e_attack_fired"] = gs.Turn
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.IsAttacking() {
			count++
		}
	}
	current := 0
	if perm.Counters != nil {
		current = perm.Counters["quest"]
	}
	if count <= current {
		return
	}
	perm.AddCounter("quest", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"attackers":      count,
		"quest_counters": current + 1,
	})
}

func edESacrificeDraw(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ed_e_sacrifice_draw"
	if gs == nil || src == nil {
		return
	}
	quest := 0
	if src.Counters != nil {
		quest = src.Counters["quest"]
	}
	totalDraws := 1 + quest
	gameengine.SacrificePermanent(gs, src, "ed_e_sacrifice_draw")
	drawn := 0
	for i := 0; i < totalDraws; i++ {
		if drawOne(gs, src.Controller, src.Card.DisplayName()) != nil {
			drawn++
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  src.Controller,
		"quest": quest,
		"drawn": drawn,
	})
}
