package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch #16 — Tribal lords and cross-deck support cards.
//
// Implements lords (static P/T buffs), tribal token generators, cost
// reducers, and tribal payoff triggers. These are the backbone of
// tribal strategies — without them tribal decks have no synergy engine.

// ---------------------------------------------------------------------------
// Rooftop Storm
//
// "You may pay {0} rather than pay the mana cost for Zombie creature
// spells you cast."
// Implementation: cost modifier that zeroes out zombie creature spell costs.
// ---------------------------------------------------------------------------

func registerRooftopStorm(r *Registry) {
	r.OnETB("Rooftop Storm", rooftopStormETB)
}

func rooftopStormETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["rooftop_storm_seat"] = perm.Controller + 1
	emit(gs, "rooftop_storm_etb", "Rooftop Storm", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "zombie_creatures_cost_zero",
	})
}

// ---------------------------------------------------------------------------
// Endless Ranks of the Dead
//
// "At the beginning of your upkeep, create X 2/2 black Zombie creature
// tokens, where X is half the number of Zombies you control, rounded
// down."
// ---------------------------------------------------------------------------

func registerEndlessRanks(r *Registry) {
	r.OnTrigger("Endless Ranks of the Dead", "upkeep_controller", endlessRanksUpkeep)
}

func endlessRanksUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	zombies := countCreatureType(gs, perm.Controller, "zombie")
	n := zombies / 2
	if n <= 0 {
		return
	}
	for i := 0; i < n; i++ {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie",
			[]string{"creature", "zombie"}, 2, 2)
	}
	emit(gs, "endless_ranks_trigger", "Endless Ranks of the Dead", map[string]interface{}{
		"seat":    perm.Controller,
		"zombies": zombies,
		"created": n,
	})
}

// ---------------------------------------------------------------------------
// Death Baron
//
// "Skeletons you control and other Zombies you control get +1/+1 and
// have deathtouch."
// Implementation: static buff via Modifications on all zombies on ETB.
// Limitations: doesn't track new zombies entering after ETB.
// ---------------------------------------------------------------------------

func registerDeathBaron(r *Registry) {
	r.OnETB("Death Baron", deathBaronETB)
}

func deathBaronETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	applyTribalBuff(gs, perm, "zombie", 1, 1, "Death Baron")
}

// ---------------------------------------------------------------------------
// Lord of the Accursed
//
// "Other Zombies you control get +1/+1."
// ---------------------------------------------------------------------------

func registerLordOfTheAccursed(r *Registry) {
	r.OnETB("Lord of the Accursed", lordOfTheAccursedETB)
}

func lordOfTheAccursedETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	applyTribalBuff(gs, perm, "zombie", 1, 1, "Lord of the Accursed")
}

// ---------------------------------------------------------------------------
// Undead Warchief
//
// "Other Zombie creatures you control get +2/+1."
// "Zombie spells you cast cost {1} less to cast."
// ---------------------------------------------------------------------------

func registerUndeadWarchief(r *Registry) {
	r.OnETB("Undead Warchief", undeadWarchiefETB)
}

func undeadWarchiefETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	applyTribalBuff(gs, perm, "zombie", 2, 1, "Undead Warchief")
}

// ---------------------------------------------------------------------------
// Diregraf Captain
//
// "Other Zombie creatures you control get +1/+1."
// ---------------------------------------------------------------------------

func registerDiregrafCaptain(r *Registry) {
	r.OnETB("Diregraf Captain", diregrafCaptainETB)
}

func diregrafCaptainETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	applyTribalBuff(gs, perm, "zombie", 1, 1, "Diregraf Captain")
}

// ---------------------------------------------------------------------------
// Diregraf Colossus
//
// "Diregraf Colossus enters the battlefield with a +1/+1 counter on it
// for each Zombie card in your graveyard."
// "Whenever you cast a Zombie creature spell, create a tapped 2/2 black
// Zombie creature token."
// ---------------------------------------------------------------------------

func registerDiregrafColossus(r *Registry) {
	r.OnETB("Diregraf Colossus", diregrafColossusETB)
}

func diregrafColossusETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	zombiesInGY := 0
	for _, c := range seat.Graveyard {
		if c != nil && isZombieCard(c) {
			zombiesInGY++
		}
	}
	if zombiesInGY > 0 {
		if perm.Counters == nil {
			perm.Counters = map[string]int{}
		}
		perm.Counters["+1/+1"] += zombiesInGY
	}
	emit(gs, "diregraf_colossus_etb", "Diregraf Colossus", map[string]interface{}{
		"seat":     perm.Controller,
		"counters": zombiesInGY,
	})
}

// ---------------------------------------------------------------------------
// Wayward Servant
//
// "Whenever another Zombie enters the battlefield under your control,
// each opponent loses 1 life and you gain 1 life."
// Implementation: ETB flag — engine checks in CreateCreatureToken flow.
// For now: stamp flag, checked post-cast in FireCastTriggerObservers.
// ---------------------------------------------------------------------------

func registerWaywardServant(r *Registry) {
	r.OnETB("Wayward Servant", waywardServantETB)
	r.OnTrigger("Wayward Servant", "token_created", waywardServantTokenCreated)
	r.OnTrigger("Wayward Servant", "permanent_etb", waywardServantPermanentETB)
}

func waywardServantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "wayward_servant_etb", "Wayward Servant", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "zombie_etb_drain",
	})
}

// waywardServantTokenCreated — "Whenever another Zombie enters the
// battlefield under your control, each opponent loses 1 life and you
// gain 1 life." Fires on token_created for Zombie tokens.
func waywardServantTokenCreated(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}
	// Check if the token is a Zombie.
	types, _ := ctx["types"].([]string)
	if !typesContain(types, "zombie") {
		return
	}
	waywardServantDrain(gs, seat)
}

// waywardServantPermanentETB — fires on permanent_etb for non-token
// Zombie creatures entering under controller.
func waywardServantPermanentETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	etbSeat, _ := ctx["controller_seat"].(int)
	if etbSeat != seat {
		return
	}
	// Check if the entering permanent is a Zombie creature.
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil || enteringPerm == perm || enteringPerm.Card == nil {
		return
	}
	if !enteringPerm.IsCreature() {
		return
	}
	isZombie := false
	for _, t := range enteringPerm.Card.Types {
		if strings.EqualFold(t, "zombie") {
			isZombie = true
			break
		}
	}
	if !isZombie {
		return
	}
	waywardServantDrain(gs, seat)
}

func waywardServantDrain(gs *gameengine.GameState, seat int) {
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for _, opp := range gs.Opponents(seat) {
		if opp < 0 || opp >= len(gs.Seats) {
			continue
		}
		gs.Seats[opp].Life--
	}
	gameengine.GainLife(gs, seat, 1, "Wayward Servant")
	emit(gs, "wayward_servant_drain", "Wayward Servant", map[string]interface{}{
		"seat": seat,
	})
}

// typesContain checks if a string slice contains a type (case-insensitive).
func typesContain(types []string, want string) bool {
	for _, t := range types {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Coat of Arms
//
// "Each creature gets +1/+1 for each other creature on the battlefield
// that shares at least one creature type with it."
// Implementation: stamp flag. The actual P/T calculation would require
// characteristics-cache integration we don't have, so log partial.
// ---------------------------------------------------------------------------

func registerCoatOfArms(r *Registry) {
	r.OnETB("Coat of Arms", coatOfArmsETB)
}

func coatOfArmsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["coat_of_arms_active"] = 1
	// Register a layer 7c continuous effect for Coat of Arms.
	// "Each creature gets +1/+1 for each other creature on the battlefield
	// that shares at least one creature type with it."
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "c",
		SourcePerm:     perm,
		SourceCardName: "Coat of Arms",
		ControllerSeat: perm.Controller,
		HandlerID:      "coat_of_arms_" + itoa(perm.Controller),
		Duration:       gameengine.DurationPermanent,
		Predicate: func(_ *gameengine.GameState, target *gameengine.Permanent) bool {
			return target != nil && target.IsCreature()
		},
		ApplyFn: func(g *gameengine.GameState, target *gameengine.Permanent, chars *gameengine.Characteristics) {
			if g == nil || target == nil || target.Card == nil {
				return
			}
			bonus := coatOfArmsBonus(g, target)
			chars.Power += bonus
			chars.Toughness += bonus
		},
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, "coat_of_arms_etb", "Coat of Arms", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "layer_7c_shared_type_buff",
	})
}

// coatOfArmsBonus counts how many other creatures on the battlefield
// share at least one creature type with the target.
func coatOfArmsBonus(gs *gameengine.GameState, target *gameengine.Permanent) int {
	if gs == nil || target == nil || target.Card == nil {
		return 0
	}
	// Collect target's creature subtypes.
	targetTypes := map[string]bool{}
	for _, t := range target.Card.Types {
		lower := strings.ToLower(t)
		// Skip non-creature types.
		if lower == "creature" || lower == "artifact" || lower == "enchantment" ||
			lower == "land" || lower == "planeswalker" || lower == "battle" ||
			lower == "token" || lower == "legendary" || lower == "snow" ||
			lower == "basic" || lower == "world" || strings.HasPrefix(lower, "pip:") ||
			strings.HasPrefix(lower, "cmc:") {
			continue
		}
		targetTypes[lower] = true
	}
	if len(targetTypes) == 0 {
		return 0
	}
	count := 0
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p == target || !p.IsCreature() || p.Card == nil {
				continue
			}
			for _, t := range p.Card.Types {
				if targetTypes[strings.ToLower(t)] {
					count++
					break // each creature counted at most once
				}
			}
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Door of Destinies
//
// "As Door of Destinies enters the battlefield, choose a creature type."
// "Whenever you cast a spell of the chosen type, put a charge counter
// on Door of Destinies."
// "Creatures you control of the chosen type get +1/+1 for each charge
// counter on Door of Destinies."
// Simplified: increment charge counters on cast.
// ---------------------------------------------------------------------------

func registerDoorOfDestinies(r *Registry) {
	r.OnETB("Door of Destinies", doorOfDestiniesETB)
	r.OnTrigger("Door of Destinies", "spell_cast", doorOfDestiniesSpellCast)
}

func doorOfDestiniesETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	emit(gs, "door_of_destinies_etb", "Door of Destinies", map[string]interface{}{
		"seat": perm.Controller,
	})
}

func doorOfDestiniesSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["charge"]++
	emit(gs, "door_of_destinies_charge", "Door of Destinies", map[string]interface{}{
		"seat":   perm.Controller,
		"charge": perm.Counters["charge"],
	})
}

// ---------------------------------------------------------------------------
// Sai, Master Thopterist
//
// "Whenever you cast an artifact spell, create a 1/1 colorless Thopter
// artifact creature token with flying."
// ---------------------------------------------------------------------------

func registerSaiMasterThopterist(r *Registry) {
	r.OnETB("Sai, Master Thopterist", saiETB)
}

func saiETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "sai_etb", "Sai, Master Thopterist", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "artifact_cast_creates_thopter",
	})
	emitPartial(gs, "sai", "Sai, Master Thopterist", "thopter creation requires cast observer hook for artifacts")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func countCreatureType(gs *gameengine.GameState, seatIdx int, creatureType string) int {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	n := 0
	lower := strings.ToLower(creatureType)
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.ToLower(t) == lower {
				n++
				break
			}
		}
	}
	return n
}

func isZombieCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "zombie") {
			return true
		}
	}
	tl := strings.ToLower(c.TypeLine)
	return strings.Contains(tl, "zombie")
}

func applyTribalBuff(gs *gameengine.GameState, lord *gameengine.Permanent, creatureType string, power, toughness int, source string) {
	if gs == nil || lord == nil {
		return
	}
	seat := gs.Seats[lord.Controller]
	if seat == nil {
		return
	}
	lower := strings.ToLower(creatureType)
	buffed := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == lord || p.Card == nil || !p.IsCreature() {
			continue
		}
		isType := false
		for _, t := range p.Card.Types {
			if strings.ToLower(t) == lower {
				isType = true
				break
			}
		}
		if !isType {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     power,
			Toughness: toughness,
			Duration:  "while_source_on_battlefield",
			Timestamp: gs.NextTimestamp(),
		})
		buffed++
	}
	if buffed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, strings.ReplaceAll(strings.ToLower(source), " ", "_")+"_buff", source, map[string]interface{}{
		"seat":    lord.Controller,
		"type":    creatureType,
		"buffed":  buffed,
		"power":   power,
		"toughness": toughness,
	})
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func init() {
	r := Global()
	registerRooftopStorm(r)
	registerEndlessRanks(r)
	registerDeathBaron(r)
	registerLordOfTheAccursed(r)
	registerUndeadWarchief(r)
	registerDiregrafCaptain(r)
	registerDiregrafColossus(r)
	registerWaywardServant(r)
	registerCoatOfArms(r)
	registerDoorOfDestinies(r)
	registerSaiMasterThopterist(r)
}
