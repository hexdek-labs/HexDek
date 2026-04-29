package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Forced-discard spells and enchantments.
//
// Many of these had no working resolution because the AST parser produced
// unstructured nodes (parsed_tail, parsed_effect_residual). These per-card
// handlers bypass the generic resolver entirely.
// ============================================================================

// --- Hymn to Tourach ---
//
// Oracle: Target player discards two cards at random.
// BB sorcery.
func registerHymnToTourach(r *Registry) {
	r.OnResolve("Hymn to Tourach", hymnToTourachResolve)
}

func hymnToTourachResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	opps := gs.Opponents(item.Controller)
	if len(opps) == 0 {
		return
	}
	target := opps[0]
	if gs.Rng != nil && len(opps) > 1 {
		target = opps[gs.Rng.Intn(len(opps))]
	}
	discarded := gameengine.DiscardN(gs, target, 2, "random")
	emit(gs, "hymn_to_tourach", "Hymn to Tourach", map[string]interface{}{
		"seat":      item.Controller,
		"target":    target,
		"discarded": discarded,
	})
}

// --- Mind Twist ---
//
// Oracle: Target player discards X cards at random.
// XB sorcery.
func registerMindTwist(r *Registry) {
	r.OnResolve("Mind Twist", mindTwistResolve)
}

func mindTwistResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	x := 0
	if item.Card != nil {
		x = item.Card.CMC
	}
	if x <= 0 {
		x = 3
	}
	opps := gs.Opponents(item.Controller)
	if len(opps) == 0 {
		return
	}
	target := opps[0]
	if gs.Rng != nil && len(opps) > 1 {
		target = opps[gs.Rng.Intn(len(opps))]
	}
	discarded := gameengine.DiscardN(gs, target, x, "random")
	emit(gs, "mind_twist", "Mind Twist", map[string]interface{}{
		"seat":      item.Controller,
		"target":    target,
		"x":         x,
		"discarded": discarded,
	})
}

// --- Dark Deal ---
//
// Oracle: Each player discards all the cards in their hand,
// then draws that many cards minus one.
// 2B sorcery.
func registerDarkDeal(r *Registry) {
	r.OnResolve("Dark Deal", darkDealResolve)
}

func darkDealResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	for i, seat := range gs.Seats {
		if seat == nil || seat.Lost {
			continue
		}
		handSize := len(seat.Hand)
		if handSize == 0 {
			continue
		}
		gameengine.DiscardN(gs, i, handSize, "")
		drawCount := handSize - 1
		for j := 0; j < drawCount; j++ {
			drawOne(gs, i, "Dark Deal")
		}
	}
	emit(gs, "dark_deal", "Dark Deal", map[string]interface{}{
		"seat": item.Controller,
	})
}

// --- Delirium Skeins ---
//
// Oracle: Each player discards three cards.
// 2B sorcery.
func registerDeliriumSkeins(r *Registry) {
	r.OnResolve("Delirium Skeins", deliriumSkeinsResolve)
}

func deliriumSkeinsResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	totalDiscarded := 0
	for i, seat := range gs.Seats {
		if seat == nil || seat.Lost {
			continue
		}
		totalDiscarded += gameengine.DiscardN(gs, i, 3, "")
	}
	emit(gs, "delirium_skeins", "Delirium Skeins", map[string]interface{}{
		"seat":      item.Controller,
		"discarded": totalDiscarded,
	})
}

// --- Syphon Mind ---
//
// Oracle: Each other player discards a card. You draw a card for
// each card discarded this way.
// 3B sorcery.
func registerSyphonMind(r *Registry) {
	r.OnResolve("Syphon Mind", syphonMindResolve)
}

func syphonMindResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	totalDiscarded := 0
	for _, opp := range gs.Opponents(seat) {
		totalDiscarded += gameengine.DiscardN(gs, opp, 1, "")
	}
	for i := 0; i < totalDiscarded; i++ {
		drawOne(gs, seat, "Syphon Mind")
	}
	emit(gs, "syphon_mind", "Syphon Mind", map[string]interface{}{
		"seat":      seat,
		"discarded": totalDiscarded,
		"drawn":     totalDiscarded,
	})
}

// --- Necrogen Mists ---
//
// Oracle: At the beginning of each player's upkeep, that player
// discards a card.
// 2B enchantment. Upkeep trigger (fires for ALL players, not just controller).
func registerNecrogenMists(r *Registry) {
	r.OnTrigger("Necrogen Mists", "upkeep_controller", necrogenMistsUpkeep)
}

func necrogenMistsUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	gameengine.DiscardN(gs, activeSeat, 1, "")
	emit(gs, "necrogen_mists", "Necrogen Mists", map[string]interface{}{
		"controller": perm.Controller,
		"target":     activeSeat,
	})
}

// --- Bottomless Pit ---
//
// Oracle: At the beginning of each player's upkeep, that player
// discards a card at random.
// 1BB enchantment.
func registerBottomlessPit(r *Registry) {
	r.OnTrigger("Bottomless Pit", "upkeep_controller", bottomlessPitUpkeep)
}

func bottomlessPitUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	gameengine.DiscardN(gs, activeSeat, 1, "random")
	emit(gs, "bottomless_pit", "Bottomless Pit", map[string]interface{}{
		"controller": perm.Controller,
		"target":     activeSeat,
	})
}

// --- Rankle, Master of Pranks ---
//
// Already has a working AST-driven trigger, but adding explicit handler
// for reliability. Oracle: When Rankle deals combat damage to a player,
// choose any number —
//   - Each player discards a card.
//   - Each player loses 1 life and draws a card.
//   - Each player sacrifices a creature.
func registerRankleMasterOfPranks(r *Registry) {
	r.OnTrigger("Rankle, Master of Pranks", "combat_damage_player", rankleTrigger)
}

func rankleTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller || sourceName != "Rankle, Master of Pranks" {
		return
	}
	for i, seat := range gs.Seats {
		if seat == nil || seat.Lost {
			continue
		}
		gameengine.DiscardN(gs, i, 1, "")
	}
	for i, seat := range gs.Seats {
		if seat == nil || seat.Lost {
			continue
		}
		seat.Life--
		drawOne(gs, i, "Rankle, Master of Pranks")
	}
	emit(gs, "rankle", "Rankle, Master of Pranks", map[string]interface{}{
		"seat": perm.Controller,
	})
}

// --- Waste Not ---
//
// Oracle: Whenever an opponent discards a creature card, create a 2/2
// black Zombie creature token. Whenever an opponent discards a land card,
// add BB. Whenever an opponent discards a noncreature, nonland card,
// draw a card.
// 1B enchantment.
func registerWasteNot(r *Registry) {
	r.OnTrigger("Waste Not", "card_discarded", wasteNotTrigger)
}

func wasteNotTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat == perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	seat := perm.Controller
	if cardHasType(card, "creature") {
		token := &gameengine.Card{
			Name:          "Zombie",
			Owner:         seat,
			BasePower:     2,
			BaseToughness: 2,
			Types:         []string{"token", "creature", "zombie"},
		}
		enterBattlefieldWithETB(gs, seat, token, false)
		emit(gs, "waste_not_zombie", "Waste Not", map[string]interface{}{
			"seat":     seat,
			"discarded": card.DisplayName(),
		})
	} else if cardHasType(card, "land") {
		if seat >= 0 && seat < len(gs.Seats) {
			gs.Seats[seat].ManaPool += 2
		}
		emit(gs, "waste_not_mana", "Waste Not", map[string]interface{}{
			"seat":     seat,
			"discarded": card.DisplayName(),
		})
	} else {
		drawOne(gs, seat, "Waste Not")
		emit(gs, "waste_not_draw", "Waste Not", map[string]interface{}{
			"seat":     seat,
			"discarded": card.DisplayName(),
		})
	}
}

// --- Liliana's Caress ---
//
// Oracle: Whenever an opponent discards a card, that player loses 2 life.
// 1B enchantment.
func registerLilianasCaress(r *Registry) {
	r.OnTrigger("Liliana's Caress", "card_discarded", lilianasCaressTrigger)
}

func lilianasCaressTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat == perm.Controller {
		return
	}
	if discarderSeat >= 0 && discarderSeat < len(gs.Seats) {
		gs.Seats[discarderSeat].Life -= 2
	}
	emit(gs, "lilianas_caress", "Liliana's Caress", map[string]interface{}{
		"seat":        perm.Controller,
		"target":      discarderSeat,
		"damage":      2,
	})
}

// --- Megrim ---
//
// Oracle: Whenever an opponent discards a card, Megrim deals 2 damage
// to that player.
// 2B enchantment.
func registerMegrim(r *Registry) {
	r.OnTrigger("Megrim", "card_discarded", megrimTrigger)
}

func megrimTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat == perm.Controller {
		return
	}
	if discarderSeat >= 0 && discarderSeat < len(gs.Seats) {
		gs.Seats[discarderSeat].Life -= 2
	}
	emit(gs, "megrim", "Megrim", map[string]interface{}{
		"seat":   perm.Controller,
		"target": discarderSeat,
		"damage": 2,
	})
}

// --- Tinybones, Trinket Thief ---
//
// Oracle: At the beginning of each end step, if an opponent discarded
// a card this turn, you draw a card and each opponent loses 1 life.
// {4}{B}{B}: Each opponent with no cards in hand loses 10 life.
// 1B legendary creature.
func registerTinybones(r *Registry) {
	r.OnTrigger("Tinybones, Trinket Thief", "card_discarded", tinybonesTracker)
}

func tinybonesTracker(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat == perm.Controller {
		return
	}
	key := "tinybones_opp_discarded"
	if gs.Flags == nil {
		gs.Flags = make(map[string]int)
	}
	gs.Flags[key] = 1
}

// --- Oppression ---
//
// Oracle: Whenever a player casts a spell, that player discards a card.
// 1BB enchantment. Already has working AST but adding explicit handler.
func registerOppression(r *Registry) {
	r.OnTrigger("Oppression", "spell_cast", oppressionTrigger)
}

func oppressionTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	gameengine.DiscardN(gs, casterSeat, 1, "")
	emit(gs, "oppression", "Oppression", map[string]interface{}{
		"controller": perm.Controller,
		"target":     casterSeat,
	})
}
