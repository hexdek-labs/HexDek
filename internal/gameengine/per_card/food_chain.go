package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFoodChain wires up Food Chain.
//
// Oracle text:
//
//	Exile a creature you control: Add an amount of mana of any one
//	color equal to the exiled creature's mana value plus one. Spend
//	this mana only to cast creature spells.
//
// Infinite-mana enabler. Combined with a creature that returns itself
// from exile to hand (Misthollow Griffin / Eternal Scourge / Squee,
// the Immortal), this loops: exile Griffin (mana_value=4) → get 5 mana
// → cast Griffin for 4 → exile again → net +1 mana per loop, unbounded
// creature-only mana.
//
// Batch #1 scope:
//   - Implement the activated ability: pick a target creature by ctx or
//     highest-CMC; exile it; add CMC+1 mana to the controller's pool.
//   - "Spend only on creatures" restriction is NOT modeled — the engine
//     has a single mana bucket. We log a partial. Real tracking would
//     need a mana-pool type overhaul; out of scope for this batch.
//
// Activation contract:
//
//	ctx["creature_perm"] *gameengine.Permanent  — which creature to exile.
//	                                             When absent, pick the
//	                                             highest-CMC creature.
func registerFoodChain(r *Registry) {
	r.OnActivated("Food Chain", foodChainActivate)
}

func foodChainActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "food_chain_exile_for_mana"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Pick the victim creature.
	var victim *gameengine.Permanent
	if v, ok := ctx["creature_perm"].(*gameengine.Permanent); ok && v != nil {
		victim = v
	} else {
		// Highest-CMC creature under controller. "CMC" is modeled via
		// a heuristic: BasePower + BaseToughness as a proxy (we don't
		// have a real CMC field on Card yet); tests can set it via
		// Card.Types with "cmc:N".
		best := -1
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc > best {
				best = cmc
				victim = p
			}
		}
	}
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_to_exile", nil)
		return
	}

	cmc := cardCMC(victim.Card)
	manaGained := cmc + 1

	// Exile the creature via ExilePermanent for proper zone-change handling:
	// replacement effects, LTB triggers, commander redirect.
	gameengine.ExilePermanent(gs, victim, src)

	// Add mana. MVP: generic bucket.
	s.ManaPool += manaGained
	gameengine.SyncManaAfterAdd(s, manaGained)

	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   seat,
		Target: seat,
		Source: src.Card.DisplayName(),
		Amount: manaGained,
		Details: map[string]interface{}{
			"reason":        "food_chain_exile",
			"exiled_card":   victim.Card.DisplayName(),
			"exiled_cmc":    cmc,
			"new_pool":      s.ManaPool,
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"exiled_card":  victim.Card.DisplayName(),
		"mana_gained":  manaGained,
		"new_pool":     s.ManaPool,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"spend_only_on_creature_spells_restriction_not_enforced")
}

// cardCMC reads the card's mana value. Prefers an explicit "cmc:N"
// token in Card.Types (test-friendly); falls back to BasePower +
// BaseToughness as a rough proxy for creatures; else 0.
func cardCMC(c *gameengine.Card) int {
	if c == nil {
		return 0
	}
	for _, t := range c.Types {
		if len(t) > 4 && t[:4] == "cmc:" {
			n := 0
			for _, ch := range t[4:] {
				if ch < '0' || ch > '9' {
					break
				}
				n = n*10 + int(ch-'0')
			}
			return n
		}
	}
	// Proxy for test creatures without cmc: marker.
	if c.BasePower > 0 || c.BaseToughness > 0 {
		// Rough: power+toughness divided by 2 (MTG cards are usually
		// slightly cheaper than p+t).
		p := c.BasePower + c.BaseToughness
		if p > 0 {
			return (p + 1) / 2
		}
	}
	return 0
}
