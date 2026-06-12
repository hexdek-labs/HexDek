package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// card_cycled_consumers_r60.go — per_card OnTrigger handlers for the
// cycling-payoff family (CR §702.29, cycling action). Picks up the
// next Phase 1B audit gap after PR #524's false-positive sweep.
//
// Engine fire site: `keywords_p0.go:230` fires
// FireCardTrigger("card_cycled", ctx) once per cycle action. ctx:
//   "cycled_card":      *Card  — the card that was cycled (in graveyard
//                                or under the player's hand-discard pile,
//                                depending on the cycle path)
//   "cycled_card_name": string
//   "controller_seat":  int    — the player who cycled
//
// Two flavors of observer trigger in the corpus:
//   (A) "Whenever a PLAYER cycles a card, ..."  — any cycler triggers
//   (B) "Whenever YOU cycle a card, ..."        — gate on
//                                                 controller_seat ==
//                                                 perm.Controller
// And one common "another card" variant:
//   (C) "Whenever you cycle or discard ANOTHER card, ..." — gate
//       additionally on cycled_card_name != perm.Card.DisplayName()
//
// All 6 wired handlers below fall into one of these three shapes.

func init() {
	registerCardCycledConsumersR60(Global())
	AddResetHook(registerCardCycledConsumersR60)
}

func registerCardCycledConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Astral Slide", "card_cycled", astralSlideCycled)
	r.OnTrigger("Lightning Rift", "card_cycled", lightningRiftCycled)
	r.OnTrigger("Faith of the Devoted", "card_cycled", faithOfTheDevotedCycled)
	r.OnTrigger("Drake Haven", "card_cycled", drakeHavenCycled)
	r.OnTrigger("Curator of Mysteries", "card_cycled", curatorOfMysteriesCycled)
	r.OnTrigger("Archfiend of Ifnir", "card_cycled", archfiendOfIfnirCycled)
}

// cycledGuard extracts the cycler seat from ctx and applies an
// optional "your-only" gate. Returns (-1, "") on malformed ctx or
// when the gate rejects.
func cycledGuard(perm *gameengine.Permanent, ctx map[string]interface{}, yourOnly bool) (int, string) {
	if perm == nil || perm.Card == nil || ctx == nil {
		return -1, ""
	}
	cyclerSeat, ok := ctx["controller_seat"].(int)
	if !ok {
		return -1, ""
	}
	if yourOnly && cyclerSeat != perm.Controller {
		return -1, ""
	}
	cycledName, _ := ctx["cycled_card_name"].(string)
	return cyclerSeat, cycledName
}

// payOneMana decrements seat.ManaPool by 1 and returns true. Returns
// false (no-op) when the seat doesn't have a mana point spare. Used
// by the optional "may pay {1}" cycling triggers (Lightning Rift,
// Faith of the Devoted, Drake Haven).
func payOneMana(gs *gameengine.GameState, seat int) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
	if s == nil || s.ManaPool < 1 {
		return false
	}
	s.ManaPool--
	gameengine.SyncManaAfterSpend(s)
	// Aux-credit: this optional unless-pay is invisible to the cost
	// check's announced-cost reconstruction (a trigger payment, never a
	// dispatcher-charged cost — no masking risk).
	gs.Legality.NoteManaSpend(seat, 1)
	return true
}

// -----------------------------------------------------------------------------
// Astral Slide — {2}{W} Enchantment
//
// Whenever a player cycles a card, you may exile target creature. If
// you do, return that card to the battlefield under its owner's
// control at the beginning of the next end step.
//
// Target picker: highest-power opp creature (a "blink the biggest
// threat for a turn" defensive play). Schedule the return via a
// next_end_step delayed trigger that captures the exiled card.
// -----------------------------------------------------------------------------

func astralSlideCycled(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "astral_slide_exile_blink"
	cyclerSeat, _ := cycledGuard(perm, ctx, false /* any cycler */)
	if cyclerSeat < 0 {
		return
	}
	target := pickBestOppCreature(gs, perm.Controller)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	exiledCard := target.Card
	exiledOwner := target.Card.Owner
	gameengine.ExilePermanent(gs, target, perm)

	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName() + " (Astral Slide return)",
		CreatedTurn:    gs.Turn,
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if exiledCard == nil || exiledOwner < 0 || exiledOwner >= len(gs.Seats) {
				return
			}
			s := gs.Seats[exiledOwner]
			if s == nil {
				return
			}
			// Remove from exile if still there (could have been
			// moved by another effect — guard).
			removed := false
			for i, c := range s.Exile {
				if c == exiledCard {
					s.Exile = append(s.Exile[:i], s.Exile[i+1:]...)
					removed = true
					break
				}
			}
			if !removed {
				return
			}
			enterBattlefieldWithETB(gs, exiledOwner, exiledCard, false)
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": exiledCard.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Lightning Rift — {1}{R} Enchantment
//
// Whenever a player cycles a card, you may pay {1}. If you do,
// Lightning Rift deals 2 damage to any target.
//
// Target picker: highest-life living opponent (pings the runway).
// -----------------------------------------------------------------------------

func lightningRiftCycled(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lightning_rift_ping"
	cyclerSeat, _ := cycledGuard(perm, ctx, false /* any cycler */)
	if cyclerSeat < 0 {
		return
	}
	if !payOneMana(gs, perm.Controller) {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_mana_to_pay", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	bestLife := -1
	pick := -1
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			pick = opp
		}
	}
	if pick < 0 {
		return
	}
	gameengine.DealDamage(gs, pick, 2, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": pick,
	})
}

// -----------------------------------------------------------------------------
// Faith of the Devoted — {2}{B} Enchantment
//
// Whenever you cycle or discard a card, you may pay {1}. If you do,
// each opponent loses 2 life and you gain 2 life.
//
// "Whenever YOU cycle" — gate on controller match. The discard half
// is wired separately under `card_discarded` in a follow-up if needed.
// -----------------------------------------------------------------------------

func faithOfTheDevotedCycled(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "faith_of_the_devoted_drain"
	cyclerSeat, _ := cycledGuard(perm, ctx, true /* your only */)
	if cyclerSeat < 0 {
		return
	}
	if !payOneMana(gs, perm.Controller) {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_mana_to_pay", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	hit := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.LoseLife(gs, opp, 2, perm.Card.DisplayName())
		hit++
	}
	gameengine.GainLife(gs, perm.Controller, 2, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"opps_hit": hit,
	})
}

// -----------------------------------------------------------------------------
// Drake Haven — {2}{U} Enchantment
//
// Whenever you cycle or discard a card, you may pay {1}. If you do,
// create a 2/2 blue Drake creature token with flying.
// -----------------------------------------------------------------------------

func drakeHavenCycled(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "drake_haven_drake_token"
	cyclerSeat, _ := cycledGuard(perm, ctx, true /* your only */)
	if cyclerSeat < 0 {
		return
	}
	if !payOneMana(gs, perm.Controller) {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_mana_to_pay", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	gameengine.CreateCreatureToken(gs, perm.Controller, "Drake",
		[]string{"creature", "drake", "pip:U", "kw:flying"}, 2, 2)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

// -----------------------------------------------------------------------------
// Curator of Mysteries — {2}{U}{U} Creature — Sphinx
//
// Flying. Whenever you cycle or discard ANOTHER card, scry 1.
//
// "Another card" — when Curator is itself cycled (a hand-discard
// path), the trigger shouldn't fire on its own cycle. Filter on
// cycled_card_name != perm.Card.DisplayName().
// -----------------------------------------------------------------------------

func curatorOfMysteriesCycled(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "curator_of_mysteries_scry"
	cyclerSeat, cycledName := cycledGuard(perm, ctx, true /* your only */)
	if cyclerSeat < 0 {
		return
	}
	if cycledName == perm.Card.DisplayName() {
		return
	}
	gameengine.Scry(gs, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"cycled": cycledName,
	})
}

// -----------------------------------------------------------------------------
// Archfiend of Ifnir — {3}{B}{B} Creature — Demon
//
// Flying. Whenever you cycle or discard ANOTHER card, put a -1/-1
// counter on each creature your opponents control.
// -----------------------------------------------------------------------------

func archfiendOfIfnirCycled(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "archfiend_of_ifnir_minus_counters"
	cyclerSeat, cycledName := cycledGuard(perm, ctx, true /* your only */)
	if cyclerSeat < 0 {
		return
	}
	if cycledName == perm.Card.DisplayName() {
		return
	}
	hit := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.AddCounter("-1/-1", 1)
			hit++
		}
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"creatures_marked":  hit,
	})
}
