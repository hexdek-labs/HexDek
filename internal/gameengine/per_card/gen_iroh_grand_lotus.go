package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIrohGrandLotus wires Iroh, Grand Lotus.
//
// Oracle text (Scryfall, verified):
//
//	Firebending 2
//	During your turn, each non-Lesson instant and sorcery card in your
//	graveyard has flashback. The flashback cost is equal to that
//	card's mana cost.
//	During your turn, each Lesson card in your graveyard has
//	flashback {1}.
//
// Implementation (R43 stub port):
//   - Firebending 2: AST keyword pipeline (attack-trigger mana add).
//   - Flashback grants are turn-scoped: at the controller's upkeep
//     we scan the graveyard and register flashback ZoneCastPermissions
//     keyed by card. Non-Lesson I/S cards get flashback at their own
//     mana cost (-1 sentinel); Lesson cards get flashback {1}. The
//     grants are tagged with Duration "until_end_of_turn" so
//     zone_cast.go's per-turn cleanup expires them automatically at
//     turn end.
//   - We also refresh on creature_dies and permanent_ltb during the
//     controller's turn (cards entering graveyard mid-turn become
//     eligible the moment they land — matches the printed timing).
//   - The "exile after flashback resolve" rider is part of the
//     standard flashback ZoneCastPermission (ExileOnResolve: true via
//     NewFlashbackPermission). No per-card stamping needed for that.
func registerIrohGrandLotus(r *Registry) {
	r.OnETB("Iroh, Grand Lotus", irohGrandLotusETB)
	r.OnTrigger("Iroh, Grand Lotus", "upkeep", irohRefreshOnUpkeep)
	r.OnTrigger("Iroh, Grand Lotus", "creature_dies", irohRefreshOnGyChange)
	r.OnTrigger("Iroh, Grand Lotus", "permanent_ltb", irohRefreshOnGyChange)
}

func irohGrandLotusETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "iroh_grand_lotus_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// On ETB, only grant if it's actually the controller's turn —
	// otherwise we'd hand out grants to be cleaned up immediately.
	granted := 0
	if gs.Active == perm.Controller {
		granted = irohRegisterFlashbacks(gs, perm)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"granted": granted,
		"active":  gs.Active == perm.Controller,
	})
}

func irohRefreshOnUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	irohRegisterFlashbacks(gs, perm)
}

func irohRefreshOnGyChange(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if gs.Active != perm.Controller {
		return
	}
	irohRegisterFlashbacks(gs, perm)
}

// irohRegisterFlashbacks installs flashback permissions for every
// eligible I/S card in the controller's graveyard. Returns count
// installed. Duplicate registrations replace prior entries so this is
// idempotent.
func irohRegisterFlashbacks(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil || perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return 0
	}
	count := 0
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			continue
		}
		// Lesson cards: flashback {1}. Otherwise: flashback at normal
		// mana cost (-1 sentinel = use printed mana cost).
		mana := -1
		if cardHasSubtype(c, "lesson") {
			mana = 1
		}
		fb := gameengine.NewFlashbackPermission(mana)
		fb.RequireController = perm.Controller
		fb.SourceName = "Iroh, Grand Lotus"
		fb.Duration = "until_end_of_turn"
		fb.GrantTurn = gs.Turn
		gameengine.RegisterZoneCastGrant(gs, c, fb)
		count++
	}
	return count
}
