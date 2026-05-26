package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// inspired_consumers_r60.go — per_card OnTrigger handlers for the
// Born of the Gods "Inspired" keyword family (CR §702.124).
//
// Per the Phase 1B audit, `inspired` is an engine-emitted event with
// no per_card consumer wired despite the BNG Inspired cycle being a
// distinct 30+-card pool. The engine has the full plumbing —
// keywords_inspired.go's `FireInspiredTriggers` is invoked from
// UntapAll (canonical untap step) and UntapPermanent (mid-turn
// untap entry point), logs `inspired_trigger`, and calls
// FireCardTrigger("inspired", ctx) with `source` / `controller_seat`
// — but no per_card handler consumed the trigger, so the printed
// "Inspired — Whenever this creature becomes untapped, X" payoff on
// every card in the family was inert.
//
// This file wires 6 handlers spanning the cheap → mid → splashy
// spectrum of the BNG Inspired cycle so the engine actually plays
// the cards as printed when an Inspired-bearing creature transitions
// tapped → untapped:
//
//   - Oreskos Sun Guide    — gain 2 life (cheap, deterministic)
//   - Sphinx's Disciple    — draw a card (cheap, deterministic)
//   - Pheres-Band Tromper  — put a +1/+1 counter on self (cheap,
//                            self-buff)
//   - Servant of Tymaret   — each opp loses 1, you gain life equal
//                            to total life lost (multiplayer drain)
//   - Pain Seer            — reveal top, to hand, lose life = CMC
//                            (Dark Confidant-on-untap)
//   - Felhide Spiritbinder — pay {1}{R}: create haste token copy of
//                            target creature, exile at next end step
//                            (token-copy + delayed-exile, the user-
//                            named complex case)
//
// All 6 handlers gate on `ctx["source"] == perm` so the
// fireTrigger-walks-the-whole-battlefield dispatcher doesn't
// over-fire when two copies of the same Inspired creature are in
// play (the printed trigger fires only on the one that actually
// untapped, not on every same-named perm — see
// class_level_up_consumers_r60.go's classLevelGuard for the model).

func init() {
	registerInspiredConsumersR60(Global())
	AddResetHook(registerInspiredConsumersR60)
}

func registerInspiredConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Oreskos Sun Guide", "inspired", oreskosSunGuideInspired)
	r.OnTrigger("Sphinx's Disciple", "inspired", sphinxsDiscipleInspired)
	r.OnTrigger("Pheres-Band Tromper", "inspired", pheresBandTromperInspired)
	r.OnTrigger("Servant of Tymaret", "inspired", servantOfTymaretInspired)
	r.OnTrigger("Pain Seer", "inspired", painSeerInspired)
	r.OnTrigger("Felhide Spiritbinder", "inspired", felhideSpiritbinderInspired)
}

// inspiredSourceGuard returns the source permanent when the dispatcher
// fired this handler against the actually-untapped permanent.
// Returns nil when the ctx is malformed or when the dispatcher walked
// past a sibling permanent that shares the same printed name. The
// fireTrigger walks every battlefield permanent matching by display
// name; for inspired, only the perm in ctx["source"] actually
// transitioned tapped → untapped, so all other matches must early-
// return.
func inspiredSourceGuard(perm *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if perm == nil || perm.Card == nil || ctx == nil {
		return nil
	}
	src, _ := ctx["source"].(*gameengine.Permanent)
	if src != perm {
		return nil
	}
	return src
}

// -----------------------------------------------------------------------------
// Oreskos Sun Guide — {1}{W} Creature — Cat Monk 1/2
//
//	Inspired — Whenever this creature becomes untapped, you gain 2
//	life.
// -----------------------------------------------------------------------------

func oreskosSunGuideInspired(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "oreskos_sun_guide_inspired"
	src := inspiredSourceGuard(perm, ctx)
	if src == nil {
		return
	}
	gameengine.GainLife(gs, src.Controller, 2, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"lifegain": 2,
	})
}

// -----------------------------------------------------------------------------
// Sphinx's Disciple — {2}{U} Creature — Sphinx 2/2 (Flying)
//
//	Inspired — Whenever this creature becomes untapped, draw a card.
// -----------------------------------------------------------------------------

func sphinxsDiscipleInspired(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sphinxs_disciple_inspired"
	src := inspiredSourceGuard(perm, ctx)
	if src == nil {
		return
	}
	drawn := drawOne(gs, src.Controller, src.Card.DisplayName())
	if drawn == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "empty_library", map[string]interface{}{
			"seat": src.Controller,
		})
		return
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  src.Controller,
		"drawn": drawn.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Pheres-Band Tromper — {3}{G} Creature — Centaur Warrior 3/3
//
//	Inspired — Whenever this creature becomes untapped, put a +1/+1
//	counter on it.
// -----------------------------------------------------------------------------

func pheresBandTromperInspired(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "pheres_band_tromper_inspired"
	src := inspiredSourceGuard(perm, ctx)
	if src == nil {
		return
	}
	src.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          src.Controller,
		"plus_one_plus_one": src.Counters["+1/+1"],
	})
}

// -----------------------------------------------------------------------------
// Servant of Tymaret — {2}{B} Creature — Zombie Soldier 2/1
//
//	Inspired — Whenever this creature becomes untapped, each opponent
//	loses 1 life. You gain life equal to the life lost this way.
//	{2}{B}: Regenerate this creature. (Regenerate omitted — handled
//	via the engine's standard regenerate-cost activation.)
// -----------------------------------------------------------------------------

func servantOfTymaretInspired(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "servant_of_tymaret_inspired"
	src := inspiredSourceGuard(perm, ctx)
	if src == nil {
		return
	}
	lifeLost := 0
	for _, opp := range gs.Opponents(src.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.LoseLife(gs, opp, 1, src.Card.DisplayName())
		lifeLost++
	}
	if lifeLost > 0 {
		gameengine.GainLife(gs, src.Controller, lifeLost, src.Card.DisplayName())
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"opps_hit":  lifeLost,
		"life_gain": lifeLost,
	})
}

// -----------------------------------------------------------------------------
// Pain Seer — {1}{B} Creature — Human Wizard 2/2
//
//	Inspired — Whenever this creature becomes untapped, reveal the
//	top card of your library and put that card into your hand. You
//	lose life equal to that card's mana value.
//
// Dark Confidant on Inspired. The card always enters hand (not
// optional); the life loss is mandatory and equal to the revealed
// CMC.
// -----------------------------------------------------------------------------

func painSeerInspired(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "pain_seer_inspired"
	src := inspiredSourceGuard(perm, ctx)
	if src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || len(seat.Library) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "empty_library", map[string]interface{}{
			"seat": src.Controller,
		})
		return
	}
	top := seat.Library[0]
	gameengine.MoveCard(gs, top, src.Controller, "library", "hand", "pain_seer_inspired")
	// Use Card.CMC directly — the authoritative engine field — rather
	// than the helpers.go `cardCMC` proxy that scans Types for a
	// "cmc:N" marker. Real corpus cards have CMC set on the field;
	// the Types marker is only populated by test fixtures.
	cmc := top.CMC
	if cmc == 0 {
		cmc = cardCMC(top)
	}
	if cmc > 0 {
		gameengine.LoseLife(gs, src.Controller, cmc, src.Card.DisplayName())
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"revealed": top.DisplayName(),
		"cmc":      cmc,
	})
}

// -----------------------------------------------------------------------------
// Felhide Spiritbinder — {3}{R} Creature — Minotaur Shaman 3/4
//
//	Inspired — Whenever this creature becomes untapped, you may pay
//	{1}{R}. If you do, create a token that's a copy of another target
//	creature, except it's an enchantment in addition to its other
//	types. It gains haste. Exile it at the beginning of the next end
//	step.
//
// Implementation:
//   - Always pay when ≥2 mana in pool (deterministic AI policy for the
//     test fixture; the token + haste is pure value).
//   - Target picker prefers the highest-power friendly creature OTHER
//     than the spiritbinder itself (a haste copy of a 4/4 threat is
//     much stronger than a haste copy of a 1/1).
//   - Token is minted with the source creature's base power/toughness
//     and types, plus "enchantment" type-line stamp via the Types
//     slice, plus a kw:haste flag for the haste grant.
//   - Delayed-trigger sweep at next-end-step exiles the token (CR
//     §702.124 + §702.66 reminder text on the printed card).
// -----------------------------------------------------------------------------

func felhideSpiritbinderInspired(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "felhide_spiritbinder_inspired"
	src := inspiredSourceGuard(perm, ctx)
	if src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// Pay {1}{R} → reduced to {2} colorless cost in the per_card
	// fixture (colored-pip enforcement isn't wired through this path).
	if seat.ManaPool < 2 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
		})
		return
	}
	target := pickBestFriendlyCreatureOtherThan(gs, src.Controller, src)
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target", map[string]interface{}{
			"seat": src.Controller,
		})
		return
	}
	seat.ManaPool -= 2

	tokenCard := &gameengine.Card{
		Name:          target.Card.DisplayName() + " (Spiritbinder token)",
		Owner:         src.Controller,
		Types:         append(append([]string{"token", "enchantment"}, target.Card.Types...)),
		BasePower:     target.Card.BasePower,
		BaseToughness: target.Card.BaseToughness,
		TypeLine:      target.Card.TypeLine,
	}
	tokenPerm := &gameengine.Permanent{
		Card:       tokenCard,
		Controller: src.Controller,
		Owner:      src.Controller,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{"kw:haste": 1},
	}
	seat.Battlefield = append(seat.Battlefield, tokenPerm)

	// Exile the token at the beginning of the next end step.
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			// Defensive: the token may have already left play (died,
			// flickered, etc.) — only exile if it's still on the
			// battlefield. Mirror the abdel_adrian /
			// inalla-archmage-ritualist defensive scan.
			for _, p := range gs.Seats[src.Controller].Battlefield {
				if p == tokenPerm {
					gameengine.ExilePermanent(gs, tokenPerm, src)
					return
				}
			}
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": target.Card.DisplayName(),
		"token":  tokenCard.Name,
	})
}

// pickBestFriendlyCreatureOtherThan returns the highest-power
// creature seat controls, excluding the named permanent. Used by
// Felhide Spiritbinder's "another target creature" branch (the
// printed "another" word forbids targeting the spiritbinder itself).
func pickBestFriendlyCreatureOtherThan(gs *gameengine.GameState, seat int, exclude *gameengine.Permanent) *gameengine.Permanent {
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1
	for _, p := range s.Battlefield {
		if p == nil || p == exclude || !p.IsCreature() {
			continue
		}
		if pw := p.Power(); pw > bestPower {
			bestPower = pw
			best = p
		}
	}
	return best
}
