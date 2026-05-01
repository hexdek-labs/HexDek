package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAjaniNacatlPariah wires Ajani, Nacatl Pariah // Ajani, Nacatl
// Avenger (Outlaws of Thunder Junction transforming planeswalker DFC).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
// Front — Ajani, Nacatl Pariah (Legendary Creature — Cat Warrior, 1/1):
//
//	When Ajani enters, create a 2/1 white Cat Warrior creature token.
//	Whenever one or more other Cats you control die, you may exile
//	Ajani, then return him to the battlefield transformed under his
//	owner's control.
//
// Back — Ajani, Nacatl Avenger (Legendary Planeswalker — Ajani):
//
//	+2: Put a +1/+1 counter on each Cat you control.
//	0: Create a 2/1 white Cat Warrior creature token. When you do, if
//	   you control a red permanent other than Ajani, he deals damage
//	   equal to the number of creatures you control to any target.
//	−4: Each opponent chooses an artifact, a creature, an enchantment,
//	    and a planeswalker from among the nonland permanents they
//	    control, then sacrifices the rest.
//
// Implementation:
//   - ETB (front): mints a 2/1 white Cat Warrior token.
//   - "creature_dies" trigger: when at least one OTHER Cat controlled
//     by Ajani's controller dies, opt into the may-transform clause and
//     flip Ajani via TransformPermanent. Multiple Cats dying in the same
//     event still resolves once (we dedupe on perm.Flags["ajani_pariah_flipped"]).
//   - Back-face activated abilities (loyalty cost paid by activation
//     pipeline; this handler delivers the effects):
//       * abilityIdx 0 (+2): walk Ajani's controller's battlefield, add
//         a +1/+1 counter to each Cat. Includes Ajani himself if he
//         retained the Cat type post-transform; Avenger is a planeswalker
//         only, so we exclude src.
//       * abilityIdx 1 (0): mint a 2/1 Cat token. If controller has a
//         red permanent other than Ajani, deal damage = creature count
//         to lowest-life opponent (best target by life-loss heuristic).
//       * abilityIdx 2 (-4): for each opponent, retain one of each card
//         type (artifact, creature, enchantment, planeswalker) chosen
//         to maximize value, sacrifice the rest of the nonland permanents.
//
// DFC dispatch: register all three name forms (full DFC, front, back).
func registerAjaniNacatlPariah(r *Registry) {
	r.OnETB("Ajani, Nacatl Pariah // Ajani, Nacatl Avenger", ajaniNacatlPariahETB)
	r.OnETB("Ajani, Nacatl Pariah", ajaniNacatlPariahETB)
	r.OnTrigger("Ajani, Nacatl Pariah // Ajani, Nacatl Avenger", "creature_dies", ajaniNacatlPariahCatDies)
	r.OnTrigger("Ajani, Nacatl Pariah", "creature_dies", ajaniNacatlPariahCatDies)
	r.OnActivated("Ajani, Nacatl Pariah // Ajani, Nacatl Avenger", ajaniNacatlAvengerActivate)
	r.OnActivated("Ajani, Nacatl Avenger", ajaniNacatlAvengerActivate)
}

func ajaniNacatlPariahETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ajani_nacatl_pariah_etb_cat_token"
	if gs == nil || perm == nil {
		return
	}
	if perm.Transformed {
		return
	}
	mintAjaniCatToken(gs, perm.Controller, perm.Card.DisplayName(), slug)
}

func ajaniNacatlPariahCatDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ajani_nacatl_pariah_cat_died_transform"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if perm.Transformed {
		return
	}
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	if dyingCard == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	// "another Cat" — not Ajani himself, must be a Cat.
	if dyingCard == perm.Card {
		return
	}
	if !cardHasType(dyingCard, "cat") {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["ajani_pariah_flipped"] == 1 {
		return
	}

	if !gameengine.TransformPermanent(gs, perm, "ajani_nacatl_pariah_cat_died") {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"transform_failed_face_data_missing")
		return
	}
	perm.Flags["ajani_pariah_flipped"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"dead_cat":  dyingCard.DisplayName(),
		"to":        "Ajani, Nacatl Avenger",
	})
}

func ajaniNacatlAvengerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	switch abilityIdx {
	case 0:
		ajaniAvengerPlusTwo(gs, src)
	case 1:
		ajaniAvengerZero(gs, src)
	case 2:
		ajaniAvengerMinusFour(gs, src)
	}
}

func ajaniAvengerPlusTwo(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "ajani_nacatl_avenger_plus_two"
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == src {
			continue
		}
		if !cardHasType(p.Card, "cat") {
			continue
		}
		p.AddCounter("+1/+1", 1)
		count++
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           src.Controller,
		"cats_buffed":    count,
	})
}

func ajaniAvengerZero(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "ajani_nacatl_avenger_zero"
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	mintAjaniCatToken(gs, src.Controller, src.Card.DisplayName(), slug)

	// "if you control a red permanent other than Ajani, he deals damage
	// equal to the number of creatures you control to any target."
	hasRed := false
	creatureCount := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsCreature() {
			creatureCount++
		}
		if p == src {
			continue
		}
		for _, c := range p.Card.Colors {
			if c == "R" || c == "r" {
				hasRed = true
				break
			}
		}
		if !hasRed {
			for _, t := range p.Card.Types {
				if t == "pip:R" {
					hasRed = true
					break
				}
			}
		}
	}
	if !hasRed || creatureCount == 0 {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":           src.Controller,
			"red_permanent":  hasRed,
			"creature_count": creatureCount,
			"damage":         0,
		})
		return
	}

	// Best target — pick lowest-life living opponent.
	target := -1
	bestLife := 1 << 30
	for _, opp := range gs.Opponents(src.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		return
	}
	gs.Seats[target].Life -= creatureCount
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   src.Controller,
		Target: target,
		Source: src.Card.DisplayName(),
		Amount: creatureCount,
		Details: map[string]interface{}{
			"cause": "ajani_nacatl_avenger_zero",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           src.Controller,
		"red_permanent":  true,
		"creature_count": creatureCount,
		"damage_to":      target,
		"damage":         creatureCount,
	})
}

func ajaniAvengerMinusFour(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "ajani_nacatl_avenger_minus_four"
	totalSacced := 0
	keptByOpp := map[int][]string{}
	for _, oppIdx := range gs.Opponents(src.Controller) {
		opp := gs.Seats[oppIdx]
		if opp == nil || opp.Lost {
			continue
		}
		// Choose one of each: artifact, creature, enchantment, planeswalker.
		// Heuristic for "their choice": pick the most-valuable (highest CMC)
		// in each category.
		keptByType := map[string]*gameengine.Permanent{}
		for _, p := range opp.Battlefield {
			if p == nil || p.Card == nil || p.IsLand() {
				continue
			}
			for _, kind := range []string{"artifact", "creature", "enchantment", "planeswalker"} {
				if !cardHasType(p.Card, kind) {
					continue
				}
				cur := keptByType[kind]
				if cur == nil || cardCMC(p.Card) > cardCMC(cur.Card) {
					keptByType[kind] = p
				}
			}
		}
		var victims []*gameengine.Permanent
		var keptNames []string
		for _, p := range opp.Battlefield {
			if p == nil || p.Card == nil || p.IsLand() {
				continue
			}
			kept := false
			for _, k := range keptByType {
				if k == p {
					kept = true
					keptNames = append(keptNames, p.Card.DisplayName())
					break
				}
			}
			if !kept {
				victims = append(victims, p)
			}
		}
		// Dedupe keptNames since a single permanent can match multiple types.
		seen := map[string]bool{}
		dedup := keptNames[:0]
		for _, n := range keptNames {
			if seen[n] {
				continue
			}
			seen[n] = true
			dedup = append(dedup, n)
		}
		keptByOpp[oppIdx] = dedup
		for _, v := range victims {
			gameengine.SacrificePermanent(gs, v, "ajani_nacatl_avenger_minus_four")
			totalSacced++
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          src.Controller,
		"total_sacced":  totalSacced,
		"kept_by_opp":   keptByOpp,
	})
}

// mintAjaniCatToken mints a 2/1 white Cat Warrior token for seat.
func mintAjaniCatToken(gs *gameengine.GameState, seat int, sourceName, slug string) {
	tokenCard := &gameengine.Card{
		Name:          "Cat Warrior Token",
		Owner:         seat,
		Types:         []string{"creature", "token", "cat", "warrior", "pip:W"},
		Colors:        []string{"W"},
		BasePower:     2,
		BaseToughness: 1,
	}
	enterBattlefieldWithETB(gs, seat, tokenCard, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   seat,
		Source: sourceName,
		Details: map[string]interface{}{
			"token":  "Cat Warrior Token",
			"reason": slug,
			"power":  2,
			"tough":  1,
		},
	})
}
