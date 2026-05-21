package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// counter_accumulator_r58 — four ports through the R58 counter-
// accumulator-threshold primitive. Each card has the shape "permanent
// accumulates counters via a trigger or activation; at the beginning
// of its controller's upkeep, if it has >= N counters of a specified
// type, [effect]."
//
// Picks:
//
//   1. Helix Pinnacle       — tower counters; 100+ → controller wins
//   2. Darksteel Reactor    — charge counters; 20+ → controller wins
//   3. Azor's Elocutors     — filibuster counters; 5+ → controller wins
//                             (with spell-cast removal trigger)
//   4. Quest for Ula's Temple — quest counters; 3+ → recurring upkeep
//                               put-Kraken-from-hand
//
// The first three are one-shot (Repeatable=false); the fourth is
// recurring (Repeatable=true) per its printed "At the beginning of
// each of your upkeeps" wording.

// ---------------------------------------------------------------------
// Helix Pinnacle — {G}, Enchantment
//
// Oracle text (Scryfall, verified):
//
//   {X}, {T}: Put X tower counters on Helix Pinnacle.
//   Helix Pinnacle has shroud as long as it has fewer than 100 tower
//   counters on it. (It can't be the target of spells or abilities.)
//   At the beginning of your upkeep, if Helix Pinnacle has 100 or more
//   tower counters on it, you win the game.
//
// Implementation:
//   - ETB registers a one-shot CounterThresholdEffect (Counter="tower",
//     Threshold=100) whose OnReach calls emitWin for the controller.
//   - Activated ability ({X}, {T}): pays X mana + taps, adds X tower
//     counters. (ctx["x"] = mana to pour; fall back to the controller's
//     mana pool minus 1 as a reasonable default for headless tests.)
//   - upkeep_controller trigger calls gs.EvaluateCounterThresholds()
//     so the win check fires at the printed timing.
//   - LTB unregisters.
//   - Shroud-while-under-100 rider: stamp/clear perm.Flags["kw:shroud"]
//     based on counter count at every relevant tick (ETB / activation /
//     upkeep). True continuous evaluation is engine territory; the
//     coarse refresh covers the common case.
// ---------------------------------------------------------------------

func registerHelixPinnacle(r *Registry) {
	r.OnETB("Helix Pinnacle", helixPinnacleETB)
	r.OnActivated("Helix Pinnacle", helixPinnacleActivate)
	r.OnTrigger("Helix Pinnacle", "upkeep_controller", helixPinnacleUpkeep)
	r.OnTrigger("Helix Pinnacle", "permanent_ltb", helixPinnacleLTB)
}

func helixPinnacleETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "helix_pinnacle_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller
	gs.RegisterCounterThreshold(&gameengine.CounterThresholdEffect{
		SourcePerm: perm,
		HandlerID:  "helix_pinnacle_win_at_100_tower",
		Counter:    "tower",
		Threshold:  100,
		Repeatable: false,
		OnReach: func(gs *gameengine.GameState, p *gameengine.Permanent) {
			if p == nil || p.Card == nil {
				return
			}
			emitWin(gs, controller, "helix_pinnacle_win_100_tower",
				p.Card.DisplayName(), "helix_pinnacle_100_tower")
		},
	})
	helixPinnacleRefreshShroud(perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       controller,
		"threshold":  100,
		"counter":    "tower",
		"shroud_set": perm.Flags["kw:shroud"],
	})
}

func helixPinnacleActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "helix_pinnacle_pour_x"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	x := 0
	if ctx != nil {
		if v, ok := ctx["x"].(int); ok {
			x = v
		}
	}
	if x <= 0 {
		// Default: pour everything but 1 (keep a reserve).
		x = seat.ManaPool - 1
		if x < 0 {
			x = 0
		}
	}
	if x == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_x_chosen", map[string]interface{}{
			"mana_pool": seat.ManaPool,
		})
		return
	}
	if seat.ManaPool < x {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"x": x, "mana_pool": seat.ManaPool,
		})
		return
	}
	seat.ManaPool -= x
	gameengine.SyncManaAfterSpend(seat)
	src.Tapped = true
	src.AddCounter("tower", x)
	helixPinnacleRefreshShroud(src)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seatIdx,
		"x":             x,
		"tower_counters": gameengine.CounterThresholdCount(src, "tower"),
		"shroud":        src.Flags["kw:shroud"],
	})
}

func helixPinnacleUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	helixPinnacleRefreshShroud(perm)
	gs.EvaluateCounterThresholds()
}

func helixPinnacleLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterCounterThresholdsForPermanent(perm)
}

func helixPinnacleRefreshShroud(perm *gameengine.Permanent) {
	if perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if gameengine.CounterThresholdCount(perm, "tower") < 100 {
		perm.Flags["kw:shroud"] = 1
	} else {
		delete(perm.Flags, "kw:shroud")
	}
}

// ---------------------------------------------------------------------
// Darksteel Reactor — {4}, Legendary Artifact
//
// Oracle text (Scryfall, verified):
//
//   Darksteel Reactor enters with a charge counter on it.
//   At the beginning of your upkeep, you may put a charge counter on
//   Darksteel Reactor.
//   At the beginning of your upkeep, if Darksteel Reactor has twenty
//   or more charge counters on it, you win the game.
//
// Implementation:
//   - ETB: place 1 charge counter immediately (the "enters with" rider)
//     + register the threshold (Counter="charge", Threshold=20).
//   - upkeep_controller trigger gated on active_seat == controller:
//     bump charge by 1 (the "may" choice is always-yes for the AI —
//     the only downside is winning later instead of sooner), then
//     evaluate.
//   - LTB unregisters.
//   - Indestructible (the "Darksteel" rider in the printed cycle) is
//     AST keyword pipeline territory; not surfaced here.
// ---------------------------------------------------------------------

func registerDarksteelReactor(r *Registry) {
	r.OnETB("Darksteel Reactor", darksteelReactorETB)
	r.OnTrigger("Darksteel Reactor", "upkeep_controller", darksteelReactorUpkeep)
	r.OnTrigger("Darksteel Reactor", "permanent_ltb", darksteelReactorLTB)
}

func darksteelReactorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "darksteel_reactor_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	perm.AddCounter("charge", 1)
	controller := perm.Controller
	gs.RegisterCounterThreshold(&gameengine.CounterThresholdEffect{
		SourcePerm: perm,
		HandlerID:  "darksteel_reactor_win_at_20_charge",
		Counter:    "charge",
		Threshold:  20,
		Repeatable: false,
		OnReach: func(gs *gameengine.GameState, p *gameengine.Permanent) {
			if p == nil || p.Card == nil {
				return
			}
			emitWin(gs, controller, "darksteel_reactor_win_20_charge",
				p.Card.DisplayName(), "darksteel_reactor_20_charge")
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     controller,
		"counters": gameengine.CounterThresholdCount(perm, "charge"),
	})
}

func darksteelReactorUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "darksteel_reactor_upkeep_charge"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	perm.AddCounter("charge", 1)
	gs.EvaluateCounterThresholds()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": gameengine.CounterThresholdCount(perm, "charge"),
	})
}

func darksteelReactorLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterCounterThresholdsForPermanent(perm)
}

// ---------------------------------------------------------------------
// Azor's Elocutors — {3}{W}{U}, Creature — Human Advisor
//
// Oracle text (Scryfall, verified):
//
//   At the beginning of your upkeep, put a filibuster counter on
//   Azor's Elocutors. Then if Azor's Elocutors has five or more
//   filibuster counters on it, you win the game.
//   Whenever a source deals damage to you, remove all filibuster
//   counters from Azor's Elocutors.
//
// Implementation:
//   - ETB registers the threshold (Counter="filibuster", Threshold=5).
//   - upkeep_controller trigger gated on controller: +1 filibuster
//     counter, then EvaluateCounterThresholds (printed text: "then
//     if … five or more" is the same evaluation tick).
//   - "life_lost" trigger gated on damaged seat == controller AND
//     amount > 0: removes all filibuster counters from Azor's
//     (the printed text says "damage to you"; HexDek's nearest
//     event is life_lost — combat damage and noncombat damage both
//     surface here per state.go's DealDamage path).
//   - LTB unregisters.
// ---------------------------------------------------------------------

func registerAzorsElocutors(r *Registry) {
	r.OnETB("Azor's Elocutors", azorsElocutorsETB)
	r.OnTrigger("Azor's Elocutors", "upkeep_controller", azorsElocutorsUpkeep)
	r.OnTrigger("Azor's Elocutors", "life_lost", azorsElocutorsDamageReset)
	r.OnTrigger("Azor's Elocutors", "permanent_ltb", azorsElocutorsLTB)
}

func azorsElocutorsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "azors_elocutors_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller
	gs.RegisterCounterThreshold(&gameengine.CounterThresholdEffect{
		SourcePerm: perm,
		HandlerID:  "azors_elocutors_win_at_5_filibuster",
		Counter:    "filibuster",
		Threshold:  5,
		Repeatable: false,
		OnReach: func(gs *gameengine.GameState, p *gameengine.Permanent) {
			if p == nil || p.Card == nil {
				return
			}
			emitWin(gs, controller, "azors_elocutors_win_5_filibuster",
				p.Card.DisplayName(), "azors_elocutors_5_filibuster")
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      controller,
		"threshold": 5,
	})
}

func azorsElocutorsUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "azors_elocutors_upkeep_filibuster"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	perm.AddCounter("filibuster", 1)
	gs.EvaluateCounterThresholds()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": gameengine.CounterThresholdCount(perm, "filibuster"),
	})
}

func azorsElocutorsDamageReset(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "azors_elocutors_damage_reset"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	damagedSeat, _ := ctx["seat"].(int)
	if damagedSeat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	prev := gameengine.CounterThresholdCount(perm, "filibuster")
	if prev > 0 && perm.Counters != nil {
		delete(perm.Counters, "filibuster")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"removed": prev,
	})
}

func azorsElocutorsLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterCounterThresholdsForPermanent(perm)
}

// ---------------------------------------------------------------------
// Quest for Ula's Temple — {U}, Enchantment
//
// Oracle text (Scryfall, verified — Worldwake):
//
//   At the beginning of each of your upkeeps, you may put a quest
//   counter on Quest for Ula's Temple.
//   At the beginning of each of your upkeeps, if there are three or
//   more quest counters on Quest for Ula's Temple, you may put a
//   Kraken, Leviathan, Octopus, or Serpent creature card from your
//   hand onto the battlefield.
//
// Implementation:
//   - ETB registers the threshold (Counter="quest", Threshold=3,
//     Repeatable=true). The OnReach scans the controller's hand for
//     a Kraken/Leviathan/Octopus/Serpent creature card; if found,
//     puts it onto the battlefield via the standard ETB-from-hand
//     pipeline.
//   - upkeep_controller trigger: +1 quest counter (the "may" is
//     always-yes for the AI policy — moving toward threshold is
//     strictly positive), then EvaluateCounterThresholds (Repeatable
//     => OnReach can fire every upkeep while at threshold).
//   - LTB unregisters.
// ---------------------------------------------------------------------

func registerQuestForUlasTemple(r *Registry) {
	r.OnETB("Quest for Ula's Temple", questForUlasTempleETB)
	r.OnTrigger("Quest for Ula's Temple", "upkeep_controller", questForUlasTempleUpkeep)
	r.OnTrigger("Quest for Ula's Temple", "permanent_ltb", questForUlasTempleLTB)
}

func questForUlasTempleETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "quest_for_ulas_temple_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller
	gs.RegisterCounterThreshold(&gameengine.CounterThresholdEffect{
		SourcePerm: perm,
		HandlerID:  "quest_for_ulas_temple_kraken_cheat",
		Counter:    "quest",
		Threshold:  3,
		Repeatable: true,
		OnReach: func(gs *gameengine.GameState, p *gameengine.Permanent) {
			if p == nil || p.Card == nil {
				return
			}
			seat := gs.Seats[controller]
			if seat == nil || len(seat.Hand) == 0 {
				return
			}
			// Pick the highest-CMC qualifying creature from hand
			// (most value to cheat into play). Eligibility: creature
			// with subtype Kraken / Leviathan / Octopus / Serpent.
			pickIdx := -1
			bestCMC := -1
			for i, c := range seat.Hand {
				if c == nil {
					continue
				}
				if !cardHasType(c, "creature") {
					continue
				}
				if !questForUlasTempleEligibleCreature(c) {
					continue
				}
				cmc := cardCMC(c)
				if cmc > bestCMC {
					bestCMC = cmc
					pickIdx = i
				}
			}
			if pickIdx < 0 {
				return
			}
			pick := seat.Hand[pickIdx]
			gameengine.MoveCard(gs, pick, controller, "hand", "battlefield",
				"quest_for_ulas_temple_cheat")
			emit(gs, "quest_for_ulas_temple_cheat", p.Card.DisplayName(),
				map[string]interface{}{
					"seat":  controller,
					"card":  pick.DisplayName(),
					"cmc":   bestCMC,
				})
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      controller,
		"threshold": 3,
	})
}

func questForUlasTempleEligibleCreature(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		switch t {
		case "kraken", "leviathan", "octopus", "serpent":
			return true
		}
	}
	return false
}

func questForUlasTempleUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "quest_for_ulas_temple_upkeep"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	perm.AddCounter("quest", 1)
	gs.EvaluateCounterThresholds()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": gameengine.CounterThresholdCount(perm, "quest"),
	})
}

func questForUlasTempleLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterCounterThresholdsForPermanent(perm)
}
