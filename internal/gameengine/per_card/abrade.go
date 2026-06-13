package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// abrade.go — per_card handler for Abrade.
//
// Oracle text (Instant, {1}{R}):
//
//	Choose one —
//	• Abrade deals 3 damage to target creature.
//	• Destroy target artifact.
//
// Why a per_card handler: the modal body parsed to an inert `parsed_tail`
// scaffold, so this premier flexible removal spell (72 decks in the local
// pool) did NOTHING. Verified inert: no registered handler.
//
// Mode policy (AI): destroying an artifact is usually the higher-leverage
// line (mana rocks, equipment, combo pieces), so prefer destroying the
// best opponent artifact; otherwise fall back to 3 damage to the best
// killable opponent creature (toughness ≤ 3, highest power), else the
// highest-power opponent creature. Fizzles only when no legal target exists.
func init() {
	registerAbrade(Global())
	AddResetHook(registerAbrade)
}

func registerAbrade(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Abrade", abradeResolve)
}

func abradeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "abrade"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller

	if art := bestOpponentArtifact(gs, seat); art != nil {
		name := art.Card.DisplayName()
		gameengine.DestroyPermanent(gs, art, nil)
		emit(gs, slug, "Abrade", map[string]interface{}{
			"seat": seat, "mode": "destroy_artifact", "target": name,
		})
		_ = gs.CheckEnd()
		return
	}

	if c := bestThreeDamageCreatureTarget(gs, seat); c != nil {
		c.MarkedDamage += 3
		gs.InvalidateCharacteristicsCache()
		gs.LogEvent(gameengine.Event{
			Kind: "damage", Seat: c.Controller, Source: "Abrade", Amount: 3,
			Details: map[string]interface{}{"target": c.Card.DisplayName()},
		})
		emit(gs, slug, "Abrade", map[string]interface{}{
			"seat": seat, "mode": "damage_creature", "target": c.Card.DisplayName(), "damage": 3,
		})
		_ = gs.CheckEnd()
		return
	}

	emitFail(gs, slug, "Abrade", "no_legal_target", nil)
}

// bestOpponentArtifact returns the highest-mana-value nonland artifact an
// opponent of `controller` controls, or nil.
func bestOpponentArtifact(gs *gameengine.GameState, controller int) *gameengine.Permanent {
	var best *gameengine.Permanent
	for _, opp := range gs.Opponents(controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !cardHasType(p.Card, "artifact") {
				continue
			}
			if p.IsLand() {
				continue
			}
			if best == nil || p.Card.CMC > best.Card.CMC {
				best = p
			}
		}
	}
	return best
}

// bestThreeDamageCreatureTarget prefers a creature 3 damage would kill
// (toughness ≤ 3), highest power among those; else the highest-power
// opponent creature.
func bestThreeDamageCreatureTarget(gs *gameengine.GameState, controller int) *gameengine.Permanent {
	var killable, anyc *gameengine.Permanent
	for _, opp := range gs.Opponents(controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if anyc == nil || p.Power() > anyc.Power() {
				anyc = p
			}
			if gs.ToughnessOf(p)-p.MarkedDamage <= 3 {
				if killable == nil || p.Power() > killable.Power() {
					killable = p
				}
			}
		}
	}
	if killable != nil {
		return killable
	}
	return anyc
}
