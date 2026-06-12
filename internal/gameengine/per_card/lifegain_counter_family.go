package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// lifegain_counter_family.go — generic handler for the
// "Whenever you gain life, put a +1/+1 counter on <target>" family.
//
// Shape (Celestial Unicorn, Exemplar of Light, Archangel of Thune, ...):
//
//	Whenever you gain life, put a +1/+1 counter on <target>.
//
// The variants differ only in:
//   - target: this creature ("self") vs each creature you control ("each_own")
//   - bonus draw: Exemplar of Light has a secondary "Whenever you put one
//     or more +1/+1 counters on this creature, draw a card. This ability
//     triggers only once each turn." We collapse the secondary into a
//     once-per-turn flag check inside the same handler — the trigger fires
//     in the same atomic frame as the counter placement so the engine
//     ordering is irrelevant.
//
// Hand-rolled siblings (Heliod, Sun-Crowned with its targeted picker, Karlov
// with its 2-counter doubler, Vito with its life-drain conversion) keep
// their bespoke handlers — this file only owns the gap cards whose effects
// fit the table.

type lifegainCounterTarget int

const (
	// Counter on this creature.
	lcTargetSelf lifegainCounterTarget = iota
	// Counter on each creature you control.
	lcTargetEachOwnCreature
)

type lifegainCounterEntry struct {
	cardName string
	target   lifegainCounterTarget
	// drawOnSelfCounter: if true, draw a card the first time a +1/+1
	// counter is placed on this creature each turn. Used by Exemplar of
	// Light's secondary ability.
	drawOnSelfCounter bool
}

var lifegainCounterEntries = []lifegainCounterEntry{
	// Celestial Unicorn intentionally NOT in this table — it has a
	// dedicated handler (celestial_unicorn.go) and BOTH fired per
	// life-gain event until r62, granting double +1/+1 counters (a
	// closure-vs-named-fn pair the registry's handler-identity dedupe
	// cannot collapse). One implementation per card.
	{
		// Exemplar of Light — {3}{W}, 3/3 Angel with flying.
		//   Whenever you gain life, put a +1/+1 counter on this creature.
		//   Whenever you put one or more +1/+1 counters on this creature,
		//   draw a card. This ability triggers only once each turn.
		cardName:          "Exemplar of Light",
		target:            lcTargetSelf,
		drawOnSelfCounter: true,
	},
	{
		// Archangel of Thune — {3}{W}{W}, 3/4 Angel with flying, lifelink.
		//   Whenever you gain life, put a +1/+1 counter on each creature
		//   you control.
		cardName: "Archangel of Thune",
		target:   lcTargetEachOwnCreature,
	},
}

func registerLifegainCounterFamily(r *Registry) {
	for _, e := range lifegainCounterEntries {
		e := e
		r.OnTrigger(e.cardName, "life_gained", func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			runLifegainCounter(gs, perm, e, ctx)
		})
	}
}

func runLifegainCounter(gs *gameengine.GameState, perm *gameengine.Permanent, e lifegainCounterEntry, ctx map[string]interface{}) {
	slug := "lifegain_counter_family:" + landFetchSlug(e.cardName)
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	counted := 0
	selfCountered := false
	switch e.target {
	case lcTargetSelf:
		perm.AddCounter("+1/+1", 1)
		counted = 1
		selfCountered = true
	case lcTargetEachOwnCreature:
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.AddCounter("+1/+1", 1)
			counted++
			if p == perm {
				selfCountered = true
			}
		}
	}
	gs.InvalidateCharacteristicsCache()

	drew := false
	if e.drawOnSelfCounter && selfCountered {
		// "Triggers only once each turn." Gate on a per-turn perm flag.
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		key := "exemplar_draw_fired_turn"
		if perm.Flags[key] != gs.Turn+1 {
			perm.Flags[key] = gs.Turn + 1
			drawOne(gs, perm.Controller, perm.Card.DisplayName())
			drew = true
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"amount":   amount,
		"counters": counted,
		"drew":     drew,
	})
}
