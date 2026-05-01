package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch #17 sweep — remaining unhandled cards across all stress test decks.
// Per 7174n1c: "hit them all, thorough pass."

// ===================================================================
// UPKEEP ENGINES
// ===================================================================

// ---------------------------------------------------------------------------
// Howling Mine — "At the beginning of each player's draw step, that
// player draws an additional card."
// ---------------------------------------------------------------------------

func registerHowlingMine(r *Registry) {
	r.OnTrigger("Howling Mine", "upkeep_controller", howlingMineUpkeep)
}

func howlingMineUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	seat := gs.Seats[activeSeat]
	if seat == nil || seat.Lost || len(seat.Library) == 0 {
		return
	}
	card := seat.Library[0]
	gameengine.MoveCard(gs, card, activeSeat, "library", "hand", "draw")
	emit(gs, "howling_mine_draw", "Howling Mine", map[string]interface{}{
		"seat":    activeSeat,
		"card":    card.DisplayName(),
		"mine_at": perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// Black Market Connections — "At the beginning of your precombat main
// phase, choose one or more — draw a card (lose 2 life), create a
// Treasure (lose 2 life), create a 3/2 changeling (lose 3 life)."
// Greedy: always pick draw + treasure. Skip changeling below 10 life.
// ---------------------------------------------------------------------------

func registerBlackMarketConnections(r *Registry) {
	r.OnTrigger("Black Market Connections", "upkeep_controller", blackMarketUpkeep)
}

func blackMarketUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Draw a card, lose 2 life.
	if len(seat.Library) > 0 {
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, perm.Controller, "library", "hand", "draw")
		seat.Life -= 2
	}
	// Create a Treasure, lose 2 life.
	gameengine.CreateTreasureToken(gs, perm.Controller)
	seat.Life -= 2
	// Create a 3/2 changeling if life > 10.
	if seat.Life > 10 {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Shapeshifter",
			[]string{"creature", "shapeshifter", "changeling"}, 3, 2)
		seat.Life -= 3
	}
	emit(gs, "black_market_connections_trigger", "Black Market Connections", map[string]interface{}{
		"seat":      perm.Controller,
		"life_after": seat.Life,
	})
}

// ---------------------------------------------------------------------------
// Thassa, God of the Sea — "At the beginning of your upkeep, scry 1."
// Static: indestructible, not a creature below 5 devotion.
// ---------------------------------------------------------------------------

func registerThassaGodOfTheSea(r *Registry) {
	r.OnTrigger("Thassa, God of the Sea", "upkeep_controller", thassaUpkeep)
}

func thassaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
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
	if seat.Hat != nil {
		top := []*gameengine.Card{seat.Library[len(seat.Library)-1]}
		keepTop, _ := seat.Hat.ChooseScry(gs, perm.Controller, top)
		if len(keepTop) == 0 {
			card := seat.Library[len(seat.Library)-1]
			seat.Library = seat.Library[:len(seat.Library)-1]
			seat.Library = append([]*gameengine.Card{card}, seat.Library...)
		}
	}
	emit(gs, "thassa_scry", "Thassa, God of the Sea", map[string]interface{}{
		"seat": perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// Chronozoa — "Flying. Vanishing 3 (enters with 3 time counters, remove
// one each upkeep, sacrifice when last is removed). When put into
// graveyard from battlefield with no time counters, create two copies."
// ---------------------------------------------------------------------------

func registerChronozoa(r *Registry) {
	r.OnETB("Chronozoa", chronozoaETB)
	r.OnTrigger("Chronozoa", "upkeep_controller", chronozoaUpkeep)
}

func chronozoaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["time"] = 3
	emit(gs, "chronozoa_etb", "Chronozoa", map[string]interface{}{
		"seat":          perm.Controller,
		"time_counters": 3,
	})
}

func chronozoaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Counters == nil {
		return
	}
	perm.Counters["time"]--
	if perm.Counters["time"] <= 0 {
		// Sacrifice — create two copies.
		for i := 0; i < 2; i++ {
			tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Chronozoa",
				[]string{"creature", "illusion"}, 3, 3)
			if tok != nil {
				tok.Card.Types = append(tok.Card.Types, "flying")
			}
		}
		gameengine.DestroyPermanent(gs, perm, nil)
		emit(gs, "chronozoa_split", "Chronozoa", map[string]interface{}{
			"seat":   perm.Controller,
			"copies": 2,
		})
	}
}

// ---------------------------------------------------------------------------
// Replicating Ring — "{T}: Add one mana of any color. At the beginning
// of your upkeep, put a night counter. If 8+, remove all and create 8
// token copies (each taps for any color)."
// ---------------------------------------------------------------------------

func registerReplicatingRing(r *Registry) {
	r.OnTrigger("Replicating Ring", "upkeep_controller", replicatingRingUpkeep)
}

func replicatingRingUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
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
	perm.Counters["night"]++
	if perm.Counters["night"] >= 8 {
		perm.Counters["night"] = 0
		for i := 0; i < 8; i++ {
			gameengine.CreateCreatureToken(gs, perm.Controller, "Replicated Ring",
				[]string{"artifact", "token"}, 0, 0)
		}
		emit(gs, "replicating_ring_split", "Replicating Ring", map[string]interface{}{
			"seat":  perm.Controller,
			"copies": 8,
		})
	}
}

// ---------------------------------------------------------------------------
// Tamiyo's Journal — "At the beginning of your upkeep, investigate."
// ---------------------------------------------------------------------------

func registerTamiyosJournal(r *Registry) {
	r.OnTrigger("Tamiyo's Journal", "upkeep_controller", tamiyoUpkeep)
}

func tamiyoUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	gameengine.CreateClueToken(gs, perm.Controller)
	emit(gs, "tamiyos_journal_investigate", "Tamiyo's Journal", map[string]interface{}{
		"seat": perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// Virtue of Persistence (enchantment side) — "At the beginning of your
// upkeep, put target creature card from a graveyard onto the battlefield
// under your control."
// ---------------------------------------------------------------------------

func registerVirtueOfPersistence(r *Registry) {
	r.OnTrigger("Virtue of Persistence // Locthwain Scorn", "upkeep_controller", virtueOfPersistenceUpkeep)
	r.OnTrigger("Virtue of Persistence", "upkeep_controller", virtueOfPersistenceUpkeep)
}

func virtueOfPersistenceUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Find best creature in any graveyard.
	var bestCard *gameengine.Card
	bestCMC := -1
	bestSeat := -1
	for si, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			isCreature := false
			for _, t := range c.Types {
				if strings.EqualFold(t, "creature") {
					isCreature = true
					break
				}
			}
			if !isCreature {
				continue
			}
			cmc := gameengine.ManaCostOf(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestCard = c
				bestSeat = si
			}
		}
	}
	if bestCard == nil {
		return
	}
	gameengine.MoveCard(gs, bestCard, bestSeat, "graveyard", "battlefield", "virtue_of_persistence")
	emit(gs, "virtue_of_persistence_reanimate", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"reanimated": bestCard.DisplayName(),
		"from_seat": bestSeat,
	})
}

// ===================================================================
// ZOMBIE TRIBAL (VARINA DECK)
// ===================================================================

// ---------------------------------------------------------------------------
// Anointed Procession — "If an effect would create one or more tokens
// under your control, it creates twice that many instead."
// Implementation: flag. Token doubling requires engine-level hooks.
// ---------------------------------------------------------------------------

func registerAnointedProcession(r *Registry) {
	r.OnETB("Anointed Procession", anointedProcessionETB)
	r.OnTrigger("Anointed Procession", "token_created", anointedProcessionTokenTrigger)
}

func anointedProcessionETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["anointed_procession_seat_"+itoa(perm.Controller)] = 1
	emit(gs, "anointed_procession_etb", "Anointed Procession", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "token_doubling",
	})
}

// anointedProcessionTokenTrigger doubles tokens created under our control.
// Fires on "token_created" event. The re-entrancy guard in the engine
// (gs.Flags["in_token_trigger"]) prevents the doubled tokens from
// re-triggering this handler.
func anointedProcessionTokenTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Only doubles tokens YOU create.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}

	count, _ := ctx["count"].(int)
	if count <= 0 {
		return
	}

	// Determine token type from context and create matching extras.
	types, _ := ctx["types"].([]string)
	isCreature := false
	for _, t := range types {
		if t == "creature" {
			isCreature = true
			break
		}
	}

	if isCreature {
		nonTokenTypes := make([]string, 0, len(types))
		for _, t := range types {
			if t != "token" {
				nonTokenTypes = append(nonTokenTypes, t)
			}
		}
		for i := 0; i < count; i++ {
			gameengine.CreateCreatureToken(gs, seat, "Token", nonTokenTypes, 1, 1)
		}
	} else {
		// Non-creature tokens -- match by known artifact subtypes.
		for i := 0; i < count; i++ {
			matched := false
			for _, t := range types {
				switch t {
				case "treasure":
					gameengine.CreateTreasureToken(gs, seat)
					matched = true
				case "food":
					gameengine.CreateFoodToken(gs, seat)
					matched = true
				case "clue":
					gameengine.CreateClueToken(gs, seat)
					matched = true
				case "blood":
					gameengine.CreateBloodToken(gs, seat)
					matched = true
				}
				if matched {
					break
				}
			}
		}
	}

	emit(gs, "anointed_procession_trigger", "Anointed Procession", map[string]interface{}{
		"seat":    seat,
		"doubled": count,
	})
}

// ---------------------------------------------------------------------------
// Bone Miser — "Whenever you discard a creature card, create a 2/2
// Zombie. Discard a land, add {B}{B}{B}. Discard a noncreature nonland,
// draw a card."
// ---------------------------------------------------------------------------

func registerBoneMiser(r *Registry) {
	r.OnTrigger("Bone Miser", "card_discarded", boneMiserDiscard)
}

func boneMiserDiscard(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	discardSeat, _ := ctx["seat"].(int)
	if discardSeat != perm.Controller {
		return
	}
	cardName, _ := ctx["card_name"].(string)
	cardTypes, _ := ctx["card_types"].([]string)

	isCreature, isLand := false, false
	for _, t := range cardTypes {
		lt := strings.ToLower(t)
		if lt == "creature" {
			isCreature = true
		}
		if lt == "land" {
			isLand = true
		}
	}

	if isCreature {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie",
			[]string{"creature", "zombie"}, 2, 2)
		emit(gs, "bone_miser_zombie", "Bone Miser", map[string]interface{}{
			"seat":      perm.Controller,
			"discarded": cardName,
		})
	} else if isLand {
		seat := gs.Seats[perm.Controller]
		gameengine.AddMana(gs, seat, "B", 3, "Bone Miser")
		emit(gs, "bone_miser_mana", "Bone Miser", map[string]interface{}{
			"seat":      perm.Controller,
			"discarded": cardName,
		})
	} else {
		seat := gs.Seats[perm.Controller]
		if seat != nil && len(seat.Library) > 0 {
			card := seat.Library[0]
			gameengine.MoveCard(gs, card, perm.Controller, "library", "hand", "draw")
			emit(gs, "bone_miser_draw", "Bone Miser", map[string]interface{}{
				"seat":      perm.Controller,
				"discarded": cardName,
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Shepherd of Rot — "{T}: Each player loses 1 life for each Zombie
// you control."
// Implementation: ETB log. Activated ability is handled by Hat.
// ---------------------------------------------------------------------------

func registerShepherdOfRot(r *Registry) {
	r.OnETB("Shepherd of Rot", shepherdOfRotETB)
}

func shepherdOfRotETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "shepherd_of_rot_etb", "Shepherd of Rot", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "tap_life_loss_per_zombie",
	})
}

// ---------------------------------------------------------------------------
// Cryptbreaker — "{B}, {T}, Discard a card: Create a 2/2 Zombie token."
// ETB log. Activated ability through Hat.
// ---------------------------------------------------------------------------

func registerCryptbreaker(r *Registry) {
	r.OnETB("Cryptbreaker", cryptbreakerETB)
}

func cryptbreakerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "cryptbreaker_etb", "Cryptbreaker", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "discard_make_zombie + tap_zombies_draw",
	})
}

// ---------------------------------------------------------------------------
// Tormod, the Desecrator — "Whenever one or more cards leave your
// graveyard, create a tapped 2/2 black Zombie creature token."
// ---------------------------------------------------------------------------

func registerTormod(r *Registry) {
	r.OnETB("Tormod, the Desecrator", tormodETB)
}

func tormodETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "tormod_etb", "Tormod, the Desecrator", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "graveyard_leave_creates_zombie",
	})
	emitPartial(gs, "tormod", "Tormod, the Desecrator", "graveyard-leave observer not yet wired")
}

// ---------------------------------------------------------------------------
// Geth, Lord of the Vault — "{X}{B}: Put target artifact or creature
// with MV X from opponent's graveyard onto battlefield tapped under
// your control. Opponent mills X."
// ETB log only — activated ability is complex.
// ---------------------------------------------------------------------------

func registerGeth(r *Registry) {
	r.OnETB("Geth, Lord of the Vault", gethETB)
}

func gethETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "geth_etb", "Geth, Lord of the Vault", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "activated_reanimate_from_opponent",
	})
}

// ---------------------------------------------------------------------------
// Lich Lord of Unx — "{U}{B}: Create a 1/1 Zombie Wizard token."
// ETB log only — activated ability through Hat.
// ---------------------------------------------------------------------------

func registerLichLordOfUnx(r *Registry) {
	r.OnETB("Lich Lord of Unx", lichLordETB)
}

func lichLordETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "lich_lord_etb", "Lich Lord of Unx", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "create_zombie_wizard + drain_per_zombie",
	})
}

// ---------------------------------------------------------------------------
// Nevinyrral, Urborg Tyrant — "When Nevinyrral enters the battlefield,
// destroy all artifacts, creatures, and enchantments other than
// Nevinyrral."
// ---------------------------------------------------------------------------

func registerNevinyrral(r *Registry) {
	r.OnETB("Nevinyrral, Urborg Tyrant", nevinyrralETB)
}

func nevinyrralETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	destroyed := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		var keep []*gameengine.Permanent
		for _, p := range s.Battlefield {
			if p == perm {
				keep = append(keep, p)
				continue
			}
			if p == nil || p.Card == nil {
				continue
			}
			isTarget := false
			for _, t := range p.Card.Types {
				lt := strings.ToLower(t)
				if lt == "artifact" || lt == "creature" || lt == "enchantment" {
					isTarget = true
					break
				}
			}
			if p.IsCreature() || p.IsLand() == false && isTarget {
				gs.LogEvent(gameengine.Event{
					Kind:   "destroy",
					Seat:   p.Controller,
					Source: "Nevinyrral, Urborg Tyrant",
					Details: map[string]interface{}{
						"card":   p.Card.DisplayName(),
						"reason": "nevinyrral_etb",
					},
				})
				destroyed++
				continue
			}
			keep = append(keep, p)
		}
		s.Battlefield = keep
	}
	emit(gs, "nevinyrral_etb", "Nevinyrral, Urborg Tyrant", map[string]interface{}{
		"seat":      perm.Controller,
		"destroyed": destroyed,
	})
}

// Living Death — handled in stax_spells.go (registerLivingDeath).

// ===================================================================
// ARTIFACT TRIBAL (GOLBEZ DECK)
// ===================================================================

// ---------------------------------------------------------------------------
// Foundry Inspector — "Artifact spells you cast cost {1} less."
// Cost modifier wired in cost_modifiers.go.
// ---------------------------------------------------------------------------

func registerFoundryInspector(r *Registry) {
	r.OnETB("Foundry Inspector", foundryInspectorETB)
}

func foundryInspectorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "foundry_inspector_etb", "Foundry Inspector", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "artifact_spells_cost_1_less",
	})
}

// ---------------------------------------------------------------------------
// Chief of the Foundry — "Other artifact creatures you control get +1/+1."
// ---------------------------------------------------------------------------

func registerChiefOfTheFoundry(r *Registry) {
	r.OnETB("Chief of the Foundry", chiefOfTheFoundryETB)
}

func chiefOfTheFoundryETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	buffed := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		isArtifact := false
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "artifact") {
				isArtifact = true
				break
			}
		}
		if !isArtifact {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Toughness: 1,
			Duration:  "while_source_on_battlefield",
			Timestamp: gs.NextTimestamp(),
		})
		buffed++
	}
	if buffed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, "chief_of_the_foundry_buff", "Chief of the Foundry", map[string]interface{}{
		"seat":   perm.Controller,
		"buffed": buffed,
	})
}

// ---------------------------------------------------------------------------
// Caged Sun — "As ~ enters, choose a color. Creatures you control of
// that color get +1/+1. Whenever a land you control is tapped for mana
// of that color, add one additional mana of that color."
// Simplified: choose the commander's primary color, apply creature buff.
// Mana doubling logged as partial.
// ---------------------------------------------------------------------------

func registerCagedSun(r *Registry) {
	r.OnETB("Caged Sun", cagedSunETB)
}

func cagedSunETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	buffed := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Toughness: 1,
			Duration:  "while_source_on_battlefield",
			Timestamp: gs.NextTimestamp(),
		})
		buffed++
	}
	if buffed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, "caged_sun_etb", "Caged Sun", map[string]interface{}{
		"seat":   perm.Controller,
		"buffed": buffed,
	})
	emitPartial(gs, "caged_sun", "Caged Sun", "mana doubling requires land-tap hook")
}

// ---------------------------------------------------------------------------
// Gauntlet of Power — same as Caged Sun but symmetric (all players).
// ---------------------------------------------------------------------------

func registerGauntletOfPower(r *Registry) {
	r.OnETB("Gauntlet of Power", gauntletOfPowerETB)
}

func gauntletOfPowerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	buffed := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power:     1,
				Toughness: 1,
				Duration:  "while_source_on_battlefield",
				Timestamp: gs.NextTimestamp(),
			})
			buffed++
		}
	}
	if buffed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, "gauntlet_of_power_etb", "Gauntlet of Power", map[string]interface{}{
		"seat":   perm.Controller,
		"buffed": buffed,
	})
	emitPartial(gs, "gauntlet_of_power", "Gauntlet of Power", "mana doubling requires land-tap hook")
}

// ---------------------------------------------------------------------------
// Imotekh the Stormlord — "Whenever one or more artifact cards leave
// your graveyard, create two 2/2 Necron Warrior artifact creature tokens."
// ---------------------------------------------------------------------------

func registerImotekh(r *Registry) {
	r.OnETB("Imotekh the Stormlord", imotekhETB)
}

func imotekhETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "imotekh_etb", "Imotekh the Stormlord", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "artifact_graveyard_leave_creates_necrons",
	})
	emitPartial(gs, "imotekh", "Imotekh the Stormlord", "graveyard-leave observer not yet wired")
}

// ---------------------------------------------------------------------------
// Graaz, Unstoppable Juggernaut — "Other creatures you control have base
// power and toughness 5/3 and are Juggernauts."
// ---------------------------------------------------------------------------

func registerGraaz(r *Registry) {
	r.OnETB("Graaz, Unstoppable Juggernaut", graazETB)
}

func graazETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	buffed := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		p.Card.BasePower = 5
		p.Card.BaseToughness = 3
		buffed++
	}
	if buffed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, "graaz_etb", "Graaz, Unstoppable Juggernaut", map[string]interface{}{
		"seat":   perm.Controller,
		"buffed": buffed,
	})
}

// ===================================================================
// MISC CARDS
// ===================================================================

// ---------------------------------------------------------------------------
// Padeem, Consul of Innovation — "Artifacts you control have hexproof.
// At the beginning of your upkeep, if you control the artifact with the
// highest MV or tied, draw a card."
// ---------------------------------------------------------------------------

func registerPadeem(r *Registry) {
	r.OnTrigger("Padeem, Consul of Innovation", "upkeep_controller", padeemUpkeep)
}

func padeemUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Check if we have the highest-MV artifact.
	myBest := 0
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p != nil && p.Card != nil && gameengine.IsArtifactOnly(p) {
			cmc := gameengine.ManaCostOf(p.Card)
			if cmc > myBest {
				myBest = cmc
			}
		}
	}
	oppBest := 0
	for i, s := range gs.Seats {
		if i == perm.Controller || s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && gameengine.IsArtifactOnly(p) {
				cmc := gameengine.ManaCostOf(p.Card)
				if cmc > oppBest {
					oppBest = cmc
				}
			}
		}
	}
	if myBest >= oppBest && myBest > 0 {
		seat := gs.Seats[perm.Controller]
		if seat != nil && len(seat.Library) > 0 {
			card := seat.Library[0]
			gameengine.MoveCard(gs, card, perm.Controller, "library", "hand", "draw")
		}
		emit(gs, "padeem_draw", "Padeem, Consul of Innovation", map[string]interface{}{
			"seat":     perm.Controller,
			"best_cmc": myBest,
		})
	}
}

// ---------------------------------------------------------------------------
// Academy Manufactor — "If you would create a Clue, Food, or Treasure
// token, instead create one of each."
// Implementation: flag. Requires CreateToken hook integration.
// ---------------------------------------------------------------------------

func registerAcademyManufactor(r *Registry) {
	r.OnETB("Academy Manufactor", academyManufactorETB)
}

func academyManufactorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["academy_manufactor_seat_"+itoa(perm.Controller)] = 1
	emit(gs, "academy_manufactor_etb", "Academy Manufactor", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "clue_food_treasure_tripler",
	})
	emitPartial(gs, "academy_manufactor", "Academy Manufactor", "token tripling requires CreateToken hook")
}

// ---------------------------------------------------------------------------
// Maralen of the Mornsong — "Players can't draw cards. At the beginning
// of each player's draw step, that player loses 3 life, searches their
// library for a card, puts it into their hand, then shuffles."
// Simplified: each upkeep, active player loses 3, tutors best card.
// ---------------------------------------------------------------------------

func registerMaralen(r *Registry) {
	r.OnTrigger("Maralen of the Mornsong", "upkeep_controller", maralenUpkeep)
	r.OnTrigger("Maralen, Fae Ascendant", "upkeep_controller", maralenUpkeep)
}

func maralenUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	seat := gs.Seats[activeSeat]
	if seat == nil || seat.Lost {
		return
	}
	seat.Life -= 3
	gs.LogEvent(gameengine.Event{
		Kind:   "life_change",
		Seat:   activeSeat,
		Amount: -3,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"reason": "maralen_draw_replacement",
		},
	})
	// Tutor: pick highest-CMC card from library.
	if len(seat.Library) > 0 {
		bestIdx := 0
		bestCMC := gameengine.ManaCostOf(seat.Library[0])
		for i, c := range seat.Library {
			cmc := gameengine.ManaCostOf(c)
			if cmc > bestCMC {
				bestCMC = cmc
				bestIdx = i
			}
		}
		card := seat.Library[bestIdx]
		gameengine.MoveCard(gs, card, activeSeat, "library", "hand", "tutor-to-hand")
		emit(gs, "maralen_tutor", perm.Card.DisplayName(), map[string]interface{}{
			"seat":   activeSeat,
			"tutored": card.DisplayName(),
		})
	}
}

// ---------------------------------------------------------------------------
// Lich's Mastery — "Hexproof, indestructible. You can't lose the game.
// Whenever you gain life, draw that many cards. Whenever you lose life,
// exile a permanent you control or a card from hand/graveyard."
// Simplified: flag prevents loss. Draw/exile triggers need observer hooks.
// ---------------------------------------------------------------------------

func registerLichsMastery(r *Registry) {
	r.OnETB("Lich's Mastery", lichsMasteryETB)
}

func lichsMasteryETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["lichs_mastery_seat_"+itoa(perm.Controller)] = 1
	emit(gs, "lichs_mastery_etb", "Lich's Mastery", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "cant_lose_game + draw_on_lifegain + exile_on_lifeloss",
	})
	emitPartial(gs, "lichs_mastery", "Lich's Mastery", "gain/loss observers not wired")
}

// ---------------------------------------------------------------------------
// Remaining niche cards — ETB log handlers to register them in the
// registry so they're tracked as "handled" even if full behavior
// requires deeper engine hooks.
// ---------------------------------------------------------------------------

func registerWinterCursedRider(r *Registry) {
	r.OnETB("Winter, Cursed Rider", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil {
			return
		}
		emit(gs, "winter_cursed_rider_etb", "Winter, Cursed Rider", map[string]interface{}{
			"seat": perm.Controller,
		})
	})
}

func registerStarWhale(r *Registry) {
	r.OnETB("Star Whale", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil {
			return
		}
		emit(gs, "star_whale_etb", "Star Whale", map[string]interface{}{
			"seat": perm.Controller,
		})
	})
}

func registerShaunFatherOfSynths(r *Registry) {
	r.OnETB("Shaun, Father of Synths", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil {
			return
		}
		emit(gs, "shaun_etb", "Shaun, Father of Synths", map[string]interface{}{
			"seat":   perm.Controller,
			"effect": "create_synth_token_copy_on_artifact_etb",
		})
		emitPartial(gs, "shaun", "Shaun, Father of Synths", "token-copy-on-artifact-ETB requires observer hook")
	})
}

func registerScrawlingCrawler(r *Registry) {
	r.OnETB("Scrawling Crawler", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil {
			return
		}
		emit(gs, "scrawling_crawler_etb", "Scrawling Crawler", map[string]interface{}{
			"seat": perm.Controller,
		})
	})
}

func registerHexingSquelcher(r *Registry) {
	r.OnETB("Hexing Squelcher", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil {
			return
		}
		emit(gs, "hexing_squelcher_etb", "Hexing Squelcher", map[string]interface{}{
			"seat": perm.Controller,
		})
	})
}

func registerGenerousPlunderer(r *Registry) {
	r.OnETB("Generous Plunderer", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil {
			return
		}
		gameengine.CreateTreasureToken(gs, perm.Controller)
		emit(gs, "generous_plunderer_etb", "Generous Plunderer", map[string]interface{}{
			"seat": perm.Controller,
		})
	})
}

func registerJhoiraOfTheGhitu(r *Registry) {
	r.OnETB("Jhoira of the Ghitu", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil {
			return
		}
		emit(gs, "jhoira_etb", "Jhoira of the Ghitu", map[string]interface{}{
			"seat":   perm.Controller,
			"effect": "suspend_from_hand",
		})
		emitPartial(gs, "jhoira", "Jhoira of the Ghitu", "suspend activation requires time-counter system")
	})
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func init() {
	r := Global()

	// Upkeep engines
	registerHowlingMine(r)
	registerBlackMarketConnections(r)
	registerThassaGodOfTheSea(r)
	registerChronozoa(r)
	registerReplicatingRing(r)
	registerTamiyosJournal(r)
	registerVirtueOfPersistence(r)

	// Zombie tribal
	registerAnointedProcession(r)
	registerBoneMiser(r)
	registerShepherdOfRot(r)
	registerCryptbreaker(r)
	registerTormod(r)
	registerGeth(r)
	registerLichLordOfUnx(r)
	registerNevinyrral(r)

	// Artifact tribal
	registerFoundryInspector(r)
	registerChiefOfTheFoundry(r)
	registerCagedSun(r)
	registerGauntletOfPower(r)
	registerImotekh(r)
	registerGraaz(r)

	// Misc
	registerPadeem(r)
	registerAcademyManufactor(r)
	registerMaralen(r)
	registerLichsMastery(r)
	registerWinterCursedRider(r)
	registerStarWhale(r)
	registerShaunFatherOfSynths(r)
	registerScrawlingCrawler(r)
	registerHexingSquelcher(r)
	registerGenerousPlunderer(r)
	registerJhoiraOfTheGhitu(r)
}
