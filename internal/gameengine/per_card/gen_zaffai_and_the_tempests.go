package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZaffaiAndTheTempests wires Zaffai and the Tempests.
//
// Oracle text (Scryfall, verified):
//
//	Once during each of your turns, you may cast an instant or sorcery
//	spell from your hand without paying its mana cost.
//
// Implementation (R42b stub port):
//   - ETB: stamp seat.Flags["zaffai_free_cast_available"] =
//     perm.Timestamp so any downstream alt-cost cast pipeline can
//     observe the grant. The flag is rewritten each turn at ETB time
//     and at the controller's upkeep refresh (below).
//   - "spell_cast" trigger gated on caster_seat == controller and an
//     I/S spell: if seat.Flags["zaffai_free_cast_used_t<turn>"] is
//     unset, we consume the per-turn ration by setting that flag to
//     gs.Turn+1 (avoids the zero-turn ambiguity). A future cast hook
//     that wants to apply the alt-cost reads
//     seat.Flags["zaffai_free_cast_available"] != 0 AND
//     seat.Flags["zaffai_free_cast_used_t<turn>"] == 0 to gate
//     payment. emitPartial documents the alt-cost wiring gap.
//   - "upkeep" trigger gated on active_seat == controller refreshes
//     the per-turn used flag (deletes the stale zaffai_free_cast_used
//     entry for the previous turn). The active-seat gate ensures the
//     refresh fires exactly once per Zaffai-turn.
func registerZaffaiAndTheTempests(r *Registry) {
	r.OnETB("Zaffai and the Tempests", zaffaiAndTheTempestsETB)
	r.OnTrigger("Zaffai and the Tempests", "upkeep", zaffaiUpkeepRefresh)
	r.OnTrigger("Zaffai and the Tempests", "spell_cast", zaffaiSpellCastConsume)
	// R51 batch I: LTB clears the alt-cost grant flag + any stale
	// per-turn used markers so a removed Zaffai doesn't leave the
	// once-per-turn free-cast contract active for downstream consumers.
	r.OnTrigger("Zaffai and the Tempests", "permanent_ltb", zaffaiLTBClearFlags)
}

func zaffaiLTBClearFlags(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	// R55: drop the Zaffai-owned ZoneCastPolicy. Concurrent Aluren /
	// Karn / Cecily policies are unaffected since they're keyed by a
	// different SourcePerm.
	gs.UnregisterZoneCastPoliciesForPermanent(perm)
	s := gs.Seats[perm.Controller]
	if s == nil || s.Flags == nil {
		return
	}
	delete(s.Flags, "zaffai_free_cast_available")
	for k := range s.Flags {
		if len(k) > len(zaffaiUsedPrefix) && k[:len(zaffaiUsedPrefix)] == zaffaiUsedPrefix {
			delete(s.Flags, k)
		}
	}
}

func zaffaiAndTheTempestsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zaffai_and_the_tempests_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["zaffai_free_cast_available"] = perm.Timestamp
	// R55: register a ZoneCastPolicy for the once-per-turn free
	// I/S cast. The standing-policy lifetime is while_source_on_bf
	// (Zaffai must be on the battlefield to grant the free cast).
	// The "once per turn" cap is enforced by the spell_cast trigger
	// below stamping the zaffai_free_cast_used_t<turn> key — the
	// cast pipeline should check that key alongside the policy
	// before applying ManaCost=0. The policy itself is duration-less
	// (it covers every cast attempt; the per-turn cap is upstream).
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:      perm,
		HandlerID:       "zaffai_free_is_cast_once_per_turn",
		Zone:            gameengine.ZoneHand,
		OwnerScope:      "self",
		CasterScope:     "controller",
		ControllerSeat:  perm.Controller,
		Predicate:       zaffaiIsInstantOrSorceryPredicate,
		ManaCost:        0,
		Duration:        "while_source_on_bf",
		SourceTimestamp: perm.Timestamp,
		GrantTurn:       gs.Turn,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"grant": "once_per_turn_free_is_cast",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"once_per_turn_cap_enforced_via_consume_trigger_cast_pipeline_must_check_zaffai_free_cast_used_t<turn>")
}

func zaffaiIsInstantOrSorceryPredicate(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return cardHasType(c, "instant") || cardHasType(c, "sorcery")
}

func zaffaiUpkeepRefresh(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || s.Flags == nil {
		return
	}
	// Clear any stale used-marker so the new turn's free cast is
	// available again. The marker key is computed from gs.Turn so
	// stale entries effectively become dead keys; deleting them keeps
	// the seat Flags map small.
	for k := range s.Flags {
		if len(k) > len(zaffaiUsedPrefix) && k[:len(zaffaiUsedPrefix)] == zaffaiUsedPrefix {
			delete(s.Flags, k)
		}
	}
}

func zaffaiSpellCastConsume(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zaffai_free_cast_consume"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	spellCard, _ := ctx["card"].(*gameengine.Card)
	if spellCard == nil {
		return
	}
	if !cardHasType(spellCard, "instant") && !cardHasType(spellCard, "sorcery") {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	key := zaffaiUsedKey(gs.Turn)
	if s.Flags[key] != 0 {
		// Already consumed this turn — observation only, no flag change.
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"consumed": true,
			"reason":   "already_used_this_turn",
		})
		return
	}
	s.Flags[key] = gs.Turn + 1 // mark used (turn+1 avoids zero ambiguity)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"consumed": true,
		"key":      key,
	})
}

const zaffaiUsedPrefix = "zaffai_free_cast_used_t"

func zaffaiUsedKey(turn int) string {
	return zaffaiUsedPrefix + itoa(turn)
}
