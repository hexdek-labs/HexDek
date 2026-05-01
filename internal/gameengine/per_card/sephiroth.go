package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSephiroth wires Sephiroth, Fabled SOLDIER // Sephiroth, One-Winged Angel.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
// Front face — Sephiroth, Fabled SOLDIER (Legendary Creature — Human Avatar Soldier, 3/3):
//
//	Whenever Sephiroth enters or attacks, you may sacrifice another
//	creature. If you do, draw a card.
//	Whenever another creature dies, target opponent loses 1 life and
//	you gain 1 life. If this is the fourth time this ability has
//	resolved this turn, transform Sephiroth.
//
// Back face — Sephiroth, One-Winged Angel (Legendary Creature — Angel Nightmare Avatar, 5/5):
//
//	Flying
//	Super Nova — As this creature transforms into Sephiroth, One-Winged
//	Angel, you get an emblem with "Whenever a creature dies, target
//	opponent loses 1 life and you gain 1 life."
//	Whenever Sephiroth attacks, you may sacrifice any number of other
//	creatures. If you do, draw that many cards.
//
// Implementation:
//   - OnETB (front face only): may sac another creature → draw 1.
//   - "creature_attacks": branch on perm.Transformed.
//       * Front: may sac another creature → draw 1.
//       * Back: may sac any number of other creatures → draw that many.
//   - "creature_dies" (front face only): drain 1 from first alive opp,
//     gain 1; increment a per-turn counter on Sephiroth's flags. On the
//     4th resolution this turn, call gameengine.TransformPermanent and
//     emitPartial for the Super Nova emblem (no engine emblem support).
//
// DFC dispatch: register by both face names per esika.go's pattern.
// After TransformPermanent the card's display name becomes the back face
// name, so the registry's " // " fallback alone wouldn't catch it.
func registerSephiroth(r *Registry) {
	r.OnETB("Sephiroth, Fabled SOLDIER", sephirothETB)
	r.OnETB("Sephiroth, Fabled SOLDIER // Sephiroth, One-Winged Angel", sephirothETB)
	r.OnTrigger("Sephiroth, Fabled SOLDIER", "creature_attacks", sephirothAttack)
	r.OnTrigger("Sephiroth, One-Winged Angel", "creature_attacks", sephirothAttack)
	r.OnTrigger("Sephiroth, Fabled SOLDIER // Sephiroth, One-Winged Angel", "creature_attacks", sephirothAttack)
	r.OnTrigger("Sephiroth, Fabled SOLDIER", "creature_dies", sephirothCreatureDies)
	r.OnTrigger("Sephiroth, Fabled SOLDIER // Sephiroth, One-Winged Angel", "creature_dies", sephirothCreatureDies)
}

func sephirothETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sephiroth_etb_sac_draw"
	if gs == nil || perm == nil {
		return
	}
	if perm.Transformed {
		return
	}
	sephirothMaySacOneDraw(gs, perm, slug, "etb")
}

func sephirothAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm != perm {
		return
	}
	if perm.Transformed {
		sephirothBackAttackSacDraw(gs, perm)
		return
	}
	sephirothMaySacOneDraw(gs, perm, "sephiroth_attack_sac_draw", "attack")
}

// sephirothMaySacOneDraw — front face's "may sacrifice another creature.
// If you do, draw a card." Hat opts in iff a positive-score victim
// exists (chooseSacVictimNotSelf will surface the best candidate; we
// gate on its score to avoid trading a high-value creature for one card).
func sephirothMaySacOneDraw(gs *gameengine.GameState, perm *gameengine.Permanent, slug, cause string) {
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	victim := chooseSacVictimNotSelf(gs, seat, perm, nil)
	if victim == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_other_creature_to_sacrifice", map[string]interface{}{
			"seat":  seat,
			"cause": cause,
		})
		return
	}
	score := sacVictimScore(gs, seat, victim, perm)
	if score < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_worthwhile_sac_target", map[string]interface{}{
			"seat":  seat,
			"cause": cause,
			"score": score,
		})
		return
	}
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "sephiroth_front_"+cause)
	drawn := drawOne(gs, seat, perm.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"cause":      cause,
		"sacrificed": victimName,
		"drawn_card": drawnName,
	})
}

// sephirothBackAttackSacDraw — back face's "may sacrifice any number of
// other creatures. If you do, draw that many cards." Hat picks every
// creature the score function flags as worth sacrificing (score > 0)
// then draws that many cards.
func sephirothBackAttackSacDraw(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sephiroth_back_attack_sac_n_draw_n"
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	var victims []*gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		if sacVictimScore(gs, seat, p, perm) > 0 {
			victims = append(victims, p)
		}
	}
	if len(victims) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_worthwhile_sac_targets", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	names := make([]string, 0, len(victims))
	for _, v := range victims {
		names = append(names, v.Card.DisplayName())
		gameengine.SacrificePermanent(gs, v, "sephiroth_back_attack")
	}
	drawn := 0
	for i := 0; i < len(victims); i++ {
		if c := drawOne(gs, seat, perm.Card.DisplayName()); c != nil {
			drawn++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"sacrificed":  names,
		"sac_count":   len(victims),
		"draw_count":  drawn,
	})
}

func sephirothCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sephiroth_creature_dies_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if perm.Transformed {
		// Back face's drain is an emblem, not a permanent ability —
		// emitPartial flagged it on transform.
		return
	}
	// "Whenever ANOTHER creature dies" — exclude Sephiroth himself.
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if dyingPerm != nil && dyingPerm == perm {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Drain 1 from first alive opponent, gain 1.
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		s.Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Target: -1,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"amount": -1,
				"cause":  "sephiroth_drain",
			},
		})
		break
	}
	gameengine.GainLife(gs, seat, 1, perm.Card.DisplayName())

	// Increment per-turn resolution counter and transform on the 4th.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := fmt.Sprintf("sephiroth_drain_count_t%d", gs.Turn+1)
	perm.Flags[turnKey]++
	count := perm.Flags[turnKey]
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              seat,
		"resolutions_turn":  count,
	})
	if count == 4 {
		if gameengine.TransformPermanent(gs, perm, "sephiroth_fourth_drain") {
			emitPartial(gs, "sephiroth_super_nova_emblem", perm.Card.DisplayName(),
				"super_nova_emblem_creation_unimplemented")
		}
	}
}
