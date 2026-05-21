package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAshlingFlameDancer wires Ashling, Flame Dancer.
//
// Oracle text (Scryfall, verified):
//
//	You don't lose unspent red mana as steps and phases end.
//	Magecraft — Whenever you cast or copy an instant or sorcery spell,
//	discard a card, then draw a card. If this is the second time this
//	ability has resolved this turn, Ashling deals 2 damage to each
//	opponent and each creature they control. If it's the third time,
//	add {R}{R}{R}{R}.
//
// Implementation (R36 stub port):
//   - "Don't lose unspent red mana": handled by the R57 side-handler
//     registerAshlingFlameDancerManaExemption (mana_pool_primitive_r57.go)
//     which calls RegisterManaPoolExemption({R}) at ETB and unregisters
//     on LTB. The R58 stale-partial sweep drops the breadcrumb here
//     since the primitive is wired.
//   - Magecraft trigger: listen on "instant_or_sorcery_cast" gated
//     to caster_seat == controller. The keywords_magecraft.go canonical
//     surface fires this on both real casts AND copies (R28), which
//     matches the "cast OR copy" oracle clause.
//   - State machine: perm.Flags["ashling_resolutions_t<N>"] tracks the
//     per-turn fire count. Each fire performs discard+draw. At the 2nd
//     fire, Ashling deals 2 damage to each opponent and each creature
//     they control. At the 3rd, controller's mana pool gets +4 (R).
//   - Per-turn key uses gs.Turn+1 to avoid zero-collision on turn 0
//     (same pattern as Scorpion Seething Striker, Sand Scout, etc.).
func registerAshlingFlameDancer(r *Registry) {
	r.OnETB("Ashling, Flame Dancer", ashlingFlameDancerETB)
	r.OnTrigger("Ashling, Flame Dancer", "instant_or_sorcery_cast", ashlingFlameDancerMagecraft)
}

func ashlingFlameDancerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ashling_flame_dancer_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func ashlingFlameDancerMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ashling_flame_dancer_magecraft"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Per-turn resolution counter on the permanent (not the seat —
	// each Ashling tracks its own chain if a corpus ever runs two).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := ashlingFlameDancerTurnKey(gs.Turn)
	perm.Flags[turnKey]++
	resolutions := perm.Flags[turnKey]

	// Discard then draw. The discard target is the lowest-MV card in
	// hand (cheapest, least valuable). If the hand is empty we skip
	// the discard but still draw — matches "discard a card, then draw
	// a card" when discard has nothing to act on (printed sequencing).
	discarded := ""
	if len(seat.Hand) > 0 {
		victim := ashlingFlameDancerDiscardPick(seat.Hand)
		if victim != nil {
			discarded = victim.DisplayName()
			gameengine.DiscardCard(gs, victim, perm.Controller)
		}
	}
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}

	// 2nd resolution this turn: deal 2 damage to each opponent and
	// each creature they control. Use MarkedDamage for creatures
	// (SBA cleans up dead creatures next state-based pass).
	dmgPlayers := 0
	dmgCreatures := 0
	if resolutions == 2 {
		for _, opp := range gs.Opponents(perm.Controller) {
			s := gs.Seats[opp]
			if s == nil || s.Lost {
				continue
			}
			gameengine.DealDamage(gs, opp, 2, perm.Card.DisplayName())
			dmgPlayers++
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil || !p.IsCreature() {
					continue
				}
				p.MarkedDamage += 2
				dmgCreatures++
			}
		}
	}

	// 3rd resolution this turn: add {R}{R}{R}{R}.
	addedMana := 0
	if resolutions == 3 {
		gameengine.AddMana(gs, seat, "R", 4, perm.Card.DisplayName())
		addedMana = 4
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"resolutions":   resolutions,
		"discarded":     discarded,
		"drawn":         drawnName,
		"dmg_players":   dmgPlayers,
		"dmg_creatures": dmgCreatures,
		"added_mana":    addedMana,
	})
}

// ashlingFlameDancerDiscardPick returns the lowest-MV card from hand.
// Ties go to the first-found card in iteration order (which is the
// deck-add order — stable across reruns).
func ashlingFlameDancerDiscardPick(hand []*gameengine.Card) *gameengine.Card {
	var pick *gameengine.Card
	bestCMC := -1
	for _, c := range hand {
		if c == nil {
			continue
		}
		cmc := cardCMC(c)
		if pick == nil || cmc < bestCMC {
			pick = c
			bestCMC = cmc
		}
	}
	return pick
}

func ashlingFlameDancerTurnKey(turn int) string {
	return fmt.Sprintf("ashling_resolutions_t%d", turn+1)
}
