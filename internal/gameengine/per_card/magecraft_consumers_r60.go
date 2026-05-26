package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// magecraft_consumers_r60.go — per_card OnTrigger handlers for magecraft
// payoffs (Strixhaven, CR §702.137a). The engine fires
// FireCardTrigger("magecraft", ctx) from keywords_magecraft.go whenever
// the controller of a magecraft permanent casts or copies an instant or
// sorcery spell.
//
// ctx keys forwarded by FireMagecraftTriggers:
//
//	"source":     *Permanent  — the magecraft-bearing permanent
//	"controller": int          — caster seat (== source.Controller)
//	"spell":      *Card        — the cast/copied spell
//	"spell_name": string       — display name for logging
//	"is_copy":    bool         — true if from a copy
//
// Per the Versailles Phase 1B audit (PR #477 §3) magecraft is one of 29
// engine-emitted events with no per_card consumer wired despite ~30+
// magecraft-bearing cards in the corpus. This file wires 8 of the most
// iconic / mechanically simple bearers.

func init() {
	registerMagecraftConsumersR60(Global())
	AddResetHook(registerMagecraftConsumersR60)
}

func registerMagecraftConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Archmage Emeritus", "magecraft", archmageEmeritusMagecraft)
	r.OnTrigger("Witherbloom Apprentice", "magecraft", witherbloomApprenticeMagecraft)
	r.OnTrigger("Witherbloom Pledgemage", "magecraft", witherbloomPledgemageMagecraft)
	r.OnTrigger("Leonin Lightscribe", "magecraft", leoninLightscribeMagecraft)
	r.OnTrigger("Storm-Kiln Artist", "magecraft", stormKilnArtistMagecraft)
	r.OnTrigger("Dragonsguard Elite", "magecraft", dragonsguardEliteMagecraft)
	r.OnTrigger("Quandrix Pledgemage", "magecraft", quandrixPledgemageMagecraft)
	r.OnTrigger("Karok Wrangler", "magecraft", karokWranglerMagecraft)
}

// magecraftCommonGuard returns the magecraft-bearing permanent and the
// caster seat from ctx, or (nil, -1) when the ctx is malformed or the
// dispatch ran on a permanent that doesn't belong to the caster. Every
// handler in this file shares the same shape, so consolidating the
// extraction keeps the per-handler code focused on the payoff.
func magecraftCommonGuard(perm *gameengine.Permanent, ctx map[string]interface{}) (*gameengine.Permanent, int) {
	if perm == nil || perm.Card == nil || ctx == nil {
		return nil, -1
	}
	src, _ := ctx["source"].(*gameengine.Permanent)
	if src == nil {
		// FireMagecraftTriggers should always supply "source", but if a
		// future caller forgets, fall back to the dispatched perm.
		src = perm
	}
	if src != perm {
		// The dispatcher walks every battlefield permanent and matches by
		// card name, so this can fire for a Permanent that isn't the one
		// FireMagecraftTriggers picked (e.g. two Archmage Emeritus copies
		// each receive the trigger as the dispatcher iterates). Use the
		// dispatched perm — the engine fires once per magecraft permanent
		// already (keywords_magecraft.go:138), so this branch is the
		// non-pathological one.
		src = perm
	}
	seat := src.Controller
	if seat < 0 || seat >= 16 {
		return nil, -1
	}
	return src, seat
}

// Archmage Emeritus — {2}{U}{U} Creature
// Magecraft — Whenever you cast or copy an instant or sorcery spell, draw a card.
func archmageEmeritusMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "archmage_emeritus_draw"
	src, seat := magecraftCommonGuard(perm, ctx)
	if src == nil {
		return
	}
	drawOne(gs, seat, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

// Witherbloom Apprentice — {B}{G} Creature
// Magecraft — Whenever you cast or copy an instant or sorcery spell,
// each opponent loses 1 life and you gain 1 life.
func witherbloomApprenticeMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "witherbloom_apprentice_drain"
	src, seat := magecraftCommonGuard(perm, ctx)
	if src == nil {
		return
	}
	drained := 0
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.LoseLife(gs, opp, 1, src.Card.DisplayName())
		drained++
	}
	gameengine.GainLife(gs, seat, 1, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    seat,
		"drained": drained,
	})
}

// Witherbloom Pledgemage — {3}{B/G}{B/G} Creature
// Magecraft — Whenever you cast or copy an instant or sorcery spell, you gain 1 life.
func witherbloomPledgemageMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "witherbloom_pledgemage_lifegain"
	src, seat := magecraftCommonGuard(perm, ctx)
	if src == nil {
		return
	}
	gameengine.GainLife(gs, seat, 1, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

// Leonin Lightscribe — {1}{W} Creature
// Magecraft — Whenever you cast or copy an instant or sorcery spell,
// creatures you control get +1/+1 until end of turn.
func leoninLightscribeMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "leonin_lightscribe_anthem"
	src, seat := magecraftCommonGuard(perm, ctx)
	if src == nil {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	ts := gs.NextTimestamp()
	pumped := 0
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Toughness: 1,
			Duration:  "until_end_of_turn",
			Timestamp: ts,
		})
		pumped++
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"pumped": pumped,
	})
}

// Storm-Kiln Artist — {3}{R} Creature
// Magecraft — Whenever you cast or copy an instant or sorcery spell, create a Treasure token.
func stormKilnArtistMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "storm_kiln_artist_treasure"
	src, seat := magecraftCommonGuard(perm, ctx)
	if src == nil {
		return
	}
	gameengine.CreateTreasureToken(gs, seat)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

// Dragonsguard Elite — {1}{G} Creature
// Magecraft — Whenever you cast or copy an instant or sorcery spell,
// put a +1/+1 counter on this creature.
func dragonsguardEliteMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dragonsguard_elite_self_counter"
	src, seat := magecraftCommonGuard(perm, ctx)
	if src == nil {
		return
	}
	src.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"counters": src.Counters["+1/+1"],
	})
}

// Quandrix Pledgemage — {1}{G/U}{G/U} Creature
// Magecraft — Whenever you cast or copy an instant or sorcery spell,
// put a +1/+1 counter on this creature.
func quandrixPledgemageMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "quandrix_pledgemage_self_counter"
	src, seat := magecraftCommonGuard(perm, ctx)
	if src == nil {
		return
	}
	src.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"counters": src.Counters["+1/+1"],
	})
}

// Karok Wrangler — {4}{G} Creature
// Magecraft — Whenever you cast or copy an instant or sorcery spell,
// put a +1/+1 counter on target creature you control.
//
// Target choice: highest-power friendly creature (favors snowballing
// the biggest threat). If there are no friendly creatures, emitFail.
func karokWranglerMagecraft(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "karok_wrangler_target_counter"
	src, seat := magecraftCommonGuard(perm, ctx)
	if src == nil {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
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
	if best == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_friendly_creature", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	best.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"target": best.Card.DisplayName(),
	})
}
