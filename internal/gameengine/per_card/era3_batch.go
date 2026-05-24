package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Era 3 unification batch — Streets of New Capenna, Brothers' War,
// Dominaria United, Kamigawa: Neon Dynasty, Battle for Baldur's Gate
// (2022–2023). Each handler stacks on top of an auto-generated stub
// from gen_*.go that emitPartials but does no real work; the live
// dispatcher fires every registered handler for a given (card, event)
// so the visible behaviour switches from a no-op to the implementation
// below.

// ---------------------------------------------------------------------------
// 1. Jetmir, Nexus of Revels (SNC)
//
//   Creatures you control get +1/+0 and have vigilance as long as you
//   control three or more creatures.
//   Creatures you control also get +1/+0 and have trample as long as
//   you control six or more creatures.
//   Creatures you control also get +1/+0 and have double strike as long
//   as you control nine or more creatures.
//
// True continuous effects need the layers system; we approximate by
// re-applying keyword flags whenever creature count on Jetmir's
// controller changes (creature_etb / creature_dies). +1/+0 anthem is
// emitted as partial — power is read off Card/Permanent without a flag
// hook in this engine.
// ---------------------------------------------------------------------------

func registerJetmirEra3(r *Registry) {
	r.OnETB("Jetmir, Nexus of Revels", jetmirEra3ETB)
	r.OnTrigger("Jetmir, Nexus of Revels", "permanent_etb", jetmirEra3Recheck)
	r.OnTrigger("Jetmir, Nexus of Revels", "creature_dies", jetmirEra3Recheck)
}

func jetmirEra3ETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	jetmirEra3Apply(gs, perm)
}

func jetmirEra3Recheck(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	jetmirEra3Apply(gs, perm)
}

func jetmirEra3Apply(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "jetmir_nexus_of_revels_tiers"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			count++
		}
	}
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if count >= 3 {
			p.Flags["kw:vigilance"] = 1
		} else {
			delete(p.Flags, "kw:vigilance")
		}
		if count >= 6 {
			p.Flags["kw:trample"] = 1
		} else {
			delete(p.Flags, "kw:trample")
		}
		if count >= 9 {
			p.Flags["kw:double strike"] = 1
		} else {
			delete(p.Flags, "kw:double strike")
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"creature_count": count,
	})
	if count >= 3 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"plus_one_plus_zero_anthem_not_applied_engine_layer_gap")
	}
}

// ---------------------------------------------------------------------------
// 2. Falco Spara, Pactweaver (SNC)
//
//   Flying, trample.
//   Falco Spara enters with three +1/+1 counters on it.
//   Pay 1 life and remove a +1/+1 counter from a creature you control:
//   You may play the top card of your library if it's a creature card.
//   Activate only as a sorcery.
//
// ETB stamps three +1/+1 counters; activated removes a counter, pays
// 1 life, and either plays the top card if it's a creature or marks
// the seat as having a top-of-library cast available this turn.
// ---------------------------------------------------------------------------

func registerFalcoSparaEra3(r *Registry) {
	r.OnETB("Falco Spara, Pactweaver", falcoSparaEra3ETB)
	r.OnActivated("Falco Spara, Pactweaver", falcoSparaEra3Activate)
}

func falcoSparaEra3ETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "falco_spara_pactweaver_etb_counters"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	perm.AddCounter("+1/+1", 3)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": 3,
	})
}

func falcoSparaEra3Activate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "falco_spara_pactweaver_topcast"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// Find a creature with a +1/+1 counter to remove.
	var donor *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Counters != nil && p.Counters["+1/+1"] > 0 {
			donor = p
			break
		}
	}
	if donor == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_plus1_counter_to_remove", nil)
		return
	}
	donor.Counters["+1/+1"]--
	if donor.Counters["+1/+1"] == 0 {
		delete(donor.Counters, "+1/+1")
	}
	gs.InvalidateCharacteristicsCache()
	gameengine.LoseLife(gs, src.Controller, 1, src.Card.DisplayName())

	played := ""
	if len(seat.Library) > 0 {
		top := seat.Library[0]
		if top != nil && cardHasType(top, "creature") {
			gameengine.MoveCard(gs, top, src.Controller, "library", "hand", "falco_top_to_hand")
			played = top.DisplayName()
		}
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["falco_top_play_available"] = 1
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"donor":  donor.Card.DisplayName(),
		"played": played,
	})
}

// ---------------------------------------------------------------------------
// 3. Lord Xander, the Collector (SNC)
//
//   When Lord Xander enters, target opponent discards three cards.
//   Whenever Lord Xander attacks, defending player mills half their
//   library, rounded down.
//   When Lord Xander dies, target opponent sacrifices half the nonland
//   permanents they control, rounded down.
// ---------------------------------------------------------------------------

func registerLordXanderEra3(r *Registry) {
	r.OnETB("Lord Xander, the Collector", lordXanderEra3ETB)
	r.OnTrigger("Lord Xander, the Collector", "creature_attacks", lordXanderEra3Attack)
	r.OnTrigger("Lord Xander, the Collector", "creature_dies", lordXanderEra3Dies)
}

func lordXanderEra3PickOpponent(gs *gameengine.GameState, controller int) int {
	for _, opp := range gs.LivingOpponents(controller) {
		return opp
	}
	return -1
}

func lordXanderEra3ETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lord_xander_etb_discard"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	target := lordXanderEra3PickOpponent(gs, perm.Controller)
	if target < 0 {
		return
	}
	tseat := gs.Seats[target]
	discarded := 0
	for i := 0; i < 3 && len(tseat.Hand) > 0; i++ {
		card := tseat.Hand[0]
		gameengine.DiscardCard(gs, card, target)
		discarded++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"victim":    target,
		"discarded": discarded,
	})
}

func lordXanderEra3Attack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lord_xander_attack_mill"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	defenderSeat := -1
	if d, ok := gameengine.AttackerDefender(perm); ok {
		defenderSeat = d
	}
	if defenderSeat < 0 {
		defenderSeat = lordXanderEra3PickOpponent(gs, perm.Controller)
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	dseat := gs.Seats[defenderSeat]
	half := len(dseat.Library) / 2
	for i := 0; i < half && len(dseat.Library) > 0; i++ {
		card := dseat.Library[0]
		gameengine.MoveCard(gs, card, defenderSeat, "library", "graveyard", "lord_xander_mill")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"defender": defenderSeat,
		"milled":   half,
	})
}

func lordXanderEra3Dies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lord_xander_dies_sac"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying != perm {
		// Also accept identity-by-name in case the engine already moved
		// the perm pointer off the battlefield before firing the trigger.
		card, _ := ctx["card"].(*gameengine.Card)
		if card == nil || perm.Card == nil || card.DisplayName() != perm.Card.DisplayName() {
			return
		}
	}
	target := lordXanderEra3PickOpponent(gs, perm.Controller)
	if target < 0 {
		return
	}
	tseat := gs.Seats[target]
	var nonlands []*gameengine.Permanent
	for _, p := range tseat.Battlefield {
		if p == nil || p.IsLand() {
			continue
		}
		nonlands = append(nonlands, p)
	}
	half := len(nonlands) / 2
	sacced := 0
	for i := 0; i < half; i++ {
		victim := nonlands[i]
		if victim == nil {
			continue
		}
		removePermanent(gs, victim)
		gameengine.DetachAll(gs, victim)
		if victim.Card != nil {
			gameengine.MoveCard(gs, victim.Card, target, "battlefield", "graveyard", "lord_xander_sac")
		}
		sacced++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"victim":       target,
		"nonlands":     len(nonlands),
		"sacrificed":   sacced,
	})
}

// ---------------------------------------------------------------------------
// 4. Hidetsugu and Kairi (NEO)
//
//   Flying.
//   When Hidetsugu and Kairi enters, draw three cards, then put two
//   cards from your hand on top of your library in any order.
//   When Hidetsugu and Kairi dies, exile the top card of your library.
//   Target opponent loses life equal to its mana value. If it's an
//   instant or sorcery card, you may cast it without paying its mana
//   cost.
//
// gen_*.go already drew 3 — we add the put-2-back on top and the
// dies → drain effect.
// ---------------------------------------------------------------------------

func registerHidetsuguAndKairiEra3(r *Registry) {
	r.OnETB("Hidetsugu and Kairi", hidetsuguEra3ETB)
	r.OnTrigger("Hidetsugu and Kairi", "creature_dies", hidetsuguEra3Dies)
}

func hidetsuguEra3ETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hidetsugu_kairi_etb_topdeck"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	moved := 0
	for i := 0; i < 2 && len(seat.Hand) > 0; i++ {
		card := seat.Hand[len(seat.Hand)-1]
		gameengine.MoveCard(gs, card, perm.Controller, "hand", "library", "hidetsugu_top")
		moved++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"to_top":    moved,
	})
}

func hidetsuguEra3Dies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hidetsugu_kairi_dies_drain"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || len(seat.Library) == 0 {
		return
	}
	top := seat.Library[0]
	if top == nil {
		return
	}
	gameengine.MoveCard(gs, top, perm.Controller, "library", "exile", "hidetsugu_dies_exile")
	mv := cardCMC(top)
	target := lordXanderEra3PickOpponent(gs, perm.Controller)
	if target >= 0 {
		gameengine.LoseLife(gs, target, mv, perm.Card.DisplayName())
	}
	if cardHasType(top, "instant") || cardHasType(top, "sorcery") {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"free_cast_of_exiled_instant_or_sorcery_not_modeled")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"victim":    target,
		"exiled":    top.DisplayName(),
		"mv":        mv,
	})
}

// ---------------------------------------------------------------------------
// 5. Shorikai, Genesis Engine (NEO)
//
//   Crew 3.
//   {1}, {T}: Draw two cards, then discard a card. Create a 1/1
//   colorless Pilot creature token.
// ---------------------------------------------------------------------------

func registerShorikaiEra3(r *Registry) {
	r.OnActivated("Shorikai, Genesis Engine", shorikaiEra3Activate)
}

func shorikaiEra3Activate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "shorikai_genesis_engine_loot"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	drawOne(gs, src.Controller, src.Card.DisplayName())
	drawOne(gs, src.Controller, src.Card.DisplayName())
	if len(seat.Hand) > 0 {
		gameengine.DiscardCard(gs, seat.Hand[len(seat.Hand)-1], src.Controller)
	}
	pilot := &gameengine.Card{
		Name:          "Pilot Token",
		Owner:         src.Controller,
		Types:         []string{"creature", "token", "pilot"},
		BasePower:     1,
		BaseToughness: 1,
	}
	enterBattlefieldWithETB(gs, src.Controller, pilot, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// ---------------------------------------------------------------------------
// 6. Acererak the Archlich (CLB)
//
//   Whenever Acererak the Archlich attacks, for each opponent, you
//   create a 2/2 black Zombie creature token unless that player
//   sacrifices a creature of their choice.
//
// In our model opponents have no choice hook for arbitrary "sac or
// take" prompts; we pessimise for them and always create the tokens.
// ---------------------------------------------------------------------------

func registerAcererakEra3(r *Registry) {
	r.OnTrigger("Acererak the Archlich", "creature_attacks", acererakEra3Attack)
}

func acererakEra3Attack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "acererak_attack_zombies"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	tokens := 0
	for _, opp := range gs.LivingOpponents(perm.Controller) {
		_ = opp
		zombie := &gameengine.Card{
			Name:          "Zombie Token",
			Owner:         perm.Controller,
			Types:         []string{"creature", "token", "zombie", "pip:B"},
			Colors:        []string{"B"},
			BasePower:     2,
			BaseToughness: 2,
		}
		enterBattlefieldWithETB(gs, perm.Controller, zombie, false)
		tokens++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tokens": tokens,
	})
}

// ---------------------------------------------------------------------------
// 7. Tazri, Beacon of Unity (CLB)
//
//   {2/W}{2/U}{2/B}{2/R}{2/G}: Look at the top six cards of your
//   library. You may reveal up to two Cleric, Rogue, Warrior, Wizard,
//   and/or Ally creature cards from among them and put them into your
//   hand. Put the rest on the bottom of your library in a random order.
//
// Activated handler picks up to two matching cards from the top six.
// ---------------------------------------------------------------------------

func registerTazriEra3(r *Registry) {
	r.OnActivated("Tazri, Beacon of Unity", tazriEra3Activate)
}

var tazriEra3PartySubtypes = []string{"cleric", "rogue", "warrior", "wizard", "ally"}

func tazriEra3IsParty(card *gameengine.Card) bool {
	if card == nil || !cardHasType(card, "creature") {
		return false
	}
	for _, st := range tazriEra3PartySubtypes {
		if cardHasType(card, st) {
			return true
		}
	}
	return false
}

func tazriEra3Activate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "tazri_beacon_party_search"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	picked := 0
	scanned := 0
	for i := 0; i < len(seat.Library) && scanned < 6 && picked < 2; i++ {
		card := seat.Library[i]
		scanned++
		if !tazriEra3IsParty(card) {
			continue
		}
		gameengine.MoveCard(gs, card, src.Controller, "library", "hand", "tazri_party_search")
		picked++
		i--
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    src.Controller,
		"picked":  picked,
		"scanned": scanned,
	})
}

// ---------------------------------------------------------------------------
// 8. Sivriss, Nightmare Speaker (CLB)
//
//   {T}, Sacrifice another creature or an artifact: For each opponent,
//   you mill a card, then return that card from your graveyard to your
//   hand unless that player pays 3 life.
//
// We model "opponent always pays" by default — the milled cards stay
// in the graveyard. If a flag "sivriss_opp_skip_pay" is set on the
// active seat, we instead return the milled cards. This mirrors the
// existing per-card pattern of letting tests drive the branch directly.
// ---------------------------------------------------------------------------

func registerSivrissEra3(r *Registry) {
	r.OnActivated("Sivriss, Nightmare Speaker", sivrissEra3Activate)
}

func sivrissEra3Activate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sivriss_nightmare_mill_each_opp"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	milled := 0
	returned := 0
	lifeLost := 0
	skipPay := false
	if seat.Flags != nil && seat.Flags["sivriss_opp_skip_pay"] == 1 {
		skipPay = true
	}
	opps := gs.LivingOpponents(src.Controller)
	for range opps {
		if len(seat.Library) == 0 {
			break
		}
		top := seat.Library[0]
		gameengine.MoveCard(gs, top, src.Controller, "library", "graveyard", "sivriss_mill")
		milled++
		if skipPay {
			gameengine.MoveCard(gs, top, src.Controller, "graveyard", "hand", "sivriss_return")
			returned++
		}
	}
	if !skipPay {
		for _, opp := range opps {
			gameengine.LoseLife(gs, opp, 3, src.Card.DisplayName())
			lifeLost += 3
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"milled":   milled,
		"returned": returned,
		"life":     lifeLost,
	})
}

// ---------------------------------------------------------------------------
// 9. Urza, Prince of Kroog (BRO)
//
//   Artifact creatures you control get +2/+2.
//   {6}: Create a token that's a copy of target artifact you control,
//   except it's a 1/1 Soldier creature in addition to its other types.
//
// Activated handler builds a token copy of an artifact and overrides
// its P/T to 1/1 + creature/soldier.
// ---------------------------------------------------------------------------

func registerUrzaPrinceEra3(r *Registry) {
	r.OnActivated("Urza, Prince of Kroog", urzaPrinceEra3Activate)
	r.OnETB("Urza, Prince of Kroog", urzaPrinceEra3ETB)
	r.OnTrigger("Urza, Prince of Kroog", "permanent_etb", urzaPrinceEra3Recheck)
	r.OnTrigger("Urza, Prince of Kroog", "creature_dies", urzaPrinceEra3Recheck)
}

func urzaPrinceEra3ETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	urzaPrinceEra3ApplyAnthem(gs, perm)
}

func urzaPrinceEra3Recheck(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	urzaPrinceEra3ApplyAnthem(gs, perm)
}

func urzaPrinceEra3ApplyAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() || !p.IsArtifact() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["urza_prince_anthem"] = 1
	}
	emitPartial(gs, "urza_prince_kroog_anthem", perm.Card.DisplayName(),
		"plus_two_plus_two_anthem_marked_via_flag_only")
}

func urzaPrinceEra3Activate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "urza_prince_kroog_artifact_token"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	var target *gameengine.Permanent
	if t, ok := ctx["target_perm"].(*gameengine.Permanent); ok {
		target = t
	}
	if target == nil {
		for _, p := range seat.Battlefield {
			if p == nil || p == src || !p.IsArtifact() {
				continue
			}
			target = p
			break
		}
	}
	if target == nil || target.Card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_artifact_to_copy", nil)
		return
	}
	tokenCard := target.Card.DeepCopy()
	tokenCard.Owner = src.Controller
	tokenCard.IsCopy = true
	tokenCard.Name = target.Card.DisplayName() + " (Urza copy)"
	tokenCard.BasePower = 1
	tokenCard.BaseToughness = 1
	hasCreature := false
	hasSoldier := false
	hasToken := false
	for _, t := range tokenCard.Types {
		switch t {
		case "creature":
			hasCreature = true
		case "soldier":
			hasSoldier = true
		case "token":
			hasToken = true
		}
	}
	if !hasCreature {
		tokenCard.Types = append(tokenCard.Types, "creature")
	}
	if !hasSoldier {
		tokenCard.Types = append(tokenCard.Types, "soldier")
	}
	if !hasToken {
		tokenCard.Types = append(tokenCard.Types, "token")
	}
	enterBattlefieldWithETB(gs, src.Controller, tokenCard, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"copied": target.Card.DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// 10. Plargg and Nassari (CLB)
//
//   At the beginning of your upkeep, each player exiles cards from
//   the top of their library until they exile a nonland card. An
//   opponent chooses a nonland card exiled this way. You may cast up
//   to two spells from among the other cards exiled this way without
//   paying their mana costs.
//
// Implementation: on the controller's upkeep, walk every player's
// library top-down, moving cards to exile until a nonland surfaces.
// Tag the exiled nonland cards from the controller as castable-for-free.
// ---------------------------------------------------------------------------

func registerPlarggNassariEra3(r *Registry) {
	r.OnTrigger("Plargg and Nassari", "upkeep_controller", plarggNassariEra3Upkeep)
}

func plarggNassariEra3Upkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "plargg_nassari_upkeep_exile"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	freeCastable := 0
	exiledTotal := 0
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for len(s.Library) > 0 {
			card := s.Library[0]
			gameengine.MoveCard(gs, card, i, "library", "exile", "plargg_nassari_exile")
			exiledTotal++
			if card != nil && !cardHasType(card, "land") {
				if i == perm.Controller {
					freeCastable++
					if card.Types == nil {
						card.Types = []string{}
					}
					card.Types = append(card.Types, "plargg_free_cast")
				}
				break
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"exiled":        exiledTotal,
		"free_castable": freeCastable,
	})
}

// ---------------------------------------------------------------------------
// 11. Will, Scion of Peace (CLB)
//
//   Vigilance.
//   {T}: White and blue spells you cast this turn cost {X} less to
//   cast, where X is the amount of life you gained this turn.
//   Activate only as a sorcery.
//
// Activated handler stamps the per-turn cost reduction on the seat
// using the engine's life-gain ledger. Cost-modifier hookup is the
// engine's job; we record the value and emit a partial for the wiring.
// ---------------------------------------------------------------------------

func registerWillScionEra3(r *Registry) {
	r.OnActivated("Will, Scion of Peace", willScionEra3Activate)
}

func willScionEra3Activate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "will_scion_peace_cost_reduction"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	gain := 0
	if seat.Flags != nil {
		gain = seat.Flags["life_gained_this_turn"]
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["will_scion_wu_discount"] = gain
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             src.Controller,
		"discount":         gain,
		"life_gained_turn": gain,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"cost_modifier_pipeline_hookup_pending_engine_layer_gap")
}

// ---------------------------------------------------------------------------
// 12. Felothar the Steadfast (CLB)
//
//   Each creature you control assigns combat damage equal to its
//   toughness rather than its power.
//   Creatures you control can attack as though they didn't have
//   defender.
//   {3}, {T}, Sacrifice another creature: Draw cards equal to the
//   sacrificed creature's toughness, then discard cards equal to its
//   power.
//
// Activated handler picks a creature to sacrifice (other than Felothar),
// then does the asymmetric loot.
// ---------------------------------------------------------------------------

func registerFelotharEra3(r *Registry) {
	// OnActivated is owned by custom_felothar_steadfast.go (custom wins
	// over era3 stub per QA cleanup). This handler keeps the static
	// "ignore defender / damage as toughness" marker layer.
	r.OnETB("Felothar the Steadfast", felotharEra3MarkDefenders)
	r.OnTrigger("Felothar the Steadfast", "permanent_etb", felotharEra3MarkDefendersTrigger)
}

func felotharEra3MarkDefenders(gs *gameengine.GameState, perm *gameengine.Permanent) {
	felotharEra3Apply(gs, perm)
}

func felotharEra3MarkDefendersTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	felotharEra3Apply(gs, perm)
}

func felotharEra3Apply(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["felothar_attack_with_defender"] = 1
		p.Flags["felothar_damage_by_toughness"] = 1
	}
}

