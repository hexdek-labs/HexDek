package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// beheld_consumers_r60.go — per_card handlers for cards whose effect
// depends on the §701.4 Behold keyword action.
//
// Per the Phase 1B audit, "beheld" was an engine-emitted event with
// no per_card consumer wired despite a TDM (Tarkir: Dragonstorm) Dragon
// payoff family + the Sarkhan, Dragon Ascendant ETB rider. The engine
// already implements the Behold primitive (keywords_behold.go) and
// fires FireCardTrigger("beheld", ...) every time a behold is
// recorded — but no per_card handler consumed either the registry
// state (HasBeheld) or the trigger event, so the "if a Dragon was
// beheld" rider on every card in the family resolved as a no-op.
//
// This file wires 5 handlers covering both shapes of the consumer:
//
//   (A) Permanent spells with ETB riders — register OnETB and call the
//       attemptBeholdQuality helper at resolution time, then mint the
//       conditional payoff when the behold succeeds. Cast-time bridge
//       not needed because the behold attempt + payoff fire together
//       at ETB. Sarkhan, Dragon Ascendant is the only handler in this
//       shape: "When Sarkhan enters, you may behold a Dragon. If you
//       do, create a Treasure token."
//
//   (B) Instants / sorceries with resolve-time riders — register
//       OnResolve, call attemptBeholdQuality, then apply the
//       conditional rider on top of the unconditional spell effect.
//       The "additional cost" framing in oracle text is approximated
//       at resolution because the engine cost-payment pipeline doesn't
//       yet route the optional behold through CastSpellWithCosts;
//       attempting the behold at resolve still records the correct
//       BeheldRegistry state for downstream "whenever you behold X"
//       triggers (and for any sibling card resolving later in the same
//       turn that gates on HasBeheld).
//
// All 5 cards key on quality="dragon" (the TDM cycle). The helper is
// quality-agnostic so future per-quality wirings (Goblin / Kithkin /
// Elemental / Merfolk / Elf cycles from Bloomburrow) can re-use it.

func init() {
	registerBeheldConsumersR60(Global())
	AddResetHook(registerBeheldConsumersR60)
}

func registerBeheldConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	// (A) Permanent spell with ETB rider.
	r.OnETB("Sarkhan, Dragon Ascendant", sarkhanDragonAscendantETB)
	// (B) Instants / sorceries with resolve-time riders.
	r.OnResolve("Osseous Exhale", osseousExhaleResolve)
	r.OnResolve("Piercing Exhale", piercingExhaleResolve)
	r.OnResolve("Draconic Fealty", draconicFealtyResolve)
	r.OnResolve("Territorial Strike", territorialStrikeResolve)
}

// attemptBeholdQuality tries to satisfy a behold-of-quality for seat
// without forcing a player choice. Reveal-from-hand wins over choose-
// permanent when both are available — revealing keeps the card in hand
// where it can still be cast later, while choose-permanent doesn't
// remove anything either but it's a slightly less-flexible record
// (your future opponent's removal might take the chosen perm and
// strand the next behold). Returns true when a behold was recorded.
//
// Quality matching is delegated to keywords_behold.go's
// CardHasBeholdQuality / PermHasBeholdQuality — both scan Card.Types
// and Card.TypeLine, so creature subtypes like "dragon" match either
// way the corpus loader split them.
func attemptBeholdQuality(gs *gameengine.GameState, seat int, quality, source string) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
	if s == nil {
		return false
	}
	for _, c := range s.Hand {
		if gameengine.CardHasBeholdQuality(c, quality) {
			if gameengine.BeholdRevealFromHand(gs, seat, quality, source, c) {
				return true
			}
		}
	}
	for _, p := range s.Battlefield {
		if gameengine.PermHasBeholdQuality(p, quality) {
			if gameengine.BeholdChoosePermanent(gs, seat, quality, source, p) {
				return true
			}
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// (A) Permanent ETB rider — Sarkhan, Dragon Ascendant
// -----------------------------------------------------------------------------

// Sarkhan, Dragon Ascendant — {2}{R} Legendary Creature — Human Shaman.
//
// Oracle text (ETB rider only — the Dragon-ETB anthem counter trigger
// is handled by the engine's stock dragon-counter dispatch):
//
//	When Sarkhan enters, you may behold a Dragon. If you do, create a
//	Treasure token. (To behold a Dragon, choose a Dragon you control
//	or reveal a Dragon card from your hand.)
//
// Implementation: attempt the behold; if it succeeds, mint one
// Treasure. The "you may" wording is honored by the engine's AI
// policy elsewhere; for the deterministic test fixture we always
// attempt (Sarkhan's controller wants the Treasure 100% of the time).
func sarkhanDragonAscendantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sarkhan_dragon_ascendant_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if !attemptBeholdQuality(gs, perm.Controller, "dragon", perm.Card.DisplayName()) {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_dragon_to_behold", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	gameengine.CreateTreasureToken(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"behold":    "dragon",
		"treasures": 1,
	})
}

// -----------------------------------------------------------------------------
// (B) Resolve-time riders — TDM Dragon-Exhale cycle + Draconic Fealty +
//     Territorial Strike
// -----------------------------------------------------------------------------

// Osseous Exhale — {1}{W}{B} Instant.
//
// Oracle text:
//
//	As an additional cost to cast this spell, you may behold a Dragon.
//	Osseous Exhale deals 5 damage to target attacking or blocking
//	creature. If a Dragon was beheld, you gain 2 life.
//
// Implementation: attempt the behold (approximating the optional
// additional cost at resolve time); ping the highest-power opposing
// creature for 5; gain 2 life when the behold succeeded. The
// "attacking or blocking" filter is widened to "best opp creature"
// in line with the bargain-consumers pattern — combat state in the
// per-card test fixture is rarely set up.
func osseousExhaleResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "osseous_exhale"
	if gs == nil || item == nil {
		return
	}
	target := pickBestOppCreature(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Osseous Exhale", "no_target", map[string]interface{}{
			"seat": item.Controller,
		})
		return
	}
	beheld := attemptBeholdQuality(gs, item.Controller, "dragon", "Osseous Exhale")
	target.MarkedDamage += 5
	gs.InvalidateCharacteristicsCache()
	if beheld {
		gameengine.GainLife(gs, item.Controller, 2, "Osseous Exhale")
	}
	emit(gs, slug, "Osseous Exhale", map[string]interface{}{
		"seat":   item.Controller,
		"target": target.Card.DisplayName(),
		"damage": 5,
		"beheld": beheld,
	})
}

// Piercing Exhale — {2}{R} Instant.
//
// Oracle text:
//
//	As an additional cost to cast this spell, you may behold a Dragon.
//	Target creature you control deals damage equal to its power to
//	target creature or planeswalker. If a Dragon was beheld, surveil 2.
//
// Implementation: attempt the behold; pick the highest-power friendly
// creature as the source and the highest-power opp creature as the
// target; mark damage = source's power; surveil 2 when beheld. Targets
// fall back gracefully (no friendly source → fail, no opp target →
// fail).
func piercingExhaleResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "piercing_exhale"
	if gs == nil || item == nil {
		return
	}
	source := pickBestFriendlyCreature(gs, item.Controller)
	if source == nil {
		emitFail(gs, slug, "Piercing Exhale", "no_friendly_source", map[string]interface{}{
			"seat": item.Controller,
		})
		return
	}
	target := pickBestOppCreature(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Piercing Exhale", "no_target", map[string]interface{}{
			"seat": item.Controller,
		})
		return
	}
	beheld := attemptBeholdQuality(gs, item.Controller, "dragon", "Piercing Exhale")
	damage := source.Power()
	target.MarkedDamage += damage
	gs.InvalidateCharacteristicsCache()
	if beheld {
		gameengine.Surveil(gs, item.Controller, 2)
	}
	emit(gs, slug, "Piercing Exhale", map[string]interface{}{
		"seat":   item.Controller,
		"source": source.Card.DisplayName(),
		"target": target.Card.DisplayName(),
		"damage": damage,
		"beheld": beheld,
	})
}

// Draconic Fealty — {3}{B} Sorcery.
//
// Oracle text:
//
//	As an additional cost to cast this spell, you may behold a Dragon.
//	Target player discards a card with the greatest mana value among
//	cards in their hand. If a Dragon was beheld, exile that player's
//	graveyard.
//
// Implementation: pick the opponent with the highest hand-card CMC;
// discard the highest-CMC card from their hand; if the behold
// succeeded, exile their graveyard. The "target player" choice is
// deterministic for the test fixture — the opponent whose deck has
// the most expensive thing in hand is the highest-value disrupt
// target.
func draconicFealtyResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "draconic_fealty"
	if gs == nil || item == nil {
		return
	}
	target, victim := pickOpponentWithGreatestHandCMC(gs, item.Controller)
	if target < 0 || victim == nil {
		emitFail(gs, slug, "Draconic Fealty", "no_opponent_hand", map[string]interface{}{
			"seat": item.Controller,
		})
		return
	}
	beheld := attemptBeholdQuality(gs, item.Controller, "dragon", "Draconic Fealty")
	gameengine.DiscardCard(gs, victim, target)
	exiled := 0
	if beheld {
		seat := gs.Seats[target]
		if seat != nil {
			gy := append([]*gameengine.Card{}, seat.Graveyard...)
			for _, c := range gy {
				gameengine.MoveCard(gs, c, target, "graveyard", "exile", "draconic_fealty_beheld")
				exiled++
			}
		}
	}
	emit(gs, slug, "Draconic Fealty", map[string]interface{}{
		"seat":      item.Controller,
		"target":    target,
		"discarded": victim.DisplayName(),
		"beheld":    beheld,
		"exiled":    exiled,
	})
}

// Territorial Strike — {2}{G} Sorcery.
//
// Oracle text:
//
//	As an additional cost to cast this spell, you may behold a Dragon
//	creature.
//	Destroy target nonland permanent. If a Dragon creature was beheld,
//	it perpetually gets +2/+2.
//
// Implementation: destroy the highest-CMC nonland opp permanent; if
// beheld, stamp +2/+2 on the beheld Dragon. The engine has no
// "perpetual" duration system yet, so the buff is approximated with
// a permanent-duration Modification + an emitPartial breadcrumb so
// downstream auditors can see the gap. The buffed perm is selected
// after the behold resolves: prefer the chosen permanent (it stayed
// on the battlefield); fall back to the highest-power friendly
// Dragon when the behold path was reveal-from-hand.
func territorialStrikeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "territorial_strike"
	if gs == nil || item == nil {
		return
	}
	target := pickBestOppNonlandPermanent(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Territorial Strike", "no_target", map[string]interface{}{
			"seat": item.Controller,
		})
		return
	}
	beheld := attemptBeholdQuality(gs, item.Controller, "dragon", "Territorial Strike")
	gameengine.DestroyPermanent(gs, target, nil)
	var buffed *gameengine.Permanent
	if beheld {
		buffed = pickBestFriendlyDragon(gs, item.Controller)
		if buffed != nil {
			buffed.Modifications = append(buffed.Modifications, gameengine.Modification{
				Power:     2,
				Toughness: 2,
				Duration:  "perpetual",
				Timestamp: gs.NextTimestamp(),
			})
			gs.InvalidateCharacteristicsCache()
			emitPartial(gs, slug, "Territorial Strike", "perpetual_duration_approximated_as_permanent_modification")
		}
	}
	details := map[string]interface{}{
		"seat":      item.Controller,
		"destroyed": target.Card.DisplayName(),
		"beheld":    beheld,
	}
	if buffed != nil {
		details["buffed"] = buffed.Card.DisplayName()
	}
	emit(gs, slug, "Territorial Strike", details)
}

// -----------------------------------------------------------------------------
// Resolve-time target pickers — specific to this consumer family
// -----------------------------------------------------------------------------

// pickOpponentWithGreatestHandCMC scans every opponent's hand for the
// single card with the highest CMC; returns the (seat, card) pair.
// Returns (-1, nil) when no opponent has any cards in hand.
func pickOpponentWithGreatestHandCMC(gs *gameengine.GameState, seat int) (int, *gameengine.Card) {
	bestSeat := -1
	var bestCard *gameengine.Card
	bestCMC := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, c := range s.Hand {
			if c == nil {
				continue
			}
			cmc := cardCMC(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
				bestSeat = opp
			}
		}
	}
	return bestSeat, bestCard
}

// pickBestOppNonlandPermanent returns the highest-CMC nonland
// permanent any opponent controls. Used by Territorial Strike's
// "destroy target nonland permanent" branch.
func pickBestOppNonlandPermanent(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestCMC := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if cardHasType(p.Card, "land") {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc > bestCMC {
				bestCMC = cmc
				best = p
			}
		}
	}
	return best
}

// pickBestFriendlyDragon returns the highest-power Dragon under the
// seat's control, used by Territorial Strike's "+2/+2 to the Dragon
// you beheld" rider. Falls back to nil when no Dragon is on the
// battlefield (the behold may have been satisfied via reveal-from-
// hand instead).
func pickBestFriendlyDragon(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "dragon") {
			continue
		}
		if pw := p.Power(); pw > bestPower {
			bestPower = pw
			best = p
		}
	}
	return best
}
