package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// eerie_consumers_r60.go — per_card OnTrigger handlers for the
// Duskmourn (DSK) Eerie keyword family (CR §702.182, ability word).
//
// Per the Versailles Phase 1B audit (PR #477 §3) `eerie` is one of 29
// engine-emitted events with no per_card consumer wired. The engine
// has the full plumbing — `OnEnchantmentETB` and `OnRoomFullyUnlocked`
// in keywords_eerie.go fan out an "eerie" trigger per eerie-carrying
// permanent — but no per_card handler subscribed to the payoff.
//
// ctx keys forwarded by both fan-out paths:
//
//	"trigger_perm":    *Permanent — the enchantment that ETB'd or the
//	                                Room that just fully unlocked.
//	"eerie_source":    *Permanent — the eerie-bearing permanent (the
//	                                bearer that should run the payoff).
//	"controller_seat": int         — eerie source's controller.
//	"cause":           string      — "enchantment_etb" or "room_unlocked"
//
// The dispatcher walks the battlefield and matches by card name, so
// `perm` (the handler's first arg) is the eerie-bearing permanent the
// dispatcher picked. `eerie_source == perm` for cleanly dispatched
// events — we still defend the assumption in eerieGuard below.

func init() {
	registerEerieConsumersR60(Global())
	AddResetHook(registerEerieConsumersR60)
}

func registerEerieConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Entity Tracker", "eerie", entityTrackerEerie)
	r.OnTrigger("Balemurk Leech", "eerie", balemurkLeechEerie)
	r.OnTrigger("Skullsnap Nuisance", "eerie", skullsnapNuisanceEerie)
	r.OnTrigger("Erratic Apparition", "eerie", erraticApparitionEerie)
	r.OnTrigger("Gremlin Tamer", "eerie", gremlinTamerEerie)
	r.OnTrigger("Optimistic Scavenger", "eerie", optimisticScavengerEerie)
}

// eerieGuard returns the eerie-bearing perm + controller seat when the
// ctx is well-formed and dispatched to the right perm; nil + -1 when
// the dispatcher accidentally fired this handler against a stale or
// mismatched ctx.
func eerieGuard(perm *gameengine.Permanent, ctx map[string]interface{}) (*gameengine.Permanent, int) {
	if perm == nil || perm.Card == nil || ctx == nil {
		return nil, -1
	}
	src, _ := ctx["eerie_source"].(*gameengine.Permanent)
	if src != nil && src != perm {
		// The fan-out fires one trigger per eerie carrier; the dispatcher
		// can re-walk every permanent with the matching name. Only run
		// when the dispatched perm IS the carrier the fan-out picked.
		return nil, -1
	}
	seat, ok := ctx["controller_seat"].(int)
	if !ok {
		seat = perm.Controller
	}
	// Defensive bound on the seat index. Commander pods top out at 6;
	// 16 is more than enough headroom to catch any malformed ctx.
	if seat < 0 || seat >= 16 {
		return nil, -1
	}
	return perm, seat
}

// -----------------------------------------------------------------------------
// Entity Tracker — {2}{U} Creature — Human Scout (Flash)
//
// Eerie — Whenever an enchantment you control enters and whenever you
// fully unlock a Room, draw a card.
// -----------------------------------------------------------------------------

func entityTrackerEerie(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "entity_tracker_draw"
	src, seat := eerieGuard(perm, ctx)
	if src == nil {
		return
	}
	drawOne(gs, seat, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

// -----------------------------------------------------------------------------
// Balemurk Leech — {1}{B} Creature — Leech
//
// Eerie — Whenever an enchantment you control enters and whenever you
// fully unlock a Room, each opponent loses 1 life.
// -----------------------------------------------------------------------------

func balemurkLeechEerie(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "balemurk_leech_drain"
	src, seat := eerieGuard(perm, ctx)
	if src == nil {
		return
	}
	hit := 0
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.LoseLife(gs, opp, 1, src.Card.DisplayName())
		hit++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"opps_hit": hit,
	})
}

// -----------------------------------------------------------------------------
// Skullsnap Nuisance — {U}{B} Creature — Insect Skeleton (Flying)
//
// Eerie — Whenever an enchantment you control enters and whenever you
// fully unlock a Room, surveil 1.
// -----------------------------------------------------------------------------

func skullsnapNuisanceEerie(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "skullsnap_nuisance_surveil"
	src, seat := eerieGuard(perm, ctx)
	if src == nil {
		return
	}
	gameengine.Surveil(gs, seat, 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

// -----------------------------------------------------------------------------
// Erratic Apparition — {2}{U} Creature — Spirit (Flying, Vigilance)
//
// Eerie — Whenever an enchantment you control enters and whenever you
// fully unlock a Room, this creature gets +1/+1 until end of turn.
// -----------------------------------------------------------------------------

func erraticApparitionEerie(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "erratic_apparition_pump"
	src, _ := eerieGuard(perm, ctx)
	if src == nil {
		return
	}
	ts := gs.NextTimestamp()
	src.Modifications = append(src.Modifications, gameengine.Modification{
		Power:     1,
		Toughness: 1,
		Duration:  "until_end_of_turn",
		Timestamp: ts,
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// -----------------------------------------------------------------------------
// Gremlin Tamer — {W}{U} Creature — Human Scout
//
// Eerie — Whenever an enchantment you control enters and whenever you
// fully unlock a Room, create a 1/1 red Gremlin creature token.
// -----------------------------------------------------------------------------

func gremlinTamerEerie(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gremlin_tamer_token"
	src, seat := eerieGuard(perm, ctx)
	if src == nil {
		return
	}
	gameengine.CreateCreatureToken(gs, seat, "Gremlin",
		[]string{"creature", "gremlin", "pip:R"}, 1, 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

// -----------------------------------------------------------------------------
// Optimistic Scavenger — {W} Creature — Human Scout
//
// Eerie — Whenever an enchantment you control enters and whenever you
// fully unlock a Room, put a +1/+1 counter on target creature.
//
// Target choice: highest-power friendly creature (snowballs your board).
// If no friendly creature exists, emit a fail breadcrumb.
// -----------------------------------------------------------------------------

func optimisticScavengerEerie(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "optimistic_scavenger_counter"
	src, seat := eerieGuard(perm, ctx)
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
