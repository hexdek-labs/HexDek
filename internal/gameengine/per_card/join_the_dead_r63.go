package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// join_the_dead_r63.go — per_card handler for Join the Dead.
//
// Oracle text (Instant, {1}{B}{B}):
//
//	Target creature gets -5/-5 until end of turn.
//	Descend 4 — That creature gets -10/-10 until end of turn instead if
//	there are four or more permanent cards in your graveyard.
//
// Replacement-orphan tail (r63 census): the base "-5/-5" parses to a
// structured Buff node but the conditional Descend-4 upgrade dumped to an
// inert parsed_tail, so the spell only ever applied -5/-5 — the -10/-10
// line was dead. A bespoke OnResolve handler evaluates Descend 4 (count of
// permanent cards in the caster's graveyard) and applies the correct
// modifier. No damage pipeline is involved (pure §613 P/T modification),
// so reimplementing the base alongside the conditional is unambiguous.
//
// AI targeting: removal — pick the opponent creature the debuff best
// answers (highest effective toughness, preferring one the modifier kills).
func init() {
	registerJoinTheDeadR63(Global())
	AddResetHook(registerJoinTheDeadR63)
}

func registerJoinTheDeadR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Join the Dead", joinTheDeadResolve)
}

// permanentCardsInGraveyard counts permanent-type cards (creature,
// artifact, enchantment, land, planeswalker, battle) in a seat's
// graveyard — the Descend metric (CR §702.166, "permanent card").
func permanentCardsInGraveyard(gs *gameengine.GameState, seat int) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	n := 0
	for _, c := range gs.Seats[seat].Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") || cardHasType(c, "artifact") ||
			cardHasType(c, "enchantment") || cardHasType(c, "land") ||
			cardHasType(c, "planeswalker") || cardHasType(c, "battle") {
			n++
		}
	}
	return n
}

func joinTheDeadResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "join_the_dead"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller

	// Descend 4 — the conditional that was inert pre-r63.
	delta := -5
	descend := permanentCardsInGraveyard(gs, seat) >= 4
	if descend {
		delta = -10
	}

	target := bestRemovalTarget(gs, seat, -delta)
	if target == nil {
		emitFail(gs, slug, "Join the Dead", "no_legal_target", nil)
		return
	}
	target.Modifications = append(target.Modifications, gameengine.Modification{
		Power:     delta,
		Toughness: delta,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
	gs.LogEvent(gameengine.Event{
		Kind: "pt_modification", Seat: seat, Source: "Join the Dead",
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"power":  delta, "toughness": delta, "descend4": descend,
			"rule": "613.4",
		},
	})
	emit(gs, slug, "Join the Dead", map[string]interface{}{
		"seat": seat, "target": target.Card.DisplayName(),
		"delta": delta, "descend4": descend,
	})
	_ = gs.CheckEnd()
}

// bestRemovalTarget picks the opponent creature a -k/-k (k>0) removal
// spell best answers: prefer one it kills (effective toughness ≤ k),
// highest power among those; else the highest-power opponent creature.
func bestRemovalTarget(gs *gameengine.GameState, controller, k int) *gameengine.Permanent {
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
			if gs.ToughnessOf(p) <= k {
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
