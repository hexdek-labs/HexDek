package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChitinousCrawler wires up Chitinous Crawler.
//
// Oracle text:
//
//	At the beginning of combat on your turn, choose target creature card in
//	your graveyard. Conjure a duplicate of it into your graveyard.
//
//	Descend 8 — Exile a permanent card from your graveyard: You may play
//	it. Activate only as a sorcery and only if there are eight or more
//	permanent cards in your graveyard.
//
// R60: the AST encodes the Descend 8 activated ability as
// `ModKind="impulse_play"` with args[0]=`"you may play it"` and the cost
// embedded as a free-text Cost.extra entry. Before r60 the engine's
// `case "impulse_play":` arm interpreted ALL impulse_play modkinds as
// "exile the top of your library, you may play that exiled card" — which
// (a) exiled an unrelated library card (Swamp / LibCard 0-0 in fuzz
// reports), (b) registered a ZoneCastPermission against it with
// SourceTimestamp=0, and (c) left the grant to leak. Confirmed by loki
// r60 round 2 — Chitinous Crawler was 2 of the 4 remaining
// ZoneCastGrantExpiry hits, cross-validated with the same shape in
// goldilocks-r60.
//
// The resolve_helpers.go fix gates the impulse_play arm on args[0] —
// it skips the library-top exile for "face down" / "from your hand" /
// "from your graveyard" hints. Chitinous Crawler's args[0] is the bare
// "you may play it" (the zone hint lives in Cost.extra), so the
// impulse_play arm doesn't see a graveyard hint and would still default
// to library-top. This per_card handler:
//
//   - Selects a permanent card from the controller's graveyard.
//   - Moves it to exile (the "cost" half of the activation).
//   - Registers a properly-stamped ZoneCastPermission against THAT card
//     (zone=exile, duration=until_end_of_turn, GrantTurn=gs.Turn,
//     SourceTimestamp=src.Timestamp).
//   - Sets the chitinous_crawler_handled flag on the source so any
//     downstream resolver knows the cost-and-grant cycle was handled.
//
// In a real game the cost would already have been paid before the
// activated ability resolved; this handler models the cost-and-effect as
// one atomic resolve step because the goldilocks / loki harnesses
// invoke OnActivated without going through the priority/cost-pay
// pipeline. Tests pin both shapes (cost paid externally + handler
// invoked, and handler-driven cost-and-grant in one call).
func registerChitinousCrawler(r *Registry) {
	r.OnActivated("Chitinous Crawler", chitinousCrawlerActivated)
}

func chitinousCrawlerActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "chitinous_crawler_descend_8_exile_play"
	if gs == nil || src == nil || src.Controller < 0 || src.Controller >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}

	// Find a permanent card in the controller's graveyard. Per oracle
	// "permanent card" = any card whose printed type line includes a
	// permanent type (creature / artifact / enchantment / land /
	// planeswalker / battle). Prefer creature cards when available
	// (highest-impact reanimate target); fall back to the first
	// permanent we find.
	var picked *gameengine.Card
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !isPermanentCard(c) {
			continue
		}
		if picked == nil || (cardHasType(c, "creature") && !cardHasType(picked, "creature")) {
			picked = c
			if cardHasType(c, "creature") {
				break
			}
		}
	}
	if picked == nil {
		emitFail(gs, slug, "Chitinous Crawler", "no_permanent_card_in_graveyard", nil)
		return
	}

	// Pay the cost: move the chosen card from graveyard to exile.
	gameengine.MoveCard(gs, picked, src.Controller, "graveyard", "exile", "chitinous_crawler_cost")

	// Register a properly-stamped exile-cast grant for THIS card.
	perm := &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneExile,
		Keyword:           "chitinous_crawler_play",
		ManaCost:          -1, // pay the card's own mana cost
		ExileOnResolve:    false,
		RequireController: src.Controller,
		SourceName:        "Chitinous Crawler",
		Duration:          "until_end_of_turn",
		GrantTurn:         gs.Turn,
		SourceTimestamp:   src.Timestamp,
	}
	gameengine.RegisterZoneCastGrant(gs, picked, perm)

	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["chitinous_crawler_handled"] = 1

	emit(gs, slug, "Chitinous Crawler", map[string]interface{}{
		"seat":              src.Controller,
		"exiled_card":       picked.DisplayName(),
		"source_timestamp":  src.Timestamp,
		"grant_turn":        gs.Turn,
		"duration":          "until_end_of_turn",
		"source_timestamp_str": strconv.Itoa(src.Timestamp),
	})
}

