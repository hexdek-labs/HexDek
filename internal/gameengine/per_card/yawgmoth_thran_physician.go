package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYawgmothThranPhysician wires Yawgmoth, Thran Physician.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Protection from Humans
//	Pay 1 life, Sacrifice another creature: Put a -1/-1 counter on up to
//	  one target creature and draw a card.
//	{B}{B}, Discard a card: Proliferate.
//
// Implementation:
//   - Protection from Humans — handled by the AST keyword pipeline; no
//     per-card hook needed.
//   - abilityIdx 0: Pay 1 life + sacrifice another creature (best
//     expendable: smallest non-commander creature, tiebreak newest by
//     Timestamp). Put a -1/-1 counter on opponent's best creature by
//     P+T (skip if no opponent creatures). Draw a card via MoveCard.
//     Gate: life > 1 (don't self-kill) + another creature on battlefield.
//   - abilityIdx 1: {B}{B} + discard a card: Proliferate. Discard the
//     worst card in hand (highest-CMC non-land, then lowest-CMC land).
//     Full CR §701.27 proliferate: all own permanents' counters, own
//     beneficial player counters, opponent poison/rad counters.
//     Gate: ManaPool >= 2 + at least 1 card in hand.
func registerYawgmothThranPhysician(r *Registry) {
	r.OnActivated("Yawgmoth, Thran Physician", yawgmothActivate)
}

func yawgmothActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		yawgmothSacDraw(gs, src, ctx)
	case 1:
		yawgmothProliferate(gs, src, ctx)
	}
}

// yawgmothSacDraw implements: Pay 1 life, Sacrifice another creature:
// Put a -1/-1 counter on up to one target creature and draw a card.
func yawgmothSacDraw(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yawgmoth_sac_draw"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Gate: must have life > 1 (don't sac ourselves to death).
	if s.Life <= 1 {
		emitFail(gs, slug, "Yawgmoth, Thran Physician", "life_too_low", map[string]interface{}{
			"seat": seat,
			"life": s.Life,
		})
		return
	}

	// Gate: must have another creature on the battlefield to sacrifice.
	victim := yawgmothPickSacVictim(gs, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Yawgmoth, Thran Physician", "no_creature_to_sacrifice", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	// --- Pay costs ---

	// Pay 1 life.
	s.Life -= 1
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: "Yawgmoth, Thran Physician",
		Amount: 1,
		Details: map[string]interface{}{
			"reason": "yawgmoth_ability_cost",
		},
	})

	// Sacrifice the creature.
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "yawgmoth_sac_cost")

	// --- Effects ---

	// Put a -1/-1 counter on up to one target creature (opponent's best
	// creature by P+T; skip counter placement if no valid target).
	counterTarget := yawgmothPickCounterTarget(gs, seat)
	counterTargetName := ""
	if counterTarget != nil {
		counterTarget.AddCounter("-1/-1", 1)
		counterTargetName = counterTarget.Card.DisplayName()
		gs.InvalidateCharacteristicsCache()
	}

	// Draw a card.
	if len(s.Library) > 0 {
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
	}

	emit(gs, slug, "Yawgmoth, Thran Physician", map[string]interface{}{
		"seat":           seat,
		"sacrificed":     victimName,
		"drew_card":      true,
		"counter_placed": counterTarget != nil,
		"counter_target": counterTargetName,
	})
	_ = gs.CheckEnd()
}

// yawgmothProliferate implements: {B}{B}, Discard a card: Proliferate.
func yawgmothProliferate(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yawgmoth_proliferate"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Gate: at least 1 card in hand.
	if len(s.Hand) == 0 {
		emitFail(gs, slug, "Yawgmoth, Thran Physician", "no_card_to_discard", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	// Gate: at least 2 black mana.
	if s.ManaPool < 2 {
		emitFail(gs, slug, "Yawgmoth, Thran Physician", "insufficient_mana", map[string]interface{}{
			"seat":      seat,
			"required":  2,
			"available": s.ManaPool,
		})
		return
	}

	// --- Pay costs ---

	// Pay {B}{B}.
	s.ManaPool -= 2
	gameengine.SyncManaAfterSpend(s)

	// Discard a card (pick the worst card in hand).
	discardTarget := yawgmothPickDiscardTarget(s)
	if discardTarget == nil {
		// Shouldn't happen given the hand-size gate, but be safe.
		discardTarget = s.Hand[0]
	}
	discardedName := discardTarget.DisplayName()
	gameengine.DiscardCard(gs, discardTarget, seat)

	// --- Effect: Proliferate (CR §701.27) ---
	// "Choose any number of permanents and/or players, then give each
	// another counter of each kind already there."
	//
	// GreedyHat policy: proliferate everything we control, own beneficial
	// player counters, and opponents' harmful counters. Skip opponent
	// +1/+1 counters.
	proliferatedCount := 0

	// 1. Walk all permanents on the battlefield.
	for _, ps := range gs.Seats {
		if ps == nil {
			continue
		}
		for _, p := range ps.Battlefield {
			if p == nil || len(p.Counters) == 0 {
				continue
			}
			isOurs := p.Controller == seat
			for kind, count := range p.Counters {
				if count <= 0 {
					continue
				}
				if !isOurs && kind == "+1/+1" {
					continue // don't help opponents
				}
				p.AddCounter(kind, 1)
				proliferatedCount++
			}
		}
	}

	// 2. Walk all players — proliferate player-level counters.
	// Beneficial on self (energy, experience); harmful on opponents
	// (poison, rad).
	for i, ps := range gs.Seats {
		if ps == nil {
			continue
		}
		isUs := i == seat
		if isUs {
			if ps.Flags != nil {
				if ps.Flags["energy_counters"] > 0 {
					ps.Flags["energy_counters"]++
					proliferatedCount++
				}
				if ps.Flags["experience_counters"] > 0 {
					ps.Flags["experience_counters"]++
					proliferatedCount++
				}
			}
		} else {
			if ps.PoisonCounters > 0 {
				ps.PoisonCounters++
				proliferatedCount++
			}
			if ps.Flags != nil && ps.Flags["rad_counters"] > 0 {
				ps.Flags["rad_counters"]++
				proliferatedCount++
			}
		}
	}

	if proliferatedCount > 0 {
		gs.InvalidateCharacteristicsCache()
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "proliferate",
		Seat:   seat,
		Source: "Yawgmoth, Thran Physician",
		Amount: proliferatedCount,
		Details: map[string]interface{}{
			"rule": "701.27",
		},
	})

	emit(gs, slug, "Yawgmoth, Thran Physician", map[string]interface{}{
		"seat":              seat,
		"discarded":         discardedName,
		"proliferated_count": proliferatedCount,
	})
}

// ---------------------------------------------------------------------------
// Target selection helpers
// ---------------------------------------------------------------------------

// yawgmothPickSacVictim selects the best expendable creature to sacrifice.
// Priority:
//  1. ctx-supplied "creature_perm" (hat override).
//  2. Smallest non-commander creature by Power+Toughness; tiebreak by
//     highest Timestamp (newest permanent = least established).
//
// Commanders are strongly avoided — sacrificing your commander sends it
// to the command zone and increases tax, which is rarely worth it.
func yawgmothPickSacVictim(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	// Hat override: if the hat picked a specific creature, use it.
	if ctx != nil {
		if p, ok := ctx["creature_perm"].(*gameengine.Permanent); ok && p != nil && p != src && p.IsCreature() {
			return p
		}
	}

	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}

	var best *gameengine.Permanent
	bestScore := 1<<31 - 1 // lower is better (smaller creature = more expendable)
	bestTS := -1            // higher is better (newest = most expendable), tiebreak

	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || p == src || !p.IsCreature() {
			continue
		}

		// Strongly avoid commanders.
		if yawgmothIsCommander(gs, p) {
			continue
		}

		score := p.Power() + p.Toughness()
		if best == nil || score < bestScore || (score == bestScore && p.Timestamp > bestTS) {
			bestScore = score
			bestTS = p.Timestamp
			best = p
		}
	}

	// If no non-commander creature found, allow commanders as a last
	// resort (edge case: only creature is a partner commander).
	if best == nil {
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || p == src || !p.IsCreature() {
				continue
			}
			score := p.Power() + p.Toughness()
			if best == nil || score < bestScore || (score == bestScore && p.Timestamp > bestTS) {
				bestScore = score
				bestTS = p.Timestamp
				best = p
			}
		}
	}

	return best
}

// yawgmothPickCounterTarget selects the best opponent creature to place a
// -1/-1 counter on. Highest Power+Toughness wins (weaken their biggest
// threat); tiebreak by lowest Timestamp (most-established permanent).
// Returns nil if no opponent has creatures.
func yawgmothPickCounterTarget(gs *gameengine.GameState, controllerSeat int) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestScore := -1 << 30
	bestTS := 1<<62 - 1

	for _, opp := range gs.Opponents(controllerSeat) {
		os := gs.Seats[opp]
		if os == nil {
			continue
		}
		for _, p := range os.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			score := p.Power() + p.Toughness()
			if score > bestScore || (score == bestScore && p.Timestamp < bestTS) {
				bestScore = score
				bestTS = p.Timestamp
				best = p
			}
		}
	}
	return best
}

// yawgmothPickDiscardTarget selects the worst card in hand to discard.
// Priority: highest-CMC non-land card (least likely to be castable soon),
// then lowest-CMC land (least value). Falls back to the first card.
func yawgmothPickDiscardTarget(s *gameengine.Seat) *gameengine.Card {
	if s == nil || len(s.Hand) == 0 {
		return nil
	}

	var worstNonLand *gameengine.Card
	worstNonLandCMC := -1

	var worstLand *gameengine.Card
	worstLandCMC := 1<<31 - 1

	for _, c := range s.Hand {
		if c == nil {
			continue
		}
		isLand := false
		for _, t := range c.Types {
			if strings.EqualFold(t, "land") {
				isLand = true
				break
			}
		}
		cmc := cardCMC(c)
		if isLand {
			if cmc <= worstLandCMC {
				worstLandCMC = cmc
				worstLand = c
			}
		} else {
			if cmc > worstNonLandCMC {
				worstNonLandCMC = cmc
				worstNonLand = c
			}
		}
	}

	// Prefer discarding a non-land (they cost mana to cast and a
	// high-CMC card stuck in hand is the least useful). Fall back to a
	// land, then the first card.
	if worstNonLand != nil {
		return worstNonLand
	}
	if worstLand != nil {
		return worstLand
	}
	return s.Hand[0]
}

// yawgmothIsCommander returns true if the permanent is one of its owner's
// commanders.
func yawgmothIsCommander(gs *gameengine.GameState, p *gameengine.Permanent) bool {
	if gs == nil || p == nil || p.Card == nil {
		return false
	}
	owner := p.Owner
	if owner < 0 || owner >= len(gs.Seats) || gs.Seats[owner] == nil {
		return false
	}
	want := strings.ToLower(p.Card.DisplayName())
	for _, name := range gs.Seats[owner].CommanderNames {
		if strings.ToLower(name) == want {
			return true
		}
	}
	return false
}
