package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBeastWhisperer wires Beast Whisperer.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Beast%20Whisperer):
//
//	Whenever you cast a creature spell, draw a card.
//
// {2}{G}{G} 2/3 Creature — Elf Druid. Elf-tribal and creature-heavy
// staple — every creature you cast cantrips. Sibling effects:
// Glimpse of Nature, Lifecrafter's Bestiary, Garruk's Uprising, Guardian
// Project. Lower fidelity than Esper Sentinel (no opponent tax) but
// strictly own-side and triggers on every creature cast, not just the
// first.
//
// Implementation:
//   - OnTrigger("creature_spell_cast"). The engine fires this on a
//     creature spell cast by ANY seat (see stack.go:1885); filter to
//     caster_seat == perm.Controller.
//   - Action: draw 1.
//
// Pre-fix note: the "creature_spell_cast" trigger is fired AFTER
// "spell_cast" in stack.go:1880-1885, so by the time this handler runs
// the cast log already records the spell — but we don't read the log
// here (no "first creature this turn" gate), so ordering is irrelevant.
func registerBeastWhisperer(r *Registry) {
	r.OnTrigger("Beast Whisperer", "creature_spell_cast", beastWhispererOnCast)
}

func beastWhispererOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "beast_whisperer"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	drawOne(gs, perm.Controller, "Beast Whisperer")

	emit(gs, slug, "Beast Whisperer", map[string]interface{}{
		"seat": perm.Controller,
	})
}
