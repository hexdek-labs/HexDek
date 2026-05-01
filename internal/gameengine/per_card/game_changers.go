package per_card

import (
	"strings"
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// panopticImprinted maps a Panoptic Mirror permanent to the card it imprinted.
var panopticImprinted sync.Map

// ============================================================================
// Game Changers — the 25 highest-impact WotC Game Changer cards in
// Commander. Without handlers, cEDH decks can't resolve their key
// cards and perform worse than precons.
//
// Cards in this file:
//
// WHITE:
//   - Humility (static — creatures lose abilities, become 1/1)
//   - Teferi's Protection (instant — protection from everything + phase out)
//
// BLUE:
//   - Consecrated Sphinx (trigger — draw 2 when opponent draws)
//   - Gifts Ungiven (instant — tutor 4, opponent picks 2 for GY)
//   - Intuition (instant — tutor 3, opponent picks 1 for hand)
//   - Narset, Parter of Veils (static — opponents draw max 1/turn)
//
// BLACK:
//   - Braids, Cabal Minion (upkeep trigger — each player sacs)
//   - Imperial Seal (sorcery — tutor to top, lose 2 life)
//   - Orcish Bowmasters (ETB + trigger — orc army + ping on opponent draw)
//
// RED:
//   - Gamble (sorcery — tutor to hand, discard random)
//   - Jeska's Will (sorcery — mana + exile cards if commander in play)
//
// GREEN:
//   - Biorhythm (sorcery — each player's life = creatures they control)
//   - Crop Rotation (instant — sac land, tutor land to battlefield)
//   - Natural Order (sorcery — sac green creature, tutor green creature to BF)
//   - Seedborn Muse (static — untap all during other players' untap steps)
//
// MULTI:
//   - Aura Shards (trigger — creature ETB destroys artifact/enchantment)
//   - Coalition Victory (sorcery — win if 5 colors + 5 land types)
//   - Grand Arbiter Augustin IV (static — cost reduction/increase)
//
// COLORLESS:
//   - Field of the Dead (trigger — land ETB with 7+ unique lands → zombie)
//   - Gaea's Cradle (tap for G per creature)
//   - Glacial Chasm (cumulative upkeep, prevent all damage, can't attack)
//   - Mishra's Workshop (tap for 3 colorless, artifacts only)
//   - Panoptic Mirror (imprint + upkeep cast for free)
//   - Serra's Sanctum (tap for W per enchantment)
//   - The Tabernacle at Pendrell Vale (upkeep destroy unless pay 1)
// ============================================================================

// ---------------------------------------------------------------------------
// Humility — {2}{W}{W} Enchantment
//
// Oracle text:
//   All creatures lose all abilities and have base power and toughness
//   1/1.
//
// Layer 6 (ability removing) + Layer 7b (P/T setting). This is one of
// the most complex cards in Magic due to dependency/timestamp
// interactions with other continuous effects. Implementation:
//   - ETB: register continuous effect at layer 6 (remove abilities) +
//     layer 7b (set P/T to 1/1).
//   - We use the Flags approach: stamp creatures so the engine knows
//     they are affected.
// ---------------------------------------------------------------------------

func registerHumility(r *Registry) {
	r.OnETB("Humility", humilityETB)
}

func humilityETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "humility_static"
	if gs == nil || perm == nil {
		return
	}
	// Register a layer 7b continuous effect: all creatures become 1/1.
	ce := &gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "b", // 7b: set P/T
		Timestamp:      perm.Timestamp,
		SourcePerm:     perm,
		SourceCardName: "Humility",
		ControllerSeat: perm.Controller,
		HandlerID:      "humility_pt_" + intToStr(perm.Timestamp),
		Duration:       gameengine.DurationUntilSourceLeaves,
		Predicate: func(gs *gameengine.GameState, target *gameengine.Permanent) bool {
			return target != nil && target.IsCreature()
		},
		ApplyFn: func(gs *gameengine.GameState, target *gameengine.Permanent, chars *gameengine.Characteristics) {
			if chars == nil {
				return
			}
			chars.Power = 1
			chars.Toughness = 1
		},
	}
	gs.ContinuousEffects = append(gs.ContinuousEffects, ce)

	// Register a layer 6 continuous effect: all creatures lose abilities.
	ce2 := &gameengine.ContinuousEffect{
		Layer:          gameengine.LayerAbility,
		Sublayer:       "",
		Timestamp:      perm.Timestamp,
		SourcePerm:     perm,
		SourceCardName: "Humility",
		ControllerSeat: perm.Controller,
		HandlerID:      "humility_ability_" + intToStr(perm.Timestamp),
		Duration:       gameengine.DurationUntilSourceLeaves,
		Predicate: func(gs *gameengine.GameState, target *gameengine.Permanent) bool {
			return target != nil && target.IsCreature()
		},
		ApplyFn: func(gs *gameengine.GameState, target *gameengine.Permanent, chars *gameengine.Characteristics) {
			if chars == nil {
				return
			}
			// Remove all abilities (§613.1f layer 6).
			chars.Abilities = nil
			chars.Keywords = nil
		},
	}
	gs.ContinuousEffects = append(gs.ContinuousEffects, ce2)

	// Also set a global flag so dispatch checks can short-circuit.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["humility_count"]++

	emit(gs, slug, "Humility", map[string]interface{}{
		"seat":      perm.Controller,
		"timestamp": perm.Timestamp,
		"effect":    "all_creatures_lose_abilities_and_are_1_1",
	})
}

// ---------------------------------------------------------------------------
// Teferi's Protection — {2}{W} Instant
//
// Oracle text:
//   Until your next turn, your life total can't change and you gain
//   protection from everything. All permanents you control phase out.
//   (While they're phased out, they're treated as though they don't
//   exist. They phase in before you untap during your untap step.)
//
// Implementation:
//   - Set seat flags for protection from everything + life can't change.
//   - Phase out all permanents the controller controls.
//   - Register delayed trigger to remove flags at controller's next turn.
// ---------------------------------------------------------------------------

func registerTeferisProtection(r *Registry) {
	r.OnResolve("Teferi's Protection", teferisProtectionResolve)
}

func teferisProtectionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "teferis_protection"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}

	// Protection from everything — prevents all damage.
	s.Flags["protection_from_everything"] = 1
	// Life total can't change.
	s.Flags["life_cant_change"] = 1

	// Phase out all permanents controller controls.
	phasedCount := 0
	for _, p := range s.Battlefield {
		if p == nil || p.PhasedOut {
			continue
		}
		p.PhasedOut = true
		phasedCount++
	}

	// Register delayed trigger: "until your next turn" — clear flags
	// at the start of controller's next turn.
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_turn",
		ControllerSeat: seat,
		SourceCardName: "Teferi's Protection",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if seat < 0 || seat >= len(gs.Seats) {
				return
			}
			s := gs.Seats[seat]
			if s == nil || s.Flags == nil {
				return
			}
			delete(s.Flags, "protection_from_everything")
			delete(s.Flags, "life_cant_change")
			gs.LogEvent(gameengine.Event{
				Kind:   "per_card_handler",
				Seat:   seat,
				Source: "Teferi's Protection",
				Details: map[string]interface{}{
					"slug":   "teferis_protection_end",
					"effect": "protection_expired",
				},
			})
		},
	})

	emit(gs, slug, "Teferi's Protection", map[string]interface{}{
		"seat":         seat,
		"phased_out":   phasedCount,
		"protection":   true,
		"life_locked":  true,
	})
}

// ---------------------------------------------------------------------------
// Consecrated Sphinx — {4}{U}{U} Creature (4/6 Flying)
//
// Oracle text:
//   Flying. Whenever an opponent draws a card, you may draw two cards.
//
// Trigger on opponent draw events.
// ---------------------------------------------------------------------------

func registerConsecratedSphinx(r *Registry) {
	r.OnTrigger("Consecrated Sphinx", "card_drawn", consecratedSphinxTrigger)
	r.OnTrigger("Consecrated Sphinx", "opponent_draws", consecratedSphinxTrigger)
}

func consecratedSphinxTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "consecrated_sphinx_draw"
	if gs == nil || perm == nil {
		return
	}
	drawerSeat, ok := ctx["drawer_seat"].(int)
	if !ok {
		return
	}
	// Only triggers when an OPPONENT draws.
	if drawerSeat == perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	// Draw 2 cards.
	drawn := 0
	for i := 0; i < 2 && len(s.Library) > 0; i++ {
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
		drawn++
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "draw",
		Seat:   seat,
		Source: "Consecrated Sphinx",
		Amount: drawn,
		Details: map[string]interface{}{
			"reason":      "consecrated_sphinx_trigger",
			"drawer_seat": drawerSeat,
		},
	})
	emit(gs, slug, "Consecrated Sphinx", map[string]interface{}{
		"seat":        seat,
		"drawn":       drawn,
		"trigger_by":  drawerSeat,
	})
}

// ---------------------------------------------------------------------------
// Gifts Ungiven — {3}{U} Instant
//
// Oracle text:
//   Search your library for up to four cards with different names and
//   reveal them. Target opponent chooses two of those cards. Put the
//   chosen cards into your graveyard and the rest into your hand. Then
//   shuffle your library.
//
// In practice, cEDH players search for exactly 2 cards (so the opponent
// MUST put both in graveyard — there's no choice). We model the general
// case: find 4, opponent picks 2 for GY, 2 go to hand.
// ---------------------------------------------------------------------------

func registerGiftsUngiven(r *Registry) {
	r.OnResolve("Gifts Ungiven", giftsUngivenResolve)
}

func giftsUngivenResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "gifts_ungiven"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		emitFail(gs, slug, "Gifts Ungiven", "library_empty", nil)
		return
	}

	// Find up to 4 different cards from library.
	var found []*gameengine.Card
	seen := map[string]bool{}
	for _, c := range s.Library {
		if c == nil {
			continue
		}
		name := c.DisplayName()
		if seen[name] {
			continue
		}
		seen[name] = true
		found = append(found, c)
		if len(found) >= 4 {
			break
		}
	}

	if len(found) == 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, "Gifts Ungiven", "no_cards_found", nil)
		return
	}

	// AI policy for opponent's choice: first 2 go to graveyard, rest to hand.
	// (In real play the opponent would choose the worst 2 for the caster.)
	toGY := 2
	if len(found) <= 2 {
		// If only 2 or fewer found, all go to graveyard (the cEDH line).
		toGY = len(found)
	}

	for i, c := range found {
		if i < toGY {
			gameengine.MoveCard(gs, c, seat, "library", "graveyard", "gifts_ungiven_opponent_choice")
		} else {
			gameengine.MoveCard(gs, c, seat, "library", "hand", "gifts_ungiven_to_hand")
		}
	}
	shuffleLibraryPerCard(gs, seat)

	names := make([]string, len(found))
	for i, c := range found {
		names[i] = c.DisplayName()
	}
	emit(gs, slug, "Gifts Ungiven", map[string]interface{}{
		"seat":  seat,
		"found": names,
		"to_gy": toGY,
	})
}

// ---------------------------------------------------------------------------
// Intuition — {2}{U} Instant
//
// Oracle text:
//   Search your library for three cards and reveal them. Target opponent
//   chooses one. Put that card into your hand and the rest into your
//   graveyard. Then shuffle your library.
//
// Similar to Gifts but 3 cards, opponent picks 1 for hand.
// ---------------------------------------------------------------------------

func registerIntuition(r *Registry) {
	r.OnResolve("Intuition", intuitionResolve)
}

func intuitionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "intuition"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		emitFail(gs, slug, "Intuition", "library_empty", nil)
		return
	}

	// Find up to 3 cards.
	var found []*gameengine.Card
	for _, c := range s.Library {
		if c == nil {
			continue
		}
		found = append(found, c)
		if len(found) >= 3 {
			break
		}
	}

	if len(found) == 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, "Intuition", "no_cards_found", nil)
		return
	}

	// AI policy: opponent chooses the WORST card to give to hand.
	// MVP: first card goes to hand, rest to GY.
	gameengine.MoveCard(gs, found[0], seat, "library", "hand", "intuition_to_hand")
	for i := 1; i < len(found); i++ {
		gameengine.MoveCard(gs, found[i], seat, "library", "graveyard", "intuition_to_gy")
	}
	shuffleLibraryPerCard(gs, seat)

	names := make([]string, len(found))
	for i, c := range found {
		names[i] = c.DisplayName()
	}
	emit(gs, slug, "Intuition", map[string]interface{}{
		"seat":    seat,
		"found":   names,
		"to_hand": found[0].DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// Narset, Parter of Veils — {1}{U}{U} Planeswalker (Narset)
//
// Oracle text:
//   Each opponent can't draw more than one card each turn.
//
// Static ability — we use a flag approach. The draw engine consults
// this flag to suppress excess draws.
// ---------------------------------------------------------------------------

func registerNarsetParterOfVeils(r *Registry) {
	r.OnETB("Narset, Parter of Veils", narsetParterETB)
}

func narsetParterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "narset_parter_static"
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["narset_parter_count"]++
	gs.Flags["narset_parter_seat_"+intToStr(perm.Controller)] = perm.Timestamp

	emit(gs, slug, "Narset, Parter of Veils", map[string]interface{}{
		"seat":      perm.Controller,
		"timestamp": perm.Timestamp,
		"effect":    "opponents_draw_max_1_per_turn",
	})
}

// ---------------------------------------------------------------------------
// Braids, Cabal Minion — {2}{B}{B} Creature (2/2)
//
// Oracle text:
//   At the beginning of each player's upkeep, that player sacrifices an
//   artifact, creature, or land.
//
// Upkeep trigger for ALL players.
// ---------------------------------------------------------------------------

func registerBraidsCabalMinion(r *Registry) {
	r.OnTrigger("Braids, Cabal Minion", "upkeep_start", braidsCabalMinionUpkeep)
}

func braidsCabalMinionUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "braids_cabal_minion_sac"
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[activeSeat]
	if s == nil || s.Lost {
		return
	}

	// Find an artifact, creature, or land to sacrifice. Prefer the least
	// valuable (lowest CMC non-land, or a tapped land).
	var victim *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil {
			continue
		}
		if p.IsCreature() || p.IsArtifact() || p.IsLand() {
			if victim == nil {
				victim = p
			} else {
				// Prefer non-creature, tapped, or cheaper permanent.
				if p.IsLand() && p.Tapped && !victim.IsLand() {
					victim = p
				} else if !p.IsCreature() && victim.IsCreature() {
					victim = p
				}
			}
		}
	}

	if victim == nil {
		emitFail(gs, slug, "Braids, Cabal Minion", "nothing_to_sacrifice", map[string]interface{}{
			"active_seat": activeSeat,
		})
		return
	}

	gameengine.SacrificePermanent(gs, victim, "braids_cabal_minion")
	emit(gs, slug, "Braids, Cabal Minion", map[string]interface{}{
		"active_seat": activeSeat,
		"sacrificed":  victim.Card.DisplayName(),
		"controller":  perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// Imperial Seal — {B} Sorcery
//
// Oracle text:
//   Search your library for a card, then shuffle your library and put
//   that card on top of it. You lose 2 life.
//
// Identical to Vampiric Tutor but sorcery speed.
// ---------------------------------------------------------------------------

func registerImperialSeal(r *Registry) {
	r.OnResolve("Imperial Seal", imperialSealResolve)
}

func imperialSealResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "imperial_seal"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Pay 2 life.
	gs.Seats[seat].Life -= 2
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: "Imperial Seal",
		Amount: 2,
		Details: map[string]interface{}{
			"reason": "imperial_seal_cost",
		},
	})
	found := tutorToTop(gs, seat, nil, "Imperial Seal")
	emit(gs, slug, "Imperial Seal", map[string]interface{}{
		"seat":  seat,
		"found": found,
	})
	_ = gs.CheckEnd()
}

// ---------------------------------------------------------------------------
// Orcish Bowmasters — {1}{B} Creature (1/1)
//
// Oracle text:
//   Flash. When Orcish Bowmasters enters the battlefield, and whenever
//   an opponent draws a card except the first one they draw in each of
//   their draw steps, Orcish Bowmasters deals 1 damage to any target
//   and amass Orcs 1.
//
// ETB: deal 1 damage + create 1/1 Orc Army (or add counter to existing).
// Trigger: same on opponent extra draws.
// ---------------------------------------------------------------------------

func registerOrcishBowmasters(r *Registry) {
	r.OnETB("Orcish Bowmasters", orcishBowmastersETB)
	r.OnTrigger("Orcish Bowmasters", "card_drawn", orcishBowmastersTrigger)
	r.OnTrigger("Orcish Bowmasters", "opponent_draws", orcishBowmastersTrigger)
}

func orcishBowmastersETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "orcish_bowmasters_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller

	// Deal 1 damage to an opponent (pick the one with highest life).
	bestOpp := -1
	bestLife := -1
	for _, opp := range gs.Opponents(seat) {
		if gs.Seats[opp].Life > bestLife {
			bestLife = gs.Seats[opp].Life
			bestOpp = opp
		}
	}
	if bestOpp >= 0 {
		gs.Seats[bestOpp].Life -= 1
		gs.LogEvent(gameengine.Event{
			Kind:   "damage",
			Seat:   seat,
			Target: bestOpp,
			Source: "Orcish Bowmasters",
			Amount: 1,
			Details: map[string]interface{}{
				"reason": "orcish_bowmasters_etb",
			},
		})
	}

	// Amass Orcs 1: create a 1/1 Orc Army or add +1/+1 counter to existing.
	amassOrcs(gs, seat)

	emit(gs, slug, "Orcish Bowmasters", map[string]interface{}{
		"seat":      seat,
		"damage_to": bestOpp,
	})
	_ = gs.CheckEnd()
}

func orcishBowmastersTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "orcish_bowmasters_trigger"
	if gs == nil || perm == nil {
		return
	}
	drawerSeat, ok := ctx["drawer_seat"].(int)
	if !ok {
		return
	}
	// Only triggers when an OPPONENT draws.
	if drawerSeat == perm.Controller {
		return
	}
	seat := perm.Controller

	// Deal 1 damage to the drawing opponent.
	gs.Seats[drawerSeat].Life -= 1
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   seat,
		Target: drawerSeat,
		Source: "Orcish Bowmasters",
		Amount: 1,
		Details: map[string]interface{}{
			"reason":      "orcish_bowmasters_draw_trigger",
			"drawer_seat": drawerSeat,
		},
	})

	// Amass Orcs 1.
	amassOrcs(gs, seat)

	emit(gs, slug, "Orcish Bowmasters", map[string]interface{}{
		"seat":        seat,
		"damage_to":   drawerSeat,
		"trigger":     "opponent_draw",
	})
	_ = gs.CheckEnd()
}

// amassOrcs implements "amass Orcs 1": if you control an Army creature,
// put a +1/+1 counter on it; otherwise create a 0/0 Orc Army token and
// put a +1/+1 counter on it (making it 1/1).
func amassOrcs(gs *gameengine.GameState, seat int) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	// Look for an existing Army creature.
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.ToLower(t) == "army" {
				p.AddCounter("+1/+1", 1)
				return
			}
		}
	}
	// No army — create a 0/0 Orc Army token + 1 counter.
	token := gameengine.CreateCreatureToken(gs, seat, "Orc Army Token",
		[]string{"creature", "orc", "army"}, 0, 0)
	if token != nil {
		token.AddCounter("+1/+1", 1)
	}
}

// ---------------------------------------------------------------------------
// Gamble — {R} Sorcery
//
// Oracle text:
//   Search your library for a card, put that card into your hand, shuffle
//   your library, then discard a card at random.
// ---------------------------------------------------------------------------

func registerGamble(r *Registry) {
	r.OnResolve("Gamble", gambleResolve)
}

func gambleResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "gamble"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Tutor any card to hand.
	found := tutorToHand(gs, seat, nil, "Gamble")

	// Discard a card at random.
	s := gs.Seats[seat]
	if len(s.Hand) > 0 {
		idx := 0
		if gs.Rng != nil && len(s.Hand) > 1 {
			idx = gs.Rng.Intn(len(s.Hand))
		}
		discarded := s.Hand[idx]
		gameengine.DiscardCard(gs, discarded, seat)
		emit(gs, slug, "Gamble", map[string]interface{}{
			"seat":      seat,
			"found":     found,
			"discarded": discarded.DisplayName(),
		})
	} else {
		emit(gs, slug, "Gamble", map[string]interface{}{
			"seat":  seat,
			"found": found,
		})
	}
}

// ---------------------------------------------------------------------------
// Jeska's Will — {2}{R} Sorcery
//
// Oracle text:
//   Choose one. If you control a commander as you cast this spell, you
//   may choose both.
//   - Add R for each card in target opponent's hand.
//   - Exile the top three cards of your library. You may play them this
//     turn.
// ---------------------------------------------------------------------------

func registerJeskasWill(r *Registry) {
	r.OnResolve("Jeska's Will", jeskasWillResolve)
}

func jeskasWillResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "jeskas_will"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Check if controller has a commander on the battlefield.
	hasCommander := false
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, cn := range s.CommanderNames {
			if strings.EqualFold(p.Card.DisplayName(), cn) {
				hasCommander = true
				break
			}
		}
		if hasCommander {
			break
		}
	}

	manaAdded := 0
	exiled := 0

	// Mode 1: Add R for each card in target opponent's hand.
	// Find opponent with most cards in hand.
	bestOpp := -1
	bestHandSize := -1
	for _, opp := range gs.Opponents(seat) {
		handSize := len(gs.Seats[opp].Hand)
		if handSize > bestHandSize {
			bestHandSize = handSize
			bestOpp = opp
		}
	}
	if bestOpp >= 0 && bestHandSize > 0 {
		manaAdded = bestHandSize
		s.ManaPool += manaAdded
		gameengine.SyncManaAfterAdd(s, manaAdded)
		gs.LogEvent(gameengine.Event{
			Kind:   "add_mana",
			Seat:   seat,
			Target: seat,
			Source: "Jeska's Will",
			Amount: manaAdded,
			Details: map[string]interface{}{
				"reason":      "jeskas_will_mode_1",
				"opponent":    bestOpp,
				"hand_size":   bestHandSize,
			},
		})
	}

	// Mode 2 (only if commander in play, or always in MVP since both modes
	// are generally chosen): exile top 3 of library, may play this turn.
	var exiledCards []*gameengine.Card
	if hasCommander || bestOpp < 0 {
		for i := 0; i < 3 && len(s.Library) > 0; i++ {
			card := s.Library[0]
			gameengine.MoveCard(gs, card, seat, "library", "exile", "jeskas_will_exile")
			exiledCards = append(exiledCards, card)
			exiled++
		}
	}

	// Register zone-cast permission so exiled cards can be played this turn.
	for _, ec := range exiledCards {
		gameengine.RegisterZoneCastGrant(gs, ec, &gameengine.ZoneCastPermission{
			Zone:              gameengine.ZoneExile,
			Keyword:           "jeskas_will_exile_play",
			ManaCost:          -1, // pay normal mana cost
			RequireController: seat,
			SourceName:        "Jeska's Will",
		})
	}

	// Clean up permissions at end of turn.
	if len(exiledCards) > 0 {
		cleanup := make([]*gameengine.Card, len(exiledCards))
		copy(cleanup, exiledCards)
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "end_of_turn",
			ControllerSeat: seat,
			SourceCardName: "Jeska's Will",
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				for _, ec := range cleanup {
					gameengine.RemoveZoneCastGrant(gs, ec)
				}
			},
		})
	}

	emit(gs, slug, "Jeska's Will", map[string]interface{}{
		"seat":               seat,
		"mana_added":         manaAdded,
		"exiled":             exiled,
		"has_commander":      hasCommander,
		"exile_play_granted": len(exiledCards) > 0,
	})
}

// ---------------------------------------------------------------------------
// Biorhythm — {6}{G}{G} Sorcery
//
// Oracle text:
//   Each player's life total becomes the number of creatures they
//   control.
// ---------------------------------------------------------------------------

func registerBiorhythm(r *Registry) {
	r.OnResolve("Biorhythm", biorhythmResolve)
}

func biorhythmResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "biorhythm"
	if gs == nil || item == nil {
		return
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		creatures := 0
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				creatures++
			}
		}
		s.Life = creatures
		gs.LogEvent(gameengine.Event{
			Kind:   "set_life",
			Seat:   i,
			Target: i,
			Source: "Biorhythm",
			Amount: creatures,
			Details: map[string]interface{}{
				"reason":    "biorhythm",
				"creatures": creatures,
			},
		})
	}
	emit(gs, slug, "Biorhythm", map[string]interface{}{
		"seat": item.Controller,
	})
	_ = gs.CheckEnd()
}

// ---------------------------------------------------------------------------
// Crop Rotation — {G} Instant
//
// Oracle text:
//   As an additional cost to cast this spell, sacrifice a land.
//   Search your library for a land card, put that card onto the
//   battlefield, then shuffle your library.
//
// The sacrifice happens at cast time (additional cost). At resolution,
// we search for a land and put it onto the battlefield.
// ---------------------------------------------------------------------------

func registerCropRotation(r *Registry) {
	r.OnResolve("Crop Rotation", cropRotationResolve)
}

func cropRotationResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "crop_rotation"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Additional cost: sacrifice a land. (In a full engine this happens
	// at cast time, but we model it here for correctness.)
	var sacced *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p != nil && p.IsLand() {
			sacced = p
			break
		}
	}
	if sacced != nil {
		gameengine.SacrificePermanent(gs, sacced, "crop_rotation_cost")
	}

	// Search library for a land card, put it onto the battlefield.
	foundIdx := -1
	for i, c := range s.Library {
		if c != nil && cardHasType(c, "land") {
			foundIdx = i
			break
		}
	}
	if foundIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, "Crop Rotation", "no_land_in_library", nil)
		return
	}

	card := s.Library[foundIdx]
	gameengine.MoveCard(gs, card, seat, "library", "battlefield", "crop_rotation")
	enterBattlefieldWithETB(gs, seat, card, false)
	shuffleLibraryPerCard(gs, seat)

	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: "Crop Rotation",
		Details: map[string]interface{}{
			"found_card":  card.DisplayName(),
			"destination": "battlefield",
			"reason":      "crop_rotation",
		},
	})
	emit(gs, slug, "Crop Rotation", map[string]interface{}{
		"seat":      seat,
		"found":     card.DisplayName(),
		"sacrificed": sacced != nil,
	})
}

// ---------------------------------------------------------------------------
// Natural Order — {2}{G}{G} Sorcery
//
// Oracle text:
//   As an additional cost to cast this spell, sacrifice a green creature.
//   Search your library for a green creature card, put it onto the
//   battlefield, then shuffle your library.
// ---------------------------------------------------------------------------

func registerNaturalOrder(r *Registry) {
	r.OnResolve("Natural Order", naturalOrderResolve)
}

func naturalOrderResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "natural_order"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Additional cost: sacrifice a green creature.
	var sacced *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Card != nil && cardHasColor(p.Card, "G") {
			sacced = p
			break
		}
	}
	if sacced != nil {
		gameengine.SacrificePermanent(gs, sacced, "natural_order_cost")
	}

	// Search library for a green creature card.
	foundIdx := -1
	bestCMC := -1
	for i, c := range s.Library {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		if !cardHasColor(c, "G") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			foundIdx = i
		}
	}
	if foundIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, "Natural Order", "no_green_creature_in_library", nil)
		return
	}

	card := s.Library[foundIdx]
	gameengine.MoveCard(gs, card, seat, "library", "battlefield", "natural_order")
	enterBattlefieldWithETB(gs, seat, card, false)
	shuffleLibraryPerCard(gs, seat)

	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: "Natural Order",
		Details: map[string]interface{}{
			"found_card":  card.DisplayName(),
			"destination": "battlefield",
			"reason":      "natural_order",
		},
	})
	emit(gs, slug, "Natural Order", map[string]interface{}{
		"seat":       seat,
		"found":      card.DisplayName(),
		"sacrificed": sacced != nil,
	})
}

// cardHasColor checks if a card has a specific color letter (W, U, B, R, G).
func cardHasColor(c *gameengine.Card, color string) bool {
	if c == nil {
		return false
	}
	want := strings.ToUpper(color)
	for _, col := range c.Colors {
		if strings.ToUpper(col) == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Seedborn Muse — {3}{G}{G} Creature (2/4)
//
// Oracle text:
//   Untap all permanents you control during each other player's untap
//   step.
//
// Trigger on other players' untap steps to untap all of controller's
// permanents.
// ---------------------------------------------------------------------------

func registerSeedbornMuse(r *Registry) {
	r.OnTrigger("Seedborn Muse", "untap_step", seedbornMuseTrigger)
}

func seedbornMuseTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "seedborn_muse_untap"
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	// Only trigger during OTHER players' untap steps.
	if activeSeat == perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	untapped := 0
	for _, p := range s.Battlefield {
		if p != nil && p.Tapped && !p.DoesNotUntap {
			p.Tapped = false
			untapped++
		}
	}
	emit(gs, slug, "Seedborn Muse", map[string]interface{}{
		"seat":        seat,
		"untapped":    untapped,
		"active_seat": activeSeat,
	})
}

// ---------------------------------------------------------------------------
// Aura Shards — {1}{G}{W} Enchantment
//
// Oracle text:
//   Whenever a creature enters the battlefield under your control, you
//   may destroy target artifact or enchantment.
//
// Trigger on creature ETB.
// ---------------------------------------------------------------------------

func registerAuraShards(r *Registry) {
	r.OnTrigger("Aura Shards", "creature_enters_battlefield", auraShardsETBTrigger)
	r.OnTrigger("Aura Shards", "permanent_entered_battlefield", auraShardsETBTrigger)
}

func auraShardsETBTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aura_shards_destroy"
	if gs == nil || perm == nil {
		return
	}
	// Check if the entering creature is controlled by the Aura Shards controller.
	enteringSeat, _ := ctx["entering_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	seat := perm.Controller

	// Find the best artifact or enchantment to destroy (opponent's, highest CMC).
	var bestTarget *gameengine.Permanent
	bestCMC := -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil {
			continue
		}
		for _, p := range os.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsArtifact() || p.IsEnchantment() {
				cmc := cardCMC(p.Card)
				if cmc > bestCMC {
					bestCMC = cmc
					bestTarget = p
				}
			}
		}
	}

	if bestTarget == nil {
		emitFail(gs, slug, "Aura Shards", "no_artifact_or_enchantment", nil)
		return
	}

	gameengine.SacrificePermanent(gs, bestTarget, "aura_shards_destroy")
	emit(gs, slug, "Aura Shards", map[string]interface{}{
		"seat":      seat,
		"destroyed": bestTarget.Card.DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// Coalition Victory — {3}{W}{U}{B}{R}{G} Sorcery
//
// Oracle text:
//   You win the game if you control a land of each basic land type and
//   a creature of each color.
// ---------------------------------------------------------------------------

func registerCoalitionVictory(r *Registry) {
	r.OnResolve("Coalition Victory", coalitionVictoryResolve)
}

func coalitionVictoryResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "coalition_victory"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Check for a land of each basic type: Plains, Island, Swamp, Mountain, Forest.
	basicTypes := map[string]bool{
		"plains": false, "island": false, "swamp": false,
		"mountain": false, "forest": false,
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsLand() {
			continue
		}
		for _, t := range p.Card.Types {
			tl := strings.ToLower(t)
			if _, ok := basicTypes[tl]; ok {
				basicTypes[tl] = true
			}
		}
	}

	// Check for a creature of each color: W, U, B, R, G.
	creatureColors := map[string]bool{
		"W": false, "U": false, "B": false, "R": false, "G": false,
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		for _, col := range p.Card.Colors {
			upper := strings.ToUpper(col)
			if _, ok := creatureColors[upper]; ok {
				creatureColors[upper] = true
			}
		}
	}

	// Check all conditions.
	hasAllLands := true
	for _, v := range basicTypes {
		if !v {
			hasAllLands = false
			break
		}
	}
	hasAllColors := true
	for _, v := range creatureColors {
		if !v {
			hasAllColors = false
			break
		}
	}

	if hasAllLands && hasAllColors {
		emitWin(gs, seat, slug, "Coalition Victory", "coalition_victory_win_condition")
	} else {
		emitFail(gs, slug, "Coalition Victory", "conditions_not_met", map[string]interface{}{
			"has_all_lands":  hasAllLands,
			"has_all_colors": hasAllColors,
		})
	}
}

// ---------------------------------------------------------------------------
// Grand Arbiter Augustin IV — {2}{W}{U} Creature (2/3)
//
// Oracle text:
//   White spells you cast cost {1} less to cast.
//   Blue spells you cast cost {1} less to cast.
//   Spells your opponents cast cost {1} more to cast.
//
// Static ability — uses flag approach for cost modifiers. The engine's
// ScanCostModifiers already handles named cards; we stamp a flag.
// ---------------------------------------------------------------------------

func registerGrandArbiterAugustinIV(r *Registry) {
	r.OnETB("Grand Arbiter Augustin IV", grandArbiterETB)
}

func grandArbiterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "grand_arbiter_static"
	if gs == nil || perm == nil {
		return
	}
	// Grand Arbiter is recognized by name in ScanCostModifiers.
	// ETB handler stamps a log entry so tournament replay can see it.
	emit(gs, slug, "Grand Arbiter Augustin IV", map[string]interface{}{
		"seat":      perm.Controller,
		"timestamp": perm.Timestamp,
		"effect":    "wu_cost_reduction_opponent_cost_increase",
	})
}

// ---------------------------------------------------------------------------
// Field of the Dead — Land
//
// Oracle text:
//   Field of the Dead enters the battlefield tapped.
//   {T}: Add {C}.
//   Whenever Field of the Dead or another land enters the battlefield
//   under your control, if you control seven or more lands with
//   different names, create a 2/2 black Zombie creature token.
//
// Trigger on land ETB.
// ---------------------------------------------------------------------------

func registerFieldOfTheDead(r *Registry) {
	r.OnTrigger("Field of the Dead", "permanent_entered_battlefield", fieldOfTheDeadTrigger)
	r.OnTrigger("Field of the Dead", "land_entered_battlefield", fieldOfTheDeadTrigger)
}

func fieldOfTheDeadTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "field_of_the_dead_zombie"
	if gs == nil || perm == nil {
		return
	}
	// Check if a land entered under the controller's control.
	enteringSeat, _ := ctx["entering_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	enteringPerm, _ := ctx["entering_permanent"].(*gameengine.Permanent)
	if enteringPerm != nil && !enteringPerm.IsLand() {
		return
	}

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Count differently named lands.
	landNames := map[string]bool{}
	for _, p := range s.Battlefield {
		if p != nil && p.IsLand() && p.Card != nil {
			landNames[p.Card.DisplayName()] = true
		}
	}

	if len(landNames) < 7 {
		return
	}

	// Create a 2/2 black Zombie creature token.
	gameengine.CreateCreatureToken(gs, seat, "Zombie Token",
		[]string{"creature", "zombie"}, 2, 2)

	emit(gs, slug, "Field of the Dead", map[string]interface{}{
		"seat":        seat,
		"unique_lands": len(landNames),
	})
}

// ---------------------------------------------------------------------------
// Gaea's Cradle — Legendary Land
//
// Oracle text:
//   {T}: Add {G} for each creature you control.
// ---------------------------------------------------------------------------

func registerGaeasCradle(r *Registry) {
	r.OnActivated("Gaea's Cradle", gaeasCradleActivate)
}

func gaeasCradleActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "gaeas_cradle_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Gaea's Cradle", "already_tapped", nil)
		return
	}
	src.Tapped = true
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Count creatures controlled.
	creatures := 0
	for _, p := range s.Battlefield {
		if p != nil && p.IsCreature() {
			creatures++
		}
	}

	if creatures > 0 {
		s.ManaPool += creatures
		gameengine.SyncManaAfterAdd(s, creatures)
		gs.LogEvent(gameengine.Event{
			Kind:   "add_mana",
			Seat:   seat,
			Target: seat,
			Source: "Gaea's Cradle",
			Amount: creatures,
			Details: map[string]interface{}{
				"reason":    "gaeas_cradle_tap",
				"creatures": creatures,
			},
		})
	}

	emit(gs, slug, "Gaea's Cradle", map[string]interface{}{
		"seat":      seat,
		"creatures": creatures,
		"mana":      creatures,
	})
}

// ---------------------------------------------------------------------------
// Glacial Chasm — Land
//
// Oracle text:
//   Cumulative upkeep — Pay 2 life.
//   When Glacial Chasm enters the battlefield, sacrifice a land.
//   Creatures you control can't attack.
//   Prevent all damage that would be dealt to you.
// ---------------------------------------------------------------------------

func registerGlacialChasm(r *Registry) {
	r.OnETB("Glacial Chasm", glacialChasmETB)
	r.OnTrigger("Glacial Chasm", "upkeep_controller", glacialChasmUpkeep)
}

func glacialChasmETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "glacial_chasm_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Sacrifice a land.
	var sacced *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p != nil && p.IsLand() && p != perm {
			sacced = p
			break
		}
	}
	if sacced != nil {
		gameengine.SacrificePermanent(gs, sacced, "glacial_chasm_etb_cost")
	}

	// Prevent all damage to controller.
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["prevent_all_damage"] = 1

	// Creatures can't attack (flag-based).
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["glacial_chasm_seat_"+intToStr(seat)] = 1

	// Initialize age counter for cumulative upkeep.
	perm.AddCounter("age", 1)

	emit(gs, slug, "Glacial Chasm", map[string]interface{}{
		"seat":       seat,
		"sacrificed": sacced != nil,
		"prevents":   "all_damage_to_controller",
	})
}

func glacialChasmUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "glacial_chasm_upkeep"
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Cumulative upkeep: add an age counter, pay 2 life per counter.
	perm.AddCounter("age", 1)
	ageCost := 0
	if perm.Counters != nil {
		ageCost = perm.Counters["age"] * 2
	}

	gs.Seats[seat].Life -= ageCost
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: "Glacial Chasm",
		Amount: ageCost,
		Details: map[string]interface{}{
			"reason":      "cumulative_upkeep",
			"age_counters": perm.Counters["age"],
		},
	})

	emit(gs, slug, "Glacial Chasm", map[string]interface{}{
		"seat":     seat,
		"age":      perm.Counters["age"],
		"life_paid": ageCost,
	})
	_ = gs.CheckEnd()
}

// ---------------------------------------------------------------------------
// Mishra's Workshop — Land
//
// Oracle text:
//   {T}: Add {C}{C}{C}. Spend this mana only to cast artifact spells.
//
// MVP: adds 3 generic mana (the artifact-only restriction is not
// enforced in the simplified mana model).
// ---------------------------------------------------------------------------

func registerMishrasWorkshop(r *Registry) {
	r.OnActivated("Mishra's Workshop", mishrasWorkshopActivate)
}

func mishrasWorkshopActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mishras_workshop_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Mishra's Workshop", "already_tapped", nil)
		return
	}
	src.Tapped = true
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	gameengine.AddRestrictedMana(gs, s, 3, "C", "artifact_only", "Mishra's Workshop")
	emit(gs, slug, "Mishra's Workshop", map[string]interface{}{
		"seat":        seat,
		"added":       3,
		"restriction": "artifact_only",
		"new_pool":    s.ManaPool,
	})
}

// ---------------------------------------------------------------------------
// Panoptic Mirror — {5} Artifact
//
// Oracle text:
//   Imprint — {X}, {T}: You may exile an instant or sorcery card with
//   mana value X from your hand.
//   At the beginning of your upkeep, you may copy a card exiled with
//   Panoptic Mirror. If you do, you may cast the copy without paying its
//   mana cost.
//
// Implementation:
//   - Activated: exile an instant/sorcery from hand (imprint).
//   - Upkeep trigger: cast a copy of the imprinted card for free.
// ---------------------------------------------------------------------------

func registerPanopticMirror(r *Registry) {
	r.OnActivated("Panoptic Mirror", panopticMirrorActivate)
	r.OnTrigger("Panoptic Mirror", "upkeep_controller", panopticMirrorUpkeep)
}

func panopticMirrorActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "panoptic_mirror_imprint"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Panoptic Mirror", "already_tapped", nil)
		return
	}
	src.Tapped = true
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Find an instant or sorcery in hand to exile (imprint).
	var best *gameengine.Card
	bestIdx := -1
	for i, c := range s.Hand {
		if c == nil {
			continue
		}
		if cardHasType(c, "instant") || cardHasType(c, "sorcery") {
			if best == nil || cardCMC(c) > cardCMC(best) {
				best = c
				bestIdx = i
			}
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, "Panoptic Mirror", "no_instant_or_sorcery_in_hand", nil)
		return
	}

	// Exile the card (imprint).
	gameengine.MoveCard(gs, best, seat, "hand", "exile", "panoptic_mirror_imprint")

	// Store the imprinted card on the sync.Map keyed by the permanent.
	panopticImprinted.Store(src, best)
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["imprint_present"] = 1

	emit(gs, slug, "Panoptic Mirror", map[string]interface{}{
		"seat":      seat,
		"imprinted": best.DisplayName(),
	})
}

func panopticMirrorUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "panoptic_mirror_cast"
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Check if anything is imprinted.
	if perm.Flags == nil || perm.Flags["imprint_present"] == 0 {
		return
	}

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Look up the actual imprinted card from the sync.Map.
	var imprinted *gameengine.Card
	if v, ok := panopticImprinted.Load(perm); ok {
		imprinted = v.(*gameengine.Card)
	}
	if imprinted == nil {
		return
	}

	// Cast a copy of the imprinted card without paying its mana cost.
	item := &gameengine.StackItem{
		Controller: seat,
		Card:       imprinted,
		IsCopy:     true,
	}
	fired := gameengine.InvokeResolveHook(gs, item)

	emit(gs, slug, "Panoptic Mirror", map[string]interface{}{
		"seat":           seat,
		"copied":         imprinted.DisplayName(),
		"handlers_fired": fired,
	})
	if fired == 0 {
		emitPartial(gs, slug, "Panoptic Mirror",
			"no_resolve_handler_for_imprinted_card_copy_logged_but_no_effect")
	}
}

// ---------------------------------------------------------------------------
// Serra's Sanctum — Legendary Land
//
// Oracle text:
//   {T}: Add {W} for each enchantment you control.
// ---------------------------------------------------------------------------

func registerSerrasSanctum(r *Registry) {
	r.OnActivated("Serra's Sanctum", serrasSanctumActivate)
}

func serrasSanctumActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "serras_sanctum_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Serra's Sanctum", "already_tapped", nil)
		return
	}
	src.Tapped = true
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Count enchantments controlled.
	enchantments := 0
	for _, p := range s.Battlefield {
		if p != nil && p.IsEnchantment() {
			enchantments++
		}
	}

	if enchantments > 0 {
		s.ManaPool += enchantments
		gameengine.SyncManaAfterAdd(s, enchantments)
		gs.LogEvent(gameengine.Event{
			Kind:   "add_mana",
			Seat:   seat,
			Target: seat,
			Source: "Serra's Sanctum",
			Amount: enchantments,
			Details: map[string]interface{}{
				"reason":       "serras_sanctum_tap",
				"enchantments": enchantments,
			},
		})
	}

	emit(gs, slug, "Serra's Sanctum", map[string]interface{}{
		"seat":         seat,
		"enchantments": enchantments,
		"mana":         enchantments,
	})
}

// ---------------------------------------------------------------------------
// The Tabernacle at Pendrell Vale — Legendary Land
//
// Oracle text:
//   All creatures have "At the beginning of your upkeep, destroy this
//   creature unless you pay {1}."
//
// Implementation: upkeep trigger that destroys each creature whose
// controller can't/doesn't pay {1}. AI policy: pay for creatures with
// CMC >= 3, sacrifice cheaper ones.
// ---------------------------------------------------------------------------

func registerTabernacleAtPendrellVale(r *Registry) {
	r.OnTrigger("The Tabernacle at Pendrell Vale", "upkeep_start", tabernacleUpkeep)
}

func tabernacleUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tabernacle_upkeep"
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[activeSeat]
	if s == nil || s.Lost {
		return
	}

	// For each creature the active player controls, they must pay {1} or
	// destroy it. AI policy: pay for valuable creatures (CMC >= 3).
	var toDestroy []*gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		cmc := 0
		if p.Card != nil {
			cmc = cardCMC(p.Card)
		}
		if cmc >= 3 && s.ManaPool >= 1 {
			// Pay {1} to keep this creature.
			s.ManaPool -= 1
			gameengine.SyncManaAfterSpend(s)
		} else {
			toDestroy = append(toDestroy, p)
		}
	}

	for _, p := range toDestroy {
		gameengine.SacrificePermanent(gs, p, "tabernacle_destroy")
	}

	emit(gs, slug, "The Tabernacle at Pendrell Vale", map[string]interface{}{
		"active_seat": activeSeat,
		"destroyed":   len(toDestroy),
		"controller":  perm.Controller,
	})
}
