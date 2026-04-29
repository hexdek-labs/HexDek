package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Stax spells and sacrifice-forcers — core Tergrid ecosystem cards.
// ============================================================================

// pickWeakestCreature returns the lowest-power creature on a seat's
// battlefield, preferring non-commander tokens first.
func pickWeakestCreature(gs *gameengine.GameState, seatIdx int) *gameengine.Permanent {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	var best *gameengine.Permanent
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if best == nil || p.Power() < best.Power() {
			best = p
		}
	}
	return best
}

// pickWeakestLand returns a basic land if possible, else any land.
func pickWeakestLand(gs *gameengine.GameState, seatIdx int) *gameengine.Permanent {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	var anyLand *gameengine.Permanent
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsLand() {
			continue
		}
		if anyLand == nil {
			anyLand = p
		}
		if p.Card != nil && cardHasType(p.Card, "basic") {
			return p
		}
	}
	return anyLand
}

// eachPlayerSacrificeCreature forces every player to sacrifice their
// weakest creature. Returns total sacrificed.
func eachPlayerSacrificeCreature(gs *gameengine.GameState) int {
	count := 0
	for i := range gs.Seats {
		if gs.Seats[i] == nil || gs.Seats[i].Lost {
			continue
		}
		victim := pickWeakestCreature(gs, i)
		if victim != nil {
			gameengine.SacrificePermanent(gs, victim, "forced_sacrifice")
			count++
		}
	}
	return count
}

// eachOpponentSacrificeCreature forces each opponent to sacrifice.
func eachOpponentSacrificeCreature(gs *gameengine.GameState, controllerSeat int) int {
	count := 0
	for _, opp := range gs.Opponents(controllerSeat) {
		victim := pickWeakestCreature(gs, opp)
		if victim != nil {
			gameengine.SacrificePermanent(gs, victim, "forced_sacrifice")
			count++
		}
	}
	return count
}

// --- Fleshbag Marauder ---
//
// Oracle: When Fleshbag Marauder enters the battlefield, each player
// sacrifices a creature.
// 2B creature 3/1.
func registerFleshbagMarauder(r *Registry) {
	r.OnETB("Fleshbag Marauder", fleshbagMarauderETB)
}

func fleshbagMarauderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	count := eachPlayerSacrificeCreature(gs)
	emit(gs, "fleshbag_marauder", "Fleshbag Marauder", map[string]interface{}{
		"seat":       perm.Controller,
		"sacrificed": count,
	})
}

// --- Merciless Executioner ---
//
// Oracle: When Merciless Executioner enters the battlefield, each player
// sacrifices a creature.
// 2B creature 3/1.
func registerMercilessExecutioner(r *Registry) {
	r.OnETB("Merciless Executioner", mercilessExecutionerETB)
}

func mercilessExecutionerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	count := eachPlayerSacrificeCreature(gs)
	emit(gs, "merciless_executioner", "Merciless Executioner", map[string]interface{}{
		"seat":       perm.Controller,
		"sacrificed": count,
	})
}

// --- Plaguecrafter ---
//
// Oracle: When Plaguecrafter enters the battlefield, each player
// sacrifices a creature or planeswalker, or discards a card.
// 2B creature 3/2.
func registerPlaguecrafter(r *Registry) {
	r.OnETB("Plaguecrafter", plaguecrafterETB)
}

func plaguecrafterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	for i := range gs.Seats {
		if gs.Seats[i] == nil || gs.Seats[i].Lost {
			continue
		}
		victim := pickWeakestCreature(gs, i)
		if victim != nil {
			gameengine.SacrificePermanent(gs, victim, "plaguecrafter")
		} else {
			gameengine.DiscardN(gs, i, 1, "")
		}
	}
	emit(gs, "plaguecrafter", "Plaguecrafter", map[string]interface{}{
		"seat": perm.Controller,
	})
}

// --- Innocent Blood ---
//
// Oracle: Each player sacrifices a creature.
// B sorcery.
func registerInnocentBlood(r *Registry) {
	r.OnResolve("Innocent Blood", innocentBloodResolve)
}

func innocentBloodResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	count := eachPlayerSacrificeCreature(gs)
	emit(gs, "innocent_blood", "Innocent Blood", map[string]interface{}{
		"seat":       item.Controller,
		"sacrificed": count,
	})
}

// --- Smallpox ---
//
// Oracle: Each player loses 1 life and discards a card, sacrifices a
// creature, then sacrifices a land.
// BB sorcery.
func registerSmallpox(r *Registry) {
	r.OnResolve("Smallpox", smallpoxResolve)
}

func smallpoxResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	for i := range gs.Seats {
		if gs.Seats[i] == nil || gs.Seats[i].Lost {
			continue
		}
		gs.Seats[i].Life--
		gameengine.DiscardN(gs, i, 1, "")
	}
	eachPlayerSacrificeCreature(gs)
	for i := range gs.Seats {
		if gs.Seats[i] == nil || gs.Seats[i].Lost {
			continue
		}
		land := pickWeakestLand(gs, i)
		if land != nil {
			gameengine.SacrificePermanent(gs, land, "smallpox")
		}
	}
	emit(gs, "smallpox", "Smallpox", map[string]interface{}{
		"seat": item.Controller,
	})
}

// --- Pox ---
//
// Oracle: Each player loses a third of their life, then discards a third
// of the cards in their hand, then sacrifices a third of the creatures
// they control, then sacrifices a third of the lands they control.
// Round up in each case.
// BBB sorcery.
func registerPox(r *Registry) {
	r.OnResolve("Pox", poxResolve)
}

func poxResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	for i := range gs.Seats {
		s := gs.Seats[i]
		if s == nil || s.Lost {
			continue
		}
		lifeLoss := (s.Life + 2) / 3
		s.Life -= lifeLoss

		handLoss := (len(s.Hand) + 2) / 3
		gameengine.DiscardN(gs, i, handLoss, "")

		creatures := 0
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				creatures++
			}
		}
		creatureLoss := (creatures + 2) / 3
		for j := 0; j < creatureLoss; j++ {
			victim := pickWeakestCreature(gs, i)
			if victim != nil {
				gameengine.SacrificePermanent(gs, victim, "pox")
			}
		}

		lands := 0
		for _, p := range s.Battlefield {
			if p != nil && p.IsLand() {
				lands++
			}
		}
		landLoss := (lands + 2) / 3
		for j := 0; j < landLoss; j++ {
			land := pickWeakestLand(gs, i)
			if land != nil {
				gameengine.SacrificePermanent(gs, land, "pox")
			}
		}
	}
	emit(gs, "pox", "Pox", map[string]interface{}{
		"seat": item.Controller,
	})
}

// --- Grave Pact ---
//
// Oracle: Whenever a creature you control dies, each other player
// sacrifices a creature.
// 1BBB enchantment.
func registerGravePact(r *Registry) {
	r.OnTrigger("Grave Pact", "creature_dies", gravePactTrigger)
}

func gravePactTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["grave_pact_resolving"] > 0 {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	gs.Flags["grave_pact_resolving"]++
	defer func() { gs.Flags["grave_pact_resolving"]-- }()
	count := eachOpponentSacrificeCreature(gs, perm.Controller)
	emit(gs, "grave_pact", "Grave Pact", map[string]interface{}{
		"seat":       perm.Controller,
		"sacrificed": count,
	})
}

// --- Grave Betrayal ---
//
// Oracle: Whenever a creature you don't control dies, return it to the
// battlefield under your control with an additional +1/+1 counter on it
// at the beginning of the next end step. That creature is a black Zombie
// in addition to its other colors and types.
// 5BB enchantment. Simplified: steal immediately instead of delayed trigger.
func registerGraveBetrayal(r *Registry) {
	r.OnTrigger("Grave Betrayal", "creature_dies", graveBetrayalTrigger)
}

func graveBetrayalTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat == perm.Controller {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if dyingPerm == nil || dyingPerm.Card == nil || dyingPerm.IsToken() {
		return
	}
	card := dyingPerm.Card
	oppSeat := controllerSeat
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
	if newPerm != nil {
		newPerm.AddCounter("+1/+1", 1)
	}
	emit(gs, "grave_betrayal", "Grave Betrayal", map[string]interface{}{
		"seat":   seat,
		"stolen": card.DisplayName(),
	})
}

// --- Living Death ---
//
// Oracle: Each player exiles all creature cards from their graveyard,
// then sacrifices all creatures they control, then puts all cards they
// exiled this way onto the battlefield.
// 3BB sorcery.
func registerLivingDeath(r *Registry) {
	r.OnResolve("Living Death", livingDeathResolve)
}

func livingDeathResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	exiled := make([][]*gameengine.Card, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		var kept []*gameengine.Card
		for _, c := range s.Graveyard {
			if c != nil && cardHasType(c, "creature") && !cardHasType(c, "token") {
				exiled[i] = append(exiled[i], c)
			} else {
				kept = append(kept, c)
			}
		}
		s.Graveyard = kept
	}

	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		var toSac []*gameengine.Permanent
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				toSac = append(toSac, p)
			}
		}
		for _, p := range toSac {
			gameengine.SacrificePermanent(gs, p, "living_death")
		}
		_ = i
	}

	totalReturned := 0
	for i, cards := range exiled {
		for _, c := range cards {
			enterBattlefieldWithETB(gs, i, c, false)
			totalReturned++
		}
	}
	emit(gs, "living_death", "Living Death", map[string]interface{}{
		"seat":     item.Controller,
		"returned": totalReturned,
	})
}

// --- Archfiend of Depravity ---
//
// Oracle: At the beginning of each opponent's end step, that player
// chooses up to two creatures they control, then sacrifices the rest.
// 3BB creature 5/4.
func registerArchfiendOfDepravity(r *Registry) {
	r.OnTrigger("Archfiend of Depravity", "upkeep_controller", archfiendOfDepravityTrigger)
}

func archfiendOfDepravityTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat == perm.Controller {
		return
	}
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	var creatures []*gameengine.Permanent
	for _, p := range gs.Seats[activeSeat].Battlefield {
		if p != nil && p.IsCreature() {
			creatures = append(creatures, p)
		}
	}
	if len(creatures) <= 2 {
		return
	}
	// Keep 2 strongest, sacrifice the rest.
	type cv struct {
		p   *gameengine.Permanent
		val int
	}
	ranked := make([]cv, len(creatures))
	for i, c := range creatures {
		ranked[i] = cv{c, c.Power() + c.Toughness()}
	}
	for i := 0; i < len(ranked)-1; i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].val > ranked[i].val {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}
	sacced := 0
	for k := 2; k < len(ranked); k++ {
		gameengine.SacrificePermanent(gs, ranked[k].p, "archfiend_of_depravity")
		sacced++
	}
	emit(gs, "archfiend_of_depravity", "Archfiend of Depravity", map[string]interface{}{
		"seat":       perm.Controller,
		"target":     activeSeat,
		"sacrificed": sacced,
	})
}

// --- Death Cloud ---
//
// Oracle: Each player loses X life, discards X cards, sacrifices X
// creatures, then sacrifices X lands.
// XBBB sorcery.
func registerDeathCloud(r *Registry) {
	r.OnResolve("Death Cloud", deathCloudResolve)
}

func deathCloudResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	x := 0
	if item.Card != nil {
		x = item.Card.CMC
		if x >= 3 {
			x -= 3
		}
	}
	if x <= 0 {
		x = 2
	}
	for i := range gs.Seats {
		s := gs.Seats[i]
		if s == nil || s.Lost {
			continue
		}
		s.Life -= x
		gameengine.DiscardN(gs, i, x, "")
		for j := 0; j < x; j++ {
			victim := pickWeakestCreature(gs, i)
			if victim != nil {
				gameengine.SacrificePermanent(gs, victim, "death_cloud")
			}
		}
		for j := 0; j < x; j++ {
			land := pickWeakestLand(gs, i)
			if land != nil {
				gameengine.SacrificePermanent(gs, land, "death_cloud")
			}
		}
	}
	emit(gs, "death_cloud", "Death Cloud", map[string]interface{}{
		"seat": item.Controller,
		"x":    x,
	})
}

// --- Victimize ---
//
// Oracle: Choose two target creature cards in your graveyard. Sacrifice
// a creature. If you do, return the chosen cards to the battlefield
// tapped.
// 2B sorcery.
func registerVictimize(r *Registry) {
	r.OnResolve("Victimize", victimizeResolve)
}

func victimizeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	victim := pickWeakestCreature(gs, seat)
	if victim == nil {
		emitFail(gs, "victimize", "Victimize", "no_creature_to_sacrifice", nil)
		return
	}
	var targets []*gameengine.Card
	for _, c := range gs.Seats[seat].Graveyard {
		if c != nil && cardHasType(c, "creature") && !cardHasType(c, "token") {
			targets = append(targets, c)
			if len(targets) == 2 {
				break
			}
		}
	}
	if len(targets) == 0 {
		emitFail(gs, "victimize", "Victimize", "no_graveyard_creatures", nil)
		return
	}
	gameengine.SacrificePermanent(gs, victim, "victimize")
	returned := 0
	for _, c := range targets {
		found := false
		for j, gc := range gs.Seats[seat].Graveyard {
			if gc == c {
				gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard[:j], gs.Seats[seat].Graveyard[j+1:]...)
				found = true
				break
			}
		}
		if found {
			enterBattlefieldWithETB(gs, seat, c, true)
			returned++
		}
	}
	emit(gs, "victimize", "Victimize", map[string]interface{}{
		"seat":     seat,
		"returned": returned,
	})
}

// --- Vona's Hunger ---
//
// Oracle: Ascend. Each opponent sacrifices a creature. If you have the
// city's blessing, instead each opponent sacrifices half the creatures
// they control, rounded up.
// 2B instant. MVP: always non-ascend mode (sac one each).
func registerVonasHunger(r *Registry) {
	r.OnResolve("Vona's Hunger", vonasHungerResolve)
}

func vonasHungerResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	count := eachOpponentSacrificeCreature(gs, item.Controller)
	emit(gs, "vonas_hunger", "Vona's Hunger", map[string]interface{}{
		"seat":       item.Controller,
		"sacrificed": count,
	})
}
