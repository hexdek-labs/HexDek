package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// lifegain_endstep_family.go — generic handler for the
// "At the beginning of your end step, if you gained life this turn,
// you may pay [cost], [effect]" family.
//
// Shape (Markov Purifier, Tivash/Gloom Summoner, ...):
//
//	Lifelink (often, not required).
//	At the beginning of your end step, if you gained life this turn,
//	you may pay <cost>. If you do, <effect>.
//
// Every member of the family shares one algorithm:
//   1. End-step trigger gated on activeSeat == controller.
//   2. Check seat.Turn.LifeGained > 0 (canonical per-turn counter,
//      reset in UntapAll — see state.go:480 TurnCounters).
//   3. Resolve optional cost (mana from ManaPool, or life paid through
//      gameengine.LoseLife).
//   4. Run the effect closure with whatever amount the cost referenced.
//
// Hand-rolled siblings already registered (Witch of the Moors, Lathiel,
// Bre of Clan Stoutarm) keep their bespoke implementations — this file
// only owns the gap cards that share the cleanest shape. Adding another
// family member is one entry in lifegainEndStepEntries.

// lifegainEndStepCost picks the optional cost the controller has to pay.
type lifegainEndStepCost int

const (
	// No cost — the effect fires whenever the lifegain gate opens.
	costNone lifegainEndStepCost = iota
	// Pay a fixed amount of generic mana from ManaPool. Skip if short.
	costFixedMana
	// Pay X life, where X is the amount of life gained this turn.
	// The effect closure receives that X as "amount".
	costLifeEqualsGain
)

// lifegainEndStepEntry — one row of the family table. The effect closure
// gets the controller seat plus the "amount" derived from the gate
// (life gained this turn) so it can scale the payoff naturally.
type lifegainEndStepEntry struct {
	cardName  string
	cost      lifegainEndStepCost
	manaCost  int // only used when cost == costFixedMana
	effect    func(gs *gameengine.GameState, perm *gameengine.Permanent, lifeGained, paid int)
	willPay   func(gs *gameengine.GameState, perm *gameengine.Permanent, lifeGained int) bool
	partial   string // optional emitPartial reason if the effect is approximated
}

var lifegainEndStepEntries = []lifegainEndStepEntry{
	{
		// Markov Purifier — {3}{W}{B}, lifelink.
		//   At the beginning of your end step, if you gained life this turn,
		//   you may pay {2}. If you do, draw a card.
		cardName: "Markov Purifier",
		cost:     costFixedMana,
		manaCost: 2,
		willPay:  markovPurifierWillPay,
		effect:   markovPurifierDrawCard,
	},
	{
		// Tivash, Gloom Summoner — {4}{B}{B}, lifelink.
		//   At the beginning of your end step, if you gained life this turn,
		//   you may pay X life, where X is the amount of life you gained
		//   this turn. If you do, create an X/X black Demon creature token
		//   with flying.
		cardName: "Tivash, Gloom Summoner",
		cost:     costLifeEqualsGain,
		willPay:  tivashWillPay,
		effect:   tivashCreateDemonToken,
	},
}

func registerLifegainEndStepFamily(r *Registry) {
	for _, e := range lifegainEndStepEntries {
		e := e
		r.OnTrigger(e.cardName, "end_step", func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			runLifegainEndStep(gs, perm, e, ctx)
		})
	}
}

func runLifegainEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, e lifegainEndStepEntry, ctx map[string]interface{}) {
	slug := "lifegain_endstep_family:" + landFetchSlug(e.cardName)
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	gained := seat.Turn.LifeGained
	if gained <= 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_life_gained_this_turn", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	if e.willPay != nil && !e.willPay(gs, perm, gained) {
		emitFail(gs, slug, perm.Card.DisplayName(), "declined_optional_cost", map[string]interface{}{
			"seat":         perm.Controller,
			"life_gained":  gained,
		})
		return
	}

	paid := 0
	switch e.cost {
	case costNone:
		// nothing
	case costFixedMana:
		if seat.ManaPool < e.manaCost {
			emitFail(gs, slug, perm.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
				"seat":      perm.Controller,
				"required":  e.manaCost,
				"available": seat.ManaPool,
			})
			return
		}
		seat.ManaPool -= e.manaCost
		gameengine.SyncManaAfterSpend(seat)
		paid = e.manaCost
	case costLifeEqualsGain:
		// Pay X life. Decline if we'd kill ourselves; willPay should
		// have already screened this, but double-check defensively.
		if seat.Life <= gained {
			emitFail(gs, slug, perm.Card.DisplayName(), "would_kill_self", map[string]interface{}{
				"seat":        perm.Controller,
				"life":        seat.Life,
				"life_to_pay": gained,
			})
			return
		}
		gameengine.LoseLife(gs, perm.Controller, gained, perm.Card.DisplayName())
		paid = gained
	}

	if e.effect != nil {
		e.effect(gs, perm, gained, paid)
	}
	_ = gs.CheckEnd()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"life_gained": gained,
		"paid":        paid,
	})
	if e.partial != "" {
		emitPartial(gs, slug, perm.Card.DisplayName(), e.partial)
	}
}

// ---------------------------------------------------------------------------
// Card bodies.
// ---------------------------------------------------------------------------

func markovPurifierWillPay(gs *gameengine.GameState, perm *gameengine.Permanent, lifeGained int) bool {
	// Pay {2} whenever we have it — drawing is monotone upside, and the
	// gate already required life gain (i.e. a real proc on the lifelink
	// commander, not a worthless tick).
	if perm == nil || gs == nil {
		return false
	}
	seat := gs.Seats[perm.Controller]
	return seat != nil && seat.ManaPool >= 2
}

func markovPurifierDrawCard(gs *gameengine.GameState, perm *gameengine.Permanent, lifeGained, paid int) {
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
}

func tivashWillPay(gs *gameengine.GameState, perm *gameengine.Permanent, lifeGained int) bool {
	// Pay X life iff we'll survive AND the resulting token is worth it
	// (X >= 1 is the floor; the engine gate already guarantees X >= 1).
	// Skip when we'd drop below 4 life since opening up a topdeck Bolt
	// or attacker kill for the difference of one Demon isn't worth it
	// at low life totals.
	if perm == nil || gs == nil {
		return false
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return false
	}
	// Need life > gained to satisfy "pay X life" without losing the game.
	if seat.Life <= lifeGained {
		return false
	}
	// Conservative life floor: don't drop below 4.
	if seat.Life-lifeGained < 4 {
		return false
	}
	return true
}

func tivashCreateDemonToken(gs *gameengine.GameState, perm *gameengine.Permanent, lifeGained, paid int) {
	if lifeGained <= 0 {
		return
	}
	// X/X black Demon creature token with flying — flying is a keyword
	// granted via the AST pipeline downstream of the type tag "flying"
	// when present, but CreateCreatureToken doesn't take a keyword set,
	// so flying is left implicit and emitPartial below documents the gap.
	tok := gameengine.CreateCreatureToken(gs, perm.Controller,
		"Demon Token", []string{"creature", "demon"}, lifeGained, lifeGained)
	if tok == nil {
		return
	}
	if tok.Flags == nil {
		tok.Flags = map[string]int{}
	}
	gameengine.SetKeywordFlags(gs, tok, "flying")
	tok.Card.Colors = []string{"B"}
}
