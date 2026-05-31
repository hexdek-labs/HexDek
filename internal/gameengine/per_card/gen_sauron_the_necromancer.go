package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSauronTheNecromancer wires Sauron, the Necromancer.
//
// Oracle text:
//
//	Menace
//	Whenever Sauron attacks, exile target creature card from your
//	graveyard. Create a tapped and attacking token that's a copy of
//	that card, except it's a 3/3 black Wraith with menace. At the
//	beginning of the next end step, exile that token unless Sauron is
//	your Ring-bearer.
//
// Implementation:
//   - "creature_attacks" trigger gated on Sauron being the attacker.
//     Picks the highest-CMC creature card in the controller's graveyard
//     as the target. Exiles it, mints a 3/3 black Wraith token copy
//     (tapped + attacking + menace) inheriting Sauron's attack target.
//   - Schedules an end-of-turn delayed trigger that exiles the token
//     unless Sauron is Ring-bearer (perm.Flags["ring_bearer"]).
//   - Menace static handled by the AST keyword pipeline.
func registerSauronTheNecromancer(r *Registry) {
	r.OnTrigger("Sauron, the Necromancer", "creature_attacks", sauronTheNecromancerAttack)
}

func sauronTheNecromancerAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sauron_necromancer_attack_wraith"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Pick the best (highest-CMC) creature card from graveyard.
	bestIdx := -1
	bestCMC := -1
	for i, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_creature_in_graveyard", nil)
		return
	}
	chosen := seat.Graveyard[bestIdx]
	// Exile the chosen card.
	gameengine.MoveCard(gs, chosen, perm.Controller, "graveyard", "exile", "sauron_necro_exile_target")

	// Build a 3/3 black Wraith token copy with menace.
	tokenTypes := append([]string{"creature", "token", "wraith", "pip:B", "kw:menace"}, chosen.Types...)
	tokenCard := &gameengine.Card{
		Name:          chosen.DisplayName() + " (Wraith token)",
		Owner:         perm.Controller,
		Types:         tokenTypes,
		Colors:        []string{"B"},
		BasePower:     3,
		BaseToughness: 3,
	}
	tokenPerm := createPermanent(gs, perm.Controller, tokenCard, true)
	if tokenPerm == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_create_failed", nil)
		return
	}
	gameengine.RegisterReplacementsForPermanent(gs, tokenPerm)
	gameengine.FirePermanentETBTriggers(gs, tokenPerm)
	if tokenPerm.Flags == nil {
		tokenPerm.Flags = map[string]int{}
	}
	// CR §508.1g — Sauron creates the Nazgûl token attacking.
	// MarkEnteredAttacking stamps both flagAttacking and the carve-out
	// tag so the legality invariant honors the bypass.
	gameengine.MarkEnteredAttacking(tokenPerm)
	tokenPerm.Flags["kw:menace"] = 1
	tokenPerm.SummoningSick = false
	if def, ok := gameengine.AttackerDefender(perm); ok {
		gameengine.SetAttackerDefender(tokenPerm, def)
	}

	// EOS exile delayed trigger — unless Sauron is Ring-bearer.
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			// Skip exile if Sauron is Ring-bearer.
			if perm.Flags != nil && perm.Flags["ring_bearer"] == 1 {
				return
			}
			gameengine.MoveCard(gs, tokenCard, perm.Controller, "battlefield", "exile", "sauron_necro_eos_exile")
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copied": chosen.DisplayName(),
		"token":  "3/3 black Wraith with menace",
	})
}
