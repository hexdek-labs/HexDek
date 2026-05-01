package per_card

import (
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Commander staples — high-frequency cards across the 1,135-deck pool
// that were missing per-card handlers, causing incorrect simulation
// outcomes and ELO skew.
//
// Cards in this file:
//   - Sol Ring (fast mana)
//   - Mana Vault (fast mana + upkeep tax)
//   - Dark Ritual (BBB ritual)
//   - Smothering Tithe (tax draw trigger)
//   - Windfall (wheel)
//   - Wheel of Fortune (wheel)
//   - Entomb (graveyard tutor)
//   - Reanimate (reanimation)
//   - Animate Dead (reanimation enchantment)
//   - Force of Will (free counter)
//   - Force of Negation (free counter, opponents' turns)
//   - Fierce Guardianship (free counter, commander-gated)
//   - Lion's Eye Diamond (ritual + discard)
//   - Dauthi Voidwalker (exile opponent GY entries)
//   - Yawgmoth, Thran Physician (sac engine + draw)
//   - Gilded Drake (control-swap creature)
//   - Survival of the Fittest (creature tutor, repeatable)
// ============================================================================

// ---------------------------------------------------------------------------
// Sol Ring — {T}: Add {C}{C}.
// The most-played card in Commander. 0 mana → 2 colorless is tempo-
// defining on turn 1.
// ---------------------------------------------------------------------------

func registerSolRing(r *Registry) {
	r.OnActivated("Sol Ring", solRingActivate)
}

func solRingActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sol_ring_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Sol Ring", "already_tapped", nil)
		return
	}
	src.Tapped = true
	seat := src.Controller
	s := gs.Seats[seat]
	s.ManaPool += 2
	gameengine.SyncManaAfterAdd(s, 2)
	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   seat,
		Target: seat,
		Source: "Sol Ring",
		Amount: 2,
		Details: map[string]interface{}{
			"reason": "sol_ring_tap",
		},
	})
	emit(gs, slug, "Sol Ring", map[string]interface{}{
		"seat":     seat,
		"added":    2,
		"new_pool": s.ManaPool,
	})
}

// ---------------------------------------------------------------------------
// Mana Vault — {T}: Add {C}{C}{C}.
// At the beginning of your upkeep, if Mana Vault is tapped, it deals
// 1 damage to you. Does not untap during your untap step. {4}: Untap.
// ---------------------------------------------------------------------------

func registerManaVault(r *Registry) {
	r.OnActivated("Mana Vault", manaVaultActivate)
	r.OnTrigger("Mana Vault", "upkeep_controller", manaVaultUpkeep)
}

func manaVaultActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	switch abilityIdx {
	case 0:
		// {T}: Add {C}{C}{C}
		const slug = "mana_vault_tap"
		if src.Tapped {
			emitFail(gs, slug, "Mana Vault", "already_tapped", nil)
			return
		}
		src.Tapped = true
		seat := src.Controller
		s := gs.Seats[seat]
		s.ManaPool += 3
		gameengine.SyncManaAfterAdd(s, 3)
		gs.LogEvent(gameengine.Event{
			Kind:   "add_mana",
			Seat:   seat,
			Target: seat,
			Source: "Mana Vault",
			Amount: 3,
			Details: map[string]interface{}{
				"reason": "mana_vault_tap",
			},
		})
		emit(gs, slug, "Mana Vault", map[string]interface{}{
			"seat":     seat,
			"added":    3,
			"new_pool": s.ManaPool,
		})
	case 1:
		// {4}: Untap Mana Vault
		const slug = "mana_vault_untap"
		seat := src.Controller
		s := gs.Seats[seat]
		if s.ManaPool < 4 {
			emitFail(gs, slug, "Mana Vault", "insufficient_mana", nil)
			return
		}
		s.ManaPool -= 4
		gameengine.SyncManaAfterSpend(s)
		src.Tapped = false
		emit(gs, slug, "Mana Vault", map[string]interface{}{
			"seat": seat,
		})
	}
}

func manaVaultUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mana_vault_upkeep"
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if !perm.Tapped {
		return
	}
	// Deals 1 damage to controller while tapped.
	seat := perm.Controller
	gs.Seats[seat].Life -= 1
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   seat,
		Target: seat,
		Source: "Mana Vault",
		Amount: 1,
		Details: map[string]interface{}{
			"reason": "mana_vault_upkeep_tapped",
		},
	})
	emit(gs, slug, "Mana Vault", map[string]interface{}{
		"seat":   seat,
		"damage": 1,
	})
	_ = gs.CheckEnd()
}

// ---------------------------------------------------------------------------
// Dark Ritual — Add {B}{B}{B}.
// B instant. THE fast-mana ritual.
// ---------------------------------------------------------------------------

func registerDarkRitual(r *Registry) {
	r.OnResolve("Dark Ritual", darkRitualResolve)
}

func darkRitualResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "dark_ritual"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	s.ManaPool += 3
	gameengine.SyncManaAfterAdd(s, 3)
	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   seat,
		Target: seat,
		Source: "Dark Ritual",
		Amount: 3,
		Details: map[string]interface{}{
			"reason": "dark_ritual",
			"pool":   "BBB",
		},
	})
	emit(gs, slug, "Dark Ritual", map[string]interface{}{
		"seat":     seat,
		"added":    3,
		"new_pool": s.ManaPool,
	})
}

// ---------------------------------------------------------------------------
// Smothering Tithe
//
// Oracle text:
//   Whenever an opponent draws a card, that player may pay {2}. If the
//   player doesn't, you create a Treasure token.
//
// 3W enchantment. One of the most impactful white cards in Commander.
// In practice, opponents almost never pay the 2 tax — they'd rather
// give the Tithe controller a Treasure than lose 2 mana.
//
// Implementation: trigger on opponent_draw. AI policy: opponents never
// pay the tax (correct for cEDH where tempo > treasure denial).
// ---------------------------------------------------------------------------

func registerSmotheringTithe(r *Registry) {
	r.OnTrigger("Smothering Tithe", "opponent_draws", smotheringTitheTrigger)
	r.OnTrigger("Smothering Tithe", "card_drawn", smotheringTitheTrigger)
}

func smotheringTitheTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "smothering_tithe_treasure"
	if gs == nil || perm == nil {
		return
	}
	// Determine who drew. In the engine's draw observer pattern,
	// ctx["drawer_seat"] holds the seat that drew.
	drawerSeat, ok := ctx["drawer_seat"].(int)
	if !ok {
		return
	}
	// Only triggers when an OPPONENT draws.
	if drawerSeat == perm.Controller {
		return
	}
	// AI policy: opponent doesn't pay {2}. Create a Treasure for the
	// Tithe controller.
	gameengine.CreateTreasureToken(gs, perm.Controller)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   perm.Controller,
		Source: "Smothering Tithe",
		Details: map[string]interface{}{
			"token":       "Treasure Token",
			"reason":      "smothering_tithe_opponent_drew",
			"drawer_seat": drawerSeat,
		},
	})
	emit(gs, slug, "Smothering Tithe", map[string]interface{}{
		"controller":  perm.Controller,
		"drawer_seat": drawerSeat,
	})
}

// ---------------------------------------------------------------------------
// Windfall
//
// Oracle text:
//   Each player discards their hand, then draws cards equal to the
//   greatest number of cards a player discarded this way.
//
// 2U sorcery. Wheel effect keyed on highest hand size before discard.
// ---------------------------------------------------------------------------

func registerWindfall(r *Registry) {
	r.OnResolve("Windfall", windfallResolve)
}

func windfallResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "windfall"
	if gs == nil || item == nil {
		return
	}
	// Determine the greatest hand size before discard.
	maxDiscarded := 0
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if len(s.Hand) > maxDiscarded {
			maxDiscarded = len(s.Hand)
		}
	}
	// Each player discards their hand.
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for len(s.Hand) > 0 {
			gameengine.DiscardCard(gs, s.Hand[0], i)
		}
	}
	// Each player draws cards equal to the max.
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for j := 0; j < maxDiscarded && len(s.Library) > 0; j++ {
			card := s.Library[0]
			gameengine.MoveCard(gs, card, i, "library", "hand", "draw")
		}
	}
	emit(gs, slug, "Windfall", map[string]interface{}{
		"seat":          item.Controller,
		"max_discarded": maxDiscarded,
	})
}

// ---------------------------------------------------------------------------
// Wheel of Fortune
//
// Oracle text:
//   Each player discards their hand, then draws seven cards.
//
// 2R sorcery. The classic wheel — everyone goes to 7.
// ---------------------------------------------------------------------------

func registerWheelOfFortune(r *Registry) {
	r.OnResolve("Wheel of Fortune", wheelOfFortuneResolve)
}

func wheelOfFortuneResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "wheel_of_fortune"
	if gs == nil || item == nil {
		return
	}
	// Each player discards their hand.
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for len(s.Hand) > 0 {
			gameengine.DiscardCard(gs, s.Hand[0], i)
		}
	}
	// Each player draws 7.
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for j := 0; j < 7 && len(s.Library) > 0; j++ {
			card := s.Library[0]
			gameengine.MoveCard(gs, card, i, "library", "hand", "draw")
		}
	}
	emit(gs, slug, "Wheel of Fortune", map[string]interface{}{
		"seat": item.Controller,
	})
}

// ---------------------------------------------------------------------------
// Entomb
//
// Oracle text:
//   Search your library for a card, put that card into your graveyard,
//   then shuffle your library.
//
// B instant. Graveyard tutor — sets up Reanimate / Animate Dead / Breach.
// ---------------------------------------------------------------------------

func registerEntomb(r *Registry) {
	r.OnResolve("Entomb", entombResolve)
}

func entombResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "entomb"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		emitFail(gs, slug, "Entomb", "library_empty", nil)
		return
	}
	// Policy: find the best creature to reanimate. Prefer highest CMC
	// creature; fallback to any card.
	bestIdx := -1
	bestCMC := -1
	for i, c := range s.Library {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") {
			cmc := cardCMC(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestIdx = i
			}
		}
	}
	if bestIdx < 0 {
		// No creature found; just pick the first card.
		bestIdx = 0
	}
	card := s.Library[bestIdx]
	gameengine.MoveCard(gs, card, seat, "library", "graveyard", "entomb")
	shuffleLibraryPerCard(gs, seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: "Entomb",
		Details: map[string]interface{}{
			"found_card":  card.DisplayName(),
			"destination": "graveyard",
			"reason":      "entomb",
		},
	})
	emit(gs, slug, "Entomb", map[string]interface{}{
		"seat":     seat,
		"entombed": card.DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// Reanimate
//
// Oracle text:
//   Put target creature card from a graveyard onto the battlefield
//   under your control. You lose life equal to its mana value.
//
// B sorcery. The best single-target reanimation spell. Can target ANY
// graveyard, not just your own.
// ---------------------------------------------------------------------------

func registerReanimate(r *Registry) {
	r.OnResolve("Reanimate", reanimateResolve)
}

func reanimateResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "reanimate"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Find the best creature in any graveyard (highest CMC).
	var bestCard *gameengine.Card
	bestCMC := -1
	bestSeat := -1
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil || !cardHasType(c, "creature") {
				continue
			}
			cmc := cardCMC(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
				bestSeat = i
			}
		}
	}
	if bestCard == nil {
		emitFail(gs, slug, "Reanimate", "no_creature_in_any_graveyard", nil)
		return
	}
	// Move from graveyard to battlefield under controller's control.
	gameengine.MoveCard(gs, bestCard, bestSeat, "graveyard", "battlefield", "reanimate")
	perm := enterBattlefieldWithETB(gs, seat, bestCard, false)
	// Lose life equal to CMC.
	lifeLost := cardCMC(bestCard)
	gs.Seats[seat].Life -= lifeLost
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: "Reanimate",
		Amount: lifeLost,
		Details: map[string]interface{}{
			"reason":     "reanimate_life_cost",
			"reanimated": bestCard.DisplayName(),
			"cmc":        lifeLost,
		},
	})
	emit(gs, slug, "Reanimate", map[string]interface{}{
		"seat":       seat,
		"reanimated": bestCard.DisplayName(),
		"life_lost":  lifeLost,
		"from_seat":  bestSeat,
	})
	_ = perm // suppress unused warning if enterBattlefieldWithETB returns nil
	_ = gs.CheckEnd()
}

// ---------------------------------------------------------------------------
// Animate Dead
//
// Oracle text:
//   Enchant creature card in a graveyard. When Animate Dead enters the
//   battlefield, if it's on the battlefield, it loses "enchant creature
//   card in a graveyard" and gains "enchant creature put onto the
//   battlefield with Animate Dead." Return enchanted creature card to
//   the battlefield under your control and attach Animate Dead to it.
//   When Animate Dead leaves the battlefield, that creature's controller
//   sacrifices it. Enchanted creature gets -1/-0.
//
// 1B enchantment. Simplified: reanimate best creature from any GY; if
// Animate Dead leaves play, sacrifice the creature (delayed trigger).
// ---------------------------------------------------------------------------

func registerAnimateDead(r *Registry) {
	r.OnETB("Animate Dead", animateDeadETB)
}

var animateDeadTargets sync.Map

func animateDeadETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "animate_dead_reanimate"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Find the best creature in any graveyard (highest CMC).
	var bestCard *gameengine.Card
	bestCMC := -1
	bestSeat := -1
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil || !cardHasType(c, "creature") {
				continue
			}
			cmc := cardCMC(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
				bestSeat = i
			}
		}
	}
	if bestCard == nil {
		emitFail(gs, slug, "Animate Dead", "no_creature_in_any_graveyard", nil)
		return
	}
	// Move from graveyard to battlefield under controller's control.
	gameengine.MoveCard(gs, bestCard, bestSeat, "graveyard", "battlefield", "animate-dead")
	creature := enterBattlefieldWithETB(gs, seat, bestCard, false)
	if creature != nil {
		// Apply -1/-0 modifier.
		creature.Card.BasePower -= 1
		animateDeadTargets.Store(perm, creature)
	}
	emit(gs, slug, "Animate Dead", map[string]interface{}{
		"seat":       seat,
		"reanimated": bestCard.DisplayName(),
		"from_seat":  bestSeat,
	})
}

// ---------------------------------------------------------------------------
// Force of Will
//
// Oracle text:
//   You may pay 1 life and exile a blue card from your hand rather than
//   pay this spell's mana cost.
//   Counter target spell.
//
// 3UU instant. THE free counter. In our model we resolve it as a
// straight counterspell since the engine's cost-payment path handles
// the alternate cost at cast time.
// ---------------------------------------------------------------------------

func registerForceOfWill(r *Registry) {
	r.OnResolve("Force of Will", forceOfWillResolve)
}

func forceOfWillResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "force_of_will"
	if gs == nil || item == nil {
		return
	}
	target := findCounterableSpell(gs, item.Controller, nil)
	if target == nil {
		emitFail(gs, slug, "Force of Will", "no_spell_on_stack", nil)
		return
	}
	target.Countered = true
	emitCounter(gs, slug, "Force of Will", item.Controller, target)
}

// ---------------------------------------------------------------------------
// Force of Negation
//
// Oracle text:
//   If it's not your turn, you may exile a blue card from your hand
//   rather than pay this spell's mana cost.
//   Counter target noncreature spell.
//
// 1UU instant. Free counter for noncreature spells on opponents' turns.
// ---------------------------------------------------------------------------

func registerForceOfNegation(r *Registry) {
	r.OnResolve("Force of Negation", forceOfNegationResolve)
}

func forceOfNegationResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "force_of_negation"
	if gs == nil || item == nil {
		return
	}
	target := findCounterableSpell(gs, item.Controller, func(si *gameengine.StackItem) bool {
		return !isCreatureSpell(si)
	})
	if target == nil {
		emitFail(gs, slug, "Force of Negation", "no_noncreature_spell_on_stack", nil)
		return
	}
	target.Countered = true
	// If countered spell would go to graveyard, exile it instead.
	if target.Card != nil {
		target.CostMeta = map[string]interface{}{
			"exile_on_resolve": true,
		}
	}
	emitCounter(gs, slug, "Force of Negation", item.Controller, target)
}

// ---------------------------------------------------------------------------
// Fierce Guardianship
//
// Oracle text:
//   If you control a commander, you may cast this spell without paying
//   its mana cost.
//   Counter target noncreature spell.
//
// 2U instant. Free counter when your commander is in play.
// ---------------------------------------------------------------------------

func registerFierceGuardianship(r *Registry) {
	r.OnResolve("Fierce Guardianship", fierceGuardianshipResolve)
}

func fierceGuardianshipResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "fierce_guardianship"
	if gs == nil || item == nil {
		return
	}
	target := findCounterableSpell(gs, item.Controller, func(si *gameengine.StackItem) bool {
		return !isCreatureSpell(si)
	})
	if target == nil {
		emitFail(gs, slug, "Fierce Guardianship", "no_noncreature_spell_on_stack", nil)
		return
	}
	target.Countered = true
	emitCounter(gs, slug, "Fierce Guardianship", item.Controller, target)
}

// ---------------------------------------------------------------------------
// Lion's Eye Diamond
//
// Oracle text:
//   Sacrifice Lion's Eye Diamond, Discard your hand: Add three mana
//   of any one color. Activate only as an instant.
//
// 0-cost artifact. Key combo piece with Underworld Breach / Auriok
// Salvagers. The "discard your hand" cost is brutal but in combo turns
// the hand is often empty or irrelevant.
// ---------------------------------------------------------------------------

func registerLionsEyeDiamond(r *Registry) {
	r.OnActivated("Lion's Eye Diamond", lionsEyeDiamondActivate)
}

func lionsEyeDiamondActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "lions_eye_diamond"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Cost: sacrifice LED + discard entire hand.
	gameengine.SacrificePermanent(gs, src, "lions_eye_diamond_cost")
	discarded := 0
	for len(s.Hand) > 0 {
		gameengine.DiscardCard(gs, s.Hand[0], seat)
		discarded++
	}

	// Effect: add 3 mana of any color (generic in MVP).
	s.ManaPool += 3
	gameengine.SyncManaAfterAdd(s, 3)
	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   seat,
		Target: seat,
		Source: "Lion's Eye Diamond",
		Amount: 3,
		Details: map[string]interface{}{
			"reason":    "lions_eye_diamond",
			"discarded": discarded,
		},
	})
	emit(gs, slug, "Lion's Eye Diamond", map[string]interface{}{
		"seat":      seat,
		"mana":      3,
		"discarded": discarded,
		"new_pool":  s.ManaPool,
	})
}

// ---------------------------------------------------------------------------
// Dauthi Voidwalker
//
// Oracle text:
//   Shadow. If a card would be put into an opponent's graveyard from
//   anywhere, instead exile it with a void counter on it. {T}, Sacrifice
//   Dauthi Voidwalker: Choose an exiled card with a void counter on it.
//   You may play it this turn without paying its mana cost.
//
// 1B creature. Anti-graveyard stax piece. In our model:
//   - ETB: flag that opponents' cards go to exile instead of GY.
//   - Activated: sacrifice to play an exiled card for free.
// ---------------------------------------------------------------------------

func registerDauthiVoidwalker(r *Registry) {
	r.OnETB("Dauthi Voidwalker", dauthiVoidwalkerETB)
	r.OnActivated("Dauthi Voidwalker", dauthiVoidwalkerActivate)
}

func dauthiVoidwalkerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dauthi_voidwalker_static"
	if gs == nil || perm == nil {
		return
	}
	// Register replacement effects via the replacement engine so
	// opponents' cards are exiled (with void counters) instead of
	// going to graveyard. RegisterReplacementsForPermanent dispatches
	// to RegisterDauthiVoidwalker in replacement.go.
	gameengine.RegisterReplacementsForPermanent(gs, perm)
	emit(gs, slug, "Dauthi Voidwalker", map[string]interface{}{
		"seat":    perm.Controller,
		"effect":  "opponent_cards_exiled_instead_of_graveyard",
		"shadow":  true,
	})
}

func dauthiVoidwalkerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "dauthi_voidwalker_play_exiled"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Sacrifice Voidwalker as cost. This also unregisters its
	// replacement effects via UnregisterReplacementsForPermanent
	// (called from the sacrifice path on LTB).
	gameengine.SacrificePermanent(gs, src, "dauthi_voidwalker_cost")

	// Find an exiled card with a void counter. MVP: pick any opponent's
	// exiled card (highest CMC creature preferred).
	var bestCard *gameengine.Card
	bestCMC := -1
	bestSeat := -1
	for i, s := range gs.Seats {
		if s == nil || i == seat {
			continue
		}
		for _, c := range s.Exile {
			if c == nil {
				continue
			}
			cmc := cardCMC(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
				bestSeat = i
			}
		}
	}
	if bestCard == nil {
		emitFail(gs, slug, "Dauthi Voidwalker", "no_exiled_card", nil)
		return
	}
	// Grant free cast from exile.
	freePerm := gameengine.NewFreeCastFromExilePermission(seat, "Dauthi Voidwalker")
	gameengine.RegisterZoneCastGrant(gs, bestCard, freePerm)
	emit(gs, slug, "Dauthi Voidwalker", map[string]interface{}{
		"seat":         seat,
		"exiled_card":  bestCard.DisplayName(),
		"from_seat":    bestSeat,
		"free_cast":    true,
	})
}

// ---------------------------------------------------------------------------
// Yawgmoth, Thran Physician
//
// Oracle text:
//   Protection from Humans.
//   Pay 1 life, Sacrifice another creature: Put a -1/-1 counter on up
//   to one target creature and draw a card.
//   {B}{B}, Discard a card: Proliferate.
//
// Sac engine + card draw. The first ability is the key one for combo
// decks: sacrifice creatures to draw cards and put -1/-1 counters.
// ---------------------------------------------------------------------------

func registerYawgmothThranPhysician(r *Registry) {
	r.OnActivated("Yawgmoth, Thran Physician", yawgmothActivate)
}

func yawgmothActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	switch abilityIdx {
	case 0:
		// Pay 1 life, sacrifice another creature: -1/-1 counter + draw.
		const slug = "yawgmoth_sac_draw"
		// Find a creature to sacrifice (not Yawgmoth itself).
		var victim *gameengine.Permanent
		for _, p := range s.Battlefield {
			if p == nil || p == src || !p.IsCreature() {
				continue
			}
			victim = p
			break
		}
		if victim == nil {
			emitFail(gs, slug, "Yawgmoth, Thran Physician", "no_creature_to_sacrifice", nil)
			return
		}
		// Pay 1 life.
		s.Life -= 1
		// Sacrifice the creature.
		gameengine.SacrificePermanent(gs, victim, "yawgmoth_sac_cost")
		// Draw a card.
		if len(s.Library) > 0 {
			card := s.Library[0]
			gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
		}
		// Put -1/-1 counter on up to one target creature (best: opponent's
		// smallest creature, or skip if none).
		var counterTarget *gameengine.Permanent
		for _, opp := range gs.Opponents(seat) {
			os := gs.Seats[opp]
			if os == nil {
				continue
			}
			for _, p := range os.Battlefield {
				if p != nil && p.IsCreature() {
					counterTarget = p
					break
				}
			}
			if counterTarget != nil {
				break
			}
		}
		if counterTarget != nil {
			if counterTarget.Counters == nil {
				counterTarget.Counters = map[string]int{}
			}
			counterTarget.Counters["-1/-1"]++
		}
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
		emit(gs, slug, "Yawgmoth, Thran Physician", map[string]interface{}{
			"seat":          seat,
			"sacrificed":    victim.Card.DisplayName(),
			"drew_card":     true,
			"counter_placed": counterTarget != nil,
		})
		_ = gs.CheckEnd()
	case 1:
		// {B}{B}, discard a card: Proliferate.
		const slug = "yawgmoth_proliferate"
		if len(s.Hand) == 0 {
			emitFail(gs, slug, "Yawgmoth, Thran Physician", "no_card_to_discard", nil)
			return
		}
		if s.ManaPool < 2 {
			emitFail(gs, slug, "Yawgmoth, Thran Physician", "insufficient_mana", nil)
			return
		}
		s.ManaPool -= 2
		gameengine.SyncManaAfterSpend(s)
		gameengine.DiscardCard(gs, s.Hand[0], seat)
		// Proliferate: add a counter of a type already present on each
		// permanent/player. MVP: increment each counter type on each
		// permanent we control by 1 (poison, +1/+1, etc.).
		for _, p := range s.Battlefield {
			if p == nil || p.Counters == nil {
				continue
			}
			for cType := range p.Counters {
				if p.Counters[cType] > 0 {
					p.Counters[cType]++
				}
			}
		}
		emit(gs, slug, "Yawgmoth, Thran Physician", map[string]interface{}{
			"seat": seat,
		})
	}
}

// ---------------------------------------------------------------------------
// Gilded Drake
//
// Oracle text:
//   Flying. When Gilded Drake enters the battlefield, exchange control
//   of Gilded Drake and up to one target creature an opponent controls.
//   If you don't make an exchange, sacrifice Gilded Drake.
//
// 1U creature. One of the most powerful creature-steal effects. You give
// them a 3/3 flyer and take their best creature.
// ---------------------------------------------------------------------------

func registerGildedDrake(r *Registry) {
	r.OnETB("Gilded Drake", gildedDrakeETB)
}

func gildedDrakeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "gilded_drake_exchange"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	// Find the best opponent creature to steal.
	var bestTarget *gameengine.Permanent
	bestScore := -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil {
			continue
		}
		for _, p := range os.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			score := p.Card.BasePower + p.Card.BaseToughness + cardCMC(p.Card)
			if score > bestScore {
				bestScore = score
				bestTarget = p
			}
		}
	}
	if bestTarget == nil {
		// No target — sacrifice Gilded Drake.
		gameengine.SacrificePermanent(gs, perm, "gilded_drake_no_target")
		emitFail(gs, slug, "Gilded Drake", "no_opponent_creature", nil)
		return
	}
	// Exchange control: Drake goes to opponent, target comes to us.
	opponentSeat := bestTarget.Controller
	// Give Drake to opponent.
	removePermanent(gs, perm)
	perm.Controller = opponentSeat
	gs.Seats[opponentSeat].Battlefield = append(gs.Seats[opponentSeat].Battlefield, perm)
	// Take target creature.
	removePermanent(gs, bestTarget)
	bestTarget.Controller = seat
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, bestTarget)
	emit(gs, slug, "Gilded Drake", map[string]interface{}{
		"seat":         seat,
		"stole":        bestTarget.Card.DisplayName(),
		"gave_drake_to": opponentSeat,
	})
}

// ---------------------------------------------------------------------------
// Survival of the Fittest
//
// Oracle text:
//   {G}, Discard a creature card: Search your library for a creature
//   card, reveal that card, put it into your hand, then shuffle your
//   library.
//
// 1G enchantment. Repeatable creature tutor. Discard a creature, find
// any creature. Key for toolbox and reanimator strategies.
// ---------------------------------------------------------------------------

func registerSurvivalOfTheFittest(r *Registry) {
	r.OnActivated("Survival of the Fittest", survivalActivate)
}

func survivalActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "survival_of_the_fittest"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Cost: {G} + discard a creature card.
	// Find a creature in hand to discard.
	discardIdx := -1
	for i, c := range s.Hand {
		if c != nil && cardHasType(c, "creature") {
			discardIdx = i
			break
		}
	}
	if discardIdx < 0 {
		emitFail(gs, slug, "Survival of the Fittest", "no_creature_in_hand_to_discard", nil)
		return
	}
	discarded := s.Hand[discardIdx]
	gameengine.DiscardCard(gs, discarded, seat)

	// Effect: search library for a creature card, put it into hand.
	found := tutorToHand(gs, seat, func(c *gameengine.Card) bool {
		return cardHasType(c, "creature")
	}, "Survival of the Fittest")
	emit(gs, slug, "Survival of the Fittest", map[string]interface{}{
		"seat":      seat,
		"discarded": discarded.DisplayName(),
		"found":     found,
	})
}

// Note: this file uses findCounterableSpell, isCreatureSpell, and
// emitCounter from counterspells.go — they are package-level functions
// visible across the per_card package.
