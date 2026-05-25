package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKessDissidentMageCustom replaces the auto-generated stub with
// a real implementation of Kess's once-per-turn cast-from-graveyard
// privilege, built on the engine's once_per_turn_cast_from_graveyard
// primitive (see gameengine/zone_cast.go).
//
// Oracle text:
//
//	Flying
//	Once during each of your turns, you may cast an instant or
//	sorcery spell from your graveyard. If a spell cast this way
//	would be put into your graveyard, exile it instead.
//
// Implementation:
//   - OnETB: scan controller's graveyard, register a
//     NewOncePerTurnGraveyardCastPermission on each instant or sorcery.
//     The permission's SourceTimestamp pins the grant to this Kess so
//     ExpireSourceGrants can revoke them on LTB, and the engine's
//     once-per-turn gate (zone_cast.go) refuses a second cast after the
//     first one resolves this turn.
//   - OnTrigger "zone_change" + "creature_dies": refresh grants as new
//     instants/sorceries enter the graveyard mid-turn (mill, discard,
//     bounce-to-grave, etc.).
//   - When Kess leaves the battlefield, the engine's EOT cleanup
//     (ExpireZoneCastGrants in phases.go) prunes the grants via the
//     "while_source_on_bf" duration. Until that runs, the
//     oncePerTurnConsumed guard in CastFromZone rejects any cast
//     through an orphaned grant because the source permanent no longer
//     exists, so stale grants are inert. Eager LTB cleanup would
//     require a permanent_ltb observer pathway that this branch
//     doesn't add.
//
// The engine handles the actual cast through CastFromZone — the Hat
// picks a spell from AvailableZoneCastGrants and the engine pays its
// cost, fires cast triggers, and routes the resolved spell to exile
// via the ExileOnResolve flag.
func registerKessDissidentMageCustom(r *Registry) {
	r.OnETB("Kess, Dissident Mage", kessETB)
	r.OnTrigger("Kess, Dissident Mage", "zone_change", kessRefreshGrants)
	r.OnTrigger("Kess, Dissident Mage", "creature_dies", kessRefreshGrants)
}

func kessETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kess_dissident_mage"
	if gs == nil || perm == nil {
		return
	}
	granted := grantKessOncePerTurn(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"grants_added": granted,
		"keyword":      "once_per_turn_cast_from_graveyard",
		"filter":       "instant_or_sorcery",
		"exile_on_resolve": true,
	})
}

func kessRefreshGrants(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	grantKessOncePerTurn(gs, perm)
}

// grantKessOncePerTurn scans the controller's graveyard and attaches a
// once-per-turn cast permission to every instant or sorcery that does
// not already have a more specific grant (e.g. flashback from oracle
// text, Underworld Breach escape). Returns the number of new grants.
func grantKessOncePerTurn(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	granted := 0
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			continue
		}
		if gameengine.GetZoneCastGrant(gs, c) != nil {
			continue
		}
		p := gameengine.NewOncePerTurnGraveyardCastPermission(
			seatIdx,
			perm.Card.DisplayName(),
			perm.Timestamp,
			-1,   // use card's own mana cost
			true, // exile on resolve
			nil,  // no additional costs
		)
		gameengine.RegisterZoneCastGrant(gs, c, p)
		granted++
	}
	return granted
}
