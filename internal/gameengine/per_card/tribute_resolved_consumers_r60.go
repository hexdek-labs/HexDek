package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// tribute_resolved_consumers_r60.go — per_card OnTrigger handlers for
// the Born of the Gods "Tribute N" mechanic (CR §702.121, BNG 2014).
//
// Per the Versailles Phase 1B audit, `tribute_resolved` is the last
// Phase 1B engine-emitted event with no per_card consumer wired. The
// engine has the full plumbing — keywords_tribute.go::ApplyTribute
// drives the §702.121a opponent-decide choice, records the decision
// on perm.Flags, and fires FireCardTrigger("tribute_resolved", ctx)
// with `perm` / `card_name` / `controller_seat` / `opponent_seat` /
// `tribute_n` / `accepted` — but no per_card handler consumed it, so
// every printed "if tribute wasn't paid, ..." rider in the BNG corpus
// was inert (creatures got the counters when opponents accepted, but
// no fallback payoff when refused).
//
// This file wires 6 handlers covering the canonical refused-payoff
// shapes:
//
//   - Pharagax Giant      — refused → 5 damage to each opponent
//   - Snake of the        — refused → controller gains 4 life
//     Golden Grove
//   - Ornitharch          — refused → create two 1/1 white Bird tokens
//                           with flying
//   - Nessian Demolok     — refused → destroy target noncreature
//                           permanent (highest-CMC opp pick)
//   - Fanatic of Xenagos  — refused → self gets +1/+1 and haste UEOT
//   - Thunder Brute       — refused → self gains haste UEOT
//
// All handlers gate on:
//   (a) perm == ctx["perm"] — the dispatcher walks the battlefield by
//       name; with two copies of the same tribute bearer on board, only
//       the bearer whose ETB drove this ApplyTribute call should fire
//       its refused-payoff.
//   (b) accepted == false — when tribute was paid, the engine added the
//       counters and CR §702.121b's "if tribute wasn't paid" rider
//       explicitly does NOT trigger; the per_card handler must abstain.

func init() {
	registerTributeResolvedConsumersR60(Global())
	AddResetHook(registerTributeResolvedConsumersR60)
}

func registerTributeResolvedConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Pharagax Giant", "tribute_resolved", pharagaxGiantTribute)
	r.OnTrigger("Snake of the Golden Grove", "tribute_resolved", snakeGoldenGroveTribute)
	r.OnTrigger("Ornitharch", "tribute_resolved", ornitharchTribute)
	r.OnTrigger("Nessian Demolok", "tribute_resolved", nessianDemolokTribute)
	r.OnTrigger("Fanatic of Xenagos", "tribute_resolved", fanaticXenagosTribute)
	r.OnTrigger("Thunder Brute", "tribute_resolved", thunderBruteTribute)
}

// tributeGuard extracts the bearer perm + decision from ctx and
// verifies the dispatch matched the bearer. Returns (nil, false) when
// ctx is malformed, fireTrigger fired against an unrelated permanent
// (sibling-copy case), or the bearer is no longer on the battlefield.
//
// When accepted == true the bearer already got its N +1/+1 counters
// from the engine and the printed "if tribute wasn't paid" rider must
// abstain — the second return distinguishes "accepted, skip" from
// "refused, fire payoff." Callers branch on the second return; nil
// first return is the universal abstain signal.
func tributeGuard(perm *gameengine.Permanent, ctx map[string]interface{}) (*gameengine.Permanent, bool) {
	if perm == nil || perm.Card == nil || ctx == nil {
		return nil, false
	}
	src, _ := ctx["perm"].(*gameengine.Permanent)
	if src != perm {
		return nil, false
	}
	accepted, _ := ctx["accepted"].(bool)
	return src, accepted
}

// -----------------------------------------------------------------------------
// Pharagax Giant — {4}{R} Creature — Giant
//
//	Tribute 2
//	When this creature enters, if tribute wasn't paid, this creature
//	deals 5 damage to each opponent.
//
// Mirrors the Teapot Slinger / Y'shtola each-opponent damage shape.
// Skips Lost seats; CheckEnd after the fan-out so a lethal hit closes
// the game promptly.
// -----------------------------------------------------------------------------

func pharagaxGiantTribute(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "pharagax_giant_refused_burn"
	src, accepted := tributeGuard(perm, ctx)
	if src == nil || accepted {
		return
	}
	hits := 0
	for _, opp := range gs.Opponents(src.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.DealDamage(gs, opp, 5, src.Card.DisplayName())
		hits++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"opps_hit": hits,
		"damage":   5,
	})
	_ = gs.CheckEnd()
}

// -----------------------------------------------------------------------------
// Snake of the Golden Grove — {4}{G} Creature — Snake
//
//	Tribute 3
//	When this creature enters, if tribute wasn't paid, you gain 4 life.
// -----------------------------------------------------------------------------

func snakeGoldenGroveTribute(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "snake_golden_grove_refused_lifegain"
	src, accepted := tributeGuard(perm, ctx)
	if src == nil || accepted {
		return
	}
	gameengine.GainLife(gs, src.Controller, 4, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"life_gain": 4,
	})
}

// -----------------------------------------------------------------------------
// Ornitharch — {3}{W}{W} Creature — Archon
//
//	Flying
//	Tribute 2
//	When this creature enters, if tribute wasn't paid, create two 1/1
//	white Bird creature tokens with flying.
//
// Mirrors the radagast_wizard_of_wilds Bird token construction (kw:flying
// in Types so the keyword pipeline picks it up).
// -----------------------------------------------------------------------------

func ornitharchTribute(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ornitharch_refused_bird_tokens"
	src, accepted := tributeGuard(perm, ctx)
	if src == nil || accepted {
		return
	}
	created := 0
	for i := 0; i < 2; i++ {
		token := gameengine.CreateCreatureToken(gs, src.Controller, "Bird Token",
			[]string{"creature", "bird", "kw:flying"}, 1, 1)
		if token != nil {
			if token.Card != nil {
				token.Card.Colors = []string{"W"}
				token.Card.TypeLine = "Token Creature — Bird"
			}
			created++
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    src.Controller,
		"created": created,
	})
}

// -----------------------------------------------------------------------------
// Nessian Demolok — {3}{G}{G} Creature — Beast
//
//	Tribute 3
//	When this creature enters, if tribute wasn't paid, destroy target
//	noncreature permanent.
//
// Target picker: highest-CMC noncreature, non-land permanent across
// opponents. Lands are filtered (the printed text reads "noncreature
// permanent" which technically includes lands, but the corpus-wide
// per_card convention is to leave lands to dedicated LD effects; this
// keeps the AI from cracking opponents' mana base on a refused tribute
// where the value is the threat-removal pivot).
// -----------------------------------------------------------------------------

func nessianDemolokTribute(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "nessian_demolok_refused_destroy"
	src, accepted := tributeGuard(perm, ctx)
	if src == nil || accepted {
		return
	}
	target := pickHighestCMCOppNoncreatureNonland(gs, src.Controller)
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_eligible_target", map[string]interface{}{
			"seat": src.Controller,
		})
		return
	}
	if gameengine.DestroyPermanent(gs, target, src) {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":   src.Controller,
			"target": target.Card.DisplayName(),
		})
	}
}

// -----------------------------------------------------------------------------
// Fanatic of Xenagos — {1}{R}{G} Creature — Centaur Warrior
//
//	Trample
//	Tribute 1
//	When this creature enters, if tribute wasn't paid, it gets +1/+1 and
//	gains haste until end of turn.
//
// Combines the UEOT P/T mod with a haste keyword grant. Cleanup runs
// at the engine-managed UEOT sweep (Modifications) + a next_end_step
// delayed trigger for the keyword flag (mirrors ellie_vengeful_hunter).
// -----------------------------------------------------------------------------

func fanaticXenagosTribute(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fanatic_xenagos_refused_buff_haste"
	src, accepted := tributeGuard(perm, ctx)
	if src == nil || accepted {
		return
	}
	src.Modifications = append(src.Modifications, gameengine.Modification{
		Power:     1,
		Toughness: 1,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	grantHasteUEOT(gs, src)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// -----------------------------------------------------------------------------
// Thunder Brute — {4}{R}{R} Creature — Cyclops
//
//	Trample
//	Tribute 3
//	When this creature enters, if tribute wasn't paid, it gains haste
//	until end of turn.
// -----------------------------------------------------------------------------

func thunderBruteTribute(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "thunder_brute_refused_haste"
	src, accepted := tributeGuard(perm, ctx)
	if src == nil || accepted {
		return
	}
	grantHasteUEOT(gs, src)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// pickHighestCMCOppNoncreatureNonland returns the highest-CMC
// non-creature, non-land permanent any opponent controls. Returns nil
// if none exist. Skips Lost seats.
func pickHighestCMCOppNoncreatureNonland(gs *gameengine.GameState, mySeat int) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestCMC := -1
	for i, s := range gs.Seats {
		if i == mySeat || s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if cardHasType(p.Card, "creature") || cardHasType(p.Card, "land") {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc > bestCMC {
				best = p
				bestCMC = cmc
			}
		}
	}
	return best
}

// grantHasteUEOT stamps kw:haste on the perm and schedules a
// next_end_step cleanup that scrubs the flag. Captures the perm by
// closure with a nil-Flags guard so a destroy-then-reanimate window
// inside the same turn doesn't NPE on cleanup.
func grantHasteUEOT(gs *gameengine.GameState, src *gameengine.Permanent) {
	if src == nil {
		return
	}
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["kw:haste"] = 1
	captured := src
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			delete(captured.Flags, "kw:haste")
		},
	})
}
