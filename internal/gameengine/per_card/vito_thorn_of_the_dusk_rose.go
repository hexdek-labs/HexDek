package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVitoThornOfTheDuskRose wires Vito, Thorn of the Dusk Rose.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Whenever you gain life, target opponent loses that much life.
//	{3}{B}{B}: Creatures you control gain lifelink until end of turn.
//
// Implementation:
//   - OnTrigger("life_gained"): when Vito's controller gains life, the
//     opponent with the highest life total loses that much life. This is
//     a simplification of "target opponent" — the hat targets the most
//     threatening opponent automatically.
//   - OnActivated(0): {3}{B}{B} — grant lifelink to every creature the
//     controller controls until end of turn via kw:lifelink flags.
//     A "next_end_step" delayed trigger strips the flags.
//
// Coverage gaps (emitPartial):
//   - None for the drain trigger (fully modelled).
//   - Lifelink grant: relies on kw:lifelink flag checked by
//     HasKeyword("lifelink") in the combat system; creatures that enter
//     after activation do NOT gain lifelink (matches oracle: "creatures
//     you control" is a snapshot at resolution).
func registerVitoThornOfTheDuskRose(r *Registry) {
	r.OnTrigger("Vito, Thorn of the Dusk Rose", "life_gained", vitoThornLifeGained)
	r.OnActivated("Vito, Thorn of the Dusk Rose", vitoThornActivated)
}

// vitoThornLifeGained drains the opponent with the highest life total
// for the amount of life gained. Identical behaviour to vitoDrainTrigger
// in drain_commanders.go (which still serves Vito, Fanatic of Aclazotz
// and future drain variants). Duplicated here so that the standalone
// file is self-contained and the drain_commanders registration for this
// card name is removed.
func vitoThornLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vito_thorn_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: only fire when Vito's controller gained the life.
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller {
		return
	}

	// Read the life gain amount from context.
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}

	// Pick the opponent with the highest life total.
	opps := gs.Opponents(perm.Controller)
	if len(opps) == 0 {
		return
	}
	bestOpp := opps[0]
	bestLife := gs.Seats[opps[0]].Life
	for _, opp := range opps[1:] {
		if gs.Seats[opp] == nil {
			continue
		}
		if gs.Seats[opp].Life > bestLife {
			bestLife = gs.Seats[opp].Life
			bestOpp = opp
		}
	}

	// Apply life loss.
	gs.Seats[bestOpp].Life -= amount
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   bestOpp,
		Source: perm.Card.DisplayName(),
		Amount: amount,
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"target":    bestOpp,
		"life_lost": amount,
	})

	gs.CheckEnd()
}

// vitoThornActivated dispatches activated abilities by index.
func vitoThornActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	switch abilityIdx {
	case 0:
		vitoThornGrantLifelink(gs, src)
	}
}

// vitoThornGrantLifelink implements {3}{B}{B}: Creatures you control
// gain lifelink until end of turn.
func vitoThornGrantLifelink(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "vito_thorn_3bb_lifelink_grant"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Cost: {3}{B}{B} = 5 total mana (3 generic + 2 black).
	// The engine's mana pool is colour-collapsed; check pool >= 5.
	const cost = 5
	if s.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      seat,
			"required":  cost,
			"available": s.ManaPool,
		})
		return
	}
	s.ManaPool -= cost
	gameengine.SyncManaAfterSpend(s)

	// Grant lifelink to every creature the controller controls.
	// Track which creatures we granted it to so we only strip flags we set.
	var granted []*gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		// Skip creatures that already have lifelink (from AST or prior grant).
		if p.HasKeyword("lifelink") {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:lifelink"] = 1
		granted = append(granted, p)
	}

	if len(granted) == 0 {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":          seat,
			"cost_paid":     cost,
			"granted_count": 0,
			"note":          "no_creatures_or_all_already_have_lifelink",
		})
		return
	}

	// Schedule end-of-turn cleanup to remove granted lifelink flags.
	captured := make([]*gameengine.Permanent, len(granted))
	copy(captured, granted)
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seat,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			for _, p := range captured {
				if p == nil || p.Flags == nil {
					continue
				}
				delete(p.Flags, "kw:lifelink")
			}
		},
	})

	names := make([]string, 0, len(granted))
	for _, p := range granted {
		names = append(names, p.Card.DisplayName())
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"cost_paid":     cost,
		"granted_count": len(granted),
		"granted_to":    names,
		"duration":      "until_end_of_turn",
	})
}
