package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheUrDragon wires The Ur-Dragon.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Eminence — As long as The Ur-Dragon is in the command zone or on
//	the battlefield, other Dragon spells you cast cost {1} less to
//	cast.
//	Flying
//	Whenever one or more Dragons you control attack, draw that many
//	cards, then you may put a permanent card from your hand onto the
//	battlefield.
//
// Implementation:
//   - Eminence — handled in cost_modifiers.go (command-zone preamble +
//     battlefield switch entry under "The Ur-Dragon"). Dragon spells the
//     controller casts that aren't Ur-Dragon herself get {1} off.
//   - Flying — handled by the AST keyword pipeline.
//   - "Whenever one or more Dragons you control attack" — the engine
//     fires `creature_attacks` once per declared attacker, but the
//     printed trigger groups all attackers into a single resolution. We
//     hook `creature_attacks` and gate on a per-turn flag so we only
//     fire once per combat. At that point every declared attacker is
//     already flagged `attacking=true`, so we can count Dragons in a
//     single sweep of the controller's battlefield.
//   - Effect: draw N (Dragons attacking), then drop the highest-MV
//     permanent card from hand onto the battlefield (the "may" defaults
//     to YES — a free permanent at sorcery speed is strictly value).
func registerTheUrDragon(r *Registry) {
	r.OnTrigger("The Ur-Dragon", "creature_attacks", theUrDragonAttacks)
}

func theUrDragonAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_ur_dragon_attack_draw_cheat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Engine fires this trigger once per declared attacker. Gate via a
	// per-turn flag so the grouped trigger fires exactly once per combat.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := "ur_dragon_attack_fired_t" + strconv.Itoa(gs.Turn)
	if perm.Flags[key] > 0 {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Count attacking Dragons under perm.Controller's control.
	dragons := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsAttacking() {
			continue
		}
		if !cardHasType(p.Card, "creature") {
			continue
		}
		if !cardHasType(p.Card, "dragon") {
			continue
		}
		dragons++
	}
	if dragons == 0 {
		// Trigger fired but no Dragons attacking — the active attacker
		// must have been a non-Dragon. Don't lock the per-turn key so a
		// later combat phase with Dragons can still fire.
		return
	}
	perm.Flags[key] = 1

	// Step 1: draw N cards.
	drawn := 0
	for i := 0; i < dragons && len(seat.Library) > 0; i++ {
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, perm.Controller, "library", "hand", "draw")
		drawn++
	}

	// Step 2: may put a permanent from hand onto the battlefield. Greedy
	// choice — pick the highest-CMC permanent we can find (best free drop).
	bestIdx := -1
	bestCMC := -1
	for i, c := range seat.Hand {
		if c == nil {
			continue
		}
		if !theUrDragonIsPermanent(c) {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	cheatedName := ""
	if bestIdx >= 0 {
		card := seat.Hand[bestIdx]
		seat.Hand = append(seat.Hand[:bestIdx], seat.Hand[bestIdx+1:]...)
		enterBattlefieldWithETB(gs, perm.Controller, card, false)
		cheatedName = card.DisplayName()
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"attacking_dragons": dragons,
		"drawn":           drawn,
		"cheated":         cheatedName,
		"cheated_cmc":     bestCMC,
	})
}

// theUrDragonIsPermanent returns true if the card has any permanent type
// (creature, artifact, enchantment, planeswalker, battle, or land).
func theUrDragonIsPermanent(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if cardHasType(c, "creature") || cardHasType(c, "artifact") ||
		cardHasType(c, "enchantment") || cardHasType(c, "planeswalker") ||
		cardHasType(c, "battle") || cardHasType(c, "land") {
		return true
	}
	return false
}
