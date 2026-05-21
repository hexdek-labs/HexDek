package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLoreholdArchivist wires Lorehold Archivist // Restore Relic
// (Muninn parser-gap #80, ~7.2K hits).
//
// Oracle text (Scryfall, verified 2026-05-17):
//
//	Front — Lorehold Archivist {1}{R}{W} Creature — Dwarf Artificer 2/3
//	First strike
//	At the beginning of your upkeep, if there are three or more
//	artifact and/or creature cards in your graveyard, this creature
//	becomes prepared. (While it's prepared, you may cast a copy of its
//	spell. Doing so unprepares it.)
//
//	Back — Restore Relic {2}{R}{W} Sorcery
//	Exile target artifact or creature card from your graveyard. Create
//	a token that's a copy of it.
//
// Implementation:
//   - First strike is AST-side.
//   - upkeep_controller: if controller has ≥3 (artifact|creature) cards
//     in graveyard, mark Lorehold Archivist as "prepared" via a perm
//     flag. The full Prepared mechanic (CR §702.165) needs cast-copy
//     plumbing on the stack and an "unprepare on cast" replacement —
//     that's a new engine subsystem. emitPartial flags it.
//   - As a runtime placeholder: when prepared, fire the Restore Relic
//     effect once immediately (controller "may" cast the copy — Hat
//     opts in). Pick the highest-CMC artifact/creature card in the
//     controller's graveyard, exile it, mint a token copy.
func registerLoreholdArchivist(r *Registry) {
	r.OnTrigger("Lorehold Archivist", "upkeep_controller", loreholdArchivistUpkeep)
	r.OnTrigger("Lorehold Archivist // Restore Relic", "upkeep_controller", loreholdArchivistUpkeep)
	r.OnTrigger("Lorehold Archivist", "end_step", loreholdArchivistEndStep)
	r.OnTrigger("Lorehold Archivist // Restore Relic", "end_step", loreholdArchivistEndStep)
}

// loreholdArchivistEndStep clears the "prepared" flag at end of turn so
// each turn's upkeep can re-evaluate the ≥3 yard threshold against
// current graveyard state.
func loreholdArchivistEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || perm.Flags == nil {
		return
	}
	if perm.Flags["prepared"] == 0 {
		return
	}
	delete(perm.Flags, "prepared")
	emit(gs, "lorehold_archivist_unprepare_eos", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func loreholdArchivistUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lorehold_archivist_upkeep_prepare"
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
	count := 0
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "artifact") || cardHasType(c, "creature") {
			count++
		}
	}
	if count < 3 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
			"yard_a_c":  count,
		})
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["prepared"] = 1
	// Fire the Restore Relic effect once as a runtime stand-in for
	// "may cast a copy of its spell".
	var pick *gameengine.Card
	pickCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "artifact") && !cardHasType(c, "creature") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > pickCMC {
			pickCMC = cmc
			pick = c
		}
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": true,
			"copied":    "none",
			"reason":    "yard_filtered_to_zero",
		})
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"prepared_cast_a_copy_stack_plumbing_unimplemented")
		return
	}
	gameengine.MoveCard(gs, pick, perm.Controller, "graveyard", "exile", slug)
	token := pick.DeepCopy()
	token.Owner = perm.Controller
	token.Name = pick.DisplayName() + " (Restore-Relic token)"
	token.Types = append(token.Types, "token")
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"triggered": true,
		"exiled":    pick.DisplayName(),
		"token":     token.DisplayName(),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"prepared_cast_a_copy_stack_plumbing_resolved_immediately")
}
