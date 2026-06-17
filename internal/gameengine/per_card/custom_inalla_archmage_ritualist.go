package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerInallaCustom adds Inalla's eminence Wizard-copy trigger and the
// "tap five wizards: drain 7" activated ability that the auto-generated
// stubs omit.
//
// Oracle text:
//
//	Eminence — Whenever another nontoken Wizard you control enters, if
//	Inalla is in the command zone or on the battlefield, you may pay
//	{1}. If you do, create a token that's a copy of that Wizard. The
//	token gains haste. Exile it at the beginning of the next end step.
//	Tap five untapped Wizards you control: Target player loses 7 life.
//
// The eminence trigger fires when a nontoken Wizard ETBs and we have
// ≥1 mana to spare; we always pay since the haste copy compounds value
// in simulation. The activated ability is wired to require exactly five
// untapped wizards (no partial — the cost can't be reduced).
func registerInallaCustom(r *Registry) {
	r.OnTrigger("Inalla, Archmage Ritualist", "permanent_etb", inallaEminenceCopy)
	r.OnActivated("Inalla, Archmage Ritualist", inallaTapFiveDrain)
}

func inallaEminenceCopy(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "inalla_eminence_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered == perm || entered.Card == nil {
		return
	}
	if entered.Controller != perm.Controller {
		return
	}
	if entered.IsToken() {
		return
	}
	if !entered.IsCreature() {
		return
	}
	if !cardSubtypeMatches(entered.Card, "wizard") {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.ManaPool < 1 {
		emitFail(gs, slug, perm.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seatPool(seat),
		})
		return
	}
	seat.ManaPool -= 1
	gameengine.SyncManaAfterSpend(seat)
	// CR §603 optional eminence-trigger cost — credit it to any in-flight
	// legality window for this seat (Hashaton/extort aux-spend class,
	// legality.go NoteManaSpend). This trigger fires INSIDE the window of
	// whatever made a Wizard enter — including a Wizard with persist that
	// sacrificed itself as an activation cost and re-entered mid-cost (Glen
	// Elendra Archmage's {U},Sac counter ability); without this credit the {1}
	// reads as that activation over-paying its announced total.
	gs.Legality.NoteManaSpend(perm.Controller, 1)
	tokenCard := &gameengine.Card{
		Name:          entered.Card.DisplayName() + " (Inalla token)",
		Owner:         perm.Controller,
		Types:         append([]string{"token"}, entered.Card.Types...),
		BasePower:     entered.Card.BasePower,
		BaseToughness: entered.Card.BaseToughness,
	}
	tokenPerm := &gameengine.Permanent{
		Card:       tokenCard,
		Controller: perm.Controller,
		Owner:      perm.Controller,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{"kw:haste": 1},
	}
	seat.Battlefield = append(seat.Battlefield, tokenPerm)
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.MoveCard(gs, tokenCard, perm.Controller, "battlefield", "exile", "inalla_eos_exile")
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"copied":    entered.Card.DisplayName(),
		"mana_paid": 1,
	})
}

func inallaTapFiveDrain(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "inalla_tap5_drain"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	wizards := []*gameengine.Permanent{}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.Tapped {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if !cardSubtypeMatches(p.Card, "wizard") {
			continue
		}
		wizards = append(wizards, p)
		if len(wizards) == 5 {
			break
		}
	}
	if len(wizards) < 5 {
		emitFail(gs, slug, src.Card.DisplayName(), "fewer_than_5_untapped_wizards", map[string]interface{}{
			"available": len(wizards),
		})
		return
	}
	for _, w := range wizards {
		w.Tapped = true
	}
	target := -1
	if v, ok := ctx["target_seat"].(int); ok {
		target = v
	}
	if target < 0 || target >= len(gs.Seats) {
		opps := gs.Opponents(src.Controller)
		if len(opps) > 0 {
			target = opps[0]
		}
	}
	if target < 0 || target >= len(gs.Seats) {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target", nil)
		return
	}
	gameengine.LoseLife(gs, target, 7, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":        src.Controller,
		"target_seat": target,
		"life_lost":   7,
	})
}

// cardSubtypeMatches returns true if `c.Types` contains the subtype
// (case-insensitive). Wizard / Angel / Dragon etc. are stored as Types
// entries by the corpus loader.
func cardSubtypeMatches(c *gameengine.Card, sub string) bool {
	if c == nil {
		return false
	}
	want := strings.ToLower(sub)
	for _, t := range c.Types {
		if strings.ToLower(t) == want {
			return true
		}
	}
	return false
}

func seatPool(s *gameengine.Seat) int {
	if s == nil {
		return 0
	}
	return s.ManaPool
}
