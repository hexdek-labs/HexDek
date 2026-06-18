package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// bargain_consumers_r60.go — per_card handlers for the WOE Bargain
// keyword family (CR §702.166, ability word "bargain").
//
// Per the Versailles Phase 1B audit (PR #477 §3) `bargain_paid` is an
// engine-emitted event with no per_card consumer wired despite a
// substantial WOE card pool whose oracle text relies on
// "if this spell was bargained" or "when this enters, if it was
// bargained" riders. The engine handles the cost itself via
// keywords_bargain.go's CastBargained — it pushes a StackItem flagged
// `CostMeta["bargained"] = true` and fires
// FireCardTrigger("bargain_paid", ctx) — but no per_card handler
// consumed either signal, leaving the bargain riders dead code in
// production.
//
// This file wires 6 handlers covering both shapes:
//
//   (A) Instants / sorceries with resolve-time riders — register
//       OnResolve; read item.CostMeta["bargained"] directly to gate
//       the enhanced effect. No cast-time bridge needed because the
//       StackItem is still available at resolution.
//
//   (B) Permanent spells with ETB riders — register OnETB and read
//       perm.Flags["bargained"], which the engine mirrors from the
//       resolving spell's CostMeta at ETB via MirrorBargainToPermanent
//       (CR §702.176c — the bargained state travels with the permanent,
//       mirroring kicker/squad). Reading it off THE PERMANENT that
//       entered is leak-free: a bargained spell countered before it
//       enters never stamps a flag, and two simultaneous casts each carry
//       their own decision — neither edge the old per-seat cast counter
//       handled.

func init() {
	registerBargainConsumersR60(Global())
	AddResetHook(registerBargainConsumersR60)
}

func registerBargainConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	// (A) Resolve-time riders.
	r.OnResolve("Torch the Tower", torchTheTowerResolve)
	r.OnResolve("Candy Grapple", candyGrappleResolve)
	r.OnResolve("Archon's Glory", archonsGloryResolve)
	// (B) ETB riders — gated on the engine-mirrored perm.Flags["bargained"].
	r.OnETB("Troublemaker Ouphe", troublemakerOupheETB)
	r.OnETB("High Fae Negotiator", highFaeNegotiatorETB)
	r.OnETB("Tenacious Tomeseeker", tenaciousTomeseekerETB)
}

// -----------------------------------------------------------------------------
// Bargained-permanent gate
// -----------------------------------------------------------------------------

// permWasBargained reports whether the permanent entered via a bargained
// cast. The engine mirrors the resolving spell's CostMeta["bargained"] onto
// perm.Flags["bargained"] at ETB (MirrorBargainToPermanent, CR §702.176c),
// so the ETB rider reads the decision made for exactly this permanent.
func permWasBargained(perm *gameengine.Permanent) bool {
	return perm != nil && perm.Flags != nil && perm.Flags["bargained"] > 0
}

// -----------------------------------------------------------------------------
// Resolve-time target pickers
// -----------------------------------------------------------------------------

// pickBestOppCreature returns the highest-power creature controlled by
// any of seat's opponents. Used by damage / -X/-X effects whose
// "target creature" should kill the biggest threat.
func pickBestOppCreature(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestPower := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if pw := p.Power(); pw > bestPower {
				bestPower = pw
				best = p
			}
		}
	}
	return best
}

// pickBestFriendlyCreature returns the highest-power creature seat
// controls. Used by buff effects whose "target creature you control"
// should amplify the biggest threat.
func pickBestFriendlyCreature(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if pw := p.Power(); pw > bestPower {
			bestPower = pw
			best = p
		}
	}
	return best
}

// pickBestOppArtifactOrEnchantment returns the highest-CMC artifact or
// enchantment controlled by an opponent. Used by Troublemaker Ouphe's
// bargain rider.
func pickBestOppArtifactOrEnchantment(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestCMC := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !p.IsArtifact() && !p.IsEnchantment() {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc > bestCMC {
				bestCMC = cmc
				best = p
			}
		}
	}
	return best
}

// -----------------------------------------------------------------------------
// (A) Resolve-time riders
// -----------------------------------------------------------------------------

// Torch the Tower — {R} Instant, Bargain.
//
// Deals 2 damage to target creature or planeswalker. If bargained,
// deals 3 damage and you scry 1. (Exile-on-death rider is the
// engine's job via the death-replacement registration; this handler
// only models the damage + scry.)
func torchTheTowerResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "torch_the_tower"
	if gs == nil || item == nil {
		return
	}
	target := pickBestOppCreature(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Torch the Tower", "no_target", map[string]interface{}{
			"seat": item.Controller,
		})
		return
	}
	bargained, _ := item.CostMeta["bargained"].(bool)
	damage := 2
	if bargained {
		damage = 3
	}
	target.MarkedDamage += damage
	gs.InvalidateCharacteristicsCache()
	if bargained {
		gameengine.Scry(gs, item.Controller, 1)
	}
	emit(gs, slug, "Torch the Tower", map[string]interface{}{
		"seat":      item.Controller,
		"target":    target.Card.DisplayName(),
		"damage":    damage,
		"bargained": bargained,
	})
}

// Candy Grapple — {1}{B} Instant, Bargain.
//
// Target creature gets -3/-3 UEOT. If bargained, -5/-5 instead.
func candyGrappleResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "candy_grapple"
	if gs == nil || item == nil {
		return
	}
	target := pickBestOppCreature(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Candy Grapple", "no_target", map[string]interface{}{
			"seat": item.Controller,
		})
		return
	}
	bargained, _ := item.CostMeta["bargained"].(bool)
	penalty := 3
	if bargained {
		penalty = 5
	}
	ts := gs.NextTimestamp()
	target.Modifications = append(target.Modifications, gameengine.Modification{
		Power:     -penalty,
		Toughness: -penalty,
		Duration:  "until_end_of_turn",
		Timestamp: ts,
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, "Candy Grapple", map[string]interface{}{
		"seat":      item.Controller,
		"target":    target.Card.DisplayName(),
		"penalty":   penalty,
		"bargained": bargained,
	})
}

// Archon's Glory — {W} Instant, Bargain.
//
// Target creature gets +2/+2 UEOT. If bargained, it also gains flying
// and lifelink until end of turn.
func archonsGloryResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "archons_glory"
	if gs == nil || item == nil {
		return
	}
	target := pickBestFriendlyCreature(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Archon's Glory", "no_target", map[string]interface{}{
			"seat": item.Controller,
		})
		return
	}
	bargained, _ := item.CostMeta["bargained"].(bool)
	ts := gs.NextTimestamp()
	target.Modifications = append(target.Modifications, gameengine.Modification{
		Power:     2,
		Toughness: 2,
		Duration:  "until_end_of_turn",
		Timestamp: ts,
	})
	if bargained {
		if target.Flags == nil {
			target.Flags = map[string]int{}
		}
		target.Flags["kw:flying"] = 1
		target.Flags["kw:lifelink"] = 1
		target.GrantedAbilities = append(target.GrantedAbilities, "flying", "lifelink")
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, "Archon's Glory", map[string]interface{}{
		"seat":      item.Controller,
		"target":    target.Card.DisplayName(),
		"bargained": bargained,
	})
}

// -----------------------------------------------------------------------------
// (B) Permanent ETB riders (cast→ETB bridge)
// -----------------------------------------------------------------------------

// Troublemaker Ouphe — {1}{G} Creature — Ouphe, Bargain.
//
// When this enters, if it was bargained, exile target artifact or
// enchantment an opponent controls.
func troublemakerOupheETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "troublemaker_ouphe_bargain_exile"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if !permWasBargained(perm) {
		// Non-bargained cast → ETB rider does nothing.
		return
	}
	target := pickBestOppArtifactOrEnchantment(gs, perm.Controller)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	gameengine.ExilePermanent(gs, target, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"exiled": target.Card.DisplayName(),
	})
}

// High Fae Negotiator — {3}{B}{B} Creature — Faerie Warlock (Flying), Bargain.
//
// When this enters, if it was bargained, each opponent loses 3 life
// and you gain 3 life.
func highFaeNegotiatorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "high_fae_negotiator_drain"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if !permWasBargained(perm) {
		return
	}
	hit := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.LoseLife(gs, opp, 3, perm.Card.DisplayName())
		hit++
	}
	gameengine.GainLife(gs, perm.Controller, 3, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"opps_hit": hit,
	})
}

// Tenacious Tomeseeker — {2}{U} Creature — Human Knight, Bargain.
//
// When this enters, if it was bargained, return target instant or
// sorcery card from your graveyard to your hand.
func tenaciousTomeseekerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tenacious_tomeseeker_recur"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if !permWasBargained(perm) {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	target := pickHighestCMCInstantOrSorcery(seat.Graveyard, nil)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	gameengine.MoveCard(gs, target, perm.Controller, "graveyard", "hand", "tenacious_tomeseeker_return")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.DisplayName(),
	})
}
