package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Phase 7 Counter DB — Saga chapter handlers.
//
// Each saga registers a single OnTrigger("Card Name", "lore_counter_added",
// handler). The handler reads ctx["chapter"] (int) and dispatches per
// CR §714.2c. The lore counter itself is placed by AdvanceSagaChapter
// (CR §714.2b precombat-main tick, or §714.3a on-ETB seed); SBA §704.5s
// sacrifices the saga once lore reaches saga_final_chapter.
//
// Chapter effects are pragmatic approximations: where the engine has the
// primitive (token mint, life change, sac, mill, destroy) we use it;
// where the printed effect needs targeting / choice modeling the
// handler emits a partial and falls back to a deterministic pick.

func init() {
	registerSagasPhase7(Global())
	AddResetHook(registerSagasPhase7)
}

func registerSagasPhase7(r *Registry) {
	for _, name := range sagaPhase7Names {
		r.OnTrigger(name, "lore_counter_added", dispatchSagaPhase7)
	}
}

// sagaPhase7Names is the canonical list of Sagas wired in Phase 7. The
// dispatcher switches on perm.Card.DisplayName() so DFC alt-faces are
// not registered here (those keep their dedicated handlers — Sheoldred,
// Elesh Norn).
var sagaPhase7Names = []string{
	"History of Benalia",
	"Elspeth Conquers Death",
	"The Eldest Reborn",
	"The Antiquities War",
	"Phyrexian Scriptures",
	"The Mirari Conjecture",
	"Triumphant Reckoning",
	"Fall of the Thran",
	"The Flame of Keld",
	"The First Eruption",
	"Song of Freyalise",
	"The Birth of Meletis",
	"The Binding of the Titans",
	"The Triumph of Anax",
	"The Cruelty of Gix",
	"The Phasing of Zhalfir",
	"Showdown of the Skalds",
	"King Harald's Revenge",
	"The Bloodsky Massacre",
	"The Bears of Littjara",
	"The Raven's Warning",
	"Tribute to Horobi",
	"Battle of Frost and Fire",
	"In Search of Greatness",
	"Fall of the Impostor",
}

func dispatchSagaPhase7(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	chapter, _ := ctx["chapter"].(int)
	if chapter <= 0 {
		return
	}
	name := perm.Card.DisplayName()
	switch name {
	case "History of Benalia":
		sagaHistoryOfBenalia(gs, perm, chapter)
	case "Elspeth Conquers Death":
		sagaElspethConquersDeath(gs, perm, chapter)
	case "The Eldest Reborn":
		sagaTheEldestReborn(gs, perm, chapter)
	case "The Antiquities War":
		sagaTheAntiquitiesWar(gs, perm, chapter)
	case "Phyrexian Scriptures":
		sagaPhyrexianScriptures(gs, perm, chapter)
	case "The Mirari Conjecture":
		sagaTheMirariConjecture(gs, perm, chapter)
	case "Triumphant Reckoning":
		sagaTriumphantReckoning(gs, perm, chapter)
	case "Fall of the Thran":
		sagaFallOfTheThran(gs, perm, chapter)
	case "The Flame of Keld":
		sagaTheFlameOfKeld(gs, perm, chapter)
	case "The First Eruption":
		sagaTheFirstEruption(gs, perm, chapter)
	case "Song of Freyalise":
		sagaSongOfFreyalise(gs, perm, chapter)
	case "The Birth of Meletis":
		sagaTheBirthOfMeletis(gs, perm, chapter)
	case "The Binding of the Titans":
		sagaTheBindingOfTheTitans(gs, perm, chapter)
	case "The Triumph of Anax":
		sagaTheTriumphOfAnax(gs, perm, chapter)
	case "The Cruelty of Gix":
		sagaTheCrueltyOfGix(gs, perm, chapter)
	case "The Phasing of Zhalfir":
		sagaThePhasingOfZhalfir(gs, perm, chapter)
	case "Showdown of the Skalds":
		sagaShowdownOfTheSkalds(gs, perm, chapter)
	case "King Harald's Revenge":
		sagaKingHaraldsRevenge(gs, perm, chapter)
	case "The Bloodsky Massacre":
		sagaTheBloodskyMassacre(gs, perm, chapter)
	case "The Bears of Littjara":
		sagaTheBearsOfLittjara(gs, perm, chapter)
	case "The Raven's Warning":
		sagaTheRavensWarning(gs, perm, chapter)
	case "Tribute to Horobi":
		sagaTributeToHorobi(gs, perm, chapter)
	case "Battle of Frost and Fire":
		sagaBattleOfFrostAndFire(gs, perm, chapter)
	case "In Search of Greatness":
		sagaInSearchOfGreatness(gs, perm, chapter)
	case "Fall of the Impostor":
		sagaFallOfTheImpostor(gs, perm, chapter)
	}
}

// ---------------------------------------------------------------------------
// Individual saga chapter handlers
// ---------------------------------------------------------------------------

func sagaHistoryOfBenalia(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "history_of_benalia_chapter"
	switch chapter {
	case 1, 2:
		gameengine.CreateCreatureToken(gs, perm.Controller, "Knight Token",
			[]string{"creature", "knight", "token", "pip:white"}, 2, 2)
	case 3:
		seat := gs.Seats[perm.Controller]
		if seat == nil {
			return
		}
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if !cardHasSubtype(p.Card, "knight") {
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power: 2, Toughness: 1, Duration: "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["kw:vigilance"] = 1
		}
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaElspethConquersDeath(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "elspeth_conquers_death_chapter"
	switch chapter {
	case 1:
		// Exile target nonland, nontoken permanent an opponent controls.
		// Pick highest-CMC nonland permanent across opponents.
		var pick *gameengine.Permanent
		bestCMC := -1
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			s := gs.Seats[oppIdx]
			if s == nil || s.Lost {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil || p.IsLand() {
					continue
				}
				if cardHasType(p.Card, "token") {
					continue
				}
				if cm := cardCMC(p.Card); cm > bestCMC {
					bestCMC = cm
					pick = p
				}
			}
		}
		if pick != nil {
			gameengine.ExilePermanent(gs, pick, perm)
		}
	case 2:
		// Each opponent sacrifices a creature or pw with mv >= 3.
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			s := gs.Seats[oppIdx]
			if s == nil || s.Lost {
				continue
			}
			var pick *gameengine.Permanent
			lowCMC := 1 << 30
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if !p.IsCreature() && !p.IsPlaneswalker() {
					continue
				}
				cm := cardCMC(p.Card)
				if cm < 3 {
					continue
				}
				if cm < lowCMC {
					lowCMC = cm
					pick = p
				}
			}
			if pick != nil {
				gameengine.SacrificePermanent(gs, pick, "elspeth_chapter_2")
			}
		}
	case 3:
		// Return creature or pw from gy +1/+1 + flying (approximation:
		// return highest-CMC from controller's graveyard).
		seat := gs.Seats[perm.Controller]
		if seat == nil {
			return
		}
		var picked *gameengine.Card
		bestCMC := -1
		for _, c := range seat.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasType(c, "creature") && !cardHasType(c, "planeswalker") {
				continue
			}
			if cm := cardCMC(c); cm > bestCMC {
				bestCMC = cm
				picked = c
			}
		}
		if picked != nil {
			ret := enterBattlefieldWithETB(gs, perm.Controller, picked, false)
			if ret != nil && ret.IsCreature() {
				ret.Modifications = append(ret.Modifications, gameengine.Modification{
					Power: 1, Toughness: 1, Duration: "",
					Timestamp: gs.NextTimestamp(),
				})
				if ret.Flags == nil {
					ret.Flags = map[string]int{}
				}
				ret.Flags["kw:flying"] = 1
				gs.InvalidateCharacteristicsCache()
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheEldestReborn(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "the_eldest_reborn_chapter"
	switch chapter {
	case 1:
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			s := gs.Seats[oppIdx]
			if s == nil || s.Lost {
				continue
			}
			var pick *gameengine.Permanent
			low := 1 << 30
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if !p.IsCreature() && !p.IsPlaneswalker() {
					continue
				}
				if cm := cardCMC(p.Card); cm < low {
					low = cm
					pick = p
				}
			}
			if pick != nil {
				gameengine.SacrificePermanent(gs, pick, "eldest_reborn_chapter_1")
			}
		}
	case 2:
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			s := gs.Seats[oppIdx]
			if s == nil || s.Lost || len(s.Hand) == 0 {
				continue
			}
			high := -1
			idx := 0
			for j, c := range s.Hand {
				if c == nil {
					continue
				}
				if cardCMC(c) > high {
					high = cardCMC(c)
					idx = j
				}
			}
			c := s.Hand[idx]
			gameengine.MoveCard(gs, c, oppIdx, "hand", "graveyard", "eldest_reborn_chapter_2")
		}
	case 3:
		// Return creature or pw from any graveyard under your control.
		var pickCard *gameengine.Card
		var pickSeat int
		bestCMC := -1
		for i, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, c := range s.Graveyard {
				if c == nil {
					continue
				}
				if !cardHasType(c, "creature") && !cardHasType(c, "planeswalker") {
					continue
				}
				if cm := cardCMC(c); cm > bestCMC {
					bestCMC = cm
					pickCard = c
					pickSeat = i
				}
			}
		}
		_ = pickSeat
		if pickCard != nil {
			enterBattlefieldWithETB(gs, perm.Controller, pickCard, false)
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheAntiquitiesWar(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "antiquities_war_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1, 2:
		// Look top 5; may put artifact in hand. Approximation: scan top
		// 5, move first artifact to hand.
		n := 5
		if n > len(seat.Library) {
			n = len(seat.Library)
		}
		for i := 0; i < n; i++ {
			c := seat.Library[i]
			if c == nil {
				continue
			}
			if cardHasType(c, "artifact") {
				gameengine.MoveCard(gs, c, perm.Controller, "library", "hand", "antiquities_war_chapter")
				break
			}
		}
	case 3:
		// Each artifact you control becomes 5/5 artifact creature UEOT
		// (we model "until end of turn" via Modifications; type change
		// is approximated by skipping non-creature artifacts in this
		// engine — emit partial).
		buffed := 0
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !cardHasType(p.Card, "artifact") {
				continue
			}
			if !p.IsCreature() {
				emitPartial(gs, slug, perm.Card.DisplayName(), "artifact_to_creature_type_change_unmodeled")
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power: 5 - p.Power(), Toughness: 5 - p.Toughness(),
				Duration:  "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			buffed++
		}
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaPhyrexianScriptures(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "phyrexian_scriptures_chapter"
	switch chapter {
	case 1:
		// Put a +1/+1 counter on target non-artifact creature, then it
		// becomes an artifact. Approximation: pick a non-artifact
		// creature controller controls; add a +1/+1 counter only.
		seat := gs.Seats[perm.Controller]
		if seat == nil {
			return
		}
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() || p.Card == nil {
				continue
			}
			if cardHasType(p.Card, "artifact") {
				continue
			}
			p.AddCounter("+1/+1", 1)
			break
		}
	case 2:
		// Destroy all non-artifact creatures.
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range snapshotBattlefieldPer(s) {
				if p == nil || p.Card == nil || !p.IsCreature() {
					continue
				}
				if cardHasType(p.Card, "artifact") {
					continue
				}
				gameengine.DestroyPermanent(gs, p, perm)
			}
		}
	case 3:
		// Exile all cards from all graveyards.
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			gy := append([]*gameengine.Card(nil), s.Graveyard...)
			for _, c := range gy {
				if c == nil {
					continue
				}
				gameengine.MoveCard(gs, c, s.Idx, "graveyard", "exile", "phyrexian_scriptures_chapter_3")
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheMirariConjecture(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "mirari_conjecture_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1, 2:
		typ := "instant"
		if chapter == 2 {
			typ = "sorcery"
		}
		var pickCard *gameengine.Card
		bestCMC := -1
		for _, c := range seat.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasType(c, typ) {
				continue
			}
			if cm := cardCMC(c); cm > bestCMC {
				bestCMC = cm
				pickCard = c
			}
		}
		if pickCard != nil {
			gameengine.MoveCard(gs, pickCard, perm.Controller, "graveyard", "hand", "mirari_chapter")
		}
	case 3:
		emitPartial(gs, slug, perm.Card.DisplayName(), "copy_each_instant_sorcery_cast_this_turn_unmodeled")
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTriumphantReckoning(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "triumphant_reckoning_chapter"
	if chapter != 3 {
		emit(gs, slug, perm.Card.DisplayName(),
			map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	targets := []*gameengine.Card{}
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "artifact") || cardHasType(c, "enchantment") || cardHasType(c, "planeswalker") {
			targets = append(targets, c)
		}
	}
	for _, c := range targets {
		enterBattlefieldWithETB(gs, perm.Controller, c, false)
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": 3, "returned": len(targets)})
}

func sagaFallOfTheThran(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "fall_of_the_thran_chapter"
	switch chapter {
	case 1:
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range snapshotBattlefieldPer(s) {
				if p == nil || !p.IsLand() {
					continue
				}
				gameengine.DestroyPermanent(gs, p, perm)
			}
		}
	case 2, 3:
		// Each player may return up to two land cards from gy to bf.
		// Approximation: return first two land cards (by CMC desc).
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			returned := 0
			for _, c := range append([]*gameengine.Card(nil), s.Graveyard...) {
				if c == nil || !cardHasType(c, "land") {
					continue
				}
				enterBattlefieldWithETB(gs, s.Idx, c, false)
				returned++
				if returned >= 2 {
					break
				}
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheFlameOfKeld(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "flame_of_keld_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1:
		// Discard hand, draw 2. Approximation: dump hand to gy, draw 2
		// (via library top).
		hand := append([]*gameengine.Card(nil), seat.Hand...)
		for _, c := range hand {
			if c == nil {
				continue
			}
			gameengine.MoveCard(gs, c, perm.Controller, "hand", "graveyard", "flame_of_keld_discard")
		}
		for i := 0; i < 2 && len(seat.Library) > 0; i++ {
			top := seat.Library[0]
			gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "flame_of_keld_draw")
		}
	case 2:
		emitPartial(gs, slug, perm.Card.DisplayName(), "minimum_damage_floor_unmodeled")
	case 3:
		// Permanent +2 damage from red sources UEOT. Engine has no
		// damage-amp continuous effect; emit partial.
		emitPartial(gs, slug, perm.Card.DisplayName(), "red_source_damage_bonus_unmodeled")
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheFirstEruption(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "first_eruption_chapter"
	dmg := 0
	switch chapter {
	case 1, 2:
		dmg = 1
	case 3:
		dmg = 3
	}
	if dmg <= 0 {
		return
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range snapshotBattlefieldPer(s) {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.MarkedDamage += dmg
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter, "damage": dmg})
}

func sagaSongOfFreyalise(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "song_of_freyalise_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1, 2:
		// Untap each creature you control; you may add one mana per
		// creature. Approximation: just untap creatures.
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.Tapped = false
		}
	case 3:
		// Creatures get +1/+1, vigilance, indestructible UEOT.
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power: 1, Toughness: 1, Duration: "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["kw:vigilance"] = 1
			p.Flags["kw:indestructible"] = 1
		}
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheBirthOfMeletis(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "birth_of_meletis_chapter"
	switch chapter {
	case 1:
		gameengine.CreateCreatureToken(gs, perm.Controller, "Wall Token",
			[]string{"creature", "wall", "token", "pip:white"}, 0, 4)
	case 2:
		gameengine.GainLife(gs, perm.Controller, 2, perm.Card.DisplayName())
	case 3:
		// Search library for basic Plains. Approximation: pull first
		// basic Plains from library to battlefield tapped.
		seat := gs.Seats[perm.Controller]
		if seat == nil {
			return
		}
		for i, c := range seat.Library {
			if c == nil {
				continue
			}
			if cardHasType(c, "land") && cardHasSubtype(c, "plains") {
				_ = i
				enterBattlefieldWithETB(gs, perm.Controller, c, true)
				break
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheBindingOfTheTitans(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "binding_of_titans_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1:
		// Mill 4.
		for i := 0; i < 4 && len(seat.Library) > 0; i++ {
			top := seat.Library[0]
			gameengine.MoveCard(gs, top, perm.Controller, "library", "graveyard", "binding_titans_mill")
		}
	case 2:
		// Scry 2; draw a card if X. Approximation: draw 1.
		if len(seat.Library) > 0 {
			top := seat.Library[0]
			gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "binding_titans_draw")
		}
	case 3:
		// Return creature or land from gy to hand.
		var pick *gameengine.Card
		bestCMC := -1
		for _, c := range seat.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasType(c, "creature") && !cardHasType(c, "land") {
				continue
			}
			if cm := cardCMC(c); cm > bestCMC {
				bestCMC = cm
				pick = c
			}
		}
		if pick != nil {
			gameengine.MoveCard(gs, pick, perm.Controller, "graveyard", "hand", "binding_titans_return")
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheTriumphOfAnax(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "triumph_of_anax_chapter"
	switch chapter {
	case 1:
		// Create 1/1 Minotaur with menace + haste.
		tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Minotaur Token",
			[]string{"creature", "minotaur", "token", "pip:red"}, 1, 1)
		if tok != nil {
			if tok.Flags == nil {
				tok.Flags = map[string]int{}
			}
			tok.Flags["kw:menace"] = 1
			tok.Flags["kw:haste"] = 1
		}
	case 2, 3:
		// Devotion-scaled buff. Approximation: +1/+0 UEOT to each
		// creature you control.
		seat := gs.Seats[perm.Controller]
		if seat == nil {
			return
		}
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power: 1, Toughness: 0, Duration: "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
		}
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheCrueltyOfGix(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "cruelty_of_gix_chapter"
	switch chapter {
	case 1:
		// Target opp loses 2 life.
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			gameengine.LoseLife(gs, oppIdx, 2, perm.Card.DisplayName())
			break
		}
	case 2:
		// Scry 2 — approximation: nothing observable.
		emitPartial(gs, slug, perm.Card.DisplayName(), "scry_2_unmodeled")
	case 3:
		// Reanimate a creature with mv >= 4 (oracle says scaling); pick
		// highest-CMC creature from any graveyard.
		var pick *gameengine.Card
		bestCMC := -1
		for _, s := range gs.Seats {
			if s == nil || s.LeftGame {
				// CR §800.4a: cards owned by a departed player left the
				// game with them — they are not legal reanimation targets
				// (their graveyard pointers persist only for forensics).
				continue
			}
			for _, c := range s.Graveyard {
				if c == nil || !cardHasType(c, "creature") {
					continue
				}
				if cm := cardCMC(c); cm > bestCMC && cm >= 4 {
					bestCMC = cm
					pick = c
				}
			}
		}
		if pick != nil {
			enterBattlefieldWithETB(gs, perm.Controller, pick, false)
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaThePhasingOfZhalfir(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "phasing_of_zhalfir_chapter"
	emitPartial(gs, slug, perm.Card.DisplayName(), "phase_out_temporary_zone_unmodeled")
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaShowdownOfTheSkalds(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "showdown_of_skalds_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1:
		// Exile top 2 of library; may cast or put +1/+1 on creature.
		// Approximation: move to hand.
		for i := 0; i < 2 && len(seat.Library) > 0; i++ {
			top := seat.Library[0]
			gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "showdown_skalds")
		}
	case 2, 3:
		// Each creature gets a +1/+1 counter.
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.AddCounter("+1/+1", 1)
		}
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaKingHaraldsRevenge(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "king_haralds_revenge_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Creatures get +X/+X UEOT where X = number of creatures you control.
	x := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			x++
		}
	}
	if x == 0 {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power: x, Toughness: x, Duration: "until_end_of_turn",
			Timestamp: gs.NextTimestamp(),
		})
		if chapter == 3 {
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["kw:trample"] = 1
		}
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter, "x": x})
}

func sagaTheBloodskyMassacre(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "bloodsky_massacre_chapter"
	switch chapter {
	case 1, 2:
		gameengine.CreateCreatureToken(gs, perm.Controller, "Demon Berserker Token",
			[]string{"creature", "demon", "berserker", "token", "pip:black", "pip:red"}, 3, 2)
	case 3:
		// Each opp loses life equal to creatures you control.
		seat := gs.Seats[perm.Controller]
		if seat == nil {
			return
		}
		n := 0
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				n++
			}
		}
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			gameengine.LoseLife(gs, oppIdx, n, perm.Card.DisplayName())
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheBearsOfLittjara(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "bears_of_littjara_chapter"
	gameengine.CreateCreatureToken(gs, perm.Controller, "Shapeshifter Bear Token",
		[]string{"creature", "shapeshifter", "token", "pip:blue"}, 2, 2)
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTheRavensWarning(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "ravens_warning_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1:
		// Reveal top 4, may put a land in hand.
		n := 4
		if n > len(seat.Library) {
			n = len(seat.Library)
		}
		for i := 0; i < n; i++ {
			c := seat.Library[i]
			if c == nil {
				continue
			}
			if cardHasType(c, "land") {
				gameengine.MoveCard(gs, c, perm.Controller, "library", "hand", "ravens_warning")
				break
			}
		}
	case 2:
		gameengine.GainLife(gs, perm.Controller, 3, perm.Card.DisplayName())
	case 3:
		// Draw 2.
		for i := 0; i < 2 && len(seat.Library) > 0; i++ {
			top := seat.Library[0]
			gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "ravens_warning_draw")
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaTributeToHorobi(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "tribute_to_horobi_chapter"
	switch chapter {
	case 1, 2, 3:
		// Each opp's lowest-toughness creature dies.
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			s := gs.Seats[oppIdx]
			if s == nil || s.Lost {
				continue
			}
			var pick *gameengine.Permanent
			low := 1 << 30
			for _, p := range s.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				t := p.Toughness()
				if t < low {
					low = t
					pick = p
				}
			}
			if pick != nil {
				gameengine.DestroyPermanent(gs, pick, perm)
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaBattleOfFrostAndFire(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "battle_frost_fire_chapter"
	switch chapter {
	case 1, 2:
		// 2 damage to each non-snow creature.
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range snapshotBattlefieldPer(s) {
				if p == nil || !p.IsCreature() {
					continue
				}
				p.MarkedDamage += 2
			}
		}
	case 3:
		// 5 damage to each opp; tap each non-snow creature.
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			gameengine.LoseLife(gs, oppIdx, 5, perm.Card.DisplayName())
		}
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				p.Tapped = true
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaInSearchOfGreatness(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "in_search_greatness_chapter"
	// Each chapter: cascade-like — find next creature/pw cheaper than top
	// creature you control. Approximation: draw 1.
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if len(seat.Library) > 0 {
		top := seat.Library[0]
		gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "in_search_greatness")
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

func sagaFallOfTheImpostor(gs *gameengine.GameState, perm *gameengine.Permanent, chapter int) {
	const slug = "fall_of_impostor_chapter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1:
		// Put 2 +1/+1 on a creature you control.
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				p.AddCounter("+1/+1", 2)
				break
			}
		}
		gs.InvalidateCharacteristicsCache()
	case 2:
		// Gain control of target creature this turn — emitPartial.
		emitPartial(gs, slug, perm.Card.DisplayName(), "gain_control_target_creature_temporary_unmodeled")
	case 3:
		// Highest-power creature controller picks dies.
		var pick *gameengine.Permanent
		bestPower := -1
		for _, s := range gs.Seats {
			if s == nil || s.Idx == perm.Controller {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				if p.Power() > bestPower {
					bestPower = p.Power()
					pick = p
				}
			}
		}
		if pick != nil {
			gameengine.DestroyPermanent(gs, pick, perm)
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "chapter": chapter})
}

// snapshotBattlefieldPer returns a defensive copy of seat.Battlefield
// so chapter loops that destroy/sac/exile permanents in-place don't
// corrupt the iteration index. Package-local mirror of the gameengine
// internal helper.
func snapshotBattlefieldPer(s *gameengine.Seat) []*gameengine.Permanent {
	if s == nil {
		return nil
	}
	out := make([]*gameengine.Permanent, len(s.Battlefield))
	copy(out, s.Battlefield)
	return out
}
