package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSaheeliRadiantCreatorCustom adds Saheeli's energy + combat
// copy triggers that the auto-gen static stub ignores.
//
// Oracle text:
//
//	Whenever you cast an Artificer or artifact spell, you get {E} (an
//	energy counter).
//	At the beginning of combat on your turn, you may pay {E}{E}{E}.
//	When you do, create a token that's a copy of target permanent you
//	control, except it's a 5/5 artifact creature in addition to its
//	other types and has haste. Sacrifice it at the beginning of the
//	next end step.
//
// Cast trigger: gates on artifact OR artificer spell types from the
// caster_seat == controller. Combat trigger: at upkeep_combat (we use
// "combat_begin"), if owner has ≥3 energy, spend it and copy the
// best permanent we control (highest CMC nontoken). Token sacrifices
// at end of turn via DelayedTrigger.
func registerSaheeliRadiantCreatorCustom(r *Registry) {
	r.OnTrigger("Saheeli, Radiant Creator", "spell_cast", saheeliEnergyOnArtifactCast)
	r.OnTrigger("Saheeli, Radiant Creator", "begin_combat", saheeliCombatCopy)
}

func saheeliEnergyOnArtifactCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "saheeli_energy_on_cast"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	// Determine spell type — accept either is_artifact bool, or a
	// types []string in ctx, or fall back to spell_name match.
	isArtifact := false
	isArtificer := false
	if v, ok := ctx["is_artifact"].(bool); ok {
		isArtifact = v
	}
	if types, ok := ctx["types"].([]string); ok {
		for _, t := range types {
			if t == "artifact" {
				isArtifact = true
			}
			if t == "artificer" {
				isArtificer = true
			}
		}
	}
	if card, ok := ctx["card"].(*gameengine.Card); ok && card != nil {
		if cardHasType(card, "artifact") {
			isArtifact = true
		}
		if cardSubtypeMatches(card, "artificer") {
			isArtificer = true
		}
	}
	if !isArtifact && !isArtificer {
		return
	}
	gameengine.GainEnergy(gs, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"is_artifact":   isArtifact,
		"is_artificer":  isArtificer,
	})
}

func saheeliCombatCopy(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "saheeli_combat_copy"
	if gs == nil || perm == nil {
		return
	}
	// Only on the controller's own turn.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		// Fall back to gs.Active if context didn't carry it.
		if gs.Active != perm.Controller {
			return
		}
	}
	if gameengine.GetEnergy(gs, perm.Controller) < 3 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Choose a copy target: highest-CMC nontoken permanent we control.
	var pick *gameengine.Permanent
	bestCMC := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsToken() {
			continue
		}
		c := cardCMC(p.Card)
		if c > bestCMC {
			pick = p
			bestCMC = c
		}
	}
	if pick == nil {
		return
	}
	if !gameengine.PayEnergy(gs, perm.Controller, 3) {
		return
	}
	// CR §707 token-copy: "a token that's a copy of [target], except it's a
	// 5/5 artifact ... and it has haste." Route through MintTokenAsCopyOf so
	// the token carries the source's FULL copiable values — including its
	// ABILITIES (AST) — then apply the P/T + type overrides. The old
	// hand-built Card dropped the AST entirely (a copy with no rules text) and
	// never fired the copied creature's ETB triggers.
	tokenCard := gameengine.MintTokenAsCopyOf(gs, pick.Card, perm.Controller, "")
	if tokenCard == nil {
		tokenCard = &gameengine.Card{
			Name:  pick.Card.DisplayName() + " (Saheeli token)",
			Owner: perm.Controller,
			Types: append([]string{"token", "artifact", "creature"}, pick.Card.Types...),
		}
	}
	tokenCard.BasePower = 5
	tokenCard.BaseToughness = 5
	for _, t := range []string{"creature", "artifact", "token"} {
		if !hasType(tokenCard.Types, t) {
			tokenCard.Types = append([]string{t}, tokenCard.Types...)
		}
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
	gameengine.RegisterReplacementsForPermanent(gs, tokenPerm)
	gameengine.FirePermanentETBTriggers(gs, tokenPerm)
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.MoveCard(gs, tokenCard, perm.Controller, "battlefield", "graveyard", "saheeli_eos_sacrifice")
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copied": pick.Card.DisplayName(),
	})
}
