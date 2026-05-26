package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// expend_consumers_r60.go — per_card OnTrigger handlers for the
// Bloomburrow "Expend N" mechanic (CR §702.190, BLB 2024).
//
// Per the Versailles Phase 1B audit, `expend` is an engine-emitted
// event (keywords_expend.go::FireExpendTriggers fires the threshold-
// crossing fan-out the moment a seat's per-turn mana-spent counter
// crosses N). The engine has the full plumbing — HasExpendTrigger /
// ExpendThresholds / TrackManaSpentThisTurn / FireExpendTriggers — but
// no per_card handler consumed the trigger, so every printed "Expend N
// — ..." payoff in the BLB corpus was inert.
//
// This file wires 6 handlers covering the canonical payoff shapes:
//
//   - Bark-Knuckle Boxer    — Expend 4 → self gains indestructible UEOT
//   - Junkblade Bruiser     — Expend 4 → self +2/+1 UEOT
//   - Bakersbane Duo        — Expend 4 → self +1/+1 UEOT
//   - Teapot Slinger        — Expend 4 → 2 damage to each opponent
//   - Wandertale Mentor     — Expend 4 → +1/+1 counter on self
//   - Trailtracker Scout    — Expend 8 → return permanent card from
//                             your graveyard to your hand
//
// All handlers gate on (a) `source == perm` (FireExpendTriggers walks
// the battlefield emitting one fan-out per bearer, but the downstream
// fireTrigger dispatcher walks the battlefield AGAIN by name — so two
// copies of Wandertale Mentor would cause each name-match to fire on
// BOTH copies. The src-eq guard ensures each bearer fires its own
// payoff exactly once per threshold crossing) and (b) the expected
// threshold (Trailtracker is the only Expend 8 in this batch; the
// others all read 4).

func init() {
	registerExpendConsumersR60(Global())
	AddResetHook(registerExpendConsumersR60)
}

func registerExpendConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Bark-Knuckle Boxer", "expend", barkKnuckleBoxerExpend)
	r.OnTrigger("Junkblade Bruiser", "expend", junkbladeBruiserExpend)
	r.OnTrigger("Bakersbane Duo", "expend", bakersbaneDuoExpend)
	r.OnTrigger("Teapot Slinger", "expend", teapotSlingerExpend)
	r.OnTrigger("Wandertale Mentor", "expend", wandertaleMentorExpend)
	r.OnTrigger("Trailtracker Scout", "expend", trailtrackerScoutExpend)
}

// expendGuard extracts (src, threshold) from ctx and verifies the
// dispatch matched the bearer. Returns (nil, -1) when ctx is
// malformed or fireTrigger fired the handler against an unrelated
// permanent (sibling-copy case).
func expendGuard(perm *gameengine.Permanent, ctx map[string]interface{}) (*gameengine.Permanent, int) {
	if perm == nil || perm.Card == nil || ctx == nil {
		return nil, -1
	}
	src, _ := ctx["source"].(*gameengine.Permanent)
	if src != perm {
		return nil, -1
	}
	threshold, _ := ctx["threshold"].(int)
	return src, threshold
}

// -----------------------------------------------------------------------------
// Bark-Knuckle Boxer — {1}{G} Creature — Raccoon Berserker
//
//	Whenever you expend 4, this creature gains indestructible until end
//	of turn.
//
// Standard UEOT-keyword pattern: stamp kw:indestructible + register a
// next_end_step delayed cleanup that scrubs the flag. The captured perm
// is rechecked for nil-Flags inside the closure so a destroy-then-
// reanimate window doesn't NPE.
// -----------------------------------------------------------------------------

func barkKnuckleBoxerExpend(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bark_knuckle_boxer_expend4_indestructible"
	src, threshold := expendGuard(perm, ctx)
	if src == nil || threshold != 4 {
		return
	}
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["kw:indestructible"] = 1
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
			delete(captured.Flags, "kw:indestructible")
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"threshold": threshold,
	})
}

// -----------------------------------------------------------------------------
// Junkblade Bruiser — {3}{R/G}{R/G} Creature — Raccoon Berserker
//
//	Trample
//	Whenever you expend 4, this creature gets +2/+1 until end of turn.
//
// Modification with Duration=until_end_of_turn — the engine sweeps
// these at cleanup; no per_card scheduling needed.
// -----------------------------------------------------------------------------

func junkbladeBruiserExpend(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "junkblade_bruiser_expend4_buff"
	src, threshold := expendGuard(perm, ctx)
	if src == nil || threshold != 4 {
		return
	}
	src.Modifications = append(src.Modifications, gameengine.Modification{
		Power:     2,
		Toughness: 1,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"threshold": threshold,
	})
}

// -----------------------------------------------------------------------------
// Bakersbane Duo — {1}{G} Creature — Squirrel Raccoon
//
//	When this creature enters, create a Food token.
//	Whenever you expend 4, this creature gets +1/+1 until end of turn.
//
// Food-token ETB is a separate concern (BLB food token plumbing); this
// handler covers only the expend payoff per the Phase 1B scope.
// -----------------------------------------------------------------------------

func bakersbaneDuoExpend(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bakersbane_duo_expend4_buff"
	src, threshold := expendGuard(perm, ctx)
	if src == nil || threshold != 4 {
		return
	}
	src.Modifications = append(src.Modifications, gameengine.Modification{
		Power:     1,
		Toughness: 1,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"threshold": threshold,
	})
}

// -----------------------------------------------------------------------------
// Teapot Slinger — {3}{R} Creature — Raccoon Warrior
//
//	Menace
//	Whenever you expend 4, this creature deals 2 damage to each opponent.
//
// Mirrors the Y'shtola pattern (yshtola_nights_blessed.go:132-141) —
// loop over Opponents(), skip Lost seats, DealDamage(2). CheckEnd
// after the fan-out so a lethal hit closes the game promptly rather
// than waiting for the next SBA pass.
// -----------------------------------------------------------------------------

func teapotSlingerExpend(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "teapot_slinger_expend4_pinger"
	src, threshold := expendGuard(perm, ctx)
	if src == nil || threshold != 4 {
		return
	}
	hits := 0
	for _, opp := range gs.Opponents(src.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.DealDamage(gs, opp, 2, src.Card.DisplayName())
		hits++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"threshold": threshold,
		"opps_hit":  hits,
		"damage":    2,
	})
	_ = gs.CheckEnd()
}

// -----------------------------------------------------------------------------
// Wandertale Mentor — {R}{G} Creature — Raccoon Bard
//
//	Whenever you expend 4, put a +1/+1 counter on this creature.
//	{T}: Add {R} or {G}.
//
// Permanent counter — accumulates across triggers (a deck that crosses
// 4 mana on turn 3 and 4 should end turn 4 with 2 +1/+1 counters on
// Mentor). The +1/+1 cap of CR §704.5q is left to the SBA layer; this
// handler just stamps the counter.
// -----------------------------------------------------------------------------

func wandertaleMentorExpend(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wandertale_mentor_expend4_counter"
	src, threshold := expendGuard(perm, ctx)
	if src == nil || threshold != 4 {
		return
	}
	src.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"threshold": threshold,
		"counters":  src.Counters["+1/+1"],
	})
}

// -----------------------------------------------------------------------------
// Trailtracker Scout — {1}{G} Creature — Raccoon Scout
//
//	{T}: Add one mana of any color.
//	Whenever you expend 8, return up to one target permanent card from
//	your graveyard to your hand.
//
// "Permanent card" = artifact / creature / enchantment / planeswalker /
// battle / land. Target picker: highest-CMC eligible card in the
// controller's graveyard. "Up to one" — no eligible target is a clean
// no-op (not a failure), mirroring monk_class's bounce-or-skip shape.
// -----------------------------------------------------------------------------

func trailtrackerScoutExpend(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "trailtracker_scout_expend8_return"
	src, threshold := expendGuard(perm, ctx)
	if src == nil || threshold != 8 {
		return
	}
	s := gs.Seats[src.Controller]
	if s == nil {
		return
	}
	var target *gameengine.Card
	bestCMC := -1
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "artifact") && !cardHasType(c, "creature") &&
			!cardHasType(c, "enchantment") && !cardHasType(c, "planeswalker") &&
			!cardHasType(c, "battle") && !cardHasType(c, "land") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			target = c
		}
	}
	if target == nil {
		// "Up to one" — clean no-op, not a failure.
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      src.Controller,
			"threshold": threshold,
			"returned":  false,
		})
		return
	}
	gameengine.MoveCard(gs, target, src.Controller, "graveyard", "hand", "trailtracker_scout_return")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"threshold": threshold,
		"returned":  true,
		"target":    target.DisplayName(),
	})
}
