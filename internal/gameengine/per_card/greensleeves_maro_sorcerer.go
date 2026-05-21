package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGreensleevesMaroSorcerer wires Greensleeves, Maro-Sorcerer.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Protection from planeswalkers and from Wizards
//	Greensleeves's power and toughness are each equal to the number of
//	  lands you control.
//	Landfall — Whenever a land you control enters, create a 3/3 green
//	  Badger creature token.
//
// Implementation:
//   - "permanent_etb": land entering controlled by us. Mint a 3/3 green
//     Badger token.
//   - Protection statics and PT-equals-lands characteristic-defining
//     abilities are continuous-effect layers — emitPartial on ETB.
func registerGreensleevesMaroSorcerer(r *Registry) {
	r.OnETB("Greensleeves, Maro-Sorcerer", greensleevesETB)
	r.OnTrigger("Greensleeves, Maro-Sorcerer", "permanent_etb", greensleevesLandfall)
}

func greensleevesETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "greensleeves_cda_layer7b"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// R55 port: P/T = lands you control. Layer 7b CDA.
	gameengine.RegisterDynamicSetPT(gs, perm, greensleevesCountLands,
		gameengine.DurationUntilSourceLeaves, "Greensleeves, Maro-Sorcerer", "cda_pt")
	// Protection from planeswalkers / Wizards is engine-deep target
	// filtering; keep the partial breadcrumb for that half only.
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, "greensleeves_static", perm.Card.DisplayName(),
		"protection_from_planeswalkers_and_wizards_target_filter_not_modeled")
}

func greensleevesCountLands(gs *gameengine.GameState, perm *gameengine.Permanent) (int, int) {
	if gs == nil || perm == nil {
		return 0, 0
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return 0, 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "land") {
			n++
		}
	}
	return n, n
}

func greensleevesLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "greensleeves_landfall_badger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enter, _ := ctx["perm"].(*gameengine.Permanent)
	if enter == nil || enter.Card == nil {
		return
	}
	if enter.Controller != perm.Controller {
		return
	}
	if !enter.IsLand() {
		return
	}
	token := &gameengine.Card{
		Name:          "Badger Token",
		Owner:         perm.Controller,
		BasePower:     3,
		BaseToughness: 3,
		Types:         []string{"token", "creature", "badger"},
		Colors:        []string{"G"},
		TypeLine:      "Token Creature — Badger",
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"land": enter.Card.DisplayName(),
	})
}
