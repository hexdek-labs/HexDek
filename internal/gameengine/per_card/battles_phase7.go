package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Phase 7 Counter DB — Battle "becomes_defeated" handlers (CR §310.10).
//
// The defense counter lifecycle is fully wired in the engine: ETB seeds
// perm.Counters["defense"] from BaseToughness (stack.go); combat damage
// routes through ApplyCombatDamageToBattle → RemoveDefenseCounters →
// FireBattleZeroDefense, which fires "becomes_defeated" and latches the
// battle_defeated flag. SBA §704.5v destroys the battle on the next
// pass. Battles in this engine model the "transform when defeated"
// flavour by firing the per-card defeat payoff immediately, since the
// engine has no Battle DFC back-face plumbing.
//
// Each handler approximates the printed back-face payoff with the
// closest deterministic effect the engine already supports.

func init() {
	registerBattlesPhase7(Global())
	AddResetHook(registerBattlesPhase7)
}

func registerBattlesPhase7(r *Registry) {
	for _, name := range battlePhase7Names {
		r.OnTrigger(name, "becomes_defeated", dispatchBattlePhase7)
	}
}

var battlePhase7Names = []string{
	"Invasion of Alara",
	"Invasion of Amonkhet",
	"Invasion of Arcavios",
	"Invasion of Belenon",
	"Invasion of Dominaria",
	"Invasion of Fiora",
	"Invasion of Ikoria",
	"Invasion of Innistrad",
	"Invasion of Ixalan",
	"Invasion of Kaladesh",
	"Invasion of Lorwyn",
	"Invasion of New Phyrexia",
}

func dispatchBattlePhase7(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	switch perm.Card.DisplayName() {
	case "Invasion of Alara":
		battleInvasionOfAlara(gs, perm)
	case "Invasion of Amonkhet":
		battleInvasionOfAmonkhet(gs, perm)
	case "Invasion of Arcavios":
		battleInvasionOfArcavios(gs, perm)
	case "Invasion of Belenon":
		battleInvasionOfBelenon(gs, perm)
	case "Invasion of Dominaria":
		battleInvasionOfDominaria(gs, perm)
	case "Invasion of Fiora":
		battleInvasionOfFiora(gs, perm)
	case "Invasion of Ikoria":
		battleInvasionOfIkoria(gs, perm)
	case "Invasion of Innistrad":
		battleInvasionOfInnistrad(gs, perm)
	case "Invasion of Ixalan":
		battleInvasionOfIxalan(gs, perm)
	case "Invasion of Kaladesh":
		battleInvasionOfKaladesh(gs, perm)
	case "Invasion of Lorwyn":
		battleInvasionOfLorwyn(gs, perm)
	case "Invasion of New Phyrexia":
		battleInvasionOfNewPhyrexia(gs, perm)
	}
}

// ---------------------------------------------------------------------------
// Individual battle defeat handlers (back-face payoffs, approximated)
// ---------------------------------------------------------------------------

func battleInvasionOfAlara(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_alara_defeated"
	// Back: Awaken the Maelstrom — cast up to five spells from gy at
	// random. Approximation: return up to 3 highest-CMC cards from
	// controller's graveyard to hand.
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	type entry struct {
		c   *gameengine.Card
		cmc int
	}
	pool := make([]entry, 0, len(seat.Graveyard))
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		pool = append(pool, entry{c, cardCMC(c)})
	}
	for i := 0; i < len(pool); i++ {
		for j := i + 1; j < len(pool); j++ {
			if pool[j].cmc > pool[i].cmc {
				pool[i], pool[j] = pool[j], pool[i]
			}
		}
	}
	n := 3
	if n > len(pool) {
		n = len(pool)
	}
	for i := 0; i < n; i++ {
		gameengine.MoveCard(gs, pool[i].c, perm.Controller, "graveyard", "hand", "alara_defeated")
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "returned": n})
}

func battleInvasionOfAmonkhet(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_amonkhet_defeated"
	// Back: Lazotep Convert — 5/5 zombie. Approximation: 5/5 zombie token.
	gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie God Token",
		[]string{"creature", "zombie", "god", "token", "pip:black"}, 5, 5)
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

func battleInvasionOfArcavios(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_arcavios_defeated"
	// Back: Invocation of the Founders — search lib for instant/sorcery,
	// put into hand. Approximation: draw 2.
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for i := 0; i < 2 && len(seat.Library) > 0; i++ {
		top := seat.Library[0]
		gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "arcavios_defeated")
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

func battleInvasionOfBelenon(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_belenon_defeated"
	// Back: Belenon War Anthem — 3/3 knight token + buff knights.
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	gameengine.CreateCreatureToken(gs, perm.Controller, "Knight Token",
		[]string{"creature", "knight", "token", "pip:white"}, 3, 3)
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if !cardHasSubtype(p.Card, "knight") {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power: 1, Toughness: 1, Duration: "",
			Timestamp: gs.NextTimestamp(),
		})
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

func battleInvasionOfDominaria(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_dominaria_defeated"
	// Back: Serra Faithkeeper — 4/4 angel with flying + lifelink.
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Angel Token",
		[]string{"creature", "angel", "token", "pip:white"}, 4, 4)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["kw:flying"] = 1
		tok.Flags["kw:lifelink"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

func battleInvasionOfFiora(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_fiora_defeated"
	// Back: Marchesa, Resolute Monarch — destroy each creature with the
	// greatest power. Approximation: find max-power creature(s) globally
	// and destroy them.
	maxPower := -1
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if p.Power() > maxPower {
				maxPower = p.Power()
			}
		}
	}
	if maxPower < 0 {
		return
	}
	destroyed := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range snapshotBattlefieldPer(s) {
			if p == nil || !p.IsCreature() {
				continue
			}
			if p.Power() != maxPower {
				continue
			}
			gameengine.DestroyPermanent(gs, p, perm)
			destroyed++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "destroyed": destroyed})
}

func battleInvasionOfIkoria(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_ikoria_defeated"
	// Back: Zilortha, Apex of Ikoria — 7/3 dinosaur with trample.
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Dinosaur Token",
		[]string{"creature", "dinosaur", "token", "pip:red", "pip:green"}, 7, 3)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["kw:trample"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

func battleInvasionOfInnistrad(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_innistrad_defeated"
	// Back: Deluge of the Dead — 3 zombie tokens.
	for i := 0; i < 3; i++ {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie Token",
			[]string{"creature", "zombie", "token", "pip:black"}, 2, 2)
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "tokens": 3})
}

func battleInvasionOfIxalan(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_ixalan_defeated"
	// Back: Belligerent Regisaur — 6/6 dinosaur with trample + haste.
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Dinosaur Token",
		[]string{"creature", "dinosaur", "token", "pip:red", "pip:green"}, 6, 6)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["kw:trample"] = 1
		tok.Flags["kw:haste"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

func battleInvasionOfKaladesh(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_kaladesh_defeated"
	// Back: Aetherwing, Golden-Scale Flagship — 6/5 vehicle with flying.
	// Approximation: 6/5 artifact creature token with flying.
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Aetherwing Token",
		[]string{"creature", "artifact", "construct", "token", "pip:colorless"}, 6, 5)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["kw:flying"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

func battleInvasionOfLorwyn(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_lorwyn_defeated"
	// Back: Winnowing Forces — destroy 3 creatures of opp's choice.
	// Approximation: destroy each opp's strongest (highest-power) creature.
	destroyed := 0
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil || s.Lost {
			continue
		}
		var pick *gameengine.Permanent
		bestPower := -1
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if p.Power() > bestPower {
				bestPower = p.Power()
				pick = p
			}
		}
		if pick != nil {
			gameengine.DestroyPermanent(gs, pick, perm)
			destroyed++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller, "destroyed": destroyed})
}

func battleInvasionOfNewPhyrexia(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "invasion_of_new_phyrexia_defeated"
	// Back: Teferi Akosa of Zhalfir — 5-loyalty pw. Engine has no DFC
	// pw spawning here; emitPartial and as a tangible payoff, drain each
	// opp for 3 life.
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		gameengine.LoseLife(gs, oppIdx, 3, perm.Card.DisplayName())
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "pw_back_face_transform_unmodeled")
	emit(gs, slug, perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}
