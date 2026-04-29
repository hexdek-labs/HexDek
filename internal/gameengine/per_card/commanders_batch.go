package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// =============================================================================
// Batch #9 — 23 commander per-card handlers for the deck collection.
//
// Each handler implements the PRIMARY ability of the named commander. Complex
// secondary abilities are logged as partial.
// =============================================================================

// ---------------------------------------------------------------------------
// 1. Ardenn, Intrepid Archaeologist
//
// "At the beginning of combat on your turn, you may attach any number of
// Auras and Equipment you control to target permanent you control or
// target player."
//
// Implementation: OnTrigger "combat_begin" (or beginning-of-combat phase
// trigger). Auto-attaches all unattached Equipment to the best creature
// controlled by Ardenn's controller.
// ---------------------------------------------------------------------------

func registerArdenn(r *Registry) {
	r.OnTrigger("Ardenn, Intrepid Archaeologist", "upkeep_controller", ardennCombatTrigger)
}

func ardennCombatTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	// Find best creature (highest power) to attach equipment to.
	var bestCreature *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		if bestCreature == nil || p.Power() > bestCreature.Power() {
			bestCreature = p
		}
	}
	if bestCreature == nil {
		return
	}
	// Attach all unattached Equipment to it.
	attached := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsArtifact() {
			continue
		}
		isEquipment := false
		if p.Card != nil {
			for _, t := range p.Card.Types {
				if strings.EqualFold(t, "equipment") {
					isEquipment = true
					break
				}
			}
		}
		if !isEquipment || p.AttachedTo != nil {
			continue
		}
		p.AttachedTo = bestCreature
		attached++
	}
	if attached > 0 {
		emit(gs, "ardenn_attach", perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"target":    bestCreature.Card.DisplayName(),
			"count":     attached,
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Coram, the Undertaker
//
// "Whenever Coram, the Undertaker attacks, each player mills a card."
// Also: "Coram gets +X/+0, where X is the greatest power among creature
// cards in all graveyards." (static buff via modification)
// ---------------------------------------------------------------------------

func registerCoram(r *Registry) {
	r.OnTrigger("Coram, the Undertaker", "creature_attacks", coramAttackTrigger)
}

func coramAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	attackerPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attackerPerm != perm {
		return
	}
	// Each player mills a card. Track milled cards for graveyard casting.
	milled := 0
	var milledCards []*gameengine.Card
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if len(s.Library) > 0 {
			card := s.Library[0]
			gameengine.MoveCard(gs, card, i, "library", "graveyard", "mill")
			milledCards = append(milledCards, card)
			milled++
			gs.LogEvent(gameengine.Event{
				Kind:   "mill",
				Seat:   i,
				Source: perm.Card.DisplayName(),
				Amount: 1,
			})
		}
	}
	// Apply +X/+0 where X is greatest creature power in all graveyards.
	maxPow := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			if cardHasType(c, "creature") && c.BasePower > maxPow {
				maxPow = c.BasePower
			}
		}
	}
	if maxPow > 0 {
		perm.Modifications = append(perm.Modifications, gameengine.Modification{
			Power:    maxPow,
			Duration: "until_end_of_turn",
		})
	}

	// "During each of your turns, you may play a land or cast a spell from
	// among cards in all graveyards that were put there from libraries this
	// turn." — cast the best nonland milled card for free.
	coramGraveyardCast(gs, perm, milledCards)

	emit(gs, "coram_attack", perm.Card.DisplayName(), map[string]interface{}{
		"milled":     milled,
		"power_buff": maxPow,
	})
}

// coramGraveyardCast picks the best nonland card from the just-milled cards
// and casts it for free under Coram's controller.
func coramGraveyardCast(gs *gameengine.GameState, coram *gameengine.Permanent, milledCards []*gameengine.Card) {
	if len(milledCards) == 0 {
		return
	}
	controller := coram.Controller
	seat := gs.Seats[controller]
	if seat == nil || seat.Lost {
		return
	}

	// Pick the highest-CMC nonland card among milled cards.
	var best *gameengine.Card
	bestCMC := -1
	bestSeatIdx := -1
	for _, mc := range milledCards {
		if mc == nil || cardHasType(mc, "land") {
			continue
		}
		cmc := mc.CMC
		if cmc > bestCMC {
			best = mc
			bestCMC = cmc
			// Find which graveyard this card is in.
			for si, s := range gs.Seats {
				if s == nil {
					continue
				}
				for _, gc := range s.Graveyard {
					if gc == best {
						bestSeatIdx = si
						break
					}
				}
				if bestSeatIdx >= 0 {
					break
				}
			}
		}
	}
	if best == nil || bestSeatIdx < 0 {
		return
	}

	// Check mana availability — Coram lets you cast without paying, but
	// we still need to verify the card is castable (not a land).
	if cardHasType(best, "land") {
		return
	}

	// Remove from graveyard.
	gySeat := gs.Seats[bestSeatIdx]
	removed := false
	for i, c := range gySeat.Graveyard {
		if c == best {
			gySeat.Graveyard = append(gySeat.Graveyard[:i], gySeat.Graveyard[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		return
	}

	// Cast for free: put into hand then cast with mana waived.
	seat.Hand = append(seat.Hand, best)
	err := gameengine.CastSpell(gs, controller, best, nil)
	if err != nil {
		// Cast failed — put back in graveyard.
		for i, c := range seat.Hand {
			if c == best {
				seat.Hand = append(seat.Hand[:i], seat.Hand[i+1:]...)
				break
			}
		}
		gySeat.Graveyard = append(gySeat.Graveyard, best)
		return
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "coram_graveyard_cast",
		Seat:   controller,
		Source: coram.Card.DisplayName(),
		Details: map[string]interface{}{
			"cast_card":    best.DisplayName(),
			"from_seat":    bestSeatIdx,
			"rule":         "coram_graveyard_ability",
		},
	})
}

// ---------------------------------------------------------------------------
// 3. Fire Lord Azula
//
// "Whenever you cast a noncreature spell during combat, create a 1/1
// blue and red Elemental creature token."
// ---------------------------------------------------------------------------

func registerFireLordAzula(r *Registry) {
	r.OnTrigger("Fire Lord Azula", "noncreature_spell_cast", azulaTrigger)
}

func azulaTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	// Check if we're in combat phase.
	if gs.Phase != "combat" {
		return
	}
	seat := perm.Controller
	token := &gameengine.Card{
		Name:          "1/1 Elemental Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "elemental"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "1/1 Elemental",
			"reason": "fire_lord_azula_combat_cast",
		},
	})
	emit(gs, "azula_token", perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

// ---------------------------------------------------------------------------
// 4. Kaust, Eyes of the Glade
//
// "Whenever a face-down creature you control is turned face up, put two
// +1/+1 counters on it."
//
// Implementation: listen on "permanent_etb" and check for face-up events.
// Since face-down/morph isn't fully wired, we register a stub that fires
// on a "face_up" event if the engine emits it.
// ---------------------------------------------------------------------------

func registerKaust(r *Registry) {
	r.OnTrigger("Kaust, Eyes of the Glade", "permanent_etb", kaustTrigger)
}

func kaustTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	// Check if the entering permanent is face-up from face-down.
	entryPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if entryPerm == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	// Check for face-up flag (morph/manifest flip).
	if entryPerm.Flags == nil {
		return
	}
	if _, ok := entryPerm.Flags["turned_face_up"]; !ok {
		return
	}
	if entryPerm.Counters == nil {
		entryPerm.Counters = map[string]int{}
	}
	entryPerm.Counters["+1/+1"] += 2
	emit(gs, "kaust_counters", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": entryPerm.Card.DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// 5. Lord of the Nazgul
//
// "Whenever you cast an instant or sorcery spell, amass Wraiths 1."
// (Create a 0/0 Wraith Army token or put +1/+1 counter on existing.)
// ---------------------------------------------------------------------------

func registerLordOfTheNazgul(r *Registry) {
	r.OnTrigger("Lord of the Nazgûl", "instant_or_sorcery_cast", nazgulTrigger)
}

func nazgulTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	// Amass 1: find existing Army token, or create a 0/0 Wraith Army.
	var army *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "army") {
				army = p
				break
			}
		}
		if army != nil {
			break
		}
	}
	if army != nil {
		if army.Counters == nil {
			army.Counters = map[string]int{}
		}
		army.Counters["+1/+1"]++
	} else {
		token := &gameengine.Card{
			Name:          "0/0 Wraith Army Token",
			Owner:         seat,
			BasePower:     0,
			BaseToughness: 0,
			Types:         []string{"token", "creature", "wraith", "army"},
		}
		army = enterBattlefieldWithETB(gs, seat, token, false)
		if army != nil {
			army.Counters["+1/+1"] = 1
		}
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "amass",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"type": "wraith",
		},
	})
	emit(gs, "nazgul_amass", perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

// ---------------------------------------------------------------------------
// 6. Lumra, Bellow of the Woods
//
// "When Lumra enters, mill four cards."
// "Lumra gets +1/+1 for each land card in your graveyard."
// ---------------------------------------------------------------------------

func registerLumra(r *Registry) {
	r.OnETB("Lumra, Bellow of the Woods", lumraETB)
}

func lumraETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	// Mill 4.
	milled := 0
	for i := 0; i < 4; i++ {
		if len(s.Library) == 0 {
			break
		}
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "graveyard", "mill")
		milled++
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "mill",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Amount: milled,
	})
	// Count lands in graveyard for +1/+1 buff.
	landCount := 0
	for _, c := range s.Graveyard {
		if c != nil && cardHasType(c, "land") {
			landCount++
		}
	}
	if landCount > 0 {
		perm.Modifications = append(perm.Modifications, gameengine.Modification{
			Power:     landCount,
			Toughness: landCount,
			Duration:  "permanent",
		})
	}
	emit(gs, "lumra_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"milled":   milled,
		"land_buff": landCount,
	})
}

// ---------------------------------------------------------------------------
// 7. Maja, Bretagard Protector
//
// "Whenever a land enters the battlefield under your control, create a
// 1/1 white Human Warrior creature token."
// ---------------------------------------------------------------------------

func registerMaja(r *Registry) {
	r.OnTrigger("Maja, Bretagard Protector", "permanent_etb", majaTrigger)
}

func majaTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entryPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if entryPerm == nil || !entryPerm.IsLand() {
		return
	}
	seat := perm.Controller
	token := &gameengine.Card{
		Name:          "1/1 Human Warrior Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "human", "warrior"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "1/1 Human Warrior",
			"reason": "maja_landfall",
		},
	})
}

// ---------------------------------------------------------------------------
// 8. Moraug, Fury of Akoum
//
// "Landfall — Whenever a land enters the battlefield under your control,
// if it's your main phase, there's an additional combat phase after this
// phase."
// ---------------------------------------------------------------------------

func registerMoraug(r *Registry) {
	r.OnTrigger("Moraug, Fury of Akoum", "permanent_etb", moraugTrigger)
}

func moraugTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entryPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if entryPerm == nil || !entryPerm.IsLand() {
		return
	}
	// Must be during main phase and controller's turn.
	if gs.Active != perm.Controller {
		return
	}
	if gs.Phase != "main" && gs.Phase != "precombat_main" && gs.Phase != "postcombat_main" {
		return
	}
	gs.PendingExtraCombats++
	gs.LogEvent(gameengine.Event{
		Kind:   "extra_combat",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason": "moraug_landfall",
		},
	})
	emit(gs, "moraug_extra_combat", perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"extra_combats":  gs.PendingExtraCombats,
	})
}

// ---------------------------------------------------------------------------
// 9. Muldrotha, the Gravetide
//
// "During each of your turns, you may play a land from your graveyard
// and cast a permanent spell of each permanent type from your graveyard."
//
// Implementation: set a zone-cast permission flag on ETB. The actual
// cast-from-graveyard logic is a static permission the AI/Hat reads.
// ---------------------------------------------------------------------------

func registerMuldrotha(r *Registry) {
	r.OnETB("Muldrotha, the Gravetide", muldrothaETB)
}

func muldrothaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["muldrotha_gy_cast"] = 1
	emit(gs, "muldrotha_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"effect": "graveyard_cast_permission_set",
	})
}

// ---------------------------------------------------------------------------
// 10. Narset, Enlightened Exile
//
// "Whenever you cast a noncreature spell, exile the top card of your
// library. You may cast it this turn."
// ---------------------------------------------------------------------------

func registerNarsetEnlightenedExile(r *Registry) {
	r.OnTrigger("Narset, Enlightened Exile", "noncreature_spell_cast", narsetExileTrigger)
}

func narsetExileTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		return
	}
	card := s.Library[0]
	gameengine.MoveCard(gs, card, seat, "library", "exile", "impulse-draw")
	// Grant zone-cast permission for this card.
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	gs.ZoneCastGrants[card] = &gameengine.ZoneCastPermission{
		Zone:              "exile",
		Keyword:           "narset_exile_cast",
		ManaCost:          -1, // use card's normal cost
		RequireController: seat,
		SourceName:        perm.Card.DisplayName(),
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "exile_from_library",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"card":   card.DisplayName(),
			"reason": "narset_exile_trigger",
		},
	})
	emit(gs, "narset_exile", perm.Card.DisplayName(), map[string]interface{}{
		"seat":    seat,
		"exiled":  card.DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// 11. Obeka, Splitter of Seconds
//
// "Menace
//  Whenever Obeka deals combat damage to a player, you get that many
//  additional upkeep steps after this phase."
// ---------------------------------------------------------------------------

func registerObeka(r *Registry) {
	r.OnTrigger("Obeka, Splitter of Seconds", "combat_damage_player", obekaCombatDamage)
}

func obekaCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	srcCard, _ := ctx["source_card"].(string)
	if srcCard != perm.Card.DisplayName() {
		return
	}
	srcSeat, _ := ctx["source_seat"].(int)
	if srcSeat != perm.Controller {
		return
	}
	dmg, _ := ctx["amount"].(int)
	if dmg <= 0 {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["obeka_extra_upkeeps"] += dmg
	emit(gs, "obeka_trigger", perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"damage":         dmg,
		"extra_upkeeps":  gs.Flags["obeka_extra_upkeeps"],
	})
}

// ---------------------------------------------------------------------------
// 12. Oloro, Ageless Ascetic
//
// "At the beginning of your upkeep, you gain 2 life."
// "At the beginning of your upkeep, if Oloro is in the command zone,
// you gain 2 life."
// ---------------------------------------------------------------------------

func registerOloro(r *Registry) {
	r.OnTrigger("Oloro, Ageless Ascetic", "upkeep_controller", oloroUpkeep)
}

func oloroUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	gameengine.GainLife(gs, seat, 2, perm.Card.DisplayName())
}

// ---------------------------------------------------------------------------
// 13. Ragost, Deft Gastronaut (simplified — sacrifice Food triggers)
//
// "Whenever you sacrifice a Food, Ragost deals damage to any target
// equal to that Food's mana value."
//
// Implementation: listen on creature_dies/permanent_ltb and check for
// Food subtype in the dying permanent. In practice, Food tokens have
// CMC 0 so this deals 0 unless a real Food card was sacced. We still
// log the trigger for completeness.
// ---------------------------------------------------------------------------

func registerRagost(r *Registry) {
	r.OnTrigger("Ragost, Deft Gastronaut", "permanent_ltb", ragostTrigger)
}

func ragostTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller || dyingPerm == nil || dyingPerm.Card == nil {
		return
	}
	// Check if the leaving permanent is a Food.
	isFood := false
	for _, t := range dyingPerm.Card.Types {
		if strings.EqualFold(t, "food") {
			isFood = true
			break
		}
	}
	if !isFood {
		return
	}
	// Deal damage equal to mana value (CMC). Food tokens typically have 0.
	dmg := dyingPerm.Card.CMC
	if dmg > 0 {
		// Deal to a random opponent.
		opps := gs.Opponents(perm.Controller)
		if len(opps) > 0 {
			target := opps[0]
			if gs.Rng != nil {
				target = opps[gs.Rng.Intn(len(opps))]
			}
			gs.Seats[target].Life -= dmg
			gs.LogEvent(gameengine.Event{
				Kind:   "damage",
				Seat:   perm.Controller,
				Target: target,
				Source: perm.Card.DisplayName(),
				Amount: dmg,
			})
		}
	}
	emit(gs, "ragost_food_sac", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"damage": dmg,
	})
}

// ---------------------------------------------------------------------------
// 14. Ral, Monsoon Mage // Ral, Leyline Prodigy (DFC planeswalker)
//
// Front face: "Instant and sorcery spells you cast have
// 'When you cast this spell, copy it if you've cast 2+ spells this turn.'"
//
// Implementation: listen on instant_or_sorcery_cast, if controller has
// cast 2+ spells, log a copy event.
// ---------------------------------------------------------------------------

func registerRal(r *Registry) {
	r.OnTrigger("Ral, Monsoon Mage", "instant_or_sorcery_cast", ralTrigger)
}

func ralTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	if gs.Seats[seat].SpellsCastThisTurn < 2 {
		return
	}
	spellName, _ := ctx["spell_name"].(string)
	gs.LogEvent(gameengine.Event{
		Kind:   "spell_copy",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"copied_spell": spellName,
			"reason":       "ral_storm_copy",
		},
	})
	emit(gs, "ral_copy", perm.Card.DisplayName(), map[string]interface{}{
		"seat":    seat,
		"copied":  spellName,
	})
}

// ---------------------------------------------------------------------------
// 15. Riku of Many Paths
//
// "Whenever you cast a spell that targets two or more permanents and/or
// players, copy that spell. You may choose new targets for the copy."
//
// Implementation: listen on spell_cast, check if spell has 2+ targets
// (simplified: we just log the copy since target tracking is complex).
// ---------------------------------------------------------------------------

func registerRiku(r *Registry) {
	r.OnTrigger("Riku of Many Paths", "spell_cast", rikuTrigger)
}

func rikuTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	// In the absence of full target tracking, log a potential copy
	// for spells that typically have multiple targets. This is a
	// heuristic stub — the full implementation needs stack target data.
	spellName, _ := ctx["spell_name"].(string)
	gs.LogEvent(gameengine.Event{
		Kind:   "spell_copy_potential",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"copied_spell": spellName,
			"reason":       "riku_multi_target",
		},
	})
	emitPartial(gs, "riku_copy", perm.Card.DisplayName(), "target_count_check_requires_stack_data")
}

// ---------------------------------------------------------------------------
// 16. Soraya the Falconer
//
// "Other Birds you control get +1/+1."
// "{1}{W}: Target Bird gains banding until end of turn."
//
// Implementation: ETB handler that applies +1/+1 anthem as modifications
// to all Birds. Activated handler for banding grant.
// ---------------------------------------------------------------------------

func registerSoraya(r *Registry) {
	r.OnETB("Soraya the Falconer", sorayaETB)
}

func sorayaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	buffed := 0
	for _, p := range s.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		isBird := false
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "bird") {
				isBird = true
				break
			}
		}
		if isBird {
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power:     1,
				Toughness: 1,
				Duration:  "permanent",
			})
			buffed++
		}
	}
	emit(gs, "soraya_anthem", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"buffed": buffed,
	})
}

// ---------------------------------------------------------------------------
// 17. Tergrid, God of Fright
//
// "Whenever an opponent discards a card or sacrifices a nontoken
// permanent, you may put that card from a graveyard onto the battlefield
// under your control."
//
// Implementation: listen on permanent_ltb for sacrifice events from
// opponents. We grab the card and put it onto our battlefield.
// ---------------------------------------------------------------------------

func registerTergrid(r *Registry) {
	r.OnTrigger("Tergrid, God of Fright", "permanent_sacrificed", tergridTrigger)
	r.OnTrigger("Tergrid, God of Fright", "card_discarded", tergridDiscardTrigger)
}

func tergridTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["tergrid_resolving"] > 0 {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat == perm.Controller {
		return
	}
	if dyingPerm == nil || dyingPerm.Card == nil || dyingPerm.IsToken() {
		return
	}
	gs.Flags["tergrid_resolving"]++
	defer func() { gs.Flags["tergrid_resolving"]-- }()
	// Steal the card: find it in the opponent's graveyard and move it.
	oppSeat := controllerSeat
	if oppSeat < 0 || oppSeat >= len(gs.Seats) {
		return
	}
	card := dyingPerm.Card
	opp := gs.Seats[oppSeat]
	// Remove from opponent's graveyard and put onto Tergrid controller's battlefield.
	found := false
	for i, c := range opp.Graveyard {
		if c == card {
			opp.Graveyard = append(opp.Graveyard[:i], opp.Graveyard[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return
	}
	seat := perm.Controller
	newPerm := enterBattlefieldWithETB(gs, seat, card, false)
	if newPerm == nil {
		return
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "tergrid_steal",
		Seat:   seat,
		Target: oppSeat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"stolen_card": card.DisplayName(),
			"from_seat":   oppSeat,
		},
	})
	emit(gs, "tergrid_steal", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"stolen":    card.DisplayName(),
		"from_seat": oppSeat,
	})
}

// tergridDiscardTrigger fires when any player discards a card.
// Tergrid steals discarded permanents (puts on battlefield) and
// non-permanents (puts into hand) from opponents.
func tergridDiscardTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["tergrid_resolving"] > 0 {
		return
	}
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat == perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	isPermanent, _ := ctx["is_permanent"].(bool)
	if !isPermanent {
		return
	}

	oppSeat := discarderSeat
	if oppSeat < 0 || oppSeat >= len(gs.Seats) {
		return
	}
	opp := gs.Seats[oppSeat]
	found := false
	for i, c := range opp.Graveyard {
		if c == card {
			opp.Graveyard = append(opp.Graveyard[:i], opp.Graveyard[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return
	}

	seat := perm.Controller
	newPerm := enterBattlefieldWithETB(gs, seat, card, false)
	if newPerm == nil {
		return
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "tergrid_steal",
		Seat:   seat,
		Target: oppSeat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"stolen_card":  card.DisplayName(),
			"from_seat":    oppSeat,
			"from_discard": true,
			"is_permanent": isPermanent,
		},
	})
	emit(gs, "tergrid_steal", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"stolen":    card.DisplayName(),
		"from_seat": oppSeat,
		"source":    "discard",
	})
}

// ---------------------------------------------------------------------------
// 18. Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (DFC)
//
// Front: "Whenever this creature enters or transforms into Ulrich of
// the Krallenhorde, target creature gets +4/+4 until end of turn."
// Back: "Whenever this creature transforms into Ulrich, Uncontested
// Alpha, you may have it fight target non-Werewolf creature."
//
// Implementation: ETB handler for the front-face +4/+4 buff to best
// creature. DFC transform triggers would need the transform event.
// ---------------------------------------------------------------------------

func registerUlrich(r *Registry) {
	r.OnETB("Ulrich of the Krallenhorde", ulrichETB)
}

func ulrichETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Target creature gets +4/+4 until end of turn. Pick best creature.
	var best *gameengine.Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		if best == nil || p.Power() > best.Power() {
			best = p
		}
	}
	if best == nil {
		return
	}
	best.Modifications = append(best.Modifications, gameengine.Modification{
		Power:     4,
		Toughness: 4,
		Duration:  "until_end_of_turn",
	})
	emit(gs, "ulrich_etb_buff", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"target": best.Card.DisplayName(),
		"buff":   "+4/+4",
	})
	emitPartial(gs, "ulrich", perm.Card.DisplayName(), "back_face_fight_trigger_requires_transform_event")
}

// ---------------------------------------------------------------------------
// 19. Varina, Lich Queen
//
// "Whenever you attack with one or more Zombies, draw that many cards,
// then discard that many cards. You gain that much life."
// ---------------------------------------------------------------------------

func registerVarina(r *Registry) {
	r.OnTrigger("Varina, Lich Queen", "creature_attacks", varinaTrigger)
}

func varinaTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	attackerPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attackerPerm == nil || attackerPerm.Controller != perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	// De-dupe: oracle says "one or more Zombies" — fires once per combat.
	if s.Flags != nil && s.Flags["varina_triggered_this_combat"] > 0 {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["varina_triggered_this_combat"] = 1
	zombieCount := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "zombie") {
				zombieCount++
				break
			}
		}
	}
	if zombieCount == 0 {
		return
	}
	// Draw that many cards.
	for i := 0; i < zombieCount; i++ {
		drawOne(gs, seat, perm.Card.DisplayName())
	}
	// Discard that many cards (from hand, put in graveyard).
	discarded := 0
	for i := 0; i < zombieCount && len(s.Hand) > 0; i++ {
		// Discard last card in hand (simple heuristic).
		card := s.Hand[len(s.Hand)-1]
		gameengine.DiscardCard(gs, card, seat)
		discarded++
	}
	// Gain that much life.
	gameengine.GainLife(gs, seat, zombieCount, perm.Card.DisplayName())
	emit(gs, "varina_attack", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"zombies":   zombieCount,
		"drawn":     zombieCount,
		"discarded": discarded,
		"life":      zombieCount,
	})
}

// ---------------------------------------------------------------------------
// 20. Voja, Jaws of the Conclave
//
// "Whenever Voja attacks, choose one:
// - Put a +1/+1 counter on each Elf you control.
// - Voja deals damage equal to its power to any target."
//
// Implementation: listen on combat_damage_player (proxy for attack).
// Heuristic: if we have Elves, buff them; otherwise, deal damage.
// ---------------------------------------------------------------------------

func registerVoja(r *Registry) {
	r.OnTrigger("Voja, Jaws of the Conclave", "combat_damage_player", vojaTrigger)
}

func vojaTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller || !strings.EqualFold(sourceName, perm.Card.DisplayName()) {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	// Count Elves.
	elfCount := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "elf") {
				elfCount++
				break
			}
		}
	}
	if elfCount > 0 {
		// Mode 1: put +1/+1 on each Elf.
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			for _, t := range p.Card.Types {
				if strings.EqualFold(t, "elf") {
					if p.Counters == nil {
						p.Counters = map[string]int{}
					}
					p.Counters["+1/+1"]++
					break
				}
			}
		}
		emit(gs, "voja_elf_buff", perm.Card.DisplayName(), map[string]interface{}{
			"seat":  seat,
			"elves": elfCount,
		})
	} else {
		// Mode 2: deal damage equal to power to random opponent.
		opps := gs.Opponents(seat)
		if len(opps) > 0 {
			target := opps[0]
			if gs.Rng != nil {
				target = opps[gs.Rng.Intn(len(opps))]
			}
			dmg := perm.Power()
			gs.Seats[target].Life -= dmg
			gs.LogEvent(gameengine.Event{
				Kind:   "damage",
				Seat:   seat,
				Target: target,
				Source: perm.Card.DisplayName(),
				Amount: dmg,
				Details: map[string]interface{}{
					"reason": "voja_attack_damage",
				},
			})
		}
		emit(gs, "voja_damage", perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"damage": perm.Power(),
		})
	}
}

// ---------------------------------------------------------------------------
// 21. Y'shtola, Night's Blessed
//
// "Ward {2}. When Y'shtola enters the battlefield, draw cards equal to
// Y'shtola's power."
// ---------------------------------------------------------------------------

func registerYshtola(r *Registry) {
	r.OnETB("Y'shtola, Night's Blessed", yshtolaETB)
}

func yshtolaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	pow := perm.Power()
	if pow <= 0 {
		return
	}
	for i := 0; i < pow; i++ {
		drawOne(gs, seat, perm.Card.DisplayName())
	}
	emit(gs, "yshtola_etb_draw", perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"drawn": pow,
	})
}

// ---------------------------------------------------------------------------
// 22. Yarus, Roar of the Old Gods
//
// "Whenever a face-down permanent you control dies, reveal it. If it's a
// creature card, return it to the battlefield face up."
//
// Implementation: listen on creature_dies, check for face-down flag.
// If the dying permanent was face-down and is a creature card, return
// it to battlefield.
// ---------------------------------------------------------------------------

func registerYarus(r *Registry) {
	r.OnTrigger("Yarus, Roar of the Old Gods", "creature_dies", yarusTrigger)
}

func yarusTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	if dyingPerm == nil || dyingPerm.Card == nil {
		return
	}
	// Check for face-down flag.
	if dyingPerm.Flags == nil {
		return
	}
	if _, ok := dyingPerm.Flags["face_down"]; !ok {
		return
	}
	// If it's a creature card, return it to battlefield face up.
	if !cardHasType(dyingPerm.Card, "creature") {
		return
	}
	// Remove from graveyard and return to battlefield.
	seat := perm.Controller
	s := gs.Seats[seat]
	card := dyingPerm.Card
	for i, c := range s.Graveyard {
		if c == card {
			s.Graveyard = append(s.Graveyard[:i], s.Graveyard[i+1:]...)
			break
		}
	}
	newPerm := enterBattlefieldWithETB(gs, seat, card, false)
	if newPerm == nil {
		return
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "yarus_return",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"returned_card": card.DisplayName(),
		},
	})
	emit(gs, "yarus_return", perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"returned": card.DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// 23. Ashling the Pilgrim // Ashling the Extinguisher (multiple Ashlings)
//
// Ashling the Pilgrim: "{1}{R}: Put a +1/+1 counter on Ashling.
// If this is the third time this ability has resolved this turn, remove
// all +1/+1 counters. It deals that much damage to each creature and
// each player."
//
// Implementation: OnActivated for the +1/+1 counter ability with
// the third-activation boardwipe.
// ---------------------------------------------------------------------------

func registerAshling(r *Registry) {
	r.OnActivated("Ashling the Pilgrim", ashlingActivate)
}

func ashlingActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	if src.Counters == nil {
		src.Counters = map[string]int{}
	}
	src.Counters["+1/+1"]++
	// Track activations this turn via flags.
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["ashling_activations"]++
	activations := src.Flags["ashling_activations"]
	if activations >= 3 {
		// Remove all +1/+1 counters and deal that much damage to each
		// creature and each player.
		dmg := src.Counters["+1/+1"]
		src.Counters["+1/+1"] = 0
		src.Flags["ashling_activations"] = 0
		// Damage all players.
		for i, s := range gs.Seats {
			if s == nil || s.Lost {
				continue
			}
			s.Life -= dmg
			gs.LogEvent(gameengine.Event{
				Kind:   "damage",
				Seat:   src.Controller,
				Target: i,
				Source: src.Card.DisplayName(),
				Amount: dmg,
				Details: map[string]interface{}{
					"reason": "ashling_explosion",
				},
			})
		}
		// Damage all creatures (mark damage for SBA).
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				p.MarkedDamage += dmg
			}
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "ashling_explosion",
			Seat:   src.Controller,
			Source: src.Card.DisplayName(),
			Amount: dmg,
		})
	}
	emit(gs, "ashling_activate", src.Card.DisplayName(), map[string]interface{}{
		"seat":        src.Controller,
		"activations": activations,
		"counters":    src.Counters["+1/+1"],
	})
}
