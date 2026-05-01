package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGhaveGuruOfSpores wires Ghave, Guru of Spores.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary Creature — Fungus Shaman. 0/0.
//	Ghave, Guru of Spores enters the battlefield with five +1/+1
//	counters on it.
//	{1}, Remove a +1/+1 counter from a creature you control: Create a
//	  1/1 green Saproling creature token.
//	{1}, Sacrifice a creature: Put a +1/+1 counter on target creature.
//
// Ghave is the canonical Abzan +1/+1 / Saproling combo commander. Loops
// against persist creatures (Murderous Redcap, Cauldron of Souls) and
// pairs with Doubling Season / Hardened Scales for runaway value. The
// activations are mana abilities only insofar as they don't produce
// mana — they go on the stack normally.
//
// Implementation:
//   - OnETB: pin counters to 5 (CR §122.2 — replacement effects on the
//     printed "enters with N counters" clause are layered into the entry,
//     and Doubling Season / Hardened Scales / Pir would already have
//     adjusted the count via the engine's counter-replacement pipeline if
//     wired; we set the printed value directly here).
//   - OnActivated(0): {1}, Remove a +1/+1 counter from a creature you
//     control → mint a 1/1 green Saproling token. Picks ctx["counter_perm"]
//     if supplied (the hat's preferred donor); otherwise picks any
//     creature you control with a +1/+1 counter, biasing to non-Ghave
//     fodder so Ghave's own body isn't drained first.
//   - OnActivated(1): {1}, Sacrifice a creature → put a +1/+1 counter on
//     target creature. ctx["sac_perm"] is the sac fodder (token preferred);
//     ctx["target_perm"] receives the counter (defaults to Ghave to grow
//     her body and feed activation 0 again).
func registerGhaveGuruOfSpores(r *Registry) {
	r.OnETB("Ghave, Guru of Spores", ghaveETB)
	r.OnActivated("Ghave, Guru of Spores", ghaveActivate)
}

func ghaveETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	perm.AddCounter("+1/+1", 5)
	gs.InvalidateCharacteristicsCache()
	emit(gs, "ghave_guru_of_spores_etb_counters", perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": perm.Counters["+1/+1"],
	})
}

func ghaveActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		ghaveCounterToToken(gs, src, ctx)
	case 1:
		ghaveSacForCounter(gs, src, ctx)
	}
}

func ghaveCounterToToken(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ghave_remove_counter_for_saproling"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	donor := ghavePickCounterDonor(gs, src, ctx)
	if donor == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_with_plus1_counter", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	donor.AddCounter("+1/+1", -1)
	gs.InvalidateCharacteristicsCache()

	token := &gameengine.Card{
		Name:          "Saproling Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"creature", "token", "saproling", "pip:G"},
		Colors:        []string{"G"},
		TypeLine:      "Token Creature — Saproling",
	}
	enterBattlefieldWithETB(gs, seat, token, false)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"donor":         donor.Card.DisplayName(),
		"donor_counters": donor.Counters["+1/+1"],
		"token":         "Saproling Token",
	})
}

func ghaveSacForCounter(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ghave_sac_creature_for_counter"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := ghavePickSacFodder(gs, src, ctx)
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_to_sacrifice", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	target := ghavePickCounterRecipient(gs, src, victim, ctx)
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_to_target", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "ghave_sac_for_counter")
	target.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            seat,
		"sacrificed":      victimName,
		"target":          target.Card.DisplayName(),
		"target_counters": target.Counters["+1/+1"],
	})
}

// ghavePickCounterDonor returns the creature to remove a +1/+1 counter from.
// Priority: ctx-supplied "counter_perm", then any non-Ghave creature with a
// +1/+1 counter (token-bodies first as fodder), then Ghave herself as a
// last resort.
func ghavePickCounterDonor(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if ctx != nil {
		if p, ok := ctx["counter_perm"].(*gameengine.Permanent); ok && p != nil {
			if p.Controller == src.Controller && p.IsCreature() && p.Counters["+1/+1"] > 0 {
				return p
			}
		}
	}
	s := gs.Seats[src.Controller]
	if s == nil {
		return nil
	}
	var ghaveSelf *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Counters == nil || p.Counters["+1/+1"] <= 0 {
			continue
		}
		if p == src {
			ghaveSelf = p
			continue
		}
		// Found a non-Ghave creature with a counter — prefer it.
		return p
	}
	return ghaveSelf
}

// ghavePickSacFodder returns the cheapest expendable creature to sacrifice.
// Priority: ctx-supplied "sac_perm", then token creatures (Saprolings from
// activation 0 are perfect fodder), then lowest-CMC non-Ghave creature.
func ghavePickSacFodder(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if ctx != nil {
		if p, ok := ctx["sac_perm"].(*gameengine.Permanent); ok && p != nil {
			if p.Controller == src.Controller && p.IsCreature() {
				return p
			}
		}
	}
	s := gs.Seats[src.Controller]
	if s == nil {
		return nil
	}
	// Pass 1: token creatures.
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil || !p.IsCreature() {
			continue
		}
		if cardHasType(p.Card, "token") {
			return p
		}
	}
	// Pass 2: lowest-CMC non-Ghave non-commander creature.
	var best *gameengine.Permanent
	bestCMC := 1<<31 - 1
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil || !p.IsCreature() {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc < bestCMC {
			bestCMC = cmc
			best = p
		}
	}
	return best
}

// ghavePickCounterRecipient returns the creature to receive the +1/+1
// counter from activation 1. Priority: ctx-supplied "target_perm" (must be
// a creature), then Ghave herself (combo line: grow her so activation 0
// has more donors), then any other creature you control.
func ghavePickCounterRecipient(gs *gameengine.GameState, src, victim *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if ctx != nil {
		if p, ok := ctx["target_perm"].(*gameengine.Permanent); ok && p != nil {
			if p != victim && p.IsCreature() {
				return p
			}
		}
	}
	// Ghave herself if she's still on the battlefield.
	if src != nil && src.IsCreature() && src != victim {
		return src
	}
	// Fall back to first remaining creature.
	for _, sp := range gs.Seats {
		if sp == nil {
			continue
		}
		for _, p := range sp.Battlefield {
			if p == nil || p == victim || p.Card == nil || !p.IsCreature() {
				continue
			}
			if p.Controller == src.Controller {
				return p
			}
		}
	}
	return nil
}
