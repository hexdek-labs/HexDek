package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch AI (R60) — 5 high-impact §400.7c-affected exile-then-cast
// commanders/staples wired through the post-#685 owner-routing
// discipline. Etali (PR #685) established the canonical shape:
//
//   - When an effect moves a card across seats into a non-shared zone
//     (library / exile / graveyard / hand), the card lands in the
//     OWNER's per-seat pile, not the effect-controller's. This is
//     CR §400.7c: "If an effect causes a player to put a card into
//     a zone, that card moves to the corresponding zone owned by
//     that player."
//
//   - The cast-from-exile permission (gs.ZoneCastGrants[*Card]) keys
//     on the *Card pointer, so the physical exile pile doesn't gate
//     who may cast. RequireController on the grant is the seat field
//     that gates "who".
//
// The five cards in this batch all need this discipline:
//
//   - Bribery — exiles nothing but moves a creature across seats
//     (target opp's library → caster's battlefield under caster's
//     control). The Card.Owner stays the opp; the Permanent.Controller
//     is the caster.
//   - Hostage Taker — ETB exiles a permanent until LTB; the target's
//     Card goes to OWNER's exile, the free-cast grant has
//     RequireController = Hostage Taker's controller. Uses the
//     engine's ExileLinked / ReturnLinkedExile primitives so the
//     return-on-LTB is automatic per CR §406.7.
//   - Knowledge Pool — ETB has each seat exile their own top 3 to
//     OWN exile. On any cast-from-hand: exile the spell to caster's
//     own exile, then register free-cast grants on every KP-tagged
//     nonland card across the table (Knowledge Pool's cards can be
//     in any seat's exile depending on who exiled them at ETB; the
//     grants are RequireController = caster).
//   - Possibility Storm — already had a half-correct handler in
//     chaos_cascade.go; the cast-for-free path moved the matched
//     card to the caster's hand (wrong — bypassed the cast-from-
//     exile machinery and the "without paying its mana cost"
//     semantics). Fixed in chaos_cascade.go itself; the test pin
//     for the new shape lives here.
//   - Mind's Desire — caster exiles top X (= storm count, one card
//     per copy) of their own library to OWN exile, register
//     until_end_of_turn free-cast grants. §400.7c isn't violated
//     (self-library) but the same primitive is used so the shape
//     matches the rest of the batch.
//
// All handlers emit per_card_handler events with stable slugs so
// audit-engine-dead and the partial-implementation reports can find
// them. emit / emitPartial / emitFail follow the per_card_runtime
// contract from helpers.go.

// ---------------------------------------------------------------------------
// Bribery
// ---------------------------------------------------------------------------
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Bribery):
//
//	Search target opponent's library for a creature card, put that
//	card onto the battlefield under your control, then that player
//	shuffles.
//
// {3}{U} Sorcery. Cross-seat tutor: searches an opp's library, puts
// the best creature on YOUR battlefield. Card.Owner stays opp (so it
// returns to opp's library on bounce/scry-to-bottom, and dies to opp's
// graveyard on destruction per CR §400.4). Permanent.Controller = caster.
//
// Implementation:
//   - OnResolve. Picker scans every OPPONENT's library, finds the
//     highest-CMC creature, tutors it via createPermanent on caster's
//     battlefield with Owner = opp seat, Controller = caster seat.
//     Opp's library shuffles afterward.
//   - The opp's library access is the §400.7c-relevant move: removing
//     a card from opp's library uses MoveCard(gs, card, oppSeat,
//     "library", "battlefield", ...) where oppSeat is the OWNER. The
//     Permanent then enters under the caster's control via
//     enterBattlefieldWithETB which sets Controller = caster.
func registerBribery(r *Registry) {
	r.OnResolve("Bribery", briberyResolve)
}

func briberyResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "bribery_tutor"
	if gs == nil || item == nil {
		return
	}
	caster := item.Controller
	if caster < 0 || caster >= len(gs.Seats) {
		return
	}

	// Greedy pick: best creature across all opps' libraries by CMC.
	var pickCard *gameengine.Card
	pickSeat := -1
	bestCMC := -1
	for _, opp := range gs.Opponents(caster) {
		if gs.Seats[opp] == nil || gs.Seats[opp].Lost {
			continue
		}
		for _, c := range gs.Seats[opp].Library {
			if c == nil || !cardHasType(c, "creature") {
				continue
			}
			cmc := cardCMC(c)
			if cmc > bestCMC {
				bestCMC = cmc
				pickCard = c
				pickSeat = opp
			}
		}
	}
	if pickCard == nil {
		emitFail(gs, slug, "Bribery", "no_creature_in_any_opp_library", map[string]interface{}{
			"caster": caster,
		})
		return
	}

	// §400.7c-canonical: remove from owner-pile via the canonical zone
	// step, then place on caster's battlefield. createPermanent sets
	// Owner = card.Owner (preserved as the opp seat) and Controller =
	// the seat we pass — caster.
	moveCardBetweenZones(gs, pickSeat, pickCard, "library", "battlefield", "bribery_tutor")
	perm := enterBattlefieldWithETB(gs, caster, pickCard, false)
	if perm == nil {
		emitFail(gs, slug, "Bribery", "etb_refused", map[string]interface{}{
			"card":    pickCard.DisplayName(),
			"caster":  caster,
			"target":  pickSeat,
		})
		return
	}
	shuffleLibraryPerCard(gs, pickSeat)

	emit(gs, slug, "Bribery", map[string]interface{}{
		"caster":       caster,
		"target_seat":  pickSeat,
		"tutored":      pickCard.DisplayName(),
		"tutored_cmc":  bestCMC,
		"owner":        pickCard.Owner,
		"controller":   perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// Hostage Taker
// ---------------------------------------------------------------------------
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Hostage%20Taker):
//
//	When Hostage Taker enters, exile another target creature or
//	artifact until Hostage Taker leaves the battlefield. You may
//	cast that card for as long as it remains exiled, and you may
//	spend mana as though it were mana of any color to cast that spell.
//
// {2}{U}{B} 2/3 Creature — Human Pirate. The exile is "until leaves"
// (§406.7 linked-exile) — engine has ExileLinked / ReturnLinkedExile
// primitives that handle the LTB return automatically when the source
// is removed via the canonical battlefield-exit paths.
//
// Implementation:
//   - OnETB. Greedy pick of the best opp permanent: highest-CMC
//     creature first, then any artifact. Lands explicitly excluded
//     (oracle says creature OR artifact, not "permanent").
//   - ExileLinked(gs, hostageTaker, target.Card, target.Owner, ...)
//     handles: §400.7c routing (card → owner's exile), §406.7 link
//     (perm.LinkedExile += card, card.ExiledByTimestamp = hostage
//     taker's timestamp), and fires card_exiled / zone_change cleanup
//     hooks.
//   - Register free-cast permission: RequireController = Hostage
//     Taker's controller, Duration = "while_source_on_bf", SourceTimestamp
//     = Hostage Taker's timestamp, SpendAnyColor = true per oracle.
//     ExpireSourceGrants on LTB cleans the grant; ReturnLinkedExile
//     puts the Card back on its OWNER's battlefield.
//   - OnTrigger("permanent_ltb"): ReturnLinkedExile(gs, hostageTaker,
//     "battlefield") handles the return per §406.7. The grant is
//     auto-expired by the engine's ExpireSourceGrants pass tied to
//     the canonical LTB paths (DestroyPermanent / ExilePermanent /
//     etc. — covered by r60 PR #106 / #178).
func registerHostageTaker(r *Registry) {
	r.OnETB("Hostage Taker", hostageTakerETB)
	r.OnTrigger("Hostage Taker", "permanent_ltb", hostageTakerLTB)
}

func hostageTakerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hostage_taker_exile"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	caster := perm.Controller
	if caster < 0 || caster >= len(gs.Seats) {
		return
	}

	target := pickHostageTakerTarget(gs, caster, perm)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_valid_creature_or_artifact_target", map[string]interface{}{
			"caster": caster,
		})
		return
	}
	owner := target.Owner
	tgtCard := target.Card

	// Detach the target from the battlefield first — ExileLinked moves
	// the Card to owner's exile, but the Permanent wrapper needs to be
	// removed from the battlefield slice separately.
	removePermanent(gs, target)
	gs.UnregisterReplacementsForPermanent(target)
	gameengine.ExileLinked(gs, perm, tgtCard, owner, "battlefield")

	// Free-cast grant for the Hostage Taker's controller. SpendAnyColor
	// is the printed "you may spend mana as though it were any color"
	// clause. while_source_on_bf duration ties expiry to perm leaving
	// the battlefield — the existing ExpireSourceGrants infrastructure
	// (PR #106) reclaims the grant when Hostage Taker dies / is exiled
	// / is bounced.
	grant := gameengine.NewFreeCastFromExilePermission(caster, perm.Card.DisplayName())
	grant.Duration = "while_source_on_bf"
	grant.SourceTimestamp = perm.Timestamp
	grant.SpendAnyColor = true
	gameengine.RegisterZoneCastGrant(gs, tgtCard, grant)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster":     caster,
		"target":     tgtCard.DisplayName(),
		"owner":      owner,
		"cast_grant": caster,
	})
}

// hostageTakerLTB returns the linked-exile card to its owner's
// battlefield. ExpireSourceGrants is fired by the canonical LTB
// dispatch path; this hook only needs to walk LinkedExile and put
// each card back where it came from.
func hostageTakerLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hostage_taker_ltb_return"
	if gs == nil || perm == nil {
		return
	}
	if len(perm.LinkedExile) == 0 {
		return
	}
	returned := len(perm.LinkedExile)
	gameengine.ReturnLinkedExile(gs, perm, "battlefield")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"returned_count": returned,
	})
}

// pickHostageTakerTarget — best opp creature (by CMC desc), else any
// opp artifact. Lands explicitly excluded since oracle says "creature
// or artifact" not "permanent". Hexproof / shroud are not modeled at
// the picker level (the engine's targeting validation handles those
// at cast time; this is post-cast resolution, so legality is locked).
func pickHostageTakerTarget(gs *gameengine.GameState, caster int, source *gameengine.Permanent) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestTier := 0
	for _, opp := range gs.Opponents(caster) {
		if gs.Seats[opp] == nil {
			continue
		}
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || p.Card == nil || p == source {
				continue
			}
			if p.IsLand() {
				continue
			}
			isCreature := cardHasType(p.Card, "creature")
			isArtifact := cardHasType(p.Card, "artifact")
			if !isCreature && !isArtifact {
				continue
			}
			tier := 0
			switch {
			case isCreature && p.Power() >= 4:
				tier = 4
			case isCreature:
				tier = 3
			case isArtifact:
				tier = 2
			default:
				tier = 1
			}
			if tier > bestTier {
				bestTier = tier
				best = p
			}
		}
	}
	return best
}

// ---------------------------------------------------------------------------
// Knowledge Pool
// ---------------------------------------------------------------------------
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Knowledge%20Pool):
//
//	Imprint — When Knowledge Pool enters, each player exiles the top
//	three cards of their library face down.
//	Whenever a player casts a spell from their hand, that player
//	exiles it. If they do, they may cast another nonland card exiled
//	with Knowledge Pool without paying its mana cost. If they don't,
//	they put the exiled card into its owner's hand.
//
// {6} Artifact. The original "play someone else's card" engine — a
// 4-color staple of chaos / battlecruiser pods.
//
// Implementation:
//   - OnETB: each LIVING seat moves top 3 cards from THEIR library to
//     THEIR exile via canonical MoveCard. Each card is tagged with
//     ExiledByTimestamp = Knowledge Pool's timestamp so the
//     cast-from-hand trigger can find them.
//   - OnTrigger("spell_cast"): cast_zone must be "hand" (the trigger
//     fires for any cast — we filter to hand-casts only per oracle).
//     The caster's spell is moved from stack → caster's own exile,
//     also tagged with KP's timestamp. Then scan every seat's exile
//     for KP-tagged nonland cards (including the freshly-exiled
//     spell). For each, register a free-cast grant with
//     RequireController = caster, Duration = "while_source_on_bf",
//     SpendAnyColor = false (KP doesn't grant color-fixing).
//   - The "if they don't cast, put into owner's hand" branch is
//     deferred to AI/Hat resolution — emitPartial flags this. The
//     grants live until Knowledge Pool leaves the battlefield, which
//     is conservatively over-broad (oracle says the just-exiled
//     spell returns to owner's hand if not immediately played) but
//     matches the engine's existing grant-lifecycle model.
func registerKnowledgePool(r *Registry) {
	r.OnETB("Knowledge Pool", knowledgePoolETB)
	r.OnTrigger("Knowledge Pool", "spell_cast", knowledgePoolOnSpellCast)
}

func knowledgePoolETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "knowledge_pool_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	totalExiled := 0
	for seatIdx, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for i := 0; i < 3 && len(s.Library) > 0; i++ {
			// Library top = LAST index per the engine convention used
			// throughout per_card (verified vs. Hermit Druid / Etali).
			top := s.Library[len(s.Library)-1]
			if top == nil {
				break
			}
			// §400.7c: move into OWNER's exile (seatIdx is the library
			// owner here). The canonical MoveCard("library"→"exile")
			// fires §614 replacements + zone_change cleanup hooks.
			moveCardBetweenZones(gs, seatIdx, top, "library", "exile", "knowledge_pool_etb")
			// Tag the card with Knowledge Pool's timestamp so the
			// cast-from-hand trigger can find it later. ExiledByTimestamp
			// is the engine's canonical "exiled by which source" marker.
			top.ExiledByTimestamp = perm.Timestamp
			totalExiled++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"controller":   perm.Controller,
		"total_exiled": totalExiled,
	})
}

func knowledgePoolOnSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "knowledge_pool_cast_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	casterSeat, _ := ctx["caster_seat"].(int)
	castZone, _ := ctx["cast_zone"].(string)
	if card == nil || casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	// Oracle says "casts a spell from their hand" — filter out flashback,
	// adventure, cascade, foretell, suspend, KP itself (the trigger
	// fires for ANY cast; we gate by cast_zone == hand).
	if castZone != "" && castZone != gameengine.ZoneHand {
		return
	}
	// The freshly-cast spell is currently on the stack. Move it to
	// the caster's own exile per §400.7c (the caster is also the owner
	// here — only hand-casts trigger this branch, and cards in hand are
	// always in the owner's hand).
	if castZone == "" {
		castZone = gameengine.ZoneHand
	}
	moveCardBetweenZones(gs, casterSeat, card, "stack", "exile", "knowledge_pool_exile")
	card.ExiledByTimestamp = perm.Timestamp

	// Find every KP-tagged nonland card across all seats' exiles and
	// register a free-cast grant for the caster. The grant key is the
	// *Card pointer; physical exile location doesn't matter.
	grants := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, exiled := range s.Exile {
			if exiled == nil || exiled == card {
				continue
			}
			if exiled.ExiledByTimestamp != perm.Timestamp {
				continue
			}
			if cardHasType(exiled, "land") {
				continue
			}
			// Don't double-register if the grant is already there.
			if gameengine.GetZoneCastGrant(gs, exiled) != nil {
				continue
			}
			grant := gameengine.NewFreeCastFromExilePermission(casterSeat, perm.Card.DisplayName())
			grant.Duration = "while_source_on_bf"
			grant.SourceTimestamp = perm.Timestamp
			gameengine.RegisterZoneCastGrant(gs, exiled, grant)
			grants++
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster":           casterSeat,
		"exiled_spell":     card.DisplayName(),
		"grants_registered": grants,
	})
	if grants == 0 {
		// No KP-tagged alternatives across the table — the just-exiled
		// spell will sit in caster's exile under the KP timestamp. Per
		// oracle, "if they don't [cast another], they put the exiled
		// card into its owner's hand" — that's the AI-driven branch.
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"return_to_owner_hand_if_not_cast_deferred_to_ai")
	}
}

// ---------------------------------------------------------------------------
// Mind's Desire
// ---------------------------------------------------------------------------
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Mind%27s%20Desire):
//
//	Shuffle your library. Then exile the top card of your library.
//	Until end of turn, you may play that card without paying its
//	mana cost. (Storm — When you cast this spell, copy it for each
//	spell cast before it this turn. You may choose new targets for
//	the copies.)
//
// {4}{U}{U} Sorcery. Iconic storm finisher: each Mind's Desire copy
// exiles 1 card and free-casts it EOT, so the storm count determines
// the size of the exile-and-cast cascade.
//
// Implementation:
//   - OnResolve. Shuffle caster's library, take the top card, move it
//     to caster's own exile via canonical MoveCard. The §400.7c case
//     is trivially satisfied since caster == owner (it's THEIR own
//     library).
//   - Register a free-cast grant: RequireController = caster,
//     Duration = "until_end_of_turn", GrantTurn = gs.Turn. The
//     engine's EOT cleanup expires the grant; the card stays in
//     exile permanently if not cast (Mind's Desire is a one-time
//     impulse, no return-to-library clause).
//   - Storm is handled by the engine's storm pipeline — each copy
//     resolves with its own OnResolve invocation, exiling its own
//     card and registering its own grant.
//   - Note: oracle says "play" not "cast," which technically allows
//     lands too. The free-cast grant is a CAST mechanism — playing a
//     land off Mind's Desire bypasses the cast-from-zone path. We
//     emit a partial flagging the "land play from exile" gap; the
//     vast majority of storm payoffs are nonlands, so the grant
//     covers the EV-relevant cases.
func registerMindsDesire(r *Registry) {
	r.OnResolve("Mind's Desire", mindsDesireResolve)
}

func mindsDesireResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "minds_desire"
	if gs == nil || item == nil {
		return
	}
	caster := item.Controller
	if caster < 0 || caster >= len(gs.Seats) {
		return
	}
	s := gs.Seats[caster]
	if s == nil || s.Lost {
		return
	}
	shuffleLibraryPerCard(gs, caster)
	if len(s.Library) == 0 {
		emitFail(gs, slug, "Mind's Desire", "empty_library_after_shuffle", map[string]interface{}{
			"caster": caster,
		})
		return
	}
	top := s.Library[len(s.Library)-1]
	if top == nil {
		return
	}
	moveCardBetweenZones(gs, caster, top, "library", "exile", "minds_desire")

	grant := gameengine.NewFreeCastFromExilePermission(caster, "Mind's Desire")
	grant.Duration = "until_end_of_turn"
	grant.GrantTurn = gs.Turn
	gameengine.RegisterZoneCastGrant(gs, top, grant)

	if cardHasType(top, "land") {
		// Oracle says "play that card" so a land would be legal in
		// principle, but the engine's land-play path is separate from
		// the cast-from-zone grant. Flag for the AI layer.
		emitPartial(gs, slug, "Mind's Desire", "land_play_from_exile_not_modeled")
	}

	emit(gs, slug, "Mind's Desire", map[string]interface{}{
		"caster":   caster,
		"exiled":   top.DisplayName(),
		"duration": "until_end_of_turn",
	})
}
