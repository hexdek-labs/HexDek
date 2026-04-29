package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch #15 — Obeka support cards.
//
// Strategy-critical cards from the Obeka, Splitter of Seconds deck
// that rely on upkeep triggers, alternate win conditions, and extra
// phase generation. Without these handlers the cards are vanilla
// permanents that sit on the battlefield doing nothing.

// ---------------------------------------------------------------------------
// Braid of Fire
//
// "Cumulative upkeep — Add {R}."
// Each upkeep, add an age counter, then add that many R mana. Unlike
// normal cumulative upkeep, the "cost" is gaining mana — it's always
// paid. We just add age counters and mana directly.
// ---------------------------------------------------------------------------

func registerBraidOfFire(r *Registry) {
	r.OnTrigger("Braid of Fire", "upkeep_controller", braidOfFireUpkeep)
}

func braidOfFireUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["age"]++
	amount := perm.Counters["age"]
	seat := gs.Seats[perm.Controller]
	gameengine.AddMana(gs, seat, "R", amount, "Braid of Fire")
	emit(gs, "braid_of_fire_upkeep", "Braid of Fire", map[string]interface{}{
		"seat":         perm.Controller,
		"age_counters": amount,
		"mana_added":   amount,
	})
}

// ---------------------------------------------------------------------------
// Revel in Riches
//
// "At the beginning of your upkeep, if you control ten or more
// Treasures, you win the game."
// Also: "Whenever a creature an opponent controls dies, create a
// Treasure token." (death trigger handled separately via die_trigger).
// ---------------------------------------------------------------------------

func registerRevelInRiches(r *Registry) {
	r.OnTrigger("Revel in Riches", "upkeep_controller", revelInRichesUpkeep)
}

func revelInRichesUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	treasures := countTreasures(gs, perm.Controller)
	if treasures >= 10 {
		emitWin(gs, perm.Controller, "revel_in_riches_win", "Revel in Riches",
			"controlled 10+ treasures at upkeep")
	} else {
		emit(gs, "revel_in_riches_check", "Revel in Riches", map[string]interface{}{
			"seat":      perm.Controller,
			"treasures": treasures,
			"needed":    10,
		})
	}
}

func countTreasures(gs *gameengine.GameState, seatIdx int) int {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "treasure") || strings.EqualFold(t, "token") && strings.Contains(strings.ToLower(p.Card.Name), "treasure") {
				n++
				break
			}
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Sphinx of the Second Sun
//
// "At the beginning of your postcombat main phase, you get an
// additional beginning phase after this phase. (Untap, upkeep, draw.)"
// Implementation: set a flag that turn.go reads after main phase 2.
// ---------------------------------------------------------------------------

func registerSphinxOfTheSecondSun(r *Registry) {
	r.OnETB("Sphinx of the Second Sun", sphinxSecondSunETB)
}

func sphinxSecondSunETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["sphinx_second_sun_seat"] = perm.Controller + 1
	emit(gs, "sphinx_second_sun_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "extra_beginning_phase_after_postcombat_main",
	})
}

// ---------------------------------------------------------------------------
// Shadow of the Second Sun
//
// Same effect as Sphinx of the Second Sun but as an enchantment.
// "At the beginning of your postcombat main phase, you get an
// additional beginning phase after this phase."
// ---------------------------------------------------------------------------

func registerShadowOfTheSecondSun(r *Registry) {
	r.OnETB("Shadow of the Second Sun", shadowSecondSunETB)
}

func shadowSecondSunETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["shadow_second_sun_seat"] = perm.Controller + 1
	emit(gs, "shadow_second_sun_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "extra_beginning_phase_after_postcombat_main",
	})
}

// CheckSecondSunExtraPhase returns true if the active seat should get
// an extra beginning phase after their postcombat main (Sphinx/Shadow
// of the Second Sun effect). Called from turn.go.
func CheckSecondSunExtraPhase(gs *gameengine.GameState, activeSeat int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	// Check if the permanent is still on the battlefield.
	for _, name := range []string{"Sphinx of the Second Sun", "Shadow of the Second Sun"} {
		flagKey := strings.ReplaceAll(strings.ToLower(name), " ", "_") + "_seat"
		if v, ok := gs.Flags[flagKey]; ok && v == activeSeat+1 {
			if permanentOnBattlefield(gs, activeSeat, name) {
				return true
			}
		}
	}
	return false
}

func permanentOnBattlefield(gs *gameengine.GameState, seatIdx int, name string) bool {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == name {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Dragonmaster Outcast
//
// "At the beginning of your upkeep, if you control six or more lands,
// create a 5/5 red Dragon creature token with flying."
// ---------------------------------------------------------------------------

func registerDragonmasterOutcast(r *Registry) {
	r.OnTrigger("Dragonmaster Outcast", "upkeep_controller", dragonmasterOutcastUpkeep)
}

func dragonmasterOutcastUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	lands := 0
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p != nil && p.IsLand() {
			lands++
		}
	}
	if lands < 6 {
		return
	}
	dragon := gameengine.CreateCreatureToken(gs, perm.Controller, "Dragon",
		[]string{"creature", "dragon"}, 5, 5)
	if dragon != nil {
		dragon.Card.Types = append(dragon.Card.Types, "flying")
	}
	emit(gs, "dragonmaster_outcast_trigger", "Dragonmaster Outcast", map[string]interface{}{
		"seat":  perm.Controller,
		"lands": lands,
	})
}

// ---------------------------------------------------------------------------
// Braids, Conjurer Adept
//
// "At the beginning of each player's upkeep, that player may put an
// artifact, creature, or land card from their hand onto the battlefield."
// Implementation: each player picks their highest-CMC eligible card.
// ---------------------------------------------------------------------------

func registerBraidsConjurerAdept(r *Registry) {
	r.OnTrigger("Braids, Conjurer Adept", "upkeep_controller", braidsConjurerUpkeep)
}

func braidsConjurerUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	seat := gs.Seats[activeSeat]
	if seat == nil || seat.Lost {
		return
	}

	var best *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		tl := strings.ToLower(c.TypeLine)
		if !strings.Contains(tl, "creature") && !strings.Contains(tl, "artifact") && !strings.Contains(tl, "land") {
			eligible := false
			for _, t := range c.Types {
				lt := strings.ToLower(t)
				if lt == "creature" || lt == "artifact" || lt == "land" {
					eligible = true
					break
				}
			}
			if !eligible {
				continue
			}
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	if best == nil {
		return
	}

	gameengine.MoveCard(gs, best, activeSeat, "hand", "battlefield", "braids_conjurer")
	emit(gs, "braids_conjurer_trigger", "Braids, Conjurer Adept", map[string]interface{}{
		"seat":      activeSeat,
		"card":      best.DisplayName(),
		"braids_at": perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// As Foretold
//
// "At the beginning of your upkeep, put a time counter on As Foretold."
// "Once each turn, you may pay {0} rather than pay the mana cost of a
// spell you cast with mana value X or less, where X is the number of
// time counters on As Foretold."
// Implementation: upkeep counter increment + cost modifier.
// ---------------------------------------------------------------------------

func registerAsForetold(r *Registry) {
	r.OnTrigger("As Foretold", "upkeep_controller", asForetoldUpkeep)
}

func asForetoldUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["time"]++
	emit(gs, "as_foretold_upkeep", "As Foretold", map[string]interface{}{
		"seat":          perm.Controller,
		"time_counters": perm.Counters["time"],
	})
}

// ---------------------------------------------------------------------------
// Court of Embereth
//
// "When Court of Embereth enters the battlefield, you become the monarch."
// "At the beginning of your upkeep, create a 1/1 red Knight creature
// token with haste. If you're the monarch, create three of those tokens
// instead, then Court of Embereth deals damage to each opponent equal
// to the number of creatures you control."
// Simplified: create 1 token (monarch check omitted — no monarch system).
// ---------------------------------------------------------------------------

func registerCourtOfEmbereth(r *Registry) {
	r.OnTrigger("Court of Embereth", "upkeep_controller", courtOfEmberethUpkeep)
}

func courtOfEmberethUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	gameengine.CreateCreatureToken(gs, perm.Controller, "Knight",
		[]string{"creature", "knight"}, 1, 1)
	emit(gs, "court_of_embereth_trigger", "Court of Embereth", map[string]interface{}{
		"seat":   perm.Controller,
		"tokens": 1,
	})
	emitPartial(gs, "court_of_embereth", "Court of Embereth", "monarch system not implemented")
}

// ---------------------------------------------------------------------------
// Court of Vantress
//
// "When Court of Vantress enters the battlefield, you become the monarch."
// "At the beginning of your upkeep, look at the top card of each player's
// library. If you're the monarch, you may put any number of them on the
// bottom."
// Simplified: scry 1 (look at own top card, optionally bottom it).
// ---------------------------------------------------------------------------

func registerCourtOfVantress(r *Registry) {
	r.OnTrigger("Court of Vantress", "upkeep_controller", courtOfVantressUpkeep)
}

func courtOfVantressUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || len(seat.Library) == 0 {
		return
	}
	// Simplified: the Hat decides via ChooseScry if available.
	if seat.Hat != nil && len(seat.Library) > 0 {
		top := []*gameengine.Card{seat.Library[len(seat.Library)-1]}
		keepTop, _ := seat.Hat.ChooseScry(gs, perm.Controller, top)
		if len(keepTop) == 0 {
			card := seat.Library[len(seat.Library)-1]
			seat.Library = seat.Library[:len(seat.Library)-1]
			seat.Library = append([]*gameengine.Card{card}, seat.Library...)
		}
	}
	emit(gs, "court_of_vantress_trigger", "Court of Vantress", map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, "court_of_vantress", "Court of Vantress", "monarch system not implemented")
}

// ---------------------------------------------------------------------------
// Skyline Despot
//
// "When Skyline Despot enters the battlefield, you become the monarch."
// "At the beginning of your upkeep, if you're the monarch, create a
// 5/5 red Dragon creature token with flying."
// Simplified: create a dragon every upkeep (always treated as monarch).
// ---------------------------------------------------------------------------

func registerSkylineDespot(r *Registry) {
	r.OnTrigger("Skyline Despot", "upkeep_controller", skylineDespotUpkeep)
}

func skylineDespotUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	dragon := gameengine.CreateCreatureToken(gs, perm.Controller, "Dragon",
		[]string{"creature", "dragon"}, 5, 5)
	if dragon != nil {
		dragon.Card.Types = append(dragon.Card.Types, "flying")
	}
	emit(gs, "skyline_despot_trigger", "Skyline Despot", map[string]interface{}{
		"seat": perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// Extravagant Replication
//
// "At the beginning of your upkeep, create a token that's a copy of
// target nonland permanent you control."
// Simplified: copy the highest-CMC nonland permanent you control.
// ---------------------------------------------------------------------------

func registerExtravagantReplication(r *Registry) {
	r.OnTrigger("Extravagant Replication", "upkeep_controller", extravagantReplicationUpkeep)
}

func extravagantReplicationUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	var best *gameengine.Permanent
	bestCMC := -1
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p == nil || p.Card == nil || p.IsLand() {
			continue
		}
		if p.Card.DisplayName() == "Extravagant Replication" {
			continue
		}
		cmc := gameengine.ManaCostOf(p.Card)
		if cmc > bestCMC {
			bestCMC = cmc
			best = p
		}
	}
	if best == nil {
		return
	}
	// Create a token copy.
	if best.IsCreature() {
		gameengine.CreateCreatureToken(gs, perm.Controller,
			best.Card.DisplayName(),
			best.Card.Types,
			best.Card.BasePower, best.Card.BaseToughness)
	} else {
		gameengine.CreateCreatureToken(gs, perm.Controller,
			best.Card.DisplayName(),
			best.Card.Types, 0, 0)
	}
	emit(gs, "extravagant_replication_trigger", "Extravagant Replication", map[string]interface{}{
		"seat":   perm.Controller,
		"copied": best.Card.DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// Mechanized Production
//
// "At the beginning of your upkeep, create a token that's a copy of
// enchanted artifact. Then if you control eight or more artifacts with
// the same name as one another, you win the game."
// Simplified: copy the enchanted artifact (best artifact), check 8+ same.
// ---------------------------------------------------------------------------

func registerMechanizedProduction(r *Registry) {
	r.OnTrigger("Mechanized Production", "upkeep_controller", mechanizedProductionUpkeep)
}

func mechanizedProductionUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	// Find an artifact to copy (simulating "enchanted artifact" as best artifact).
	var target *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if gameengine.IsArtifactOnly(p) || hasType(p.Card.Types, "artifact") {
			if target == nil || gameengine.ManaCostOf(p.Card) > gameengine.ManaCostOf(target.Card) {
				target = p
			}
		}
	}
	if target == nil {
		return
	}

	// Create token copy.
	gameengine.CreateCreatureToken(gs, perm.Controller,
		target.Card.DisplayName(), target.Card.Types,
		target.Card.BasePower, target.Card.BaseToughness)

	// Check win condition: 8+ artifacts with the same name.
	nameCounts := map[string]int{}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if gameengine.IsArtifactOnly(p) || hasType(p.Card.Types, "artifact") {
			nameCounts[p.Card.DisplayName()]++
		}
	}
	for name, count := range nameCounts {
		if count >= 8 {
			emitWin(gs, perm.Controller, "mechanized_production_win", "Mechanized Production",
				"controlled 8+ artifacts named "+name)
			return
		}
	}

	emit(gs, "mechanized_production_trigger", "Mechanized Production", map[string]interface{}{
		"seat":   perm.Controller,
		"copied": target.Card.DisplayName(),
	})
}


// ---------------------------------------------------------------------------
// Roaming Throne
//
// "As Roaming Throne enters the battlefield, choose a creature type."
// "If a triggered ability of a creature you control of the chosen type
// triggers, it triggers an additional time."
// Simplified: stamp the chosen type flag. Trigger doubling requires
// engine-level support we don't have yet.
// ---------------------------------------------------------------------------

func registerRoamingThrone(r *Registry) {
	r.OnETB("Roaming Throne", roamingThroneETB)
}

func roamingThroneETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "roaming_throne_etb", "Roaming Throne", map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, "roaming_throne", "Roaming Throne", "trigger doubling not yet implemented")
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func init() {
	r := Global()
	registerBraidOfFire(r)
	registerRevelInRiches(r)
	registerSphinxOfTheSecondSun(r)
	registerShadowOfTheSecondSun(r)
	registerDragonmasterOutcast(r)
	registerBraidsConjurerAdept(r)
	registerAsForetold(r)
	registerCourtOfEmbereth(r)
	registerCourtOfVantress(r)
	registerSkylineDespot(r)
	registerExtravagantReplication(r)
	registerMechanizedProduction(r)
	registerRoamingThrone(r)
}
