package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLurrus wires Lurrus of the Dream-Den.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Lurrus%20of%20the%20Dream-Den):
//
//	{W}{B}{B}
//	Legendary Creature — Cat Nightmare
//	3/2
//	Companion — Each permanent card in your starting deck has mana
//	value 2 or less.
//	Lifelink
//	During each of your turns, you may cast one permanent spell with
//	mana value 2 or less from your graveyard.
//
// Implementation:
//   - OnETB: scan controller's graveyard, register a
//     NewOncePerTurnGraveyardCastPermission on each permanent card with
//     CMC ≤ 2. Mirrors the Kess Dissident Mage pattern (custom_kess_dissident_mage.go)
//     with two differences: (1) filter is permanent-card + CMC ≤ 2
//     instead of instant/sorcery, and (2) ExileOnResolve is false —
//     Lurrus's grant doesn't exile the spell on resolution (the
//     permanent enters the battlefield normally).
//   - OnTrigger("zone_change") + "creature_dies": refresh grants as new
//     ≤2-CMC permanents enter the graveyard mid-turn.
//   - Companion eligibility is checked at game start via the
//     companion.go pipeline; this handler doesn't need to enforce it.
//   - Lifelink is engine-side (AST keyword pipeline).
//   - The once-per-turn budget is enforced by the engine's
//     oncePerTurnConsumed gate on SourceTimestamp.
func registerLurrus(r *Registry) {
	r.OnETB("Lurrus of the Dream-Den", lurrusETB)
	r.OnTrigger("Lurrus of the Dream-Den", "zone_change", lurrusRefreshGrants)
	r.OnTrigger("Lurrus of the Dream-Den", "creature_dies", lurrusRefreshGrants)
}

func lurrusETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lurrus_of_the_dream_den"
	if gs == nil || perm == nil {
		return
	}
	granted := grantLurrusOncePerTurn(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"grants_added":     granted,
		"keyword":          "once_per_turn_cast_from_graveyard",
		"filter":           "permanent_cmc_2_or_less",
		"exile_on_resolve": false,
	})
}

func lurrusRefreshGrants(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	grantLurrusOncePerTurn(gs, perm)
}

// grantLurrusOncePerTurn scans the controller's graveyard and attaches
// a once-per-turn cast permission to every permanent card with mana
// value ≤ 2 that doesn't already have a more specific grant. Returns
// the count of new grants.
func grantLurrusOncePerTurn(gs *gameengine.GameState, perm *gameengine.Permanent) int {
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
		// Permanent cards only (artifact, creature, enchantment,
		// planeswalker, land, battle). Lands are technically eligible
		// under Lurrus's text but "cast" never applies to a land — the
		// CastFromZone path will reject non-castable cards.
		isPermanent := cardHasType(c, "creature") || cardHasType(c, "artifact") ||
			cardHasType(c, "enchantment") || cardHasType(c, "planeswalker") ||
			cardHasType(c, "battle")
		if !isPermanent {
			continue
		}
		if cardCMC(c) > 2 {
			continue
		}
		if gameengine.GetZoneCastGrant(gs, c) != nil {
			continue
		}
		p := gameengine.NewOncePerTurnGraveyardCastPermission(
			seatIdx,
			perm.Card.DisplayName(),
			perm.Timestamp,
			-1,    // use card's own mana cost
			false, // don't exile on resolve — permanent enters normally
			nil,   // no additional costs
		)
		gameengine.RegisterZoneCastGrant(gs, c, p)
		granted++
	}
	return granted
}
