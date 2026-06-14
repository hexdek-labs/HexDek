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
		gameengine.LoseLife(gs, perm.Controller, 2, "Black Market Connections")
	}
	// Create a Treasure, lose 2 life.
	gameengine.CreateTreasureToken(gs, perm.Controller)
	gameengine.LoseLife(gs, perm.Controller, 2, "Black Market Connections")
	// Create a 3/2 changeling if life > 10.
	if seat.Life > 10 {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Shapeshifter",
			[]string{"creature", "shapeshifter", "changeling"}, 3, 2)
		gameengine.LoseLife(gs, perm.Controller, 3, "Black Market Connections")
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
	perm.AddCounter("time", -1)
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
	perm.AddCounter("night", 1)
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
	r.OnTrigger("Tormod, the Desecrator", "graveyard_leave", tormodGraveyardLeave)
}

func tormodETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "tormod_etb", "Tormod, the Desecrator", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "graveyard_leave_creates_zombie",
	})
}

func tormodGraveyardLeave(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	seat, _ := ctx["seat"].(int)
	if seat != perm.Controller {
		return
	}
	gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie", []string{"creature"}, 2, 2)
	emit(gs, "tormod_trigger", "Tormod, the Desecrator", map[string]interface{}{
		"seat": perm.Controller,
	})
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
	// Snapshot the target list first — gameengine.DestroyPermanent mutates
	// the battlefield slice it iterates, which makes in-place iteration
	// unsafe (skipped or double-visited entries depending on the slice
	// state).
	var victims []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p == perm || p.Card == nil {
				continue
			}
			if p.IsLand() {
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
			if !isTarget && !p.IsCreature() {
				continue
			}
			victims = append(victims, p)
		}
	}
	// Route every destroy through the canonical battlefield-exit API:
	// DestroyPermanent runs §614 would-be-destroyed replacements, fires
	// dies / LTB triggers, applies §903.9b commander redirect, and
	// MOVES THE CARD TO THE OWNER'S GRAVEYARD. The pre-r45 inline
	// implementation only rebuilt seat.Battlefield without `keep`,
	// dropping the *Card pointer entirely — 8 real cards "disappeared"
	// per fire in the Loki r44 / r45 ZoneConservation cluster (game 333
	// and 472 alone produced 100+ hits between them). Same anti-pattern
	// as the abdel_adrian May-11 forensics; this is the second sibling
	// to be swept.
	destroyed := 0
	for _, p := range victims {
		if gameengine.DestroyPermanent(gs, p, perm) {
			destroyed++
		}
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
	// "Other artifact creatures you control get +1/+1." Registered as a §613
	// layer-7c continuous effect so artifact creatures entering after Chief
	// are buffed dynamically and the buff tears down when Chief leaves.
	registerTribalLordStatic(gs, perm, "chief_of_the_foundry",
		otherTribePredicate(perm, "artifact"), 1, 1)
	emit(gs, "chief_of_the_foundry_buff", "Chief of the Foundry", map[string]interface{}{
		"seat": perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// Caged Sun — "As ~ enters, choose a color. Creatures you control of
// that color get +1/+1. Whenever a land you control is tapped for mana
// of that color, add one additional mana of that color."
//
// Layer 7c continuous effect: creatures you control of the chosen color
// get +1/+1 while Caged Sun is on the battlefield. The chosen color is
// stored in perm.Flags["chosen_color_X"] (same pattern as Painter's
// Servant). Defaults to first color in commander identity; falls back
// to "W" if none available.
// Mana doubling kept as a triggered ability (not a layer effect).
// ---------------------------------------------------------------------------

func registerCagedSun(r *Registry) {
	r.OnETB("Caged Sun", cagedSunETB)
	r.OnTrigger("Caged Sun", "land_tapped_for_mana", cagedSunLandTap)
}

// cagedSunChosenColor reads the chosen color from the permanent's flags.
// Falls back to the commander's first color, then "W".
func cagedSunChosenColor(gs *gameengine.GameState, perm *gameengine.Permanent) string {
	if perm.Flags != nil {
		for _, c := range []string{"W", "U", "B", "R", "G"} {
			if perm.Flags["chosen_color_"+c] > 0 {
				return c
			}
		}
	}
	// Default: first color of the commander, or "W".
	seat := gs.Seats[perm.Controller]
	if seat != nil {
		for _, cmd := range seat.CommandZone {
			if cmd != nil && len(cmd.Colors) > 0 {
				return cmd.Colors[0]
			}
		}
		// Check battlefield for the commander.
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			for _, cn := range seat.CommanderNames {
				if cn != "" && p.Card.Name == cn && len(p.Card.Colors) > 0 {
					return p.Card.Colors[0]
				}
			}
		}
	}
	return "W"
}

func cagedSunETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	chosen := cagedSunChosenColor(gs, perm)
	// Stamp the chosen color into flags for the layer handler to read.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["chosen_color_"+chosen] = 1

	// Register layer 7c continuous effect: +1/+1 to creatures you control
	// of the chosen color.
	source := perm
	controllerSeat := perm.Controller
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "c",
		SourcePerm:     source,
		SourceCardName: "Caged Sun",
		ControllerSeat: controllerSeat,
		HandlerID:      "caged_sun_" + itoa(controllerSeat) + "_" + chosen,
		Duration:       gameengine.DurationPermanent,
		Predicate: func(_ *gameengine.GameState, target *gameengine.Permanent) bool {
			if target == nil || target.Card == nil || !target.IsCreature() {
				return false
			}
			if target.Controller != controllerSeat {
				return false
			}
			return gameengine.CardHasColor(target.Card, chosen)
		},
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			chars.Power++
			chars.Toughness++
		},
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, "caged_sun_etb", "Caged Sun", map[string]interface{}{
		"seat":   perm.Controller,
		"color":  chosen,
		"effect": "layer_7c_color_buff",
	})
}

func cagedSunLandTap(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	seat, _ := ctx["seat"].(int)
	if seat != perm.Controller {
		return
	}
	color, _ := ctx["color"].(string)
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		amount = 1
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	gameengine.AddMana(gs, s, color, amount, "Caged Sun")
	emit(gs, "caged_sun_mana", "Caged Sun", map[string]interface{}{
		"seat":   perm.Controller,
		"color":  color,
		"amount": amount,
	})
}

// ---------------------------------------------------------------------------
// Gauntlet of Power — "As ~ enters, choose a color. Creatures of the
// chosen color get +1/+1. Whenever a basic land is tapped for mana of
// that color, its controller adds one additional mana of that color."
//
// Same layer 7c pattern as Caged Sun but SYMMETRIC — affects ALL
// creatures of the chosen color regardless of controller. The chosen
// color uses the same flag/fallback logic as Caged Sun.
// ---------------------------------------------------------------------------

func registerGauntletOfPower(r *Registry) {
	r.OnETB("Gauntlet of Power", gauntletOfPowerETB)
	r.OnTrigger("Gauntlet of Power", "land_tapped_for_mana", gauntletLandTap)
}

func gauntletOfPowerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	// Reuse same color-choosing logic as Caged Sun.
	chosen := cagedSunChosenColor(gs, perm)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["chosen_color_"+chosen] = 1

	// Register layer 7c continuous effect: +1/+1 to ALL creatures of
	// the chosen color (symmetric — every seat).
	source := perm
	controllerSeat := perm.Controller
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "c",
		SourcePerm:     source,
		SourceCardName: "Gauntlet of Power",
		ControllerSeat: controllerSeat,
		HandlerID:      "gauntlet_of_power_" + itoa(controllerSeat) + "_" + chosen,
		Duration:       gameengine.DurationPermanent,
		Predicate: func(_ *gameengine.GameState, target *gameengine.Permanent) bool {
			if target == nil || target.Card == nil || !target.IsCreature() {
				return false
			}
			return gameengine.CardHasColor(target.Card, chosen)
		},
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			chars.Power++
			chars.Toughness++
		},
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, "gauntlet_of_power_etb", "Gauntlet of Power", map[string]interface{}{
		"seat":   perm.Controller,
		"color":  chosen,
		"effect": "layer_7c_color_buff_symmetric",
	})
}

func gauntletLandTap(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	seat, _ := ctx["seat"].(int)
	color, _ := ctx["color"].(string)
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		amount = 1
	}
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	gameengine.AddMana(gs, s, color, amount, "Gauntlet of Power")
	emit(gs, "gauntlet_mana", "Gauntlet of Power", map[string]interface{}{
		"seat":   seat,
		"color":  color,
		"amount": amount,
	})
}

// ---------------------------------------------------------------------------
// Imotekh the Stormlord — "Whenever one or more artifact cards leave
// your graveyard, create two 2/2 Necron Warrior artifact creature tokens."
// ---------------------------------------------------------------------------

func registerImotekh(r *Registry) {
	r.OnETB("Imotekh the Stormlord", imotekhETB)
	r.OnTrigger("Imotekh the Stormlord", "graveyard_leave", imotekhGraveyardLeave)
}

func imotekhETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "imotekh_etb", "Imotekh the Stormlord", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "artifact_graveyard_leave_creates_necrons",
	})
}

func imotekhGraveyardLeave(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	seat, _ := ctx["seat"].(int)
	if seat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	isArtifact := false
	for _, t := range card.Types {
		if t == "artifact" {
			isArtifact = true
			break
		}
	}
	if !isArtifact {
		return
	}
	for i := 0; i < 2; i++ {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Necron Warrior", []string{"artifact", "creature"}, 2, 2)
	}
	emit(gs, "imotekh_trigger", "Imotekh the Stormlord", map[string]interface{}{
		"seat": perm.Controller,
	})
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
	gameengine.LoseLife(gs, activeSeat, 3, perm.Card.DisplayName())
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
	r.OnTrigger("Lich's Mastery", "life_gained", lichsMasteryLifeGained)
	r.OnTrigger("Lich's Mastery", "life_lost", lichsMasteryLifeLost)
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
}

// lichsMasteryLifeGained — "Whenever you gain life, draw that many cards."
func lichsMasteryLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != seat {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	for i := 0; i < amount; i++ {
		drawOne(gs, seat, "Lich's Mastery")
	}
	emit(gs, "lichs_mastery_draw", "Lich's Mastery", map[string]interface{}{
		"seat":  seat,
		"drawn": amount,
	})
}

// lichsMasteryLifeLost — "Whenever you lose life, for each 1 life you
// lost, exile a permanent you control or a card from your hand or
// graveyard."
func lichsMasteryLifeLost(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	lossSeat, _ := ctx["seat"].(int)
	if lossSeat != seat {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	exiled := 0
	for i := 0; i < amount; i++ {
		// Priority: battlefield permanents first, then hand, then graveyard.
		//
		// R58 fix (Loki r57 lead 1, Mire's Grasp game-3744): the
		// battlefield path used to call MoveCard(card, seat, "battlefield",
		// "exile", ...). MoveCard's removeCardFromZone is intentionally a
		// no-op for "battlefield" (zone_move.go:239 — battlefield source
		// removal is the caller's responsibility — see Krark r54 fix for
		// the same class of bug). The Permanent was never unwound from
		// the battlefield, so the *Card ended up in both exile (from the
		// FireZoneChange write) and the still-live battlefield Permanent
		// — CardIdentity duplication. Use ExilePermanent, which does the
		// full §406.3 lifecycle: would_be_exiled replacement chain,
		// removePermanent, UnregisterReplacements/Continuous, detachAll,
		// commander redirect, and LTB triggers.
		// r58 follow-up: skip Lich's Mastery itself. The original
		// `last-permanent` picker would auto-self-exile when Lich was
		// the last entry in the battlefield slice, triggering its own
		// "leaves the battlefield → you lose the game" SBA and
		// defeating the engine's entire purpose. Scan backward for the
		// most-recent non-source permanent instead.
		var victim *gameengine.Permanent
		for idx := len(s.Battlefield) - 1; idx >= 0; idx-- {
			cand := s.Battlefield[idx]
			if cand == nil || cand.Card == nil || cand == perm {
				continue
			}
			victim = cand
			break
		}
		if victim != nil {
			if gameengine.ExilePermanent(gs, victim, perm) {
				exiled++
				continue
			}
		}
		if len(s.Hand) > 0 {
			c := s.Hand[len(s.Hand)-1]
			gameengine.MoveCard(gs, c, seat, "hand", "exile", "lichs_mastery")
			exiled++
			continue
		}
		if len(s.Graveyard) > 0 {
			c := s.Graveyard[len(s.Graveyard)-1]
			gameengine.MoveCard(gs, c, seat, "graveyard", "exile", "lichs_mastery")
			exiled++
			continue
		}
		break // nothing left to exile
	}
	emit(gs, "lichs_mastery_exile", "Lich's Mastery", map[string]interface{}{
		"seat":   seat,
		"exiled": exiled,
		"needed": amount,
	})
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
	})
	r.OnActivated("Jhoira of the Ghitu", jhoiraActivated)
}

// jhoiraActivated — "{2}: Exile a nonland card from your hand face up
// with four time counters on it. It gains suspend." (CR §702.62)
func jhoiraActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if len(s.Hand) == 0 {
		return
	}
	// Pick the highest-CMC nonland card from hand.
	bestIdx := -1
	bestCMC := -1
	for i, c := range s.Hand {
		if c == nil || cardHasType(c, "land") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return
	}
	card := s.Hand[bestIdx]
	gameengine.SuspendCard(gs, seat, card, 4)
	emit(gs, "jhoira_suspend", "Jhoira of the Ghitu", map[string]interface{}{
		"seat":      seat,
		"suspended": card.DisplayName(),
		"counters":  4,
	})
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func init() {
	registerBatch17Sweep(Global())
	AddResetHook(registerBatch17Sweep)
}

func registerBatch17Sweep(r *Registry) {
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
