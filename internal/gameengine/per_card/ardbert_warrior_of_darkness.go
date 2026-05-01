package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArdbertWarriorOfDarkness wires Ardbert, Warrior of Darkness. Batch #33.
//
// Oracle text (Scryfall, verified 2026-05-01; Final Fantasy Commander):
//
//	{1}{W}{B} Legendary Creature — Spirit Warrior 2/2
//	Whenever you cast a white spell, put a +1/+1 counter on each
//	legendary creature you control. They gain vigilance until end of turn.
//	Whenever you cast a black spell, put a +1/+1 counter on each
//	legendary creature you control. They gain menace until end of turn.
//
// Implementation:
//   - Single OnTrigger("spell_cast") handler. Gates on caster_seat ==
//     controller and inspects card.Colors. White → +1/+1 counter +
//     vigilance UEOT on every legendary creature controller controls;
//     black → +1/+1 counter + menace UEOT. Multicolored W&B spells
//     trigger both branches independently (CR §603.2 — each ability
//     triggers off the same event).
//   - Keyword grant via Flags["kw:vigilance"] / Flags["kw:menace"]
//     (the leonardo_the_balance pattern), cleared by a next_end_step
//     delayed trigger.
//   - Trigger walks battlefield only — Ardbert herself can't trigger her
//     own ability while on the stack, which matches printed timing
//     ("Whenever you cast" fires after the spell is on the stack but the
//     listener has to be on the battlefield).
func registerArdbertWarriorOfDarkness(r *Registry) {
	r.OnTrigger("Ardbert, Warrior of Darkness", "spell_cast", ardbertWarriorOfDarknessSpellCast)
}

func ardbertWarriorOfDarknessSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ardbert_warrior_of_darkness_color_cast"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}

	hasW, hasB := false, false
	for _, c := range card.Colors {
		switch strings.ToUpper(strings.TrimSpace(c)) {
		case "W":
			hasW = true
		case "B":
			hasB = true
		}
	}
	if !hasW && !hasB {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	var legends []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "legendary") {
			continue
		}
		legends = append(legends, p)
	}
	if len(legends) == 0 {
		return
	}

	branches := []string{}
	if hasW {
		ardbertWarriorOfDarknessApply(gs, perm, legends, "vigilance")
		branches = append(branches, "white_vigilance")
	}
	if hasB {
		ardbertWarriorOfDarknessApply(gs, perm, legends, "menace")
		branches = append(branches, "black_menace")
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"spell":          card.DisplayName(),
		"legends_buffed": len(legends),
		"branches":       branches,
	})
}

func ardbertWarriorOfDarknessApply(gs *gameengine.GameState, src *gameengine.Permanent, legends []*gameengine.Permanent, keyword string) {
	flagKey := "kw:" + keyword
	captured := make([]*gameengine.Permanent, 0, len(legends))
	for _, p := range legends {
		if p == nil {
			continue
		}
		p.AddCounter("+1/+1", 1)
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags[flagKey] = 1
		captured = append(captured, p)
	}
	if len(captured) == 0 {
		return
	}
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: src.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			for _, p := range captured {
				if p == nil || p.Flags == nil {
					continue
				}
				delete(p.Flags, flagKey)
			}
		},
	})
}
