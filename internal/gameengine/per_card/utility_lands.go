package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Bojuka Bog
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   Bojuka Bog enters the battlefield tapped.
//   When Bojuka Bog enters the battlefield, exile all cards from target
//   player's graveyard.
//   {T}: Add {B}.
//
// Utility land — free graveyard hate stapled to a land.

func registerBojukaBog(r *Registry) {
	r.OnETB("Bojuka Bog", bojukaBogETB)
}

func bojukaBogETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "bojuka_bog_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Enters tapped.
	perm.Tapped = true

	// Target an opponent's graveyard (MVP: pick the opponent with the
	// largest graveyard).
	opps := gs.Opponents(seat)
	if len(opps) == 0 {
		emit(gs, slug, "Bojuka Bog", map[string]interface{}{
			"seat":   seat,
			"exiled": 0,
			"reason": "no_opponents",
		})
		return
	}
	bestOpp := opps[0]
	bestSize := len(gs.Seats[opps[0]].Graveyard)
	for _, opp := range opps[1:] {
		sz := len(gs.Seats[opp].Graveyard)
		if sz > bestSize {
			bestOpp = opp
			bestSize = sz
		}
	}

	// Exile all cards from that player's graveyard.
	target := gs.Seats[bestOpp]
	exiled := len(target.Graveyard)
	gyCards := append([]*gameengine.Card(nil), target.Graveyard...)
	target.Graveyard = nil
	for _, c := range gyCards {
		gameengine.MoveCard(gs, c, bestOpp, "graveyard", "exile", "exile-from-graveyard")
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "exile_graveyard",
		Seat:   seat,
		Target: bestOpp,
		Source: "Bojuka Bog",
		Amount: exiled,
		Details: map[string]interface{}{
			"reason": "bojuka_bog_etb",
		},
	})
	emit(gs, slug, "Bojuka Bog", map[string]interface{}{
		"seat":       seat,
		"target":     bestOpp,
		"exiled":     exiled,
	})
}

// -----------------------------------------------------------------------------
// Otawara, Soaring City
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   {T}: Add {U}.
//   Channel — {3}{U}, Discard Otawara, Soaring City: Return target
//   nonland permanent to its owner's hand.
//
// Legendary land. The channel ability is an activated ability from hand
// that bypasses the stack restriction (it's not a spell). We model it
// as OnActivated since the engine routes channel through the same path.

func registerOtawara(r *Registry) {
	r.OnActivated("Otawara, Soaring City", otawaraActivated)
}

func otawaraActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "otawara_channel_bounce"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Channel is ability index 1 (0 is tap-for-mana). If we get abilityIdx==0,
	// it's the tap-for-mana (handled by generic AST). Only handle channel.
	if abilityIdx == 0 {
		return // let generic handle tap-for-mana
	}

	// Find the best target nonland permanent to bounce (opponent's side).
	var bestTarget *gameengine.Permanent
	for _, opp := range gs.Opponents(seat) {
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || p.IsLand() {
				continue
			}
			if bestTarget == nil {
				bestTarget = p
			}
		}
	}
	if bestTarget == nil {
		emitFail(gs, slug, "Otawara, Soaring City", "no_valid_target", nil)
		return
	}

	gameengine.BouncePermanent(gs, bestTarget, src, "hand")
	emit(gs, slug, "Otawara, Soaring City", map[string]interface{}{
		"seat":     seat,
		"bounced":  bestTarget.Card.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Boseiju, Who Endures
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   {T}: Add {G}.
//   Channel — {1}{G}, Discard Boseiju, Who Endures: Destroy target
//   artifact, enchantment, or nonbasic land an opponent controls. That
//   player may search their library for a land card with a basic land
//   type, put it onto the battlefield, then shuffle.
//
// Legendary land. Uncounterable removal for problematic permanents.

func registerBoseiju(r *Registry) {
	r.OnActivated("Boseiju, Who Endures", boseijuActivated)
}

func boseijuActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "boseiju_channel_destroy"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	if abilityIdx == 0 {
		return // tap-for-mana handled by generic
	}

	// Find target: artifact, enchantment, or nonbasic land an opponent controls.
	var bestTarget *gameengine.Permanent
	var targetSeat int
	for _, opp := range gs.Opponents(seat) {
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil {
				continue
			}
			eligible := p.IsArtifact() || p.IsEnchantment()
			if p.IsLand() && p.Card != nil && !isBasicLand(p.Card) {
				eligible = true
			}
			if eligible && bestTarget == nil {
				bestTarget = p
				targetSeat = opp
			}
		}
	}
	if bestTarget == nil {
		emitFail(gs, slug, "Boseiju, Who Endures", "no_valid_target", nil)
		return
	}

	// Destroy the target.
	gameengine.DestroyPermanent(gs, bestTarget, src)

	// The opponent may search for a basic land. MVP: give them one
	// if their library has a basic land.
	ts := gs.Seats[targetSeat]
	for i, c := range ts.Library {
		if c == nil {
			continue
		}
		if landMatchesFetchTypes(c, []string{"plains", "island", "swamp", "mountain", "forest"}) {
			land := ts.Library[i]
			ts.Library = append(ts.Library[:i], ts.Library[i+1:]...)
			enterBattlefieldWithETB(gs, targetSeat, land, false)
			shuffleLibraryPerCard(gs, targetSeat)
			break
		}
	}

	emit(gs, slug, "Boseiju, Who Endures", map[string]interface{}{
		"seat":      seat,
		"destroyed": bestTarget.Card.DisplayName(),
		"target":    targetSeat,
	})
}

// -----------------------------------------------------------------------------
// Rogue's Passage
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   {T}: Add {C}.
//   {4}, {T}: Target creature can't be blocked this turn.
//
// Utility land for pushing through combat damage.

func registerRoguesPassage(r *Registry) {
	r.OnActivated("Rogue's Passage", roguesPassageActivated)
}

func roguesPassageActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "rogues_passage_unblockable"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx == 0 {
		return // tap-for-mana handled by generic
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	if src.Tapped {
		emitFail(gs, slug, "Rogue's Passage", "already_tapped", nil)
		return
	}

	// Pay {4} from mana pool.
	s := gs.Seats[seat]
	if s.ManaPool < 4 {
		emitFail(gs, slug, "Rogue's Passage", "insufficient_mana", nil)
		return
	}
	s.ManaPool -= 4
	gameengine.SyncManaAfterSpend(s)
	src.Tapped = true

	// Target the best creature we control (highest power).
	var bestCreature *gameengine.Permanent
	bestPow := -1
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		pow := p.Power()
		if pow > bestPow {
			bestPow = pow
			bestCreature = p
		}
	}
	if bestCreature == nil {
		emitFail(gs, slug, "Rogue's Passage", "no_creatures", nil)
		return
	}

	// Grant "can't be blocked" until end of turn.
	if bestCreature.Flags == nil {
		bestCreature.Flags = map[string]int{}
	}
	bestCreature.Flags["unblockable"] = 1

	emit(gs, slug, "Rogue's Passage", map[string]interface{}{
		"seat":     seat,
		"target":   bestCreature.Card.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Reliquary Tower
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   You have no maximum hand size.
//   {T}: Add {C}.
//
// Utility land. The "no maximum hand size" is a static ability modeled
// via a game flag. The tap-for-mana is handled by generic.

func registerReliquaryTower(r *Registry) {
	r.OnETB("Reliquary Tower", reliquaryTowerETB)
}

func reliquaryTowerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "reliquary_tower_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["no_max_hand_size_seat_"+intToStr(seat)] = 1

	emit(gs, slug, "Reliquary Tower", map[string]interface{}{
		"seat":          seat,
		"no_max_hand":   true,
	})
}

// -----------------------------------------------------------------------------
// Ancient Tomb
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   {T}: Add {C}{C}. Ancient Tomb deals 2 damage to you.
//
// Fast mana land. Produces 2 colorless at the cost of 2 life per use.

func registerAncientTomb(r *Registry) {
	r.OnActivated("Ancient Tomb", ancientTombActivated)
}

func ancientTombActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ancient_tomb_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Ancient Tomb", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	src.Tapped = true

	// Add {C}{C}.
	gameengine.AddMana(gs, gs.Seats[seat], "C", 2, "Ancient Tomb")

	// Deal 2 damage to controller.
	gs.Seats[seat].Life -= 2
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   seat,
		Target: seat,
		Source: "Ancient Tomb",
		Amount: 2,
		Details: map[string]interface{}{
			"reason": "ancient_tomb_self_damage",
		},
	})

	emit(gs, slug, "Ancient Tomb", map[string]interface{}{
		"seat":    seat,
		"mana":    2,
		"damage":  2,
		"life":    gs.Seats[seat].Life,
	})
	_ = gs.CheckEnd()
}

// -----------------------------------------------------------------------------
// Urza's Saga
// -----------------------------------------------------------------------------
//
// Oracle text (saga):
//
//   (As this Saga enters and after your draw step, add a lore counter.)
//   I — Urza's Saga gains "{T}: Add {C}."
//   II — Urza's Saga gains "{2}, {T}: Create a 0/0 colorless Construct
//        artifact creature token with 'This creature gets +1/+1 for each
//        artifact you control.'"
//   III — Search your library for an artifact card with mana cost {0}
//         or {1}, put it onto the battlefield, then shuffle.
//
// Enchantment Land — Urza's Saga. The III chapter searches for cheap
// artifacts (Sol Ring, Mana Crypt, Mana Vault, etc.).

func registerUrzasSaga(r *Registry) {
	r.OnETB("Urza's Saga", urzasSagaETB)
	r.OnTrigger("Urza's Saga", "lore_counter_added", urzasSagaLore)
}

func urzasSagaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "urzas_saga_etb"
	if gs == nil || perm == nil {
		return
	}
	// Chapter I: gains tap for {C}. This is implicit from being a land.
	perm.AddCounter("lore", 1)
	emit(gs, slug, "Urza's Saga", map[string]interface{}{
		"seat":    perm.Controller,
		"chapter": 1,
	})
}

func urzasSagaLore(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "urzas_saga_chapter"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	lore := 0
	if perm.Counters != nil {
		lore = perm.Counters["lore"]
	}

	switch lore {
	case 2:
		// Chapter II: create a Construct token. MVP: create a 0/0
		// artifact creature that gets +1/+1 per artifact (we approximate
		// by setting base P/T to artifact count).
		artCount := 0
		for _, p := range gs.Seats[seat].Battlefield {
			if p != nil && p.IsArtifact() {
				artCount++
			}
		}
		token := &gameengine.Card{
			Name:          "Construct Token",
			Owner:         seat,
			Types:         []string{"token", "artifact", "creature", "construct"},
			BasePower:     artCount,
			BaseToughness: artCount,
		}
		enterBattlefieldWithETB(gs, seat, token, false)
		emit(gs, slug, "Urza's Saga", map[string]interface{}{
			"seat":    seat,
			"chapter": 2,
			"size":    artCount,
		})

	case 3:
		// Chapter III: search for 0-or-1 CMC artifact, put onto battlefield.
		s := gs.Seats[seat]
		foundIdx := -1
		for i, c := range s.Library {
			if c == nil {
				continue
			}
			isArt := false
			for _, t := range c.Types {
				if t == "artifact" {
					isArt = true
					break
				}
			}
			if isArt && c.CMC <= 1 {
				foundIdx = i
				break
			}
		}
		if foundIdx >= 0 {
			found := s.Library[foundIdx]
			s.Library = append(s.Library[:foundIdx], s.Library[foundIdx+1:]...)
			enterBattlefieldWithETB(gs, seat, found, false)
			gs.LogEvent(gameengine.Event{
				Kind:   "search_library",
				Seat:   seat,
				Source: "Urza's Saga",
				Details: map[string]interface{}{
					"found_card":  found.DisplayName(),
					"destination": "battlefield",
				},
			})
		}
		shuffleLibraryPerCard(gs, seat)

		// Saga is sacrificed after chapter III.
		gameengine.SacrificePermanent(gs, perm, "saga_chapter_iii_complete")

		emit(gs, slug, "Urza's Saga", map[string]interface{}{
			"seat":      seat,
			"chapter":   3,
			"found":     foundIdx >= 0,
			"sacrificed": true,
		})
	}
}
