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
//   (B) Permanent spells with ETB riders — register OnCast to stash a
//       per-seat "this card was bargained" counter at cast time, then
//       OnETB reads + decrements the counter to gate the bargained
//       branch of the ETB. The cast→ETB bridge is necessary because
//       OnETB receives only the Permanent (no CostMeta access).

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
	// (B) Cast→ETB bridge + ETB rider.
	r.OnCast("Troublemaker Ouphe", bargainCastBridge("troublemaker_ouphe"))
	r.OnETB("Troublemaker Ouphe", troublemakerOupheETB)
	r.OnCast("High Fae Negotiator", bargainCastBridge("high_fae_negotiator"))
	r.OnETB("High Fae Negotiator", highFaeNegotiatorETB)
	r.OnCast("Tenacious Tomeseeker", bargainCastBridge("tenacious_tomeseeker"))
	r.OnETB("Tenacious Tomeseeker", tenaciousTomeseekerETB)
}

// -----------------------------------------------------------------------------
// Cast→ETB bridge
// -----------------------------------------------------------------------------

// bargainCastBridge returns a CastHandler that, when the cast is
// bargained, increments a per-seat flag keyed by `slug`. The matching
// OnETB handler reads + decrements that flag to decide whether to run
// the bargain branch of its ETB rider.
//
// The counter (not bool) handles the multi-cast-same-turn edge: if you
// cast Troublemaker Ouphe twice in one turn and bargain both, the
// counter goes 2→1→0 as each Ouphe resolves and ETBs.
func bargainCastBridge(slug string) CastHandler {
	flag := "bargained_pending:" + slug
	return func(gs *gameengine.GameState, item *gameengine.StackItem) {
		if gs == nil || item == nil || item.CostMeta == nil {
			return
		}
		if v, _ := item.CostMeta["bargained"].(bool); !v {
			return
		}
		seat := item.Controller
		if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
			return
		}
		if gs.Seats[seat].Flags == nil {
			gs.Seats[seat].Flags = map[string]int{}
		}
		gs.Seats[seat].Flags[flag]++
	}
}

// consumeBargainFlag pops one bargain credit from the seat's flag
// counter. Returns true if a credit was found (caller should run the
// bargained branch).
func consumeBargainFlag(gs *gameengine.GameState, seat int, slug string) bool {
	flag := "bargained_pending:" + slug
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return false
	}
	s := gs.Seats[seat]
	if s.Flags == nil || s.Flags[flag] <= 0 {
		return false
	}
	s.Flags[flag]--
	if s.Flags[flag] == 0 {
		delete(s.Flags, flag)
	}
	return true
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
	if !consumeBargainFlag(gs, perm.Controller, "troublemaker_ouphe") {
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
	if !consumeBargainFlag(gs, perm.Controller, "high_fae_negotiator") {
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
	if !consumeBargainFlag(gs, perm.Controller, "tenacious_tomeseeker") {
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
