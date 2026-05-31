package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGhenArcanumWeaverCustom implements Ghen's enchantment-recursion
// activation. The auto-generated stub is a no-op.
//
// Oracle text:
//
//	{R}{W}{B}, {T}, Sacrifice an enchantment: Return target enchantment
//	card from your graveyard to the battlefield.
//
// Implementation notes:
//   - Cost: {R}{W}{B} = 3 generic + tap + sac an enchantment we control.
//     Engine activation dispatch handles the {RWB}; defensive in-handler
//     check on ManaPool covers non-engine callers.
//   - Sacrifice cost: pick the worst enchantment we control to sac
//     (lowest CMC, prefers non-Ghen-itself). If we control no other
//     enchantment, fail.
//   - Effect: pick the highest-CMC enchantment in our graveyard and
//     return it to the battlefield via enterBattlefieldWithETB.
//   - Sacrifice cost is the resolution cost (cost-unenforced caveat
//     from the structure audit, batch 2): the engine doesn't gate
//     activation on sacrifice availability before dispatch, so we
//     enforce it ourselves.
func registerGhenArcanumWeaverCustom(r *Registry) {
	r.OnActivated("Ghen, Arcanum Weaver", ghenEnchantmentRecursion)
}

func ghenEnchantmentRecursion(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ghen_enchantment_recursion"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	if seat.ManaPool < 3 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  3,
			"available": seat.ManaPool,
		})
		return
	}

	// Pick the worst enchantment we control to sac. Lowest CMC; tiebreak
	// newest Timestamp (least established). Excludes Ghen himself.
	var sacPerm *gameengine.Permanent
	worstCMC := 1 << 30
	worstTS := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == src {
			continue
		}
		if !p.IsEnchantment() {
			continue
		}
		c := cardCMC(p.Card)
		if c < worstCMC || (c == worstCMC && p.Timestamp > worstTS) {
			worstCMC = c
			worstTS = p.Timestamp
			sacPerm = p
		}
	}
	if sacPerm == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_enchantment_to_sacrifice", nil)
		return
	}

	// Pick best enchantment in graveyard (highest CMC).
	var pick *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "enchantment") {
			continue
		}
		if cmc := cardCMC(c); cmc > bestCMC {
			pick = c
			bestCMC = cmc
		}
	}
	if pick == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_enchantment_in_graveyard", nil)
		return
	}

	// Pay costs.
	seat.ManaPool -= 3
	gameengine.SyncManaAfterSpend(seat)
	src.Tapped = true
	sacName := sacPerm.Card.DisplayName()
	gameengine.SacrificePermanent(gs, sacPerm, "ghen_sac_cost")

	// Effect: return enchantment from GY → battlefield. createPermanent
	// (called via enterBattlefieldWithETB) sweeps the source from every
	// private zone, so the manual splice is redundant (Wave 2 final-audit
	// cleanup).
	enterBattlefieldWithETB(gs, src.Controller, pick, false)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       src.Controller,
		"sacrificed": sacName,
		"returned":   pick.DisplayName(),
		"cmc":        bestCMC,
	})
}
