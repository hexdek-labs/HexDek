package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLaraCroftTombRaider wires Lara Croft, Tomb Raider.
//
// Oracle text:
//
//   First strike, reach
//   Whenever Lara Croft attacks, exile up to one target legendary
//   artifact card or legendary land card from a graveyard and put a
//   discovery counter on it. You may play a card from exile with a
//   discovery counter on it this turn.
//   Raid — At end of combat on your turn, if you attacked this turn,
//          create a Treasure token.
//
// R37 port; R58 zone-cast-policy port:
//
//   - First strike + reach: AST keyword pipeline.
//   - Attack trigger (exile + discovery counter + play-from-exile
//     permission): R58 — the exile + discovery-counter stamp ports
//     directly. The play-from-exile permission is now a per-attack
//     ZoneCastPolicy(Zone=exile, predicate=has discovery_counter,
//     Duration=until_end_of_turn).
//   - Raid: PORTED. end_of_combat_controller trigger fires
//     CreateTreasureToken when the controller attacked this turn.
//     Reads Seat.Turn.Attacked (set by combat.go's DeclareAttackers).
//     The "on your turn" half is enforced by registering against
//     end_of_combat_controller (event_aliases routes the controller
//     variant when gs.Active == perm.Controller).
func registerLaraCroftTombRaider(r *Registry) {
	r.OnETB("Lara Croft, Tomb Raider", laraCroftStaticETB)
	r.OnTrigger("Lara Croft, Tomb Raider", "creature_attacks", laraCroftAttackTrigger)
	r.OnTrigger("Lara Croft, Tomb Raider", "end_of_combat", laraCroftRaidTreasure)
	r.OnTrigger("Lara Croft, Tomb Raider", "permanent_ltb", laraCroftLTB)
}

func laraCroftLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterZoneCastPoliciesForPermanent(perm)
}

// laraCroftDiscoveryPredicate matches any card carrying the
// "discovery_counter" type tag laraCroftAttackTrigger stamps. Defined at
// file scope so the closure registered by every Lara Croft attack
// shares the same predicate pointer.
func laraCroftDiscoveryPredicate(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "discovery_counter" {
			return true
		}
	}
	return false
}

func laraCroftStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lara_croft_static"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

// laraCroftAttackTrigger fires when Lara Croft attacks (CR §702.0). The
// engine fires "creature_attacks" with ctx["attacker_perm"]; we filter
// to Lara herself. Scans every graveyard for a legendary artifact or
// legendary land card, picks the highest-CMC, exiles it, stamps a
// "discovery_counter" flag on the card so future cast-from-exile
// pipeline work can honor the "you may play it this turn" permission.
//
// emitPartial flags the play-from-exile gap (engine doesn't yet support
// generalized "play from exile if it has a discovery counter").
func laraCroftAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lara_croft_attack_exile_discovery"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk != perm {
		return
	}
	var bestCard *gameengine.Card
	var bestSeat int
	bestCMC := -1
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasType(c, "legendary") {
				continue
			}
			if !cardHasType(c, "artifact") && !cardHasType(c, "land") {
				continue
			}
			if cmc := cardCMC(c); cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
				bestSeat = i
			}
		}
	}
	if bestCard == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"exiled":  false,
			"reason":  "no_legendary_artifact_or_land_in_any_graveyard",
		})
		return
	}
	gameengine.MoveCard(gs, bestCard, bestSeat, "graveyard", "exile", slug)
	// Tag the card with a discovery_counter marker. The card's Types
	// slice is the per_card runtime carrier for ad-hoc flags (mirror
	// the "discovery_counter" tag the play-from-exile scanner would
	// read once wired).
	tagged := false
	for _, t := range bestCard.Types {
		if t == "discovery_counter" {
			tagged = true
			break
		}
	}
	if !tagged {
		bestCard.Types = append(bestCard.Types, "discovery_counter")
	}
	// R58: register a ZoneCastPolicy granting Lara's controller
	// permission to cast any discovery-counter-tagged card from exile
	// until end of turn. ManaCost=-1 honors the card's printed cost; the
	// "you may play it" wording covers lands and spells alike — the
	// engine's CastFromZone path handles the land-vs-spell branch.
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:      perm,
		HandlerID:       "lara_croft_discovery_play_from_exile",
		Zone:            gameengine.ZoneExile,
		OwnerScope:      "any",
		CasterScope:     "controller",
		ControllerSeat:  perm.Controller,
		Predicate:       laraCroftDiscoveryPredicate,
		ManaCost:        -1,
		Duration:        "until_end_of_turn",
		SourceTimestamp: perm.Timestamp,
		GrantTurn:       gs.Turn,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"exiled":        bestCard.DisplayName(),
		"from_seat":     bestSeat,
		"cmc":           bestCMC,
		"discovery_set": true,
		"policy":        "lara_croft_discovery_play_from_exile",
	})
}

// laraCroftRaidTreasure fires at end of combat. Per CR §702.128a
// (Raid), "attacked this turn" is read from Seat.Turn.Attacked. Gated
// on (a) it being our controller's turn — checked via gs.Active —
// and (b) the Seat.Turn.Attacked flag.
func laraCroftRaidTreasure(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lara_croft_raid_treasure"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	// "On your turn" — Raid only fires on the controller's own turn.
	if gs.Active != seat {
		return
	}
	if !gs.Seats[seat].Turn.Attacked {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"reason": "raid_not_satisfied",
		})
		return
	}
	gameengine.CreateTreasureToken(gs, seat)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"treasure": true,
	})
}
